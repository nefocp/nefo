package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/nefocp/nefo/internal/database"
	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/jobs"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
	"github.com/nefocp/nefo/internal/wptoolkit"
)

// The WordPress toolkit page: update checks and updates, maintenance and
// debug mode, automatic updates, administrator passwords and staging copies
// of the panel's WordPress sites (see package wptoolkit).

// wpKits holds each Server's toolkit, so this file needs no field in
// Server; the toolkit keeps state (the wp-cli install, a limit on status
// checks running at once).
var wpKits sync.Map // *Server → *wptoolkit.Toolkit

func (s *Server) wpkit() *wptoolkit.Toolkit {
	if k, ok := wpKits.Load(s); ok {
		return k.(*wptoolkit.Toolkit)
	}
	k, _ := wpKits.LoadOrStore(s, wptoolkit.New(run.System{}, s.hosting.Root))
	return k.(*wptoolkit.Toolkit)
}

type wpSiteResponse struct {
	ID         int64  `json:"id"`
	Domain     string `json:"domain"`
	PHPVersion string `json:"php_version"`
	SSL        bool   `json:"ssl"`
	Disabled   bool   `json:"disabled"`
	URL        string `json:"url"`
	// StagingOf is the live site of a staging copy.
	StagingOf *wpSiteRef `json:"staging_of,omitempty"`
}

type wpSiteRef struct {
	ID     int64  `json:"id"`
	Domain string `json:"domain"`
}

func siteBaseURL(site *store.Site) string {
	if site.SSL {
		return "https://" + site.Domain
	}
	return "http://" + site.Domain
}

// handleWPList lists the WordPress sites. It is fast: the status of each
// site comes from handleWPStatus.
func (s *Server) handleWPList(w http.ResponseWriter, r *http.Request, a *authed) {
	sites, err := s.store.ListSites(r.Context())
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	out := []wpSiteResponse{}
	for _, site := range sites {
		if site.App == hosting.AppWordPress && site.PHPVersion != "" {
			ws := wpSiteJSON(site)
			if live, err := s.stagingLive(r.Context(), site); err == nil && live != nil && live.App == hosting.AppWordPress {
				ws.StagingOf = &wpSiteRef{ID: live.ID, Domain: live.Domain}
			}
			out = append(out, ws)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"sites":     out,
		"supported": s.hosting.Supported(),
		"wp_cli":    map[string]any{"version": s.wpkit().CLIVersion(), "installed": s.wpkit().Installed()},
	})
}

// wpSite returns the WordPress site named in the path, or answers the
// request with an error.
func (s *Server) wpSite(w http.ResponseWriter, r *http.Request) (*store.Site, bool) {
	if !s.hosting.Supported() {
		s.opError(w, r, hosting.ErrUnsupported)
		return nil, false
	}
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return nil, false
	}
	site, err := s.store.SiteByID(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return nil, false
	}
	if site.App != hosting.AppWordPress || site.PHPVersion == "" {
		s.opError(w, r, usererr.New("wp_not_wordpress", site.Domain))
		return nil, false
	}
	return site, true
}

// handleWPStatus checks one site with wp-cli (installed on first use).
func (s *Server) handleWPStatus(w http.ResponseWriter, r *http.Request, a *authed) {
	site, ok := s.wpSite(w, r)
	if !ok {
		return
	}
	st, err := s.wpkit().Status(r.Context(), site)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// handleWPUpdate backs up the database and updates core, plugins and themes.
func (s *Server) handleWPUpdate(w http.ResponseWriter, r *http.Request, a *authed) {
	site, ok := s.wpSite(w, r)
	if !ok {
		return
	}
	user, ip, domain := a.user.Username, clientIP(r), site.Domain
	job, err := s.jobs.Start("wp_update", domain, 90*time.Minute, func(ctx context.Context, j *jobs.Job) error {
		if err := s.wpkit().Update(ctx, site, j); err != nil {
			s.audit(ctx, user, "wp_update_failed", ip, domain)
			return err
		}
		s.audit(ctx, user, "wp_updated", ip, domain)
		return nil
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job": job.Snapshot()})
}

// wpToggle answers PUT {"on": bool} for a site setting.
func (s *Server) wpToggle(w http.ResponseWriter, r *http.Request, a *authed, what string,
	set func(context.Context, *store.Site, bool) error) {
	site, ok := s.wpSite(w, r)
	if !ok {
		return
	}
	var req struct {
		On bool `json:"on"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := set(r.Context(), site, req.On); err != nil {
		s.opError(w, r, err)
		return
	}
	state := "off"
	if req.On {
		state = "on"
	}
	s.audit(r.Context(), a.user.Username, "wp_"+what+"_"+state, clientIP(r), site.Domain)
	writeJSON(w, http.StatusOK, map[string]any{"on": req.On})
}

func (s *Server) handleWPMaintenance(w http.ResponseWriter, r *http.Request, a *authed) {
	s.wpToggle(w, r, a, "maintenance", s.wpkit().SetMaintenance)
}

func (s *Server) handleWPDebug(w http.ResponseWriter, r *http.Request, a *authed) {
	s.wpToggle(w, r, a, "debug", s.wpkit().SetDebug)
}

func (s *Server) handleWPAutoUpdates(w http.ResponseWriter, r *http.Request, a *authed) {
	s.wpToggle(w, r, a, "auto_updates", s.wpkit().SetAutoUpdates)
}

// handleWPAdmins lists the site's administrators, for the password reset.
func (s *Server) handleWPAdmins(w http.ResponseWriter, r *http.Request, a *authed) {
	site, ok := s.wpSite(w, r)
	if !ok {
		return
	}
	users, err := s.wpkit().Admins(r.Context(), site)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"admins": users})
}

// handleWPPassword sets an administrator's password: the one given, or a
// strong random one. The answer shows it once; the panel does not keep it.
func (s *Server) handleWPPassword(w http.ResponseWriter, r *http.Request, a *authed) {
	site, ok := s.wpSite(w, r)
	if !ok {
		return
	}
	var req struct {
		UserID   int64  `json:"user_id"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Password == "" {
		req.Password = wptoolkit.NewPassword()
	}
	login, err := s.wpkit().ResetPassword(r.Context(), site, req.UserID, req.Password)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "wp_admin_password_reset", clientIP(r), site.Domain+" "+login)
	writeJSON(w, http.StatusOK, map[string]any{
		"user": login, "password": req.Password, "login_url": siteBaseURL(site) + "/wp-login.php",
	})
}

// handleWPStaging copies a site to a new website, staging.<domain> unless
// another domain is given. Mistakes are reported before the job starts.
func (s *Server) handleWPStaging(w http.ResponseWriter, r *http.Request, a *authed) {
	site, ok := s.wpSite(w, r)
	if !ok {
		return
	}
	var req struct {
		Domain string `json:"domain"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	ctx := r.Context()
	domain := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(req.Domain)), ".")
	if domain == "" {
		domain = "staging." + site.Domain
	}
	if validate.Domain(domain) != nil {
		s.opError(w, r, usererr.New("invalid_domain", domain))
		return
	}
	if !s.db.Status(ctx).Running {
		s.opError(w, r, database.ErrNotInstalled)
		return
	}
	sites, err := s.store.ListSites(ctx)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	for _, other := range sites {
		if slices.Contains(other.Names(), domain) {
			s.opError(w, r, usererr.New("domain_exists", domain))
			return
		}
	}
	if root := s.hosting.Root(domain); fileOrDirExists(root) {
		s.opError(w, r, usererr.New("site_dir_exists", root))
		return
	}
	kit := s.wpkit()
	if _, err := kit.PrepareClone(site); err != nil {
		s.opError(w, r, err)
		return
	}
	user, ip := a.user.Username, clientIP(r)
	job, err := s.jobs.Start("wp_staging", domain, 3*time.Hour, func(ctx context.Context, j *jobs.Job) error {
		to, err := kit.Clone(ctx, site, domain, s.wpCloneHost(user, ip), j)
		if err != nil {
			s.audit(ctx, user, "wp_staging_failed", ip, site.Domain+" to "+domain)
			return err
		}
		s.audit(ctx, user, "wp_staging_created", ip, site.Domain+" to "+domain)
		// Remembered so the copy can be pushed back (see wppush.go).
		if err := s.saveStagingLink(ctx, site, to.ID); err != nil {
			j.Logf("Warning: the panel could not remember that %s is a copy of %s, so it cannot be pushed back: %v", domain, site.Domain, err)
		}
		return nil
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job": job.Snapshot(), "domain": domain, "url": "http://" + domain + "/"})
}

func fileOrDirExists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// wpCloneHost gives a staging copy the panel's website and database
// functions.
func (s *Server) wpCloneHost(user, ip string) wptoolkit.CloneHost {
	return wptoolkit.CloneHost{
		CreateSite: func(ctx context.Context, domain, php string) (*store.Site, error) {
			site, err := s.hosting.CreateSite(ctx, hosting.CreateRequest{Domain: domain, PHPVersion: php})
			if err == nil {
				s.audit(ctx, user, "site_created", ip, site.Domain)
			}
			return site, err
		},
		DeleteSite: func(ctx context.Context, site *store.Site) error {
			if _, err := s.hosting.DeleteSite(ctx, site.ID, site.Domain); err != nil {
				return err
			}
			s.audit(ctx, user, "site_deleted", ip, site.Domain)
			return nil
		},
		CreateDB: func(ctx context.Context, site *store.Site) (*store.Database, error) {
			return s.createDBWithFreeName(ctx, wpDBBase(site.Domain), site.ID)
		},
		DeleteDB: func(ctx context.Context, db *store.Database) error {
			return s.db.Delete(ctx, db.ID, db.Name)
		},
		CopyDB: s.wpCopyDB,
		MarkWordPress: func(ctx context.Context, from, to *store.Site, lg wptoolkit.Logger) error {
			site, err := s.store.SiteByID(ctx, to.ID)
			if err != nil {
				return err
			}
			site.App = hosting.AppWordPress
			if err := s.store.UpdateSite(ctx, site); err != nil {
				return err
			}
			// Saving the rewrite preset also writes the web firewall's
			// exclusions for the app.
			rewrite := from.Rewrite
			if rewrite == "" {
				rewrite = hosting.AppWordPress
			}
			lg.Logf("Applying the %s rewrite rules", rewrite)
			if _, err := s.hosting.SetNginxConfig(ctx, to.ID, rewrite, ""); err != nil {
				lg.Logf("Warning: could not apply the rewrite rules: %v", err)
			}
			return nil
		},
	}
}

// wpCopyDB streams a dump of one database into another, without a file
// and without stopping the live site.
func (s *Server) wpCopyDB(ctx context.Context, from, to string) error {
	pr, pw := io.Pipe()
	dumped := make(chan error, 1)
	go func() {
		err := s.db.Dump(ctx, from, pw)
		pw.CloseWithError(err)
		dumped <- err
	}()
	ierr := s.db.Import(ctx, to, pr)
	// Unblock the dump if the import stopped early.
	pr.CloseWithError(errors.New("the import stopped"))
	derr := <-dumped
	if ierr != nil {
		return fmt.Errorf("import into %s: %w", to, ierr)
	}
	if derr != nil {
		return fmt.Errorf("dump %s: %w", from, derr)
	}
	return nil
}
