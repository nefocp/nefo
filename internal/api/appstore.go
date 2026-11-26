package api

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nefocp/nefo/internal/appstore"
	"github.com/nefocp/nefo/internal/backup"
	"github.com/nefocp/nefo/internal/database"
	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/jobs"
	"github.com/nefocp/nefo/internal/secure"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/update"
	"github.com/nefocp/nefo/internal/usererr"
)

// The app store: the catalog (internal/appstore), installs of PHP apps into
// a website, of Docker apps with their own website and of service apps,
// and the installed apps' updates, backups, credentials and logs.

// appDBBase turns a domain into a database name such as "nc_cloud_example_com".
func appDBBase(prefix, domain string) string {
	return prefix + strings.TrimPrefix(wpDBBase(domain), "wp")
}

// ---- the remote catalog ----

var (
	catalogOnce   sync.Once
	catalogRemote *appstore.Remote
	catalogMu     sync.Mutex
	catalogLast   time.Time
	catalogErr    string
)

// remoteCatalog returns the signed remote catalog, configured from the
// update address; without a release key it is off.
func (s *Server) remoteCatalog() *appstore.Remote {
	catalogOnce.Do(func() {
		base := s.cfg.UpdateURL
		if base == "" {
			base = update.DefaultBase
		}
		catalogRemote = &appstore.Remote{Base: base, CacheDir: filepath.Join(s.cfg.DataDir, "appstore")}
		if update.HasSigningKey() {
			catalogRemote.Verify = update.Verify
		}
		if err := catalogRemote.LoadCache(); err != nil {
			s.log.Warn("app catalog cache", "err", err)
		}
	})
	return catalogRemote
}

// refreshCatalog fetches the remote catalog once a day (called by the
// hourly maintenance loop) or when force is set.
func (s *Server) refreshCatalog(ctx context.Context, force bool) error {
	r := s.remoteCatalog()
	if !r.Enabled() {
		return appstore.ErrRemoteDisabled
	}
	catalogMu.Lock()
	defer catalogMu.Unlock()
	if !force && time.Since(catalogLast) < 24*time.Hour {
		return nil
	}
	catalogLast = time.Now()
	_, err := r.Refresh(ctx)
	switch {
	case err == nil:
		catalogErr = ""
		s.log.Info("app catalog updated", "version", appstore.Current().CatalogVersion)
	case appstore.IsNotNewer(err):
		catalogErr = ""
		err = nil
	default:
		catalogErr = err.Error()
		s.log.Warn("app catalog refresh", "err", err)
	}
	return err
}

// ---- listing ----

// appView is an app as the UI shows it: the catalog entry without its
// recipe, plus the resource check.
type appView struct {
	ID              string                `json:"id"`
	Name            string                `json:"name"`
	Category        string                `json:"category"`
	Kind            string                `json:"kind"`
	Version         string                `json:"version"`
	Description     appstore.Text         `json:"description"`
	Notes           appstore.Text         `json:"notes,omitempty"`
	License         string                `json:"license"`
	SourceAvailable bool                  `json:"source_available"`
	Homepage        string                `json:"homepage"`
	Logo            string                `json:"logo"`
	Replaces        string                `json:"replaces,omitempty"`
	Resources       appstore.Resources    `json:"resources"`
	HTTPS           bool                  `json:"needs_https"`
	Admin           string                `json:"admin_method"`
	AdminEmail      bool                  `json:"needs_admin_email"`
	Updates         string                `json:"update_strategy"`
	MinPHP          string                `json:"min_php,omitempty"`
	MaxPHP          string                `json:"max_php,omitempty"`
	AdminForm       bool                  `json:"admin_form,omitempty"`
	Composer        bool                  `json:"composer,omitempty"`
	SiteFolder      bool                  `json:"site_folder,omitempty"` // takes the whole site folder (hosting.NeedsSiteFolder)
	Inputs          map[string]inputView  `json:"inputs"`
	Installed       bool                  `json:"installed"` // service apps: one per server
	Check           *appstore.CheckResult `json:"check"`
}

type inputView struct {
	Type     string        `json:"type"`
	Default  string        `json:"default,omitempty"`
	Optional bool          `json:"optional,omitempty"`
	Label    appstore.Text `json:"label,omitempty"`
}

func viewApp(a *appstore.App, check *appstore.CheckResult) appView {
	v := appView{ID: a.ID, Name: a.Name, Category: a.Category, Kind: a.Kind, Version: a.Version, Description: a.Description,
		Notes: a.Notes, License: a.License, SourceAvailable: a.SourceAvailable, Homepage: a.Homepage, Logo: a.Logo,
		Replaces: a.Replaces, Resources: a.Resources, HTTPS: a.Requires.HTTPS, Admin: a.Admin.Method,
		Updates: a.Updates.Strategy, Inputs: map[string]inputView{}, Check: check}
	var inputs map[string]*appstore.Input
	if a.PHP != nil {
		v.MinPHP, v.MaxPHP, v.AdminForm, v.Composer = a.PHP.MinPHP, a.PHP.MaxPHP, a.PHP.Admin, a.PHP.Composer != nil
		v.SiteFolder = hosting.NeedsSiteFolder(a)
		inputs = a.PHP.Inputs
	}
	if a.Docker != nil {
		v.AdminEmail = hosting.NeedsAdminEmail(a)
		inputs = a.Docker.Inputs
	}
	for k, in := range inputs {
		v.Inputs[k] = inputView{Type: in.Type, Default: in.Default, Optional: in.Optional, Label: in.Label}
	}
	return v
}

func (s *Server) handleAppCatalog(w http.ResponseWriter, r *http.Request, a *authed) {
	ctx := r.Context()
	s.remoteCatalog()
	c := appstore.Current()
	// Without memory use: docker stats would slow the catalog down; the
	// Installed tab asks GET /api/apps/installed for it.
	installed, err := s.hosting.InstalledApps(ctx, false)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	services := map[string]bool{}
	for _, ia := range installed {
		if ia.Kind == appstore.KindService {
			services[ia.AppID] = true
		}
	}
	checks := s.hosting.AppChecks(ctx, c)
	apps := make([]appView, 0, len(c.Apps))
	legacyDocker := []map[string]any{}
	for _, app := range c.Apps {
		v := viewApp(app, checks[app.ID])
		v.Installed = services[app.ID]
		apps = append(apps, v)
		if app.Kind == appstore.KindDocker {
			legacyDocker = append(legacyDocker, map[string]any{"id": app.ID, "name": app.Name,
				"image": app.Docker.Services[app.Docker.MainService()].Image, "memory_mb": app.Resources.MemoryLimitMB})
		}
	}
	r2 := s.remoteCatalog()
	catalogMu.Lock()
	lastErr := catalogErr
	catalogMu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"catalog": map[string]any{"version": c.CatalogVersion, "generated": c.Generated, "source": appstore.Source(),
			"remote_enabled": r2.Enabled(), "last_error": lastErr},
		"apps":             apps,
		"installed":        installed,
		"docker_installed": s.docker.Installed(),
		// The keys the UI before the app store redesign reads.
		"php":    hosting.PHPApps(),
		"docker": legacyDocker,
	})
}

func (s *Server) handleAppLogo(w http.ResponseWriter, r *http.Request, a *authed) {
	b, ok := appstore.Logo(strings.TrimSuffix(r.PathValue("key"), ".svg"))
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	h := w.Header()
	h.Set("Content-Type", "image/svg+xml")
	// The files are plain shapes; this policy makes sure nothing in one
	// could run even if it were opened on its own.
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	h.Set("Cache-Control", "private, max-age=86400")
	w.Write(b)
}

func (s *Server) handleRefreshCatalog(w http.ResponseWriter, r *http.Request, a *authed) {
	err := s.refreshCatalog(r.Context(), true)
	if err == appstore.ErrRemoteDisabled {
		writeError(w, http.StatusConflict, "catalog_remote_disabled")
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "catalog_refresh_failed", "detail": err.Error()})
		return
	}
	c := appstore.Current()
	writeJSON(w, http.StatusOK, map[string]any{"version": c.CatalogVersion, "source": appstore.Source()})
}

// ---- checks and installs ----

type appInstallRequest struct {
	// Docker and service apps
	Domain     string   `json:"domain"`
	Aliases    []string `json:"aliases"`
	AdminEmail string   `json:"admin_email"`
	// PHP apps
	SiteID        int64  `json:"site_id"`
	Title         string `json:"title"`
	AdminUser     string `json:"admin_user"`
	AdminPassword string `json:"admin_password"`
	// both
	Inputs map[string]string `json:"inputs"`
}

func (s *Server) handleCheckApp(w http.ResponseWriter, r *http.Request, a *authed) {
	app, ok := appstore.Lookup(r.PathValue("id"))
	if !ok {
		s.opError(w, r, usererr.New("unknown_app", r.PathValue("id")))
		return
	}
	var req appInstallRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	res := map[string]any{"ok": true}
	var check *appstore.CheckResult
	var err error
	if app.Docker != nil {
		_, check, err = s.hosting.CheckDockerApp(r.Context(), hosting.DockerAppRequest{App: app.ID, Domain: req.Domain,
			Aliases: req.Aliases, AdminEmail: req.AdminEmail, Inputs: req.Inputs})
	} else {
		check = s.hosting.CheckResources(r.Context(), app)
		if req.SiteID != 0 {
			in := hosting.AppRequest{Title: req.Title, AdminUser: req.AdminUser, AdminEmail: req.AdminEmail,
				AdminPassword: req.AdminPassword, Inputs: req.Inputs}
			in.MariaDB, in.DBEngine = appDBServer(s.db.Status(r.Context()))
			if in.AdminPassword == "" {
				in.AdminPassword = secure.String(20, secure.PasswordChars)
			}
			err = s.hosting.CheckPHPApp(r.Context(), req.SiteID, app.ID, in)
		}
	}
	if check != nil {
		res["refusals"], res["warnings"], res["memory_limit_mb"] = check.Refusals, check.Warnings, check.MemoryLimitMB
	}
	if err != nil {
		res["ok"] = false
		if ue, isUser := err.(*usererr.Error); isUser {
			res["error"], res["detail"] = ue.Code, ue.Detail
		} else {
			res["error"], res["detail"] = "operation_failed", err.Error()
		}
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleInstallCatalogApp(w http.ResponseWriter, r *http.Request, a *authed) {
	app, ok := appstore.Lookup(r.PathValue("id"))
	if !ok {
		s.opError(w, r, usererr.New("unknown_app", r.PathValue("id")))
		return
	}
	var req appInstallRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if app.PHP != nil {
		s.installPHPApp(w, r, a, req.SiteID, app.ID, req)
		return
	}
	s.installDockerApp(w, r, a, app, req)
}

func (s *Server) installDockerApp(w http.ResponseWriter, r *http.Request, a *authed, app *appstore.App, req appInstallRequest) {
	in := hosting.DockerAppRequest{App: app.ID, Domain: strings.TrimSpace(req.Domain), Aliases: req.Aliases,
		AdminEmail: strings.TrimSpace(req.AdminEmail), Inputs: req.Inputs}
	if app.Kind == appstore.KindService {
		in.Domain, in.Aliases = "", nil
	}
	if _, _, err := s.hosting.CheckDockerApp(r.Context(), in); err != nil {
		s.opError(w, r, err)
		return
	}
	user, ip, target := a.user.Username, clientIP(r), in.Domain
	if target == "" {
		target = app.ID
	}
	job, err := s.jobs.Start("install_"+app.ID, target, 45*time.Minute, func(ctx context.Context, j *jobs.Job) error {
		if err := s.docker.EnsureCompose(ctx, j); err != nil {
			return err
		}
		res, err := s.hosting.InstallDockerApp(ctx, in, j)
		if err != nil {
			s.audit(ctx, user, "app_install_failed", ip, app.ID+" "+target)
			return err
		}
		s.audit(ctx, user, "app_installed", ip, app.ID+" "+res.Instance.Name)
		return nil
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job": job.Snapshot()})
}

// handleInstallApp installs a PHP app (WordPress, Joomla…) into a site:
// POST /api/sites/{id}/apps/{app} and /api/sites/{id}/wordpress.
func (s *Server) handleInstallApp(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	app := r.PathValue("app")
	if app == "" {
		app = hosting.AppWordPress // POST /api/sites/{id}/wordpress
	}
	var req appInstallRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	s.installPHPApp(w, r, a, id, app, req)
}

func (s *Server) installPHPApp(w http.ResponseWriter, r *http.Request, a *authed, id int64, app string, req appInstallRequest) {
	st := s.db.Status(r.Context())
	if !st.Running {
		s.opError(w, r, database.ErrNotInstalled)
		return
	}
	if req.AdminPassword == "" {
		req.AdminPassword = secure.String(20, secure.PasswordChars)
	}
	in := hosting.AppRequest{
		Title: req.Title, AdminUser: strings.TrimSpace(req.AdminUser),
		AdminEmail: strings.TrimSpace(req.AdminEmail), AdminPassword: req.AdminPassword,
		Inputs: req.Inputs,
	}
	in.MariaDB, in.DBEngine = appDBServer(st)
	if err := s.hosting.CheckPHPApp(r.Context(), id, app, in); err != nil {
		s.opError(w, r, err)
		return
	}
	site, err := s.store.SiteByID(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	catalogApp, _ := appstore.Lookup(app)
	user, ip, domain := a.user.Username, clientIP(r), site.Domain
	job, err := s.jobs.Start("install_"+app, domain, 45*time.Minute, func(ctx context.Context, j *jobs.Job) error {
		j.Logf("Creating the database")
		db, err := s.createDBWithFreeName(ctx, appDBBase(hosting.AppDBPrefix(app), domain), id)
		if err != nil {
			return err
		}
		j.Logf("Database %s created", db.Name)
		in.DB = db
		if err := s.hosting.InstallPHPApp(ctx, id, app, in, j); err != nil {
			j.Logf("Removing database %s", db.Name)
			if derr := s.db.Delete(context.WithoutCancel(ctx), db.ID, db.Name); derr != nil {
				j.Logf("Warning: %v", derr)
			}
			s.audit(ctx, user, "app_install_failed", ip, app+" "+domain)
			return err
		}
		s.addAppCron(ctx, id, app, j)
		s.audit(ctx, user, "app_installed", ip, app+" "+domain)
		return nil
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	scheme := "http"
	if site.SSL {
		scheme = "https"
	}
	res := map[string]any{"job": job.Snapshot(), "admin_url": fmt.Sprintf("%s://%s%s", scheme, domain, catalogApp.Admin.URLPath)}
	if catalogApp.PHP.Admin {
		res["admin_user"], res["admin_password"] = in.AdminUser, in.AdminPassword
		if app == "moodle" {
			res["admin_user"] = strings.ToLower(in.AdminUser)
		}
	}
	writeJSON(w, http.StatusAccepted, res)
}

// addAppCron adds the cron job a PHP app needs (Moodle every minute,
// WooCommerce's scheduled tasks every five), run as the site's user.
func (s *Server) addAppCron(ctx context.Context, siteID int64, app string, lg *jobs.Job) {
	site, err := s.store.SiteByID(ctx, siteID)
	if err != nil {
		return
	}
	sched, cmd, ok := s.hosting.PHPAppCron(site, app)
	if !ok {
		return
	}
	c, _ := appstore.Lookup(app)
	job := &store.CronJob{Name: c.Name + " (" + site.Domain + ")", Schedule: sched, Command: cmd, RunAs: site.SysUser, Enabled: true}
	if err := s.cron.Save(ctx, job); err != nil {
		lg.Logf("Warning: could not add the cron job: %v", err)
		return
	}
	lg.Logf("Added the cron job %q (%s)", job.Name, sched)
}

// handleInstallDockerApp is the install call of the UI before the app
// store redesign: POST /api/apps/docker {app, domain}.
func (s *Server) handleInstallDockerApp(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		App        string            `json:"app"`
		Domain     string            `json:"domain"`
		Aliases    []string          `json:"aliases"`
		AdminEmail string            `json:"admin_email"`
		Inputs     map[string]string `json:"inputs"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	app, ok := appstore.Lookup(req.App)
	if !ok || app.Docker == nil {
		s.opError(w, r, usererr.New("unknown_app", req.App))
		return
	}
	s.installDockerApp(w, r, a, app, appInstallRequest{Domain: req.Domain, Aliases: req.Aliases, AdminEmail: req.AdminEmail, Inputs: req.Inputs})
}

// ---- installed apps ----

func (s *Server) handleInstalledApps(w http.ResponseWriter, r *http.Request, a *authed) {
	list, err := s.hosting.InstalledApps(r.Context(), true)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"installed": list})
}

func (s *Server) handleAppCredentials(w http.ResponseWriter, r *http.Request, a *authed) {
	c, err := s.hosting.AppCredentials(r.Context(), r.PathValue("name"))
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "app_credentials_viewed", clientIP(r), c.Name)
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) handleInstanceLogs(w http.ResponseWriter, r *http.Request, a *authed) {
	out, err := s.hosting.AppInstanceLogs(r.Context(), r.PathValue("name"))
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"log": out})
}

func (s *Server) handleInstanceAction(w http.ResponseWriter, r *http.Request, a *authed) {
	name, action := r.PathValue("name"), r.PathValue("action")
	switch action {
	case "start", "stop", "restart":
		if err := s.hosting.AppInstanceAction(r.Context(), name, action); err != nil {
			s.opError(w, r, err)
			return
		}
		s.audit(r.Context(), a.user.Username, "app_"+action, clientIP(r), name)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case "update":
		s.startAppJob(w, r, a, name, "update_app", func(ctx context.Context, j *jobs.Job) error {
			if _, _, _, err := s.hosting.CheckUpdate(ctx, name); err != nil {
				return err
			}
			if err := s.backupBeforeUpdate(ctx, name, j); err != nil {
				return fmt.Errorf("the backup before the update failed, so nothing was changed: %w", err)
			}
			return s.hosting.UpdateApp(ctx, name, j)
		}, true)
	case "backup":
		s.startAppJob(w, r, a, name, "backup", func(ctx context.Context, j *jobs.Job) error {
			_, err := s.backup.BackupApp(ctx, name, 0, false, j)
			return err
		}, false)
	default:
		writeError(w, http.StatusNotFound, "not_found")
	}
}

// backupBeforeUpdate backs up an instance, or a legacy app's website (its
// data is still in the site's folder).
func (s *Server) backupBeforeUpdate(ctx context.Context, name string, j *jobs.Job) error {
	if _, err := s.store.AppByName(ctx, name); err == nil {
		_, err := s.backup.BackupApp(ctx, name, 0, false, j)
		return err
	}
	_, _, site, err := s.hosting.CheckUpdate(ctx, name)
	if err != nil {
		return err
	}
	_, err = s.backup.Backup(ctx, backup.KindSite, site.Domain, 0, false, j)
	return err
}

func (s *Server) startAppJob(w http.ResponseWriter, r *http.Request, a *authed, name, kind string, fn func(context.Context, *jobs.Job) error, check bool) {
	if check {
		if _, _, _, err := s.hosting.CheckUpdate(r.Context(), name); err != nil {
			s.opError(w, r, err)
			return
		}
	} else if _, err := s.store.AppByName(r.Context(), name); err != nil {
		s.opError(w, r, usererr.New("app_not_installed", name))
		return
	}
	user, ip := a.user.Username, clientIP(r)
	job, err := s.jobs.Start(kind, name, 2*time.Hour, func(ctx context.Context, j *jobs.Job) error {
		err := fn(ctx, j)
		action := kind + "_done"
		if err != nil {
			action = kind + "_failed"
		}
		s.audit(ctx, user, action, ip, name)
		return err
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job": job.Snapshot()})
}

// handleUninstallApp removes a service app (an app with a website is
// removed with its website).
func (s *Server) handleUninstallApp(w http.ResponseWriter, r *http.Request, a *authed) {
	name := r.PathValue("name")
	var req struct {
		Confirm string `json:"confirm"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	inst, err := s.store.AppByName(r.Context(), name)
	if err != nil {
		s.opError(w, r, usererr.New("app_not_installed", name))
		return
	}
	if inst.Kind != appstore.KindService {
		s.opError(w, r, usererr.New("app_has_site"))
		return
	}
	if req.Confirm != inst.Name {
		s.opError(w, r, usererr.New("confirm_mismatch"))
		return
	}
	user, ip := a.user.Username, clientIP(r)
	job, err := s.jobs.Start("uninstall_app", name, 30*time.Minute, func(ctx context.Context, j *jobs.Job) error {
		err := s.hosting.UninstallService(ctx, name, req.Confirm, j)
		action := "app_uninstalled"
		if err != nil {
			action = "app_uninstall_failed"
		} else {
			s.store.DeleteSchedulesFor(ctx, backup.KindApp, name) // its backups stay
		}
		s.audit(ctx, user, action, ip, name)
		return err
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job": job.Snapshot()})
}
