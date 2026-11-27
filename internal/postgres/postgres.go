// Package postgres installs PostgreSQL from the distribution's packages and
// manages databases, each owned by a login role of the same name.
//
// The panel runs as root and talks to the server as the postgres OS user
// (runuser, then psql over the local socket, where pg_hba.conf allows peer
// authentication for that user only). SQL goes on stdin. Passwords never
// reach the server in clear: the panel sends a SCRAM-SHA-256 verifier (see
// scram.go), so not even the server's log can show one. Names pass a strict
// allow-list (see ident) and are always quoted; literals are escaped (see
// literal).
//
// Imports, exports and backups connect as the database's own role, never
// as postgres, with the password in a temporary file only root can read.
package postgres

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/meminfo"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// Where the distribution keeps PostgreSQL. Debian and Ubuntu run each major
// version's "main" cluster from /etc/postgresql/<major>/main.
const (
	defaultEtcDir    = "/etc/postgresql"
	defaultLibDir    = "/usr/lib/postgresql"
	defaultDataDir   = "/var/lib/postgresql"
	defaultSocketDir = "/var/run/postgresql"
	confName         = "90-nefo.conf"
	defaultPort      = 5432
)

// Logger receives progress messages for a job log.
type Logger interface {
	io.Writer
	Logf(format string, args ...any)
}

// Manager manages PostgreSQL.
type Manager struct {
	run   run.Runner
	store *store.Store
	log   *slog.Logger
	mu    sync.Mutex

	importMu sync.Mutex
	imports  map[string]*sync.Mutex // one lock per database, see Import

	fw   Firewall // for remote access, see SetFirewall
	pgdg pgdgRepo // apt.postgresql.org, see pgdg.go

	// replaceable in tests
	supported func() bool
	memTotal  func() int64
	etcDir    string
	libDir    string
	dataDir   string
	socketDir string
	tempDir   string // for password files; "" for the system's
	wait      time.Duration
	portFree  func(port int) bool // nil: try to listen on it
	backupDir string              // where Upgrade keeps its dump
	freeBytes func(path string) (uint64, error)
}

// New returns a Manager.
func New(r run.Runner, st *store.Store, log *slog.Logger) *Manager {
	return &Manager{run: r, store: st, log: log, supported: supported, memTotal: memTotal,
		etcDir: defaultEtcDir, libDir: defaultLibDir, dataDir: defaultDataDir, socketDir: defaultSocketDir,
		wait: time.Second, pgdg: defaultPGDGRepo(), backupDir: defaultBackupDir, freeBytes: freeBytes}
}

func supported() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	_, err := exec.LookPath("apt-get")
	return err == nil
}

// Installed reports whether a PostgreSQL server is installed, without
// running anything.
func Installed() bool { return len(serverVersions(defaultLibDir)) > 0 }

var majorRe = regexp.MustCompile(`^[0-9]{1,3}$`)

// serverVersions returns the installed major versions, oldest first.
func serverVersions(libDir string) []string {
	matches, _ := filepath.Glob(filepath.Join(libDir, "*", "bin", "postgres"))
	var vs []string
	for _, p := range matches {
		if v := filepath.Base(filepath.Dir(filepath.Dir(p))); majorRe.MatchString(v) {
			vs = append(vs, v)
		}
	}
	slices.SortFunc(vs, func(a, b string) int { x, _ := strconv.Atoi(a); y, _ := strconv.Atoi(b); return x - y })
	return vs
}

// cluster is the "main" cluster the panel manages.
type cluster struct {
	version string // major version, such as "16"
	port    int
}

func (c cluster) unit() string { return "postgresql@" + c.version + "-main" }

func (c cluster) major() int { n, _ := strconv.Atoi(c.version); return n }

func (m *Manager) mainDir(v string) string { return filepath.Join(m.etcDir, v, "main") }

func (m *Manager) bin(c cluster, name string) string {
	return filepath.Join(m.libDir, c.version, "bin", name)
}

// cluster returns the main cluster of the newest installed version that has
// one.
func (m *Manager) cluster() (cluster, bool) {
	vs := serverVersions(m.libDir)
	for i := len(vs) - 1; i >= 0; i-- {
		if c, ok := m.clusterOf(vs[i]); ok {
			return c, true
		}
	}
	return cluster{}, false
}

func (m *Manager) clusterOf(v string) (cluster, bool) {
	b, err := os.ReadFile(filepath.Join(m.mainDir(v), "postgresql.conf"))
	if err != nil {
		return cluster{}, false
	}
	return cluster{version: v, port: parsePort(b)}, true
}

var portRe = regexp.MustCompile(`(?m)^[ \t]*port[ \t]*=[ \t]*([0-9]{2,5})\b`)

// parsePort reads the port of a postgresql.conf; the last setting wins.
func parsePort(conf []byte) int {
	port := defaultPort
	for _, mm := range portRe.FindAllSubmatch(conf, -1) {
		if n, err := strconv.Atoi(string(mm[1])); err == nil && n > 0 && n < 65536 {
			port = n
		}
	}
	return port
}

// Status describes the PostgreSQL server.
type Status struct {
	Supported bool   `json:"supported"`
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
	// Major is the major version ("16"); Source says where its packages
	// come from: "distribution" or "pgdg" (apt.postgresql.org).
	Major   string `json:"major,omitempty"`
	Source  string `json:"source,omitempty"`
	Running bool   `json:"running"`
	Port    int    `json:"port,omitempty"`
}

var versionRe = regexp.MustCompile(`PostgreSQL\)? ([0-9]+(?:\.[0-9]+)?)`)

// Status reports whether PostgreSQL is installed and running.
func (m *Manager) Status(ctx context.Context) Status {
	st := Status{Supported: m.supported()}
	if !st.Supported {
		return st
	}
	vs := serverVersions(m.libDir)
	if len(vs) == 0 {
		return st
	}
	st.Installed = true
	st.Source = "distribution"
	c, ok := m.cluster()
	if !ok {
		st.Version = vs[len(vs)-1]
		st.Major = st.Version
		return st
	}
	st.Version, st.Major, st.Port = c.version, c.version, c.port
	// Debian and Ubuntu builds add their package version:
	// "postgres (PostgreSQL) 17.12 (Ubuntu 17.12-1.pgdg24.04+1)".
	if out, err := m.run.Output(ctx, m.bin(c, "postgres"), "--version"); err == nil {
		if mm := versionRe.FindStringSubmatch(out); mm != nil {
			st.Version = mm[1]
		}
		if strings.Contains(out, ".pgdg") {
			st.Source = "pgdg"
		}
	}
	out, err := m.run.Output(ctx, "systemctl", "is-active", c.unit())
	st.Running = err == nil && strings.HasPrefix(out, "active")
	return st
}

// ready returns the cluster when PostgreSQL is installed and running.
func (m *Manager) ready(ctx context.Context) (cluster, error) {
	st := m.Status(ctx)
	if !st.Installed {
		return cluster{}, usererr.New("pg_not_installed")
	}
	c, ok := m.cluster()
	if !ok || !st.Running {
		return cluster{}, usererr.New("pg_not_running")
	}
	return c, nil
}

// sql runs statements as the postgres OS user, connected to database db,
// and returns the rows, one per line with tab-separated columns. psql's
// messages go to standard error, which is kept for the error only.
func (m *Manager) sql(ctx context.Context, c cluster, db, statements string) (string, error) {
	if db != "postgres" {
		if _, err := ident(db); err != nil {
			return "", err
		}
	}
	var out bytes.Buffer
	err := m.run.Pipe(ctx, strings.NewReader(statements), &out, "runuser", "-u", "postgres", "--",
		m.bin(c, "psql"), "-X", "-q", "-A", "-t", "-F", "\t", "-v", "ON_ERROR_STOP=1",
		"-h", m.socketDir, "-p", strconv.Itoa(c.port), "-d", db)
	return out.String(), err
}

// ---- installing ----

// Install installs PostgreSQL, tunes it for this server's memory in a
// conf.d drop-in, lets it listen on localhost only and writes a
// pg_hba.conf that only accepts passwords (SCRAM-SHA-256), except for the
// postgres OS user on the local socket. major "" is the distribution's
// version (16 on Ubuntu 24.04, 15 on Debian 12…), or the one installed
// already; another major comes from apt.postgresql.org (see pgdg.go). One
// major runs at a time: another one, installed or with its cluster still
// on disk, is refused.
func (m *Manager) Install(ctx context.Context, major string, lg Logger) error {
	if !m.supported() {
		return errors.New("PostgreSQL can only be installed on Debian or Ubuntu")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	major, want, err := m.installTarget(major)
	if err != nil {
		return err
	}
	pgdg := m.usePGDG(major)

	had := map[string]bool{}
	for _, v := range serverVersions(m.libDir) {
		if _, ok := m.clusterOf(v); ok {
			had[v] = true
		}
	}
	// A new cluster takes port 5432; another program there (such as the
	// App store's PostgreSQL container) would make it fail to start.
	if len(had) == 0 && m.portBusy(defaultPort) {
		return usererr.New("pg_port_busy", strconv.Itoa(defaultPort))
	}
	// With the PostgreSQL project's repository, packages go by name: its
	// "postgresql" depends on the newest major, not the distribution's.
	pkg := "postgresql"
	switch {
	case pgdg:
		lg.Logf("Adding the PostgreSQL project's repository (apt.postgresql.org, signing key %s)", pgdgFingerprint)
		if err := m.addPGDGRepo(ctx); err != nil {
			return err
		}
		pkg = "postgresql-" + major
	case m.hasPGDGRepo() && major != "":
		// The distribution's version installed next to the repository
		// (left by an upgrade that was rolled back), being repaired.
		pkg = "postgresql-" + major
	case m.hasPGDGRepo():
		lg.Logf("Removing the PostgreSQL project's repository: this installs the distribution's version")
		if err := m.removePGDGRepo(lg); err != nil {
			return err
		}
	}
	lg.Logf("Updating the package lists")
	if err := run.Apt(ctx, m.run, lg, "update"); err != nil {
		return err
	}
	if pgdg {
		lg.Logf("Installing PostgreSQL %s from apt.postgresql.org", major)
	} else {
		lg.Logf("Installing PostgreSQL from the distribution's packages")
	}
	if err := run.Apt(ctx, m.run, lg, "install", "--no-install-recommends", pkg); err != nil {
		return err
	}
	vs := serverVersions(m.libDir)
	if len(vs) == 0 {
		return fmt.Errorf("the %s package did not install a server", pkg)
	}
	v := vs[len(vs)-1]
	if want != "" {
		if !slices.Contains(vs, want) {
			return fmt.Errorf("the %s package did not install PostgreSQL %s", pkg, want)
		}
		v = want
	}
	c, ok := m.clusterOf(v)
	switch {
	case !ok:
		lg.Logf("Creating the main cluster of PostgreSQL %s", v)
		if err := m.createCluster(ctx, v, lg); err != nil {
			return err
		}
	case !had[v]:
		// apt runs the package scripts in the C locale, so the cluster they
		// just made encodes text as SQL_ASCII. Nothing is in it yet: make
		// it again in UTF-8.
		if enc, err := m.sql(ctx, c, "postgres", "SHOW server_encoding;"); err == nil && strings.TrimSpace(enc) != "UTF8" {
			lg.Logf("Making the new cluster again with UTF-8 text (it was %s)", strings.TrimSpace(enc))
			if out, err := m.run.Output(ctx, "pg_dropcluster", "--stop", v, "main"); err != nil {
				return fmt.Errorf("pg_dropcluster: %s", run.Tail(out, 5))
			}
			if err := m.createCluster(ctx, v, lg); err != nil {
				return err
			}
		}
	}
	if c, ok = m.clusterOf(v); !ok {
		return fmt.Errorf("PostgreSQL %s has no main cluster in %s", v, m.mainDir(v))
	}
	t := tuningFor(m.memTotal())
	lg.Logf("Tuning PostgreSQL for this server's memory: shared_buffers %d MB, max_connections %d", t.SharedBuffersMB, t.MaxConnections)
	if err := m.writeConfig(c, t); err != nil {
		return err
	}
	if _, err := m.run.Output(ctx, "systemctl", "enable", "postgresql"); err != nil {
		return err
	}
	if out, err := m.run.Output(ctx, "systemctl", "restart", c.unit()); err != nil {
		return fmt.Errorf("starting PostgreSQL failed: %s", run.Tail(out, 10))
	}
	if err := m.waitReady(ctx, c); err != nil {
		return err
	}
	// The drop-in only counts if postgresql.conf includes conf.d, as the
	// clusters Debian and Ubuntu make do.
	if out, err := m.sql(ctx, c, "postgres", "SHOW max_connections;"); err == nil && strings.TrimSpace(out) != strconv.Itoa(t.MaxConnections) {
		lg.Logf("Warning: PostgreSQL did not read %s: add include_dir = 'conf.d' to its postgresql.conf", filepath.Join(m.mainDir(v), "conf.d", confName))
	}
	// A cluster installed again keeps the remote access of its databases:
	// pg_hba.conf was just written anew.
	if entries, err := m.remoteEntries(ctx); err == nil && len(entries) > 0 {
		lg.Logf("Restoring the remote access of %d database(s)", len(entries))
		if err := m.applyRemote(ctx, c); err != nil {
			return err
		}
		if m.restartPending(ctx, c) {
			if out, err := m.run.Output(ctx, "systemctl", "restart", c.unit()); err != nil {
				return fmt.Errorf("restarting PostgreSQL failed: %s", run.Tail(out, 10))
			}
			if err := m.waitReady(ctx, c); err != nil {
				return err
			}
		}
	} else if err == nil {
		// No database allows remote access: no drop-in from before either.
		if err := os.Remove(m.remoteConfPath(c)); err == nil {
			lg.Logf("Removed %s, left from an earlier install", m.remoteConfPath(c))
		}
	}
	lg.Logf("PostgreSQL %s is ready on localhost, port %d. Create databases on the Databases page.", v, c.port)
	return nil
}

// CheckInstall returns why major ("" for the distribution's) cannot be
// installed now (a usererr), or nil. It only reads files.
func (m *Manager) CheckInstall(major string) error {
	_, _, err := m.installTarget(major)
	return err
}

// installTarget checks an install of major and returns it (the installed
// one when major is "" and one is installed) with the major version that
// must end up installed ("" on a system the panel does not know: whatever
// the "postgresql" package brings). One major at a time: another one
// installed, or with a cluster left on disk, is refused.
func (m *Manager) installTarget(major string) (string, string, error) {
	if err := m.checkMajor(major); err != nil {
		return "", "", err
	}
	installed := serverVersions(m.libDir)
	if major == "" && len(installed) > 0 {
		major = installed[len(installed)-1] // repairing the installed one
	}
	want := major
	if want == "" {
		r, _, _ := m.release()
		want = r.distro
	}
	if want != "" {
		for _, v := range installed {
			if v != want {
				return "", "", usererr.New("pg_other_major", v)
			}
		}
		for _, v := range m.clusterVersions() {
			if v != want {
				return "", "", usererr.New("pg_old_cluster", v)
			}
		}
	}
	return major, want, nil
}

// clusterVersions returns the major versions that have a main cluster on
// disk, configured or with data, installed or not.
func (m *Manager) clusterVersions() []string {
	var vs []string
	for _, pat := range []string{filepath.Join(m.etcDir, "*", "main", "postgresql.conf"), filepath.Join(m.dataDir, "*", "main", "PG_VERSION")} {
		matches, _ := filepath.Glob(pat)
		for _, p := range matches {
			if v := filepath.Base(filepath.Dir(filepath.Dir(p))); majorRe.MatchString(v) && !slices.Contains(vs, v) {
				vs = append(vs, v)
			}
		}
	}
	slices.Sort(vs)
	return vs
}

// portBusy reports whether something listens on a local TCP port.
func (m *Manager) portBusy(port int) bool {
	if m.portFree != nil {
		return !m.portFree(port)
	}
	l, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		return true
	}
	l.Close()
	return false
}

// createCluster makes the main cluster of version v with UTF-8 text.
func (m *Manager) createCluster(ctx context.Context, v string, lg Logger) error {
	out, err := m.run.Output(ctx, "pg_createcluster", "--locale", "C.UTF-8", "--encoding", "UTF8", v, "main")
	if err != nil {
		return fmt.Errorf("pg_createcluster: %s", run.Tail(out, 5))
	}
	return nil
}

// waitReady waits until the server answers, at most 30 seconds.
func (m *Manager) waitReady(ctx context.Context, c cluster) error {
	var err error
	for i := 0; i < 30; i++ {
		if _, err = m.sql(ctx, c, "postgres", "SELECT 1;"); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(m.wait):
		}
	}
	return fmt.Errorf("PostgreSQL does not answer: %w", err)
}

// writeConfig writes the panel's drop-in and pg_hba.conf, owned like the
// cluster's postgresql.conf (by postgres) so the server can read them.
func (m *Manager) writeConfig(c cluster, t tuning) error {
	if err := m.writeTuning(c, t); err != nil {
		return err
	}
	return m.owned(c, filepath.Join(m.mainDir(c.version), "pg_hba.conf"), []byte(hbaConf), 0o640)
}

// writeTuning writes the panel's drop-in only.
func (m *Manager) writeTuning(c cluster, t tuning) error {
	confd := filepath.Join(m.mainDir(c.version), "conf.d")
	if err := fsutil.MkdirAll(confd, 0o755); err != nil {
		return err
	}
	if fi, err := os.Stat(filepath.Join(m.mainDir(c.version), "postgresql.conf")); err == nil {
		if uid, gid := fsutil.Owner(fi); uid >= 0 {
			os.Chown(confd, uid, gid)
		}
	}
	return m.owned(c, filepath.Join(confd, confName), []byte(t.conf()), 0o644)
}

// tuning is the panel's PostgreSQL memory settings for a server.
type tuning struct {
	SharedBuffersMB  int
	EffectiveCacheMB int
	WorkMemMB        int
	MaintenanceMB    int
	MaxConnections   int
}

// tuningFor returns the settings for memTotal bytes of RAM (0: unknown, as
// for 1 GB). The cache hint is about half the memory, as the server also
// runs a web server, PHP and maybe MariaDB.
func tuningFor(memTotal int64) tuning {
	switch {
	case memTotal >= 7<<30:
		return tuning{1024, 4096, 16, 256, 100}
	case memTotal >= 3<<30:
		return tuning{512, 2048, 8, 128, 75}
	case memTotal >= 1500<<20:
		return tuning{256, 1024, 4, 64, 50}
	case memTotal >= 768<<20 || memTotal == 0:
		return tuning{128, 512, 4, 32, 40}
	}
	return tuning{64, 256, 2, 16, 30}
}

func (t tuning) conf() string {
	return fmt.Sprintf(`# Managed by Nefo. Do not edit: the panel rewrites this file.
# PostgreSQL listens on this server only and is sized for its memory.
listen_addresses = 'localhost'
password_encryption = 'scram-sha-256'
max_connections = %d
shared_buffers = %dMB
effective_cache_size = %dMB
work_mem = %dMB
maintenance_work_mem = %dMB
random_page_cost = 1.1
jit = off
`, t.MaxConnections, t.SharedBuffersMB, t.EffectiveCacheMB, t.WorkMemMB, t.MaintenanceMB)
}

// hbaConf lets the postgres OS user in on the local socket without a
// password (peer), which is how the panel manages the server, and everyone
// else only with a password, on the socket or over TCP from this machine.
// Unlike the distribution's default there is no peer login for other OS
// users: a website's Linux user named like a database role would otherwise
// get into that database without its password.
const hbaConf = `# Managed by Nefo. Do not edit: the panel rewrites this file.
# TYPE  DATABASE  USER      ADDRESS       METHOD
local   all       postgres                peer
local   all       all                     scram-sha-256
host    all       all       127.0.0.1/32  scram-sha-256
host    all       all       ::1/128       scram-sha-256
`

// Restart restarts PostgreSQL, which also applies a new listen setting. If
// it does not come back with the panel's remote access drop-in, the drop-in
// is removed and the server started again without it, so the websites keep
// working.
func (m *Manager) Restart(ctx context.Context, lg Logger) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.cluster()
	if !ok {
		return usererr.New("pg_not_installed")
	}
	lg.Logf("Restarting PostgreSQL %s", c.version)
	if out, err := m.run.Output(ctx, "systemctl", "restart", c.unit()); err != nil {
		conf := m.remoteConfPath(c)
		if !fsutil.Exists(conf) {
			return fmt.Errorf("restarting PostgreSQL failed: %s", run.Tail(out, 10))
		}
		lg.Logf("PostgreSQL did not start: %s", run.Tail(out, 10))
		lg.Logf("Removing %s and starting PostgreSQL again", conf)
		if err := os.Remove(conf); err != nil {
			return err
		}
		if out, err := m.run.Output(ctx, "systemctl", "restart", c.unit()); err != nil {
			return fmt.Errorf("restarting PostgreSQL failed: %s", run.Tail(out, 10))
		}
		if err := m.waitReady(ctx, c); err != nil {
			return err
		}
		return errors.New("PostgreSQL could not start with remote access, so it was started without it")
	}
	if err := m.waitReady(ctx, c); err != nil {
		return err
	}
	if v, err := m.show(ctx, c, "listen_addresses"); err == nil {
		if listensAll(v) {
			lg.Logf("PostgreSQL listens on all interfaces (%s), port %d", v, c.port)
		} else {
			lg.Logf("PostgreSQL listens on %s only, port %d", v, c.port)
		}
	}
	return nil
}

// memTotal is the server's memory in bytes: in a container, its limit
// when that is smaller than what /proc/meminfo shows (see meminfo).
func memTotal() int64 { return meminfo.Total() }

// MemoryMB is roughly how much memory PostgreSQL uses with the panel's
// settings on this server once its cache has filled: shared_buffers and
// about 40 MB for its processes. Idle and empty it uses about 30-40 MB.
func MemoryMB() int { return tuningFor(memTotal()).SharedBuffersMB + 40 }
