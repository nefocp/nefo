// Package services shows and controls the system services the panel
// manages: start, stop, restart and reload, from a fixed list.
package services

import (
	"context"
	"errors"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/usererr"
)

// Service is the state of one systemd unit.
type Service struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Active      string     `json:"active"` // active, inactive, failed…
	Sub         string     `json:"sub"`    // running, dead, exited…
	Enabled     bool       `json:"enabled"`
	Since       *time.Time `json:"since,omitempty"`
	Memory      int64      `json:"memory"`
	Actions     []string   `json:"actions"`
}

// candidates are the units the panel shows when they are installed.
var candidates = []string{"nefo", "nginx", "apache2", "lshttpd", "mariadb", "redis-server", "fail2ban", "ssh", "cron", "ufw"}

var phpFPMRe = regexp.MustCompile(`^php[0-9]+\.[0-9]+-fpm$`)

// Manager reads and changes services.
type Manager struct {
	run    run.Runner
	phpDir string
}

// New returns a Manager.
func New(r run.Runner) *Manager { return &Manager{run: r, phpDir: "/etc/php"} }

// names returns the candidate units plus every installed PHP-FPM version.
func (m *Manager) names() []string {
	names := slices.Clone(candidates)
	matches, _ := filepath.Glob(filepath.Join(m.phpDir, "*", "fpm"))
	for _, p := range matches {
		n := "php" + filepath.Base(filepath.Dir(p)) + "-fpm"
		if phpFPMRe.MatchString(n) {
			names = append(names, n)
		}
	}
	return names
}

// actions lists what may be done to a unit. SSH can never be stopped from
// the panel (that would lock the administrator out), and the panel can only
// be restarted.
func actions(name string) []string {
	switch {
	case name == "nefo":
		return []string{"restart"}
	case name == "ssh":
		return []string{"restart", "reload"}
	case name == "nginx" || name == "apache2" || name == "lshttpd" || phpFPMRe.MatchString(name):
		return []string{"start", "stop", "restart", "reload"}
	}
	return []string{"start", "stop", "restart"}
}

// List returns the installed services.
func (m *Manager) List(ctx context.Context) []Service {
	out := []Service{}
	for _, name := range m.names() {
		if s, ok := m.show(ctx, name); ok {
			out = append(out, s)
		}
	}
	return out
}

func (m *Manager) show(ctx context.Context, name string) (Service, bool) {
	text, err := m.run.Output(ctx, "systemctl", "show", name+".service", "--no-pager",
		"--property=LoadState,ActiveState,SubState,UnitFileState,ActiveEnterTimestamp,MemoryCurrent,Description")
	if err != nil {
		return Service{}, false
	}
	props := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			props[k] = strings.TrimSpace(v)
		}
	}
	if props["LoadState"] != "loaded" {
		return Service{}, false
	}
	s := Service{
		Name:        name,
		Description: props["Description"],
		Active:      props["ActiveState"],
		Sub:         props["SubState"],
		Enabled:     props["UnitFileState"] == "enabled" || props["UnitFileState"] == "static",
		Actions:     actions(name),
	}
	if n, err := strconv.ParseInt(props["MemoryCurrent"], 10, 64); err == nil {
		s.Memory = n
	}
	if t, err := time.Parse("Mon 2006-01-02 15:04:05 MST", props["ActiveEnterTimestamp"]); err == nil && s.Active == "active" {
		s.Since = &t
	}
	return s, true
}

func (m *Manager) allowed(name, action string) error {
	if !slices.Contains(m.names(), name) || !slices.Contains(actions(name), action) {
		return usererr.New("action_not_allowed", name+" "+action)
	}
	return nil
}

// Do runs an action on a service. Restarting the panel itself is queued with
// --no-block, so systemd restarts it after this request has been answered.
func (m *Manager) Do(ctx context.Context, name, action string) error {
	if err := m.allowed(name, action); err != nil {
		return err
	}
	if name == "nginx" && (action == "reload" || action == "restart") {
		if out, err := m.run.Output(ctx, "nginx", "-t"); err != nil {
			return usererr.New("config_test_failed", run.Tail(out, 10))
		}
	}
	if name == "apache2" && (action == "reload" || action == "restart") {
		if out, err := m.run.Output(ctx, "apache2ctl", "-t"); err != nil {
			return usererr.New("config_test_failed", run.Tail(out, 10))
		}
	}
	if name == "lshttpd" && (action == "reload" || action == "restart") {
		if out, err := olsConfigTest(ctx, m.run); err != nil {
			return usererr.New("config_test_failed", run.Tail(out, 10))
		}
	}
	args := []string{action, name + ".service"}
	if name == "nefo" {
		args = []string{"--no-block", action, name + ".service"}
	}
	_, err := m.run.Output(ctx, "systemctl", args...)
	return err
}

// olsConfigTest runs OpenLiteSpeed's configuration test. It exits with 1
// for warnings too: only errors count, except those about a website's
// folder that is missing or a .htaccess rewrite rule, which OpenLiteSpeed
// skips while running.
func olsConfigTest(ctx context.Context, r run.Runner) (string, error) {
	out, err := r.Output(ctx, "/usr/local/lsws/bin/openlitespeed", "-t")
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "[ERROR]") && !strings.Contains(l, "path is not accessible: /var/www/") &&
			!strings.Contains(l, "[REWRITE]") {
			return out, errors.New("configuration error")
		}
	}
	if err != nil && !strings.Contains(out, "[WARN]") {
		return out, err
	}
	return out, nil
}

// Logs returns the last lines of a service's journal.
func (m *Manager) Logs(ctx context.Context, name string) (string, error) {
	if !slices.Contains(m.names(), name) {
		return "", usererr.New("action_not_allowed", name)
	}
	return m.run.Output(ctx, "journalctl", "-u", name+".service", "-n", "150", "--no-pager", "-o", "short-iso")
}
