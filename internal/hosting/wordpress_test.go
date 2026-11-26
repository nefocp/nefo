package hosting

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

func tarGz(t *testing.T, entries []tar.Header, bodies map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, h := range entries {
		body := bodies[h.Name]
		h.Size = int64(len(body))
		if h.Typeflag != tar.TypeReg {
			h.Size = 0
		}
		if err := tw.WriteHeader(&h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			tw.Write([]byte(body))
		}
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func TestExtractWordPressStaysInside(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chown is not available on Windows")
	}
	parent := t.TempDir()
	dir := filepath.Join(parent, "public")
	os.Mkdir(dir, 0o755)
	archive := tarGz(t, []tar.Header{
		{Name: "wordpress/", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "wordpress/index.php", Typeflag: tar.TypeReg, Mode: 0o644},
		{Name: "wordpress/wp-admin/", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "wordpress/wp-admin/x.php", Typeflag: tar.TypeReg, Mode: 0o777},
		{Name: "wordpress/../escape.php", Typeflag: tar.TypeReg, Mode: 0o644},
		{Name: "/etc/evil", Typeflag: tar.TypeReg, Mode: 0o644},
		{Name: "wordpress/link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"},
		{Name: "other/file.php", Typeflag: tar.TypeReg, Mode: 0o644},
	}, map[string]string{
		"wordpress/index.php":      "<?php // index",
		"wordpress/wp-admin/x.php": "<?php // admin",
		"wordpress/../escape.php":  "bad",
		"/etc/evil":                "bad",
		"other/file.php":           "bad",
	})
	n, err := extractWordPress(bytes.NewReader(archive), dir, os.Getuid(), os.Getgid())
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("extracted %d files, want 2", n)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "wp-admin", "x.php")); string(b) != "<?php // admin" {
		t.Fatalf("x.php = %q", b)
	}
	if fi, _ := os.Stat(filepath.Join(dir, "wp-admin", "x.php")); fi.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %v, want 0644", fi.Mode().Perm())
	}
	for _, p := range []string{filepath.Join(parent, "escape.php"), filepath.Join(dir, "link"), filepath.Join(dir, "other")} {
		if _, err := os.Lstat(p); err == nil {
			t.Errorf("%s was created", p)
		}
	}
}

func TestWPConfig(t *testing.T) {
	out, err := wpConfig(&store.Database{Name: "wp_shop", Username: "wp_shop", Password: "Secret-123_abc"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	mustContain(t, s, "define( 'DB_NAME', 'wp_shop' );", "define( 'DB_PASSWORD', 'Secret-123_abc' );",
		"define( 'NONCE_SALT', '", "require_once ABSPATH . 'wp-settings.php';")
	// Salts must not break out of their quotes.
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, "_KEY'") || strings.Contains(line, "_SALT'") {
			if strings.Count(line, "'") != 4 || strings.Contains(line, `\`) {
				t.Errorf("unsafe line: %s", line)
			}
		}
	}
	// The password replacement used by UpdateDBPassword finds the line.
	if !wpDefineRe.Match(out) {
		t.Error("DB_PASSWORD line not matched")
	}
}

// WordPress and the other PHP apps take the same administrator passwords.
func TestAdminPassword(t *testing.T) {
	for p, ok := range map[string]bool{
		"":                       false,
		"elevenchars":            false,
		"twelve-chars":           true,
		strings.Repeat("x", 128): true,
		strings.Repeat("x", 129): false,
	} {
		err := checkAdminPassword(p)
		var ue *usererr.Error
		if ok != (err == nil) || (err != nil && (!errors.As(err, &ue) || ue.Code != "invalid_app_password")) {
			t.Errorf("%d characters: %v", len(p), err)
		}
	}
}
