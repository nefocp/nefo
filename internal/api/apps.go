package api

import (
	"context"
	"net/http"
	"time"

	"github.com/nefocp/nefo/internal/docker"
	"github.com/nefocp/nefo/internal/jobs"
)

func (s *Server) handleAppAction(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	action := r.PathValue("action")
	if err := s.hosting.AppAction(r.Context(), id, action); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "app_"+action, clientIP(r), r.PathValue("id"))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleAppLogs(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	text, err := s.hosting.AppLogs(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"log": text})
}

func (s *Server) handleDocker(w http.ResponseWriter, r *http.Request, a *authed) {
	if !s.docker.Installed() {
		writeJSON(w, http.StatusOK, map[string]any{"installed": false})
		return
	}
	containers, err := s.docker.Containers(r.Context())
	if err != nil {
		s.opError(w, r, err)
		return
	}
	images, err := s.docker.Images(r.Context())
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"installed": true, "containers": containers, "images": images})
}

func (s *Server) handleDockerAction(w http.ResponseWriter, r *http.Request, a *authed) {
	id, action := r.PathValue("id"), r.PathValue("action")
	if err := s.docker.Action(r.Context(), id, action); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "container_"+action, clientIP(r), id)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDockerLogs(w http.ResponseWriter, r *http.Request, a *authed) {
	text, err := s.docker.Logs(r.Context(), r.PathValue("id"))
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"log": text})
}

func (s *Server) handleDockerRun(w http.ResponseWriter, r *http.Request, a *authed) {
	var req docker.RunRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := req.Check(); err != nil {
		s.opError(w, r, err)
		return
	}
	user, ip := a.user.Username, clientIP(r)
	job, err := s.jobs.Start("docker_run", req.Name, 30*time.Minute, func(ctx context.Context, j *jobs.Job) error {
		j.Logf("Starting %s from %s", req.Name, req.Image)
		err := s.docker.Run(ctx, req, j)
		if err == nil {
			s.audit(ctx, user, "container_created", ip, req.Name+" "+req.Image)
		}
		return err
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}
