package database

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/pgpkey"
	"github.com/nefocp/nefo/internal/security"
	"github.com/nefocp/nefo/internal/store"
)

// mysqlRunner is fakeRunner playing MySQL 8.4: it answers mysqld
// --version, and records the programs SQL goes to and the apt runs.
type mysqlRunner struct {
	*fakeRunner
	inputs  []string // program names given SQL or other stdin
	apt     []string // apt-get arguments after the fixed options
	onApt   func(args string)
	version string
}

func (r *mysqlRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	if name == "mysqld" {
		return r.version, nil
	}
	return r.fakeRunner.Output(ctx, name, args...)
}

func (r *mysqlRunner) Input(ctx context.Context, stdin []byte, name string, args ...string) (string, error) {
	r.inputs = append(r.inputs, name)
	return r.fakeRunner.Input(ctx, stdin, name, args...)
}

func (r *mysqlRunner) Stream(ctx context.Context, w io.Writer, name string, args ...string) error {
	if len(args) > 11 {
		a := strings.Join(args[11:], " ")
		r.apt = append(r.apt, a)
		if r.onApt != nil {
			r.onApt(a)
		}
	}
	return nil
}

// keyServer serves the files of testdata under their names.
func keyServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := os.ReadFile(filepath.Join("testdata", filepath.Base(r.URL.Path)))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Write(b)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newMySQLTest returns a manager on a server whose installed engine is
// installed ("" for none), with every path in a temporary folder.
func newMySQLTest(t *testing.T, installed Engine) (*Manager, *mysqlRunner, string) {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	f := &mysqlRunner{fakeRunner: &fakeRunner{answer: map[string]string{"information_schema.SCHEMATA WHERE": "0\n"}},
		version: "/usr/sbin/mysqld  Ver 8.4.13 for Linux on x86_64 (MySQL Community Server - GPL)"}
	m := New(f, st, slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.supported = func() bool { return true }
	m.detect = func() Engine { return installed }
	dir := t.TempDir()
	m.etcDir = filepath.Join(dir, "etc/mysql")
	os.MkdirAll(filepath.Join(m.etcDir, "mysql.conf.d"), 0o755)
	m.dataDir = filepath.Join(dir, "var/lib/mysql")
	m.slowLog = filepath.Join(dir, "mysql-slow.log")
	m.mariaKeyring = filepath.Join(dir, "keyrings/panel-mariadb.gpg")
	m.mariaList = filepath.Join(dir, "sources.list.d/panel-mariadb.list")
	m.dumpsDir = filepath.Join(dir, "db-server")
	srv := keyServer(t)
	m.repo = mysqlRepo{
		keyURLs: []string{srv.URL + "/RPM-GPG-KEY-mysql-2025"},
		keyring: filepath.Join(dir, "keyrings/panel-mysql.gpg"),
		list:    filepath.Join(dir, "sources.list.d/panel-mysql.list"),
		osRelease: func() (string, string) {
			return "ubuntu", "noble"
		},
		arch:   func() string { return "amd64" },
		client: srv.Client(),
	}
	return m, f, dir
}

func TestMySQLTuning(t *testing.T) {
	for _, c := range []struct {
		mem          int64
		pool, ps     string
		conns, cache string
	}{
		{1 << 30, "128M", "OFF", "75", "400"},
		{2 << 30, "256M", "OFF", "100", "600"},
		{3800 << 20, "512M", "ON", "150", "1000"}, // a "4 GB" server
		{16 << 30, "1024M", "ON", "200", "2000"},
	} {
		got := mysqlTuning(c.mem)
		for _, want := range []string{
			"[mysqld]\n", "bind-address = 127.0.0.1\n", "mysqlx = OFF\n", "disable_log_bin\n",
			"skip-name-resolve\n", "host_cache_size = 0\n",
			"innodb_buffer_pool_size = " + c.pool + "\n", "performance_schema = " + c.ps + "\n",
			"max_connections = " + c.conns + "\n", "table_open_cache = " + c.cache + "\n",
			"collation-server = utf8mb4_unicode_ci\n",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("mem %d: missing %q in\n%s", c.mem, want, got)
			}
		}
	}
	// Under 3 GB the performance schema is off even at the threshold's edge.
	if performanceSchema(3<<30) || !performanceSchema(4<<30) {
		t.Error("performance schema threshold")
	}
	if MemoryMBFor(MySQL) <= MemoryMBFor(MariaDB) {
		t.Error("MySQL estimated lighter than MariaDB")
	}
}

func TestMySQLAvailable(t *testing.T) {
	m, _, _ := newMySQLTest(t, "")
	for _, c := range []struct{ id, codename, arch, want string }{
		{"ubuntu", "jammy", "amd64", ""},
		{"ubuntu", "noble", "amd64", ""},
		{"ubuntu", "resolute", "amd64", ""},
		{"debian", "bookworm", "amd64", ""},
		{"debian", "trixie", "amd64", ""},
		{"ubuntu", "noble", "arm64", "mysql_unsupported_arch:arm64"},
		{"ubuntu", "focal", "amd64", "mysql_unsupported_os:ubuntu focal"},
		{"debian", "noble", "amd64", "mysql_unsupported_os:debian noble"},
		{"", "", "amd64", "mysql_unsupported_os:?"},
	} {
		m.repo.osRelease = func() (string, string) { return c.id, c.codename }
		m.repo.arch = func() string { return c.arch }
		if got := errCode(m.MySQLAvailable()); got != c.want {
			t.Errorf("%s %s %s: %q, want %q", c.id, c.codename, c.arch, got, c.want)
		}
	}
	// Refused before anything is downloaded or written.
	m.repo.arch = func() string { return "arm64" }
	m.repo.osRelease = func() (string, string) { return "ubuntu", "noble" }
	if err := m.InstallMySQL(context.Background(), &nopLog{}); errCode(err) != "mysql_unsupported_arch:arm64" {
		t.Fatalf("install on arm64: %v", err)
	}
	if fileExists(m.repo.list) {
		t.Fatal("repository added on arm64")
	}
}

func TestInstallMySQL(t *testing.T) {
	m, f, _ := newMySQLTest(t, "")
	f.answer["CURRENT_USER()"] = "root@localhost\n"
	conf := filepath.Join(m.etcDir, "mysql.conf.d", "90-panel.cnf")
	os.RemoveAll(filepath.Dir(conf)) // the package creates it; the panel's file comes first
	installed := Engine("")
	m.detect = func() Engine { return installed }
	f.onApt = func(args string) {
		if strings.HasPrefix(args, "install") {
			if !fileExists(conf) {
				t.Error("MySQL installed before its settings were written")
			}
			installed = MySQL
		}
	}
	if err := m.InstallMySQL(context.Background(), &nopLog{}); err != nil {
		t.Fatal(err)
	}
	list, _ := os.ReadFile(m.repo.list)
	wantList := "deb [arch=amd64 signed-by=" + m.repo.keyring + "] https://repo.mysql.com/apt/ubuntu/ noble mysql-8.4-lts\n"
	if !strings.HasSuffix(string(list), wantList) {
		t.Errorf("source list:\n%s\nwant it to end with:\n%s", list, wantList)
	}
	key, _ := os.ReadFile(m.repo.keyring)
	if fp, err := pgpkey.Fingerprint(key); err != nil || fp != mysqlKeyFingerprint {
		t.Errorf("keyring holds %s (%v)", fp, err)
	}
	if !slices.Equal(f.apt, []string{"update", "install --no-install-recommends mysql-community-server mysql-community-client"}) {
		t.Errorf("apt: %q", f.apt)
	}
	seed := f.sql[0]
	for _, want := range []string{
		"mysql-community-server mysql-community-server/root-pass password \n",
		"mysql-community-server mysql-community-server/re-root-pass password \n",
		"mysql-community-server mysql-community-server/remove-data-dir boolean false\n",
	} {
		if !strings.Contains(seed, want) {
			t.Errorf("debconf seed lacks %q:\n%s", want, seed)
		}
	}
	if f.inputs[0] != "debconf-set-selections" {
		t.Errorf("seeded with %q", f.inputs[0])
	}
	if b, _ := os.ReadFile(conf); !strings.Contains(string(b), "mysqlx = OFF") {
		t.Errorf("tuning:\n%s", b)
	}
	if !slices.Contains(f.cmds, "systemctl enable mysql") || !slices.Contains(f.cmds, "systemctl restart mysql") {
		t.Errorf("commands: %q", f.cmds)
	}
	for _, name := range f.inputs[1:] {
		if name != "mysql" {
			t.Errorf("SQL sent to %q", name)
		}
	}
	if last := f.sql[len(f.sql)-1]; !strings.Contains(last, "DROP DATABASE IF EXISTS test") {
		t.Errorf("last SQL: %q", last)
	}

	// root must come in through the socket.
	m, f, _ = newMySQLTest(t, "")
	f.failOn = "CURRENT_USER()"
	if err := m.InstallMySQL(context.Background(), &nopLog{}); err == nil || !strings.Contains(err.Error(), "socket") {
		t.Fatalf("root check: %v", err)
	}
}

func TestInstallMySQLRefusesOtherKey(t *testing.T) {
	m, f, _ := newMySQLTest(t, "")
	// Another key (or a key server answering with the wrong one) is refused.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		armored, _ := os.ReadFile(filepath.Join("testdata", "RPM-GPG-KEY-mysql-2025"))
		key, _ := pgpkey.Dearmor(armored)
		// A copy with one byte of the key material changed: its
		// fingerprint differs.
		key[20] ^= 0xff
		w.Write(key)
	}))
	defer srv.Close()
	m.repo.keyURLs = []string{srv.URL + "/key"}
	m.repo.client = srv.Client()
	err := m.InstallMySQL(context.Background(), &nopLog{})
	if err == nil || !strings.Contains(err.Error(), "not the expected "+mysqlKeyFingerprint) {
		t.Fatalf("err = %v", err)
	}
	if fileExists(m.repo.keyring) || fileExists(m.repo.list) || len(f.apt) != 0 {
		t.Fatalf("installed anyway: %q", f.apt)
	}
}

func TestEnginesExcludeEachOther(t *testing.T) {
	ctx := context.Background()
	// MariaDB is installed: no MySQL.
	m, f, _ := newMySQLTest(t, MariaDB)
	if err := m.InstallMySQL(ctx, &nopLog{}); errCode(err) != "db_engine_conflict:MariaDB" {
		t.Fatalf("MySQL over MariaDB: %v", err)
	}
	if fileExists(m.repo.list) || len(f.apt) != 0 {
		t.Fatal("repository added anyway")
	}
	// MySQL is installed: no MariaDB.
	m, f, _ = newMySQLTest(t, MySQL)
	if err := m.Install(ctx, &nopLog{}); errCode(err) != "db_engine_conflict:MySQL" {
		t.Fatalf("MariaDB over MySQL: %v", err)
	}
	if len(f.apt) != 0 {
		t.Fatalf("apt ran: %q", f.apt)
	}
	// The packages count too, when the program is missing.
	m, f, _ = newMySQLTest(t, "")
	dpkg := &dpkgRunner{mysqlRunner: f, out: "mariadb-server-core\tinstalled\n"}
	m.run = dpkg
	if err := m.InstallMySQL(ctx, &nopLog{}); errCode(err) != "db_engine_conflict:MariaDB" {
		t.Fatalf("MariaDB packages: %v", err)
	}

	// Data left by the other engine.
	m, _, _ = newMySQLTest(t, "")
	os.MkdirAll(filepath.Join(m.dataDir, "mysql"), 0o750)
	os.WriteFile(filepath.Join(m.dataDir, "aria_log_control"), nil, 0o640)
	if err := m.InstallMySQL(ctx, &nopLog{}); errCode(err) != "db_datadir_other:"+m.dataDir {
		t.Fatalf("MySQL over MariaDB's data: %v", err)
	}
	m, _, _ = newMySQLTest(t, "")
	os.MkdirAll(m.dataDir, 0o750)
	os.WriteFile(filepath.Join(m.dataDir, "mysql.ibd"), nil, 0o640)
	if err := m.Install(ctx, &nopLog{}); errCode(err) != "db_datadir_other:"+m.dataDir {
		t.Fatalf("MariaDB over MySQL's data: %v", err)
	}
	// MySQL's own data is used again, and an empty folder is fine.
	if err := m.checkInstall(ctx, MySQL); err != nil {
		t.Fatalf("MySQL data for MySQL: %v", err)
	}
	m, _, _ = newMySQLTest(t, "")
	os.MkdirAll(filepath.Join(m.dataDir, "lost+found"), 0o750)
	if err := m.checkInstall(ctx, MySQL); err != nil {
		t.Fatalf("empty data directory: %v", err)
	}
}

// dpkgRunner answers dpkg-query with out.
type dpkgRunner struct {
	*mysqlRunner
	out string
}

func (r *dpkgRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	if name == "dpkg-query" {
		return r.out, nil
	}
	return r.mysqlRunner.Output(ctx, name, args...)
}

func TestMySQLCommands(t *testing.T) {
	ctx := context.Background()
	m, f, _ := newMySQLTest(t, MySQL)
	st := m.Status(ctx)
	if !st.Installed || st.Engine != MySQL || st.Version != "8.4.13" || !st.Running {
		t.Fatalf("status %+v", st)
	}
	if m.Engine(ctx) != MySQL || m.AcceptsNativeHash(ctx) {
		t.Fatal("engine")
	}

	d, err := m.Create(ctx, "shop", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range f.inputs {
		if name != "mysql" {
			t.Fatalf("SQL went to %q", name)
		}
	}
	// No native hashes on MySQL.
	if _, err := m.CreateWithUser(ctx, NewDB{Name: "bob_wp", Hash: "*" + strings.Repeat("A", 40)}); errCode(err) != "native_hash_unsupported:bob_wp" {
		t.Fatalf("hash: %v", err)
	}

	// The size list asks for fresh statistics.
	f.sql = nil
	if _, err := m.List(ctx); err != nil {
		t.Fatal(err)
	}
	if len(f.sql) == 0 || !strings.HasPrefix(f.sql[0], "SET SESSION information_schema_stats_expiry = 0;\n") {
		t.Errorf("list SQL: %q", f.sql)
	}

	// Export with mysqldump.
	var dumped []string
	f.onPipe = func(ctx context.Context, stdin io.Reader, name string, args []string) error {
		dumped = append([]string{name}, args...)
		return nil
	}
	if err := m.Dump(ctx, "shop", io.Discard); err != nil {
		t.Fatal(err)
	}
	if dumped[0] != "mysqldump" || !slices.Contains(dumped, "--set-gtid-purged=OFF") || dumped[len(dumped)-1] != "shop" {
		t.Errorf("dump ran %q", dumped)
	}

	// Remote twins: no CREATE OR REPLACE USER on MySQL.
	fw := &fakeFirewall{installed: true, active: true}
	m.SetFirewall(fw)
	f.sql = nil
	if _, err := m.SetRemote(ctx, d.ID, []string{"203.0.113.5"}); err != nil {
		t.Fatal(err)
	}
	all := strings.Join(f.sql, "\n")
	if strings.Contains(all, "CREATE OR REPLACE") ||
		!strings.Contains(all, "DROP USER IF EXISTS 'shop'@'203.0.113.5';\nCREATE USER 'shop'@'203.0.113.5' IDENTIFIED BY '"+d.Password+"';") {
		t.Errorf("remote SQL:\n%s", all)
	}
	if got := fw.froms("MySQL shop"); !slices.Equal(got, []string{"203.0.113.5"}) {
		t.Errorf("firewall rules %+v", fw.rules)
	}
	if b, _ := os.ReadFile(m.remoteConfPath()); !strings.Contains(string(b), "bind-address = 0.0.0.0") ||
		filepath.Base(filepath.Dir(m.remoteConfPath())) != "mysql.conf.d" {
		t.Errorf("listen drop-in %s:\n%s", m.remoteConfPath(), b)
	}
	// A rule made while the server was MariaDB still counts as the database's.
	fw.rules = append(fw.rules, security.Rule{To: "3306/tcp", Action: "ALLOW", From: "198.51.100.9", Comment: "MariaDB shop"})
	if _, err := m.SetRemote(ctx, d.ID, nil); err != nil {
		t.Fatal(err)
	}
	if len(fw.rules) != 0 {
		t.Errorf("rules left: %+v", fw.rules)
	}
	f.cmds = nil
	if err := m.Restart(ctx, &nopLog{}); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(f.cmds, "systemctl restart mysql") {
		t.Errorf("restart ran %q", f.cmds)
	}

	// The slow query log goes to MySQL's file.
	f.sql = nil
	if err := m.SetSlowLog(ctx, true, 1); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.sql[0], "slow_query_log_file = '/var/log/mysql/mysql-slow.log'") {
		t.Errorf("slow log SQL: %q", f.sql[0])
	}
	if b, _ := os.ReadFile(filepath.Join(m.etcDir, "mysql.conf.d", "99-panel-slowlog.cnf")); !strings.Contains(string(b), "slow_query_log_file = /var/log/mysql/mysql-slow.log") {
		t.Errorf("slow log drop-in:\n%s", b)
	}
}

func TestMySQLImport(t *testing.T) {
	ctx := context.Background()
	m, f, _ := newMySQLTest(t, MySQL)
	call := fakeImport(f.fakeRunner, "")
	dump := "/*M!999999\\- enable the sandbox mode */\n-- MariaDB dump\nINSERT INTO t VALUES (1);\n"
	if err := m.Import(ctx, "shop_db", strings.NewReader(dump)); err != nil {
		t.Fatal(err)
	}
	if call.name != "mysql" || call.args[2] != "shop_db" {
		t.Errorf("ran %s %q", call.name, call.args)
	}
	if call.dump != "-- MariaDB dump\nINSERT INTO t VALUES (1);\n" {
		t.Errorf("sandbox line not skipped: %q", call.dump)
	}
	if !slices.Contains(f.sql, "GRANT SET_ANY_DEFINER, ALLOW_NONEXISTENT_DEFINER ON *.* TO 'nefo-import-shop_db'@'localhost';") {
		t.Errorf("definer grant: %q", f.sql)
	}
	for _, q := range f.sql {
		if strings.Contains(q, "SET USER") || strings.Contains(q, "SUPER") {
			t.Errorf("MariaDB grant on MySQL: %q", q)
		}
	}

	// MySQL user names stop at 32 characters.
	long := "a_rather_long_database_name_x123" // 32 characters
	other := "a_rather_long_database_name_y123"
	u1, u2 := m.importUser(long), m.importUser(other)
	if len(u1) > mysqlUserMax || len(u2) > mysqlUserMax || u1 == u2 || !strings.HasPrefix(u1, importUserPrefix) {
		t.Errorf("import users %q %q", u1, u2)
	}
	if m.importUser("shop") != "nefo-import-shop" {
		t.Errorf("short name: %q", m.importUser("shop"))
	}
	m.detect = func() Engine { return MariaDB }
	if m.importUser(long) != importUserPrefix+long {
		t.Error("MariaDB names cut")
	}
}

func TestSkipSandboxLine(t *testing.T) {
	for in, want := range map[string]string{
		"/*M!999999\\- enable the sandbox mode */\nSELECT 1;\n": "SELECT 1;\n",
		"/*!999999\\- enable the sandbox mode */\r\nSELECT 1;":  "SELECT 1;",
		"/*M!999999\\- enable the sandbox mode */":              "",
		"SELECT 1;\n/*M!999999\\- enable the sandbox mode */\n": "SELECT 1;\n/*M!999999\\- enable the sandbox mode */\n",
		"-- dump\n":               "-- dump\n",
		strings.Repeat("x", 9000): strings.Repeat("x", 9000),
		"":                        "",
	} {
		b, err := io.ReadAll(skipSandboxLine(strings.NewReader(in)))
		if err != nil || string(b) != want {
			t.Errorf("%.40q: got %.40q (%v)", in, b, err)
		}
	}
}

func TestRemoveMySQL(t *testing.T) {
	ctx := context.Background()
	m, _, _ := newMySQLTest(t, MySQL)
	conf := filepath.Join(m.etcDir, "mysql.conf.d")
	os.MkdirAll(conf, 0o755)
	for _, n := range []string{"90-panel.cnf", "99-panel-remote.cnf", "99-panel-slowlog.cnf", "mysqld.cnf"} {
		os.WriteFile(filepath.Join(conf, n), nil, 0o644)
	}
	os.MkdirAll(filepath.Dir(m.repo.list), 0o755)
	os.WriteFile(m.repo.list, nil, 0o644)
	os.MkdirAll(filepath.Join(m.dataDir, "mysql"), 0o750)
	rr := &removeRunner{running: true, databases: "information_schema\nmysql\nperformance_schema\nsys\n",
		installed: "mysql-community-server\tinstalled\nmysql-community-server-core\tinstalled\nmysql-community-client\tinstalled\nmysql-client\tinstalled\n",
		sim:       "Purg mysql-client [1]\nPurg mysql-community-client [1]\nPurg mysql-community-server [1]\nPurg mysql-community-server-core [1]\n"}
	m.run = &mysqlRemoveRunner{removeRunner: rr}
	// The other engine's item cannot remove it.
	if err := m.CheckRemoveEngine(ctx, MariaDB, false, ""); errCode(err) != "software_not_installed:MariaDB" {
		t.Fatalf("remove as MariaDB: %v", err)
	}
	if err := m.CheckRemoveEngine(ctx, MySQL, true, "mariadb"); errCode(err) != "software_confirm_mismatch:mysql" {
		t.Fatalf("confirm: %v", err)
	}
	rr.databases += "shop\n"
	if err := m.CheckRemoveEngine(ctx, MySQL, false, ""); errCode(err) != "mysql_has_databases:shop" {
		t.Fatalf("with a database: %v", err)
	}
	rr.databases = "information_schema\nmysql\nsys\n"
	if err := m.RemoveEngine(ctx, MySQL, false, "", &nopLog{}); err != nil {
		t.Fatal(err)
	}
	if len(rr.apt) != 1 || rr.apt[0] != "purge mysql-client mysql-community-client mysql-community-server mysql-community-server-core" {
		t.Fatalf("apt %q", rr.apt)
	}
	if !strings.Contains(strings.Join(rr.stdin, "\n"), "mysql-community-server mysql-community-server/remove-data-dir boolean false") {
		t.Errorf("debconf: %q", rr.stdin)
	}
	for _, n := range []string{"90-panel.cnf", "99-panel-remote.cnf", "99-panel-slowlog.cnf"} {
		if fileExists(filepath.Join(conf, n)) {
			t.Errorf("%s left behind", n)
		}
	}
	if !fileExists(filepath.Join(conf, "mysqld.cnf")) || !fileExists(filepath.Join(m.dataDir, "mysql")) {
		t.Error("removed what is not the panel's, or the data")
	}
	if fileExists(m.repo.list) {
		t.Error("repository kept")
	}
}

// mysqlRemoveRunner is removeRunner playing MySQL.
type mysqlRemoveRunner struct{ *removeRunner }

func (r *mysqlRemoveRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	if name == "mysqld" {
		return "/usr/sbin/mysqld  Ver 8.4.13 for Linux on x86_64 (MySQL Community Server - GPL)", nil
	}
	return r.removeRunner.Output(ctx, name, args...)
}

func TestRefreshMySQLKey(t *testing.T) {
	ctx := context.Background()
	m, _, _ := newMySQLTest(t, MySQL)
	// Not set up by the panel: nothing to do.
	if err := m.RefreshMySQLKey(ctx); err != nil {
		t.Fatal(err)
	}
	armored, _ := os.ReadFile(filepath.Join("testdata", "RPM-GPG-KEY-mysql-2025"))
	renewed, _ := pgpkey.Dearmor(armored)
	// The copy from before the renewal (expired on 2025-10-22).
	armoredOld, _ := os.ReadFile(filepath.Join("testdata", "RPM-GPG-KEY-mysql-2023"))
	old, err := pgpkey.Dearmor(armoredOld)
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Dir(m.repo.keyring), 0o755)
	os.MkdirAll(filepath.Dir(m.repo.list), 0o755)
	os.WriteFile(m.repo.list, []byte("deb ...\n"), 0o644)
	os.WriteFile(m.repo.keyring, old, 0o644)
	// Checked recently: left alone.
	if err := m.RefreshMySQLKey(ctx); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(m.repo.keyring); string(b) != string(old) {
		t.Fatal("refreshed before a week passed")
	}
	week := time.Now().Add(-8 * 24 * time.Hour)
	os.Chtimes(m.repo.keyring, week, week)
	if err := m.RefreshMySQLKey(ctx); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(m.repo.keyring); string(b) != string(renewed) {
		t.Fatal("not renewed")
	}
	// The renewed key is not replaced by an older copy, and the check
	// waits another week.
	m.repo.keyURLs = []string{m.repo.keyURLs[0], strings.Replace(m.repo.keyURLs[0], "2025", "2023", 1)}
	os.Chtimes(m.repo.keyring, week, week)
	if err := m.RefreshMySQLKey(ctx); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(m.repo.keyring); string(b) != string(renewed) {
		t.Fatal("replaced by an older copy")
	}
	if fi, _ := os.Stat(m.repo.keyring); time.Since(fi.ModTime()) > time.Minute {
		t.Errorf("checked, but the next check is not in a week: %v", fi.ModTime())
	}
	// Nothing to download: an error, and another try in a day.
	m.repo.keyURLs = []string{keyServer(t).URL + "/missing-key"}
	os.Chtimes(m.repo.keyring, week, week)
	if err := m.RefreshMySQLKey(ctx); err == nil {
		t.Fatal("a missing key gave no error")
	}
	if b, _ := os.ReadFile(m.repo.keyring); string(b) != string(renewed) {
		t.Fatal("keyring changed")
	}
	if fi, _ := os.Stat(m.repo.keyring); time.Since(fi.ModTime()) < 6*24*time.Hour-time.Hour {
		t.Errorf("next check not in a day: %v", fi.ModTime())
	}
}

func TestParseMySQLSlowTime(t *testing.T) {
	got, ok := parseSlowTime("2025-11-25T14:03:07.123456Z")
	if !ok || got.UTC().Format(time.RFC3339) != "2025-11-25T14:03:07Z" {
		t.Fatalf("%v %v", got, ok)
	}
	q := parseSlowLog(strings.NewReader(`/usr/sbin/mysqld, Version: 8.4.13 (MySQL Community Server - GPL). started with:
Tcp port: 3306  Unix socket: /var/run/mysqld/mysqld.sock
Time                 Id Command    Argument
# Time: 2025-11-25T14:03:07.123456Z
# User@Host: shop[shop] @ localhost []  Id:     8
# Query_time: 1.500321  Lock_time: 0.000002 Rows_sent: 1  Rows_examined: 3
use shop;
SET timestamp=1764079387;
SELECT SLEEP(1.5);
`), false, 10)
	if len(q) != 1 || q[0].User != "shop@localhost" || q[0].Database != "shop" || q[0].QueryTime != 1.500321 ||
		q[0].Query != "SELECT SLEEP(1.5)" || q[0].RowsExamined != 3 {
		t.Fatalf("%+v", q)
	}
}
