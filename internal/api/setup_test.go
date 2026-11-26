package api

import (
	"context"
	"errors"
	"net"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/setup"
)

// fakeSetup is a 1 GB Ubuntu server with nothing installed, whose installs
// only record what they were asked.
type fakeSetup struct {
	mu        sync.Mutex
	installed map[string]bool
	fail      map[string]error
	calls     []string
	release   chan struct{} // when set, each install waits for it
}

func newFakeSetup(srv *Server) *fakeSetup {
	f := &fakeSetup{installed: map[string]bool{}, fail: map[string]error{}}
	srv.setupTest = &setupHooks{
		env: func(context.Context) (setup.Env, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			inst := map[string]bool{}
			web := ""
			for k, v := range f.installed {
				inst[k] = v
				if v && (k == "nginx" || k == "apache") {
					web = k
				}
			}
			return setup.NewEnv(setup.Facts{
				Supported:   true,
				Server:      setup.Server{CPUs: 1, MemTotalMB: 960, MemUsedMB: 250, DiskTotalMB: 25000, DiskFreeMB: 20000, Arch: "amd64"},
				WebServer:   web,
				DistroPHP:   "8.3",
				PHPRepo:     "ppa:ondrej/php",
				PHPVersions: []string{"8.1", "8.2", "8.3", "8.4", "8.5"},
				Installed:   inst,
			}), nil
		},
		installer: func(user, ip string) setup.Installer { return f },
	}
	return f
}

func (f *fakeSetup) Installed(_ context.Context, id string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.installed[id]
}

func (f *fakeSetup) Install(_ context.Context, id string, lg setup.Logger) error {
	if f.release != nil {
		<-f.release
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, id)
	lg.Logf("installing %s", id)
	if err := f.fail[id]; err != nil {
		return err
	}
	f.installed[id] = true
	if id == "nginx" {
		f.installed["php8.3"] = true
	}
	return nil
}

func (f *fakeSetup) takeCalls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := f.calls
	f.calls = nil
	return c
}

func waitJob(t *testing.T, srv *Server, id string) (status, errText string) {
	t.Helper()
	j, ok := srv.jobs.Get(id)
	if !ok {
		t.Fatalf("no job %s", id)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if st, e := j.State(); st != "running" {
			return st, e
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the setup job did not finish")
	return "", ""
}

func TestSetupWizardAPI(t *testing.T) {
	e := newEnv(t)
	if res, _ := e.do(t, "GET", "/api/setup", nil, nil); res.StatusCode != 401 {
		t.Fatalf("GET /api/setup without signing in: %d", res.StatusCode)
	}
	e.login(t)
	fake := newFakeSetup(e.srv)
	ctx := context.Background()

	res, v := e.do(t, "GET", "/api/setup", nil, nil)
	if res.StatusCode != 200 || v["offered"] != true || v["supported"] != true || v["state"] != "" {
		t.Fatalf("view: %d %v", res.StatusCode, v)
	}
	def := v["defaults"].(map[string]any)
	if def["web"] != "nginx" || !reflect.DeepEqual(def["php"], []any{"8.3"}) || def["db"] != "mariadb" || !reflect.DeepEqual(def["extras"], []any{}) {
		t.Fatalf("defaults on 1 GB: %v", def)
	}
	if srv := v["server"].(map[string]any); srv["mem_total_mb"] != float64(960) || v["ram_budget_mb"] != float64(960*85/100-250) {
		t.Fatalf("server %v, budget %v", srv, v["ram_budget_mb"])
	}
	if _, f := e.do(t, "GET", "/api/features", nil, nil); f["setup"] != true {
		t.Fatalf("features: %v", f)
	}

	// A choice that can't be installed starts no job.
	res, out := e.do(t, "POST", "/api/setup", map[string]any{"web": "", "php": []string{}, "extras": []string{"waf"}}, nil)
	if res.StatusCode != 400 || out["error"] != "setup_needs_web" {
		t.Fatalf("WAF without a web server: %d %v", res.StatusCode, out)
	}
	if res, out := e.do(t, "POST", "/api/setup", map[string]any{"web": "nginx", "wipe": true}, nil); res.StatusCode != 400 || out["error"] != "bad_request" {
		t.Fatalf("unknown field: %d %v", res.StatusCode, out)
	}
	if n := len(e.srv.jobs.Recent()); n != 0 {
		t.Fatalf("%d jobs started by refused choices", n)
	}

	// The first try stops at PHP 8.4.
	fake.fail["php8.4"] = errors.New("apt-get install php8.4-fpm: exit status 100")
	choice := map[string]any{"web": "nginx", "php": []string{"8.4", "8.3"}, "db": "mariadb", "extras": []string{"redis"}}
	res, out = e.do(t, "POST", "/api/setup", choice, nil)
	if res.StatusCode != 202 || out["kind"] != "setup" || out["target"] != "Nginx and PHP 8.3, PHP 8.4, MariaDB, Redis" {
		t.Fatalf("start: %d %v", res.StatusCode, out)
	}
	steps := out["plan"].(map[string]any)["steps"].([]any)
	if len(steps) != 4 || steps[0].(map[string]any)["id"] != "nginx" {
		t.Fatalf("plan: %v", out["plan"])
	}
	st, errText := waitJob(t, e.srv, out["id"].(string))
	if st != "failed" || !strings.Contains(errText, "PHP 8.4 could not be installed") || !strings.Contains(errText, "Installed before it stopped: Nginx and PHP 8.3.") {
		t.Fatalf("first try: %s %q", st, errText)
	}
	if calls := fake.takeCalls(); !reflect.DeepEqual(calls, []string{"nginx", "php8.4"}) {
		t.Fatalf("first try installed %v", calls)
	}
	if state, _ := e.store.Setting(ctx, setup.StateKey); state != "" {
		t.Fatalf("a failed setup is marked %q", state)
	}

	// The retry skips Nginx and finishes.
	delete(fake.fail, "php8.4")
	res, out = e.do(t, "POST", "/api/setup", choice, nil)
	if res.StatusCode != 202 {
		t.Fatalf("retry: %d %v", res.StatusCode, out)
	}
	if st, errText := waitJob(t, e.srv, out["id"].(string)); st != "succeeded" {
		t.Fatalf("retry: %s %q", st, errText)
	}
	if calls := fake.takeCalls(); !reflect.DeepEqual(calls, []string{"php8.4", "mariadb", "redis"}) {
		t.Fatalf("retry installed %v", calls)
	}
	j, _ := e.srv.jobs.Get(out["id"].(string))
	if log := strings.Join(j.Snapshot().Log, "\n"); !strings.Contains(log, "[1/5] Nginx is installed already: skipped") {
		t.Fatalf("retry log:\n%s", log)
	}
	if state, _ := e.store.Setting(ctx, setup.StateKey); state != setup.StateDone {
		t.Fatalf("state after the setup: %q", state)
	}
	if _, v := e.do(t, "GET", "/api/setup", nil, nil); v["offered"] != false || v["state"] != "done" || v["web_server"] != "nginx" {
		t.Fatalf("view after the setup: %v", v)
	}
	if _, f := e.do(t, "GET", "/api/features", nil, nil); f["setup"] != false {
		t.Fatalf("features after the setup: %v", f)
	}
	// Skipping later keeps it done.
	if res, _ := e.do(t, "POST", "/api/setup/skip", nil, nil); res.StatusCode != 200 {
		t.Fatalf("skip: %d", res.StatusCode)
	}
	if state, _ := e.store.Setting(ctx, setup.StateKey); state != setup.StateDone {
		t.Fatalf("skip changed a finished setup to %q", state)
	}
}

func TestSetupSkipAndBusy(t *testing.T) {
	e := newEnv(t)
	e.login(t)
	fake := newFakeSetup(e.srv)
	fake.release = make(chan struct{})

	res, out := e.do(t, "POST", "/api/setup", map[string]any{"web": "nginx", "php": []string{}, "extras": []string{}}, nil)
	if res.StatusCode != 202 {
		t.Fatalf("start: %d %v", res.StatusCode, out)
	}
	// One job at a time, also for the setup.
	if res, out := e.do(t, "POST", "/api/setup", map[string]any{"web": "nginx"}, nil); res.StatusCode != 409 || out["error"] != "busy" {
		t.Fatalf("second setup: %d %v", res.StatusCode, out)
	}
	close(fake.release)
	waitJob(t, e.srv, out["id"].(string))

	e2 := newEnv(t)
	e2.login(t)
	newFakeSetup(e2.srv)
	if res, _ := e2.do(t, "POST", "/api/setup/skip", nil, nil); res.StatusCode != 200 {
		t.Fatalf("skip: %d", res.StatusCode)
	}
	if _, v := e2.do(t, "GET", "/api/setup", nil, nil); v["offered"] != false || v["state"] != "skipped" {
		t.Fatalf("view after skipping: %v", v)
	}
	if _, f := e2.do(t, "GET", "/api/features", nil, nil); f["setup"] != false {
		t.Fatalf("features after skipping: %v", f)
	}
}

// The real server facts: whatever this machine is, the answer is
// consistent, and on a machine that can't host sites nothing is offered.
func TestSetupViewOfThisMachine(t *testing.T) {
	e := newEnv(t)
	v, err := e.srv.SetupView(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !v.Supported {
		if v.Offered || len(v.Parts) != 0 {
			t.Fatalf("unsupported machine: %+v", v)
		}
		if _, err := e.srv.planSetup(context.Background(), setup.Choice{Web: "nginx"}); err == nil {
			t.Fatal("a setup was planned on an unsupported machine")
		}
	}
}

func TestLocalSocket(t *testing.T) {
	dir, err := os.MkdirTemp("", "nefo")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	e := newEnv(t)
	e.srv.cfg.DataDir = dir
	fake := newFakeSetup(e.srv)
	client := NewLocalClient(LocalSocket(dir))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := client.SetupView(ctx); !errors.Is(err, ErrNoPanel) {
		t.Fatalf("no panel: %v", err)
	}
	ln, err := e.srv.ListenLocal()
	if err != nil {
		t.Skipf("no Unix sockets here: %v", err)
	}
	served := make(chan struct{})
	sctx, stop := context.WithCancel(context.Background())
	go func() { e.srv.ServeLocal(sctx, ln); close(served) }()
	defer func() { stop(); <-served }()
	if runtime.GOOS != "windows" {
		if fi, err := os.Lstat(LocalSocket(dir)); err != nil || fi.Mode().Perm() != 0o600 {
			t.Fatalf("socket: %v %v", fi, err)
		}
	}

	v, err := client.SetupView(ctx)
	if err != nil || !v.Offered || v.Defaults.Web != "nginx" {
		t.Fatalf("view: %+v %v", v, err)
	}
	if _, err := v.Plan(setup.Choice{Web: "nginx", Extras: []string{"docker"}}); err != nil {
		t.Fatalf("plan from the panel's view: %v", err)
	}
	var le *LocalError
	if _, err := client.StartSetup(ctx, setup.Choice{Extras: []string{"adminer"}}); !errors.As(err, &le) || le.Code != "setup_needs_web" || le.Detail != "Adminer" {
		t.Fatalf("refused choice: %v", err)
	}
	id, err := client.StartSetup(ctx, setup.Choice{Web: "nginx", PHP: []string{"8.5"}, DB: "mariadb"})
	if err != nil || id == "" {
		t.Fatalf("start: %q %v", id, err)
	}
	var lines []string
	next := 0
	for {
		l, err := client.JobLog(ctx, id, next)
		if err != nil {
			t.Fatal(err)
		}
		lines, next = append(lines, l.Lines...), l.Next
		if l.Status != "running" {
			if l.Status != "succeeded" {
				t.Fatalf("job %s: %s", l.Status, l.Error)
			}
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := strings.Join(lines, "\n"); !strings.Contains(got, "==> [1/3] Installing Nginx and PHP 8.3") || !strings.Contains(got, "The setup has finished") {
		t.Fatalf("log:\n%s", got)
	}
	if calls := fake.takeCalls(); !reflect.DeepEqual(calls, []string{"nginx", "php8.5", "mariadb"}) {
		t.Fatalf("installed %v", calls)
	}
	if _, err := client.JobLog(ctx, "nosuchjob", 0); !errors.As(err, &le) || le.Code != "not_found" {
		t.Fatalf("unknown job: %v", err)
	}
	if err := client.SkipSetup(ctx); err != nil {
		t.Fatal(err)
	}
	if state, _ := e.store.Setting(ctx, setup.StateKey); state != setup.StateDone {
		t.Fatalf("state %q", state)
	}
	// The panel's own users are not involved: the audit log says root, from cli.
	entries, err := e.store.RecentAudit(ctx, 20)
	if err == nil {
		found := false
		for _, a := range entries {
			if a.Action == "setup_done" && a.Username == "root" && a.IP == "cli" {
				found = true
			}
		}
		if !found {
			t.Errorf("no setup_done audit entry from cli: %+v", entries)
		}
	}
	stop()
	<-served
	if _, err := os.Stat(LocalSocket(dir)); err == nil && runtime.GOOS != "windows" {
		t.Error("the socket file stays after the panel stopped")
	}
	if _, err := net.Dial("unix", LocalSocket(dir)); err == nil {
		t.Error("the socket still answers")
	}
	served = make(chan struct{})
	close(served)
}
