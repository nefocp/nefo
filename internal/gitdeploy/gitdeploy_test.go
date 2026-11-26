package gitdeploy

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/nefocp/nefo/internal/store"
)

// ---- fakes ----

type memStore struct {
	mu    sync.Mutex
	sites map[int64]*store.Site
	git   map[int64]*store.GitDeploy
}

func (s *memStore) SiteByID(_ context.Context, id int64) (*store.Site, error) {
	if site, ok := s.sites[id]; ok {
		c := *site
		return &c, nil
	}
	return nil, store.ErrNotFound
}

func (s *memStore) GitDeploy(_ context.Context, id int64) (*store.GitDeploy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if g, ok := s.git[id]; ok {
		c := *g
		return &c, nil
	}
	return nil, store.ErrNotFound
}

func (s *memStore) SaveGitDeploy(_ context.Context, g *store.GitDeploy) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := *g
	if old, ok := s.git[g.SiteID]; ok {
		c.LastCommit, c.LastDeployAt, c.LastStatus = old.LastCommit, old.LastDeployAt, old.LastStatus
	}
	s.git[g.SiteID] = &c
	return nil
}

func (s *memStore) SetGitDeployResult(_ context.Context, id int64, commit, status string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if g, ok := s.git[id]; ok {
		g.LastCommit, g.LastStatus, g.LastDeployAt = commit, status, at
	}
	return nil
}

func (s *memStore) DeleteGitDeploy(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.git, id)
	return nil
}

// userCmd is a command the fake runner saw, split into its parts.
type userCmd struct {
	runner string // the program run: "runuser", "/bin/sh"…
	user   string
	umask  string   // the shell script runuser starts
	chdir  string   // env --chdir
	env    []string // env's NAME=value arguments
	cmd    []string // the command itself
	method string   // Output or Stream
}

// splitRunuser takes "-u <user> -- sh -c <script> sh [env --chdir=… K=V…] <cmd…>" apart.
func splitRunuser(name, method string, args []string) userCmd {
	c := userCmd{runner: name, method: method}
	if name != "runuser" || len(args) < 7 || args[0] != "-u" || args[2] != "--" || args[3] != "sh" || args[4] != "-c" || args[6] != "sh" {
		c.cmd = args
		return c
	}
	c.user, c.umask = args[1], args[5]
	rest := args[7:]
	if len(rest) > 0 && rest[0] == "env" {
		rest = rest[1:]
		for len(rest) > 0 {
			if d, ok := strings.CutPrefix(rest[0], "--chdir="); ok {
				c.chdir = d
			} else if strings.Contains(rest[0], "=") {
				c.env = append(c.env, rest[0])
			} else {
				break
			}
			rest = rest[1:]
		}
	} else {
		// generateKey: the script is the command, the rest its arguments.
		rest = append([]string{"script:" + args[5]}, rest...)
	}
	c.cmd = rest
	return c
}

func (c userCmd) envValue(name string) string {
	for _, e := range c.env {
		if v, ok := strings.CutPrefix(e, name+"="); ok {
			return v
		}
	}
	return ""
}

type fakeRunner struct {
	mu      sync.Mutex
	calls   []userCmd
	respond func(c userCmd) (string, error)
}

func (f *fakeRunner) do(method, name string, args []string) (string, error) {
	c := splitRunuser(name, method, args)
	f.mu.Lock()
	f.calls = append(f.calls, c)
	f.mu.Unlock()
	if f.respond != nil {
		return f.respond(c)
	}
	return "", nil
}

func (f *fakeRunner) Output(_ context.Context, name string, args ...string) (string, error) {
	return f.do("Output", name, args)
}

func (f *fakeRunner) Input(_ context.Context, _ []byte, name string, args ...string) (string, error) {
	return f.do("Input", name, args)
}

func (f *fakeRunner) Stream(_ context.Context, w io.Writer, name string, args ...string) error {
	out, err := f.do("Stream", name, args)
	io.WriteString(w, out)
	return err
}

func (f *fakeRunner) Pipe(_ context.Context, _ io.Reader, w io.Writer, name string, args ...string) error {
	return f.Stream(context.Background(), w, name, args...)
}

// commands returns the commands run, as "git fetch …" strings.
func (f *fakeRunner) commands() []string {
	var out []string
	for _, c := range f.calls {
		out = append(out, strings.Join(c.cmd, " "))
	}
	return out
}

type testLog struct {
	mu    sync.Mutex
	lines []string
}

func (l *testLog) Logf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

func (l *testLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, string(p))
	return len(p), nil
}

func (l *testLog) String() string { return strings.Join(l.lines, "\n") }

// ---- test setup ----

const (
	testHash     = "7fd1a60b01f91b314f59955a4e4d4e80d8edf11d"
	testRepo     = "https://github.com/octocat/Hello-World.git"
	placeholder  = "<!doctype html><title>example.com</title><p>This " + placeholderMark + ". Upload your website to replace it.</p>"
	testSiteID   = 3
	testUser     = "example_com"
	testDomain   = "example.com"
	gitLogOutput = "warning: something harmless\n" + testHash + "\nMerge pull request #6 from Spaceghost/patch-1\n"
)

type testEnv struct {
	m    *Manager
	st   *memStore
	run  *fakeRunner
	root string
	log  *testLog
}

// newEnv makes a site folder as the panel creates it: public/ with the
// placeholder page, logs/ and tmp/.
func newEnv(t *testing.T, g *store.GitDeploy) *testEnv {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, testDomain)
	for _, d := range []string{"public", "logs", "tmp"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(root, "public", "index.html"), placeholder)
	st := &memStore{
		sites: map[int64]*store.Site{testSiteID: {ID: testSiteID, Domain: testDomain, SysUser: testUser, PHPVersion: "8.3"}},
		git:   map[int64]*store.GitDeploy{},
	}
	if g != nil {
		g.SiteID = testSiteID
		st.git[testSiteID] = g
	}
	f := &fakeRunner{}
	f.respond = func(c userCmd) (string, error) { return defaultRespond(t, c) }
	m := New(f, st, func(d string) string { return filepath.Join(base, d) })
	m.hasGit = func() bool { return true }
	m.now = func() time.Time { return time.Unix(1_800_000_000, 0) }
	return &testEnv{m: m, st: st, run: f, root: root, log: &testLog{}}
}

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0o640); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// defaultRespond acts like git and ssh-keygen would on the files.
func defaultRespond(t *testing.T, c userCmd) (string, error) {
	switch {
	case len(c.cmd) > 1 && c.cmd[0] == "git" && c.cmd[1] == "init":
		os.MkdirAll(filepath.Join(c.chdir, ".git"), 0o750)
	case len(c.cmd) > 1 && c.cmd[0] == "git" && c.cmd[1] == "reset":
		writeFile(t, filepath.Join(c.chdir, "README"), "Hello World!\n")
		return "HEAD is now at 7fd1a60 Merge pull request #6\n", nil
	case len(c.cmd) > 1 && c.cmd[0] == "git" && c.cmd[1] == "log":
		return gitLogOutput, nil
	case len(c.cmd) == 4 && strings.Contains(c.cmd[0], "ssh-keygen"):
		key, comment := c.cmd[2], c.cmd[3]
		pub, _, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		k, _ := ssh.NewPublicKey(pub)
		writeFile(t, key, "PRIVATE KEY")
		writeFile(t, key+".pub", strings.TrimSpace(string(ssh.MarshalAuthorizedKey(k)))+" "+comment+"\n")
	}
	return "", nil
}

func gitOnly(cmds []string) []string {
	var out []string
	for _, c := range cmds {
		if strings.HasPrefix(c, "git ") {
			out = append(out, c)
		}
	}
	return out
}

func checkAsSiteUser(t *testing.T, e *testEnv) {
	t.Helper()
	for _, c := range e.run.calls {
		if c.runner != "runuser" || c.user != testUser {
			t.Errorf("not run as the site user: %s %v", c.runner, c.cmd)
		}
		if strings.Contains(strings.Join(c.cmd, " "), "clean") {
			t.Errorf("git clean would delete uploaded files: %v", c.cmd)
		}
		if c.cmd[0] == "git" || c.cmd[0] == "sh" {
			if c.umask != `umask 027 && exec "$@"` {
				t.Errorf("umask script = %q", c.umask)
			}
			tmp := filepath.Join(e.root, "tmp")
			if c.envValue("HOME") != tmp || c.envValue("TMPDIR") != tmp {
				t.Errorf("HOME = %q, TMPDIR = %q", c.envValue("HOME"), c.envValue("TMPDIR"))
			}
			if c.envValue("GIT_ALLOW_PROTOCOL") != "https:ssh" || c.envValue("GIT_TERMINAL_PROMPT") != "0" {
				t.Errorf("git environment: %v", c.env)
			}
			sshCmd := c.envValue("GIT_SSH_COMMAND")
			key := filepath.Join(e.root, ".ssh", "panel_deploy")
			for _, want := range []string{"ssh ", "-i '" + key + "'", "-o IdentitiesOnly=yes",
				"-o UserKnownHostsFile='" + filepath.Join(e.root, ".ssh", "known_hosts") + "'",
				"-o StrictHostKeyChecking=accept-new", "-o BatchMode=yes"} {
				if !strings.Contains(sshCmd, want) {
					t.Errorf("GIT_SSH_COMMAND = %q, missing %q", sshCmd, want)
				}
			}
		}
	}
}

// ---- deploys ----

func TestDeployFirstIntoPublic(t *testing.T) {
	e := newEnv(t, &store.GitDeploy{Repo: testRepo, Branch: "master", Target: TargetPublic,
		Script: "composer install --no-dev\nphp artisan migrate --force"})
	commit, err := e.m.Deploy(context.Background(), testSiteID, e.log)
	if err != nil {
		t.Fatalf("Deploy: %v\n%s", err, e.log)
	}
	if commit != testHash {
		t.Fatalf("commit = %q", commit)
	}
	public := filepath.Join(e.root, "public")
	want := []string{
		"git init -q -b master",
		"git remote add origin " + testRepo,
		"git fetch origin +refs/heads/master:refs/remotes/origin/master",
		"git reset --hard origin/master",
		"git log -1 --format=%H%n%s",
		`sh -c mkdir -p -- "$1" && ln -sfn -- "$2" "$1/php" sh ` + filepath.Join(e.root, "tmp", ".panel-bin") + " /usr/bin/php8.3",
		"sh -e -c composer install --no-dev\nphp artisan migrate --force",
	}
	if got := e.run.commands(); !slices.Equal(got, want) {
		t.Fatalf("commands:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	checkAsSiteUser(t, e)
	for _, c := range e.run.calls[:5] {
		if c.chdir != public {
			t.Errorf("%v ran in %q, want %q", c.cmd, c.chdir, public)
		}
	}
	script := e.run.calls[6]
	if script.chdir != public || script.method != "Stream" {
		t.Errorf("script ran in %q (%s)", script.chdir, script.method)
	}
	if script.envValue("DEPLOY_COMMIT") != testHash || script.envValue("DEPLOY_BRANCH") != "master" {
		t.Errorf("script environment: %v", script.env)
	}
	if p := script.envValue("PATH"); !strings.HasPrefix(p, filepath.Join(e.root, "tmp", ".panel-bin")+":") {
		t.Errorf("PATH = %q: the site's PHP should come first", p)
	}
	if exists(filepath.Join(public, "index.html")) {
		t.Error("the placeholder page was not removed")
	}
	g := e.st.git[testSiteID]
	if g.LastCommit != testHash+" Merge pull request #6 from Spaceghost/patch-1" || g.LastStatus != StatusOK ||
		g.LastDeployAt.Unix() != 1_800_000_000 {
		t.Fatalf("result = %q %q %v", g.LastCommit, g.LastStatus, g.LastDeployAt)
	}
	if !strings.Contains(e.log.String(), "Deployed commit 7fd1a60b01f9: Merge pull request #6") {
		t.Errorf("log:\n%s", e.log)
	}
}

func TestDeployUpdate(t *testing.T) {
	e := newEnv(t, &store.GitDeploy{Repo: "git@github.com:owner/site.git", Branch: "main", Target: TargetPublic,
		LastCommit: "1111111111111111111111111111111111111111 Old", LastStatus: StatusOK})
	public := filepath.Join(e.root, "public")
	os.Remove(filepath.Join(public, "index.html"))
	writeFile(t, filepath.Join(public, ".git", "HEAD"), "ref: refs/heads/main\n")
	writeFile(t, filepath.Join(public, "index.php"), "<?php")
	writeFile(t, filepath.Join(public, "uploads", "photo.jpg"), "jpeg") // untracked: must stay
	writeFile(t, filepath.Join(e.root, ".ssh", "panel_deploy.pub"), "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJj+f3N9robQ21ttGmHatHZ4DMUsubuEgnHEEB1FUv0/ deploy@example.com\n")

	if _, err := e.m.Deploy(context.Background(), testSiteID, e.log); err != nil {
		t.Fatalf("Deploy: %v\n%s", err, e.log)
	}
	want := []string{
		"git remote set-url origin git@github.com:owner/site.git",
		"git fetch origin +refs/heads/main:refs/remotes/origin/main",
		"git reset --hard origin/main",
		"git log -1 --format=%H%n%s",
	}
	if got := e.run.commands(); !slices.Equal(got, want) {
		t.Fatalf("commands:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	checkAsSiteUser(t, e)
	if !exists(filepath.Join(public, "uploads", "photo.jpg")) {
		t.Fatal("an uploaded file was removed")
	}
	if g := e.st.git[testSiteID]; !strings.HasPrefix(g.LastCommit, testHash) || g.LastStatus != StatusOK {
		t.Fatalf("result = %q %q", g.LastCommit, g.LastStatus)
	}
}

func TestDeployUpdateWithoutOrigin(t *testing.T) {
	e := newEnv(t, &store.GitDeploy{Repo: testRepo, Branch: "main", Target: TargetRoot})
	writeFile(t, filepath.Join(e.root, ".git", "HEAD"), "ref: refs/heads/main\n")
	respond := e.run.respond
	e.run.respond = func(c userCmd) (string, error) {
		if strings.Join(c.cmd, " ") == "git remote set-url origin "+testRepo {
			return "error: No such remote 'origin'\n", errors.New("exit status 2")
		}
		return respond(c)
	}
	if _, err := e.m.Deploy(context.Background(), testSiteID, e.log); err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	got := gitOnly(e.run.commands())
	if len(got) < 3 || got[1] != "git remote add origin "+testRepo || !strings.HasPrefix(got[2], "git fetch ") {
		t.Fatalf("commands: %v", got)
	}
}

func TestDeployFirstIntoRoot(t *testing.T) {
	e := newEnv(t, &store.GitDeploy{Repo: testRepo, Branch: "main", Target: TargetRoot})
	writeFile(t, filepath.Join(e.root, ".ssh", "panel_deploy.pub"), "ssh-ed25519 AAAA deploy@example.com\n")
	writeFile(t, filepath.Join(e.root, "logs", "error.log"), "old log")
	respond := e.run.respond
	e.run.respond = func(c userCmd) (string, error) {
		if len(c.cmd) > 1 && c.cmd[1] == "reset" {
			writeFile(t, filepath.Join(c.chdir, "public", "index.php"), "<?php // Laravel")
			writeFile(t, filepath.Join(c.chdir, "artisan"), "#!/usr/bin/env php")
		}
		return respond(c)
	}
	if _, err := e.m.Deploy(context.Background(), testSiteID, e.log); err != nil {
		t.Fatalf("Deploy: %v\n%s", err, e.log)
	}
	got := e.run.commands()
	if len(got) != 5 || got[0] != "git init -q -b main" || got[3] != "git reset --hard origin/main" {
		t.Fatalf("commands: %v", got)
	}
	for _, c := range e.run.calls {
		if c.chdir != e.root {
			t.Errorf("%v ran in %q, want the site folder", c.cmd, c.chdir)
		}
	}
	if exists(filepath.Join(e.root, "public", "index.html")) {
		t.Error("the placeholder page was not removed")
	}
	if !exists(filepath.Join(e.root, "logs", "error.log")) {
		t.Error("logs were touched")
	}
}

func TestDeployRootWithoutPublicKeepsPlaceholder(t *testing.T) {
	e := newEnv(t, &store.GitDeploy{Repo: testRepo, Branch: "main", Target: TargetRoot})
	if _, err := e.m.Deploy(context.Background(), testSiteID, e.log); err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if !exists(filepath.Join(e.root, "public", "index.html")) {
		t.Error("the placeholder was removed although the repository has no public/ folder")
	}
	if !strings.Contains(e.log.String(), "has no public/ folder") {
		t.Errorf("no hint in the log:\n%s", e.log)
	}
}

// appEnv is newEnv for a Node.js or Python app site: it has app/, where its
// program runs, and no PHP.
func appEnv(t *testing.T, g *store.GitDeploy) *testEnv {
	t.Helper()
	e := newEnv(t, g)
	site := e.st.sites[testSiteID]
	site.PHPVersion, site.ProxyPort, site.AppCommand = "", 3000, "node server.js"
	if err := os.MkdirAll(filepath.Join(e.root, "app"), 0o750); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestDeployAppSite(t *testing.T) {
	// Saved before app sites had their own folder: it still goes into app/.
	e := appEnv(t, &store.GitDeploy{Repo: testRepo, Branch: "main", Target: TargetPublic, Script: "npm ci"})
	// The script's npm is the app's Node.js version (hosting.RuntimePath).
	e.m.SetAppPath(func(*store.Site) string { return "/opt/node-v22.1.0-linux-x64/bin:" + userPATH })
	app := filepath.Join(e.root, "app")
	writeFile(t, filepath.Join(app, ".venv", "bin", "python"), "") // the panel's virtual environment does not count
	if _, err := e.m.Deploy(context.Background(), testSiteID, e.log); err != nil {
		t.Fatalf("Deploy: %v\n%s", err, e.log)
	}
	got := e.run.commands()
	if len(got) != 6 || got[0] != "git init -q -b main" || got[5] != "sh -e -c npm ci" {
		t.Fatalf("commands: %v", got)
	}
	for _, c := range e.run.calls {
		if c.chdir != app {
			t.Errorf("%v ran in %q, want the app folder", c.cmd, c.chdir)
		}
		if !slices.Contains(c.env, "PATH=/opt/node-v22.1.0-linux-x64/bin:"+userPATH) {
			t.Errorf("%v ran with %v, want the app's PATH", c.cmd, c.env)
		}
	}
	checkAsSiteUser(t, e)
	if !exists(filepath.Join(e.root, "public", "index.html")) || strings.Contains(e.log.String(), "placeholder") {
		t.Errorf("the placeholder page of an app site was touched:\n%s", e.log)
	}

	// The first deploy needs an empty app folder.
	e = appEnv(t, &store.GitDeploy{Repo: testRepo, Branch: "main", Target: TargetRoot})
	writeFile(t, filepath.Join(e.root, "app", "server.js"), "mine")
	if _, err := e.m.Deploy(context.Background(), testSiteID, e.log); err == nil ||
		!strings.Contains(err.Error(), filepath.Join(e.root, "app")+" already has files in it (server.js)") || len(e.run.calls) != 0 {
		t.Fatalf("err = %v, commands %v", err, e.run.commands())
	}

	// The settings: an app site has no choice of folder.
	ctx := context.Background()
	e = appEnv(t, nil)
	app = filepath.Join(e.root, "app")
	in, err := e.m.Info(ctx, testSiteID)
	if err != nil || in.Target != TargetApp || in.Folder != app || in.AppFolder != app {
		t.Fatalf("Info before saving: %+v, %v", in, err)
	}
	in, err = e.m.Save(ctx, testSiteID, Settings{Repo: testRepo, Branch: "main", Target: TargetPublic})
	if err != nil || in.Target != TargetApp || in.Folder != app || e.st.git[testSiteID].Target != TargetApp {
		t.Fatalf("Save: %+v, %v", in, err)
	}
	// Other sites cannot choose app/.
	e = newEnv(t, nil)
	if _, err := e.m.Save(ctx, testSiteID, Settings{Repo: testRepo, Branch: "main", Target: TargetApp}); errCode(err) != "invalid_git_target" {
		t.Fatalf("app folder for a PHP site: %v", err)
	}
	if in, _ := e.m.Info(ctx, testSiteID); in.AppFolder != "" {
		t.Fatalf("Info of a PHP site: %+v", in)
	}
}

func TestDeployRefusesFolderWithFiles(t *testing.T) {
	cases := []struct {
		name, target, file string
	}{
		{"public with a page", TargetPublic, "public/index.php"},
		{"public with a changed index.html", TargetPublic, "public/index.html"},
		{"root with an app folder", TargetRoot, "app/server.js"},
		{"root with a file", TargetRoot, ".env"},
		{"root with files in public", TargetRoot, "public/wp-config.php"},
	}
	for _, c := range cases {
		e := newEnv(t, &store.GitDeploy{Repo: testRepo, Branch: "main", Target: c.target, LastCommit: testHash + " Old"})
		writeFile(t, filepath.Join(e.root, filepath.FromSlash(c.file)), "mine")
		_, err := e.m.Deploy(context.Background(), testSiteID, e.log)
		if err == nil || !strings.Contains(err.Error(), "already has files") {
			t.Errorf("%s: err = %v", c.name, err)
		}
		if len(e.run.calls) != 0 {
			t.Errorf("%s: commands ran: %v", c.name, e.run.commands())
		}
		if g := e.st.git[testSiteID]; g.LastStatus != StatusFailed || g.LastCommit != testHash+" Old" {
			t.Errorf("%s: result = %q %q", c.name, g.LastStatus, g.LastCommit)
		}
	}
}

func TestDeployFailures(t *testing.T) {
	t.Run("ssh repository without a key", func(t *testing.T) {
		e := newEnv(t, &store.GitDeploy{Repo: "git@github.com:owner/private.git", Branch: "main", Target: TargetPublic})
		_, err := e.m.Deploy(context.Background(), testSiteID, e.log)
		if err == nil || !strings.Contains(err.Error(), "deploy key is missing") || len(e.run.calls) != 0 {
			t.Fatalf("err = %v, commands %v", err, e.run.commands())
		}
	})

	t.Run("key refused", func(t *testing.T) {
		e := newEnv(t, &store.GitDeploy{Repo: "git@github.com:owner/private.git", Branch: "main", Target: TargetPublic})
		writeFile(t, filepath.Join(e.root, ".ssh", "panel_deploy.pub"), "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJj+f3N9robQ21ttGmHatHZ4DMUsubuEgnHEEB1FUv0/ deploy@example.com\n")
		respond := e.run.respond
		e.run.respond = func(c userCmd) (string, error) {
			if len(c.cmd) > 1 && c.cmd[1] == "fetch" {
				return "git@github.com: Permission denied (publickey).\nfatal: Could not read from remote repository.\n", errors.New("exit status 128")
			}
			return respond(c)
		}
		_, err := e.m.Deploy(context.Background(), testSiteID, e.log)
		if err == nil || !strings.Contains(err.Error(), "Could not fetch branch main") || !strings.Contains(err.Error(), "Deploy keys") {
			t.Fatalf("err = %v", err)
		}
		if slices.ContainsFunc(e.run.commands(), func(c string) bool { return strings.HasPrefix(c, "git reset") }) {
			t.Fatal("reset after a failed fetch")
		}
		if e.st.git[testSiteID].LastStatus != StatusFailed {
			t.Fatal("status not failed")
		}
	})

	t.Run("missing branch", func(t *testing.T) {
		e := newEnv(t, &store.GitDeploy{Repo: testRepo, Branch: "main", Target: TargetPublic})
		e.run.respond = func(c userCmd) (string, error) {
			if len(c.cmd) > 1 && c.cmd[1] == "fetch" {
				return "fatal: couldn't find remote ref refs/heads/main\n", errors.New("exit status 128")
			}
			return "", nil
		}
		_, err := e.m.Deploy(context.Background(), testSiteID, e.log)
		if err == nil || !strings.Contains(err.Error(), "has no branch main") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("script fails", func(t *testing.T) {
		e := newEnv(t, &store.GitDeploy{Repo: testRepo, Branch: "main", Target: TargetPublic, Script: "npm ci && npm run build"})
		respond := e.run.respond
		e.run.respond = func(c userCmd) (string, error) {
			if len(c.cmd) > 1 && c.cmd[0] == "sh" && c.cmd[1] == "-e" {
				return "npm ERR! missing script: build\n", errors.New("exit status 1")
			}
			return respond(c)
		}
		commit, err := e.m.Deploy(context.Background(), testSiteID, e.log)
		if err == nil || err.Error() != "The deploy script failed (exit status 1)." || commit != testHash {
			t.Fatalf("err = %v, commit %q", err, commit)
		}
		g := e.st.git[testSiteID]
		if g.LastStatus != StatusFailed || !strings.HasPrefix(g.LastCommit, testHash) {
			t.Fatalf("result = %q %q", g.LastStatus, g.LastCommit)
		}
		if !strings.Contains(e.log.String(), "missing script: build") {
			t.Errorf("script output not in the log:\n%s", e.log)
		}
	})

	t.Run("git not installed", func(t *testing.T) {
		e := newEnv(t, &store.GitDeploy{Repo: testRepo, Branch: "main", Target: TargetPublic})
		e.m.hasGit = func() bool { return false }
		if _, err := e.m.Deploy(context.Background(), testSiteID, e.log); err != nil {
			t.Fatal(err)
		}
		first := e.run.calls[0]
		if first.runner != "/bin/sh" || !strings.Contains(strings.Join(first.cmd, " "), "apt-get") || first.cmd[len(first.cmd)-1] != "git" {
			t.Fatalf("first command: %s %v", first.runner, first.cmd)
		}
	})

	t.Run("not configured", func(t *testing.T) {
		e := newEnv(t, nil)
		if _, err := e.m.Deploy(context.Background(), testSiteID, e.log); err == nil {
			t.Fatal("deployed without settings")
		}
		if _, err := e.m.CheckDeploy(context.Background(), testSiteID); errCode(err) != "git_not_configured" {
			t.Fatalf("CheckDeploy: %v", err)
		}
	})

	t.Run("invalid stored settings", func(t *testing.T) {
		e := newEnv(t, &store.GitDeploy{Repo: "--upload-pack=touch /tmp/x", Branch: "main", Target: TargetPublic})
		if _, err := e.m.Deploy(context.Background(), testSiteID, e.log); err == nil || len(e.run.calls) != 0 {
			t.Fatalf("err = %v, commands %v", err, e.run.commands())
		}
	})
}

// ---- settings, key and secret ----

func TestSaveKeySecretRemove(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t, nil)

	if _, err := e.m.Save(ctx, testSiteID, Settings{Repo: "ext::sh -c id", Branch: "main", Target: TargetPublic}); errCode(err) != "invalid_git_repo" {
		t.Fatalf("bad repo: %v", err)
	}
	if _, err := e.m.Save(ctx, testSiteID, Settings{Repo: testRepo, Branch: "--force", Target: TargetPublic}); errCode(err) != "invalid_git_branch" {
		t.Fatalf("bad branch: %v", err)
	}
	if _, err := e.m.Save(ctx, testSiteID, Settings{Repo: testRepo, Branch: "main", Target: "/etc"}); errCode(err) != "invalid_git_target" {
		t.Fatalf("bad target: %v", err)
	}
	if len(e.st.git) != 0 || len(e.run.calls) != 0 {
		t.Fatal("a refused request changed something")
	}

	in, err := e.m.Info(ctx, testSiteID)
	if err != nil || in.Configured || in.Branch != "main" || in.Target != TargetPublic || in.Folder != filepath.Join(e.root, "public") {
		t.Fatalf("Info before saving: %+v, %v", in, err)
	}

	in, err = e.m.Save(ctx, testSiteID, Settings{Repo: " git@github.com:owner/site.git ", Branch: " main ", Target: TargetRoot, Script: "php artisan migrate --force\r\n"})
	if err != nil {
		t.Fatal(err)
	}
	if !in.Configured || in.Repo != "git@github.com:owner/site.git" || in.Branch != "main" || !in.SSH ||
		in.Folder != e.root || in.Script != "php artisan migrate --force" {
		t.Fatalf("Info = %+v", in)
	}
	if !strings.HasPrefix(in.PublicKey, "ssh-ed25519 AAAA") || !strings.HasSuffix(in.PublicKey, " deploy@example.com") ||
		!strings.HasPrefix(in.KeyFingerprint, "SHA256:") {
		t.Fatalf("key = %q %q", in.PublicKey, in.KeyFingerprint)
	}
	if in.WebhookURL != "http://example.com/.panel/deploy/3" || len(in.WebhookSecret) != 64 || strings.Trim(in.WebhookSecret, "0123456789abcdef") != "" {
		t.Fatalf("webhook = %q %q", in.WebhookURL, in.WebhookSecret)
	}
	keygen := e.run.calls[0]
	if keygen.user != testUser || !strings.Contains(keygen.cmd[0], `ssh-keygen -q -t ed25519 -N "" -C "$3" -f "$2"`) ||
		keygen.cmd[1] != filepath.Join(e.root, ".ssh") || keygen.cmd[2] != filepath.Join(e.root, ".ssh", "panel_deploy") ||
		keygen.cmd[3] != "deploy@example.com" {
		t.Fatalf("key generation: %+v", keygen)
	}

	// Saving again keeps the key and the secret.
	secret, key := in.WebhookSecret, in.PublicKey
	in, err = e.m.Save(ctx, testSiteID, Settings{Repo: testRepo, Branch: "dev", Target: TargetPublic})
	if err != nil || in.WebhookSecret != secret || in.PublicKey != key || len(e.run.calls) != 1 || in.SSH {
		t.Fatalf("second save: %+v, %v, %d commands", in, err, len(e.run.calls))
	}

	in, err = e.m.NewSecret(ctx, testSiteID)
	if err != nil || in.WebhookSecret == secret || len(in.WebhookSecret) != 64 || in.Branch != "dev" {
		t.Fatalf("NewSecret: %+v, %v", in, err)
	}
	in, err = e.m.NewKey(ctx, testSiteID)
	if err != nil || in.PublicKey == key || len(e.run.calls) != 2 {
		t.Fatalf("NewKey: %+v, %v", in, err)
	}

	// The last deploy shows up.
	e.st.SetGitDeployResult(ctx, testSiteID, testHash+" Fix the menu", StatusOK, time.Unix(1_800_000_000, 0))
	in, _ = e.m.Info(ctx, testSiteID)
	if in.LastCommit != testHash || in.LastSubject != "Fix the menu" || in.LastStatus != StatusOK || in.LastDeployAt == nil {
		t.Fatalf("last deploy: %+v", in)
	}

	// Remove forgets the settings and the key; other files in .ssh stay.
	writeFile(t, filepath.Join(e.root, ".ssh", "known_hosts"), "github.com ssh-ed25519 AAAA\n")
	writeFile(t, filepath.Join(e.root, "public", "README"), "deployed")
	if err := e.m.Remove(ctx, testSiteID); err != nil {
		t.Fatal(err)
	}
	if len(e.st.git) != 0 || exists(filepath.Join(e.root, ".ssh")) || !exists(filepath.Join(e.root, "public", "README")) {
		t.Fatalf("after Remove: settings %v, .ssh %v", e.st.git, exists(filepath.Join(e.root, ".ssh")))
	}
	if _, err := e.m.NewKey(ctx, testSiteID); errCode(err) != "git_not_configured" {
		t.Fatalf("NewKey without settings: %v", err)
	}
	if _, err := e.m.NewSecret(ctx, testSiteID); errCode(err) != "git_not_configured" {
		t.Fatalf("NewSecret without settings: %v", err)
	}

	writeFile(t, filepath.Join(e.root, ".ssh", "panel_deploy"), "key")
	writeFile(t, filepath.Join(e.root, ".ssh", "authorized_keys"), "not ours")
	if err := e.m.Remove(ctx, testSiteID); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(e.root, ".ssh", "panel_deploy")) || !exists(filepath.Join(e.root, ".ssh", "authorized_keys")) {
		t.Fatal("Remove deleted the wrong files")
	}

	if _, err := e.m.Info(ctx, 99); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown site: %v", err)
	}
}

func TestWebhookURL(t *testing.T) {
	if got := WebhookURL(&store.Site{ID: 12, Domain: "shop.example.com", SSL: true}); got != "https://shop.example.com/.panel/deploy/12" {
		t.Fatal(got)
	}
	if got := WebhookURL(&store.Site{ID: 1, Domain: "example.com"}); got != "http://example.com/.panel/deploy/1" {
		t.Fatal(got)
	}
}

func TestParseCommit(t *testing.T) {
	sha256Hash := strings.Repeat("ab", 32)
	for out, want := range map[string][2]string{
		gitLogOutput:                   {testHash, "Merge pull request #6 from Spaceghost/patch-1"},
		testHash + "\r\nWindows\r\n":   {testHash, "Windows"},
		testHash + "\n":                {testHash, ""},
		sha256Hash + "\nsha256 repo\n": {sha256Hash, "sha256 repo"},
		"fatal: not a git repository":  {"", ""},
		"":                             {"", ""},
	} {
		h, s := parseCommit(out)
		if h != want[0] || s != want[1] {
			t.Errorf("parseCommit(%q) = %q, %q", out, h, s)
		}
	}
	if h, s := SplitCommit(testHash + " Fix: the menu"); h != testHash || s != "Fix: the menu" {
		t.Errorf("SplitCommit = %q, %q", h, s)
	}
}
