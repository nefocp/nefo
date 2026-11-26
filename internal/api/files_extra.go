package api

import (
	"context"
	"net/http"
	"time"

	"github.com/nefocp/nefo/internal/jobs"
)

// File manager extras: download from a URL, search, permanent delete and
// the trash. Plain deletes (POST /api/files/delete) now go to the trash.

// handleDeletePermanently removes items without going through the trash,
// for example very large files.
func (s *Server) handleDeletePermanently(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Paths []string `json:"paths"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	http.NewResponseController(w).SetWriteDeadline(time.Now().Add(10 * time.Minute))
	err := s.files.DeletePermanently(r.Context(), req.Paths)
	if len(req.Paths) > 0 {
		s.fileAudit(r, a, "file_deleted_permanently", req.Paths...)
	}
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleFetchURL downloads a file from the internet into a folder, as a job.
// The URL and its host are checked first, so mistakes get a clear answer
// at once; the download itself checks every address it connects to again.
func (s *Server) handleFetchURL(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Dir string `json:"dir"`
		URL string `json:"url"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	u, err := s.files.CheckFetch(r.Context(), req.Dir, req.URL)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	user, ip, dir := a.user.Username, clientIP(r), req.Dir
	job, err := s.jobs.Start("fetch_url", truncate(u.Redacted(), 120), 2*time.Hour, func(ctx context.Context, j *jobs.Job) error {
		p, err := s.files.Fetch(ctx, dir, u, j)
		if err == nil {
			s.audit(ctx, user, "file_fetched", ip, truncate(u.Redacted()+" → "+p, 500))
		}
		return err
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

// handleSearchFiles finds files and folders under a folder by name and,
// optionally, by content.
func (s *Server) handleSearchFiles(w http.ResponseWriter, r *http.Request, a *authed) {
	q := r.URL.Query()
	res, err := s.files.Search(r.Context(), q.Get("path"), q.Get("name"), q.Get("content"))
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// handleListTrash lists the trash of the website (or the server) that
// holds ?path=.
func (s *Server) handleListTrash(w http.ResponseWriter, r *http.Request, a *authed) {
	l, err := s.files.ListTrash(r.Context(), r.URL.Query().Get("path"))
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, l)
}

type trashRequest struct {
	Path string `json:"path"` // any path in the scope whose trash is meant
	ID   string `json:"id"`
}

func (s *Server) handleRestoreTrash(w http.ResponseWriter, r *http.Request, a *authed) {
	var req trashRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	http.NewResponseController(w).SetWriteDeadline(time.Now().Add(10 * time.Minute))
	p, err := s.files.RestoreTrash(r.Context(), req.Path, req.ID)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.fileAudit(r, a, "trash_restored", p)
	writeJSON(w, http.StatusOK, map[string]string{"path": p})
}

func (s *Server) handleDeleteTrash(w http.ResponseWriter, r *http.Request, a *authed) {
	var req trashRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.files.DeleteTrash(r.Context(), req.Path, req.ID); err != nil {
		s.opError(w, r, err)
		return
	}
	s.fileAudit(r, a, "trash_deleted", req.Path+" "+req.ID)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleEmptyTrash(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Path string `json:"path"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	http.NewResponseController(w).SetWriteDeadline(time.Now().Add(10 * time.Minute))
	n, err := s.files.EmptyTrash(r.Context(), req.Path)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.fileAudit(r, a, "trash_emptied", req.Path)
	writeJSON(w, http.StatusOK, map[string]int{"removed": n})
}
