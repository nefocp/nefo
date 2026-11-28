package hosting

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/software"
	"github.com/nefocp/nefo/internal/validate"
)

// PHPInfo describes one installed PHP-FPM version.
type PHPInfo struct {
	Version string `json:"version"`
	Running bool   `json:"running"`
	// EOL: it gets no security updates on this server (software.PHPEndOfLife).
	EOL bool `json:"eol,omitempty"`
}

// Status is the state of the web server.
type Status struct {
	Supported bool `json:"supported"`
	// Ready means the web server is installed and configured by the panel.
	Ready bool `json:"ready"`
	// Server is the web server the panel manages: "nginx", "apache",
	// "openlitespeed" or "" before one is installed.
	Server string     `json:"server"`
	Nginx  ServerInfo `json:"nginx"`
	Apache ServerInfo `json:"apache"`
	OLS    ServerInfo `json:"openlitespeed"`
	PHP    []PHPInfo  `json:"php"`
	// DefaultPHP is the PHP version the distribution offers, which Install installs.
	DefaultPHP string `json:"default_php"`
}

var (
	nginxVersionRe  = regexp.MustCompile(`nginx/([0-9]+\.[0-9]+\.[0-9]+)`)
	apacheVersionRe = regexp.MustCompile(`Apache/([0-9]+\.[0-9]+\.[0-9]+)`)
	olsVersionRe    = regexp.MustCompile(`LiteSpeed/([0-9]+\.[0-9]+(?:\.[0-9]+)?) Open`)
	phpDependsRe    = regexp.MustCompile(`Depends: php([0-9]+\.[0-9]+)-fpm`)
)

// Status reports what is installed. It does not change anything.
func (m *Manager) Status(ctx context.Context) Status {
	st := Status{Supported: m.Supported(), PHP: []PHPInfo{}}
	if !st.Supported {
		return st
	}
	st.Nginx = m.server(ServerNginx).info(ctx)
	st.Apache = m.server(ServerApache).info(ctx)
	st.OLS = m.server(ServerOLS).info(ctx)
	st.Server = m.serverKind()
	switch st.Server {
	case ServerNginx:
		st.Ready = st.Nginx.Installed && fileExists(m.web().hookFile())
	case ServerApache:
		st.Ready = st.Apache.Installed && fileExists(m.web().hookFile())
	case ServerOLS:
		st.Ready = st.OLS.Installed && fileExists(m.web().hookFile())
	}
	st.DefaultPHP = m.distroPHP(ctx)
	for _, v := range m.phpVersions() {
		st.PHP = append(st.PHP, PHPInfo{Version: v, Running: m.unitActive(ctx, "php"+v+"-fpm"),
			EOL: software.PHPEndOfLife(v, st.DefaultPHP, m.now())})
	}
	return st
}

// phpVersions returns the installed PHP-FPM versions, oldest first.
func (m *Manager) phpVersions() []string {
	matches, _ := filepath.Glob(filepath.Join(m.paths.PHP, "*", "fpm", "php-fpm.conf"))
	var versions []string
	for _, p := range matches {
		v := filepath.Base(filepath.Dir(filepath.Dir(p)))
		if validate.PHPVersion(v) == nil {
			versions = append(versions, v)
		}
	}
	sort.Slice(versions, func(i, j int) bool { return !versionAtLeast(versions[i], versions[j]) })
	return versions
}

func (m *Manager) phpInstalled(version string) bool {
	for _, v := range m.phpVersions() {
		if v == version {
			return true
		}
	}
	return false
}

// Warm runs the slow, cached lookups once in the background at start, so the
// first page that shows the web server status does not wait for them (apt
// takes seconds on a small server).
func (m *Manager) Warm(ctx context.Context) {
	if m.Supported() {
		m.distroPHP(ctx)
	}
}

// distroPHP asks apt which PHP version the distribution's php-fpm package
// stands for, such as 8.3 on Ubuntu 24.04. The answer is cached.
func (m *Manager) distroPHP(ctx context.Context) string {
	m.defaultPHPMu.Lock()
	defer m.defaultPHPMu.Unlock()
	if m.defaultPHP != "" {
		return m.defaultPHP
	}
	out, err := m.run.Output(ctx, "apt-cache", "show", "php-fpm")
	madison, _ := m.run.Output(ctx, "apt-cache", "madison", "php-fpm")
	if v := distroPHPFrom(out, phpRepoVersions(madison)); err == nil && v != "" {
		m.defaultPHP = v
		return m.defaultPHP
	}
	// A fresh server whose package lists apt has not downloaded yet (many
	// cloud images): the version the release ships, not cached, so apt's
	// own answer replaces it once it has one.
	return distroPHPByRelease[osCodename()]
}

// distroPHPFrom reads "apt-cache show php-fpm": one record per version apt
// knows, each with its Version and Depends (php8.3-fpm). The PHP
// repository (packages.sury.org, and the ondrej PPA built from it) has a
// php-fpm of its own for its newest PHP, and apt's candidate is that one
// once the repository is added (8.6 on Ubuntu 24.04, whose own is 8.3).
// Its records are skipped: the versions "apt-cache madison" lists from it
// (repo), and any with "sury" in it ("2:8.6+106~exp1+debsuryorg24.04.1+1",
// "…+deb.sury.org+1"), so the distribution's ("2:8.3+93ubuntu2") counts.
func distroPHPFrom(out string, repo map[string]bool) string {
	for _, rec := range strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n\n") {
		version, php := "", ""
		for _, line := range strings.Split(rec, "\n") {
			if v, ok := strings.CutPrefix(line, "Version:"); ok {
				version = strings.TrimSpace(v)
			}
			if mm := phpDependsRe.FindStringSubmatch(line); mm != nil && php == "" {
				php = mm[1]
			}
		}
		if php != "" && !repo[version] && !strings.Contains(strings.ToLower(version), "sury") && validate.PHPVersion(php) == nil {
			return php
		}
	}
	return ""
}

// phpRepoVersions reads "apt-cache madison php-fpm" ("php-fpm | 2:8.6+106…
// | https://packages.sury.org/php noble/main amd64 Packages") for the
// versions that come from the PHP repository.
func phpRepoVersions(madison string) map[string]bool {
	out := map[string]bool{}
	for _, l := range strings.Split(madison, "\n") {
		f := strings.Split(l, "|")
		if len(f) < 3 {
			continue
		}
		src := strings.TrimSpace(f[2])
		if strings.Contains(src, "packages.sury.org") || strings.Contains(src, "/ondrej/php") || strings.Contains(src, "ondrej_php") {
			out[strings.TrimSpace(f[1])] = true
		}
	}
	return out
}

// distroPHPByRelease is the PHP version of each supported release's
// php-fpm package.
var distroPHPByRelease = map[string]string{
	"jammy": "8.1", "noble": "8.3", "resolute": "8.5", // Ubuntu 22.04, 24.04, 26.04
	"bookworm": "8.2", "trixie": "8.4", // Debian 12, 13
}

// phpPackages are the extensions WordPress and most PHP applications need.
// From PHP 8.5 OPcache is part of PHP itself and has no package of its own
// (Ubuntu 26.04, packages.sury.org): asking apt for it fails the install.
func phpPackages(v string) []string {
	exts := []string{"fpm", "cli", "mysql", "curl", "gd", "mbstring", "xml", "zip", "intl", "bcmath"}
	if !versionAtLeast(v, "8.5") {
		exts = append(exts, "opcache")
	}
	pkgs := make([]string, len(exts))
	for i, e := range exts {
		pkgs[i] = "php" + v + "-" + e
	}
	return pkgs
}

func (m *Manager) apt(ctx context.Context, w io.Writer, args ...string) error {
	return run.Apt(ctx, m.run, w, args...)
}

// Logger receives progress messages for the job log.
type Logger interface {
	io.Writer
	Logf(format string, args ...any)
}

func (m *Manager) ufwActive(ctx context.Context) bool {
	out, err := m.run.Output(ctx, "ufw", "status")
	return err == nil && strings.Contains(out, "Status: active")
}

func ipv6Enabled() bool { return fileExists("/proc/net/if_inet6") }

// baseFiles renders the managed web server's shared files.
func (m *Manager) baseFiles(ctx context.Context) (map[string][]byte, error) {
	return m.web().baseFiles(ctx)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
