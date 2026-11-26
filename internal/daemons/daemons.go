// Package daemons keeps background programs running, like Supervisor: queue
// workers and other long-running commands of a website. Each program is a
// systemd service that runs as the website's Linux user, is started again
// when it exits, and writes its output to the journal.
//
// The command is written by the administrator (root on this server), like a
// cron job. The panel makes sure it fits on one line, that the working folder
// stays inside the website's folder and that the program never runs as root.
package daemons

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
)

// Store is the part of the panel database the manager uses.
type Store interface {
	ListDaemons(ctx context.Context) ([]*store.Daemon, error)
	DaemonsForSite(ctx context.Context, siteID int64) ([]*store.Daemon, error)
	DaemonByID(ctx context.Context, id int64) (*store.Daemon, error)
	SaveDaemon(ctx context.Context, d *store.Daemon) error
	DeleteDaemon(ctx context.Context, id int64) error
	SiteByID(ctx context.Context, id int64) (*store.Site, error)
	ListSites(ctx context.Context) ([]*store.Site, error)
}

const (
	maxName    = 60
	maxCommand = 1000
	maxEnv     = 20
	maxDir     = 200
)

var (
	// KEY=value: a shell-style name, and a value without control characters.
	envRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}=[^\x00-\x1f\x7f]{0,1000}$`)
	// A relative folder of plain names. No spaces, quotes or "%", so the
	// path is safe in the start script and in the unit file.
	dirRe = regexp.MustCompile(`^[A-Za-z0-9._@+-]+(/[A-Za-z0-9._@+-]+)*$`)
)

// Manager creates and controls background programs.
type Manager struct {
	store     Store
	run       run.Runner
	www       string // site folders: <www>/<domain>
	unitDir   string // systemd units
	scriptDir string // start scripts and environment files
	lookup    func(name string) error
	supported func() bool
	mu        sync.Mutex
}

// New returns a Manager for the standard locations.
func New(st Store, r run.Runner) *Manager {
	return &Manager{
		store:     st,
		run:       r,
		www:       "/var/www",
		unitDir:   "/etc/systemd/system",
		scriptDir: "/usr/local/lib/panel-daemons",
		lookup:    func(name string) error { _, err := user.Lookup(name); return err },
		supported: func() bool {
			_, err := exec.LookPath("systemctl")
			return runtime.GOOS == "linux" && err == nil
		},
	}
}

// Unit is the systemd unit of a program.
func Unit(id int64) string { return "panel-daemon-" + strconv.FormatInt(id, 10) + ".service" }

func (m *Manager) unitPath(id int64) string { return filepath.Join(m.unitDir, Unit(id)) }
func (m *Manager) scriptPath(id int64) string {
	return filepath.Join(m.scriptDir, strconv.FormatInt(id, 10)+".sh")
}
func (m *Manager) envPath(id int64) string {
	return filepath.Join(m.scriptDir, strconv.FormatInt(id, 10)+".env")
}

// siteRoot is the site's folder, as written into the start script.
func (m *Manager) siteRoot(site *store.Site) string {
	return path.Join(filepath.ToSlash(m.www), site.Domain)
}

// Request is what the administrator enters for a program.
type Request struct {
	Name    string   `json:"name"`
	SiteID  int64    `json:"site_id"`
	Command string   `json:"command"`
	Dir     string   `json:"dir"`
	Env     []string `json:"env"`
	// Enabled starts the program now and at boot; nil means true.
	Enabled *bool `json:"enabled"`
}

// Status is the state of a program's service.
type Status struct {
	Active   string     `json:"active"` // active, inactive, failed, activating…
	Sub      string     `json:"sub"`    // running, dead, auto-restart…
	Restarts int        `json:"restarts"`
	PID      int        `json:"pid"`
	Memory   int64      `json:"memory"`
	Since    *time.Time `json:"since,omitempty"`
}

// Program is a stored program with its website and its current state.
type Program struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	SiteID    int64     `json:"site_id"`
	Site      string    `json:"site"` // the website's domain; "" if it no longer exists
	User      string    `json:"user"`
	Command   string    `json:"command"`
	Dir       string    `json:"dir"`
	Env       []string  `json:"env"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	Unit      string    `json:"unit"`
	Status    Status    `json:"status"`
}

// Supported reports whether this machine runs systemd.
func (m *Manager) Supported() bool { return m.supported() }

// cleanDir turns what the administrator typed into a folder relative to the
// site's folder. A full path inside the site is accepted too.
func cleanDir(dir, root string) (string, bool) {
	dir = strings.TrimSpace(dir)
	if rest, ok := strings.CutPrefix(dir, root); ok && (rest == "" || rest[0] == '/') {
		dir = rest[min(1, len(rest)):]
	}
	dir = strings.TrimSuffix(strings.TrimPrefix(dir, "./"), "/")
	if dir == "" || dir == "." {
		return "", true
	}
	return dir, validDir(dir)
}

// validDir checks a relative folder: plain names, no "." or "..".
func validDir(dir string) bool {
	if len(dir) > maxDir || !dirRe.MatchString(dir) {
		return false
	}
	for _, el := range strings.Split(dir, "/") {
		if el == "." || el == ".." {
			return false
		}
	}
	return true
}

func validName(s string) bool {
	n := utf8.RuneCountInString(s)
	return n >= 1 && n <= maxName && utf8.ValidString(s) && strings.IndexFunc(s, unicode.IsControl) < 0
}

func validCommand(s string) bool {
	return s != "" && len(s) <= maxCommand && !strings.ContainsAny(s, "\n\r\x00")
}

// cleanEnv checks KEY=value lines. Empty lines and # comments are dropped.
func cleanEnv(lines []string) ([]string, error) {
	out := []string{}
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		if !envRe.MatchString(l) {
			// Only the name in the message: the value may be a secret.
			key, _, _ := strings.Cut(l, "=")
			return nil, usererr.New("invalid_daemon_env", truncate(key, 40))
		}
		out = append(out, l)
	}
	if len(out) > maxEnv {
		return nil, usererr.New("too_many_daemon_env", strconv.Itoa(maxEnv))
	}
	return out, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s + "…"
}

// checkFields validates a program that is about to be written to disk.
func checkFields(d *store.Daemon, site *store.Site) error {
	if !validName(d.Name) {
		return usererr.New("invalid_daemon_name")
	}
	if !validCommand(d.Command) {
		return usererr.New("invalid_daemon_command")
	}
	if _, err := cleanEnv(d.Env); err != nil {
		return err
	}
	if d.Dir != "" && !validDir(d.Dir) {
		return usererr.New("invalid_daemon_dir", d.Dir)
	}
	if site == nil || validate.Domain(site.Domain) != nil {
		return usererr.New("invalid_daemon_site")
	}
	if site.SysUser == "root" || validate.SystemUsername(site.SysUser) != nil {
		return usererr.New("invalid_daemon_user", site.SysUser)
	}
	return nil
}

// check validates a request and returns the program to store and its site.
func (m *Manager) check(ctx context.Context, req Request) (*store.Daemon, *store.Site, error) {
	d := &store.Daemon{
		Name:    strings.TrimSpace(req.Name),
		SiteID:  req.SiteID,
		Command: strings.TrimSpace(req.Command),
		Enabled: req.Enabled == nil || *req.Enabled,
	}
	if !validName(d.Name) {
		return nil, nil, usererr.New("invalid_daemon_name")
	}
	if !validCommand(d.Command) {
		return nil, nil, usererr.New("invalid_daemon_command")
	}
	env, err := cleanEnv(req.Env)
	if err != nil {
		return nil, nil, err
	}
	d.Env = env
	site, err := m.store.SiteByID(ctx, req.SiteID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil, usererr.New("invalid_daemon_site")
	}
	if err != nil {
		return nil, nil, err
	}
	dir, ok := cleanDir(req.Dir, m.siteRoot(site))
	if !ok {
		return nil, nil, usererr.New("invalid_daemon_dir", truncate(req.Dir, 100))
	}
	d.Dir = dir
	if err := checkFields(d, site); err != nil {
		return nil, nil, err
	}
	if err := m.lookup(site.SysUser); err != nil {
		return nil, nil, usererr.New("invalid_daemon_user", site.SysUser)
	}
	if err := m.checkDirExists(site, dir); err != nil {
		return nil, nil, err
	}
	return d, site, nil
}

// checkDirExists makes sure the working folder exists and, after following
// symbolic links, is inside the site's folder.
func (m *Manager) checkDirExists(site *store.Site, dir string) error {
	root := filepath.Join(m.www, site.Domain)
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("the website's folder %s is missing: %w", root, err)
	}
	shown := dir
	if shown == "" {
		shown = "."
	}
	real, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(dir)))
	if err != nil {
		return usererr.New("daemon_dir_missing", shown)
	}
	rel, err := filepath.Rel(realRoot, real)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return usererr.New("daemon_dir_outside", shown)
	}
	if fi, err := os.Stat(real); err != nil || !fi.IsDir() {
		return usererr.New("daemon_dir_missing", shown)
	}
	return nil
}

// ---- files ----

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// script is the start script: it changes to the working folder and replaces
// itself with the command, so systemd watches and signals the program itself.
func (m *Manager) script(d *store.Daemon, site *store.Site) string {
	dir := m.siteRoot(site)
	if d.Dir != "" {
		dir = path.Join(dir, d.Dir)
	}
	return fmt.Sprintf("#!/bin/sh\n# Managed by the panel. Do not edit: the panel rewrites this file.\n"+
		"# Background program %d of %s.\ncd %s || exit 1\nexec %s\n", d.ID, site.Domain, shellQuote(dir), d.Command)
}

// envFile is read by systemd (as root) before it starts the program. The
// file is root-only, so secrets in it stay private, unlike Environment=
// lines in the unit, which every user can read.
func envFile(env []string) string {
	var b strings.Builder
	b.WriteString("# Managed by the panel. Do not edit: the panel rewrites this file.\n")
	for _, l := range env {
		k, v, _ := strings.Cut(l, "=")
		b.WriteString(k)
		b.WriteString(`="`)
		for _, r := range v {
			// Inside double quotes, systemd treats \ " ` and $ specially.
			if strings.ContainsRune("\\\"`$", r) {
				b.WriteByte('\\')
			}
			b.WriteRune(r)
		}
		b.WriteString("\"\n")
	}
	return b.String()
}

func (m *Manager) unit(d *store.Daemon, site *store.Site) string {
	return fmt.Sprintf(`# Managed by the panel. Do not edit: the panel rewrites this file.
[Unit]
Description=Background program %[1]d of %[2]s
After=network.target mariadb.service mysql.service redis-server.service

[Service]
Type=simple
User=%[3]s
Group=%[3]s
UMask=0027
Environment=PATH=/usr/local/bin:/usr/bin:/bin
EnvironmentFile=%[4]s
ExecStart=%[5]s
Restart=always
RestartSec=3
TimeoutStopSec=30
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=full
ProtectHome=yes

[Install]
WantedBy=multi-user.target
`, d.ID, site.Domain, site.SysUser, filepath.ToSlash(m.envPath(d.ID)), filepath.ToSlash(m.scriptPath(d.ID)))
}

func (m *Manager) systemctl(ctx context.Context, args ...string) error {
	_, err := m.run.Output(ctx, "systemctl", args...)
	return err
}

// install writes a program's files and starts it, or stops it when it is
// not enabled.
func (m *Manager) install(ctx context.Context, d *store.Daemon, site *store.Site) error {
	if err := checkFields(d, site); err != nil {
		return err
	}
	if err := os.MkdirAll(m.scriptDir, 0o755); err != nil {
		return err
	}
	// The program's user runs the script, so the folder must be readable.
	if err := os.Chmod(m.scriptDir, 0o755); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(m.scriptPath(d.ID), []byte(m.script(d, site)), 0o755); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(m.envPath(d.ID), []byte(envFile(d.Env)), 0o600); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(m.unitPath(d.ID), []byte(m.unit(d, site)), 0o644); err != nil {
		return err
	}
	if err := m.systemctl(ctx, "daemon-reload"); err != nil {
		return err
	}
	unit := Unit(d.ID)
	if !d.Enabled {
		return m.systemctl(ctx, "disable", "--now", unit)
	}
	if err := m.systemctl(ctx, "enable", unit); err != nil {
		return err
	}
	// Restart, not start: a changed command must take effect.
	return m.systemctl(ctx, "restart", unit)
}

// uninstall stops a program and removes its files.
func (m *Manager) uninstall(ctx context.Context, id int64) error {
	unit := Unit(id)
	if fsutil.Exists(m.unitPath(id)) {
		m.systemctl(ctx, "disable", "--now", unit) // fine if it was not running
	}
	var errs []error
	for _, p := range []string{m.unitPath(id), m.scriptPath(id), m.envPath(id)} {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	errs = append(errs, m.systemctl(ctx, "daemon-reload"))
	m.systemctl(ctx, "reset-failed", unit) // forget a failed state; fine if there is none
	return errors.Join(errs...)
}

// ---- actions ----

// Create stores a new program and starts it (unless it is not enabled).
func (m *Manager) Create(ctx context.Context, req Request) (*store.Daemon, error) {
	if !m.supported() {
		return nil, hosting.ErrUnsupported
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	d, site, err := m.check(ctx, req)
	if err != nil {
		return nil, err
	}
	if err := m.store.SaveDaemon(ctx, d); err != nil {
		return nil, err
	}
	if err := m.install(ctx, d, site); err != nil {
		// Leave nothing half-made behind.
		m.uninstall(context.WithoutCancel(ctx), d.ID)
		m.store.DeleteDaemon(context.WithoutCancel(ctx), d.ID)
		return nil, err
	}
	return d, nil
}

// Update changes a program, rewrites its service and restarts it.
func (m *Manager) Update(ctx context.Context, id int64, req Request) (*store.Daemon, error) {
	if !m.supported() {
		return nil, hosting.ErrUnsupported
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	old, err := m.store.DaemonByID(ctx, id)
	if err != nil {
		return nil, err
	}
	d, site, err := m.check(ctx, req)
	if err != nil {
		return nil, err
	}
	d.ID, d.CreatedAt = old.ID, old.CreatedAt
	if err := m.store.SaveDaemon(ctx, d); err != nil {
		return nil, err
	}
	return d, m.install(ctx, d, site)
}

// Action starts, stops or restarts a program. Stopping also keeps it from
// starting at boot; starting or restarting turns that back on.
func (m *Manager) Action(ctx context.Context, id int64, action string) error {
	if action != "start" && action != "stop" && action != "restart" {
		return usererr.New("action_not_allowed", action)
	}
	if !m.supported() {
		return hosting.ErrUnsupported
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	d, err := m.store.DaemonByID(ctx, id)
	if err != nil {
		return err
	}
	d.Enabled = action != "stop"
	if err := m.store.SaveDaemon(ctx, d); err != nil {
		return err
	}
	if action == "stop" {
		return m.systemctl(ctx, "disable", "--now", Unit(id))
	}
	site, err := m.store.SiteByID(ctx, d.SiteID)
	if errors.Is(err, store.ErrNotFound) {
		return usererr.New("invalid_daemon_site")
	}
	if err != nil {
		return err
	}
	// Writing the files again keeps them in step with the database, for
	// example after a restore or a panel update.
	return m.install(ctx, d, site)
}

// Delete stops a program and removes it.
func (m *Manager) Delete(ctx context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.store.DaemonByID(ctx, id); err != nil {
		return err
	}
	if m.supported() {
		if err := m.uninstall(ctx, id); err != nil {
			return err
		}
	}
	return m.store.DeleteDaemon(ctx, id)
}

// RemoveForSite stops and removes all programs of a website. Call it before
// the website's Linux user is deleted: userdel refuses while the user still
// has running programs.
func (m *Manager) RemoveForSite(ctx context.Context, siteID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	list, err := m.store.DaemonsForSite(ctx, siteID)
	if err != nil {
		return err
	}
	var errs []error
	for _, d := range list {
		if m.supported() {
			if err := m.uninstall(ctx, d.ID); err != nil {
				errs = append(errs, fmt.Errorf("remove background program %q: %w", d.Name, err))
			}
		}
		if err := m.store.DeleteDaemon(ctx, d.ID); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Logs returns the last lines a program wrote.
func (m *Manager) Logs(ctx context.Context, id int64) (string, error) {
	if _, err := m.store.DaemonByID(ctx, id); err != nil {
		return "", err
	}
	if !m.supported() {
		return "", hosting.ErrUnsupported
	}
	return m.run.Output(ctx, "journalctl", "-u", Unit(id), "-n", "200", "--no-pager", "-o", "short-iso")
}

// List returns all programs with their state.
func (m *Manager) List(ctx context.Context) ([]Program, error) {
	list, err := m.store.ListDaemons(ctx)
	if err != nil {
		return nil, err
	}
	sites, err := m.store.ListSites(ctx)
	if err != nil {
		return nil, err
	}
	byID := map[int64]*store.Site{}
	for _, s := range sites {
		byID[s.ID] = s
	}
	units := make([]string, 0, len(list))
	for _, d := range list {
		units = append(units, Unit(d.ID))
	}
	states := map[string]Status{}
	if len(units) > 0 && m.supported() {
		states = m.statuses(ctx, units)
	}
	out := make([]Program, 0, len(list))
	for _, d := range list {
		p := Program{
			ID: d.ID, Name: d.Name, SiteID: d.SiteID, Command: d.Command, Dir: d.Dir,
			Env: d.Env, Enabled: d.Enabled, CreatedAt: d.CreatedAt, Unit: Unit(d.ID),
			Status: Status{Active: "unknown"},
		}
		if s, ok := byID[d.SiteID]; ok {
			p.Site, p.User = s.Domain, s.SysUser
		}
		if st, ok := states[p.Unit]; ok {
			p.Status = st
		}
		out = append(out, p)
	}
	return out, nil
}

// statuses reads the state of several units with one systemctl call.
func (m *Manager) statuses(ctx context.Context, units []string) map[string]Status {
	args := append([]string{"show", "--no-pager",
		"--property=Id,ActiveState,SubState,NRestarts,MainPID,MemoryCurrent,ActiveEnterTimestamp"}, units...)
	out, err := m.run.Output(ctx, "systemctl", args...)
	if err != nil {
		return map[string]Status{}
	}
	return parseShow(out)
}

// parseShow reads "systemctl show" output: one block of Key=value lines per
// unit, separated by empty lines.
func parseShow(text string) map[string]Status {
	out := map[string]Status{}
	for _, block := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n") {
		props := map[string]string{}
		for _, line := range strings.Split(block, "\n") {
			if k, v, ok := strings.Cut(line, "="); ok {
				props[k] = strings.TrimSpace(v)
			}
		}
		id := props["Id"]
		if id == "" {
			continue
		}
		s := Status{Active: props["ActiveState"], Sub: props["SubState"]}
		s.Restarts, _ = strconv.Atoi(props["NRestarts"])
		s.PID, _ = strconv.Atoi(props["MainPID"])
		s.Memory, _ = strconv.ParseInt(props["MemoryCurrent"], 10, 64) // "[not set]" when stopped
		if t, err := time.Parse("Mon 2006-01-02 15:04:05 MST", props["ActiveEnterTimestamp"]); err == nil && s.Active == "active" {
			s.Since = &t
		}
		out[id] = s
	}
	return out
}
