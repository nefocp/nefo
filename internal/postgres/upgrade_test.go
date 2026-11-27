package postgres

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// upgradeFake makes a test manager with PostgreSQL 16 running and fakes
// what an upgrade to 17 runs. fail makes pg_upgradecluster fail after it
// started the new cluster.
func upgradeFake(t *testing.T, fail bool) (*Manager, *fakeRunner) {
	return upgradeFakeWith(t, fail, false)
}

// upgradeFakeWith is upgradeFake; extra makes installing postgresql-17
// bring postgresql-19 along, as apt does when the old "postgresql"
// metapackage is still installed.
func upgradeFakeWith(t *testing.T, fail, extra bool) (*Manager, *fakeRunner) {
	t.Helper()
	m, f, _ := newTestManager(t)
	pgdgServer(t, m, filepath.Join("testdata", "ACCC4CF8.asc"))
	os.WriteFile(filepath.Join(m.mainDir("16"), "pg_hba.conf"), []byte(hbaConf+"# the administrator's own line\n"), 0o640)
	os.WriteFile(filepath.Join(m.mainDir("16"), "start.conf"), []byte("auto\n"), 0o644)
	// The panel's own files in the old cluster's folder (tuning, TLS).
	os.MkdirAll(filepath.Join(m.mainDir("16"), "conf.d"), 0o755)
	os.WriteFile(filepath.Join(m.mainDir("16"), "conf.d", confName), []byte("x\n"), 0o644)
	os.WriteFile(filepath.Join(m.mainDir("16"), panelCertName), []byte("x\n"), 0o644)
	os.WriteFile(filepath.Join(m.mainDir("16"), panelKeyName), []byte("x\n"), 0o600)
	installed := map[string]bool{"postgresql": true, "postgresql-16": true, "postgresql-client-16": true}
	write := func(p, s string) {
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(s), 0o644)
	}
	f.sql = func(stdin string) (string, error) {
		switch {
		case stdin == "": // pg_dumpall
			return strings.Repeat("-- PostgreSQL database cluster dump\n", 20), nil
		case strings.HasPrefix(stdin, "SELECT coalesce(sum(pg_database_size"):
			return "8388608\n", nil
		case strings.HasPrefix(stdin, "SELECT datname FROM pg_database WHERE datname <> 'template0'"):
			return "postgres\nshop\ntemplate1\n", nil
		case stdin == "SHOW listen_addresses;\n":
			return "localhost\n", nil
		case strings.HasPrefix(stdin, "SELECT setting FROM pg_file_settings"):
			return "localhost\n", nil
		}
		return "", nil
	}
	f.stream = func(name string, args []string) error {
		joined := strings.Join(args, " ")
		switch {
		case name == "/bin/sh" && strings.Contains(joined, " install ") && args[len(args)-1] == "postgresql-17":
			write(filepath.Join(m.libDir, "17", "bin", "postgres"), "")
			write(filepath.Join(m.mainDir("17"), "postgresql.conf"), "port = 5433\n") // the package's own cluster
			installed["postgresql-17"], installed["postgresql-client-17"] = true, true
			if extra {
				write(filepath.Join(m.libDir, "19", "bin", "postgres"), "")
				write(filepath.Join(m.mainDir("19"), "postgresql.conf"), "port = 5434\n")
				installed["postgresql-19"], installed["postgresql-client-19"] = true, true
			}
		case name == "/bin/sh" && strings.Contains(joined, " remove "):
			for _, a := range args {
				delete(installed, a)
			}
		case name == "pg_upgradecluster":
			if fileThere(m.mainDir("17")) {
				return errors.New("target cluster 17/main already exists")
			}
			write(filepath.Join(m.mainDir("17"), "postgresql.conf"), "port = 5432\n")
			write(filepath.Join(m.mainDir("17"), "pg_hba.conf"), "local all all peer\n")
			if fail {
				return errors.New("pg_upgrade: Your installation contains one of the reg* data types in user tables")
			}
			write(filepath.Join(m.mainDir("16"), "postgresql.conf"), "port = 5433\n")
			write(filepath.Join(m.mainDir("16"), "start.conf"), "manual\n")
		}
		return nil
	}
	f.output = func(name string, args []string) (string, error) {
		switch {
		case strings.HasSuffix(name, "/postgres") && len(args) == 1:
			return "postgres (PostgreSQL) 16.10 (Ubuntu 16.10-0ubuntu0.24.04.1)\n", nil
		case name == "systemctl" && args[0] == "is-active":
			return "active\n", nil
		case name == "pg_dropcluster":
			// Like the real one: its own files, then the folders if empty.
			dir := m.mainDir(args[len(args)-2])
			for _, n := range []string{"postgresql.conf", "pg_hba.conf", "pg_ident.conf", "start.conf"} {
				os.Remove(filepath.Join(dir, n))
			}
			os.Remove(dir)
			os.RemoveAll(filepath.Join(m.dataDir, args[len(args)-2], "main"))
		case name == "dpkg-query":
			var out []string
			for _, a := range args[3:] {
				if installed[a] {
					out = append(out, a+"\tinstalled")
				}
			}
			return strings.Join(out, "\n") + "\n", nil
		}
		return "", nil
	}
	return m, f
}

func ran(f *fakeRunner, s string) bool {
	return slices.ContainsFunc(f.all(), func(c call) bool { return strings.HasPrefix(c.String(), s) })
}

func TestUpgrade(t *testing.T) {
	m, f := upgradeFake(t, false)
	if err := m.Upgrade(context.Background(), "17", testLog{t}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"runuser -u postgres -- " + filepath.Join(m.libDir, "16", "bin", "pg_dumpall"),
		"pg_dropcluster --stop 17 main", // the package's empty cluster
		"pg_upgradecluster -v 17 -m upgrade 16 main",
		"systemctl restart postgresql@17-main",
		"pg_dropcluster --stop 16 main",
	} {
		if !ran(f, want) {
			t.Errorf("did not run %q", want)
		}
	}
	var removed [][]string
	metaGone, marked := -1, -1
	for i, c := range f.all() {
		if c.name == "/bin/sh" && slices.Contains(c.args, "remove") {
			removed = append(removed, c.args[slices.Index(c.args, "remove")+1:])
		}
		if c.name == "/bin/sh" && slices.Contains(c.args, "remove") && slices.Contains(c.args, "postgresql") && metaGone < 0 {
			metaGone = i
		}
		if c.String() == "apt-mark manual postgresql-16 postgresql-client-16" {
			marked = i
		}
		if c.name == "/bin/sh" && slices.Contains(c.args, "install") && (metaGone < 0 || marked < 0 || marked > metaGone) {
			t.Fatalf("postgresql-17 installed before 16 was marked manual (%d) and the metapackage removed (%d)", marked, metaGone)
		}
	}
	if len(removed) != 2 || !slices.Equal(removed[0], []string{"postgresql"}) || !slices.Equal(removed[1], []string{"postgresql-16", "postgresql-client-16"}) {
		t.Fatalf("removed %v", removed)
	}
	dumps, _ := filepath.Glob(filepath.Join(m.backupDir, "postgresql-16-before-17-*.sql.gz"))
	if len(dumps) != 1 {
		t.Fatalf("dumps: %v", dumps)
	}
	if hba := readFile(t, filepath.Join(m.mainDir("17"), "pg_hba.conf")); hba != hbaConf+"# the administrator's own line\n" {
		t.Fatalf("pg_hba.conf of the new cluster:\n%s", hba)
	}
	if !fileThere(filepath.Join(m.mainDir("17"), "conf.d", confName)) {
		t.Fatal("no tuning drop-in for 17")
	}
	if !m.hasPGDGRepo() {
		t.Fatal("the repository must stay for 17's updates")
	}
	if fileThere(filepath.Join(m.etcDir, "16")) {
		t.Fatal("the old version's folder stayed")
	}
}

func TestUpgradeRollsBack(t *testing.T) {
	m, f := upgradeFake(t, true)
	oldConf := readFile(t, filepath.Join(m.mainDir("16"), "postgresql.conf"))
	err := m.Upgrade(context.Background(), "17", testLog{t})
	if err == nil || !strings.Contains(err.Error(), "PostgreSQL 16 runs again") {
		t.Fatalf("%v", err)
	}
	if fileThere(m.mainDir("17")) {
		t.Fatal("the new cluster stayed")
	}
	if got := readFile(t, filepath.Join(m.mainDir("16"), "postgresql.conf")); got != oldConf {
		t.Fatalf("old settings not put back: %s", got)
	}
	for _, want := range []string{"pg_dropcluster --stop 17 main", "systemctl start postgresql@16-main"} {
		if !ran(f, want) {
			t.Errorf("did not run %q", want)
		}
	}
	if ran(f, "pg_dropcluster --stop 16 main") {
		t.Fatal("dropped the old cluster")
	}
	var removed []string
	for _, c := range f.all() {
		if c.name == "/bin/sh" && slices.Contains(c.args, "remove") {
			removed = append(removed, c.args[slices.Index(c.args, "remove")+1:]...)
		}
	}
	if !slices.Contains(removed, "postgresql-17") || slices.Contains(removed, "postgresql-16") {
		t.Fatalf("removed %v", removed)
	}
	if !m.hasPGDGRepo() {
		t.Fatal("the repository went, but postgresql-common and libpq5 come from it now")
	}
	if dumps, _ := filepath.Glob(filepath.Join(m.backupDir, "*.sql.gz")); len(dumps) != 1 {
		t.Fatalf("the dump must stay: %v", dumps)
	}
}

// A second new major that apt brings along is refused and removed too.
func TestUpgradeExtraMajor(t *testing.T) {
	m, f := upgradeFakeWith(t, false, true)
	err := m.Upgrade(context.Background(), "17", testLog{t})
	if err == nil || !strings.Contains(err.Error(), "apt also installed PostgreSQL 19") {
		t.Fatalf("%v", err)
	}
	for _, want := range []string{"pg_dropcluster --stop 19 main", "pg_dropcluster --stop 17 main", "systemctl start postgresql@16-main"} {
		if !ran(f, want) {
			t.Errorf("did not run %q", want)
		}
	}
	if ran(f, "pg_upgradecluster") {
		t.Fatal("upgraded anyway")
	}
	if fileThere(m.mainDir("19")) || fileThere(m.mainDir("17")) || !fileThere(m.mainDir("16")) {
		t.Fatal("clusters left as they should not be")
	}
}

func TestUpgradeRefused(t *testing.T) {
	ctx := context.Background()
	m, f := upgradeFake(t, false)
	if err := m.Upgrade(ctx, "16", testLog{t}); userCode(err) != "pg_upgrade_not_newer" {
		t.Fatalf("same major: %v", err)
	}
	if err := m.Upgrade(ctx, "21", testLog{t}); userCode(err) != "pg_version_unknown" {
		t.Fatalf("unknown: %v", err)
	}
	m.freeBytes = func(string) (uint64, error) { return 100 << 20, nil }
	if err := m.Upgrade(ctx, "17", testLog{t}); userCode(err) != "pg_upgrade_disk" {
		t.Fatalf("disk: %v", err)
	}
	prev := f.output
	f.output = func(name string, args []string) (string, error) {
		if name == "dpkg-query" && slices.Contains(args, "postgresql-16-*") {
			return "postgresql-16-postgis-3\tinstalled\npostgresql-16-jit\tinstalled\n", nil
		}
		return prev(name, args)
	}
	if err := m.Upgrade(ctx, "17", testLog{t}); userCode(err) != "pg_upgrade_extensions" {
		t.Fatalf("extensions: %v", err)
	}
	if v := m.Versions(ctx); v.UpgradeBlocked != "pg_upgrade_extensions" || v.UpgradeDetail != "postgresql-16-postgis-3" {
		t.Fatalf("versions: %+v", v)
	}
	if ran(f, "pg_upgradecluster") || ran(f, "runuser -u postgres -- "+filepath.Join(m.libDir, "16", "bin", "pg_dumpall")) {
		t.Fatal("started anyway")
	}
}

func TestUpgradeDumps(t *testing.T) {
	dir := t.TempDir()
	m := &Manager{backupDir: dir}
	if got := m.UpgradeDumps(); len(got) != 0 {
		t.Fatalf("empty folder: %v", got)
	}
	for _, n := range []string{"postgresql-15-before-17-20261127-101500.sql.gz", "postgresql-17-before-18-20261128-090000.sql.gz",
		"notes.txt", "postgresql-15-before-17-x.sql.gz"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("dump"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got := m.UpgradeDumps()
	if len(got) != 2 || got[0].From != "17" || got[0].To != "18" || got[1].From != "15" || got[1].Size != 4 {
		t.Fatalf("dumps %+v", got)
	}
	for _, bad := range []string{"../x", "notes.txt", "postgresql-15-before-17-x.sql.gz", "postgresql-16-before-17-20261127-101500.sql.gz"} {
		if err := m.DeleteUpgradeDump(bad); err == nil {
			t.Errorf("deleted %q", bad)
		}
	}
	if err := m.DeleteUpgradeDump(got[1].Name); err != nil {
		t.Fatal(err)
	}
	if left := m.UpgradeDumps(); len(left) != 1 || left[0].Name != got[0].Name {
		t.Fatalf("after delete: %+v", left)
	}
}
