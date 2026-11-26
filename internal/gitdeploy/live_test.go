package gitdeploy

import (
	"context"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/store"
)

// TestLive runs real deploys with git, ssh and runuser. It needs root and
// network access, and is skipped unless GITDEPLOY_LIVE_DIR (an empty
// scratch folder) and GITDEPLOY_LIVE_USER (an existing account, such as
// nobody) are set:
//
//	GITDEPLOY_LIVE_DIR=/tmp/agent-git/live GITDEPLOY_LIVE_USER=nobody ./gitdeploy.test -test.run Live -test.v
func TestLive(t *testing.T) {
	base, name := os.Getenv("GITDEPLOY_LIVE_DIR"), os.Getenv("GITDEPLOY_LIVE_USER")
	if base == "" || name == "" || os.Geteuid() != 0 {
		t.Skip("set GITDEPLOY_LIVE_DIR and GITDEPLOY_LIVE_USER and run as root")
	}
	u, err := user.Lookup(name)
	if err != nil {
		t.Fatal(err)
	}
	uid, _ := strconv.Atoi(u.Uid)
	gid, _ := strconv.Atoi(u.Gid)
	ctx := context.Background()

	newSite := func(id int64, domain string) string {
		root := filepath.Join(base, domain)
		os.RemoveAll(root)
		for _, d := range []string{"", "public", "logs", "tmp"} {
			p := filepath.Join(root, d)
			if err := os.MkdirAll(p, 0o750); err != nil {
				t.Fatal(err)
			}
			os.Chmod(p, 0o750)
			os.Chown(p, uid, gid)
		}
		index := filepath.Join(root, "public", "index.html")
		os.WriteFile(index, []byte(placeholder), 0o640)
		os.Chown(index, uid, gid)
		return root
	}
	owner := func(p string) string {
		st, err := os.Lstat(p)
		if err != nil {
			return err.Error()
		}
		o, g := fsutil.Owner(st)
		return strconv.Itoa(o) + ":" + strconv.Itoa(g) + " " + strconv.FormatUint(uint64(st.Mode().Perm()), 8)
	}
	read := func(p string) string {
		b, _ := os.ReadFile(p)
		return strings.TrimSpace(string(b))
	}
	want := strconv.Itoa(uid) + ":" + strconv.Itoa(gid)

	st := &memStore{
		sites: map[int64]*store.Site{
			1: {ID: 1, Domain: "public.test", SysUser: name},
			2: {ID: 2, Domain: "root.test", SysUser: name},
		},
		git: map[int64]*store.GitDeploy{},
	}
	m := New(run.System{}, st, func(d string) string { return filepath.Join(base, d) })
	lg := &testLog{}

	// Into the web folder.
	root := newSite(1, "public.test")
	in, err := m.Save(ctx, 1, Settings{Repo: testRepo, Branch: "master", Target: TargetPublic,
		Script: "id -un > whoami.txt\necho \"$DEPLOY_COMMIT\" > commit.txt\numask > umask.txt"})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("key %s (%s)", in.PublicKey, in.KeyFingerprint)
	if !strings.HasPrefix(in.PublicKey, "ssh-ed25519 ") || !strings.HasSuffix(in.PublicKey, " deploy@public.test") {
		t.Fatalf("public key %q", in.PublicKey)
	}
	if got := owner(filepath.Join(root, ".ssh", keyName)); got != want+" 600" {
		t.Errorf("private key: %s", got)
	}
	if got := owner(filepath.Join(root, ".ssh")); got != want+" 700" {
		t.Errorf(".ssh: %s", got)
	}
	commit, err := m.Deploy(ctx, 1, lg)
	t.Logf("first deploy:\n%s", lg)
	if err != nil {
		t.Fatal(err)
	}
	public := filepath.Join(root, "public")
	if read(filepath.Join(public, "README")) != "Hello World!" {
		t.Fatal("README not deployed")
	}
	for _, f := range []string{"README", ".git", "whoami.txt"} {
		if got := owner(filepath.Join(public, f)); !strings.HasPrefix(got, want+" ") {
			t.Errorf("%s: %s", f, got)
		}
	}
	if got := owner(filepath.Join(public, "README")); got != want+" 640" {
		t.Errorf("README mode: %s", got)
	}
	if read(filepath.Join(public, "whoami.txt")) != name || read(filepath.Join(public, "commit.txt")) != commit ||
		read(filepath.Join(public, "umask.txt")) != "0027" {
		t.Errorf("script: whoami %q, commit %q, umask %q", read(filepath.Join(public, "whoami.txt")),
			read(filepath.Join(public, "commit.txt")), read(filepath.Join(public, "umask.txt")))
	}
	if exists(filepath.Join(public, "index.html")) {
		t.Error("placeholder still there")
	}
	if g := st.git[1]; g.LastStatus != StatusOK || !strings.HasPrefix(g.LastCommit, commit+" ") {
		t.Errorf("result %q %q", g.LastStatus, g.LastCommit)
	}

	// Again: an update. A changed tracked file is reset, an uploaded one stays.
	os.WriteFile(filepath.Join(public, "README"), []byte("changed"), 0o640)
	os.WriteFile(filepath.Join(public, "upload.jpg"), []byte("jpeg"), 0o640)
	lg = &testLog{}
	if _, err := m.Deploy(ctx, 1, lg); err != nil {
		t.Fatalf("second deploy: %v\n%s", err, lg)
	}
	t.Logf("second deploy:\n%s", lg)
	if read(filepath.Join(public, "README")) != "Hello World!" || read(filepath.Join(public, "upload.jpg")) != "jpeg" {
		t.Error("update: README not reset or upload removed")
	}

	// A branch that does not exist.
	m.Save(ctx, 1, Settings{Repo: testRepo, Branch: "no-such-branch", Target: TargetPublic})
	lg = &testLog{}
	_, err = m.Deploy(ctx, 1, lg)
	t.Logf("missing branch: %v", err)
	if err == nil || !strings.Contains(err.Error(), "has no branch no-such-branch") {
		t.Errorf("missing branch: %v\n%s", err, lg)
	}

	// SSH with a key GitHub does not know: refused, with the hint.
	m.Save(ctx, 1, Settings{Repo: "git@github.com:octocat/Hello-World.git", Branch: "master", Target: TargetPublic})
	lg = &testLog{}
	_, err = m.Deploy(ctx, 1, lg)
	t.Logf("ssh with an unknown key: %v\n%s", err, lg)
	if err == nil || !strings.Contains(err.Error(), "Deploy keys") {
		t.Errorf("ssh: %v", err)
	}
	if !strings.Contains(read(filepath.Join(root, ".ssh", knownHosts)), "github.com ") {
		t.Error("known_hosts not written")
	}
	if st.git[1].LastStatus != StatusFailed || !strings.HasPrefix(st.git[1].LastCommit, commit) {
		t.Errorf("failed deploy result: %+v", st.git[1])
	}

	if err := m.Remove(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(root, ".ssh")) || !exists(filepath.Join(public, "README")) {
		t.Error("Remove: .ssh left or files removed")
	}

	// Into the site folder: the repository has no public/ folder, so the
	// placeholder stays.
	root = newSite(2, "root.test")
	os.WriteFile(filepath.Join(root, "logs", "access.log"), []byte("log"), 0o640)
	if _, err := m.Save(ctx, 2, Settings{Repo: testRepo, Branch: "master", Target: TargetRoot}); err != nil {
		t.Fatal(err)
	}
	lg = &testLog{}
	if _, err := m.Deploy(ctx, 2, lg); err != nil {
		t.Fatalf("root deploy: %v\n%s", err, lg)
	}
	t.Logf("root deploy:\n%s", lg)
	if read(filepath.Join(root, "README")) != "Hello World!" || !exists(filepath.Join(root, "public", "index.html")) ||
		read(filepath.Join(root, "logs", "access.log")) != "log" {
		t.Error("root deploy: files not as expected")
	}
	if got := owner(root); got != want+" 750" {
		t.Errorf("site folder: %s", got)
	}

	// A site folder with other files is refused.
	root = newSite(2, "root.test")
	os.WriteFile(filepath.Join(root, "app.js"), []byte("x"), 0o640)
	if _, err := m.Deploy(ctx, 2, &testLog{}); err == nil || !strings.Contains(err.Error(), "already has files") {
		t.Errorf("not empty: %v", err)
	}
}
