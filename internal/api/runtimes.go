package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/jobs"
)

// nodeInUse lists the Node.js majors app sites use.
func (s *Server) nodeInUse(ctx context.Context) []int {
	sites, _ := s.store.ListSites(ctx)
	out := []int{} // [] in JSON, never null: the UI calls in_use.includes
	for _, site := range sites {
		if n, ok := strings.CutPrefix(site.AppRuntime, "node:"); ok {
			if major, err := strconv.Atoi(n); err == nil {
				out = append(out, major)
			}
		}
	}
	return out
}

func (s *Server) handleNodeVersions(w http.ResponseWriter, r *http.Request, a *authed) {
	vs, err := s.software.NodeVersions(r.Context())
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": vs, "in_use": s.nodeInUse(r.Context())})
}

// handleNodeVersion installs (POST) or removes (DELETE) a Node.js line.
func (s *Server) handleNodeVersion(w http.ResponseWriter, r *http.Request, a *authed) {
	major, err := strconv.Atoi(r.PathValue("major"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_node_version")
		return
	}
	install := r.Method == http.MethodPost
	user, ip := a.user.Username, clientIP(r)
	inUse := s.nodeInUse(r.Context())
	job, err := s.jobs.Start("node_version", "Node.js "+strconv.Itoa(major), 20*time.Minute, func(ctx context.Context, j *jobs.Job) error {
		var err error
		if install {
			err = s.software.InstallNodeMajor(ctx, major, j)
		} else {
			err = s.software.RemoveNodeMajor(ctx, major, inUse, j)
		}
		if err == nil {
			s.audit(ctx, user, "node_version_changed", ip, strconv.Itoa(major))
		}
		return err
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

func (s *Server) handleAppDeps(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	site, err := s.store.SiteByID(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	user, ip := a.user.Username, clientIP(r)
	job, err := s.jobs.Start("app_deps", site.Domain, 25*time.Minute, func(ctx context.Context, j *jobs.Job) error {
		err := s.hosting.InstallAppDependencies(ctx, id, j)
		if err == nil {
			s.audit(ctx, user, "app_deps_installed", ip, site.Domain)
		}
		return err
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}
