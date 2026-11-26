// Package database installs MariaDB or MySQL (see Engine) and manages
// databases and their users.
//
// The panel runs as root and talks to the server with its command-line
// client (mariadb or mysql) over the local socket, where root logs in
// without a password (unix_socket on MariaDB, auth_socket on MySQL). SQL is
// sent on stdin so passwords never appear in the process list. Names and
// passwords pass strict allow-lists first (see package validate), so no
// value can break out of its quotes.
package database

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/secure"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
)

// ErrNotInstalled is returned while no database server (MariaDB or MySQL)
// is installed.
var ErrNotInstalled = errors.New("the database server (MariaDB or MySQL) is not installed")

// panelConfName holds the panel's tuning, in the engine's drop-in folder.
const panelConfName = "90-panel.cnf"

// Logger receives progress messages for a job log.
type Logger interface {
	io.Writer
	Logf(format string, args ...any)
}

// Manager manages the database server, MariaDB or MySQL.
type Manager struct {
	run   run.Runner
	store *store.Store
	log   *slog.Logger
	mu    sync.Mutex

	fw          Firewall // for remote access, see SetFirewall
	listenDirty bool     // the listen setting changed and the server has not restarted since

	importMu sync.Mutex             // guards imports
	imports  map[string]*sync.Mutex // one lock per database name, see Import

	// replaceable in tests; "" means the engine's own path
	supported  func() bool
	detect     func() Engine // nil: InstalledEngine
	remoteConf string
	slowConf   string
	slowLog    string
	dataDir    string // "" for /var/lib/mysql (see remove.go)
	etcDir     string // "" for /etc/mysql
	repo       mysqlRepo
}

// New returns a Manager.
func New(r run.Runner, st *store.Store, log *slog.Logger) *Manager {
	return &Manager{run: r, store: st, log: log, supported: supported, repo: defaultMySQLRepo()}
}

// Status describes the database server.
type Status struct {
	Supported bool `json:"supported"`
	Installed bool `json:"installed"`
	// Engine is the installed server, "mariadb" or "mysql" ("" while none is).
	Engine  Engine `json:"engine"`
	Version string `json:"version"`
	Running bool   `json:"running"`
}

var versionRe = regexp.MustCompile(`Ver ([0-9]+\.[0-9]+\.[0-9]+)`)

func supported() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	_, err := exec.LookPath("apt-get")
	return err == nil
}

// Status reports whether the database server is installed and running.
func (m *Manager) Status(ctx context.Context) Status {
	st := Status{Supported: m.supported()}
	if !st.Supported {
		return st
	}
	e := m.engine()
	out, err := m.run.Output(ctx, e.daemon(), "--version")
	if err != nil {
		return st
	}
	st.Installed, st.Engine = true, e
	if mm := versionRe.FindStringSubmatch(out); mm != nil {
		st.Version = mm[1]
	}
	out, err = m.run.Output(ctx, "systemctl", "is-active", e.service())
	st.Running = err == nil && strings.HasPrefix(out, "active")
	return st
}

func (m *Manager) ready(ctx context.Context) error {
	if st := m.Status(ctx); !st.Installed {
		return ErrNotInstalled
	}
	return nil
}

// Install installs MariaDB, limits its memory use to suit the server and
// removes the anonymous users and test database some versions ship with.
// It is refused while MySQL is installed (see InstallMySQL), or when the
// data directory holds MySQL's data.
func (m *Manager) Install(ctx context.Context, lg Logger) error {
	if !m.supported() {
		return errors.New("MariaDB can only be installed on Debian or Ubuntu")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.checkInstall(ctx, MariaDB); err != nil {
		return err
	}

	lg.Logf("Updating the package lists")
	if err := run.Apt(ctx, m.run, lg, "update"); err != nil {
		return err
	}
	lg.Logf("Installing MariaDB")
	if err := run.Apt(ctx, m.run, lg, "install", "--no-install-recommends", "mariadb-server", "mariadb-client"); err != nil {
		return err
	}
	lg.Logf("Tuning MariaDB for this server's memory")
	if err := fsutil.WriteFileAtomic(filepath.Join(m.confDir(MariaDB), panelConfName), []byte(tuning(memTotal())), 0o644); err != nil {
		return err
	}
	if _, err := m.run.Output(ctx, "systemctl", "enable", "mariadb"); err != nil {
		return err
	}
	if _, err := m.run.Output(ctx, "systemctl", "restart", "mariadb"); err != nil {
		return err
	}
	lg.Logf("Removing anonymous users and the test database")
	if _, err := m.sql(ctx, `DELETE FROM mysql.global_priv WHERE User = '';
DROP DATABASE IF EXISTS test;
DELETE FROM mysql.db WHERE Db = 'test' OR Db = 'test\\_%';
FLUSH PRIVILEGES;`); err != nil {
		return err
	}
	lg.Logf("MariaDB is ready")
	return nil
}

// tuning returns the panel's MariaDB settings for a server with memTotal
// bytes of RAM. MariaDB only listens on localhost.
func tuning(memTotal int64) string {
	pool := "128M"
	switch {
	case memTotal >= 7<<30:
		pool = "1G"
	case memTotal >= 3<<30:
		pool = "512M"
	case memTotal >= 1500<<20:
		pool = "256M"
	}
	return fmt.Sprintf(`# Managed by the panel. Do not edit: the panel rewrites this file.
[mysqld]
bind-address = 127.0.0.1
skip-name-resolve
performance_schema = OFF
innodb_buffer_pool_size = %s
max_connections = 100
character-set-server = utf8mb4
collation-server = utf8mb4_unicode_ci
`, pool)
}

func memTotal() int64 {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(b), "\n") {
		if f := strings.Fields(line); len(f) >= 2 && f[0] == "MemTotal:" {
			kb, _ := strconv.ParseInt(f[1], 10, 64)
			return kb << 10
		}
	}
	return 0
}

// sql runs statements as the server's root user and returns tab-separated
// rows.
func (m *Manager) sql(ctx context.Context, statements string) (string, error) {
	return m.sqlWith(ctx, m.engine(), statements)
}

// sqlWith is sql with engine e's client.
func (m *Manager) sqlWith(ctx context.Context, e Engine, statements string) (string, error) {
	return m.run.Input(ctx, []byte(statements), e.client(), "--batch", "--skip-column-names", "--raw")
}

// Info is a database with its size, for the list in the UI.
type Info struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Username   string `json:"username"`
	SiteID     int64  `json:"site_id,omitempty"`
	SiteDomain string `json:"site_domain,omitempty"`
	Size       int64  `json:"size"`
	// RemoteHosts are the client addresses that may connect from other
	// machines (see SetRemote).
	RemoteHosts []string `json:"remote_hosts"`
	// Missing means the database was removed outside the panel.
	Missing   bool   `json:"missing"`
	CreatedAt string `json:"created_at"`
}

// List returns the panel's databases with their sizes.
func (m *Manager) List(ctx context.Context) ([]Info, error) {
	dbs, err := m.store.ListDatabases(ctx)
	if err != nil {
		return nil, err
	}
	sizes := map[string]int64{}
	if m.Status(ctx).Running {
		fresh := ""
		if m.engine() == MySQL {
			// MySQL caches table sizes for a day by default.
			fresh = "SET SESSION information_schema_stats_expiry = 0;\n"
		}
		out, err := m.sql(ctx, fresh+`SELECT s.SCHEMA_NAME, COALESCE(SUM(t.DATA_LENGTH + t.INDEX_LENGTH), 0)
FROM information_schema.SCHEMATA s LEFT JOIN information_schema.TABLES t ON t.TABLE_SCHEMA = s.SCHEMA_NAME
GROUP BY s.SCHEMA_NAME;`)
		if err == nil {
			for _, line := range strings.Split(out, "\n") {
				if f := strings.Split(line, "\t"); len(f) == 2 {
					sizes[f[0]], _ = strconv.ParseInt(f[1], 10, 64)
				}
			}
		}
	}
	sites, err := m.store.ListSites(ctx)
	if err != nil {
		return nil, err
	}
	domains := map[int64]string{}
	for _, s := range sites {
		domains[s.ID] = s.Domain
	}
	remote, err := m.store.AllDBRemoteHosts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Info, 0, len(dbs))
	for _, d := range dbs {
		size, ok := sizes[d.Name]
		hosts := remote[d.ID]
		if hosts == nil {
			hosts = []string{}
		}
		out = append(out, Info{
			ID: d.ID, Name: d.Name, Username: d.Username, SiteID: d.SiteID, SiteDomain: domains[d.SiteID],
			Size: size, RemoteHosts: hosts,
			Missing: len(sizes) > 0 && !ok, CreatedAt: d.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		})
	}
	return out, nil
}

// NewPassword returns a random database password.
func NewPassword() string { return secure.String(24, secure.PasswordChars) }

// Create makes a database and a user of the same name that may only use it.
// An empty password is replaced by a random one.
func (m *Manager) Create(ctx context.Context, name, password string, siteID int64) (*store.Database, error) {
	if err := m.ready(ctx); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	name = strings.ToLower(strings.TrimSpace(name))
	if err := validate.DBName(name); err != nil {
		return nil, usererr.New("invalid_db_name", name)
	}
	if password == "" {
		password = NewPassword()
	} else if err := validate.DBPassword(password); err != nil {
		return nil, usererr.New("invalid_db_password")
	}
	if siteID != 0 {
		if _, err := m.store.SiteByID(ctx, siteID); err != nil {
			return nil, err
		}
	}
	if taken, err := m.store.DBNameTaken(ctx, name); err != nil {
		return nil, err
	} else if taken {
		return nil, usererr.New("db_exists", name)
	}
	out, err := m.sql(ctx, fmt.Sprintf(`SELECT
 (SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = '%[1]s') +
 (SELECT COUNT(*) FROM mysql.user WHERE User = '%[1]s');`, name))
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) != "0" {
		return nil, usererr.New("db_exists", name)
	}

	if _, err := m.sql(ctx, fmt.Sprintf(`CREATE DATABASE `+"`%[1]s`"+` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER '%[1]s'@'localhost' IDENTIFIED BY '%[2]s';
GRANT ALL PRIVILEGES ON `+"`%[1]s`"+`.* TO '%[1]s'@'localhost';`, name, password)); err != nil {
		m.drop(context.WithoutCancel(ctx), name, name)
		return nil, err
	}
	d := &store.Database{Name: name, Username: name, Password: password, SiteID: siteID}
	if err := m.store.CreateDatabase(ctx, d); err != nil {
		m.drop(context.WithoutCancel(ctx), name, name)
		return nil, err
	}
	return d, nil
}

// drop drops a database and its user, with the user's remote twins for
// remoteHosts (in MariaDB host form) and the import account Import may
// have kept.
func (m *Manager) drop(ctx context.Context, name, user string, remoteHosts ...string) error {
	if validate.DBName(name) != nil || validate.DBName(user) != nil {
		return fmt.Errorf("refusing to drop invalid name %q", name)
	}
	twins, err := remoteUserSQLFor(m.engine(), name, user, "", nil, remoteHosts)
	if err != nil {
		return err
	}
	_, err = m.sql(ctx, twins+fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;\nDROP USER IF EXISTS '%s'@'localhost';\n"+
		"DROP USER IF EXISTS '%s'@'localhost';", name, user, m.importUser(name)))
	return err
}

// SetPassword changes a database user's password, also for the user's
// remote twins. An empty password is replaced by a random one.
func (m *Manager) SetPassword(ctx context.Context, id int64, password string) (*store.Database, error) {
	if err := m.ready(ctx); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	d, err := m.store.DatabaseByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if password == "" {
		password = NewPassword()
	} else if err := validate.DBPassword(password); err != nil {
		return nil, usererr.New("invalid_db_password")
	}
	if validate.DBName(d.Username) != nil {
		return nil, fmt.Errorf("invalid user name %q", d.Username)
	}
	hosts, err := m.store.DBRemoteHosts(ctx, d.ID)
	if err != nil {
		return nil, err
	}
	var twins []string
	for _, h := range hosts {
		twins = append(twins, mariaHost(h))
	}
	// The twins are created again with the new password, which also
	// repairs one that went missing.
	twinSQL, err := remoteUserSQLFor(m.engine(), d.Name, d.Username, password, twins, nil)
	if err != nil {
		return nil, err
	}
	if _, err := m.sql(ctx, fmt.Sprintf("ALTER USER '%s'@'localhost' IDENTIFIED BY '%s';\n", d.Username, password)+twinSQL); err != nil {
		return nil, err
	}
	d.Password = password
	if err := m.store.UpdateDatabase(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

// Delete drops a database and its user. confirm must equal the name.
//
// Remote access goes with it: the user's remote twins are dropped, its ufw
// rules removed, and the listen drop-in removed when no other database
// allows remote access. The server then still listens on every interface
// until it restarts, so callers should check RestartPending afterwards.
func (m *Manager) Delete(ctx context.Context, id int64, confirm string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, err := m.store.DatabaseByID(ctx, id)
	if err != nil {
		return err
	}
	if strings.TrimSpace(confirm) != d.Name {
		return usererr.New("confirm_mismatch")
	}
	running := m.Status(ctx).Running
	twins, err := m.remoteTwins(ctx, d, running)
	if err != nil {
		return err
	}
	if running {
		if err := m.drop(ctx, d.Name, d.Username, twins...); err != nil {
			return err
		}
	}
	// The twins are gone, so a rule left behind lets nobody in: it is not a
	// reason to keep the database.
	if err := m.syncRemoteFirewall(ctx, d.Name, nil); err != nil {
		m.log.Warn("remove the firewall rules of a deleted database", "db", d.Name, "err", err)
	}
	if err := m.store.SetDBRemoteHosts(ctx, d.ID, nil); err != nil {
		return err
	}
	if err := m.store.DeleteDatabase(ctx, d.ID); err != nil {
		return err
	}
	if err := m.writeListenConf(ctx); err != nil {
		m.log.Warn("update the database server's listen setting", "err", err)
	}
	return nil
}

// Dump writes an SQL dump of a database to w, without stopping the site.
func (m *Manager) Dump(ctx context.Context, name string, w io.Writer) error {
	if err := validate.DBName(name); err != nil {
		return err
	}
	e := m.engine()
	args := []string{"--single-transaction", "--quick", "--routines", "--triggers", "--default-character-set=utf8mb4"}
	if e == MySQL {
		// Without GTID statements the dump also imports as a plain user
		// (see Import), and into MariaDB.
		args = append(args, "--set-gtid-purged=OFF")
	}
	return m.run.Pipe(ctx, nil, w, e.dumper(), append(args, name)...)
}

// importUserPrefix starts the name of the temporary account an import runs
// as. The panel's own users never have a dash in their name (see
// validate.DBName), so it can't be one of them. With a database name of at
// most 32 characters it fits MariaDB's 80-character limit; MySQL's is 32,
// see Manager.importUser.
const importUserPrefix = "nefo-import-"

// Import runs an SQL dump against a database.
//
// The dump runs as a temporary account that may only use this database,
// not as root: a dump that says USE other_db, CREATE DATABASE
// other_db or INSERT INTO other_db.t is refused (see OtherDBError) instead
// of writing into another database. This guards against mistakes, not
// against a hostile dump: the account may also set the DEFINER of
// triggers, views and routines, which a dump made with --routines
// --triggers (as Dump makes them) needs.
func (m *Manager) Import(ctx context.Context, name string, r io.Reader) error {
	if err := validate.DBName(name); err != nil {
		return err
	}
	// Two imports into the same database would share the account. Other
	// database operations don't wait for an import.
	lock := m.importLock(name)
	lock.Lock()
	defer lock.Unlock()

	user := m.importUser(name)
	password := secure.String(32, secure.PasswordChars)
	// The account goes away (see dropImportUser) also when the import
	// fails or ctx is cancelled, so the cleanup gets a context of its own.
	defer func() {
		dctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
		defer cancel()
		m.dropImportUser(dctx, name, user)
	}()
	socket, err := m.createImportUser(ctx, name, user, password)
	if err != nil {
		return err
	}
	cnf, err := writeClientConf(user, password, socket)
	if err != nil {
		return err
	}
	defer os.Remove(cnf)
	e := m.engine()
	if e == MySQL {
		r = skipSandboxLine(r)
	}
	// --defaults-file must come first. It also keeps a ~/.my.cnf of root
	// from replacing the account's user name or password.
	err = m.run.Pipe(ctx, r, io.Discard, e.client(), "--defaults-file="+cnf, "--default-character-set=utf8mb4", name)
	return importError(name, err)
}

// sandboxLineRe is the first line newer mariadb-dump versions write. The
// mysql client stops at it ("Unknown command '\-'"), so a dump from MariaDB
// imports into MySQL without it. The line does nothing on MySQL.
var sandboxLineRe = regexp.MustCompile(`^/\*M?!999999\\- enable the sandbox mode \*/\r?\n?$`)

// skipSandboxLine returns r without MariaDB's sandbox line, if it starts
// with one.
func skipSandboxLine(r io.Reader) io.Reader {
	br := bufio.NewReaderSize(r, 4096)
	line, err := br.ReadSlice('\n')
	if (err == nil || err == io.EOF) && sandboxLineRe.Match(line) {
		return br
	}
	return io.MultiReader(bytes.NewReader(bytes.Clone(line)), br)
}

// importLock returns the lock for imports into the database name.
func (m *Manager) importLock(name string) *sync.Mutex {
	m.importMu.Lock()
	defer m.importMu.Unlock()
	if m.imports == nil {
		m.imports = map[string]*sync.Mutex{}
	}
	l := m.imports[name]
	if l == nil {
		l = new(sync.Mutex)
		m.imports[name] = l
	}
	return l
}

// grantPattern returns a database name for GRANT. There _ matches any
// character: escaped, an account gets shop_db only, and not also shopxdb.
func grantPattern(name string) string { return strings.ReplaceAll(name, "_", `\_`) }

// createImportUser creates the account an import runs as, with every
// privilege on the database name and nothing else, and returns the path of
// the server's socket. An account left by an import that crashed, or kept by
// dropImportUser, gets a new password and is unlocked.
func (m *Manager) createImportUser(ctx context.Context, name, user, password string) (string, error) {
	out, err := m.sql(ctx, fmt.Sprintf(`CREATE USER IF NOT EXISTS '%[1]s'@'localhost' IDENTIFIED BY '%[2]s';
ALTER USER '%[1]s'@'localhost' IDENTIFIED BY '%[2]s';
GRANT ALL PRIVILEGES ON `+"`%[3]s`"+`.* TO '%[1]s'@'localhost';
SELECT @@socket;`, user, password, grantPattern(name)))
	if err != nil {
		return "", err
	}
	// MariaDB before 10.4 can't lock accounts, so nothing to undo there.
	_, _ = m.sql(ctx, fmt.Sprintf("ALTER USER '%s'@'localhost' ACCOUNT UNLOCK;", user))
	// Creating a trigger, view or routine for another DEFINER needs one of
	// these (see importDefinerGrants); the first that works is enough.
	var first error
	for _, q := range m.importDefinerGrants(user) {
		_, err := m.sql(ctx, q)
		if err == nil {
			return strings.TrimSpace(out), nil
		}
		if first == nil {
			first = err
		}
	}
	return "", first
}

// dropImportUser removes the account after an import. Triggers, views,
// routines and events that the dump created without a DEFINER clause run
// as the account, though, and would fail without it. Then the account is
// kept, with the privileges on its database only and locked, so nobody can
// log in with it; the next import drops it when nothing uses it any more,
// and so does deleting the database.
func (m *Manager) dropImportUser(ctx context.Context, name, user string) {
	out, err := m.sql(ctx, fmt.Sprintf(`SELECT
 (SELECT COUNT(*) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA = '%[1]s' AND DEFINER = '%[2]s@localhost') +
 (SELECT COUNT(*) FROM information_schema.VIEWS WHERE TABLE_SCHEMA = '%[1]s' AND DEFINER = '%[2]s@localhost') +
 (SELECT COUNT(*) FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA = '%[1]s' AND DEFINER = '%[2]s@localhost') +
 (SELECT COUNT(*) FROM information_schema.EVENTS WHERE EVENT_SCHEMA = '%[1]s' AND DEFINER = '%[2]s@localhost');`, name, user))
	if err == nil && strings.TrimSpace(out) == "0" {
		if _, err := m.sql(ctx, fmt.Sprintf("DROP USER IF EXISTS '%s'@'localhost';", user)); err != nil {
			m.log.Warn("drop the temporary import user", "user", user, "err", err)
		}
		return
	}
	// In use, or unknown: keep the account, without what only the import
	// needed.
	if _, err := m.sql(ctx, fmt.Sprintf("REVOKE ALL PRIVILEGES, GRANT OPTION FROM '%[1]s'@'localhost';\n"+
		"GRANT ALL PRIVILEGES ON `%[2]s`.* TO '%[1]s'@'localhost';", user, grantPattern(name))); err != nil {
		m.log.Warn("take the privileges of the import user back", "user", user, "err", err)
		return
	}
	_, _ = m.sql(ctx, fmt.Sprintf("ALTER USER '%s'@'localhost' ACCOUNT LOCK;", user))
	m.log.Info("kept the import user, as objects in the database run as it", "user", user, "db", name)
}

// writeClientConf writes a client option file with the import account's
// credentials, readable by root only, and returns its path. The password
// is never on the command line or in the environment, where other users of
// the server could read it.
func writeClientConf(user, password, socket string) (string, error) {
	f, err := os.CreateTemp("", "nefo-import-*.cnf") // mode 0600
	if err != nil {
		return "", err
	}
	s := fmt.Sprintf("[client]\nuser=%s\npassword=%s\n", user, password)
	// With --defaults-file the client reads no other option file, such as
	// the client settings in /etc/mysql, so this one names the socket the
	// server really uses.
	if strings.HasPrefix(socket, "/") && !strings.ContainsAny(socket, "\r\n#\\") {
		s += "socket=" + socket + "\n"
	}
	_, err = f.WriteString(s)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

var (
	// ERROR 1044 (42000) at line 22: Access denied for user 'u'@'localhost' to database 'other'
	dbDeniedRe = regexp.MustCompile(`ERROR 1044 \([0-9A-Z]+\)[^\n]*? to database '([^'\n]+)'`)
	// ERROR 1142 (42000) at line 9: INSERT command denied to user 'u'@'localhost' for table `other`.`t`
	tableDeniedRe = regexp.MustCompile("ERROR 1142 \\([0-9A-Z]+\\)[^\\n]*? for table `([^`\\n]+)`\\.")
)

// OtherDBError means a dump uses a database other than the one it is
// imported into. errors.As finds a *usererr.Error in it, for the API; a job
// log shows the English sentence.
type OtherDBError struct {
	DB   string // the other database
	Into string // the database the dump was imported into
	ue   *usererr.Error
}

func (e *OtherDBError) Error() string {
	return fmt.Sprintf("the SQL file uses another database, %s, but Nefo imports only into the chosen database (%s): "+
		"remove its CREATE DATABASE and USE lines (or export the database again without --databases), "+
		"and make sure no table name starts with another database's name", e.DB, e.Into)
}

func (e *OtherDBError) Unwrap() error { return e.ue }

// importError explains a failed import. The server refusing another
// database becomes an OtherDBError; any other error keeps the client's own
// message.
func importError(name string, err error) error {
	var re *run.Error
	if err == nil || !errors.As(err, &re) {
		return err
	}
	for _, rx := range []*regexp.Regexp{dbDeniedRe, tableDeniedRe} {
		if mm := rx.FindStringSubmatch(re.Output); mm != nil && mm[1] != name {
			return &OtherDBError{DB: mm[1], Into: name, ue: &usererr.Error{Code: "import_other_db", Detail: mm[1]}}
		}
	}
	return err
}

// Reset empties a database before a backup is restored into it: the
// database is dropped and created again, and its user keeps access.
func (m *Manager) Reset(ctx context.Context, name string) error {
	if err := m.ready(ctx); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	d, err := m.store.DatabaseByName(ctx, name)
	if err != nil {
		return err
	}
	if validate.DBName(d.Name) != nil || validate.DBName(d.Username) != nil {
		return fmt.Errorf("invalid database name %q", d.Name)
	}
	_, err = m.sql(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS `%[1]s`;\n"+
		"CREATE DATABASE `%[1]s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;\n"+
		"GRANT ALL PRIVILEGES ON `%[1]s`.* TO '%[2]s'@'localhost';", d.Name, d.Username))
	return err
}
