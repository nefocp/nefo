package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/nefocp/nefo/internal/docker"
	"github.com/nefocp/nefo/internal/jobs"
	"github.com/nefocp/nefo/internal/usererr"
)

// Compose projects and images. The container terminal is in terminal.go.

// dockerError answers a failed Docker operation.
func (s *Server) dockerError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, docker.ErrNoProject) {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	s.opError(w, r, err)
}

// needDocker answers docker_needed and returns false when Docker is missing.
func (s *Server) needDocker(w http.ResponseWriter, r *http.Request) bool {
	if s.docker.Installed() {
		return true
	}
	s.opError(w, r, usererr.New("docker_needed"))
	return false
}

// startDockerJob runs fn as a job and answers 202 with it. The Compose
// plugin is installed first when fn needs it and it is missing.
func (s *Server) startDockerJob(w http.ResponseWriter, r *http.Request, kind, target string, compose bool,
	fn func(ctx context.Context, j *jobs.Job) error) {
	job, err := s.jobs.Start(kind, target, 30*time.Minute, func(ctx context.Context, j *jobs.Job) error {
		if compose {
			if err := s.docker.EnsureCompose(ctx, j); err != nil {
				return err
			}
		}
		return fn(ctx, j)
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job": job.Snapshot()})
}

// handleListCompose lists the Compose projects.
func (s *Server) handleListCompose(w http.ResponseWriter, r *http.Request, a *authed) {
	if !s.docker.Installed() {
		writeJSON(w, http.StatusOK, map[string]any{"installed": false, "projects": []docker.Project{}})
		return
	}
	projects, err := s.docker.Projects(r.Context())
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"installed": true, "compose_installed": s.docker.ComposeInstalled(r.Context()), "projects": projects,
		"max_size": docker.MaxComposeSize,
	})
}

// handleGetCompose returns a project's compose.yaml and its containers.
func (s *Server) handleGetCompose(w http.ResponseWriter, r *http.Request, a *authed) {
	name := r.PathValue("name")
	text, err := s.docker.ProjectFile(name)
	if err != nil {
		s.dockerError(w, r, err)
		return
	}
	containers := []docker.ProjectContainer{}
	if s.docker.Installed() && s.docker.ComposeInstalled(r.Context()) {
		if containers, err = s.docker.ProjectContainers(r.Context(), name); err != nil {
			s.dockerError(w, r, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": name, "yaml": text, "containers": containers})
}

// handleSaveCompose creates a project (POST, with its name in the body) or
// replaces its compose.yaml (PUT), then starts it. When the Compose plugin
// is installed, the checks run before answering, so the editor can show
// what is refused; the job checks again before saving.
func (s *Server) handleSaveCompose(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Name string `json:"name"`
		YAML string `json:"yaml"`
	}
	if !decodeJSONLimit(w, r, &req, 4*docker.MaxComposeSize) {
		return
	}
	name, create := r.PathValue("name"), r.Method == http.MethodPost
	if create {
		name = req.Name
	}
	if err := s.docker.ValidateProject(name, req.YAML, create); err != nil {
		s.dockerError(w, r, err)
		return
	}
	if !s.needDocker(w, r) {
		return
	}
	if s.docker.ComposeInstalled(r.Context()) {
		if err := s.docker.CheckText(r.Context(), name, req.YAML); err != nil {
			s.opError(w, r, err)
			return
		}
	}
	kind, action := "compose_save", "compose_updated"
	if create {
		kind, action = "compose_create", "compose_created"
	}
	user, ip := a.user.Username, clientIP(r)
	s.startDockerJob(w, r, kind, name, true, func(ctx context.Context, j *jobs.Job) error {
		j.Logf("Checking the new compose.yaml")
		if err := s.docker.CheckText(ctx, name, req.YAML); err != nil {
			return err
		}
		if err := s.docker.SaveProject(name, req.YAML); err != nil {
			return err
		}
		s.audit(ctx, user, action, ip, name)
		return s.docker.Up(ctx, name, j)
	})
}

// handleComposeAction runs start, stop, restart, down or update as a job.
func (s *Server) handleComposeAction(w http.ResponseWriter, r *http.Request, a *authed) {
	name, action := r.PathValue("name"), r.PathValue("action")
	if err := s.docker.CheckAction(name, action); err != nil {
		s.dockerError(w, r, err)
		return
	}
	if !s.needDocker(w, r) {
		return
	}
	user, ip := a.user.Username, clientIP(r)
	s.startDockerJob(w, r, "compose_"+action, name, true, func(ctx context.Context, j *jobs.Job) error {
		if err := s.docker.ProjectAction(ctx, name, action, j); err != nil {
			return err
		}
		s.audit(ctx, user, "compose_"+action, ip, name)
		return nil
	})
}

// handleDeleteCompose removes a project's containers and folder, and its
// volumes when asked.
func (s *Server) handleDeleteCompose(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Volumes bool `json:"volumes"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	name := r.PathValue("name")
	if err := s.docker.CheckAction(name, "down"); err != nil {
		s.dockerError(w, r, err)
		return
	}
	if !s.needDocker(w, r) {
		return
	}
	user, ip := a.user.Username, clientIP(r)
	s.startDockerJob(w, r, "compose_delete", name, true, func(ctx context.Context, j *jobs.Job) error {
		if err := s.docker.DeleteProject(ctx, name, req.Volumes, j); err != nil {
			return err
		}
		detail := name
		if req.Volumes {
			detail += " (with volumes)"
		}
		s.audit(ctx, user, "compose_deleted", ip, detail)
		return nil
	})
}

// handleComposeLogs returns the end of a project's container logs.
func (s *Server) handleComposeLogs(w http.ResponseWriter, r *http.Request, a *authed) {
	if !s.needDocker(w, r) {
		return
	}
	text, err := s.docker.ProjectLogs(r.Context(), r.PathValue("name"))
	if err != nil {
		s.dockerError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"log": text})
}

// handleListImages lists local images and the containers using them.
func (s *Server) handleListImages(w http.ResponseWriter, r *http.Request, a *authed) {
	if !s.needDocker(w, r) {
		return
	}
	images, err := s.docker.Images(r.Context())
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"images": images})
}

// handleRemoveImage removes an image; one that a container uses only with
// force.
func (s *Server) handleRemoveImage(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Image string `json:"image"`
		Force bool   `json:"force"`
	}
	if !decodeJSON(w, r, &req) || !s.needDocker(w, r) {
		return
	}
	if err := s.docker.RemoveImage(r.Context(), req.Image, req.Force); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "image_removed", clientIP(r), req.Image)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handlePruneImages removes every image no container uses, as a job.
func (s *Server) handlePruneImages(w http.ResponseWriter, r *http.Request, a *authed) {
	if !s.needDocker(w, r) {
		return
	}
	user, ip := a.user.Username, clientIP(r)
	s.startDockerJob(w, r, "image_prune", "", false, func(ctx context.Context, j *jobs.Job) error {
		j.Logf("Removing images that no container uses")
		if err := s.docker.PruneImages(ctx, j); err != nil {
			return err
		}
		s.audit(ctx, user, "images_pruned", ip, "")
		return nil
	})
}
