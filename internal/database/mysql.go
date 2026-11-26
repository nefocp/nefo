package database

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/pgpkey"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/usererr"
)

// MySQL 8.4 LTS comes from Oracle's apt repository (repo.mysql.com), the
// mysql-8.4-lts track: MySQL 8.0 reached its end of life in April 2026,
// and 8.4 is supported until 2032. The panel adds the repository itself
// instead of installing Oracle's mysql-apt-config package, as it does for
// OpenLiteSpeed: the signing key is downloaded over HTTPS, accepted only if
// it is exactly the pinned key, and trusted for this repository only
// (signed-by), not for all of apt.
const (
	mysqlTrack = "mysql-8.4-lts"
	// mysqlKeyFingerprint is "MySQL Release Engineering
	// <mysql-build@oss.oracle.com>", RSA 4096 from 2023-10-23 (key ID
	// B7B3B788A8D3785C), which signs MySQL 8.0.44+, 8.4.7+ and 9.5+ and the
	// repository's Release files. Source: "Signature Checking Using GnuPG",
	// https://dev.mysql.com/doc/refman/8.4/en/checking-gpg-signature.html;
	// keyserver.ubuntu.com has the same key. Oracle renewed it in October
	// 2025 to expire on 2027-10-23 (RefreshMySQLKey picks up the next
	// renewal).
	mysqlKeyFingerprint = "BCA43417C3B485DD128EC6D4B7B3B788A8D3785C"
	// mysqlKeyURL is the renewed key; RPM-GPG-KEY-mysql-2023 next to it is
	// the same key with its expired signatures.
	mysqlKeyURL = "https://repo.mysql.com/RPM-GPG-KEY-mysql-2025"
	// mysqlKeyserverURL is where a later renewal shows up first.
	mysqlKeyserverURL = "https://keyserver.ubuntu.com/pks/lookup?op=get&options=mr&search=0x" + mysqlKeyFingerprint
	mysqlRepoURL      = "https://repo.mysql.com/apt/"
	mysqlKeyring      = "/etc/apt/keyrings/panel-mysql.gpg"
	mysqlRepoList     = "/etc/apt/sources.list.d/panel-mysql.list"
	// mysqlKeyRefresh is how often RefreshMySQLKey looks for a renewed key.
	mysqlKeyRefresh = 7 * 24 * time.Hour
)

// mysqlReleases are the releases whose mysql-8.4-lts track has packages,
// by codename, with their distribution: Ubuntu 22.04, 24.04 and 26.04,
// Debian 12 and 13 (https://repo.mysql.com/apt/ubuntu/dists/ and
// .../debian/dists/, November 2026). The track has amd64 packages only, for
// every release: no arm64.
var mysqlReleases = map[string]string{
	"jammy": "ubuntu", "noble": "ubuntu", "resolute": "ubuntu",
	"bookworm": "debian", "trixie": "debian",
}

// mysqlRepo is where the repository's files go and what the system is;
// tests replace it.
type mysqlRepo struct {
	keyURLs   []string // the first is used to install; RefreshMySQLKey tries all
	keyring   string
	list      string
	osRelease func() (id, codename string)
	arch      func() string
	client    *http.Client
}

func defaultMySQLRepo() mysqlRepo {
	return mysqlRepo{
		keyURLs: []string{mysqlKeyURL, mysqlKeyserverURL},
		keyring: mysqlKeyring, list: mysqlRepoList,
		osRelease: func() (string, string) { return readOSRelease("/etc/os-release") },
		arch:      func() string { return runtime.GOARCH },
		client:    &http.Client{Timeout: 2 * time.Minute},
	}
}

// readOSRelease returns ID and VERSION_CODENAME from an os-release file.
func readOSRelease(path string) (id, codename string) {
	f, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), "=")
		if !ok {
			continue
		}
		v = strings.Trim(v, `"'`)
		switch k {
		case "ID":
			id = v
		case "VERSION_CODENAME":
			codename = v
		}
	}
	return id, codename
}

// MySQLAvailable returns why MySQL 8.4 cannot be installed on this server
// (a usererr: mysql_unsupported_os with the release, or
// mysql_unsupported_arch with the CPU architecture), or nil. It runs
// nothing.
func (m *Manager) MySQLAvailable() error {
	if !m.supported() {
		return usererr.New("unsupported")
	}
	id, codename := m.repo.osRelease()
	if dist, ok := mysqlReleases[codename]; !ok || dist != id {
		release := strings.TrimSpace(id + " " + codename)
		if release == "" {
			release = "?"
		}
		return usererr.New("mysql_unsupported_os", release)
	}
	if a := m.repo.arch(); a != "amd64" {
		return usererr.New("mysql_unsupported_arch", a)
	}
	return nil
}

// InstallMySQL installs MySQL 8.4 LTS from Oracle's repository, tuned like
// MariaDB for a small server. root logs in through the socket
// (auth_socket), so no root password is set or stored. It is refused while
// MariaDB is installed, and when the data directory holds data that is not
// MySQL 8's (see checkInstall).
func (m *Manager) InstallMySQL(ctx context.Context, lg Logger) error {
	if err := m.MySQLAvailable(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.checkInstall(ctx, MySQL); err != nil {
		return err
	}
	lg.Logf("Adding Oracle's MySQL repository (%s, signing key %s)", mysqlTrack, mysqlKeyFingerprint)
	if err := m.addMySQLRepo(ctx); err != nil {
		return err
	}
	// The packages' questions, answered before apt asks them: an empty root
	// password makes the package switch root to auth_socket, and a later
	// purge keeps the databases.
	seed := "mysql-community-server mysql-community-server/root-pass password \n" +
		"mysql-community-server mysql-community-server/re-root-pass password \n" +
		"mysql-community-server mysql-community-server/remove-data-dir boolean false\n" +
		"mysql-community-server mysql-server/lowercase-table-names select Disabled\n"
	if out, err := m.run.Input(ctx, []byte(seed), "debconf-set-selections"); err != nil {
		return fmt.Errorf("debconf-set-selections: %s", strings.TrimSpace(out))
	}
	// The panel's settings are in place before the package first starts
	// the server, so it never listens on port 33060 or writes binary logs.
	lg.Logf("Tuning MySQL for this server's memory")
	conf := filepath.Join(m.confDir(MySQL), panelConfName)
	if err := fsutil.MkdirAll(filepath.Dir(conf), 0o755); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(conf, []byte(mysqlTuning(memTotal())), 0o644); err != nil {
		return err
	}
	lg.Logf("Updating the package lists")
	if err := run.Apt(ctx, m.run, lg, "update"); err != nil {
		return err
	}
	lg.Logf("Installing MySQL 8.4")
	// The community package names exist in Oracle's repository only, so apt
	// cannot pick a distribution's MySQL instead.
	if err := run.Apt(ctx, m.run, lg, "install", "--no-install-recommends", "mysql-community-server", "mysql-community-client"); err != nil {
		return err
	}
	if _, err := m.run.Output(ctx, "systemctl", "enable", MySQL.service()); err != nil {
		return err
	}
	if _, err := m.run.Output(ctx, "systemctl", "restart", MySQL.service()); err != nil {
		return err
	}
	out, err := m.sqlWith(ctx, MySQL, "SELECT CURRENT_USER();")
	if err != nil {
		return fmt.Errorf("MySQL is installed, but root cannot log in through the socket: %w", err)
	}
	if u := strings.TrimSpace(out); u != "root@localhost" {
		return fmt.Errorf("MySQL is installed, but the panel logs in as %q instead of root@localhost", u)
	}
	lg.Logf("Removing anonymous users and the test database")
	if _, err := m.sqlWith(ctx, MySQL, "DROP USER IF EXISTS ''@'localhost', ''@'%';\nDROP DATABASE IF EXISTS test;"); err != nil {
		return err
	}
	lg.Logf("MySQL is ready")
	return nil
}

// performanceSchema reports whether the panel leaves MySQL's performance
// schema on: from about 4 GB of RAM (a "4 GB" server reports a little
// less). It takes 100 to 200 MB by itself.
func performanceSchema(memTotal int64) bool { return memTotal >= 3500<<20 }

// mysqlTuning returns the panel's MySQL settings for a server with memTotal
// bytes of RAM: like MariaDB's (tuning), and without what MySQL turns on by
// default that a single small server does not need.
func mysqlTuning(memTotal int64) string {
	pool := bufferPoolMB(memTotal)
	conns, tmp, tables := 75, 32, 400
	switch {
	case memTotal >= 7<<30:
		conns, tmp, tables = 200, 256, 2000
	case memTotal >= 3<<30:
		conns, tmp, tables = 150, 128, 1000
	case memTotal >= 1500<<20:
		conns, tmp, tables = 100, 64, 600
	}
	ps := "OFF"
	if performanceSchema(memTotal) {
		ps = "ON"
	}
	return fmt.Sprintf(`# Managed by the panel. Do not edit: the panel rewrites this file.
[mysqld]
# This server only; 99-panel-remote.cnf opens it while a database allows
# remote access.
bind-address = 127.0.0.1
# No X Protocol listener on port 33060: PHP and the panel use the classic
# protocol on port 3306 and the socket.
mysqlx = OFF
# No binary logs: they would fill the disk, and there are no replicas.
disable_log_bin
skip-name-resolve
host_cache_size = 0
performance_schema = %s
innodb_buffer_pool_size = %dM
innodb_log_buffer_size = 16M
temptable_max_ram = %dM
table_open_cache = %d
max_connections = %d
character-set-server = utf8mb4
collation-server = utf8mb4_unicode_ci
`, ps, pool, tmp, tables, conns)
}

// addMySQLRepo writes the pinned key and the repository's source list.
func (m *Manager) addMySQLRepo(ctx context.Context) error {
	id, codename := m.repo.osRelease()
	if dist, ok := mysqlReleases[codename]; !ok || dist != id {
		return fmt.Errorf("Oracle's repository has no MySQL 8.4 packages for %s %s", id, codename)
	}
	key, err := m.fetchMySQLKey(ctx, m.repo.keyURLs[0])
	if err != nil {
		return err
	}
	if err := fsutil.MkdirAll(filepath.Dir(m.repo.keyring), 0o755); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(m.repo.keyring, key, 0o644); err != nil {
		return err
	}
	list := fmt.Sprintf("# Managed by the panel: MySQL 8.4 LTS from Oracle's repository.\n"+
		"deb [arch=amd64 signed-by=%s] %s%s/ %s %s\n", m.repo.keyring, mysqlRepoURL, id, codename, mysqlTrack)
	return fsutil.WriteFileAtomic(m.repo.list, []byte(list), 0o644)
}

// fetchMySQLKey downloads Oracle's signing key (armored or binary) and
// returns it in the binary form apt reads, only if it is the pinned key.
func (m *Manager) fetchMySQLKey(ctx context.Context, url string) ([]byte, error) {
	b, err := httpGet(ctx, m.repo.client, url, 256<<10)
	if err != nil {
		return nil, err
	}
	if bytes.Contains(b, []byte("-----BEGIN PGP")) {
		if b, err = pgpkey.Dearmor(b); err != nil {
			return nil, fmt.Errorf("MySQL's signing key could not be read: %w", err)
		}
	}
	fp, err := pgpkey.Fingerprint(b)
	if err != nil {
		return nil, fmt.Errorf("MySQL's signing key could not be read: %w", err)
	}
	if fp != mysqlKeyFingerprint {
		return nil, fmt.Errorf("MySQL's signing key has the fingerprint %s, not the expected %s: not installed", fp, mysqlKeyFingerprint)
	}
	return b, nil
}

func httpGet(ctx context.Context, c *http.Client, url string, max int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err == nil && int64(len(b)) > max {
		err = fmt.Errorf("download %s: larger than expected", url)
	}
	return b, err
}

// RefreshMySQLKey keeps the repository key from expiring: Oracle renews the
// same key (the copy installed in 2026 expires on 2027-10-23), and apt
// refuses an expired one, which would make every "apt-get update" fail.
// While the panel's MySQL repository is set up, it looks for a renewed copy
// at most once a week and replaces the keyring with a copy of the pinned
// key that expires later. It is cheap to call often (the maintenance loop
// does, every hour).
func (m *Manager) RefreshMySQLKey(ctx context.Context) error {
	if !fileExists(m.repo.list) {
		return nil
	}
	fi, err := os.Stat(m.repo.keyring)
	if err != nil {
		return nil // the source list is not the panel's working repository
	}
	if time.Since(fi.ModTime()) < mysqlKeyRefresh {
		return nil
	}
	cur, err := os.ReadFile(m.repo.keyring)
	if err != nil {
		return err
	}
	best := cur
	bestExp, bestNever, _ := pgpkey.Expiry(cur)
	var errs []error
	for _, u := range m.repo.keyURLs {
		key, err := m.fetchMySQLKey(ctx, u)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		exp, never, err := pgpkey.Expiry(key)
		if err != nil || bestNever || (!never && !exp.After(bestExp)) {
			continue
		}
		best, bestExp, bestNever = key, exp, never
	}
	if !bytes.Equal(best, cur) {
		if err := fsutil.WriteFileAtomic(m.repo.keyring, best, 0o644); err != nil {
			return err
		}
		if m.log != nil {
			m.log.Info("renewed the MySQL repository key", "expires", bestExp.Format(time.DateOnly), "never", bestNever)
		}
		return nil
	}
	next, failed := time.Now(), len(errs) == len(m.repo.keyURLs)
	if failed {
		// Offline, maybe: try again in a day rather than every hour.
		next = next.Add(24*time.Hour - mysqlKeyRefresh)
	}
	if err := os.Chtimes(m.repo.keyring, next, next); err != nil {
		return err
	}
	if failed {
		return errors.Join(errs...)
	}
	return nil
}
