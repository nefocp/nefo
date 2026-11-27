package software

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
	"github.com/nefocp/nefo/internal/version"
)

// Per-version PHP settings: optional extensions (distribution or
// ppa:ondrej/php packages) and a few php.ini values in a panel-managed file.
// Per-site limits (memory, upload size, time) stay in each site's pool.

// OptionalExtensions can be installed and removed from the panel; the base
// set for each version (phpExtensionsFor) is always installed.
var OptionalExtensions = []string{"apcu", "gmp", "igbinary", "imagick", "imap", "ldap", "memcached", "mongodb",
	"msgpack", "pgsql", "redis", "soap", "sqlite3", "ssh2", "tidy", "xdebug", "xsl", "yaml"}

// PHPExtension is one optional extension of a PHP version.
type PHPExtension struct {
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	Available bool   `json:"available"` // a package exists for this version
}

// PHPConfig is a PHP version's extensions and php.ini values.
type PHPConfig struct {
	Version    string            `json:"version"`
	Extensions []PHPExtension    `json:"extensions"`
	Ini        map[string]string `json:"ini"`
	Defaults   map[string]string `json:"defaults"`
	// EOL marks a version without security updates (PHPEndOfLife), Legacy
	// one of 5.6 to 7.3.
	EOL    bool `json:"eol,omitempty"`
	Legacy bool `json:"legacy,omitempty"`
}

type iniRule struct {
	def   string
	check func(string) bool
}

func intRange(lo, hi int) func(string) bool {
	return func(s string) bool {
		n, err := strconv.Atoi(s)
		return err == nil && n >= lo && n <= hi
	}
}

func oneOf(vals ...string) func(string) bool {
	return func(s string) bool { return slices.Contains(vals, s) }
}

var (
	tzRe   = regexp.MustCompile(`^[A-Za-z_]+(/[A-Za-z0-9_+-]+){0,2}$`)
	sizeRe = regexp.MustCompile(`^[0-9]{1,7}[KM]?$`)
)

// iniRules are the php.ini keys the panel may set, with their PHP defaults.
// Every key exists from PHP 5.6 on, the oldest version offered (see
// TestIniRulesOldPHP); a key that only newer versions know, such as
// opcache.jit (8.0), must not be written for older ones.
var iniRules = map[string]iniRule{
	"max_input_vars":                {"1000", intRange(100, 100000)},
	"max_input_time":                {"60", intRange(-1, 3600)},
	"date.timezone":                 {"UTC", func(s string) bool { return tzRe.MatchString(s) && fileExists("/usr/share/zoneinfo/"+s) }},
	"display_errors":                {"Off", oneOf("On", "Off")},
	"short_open_tag":                {"Off", oneOf("On", "Off")},
	"allow_url_fopen":               {"On", oneOf("On", "Off")},
	"expose_php":                    {"Off", oneOf("On", "Off")},
	"default_socket_timeout":        {"60", intRange(1, 600)},
	"session.gc_maxlifetime":        {"1440", intRange(60, 2592000)},
	"realpath_cache_size":           {"4096K", sizeRe.MatchString},
	"opcache.enable":                {"1", oneOf("0", "1")},
	"opcache.memory_consumption":    {"128", intRange(32, 2048)},
	"opcache.max_accelerated_files": {"10000", intRange(1000, 1000000)},
	"opcache.validate_timestamps":   {"1", oneOf("0", "1")},
	"opcache.revalidate_freq":       {"2", intRange(0, 3600)},
}

// fileExists is a variable so tests can run on any system.
var fileExists = func(p string) bool { _, err := os.Stat(p); return err == nil }

func panelIni(v string) string { return filepath.Join("/etc/php", v, "fpm", "conf.d", "99-panel.ini") }

func (m *Manager) checkPHP(v string) error {
	if validate.PHPVersion(v) != nil || !phpInstalled(v) {
		return usererr.New("php_not_installed", v)
	}
	return nil
}

// PHPConfig returns a PHP version's extensions and php.ini values.
func (m *Manager) PHPConfig(ctx context.Context, v string) (*PHPConfig, error) {
	if err := m.checkPHP(v); err != nil {
		return nil, err
	}
	cfg := &PHPConfig{Version: v, Ini: readIni(panelIni(v)), Defaults: map[string]string{},
		EOL: PHPEndOfLife(v, m.distroPHP(ctx), phpNow()), Legacy: IsLegacyPHP(v)}
	for k, r := range iniRules {
		cfg.Defaults[k] = r.def
	}
	installed := map[string]bool{}
	out, _ := m.run.Output(ctx, "dpkg-query", "-W", "-f=${Package} ${db:Status-Abbrev}\n", "php"+v+"-*")
	for _, l := range strings.Split(out, "\n") {
		f := strings.Fields(l)
		if len(f) >= 2 && strings.HasPrefix(f[1], "ii") {
			installed[f[0]] = true
		}
	}
	pkgs := make([]string, len(OptionalExtensions))
	for i, e := range OptionalExtensions {
		pkgs[i] = "php" + v + "-" + e
	}
	out, _ = m.run.Output(ctx, "apt-cache", append([]string{"policy"}, pkgs...)...)
	available := parseCandidates(out)
	for i, e := range OptionalExtensions {
		cfg.Extensions = append(cfg.Extensions, PHPExtension{Name: e, Installed: installed[pkgs[i]], Available: available[pkgs[i]]})
	}
	return cfg, nil
}

// parseCandidates reads "apt-cache policy" output: packages that have a
// candidate version.
func parseCandidates(out string) map[string]bool {
	res := map[string]bool{}
	pkg := ""
	for _, l := range strings.Split(out, "\n") {
		if !strings.HasPrefix(l, " ") && strings.HasSuffix(strings.TrimSpace(l), ":") {
			pkg = strings.TrimSuffix(strings.TrimSpace(l), ":")
			continue
		}
		if c, ok := strings.CutPrefix(strings.TrimSpace(l), "Candidate:"); ok && pkg != "" {
			res[pkg] = strings.TrimSpace(c) != "(none)"
		}
	}
	return res
}

func readIni(path string) map[string]string {
	res := map[string]string{}
	b, err := os.ReadFile(path)
	if err != nil {
		return res
	}
	for _, l := range strings.Split(string(b), "\n") {
		k, v, ok := strings.Cut(l, "=")
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if ok && !strings.HasPrefix(k, ";") {
			if _, known := iniRules[k]; known {
				res[k] = strings.Trim(v, `"`)
			}
		}
	}
	return res
}

func (m *Manager) checkExtension(v, ext string) error {
	if err := m.checkPHP(v); err != nil {
		return err
	}
	if !slices.Contains(OptionalExtensions, ext) {
		return usererr.New("unknown_php_extension", ext)
	}
	return nil
}

// InstallExtension installs an optional extension and reloads PHP-FPM.
func (m *Manager) InstallExtension(ctx context.Context, v, ext string, lg Logger) error {
	if err := m.checkExtension(v, ext); err != nil {
		return err
	}
	// After the move from the PPA at start-up, apt has no lists for the
	// new source until its next update.
	if suryListsMissing() {
		if err := run.Apt(ctx, m.run, lg, "update"); err != nil {
			return err
		}
	}
	lg.Logf("Installing php%s-%s", v, ext)
	if err := run.Apt(ctx, m.run, lg, "install", "-y", "--no-install-recommends", "php"+v+"-"+ext); err != nil {
		return err
	}
	return m.reloadFPM(ctx, v, lg)
}

// RemoveExtension removes an optional extension and reloads PHP-FPM.
func (m *Manager) RemoveExtension(ctx context.Context, v, ext string, lg Logger) error {
	if err := m.checkExtension(v, ext); err != nil {
		return err
	}
	lg.Logf("Removing php%s-%s", v, ext)
	if err := run.Apt(ctx, m.run, lg, "purge", "-y", "php"+v+"-"+ext); err != nil {
		return err
	}
	return m.reloadFPM(ctx, v, lg)
}

func (m *Manager) reloadFPM(ctx context.Context, v string, lg Logger) error {
	lg.Logf("Reloading PHP %s", v)
	_, err := m.run.Output(ctx, "systemctl", "reload", "php"+v+"-fpm")
	return err
}

// SetPHPIni writes the php.ini values of a version (empty values are left
// out, so PHP's default applies), checks them with php-fpm -t and reloads.
func (m *Manager) SetPHPIni(ctx context.Context, v string, values map[string]string) error {
	if err := m.checkPHP(v); err != nil {
		return err
	}
	keys := make([]string, 0, len(values))
	for k, val := range values {
		r, ok := iniRules[k]
		if !ok {
			return usererr.New("unknown_php_setting", k)
		}
		if val = strings.TrimSpace(val); val != "" && !r.check(val) {
			return usererr.New("invalid_php_setting", k+" = "+val)
		}
		values[k] = val
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	fmt.Fprintf(&b, "; Managed by %s: PHP > Settings. Do not edit: the panel rewrites this file.\n", version.AppName)
	for _, k := range keys {
		if values[k] != "" {
			fmt.Fprintf(&b, "%s = %s\n", k, values[k])
		}
	}
	path := panelIni(v)
	old, oldErr := os.ReadFile(path)
	if err := fsutil.WriteFileAtomic(path, []byte(b.String()), 0o644); err != nil {
		return err
	}
	restore := func() {
		if oldErr == nil {
			fsutil.WriteFileAtomic(path, old, 0o644)
		} else {
			os.Remove(path)
		}
	}
	if out, err := m.run.Output(ctx, "php-fpm"+v, "-t"); err != nil {
		restore()
		return usererr.New("php_config_invalid", run.Tail(out, 5))
	}
	if _, err := m.run.Output(ctx, "systemctl", "reload", "php"+v+"-fpm"); err != nil {
		restore()
		m.run.Output(context.WithoutCancel(ctx), "systemctl", "reload", "php"+v+"-fpm")
		return err
	}
	return nil
}
