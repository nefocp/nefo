package files

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/usererr"
)

// newTest returns a manager whose only scope is a temporary "site" folder.
func newTest(t *testing.T) (*Manager, string) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("the file manager needs Linux")
	}
	dir := t.TempDir()
	site := filepath.Join(dir, "site")
	os.Mkdir(site, 0o755)
	m := New(func(ctx context.Context, abs string) (Scope, error) {
		if abs == site || strings.HasPrefix(abs, site+"/") {
			return Scope{Root: site, UID: os.Getuid(), GID: os.Getgid(), Site: "example.com"}, nil
		}
		return Scope{}, usererr.New("outside_test_scope")
	})
	m.trashDir = filepath.Join(dir, "trash")
	return m, site
}

func code(err error) string {
	var ue *usererr.Error
	if errors.As(err, &ue) {
		return ue.Code
	}
	if err != nil {
		return err.Error()
	}
	return ""
}

func TestEditAndList(t *testing.T) {
	m, site := newTest(t)
	ctx := context.Background()
	if err := m.Mkdir(ctx, site+"/public"); err != nil {
		t.Fatal(err)
	}
	if err := m.WriteText(ctx, site+"/public/index.php", "<?php echo 1;", true); err != nil {
		t.Fatal(err)
	}
	if got := code(m.WriteText(ctx, site+"/public/index.php", "x", true)); got != "path_exists" {
		t.Fatalf("create over existing: %q", got)
	}
	os.Chmod(site+"/public/index.php", 0o640)
	if err := m.WriteText(ctx, site+"/public/index.php", "<?php echo 2;", false); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(site + "/public/index.php"); fi.Mode().Perm() != 0o640 {
		t.Fatalf("mode changed to %v on save", fi.Mode().Perm())
	}
	text, _, err := m.ReadText(ctx, site+"/public/index.php")
	if err != nil || text != "<?php echo 2;" {
		t.Fatalf("ReadText = %q, %v", text, err)
	}
	os.WriteFile(site+"/public/img.bin", []byte{1, 0, 2}, 0o644)
	if got := code(firstErr(m.ReadText(ctx, site+"/public/img.bin"))); got != "binary_file" {
		t.Fatalf("binary: %q", got)
	}
	l, err := m.List(ctx, site+"/public")
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Entries) != 2 || l.Entries[0].Name != "img.bin" || l.Site != "example.com" {
		t.Fatalf("listing = %+v", l)
	}
	if fi, _ := os.Stat(site + "/public"); fi.Mode().Perm() != 0o755 {
		t.Fatalf("new folder mode = %v", fi.Mode().Perm())
	}
}

func firstErr[T any, U any](_ T, _ U, err error) error { return err }

func TestSymlinkCannotEscape(t *testing.T) {
	m, site := newTest(t)
	ctx := context.Background()
	secret := filepath.Join(filepath.Dir(site), "secret.txt")
	os.WriteFile(secret, []byte("top secret"), 0o644)
	os.Symlink(secret, site+"/link.txt")
	os.Symlink(filepath.Dir(site), site+"/up")

	if _, _, err := m.ReadText(ctx, site+"/link.txt"); code(err) != "path_escapes" {
		t.Fatalf("read through link: %v", err)
	}
	if err := m.WriteText(ctx, site+"/link.txt", "pwned", false); err == nil {
		t.Fatal("write through link succeeded")
	}
	if _, err := m.Upload(ctx, site+"/up", "x.txt", strings.NewReader("pwned"), false); err == nil {
		t.Fatal("upload through a linked folder succeeded")
	}
	if b, _ := os.ReadFile(secret); string(b) != "top secret" {
		t.Fatalf("secret changed to %q", b)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(site), "x.txt")); err == nil {
		t.Fatal("file created outside the site")
	}
	// Deleting the link removes the link, not its target.
	if err := m.Delete(ctx, []string{site + "/link.txt"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(secret); err != nil {
		t.Fatal("link target was deleted")
	}
}

func TestGuards(t *testing.T) {
	m, site := newTest(t)
	ctx := context.Background()
	if got := code(m.Delete(ctx, []string{site})); got != "path_protected" {
		t.Fatalf("delete site root: %q", got)
	}
	if got := code(m.Chmod(ctx, site+"/x", "4755")); got != "invalid_mode" {
		t.Fatalf("setuid: %q", got)
	}
	for _, p := range []string{"relative/path", "", "/a\x00b"} {
		if _, err := m.List(ctx, p); code(err) != "invalid_path" {
			t.Errorf("%q: %v", p, err)
		}
	}
	for _, n := range []string{"..", "a/b", "", "."} {
		if _, err := m.Upload(ctx, site, n, strings.NewReader("x"), false); code(err) != "invalid_name" {
			t.Errorf("upload name %q: %v", n, err)
		}
	}
	m.WriteText(ctx, site+"/a.txt", "a", true)
	if _, err := m.Upload(ctx, site, "a.txt", strings.NewReader("b"), false); code(err) != "path_exists" {
		t.Fatalf("upload over existing: %v", err)
	}
	if _, err := m.Upload(ctx, site, "a.txt", strings.NewReader("b"), true); err != nil {
		t.Fatal(err)
	}
}

func TestCompressAndExtract(t *testing.T) {
	for _, name := range []string{"backup.zip", "backup.tar.gz"} {
		t.Run(name, func(t *testing.T) {
			m, site := newTest(t)
			ctx := context.Background()
			os.MkdirAll(site+"/public/wp-content", 0o755)
			os.WriteFile(site+"/public/index.php", []byte("<?php"), 0o644)
			os.WriteFile(site+"/public/wp-content/run.sh", []byte("#!/bin/sh"), 0o755)
			os.Symlink("/etc/passwd", site+"/public/passwd")
			if err := m.Compress(ctx, []string{site + "/public"}, site+"/"+name, nil); err != nil {
				t.Fatal(err)
			}
			os.Mkdir(site+"/restore", 0o755)
			if err := m.Extract(ctx, site+"/"+name, site+"/restore", nil); err != nil {
				t.Fatal(err)
			}
			if b, _ := os.ReadFile(site + "/restore/public/index.php"); string(b) != "<?php" {
				t.Fatalf("index.php = %q", b)
			}
			if fi, err := os.Stat(site + "/restore/public/wp-content/run.sh"); err != nil || fi.Mode().Perm() != 0o755 {
				t.Fatalf("run.sh: %v %v", fi, err)
			}
			if _, err := os.Lstat(site + "/restore/public/passwd"); err == nil {
				t.Fatal("symlink was archived")
			}
		})
	}
}

func TestExtractRejectsTraversal(t *testing.T) {
	m, site := newTest(t)
	ctx := context.Background()
	outside := filepath.Dir(site)

	var zbuf bytes.Buffer
	zw := zip.NewWriter(&zbuf)
	for _, n := range []string{"../evil.txt", "/abs.txt", "ok/../../evil2.txt", "good.txt"} {
		w, _ := zw.Create(n)
		w.Write([]byte("x"))
	}
	zw.Close()
	os.WriteFile(site+"/bad.zip", zbuf.Bytes(), 0o644)

	var tbuf bytes.Buffer
	gz := gzip.NewWriter(&tbuf)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: "../tar-evil.txt", Typeflag: tar.TypeReg, Size: 1, Mode: 0o644})
	tw.Write([]byte("x"))
	tw.WriteHeader(&tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "/etc"})
	tw.WriteHeader(&tar.Header{Name: "suid", Typeflag: tar.TypeReg, Size: 1, Mode: 0o4755})
	tw.Write([]byte("x"))
	tw.Close()
	gz.Close()
	os.WriteFile(site+"/bad.tar.gz", tbuf.Bytes(), 0o644)

	for _, a := range []string{"bad.zip", "bad.tar.gz"} {
		if err := m.Extract(ctx, site+"/"+a, site+"/out", nil); err != nil {
			t.Fatalf("%s: %v", a, err)
		}
	}
	for _, p := range []string{outside + "/evil.txt", outside + "/evil2.txt", outside + "/tar-evil.txt", "/abs.txt", site + "/out/link"} {
		if _, err := os.Lstat(p); err == nil {
			t.Errorf("%s was created", p)
		}
	}
	// Entries are kept inside the target folder: "../evil.txt" becomes out/evil.txt.
	if _, err := os.Stat(site + "/out/good.txt"); err != nil {
		t.Error("good.txt missing")
	}
	if fi, err := os.Stat(site + "/out/suid"); err != nil || fi.Mode()&os.ModeSetuid != 0 {
		t.Errorf("suid bit kept: %v", fi.Mode())
	}
}
