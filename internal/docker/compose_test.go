package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/usererr"
)

// The testdata/config-*.json files are what `docker compose config --format
// json` (Compose 2.40) printed for the config-*.yaml next to them.

// script is a Runner that answers each command with respond and records
// every command line.
type script struct {
	calls   []string
	respond func(cmd string) (string, error)
}

func (s *script) do(name string, args []string) (string, error) {
	cmd := strings.Join(append([]string{name}, args...), " ")
	s.calls = append(s.calls, cmd)
	if s.respond == nil {
		return "", nil
	}
	return s.respond(cmd)
}
func (s *script) Output(_ context.Context, name string, args ...string) (string, error) {
	return s.do(name, args)
}
func (s *script) Stream(_ context.Context, w io.Writer, name string, args ...string) error {
	out, err := s.do(name, args)
	io.WriteString(w, out)
	return err
}
func (s *script) Input(_ context.Context, _ []byte, name string, args ...string) (string, error) {
	return s.do(name, args)
}
func (s *script) Pipe(_ context.Context, _ io.Reader, w io.Writer, name string, args ...string) error {
	out, err := s.do(name, args)
	io.WriteString(w, out)
	return err
}

func (s *script) ran(part string) bool {
	for _, c := range s.calls {
		if strings.Contains(c, part) {
			return true
		}
	}
	return false
}

// failed is what a program that exits with an error looks like.
func failed(output string) error {
	return &run.Error{Cmd: "docker", Err: errors.New("exit status 1"), Output: output}
}

type testLog struct{ strings.Builder }

func (l *testLog) Logf(format string, args ...any) { fmt.Fprintf(l, format+"\n", args...) }

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func code(err error) string {
	var ue *usererr.Error
	if errors.As(err, &ue) {
		return ue.Code + ": " + ue.Detail
	}
	if err == nil {
		return "ok"
	}
	return "error: " + err.Error()
}

// noLinks makes resolveLink report that no path is a symbolic link.
func noLinks(t *testing.T, links map[string]string) {
	old := resolveLink
	resolveLink = func(p string) (string, error) {
		if real, ok := links[p]; ok {
			return real, nil
		}
		return "", os.ErrNotExist
	}
	t.Cleanup(func() { resolveLink = old })
}

func TestCheckConfigRealOutput(t *testing.T) {
	noLinks(t, nil)
	if err := CheckConfig([]byte(fixture(t, "config-safe.json"))); err != nil {
		t.Fatalf("safe project refused: %v", err)
	}
	// The first problem in service order: "db" mounts the Docker socket.
	if got := code(CheckConfig([]byte(fixture(t, "config-unsafe.json")))); got != "compose_forbidden_mount: db: /var/run/docker.sock" {
		t.Errorf("unsafe project: %s", got)
	}
	// "other" publishes on ::1 (allowed), "app" uses a named volume that
	// binds the root folder; the secret is a file in /root.
	if got := code(CheckConfig([]byte(fixture(t, "config-mounts.json")))); got != "compose_forbidden_mount: volume hostroot: /" {
		t.Errorf("named volume binding /: %s", got)
	}
	if CheckConfig([]byte("not json")) == nil {
		t.Error("unreadable output accepted")
	}
}

// service builds a config with one service "web" from a JSON fragment
// shaped like Compose's output.
func service(fields string) []byte {
	return []byte(`{"name":"panel-x","services":{"web":{"command":null,"entrypoint":null,"image":"alpine"` + fields + `}}}`)
}

func port(hostIP, published string) string {
	p := `{"mode":"ingress","target":80,"protocol":"tcp"`
	if hostIP != "" {
		p += `,"host_ip":"` + hostIP + `"`
	}
	if published != "" {
		p += `,"published":` + published
	}
	return `,"ports":[` + p + `}]`
}

func bind(source string, readOnly bool) string {
	ro := ""
	if readOnly {
		ro = `,"read_only":true`
	}
	return `,"volumes":[{"type":"bind","source":"` + source + `","target":"/mnt"` + ro + `,"bind":{"create_host_path":true}}]`
}

func TestCheckConfigRules(t *testing.T) {
	noLinks(t, map[string]string{"/srv/etc-link": "/etc", "/srv/zone": "/usr/share/zoneinfo/UTC"})
	for _, c := range []struct{ fields, want string }{
		{port("127.0.0.1", `"8080"`), "ok"},
		{port("::1", `"8080"`), "ok"},
		{port("127.0.0.1", `8080`), "ok"}, // older Compose prints numbers
		{port("", `"8080"`), "compose_public_port: web: 8080->80/tcp"},
		{port("0.0.0.0", `"8080"`), "compose_public_port: web: 0.0.0.0:8080->80/tcp"},
		{port("::", `"8080"`), "compose_public_port: web: :::8080->80/tcp"},
		{port("192.0.2.10", `"8080"`), "compose_public_port: web: 192.0.2.10:8080->80/tcp"},
		{port("", ""), "compose_public_port: web: 80/tcp"}, // a random port on every address
		{`,"expose":["80"]`, "ok"},
		{`,"privileged":true`, "compose_privileged: web"},
		{`,"privileged":false`, "ok"},
		{`,"network_mode":"host"`, "compose_host_namespace: web: network_mode: host"},
		{`,"network_mode":"bridge"`, "ok"},
		{`,"network_mode":"service:db"`, "ok"},
		{`,"pid":"host"`, "compose_host_namespace: web: pid: host"},
		{`,"ipc":"host"`, "compose_host_namespace: web: ipc: host"},
		{`,"ipc":"shareable"`, "ok"},
		{bind("/", true), "compose_forbidden_mount: web: /"},
		{bind("/etc", true), "compose_forbidden_mount: web: /etc"},
		{bind("/etc/", false), "compose_forbidden_mount: web: /etc/"},
		{bind("/root/../etc", false), "compose_forbidden_mount: web: /root/../etc"},
		{bind("/etc/nginx", true), "compose_forbidden_mount: web: /etc/nginx"},
		{bind("/root", false), "compose_forbidden_mount: web: /root"},
		{bind("/root/.ssh", true), "compose_forbidden_mount: web: /root/.ssh"},
		{bind("/boot", false), "compose_forbidden_mount: web: /boot"},
		{bind("/proc", true), "compose_forbidden_mount: web: /proc"},
		{bind("/proc/1/root", false), "compose_forbidden_mount: web: /proc/1/root"},
		{bind("/sys", true), "compose_forbidden_mount: web: /sys"},
		{bind("/sys/fs/cgroup", true), "compose_forbidden_mount: web: /sys/fs/cgroup"},
		{bind("/dev", false), "compose_forbidden_mount: web: /dev"},
		{bind("/dev/sda", false), "compose_forbidden_mount: web: /dev/sda"},
		{bind("/var/run/docker.sock", false), "compose_forbidden_mount: web: /var/run/docker.sock"},
		{bind("/run/docker.sock", true), "compose_forbidden_mount: web: /run/docker.sock"},
		{bind("/run", false), "compose_forbidden_mount: web: /run"},
		{bind("/var/run", false), "compose_forbidden_mount: web: /var/run"},
		{bind("/var", false), "compose_forbidden_mount: web: /var"},
		{bind("/srv/etc-link", false), "compose_forbidden_mount: web: /srv/etc-link"},
		{bind("/etc/localtime", true), "ok"},
		{bind("/etc/timezone", true), "ok"},
		{bind("/etc/localtime", false), "compose_forbidden_mount: web: /etc/localtime"},
		{bind("/srv/zone", false), "ok"},
		{bind("/etcetera", false), "ok"},
		{bind("/var/lib/panel-compose/x/data", false), "ok"},
		{bind("/var/www/example.com", false), "ok"},
		{bind("/run/user/1000/app", false), "ok"},
		{bind("/home/user/app", false), "ok"},
		{`,"volumes":[{"type":"volume","source":"data","target":"/data","volume":{}}]`, "ok"},
		{`,"volumes":[{"type":"tmpfs","target":"/tmp"}]`, "ok"},
		{`,"cap_add":["NET_ADMIN"]`, "compose_capability: web: NET_ADMIN"},
		{`,"cap_add":["SYS_ADMIN"]`, "compose_capability: web: SYS_ADMIN"},
		{`,"cap_add":[]`, "ok"},
		{`,"cap_drop":["ALL"]`, "ok"},
		{`,"devices":[{"source":"/dev/kvm","target":"/dev/kvm","permissions":"rwm"}]`, "compose_device: web"},
		{`,"devices":["/dev/fuse:/dev/fuse"]`, "compose_device: web"},
		{`,"devices":[]`, "ok"},
		{`,"device_cgroup_rules":["c 10:229 rwm"]`, "compose_device: web"},
		{`,"security_opt":["seccomp:unconfined"]`, "compose_security_opt: web: seccomp:unconfined"},
		{`,"security_opt":["apparmor=unconfined"]`, "compose_security_opt: web: apparmor=unconfined"},
		{`,"security_opt":["label:disable"]`, "compose_security_opt: web: label:disable"},
		{`,"security_opt":["no-new-privileges:true"]`, "ok"},
		{`,"security_opt":["no-new-privileges"]`, "ok"},
		{`,"sysctls":{"net.ipv4.ip_forward":"1"}`, "compose_sysctl: web: net.ipv4.ip_forward"},
		{`,"sysctls":{}`, "ok"},
		{`,"userns_mode":"host"`, "compose_host_namespace: web: userns_mode: host"},
		{`,"cgroup":"host"`, "compose_host_namespace: web: cgroup: host"},
		{`,"cgroup":"private"`, "ok"},
	} {
		if got := code(CheckConfig(service(c.fields))); got != c.want {
			t.Errorf("%s:\n got  %s\n want %s", c.fields, got, c.want)
		}
	}
	for _, c := range []struct{ top, want string }{
		{`"volumes":{"d":{"name":"panel-x_d","driver":"local","driver_opts":{"device":"/etc","o":"bind","type":"none"}}}`, "compose_forbidden_mount: volume d: /etc"},
		{`"volumes":{"d":{"name":"panel-x_d","driver":"local","driver_opts":{"device":"/dev/sdb1","type":"ext4"}}}`, "compose_forbidden_mount: volume d: /dev/sdb1"},
		{`"volumes":{"d":{"name":"panel-x_d","driver":"local","driver_opts":{"device":"/srv/data","o":"bind","type":"none"}}}`, "ok"},
		{`"volumes":{"d":{"name":"panel-x_d","driver_opts":{"device":":/export","o":"addr=10.0.0.2","type":"nfs"}}}`, "ok"},
		{`"secrets":{"s":{"name":"panel-x_s","file":"/root/.ssh/id_rsa"}}`, "compose_forbidden_mount: secret s: /root/.ssh/id_rsa"},
		{`"secrets":{"s":{"name":"panel-x_s","environment":"TOKEN"}}`, "ok"},
		{`"configs":{"c":{"name":"panel-x_c","file":"/etc/shadow"}}`, "compose_forbidden_mount: config c: /etc/shadow"},
		{`"configs":{"c":{"name":"panel-x_c","file":"/var/lib/panel-compose/x/app.conf"}}`, "ok"},
	} {
		data := `{"name":"panel-x","services":{"web":{"image":"alpine"}},` + c.top + `}`
		if got := code(CheckConfig([]byte(data))); got != c.want {
			t.Errorf("%s:\n got  %s\n want %s", c.top, got, c.want)
		}
	}
	// Services are checked in name order, so the message is stable.
	two := `{"services":{"b":{"privileged":true},"a":{"pid":"host"}}}`
	if got := code(CheckConfig([]byte(two))); got != "compose_host_namespace: a: pid: host" {
		t.Errorf("order: %s", got)
	}
}

func newCompose(t *testing.T, s *script) *Manager {
	t.Helper()
	m := New(s)
	m.composeDir = filepath.Join(t.TempDir(), "compose")
	return m
}

func TestValidateProject(t *testing.T) {
	m := newCompose(t, &script{})
	if err := m.SaveProject("web", "services: {}\n"); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, text string
		create     bool
		want       string
	}{
		{"blog", "services: {}", true, "ok"},
		{"web", "services: {}", false, "ok"},
		{"web", "services: {}", true, "project_exists: web"},
		{"nope", "services: {}", false, "error: " + ErrNoProject.Error()},
		{"a", "x", true, "ok"},
		{strings.Repeat("a", 40), "x", true, "ok"},
		{strings.Repeat("a", 41), "x", true, "invalid_project: " + strings.Repeat("a", 41)},
		{"", "x", true, "invalid_project: "},
		{"Web", "x", true, "invalid_project: Web"},
		{"-web", "x", true, "invalid_project: -web"},
		{"my_app", "x", true, "invalid_project: my_app"},
		{"../etc", "x", true, "invalid_project: ../etc"},
		{"blog", " \n\t", true, "compose_empty: "},
		{"blog", strings.Repeat("#", MaxComposeSize+1), true, "compose_too_large: "},
	} {
		if got := code(m.ValidateProject(c.name, c.text, c.create)); got != c.want {
			t.Errorf("%q create=%v: got %s, want %s", c.name, c.create, got, c.want)
		}
	}
}

func TestSaveAndCheck(t *testing.T) {
	safe, unsafe := "", ""
	s := &script{}
	m := newCompose(t, s)
	s.respond = func(cmd string) (string, error) {
		if !strings.Contains(cmd, " config --format json") {
			return "", nil
		}
		// Compose reads the draft; answer as Compose would for its text.
		file := cmd[strings.Index(cmd, " -f ")+4 : strings.Index(cmd, " -p ")]
		b, err := os.ReadFile(file)
		if err != nil {
			return "", failed("open " + file + ": no such file or directory")
		}
		switch string(b) {
		case "safe":
			return safe, nil
		case "unsafe":
			return unsafe, nil
		}
		return "", failed("time=\"2026-11-25T10:00:00Z\" level=warning msg=\"x\"\nyaml: line 2: mapping values are not allowed in this context in " + file)
	}
	noLinks(t, nil)
	safe, unsafe = fixture(t, "config-safe.json"), fixture(t, "config-unsafe.json")

	if err := m.CheckText(context.Background(), "demo", "safe"); err != nil {
		t.Fatalf("safe draft refused: %v", err)
	}
	if got := code(m.CheckText(context.Background(), "demo", "unsafe")); !strings.HasPrefix(got, "compose_forbidden_mount: ") {
		t.Errorf("unsafe draft: %s", got)
	}
	got := code(m.CheckText(context.Background(), "demo", "bad: : yaml"))
	if !strings.HasPrefix(got, "compose_invalid: ") || !strings.Contains(got, "mapping values are not allowed in this context in compose.yaml") {
		t.Errorf("invalid draft: %s", got)
	}
	// Drafts are checked as if they were in the project's folder, and
	// removed afterwards; nothing is saved.
	dir := filepath.Join(m.composeDir, "demo")
	want := "docker compose --ansi never --project-directory " + dir + " -f " + m.composeDir + string(filepath.Separator) + ".check-"
	if !strings.HasPrefix(s.calls[0], want) || !strings.HasSuffix(s.calls[0], " -p panel-demo config --format json") {
		t.Errorf("checked with %q", s.calls[0])
	}
	if left, _ := os.ReadDir(m.composeDir); len(left) != 0 {
		t.Errorf("left behind: %v", left)
	}
	if m.projectExists("demo") {
		t.Error("check saved the project")
	}

	// Save, then up: the saved file is checked again before starting.
	if err := m.SaveProject("demo", "safe"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(m.projectFile("demo"))
	if err != nil || string(b) != "safe" {
		t.Fatalf("saved %q, %v", b, err)
	}
	if text, _ := m.ProjectFile("demo"); text != "safe" {
		t.Errorf("ProjectFile = %q", text)
	}
	s.calls = nil
	lg := &testLog{}
	if err := m.Up(context.Background(), "demo", lg); err != nil {
		t.Fatal(err)
	}
	base := "docker compose --ansi never --project-directory " + dir + " -f " + m.projectFile("demo") + " -p panel-demo "
	if len(s.calls) != 2 || s.calls[0] != base+"config --format json" || s.calls[1] != base+"up --detach --remove-orphans" {
		t.Errorf("up ran %q", s.calls)
	}

	// A saved file that has become unsafe is not started.
	if err := m.SaveProject("demo", "unsafe"); err != nil {
		t.Fatal(err)
	}
	s.calls = nil
	if got := code(m.ProjectAction(context.Background(), "demo", "start", lg)); !strings.HasPrefix(got, "compose_forbidden_mount") {
		t.Errorf("start of unsafe project: %s", got)
	}
	if s.ran(" up ") {
		t.Error("unsafe project started")
	}
	s.calls = nil
	if got := code(m.ProjectAction(context.Background(), "demo", "update", lg)); !strings.HasPrefix(got, "compose_forbidden_mount") || s.ran(" pull") {
		t.Errorf("update of unsafe project: %s, ran %q", got, s.calls)
	}
}

func TestProjectActions(t *testing.T) {
	s := &script{respond: func(cmd string) (string, error) {
		if strings.Contains(cmd, " config ") {
			return `{"services":{"web":{"image":"traefik/whoami"}}}`, nil
		}
		return "", nil
	}}
	m := newCompose(t, s)
	if err := m.SaveProject("demo", "services: {}"); err != nil {
		t.Fatal(err)
	}
	lg := &testLog{}
	for action, want := range map[string][]string{
		"start":   {"config --format json", "up --detach --remove-orphans"},
		"stop":    {"stop"},
		"restart": {"restart"},
		"down":    {"down --remove-orphans"},
		"update":  {"config --format json", "pull", "config --format json", "up --detach --remove-orphans"},
	} {
		s.calls = nil
		if err := m.ProjectAction(context.Background(), "demo", action, lg); err != nil {
			t.Errorf("%s: %v", action, err)
			continue
		}
		var got []string
		for _, c := range s.calls {
			got = append(got, c[strings.Index(c, " -p panel-demo ")+len(" -p panel-demo "):])
		}
		if strings.Join(got, "; ") != strings.Join(want, "; ") {
			t.Errorf("%s ran %q, want %q", action, got, want)
		}
	}
	if got := code(m.ProjectAction(context.Background(), "demo", "exec", lg)); got != "action_not_allowed: exec" {
		t.Errorf("exec: %s", got)
	}
	if !errors.Is(m.ProjectAction(context.Background(), "other", "stop", lg), ErrNoProject) {
		t.Error("action on a missing project")
	}

	// Delete keeps volumes unless asked, and removes the folder.
	s.calls = nil
	if err := m.DeleteProject(context.Background(), "demo", false, lg); err != nil {
		t.Fatal(err)
	}
	if len(s.calls) != 1 || !strings.HasSuffix(s.calls[0], " -p panel-demo down --remove-orphans") {
		t.Errorf("delete ran %q", s.calls)
	}
	if _, err := os.Stat(m.projectDir("demo")); !os.IsNotExist(err) {
		t.Errorf("folder still there: %v", err)
	}
	m.SaveProject("demo", "services: {}")
	s.calls = nil
	if err := m.DeleteProject(context.Background(), "demo", true, lg); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(s.calls[0], " down --remove-orphans --volumes") {
		t.Errorf("delete with volumes ran %q", s.calls)
	}

	// A failed down keeps the folder, so the project can still be managed.
	m.SaveProject("demo", "services: {}")
	s.respond = func(string) (string, error) { return "", failed("Cannot connect to the Docker daemon") }
	if m.DeleteProject(context.Background(), "demo", false, lg) == nil || !m.projectExists("demo") {
		t.Error("failed down removed the project")
	}
}

func TestProjectsAndContainers(t *testing.T) {
	s := &script{}
	m := newCompose(t, s)
	for _, n := range []string{"blog", "shop", "idle"} {
		if err := m.SaveProject(n, "services: {}"); err != nil {
			t.Fatal(err)
		}
	}
	os.MkdirAll(filepath.Join(m.composeDir, "empty"), 0o700)               // no compose.yaml
	os.WriteFile(filepath.Join(m.composeDir, ".check-1.yaml"), nil, 0o600) // a draft being checked
	s.respond = func(cmd string) (string, error) {
		switch {
		case cmd == "docker compose ls --all --format json":
			return `[{"Name":"panel-blog","Status":"running(2)","ConfigFiles":"/var/lib/panel-compose/blog/compose.yaml"},` +
				`{"Name":"panel-shop","Status":"exited(1), running(2)","ConfigFiles":"/var/lib/panel-compose/shop/compose.yaml"},` +
				`{"Name":"other","Status":"running(1)","ConfigFiles":"/srv/other/compose.yaml"}]` + "\n", nil
		case strings.HasSuffix(cmd, " ps --all --format json"):
			// Compose 2.21 and later: one object per line.
			return `{"Command":"\"/whoami\"","CreatedAt":"2026-11-25 10:00:00 +0000 UTC","ExitCode":0,"Health":"","ID":"3f4e8a1c2b9d7e6f5a4b3c2d1e0f9a8b7c6d5e4f3a2b1c0d9e8f7a6b5c4d3e2f","Image":"traefik/whoami","Labels":"com.docker.compose.project=panel-blog","LocalVolumes":"0","Mounts":"","Name":"panel-blog-web-1","Names":"panel-blog-web-1","Networks":"panel-blog_default","Ports":"127.0.0.1:18081->80/tcp","Project":"panel-blog","Publishers":[{"URL":"127.0.0.1","TargetPort":80,"PublishedPort":18081,"Protocol":"tcp"}],"RunningFor":"5 minutes ago","Service":"web","Size":"0B","State":"running","Status":"Up 5 minutes"}` + "\n" +
				`{"Command":"\"sleep infinity\"","ExitCode":137,"Health":"","ID":"9a8b7c6d5e4f","Image":"alpine","Name":"panel-blog-worker-1","Ports":"","Project":"panel-blog","Publishers":[],"Service":"worker","State":"exited","Status":"Exited (137) 1 minute ago"}` + "\n", nil
		}
		return "", nil
	}
	list, err := m.Projects(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, p := range list {
		got = append(got, fmt.Sprintf("%s %s %d/%d", p.Name, p.State, p.Running, p.Containers))
	}
	if strings.Join(got, ", ") != "blog running 2/2, idle down 0/0, shop partial 2/3" {
		t.Errorf("projects: %s", strings.Join(got, ", "))
	}

	cs, err := m.ProjectContainers(context.Background(), "blog")
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) != 2 || cs[0].ID != "3f4e8a1c2b9d" || cs[0].Service != "web" || cs[0].Ports != "127.0.0.1:18081->80/tcp" ||
		cs[1].State != "exited" || cs[1].Ports != "" {
		t.Errorf("containers: %+v", cs)
	}
	// Before Compose 2.21: a JSON array, and no Publishers.
	old, err := parseComposePs(`[{"ID":"abc","Name":"panel-blog-web-1","Service":"web","State":"running","Ports":"127.0.0.1:18081->80/tcp"}]`)
	if err != nil || len(old) != 1 || old[0].Ports != "127.0.0.1:18081->80/tcp" {
		t.Errorf("old ps format: %+v, %v", old, err)
	}
	if empty, err := parseComposePs(""); err != nil || len(empty) != 0 {
		t.Errorf("no containers: %+v, %v", empty, err)
	}

	// Without the Compose plugin the projects are still listed.
	s.respond = func(string) (string, error) { return "", failed("docker: 'compose' is not a docker command.") }
	list, err = m.Projects(context.Background())
	if err != nil || len(list) != 3 || list[0].State != "unknown" {
		t.Errorf("without compose: %+v, %v", list, err)
	}
	// Nothing saved yet.
	m.composeDir = filepath.Join(t.TempDir(), "missing")
	if list, err := m.Projects(context.Background()); err != nil || len(list) != 0 {
		t.Errorf("no folder: %+v, %v", list, err)
	}
}

func TestCountStatus(t *testing.T) {
	for s, want := range map[string]string{
		"":                               "0/0 down",
		"running(3)":                     "3/3 running",
		"exited(2)":                      "0/2 stopped",
		"running(1), exited(1)":          "1/2 partial",
		"created(1)":                     "0/1 stopped",
		"paused(1), restarting(1)":       "0/2 stopped",
		"exited(1), running(2), dead(1)": "2/4 partial",
	} {
		r, n := countStatus(s)
		if got := fmt.Sprintf("%d/%d %s", r, n, projectState(r, n)); got != want {
			t.Errorf("%q: got %s, want %s", s, got, want)
		}
	}
}

func TestEnsureCompose(t *testing.T) {
	installed := false
	dockerCE := false
	s := &script{}
	s.respond = func(cmd string) (string, error) {
		switch {
		case cmd == "docker compose version":
			if installed {
				return "Docker Compose version 2.40.3", nil
			}
			return "", failed("docker: unknown command: docker compose")
		case strings.HasPrefix(cmd, "dpkg-query"):
			if dockerCE {
				return "install ok installed", nil
			}
			return "", failed("dpkg-query: no packages found matching docker-ce")
		case strings.Contains(cmd, "apt-get") && strings.Contains(cmd, " install "):
			installed = true
		}
		return "", nil
	}
	m := New(s)
	lg := &testLog{}
	if err := m.EnsureCompose(context.Background(), lg); err != nil {
		t.Fatal(err)
	}
	if !s.ran("DPkg::Lock::Timeout=300") || !s.ran(" update") ||
		!strings.HasSuffix(s.calls[len(s.calls)-2], "install -y --no-remove docker-compose-v2") {
		t.Errorf("ran %q", s.calls)
	}
	// Already there: nothing is installed.
	s.calls = nil
	if err := m.EnsureCompose(context.Background(), lg); err != nil || len(s.calls) != 1 {
		t.Errorf("installed again: %q, %v", s.calls, err)
	}
	// Docker from Docker's repository gets Docker's plugin.
	installed, dockerCE, s.calls = false, true, nil
	if err := m.EnsureCompose(context.Background(), lg); err != nil || !s.ran("install -y --no-remove docker-compose-plugin") {
		t.Errorf("docker-ce: %q, %v", s.calls, err)
	}
	// The package installs but Compose still does not work.
	s.respond = func(cmd string) (string, error) {
		if cmd == "docker compose version" {
			return "", failed("unknown command")
		}
		return "", nil
	}
	if err := m.EnsureCompose(context.Background(), lg); err == nil {
		t.Error("broken install accepted")
	}
}

func TestImages(t *testing.T) {
	const whoami = "sha256:1b2c3d4e5f60718293a4b5c6d7e8f90112233445566778899aabbccddeeff00"
	const alpine = "sha256:aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"
	const dangling = "sha256:0f0e0d0c0b0a09080706050403020100f0e0d0c0b0a090807060504030201000"
	s := &script{respond: func(cmd string) (string, error) {
		switch {
		case strings.HasPrefix(cmd, "docker images "):
			return `{"Containers":"N/A","CreatedAt":"2026-10-01 08:00:00 +0000 UTC","CreatedSince":"8 weeks ago","Digest":"\u003cnone\u003e","ID":"` + whoami + `","Repository":"traefik/whoami","SharedSize":"N/A","Size":"9.71MB","Tag":"latest","UniqueSize":"N/A","VirtualSize":"9.71MB"}` + "\n" +
				`{"Containers":"N/A","CreatedAt":"2026-09-01 08:00:00 +0000 UTC","CreatedSince":"3 months ago","ID":"` + alpine + `","Repository":"alpine","Size":"8.3MB","Tag":"3.22"}` + "\n" +
				`{"CreatedSince":"4 months ago","ID":"` + dangling + `","Repository":"\u003cnone\u003e","Size":"120MB","Tag":"\u003cnone\u003e"}` + "\n", nil
		case cmd == "docker ps -a -q --no-trunc":
			return "c1\nc2\nc3\n", nil
		case strings.HasPrefix(cmd, "docker container inspect --format {{.Image}} {{.Name}} c1 c2 c3"):
			return whoami + " /panel-demo-web-1\n" + whoami + " /api\n" + alpine + " /tools\n", nil
		case strings.HasPrefix(cmd, "docker image inspect --format {{.Id}} "):
			switch strings.TrimPrefix(cmd, "docker image inspect --format {{.Id}} ") {
			case "traefik/whoami:latest":
				return whoami + "\n", nil
			case dangling:
				return dangling + "\n", nil
			}
			return "", failed("Error: No such image")
		}
		return "", nil
	}}
	m := New(s)
	list, err := m.Images(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, i := range list {
		got = append(got, fmt.Sprintf("%s %s %s %v %v", i.ID, i.Ref, i.Created, i.InUse, i.UsedBy))
	}
	want := []string{
		"1b2c3d4e5f60 traefik/whoami:latest 8 weeks ago true [api panel-demo-web-1]",
		"aabbccddeeff alpine:3.22 3 months ago true [tools]",
		"0f0e0d0c0b0a " + dangling + " 4 months ago false []",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("images:\n%s", strings.Join(got, "\n"))
	}
	if at := list[0].CreatedAt; at == nil || at.Format(time.RFC3339) != "2026-10-01T08:00:00Z" || list[2].CreatedAt != nil {
		t.Errorf("created at: %v, %v", list[0].CreatedAt, list[2].CreatedAt)
	}

	if got := code(m.RemoveImage(context.Background(), "traefik/whoami:latest", false)); got != "image_in_use: traefik/whoami:latest (api, panel-demo-web-1)" {
		t.Errorf("in use: %s", got)
	}
	if s.ran("docker image rm") {
		t.Error("removed an image in use")
	}
	if err := m.RemoveImage(context.Background(), "traefik/whoami:latest", true); err != nil || !s.ran("docker image rm --force traefik/whoami:latest") {
		t.Errorf("forced: %v %q", err, s.calls)
	}
	if err := m.RemoveImage(context.Background(), dangling, false); err != nil || !s.ran("docker image rm "+dangling) {
		t.Errorf("unused: %v", err)
	}
	if got := code(m.RemoveImage(context.Background(), "nothere:1", false)); got != "image_not_found: nothere:1" {
		t.Errorf("missing: %s", got)
	}
	for _, bad := range []string{"", "-f", "--force", "a b", "x;rm", "../x", "$(id)"} {
		if got := code(m.RemoveImage(context.Background(), bad, true)); !strings.HasPrefix(got, "invalid_image") {
			t.Errorf("%q: %s", bad, got)
		}
	}
	for _, ok := range []string{"nginx", "nginx:1.27-alpine", "ghcr.io/org/app:v1", "localhost:5000/app:dev", "app@sha256:" + strings.Repeat("a", 64), dangling} {
		if !imageRefRe.MatchString(ok) {
			t.Errorf("%q refused", ok)
		}
	}

	s.calls = nil
	if err := m.PruneImages(context.Background(), io.Discard); err != nil || s.calls[0] != "docker image prune --all --force" {
		t.Errorf("prune ran %q, %v", s.calls, err)
	}

	// No containers: nothing is in use and inspect is not run.
	s2 := &script{respond: func(cmd string) (string, error) {
		if strings.HasPrefix(cmd, "docker images ") {
			return `{"ID":"` + alpine + `","Repository":"alpine","Tag":"3.22","Size":"8.3MB"}`, nil
		}
		return "", nil
	}}
	list, err = New(s2).Images(context.Background())
	if err != nil || len(list) != 1 || list[0].InUse || s2.ran("container inspect") {
		t.Errorf("no containers: %+v, %v, %q", list, err, s2.calls)
	}
}

func TestShell(t *testing.T) {
	bash, running := true, "true"
	s := &script{respond: func(cmd string) (string, error) {
		switch {
		case strings.HasPrefix(cmd, "docker container inspect"):
			if strings.HasSuffix(cmd, " gone") {
				return "", failed("Error response from daemon: No such container: gone")
			}
			return running + " /panel-demo-web-1\n", nil
		case strings.HasSuffix(cmd, "test -x /bin/bash"):
			if bash {
				return "", nil
			}
			return "", failed("")
		}
		return "", nil
	}}
	m := New(s)
	argv, name, err := m.Shell(context.Background(), "panel-demo-web-1")
	if err != nil || name != "panel-demo-web-1" ||
		strings.Join(argv, " ") != "docker exec -it -e TERM=xterm-256color panel-demo-web-1 /bin/bash" {
		t.Errorf("bash: %q %q %v", argv, name, err)
	}
	if !s.ran("docker container inspect --format {{.State.Running}} {{.Name}} panel-demo-web-1") ||
		!s.ran("docker exec panel-demo-web-1 test -x /bin/bash") {
		t.Errorf("ran %q", s.calls)
	}
	bash = false
	if argv, _, _ := m.Shell(context.Background(), "3f4e8a1c2b9d"); argv[len(argv)-1] != "/bin/sh" {
		t.Errorf("no bash: %q", argv)
	}
	running = "false"
	if got := code(firstErr(m.Shell(context.Background(), "web"))); got != "container_not_running: panel-demo-web-1" {
		t.Errorf("stopped: %s", got)
	}
	if got := code(firstErr(m.Shell(context.Background(), "gone"))); got != "container_not_found: gone" {
		t.Errorf("gone: %s", got)
	}
	s.calls = nil
	for _, bad := range []string{"", "-it", "--privileged", "a b", "web;id", "../x"} {
		if got := code(firstErr(m.Shell(context.Background(), bad))); !strings.HasPrefix(got, "invalid_container") {
			t.Errorf("%q: %s", bad, got)
		}
	}
	if len(s.calls) != 0 {
		t.Errorf("ran docker for invalid names: %q", s.calls)
	}
}

func firstErr(_ []string, _ string, err error) error { return err }
