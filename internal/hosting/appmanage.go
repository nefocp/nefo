package hosting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nefocp/nefo/internal/appstore"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// ---- the installed apps ----

// InstalledApp is an installed Docker or service app, or a PHP app on a
// website, for the Installed tab.
type InstalledApp struct {
	Name           string `json:"name"` // instance (Docker/service) or the site's Linux user (PHP)
	AppID          string `json:"app"`
	Kind           string `json:"kind"`
	SiteID         int64  `json:"site_id,omitempty"`
	Domain         string `json:"domain,omitempty"`
	Version        string `json:"version"`         // installed
	CatalogVersion string `json:"catalog_version"` // available
	// UpdateAvailable: the catalog has another version or other images.
	UpdateAvailable bool `json:"update_available"`
	// Legacy: installed before the catalog (docker run); the next update
	// converts it.
	Legacy     bool        `json:"legacy,omitempty"`
	State      string      `json:"state"`  // installing, ready, failed
	Status     string      `json:"status"` // containers: running, stopped, partial, missing
	MemoryMB   int         `json:"memory_mb,omitempty"`
	Port       int         `json:"port,omitempty"`
	ExtraPorts map[int]int `json:"extra_ports,omitempty"`
}

// InstalledApps lists every installed app. memory adds what each uses now
// (docker stats, which takes a second or two).
func (m *Manager) InstalledApps(ctx context.Context, memory bool) ([]InstalledApp, error) {
	sites, err := m.store.ListSites(ctx)
	if err != nil {
		return nil, err
	}
	insts, err := m.store.ListApps(ctx)
	if err != nil {
		return nil, err
	}
	status, mem := m.containerStates(ctx, memory)
	out := []InstalledApp{}
	bySite := map[int64]*store.App{}
	for _, in := range insts {
		if in.SiteID != 0 {
			bySite[in.SiteID] = in
		}
	}
	add := func(in *store.App, site *store.Site) {
		ia := InstalledApp{Name: in.Name, AppID: in.AppID, Kind: in.Kind, Version: in.Version, State: in.State,
			Port: in.Port, ExtraPorts: in.ExtraPorts, Status: status[in.Name], MemoryMB: mem[in.Name]}
		if ia.Status == "" {
			ia.Status = "missing"
		}
		if site != nil {
			ia.SiteID, ia.Domain = site.ID, site.Domain
		}
		if c, ok := appstore.Lookup(in.AppID); ok {
			ia.CatalogVersion = c.Version
			ia.UpdateAvailable = c.Docker != nil && (c.Version != in.Version || imageList(c) != in.Images)
		}
		out = append(out, ia)
	}
	for _, s := range sites {
		if in := bySite[s.ID]; in != nil {
			add(in, s)
			continue
		}
		if s.App == "" {
			continue
		}
		ia := InstalledApp{Name: s.SysUser, SiteID: s.ID, Domain: s.Domain, State: "ready"}
		if id, ok := strings.CutPrefix(s.App, dockerAppPrefix); ok {
			ia.AppID, ia.Kind, ia.Legacy, ia.Status = id, appstore.KindDocker, true, status[s.SysUser]
			if ia.Status == "" {
				ia.Status = "missing"
			}
			ia.Port, ia.MemoryMB = s.ProxyPort, mem[s.SysUser]
			if l, ok := legacyApps[id]; ok {
				ia.Version = l.image
			}
		} else {
			ia.AppID, ia.Kind = s.App, appstore.KindPHP
			// A WordPress site with WooCommerce is listed as WooCommerce.
			if s.App == AppWordPress && fileExists(filepath.Join(m.paths.siteRoot(s.Domain), "public", "wp-content", "plugins", "woocommerce", "woocommerce.php")) {
				ia.AppID = "woocommerce"
			}
			ia.Version = m.phpAppVersion(&store.Site{Domain: s.Domain, App: ia.AppID})
		}
		if c, ok := appstore.Lookup(ia.AppID); ok {
			ia.CatalogVersion = c.Version
			ia.UpdateAvailable = ia.Legacy || (ia.Kind == appstore.KindPHP && ia.Version != "" && appstore.VersionAtLeast(c.Version, ia.Version) && c.Version != ia.Version)
		}
		out = append(out, ia)
	}
	for _, in := range insts {
		if in.SiteID == 0 {
			add(in, nil)
		}
	}
	return out, nil
}

// containerStates reads the state and memory of every app container: by
// instance, "running", "stopped" or "partial".
func (m *Manager) containerStates(ctx context.Context, memory bool) (map[string]string, map[string]int) {
	status, mem := map[string]string{}, map[string]int{}
	out, err := m.run.Output(ctx, "docker", "ps", "-a", "--format", "{{.Names}}\t{{.State}}\t{{.Label \"panel.instance\"}}")
	if err != nil {
		return status, mem
	}
	running, total := map[string]int{}, map[string]int{}
	for _, l := range strings.Split(out, "\n") {
		f := strings.Split(strings.TrimSpace(l), "\t")
		if len(f) < 2 || !strings.HasPrefix(f[0], "panel-") {
			continue
		}
		inst := ""
		if len(f) == 3 && f[2] != "" {
			inst = f[2]
		} else {
			inst = strings.TrimPrefix(f[0], "panel-") // installed before the catalog
		}
		total[inst]++
		if f[1] == "running" {
			running[inst]++
		}
	}
	for inst, n := range total {
		switch running[inst] {
		case n:
			status[inst] = "running"
		case 0:
			status[inst] = "stopped"
		default:
			status[inst] = "partial"
		}
	}
	if !memory {
		return status, mem
	}
	if out, err := m.run.Output(ctx, "docker", "stats", "--no-stream", "--format", "{{.Name}}\t{{.MemUsage}}"); err == nil {
		for _, l := range strings.Split(out, "\n") {
			name, usage, ok := strings.Cut(strings.TrimSpace(l), "\t")
			if !ok || !strings.HasPrefix(name, "panel-") {
				continue
			}
			inst := strings.TrimPrefix(name, "panel-")
			for i := range total {
				if strings.HasPrefix(name, "panel-"+i+"-") {
					inst = i
				}
			}
			mem[inst] += parseMemMB(strings.TrimSpace(strings.Split(usage, "/")[0]))
		}
	}
	return status, mem
}

// parseMemMB reads docker stats' "123.4MiB" or "1.2GiB".
func parseMemMB(s string) int {
	units := []struct {
		suffix string
		mb     float64
	}{{"GiB", 1024}, {"MiB", 1}, {"KiB", 1.0 / 1024}, {"GB", 1000}, {"MB", 1}, {"kB", 0.001}, {"B", 1.0 / (1 << 20)}}
	for _, u := range units {
		if v, ok := strings.CutSuffix(s, u.suffix); ok {
			f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil {
				return 0
			}
			return int(f*u.mb + 0.5)
		}
	}
	return 0
}

// phpAppVersion reads the version of a PHP app from its files.
func (m *Manager) phpAppVersion(site *store.Site) string {
	root := m.paths.siteRoot(site.Domain)
	read := func(rel string, re string) string {
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			return ""
		}
		mm := regexpCache(re).FindSubmatch(b)
		if mm == nil {
			return ""
		}
		return string(mm[1])
	}
	switch site.App {
	case AppWordPress:
		return read("public/wp-includes/version.php", `\$wp_version\s*=\s*'([0-9.]+)'`)
	case "woocommerce":
		return read("public/wp-content/plugins/woocommerce/woocommerce.php", `(?m)^\s*\*\s*Version:\s*([0-9.]+)`)
	case "joomla":
		maj := read("public/libraries/src/Version.php", `MAJOR_VERSION\s*=\s*([0-9]+)`)
		min := read("public/libraries/src/Version.php", `MINOR_VERSION\s*=\s*([0-9]+)`)
		pat := read("public/libraries/src/Version.php", `PATCH_VERSION\s*=\s*([0-9]+)`)
		if maj == "" {
			return ""
		}
		return maj + "." + min + "." + pat
	case "nextcloud":
		return read("public/version.php", `OC_VersionString\s*=\s*'([0-9.]+)'`)
	case "moodle":
		return read("version.php", `\$release\s*=\s*'([0-9.]+)`)
	case "opencart":
		return read("public/index.php", `define\('VERSION',\s*'([0-9.]+)'`)
	}
	return ""
}

// ---- actions and logs ----

// AppInstanceAction starts, stops or restarts an instance's containers.
func (m *Manager) AppInstanceAction(ctx context.Context, name, action string) error {
	if action != "start" && action != "stop" && action != "restart" {
		return usererr.New("action_not_allowed", action)
	}
	if _, err := m.store.AppByName(ctx, name); err != nil {
		return usererr.New("app_not_installed", name)
	}
	if action == "start" {
		_, err := m.run.Output(ctx, "docker", composeArgs(name, "up", "-d", "--no-recreate")...)
		return err
	}
	_, err := m.run.Output(ctx, "docker", composeArgs(name, action)...)
	return err
}

// AppInstanceLogs returns the last lines of an instance's containers.
func (m *Manager) AppInstanceLogs(ctx context.Context, name string) (string, error) {
	if _, err := m.store.AppByName(ctx, name); err != nil {
		return "", usererr.New("app_not_installed", name)
	}
	return m.run.Output(ctx, "docker", composeArgs(name, "logs", "--no-color", "--timestamps", "--tail", "200")...)
}

// dockerSiteAction is AppAction for a Docker app's website.
func (m *Manager) dockerSiteAction(ctx context.Context, site *store.Site, action string) error {
	if inst, err := m.store.AppForSite(ctx, site.ID); err == nil {
		return m.AppInstanceAction(ctx, inst.Name, action)
	}
	_, err := m.run.Output(ctx, "docker", action, containerName(site))
	return err
}

func (m *Manager) dockerSiteLogs(ctx context.Context, site *store.Site) (string, error) {
	if inst, err := m.store.AppForSite(ctx, site.ID); err == nil {
		return m.AppInstanceLogs(ctx, inst.Name)
	}
	return m.run.Output(ctx, "docker", "logs", "--tail", "200", "--timestamps", containerName(site))
}

// dockerAppStatus returns the state of a site's main container ("active"
// when running, like a systemd service).
func (m *Manager) dockerAppStatus(ctx context.Context, site *store.Site) string {
	out, err := m.run.Output(ctx, "docker", "inspect", "-f", "{{.State.Status}}", containerName(site))
	st := strings.TrimSpace(out)
	if err != nil {
		return "missing"
	}
	if st == "running" {
		return "active"
	}
	return st
}

// ---- credentials ----

// Secret is a generated value shown to the administrator.
type Secret struct {
	Name  string        `json:"name"`
	Label appstore.Text `json:"label,omitempty"`
	Value string        `json:"value"`
}

// Credentials are how to reach and sign in to an installed app.
type Credentials struct {
	App        string        `json:"app"`
	Name       string        `json:"name"`
	Method     string        `json:"method"`
	URL        string        `json:"url,omitempty"`
	AdminURL   string        `json:"admin_url,omitempty"`
	User       string        `json:"user,omitempty"`
	Password   string        `json:"password,omitempty"`
	Connection string        `json:"connection,omitempty"`
	Host       string        `json:"host,omitempty"`
	Port       int           `json:"port,omitempty"`
	ExtraPorts map[int]int   `json:"extra_ports,omitempty"`
	Secrets    []Secret      `json:"secrets"`
	Note       appstore.Text `json:"note,omitempty"`
}

// AppCredentials returns an instance's sign-in details.
func (m *Manager) AppCredentials(ctx context.Context, name string) (*Credentials, error) {
	inst, err := m.store.AppByName(ctx, name)
	if err != nil {
		return nil, usererr.New("app_not_installed", name)
	}
	app, ok := appstore.Lookup(inst.AppID)
	if !ok {
		return nil, usererr.New("unknown_app", inst.AppID)
	}
	secrets, err := m.store.AppSecrets(ctx, inst.ID)
	if err != nil {
		return nil, err
	}
	var site *store.Site
	if inst.SiteID != 0 {
		if site, err = m.store.SiteByID(ctx, inst.SiteID); err != nil {
			return nil, err
		}
	}
	c := &Credentials{App: app.ID, Name: inst.Name, Method: app.Admin.Method, Port: inst.Port, ExtraPorts: inst.ExtraPorts,
		Secrets: []Secret{}, Note: app.Admin.Note}
	if site == nil {
		c.Host = "127.0.0.1"
	} else {
		scheme := "http"
		if site.SSL {
			scheme = "https"
		}
		c.URL = scheme + "://" + site.Domain + "/"
		c.AdminURL = scheme + "://" + site.Domain + app.Admin.URLPath
		if app.Admin.URLPath == "" {
			c.AdminURL = c.URL
		}
	}
	switch {
	case app.Admin.User != "":
		c.User = app.Admin.User
	case app.Admin.UserSecret != "":
		c.User = secrets[app.Admin.UserSecret]
	case app.Admin.EmailLogin:
		c.User = inst.AdminEmail
	}
	switch {
	case app.Admin.PasswordSecret != "":
		c.Password = secrets[app.Admin.PasswordSecret]
	case app.Admin.Method == "logs":
		c.Password = secrets["admin_password"]
	}
	if app.Admin.Connection != "" {
		env, err := m.appEnv(app, inst, site, secrets, false)
		if err == nil {
			c.Connection, _ = env.Expand(app.Admin.Connection)
		}
	}
	for _, k := range sortedNames(app.Docker.Secrets) {
		if s := app.Docker.Secrets[k]; s.Show {
			c.Secrets = append(c.Secrets, Secret{Name: k, Label: s.Label, Value: secrets[k]})
		}
	}
	return c, nil
}

// ---- HTTPS turned on ----

// refreshDockerApp recreates a Docker app's containers after its address
// changed (HTTPS turned on). The data stays.
func (m *Manager) refreshDockerApp(ctx context.Context, site *store.Site, lg Logger) {
	app, ok := siteDockerApp(site)
	if !ok {
		return
	}
	inst, err := m.store.AppForSite(ctx, site.ID)
	if errors.Is(err, store.ErrNotFound) {
		if l, ok := legacyApps[app.ID]; ok {
			lg.Logf("Restarting %s with its https:// address", app.Name)
			if err := m.runLegacy(ctx, site, l); err != nil {
				lg.Logf("Warning: %v", err)
			}
		}
		return
	}
	if err != nil || inst.State != "ready" {
		return
	}
	// The certificate code holds ops: no appOps here (lock order).
	lg.Logf("Restarting %s with its https:// address", app.Name)
	secrets, err := m.store.AppSecrets(ctx, inst.ID)
	if err == nil {
		var env *appstore.Env
		if env, err = m.appEnv(app, inst, site, secrets, false); err == nil {
			// The catalog may be newer than the instance: keep its version.
			if err = m.writeCompose(ctx, m.pinnedApp(app, inst), env); err == nil {
				_, err = m.run.Output(ctx, "docker", composeArgs(inst.Name, "up", "-d", "--remove-orphans")...)
			}
		}
	}
	if err != nil {
		lg.Logf("Warning: %v", err)
	}
}

// pinnedApp returns the recipe with the instance's own images when the
// catalog has moved on (used by re-renders that are not updates).
func (m *Manager) pinnedApp(app *appstore.App, inst *store.App) *appstore.App {
	refs := strings.Fields(inst.Images)
	names := sortedNames(app.Docker.Services)
	if len(refs) != len(names) || imageList(app) == inst.Images {
		return app
	}
	b, _ := json.Marshal(app)
	var cp appstore.App
	json.Unmarshal(b, &cp)
	for i, n := range names {
		img, digest, _ := strings.Cut(refs[i], "@")
		cp.Docker.Services[n].Image, cp.Docker.Services[n].Digest = img, digest
	}
	return &cp
}

// ---- updates ----

// CheckUpdate says whether an instance (or a website's legacy Docker app)
// can be updated in one click.
func (m *Manager) CheckUpdate(ctx context.Context, name string) (*appstore.App, *store.App, *store.Site, error) {
	inst, err := m.store.AppByName(ctx, name)
	if err == nil {
		app, ok := appstore.Lookup(inst.AppID)
		if !ok || app.Docker == nil {
			return nil, nil, nil, usererr.New("unknown_app", inst.AppID)
		}
		if inst.State == "installing" {
			return nil, nil, nil, usererr.New("app_busy", name)
		}
		if app.Version == inst.Version && imageList(app) == inst.Images {
			return nil, nil, nil, usererr.New("app_up_to_date", app.Version)
		}
		if breakingUpdate(app.Updates.Breaking, inst.Version, app.Version) {
			return nil, nil, nil, usererr.New("app_update_breaking", inst.Version+" → "+app.Version)
		}
		var site *store.Site
		if inst.SiteID != 0 {
			if site, err = m.store.SiteByID(ctx, inst.SiteID); err != nil {
				return nil, nil, nil, err
			}
		}
		return app, inst, site, nil
	}
	// A website whose Docker app was installed before the catalog.
	sites, err := m.store.ListSites(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, s := range sites {
		if s.SysUser != name {
			continue
		}
		app, ok := siteDockerApp(s)
		if !ok || app.Docker.Legacy == nil {
			break
		}
		return app, nil, s, nil
	}
	return nil, nil, nil, usererr.New("app_not_installed", name)
}

// breakingUpdate reports whether the part of the version named by
// breaking ("major" or "minor") changes.
func breakingUpdate(breaking, from, to string) bool {
	if breaking == "" {
		return false
	}
	n := 1
	if breaking == "minor" {
		n = 2
	}
	cut := func(v string) string {
		parts := strings.SplitN(v, ".", n+1)
		if len(parts) > n {
			parts = parts[:n]
		}
		return strings.Join(parts, ".")
	}
	return cut(from) != cut(to)
}

// UpdateApp moves an instance to the catalog's version: the new images are
// pulled first, then the containers are recreated; if the app does not
// come back, the previous compose file is put back. The caller makes a
// backup first. A legacy install (docker run) is converted.
func (m *Manager) UpdateApp(ctx context.Context, name string, lg Logger) error {
	app, inst, site, err := m.CheckUpdate(ctx, name)
	if err != nil {
		return err
	}
	if inst == nil {
		return m.convertLegacy(ctx, app, site, lg)
	}
	appOps.Lock()
	defer appOps.Unlock()
	have, err := m.store.AppSecrets(ctx, inst.ID)
	if err != nil {
		return err
	}
	secrets, err := appstore.GenerateSecrets(app.Docker.Secrets, have)
	if err != nil {
		return err
	}
	if err := m.store.SetAppSecrets(ctx, inst.ID, secrets); err != nil {
		return err
	}
	env, err := m.appEnv(app, inst, site, secrets, false)
	if err != nil {
		return err
	}
	file := composeFile(env.ProjectDir)
	prev, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	if err := writeFile(filepath.Join(env.ProjectDir, "compose.prev.json"), prev, 0o600); err != nil {
		return err
	}
	lg.Logf("Updating %s from %s to %s", app.Name, inst.Version, app.Version)
	if err := m.prepareInstance(ctx, app, env, lg); err != nil {
		writeFile(file, prev, 0o600)
		return err
	}
	lg.Logf("Downloading the new images before stopping anything")
	if err := m.run.Stream(ctx, lg, "docker", composeArgs(inst.Name, "pull", "--quiet")...); err != nil {
		writeFile(file, prev, 0o600)
		return err
	}
	oldImages := inst.Images
	if err := m.upInstance(ctx, app, inst, lg); err != nil {
		lg.Logf("The new version did not start; putting the previous one back")
		writeFile(file, prev, 0o600)
		if _, rerr := m.run.Output(context.WithoutCancel(ctx), "docker", composeArgs(inst.Name, "up", "-d", "--remove-orphans")...); rerr != nil {
			lg.Logf("Warning: %v", rerr)
		}
		return fmt.Errorf("%w\nThe previous version runs again; if the app changed its data, restore the backup made before the update", err)
	}
	inst.Version, inst.Images, inst.State = app.Version, imageList(app), "ready"
	if err := m.store.UpdateApp(ctx, inst); err != nil {
		return err
	}
	for _, ref := range strings.Fields(oldImages) {
		if !strings.Contains(" "+inst.Images+" ", " "+ref+" ") {
			m.run.Output(ctx, "docker", "image", "rm", ref)
		}
	}
	lg.Logf("%s now runs %s", app.Name, app.Version)
	return nil
}

// ---- backups (used by internal/backup) ----

// AppBackupPlan says how to back up an instance.
type AppBackupPlan struct {
	Name        string
	AppID       string
	Dir         string // the instance's folder
	Consistency string // pause, dump or none
	Dumps       []appstore.Dump
	Exclude     []string // volume folders under data/ not copied (dumped instead)
	// Secrets are the instance's secrets, kept in the archive (which
	// already holds compose.json and the secret files) so that a restore
	// brings back the passwords that go with the restored data.
	Secrets map[string]string
}

// AppBackup returns the backup plan of an instance.
func (m *Manager) AppBackup(ctx context.Context, name string) (*AppBackupPlan, error) {
	inst, err := m.store.AppByName(ctx, name)
	if err != nil {
		return nil, usererr.New("app_not_installed", name)
	}
	app, ok := appstore.Lookup(inst.AppID)
	if !ok || app.Docker == nil {
		return nil, usererr.New("unknown_app", inst.AppID)
	}
	dir, err := instanceDir(name)
	if err != nil {
		return nil, err
	}
	secrets, err := m.store.AppSecrets(ctx, inst.ID)
	if err != nil {
		return nil, err
	}
	b := app.Docker.Backup
	return &AppBackupPlan{Name: name, AppID: app.ID, Dir: dir, Consistency: b.Consistency, Dumps: b.Dumps,
		Exclude: b.Exclude, Secrets: secrets}, nil
}

// AppPause freezes an instance's containers; the returned function thaws
// them.
func (m *Manager) AppPause(ctx context.Context, name string) (func(), error) {
	if _, err := m.run.Output(ctx, "docker", composeArgs(name, "pause")...); err != nil {
		return func() {}, err
	}
	return func() {
		m.run.Output(context.WithoutCancel(ctx), "docker", composeArgs(name, "unpause")...)
	}, nil
}

// AppDump writes a database dump of an instance to w.
func (m *Manager) AppDump(ctx context.Context, name string, d appstore.Dump, w io.Writer) error {
	return m.run.Pipe(ctx, nil, w, "docker", composeArgs(name, append([]string{"exec", "-T", d.Service}, d.Argv...)...)...)
}

// AppRestoreDump loads a dump into a running instance.
func (m *Manager) AppRestoreDump(ctx context.Context, name string, d appstore.Dump, r io.Reader) error {
	return m.run.Pipe(ctx, r, io.Discard, "docker", composeArgs(name, append([]string{"exec", "-T", d.Service}, d.RestoreArgv...)...)...)
}

// AppStop stops and removes an instance's containers (data stays).
func (m *Manager) AppStop(ctx context.Context, name string) error {
	_, err := m.run.Output(ctx, "docker", composeArgs(name, "down", "--remove-orphans", "--timeout", "20")...)
	return err
}

// AppStart starts an instance again and waits until it answers.
func (m *Manager) AppStart(ctx context.Context, name string, lg Logger) error {
	inst, err := m.store.AppByName(ctx, name)
	if err != nil {
		return usererr.New("app_not_installed", name)
	}
	app, ok := appstore.Lookup(inst.AppID)
	if !ok || app.Docker == nil {
		return usererr.New("unknown_app", inst.AppID)
	}
	return m.upInstance(ctx, app, inst, lg)
}

// AppRestored makes an instance whole again after its folder came back
// from a backup. The backup's secrets replace the current ones (the
// restored data and files written once were made with them: a backup made
// before a reinstall has other passwords than the new instance), then
// what is missing of the folders is recreated (a dumped database's folder
// is left out of backups) and compose.json is rendered again for the
// instance as it is now (its ports and images). saved is nil for an
// archive without secrets: the current ones stay.
func (m *Manager) AppRestored(ctx context.Context, name string, saved map[string]string, lg Logger) error {
	inst, err := m.store.AppByName(ctx, name)
	if err != nil {
		return usererr.New("app_not_installed", name)
	}
	app, ok := appstore.Lookup(inst.AppID)
	if !ok || app.Docker == nil {
		return usererr.New("unknown_app", inst.AppID)
	}
	secrets, err := m.store.AppSecrets(ctx, inst.ID)
	if err != nil {
		return err
	}
	changed := map[string]string{}
	for k, v := range saved {
		if _, known := app.Docker.Secrets[k]; known && v != "" && v != secrets[k] {
			changed[k] = v
			secrets[k] = v
		}
	}
	if len(changed) > 0 {
		lg.Logf("Taking back the passwords of the backup")
		if err := m.store.SetAppSecrets(ctx, inst.ID, changed); err != nil {
			return err
		}
	}
	var site *store.Site
	if inst.SiteID != 0 {
		if site, err = m.store.SiteByID(ctx, inst.SiteID); err != nil {
			return err
		}
	}
	env, err := m.appEnv(app, inst, site, secrets, false)
	if err != nil {
		return err
	}
	pinned := m.pinnedApp(app, inst)
	if err := m.prepareInstance(ctx, pinned, env, lg); err != nil {
		return err
	}
	if w := app.Docker.Web; site != nil && w != nil && w.BasicAuth && changed[app.Admin.PasswordSecret] != "" {
		return m.setAppSitePassword(ctx, site.ID, changed[app.Admin.PasswordSecret])
	}
	return nil
}

// setAppSitePassword sets the password of a website protected by its app's
// recipe (Stirling-PDF) again.
func (m *Manager) setAppSitePassword(ctx context.Context, siteID int64, pw string) error {
	m.ops.Lock()
	defer m.ops.Unlock()
	site, err := m.store.SiteByID(ctx, siteID)
	if err != nil {
		return err
	}
	old := *site
	h, err := bcryptHash(pw)
	if err != nil {
		return err
	}
	site.AuthUser, site.AuthHash = "admin", h
	if err := m.writeAuth(site); err != nil {
		return err
	}
	if err := m.updateSite(ctx, site, &old); err != nil {
		m.writeAuth(&old)
		return err
	}
	return nil
}

// AppNameForSite returns the instance of a website's Docker app, or "".
func (m *Manager) AppNameForSite(ctx context.Context, siteID int64) string {
	if inst, err := m.store.AppForSite(ctx, siteID); err == nil {
		return inst.Name
	}
	return ""
}
