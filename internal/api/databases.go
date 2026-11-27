package api

import (
	"compress/gzip"
	"context"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/database"
	"github.com/nefocp/nefo/internal/jobs"
	"github.com/nefocp/nefo/internal/usererr"
)

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

// handleDBStatus answers GET /api/db/status: the database server's state,
// with "engine" ("mariadb", "mysql" or "" while none is installed) and,
// for the install choice, why MySQL is not offered ("mysql_unavailable",
// null when it is).
func (s *Server) handleDBStatus(w http.ResponseWriter, r *http.Request, a *authed) {
	st := s.db.Status(r.Context())
	writeJSON(w, http.StatusOK, struct {
		database.Status
		MySQLUnavailable *reason `json:"mysql_unavailable"`
	}{st, s.mysqlUnavailable(st)})
}

// handleDBInstall installs the database server: {"engine": "mariadb"} (the
// default, also for an empty body), with "series": "11.4" for a series from
// MariaDB's repository, or {"engine": "mysql"} (dbengine.go).
func (s *Server) handleDBInstall(w http.ResponseWriter, r *http.Request, a *authed) {
	user, ip := a.user.Username, clientIP(r)
	var req struct {
		Engine string `json:"engine"`
		Series string `json:"series"`
	}
	if r.ContentLength != 0 && !decodeJSON(w, r, &req) {
		return
	}
	if req.Engine == "" {
		req.Engine = string(database.MariaDB)
	}
	id := req.Engine
	if req.Series != "" && req.Engine == string(database.MariaDB) {
		id = database.SeriesID(req.Series)
	}
	e, ok := dbEngineOf(id)
	if !ok || (req.Series != "" && e != database.MariaDB) {
		s.opError(w, r, usererr.New("unknown_software", id))
		return
	}
	if err := s.checkDBInstall(r.Context(), id); err != nil {
		s.opError(w, r, err)
		return
	}
	name, _ := softwareName(id)
	job, err := s.jobs.Start("install_db", name, 30*time.Minute, func(ctx context.Context, j *jobs.Job) error {
		err := s.installSoftware(ctx, id, j)
		if err == nil {
			s.audit(ctx, user, "db_installed", ip, string(e))
		}
		return err
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

func (s *Server) handleListDatabases(w http.ResponseWriter, r *http.Request, a *authed) {
	list, err := s.db.List(r.Context())
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"databases": list})
}

type credentials struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
}

func (s *Server) handleCreateDatabase(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Name     string `json:"name"`
		Password string `json:"password"`
		SiteID   int64  `json:"site_id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	d, err := s.db.Create(r.Context(), req.Name, req.Password, req.SiteID)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "db_created", clientIP(r), d.Name)
	writeJSON(w, http.StatusCreated, credentials{d.ID, d.Name, d.Username, d.Password, "localhost"})
}

func (s *Server) handleDBCredentials(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	d, err := s.store.DatabaseByID(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, credentials{d.ID, d.Name, d.Username, d.Password, "localhost"})
}

func (s *Server) handleDBPassword(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	d, err := s.db.SetPassword(r.Context(), id, req.Password)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	// A WordPress site that uses this database needs the new password too.
	if d.SiteID != 0 {
		if err := s.hosting.UpdateDBPassword(r.Context(), d.SiteID, d.Name, d.Password); err != nil {
			s.opError(w, r, err)
			return
		}
	}
	s.audit(r.Context(), a.user.Username, "db_password_changed", clientIP(r), d.Name)
	writeJSON(w, http.StatusOK, credentials{d.ID, d.Name, d.Username, d.Password, "localhost"})
}

func (s *Server) handleDeleteDatabase(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	var req struct {
		Confirm string `json:"confirm"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.db.Delete(r.Context(), id, req.Confirm); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "db_deleted", clientIP(r), req.Confirm)
	s.store.DeleteSchedulesFor(r.Context(), "database", req.Confirm)
	res := map[string]any{"ok": true}
	if job := s.restartDBIfNeeded(r.Context(), a.user.Username, clientIP(r)); job != nil {
		res["job"] = job.Snapshot() // DatabasesView watches it
	}
	writeJSON(w, http.StatusOK, res)
}

// handleExportDatabase streams an SQL dump of a database as a download.
func (s *Server) handleExportDatabase(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	d, err := s.store.DatabaseByID(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	http.NewResponseController(w).SetWriteDeadline(time.Now().Add(6 * time.Hour))
	name := d.Name + "_" + time.Now().Format("20060102-150405") + ".sql"
	h := w.Header()
	h.Set("Content-Type", "application/octet-stream")
	h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	s.audit(r.Context(), a.user.Username, "db_exported", clientIP(r), d.Name)
	if err := s.db.Dump(r.Context(), d.Name, w); err != nil {
		s.log.Warn("export database", "db", d.Name, "err", err)
	}
}

// handleImportDatabase runs an uploaded .sql or .sql.gz file against a
// database, optionally emptying it first.
func (s *Server) handleImportDatabase(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	d, err := s.store.DatabaseByID(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	rc := http.NewResponseController(w)
	rc.SetReadDeadline(time.Now().Add(6 * time.Hour))
	rc.SetWriteDeadline(time.Now().Add(6 * time.Hour))
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
	mr, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	for {
		part, err := mr.NextPart()
		if err != nil {
			writeError(w, http.StatusBadRequest, "bad_request")
			return
		}
		if part.FormName() != "file" {
			part.Close()
			continue
		}
		var src io.Reader = part
		name := strings.ToLower(part.FileName())
		switch {
		case strings.HasSuffix(name, ".sql.gz"):
			gz, err := gzip.NewReader(part)
			if err != nil {
				s.opError(w, r, usererr.New("invalid_sql_file"))
				return
			}
			defer gz.Close()
			src = gz
		case !strings.HasSuffix(name, ".sql"):
			s.opError(w, r, usererr.New("invalid_sql_file"))
			return
		}
		if r.URL.Query().Get("reset") == "1" {
			if err := s.db.Reset(r.Context(), d.Name); err != nil {
				s.opError(w, r, err)
				return
			}
		}
		if err := s.db.Import(r.Context(), d.Name, src); err != nil {
			s.opError(w, r, err)
			return
		}
		s.audit(r.Context(), a.user.Username, "db_imported", clientIP(r), d.Name+" "+part.FileName())
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
}
