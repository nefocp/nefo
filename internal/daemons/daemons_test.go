package daemons

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// fakeRunner records commands and answers from a table keyed by the
// program name and its first argument ("systemctl show").
type fakeRunner struct {
	calls   []string
	outputs map[string]string
	fail    map[string]error
}

func (f *fakeRunner) Output(_ context.Context, name string, args ...string) (string, error) {
	f.calls = append(f.calls, strings.TrimSpace(name+" "+strings.Join(args, " ")))
	key := name
	if len(args) > 0 {
		key += " " + args[0]
	}
	return f.outputs[key], f.fail[key]
}

func (f *fakeRunner) Input(ctx context.Context, _ []byte, name string, args ...string) (string, error) {
	return f.Output(ctx, name, args...)
}

func (f *fakeRunner) Stream(ctx context.Context, _ io.Writer, name string, args ...string) error {
	_, err := f.Output(ctx, name, args...)
	return err
}

func (f *fakeRunner) Pipe(ctx context.Context, _ io.Reader, _ io.Writer, name string, args ...string) error {
	_, err := f.Output(ctx, name, args...)
	return err
}

func (f *fakeRunner) called(prefix string) bool {
	return slices.ContainsFunc(f.calls, func(c string) bool { return strings.HasPrefix(c, prefix) })
}

// fakeStore keeps programs and sites in memory.
type fakeStore struct {
	daemons map[int64]*store.Daemon
	sites   map[int64]*store.Site
	next    int64
}

func (s *fakeStore) ListDaemons(context.Context) ([]*store.Daemon, error) {
	out := []*store.Daemon{}
	for i := int64(1); i <= s.next; i++ {
		if d, ok := s.daemons[i]; ok {
			c := *d
			out = append(out, &c)
		}
	}
	return out, nil
}

func (s *fakeStore) DaemonsForSite(ctx context.Context, siteID int64) ([]*store.Daemon, error) {
	all, _ := s.ListDaemons(ctx)
	return slices.DeleteFunc(all, func(d *store.Daemon) bool { return d.SiteID != siteID }), nil
}

func (s *fakeStore) DaemonByID(_ context.Context, id int64) (*store.Daemon, error) {
	d, ok := s.daemons[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	c := *d
	return &c, nil
}

func (s *fakeStore) SaveDaemon(_ context.Context, d *store.Daemon) error {
	if d.ID == 0 {
		s.next++
		d.ID = s.next
	} else if _, ok := s.daemons[d.ID]; !ok {
		return store.ErrNotFound
	}
	c := *d
	s.daemons[d.ID] = &c
	return nil
}

func (s *fakeStore) DeleteDaemon(_ context.Context, id int64) error {
	delete(s.daemons, id)
	return nil
}

func (s *fakeStore) SiteByID(_ context.Context, id int64) (*store.Site, error) {
	site, ok := s.sites[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return site, nil
}

func (s *fakeStore) ListSites(context.Context) ([]*store.Site, error) {
	out := []*store.Site{}
	for _, site := range s.sites {
		out = append(out, site)
	}
	return out, nil
}

func newTest(t *testing.T) (*Manager, *fakeRunner, *fakeStore) {
	t.Helper()
	dir := t.TempDir()
	st := &fakeStore{daemons: map[int64]*store.Daemon{}, sites: map[int64]*store.Site{
		1: {ID: 1, Domain: "example.com", SysUser: "example_com"},
		2: {ID: 2, Domain: "root.example", SysUser: "root"},
		3: {ID: 3, Domain: "other.example", SysUser: "other_example"},
	}}
	r := &fakeRunner{outputs: map[string]string{}, fail: map[string]error{}}
	m := New(st, r)
	m.www = filepath.Join(dir, "www")
	m.unitDir = filepath.Join(dir, "units")
	m.scriptDir = filepath.Join(dir, "lib")
	m.lookup = func(string) error { return nil }
	m.supported = func() bool { return true }
	for _, d := range []string{"www/example.com/app/sub", "www/example.com/public", "www/other.example", "units", "outside"} {
		if err := os.MkdirAll(filepath.Join(dir, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return m, r, st
}

func code(err error) string {
	var ue *usererr.Error
	if errors.As(err, &ue) {
		return ue.Code
	}
	if err == nil {
		return ""
	}
	return "other: " + err.Error()
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCreateWritesUnitScriptAndEnv(t *testing.T) {
	m, r, st := newTest(t)
	ctx := context.Background()
	d, err := m.Create(ctx, Request{
		Name: " Queue worker ", SiteID: 1, Command: "php8.3 artisan queue:work --sleep=3", Dir: "app",
		Env: []string{"APP_ENV=production", "", "# a comment", `SECRET=a"b\c$d` + "`e"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != 1 || d.Name != "Queue worker" || !d.Enabled || d.Dir != "app" || len(d.Env) != 2 {
		t.Fatalf("stored %+v", d)
	}
	if got := st.daemons[1]; got == nil || got.Command != "php8.3 artisan queue:work --sleep=3" {
		t.Fatalf("not saved: %+v", got)
	}

	unit := read(t, m.unitPath(1))
	for _, want := range []string{
		"User=example_com\n", "Group=example_com\n", "UMask=0027\n", "Restart=always\n", "RestartSec=3\n",
		"NoNewPrivileges=yes\n", "PrivateTmp=yes\n", "ProtectSystem=full\n", "ProtectHome=yes\n",
		"ExecStart=" + filepath.ToSlash(m.scriptPath(1)) + "\n",
		"EnvironmentFile=" + filepath.ToSlash(m.envPath(1)) + "\n",
		"WantedBy=multi-user.target\n",
	} {
		if !strings.Contains(unit, want) {
			t.Errorf("unit lacks %q:\n%s", want, unit)
		}
	}
	// The name the administrator typed never goes into the unit file.
	if strings.Contains(unit, "Queue worker") {
		t.Error("unit contains the program's name")
	}

	script := read(t, m.scriptPath(1))
	wantDir := filepath.ToSlash(m.www) + "/example.com/app"
	if !strings.HasPrefix(script, "#!/bin/sh\n") || !strings.Contains(script, "cd '"+wantDir+"' || exit 1\n") ||
		!strings.HasSuffix(script, "\nexec php8.3 artisan queue:work --sleep=3\n") {
		t.Fatalf("script:\n%s", script)
	}

	env := read(t, m.envPath(1))
	if !strings.Contains(env, "APP_ENV=\"production\"\n") || !strings.Contains(env, `SECRET="a\"b\\c\$d`+"\\`e\"\n") {
		t.Fatalf("env file:\n%s", env)
	}
	if runtime.GOOS != "windows" {
		for path, mode := range map[string]os.FileMode{m.scriptPath(1): 0o755, m.envPath(1): 0o600, m.unitPath(1): 0o644, m.scriptDir: 0o755} {
			if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != mode {
				t.Errorf("%s: mode %v, want %v", path, fi.Mode().Perm(), mode)
			}
		}
	}

	for _, c := range []string{"systemctl daemon-reload", "systemctl enable panel-daemon-1.service", "systemctl restart panel-daemon-1.service"} {
		if !r.called(c) {
			t.Errorf("missing %q in %q", c, r.calls)
		}
	}
}

func TestCreateDisabledDoesNotStart(t *testing.T) {
	m, r, _ := newTest(t)
	off := false
	if _, err := m.Create(context.Background(), Request{Name: "w", SiteID: 1, Command: "sleep 300", Enabled: &off}); err != nil {
		t.Fatal(err)
	}
	if r.called("systemctl restart") || !r.called("systemctl disable --now panel-daemon-1.service") {
		t.Fatalf("calls %q", r.calls)
	}
	// An empty folder is the site's folder itself.
	if !strings.Contains(read(t, m.scriptPath(1)), "cd '"+filepath.ToSlash(m.www)+"/example.com' ||") {
		t.Fatal(read(t, m.scriptPath(1)))
	}
}

func TestCreateRefusesBadInput(t *testing.T) {
	m, _, st := newTest(t)
	ctx := context.Background()
	if err := os.Symlink(filepath.Join(filepath.Dir(m.www), "outside"), filepath.Join(m.www, "example.com", "escape")); err != nil {
		t.Logf("no symbolic link test: %v", err)
	}
	ok := Request{Name: "w", SiteID: 1, Command: "sleep 300"}
	cases := []struct {
		mod  func(r *Request)
		code string
	}{
		{func(r *Request) { r.Name = "" }, "invalid_daemon_name"},
		{func(r *Request) { r.Name = strings.Repeat("x", 61) }, "invalid_daemon_name"},
		{func(r *Request) { r.Name = "a\tb" }, "invalid_daemon_name"},
		{func(r *Request) { r.Command = "" }, "invalid_daemon_command"},
		{func(r *Request) { r.Command = "sleep 1\nrm -rf /" }, "invalid_daemon_command"},
		{func(r *Request) { r.Command = "sleep\x00" }, "invalid_daemon_command"},
		{func(r *Request) { r.Command = strings.Repeat("x", 1001) }, "invalid_daemon_command"},
		{func(r *Request) { r.SiteID = 99 }, "invalid_daemon_site"},
		{func(r *Request) { r.SiteID = 2 }, "invalid_daemon_user"},
		{func(r *Request) { r.Dir = "../other.example" }, "invalid_daemon_dir"},
		{func(r *Request) { r.Dir = "app/../../other.example" }, "invalid_daemon_dir"},
		{func(r *Request) { r.Dir = "/etc" }, "invalid_daemon_dir"},
		{func(r *Request) { r.Dir = "app dir" }, "invalid_daemon_dir"},
		{func(r *Request) { r.Dir = "app';reboot;'" }, "invalid_daemon_dir"},
		{func(r *Request) { r.Dir = "missing" }, "daemon_dir_missing"},
		{func(r *Request) { r.Env = []string{"1BAD=x"} }, "invalid_daemon_env"},
		{func(r *Request) { r.Env = []string{"NOEQUALS"} }, "invalid_daemon_env"},
		{func(r *Request) { r.Env = []string{"A=b\x1bc"} }, "invalid_daemon_env"},
		{func(r *Request) { r.Env = slices.Repeat([]string{"A=1"}, 21) }, "too_many_daemon_env"},
	}
	if _, err := os.Lstat(filepath.Join(m.www, "example.com", "escape")); err == nil {
		cases = append(cases, struct {
			mod  func(r *Request)
			code string
		}{func(r *Request) { r.Dir = "escape" }, "daemon_dir_outside"})
	}
	for i, c := range cases {
		req := ok
		c.mod(&req)
		if _, err := m.Create(ctx, req); code(err) != c.code {
			t.Errorf("case %d (%+v): %v, want %s", i, req, err, c.code)
		}
	}
	if len(st.daemons) != 0 {
		t.Fatalf("stored %d programs", len(st.daemons))
	}

	m.lookup = func(string) error { return errors.New("unknown user") }
	if _, err := m.Create(ctx, ok); code(err) != "invalid_daemon_user" {
		t.Fatalf("missing Linux user: %v", err)
	}
}

func TestDirForms(t *testing.T) {
	root := "/var/www/example.com"
	for in, want := range map[string]string{
		"": "", ".": "", "./": "", "app": "app", "./app/": "app", "app/sub": "app/sub",
		"/var/www/example.com/app": "app", "/var/www/example.com": "", " app ": "app",
	} {
		if got, ok := cleanDir(in, root); !ok || got != want {
			t.Errorf("cleanDir(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, bad := range []string{"..", "a/..", "a/./b", "/var/www/example.community/x", "/app", "a//b", "a\\b", "~", "$HOME", "a%n"} {
		if got, ok := cleanDir(bad, root); ok {
			t.Errorf("cleanDir(%q) accepted as %q", bad, got)
		}
	}
}

func TestCreateFailureLeavesNothing(t *testing.T) {
	m, r, st := newTest(t)
	r.fail["systemctl daemon-reload"] = errors.New("exit status 1")
	if _, err := m.Create(context.Background(), Request{Name: "w", SiteID: 1, Command: "sleep 300"}); err == nil {
		t.Fatal("no error")
	}
	if len(st.daemons) != 0 {
		t.Fatal("program kept after a failed start")
	}
	for _, p := range []string{m.unitPath(1), m.scriptPath(1), m.envPath(1)} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("%s left behind", p)
		}
	}
}

func TestUpdateActionsDelete(t *testing.T) {
	m, r, st := newTest(t)
	ctx := context.Background()
	d, err := m.Create(ctx, Request{Name: "w", SiteID: 1, Command: "sleep 300"})
	if err != nil {
		t.Fatal(err)
	}
	created := d.CreatedAt

	r.calls = nil
	d, err = m.Update(ctx, d.ID, Request{Name: "w2", SiteID: 3, Command: "sleep 600", Dir: ""})
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "w2" || d.SiteID != 3 || !d.CreatedAt.Equal(created) {
		t.Fatalf("updated %+v", d)
	}
	if !strings.Contains(read(t, m.unitPath(1)), "User=other_example\n") || !r.called("systemctl restart panel-daemon-1.service") {
		t.Fatalf("unit not rewritten and restarted: %q", r.calls)
	}
	if _, err := m.Update(ctx, 42, Request{Name: "w", SiteID: 1, Command: "x"}); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("update of a missing program: %v", err)
	}

	r.calls = nil
	if err := m.Action(ctx, 1, "stop"); err != nil {
		t.Fatal(err)
	}
	if st.daemons[1].Enabled || !r.called("systemctl disable --now panel-daemon-1.service") {
		t.Fatalf("stop: enabled=%v calls=%q", st.daemons[1].Enabled, r.calls)
	}
	r.calls = nil
	if err := m.Action(ctx, 1, "start"); err != nil {
		t.Fatal(err)
	}
	if !st.daemons[1].Enabled || !r.called("systemctl enable panel-daemon-1.service") || !r.called("systemctl restart panel-daemon-1.service") {
		t.Fatalf("start: calls=%q", r.calls)
	}
	for _, bad := range []string{"enable", "kill", "../x"} {
		if err := m.Action(ctx, 1, bad); code(err) != "action_not_allowed" {
			t.Errorf("action %q: %v", bad, err)
		}
	}

	r.outputs["journalctl -u"] = "line 1\nline 2\n"
	if text, err := m.Logs(ctx, 1); err != nil || text != "line 1\nline 2\n" {
		t.Fatalf("logs %q %v", text, err)
	}
	if !r.called("journalctl -u panel-daemon-1.service -n 200") {
		t.Fatalf("calls %q", r.calls)
	}

	r.calls = nil
	if err := m.Delete(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if len(st.daemons) != 0 || !r.called("systemctl disable --now panel-daemon-1.service") || !r.called("systemctl daemon-reload") {
		t.Fatalf("delete: %d left, calls %q", len(st.daemons), r.calls)
	}
	for _, p := range []string{m.unitPath(1), m.scriptPath(1), m.envPath(1)} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("%s left behind", p)
		}
	}
	if err := m.Delete(ctx, 1); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
}

func TestRemoveForSite(t *testing.T) {
	m, _, st := newTest(t)
	ctx := context.Background()
	for _, site := range []int64{1, 3, 1} {
		if _, err := m.Create(ctx, Request{Name: "w", SiteID: site, Command: "sleep 300"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.RemoveForSite(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if len(st.daemons) != 1 || st.daemons[2] == nil {
		t.Fatalf("left %v", st.daemons)
	}
	if _, err := os.Stat(m.unitPath(1)); err == nil {
		t.Fatal("unit of a removed program left behind")
	}
	if _, err := os.Stat(m.unitPath(2)); err != nil {
		t.Fatal("another site's program was removed")
	}
}

func TestListWithStatus(t *testing.T) {
	m, r, _ := newTest(t)
	ctx := context.Background()
	for _, site := range []int64{1, 3} {
		if _, err := m.Create(ctx, Request{Name: "w", SiteID: site, Command: "sleep 300"}); err != nil {
			t.Fatal(err)
		}
	}
	r.outputs["systemctl show"] = "MainPID=4242\nNRestarts=3\nMemoryCurrent=1048576\nId=panel-daemon-1.service\n" +
		"ActiveState=active\nSubState=running\nActiveEnterTimestamp=Tue 2026-11-24 20:08:42 UTC\n\n" +
		"MainPID=0\nNRestarts=0\nMemoryCurrent=[not set]\nId=panel-daemon-2.service\nActiveState=failed\nSubState=failed\nActiveEnterTimestamp=\n"
	list, err := m.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("%d programs", len(list))
	}
	a, b := list[0], list[1]
	if a.Site != "example.com" || a.User != "example_com" || a.Status.Active != "active" || a.Status.Restarts != 3 ||
		a.Status.PID != 4242 || a.Status.Memory != 1<<20 || a.Status.Since == nil {
		t.Fatalf("first: %+v", a)
	}
	if b.Site != "other.example" || b.Status.Active != "failed" || b.Status.Memory != 0 || b.Status.Since != nil {
		t.Fatalf("second: %+v", b)
	}
	if !r.called("systemctl show --no-pager --property=Id,ActiveState,SubState,NRestarts,MainPID,MemoryCurrent,ActiveEnterTimestamp panel-daemon-1.service panel-daemon-2.service") {
		t.Fatalf("calls %q", r.calls)
	}
}

func TestUnsupported(t *testing.T) {
	m, _, _ := newTest(t)
	m.supported = func() bool { return false }
	if _, err := m.Create(context.Background(), Request{Name: "w", SiteID: 1, Command: "sleep 1"}); err == nil {
		t.Fatal("created on a machine without systemd")
	}
}
