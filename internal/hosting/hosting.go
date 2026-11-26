// Package hosting sets up the web server (Nginx, Apache or OpenLiteSpeed,
// with PHP-FPM) and manages websites and their Let's Encrypt certificates on
// Debian and Ubuntu.
//
// Each site gets its own Linux user, which owns the files and runs the
// site's PHP, so one hacked site cannot read or change the others.
package hosting

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// Paths are the places on disk the manager works with. Tests point them at a
// temporary directory.
type Paths struct {
	WWW         string // site folders: <WWW>/<domain>
	Nginx       string // Nginx configuration
	Apache      string // Apache configuration
	OLS         string // OpenLiteSpeed's home: bin, conf, modules
	PHP         string // PHP configuration: <PHP>/<version>/fpm
	NginxLogs   string
	ApacheLogs  string
	OLSLogs     string
	PHPRun      string // PHP-FPM sockets
	ACMEWebroot string // served at /.well-known/acme-challenge/ on every site
	Certs       string // <Certs>/<domain>/{fullchain,privkey}.pem
	ACME        string // ACME account key
	Quarantine  string // files the malware scanner moved out of sites
}

// DefaultPaths returns the Debian and Ubuntu locations. dataDir is the
// panel's data directory.
func DefaultPaths(dataDir string) Paths {
	return Paths{
		WWW:        "/var/www",
		Nginx:      "/etc/nginx",
		Apache:     "/etc/apache2",
		OLS:        "/usr/local/lsws",
		PHP:        "/etc/php",
		NginxLogs:  "/var/log/nginx",
		ApacheLogs: "/var/log/apache2",
		OLSLogs:    "/var/log/openlitespeed",
		PHPRun:     "/run/php",
		// Outside dataDir, which only root may read: Nginx workers run as
		// www-data and must read the challenge files.
		ACMEWebroot: "/var/lib/panel-acme",
		Certs:       filepath.Join(dataDir, "certs"),
		ACME:        filepath.Join(dataDir, "acme"),
		Quarantine:  "/var/lib/panel-quarantine",
	}
}

func (p Paths) siteRoot(domain string) string {
	return filepath.Join(p.WWW, domain)
}
func (p Paths) poolFile(phpVersion, user string) string {
	return filepath.Join(p.PHP, phpVersion, "fpm", "pool.d", "panel-"+user+".conf")
}
func (p Paths) phpSocket(user string) string {
	return filepath.Join(p.PHPRun, "panel-"+user+".sock")
}
func (p Paths) certDir(domain string) string { return filepath.Join(p.Certs, domain) }
func (p Paths) challengeDir() string {
	return filepath.Join(p.ACMEWebroot, ".well-known", "acme-challenge")
}

// Errors the API turns into specific messages.
var (
	ErrBusy        = errors.New("another change to the web server is in progress")
	ErrUnsupported = errors.New("website management needs Debian or Ubuntu with systemd")
	ErrNotReady    = errors.New("the web server is not installed yet")
)

// inputErr reports a problem with the user's request (see package usererr).
var inputErr = usererr.New

// Manager changes the web server configuration. All changes are serialised.
type Manager struct {
	paths         Paths
	run           run.Runner
	store         *store.Store
	log           *slog.Logger
	acmeDirectory string
	now           func() time.Time

	ops sync.Mutex // held during every change to the web server, PHP or sites

	// The web servers the panel knows; serverKind says which one this
	// server runs (see webserver.go).
	nginx, apache, ols webServer
	kindMu             sync.Mutex
	kind               string
	kindLoaded         bool

	// htaccess remembers what the OpenLiteSpeed sites' .htaccess files
	// looked like when their configuration was last written (see
	// ols_htaccess.go).
	htaccessMu sync.Mutex
	htaccess   map[int64]htaccessState
	// htaccessBusy is set while the watcher holds ops.
	htaccessBusy atomic.Bool

	probe *http.Client

	acmeMu         sync.Mutex
	acmeRegistered bool

	defaultPHPMu sync.Mutex
	defaultPHP   string

	reserved []int // ports app sites may not use
}

// New returns a Manager. acmeDirectory is the ACME server for certificates.
func New(paths Paths, r run.Runner, st *store.Store, log *slog.Logger, acmeDirectory string) *Manager {
	m := &Manager{
		paths:         paths,
		run:           r,
		store:         st,
		log:           log,
		acmeDirectory: acmeDirectory,
		now:           time.Now,
		probe: &http.Client{
			Timeout: 10 * time.Second,
			// Report what the domain itself answers; do not follow it elsewhere.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
	m.nginx, m.apache, m.ols = nginxServer{m}, apacheServer{m}, olsServer{m}
	return m
}

// Supported reports whether this machine can host sites: Linux with apt and systemd.
func (m *Manager) Supported() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	for _, p := range []string{"apt-get", "systemctl", "useradd"} {
		if _, err := exec.LookPath(p); err != nil {
			return false
		}
	}
	return true
}

// tryLock takes the operations lock, or returns ErrBusy at once if a job
// such as a package installation is holding it. The .htaccess watcher holds
// it for a second at most (see ols_htaccess.go): then tryLock waits.
func (m *Manager) tryLock() error {
	if m.ops.TryLock() {
		return nil
	}
	for deadline := time.Now().Add(15 * time.Second); m.htaccessBusy.Load() && time.Now().Before(deadline); {
		time.Sleep(20 * time.Millisecond)
		if m.ops.TryLock() {
			return nil
		}
	}
	if m.ops.TryLock() {
		return nil
	}
	return ErrBusy
}

// ---- small helpers ----

// writeFile writes a file atomically with an explicit mode. The panel runs
// with umask 077, so modes must always be set explicitly.
func writeFile(path string, data []byte, mode os.FileMode) error {
	return fsutil.WriteFileAtomic(path, data, mode)
}

// mkdir creates dir with exactly mode (not affected by the umask).
func mkdir(dir string, mode os.FileMode) error {
	if err := os.MkdirAll(dir, mode); err != nil {
		return err
	}
	return os.Chmod(dir, mode)
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// systemctl runs "systemctl <action> <unit>".
func (m *Manager) systemctl(ctx context.Context, action, unit string) error {
	_, err := m.run.Output(ctx, "systemctl", action, unit)
	return err
}

func (m *Manager) unitActive(ctx context.Context, unit string) bool {
	out, err := m.run.Output(ctx, "systemctl", "is-active", unit)
	return err == nil && len(out) >= 6 && out[:6] == "active"
}

// reloadPHP checks the PHP-FPM configuration and reloads it.
func (m *Manager) reloadPHP(ctx context.Context, version string) error {
	if out, err := m.run.Output(ctx, "php-fpm"+version, "-t"); err != nil {
		return fmt.Errorf("PHP-FPM %s configuration test failed:\n%s", version, run.Tail(out, 10))
	}
	return m.systemctl(ctx, "reload", "php"+version+"-fpm")
}
