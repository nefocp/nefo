package hosting

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nefocp/nefo/internal/store"
)

func TestQuarantine(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()
	base := t.TempDir()
	m.paths.WWW = filepath.Join(base, "www")
	m.paths.Quarantine = filepath.Join(base, "quarantine")
	site := &store.Site{Domain: "example.com", SysUser: "example_com"}
	if err := m.store.CreateSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	pub := filepath.Join(m.paths.WWW, "example.com", "public")
	os.MkdirAll(pub, 0o755)
	os.WriteFile(filepath.Join(pub, "bad.php"), []byte("<?php echo 'x';"), 0o640)
	os.WriteFile(filepath.Join(base, "outside.php"), []byte("secret"), 0o600)

	if err := m.ScanSite(ctx, site.ID, false, nopLogger{}); err != nil {
		t.Fatal(err)
	}
	rep, _ := m.ScanReport(ctx, site.ID)
	if rep.FinishedAt == nil || rep.Files != 1 {
		t.Fatalf("report = %+v", rep)
	}

	// Paths that leave the site folder are refused.
	for _, p := range []string{"../../outside.php", "/etc/passwd", "public/../../../outside.php"} {
		if _, err := m.Quarantine(ctx, site.ID, p, "x"); err == nil {
			t.Errorf("quarantined %s", p)
		}
	}
	if runtime.GOOS != "windows" {
		os.Symlink(filepath.Join(base, "outside.php"), filepath.Join(pub, "link.php"))
		if _, err := m.Quarantine(ctx, site.ID, "public/link.php", "x"); err == nil {
			t.Error("quarantined a symbolic link")
		}
	}
	if b, _ := os.ReadFile(filepath.Join(base, "outside.php")); string(b) != "secret" {
		t.Fatal("a file outside the site was touched")
	}

	q, err := m.Quarantine(ctx, site.ID, "public/bad.php", "eval_request")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(pub, "bad.php")); !os.IsNotExist(err) {
		t.Fatal("file still in the site")
	}
	if b, _ := os.ReadFile(m.quarantineFile(q.ID)); string(b) != "<?php echo 'x';" {
		t.Fatalf("quarantined copy = %q", b)
	}
	rep, _ = m.ScanReport(ctx, site.ID)
	if len(rep.Quarantined) != 1 || rep.Quarantined[0].Path != "public/bad.php" {
		t.Fatalf("quarantine list = %+v", rep.Quarantined)
	}

	// Another site cannot restore it.
	other := &store.Site{Domain: "other.com", SysUser: "other_com"}
	m.store.CreateSite(ctx, other)
	if err := m.RestoreQuarantined(ctx, other.ID, q.ID); err == nil {
		t.Fatal("restored through another site")
	}

	// Restore refuses to overwrite a new file at the same place.
	os.WriteFile(filepath.Join(pub, "bad.php"), []byte("new"), 0o644)
	if err := m.RestoreQuarantined(ctx, site.ID, q.ID); err == nil {
		t.Fatal("restore overwrote a file")
	}
	os.Remove(filepath.Join(pub, "bad.php"))
	if err := m.RestoreQuarantined(ctx, site.ID, q.ID); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(filepath.Join(pub, "bad.php"))
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && st.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %v", st.Mode())
	}
	if _, err := os.Stat(m.quarantineFile(q.ID)); !os.IsNotExist(err) {
		t.Fatal("quarantine copy left behind")
	}

	// Quarantine again, then delete for good; deleting the site cleans up.
	q, _ = m.Quarantine(ctx, site.ID, "public/bad.php", "x")
	if err := m.DeleteQuarantined(ctx, site.ID, q.ID); err != nil {
		t.Fatal(err)
	}
	q, _ = m.Quarantine(ctx, site.ID, "public/index.php", "x") // missing file
	if q != nil {
		t.Fatal("quarantined a missing file")
	}
	if err := m.deleteSiteMalware(ctx, site.ID); err != nil {
		t.Fatal(err)
	}
	if rep, _ := m.ScanReport(ctx, site.ID); rep.FinishedAt != nil || len(rep.Quarantined) != 0 {
		t.Fatalf("left after delete: %+v", rep)
	}
}
