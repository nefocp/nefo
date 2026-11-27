package database

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
)

// fakeRunner records the SQL it receives and pretends MariaDB is installed.
type fakeRunner struct {
	sql     []string
	cmds    []string          // other programs run, with their arguments
	answer  map[string]string // first matching substring -> output
	failOn  string
	failCmd string // a command line that fails
	// onPipe, when set, runs in place of a piped program.
	onPipe func(ctx context.Context, stdin io.Reader, name string, args []string) error
}

func (f *fakeRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	line := strings.Join(append([]string{name}, args...), " ")
	if name != "mariadbd" && !(name == "systemctl" && len(args) > 0 && args[0] == "is-active") {
		f.cmds = append(f.cmds, line)
	}
	if f.failCmd != "" && line == f.failCmd {
		return "failed", errors.New("exit status 1")
	}
	switch name {
	case "mariadbd":
		return "mariadbd  Ver 10.11.14-MariaDB-0ubuntu0.24.04.1 for debian-linux-gnu", nil
	case "systemctl":
		return "active\n", nil
	}
	return "", nil
}

func (f *fakeRunner) Stream(ctx context.Context, w io.Writer, name string, args ...string) error {
	return nil
}

func (f *fakeRunner) Input(ctx context.Context, stdin []byte, name string, args ...string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err // a real program would not start either
	}
	s := string(stdin)
	f.sql = append(f.sql, s)
	if f.failOn != "" && strings.Contains(s, f.failOn) {
		return "ERROR 1396", errors.New("exit status 1")
	}
	for k, v := range f.answer {
		if strings.Contains(s, k) {
			return v, nil
		}
	}
	return "", nil
}

func (f *fakeRunner) Pipe(ctx context.Context, stdin io.Reader, stdout io.Writer, name string, args ...string) error {
	if f.onPipe != nil {
		return f.onPipe(ctx, stdin, name, args)
	}
	return nil
}

func newTest(t *testing.T) (*Manager, *fakeRunner) {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	f := &fakeRunner{answer: map[string]string{"information_schema.SCHEMATA WHERE": "0\n"}}
	m := New(f, st, slog.New(slog.NewTextHandler(io.Discard, nil)))
	m.supported = func() bool { return true }
	// Never touch the files of a real MariaDB.
	dir := t.TempDir()
	m.remoteConf = filepath.Join(dir, "99-panel-remote.cnf")
	m.slowConf = filepath.Join(dir, "99-panel-slowlog.cnf")
	m.slowLog = filepath.Join(dir, "mariadb-slow.log")
	m.mariaKeyring = filepath.Join(dir, "keyrings/panel-mariadb.gpg")
	m.mariaList = filepath.Join(dir, "sources.list.d/panel-mariadb.list")
	m.dumpsDir = filepath.Join(dir, "db-server")
	return m, f
}

func TestCreateAndDelete(t *testing.T) {
	m, f := newTest(t)
	ctx := context.Background()
	d, err := m.Create(ctx, " Shop_DB ", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "shop_db" || d.Username != "shop_db" || validate.DBPassword(d.Password) != nil {
		t.Fatalf("created %+v", d)
	}
	last := f.sql[len(f.sql)-1]
	for _, want := range []string{"CREATE DATABASE `shop_db`", "CREATE USER 'shop_db'@'localhost' IDENTIFIED BY '" + d.Password + "'", "ON `shop_db`.*"} {
		if !strings.Contains(last, want) {
			t.Errorf("missing %q in:\n%s", want, last)
		}
	}
	if _, err := m.Create(ctx, "shop_db", "", 0); !isCode(err, "db_exists") {
		t.Fatalf("duplicate: err = %v", err)
	}
	if err := m.Delete(ctx, d.ID, "wrong"); !isCode(err, "confirm_mismatch") {
		t.Fatalf("wrong confirm: err = %v", err)
	}
	if err := m.Delete(ctx, d.ID, "shop_db"); err != nil {
		t.Fatal(err)
	}
	if last := f.sql[len(f.sql)-1]; !strings.Contains(last, "DROP DATABASE IF EXISTS `shop_db`") {
		t.Fatalf("delete ran:\n%s", last)
	}
}

func TestCreateRollsBackOnError(t *testing.T) {
	m, f := newTest(t)
	f.failOn = "CREATE USER"
	if _, err := m.Create(context.Background(), "app", "", 0); err == nil {
		t.Fatal("no error")
	}
	if last := f.sql[len(f.sql)-1]; !strings.Contains(last, "DROP DATABASE IF EXISTS `app`") {
		t.Fatalf("no rollback, last SQL:\n%s", last)
	}
	if list, _ := m.store.ListDatabases(context.Background()); len(list) != 0 {
		t.Fatal("record kept after failure")
	}
}

func TestRejectsUnsafeInput(t *testing.T) {
	m, f := newTest(t)
	ctx := context.Background()
	for _, name := range []string{"a`b", "x'; DROP DATABASE mysql; --", "mysql", "1abc"} {
		if _, err := m.Create(ctx, name, "", 0); !isCode(err, "invalid_db_name") {
			t.Errorf("%q: err = %v", name, err)
		}
	}
	if _, err := m.Create(ctx, "ok", "pa'ss word 123", 0); !isCode(err, "invalid_db_password") {
		t.Errorf("bad password: err = %v", err)
	}
	if len(f.sql) != 0 {
		t.Fatalf("SQL ran for invalid input: %q", f.sql)
	}
}

func TestTuning(t *testing.T) {
	cases := map[int64]string{1 << 30: "128M", 2 << 30: "256M", 4 << 30: "512M", 16 << 30: "1G"}
	for mem, want := range cases {
		if got := tuning(mem); !strings.Contains(got, "innodb_buffer_pool_size = "+want) {
			t.Errorf("mem %d: %s", mem, got)
		}
	}
	if !strings.Contains(tuning(0), "bind-address = 127.0.0.1") {
		t.Error("not bound to localhost")
	}
}

func isCode(err error, code string) bool {
	var ue *usererr.Error
	return errors.As(err, &ue) && ue.Code == code
}
