package services

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeRun struct{ calls []string }

func (f *fakeRun) Output(ctx context.Context, name string, args ...string) (string, error) {
	f.calls = append(f.calls, name+" "+strings.Join(args, " "))
	if name == "systemctl" && args[0] == "show" {
		if strings.HasPrefix(args[1], "redis") {
			return "LoadState=not-found\n", nil
		}
		return "LoadState=loaded\nActiveState=active\nSubState=running\nUnitFileState=enabled\n" +
			"ActiveEnterTimestamp=Tue 2026-11-24 16:27:01 UTC\nMemoryCurrent=20971520\nDescription=A service\n", nil
	}
	return "", nil
}
func (f *fakeRun) Stream(context.Context, io.Writer, string, ...string) error { return nil }
func (f *fakeRun) Input(context.Context, []byte, string, ...string) (string, error) {
	return "", nil
}
func (f *fakeRun) Pipe(context.Context, io.Reader, io.Writer, string, ...string) error { return nil }

func TestListAndGuards(t *testing.T) {
	f := &fakeRun{}
	m := New(f)
	m.phpDir = t.TempDir()
	os.MkdirAll(filepath.Join(m.phpDir, "8.3", "fpm"), 0o755)
	os.MkdirAll(filepath.Join(m.phpDir, "junk;rm", "fpm"), 0o755)

	list := m.List(context.Background())
	names := []string{}
	for _, s := range list {
		names = append(names, s.Name)
	}
	got := strings.Join(names, ",")
	if !strings.Contains(got, "php8.3-fpm") || strings.Contains(got, "redis") || strings.Contains(got, "junk") {
		t.Fatalf("services = %s", got)
	}
	if list[0].Memory != 20<<20 || list[0].Since == nil || !list[0].Enabled {
		t.Fatalf("parsed %+v", list[0])
	}

	ctx := context.Background()
	for _, bad := range [][2]string{{"ssh", "stop"}, {"nefo", "stop"}, {"sshd", "restart"}, {"nginx", "enable"}, {"../x", "start"}} {
		if err := m.Do(ctx, bad[0], bad[1]); err == nil {
			t.Errorf("%s %s allowed", bad[0], bad[1])
		}
	}
	f.calls = nil
	if err := m.Do(ctx, "nefo", "restart"); err != nil {
		t.Fatal(err)
	}
	if f.calls[len(f.calls)-1] != "systemctl --no-block restart nefo.service" {
		t.Fatalf("calls = %q", f.calls)
	}
	if err := m.Do(ctx, "nginx", "reload"); err != nil || f.calls[len(f.calls)-2] != "nginx -t" {
		t.Fatalf("nginx reload without config test: %q %v", f.calls, err)
	}
}

// aliasRun plays systemd on a MariaDB server, where mysql.service is an
// alias of mariadb.service.
type aliasRun struct{ fakeRun }

func (f *aliasRun) Output(ctx context.Context, name string, args ...string) (string, error) {
	if name == "systemctl" && args[0] == "show" && (args[1] == "mysql.service" || args[1] == "mariadb.service") {
		return "Id=mariadb.service\nLoadState=loaded\nActiveState=active\nSubState=running\nUnitFileState=enabled\n", nil
	}
	return f.fakeRun.Output(ctx, name, args...)
}

func TestAliasShownOnce(t *testing.T) {
	m := New(&aliasRun{})
	m.phpDir = t.TempDir()
	n := 0
	for _, s := range m.List(context.Background()) {
		if s.Name == "mysql" {
			t.Error("the mysql alias of MariaDB listed")
		}
		if s.Name == "mariadb" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("mariadb listed %d times", n)
	}
}
