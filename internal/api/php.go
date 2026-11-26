package api

import (
	"context"
	"net/http"
	"time"

	"github.com/nefocp/nefo/internal/jobs"
)

func (s *Server) handlePHPConfig(w http.ResponseWriter, r *http.Request, a *authed) {
	cfg, err := s.software.PHPConfig(r.Context(), r.PathValue("version"))
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// handlePHPExtension installs (POST) or removes (DELETE) an extension.
func (s *Server) handlePHPExtension(w http.ResponseWriter, r *http.Request, a *authed) {
	v, ext := r.PathValue("version"), r.PathValue("ext")
	install := r.Method == http.MethodPost
	user, ip := a.user.Username, clientIP(r)
	job, err := s.jobs.Start("php_extension", "php"+v+"-"+ext, 20*time.Minute, func(ctx context.Context, j *jobs.Job) error {
		var err error
		action := "php_extension_installed"
		if install {
			err = s.software.InstallExtension(ctx, v, ext, j)
		} else {
			action = "php_extension_removed"
			err = s.software.RemoveExtension(ctx, v, ext, j)
		}
		if err == nil {
			s.audit(ctx, user, action, ip, "php"+v+"-"+ext)
		}
		return err
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

func (s *Server) handlePHPIni(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Ini map[string]string `json:"ini"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	v := r.PathValue("version")
	if err := s.software.SetPHPIni(r.Context(), v, req.Ini); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "php_ini_changed", clientIP(r), "php"+v)
	cfg, err := s.software.PHPConfig(r.Context(), v)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}
