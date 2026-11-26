package systools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/usererr"
)

// ---- operating system updates ----

// settingUpdates holds the last update check (JSON of UpdateCheck).
const settingUpdates = "os_updates"

// CheckEvery is how old the last update check may get before the hourly
// maintenance checks again: apt-get update downloads the package lists,
// which is too slow and too heavy to do on every page load.
const CheckEvery = 6 * time.Hour

// Package is one pending update.
type Package struct {
	Name     string `json:"name"`
	Current  string `json:"current"` // "" for a package the update installs new (a kernel)
	Version  string `json:"version"`
	Origin   string `json:"origin"` // the suite, such as "noble-security"
	Security bool   `json:"security"`
}

// UpdateCheck is the result of an update check, kept in the settings.
type UpdateCheck struct {
	CheckedAt time.Time `json:"checked_at"`
	Packages  []Package `json:"packages"`
	// Held are upgradable packages apt does not install now: kept back
	// (they would need other changes) or deferred by phased updates.
	Held []string `json:"held"`
}

// Reboot says whether the server needs a restart to finish an update.
type Reboot struct {
	Required bool     `json:"required"`
	Packages []string `json:"packages"` // from reboot-required.pkgs
	Kernel   string   `json:"kernel,omitempty"`
}

// Auto is the state of automatic security updates (unattended-upgrades).
type Auto struct {
	Installed bool `json:"installed"`
	Enabled   bool `json:"enabled"`
}

// Updates is what the Updates tab and the attention list show.
type Updates struct {
	CheckedAt *time.Time `json:"checked_at"` // nil: never checked
	Packages  []Package  `json:"packages"`
	Held      []string   `json:"held"`
	Security  int        `json:"security"`
	// Restarts names the services that installing the updates restarts
	// for a moment ("Nginx", "PHP", "MariaDB"…).
	Restarts []string `json:"restarts"`
	Reboot   Reboot   `json:"reboot"`
	Auto     Auto     `json:"auto"`
}

// Updates returns the last update check, without checking again, and the
// current restart and automatic update state.
func (m *Manager) Updates(ctx context.Context) Updates {
	u := Updates{Packages: []Package{}, Held: []string{}, Restarts: []string{}}
	if c := m.cached(ctx); c != nil {
		at := c.CheckedAt
		u.CheckedAt = &at
		u.Packages = append(u.Packages, c.Packages...)
		u.Held = append(u.Held, c.Held...)
	}
	for _, p := range u.Packages {
		if p.Security {
			u.Security++
		}
	}
	u.Restarts = restartsFor(u.Packages)
	u.Reboot = m.reboot()
	u.Auto = m.autoState(ctx)
	return u
}

// cached returns the last check, loading it from the settings once.
func (m *Manager) cached(ctx context.Context) *UpdateCheck {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()
	if m.cache == nil && m.settings != nil {
		if v, err := m.settings.Setting(ctx, settingUpdates); err == nil && v != "" {
			var c UpdateCheck
			if json.Unmarshal([]byte(v), &c) == nil {
				m.cache = &c
			}
		}
	}
	return m.cache
}

func (m *Manager) save(ctx context.Context, c *UpdateCheck) {
	m.cacheMu.Lock()
	m.cache = c
	m.cacheMu.Unlock()
	if m.settings == nil {
		return
	}
	if b, err := json.Marshal(c); err == nil {
		m.settings.SetSetting(ctx, settingUpdates, string(b))
	}
}

// CheckUpdates refreshes the package lists and lists the pending updates.
func (m *Manager) CheckUpdates(ctx context.Context, lg Logger) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.check(ctx, lg)
}

func (m *Manager) check(ctx context.Context, lg Logger) error {
	lg.Logf("Refreshing the package lists")
	if err := run.Apt(ctx, m.run, lg, "update"); err != nil {
		return err
	}
	c, err := m.simulate(ctx)
	if err != nil {
		return err
	}
	m.save(ctx, c)
	sec := 0
	for _, p := range c.Packages {
		if p.Security {
			sec++
		}
	}
	lg.Logf("%d updates available, %d of them security updates", len(c.Packages), sec)
	return nil
}

// CheckIfStale checks for updates when the last check is older than
// maxAge. It does nothing while another change of this manager runs (an
// update, a swap file…). It reports whether it checked.
func (m *Manager) CheckIfStale(ctx context.Context, maxAge time.Duration) (bool, error) {
	if c := m.cached(ctx); c != nil && m.now().Sub(c.CheckedAt) < maxAge {
		return false, nil
	}
	if !m.mu.TryLock() {
		return false, nil
	}
	defer m.mu.Unlock()
	// Ubuntu's apt-daily (or an administrator) refreshed the lists lately:
	// read them as they are rather than take apt's lock for apt-get update,
	// which would make an apt command typed at the same moment fail.
	if age, ok := m.listsAge(); ok && age < maxAge {
		c, err := m.simulate(ctx)
		if err != nil {
			return true, err
		}
		m.save(ctx, c)
		return true, nil
	}
	return true, m.check(ctx, discard{})
}

// listsAge returns how long ago apt-get update last fetched the package
// lists: the newest Release file in /var/lib/apt/lists.
func (m *Manager) listsAge() (time.Duration, bool) {
	entries, err := os.ReadDir(m.p.AptLists)
	if err != nil {
		return 0, false
	}
	var newest time.Time
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), "Release") {
			continue
		}
		if fi, err := e.Info(); err == nil && fi.ModTime().After(newest) {
			newest = fi.ModTime()
		}
	}
	if newest.IsZero() {
		return 0, false
	}
	return m.now().Sub(newest), true
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
func (discard) Logf(string, ...any)         {}

// simulate asks apt what an upgrade would install now.
func (m *Manager) simulate(ctx context.Context) (*UpdateCheck, error) {
	out, err := m.run.Output(ctx, "apt-get", "-s", "upgrade", "--with-new-pkgs")
	if err != nil {
		return nil, err
	}
	c := parseSimulation(out)
	c.CheckedAt = m.now().UTC()
	return c, nil
}

// "Inst libssl3 [3.0.2-0ubuntu1.14] (3.0.2-0ubuntu1.15 Ubuntu:22.04/jammy-updates, Ubuntu:22.04/jammy-security [amd64])"
var instRe = regexp.MustCompile(`^Inst (\S+) (?:\[([^\]]*)\] )?\((\S+)(?: (.*?))? \[[^\]]*\]\)`)

// pkgRe is a Debian package name, with an optional architecture.
var pkgRe = regexp.MustCompile(`^[a-z0-9][a-z0-9+.-]{0,99}(:[a-z0-9-]{1,20})?$`)

// parseSimulation reads the output of apt-get -s upgrade.
func parseSimulation(out string) *UpdateCheck {
	c := &UpdateCheck{Packages: []Package{}, Held: []string{}}
	held := false
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "  ") && held {
			for _, n := range strings.Fields(line) {
				if pkgRe.MatchString(n) && !slices.Contains(c.Held, n) {
					c.Held = append(c.Held, n)
				}
			}
			continue
		}
		held = strings.HasPrefix(line, "The following packages have been kept back") ||
			strings.HasPrefix(line, "The following upgrades have been deferred")
		mm := instRe.FindStringSubmatch(line)
		if mm == nil || !pkgRe.MatchString(mm[1]) {
			continue
		}
		p := Package{Name: mm[1], Current: mm[2], Version: mm[3]}
		for _, o := range strings.Split(mm[4], ",") {
			o = strings.TrimSpace(o)
			if i := strings.LastIndex(o, "/"); i >= 0 {
				o = o[i+1:]
			}
			if o == "" {
				continue
			}
			sec := strings.HasSuffix(strings.ToLower(o), "-security")
			if p.Origin == "" || (sec && !p.Security) {
				p.Origin = o
			}
			p.Security = p.Security || sec
		}
		c.Packages = append(c.Packages, p)
	}
	// A package that is installed is not also held.
	c.Held = slices.DeleteFunc(c.Held, func(n string) bool {
		return slices.ContainsFunc(c.Packages, func(p Package) bool { return p.Name == n })
	})
	return c
}

var phpPkgRe = regexp.MustCompile(`^(php\d+\.\d+|lsphp\d+)(-|$)`)

// restartsFor names the services whose packages are in the list: their
// package scripts restart them during the update.
func restartsFor(pkgs []Package) []string {
	has := map[string]bool{}
	for _, p := range pkgs {
		n, _, _ := strings.Cut(p.Name, ":")
		switch {
		case strings.HasPrefix(n, "nginx"):
			has["Nginx"] = true
		case n == "apache2" || strings.HasPrefix(n, "apache2-"):
			has["Apache"] = true
		case n == "openlitespeed":
			has["OpenLiteSpeed"] = true
		case phpPkgRe.MatchString(n):
			has["PHP"] = true
		case strings.HasPrefix(n, "mariadb-server") || strings.HasPrefix(n, "mysql-server"):
			has["MariaDB"] = true
		case n == "redis-server":
			has["Redis"] = true
		case n == "docker-ce" || n == "docker.io" || strings.HasPrefix(n, "containerd"):
			has["Docker"] = true
		}
	}
	out := []string{}
	for _, s := range []string{"Nginx", "Apache", "OpenLiteSpeed", "PHP", "MariaDB", "Redis", "Docker"} {
		if has[s] {
			out = append(out, s)
		}
	}
	return out
}

// Upgrade installs the pending updates, or only the security updates. It
// refreshes the package lists first, keeps changed configuration files
// (see run.Apt) and returns the number of packages it updated.
func (m *Manager) Upgrade(ctx context.Context, securityOnly bool, lg Logger) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	lg.Logf("Refreshing the package lists")
	if err := run.Apt(ctx, m.run, lg, "update"); err != nil {
		return 0, err
	}
	c, err := m.simulate(ctx)
	if err != nil {
		return 0, err
	}
	m.save(ctx, c)
	var names []string
	var chosen []Package
	for _, p := range c.Packages {
		if !securityOnly || (p.Security && p.Current != "") {
			chosen = append(chosen, p)
			names = append(names, p.Name)
		}
	}
	if len(chosen) == 0 {
		if securityOnly {
			lg.Logf("There are no security updates to install.")
		} else {
			lg.Logf("There are no updates to install.")
		}
		return 0, nil
	}
	if r := restartsFor(chosen); len(r) > 0 {
		lg.Logf("These updates restart %s for a moment.", strings.Join(r, ", "))
	}
	if securityOnly {
		lg.Logf("Installing %d security updates", len(names))
		err = run.Apt(ctx, m.run, lg, append([]string{"install", "--only-upgrade"}, names...)...)
	} else {
		lg.Logf("Installing %d updates", len(names))
		err = run.Apt(ctx, m.run, lg, "upgrade", "--with-new-pkgs")
	}
	// Whatever happened, the list shows what is left.
	if after, serr := m.simulate(ctx); serr == nil {
		m.save(ctx, after)
	}
	if err != nil {
		return 0, err
	}
	if m.reboot().Required {
		lg.Logf("The server needs a restart to finish the update.")
	}
	return len(names), nil
}

// ---- restart required ----

func (m *Manager) reboot() Reboot {
	r := Reboot{Packages: []string{}}
	if exists(m.p.RebootRequired) {
		r.Required = true
		for _, l := range strings.Split(readTrim(m.p.RebootPkgs), "\n") {
			if l = strings.TrimSpace(l); pkgRe.MatchString(l) && !slices.Contains(r.Packages, l) {
				r.Packages = append(r.Packages, l)
			}
		}
	}
	// Debian only writes reboot-required with some packages installed:
	// a newer kernel than the running one also means a restart.
	if k := newerKernel(m.p.Boot, readTrim(m.p.OSRelease)); k != "" {
		r.Required, r.Kernel = true, k
	}
	return r
}

var kernelRe = regexp.MustCompile(`^[0-9]+(\.[0-9]+)*(-[0-9]+)?`)

// newerKernel returns the newest kernel in boot of the same flavour
// ("-generic", "-amd64") as the running one, when it is newer.
func newerKernel(boot, running string) string {
	if running == "" {
		return ""
	}
	flavour := kernelRe.ReplaceAllString(running, "")
	entries, err := os.ReadDir(boot)
	if err != nil {
		return ""
	}
	best := running
	for _, e := range entries {
		v, ok := strings.CutPrefix(e.Name(), "vmlinuz-")
		if !ok || !kernelRe.MatchString(v) || kernelRe.ReplaceAllString(v, "") != flavour {
			continue
		}
		if compareVersions(v, best) > 0 {
			best = v
		}
	}
	if best == running {
		return ""
	}
	return best
}

// compareVersions compares two version strings piece by piece, numbers as
// numbers: "5.15.0-100" is newer than "5.15.0-91".
func compareVersions(a, b string) int {
	pa, pb := versionParts(a), versionParts(b)
	for i := 0; i < len(pa) && i < len(pb); i++ {
		x, y := pa[i], pb[i]
		nx, ex := strconv.Atoi(x)
		ny, ey := strconv.Atoi(y)
		switch {
		case ex == nil && ey == nil:
			if nx != ny {
				return cmpInt(nx, ny)
			}
		case x != y:
			return strings.Compare(x, y)
		}
	}
	return cmpInt(len(pa), len(pb))
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func versionParts(s string) []string {
	var parts []string
	start := 0
	for i := 1; i <= len(s); i++ {
		if i == len(s) || isDigit(s[i]) != isDigit(s[i-1]) {
			parts = append(parts, s[start:i])
			start = i
		}
	}
	return parts
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// ---- automatic security updates ----

// NeedsInstall reports whether turning on automatic updates must install
// unattended-upgrades first (a background job).
func (m *Manager) NeedsInstall() bool { return !exists(m.p.Unattended) }

func (m *Manager) autoState(ctx context.Context) Auto {
	a := Auto{Installed: exists(m.p.Unattended)}
	if !a.Installed {
		return a
	}
	// apt-config gives the value that counts across every file of apt.conf.d.
	out, err := m.run.Output(ctx, "apt-config", "shell",
		"UL", "APT::Periodic::Update-Package-Lists", "UU", "APT::Periodic::Unattended-Upgrade")
	if err != nil {
		return a
	}
	kv := keyValues(out)
	on := func(k string) bool {
		v := strings.Trim(kv[k], `'"`)
		return v != "" && v != "0"
	}
	a.Enabled = on("UU") && on("UL")
	return a
}

// SetAutoUpdates turns automatic security updates on or off. On installs
// unattended-upgrades when it is missing (see NeedsInstall), and sets
// APT::Periodic::Update-Package-Lists and APT::Periodic::Unattended-Upgrade
// in 20auto-upgrades; off sets Unattended-Upgrade to 0.
func (m *Manager) SetAutoUpdates(ctx context.Context, on bool, lg Logger) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if on && !exists(m.p.Unattended) {
		lg.Logf("Installing unattended-upgrades")
		if err := run.Apt(ctx, m.run, lg, "update"); err != nil {
			return err
		}
		if err := run.Apt(ctx, m.run, lg, "install", "--no-install-recommends", "unattended-upgrades"); err != nil {
			return err
		}
	}
	if !on && !exists(m.p.Unattended) {
		return nil // nothing installs updates
	}
	old := readOld(m.p.AutoUpgrades)
	vals := map[string]string{"Unattended-Upgrade": "0"}
	if on {
		vals = map[string]string{"Update-Package-Lists": "1", "Unattended-Upgrade": "1"}
	}
	lg.Logf("Writing %s", m.p.AutoUpgrades)
	if err := writeFile(m.p.AutoUpgrades, []byte(setPeriodic(string(old), vals)), 0o644); err != nil {
		return err
	}
	if on {
		// apt's timers run the daily update; they are normally on already.
		if out, err := m.run.Output(ctx, "systemctl", "enable", "--now", "apt-daily.timer", "apt-daily-upgrade.timer"); err != nil {
			lg.Logf("Could not turn on apt's timers: %s", run.Tail(out, 3))
		}
	}
	if m.autoState(ctx).Enabled != on {
		restore(m.p.AutoUpgrades, old)
		return usererr.New("auto_updates_overridden")
	}
	return nil
}

var periodicRe = regexp.MustCompile(`^\s*APT::Periodic::([A-Za-z-]+)\s+"[^"]*"\s*;\s*$`)

// setPeriodic sets APT::Periodic values in an apt.conf file, keeping its
// other lines.
func setPeriodic(text string, vals map[string]string) string {
	done := map[string]bool{}
	var lines []string
	if text != "" {
		lines = strings.Split(strings.TrimRight(text, "\n"), "\n")
	}
	for i, l := range lines {
		mm := periodicRe.FindStringSubmatch(l)
		if mm == nil {
			continue
		}
		if v, ok := vals[mm[1]]; ok {
			lines[i] = fmt.Sprintf("APT::Periodic::%s %q;", mm[1], v)
			done[mm[1]] = true
		}
	}
	for _, k := range []string{"Update-Package-Lists", "Unattended-Upgrade"} {
		if v, ok := vals[k]; ok && !done[k] {
			lines = append(lines, fmt.Sprintf("APT::Periodic::%s %q;", k, v))
		}
	}
	return strings.Join(lines, "\n") + "\n"
}
