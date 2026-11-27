package fsutil

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func put(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCopyTreeMirror(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	put(t, filepath.Join(src, "a.txt"), "new a")
	put(t, filepath.Join(src, "same.txt"), "same")
	put(t, filepath.Join(src, "was-dir"), "now a file")
	put(t, filepath.Join(src, "was-file", "inside.txt"), "now a folder")
	put(t, filepath.Join(src, "keep.php"), "from src")
	put(t, filepath.Join(src, "conf.ini"), "path=/src/")

	put(t, filepath.Join(dst, "a.txt"), "old a")
	put(t, filepath.Join(dst, "same.txt"), "same")
	put(t, filepath.Join(dst, "was-dir", "x.txt"), "x")
	put(t, filepath.Join(dst, "was-file"), "a file")
	put(t, filepath.Join(dst, "keep.php"), "dst's own")
	put(t, filepath.Join(dst, "gone", "deep", "y.txt"), "y")
	put(t, filepath.Join(dst, "gone.txt"), "z")
	mtime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, d := range []string{src, dst} {
		if err := os.Chtimes(filepath.Join(d, "same.txt"), mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	// dst's a.txt is older and of the same size: files written within one
	// clock tick share their time, and it would count as unchanged.
	if err := os.Chtimes(filepath.Join(dst, "a.txt"), mtime, mtime); err != nil {
		t.Fatal(err)
	}
	linked := os.Symlink("/elsewhere", filepath.Join(dst, "link")) == nil

	st, err := CopyTree(context.Background(), src, dst, CopyOptions{UID: -1, SrcUID: -1, Mirror: true, Skip: []string{"keep.php"},
		Transform: map[string]func([]byte) ([]byte, error){"conf.ini": func(b []byte) ([]byte, error) { return []byte("path=/dst/"), nil }}})
	if err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]string{
		"a.txt": "new a", "same.txt": "same", "was-dir": "now a file", "was-file/inside.txt": "now a folder",
		"keep.php": "dst's own", "conf.ini": "path=/dst/",
	} {
		if b, err := os.ReadFile(filepath.Join(dst, p)); err != nil || string(b) != want {
			t.Errorf("%s = %q, %v", p, b, err)
		}
	}
	for _, p := range []string{"gone", "gone.txt"} {
		if _, err := os.Lstat(filepath.Join(dst, p)); !os.IsNotExist(err) {
			t.Errorf("%s: %v", p, err)
		}
	}
	if linked {
		if _, err := os.Lstat(filepath.Join(dst, "link")); err != nil {
			t.Error("the link was removed")
		}
	}
	if st.Unchanged != 1 || st.Files != 4 || !slices.Contains(st.Skipped, "keep.php") {
		t.Fatalf("stats = %+v", st)
	}
	// The copy keeps modification times, so the next run leaves it alone.
	fi, err := os.Stat(filepath.Join(dst, "a.txt"))
	si, _ := os.Stat(filepath.Join(src, "a.txt"))
	if err != nil || !fi.ModTime().Equal(si.ModTime()) {
		t.Fatalf("mtime %v, want %v", fi.ModTime(), si.ModTime())
	}
	entries, _ := os.ReadDir(dst)
	for _, e := range entries {
		if len(e.Name()) > 10 && e.Name()[:10] == ".nefo-copy" {
			t.Errorf("temporary file left: %s", e.Name())
		}
	}
	// Only conf.ini, which is rewritten, is written again.
	st, err = CopyTree(context.Background(), src, dst, CopyOptions{UID: -1, SrcUID: -1, Mirror: true, Skip: []string{"keep.php"}})
	if err != nil || st.Files != 1 || st.Unchanged != 4 || st.Removed != 0 {
		t.Fatalf("second run: %+v, %v", st, err)
	}
}

func TestCopyTreeLinks(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	put(t, filepath.Join(src, "lib", "real.js"), "code")
	if err := os.Symlink("../lib/real.js", filepath.Join(src, "bin", "tool")); err != nil {
		if err := os.MkdirAll(filepath.Join(src, "bin"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("../lib/real.js", filepath.Join(src, "bin", "tool")); err != nil {
			t.Skip("no symbolic links here:", err)
		}
	}
	for name, target := range map[string]string{"python": "/usr/bin/python3", "self": "/old/site/lib", "gone": "/drop/me"} {
		if err := os.Symlink(target, filepath.Join(src, "bin", name)); err != nil {
			t.Fatal(err)
		}
	}
	// Without Links, the links are left out as before.
	st, err := CopyTree(context.Background(), src, dst, CopyOptions{UID: -1, SrcUID: -1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dst, "bin", "tool")); err == nil || len(st.Skipped) != 4 {
		t.Fatalf("links copied without Links: %v %q", err, st.Skipped)
	}
	dst = t.TempDir()
	st, err = CopyTree(context.Background(), src, dst, CopyOptions{UID: -1, SrcUID: -1, Links: func(target string) string {
		switch target {
		case "/drop/me":
			return ""
		case "/old/site/lib":
			return "/new/site/lib"
		}
		return target
	}})
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"tool": "../lib/real.js", "python": "/usr/bin/python3", "self": "/new/site/lib"} {
		if got, err := os.Readlink(filepath.Join(dst, "bin", name)); err != nil || filepath.ToSlash(got) != want {
			t.Errorf("%s -> %q (%v), want %q", name, got, err, want)
		}
	}
	if _, err := os.Lstat(filepath.Join(dst, "bin", "gone")); err == nil || !slices.Contains(st.Skipped, "bin/gone") {
		t.Errorf("bin/gone: %v, skipped %q", err, st.Skipped)
	}
	if b, err := os.ReadFile(filepath.Join(dst, "bin", "tool")); err != nil || string(b) != "code" {
		t.Errorf("through the copied link: %q %v", b, err)
	}
}
