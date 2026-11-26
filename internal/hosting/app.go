package hosting

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
)

// App sites are Node.js, Python or other programs that listen on a local
// port. The web server forwards requests to them, and the panel can run
// the program as a systemd service under the site's own user. The start
// command is written by the administrator (root on this server), like a
// cron job.

const appScriptDir = "/usr/local/lib/panel-apps"

func appUnit(user string) string     { return "panel-app-" + user + ".service" }
func appUnitFile(user string) string { return filepath.Join("/etc/systemd/system", appUnit(user)) }
func appScript(user string) string   { return filepath.Join(appScriptDir, user+".sh") }

// ReservedPorts are ports app sites may not use, such as the panel's.
func (m *Manager) SetReservedPorts(ports ...int) { m.reserved = ports }

func (m *Manager) checkApp(ctx context.Context, port int, command string, siteID int64) error {
	if port < 1024 || port > 65535 || slices.Contains(m.reserved, port) || port == 3306 || port == 6379 {
		return usererr.New("invalid_app_port", fmt.Sprint(port))
	}
	if len(command) > 1000 || strings.ContainsAny(command, "\n\r\x00") {
		return usererr.New("invalid_app_command")
	}
	sites, err := m.store.ListSites(ctx)
	if err != nil {
		return err
	}
	for _, s := range sites {
		if s.ID != siteID && s.ProxyPort == port {
			return usererr.New("app_port_taken", s.Domain)
		}
	}
	return nil
}

// writeApp writes (or, without a command, removes) the app's service and
// restarts it.
func (m *Manager) writeApp(ctx context.Context, site *store.Site) error {
	if validate.SystemUsername(site.SysUser) != nil || validate.Domain(site.Domain) != nil {
		return fmt.Errorf("invalid site")
	}
	if site.AppCommand == "" {
		return m.removeApp(ctx, site)
	}
	root := m.paths.siteRoot(site.Domain)
	script := fmt.Sprintf("#!/bin/sh\n# Managed by the panel. Start command of %s.\ncd %s/app || exit 1\n%s\n",
		site.Domain, root, site.AppCommand)
	unit := fmt.Sprintf(`# Managed by the panel. Do not edit: the panel rewrites this file.
[Unit]
Description=App of %[1]s
After=network.target

[Service]
Type=simple
User=%[2]s
Group=%[2]s
WorkingDirectory=%[3]s/app
Environment=PORT=%[4]d
Environment=HOST=127.0.0.1
Environment=NODE_ENV=production
Environment=PATH=%[6]s
ExecStart=%[5]s
Restart=always
RestartSec=3
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=full
ProtectHome=yes

[Install]
WantedBy=multi-user.target
`, site.Domain, site.SysUser, root, site.ProxyPort, appScript(site.SysUser), m.runtimePath(site))
	if err := mkdir(appScriptDir, 0o755); err != nil {
		return err
	}
	if err := writeFile(appScript(site.SysUser), []byte(script), 0o755); err != nil {
		return err
	}
	if err := writeFile(appUnitFile(site.SysUser), []byte(unit), 0o644); err != nil {
		return err
	}
	if _, err := m.run.Output(ctx, "systemctl", "daemon-reload"); err != nil {
		return err
	}
	if _, err := m.run.Output(ctx, "systemctl", "enable", appUnit(site.SysUser)); err != nil {
		return err
	}
	_, err := m.run.Output(ctx, "systemctl", "restart", appUnit(site.SysUser))
	return err
}

func (m *Manager) removeApp(ctx context.Context, site *store.Site) error {
	if validate.SystemUsername(site.SysUser) != nil {
		return fmt.Errorf("invalid site")
	}
	if !fileExists(appUnitFile(site.SysUser)) {
		return nil
	}
	m.run.Output(ctx, "systemctl", "disable", "--now", appUnit(site.SysUser))
	removeIfExists(appUnitFile(site.SysUser))
	removeIfExists(appScript(site.SysUser))
	_, err := m.run.Output(ctx, "systemctl", "daemon-reload")
	return err
}

// AppStatus returns the state of a site's app service ("" without one).
func (m *Manager) AppStatus(ctx context.Context, site *store.Site) string {
	if _, ok := siteDockerApp(site); ok && validate.SystemUsername(site.SysUser) == nil {
		return m.dockerAppStatus(ctx, site)
	}
	if site.AppCommand == "" || validate.SystemUsername(site.SysUser) != nil {
		return ""
	}
	out, _ := m.run.Output(ctx, "systemctl", "is-active", appUnit(site.SysUser))
	return strings.TrimSpace(out)
}

// AppAction starts, stops or restarts a site's app.
func (m *Manager) AppAction(ctx context.Context, id int64, action string) error {
	if action != "start" && action != "stop" && action != "restart" {
		return usererr.New("action_not_allowed", action)
	}
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return err
	}
	if _, ok := siteDockerApp(site); ok && validate.SystemUsername(site.SysUser) == nil {
		return m.dockerSiteAction(ctx, site, action)
	}
	if site.AppCommand == "" || validate.SystemUsername(site.SysUser) != nil {
		return usererr.New("no_app")
	}
	_, err = m.run.Output(ctx, "systemctl", action, appUnit(site.SysUser))
	return err
}

// AppLogs returns the last lines of a site's app output.
func (m *Manager) AppLogs(ctx context.Context, id int64) (string, error) {
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return "", err
	}
	if _, ok := siteDockerApp(site); ok && validate.SystemUsername(site.SysUser) == nil {
		return m.dockerSiteLogs(ctx, site)
	}
	if site.AppCommand == "" || validate.SystemUsername(site.SysUser) != nil {
		return "", usererr.New("no_app")
	}
	return m.run.Output(ctx, "journalctl", "-u", appUnit(site.SysUser), "-n", "200", "--no-pager", "-o", "short-iso")
}

// appDir creates the folder an app's code goes into.
func (m *Manager) appDir(site *store.Site, uid, gid int) error {
	dir := filepath.Join(m.paths.siteRoot(site.Domain), "app")
	if err := mkdir(dir, 0o750); err != nil {
		return err
	}
	return os.Chown(dir, uid, gid)
}
