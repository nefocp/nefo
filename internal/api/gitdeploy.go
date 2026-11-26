package api

import (
	"context"
	"net/http"

	"github.com/nefocp/nefo/internal/gitdeploy"
	"github.com/nefocp/nefo/internal/jobs"
	"github.com/nefocp/nefo/internal/run"
)

// Deploying a website from Git: settings under /api/sites/{id}/git, and the
// webhook listener that Nginx forwards /.panel/deploy/<id> to.

// gitDeploy returns the Git deploy manager. It keeps no state, so a new
// one per request is fine.
func (s *Server) gitDeploy() *gitdeploy.Manager {
	g := gitdeploy.New(run.System{}, s.store, s.hosting.Root)
	g.SetAppPath(s.hosting.RuntimePath)
	return g
}

// GitHookHandler returns the handler of the local webhook listener.
func (s *Server) GitHookHandler() http.Handler {
	return gitdeploy.NewHookHandler(s.store, s.deployFromHook, s.log)
}

// RunGitHooks serves the repositories' webhooks on gitdeploy.HookAddr
// until ctx is done.
func (s *Server) RunGitHooks(ctx context.Context) {
	if err := gitdeploy.ListenAndServe(ctx, gitdeploy.HookAddr, s.GitHookHandler()); err != nil {
		s.log.Error("git webhook listener", "addr", gitdeploy.HookAddr, "err", err)
	}
}

func (s *Server) deployFromHook(t gitdeploy.Trigger) error {
	_, err := s.startGitDeploy(context.Background(), t.SiteID, "webhook", t.IP, t.Provider+" push")
	return err
}

// startGitDeploy starts a deploy job. how says what started it, for the audit log.
func (s *Server) startGitDeploy(ctx context.Context, id int64, user, ip, how string) (*jobs.Job, error) {
	gd := s.gitDeploy()
	site, err := gd.CheckDeploy(ctx, id)
	if err != nil {
		return nil, err
	}
	domain := site.Domain
	return s.jobs.Start("git_deploy", domain, gitdeploy.JobTimeout, func(ctx context.Context, j *jobs.Job) error {
		commit, err := gd.Deploy(ctx, id, j)
		short := commit
		if len(short) > 12 {
			short = short[:12]
		}
		if err != nil {
			s.audit(ctx, user, "git_deploy_failed", ip, domain+" "+how)
			return err
		}
		s.audit(ctx, user, "git_deployed", ip, domain+" "+short+" "+how)
		return nil
	})
}

// gitDeployRunning reports whether a deploy of the site is running.
func (s *Server) gitDeployRunning(domain string) bool {
	for _, j := range s.jobs.Recent() {
		if j.Kind == "git_deploy" && j.Target == domain && j.Status == jobs.StatusRunning {
			return true
		}
	}
	return false
}

func (s *Server) handleGetGit(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	info, err := s.gitDeploy().Info(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleSaveGit(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	var req struct {
		Repo   string `json:"repo"`
		Branch string `json:"branch"`
		Target string `json:"target"`
		Script string `json:"script"`
	}
	if !decodeJSONLimit(w, r, &req, 64<<10) {
		return
	}
	info, err := s.gitDeploy().Save(r.Context(), id, gitdeploy.Settings{
		Repo: req.Repo, Branch: req.Branch, Target: req.Target, Script: req.Script,
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "git_deploy_saved", clientIP(r), info.Repo+" "+info.Branch+" → "+info.Folder)
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleGitDeploy(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	job, err := s.startGitDeploy(r.Context(), id, a.user.Username, clientIP(r), "manual")
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

func (s *Server) handleGitKey(w http.ResponseWriter, r *http.Request, a *authed) {
	s.gitChange(w, r, a, "git_key_replaced", (*gitdeploy.Manager).NewKey)
}

func (s *Server) handleGitSecret(w http.ResponseWriter, r *http.Request, a *authed) {
	s.gitChange(w, r, a, "git_secret_replaced", (*gitdeploy.Manager).NewSecret)
}

// gitChange replaces the deploy key or the webhook secret, but not in the
// middle of a deploy, which uses them.
func (s *Server) gitChange(w http.ResponseWriter, r *http.Request, a *authed, action string,
	change func(*gitdeploy.Manager, context.Context, int64) (*gitdeploy.Info, error)) {
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
	if s.gitDeployRunning(site.Domain) {
		s.opError(w, r, jobs.ErrBusy)
		return
	}
	info, err := change(s.gitDeploy(), r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, action, clientIP(r), site.Domain)
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleDeleteGit(w http.ResponseWriter, r *http.Request, a *authed) {
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
	if s.gitDeployRunning(site.Domain) {
		s.opError(w, r, jobs.ErrBusy)
		return
	}
	// Removing settings that are already gone is not an error.
	if err := s.gitDeploy().Remove(r.Context(), id); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "git_deploy_removed", clientIP(r), site.Domain)
	writeJSON(w, http.StatusOK, map[string]any{})
}
