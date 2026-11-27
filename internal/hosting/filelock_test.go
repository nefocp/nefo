package hosting

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// fakeInfo is an fs.FileInfo with a mode, for lockedState.
type fakeInfo struct{ mode fs.FileMode }

func (f fakeInfo) Name() string       { return "x" }
func (f fakeInfo) Size() int64        { return 0 }
func (f fakeInfo) Mode() fs.FileMode  { return f.mode }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeInfo) Sys() any           { return nil }

func TestLockedState(t *testing.T) {
	const site, grp, lck = 20001, 20001, 991
	for _, c := range []struct {
		name     string
		mode     fs.FileMode
		uid, gid int
		wantUID  int
		wantGID  int
		want     fs.FileMode
		change   bool
	}{
		{"file 0640", 0o640, site, grp, 0, grp, 0o640, true},
		{"file 0644", 0o644, site, grp, 0, grp, 0o644, true},
		{"file 0664 loses group write", 0o664, site, grp, 0, grp, 0o644, true},
		{"file 0666 loses all write", 0o666, site, grp, 0, grp, 0o644, true},
		{"wp-config 0600: lock group", 0o600, site, grp, 0, lck, 0o640, true},
		{"0400 stays owner-only", 0o400, site, grp, 0, lck, 0o440, true},
		{"setuid goes", 0o4755, site, grp, 0, grp, 0o755, true},
		{"dir 0750", fs.ModeDir | 0o750, site, grp, 0, grp, fs.ModeDir&0 | 0o750, true},
		{"dir 0700: lock group", fs.ModeDir | 0o700, site, grp, 0, lck, 0o750, true},
		{"setgid dir keeps it", fs.ModeDir | fs.ModeSetgid | 0o2770, site, grp, 0, grp, fs.ModeSetgid | 0o750, true},
		{"root file untouched", 0o644, 0, 0, 0, 0, 0o644, false},
		{"root file of the group loses group write", 0o664, 0, grp, 0, grp, 0o644, true},
		{"www-data file world-writable", 0o666, 33, 33, 33, 33, 0o664, true},
		{"other group's write stays", 0o660, 33, 33, 33, 33, 0o660, false},
	} {
		uid, gid, mode, change := lockedState(fakeInfo{c.mode}, c.uid, c.gid, site, grp, lck)
		if change != c.change || (change && (uid != c.wantUID || gid != c.wantGID || mode != c.want)) {
			t.Errorf("%s: got %d:%d %o %v, want %d:%d %o %v", c.name, uid, gid, mode, change, c.wantUID, c.wantGID, c.want, c.change)
		}
	}
}

func TestNormalizeWritable(t *testing.T) {
	got, err := normalizeWritable([]string{" public/wp-content/uploads/ ", "storage", "", "storage", "public\\sites\\*\\files"})
	if err != nil || len(got) != 3 || got[2] != "public/sites/*/files" {
		t.Fatalf("%q %v", got, err)
	}
	for _, bad := range []string{"../etc", "public/../../etc", "/etc", "a//b", "a/./b", "a b", "a;b", "$HOME", "public/[x]"} {
		if _, err := normalizeWritable([]string{bad}); err == nil && bad != "/etc" {
			t.Errorf("accepted %q", bad)
		}
	}
	// A leading slash is trimmed: /etc is the site's etc folder.
	if got, _ := normalizeWritable([]string{"/etc"}); len(got) != 1 || got[0] != "etc" {
		t.Errorf("/etc → %q", got)
	}
}

func TestLockPreset(t *testing.T) {
	for _, c := range []struct{ app, rewrite, want string }{
		{AppWordPress, "", "wordpress"}, {"", "wordpress", "wordpress"}, {"laravel", "laravel", "laravel"},
		{"", "drupal", "drupal"}, {"", "spa", ""}, {"docker:ghost", "", ""}, {"", "", ""},
	} {
		if got := lockPreset(&store.Site{App: c.app, Rewrite: c.rewrite}); got != c.want {
			t.Errorf("%s/%s: %q", c.app, c.rewrite, got)
		}
	}
	if !isPHPFile("x.PHP") || !isPHPFile("a.phtml") || isPHPFile("a.php.txt") || isPHPFile("php") {
		t.Error("isPHPFile")
	}
}

func TestWritableDirs(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"public/wp-content/uploads", "public/sites/a/files", "public/sites/b/files", "tmp", "logs"} {
		os.MkdirAll(filepath.Join(root, d), 0o755)
	}
	os.WriteFile(filepath.Join(root, "public/sites/c"), nil, 0o644) // a file, not a folder
	dirs, missing := writableDirs(root, []string{"public/wp-content/uploads", "public/sites/*/files", "public/wp-content/cache", "nope/x"}, -1, -1, false)
	for _, d := range []string{"tmp", "logs", "public/wp-content/uploads", "public/sites/a/files", "public/sites/b/files"} {
		if !dirs[d] {
			t.Errorf("%s missing from %v", d, dirs)
		}
	}
	if len(missing) != 2 {
		t.Errorf("missing %v", missing)
	}
	if !inWritable("public/wp-content/uploads/2026/x.jpg", dirs) || inWritable("public/wp-content/index.php", dirs) || inWritable("public/wp-content/uploadsx", dirs) {
		t.Error("inWritable")
	}
	if runtime.GOOS != "windows" {
		// A folder reached through a link is not the site's own.
		os.Symlink(filepath.Join(root, "public/wp-content"), filepath.Join(root, "link"))
		if realDirWithin(root, "link/uploads") {
			t.Error("followed a link")
		}
		dirs, _ = writableDirs(root, []string{"link/uploads"}, -1, -1, false)
		if dirs["link/uploads"] {
			t.Error("a writable folder through a link")
		}
	}
}

func TestCheckWritable(t *testing.T) {
	m := newTestManager(t)
	m.paths.WWW = t.TempDir()
	ctx := context.Background()
	site := &store.Site{Domain: "lock.example.com", SysUser: "lock_example_com"}
	if err := m.store.CreateSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	root := m.paths.siteRoot(site.Domain)
	os.MkdirAll(filepath.Join(root, "public/wp-content/uploads"), 0o755)
	os.MkdirAll(filepath.Join(root, "tmp"), 0o755)
	// Not locked: everything goes.
	if err := m.CheckWritable(ctx, site, "public/index.php", false); err != nil {
		t.Fatal(err)
	}
	m.store.SetSiteProtect(ctx, &store.SiteProtect{SiteID: site.ID, LockOn: true, LockWritable: []string{"public/wp-content/uploads"}})
	var ue *usererr.Error
	for _, c := range []struct {
		rel  string
		self bool
		ok   bool
	}{
		{"public/index.php", false, false},
		{"public/wp-content/uploads/a.jpg", false, true},
		{"public/wp-content/uploads/2026/11/a.jpg", false, true},
		{"public/wp-content/uploads", false, false}, // renaming or deleting the folder itself
		{"public/wp-content/uploads", true, true},   // its mode
		{"tmp/x", false, true},
		{"public/wp-content/uploads/../../index.php", false, false},
		{".", false, false},
	} {
		err := m.CheckWritable(ctx, site, c.rel, c.self)
		if c.ok != (err == nil) || (err != nil && (!errors.As(err, &ue) || ue.Code != "path_locked")) {
			t.Errorf("%s self=%v: %v", c.rel, c.self, err)
		}
	}
	if runtime.GOOS != "windows" {
		// A link in a writable folder that points at the code is the code.
		os.Symlink("../../index.php", filepath.Join(root, "public/wp-content/uploads/evil"))
		if err := m.CheckWritable(ctx, site, "public/wp-content/uploads/evil", false); err == nil {
			t.Error("a link into the locked code was writable (dangling: index.php does not exist)")
		}
		os.WriteFile(filepath.Join(root, "public/index.php"), nil, 0o644)
		if err := m.CheckWritable(ctx, site, "public/wp-content/uploads/evil", false); err == nil {
			t.Error("a link into the locked code was writable")
		}
		os.Symlink(filepath.Join(root, "public"), filepath.Join(root, "public/wp-content/uploads/abs"))
		if err := m.CheckWritable(ctx, site, "public/wp-content/uploads/abs/x.php", false); err == nil {
			t.Error("an absolute link into the code was writable")
		}
		os.Symlink("2026", filepath.Join(root, "public/wp-content/uploads/latest"))
		if err := m.CheckWritable(ctx, site, "public/wp-content/uploads/latest/a.jpg", false); err != nil {
			t.Errorf("a link inside uploads: %v", err)
		}
	}
	for rel, want := range map[string]string{"a/b": "a/b", "a/../b": "b", "../x": "", "./a//b/": "a/b"} {
		if got := realRel(root, rel); got != want {
			t.Errorf("realRel(%q) = %q, want %q", rel, got, want)
		}
	}
	if err := m.RefuseLocked(ctx, site); !errors.As(err, &ue) || ue.Code != "site_files_locked" {
		t.Errorf("RefuseLocked: %v", err)
	}
}

func TestReadManifest(t *testing.T) {
	file := filepath.Join(t.TempDir(), "1.lock")
	os.WriteFile(file, []byte("nefo-filelock 1\nroot \"/var/www/a.com\"\nids 20001 20001 991\nwritable \"public/wp-content/uploads\"\n"+
		"x 384 20001 20001 \"public/wp-config.php\"\nx 2147484141 20001 33 \"public/odd dir \\\"q\\\"\"\ndone 42\n"), 0o600)
	mf, err := readManifest(file)
	if err != nil {
		t.Fatal(err)
	}
	if mf.root != "/var/www/a.com" || mf.uid != 20001 || mf.lck != 991 || !mf.done || !mf.writable["public/wp-content/uploads"] {
		t.Fatalf("%+v", mf)
	}
	if e := mf.entries["public/wp-config.php"]; e.Mode != 0o600 || e.GID != 20001 {
		t.Fatalf("%+v", e)
	}
	if e := mf.entries[`public/odd dir "q"`]; !e.Mode.IsDir() || e.GID != 33 {
		t.Fatalf("%+v", mf.entries)
	}
	// A lock that stopped halfway: no "done", still readable.
	os.WriteFile(file, []byte("nefo-filelock 1\nroot \"/var/www/a.com\"\nids 20001 20001 991\nx 384 20001 20001 \"a\"\n"), 0o600)
	if mf, err := readManifest(file); err != nil || mf.done || len(mf.entries) != 1 {
		t.Fatalf("%+v %v", mf, err)
	}
	for _, bad := range []string{"", "something else\n", "nefo-filelock 1\nx 1 2\n", "nefo-filelock 1\nroot unquoted\n"} {
		os.WriteFile(file, []byte(bad), 0o600)
		if _, err := readManifest(file); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestWithUnlockedNotLocked(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()
	site := &store.Site{Domain: "open.example.com", SysUser: "open_example_com"}
	if err := m.store.CreateSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	ran := 0
	if err := m.WithUnlocked(ctx, site.ID, nil, func() error { ran++; return nil }); err != nil || ran != 1 {
		t.Fatalf("ran %d, %v", ran, err)
	}
	want := errors.New("the operation failed")
	if err := m.WithUnlocked(ctx, site.ID, nil, func() error { return want }); err != want {
		t.Fatalf("error not passed on: %v", err)
	}
	// A folder above several sites, and one in no site, run the function once.
	if err := m.UnlockedFor(ctx, m.paths.WWW, nil, func() error { ran++; return nil }); err != nil || ran != 2 {
		t.Fatalf("UnlockedFor: ran %d, %v", ran, err)
	}
	if err := m.UnlockedFor(ctx, "/srv/data", nil, func() error { ran++; return nil }); err != nil || ran != 3 {
		t.Fatalf("UnlockedFor outside: ran %d, %v", ran, err)
	}
	if m.FilesLocked(ctx, site.ID) || m.lockIsBusy(site.ID) {
		t.Fatal("locked")
	}
}

// UnlockAllFiles (nefo unlock-files) puts back what a lock record says, on
// files of the user running the test (chown to oneself needs no root).
func TestUnlockAllFiles(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("owners: Linux only")
	}
	me, err := user.Current()
	if err != nil {
		t.Skip(err)
	}
	m := newTestManager(t)
	m.paths.WWW = t.TempDir()
	ctx := context.Background()
	site := &store.Site{Domain: "u.example.com", SysUser: me.Username}
	if err := m.store.CreateSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	other := &store.Site{Domain: "open.example.com", SysUser: me.Username + "x"}
	m.store.CreateSite(ctx, other)
	root := m.paths.siteRoot(site.Domain)
	os.MkdirAll(filepath.Join(root, "public"), 0o750)
	cfg := filepath.Join(root, "public", "wp-config.php")
	os.WriteFile(cfg, []byte("<?php"), 0o644)
	os.Chmod(cfg, 0o640) // as the lock left it: group-readable
	uid, gid := os.Getuid(), os.Getgid()
	os.MkdirAll(m.paths.FileLock, 0o700)
	rec := fmt.Sprintf("nefo-filelock 1\nroot %q\nids %d %d 991\nx %d %d %d %q\ndone 3\n", root, uid, gid, uint32(0o600), uid, gid, "public/wp-config.php")
	os.WriteFile(m.lockManifest(site.ID), []byte(rec), 0o600)
	m.store.SetSiteProtect(ctx, &store.SiteProtect{SiteID: site.ID, LockOn: true, LockAlert: true, LockWritable: []string{"public/uploads"}})

	var reports []string
	n, err := m.UnlockAllFiles(ctx, func(d string, err error) { reports = append(reports, fmt.Sprint(d, " ", err)) })
	if err != nil || n != 1 || len(reports) != 1 || reports[0] != "u.example.com <nil>" {
		t.Fatalf("%d %v %q", n, err, reports)
	}
	if st, _ := os.Stat(cfg); st.Mode().Perm() != 0o600 {
		t.Fatalf("wp-config.php %o, want 600", st.Mode().Perm())
	}
	if fileExists(m.lockManifest(site.ID)) || m.FilesLocked(ctx, site.ID) {
		t.Fatal("still locked")
	}
	if p, _ := m.store.SiteProtect(ctx, site.ID); p.LockAlert || len(p.LockWritable) != 1 {
		t.Fatalf("settings %+v", p)
	}
	if n, err := m.UnlockAllFiles(ctx, func(string, error) { t.Fatal("nothing to report") }); n != 0 || err != nil {
		t.Fatalf("second run: %d %v", n, err)
	}
}
