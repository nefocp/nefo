package api

import (
	"context"
	"io"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/files"
	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/jobs"
)

const maxUpload = 4 << 30 // per request

// fileScope confines operations inside a website's folder to that folder and
// gives new files to the site's user. Elsewhere the administrator works as
// root on the whole file system.
func (s *Server) fileScope(ctx context.Context, abs string) (files.Scope, error) {
	sites, err := s.store.ListSites(ctx)
	if err != nil {
		return files.Scope{}, err
	}
	for _, site := range sites {
		root := s.hosting.Root(site.Domain)
		if abs == root || strings.HasPrefix(abs, root+"/") {
			uid, gid, err := hosting.LookupIDs(site.SysUser)
			if err != nil {
				return files.Scope{}, err
			}
			// While the site's files are locked, only its writable
			// folders change (see hosting/filelock.go).
			guard := func(rel string, self bool) error { return s.hosting.CheckWritable(ctx, site, rel, self) }
			return files.Scope{Root: root, UID: uid, GID: gid, Site: site.Domain, Guard: guard}, nil
		}
	}
	return files.Scope{Root: "/", UID: 0, GID: 0}, nil
}

func (s *Server) fileAudit(r *http.Request, a *authed, action string, paths ...string) {
	s.audit(r.Context(), a.user.Username, action, clientIP(r), truncate(strings.Join(paths, ", "), 500))
}

func (s *Server) handleListFiles(w http.ResponseWriter, r *http.Request, a *authed) {
	l, err := s.files.List(r.Context(), r.URL.Query().Get("path"))
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, l)
}

func (s *Server) handleReadFile(w http.ResponseWriter, r *http.Request, a *authed) {
	text, mod, err := s.files.ReadText(r.Context(), r.URL.Query().Get("path"))
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"content": text, "modified": mod})
}

func (s *Server) handleWriteFile(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Create  bool   `json:"create"`
	}
	if !decodeJSONLimit(w, r, &req, 4*files.MaxEditSize) {
		return
	}
	if err := s.files.WriteText(r.Context(), req.Path, req.Content, req.Create); err != nil {
		s.opError(w, r, err)
		return
	}
	s.fileAudit(r, a, "file_saved", req.Path)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleMkdir(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Path string `json:"path"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.files.Mkdir(r.Context(), req.Path); err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleRenameFile(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.files.Rename(r.Context(), req.From, req.To); err != nil {
		s.opError(w, r, err)
		return
	}
	s.fileAudit(r, a, "file_renamed", req.From, req.To)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteFiles(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Paths []string `json:"paths"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	err := s.files.Delete(r.Context(), req.Paths)
	if len(req.Paths) > 0 {
		s.fileAudit(r, a, "file_deleted", req.Paths...)
	}
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleChmod(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Path string `json:"path"`
		Mode string `json:"mode"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.files.Chmod(r.Context(), req.Path, req.Mode); err != nil {
		s.opError(w, r, err)
		return
	}
	s.fileAudit(r, a, "file_chmod", req.Path+" "+req.Mode)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleUpload streams multipart files to disk without holding them in memory.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request, a *authed) {
	// Large uploads take longer than the server's normal 30 second limit.
	rc := http.NewResponseController(w)
	rc.SetReadDeadline(time.Now().Add(2 * time.Hour))
	rc.SetWriteDeadline(time.Now().Add(2 * time.Hour))
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload)

	dir := r.URL.Query().Get("dir")
	overwrite := r.URL.Query().Get("overwrite") == "1"
	mr, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	var names []string
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.opError(w, r, err)
			return
		}
		if part.FormName() != "file" || part.FileName() == "" {
			part.Close()
			continue
		}
		name := path.Base(strings.ReplaceAll(part.FileName(), "\\", "/"))
		_, err = s.files.Upload(r.Context(), dir, name, part, overwrite)
		part.Close()
		if err != nil {
			s.opError(w, r, err)
			return
		}
		names = append(names, path.Join(dir, name))
	}
	if len(names) > 0 {
		s.fileAudit(r, a, "file_uploaded", names...)
	}
	writeJSON(w, http.StatusOK, map[string]any{"uploaded": names})
}

// handleDownload sends a file as an attachment. It is never shown inline: an
// HTML file from a website must not run in the panel's origin.
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request, a *authed) {
	f, fi, err := s.files.OpenFile(r.Context(), r.URL.Query().Get("path"))
	if err != nil {
		s.opError(w, r, err)
		return
	}
	defer f.Close()
	http.NewResponseController(w).SetWriteDeadline(time.Now().Add(2 * time.Hour))
	h := w.Header()
	h.Set("Content-Type", "application/octet-stream")
	h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": fi.Name()}))
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	h.Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "", fi.ModTime(), f)
}

func (s *Server) handleCompress(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Paths []string `json:"paths"`
		Dest  string   `json:"dest"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	user, ip := a.user.Username, clientIP(r)
	job, err := s.jobs.Start("compress", path.Base(req.Dest), 2*time.Hour, func(ctx context.Context, j *jobs.Job) error {
		err := s.files.Compress(ctx, req.Paths, req.Dest, j)
		if err == nil {
			s.audit(ctx, user, "files_compressed", ip, truncate(req.Dest, 500))
		}
		return err
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

func (s *Server) handleExtract(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Path string `json:"path"`
		Dest string `json:"dest"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	user, ip := a.user.Username, clientIP(r)
	job, err := s.jobs.Start("extract", path.Base(req.Path), 2*time.Hour, func(ctx context.Context, j *jobs.Job) error {
		err := s.files.Extract(ctx, req.Path, req.Dest, j)
		if err == nil {
			s.audit(ctx, user, "files_extracted", ip, truncate(req.Path+" → "+req.Dest, 500))
		}
		return err
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}
