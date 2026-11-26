package api

import (
	"errors"
	"net/http"

	"github.com/nefocp/nefo/internal/database"
	"github.com/nefocp/nefo/internal/files"
	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/jobs"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// opError answers a failed server operation with a status and a code the UI
// can translate.
func (s *Server) opError(w http.ResponseWriter, r *http.Request, err error) {
	var ue *usererr.Error
	switch {
	case errors.As(err, &ue):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": ue.Code, "detail": ue.Detail})
	case errors.Is(err, hosting.ErrBusy), errors.Is(err, jobs.ErrBusy):
		writeError(w, http.StatusConflict, "busy")
	case errors.Is(err, hosting.ErrUnsupported), errors.Is(err, files.ErrUnsupported):
		writeError(w, http.StatusConflict, "unsupported")
	case errors.Is(err, hosting.ErrNotReady):
		writeError(w, http.StatusConflict, "stack_not_ready")
	case errors.Is(err, database.ErrNotInstalled):
		writeError(w, http.StatusConflict, "db_not_installed")
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found")
	default:
		// The panel's only users are the server's administrators, so the
		// technical detail (such as Nginx's error) is useful to show.
		s.log.Error("operation failed", "method", r.Method, "path", r.URL.Path, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "operation_failed", "detail": err.Error()})
	}
}
