package files

import (
	"context"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/fsutil"
)

// trashEnv is a website folder and its trash folder, opened as os.Roots
// the way the Manager does, so the trash works on any system.
type trashEnv struct {
	dir, site, binDir string
	root, bin         *os.Root
	scope             Scope
}

func newTrashEnv(t *testing.T) *trashEnv {
	t.Helper()
	e := &trashEnv{dir: t.TempDir()}
	e.site = filepath.Join(e.dir, "site")
	e.binDir = filepath.Join(e.dir, "trash", "example.com")
	os.Mkdir(e.site, 0o750)
	os.MkdirAll(e.binDir, 0o700)
	var err error
	if e.root, err = os.OpenRoot(e.site); err != nil {
		t.Fatal(err)
	}
	if e.bin, err = os.OpenRoot(e.binDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { e.root.Close(); e.bin.Close() })
	uid, gid := -1, -1
	if runtime.GOOS != "windows" {
		uid, gid = os.Getuid(), os.Getgid()
	}
	e.scope = Scope{Root: filepath.ToSlash(e.site), UID: uid, GID: gid, Site: "example.com"}
	return e
}

func (e *trashEnv) write(t *testing.T, rel, content string, mode os.FileMode) {
	t.Helper()
	p := filepath.Join(e.site, filepath.FromSlash(rel))
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	os.Chmod(p, mode)
}

func (e *trashEnv) read(rel string) string {
	b, err := os.ReadFile(filepath.Join(e.site, filepath.FromSlash(rel)))
	if err != nil {
		return "<" + err.Error() + ">"
	}
	return string(b)
}

func (e *trashEnv) exists(rel string) bool {
	_, err := os.Lstat(filepath.Join(e.site, filepath.FromSlash(rel)))
	return err == nil
}

func (e *trashEnv) list(t *testing.T) *TrashListing {
	t.Helper()
	l := &TrashListing{Entries: []TrashEntry{}}
	if err := listBin(e.bin, "example.com", e.root, e.scope.Root, l); err != nil {
		t.Fatal(err)
	}
	return l
}

func ownerOf(t *testing.T, p string) (int, int) {
	t.Helper()
	fi, err := os.Lstat(p)
	if err != nil {
		t.Fatal(err)
	}
	return fsutil.Owner(fi)
}

func TestTrashMoveAndRestore(t *testing.T) {
	e := newTrashEnv(t)
	e.write(t, "public/index.php", "<?php echo 1;", 0o640)
	e.write(t, "public/assets/app.js", "console.log(1)", 0o644)

	id1, err := moveToTrash(e.root, "public/index.php", e.scope, "example.com", e.bin, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	id2, err := moveToTrash(e.root, "public/assets", e.scope, "example.com", e.bin, time.Now().Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if e.exists("public/index.php") || e.exists("public/assets") {
		t.Fatal("items still in place")
	}
	l := e.list(t)
	if l.Count != 2 || l.TotalSize != 13+14 || l.Entries[0].ID != id2 || l.Entries[1].ID != id1 {
		t.Fatalf("listing = %+v", l)
	}
	if got := l.Entries[1]; got.Path != e.scope.Root+"/public/index.php" || got.Type != "file" || got.Occupied {
		t.Fatalf("entry = %+v", got)
	}
	if runtime.GOOS != "windows" {
		if l.Entries[1].Mode != "0640" {
			t.Errorf("mode = %s", l.Entries[1].Mode)
		}
		for _, p := range []string{e.binDir, filepath.Join(e.binDir, id1)} {
			if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o700 {
				t.Errorf("%s has mode %v", p, fi.Mode().Perm())
			}
		}
	}

	// Something in the way stops the restore.
	e.write(t, "public/index.php", "new", 0o644)
	if _, err := restoreEntry(e.root, e.scope, "example.com", e.bin, id1, e.dir+"/trash"); code(err) != "restore_exists" {
		t.Fatalf("restore over a file: %v", err)
	}
	if !e.list(t).Entries[1].Occupied {
		t.Error("occupied not reported")
	}
	os.Remove(filepath.Join(e.site, "public", "index.php"))

	if p, err := restoreEntry(e.root, e.scope, "example.com", e.bin, id1, e.dir+"/trash"); err != nil || p != e.scope.Root+"/public/index.php" {
		t.Fatalf("restore = %q, %v", p, err)
	}
	if _, err := restoreEntry(e.root, e.scope, "example.com", e.bin, id2, e.dir+"/trash"); err != nil {
		t.Fatal(err)
	}
	if e.read("public/index.php") != "<?php echo 1;" || e.read("public/assets/app.js") != "console.log(1)" {
		t.Fatal("restored content differs")
	}
	if runtime.GOOS != "windows" {
		if fi, _ := os.Stat(filepath.Join(e.site, "public", "index.php")); fi.Mode().Perm() != 0o640 {
			t.Errorf("restored mode = %v", fi.Mode().Perm())
		}
	}
	if l := e.list(t); l.Count != 0 {
		t.Fatalf("trash not empty after restore: %+v", l)
	}
	if des, _ := os.ReadDir(e.binDir); len(des) != 0 {
		t.Fatalf("%d entries left in the trash folder", len(des))
	}
	if _, err := restoreEntry(e.root, e.scope, "example.com", e.bin, id1, e.dir+"/trash"); code(err) != "trash_not_found" {
		t.Fatalf("restore twice: %v", err)
	}
}

func TestTrashRestoreCreatesParents(t *testing.T) {
	e := newTrashEnv(t)
	os.Chmod(e.site, 0o751)
	e.write(t, "a/b/c.txt", "deep", 0o644)
	id, err := moveToTrash(e.root, "a/b/c.txt", e.scope, "example.com", e.bin, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	os.RemoveAll(filepath.Join(e.site, "a"))
	if _, err := restoreEntry(e.root, e.scope, "example.com", e.bin, id, e.dir+"/trash"); err != nil {
		t.Fatal(err)
	}
	if e.read("a/b/c.txt") != "deep" {
		t.Fatal("not restored")
	}
	if runtime.GOOS != "windows" {
		for _, p := range []string{"a", "a/b"} {
			fi, _ := os.Stat(filepath.Join(e.site, p))
			if fi.Mode().Perm() != 0o751 {
				t.Errorf("%s has mode %v, want the site folder's 0751", p, fi.Mode().Perm())
			}
		}
	}

	// A file where a parent folder should be stops the restore.
	e.write(t, "x/y.txt", "y", 0o644)
	id, _ = moveToTrash(e.root, "x/y.txt", e.scope, "example.com", e.bin, time.Now())
	os.RemoveAll(filepath.Join(e.site, "x"))
	e.write(t, "x", "now a file", 0o644)
	if _, err := restoreEntry(e.root, e.scope, "example.com", e.bin, id, e.dir+"/trash"); code(err) != "restore_parent_not_folder" {
		t.Fatalf("restore under a file: %v", err)
	}
}

// crossDevice makes every rename fail as if the trash were on another disk.
func crossDevice(t *testing.T) {
	old := moveEntry
	moveEntry = func(*os.File, string, *os.File, string) error {
		return &os.LinkError{Op: "rename", Err: syscall.EXDEV}
	}
	t.Cleanup(func() { moveEntry = old })
}

func TestTrashAcrossFileSystems(t *testing.T) {
	crossDevice(t)
	e := newTrashEnv(t)
	e.write(t, "public/run.sh", "#!/bin/sh", 0o750)
	e.write(t, "public/sub/data.txt", "data", 0o600)
	links := runtime.GOOS != "windows"
	if links {
		os.Symlink("/etc/passwd", filepath.Join(e.site, "public", "passwd"))
		os.Symlink("sub/data.txt", filepath.Join(e.site, "public", "data-link"))
	}
	mtime := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	os.Chtimes(filepath.Join(e.site, "public", "run.sh"), mtime, mtime)

	id, err := moveToTrash(e.root, "public", e.scope, "example.com", e.bin, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if e.exists("public") {
		t.Fatal("original not removed after the copy")
	}
	if l := e.list(t); l.Count != 1 || l.TotalSize != 13 || l.Entries[0].Type != "dir" {
		t.Fatalf("listing = %+v", l)
	}
	if _, err := restoreEntry(e.root, e.scope, "example.com", e.bin, id, e.dir+"/trash"); err != nil {
		t.Fatal(err)
	}
	if e.read("public/run.sh") != "#!/bin/sh" || e.read("public/sub/data.txt") != "data" {
		t.Fatal("content differs")
	}
	fi, _ := os.Stat(filepath.Join(e.site, "public", "run.sh"))
	if !fi.ModTime().Equal(mtime) {
		t.Errorf("modification time = %v", fi.ModTime())
	}
	if links {
		if fi.Mode().Perm() != 0o750 {
			t.Errorf("run.sh mode = %v", fi.Mode().Perm())
		}
		if fi, _ := os.Stat(filepath.Join(e.site, "public", "sub", "data.txt")); fi.Mode().Perm() != 0o600 {
			t.Errorf("data.txt mode = %v", fi.Mode().Perm())
		}
		for name, want := range map[string]string{"passwd": "/etc/passwd", "data-link": "sub/data.txt"} {
			if got, err := os.Readlink(filepath.Join(e.site, "public", name)); err != nil || got != want {
				t.Errorf("link %s = %q, %v", name, got, err)
			}
		}
	}
	if des, _ := os.ReadDir(e.binDir); len(des) != 0 {
		t.Fatal("trash entry left after restore")
	}
}

func TestTrashCopyKeepsOwners(t *testing.T) {
	if runtime.GOOS != "linux" || os.Getuid() != 0 {
		t.Skip("needs root on Linux")
	}
	crossDevice(t)
	e := newTrashEnv(t)
	e.write(t, "public/a.txt", "a", 0o644)
	os.Chown(filepath.Join(e.site, "public"), 4242, 4343)
	os.Chown(filepath.Join(e.site, "public", "a.txt"), 4242, 4343)
	os.Symlink("a.txt", filepath.Join(e.site, "public", "l"))
	os.Lchown(filepath.Join(e.site, "public", "l"), 4242, 4343)
	id, err := moveToTrash(e.root, "public", e.scope, "example.com", e.bin, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if u, g := ownerOf(t, filepath.Join(e.binDir, id, "payload", "a.txt")); u != 4242 || g != 4343 {
		t.Errorf("owner in the trash = %d:%d", u, g)
	}
	if _, err := restoreEntry(e.root, e.scope, "example.com", e.bin, id, e.dir+"/trash"); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"public", "public/a.txt", "public/l"} {
		if u, g := ownerOf(t, filepath.Join(e.site, p)); u != 4242 || g != 4343 {
			t.Errorf("%s owner = %d:%d", p, u, g)
		}
	}
}

func TestTrashReownsWhenSiteRecreated(t *testing.T) {
	if runtime.GOOS != "linux" || os.Getuid() != 0 {
		t.Skip("needs root on Linux")
	}
	e := newTrashEnv(t)
	e.write(t, "public/a.txt", "a", 0o644)
	e.write(t, "public/root.txt", "r", 0o644)
	os.Chown(filepath.Join(e.site, "public"), 5000, 5000)
	os.Chown(filepath.Join(e.site, "public", "a.txt"), 5000, 5000)
	old := e.scope
	old.UID, old.GID = 5000, 5000
	id, err := moveToTrash(e.root, "public", old, "example.com", e.bin, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// The website was deleted and created again with a new user.
	e.scope.UID, e.scope.GID = 6000, 6000
	if _, err := restoreEntry(e.root, e.scope, "example.com", e.bin, id, e.dir+"/trash"); err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]int{"public": 6000, "public/a.txt": 6000, "public/root.txt": 0} {
		if u, _ := ownerOf(t, filepath.Join(e.site, p)); u != want {
			t.Errorf("%s owner = %d, want %d", p, u, want)
		}
	}
}

func TestTrashPurge(t *testing.T) {
	dir := t.TempDir()
	m := New(nil)
	m.trashDir = filepath.Join(dir, "trash")
	site := filepath.Join(dir, "site")
	os.Mkdir(site, 0o755)
	root, _ := os.OpenRoot(site)
	defer root.Close()
	scope := Scope{Root: filepath.ToSlash(site), UID: -1, GID: -1, Site: "example.com"}
	bin, err := m.openBin("example.com", true)
	if err != nil {
		t.Fatal(err)
	}
	defer bin.Close()
	now := time.Now()
	for i, age := range []time.Duration{40 * 24 * time.Hour, 31 * 24 * time.Hour, 29 * 24 * time.Hour, time.Hour} {
		name := "f" + string(rune('0'+i))
		os.WriteFile(filepath.Join(site, name), []byte(name), 0o644)
		if _, err := moveToTrash(root, name, scope, "example.com", bin, now.Add(-age)); err != nil {
			t.Fatal(err)
		}
	}
	// A damaged entry is purged by the age of its folder.
	damaged := filepath.Join(m.trashDir, "example.com", "damaged")
	os.Mkdir(damaged, 0o700)
	old := now.Add(-60 * 24 * time.Hour)
	os.Chtimes(damaged, old, old)
	// So is an old entry of the server scope; a fresh one stays.
	srv, _ := m.openBin(serverScopeID, true)
	srv.Mkdir("20200101-000000-aaaaaaaa", 0o700)
	srv.Chtimes("20200101-000000-aaaaaaaa", old, old)
	srv.Mkdir("fresh", 0o700)
	srv.Close()

	n, err := m.PurgeTrash(context.Background(), TrashMaxAge)
	if err != nil || n != 4 {
		t.Fatalf("purged %d, %v", n, err)
	}
	l := &TrashListing{}
	listBin(bin, "example.com", root, scope.Root, l)
	if l.Count != 2 {
		t.Fatalf("%d entries left, want 2", l.Count)
	}
	for _, e := range l.Entries {
		if !strings.HasSuffix(e.Path, "/f2") && !strings.HasSuffix(e.Path, "/f3") {
			t.Errorf("kept %s", e.Path)
		}
	}
	if _, err := os.Stat(filepath.Join(m.trashDir, serverScopeID, "fresh")); err != nil {
		t.Error("fresh server entry purged")
	}
	if n, err := (&Manager{trashDir: filepath.Join(dir, "none")}).PurgeTrash(context.Background(), TrashMaxAge); n != 0 || err != nil {
		t.Errorf("purge without a trash folder: %d, %v", n, err)
	}
}

// writeRawMeta replaces an entry's meta.json, as an attacker with access
// to the trash folder could.
func writeRawMeta(t *testing.T, e *trashEnv, id string, edit func(map[string]any)) {
	t.Helper()
	p := filepath.Join(e.binDir, id, trashMetaFile)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	json.Unmarshal(b, &m)
	edit(m)
	b, _ = json.Marshal(m)
	os.WriteFile(p, b, 0o600)
}

func TestTrashCraftedMeta(t *testing.T) {
	e := newTrashEnv(t)
	outside := filepath.Join(e.dir, "outside.txt")
	for _, c := range []struct {
		name string
		edit func(map[string]any)
	}{
		{"parent path", func(m map[string]any) { m["path"] = "../outside.txt" }},
		{"nested parent path", func(m map[string]any) { m["path"] = "public/../../outside.txt" }},
		{"absolute path", func(m map[string]any) { m["path"] = "/outside.txt" }},
		{"scope folder", func(m map[string]any) { m["path"] = "." }},
		{"empty path", func(m map[string]any) { m["path"] = "" }},
		{"other scope", func(m map[string]any) { m["scope"] = "other.com" }},
		{"not JSON", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			e.write(t, "evil.txt", "evil", 0o644)
			id, err := moveToTrash(e.root, "evil.txt", e.scope, "example.com", e.bin, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if c.edit == nil {
				os.WriteFile(filepath.Join(e.binDir, id, trashMetaFile), []byte("{not json"), 0o600)
			} else {
				writeRawMeta(t, e, id, c.edit)
			}
			if _, err := restoreEntry(e.root, e.scope, "example.com", e.bin, id, e.dir+"/trash"); code(err) != "trash_damaged" {
				t.Fatalf("restore: %v", err)
			}
			if _, err := os.Lstat(outside); err == nil {
				t.Fatal("file written outside the site")
			}
			for _, en := range e.list(t).Entries {
				if en.ID == id {
					t.Fatal("damaged entry listed")
				}
			}
			e.bin.RemoveAll(id)
		})
	}
	for _, id := range []string{"../trash", "..", "a/b", "", "UPPER"} {
		if _, err := restoreEntry(e.root, e.scope, "example.com", e.bin, id, e.dir+"/trash"); code(err) != "trash_not_found" {
			t.Errorf("entry id %q: %v", id, err)
		}
	}
}

func TestTrashSymlinkedParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need privileges on Windows")
	}
	e := newTrashEnv(t)
	outDir := filepath.Join(e.dir, "outside")
	os.Mkdir(outDir, 0o755)
	os.WriteFile(filepath.Join(outDir, "secret.txt"), []byte("secret"), 0o644)
	// The website's user plants a link to a folder outside the site.
	os.Symlink(outDir, filepath.Join(e.site, "up"))

	// Deleting through the link is refused, and the outside file stays.
	if _, err := moveToTrash(e.root, "up/secret.txt", e.scope, "example.com", e.bin, time.Now()); err == nil {
		t.Fatal("moved a file through a link out of the site")
	}
	if b, _ := os.ReadFile(filepath.Join(outDir, "secret.txt")); string(b) != "secret" {
		t.Fatal("outside file changed")
	}

	// Restoring into the linked folder is refused.
	e.write(t, "evil.php", "<?php", 0o644)
	id, err := moveToTrash(e.root, "evil.php", e.scope, "example.com", e.bin, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	writeRawMeta(t, e, id, func(m map[string]any) { m["path"] = "up/evil.php" })
	if _, err := restoreEntry(e.root, e.scope, "example.com", e.bin, id, e.dir+"/trash"); code(err) != "path_escapes" {
		t.Fatalf("restore through a link: %v", err)
	}
	// A missing parent under the link is not created outside either.
	writeRawMeta(t, e, id, func(m map[string]any) { m["path"] = "up/new/evil.php" })
	if _, err := restoreEntry(e.root, e.scope, "example.com", e.bin, id, e.dir+"/trash"); code(err) != "path_escapes" {
		t.Fatalf("restore under a link: %v", err)
	}
	if des, _ := os.ReadDir(outDir); len(des) != 1 {
		t.Fatalf("outside folder holds %d items", len(des))
	}

	// Deleting the link itself moves the link, not its target.
	if _, err := moveToTrash(e.root, "up", e.scope, "example.com", e.bin, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "secret.txt")); err != nil {
		t.Fatal("link target moved")
	}
}

func TestManagerTrash(t *testing.T) {
	m, site := newTest(t)
	ctx := context.Background()
	os.MkdirAll(site+"/public/img", 0o755)
	os.WriteFile(site+"/public/index.php", []byte("<?php"), 0o644)
	os.WriteFile(site+"/public/img/logo.png", []byte("png"), 0o644)
	os.WriteFile(site+"/public/big.iso", []byte("iso"), 0o644)

	if err := m.Delete(ctx, []string{site + "/public/index.php", site + "/public/img"}); err != nil {
		t.Fatal(err)
	}
	if err := m.DeletePermanently(ctx, []string{site + "/public/big.iso"}); err != nil {
		t.Fatal(err)
	}
	l, err := m.ListTrash(ctx, site+"/public")
	if err != nil || l.Count != 2 || l.Site != "example.com" || l.MaxAgeDays != 30 {
		t.Fatalf("ListTrash = %+v, %v", l, err)
	}
	var img string
	for _, e := range l.Entries {
		if e.Path == site+"/public/img" {
			img = e.ID
		}
	}
	if p, err := m.RestoreTrash(ctx, site, img); err != nil || p != site+"/public/img" {
		t.Fatalf("RestoreTrash = %q, %v", p, err)
	}
	if b, _ := os.ReadFile(site + "/public/img/logo.png"); string(b) != "png" {
		t.Fatal("not restored")
	}
	if _, err := os.Stat(site + "/public/big.iso"); err == nil {
		t.Fatal("permanent delete left the file")
	}
	if fi, err := os.Stat(m.trashDir); err != nil || fi.Mode().Perm() != 0o700 {
		t.Fatalf("trash folder: %v %v", fi, err)
	}

	// The trash folder cannot be deleted through the file manager.
	if err := m.Delete(ctx, []string{m.trashDir}); err == nil {
		t.Error("trash folder deleted")
	}

	l, _ = m.ListTrash(ctx, site)
	if err := m.DeleteTrash(ctx, site, l.Entries[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := m.DeleteTrash(ctx, site, l.Entries[0].ID); code(err) != "trash_not_found" {
		t.Fatalf("delete twice: %v", err)
	}
	os.WriteFile(site+"/public/a.txt", []byte("a"), 0o644)
	os.WriteFile(site+"/public/b.txt", []byte("b"), 0o644)
	m.Delete(ctx, []string{site + "/public/a.txt", site + "/public/b.txt"})
	if n, err := m.EmptyTrash(ctx, site); n != 2 || err != nil {
		t.Fatalf("EmptyTrash = %d, %v", n, err)
	}
	if l, _ := m.ListTrash(ctx, site); l.Count != 0 {
		t.Fatalf("trash not empty: %+v", l)
	}
	if err := m.DropSiteTrash("example.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path.Join(m.trashDir, "example.com")); err == nil {
		t.Fatal("site trash still there")
	}
	if err := m.DropSiteTrash("../etc"); err == nil {
		t.Fatal("DropSiteTrash accepted ../etc")
	}
}

func TestTrashNeverRestoresIntoTrash(t *testing.T) {
	e := newTrashEnv(t)
	e.write(t, "bin/x.txt", "x", 0o644)
	id, err := moveToTrash(e.root, "bin/x.txt", e.scope, "example.com", e.bin, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	// As if the scope's folder held the trash folder "bin".
	if _, err := restoreEntry(e.root, e.scope, "example.com", e.bin, id, e.scope.Root+"/bin"); code(err) != "path_protected" {
		t.Fatalf("restore into the trash folder: %v", err)
	}
}
