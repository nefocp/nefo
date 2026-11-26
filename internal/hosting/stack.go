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
	"github.com/nefocp/nefo/internal/validate"
)

// PHPInfo describes one installed PHP-FPM version.
type PHPInfo struct {
	Version string `json:"version"`
	Running bool   `json:"running"`
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
	for _, v := range m.phpVersions() {
		st.PHP = append(st.PHP, PHPInfo{Version: v, Running: m.unitActive(ctx, "php"+v+"-fpm")})
	}
	st.DefaultPHP = m.distroPHP(ctx)
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

// distroPHP asks apt which PHP version the php-fpm package stands for, such
// as 8.3 on Ubuntu 24.04. The answer is cached.
func (m *Manager) distroPHP(ctx context.Context) string {
	m.defaultPHPMu.Lock()
	defer m.defaultPHPMu.Unlock()
	if m.defaultPHP != "" {
		return m.defaultPHP
	}
	out, err := m.run.Output(ctx, "apt-cache", "depends", "php-fpm")
	if err != nil {
		return ""
	}
	if mm := phpDependsRe.FindStringSubmatch(out); mm != nil && validate.PHPVersion(mm[1]) == nil {
		m.defaultPHP = mm[1]
	}
	return m.defaultPHP
}

// phpPackages are the extensions WordPress and most PHP applications need.
func phpPackages(v string) []string {
	exts := []string{"fpm", "cli", "mysql", "curl", "gd", "mbstring", "xml", "zip", "intl", "bcmath", "opcache"}
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
