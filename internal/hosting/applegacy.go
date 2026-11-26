package hosting

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"

	"github.com/nefocp/nefo/internal/appstore"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/validate"
)

// Docker apps installed before the catalog (panel 0.14 to 0.16) run as one
// container started with "docker run", with a floating tag and their data
// in <site root>/data. They keep working as they are; the first update
// converts them to a Compose instance: backup (by the caller), stop the old
// container, move the data into the instance's folder, start the pinned
// version. If the new version does not start, the data goes back and the
// old container is started again.

type legacyApp struct {
	image string
	port  int
	data  string
	env   func(url, domain string, https bool) []string
}

var legacyApps = map[string]legacyApp{
	"ghost": {image: "ghost:6-alpine", port: 2368, data: "/var/lib/ghost/content",
		env: func(url, _ string, _ bool) []string {
			return []string{"url=" + url, "database__client=sqlite3",
				"database__connection__filename=/var/lib/ghost/content/data/ghost.db"}
		}},
	"uptime-kuma": {image: "louislam/uptime-kuma:2", port: 3001, data: "/app/data",
		env: func(string, string, bool) []string { return nil }},
	"n8n": {image: "n8nio/n8n:stable", port: 5678, data: "/home/node/.n8n",
		env: func(url, domain string, https bool) []string {
			proto := "http"
			if https {
				proto = "https"
			}
			return []string{"N8N_HOST=" + domain, "N8N_PORT=5678", "N8N_PROTOCOL=" + proto, "WEBHOOK_URL=" + url + "/",
				"N8N_PROXY_HOPS=1", "N8N_SECURE_COOKIE=" + strconv.FormatBool(https), "GENERIC_TIMEZONE=UTC"}
		}},
	"gitea": {image: "gitea/gitea:1", port: 3000, data: "/data",
		env: func(url, domain string, _ bool) []string {
			return []string{"USER_UID=1000", "USER_GID=1000", "GITEA__server__ROOT_URL=" + url + "/",
				"GITEA__server__DOMAIN=" + domain, "GITEA__server__DISABLE_SSH=true", "GITEA__database__DB_TYPE=sqlite3"}
		}},
}

// runLegacy (re)creates a legacy app's container, as panel 0.16 did.
func (m *Manager) runLegacy(ctx context.Context, site *store.Site, l legacyApp) error {
	if validate.SystemUsername(site.SysUser) != nil || validate.Domain(site.Domain) != nil || site.ProxyPort < 1024 {
		return errors.New("invalid site")
	}
	scheme := "http"
	if site.SSL {
		scheme = "https"
	}
	url := scheme + "://" + site.Domain
	m.run.Output(ctx, "docker", "rm", "-f", containerName(site))
	args := []string{"run", "-d", "--name", containerName(site), "--restart", "unless-stopped",
		"-p", fmt.Sprintf("127.0.0.1:%d:%d", site.ProxyPort, l.port),
		"-v", filepath.Join(m.paths.siteRoot(site.Domain), "data") + ":" + l.data,
		"--memory", "1024m", "--log-opt", "max-size=10m", "--log-opt", "max-file=3",
		"--label", "panel.site=" + site.Domain}
	for _, e := range l.env(url, site.Domain, site.SSL) {
		args = append(args, "-e", e)
	}
	args = append(args, l.image)
	_, err := m.run.Output(ctx, "docker", args...)
	return err
}

// convertLegacy moves a legacy install to a Compose instance of the
// catalog's version.
func (m *Manager) convertLegacy(ctx context.Context, app *appstore.App, site *store.Site, lg Logger) error {
	appOps.Lock()
	defer appOps.Unlock()
	l, ok := legacyApps[app.ID]
	if !ok || app.Docker.Legacy == nil || app.Kind != appstore.KindDocker {
		return fmt.Errorf("%s cannot be converted", app.ID)
	}
	oldData := filepath.Join(m.paths.siteRoot(site.Domain), "data")
	if _, err := os.Stat(oldData); err != nil {
		return fmt.Errorf("the app's data folder %s is missing: %w", oldData, err)
	}
	inst := &store.App{Name: site.SysUser, AppID: app.ID, Kind: app.Kind, SiteID: site.ID, Version: app.Version,
		Images: imageList(app), Port: site.ProxyPort, ExtraPorts: map[int]int{}, Inputs: map[string]string{}}
	secrets, err := appstore.GenerateSecrets(app.Docker.Secrets, nil)
	if err != nil {
		return err
	}
	if err := m.store.CreateApp(ctx, inst); err != nil {
		return err
	}
	if err := m.store.SetAppSecrets(ctx, inst.ID, secrets); err != nil {
		return err
	}
	env, err := m.appEnv(app, inst, site, secrets, false)
	if err != nil {
		m.store.DeleteApp(ctx, inst.ID)
		return err
	}
	newData := filepath.Join(env.DataDir(), app.Docker.Legacy.Volume)
	bg := context.WithoutCancel(ctx)
	moved := false
	undo := func(cause error) error {
		lg.Logf("Putting the previous installation back")
		m.run.Output(bg, "docker", composeArgs(inst.Name, "down", "--remove-orphans")...)
		if moved {
			if err := m.moveTree(bg, newData, oldData); err != nil {
				lg.Logf("Warning: %v", err)
			}
		}
		os.RemoveAll(env.ProjectDir)
		m.store.DeleteApp(bg, inst.ID)
		if err := m.runLegacy(bg, site, l); err != nil {
			lg.Logf("Warning: %v", err)
		}
		return cause
	}
	lg.Logf("Converting %s to the pinned version %s", app.Name, app.Version)
	if err := mkdir(AppsDir, 0o700); err != nil {
		return undo(err)
	}
	for _, d := range []string{env.ProjectDir, env.DataDir(), env.SecretsDir()} {
		if err := mkdir(d, 0o700); err != nil {
			return undo(err)
		}
	}
	lg.Logf("Stopping the old container")
	if _, err := m.run.Output(ctx, "docker", "rm", "-f", containerName(site)); err != nil {
		return undo(err)
	}
	lg.Logf("Moving the data to %s", newData)
	if err := m.moveTree(ctx, oldData, newData); err != nil {
		return undo(err)
	}
	moved = true
	// The new recipe may run the app as another user (the website's).
	dirs, _, err := appstore.Layout(app, env)
	if err != nil {
		return undo(err)
	}
	for _, d := range dirs {
		if d.Path == newData {
			if _, err := m.run.Output(ctx, "chown", "-R", fmt.Sprintf("%d:%d", d.UID, d.GID), newData); err != nil {
				return undo(err)
			}
		}
	}
	if err := m.prepareInstance(ctx, app, env, lg); err != nil {
		return undo(err)
	}
	if err := m.startInstance(ctx, app, inst, env, false, lg); err != nil {
		return undo(err)
	}
	inst.State = "ready"
	if err := m.store.UpdateApp(ctx, inst); err != nil {
		return err
	}
	m.run.Output(ctx, "docker", "image", "rm", l.image)
	lg.Logf("%s now runs %s from %s", app.Name, app.Version, env.ProjectDir)
	return nil
}

// moveTree moves a folder, across file systems if needed (mv keeps owners
// and modes).
func (m *Manager) moveTree(ctx context.Context, from, to string) error {
	if err := os.Rename(from, to); err == nil {
		return nil
	}
	if _, err := os.Stat(to); err == nil {
		if err := os.Remove(to); err != nil {
			return fmt.Errorf("%s is not empty", to)
		}
	}
	_, err := m.run.Output(ctx, "mv", "-T", from, to)
	return err
}

var (
	reCacheMu sync.Mutex
	reCache   = map[string]*regexp.Regexp{}
)

// regexpCache compiles constant patterns once.
func regexpCache(p string) *regexp.Regexp {
	reCacheMu.Lock()
	defer reCacheMu.Unlock()
	re, ok := reCache[p]
	if !ok {
		re = regexp.MustCompile(p)
		reCache[p] = re
	}
	return re
}
