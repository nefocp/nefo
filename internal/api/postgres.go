package api

import (
	"compress/gzip"
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/jobs"
	"github.com/nefocp/nefo/internal/postgres"
	"github.com/nefocp/nefo/internal/software"
	"github.com/nefocp/nefo/internal/usererr"
)

// PostgreSQL on the Databases page (its own tab), and Memcached's status.
// Installing and removing both are on the Software page (system_tools.go).

func (s *Server) postgresRoutes(mux *http.ServeMux) {
	mux.Handle("GET /api/postgres/status", s.requireAuth(s.handlePGStatus))
	mux.Handle("POST /api/postgres/restart", s.requireAuth(s.handlePGRestart))
	mux.Handle("GET /api/postgres/databases", s.requireAuth(s.handlePGList))
	mux.Handle("POST /api/postgres/databases", s.requireAuth(s.handlePGCreate))
	mux.Handle("GET /api/postgres/databases/{id}/credentials", s.requireAuth(s.handlePGCredentials))
	mux.Handle("POST /api/postgres/databases/{id}/password", s.requireAuth(s.handlePGPassword))
	mux.Handle("DELETE /api/postgres/databases/{id}", s.requireAuth(s.handlePGDelete))
	mux.Handle("GET /api/postgres/databases/{id}/export", s.requireAuth(s.handlePGExport))
	mux.Handle("POST /api/postgres/databases/{id}/import", s.requireAuth(s.handlePGImport))
	mux.Handle("GET /api/postgres/databases/{id}/remote", s.requireAuth(s.handlePGRemote))
	mux.Handle("PUT /api/postgres/databases/{id}/remote", s.requireAuth(s.handleSetPGRemote))
	mux.Handle("GET /api/postgres/versions", s.requireAuth(s.handlePGVersions))
	mux.Handle("POST /api/postgres/upgrade", s.requireAuth(s.handlePGUpgrade))
	mux.Handle("GET /api/postgres/dumps", s.requireAuth(s.handlePGDumps))
	mux.Handle("DELETE /api/postgres/dumps/{name}", s.requireAuth(s.handleDeletePGDump))
	mux.Handle("GET /api/memcached", s.requireAuth(s.handleMemcached))
	mux.Handle("POST /api/memcached/restart", s.requireAuth(s.handleMemcachedRestart))
}

func (s *Server) handlePGStatus(w http.ResponseWriter, r *http.Request, a *authed) {
	writeJSON(w, http.StatusOK, s.pg.Status(r.Context()))
}

func (s *Server) handlePGRestart(w http.ResponseWriter, r *http.Request, a *authed) {
	job, err := s.startPGRestart(a.user.Username, clientIP(r), "")
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

func (s *Server) startPGRestart(user, ip, why string) (*jobs.Job, error) {
	return s.jobs.Start("pg_restart", "PostgreSQL", 5*time.Minute, func(ctx context.Context, j *jobs.Job) error {
		err := s.pg.Restart(ctx, j)
		if err == nil {
			s.audit(ctx, user, "pg_restarted", ip, why)
		}
		return err
	})
}

// restartPGIfNeeded restarts PostgreSQL in a job when its listen setting
// changed, for example after the last database with remote access was
// deleted. It returns the job, or nil when no restart is needed or another
// job is running (PostgreSQL then keeps listening on all interfaces until
// its next restart; pg_hba.conf and the firewall still keep everyone out).
func (s *Server) restartPGIfNeeded(ctx context.Context, user, ip string) *jobs.Job {
	if !s.pg.RestartPending(ctx) {
		return nil
	}
	job, err := s.startPGRestart(user, ip, "listen setting")
	if err != nil {
		s.log.Warn("PostgreSQL must restart to apply its listen setting", "err", err)
		return nil
	}
	return job
}

// ---- remote access ----

func (s *Server) handlePGRemote(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	ra, err := s.pg.Remote(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, ra)
}

// handleSetPGRemote replaces the client addresses that may connect to a
// PostgreSQL database from other machines; an empty list turns remote
// access off. When PostgreSQL must restart to change where it listens, the
// restart runs as a job, which the answer includes.
func (s *Server) handleSetPGRemote(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	var req struct {
		Hosts []string `json:"hosts"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	d, err := s.store.PGDatabaseByID(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	restart, err := s.pg.SetRemote(r.Context(), id, req.Hosts)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	user, ip := a.user.Username, clientIP(r)
	hosts, _ := postgres.ParseRemoteHosts(req.Hosts) // already checked by SetRemote
	detail := d.Name + ": off"
	if len(hosts) > 0 {
		detail = d.Name + ": " + strings.Join(hosts, " ")
	}
	s.audit(r.Context(), user, "pg_db_remote_access", ip, detail)
	ra, err := s.pg.Remote(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	res := map[string]any{"remote": ra}
	if restart {
		job, err := s.startPGRestart(user, ip, "listen setting")
		switch {
		case err == nil:
			res["job"] = job.Snapshot()
		case errors.Is(err, jobs.ErrBusy):
			// Another job runs: the state shows the restart as pending, and
			// the administrator can start it later (POST /api/postgres/restart).
		default:
			s.opError(w, r, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, res)
}

// ---- versions ----

func (s *Server) handlePGVersions(w http.ResponseWriter, r *http.Request, a *authed) {
	writeJSON(w, http.StatusOK, s.pg.Versions(r.Context()))
}

// handlePGUpgrade moves the cluster to a newer major version from
// apt.postgresql.org, as a job: {"version": "17"}.
func (s *Server) handlePGUpgrade(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Version string `json:"version"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	v := s.pg.Versions(r.Context())
	if !slices.Contains(v.Upgrades, req.Version) {
		s.opError(w, r, usererr.New("pg_version_unknown", req.Version))
		return
	}
	if v.UpgradeBlocked != "" {
		s.opError(w, r, usererr.New(v.UpgradeBlocked, v.UpgradeDetail))
		return
	}
	user, ip, from := a.user.Username, clientIP(r), v.Installed
	job, err := s.jobs.Start("pg_upgrade", "PostgreSQL "+req.Version, 3*time.Hour, func(ctx context.Context, j *jobs.Job) error {
		err := s.pg.Upgrade(ctx, req.Version, j)
		if err == nil {
			s.audit(ctx, user, "pg_upgraded", ip, from+" to "+req.Version)
		}
		return err
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

// handlePGDumps lists the dumps upgrades kept (GET /api/postgres/dumps).
func (s *Server) handlePGDumps(w http.ResponseWriter, r *http.Request, a *authed) {
	writeJSON(w, http.StatusOK, map[string]any{"dumps": s.pg.UpgradeDumps()})
}

// handleDeletePGDump deletes one (DELETE /api/postgres/dumps/{name}).
func (s *Server) handleDeletePGDump(w http.ResponseWriter, r *http.Request, a *authed) {
	name := r.PathValue("name")
	if err := s.pg.DeleteUpgradeDump(name); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "pg_dump_deleted", clientIP(r), name)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handlePGList(w http.ResponseWriter, r *http.Request, a *authed) {
	list, err := s.pg.List(r.Context())
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"databases": list})
}

type pgCredentials struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
}

func (s *Server) pgCredentials(ctx context.Context, id int64, name, user, password string) pgCredentials {
	port := s.pg.Status(ctx).Port
	if port == 0 {
		port = 5432
	}
	return pgCredentials{id, name, user, password, "localhost", port}
}

func (s *Server) handlePGCreate(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Name     string `json:"name"`
		Password string `json:"password"`
		SiteID   int64  `json:"site_id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	d, err := s.pg.Create(r.Context(), req.Name, req.Password, req.SiteID)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "pg_db_created", clientIP(r), d.Name)
	writeJSON(w, http.StatusCreated, s.pgCredentials(r.Context(), d.ID, d.Name, d.Username, d.Password))
}

func (s *Server) handlePGCredentials(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	d, err := s.store.PGDatabaseByID(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, s.pgCredentials(r.Context(), d.ID, d.Name, d.Username, d.Password))
}

func (s *Server) handlePGPassword(w http.ResponseWriter, r *http.Request, a *authed) {
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
	d, err := s.pg.SetPassword(r.Context(), id, req.Password)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "pg_db_password_changed", clientIP(r), d.Name)
	writeJSON(w, http.StatusOK, s.pgCredentials(r.Context(), d.ID, d.Name, d.Username, d.Password))
}

func (s *Server) handlePGDelete(w http.ResponseWriter, r *http.Request, a *authed) {
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
	if err := s.pg.Delete(r.Context(), id, req.Confirm); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "pg_db_deleted", clientIP(r), req.Confirm)
	s.store.DeleteSchedulesFor(r.Context(), "database", strings.TrimSpace(req.Confirm))
	res := map[string]any{"ok": true}
	// PostgreSQL restarts when the last database with remote access goes.
	if job := s.restartPGIfNeeded(r.Context(), a.user.Username, clientIP(r)); job != nil {
		res["job"] = job.Snapshot()
	}
	writeJSON(w, http.StatusOK, res)
}

// handlePGExport streams a gzipped plain SQL dump of a database.
func (s *Server) handlePGExport(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	d, err := s.store.PGDatabaseByID(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	http.NewResponseController(w).SetWriteDeadline(time.Now().Add(6 * time.Hour))
	name := d.Name + "_" + time.Now().Format("20060102-150405") + ".sql.gz"
	h := w.Header()
	h.Set("Content-Type", "application/gzip")
	h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	s.audit(r.Context(), a.user.Username, "pg_db_exported", clientIP(r), d.Name)
	gz := gzip.NewWriter(w)
	if err := s.pg.Dump(r.Context(), d.Name, gz); err != nil {
		s.log.Warn("export PostgreSQL database", "db", d.Name, "err", err)
		return // the download ends without gzip's trailer, so it shows as broken
	}
	if err := gz.Close(); err != nil {
		s.log.Warn("export PostgreSQL database", "db", d.Name, "err", err)
	}
}

// handlePGImport runs an uploaded .sql or .sql.gz file against a database
// as its own role, optionally emptying it first.
func (s *Server) handlePGImport(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	d, err := s.store.PGDatabaseByID(r.Context(), id)
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
			if err := s.pg.Reset(r.Context(), d.Name); err != nil {
				s.opError(w, r, err)
				return
			}
		}
		if err := s.pg.Import(r.Context(), d.Name, src); err != nil {
			s.opError(w, r, err)
			return
		}
		s.audit(r.Context(), a.user.Username, "pg_db_imported", clientIP(r), d.Name+" "+part.FileName())
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
}

// installPostgres installs PostgreSQL (major "": the distribution's, or the
// one installed), then lets Adminer open its databases.
func (s *Server) installPostgres(ctx context.Context, major string, lg software.Logger) error {
	if err := s.pg.Install(ctx, major, lg); err != nil {
		return err
	}
	return s.adminerPostgres(ctx, lg)
}

// adminerPostgres gives Adminer what it needs for PostgreSQL, when both are
// installed: the pinned build with every driver (earlier panels installed
// the MySQL-only one) and PHP's pgsql extension, which is only added
// together with PostgreSQL.
func (s *Server) adminerPostgres(ctx context.Context, lg software.Logger) error {
	if !postgres.Installed() || !s.hosting.AdminerInstalled() {
		return nil
	}
	if !s.hosting.AdminerCurrent() {
		lg.Logf("Updating Adminer to the build that also opens PostgreSQL databases")
		if err := s.hosting.InstallAdminer(ctx, lg); err != nil {
			return err
		}
	}
	v := s.hosting.AdminerPHP(ctx)
	if v == "" {
		return nil
	}
	lg.Logf("Adding PHP %s's pgsql extension, which Adminer needs for PostgreSQL", v)
	return s.software.InstallExtension(ctx, v, "pgsql", lg)
}

// ---- Memcached ----

func (s *Server) handleMemcached(w http.ResponseWriter, r *http.Request, a *authed) {
	writeJSON(w, http.StatusOK, s.software.Memcached(r.Context()))
}

func (s *Server) handleMemcachedRestart(w http.ResponseWriter, r *http.Request, a *authed) {
	if err := s.software.RestartMemcached(r.Context()); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "service_restart", clientIP(r), "memcached")
	writeJSON(w, http.StatusOK, s.software.Memcached(r.Context()))
}
