//go:build !windows

package fsutil

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// Under the panel's umask (077), the folders MkdirAll makes still get
// perm, and folders that were there keep theirs.
func TestMkdirAllIgnoresUmask(t *testing.T) {
	old := syscall.Umask(0o077)
	defer syscall.Umask(old)
	base := t.TempDir()
	if err := os.Chmod(base, 0o711); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(base, "etc", "mysql", "mysql.conf.d")
	if err := MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{filepath.Join(base, "etc"), filepath.Join(base, "etc", "mysql"), deep} {
		fi, err := os.Stat(d)
		if err != nil || fi.Mode().Perm() != 0o755 {
			t.Errorf("%s: %v %v", d, fi.Mode(), err)
		}
	}
	if fi, _ := os.Stat(base); fi.Mode().Perm() != 0o711 {
		t.Errorf("an existing folder changed: %v", fi.Mode())
	}
	if err := MkdirAll(deep, 0o700); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(deep); fi.Mode().Perm() != 0o755 {
		t.Errorf("an existing folder changed: %v", fi.Mode())
	}
	f := filepath.Join(base, "file")
	os.WriteFile(f, nil, 0o600)
	if err := MkdirAll(filepath.Join(f, "x"), 0o755); err == nil {
		t.Error("a folder inside a file")
	}
}
