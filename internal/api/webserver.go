package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/jobs"
)

// webServerError answers a refused web server install or removal: 409 with
// the code for a conflict (web_server_conflict, web_server_in_use), else as
// opError does.
func (s *Server) webServerError(w http.ResponseWriter, r *http.Request, err error) {
	var ce *hosting.ConflictError
	if errors.As(err, &ce) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": ce.Code, "detail": ce.Detail})
		return
	}
	s.opError(w, r, err)
}

// handleWebRemove uninstalls the web server as a job: {"confirm":
// "nginx"|"apache"|"openlitespeed"}. It is refused while any website exists, in this panel
// or as a file in the web server's sites folder.
func (s *Server) handleWebRemove(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Confirm string `json:"confirm"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	kind, err := s.hosting.CheckRemove(r.Context(), req.Confirm)
	if err != nil {
		s.webServerError(w, r, err)
		return
	}
	user, ip := a.user.Username, clientIP(r)
	job, err := s.jobs.Start("remove_web", kind, 30*time.Minute, func(ctx context.Context, j *jobs.Job) error {
		err := s.hosting.RemoveServer(ctx, kind, j)
		if err == nil {
			s.audit(ctx, user, "web_removed", ip, kind)
		}
		return err
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}
