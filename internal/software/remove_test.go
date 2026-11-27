package software

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// fakeRunner answers commands from canned output (first matching prefix of
// the command line) and records them.
type fakeRunner struct {
	answers map[string]string
	fail    map[string]bool // command-line prefixes that fail
	cmds    []string        // Output calls
	apt     []string        // apt-get runs (verb and packages)
}

func (f *fakeRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	line := strings.Join(append([]string{name}, args...), " ")
	f.cmds = append(f.cmds, line)
	for p := range f.fail {
		if strings.HasPrefix(line, p) {
			return "", errors.New("exit status 1")
		}
	}
	best := ""
	for p := range f.answers {
		if strings.HasPrefix(line, p) && len(p) > len(best) {
			best = p
		}
	}
	return f.answers[best], nil
}

func (f *fakeRunner) Stream(ctx context.Context, w io.Writer, name string, args ...string) error {
	if name == "/bin/sh" && len(args) > 11 && args[2] == "apt-get" {
		f.apt = append(f.apt, strings.Join(args[11:], " "))
	}
	return nil
}
func (f *fakeRunner) Input(context.Context, []byte, string, ...string) (string, error) {
	return "", nil
}
func (f *fakeRunner) Pipe(context.Context, io.Reader, io.Writer, string, ...string) error {
	return nil
}

type nopLog struct{ strings.Builder }

func (l *nopLog) Logf(format string, args ...any) {}

func newTestManager(t *testing.T) (*Manager, *fakeRunner, *store.Store) {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	f := &fakeRunner{answers: map[string]string{}, fail: map[string]bool{}}
	return New(f, st, func(context.Context) string { return "8.3" }), f, st
}

// sandbox points every path the removals use into a temporary folder.
func sandbox(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := []string{redisBin, dockerBin, nodeLinkDir, optDir, composeDir, dockerContainersDir, phpEtc}
	oldData := dockerDataDirs
	// Memcached too: on a machine that has it, PHP installs would add
	// phpX-memcached (GitHub's runners, for one).
	oldMC, oldMCConf := memcachedBin, memcachedConf
	memcachedBin, memcachedConf = filepath.Join(dir, "usr/bin/memcached"), filepath.Join(dir, "etc/memcached.conf")
	redisBin = filepath.Join(dir, "usr/bin/redis-server")
	dockerBin = filepath.Join(dir, "usr/bin/docker")
	nodeLinkDir = filepath.Join(dir, "usr/local/bin")
	optDir = filepath.Join(dir, "opt")
	composeDir = filepath.Join(dir, "var/lib/panel-compose")
	dockerDataDirs = []string{filepath.Join(dir, "var/lib/docker"), filepath.Join(dir, "var/lib/containerd")}
	dockerContainersDir = filepath.Join(dir, "var/lib/docker/containers")
	phpEtc = filepath.Join(dir, "etc/php")
	for _, d := range []string{"usr/bin", "usr/local/bin", "opt", "var/lib"} {
		os.MkdirAll(filepath.Join(dir, d), 0o755)
	}
	// apt's sources and keyrings (phprepo.go), and no downloads from
	// packages.sury.org: nothing listens there.
	oldRoot, oldSury := sysRoot, suryBase
	sysRoot, suryBase = filepath.Join(dir, "root"), "http://127.0.0.1:1/"
	for _, d := range []string{"etc/apt/sources.list.d", "etc/apt/trusted.gpg.d", "usr/share/keyrings", "var/lib/apt/lists"} {
		os.MkdirAll(filepath.Join(sysRoot, d), 0o755)
	}
	t.Cleanup(func() {
		redisBin, dockerBin, nodeLinkDir, optDir, composeDir, dockerContainersDir, phpEtc = old[0], old[1], old[2], old[3], old[4], old[5], old[6]
		dockerDataDirs = oldData
		memcachedBin, memcachedConf = oldMC, oldMCConf
		sysRoot, suryBase = oldRoot, oldSury
	})
	return dir
}

func touch(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func code(err error) string {
	var ue *usererr.Error
	if errors.As(err, &ue) {
		return ue.Code + ":" + ue.Detail
	}
	if err != nil {
		return "error:" + err.Error()
	}
	return ""
}

func addSite(t *testing.T, st *store.Store, s *store.Site) {
	t.Helper()
	if s.SysUser == "" {
		s.SysUser = strings.ReplaceAll(s.Domain, ".", "")
	}
	// CreateSite stores the basics; the rest (App, AppRuntime) is an update.
	if err := st.CreateSite(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateSite(context.Background(), s); err != nil {
		t.Fatal(err)
	}
}

func TestBlockers(t *testing.T) {
	if code(phpBlocker("8.2", 1, 3)) != "php_in_use:8.2" || code(phpBlocker("8.3", 0, 1)) != "php_last:8.3" || phpBlocker("8.2", 0, 2) != nil {
		t.Error("PHP guard")
	}
	sites := []*store.Site{
		{Domain: "node.test", ProxyPort: 3000},                       // default node
		{Domain: "n22.test", ProxyPort: 3001, AppRuntime: "node:22"}, // its own line
		{Domain: "ghost.test", ProxyPort: 3002, App: "docker:ghost"}, // brings its own
		{Domain: "py.test", ProxyPort: 3003, AppRuntime: "python"},
		{Domain: "php.test", PHPVersion: "8.3"},
	}
	if n := defaultNodeUsers(sites); n != 1 {
		t.Errorf("default node users = %d", n)
	}
	if code(nodeBlocker(22, 1)) != "node_in_use:22" || nodeBlocker(22, 0) != nil {
		t.Error("node guard")
	}
	if n := pythonUsers(sites); n != 1 || code(pythonBlocker(n)) != "python_in_use:1" {
		t.Error("python guard")
	}
	for u, want := range map[dockerUse]string{
		{apps: 1, projects: 2, containers: 5}: "docker_apps_in_use:1",
		{projects: 2, containers: 5}:          "docker_compose_in_use:2",
		{containers: 5}:                       "docker_in_use:5",
		{}:                                    "",
	} {
		if got := code(u.blocker()); got != want {
			t.Errorf("%+v: %q, want %q", u, got, want)
		}
	}
}

func TestDockerUsage(t *testing.T) {
	dir := sandbox(t)
	m, f, st := newTestManager(t)
	ctx := context.Background()
	f.answers["systemctl is-active docker"] = "active\n"
	f.answers["docker ps -aq"] = strings.Repeat("a", 64) + "\n" + strings.Repeat("b", 64) + "\n"
	os.MkdirAll(filepath.Join(composeDir, "blog"), 0o755)
	os.MkdirAll(filepath.Join(composeDir, ".tmp-x"), 0o755) // not a project
	addSite(t, st, &store.Site{Domain: "ghost.test", ProxyPort: 3002, App: "docker:ghost"})
	sites, _ := st.ListSites(ctx)
	if u := m.dockerUsage(ctx, sites); u != (dockerUse{apps: 1, projects: 1, containers: 2}) {
		t.Fatalf("usage %+v", u)
	}
	// Docker's service is stopped: the containers' folders count, and the
	// docker command, which would start the service, is not run.
	f.answers["systemctl is-active docker"] = "inactive\n"
	f.cmds = nil
	os.MkdirAll(filepath.Join(dockerContainersDir, strings.Repeat("c", 64)), 0o755)
	os.MkdirAll(filepath.Join(dockerContainersDir, "not-a-container"), 0o755)
	if n := m.dockerContainers(ctx); n != 1 {
		t.Fatalf("containers from folders = %d", n)
	}
	for _, c := range f.cmds {
		if strings.HasPrefix(c, "docker ") {
			t.Fatalf("ran %q while Docker is stopped", c)
		}
	}

	touch(t, dockerBin)
	if err := m.CheckRemove(ctx, "docker", RemoveOptions{}); code(err) != "docker_apps_in_use:1" {
		t.Fatalf("with an app site: %v", err)
	}
	if err := m.CheckRemove(ctx, "docker", RemoveOptions{DeleteData: true, Confirm: "yes"}); code(err) != "software_confirm_mismatch:docker" {
		t.Fatalf("bad confirm: %v", err)
	}
	_ = dir
}

func TestRemoveDockerKeepsData(t *testing.T) {
	sandbox(t)
	m, f, _ := newTestManager(t)
	ctx := context.Background()
	touch(t, dockerBin)
	touch(t, filepath.Join(dockerDataDirs[0], "volumes", "db", "data"))
	touch(t, filepath.Join(dockerDataDirs[0], "nuke-graph-directory.sh"))
	f.fail["docker ps"] = true // no containers either way
	f.answers["dpkg-query"] = "docker.io\tinstalled\ndocker-compose-v2\tinstalled\ndocker-ce\tnot-installed\n"
	f.answers["apt-get -s -q remove"] = "Remv docker.io [1]\nRemv docker-compose-v2 [1]\n"
	if err := m.Remove(ctx, "docker", RemoveOptions{}, &nopLog{}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(f.apt, "|") != "remove docker-compose-v2 docker.io" {
		t.Fatalf("apt %q", f.apt)
	}
	if !exists(filepath.Join(dockerDataDirs[0], "volumes", "db", "data")) {
		t.Fatal("volumes deleted without being asked")
	}
	if exists(filepath.Join(dockerDataDirs[0], "nuke-graph-directory.sh")) {
		t.Fatal("the purge script that deletes the data was left behind")
	}
}

func TestRemoveDockerDeletesDataWhenAsked(t *testing.T) {
	sandbox(t)
	m, f, _ := newTestManager(t)
	ctx := context.Background()
	touch(t, dockerBin)
	touch(t, filepath.Join(dockerDataDirs[0], "volumes", "db", "data"))
	touch(t, filepath.Join(dockerDataDirs[1], "x"))
	f.fail["docker ps"] = true
	f.answers["dpkg-query"] = "docker.io\tinstalled\n"
	f.answers["apt-get -s -q purge"] = "Purg docker.io [1]\n"
	if err := m.Remove(ctx, "docker", RemoveOptions{DeleteData: true, Confirm: "docker"}, &nopLog{}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(f.apt, "|") != "purge docker.io" {
		t.Fatalf("apt %q", f.apt)
	}
	if exists(dockerDataDirs[0]) || exists(dockerDataDirs[1]) {
		t.Fatal("data folders still there")
	}
}

func TestRemoveDockerRefusedWithContainers(t *testing.T) {
	sandbox(t)
	m, f, _ := newTestManager(t)
	touch(t, dockerBin)
	f.answers["systemctl is-active docker"] = "active\n"
	f.answers["docker ps -aq"] = strings.Repeat("d", 64) + "\n"
	err := m.Remove(context.Background(), "docker", RemoveOptions{}, &nopLog{})
	if code(err) != "docker_in_use:1" || len(f.apt) != 0 {
		t.Fatalf("err %v, apt %q", err, f.apt)
	}
}

func TestRemoveRedisRestartsPHP(t *testing.T) {
	sandbox(t)
	m, f, _ := newTestManager(t)
	touch(t, redisBin)
	touch(t, filepath.Join(phpEtc, "8.3", "fpm", "php-fpm.conf"))
	f.answers["dpkg-query"] = "redis-server\tinstalled\nredis-tools\tinstalled\nphp8.3-redis\tinstalled\nphp8.1-redis\tconfig-files\n"
	f.answers["apt-get -s -q purge"] = "Purg php8.1-redis\nPurg php8.3-redis [6]\nPurg redis-server [7]\nPurg redis-tools [7]\n"
	if err := m.Remove(context.Background(), "redis", RemoveOptions{}, &nopLog{}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(f.apt, "|") != "purge php8.1-redis php8.3-redis redis-server redis-tools" {
		t.Fatalf("apt %q", f.apt)
	}
	restarts := []string{}
	for _, c := range f.cmds {
		if strings.HasPrefix(c, "systemctl restart") {
			restarts = append(restarts, c)
		}
	}
	// PHP 8.1 is gone (only its configuration was left): nothing to restart.
	if strings.Join(restarts, "|") != "systemctl restart php8.3-fpm" {
		t.Fatalf("restarts %q", restarts)
	}
}

func TestRemovePHPGuards(t *testing.T) {
	sandbox(t)
	m, f, st := newTestManager(t)
	ctx := context.Background()
	touch(t, filepath.Join(phpEtc, "8.3", "fpm", "php-fpm.conf"))
	if err := m.Remove(ctx, "php8.3", RemoveOptions{}, &nopLog{}); code(err) != "php_last:8.3" {
		t.Fatalf("last version: %v", err)
	}
	touch(t, filepath.Join(phpEtc, "8.2", "fpm", "php-fpm.conf"))
	addSite(t, st, &store.Site{Domain: "old.test", PHPVersion: "8.2"})
	if err := m.Remove(ctx, "php8.2", RemoveOptions{}, &nopLog{}); code(err) != "php_in_use:8.2" {
		t.Fatalf("used version: %v", err)
	}
	if err := m.Remove(ctx, "php7.4", RemoveOptions{}, &nopLog{}); code(err) != "software_not_installed:PHP 7.4" {
		t.Fatalf("not installed: %v", err)
	}
	f.answers["dpkg-query"] = "php8.3\tinstalled\nphp8.3-fpm\tinstalled\nphp8.3-cli\tinstalled\n"
	f.answers["apt-get -s -q purge"] = "Purg php8.3 [1]\nPurg php8.3-cli [1]\nPurg php8.3-fpm [1]\n"
	if err := m.Remove(ctx, "php8.3", RemoveOptions{}, &nopLog{}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(f.apt, "|") != "purge php8.3 php8.3-cli php8.3-fpm" {
		t.Fatalf("apt %q", f.apt)
	}
}

func TestRemoveDefaultNode(t *testing.T) {
	sandbox(t)
	m, _, st := newTestManager(t)
	ctx := context.Background()
	dir := filepath.Join(optDir, "node-v22.11.0-linux-x64")
	touch(t, filepath.Join(dir, "bin", "node"))
	for _, b := range []string{"node", "npm", "npx"} {
		if err := os.Symlink(filepath.Join(dir, "bin", b), filepath.Join(nodeLinkDir, b)); err != nil {
			t.Skip("symlinks are not available here:", err)
		}
	}
	touch(t, filepath.Join(nodeLinkDir, "other-tool"))
	addSite(t, st, &store.Site{Domain: "app.test", ProxyPort: 3000})
	if err := m.Remove(ctx, "nodejs", RemoveOptions{}, &nopLog{}); code(err) != "node_in_use:22" {
		t.Fatalf("in use: %v", err)
	}
	// The site chooses Node.js 22 itself: node goes, the folder stays.
	sites, _ := st.ListSites(ctx)
	site := sites[0]
	site.AppRuntime = "node:22"
	if err := st.UpdateSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	if err := m.Remove(ctx, "nodejs", RemoveOptions{}, &nopLog{}); err != nil {
		t.Fatal(err)
	}
	if NodeInstalled() || exists(filepath.Join(nodeLinkDir, "npm")) || !exists(filepath.Join(nodeLinkDir, "other-tool")) {
		t.Fatal("links not removed, or too much removed")
	}
	if !exists(dir) {
		t.Fatal("a Node.js line an app chose was deleted")
	}
	// Not installed any more.
	if err := m.CheckRemove(ctx, "nodejs", RemoveOptions{}); code(err) != "software_not_installed:Node.js" {
		t.Fatalf("after removal: %v", err)
	}
	// A node the panel did not install is left alone.
	touch(t, filepath.Join(nodeLinkDir, "node"))
	if err := m.CheckRemove(ctx, "nodejs", RemoveOptions{}); code(err) != "node_not_managed:" {
		t.Fatalf("foreign node: %v", err)
	}
}

func TestRemoveUnknown(t *testing.T) {
	sandbox(t)
	m, _, _ := newTestManager(t)
	for _, id := range []string{"nginx", "php9.9", "php8.2; rm -rf /", "mariadb"} {
		if err := m.CheckRemove(context.Background(), id, RemoveOptions{}); code(err) != "unknown_software:"+id {
			t.Errorf("%s: %v", id, err)
		}
	}
}
