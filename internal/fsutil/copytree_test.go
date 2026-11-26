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
