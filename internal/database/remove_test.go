package database

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// removeRunner plays a MariaDB server (running or stopped) and apt.
type removeRunner struct {
	running   bool
	databases string // SHOW DATABASES output
	installed string // dpkg-query output
	sim       string // apt-get -s purge output
	apt       []string
	stdin     []string
}

func (f *removeRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	switch {
	case name == "mariadbd":
		return "mariadbd  Ver 10.11.14-MariaDB for debian-linux-gnu", nil
	case name == "systemctl" && args[0] != "is-active":
		return "", nil
	case name == "systemctl":
		if f.running {
			return "active\n", nil
		}
		return "inactive\n", errors.New("exit status 3")
	case name == "dpkg-query":
		return f.installed, nil
	case name == "apt-get" && len(args) > 2 && args[2] == "purge":
		return f.sim, nil
	}
	return "", nil
}

func (f *removeRunner) Stream(ctx context.Context, w io.Writer, name string, args ...string) error {
	if len(args) > 11 {
		f.apt = append(f.apt, strings.Join(args[11:], " "))
	}
	return nil
}

func (f *removeRunner) Input(ctx context.Context, stdin []byte, name string, args ...string) (string, error) {
	f.stdin = append(f.stdin, name+": "+string(stdin))
	if strings.Contains(string(stdin), "SHOW DATABASES") {
		return f.databases, nil
	}
	return "", nil
}

func (f *removeRunner) Pipe(context.Context, io.Reader, io.Writer, string, ...string) error {
	return nil
}

type nopLog struct{ strings.Builder }

func (l *nopLog) Logf(string, ...any) {}

func newRemoveTest(t *testing.T) (*Manager, *removeRunner, string) {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	f := &removeRunner{running: true}
	m := New(f, st, slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.supported = func() bool { return true }
	dir := t.TempDir()
	m.etcDir = filepath.Join(dir, "etc/mysql")
	m.mariaKeyring = filepath.Join(dir, "keyrings/panel-mariadb.gpg")
	m.mariaList = filepath.Join(dir, "sources.list.d/panel-mariadb.list")
	m.dumpsDir = filepath.Join(dir, "db-server")
	m.dataDir = filepath.Join(dir, "var/lib/mysql")
	m.remoteConf = filepath.Join(m.etcDir, "mariadb.conf.d", "99-panel-remote.cnf")
	m.slowConf = filepath.Join(m.etcDir, "mariadb.conf.d", "99-panel-slowlog.cnf")
	for _, d := range []string{"mysql", "sys", "performance_schema"} {
		os.MkdirAll(filepath.Join(m.dataDir, d), 0o750)
	}
	os.MkdirAll(filepath.Join(m.etcDir, "mariadb.conf.d"), 0o755)
	return m, f, dir
}

func errCode(err error) string {
	var ue *usererr.Error
	if errors.As(err, &ue) {
		return ue.Code + ":" + ue.Detail
	}
	if err != nil {
		return "error:" + err.Error()
	}
	return ""
}

func TestRemoveRefusedWhileDatabasesExist(t *testing.T) {
	m, f, _ := newRemoveTest(t)
	ctx := context.Background()
	// MariaDB itself is asked, so a database made by hand counts although
	// the panel's list is empty.
	f.databases = "information_schema\nmysql\nperformance_schema\nsys\ntest123\nwp_blog\n"
	if err := m.CheckRemove(ctx, false, ""); errCode(err) != "mariadb_has_databases:test123, wp_blog" {
		t.Fatalf("got %v", err)
	}
	if err := m.Remove(ctx, false, "", &nopLog{}); err == nil || len(f.apt) != 0 {
		t.Fatalf("removed anyway: %v %q", err, f.apt)
	}
	f.databases = "information_schema\nmysql\nperformance_schema\nsys\n"
	if err := m.CheckRemove(ctx, false, ""); err != nil {
		t.Fatalf("only system databases: %v", err)
	}
	if err := m.CheckRemove(ctx, true, "MariaDB?"); errCode(err) != "software_confirm_mismatch:mariadb" {
		t.Fatalf("confirm: %v", err)
	}
}

func TestRemoveWhileStoppedReadsDataDir(t *testing.T) {
	m, f, _ := newRemoveTest(t)
	ctx := context.Background()
	f.running = false
	os.MkdirAll(filepath.Join(m.dataDir, "#innodb_redo"), 0o750)
	os.MkdirAll(filepath.Join(m.dataDir, "lost+found"), 0o750)
	os.WriteFile(filepath.Join(m.dataDir, "ibdata1"), nil, 0o640)
	if err := m.CheckRemove(ctx, false, ""); err != nil {
		t.Fatalf("empty server: %v", err)
	}
	os.MkdirAll(filepath.Join(m.dataDir, "shop"), 0o750)
	if err := m.CheckRemove(ctx, false, ""); errCode(err) != "mariadb_has_databases:shop" {
		t.Fatalf("got %v", err)
	}
	// Data somewhere else: the panel cannot look, so it refuses.
	os.RemoveAll(filepath.Join(m.dataDir, "shop"))
	os.WriteFile(filepath.Join(m.etcDir, "mariadb.conf.d", "60-data.cnf"), []byte("[mysqld]\n#datadir = /var/lib/mysql\ndatadir = /srv/mysql\n"), 0o644)
	if err := m.CheckRemove(ctx, false, ""); errCode(err) != "mariadb_not_running:" {
		t.Fatalf("custom datadir: %v", err)
	}
}

func TestRemoveKeepsData(t *testing.T) {
	m, f, _ := newRemoveTest(t)
	ctx := context.Background()
	f.databases = "information_schema\nmysql\nsys\n"
	f.installed = "mariadb-server\tinstalled\nmariadb-server-core\tinstalled\nmariadb-client\tinstalled\nmariadb-client-core\tinstalled\nmariadb-plugin-provider-lz4\tnot-installed\n"
	f.sim = "Purg mariadb-client [1]\nPurg mariadb-client-core [1]\nPurg mariadb-server [1]\nPurg mariadb-server-core [1]\n"
	conf := filepath.Join(m.etcDir, "mariadb.conf.d")
	for _, n := range []string{"90-panel.cnf", "99-panel-remote.cnf", "99-panel-slowlog.cnf", "50-server.cnf"} {
		os.WriteFile(filepath.Join(conf, n), nil, 0o644)
	}
	if err := m.Remove(ctx, false, "", &nopLog{}); err != nil {
		t.Fatal(err)
	}
	if len(f.apt) != 1 || f.apt[0] != "purge mariadb-client mariadb-client-core mariadb-server mariadb-server-core" {
		t.Fatalf("apt %q", f.apt)
	}
	if !strings.Contains(strings.Join(f.stdin, "\n"), "debconf-set-selections: mariadb-server mariadb-server/postrm_remove_databases boolean false") {
		t.Fatalf("debconf not told to keep the databases: %q", f.stdin)
	}
	for _, n := range []string{"90-panel.cnf", "99-panel-remote.cnf", "99-panel-slowlog.cnf"} {
		if _, err := os.Stat(filepath.Join(conf, n)); err == nil {
			t.Errorf("%s left behind", n)
		}
	}
	if _, err := os.Stat(filepath.Join(conf, "50-server.cnf")); err != nil {
		t.Error("removed a file that is not the panel's")
	}
	if _, err := os.Stat(filepath.Join(m.dataDir, "mysql")); err != nil {
		t.Fatal("data directory deleted without being asked")
	}
}

func TestRemoveDeletesDataWhenAsked(t *testing.T) {
	m, f, _ := newRemoveTest(t)
	ctx := context.Background()
	f.databases = "information_schema\nmysql\n"
	f.installed = "mariadb-server\tinstalled\n"
	f.sim = "Purg mariadb-server [1]\n"
	// The service stops with the package; the fake keeps reporting it as
	// running, so the data must stay.
	if err := m.Remove(ctx, true, "mariadb", &nopLog{}); err == nil {
		t.Fatal("deleted the data of a running server")
	}
	if _, err := os.Stat(m.dataDir); err != nil {
		t.Fatal("data deleted while running")
	}
	f.running = false
	if err := m.Remove(ctx, true, "mariadb", &nopLog{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(m.dataDir); err == nil {
		t.Fatal("data directory still there")
	}
}
