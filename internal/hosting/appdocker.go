package hosting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nefocp/nefo/internal/appstore"
	"github.com/nefocp/nefo/internal/docker"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
)

// Docker apps from the app store run as Compose projects that the panel
// writes from the app's recipe (internal/appstore). A Docker app gets its
// own website: the site is created suspended, the containers start, the
// set-up steps close the app's default logins, and only then does the site
// open. A service app (PostgreSQL, MongoDB, Ollama) has no website and
// listens on 127.0.0.1 only.
//
// Each installed copy (an instance) is named after the website's Linux
// user, or after the service app's ID, and keeps its files in
// /var/lib/panel-apps/<name> (0700, root): compose.json, the secrets mounted
// as files, and the containers' data under data/. A host user whose uid
// happens to match a container's (uid 1000 is often the server's login
// user) cannot reach that data.

// AppsDir holds the instances; tests point it elsewhere.
var AppsDir = "/var/lib/panel-apps"

const dockerAppPrefix = "docker:"

// appOps serialises installs, updates and removals of app instances.
var appOps sync.Mutex

var instanceRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,40}$`)

func instanceDir(name string) (string, error) {
	if !instanceRe.MatchString(name) {
		return "", fmt.Errorf("invalid app instance %q", name)
	}
	return filepath.Join(AppsDir, name), nil
}

func composeFile(dir string) string { return filepath.Join(dir, "compose.json") }

// composeArgs builds "docker compose" arguments for an instance.
func composeArgs(name string, args ...string) []string {
	dir := filepath.Join(AppsDir, name)
	return append([]string{"compose", "--ansi", "never", "--project-directory", dir, "-f", composeFile(dir),
		"-p", appstore.ProjectName(name)}, args...)
}

func dockerReady() bool {
	_, err := os.Stat("/usr/bin/docker")
	return err == nil
}

// siteDockerApp returns the catalog entry of the Docker app a site runs.
func siteDockerApp(site *store.Site) (*appstore.App, bool) {
	id, ok := strings.CutPrefix(site.App, dockerAppPrefix)
	if !ok {
		return nil, false
	}
	a, ok := appstore.Lookup(id)
	if !ok || a.Docker == nil {
		return nil, false
	}
	return a, true
}

func containerName(site *store.Site) string { return "panel-" + site.SysUser }

// DockerAppRequest asks for a Docker app (with a new website) or a service
// app (Domain empty).
type DockerAppRequest struct {
	App        string
	Domain     string
	Aliases    []string
	AdminEmail string
	Inputs     map[string]string
}

// reservedTLDs are refused for administrator e-mail addresses: some apps
// (Directus) reject them, and they cannot receive mail anyway.
var reservedTLDs = []string{".test", ".example", ".invalid", ".localhost", ".local"}

func checkAdminEmail(s string) error {
	if !appstore.ValidEmail(s) {
		return usererr.New("invalid_email")
	}
	low := strings.ToLower(s)
	for _, t := range reservedTLDs {
		if strings.HasSuffix(low, t) {
			return usererr.New("invalid_email", s)
		}
	}
	return nil
}

// needsAdminEmail reports whether a recipe uses ${admin_email}.
func needsAdminEmail(a *appstore.App) bool {
	b, _ := json.Marshal(a.Docker)
	return strings.Contains(string(b), "${admin_email}")
}

// NeedsAdminEmail reports whether installing a Docker app asks for the
// administrator's e-mail address.
func NeedsAdminEmail(a *appstore.App) bool { return a.Docker != nil && needsAdminEmail(a) }

// AppChecks runs the resource check of every app of a catalog, reading
// the server's state once.
func (m *Manager) AppChecks(ctx context.Context, c *appstore.Catalog) map[string]*appstore.CheckResult {
	dockerSys, webSys := m.appSystem(ctx, true), m.appSystem(ctx, false)
	out := map[string]*appstore.CheckResult{}
	for _, a := range c.Apps {
		sys := webSys
		if a.Docker != nil {
			sys = dockerSys
		}
		out[a.ID] = appstore.Check(a, sys)
	}
	return out
}

// CheckDockerApp validates a request and checks the server's resources. The
// result lists refusals (also returned as the error) and warnings.
func (m *Manager) CheckDockerApp(ctx context.Context, req DockerAppRequest) (*appstore.App, *appstore.CheckResult, error) {
	app, ok := appstore.Lookup(req.App)
	if !ok || app.Docker == nil {
		return nil, nil, usererr.New("unknown_app", req.App)
	}
	if !m.Supported() {
		return app, nil, ErrUnsupported
	}
	if !dockerReady() {
		return app, nil, usererr.New("docker_needed")
	}
	switch app.Kind {
	case appstore.KindDocker:
		if !fileExists(m.web().hookFile()) {
			return app, nil, ErrNotReady
		}
		if _, _, err := m.checkNames(ctx, req.Domain, req.Aliases, 0); err != nil {
			return app, nil, err
		}
	case appstore.KindService:
		if _, err := m.store.AppByName(ctx, app.ID); err == nil {
			return app, nil, usererr.New("app_installed", app.Name)
		}
	}
	if needsAdminEmail(app) {
		if err := checkAdminEmail(strings.TrimSpace(req.AdminEmail)); err != nil {
			return app, nil, err
		}
	}
	if _, err := checkInputs(app.Docker.Inputs, req.Inputs); err != nil {
		return app, nil, err
	}
	res := appstore.Check(app, m.appSystem(ctx, true))
	if !res.OK() {
		p := res.Refusals[0]
		return app, res, usererr.New(p.Code, p.Detail)
	}
	return app, res, nil
}

// CheckResources returns the resource check of any catalog app.
func (m *Manager) CheckResources(ctx context.Context, app *appstore.App) *appstore.CheckResult {
	return appstore.Check(app, m.appSystem(ctx, app.Docker != nil))
}

// appSystem reads memory, disk and CPU, and adds up the memory limits of
// the installed apps.
func (m *Manager) appSystem(ctx context.Context, isDocker bool) appstore.System {
	disk := m.paths.WWW
	if isDocker {
		disk = "/var/lib/docker"
		if out, err := m.run.Output(ctx, "docker", "info", "-f", "{{.DockerRootDir}}"); err == nil && strings.HasPrefix(strings.TrimSpace(out), "/") {
			disk = strings.TrimSpace(out)
		}
		if _, err := os.Stat(disk); err != nil {
			disk = "/var/lib"
		}
	}
	sys := appstore.ReadSystem(disk)
	if apps, err := m.store.ListApps(ctx); err == nil {
		for _, a := range apps {
			if c, ok := appstore.Lookup(a.AppID); ok {
				if c.Resources.MemoryAuto {
					sys.RunningLimitsMB += appstore.AutoMemoryMB(sys.MemTotalMB)
				} else {
					sys.RunningLimitsMB += c.Resources.MemoryLimitMB
				}
			}
		}
	}
	return sys
}

var (
	countryRe  = regexp.MustCompile(`^[A-Z]{2}$`)
	currencyRe = regexp.MustCompile(`^[A-Z]{3}$`)
)

// checkInputs validates install-form values against an app's inputs and
// fills defaults.
func checkInputs(specs map[string]*appstore.Input, in map[string]string) (map[string]string, error) {
	out := map[string]string{}
	for k := range in {
		if _, ok := specs[k]; !ok {
			return nil, usererr.New("invalid_app_input", k)
		}
	}
	for name, s := range specs {
		v := strings.TrimSpace(in[name])
		if v == "" {
			v = s.Default
		}
		if v == "" && s.Optional {
			continue
		}
		ok := false
		switch s.Type {
		case "port":
			n, err := strconv.Atoi(v)
			ok = err == nil && n >= 1024 && n <= 65535
		case "country":
			v = strings.ToUpper(v)
			ok = countryRe.MatchString(v)
		case "currency":
			v = strings.ToUpper(v)
			ok = currencyRe.MatchString(v)
		case "regex":
			ok = regexp.MustCompile(s.Pattern).MatchString(v)
		}
		if !ok {
			return nil, usererr.New("invalid_app_input", name)
		}
		out[name] = v
	}
	return out, nil
}

// InstallResult is what an install created.
type InstallResult struct {
	Site     *store.Site
	Instance *store.App
}

// InstallDockerApp installs a Docker app with its website, or a service
// app. If anything fails, everything it created is removed again.
func (m *Manager) InstallDockerApp(ctx context.Context, req DockerAppRequest, lg Logger) (*InstallResult, error) {
	app, _, err := m.CheckDockerApp(ctx, req)
	if err != nil {
		return nil, err
	}
	inputs, _ := checkInputs(app.Docker.Inputs, req.Inputs)
	appOps.Lock()
	defer appOps.Unlock()

	bg := context.WithoutCancel(ctx)
	inst := &store.App{AppID: app.ID, Kind: app.Kind, Version: app.Version, Images: imageList(app),
		Inputs: inputs, AdminEmail: strings.TrimSpace(req.AdminEmail), ExtraPorts: map[int]int{}}
	var site *store.Site
	// owned: the instance record is this install's own (never remove the
	// files of an instance that existed before, such as a service app
	// installed twice at the same moment).
	ok, owned := false, false
	defer func() {
		if ok {
			return
		}
		lg.Logf("Removing what the install created")
		if owned {
			m.removeInstance(bg, inst.Name, lg)
			m.store.DeleteApp(bg, inst.ID)
		}
		if site != nil {
			// Another change may hold the web server for a moment.
			for i := 0; ; i++ {
				_, err := m.DeleteSite(bg, site.ID, site.Domain)
				if err == nil {
					break
				}
				if !errors.Is(err, ErrBusy) || i == 60 {
					lg.Logf("Warning: %v", err)
					break
				}
				time.Sleep(time.Second)
			}
		}
	}()

	used, err := m.usedAppPorts(ctx)
	if err != nil {
		return nil, err
	}
	pick := func(prefer int) (int, error) {
		if prefer > 0 && !used[prefer] && portFree(prefer) {
			used[prefer] = true
			return prefer, nil
		}
		for range 50 {
			p := freePort()
			if !used[p] && m.checkApp(ctx, p, "", 0) == nil {
				used[p] = true
				return p, nil
			}
		}
		return 0, errors.New("no free port for the app")
	}
	r := app.Docker
	if app.Kind == appstore.KindDocker {
		port, err := pick(0)
		if err != nil {
			return nil, err
		}
		lg.Logf("Creating the website %s (closed until the app is set up)", req.Domain)
		site, err = m.CreateSite(ctx, CreateRequest{Domain: req.Domain, Aliases: req.Aliases, ProxyPort: port, Suspended: true})
		if err != nil {
			site = nil
			return nil, err
		}
		inst.Name, inst.SiteID, inst.Port = site.SysUser, site.ID, port
	} else {
		inst.Name = app.ID
		if inst.Port, err = pick(r.Service.PreferPort); err != nil {
			return nil, err
		}
	}
	for _, p := range r.ExtraPorts {
		host, err := pick(0)
		if err != nil {
			return nil, err
		}
		inst.ExtraPorts[p.Port] = host
	}
	secrets, err := appstore.GenerateSecrets(r.Secrets, nil)
	if err != nil {
		return nil, err
	}
	if err := m.store.CreateApp(ctx, inst); err != nil {
		if errors.Is(err, store.ErrConflict) {
			return nil, usererr.New("app_installed", inst.Name)
		}
		return nil, err
	}
	owned = true
	if err := m.store.SetAppSecrets(ctx, inst.ID, secrets); err != nil {
		return nil, err
	}

	env, err := m.appEnv(app, inst, site, secrets, true)
	if err != nil {
		return nil, err
	}
	if err := m.prepareInstance(ctx, app, env, lg); err != nil {
		return nil, err
	}
	if err := m.startInstance(ctx, app, inst, env, true, lg); err != nil {
		return nil, err
	}
	// Apps that print a one-time password.
	if app.Admin.Method == "logs" && app.Admin.LogRegex != "" {
		pw, err := m.passwordFromLogs(ctx, inst.Name, app, 2*time.Minute)
		if err != nil {
			return nil, err
		}
		secrets["admin_password"] = pw
		if err := m.store.SetAppSecrets(ctx, inst.ID, map[string]string{"admin_password": pw}); err != nil {
			return nil, err
		}
	}
	// First-start-only variables (initial passwords) go away.
	if r.HasBootstrap() {
		lg.Logf("Removing the initial password from the container's settings")
		env.Bootstrap = false
		if err := m.writeCompose(ctx, app, env); err != nil {
			return nil, err
		}
		if err := m.upInstance(ctx, app, inst, lg); err != nil {
			return nil, err
		}
	}
	inst.State = "ready"
	if err := m.store.UpdateApp(ctx, inst); err != nil {
		return nil, err
	}
	if site != nil {
		lg.Logf("Opening the website")
		if site, err = m.openAppSite(ctx, site.ID, app, secrets); err != nil {
			return nil, err
		}
	}
	ok = true
	if site != nil {
		lg.Logf("%s is running at http://%s/", app.Name, site.Domain)
	} else {
		lg.Logf("%s is running on 127.0.0.1:%d", app.Name, inst.Port)
	}
	return &InstallResult{Site: site, Instance: inst}, nil
}

func imageList(app *appstore.App) string {
	var refs []string
	for _, name := range sortedNames(app.Docker.Services) {
		refs = append(refs, app.Docker.Services[name].ImageRef())
	}
	return strings.Join(refs, " ")
}

func sortedNames[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// usedAppPorts lists the host ports sites and app instances use.
func (m *Manager) usedAppPorts(ctx context.Context) (map[int]bool, error) {
	used := map[int]bool{}
	sites, err := m.store.ListSites(ctx)
	if err != nil {
		return nil, err
	}
	for _, s := range sites {
		used[s.ProxyPort] = true
	}
	apps, err := m.store.ListApps(ctx)
	if err != nil {
		return nil, err
	}
	for _, a := range apps {
		used[a.Port] = true
		for _, p := range a.ExtraPorts {
			used[p] = true
		}
	}
	for _, p := range m.reserved {
		used[p] = true
	}
	return used, nil
}

func portFree(p int) bool {
	l, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(p))
	if err != nil {
		return false
	}
	l.Close()
	return true
}

// appEnv collects an instance's placeholder values.
func (m *Manager) appEnv(app *appstore.App, inst *store.App, site *store.Site, secrets map[string]string, bootstrap bool) (*appstore.Env, error) {
	dir, err := instanceDir(inst.Name)
	if err != nil {
		return nil, err
	}
	e := &appstore.Env{Name: inst.Name, Port: inst.Port, ExtraPorts: inst.ExtraPorts, Secrets: secrets, Inputs: inst.Inputs,
		AdminEmail: inst.AdminEmail, ProjectDir: filepath.ToSlash(dir), Bootstrap: bootstrap, TZ: serverTimeZone()}
	if site != nil {
		uid, gid, err := lookupIDs(site.SysUser)
		if err != nil {
			return nil, err
		}
		e.Domain, e.HTTPS, e.PublicHost, e.SiteUID, e.SiteGID = site.Domain, site.SSL, site.Domain, uid, gid
	}
	if app.Resources.MemoryAuto {
		e.MemoryMB = appstore.AutoMemoryMB(appstore.ReadSystem("").MemTotalMB)
	}
	return e, nil
}

func serverTimeZone() string {
	if b, err := os.ReadFile("/etc/timezone"); err == nil {
		if tz := strings.TrimSpace(string(b)); tz != "" {
			return tz
		}
	}
	return "UTC"
}

// prepareInstance creates the instance's folders, secret files, the files
// written once before the first start and the shared networks, then
// writes compose.json.
func (m *Manager) prepareInstance(ctx context.Context, app *appstore.App, env *appstore.Env, lg Logger) error {
	if err := ensureLayout(app, env); err != nil {
		return err
	}
	for _, n := range app.Docker.Networks {
		if _, err := m.run.Output(ctx, "docker", "network", "inspect", n); err != nil {
			lg.Logf("Creating the shared network %s", n)
			if _, err := m.run.Output(ctx, "docker", "network", "create", n); err != nil {
				return err
			}
		}
	}
	return m.writeCompose(ctx, app, env)
}

// ensureLayout creates what is missing of an instance's folders, secret
// files and files written once.
func ensureLayout(app *appstore.App, env *appstore.Env) error {
	if err := mkdir(AppsDir, 0o700); err != nil {
		return err
	}
	for _, d := range []string{env.ProjectDir, env.DataDir(), env.SecretsDir()} {
		if err := mkdir(d, 0o700); err != nil {
			return err
		}
	}
	for _, name := range appstore.SecretFiles(app) {
		v, ok := env.Secrets[name]
		if !ok {
			return fmt.Errorf("secret %s missing", name)
		}
		// 0444: some images read *_FILE after dropping to their own user
		// (mongo). The folder (0700 root) keeps it from the server's users.
		if err := writeFile(filepath.Join(env.SecretsDir(), name), []byte(v), 0o444); err != nil {
			return err
		}
	}
	dirs, files, err := appstore.Layout(app, env)
	if err != nil {
		return err
	}
	for _, d := range dirs {
		if err := mkdir(d.Path, os.FileMode(d.Mode)); err != nil {
			return err
		}
		if err := os.Chown(d.Path, d.UID, d.GID); err != nil {
			return err
		}
	}
	for _, f := range files {
		if fileExists(f.Path) {
			continue // written once: the app may have changed it since
		}
		if err := writeFile(f.Path, f.Data, os.FileMode(f.Mode)); err != nil {
			return err
		}
		if err := os.Chown(f.Path, f.UID, f.GID); err != nil {
			return err
		}
	}
	return nil
}

// writeCompose renders compose.json and has Compose check it with the
// panel's container rules before anything starts.
func (m *Manager) writeCompose(ctx context.Context, app *appstore.App, env *appstore.Env) error {
	data, err := appstore.Render(app, env)
	if err != nil {
		return err
	}
	file := composeFile(env.ProjectDir)
	old, oldErr := os.ReadFile(file)
	if err := writeFile(file, data, 0o600); err != nil {
		return err
	}
	if err := m.checkInstance(ctx, env.Name); err != nil {
		if oldErr == nil {
			writeFile(file, old, 0o600)
		}
		return err
	}
	return nil
}

// checkInstance runs the Compose validator on an instance's final
// configuration, with the same checks as the administrator's own projects.
func (m *Manager) checkInstance(ctx context.Context, name string) error {
	out, err := m.run.Output(ctx, "docker", composeArgs(name, "config", "--format", "json")...)
	if err != nil {
		return fmt.Errorf("docker compose config: %w", err)
	}
	// Output may start with warnings; the JSON starts at the first brace.
	if i := strings.Index(out, "{"); i > 0 {
		out = out[i:]
	}
	return docker.CheckConfig([]byte(out))
}

// startInstance pulls the images, runs the set-up steps and starts the
// containers. first runs the before-start and after-start steps.
func (m *Manager) startInstance(ctx context.Context, app *appstore.App, inst *store.App, env *appstore.Env, first bool, lg Logger) error {
	r := app.Docker
	lg.Logf("Downloading the images (this can take a few minutes)")
	if err := m.run.Stream(ctx, lg, "docker", composeArgs(inst.Name, "pull", "--quiet")...); err != nil {
		return err
	}
	if first && len(r.BeforeStart) > 0 {
		deps := r.Services[r.MainService()].DependsOn
		if len(deps) > 0 {
			lg.Logf("Starting %s", strings.Join(deps, ", "))
			args := append([]string{"up", "-d", "--wait", "--wait-timeout", "300"}, deps...)
			if err := m.run.Stream(ctx, lg, "docker", composeArgs(inst.Name, args...)...); err != nil {
				return m.withLogs(ctx, inst.Name, err)
			}
		}
		for _, st := range r.BeforeStart {
			if err := m.runStep(ctx, inst, env, st, lg); err != nil {
				return err
			}
		}
	}
	if err := m.upInstance(ctx, app, inst, lg); err != nil {
		return err
	}
	if first {
		for _, st := range r.AfterStart {
			if err := m.runStep(ctx, inst, env, st, lg); err != nil {
				return err
			}
		}
	}
	return nil
}

// upInstance starts (or recreates) the containers and waits until the app
// answers.
func (m *Manager) upInstance(ctx context.Context, app *appstore.App, inst *store.App, lg Logger) error {
	lg.Logf("Starting %s", app.Name)
	if err := m.run.Stream(ctx, lg, "docker", composeArgs(inst.Name, "up", "-d", "--remove-orphans", "--wait", "--wait-timeout", "600")...); err != nil {
		return m.withLogs(ctx, inst.Name, err)
	}
	path := ""
	switch r := app.Docker; {
	case r.Web != nil:
		path = r.Web.Health
	case r.Service != nil:
		path = r.Service.Health
	}
	if path == "" {
		return nil
	}
	lg.Logf("Waiting for %s to answer", app.Name)
	if err := waitHTTPPath(ctx, inst.Port, path, 8*time.Minute); err != nil {
		return m.withLogs(ctx, inst.Name, fmt.Errorf("%s did not start: %w", app.Name, err))
	}
	return nil
}

// withLogs adds the end of the containers' logs to an error.
func (m *Manager) withLogs(ctx context.Context, name string, err error) error {
	out, _ := m.run.Output(context.WithoutCancel(ctx), "docker", composeArgs(name, "logs", "--no-color", "--tail", "15")...)
	if strings.TrimSpace(out) == "" {
		return err
	}
	return fmt.Errorf("%w\n%s", err, run.Tail(out, 15))
}

// passwordFromLogs reads a one-time password the app printed.
func (m *Manager) passwordFromLogs(ctx context.Context, name string, app *appstore.App, limit time.Duration) (string, error) {
	re, err := regexp.Compile(app.Admin.LogRegex)
	if err != nil {
		return "", err
	}
	main := appstore.ContainerName(name, app.Docker.MainService(), true)
	for deadline := time.Now().Add(limit); ; {
		out, _ := m.run.Output(ctx, "docker", "logs", main)
		for _, l := range strings.Split(out, "\n") {
			if mm := re.FindStringSubmatch(strings.TrimRight(l, "\r")); mm != nil && regexp.MustCompile(`^[\x21-\x7e]{6,128}$`).MatchString(mm[1]) {
				return mm[1], nil
			}
		}
		if time.Now().After(deadline) {
			return "", errors.New("the app did not print its initial password")
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// openAppSite records the app on its website, raises the upload limit,
// gives apps without accounts a site password, and opens the site.
func (m *Manager) openAppSite(ctx context.Context, siteID int64, app *appstore.App, secrets map[string]string) (*store.Site, error) {
	// Wait for other changes rather than fail the whole install at its
	// last step.
	m.ops.Lock()
	defer m.ops.Unlock()
	site, err := m.store.SiteByID(ctx, siteID)
	if err != nil {
		return nil, err
	}
	old := *site
	site.App, site.Disabled = dockerAppPrefix+app.ID, false
	if w := app.Docker.Web; w != nil {
		if w.UploadMB > site.PHPUpload {
			site.PHPUpload = w.UploadMB
		}
		if w.BasicAuth {
			pw := secrets[app.Admin.PasswordSecret]
			if pw == "" {
				return nil, errors.New("site password missing")
			}
			h, err := bcryptHash(pw)
			if err != nil {
				return nil, err
			}
			site.AuthUser, site.AuthHash = "admin", h
			if err := m.writeAuth(site); err != nil {
				return nil, err
			}
		}
	}
	if err := m.updateSite(ctx, site, &old); err != nil {
		m.writeAuth(&old)
		return nil, err
	}
	return site, nil
}

// ---- removing ----

// removeInstance stops and removes an instance's containers, networks,
// images and folder. It is used by uninstalls and failed installs.
func (m *Manager) removeInstance(ctx context.Context, name string, lg Logger) error {
	dir, err := instanceDir(name)
	if err != nil {
		return err
	}
	var errs []error
	if fileExists(composeFile(dir)) {
		lg.Logf("Removing the containers")
		if _, err := m.run.Output(ctx, "docker", composeArgs(name, "down", "--remove-orphans", "--volumes", "--timeout", "20")...); err != nil {
			errs = append(errs, err)
		}
	}
	// Leftovers of a project whose file is gone.
	if out, err := m.run.Output(ctx, "docker", "ps", "-aq", "--filter", "label=panel.instance="+name); err == nil {
		if ids := strings.Fields(out); len(ids) > 0 {
			m.run.Output(ctx, "docker", append([]string{"rm", "-f"}, ids...)...)
		}
	}
	if err := os.RemoveAll(dir); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// removeImages removes images no container uses any more (errors for
// images still in use, by another app for instance, are ignored).
func (m *Manager) removeImages(ctx context.Context, refs string) {
	for _, ref := range strings.Fields(refs) {
		m.run.Output(ctx, "docker", "image", "rm", ref)
	}
}

// removeSiteApp removes the Docker app of a website being deleted.
func (m *Manager) removeSiteApp(ctx context.Context, site *store.Site) error {
	inst, err := m.store.AppForSite(ctx, site.ID)
	if errors.Is(err, store.ErrNotFound) {
		if _, ok := strings.CutPrefix(site.App, dockerAppPrefix); ok && validate.SystemUsername(site.SysUser) == nil {
			// installed before the catalog: a plain container
			if _, err := m.run.Output(ctx, "docker", "rm", "-f", containerName(site)); err != nil {
				return fmt.Errorf("remove the container: %w", err)
			}
		}
		return nil
	}
	if err != nil {
		return err
	}
	// Called with ops held: no appOps here (installs take appOps, then ops).
	err = m.removeInstance(ctx, inst.Name, discardLogger{})
	m.removeImages(ctx, inst.Images)
	if derr := m.store.DeleteApp(ctx, inst.ID); derr != nil && err == nil {
		err = derr
	}
	return err
}

// UninstallService removes a service app and its data. confirm must be the
// app's ID.
func (m *Manager) UninstallService(ctx context.Context, name, confirm string, lg Logger) error {
	inst, err := m.store.AppByName(ctx, name)
	if err != nil {
		return usererr.New("app_not_installed", name)
	}
	if inst.Kind != appstore.KindService {
		return usererr.New("app_has_site")
	}
	if confirm != inst.Name {
		return usererr.New("confirm_mismatch")
	}
	appOps.Lock()
	defer appOps.Unlock()
	if err := m.removeInstance(ctx, inst.Name, lg); err != nil {
		return err
	}
	m.removeImages(ctx, inst.Images)
	return m.store.DeleteApp(ctx, inst.ID)
}

type discardLogger struct{}

func (discardLogger) Write(p []byte) (int, error) { return len(p), nil }
func (discardLogger) Logf(string, ...any)         {}

var _ io.Writer = discardLogger{}
