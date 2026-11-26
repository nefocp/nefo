package systools

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/usererr"
)

// ---- time and host name ----

// TimeInfo is the server's clock and name.
type TimeInfo struct {
	Timezone  string `json:"timezone"`
	Time      string `json:"time"` // now, RFC 3339 in the server's time zone
	NTP       bool   `json:"ntp"`  // time synchronisation is on
	Synced    bool   `json:"synced"`
	CanNTP    bool   `json:"can_ntp"` // an NTP service is installed
	LocalRTC  bool   `json:"local_rtc"`
	Hostname  string `json:"hostname"`
	Available bool   `json:"available"` // timedatectl answered
}

// Time reads the clock settings from timedatectl.
func (m *Manager) Time(ctx context.Context) TimeInfo {
	ti := TimeInfo{}
	ti.Hostname, _ = m.hostname()
	out, err := m.run.Output(ctx, "timedatectl", "show")
	kv := keyValues(out)
	if err == nil {
		ti.Available = true
		ti.Timezone = kv["Timezone"]
		ti.NTP = kv["NTP"] == "yes"
		ti.Synced = kv["NTPSynchronized"] == "yes"
		ti.CanNTP = kv["CanNTP"] == "yes"
		ti.LocalRTC = kv["LocalRTC"] == "yes"
	}
	now := m.now()
	if loc, err := time.LoadLocation(ti.Timezone); err == nil && ti.Timezone != "" {
		now = now.In(loc)
	}
	ti.Time = now.Format(time.RFC3339)
	return ti
}

// Timezones lists the time zones timedatectl knows (read once).
func (m *Manager) Timezones(ctx context.Context) ([]string, error) {
	m.cacheMu.Lock()
	zones := m.zones
	m.cacheMu.Unlock()
	if zones != nil {
		return zones, nil
	}
	out, err := m.run.Output(ctx, "timedatectl", "list-timezones")
	if err != nil {
		return nil, err
	}
	zones = []string{}
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); zoneRe.MatchString(l) {
			zones = append(zones, l)
		}
	}
	m.cacheMu.Lock()
	m.zones = zones
	m.cacheMu.Unlock()
	return zones, nil
}

var zoneRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_+-]*(/[A-Za-z0-9][A-Za-z0-9_+-]*){0,2}$`)

// SetTimezone sets the time zone, one that timedatectl lists.
func (m *Manager) SetTimezone(ctx context.Context, zone string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !zoneRe.MatchString(zone) {
		return usererr.New("timezone_invalid", zone)
	}
	zones, err := m.Timezones(ctx)
	if err != nil {
		return err
	}
	if !slices.Contains(zones, zone) {
		return usererr.New("timezone_invalid", zone)
	}
	if _, err := m.run.Output(ctx, "timedatectl", "set-timezone", zone); err != nil {
		return err
	}
	// cron reads the time zone when it starts: the panel's cron jobs follow
	// the new one only after a restart (running jobs are not stopped).
	m.run.Output(ctx, "systemctl", "try-restart", "cron.service")
	return nil
}

// SetNTP turns time synchronisation on or off.
func (m *Manager) SetNTP(ctx context.Context, on bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if on && !m.Time(ctx).CanNTP {
		return usererr.New("ntp_unavailable")
	}
	v := "false"
	if on {
		v = "true"
	}
	_, err := m.run.Output(ctx, "timedatectl", "set-ntp", v)
	return err
}

var labelRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// ValidHostname checks a host name by RFC 1123: labels of letters, digits
// and hyphens, not starting or ending with a hyphen, and at most 64
// characters in all (the kernel's limit). It returns the name in lower case.
func ValidHostname(name string) (string, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" || len(name) > 64 || net.ParseIP(name) != nil || name == "localhost" {
		return "", usererr.New("hostname_invalid", name)
	}
	for _, l := range strings.Split(name, ".") {
		if !labelRe.MatchString(l) {
			return "", usererr.New("hostname_invalid", name)
		}
	}
	return name, nil
}

// SetHostname sets the host name with hostnamectl, points the 127.0.1.1
// line of /etc/hosts at it and, on a cloud server, tells cloud-init to keep
// it at the next boot.
func (m *Manager) SetHostname(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	name, err := ValidHostname(name)
	if err != nil {
		return err
	}
	if out, err := m.run.Output(ctx, "hostnamectl", "set-hostname", name); err != nil {
		return usererr.New("hostname_failed", run.Tail(out, 1))
	}
	b, err := os.ReadFile(m.p.Hosts)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if text := hostsWithName(string(b), name); text != string(b) {
		if err := writeFile(m.p.Hosts, []byte(text), 0o644); err != nil {
			return err
		}
	}
	if fi, err := os.Stat(m.p.CloudCfgDir); err == nil && fi.IsDir() {
		cfg := "# Written by the panel: keep the host name set in System → Time & name.\npreserve_hostname: true\n"
		if err := writeFile(filepath.Join(m.p.CloudCfgDir, "99-panel-hostname.cfg"), []byte(cfg), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// hostsWithName points the 127.0.1.1 line of a hosts file at name (and
// its first label, for a name with dots), as Debian does. Without such a
// line, one is added after the 127.0.0.1 lines.
func hostsWithName(text, name string) string {
	entry := "127.0.1.1\t" + name
	if short, _, ok := strings.Cut(name, "."); ok {
		entry += " " + short
	}
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if text == "" {
		lines = nil
	}
	lastLoop := -1
	for i, l := range lines {
		f := strings.Fields(l)
		if len(f) == 0 {
			continue
		}
		switch f[0] {
		case "127.0.1.1":
			lines[i] = entry
			return strings.Join(lines, "\n") + "\n"
		case "127.0.0.1":
			lastLoop = i
		}
	}
	lines = slices.Insert(lines, lastLoop+1, entry)
	return strings.Join(lines, "\n") + "\n"
}
