package database

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/nefocp/nefo/internal/store"
)

// engineRunner plays a whole database server for the upgrade and switch
// tests: which engine and version are installed, its packages (dpkg-query,
// apt-get), its SQL answers, its dump and import programs, cp -a, and a
// program that fails.
type engineRunner struct {
	mu       sync.Mutex
	t        *testing.T
	m        *Manager
	engine   Engine
	version  string
	pkgs     map[string]string // installed package -> version
	sql      []string
	apt      []string
	imported map[string]string // database -> what its import read
	failRun  string            // a program (Stream or Pipe) that fails
	failSQL  string            // SQL containing this fails
}

var (
	mariaPkgs1011 = map[string]string{"mariadb-server": "1:10.11.14-0ubuntu0.24.04.1", "mariadb-client": "1:10.11.14-0ubuntu0.24.04.1",
		"mariadb-server-core": "1:10.11.14-0ubuntu0.24.04.1", "mariadb-common": "1:10.11.14-0ubuntu0.24.04.1", "mysql-common": "5.8+1.1.0build1"}
	mariaPkgs114 = map[string]string{"mariadb-server": "1:11.4.14+maria~ubu2404", "mariadb-client": "1:11.4.14+maria~ubu2404",
		"mariadb-server-core": "1:11.4.14+maria~ubu2404", "mariadb-common": "1:11.4.14+maria~ubu2404", "mysql-common": "1:11.4.14+maria~ubu2404"}
	mysqlPkgs = map[string]string{"mysql-community-server": "8.4.7-1ubuntu24.04", "mysql-community-client": "8.4.7-1ubuntu24.04",
		"mysql-community-server-core": "8.4.7-1ubuntu24.04", "mysql-common": "8.4.7-1ubuntu24.04"}
)

func (f *engineRunner) setInstalled(e Engine, version string, pkgs map[string]string) {
	f.engine, f.version = e, version
	f.pkgs = map[string]string{}
	for k, v := range pkgs {
		f.pkgs[k] = v
	}
}

func (f *engineRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch name {
	case "mariadbd":
		if f.engine == MariaDB {
			return "mariadbd  Ver " + f.version + "-MariaDB for debian-linux-gnu", nil
		}
		return "", errors.New("not found")
	case "mysqld":
		if f.engine == MySQL {
			return "/usr/sbin/mysqld  Ver " + f.version + " for Linux on x86_64", nil
		}
		return "", errors.New("not found")
	case "systemctl":
		return "active\n", nil
	case "cp":
		from, to := args[len(args)-2], args[len(args)-1]
		if err := os.CopyFS(to, os.DirFS(from)); err != nil {
			return err.Error(), err
		}
		return "", nil
	case "dpkg-query":
		var lines []string
		names := make([]string, 0, len(f.pkgs))
		for n := range f.pkgs {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			for _, p := range args[3:] {
				if ok, _ := path.Match(p, n); ok {
					if strings.Contains(args[2], "${Version}") {
						lines = append(lines, n+"\t"+f.pkgs[n]+"\tinstalled")
					} else {
						lines = append(lines, n+"\tinstalled")
					}
					break
				}
			}
		}
		return strings.Join(lines, "\n") + "\n", nil
	case "apt-cache":
		return "mariadb-server:\n  Candidate: 1:11.4.14+maria~ubu2404\n", nil
	case "pgrep":
		return "", errors.New("exit status 1")
	}
	return "", nil
}

func (f *engineRunner) Stream(ctx context.Context, w io.Writer, name string, args ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if name == f.failRun {
		return fmt.Errorf("%s: exit status 3", name)
	}
	if name != "/bin/sh" || len(args) <= 11 {
		return nil
	}
	a := args[11:]
	f.apt = append(f.apt, strings.Join(a, " "))
	switch {
	case a[0] == "purge":
		for _, p := range a[1:] {
			delete(f.pkgs, p)
		}
		if _, ok := f.pkgs["mariadb-server"]; !ok && f.engine == MariaDB {
			f.engine = ""
		}
		if _, ok := f.pkgs["mysql-community-server"]; !ok && f.engine == MySQL {
			f.engine = ""
		}
	case a[0] == "install" && !slices.Contains(a, "--download-only"):
		joined := strings.Join(a, " ")
		switch {
		case strings.Contains(joined, "mariadb-server=1:10.11"), strings.Contains(joined, "mariadb-server mariadb-client") && f.m.MariaDBRepoSeries() == "":
			f.setInstalled(MariaDB, "10.11.14", mariaPkgs1011)
		case strings.Contains(joined, "mariadb-server mariadb-client"):
			f.setInstalled(MariaDB, "11.4.14", mariaPkgs114)
		case strings.Contains(joined, "mysql-community-server"):
			f.setInstalled(MySQL, "8.4.7", mysqlPkgs)
			os.MkdirAll(f.m.dataDirPath(), 0o755)
			os.WriteFile(filepath.Join(f.m.dataDirPath(), "mysql.ibd"), []byte("mysql"), 0o644)
		}
	}
	return nil
}

func (f *engineRunner) Input(ctx context.Context, stdin []byte, name string, args ...string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := string(stdin)
	if name == "debconf-set-selections" {
		return "", nil
	}
	f.sql = append(f.sql, s)
	if f.failSQL != "" && strings.Contains(s, f.failSQL) {
		return "ERROR 1064", errors.New("exit status 1")
	}
	switch {
	case strings.Contains(s, "SHOW DATABASES"):
		return "information_schema\nmysql\nshop\n", nil
	case strings.Contains(s, "information_schema.SCHEMATA WHERE SCHEMA_NAME IN"):
		return hexOut([]string{"shop", "utf8mb4", "utf8mb4_unicode_ci"}), nil
	case strings.Contains(s, "SUM(DATA_LENGTH"):
		return hexOut([]string{"shop", "16384"}), nil
	case strings.Contains(s, "FROM information_schema.TABLES") && strings.Contains(s, "TABLE_TYPE) FROM"):
		return hexOut([]string{"shop", "items", "BASE TABLE"}), nil
	case strings.Contains(s, "UNION ALL"), strings.Contains(s, "COUNT(*))) FROM `shop`.`items`"):
		return hexOut([]string{"0", "42"}), nil
	case strings.Contains(s, "FROM mysql.user"):
		return hexOut([]string{"shop", "localhost", "mysql_native_password", "*" + strings.Repeat("A", 40)}), nil
	case strings.Contains(s, "SHOW CREATE USER"):
		return "CREATE USER `shop`@`localhost` IDENTIFIED BY PASSWORD '*AAAA'\n--nefo-grants--\n" +
			"GRANT USAGE ON *.* TO `shop`@`localhost` IDENTIFIED BY PASSWORD '*AAAA'\nGRANT ALL PRIVILEGES ON `shop`.* TO `shop`@`localhost`\n", nil
	case strings.Contains(s, "@@socket"):
		return "/run/mysqld/mysqld.sock\n", nil
	case strings.Contains(s, "CURRENT_USER()"):
		return "root@localhost\n", nil
	case strings.Contains(s, "information_schema.SCHEMATA WHERE"):
		return "0\n", nil
	}
	return "", nil
}

func (f *engineRunner) Pipe(ctx context.Context, stdin io.Reader, stdout io.Writer, name string, args ...string) error {
	if name == f.failRun {
		if stdin != nil {
			io.Copy(io.Discard, stdin)
		}
		return fmt.Errorf("%s: ERROR 1064 at line 3", name)
	}
	switch name {
	case "mariadb-dump", "mysqldump":
		_, err := io.WriteString(stdout, "-- dump of "+args[len(args)-1]+" by "+name+"\nCREATE TABLE items (id int) COLLATE=utf8mb4_unicode_ci;\n")
		return err
	case "mariadb", "mysql":
		b, err := io.ReadAll(stdin)
		f.mu.Lock()
		f.imported[args[len(args)-1]] = string(b)
		f.mu.Unlock()
		return err
	}
	return nil
}

func (f *engineRunner) aptHas(prefix string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, a := range f.apt {
		if strings.HasPrefix(a, prefix) {
			return true
		}
	}
	return false
}

func (f *engineRunner) sqlHas(s string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, q := range f.sql {
		if strings.Contains(q, s) {
			return true
		}
	}
	return false
}

// newFlowTest is a server with MariaDB 10.11 of the distribution and one
// database, shop, whose data directory holds a marker file.
func newFlowTest(t *testing.T) (*Manager, *engineRunner) {
	t.Helper()
	m, _, dir := newMariaTest(t, "")
	f := &engineRunner{t: t, m: m, imported: map[string]string{}}
	f.setInstalled(MariaDB, "10.11.14", mariaPkgs1011)
	m.run = f
	m.detect = func() Engine { f.mu.Lock(); defer f.mu.Unlock(); return f.engine }
	os.MkdirAll(m.dataDir, 0o755)
	os.WriteFile(filepath.Join(m.dataDir, "aria_log_control"), []byte("maria"), 0o644)
	os.WriteFile(filepath.Join(m.dataDir, "ibdata1"), []byte("the data from before"), 0o644)
	os.WriteFile(filepath.Join(m.etcDir, "my.cnf"), []byte("[mysqld]\n"), 0o644)
	m.slowConf = ""
	m.remoteConf = ""
	_ = dir
	return m, f
}

func dataMarker(m *Manager) string {
	b, _ := os.ReadFile(filepath.Join(m.dataDirPath(), "ibdata1"))
	return string(b)
}

func leftovers(t *testing.T, m *Manager) []string {
	t.Helper()
	list, _ := filepath.Glob(m.dataDirPath() + ".*")
	return list
}

func onlyDump(t *testing.T, m *Manager) Dump {
	t.Helper()
	dumps, err := m.ListDumps()
	if err != nil || len(dumps) != 1 {
		t.Fatalf("dumps %+v %v", dumps, err)
	}
	return dumps[0]
}

func TestUpgradeMariaDB(t *testing.T) {
	m, f := newFlowTest(t)
	ctx := context.Background()
	if err := m.UpgradeMariaDB(ctx, "11.4", &nopLog{}); err != nil {
		t.Fatal(err)
	}
	if st := m.Status(ctx); st.Version != "11.4.14" || st.Source != "mariadb" {
		t.Errorf("status %+v", st)
	}
	if dataMarker(m) != "the data from before" {
		t.Error("the data directory changed")
	}
	if l := leftovers(t, m); len(l) != 0 {
		t.Errorf("copies left: %v", l)
	}
	if fileExists(filepath.Join(m.confDir(MariaDB), readOnlyConfName)) {
		t.Error("still read-only")
	}
	if !f.sqlHas("SET GLOBAL read_only = ON;") || !f.sqlHas("SET GLOBAL read_only = OFF;") {
		t.Error("read-only not set and lifted")
	}
	for _, want := range []string{"update", "install --download-only --no-install-recommends mariadb-server mariadb-client",
		"install --download-only --reinstall --allow-downgrades --no-install-recommends mariadb-client=1:10.11.14-0ubuntu0.24.04.1",
		"install --no-install-recommends mariadb-server mariadb-client"} {
		if !f.aptHas(want) {
			t.Errorf("no apt %q in %q", want, f.apt)
		}
	}
	d := onlyDump(t, m)
	if d.Kind != "upgrade" || d.Result != "done" || d.From != "MariaDB 10.11.14" || d.To != "MariaDB 11.4" || !slices.Equal(d.Databases, []string{"shop"}) {
		t.Errorf("dump %+v", d)
	}
	if fileExists(filepath.Join(d.Path, "etc-mysql")) {
		t.Error("the configuration copy stayed")
	}
	gz, closeIt, err := gunzipFile(filepath.Join(d.Path, "001.sql.gz"))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(gz)
	closeIt()
	if !strings.Contains(string(b), "dump of shop by mariadb-dump") {
		t.Errorf("dump: %q", b)
	}
	// Then: the same series again, and back to an older one.
	if _, err := m.CheckUpgrade(ctx, "11.4"); errCode(err) != "mariadb_upgrade_same:11.4" {
		t.Errorf("same: %v", err)
	}
	f.version = "11.8.10"
	if _, err := m.CheckUpgrade(ctx, "11.4"); errCode(err) != "mariadb_downgrade:11.8" {
		t.Errorf("downgrade: %v", err)
	}
}

func TestUpgradeMariaDBGoesBack(t *testing.T) {
	m, f := newFlowTest(t)
	f.failRun = "mariadb-upgrade"
	ctx := context.Background()
	err := m.UpgradeMariaDB(ctx, "11.4", &nopLog{})
	if err == nil || !strings.Contains(err.Error(), "MariaDB 10.11.14 is back as it was") {
		t.Fatalf("err = %v", err)
	}
	if st := m.Status(ctx); st.Version != "10.11.14" || st.Source != "distribution" {
		t.Errorf("status %+v", st)
	}
	if dataMarker(m) != "the data from before" {
		t.Error("the data directory is not the one from before")
	}
	if l := leftovers(t, m); len(l) != 0 {
		t.Errorf("left: %v", l)
	}
	if fileExists(m.mariaList) {
		t.Error("MariaDB's repository stayed")
	}
	if !fileExists(filepath.Join(m.etcDir, "my.cnf")) || fileExists(filepath.Join(m.confDir(MariaDB), readOnlyConfName)) {
		t.Error("the configuration from before was not restored")
	}
	if !f.aptHas("install --no-install-recommends --allow-downgrades mariadb-client=1:10.11.14") {
		t.Errorf("not installed again in the old versions: %q", f.apt)
	}
	if d := onlyDump(t, m); d.Result != "rolled_back" {
		t.Errorf("dump %+v", d)
	}
}

func TestSwitchToMySQLAndBack(t *testing.T) {
	m, f := newFlowTest(t)
	ctx := context.Background()
	if err := m.store.CreateDatabase(ctx, &store.Database{Name: "shop", Username: "shop", Password: "Shop-pass-1"}); err != nil {
		t.Fatal(err)
	}
	var fixedFor []Engine
	fix := func(r io.Reader, to Engine) io.Reader { fixedFor = append(fixedFor, to); return r }
	if err := m.Switch(ctx, MySQL, fix, &nopLog{}); err != nil {
		t.Fatal(err)
	}
	if st := m.Status(ctx); st.Engine != MySQL || st.Version != "8.4.7" {
		t.Fatalf("status %+v", st)
	}
	if l := leftovers(t, m); len(l) != 0 {
		t.Errorf("the old data directory stayed: %v", l)
	}
	if !fileExists(filepath.Join(m.dataDirPath(), "mysql.ibd")) {
		t.Error("not MySQL's data directory")
	}
	imp := f.imported["shop"]
	if !strings.HasPrefix(imp, "ALTER DATABASE CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;\n-- dump of shop by mariadb-dump") {
		t.Errorf("import: %q", imp)
	}
	if !slices.Equal(fixedFor, []Engine{MySQL}) {
		t.Errorf("fixed for %v", fixedFor)
	}
	if !f.sqlHas("CREATE USER 'shop'@'localhost' IDENTIFIED BY 'Shop-pass-1';") || !f.sqlHas("GRANT ALL PRIVILEGES ON `shop`.* TO `shop`@`localhost`;") {
		t.Errorf("account not carried: %q", f.sql)
	}
	if f.sqlHas("IDENTIFIED BY PASSWORD '*AAAA'") {
		t.Error("MariaDB's hash went to MySQL")
	}
	if fileExists(filepath.Join(m.confDir(MySQL), readOnlyConfName)) || !f.sqlHas("SET GLOBAL read_only = OFF;") {
		t.Error("MySQL stayed read-only")
	}
	if !fileExists(m.repo.list) {
		t.Error("Oracle's repository is missing")
	}
	d := onlyDump(t, m)
	if d.Kind != "switch" || d.Result != "done" || d.To != "MySQL 8.4" {
		t.Errorf("dump %+v", d)
	}

	// And back: MySQL's dumps go into the distribution's MariaDB.
	f.imported = map[string]string{}
	if err := m.Switch(ctx, MariaDB, fix, &nopLog{}); err != nil {
		t.Fatal(err)
	}
	if st := m.Status(ctx); st.Engine != MariaDB || st.Version != "10.11.14" {
		t.Fatalf("status %+v", st)
	}
	if !strings.Contains(f.imported["shop"], "by mysqldump") || !slices.Equal(fixedFor, []Engine{MySQL, MariaDB}) {
		t.Errorf("import back: %q %v", f.imported["shop"], fixedFor)
	}
	if fileExists(m.repo.list) || fileExists(m.repo.keyring) {
		t.Error("Oracle's repository stayed after leaving MySQL")
	}
}

func TestSwitchGoesBack(t *testing.T) {
	m, f := newFlowTest(t)
	ctx := context.Background()
	f.failRun = "mysql" // the import fails
	err := m.Switch(ctx, MySQL, nil, &nopLog{})
	if err == nil || !strings.Contains(err.Error(), "MariaDB 10.11.14 is back as it was") {
		t.Fatalf("err = %v", err)
	}
	if st := m.Status(ctx); st.Engine != MariaDB || st.Version != "10.11.14" {
		t.Fatalf("status %+v", st)
	}
	if dataMarker(m) != "the data from before" {
		t.Error("the data directory is not MariaDB's from before")
	}
	if l := leftovers(t, m); len(l) != 0 {
		t.Errorf("left: %v", l)
	}
	if fileExists(m.repo.list) || fileExists(m.repo.keyring) {
		t.Error("Oracle's repository stayed")
	}
	if !fileExists(filepath.Join(m.etcDir, "my.cnf")) {
		t.Error("/etc/mysql not restored")
	}
	if !f.aptHas("install --no-install-recommends --allow-downgrades mariadb-client=1:10.11.14") {
		t.Errorf("MariaDB not installed again: %q", f.apt)
	}
	if d := onlyDump(t, m); d.Result != "rolled_back" {
		t.Errorf("dump %+v", d)
	}
	// A failure before the server stops changes nothing but read-only.
	m2, f2 := newFlowTest(t)
	f2.failRun = "mariadb-dump"
	if err := m2.Switch(ctx, MySQL, nil, &nopLog{}); err == nil {
		t.Fatal("no error")
	}
	if dataMarker(m2) != "the data from before" || !f2.sqlHas("SET GLOBAL read_only = OFF;") || f2.aptHas("purge") {
		t.Error("a failed export touched the server")
	}
}
