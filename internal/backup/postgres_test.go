package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/store"
)

// fakePG stands in for the PostgreSQL manager.
type fakePG struct {
	st       *store.Store
	mu       sync.Mutex
	imported map[string]string
	resets   []string
	created  []string
}

func (f *fakePG) Dump(ctx context.Context, name string, w io.Writer) error {
	_, err := io.WriteString(w, "-- pg dump of "+name+"\nCREATE TABLE t (a int);\n")
	return err
}

func (f *fakePG) Create(ctx context.Context, name, password string, siteID int64) (*store.Database, error) {
	f.mu.Lock()
	f.created = append(f.created, name)
	f.mu.Unlock()
	d := &store.Database{Name: name, Username: name, Password: "New-Password-123", SiteID: siteID, Engine: store.EnginePostgres}
	return d, f.st.CreateDatabase(ctx, d)
}

func (f *fakePG) Reset(ctx context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resets = append(f.resets, name)
	return nil
}

func (f *fakePG) Import(ctx context.Context, name string, r io.Reader) error {
	b, err := io.ReadAll(r)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.imported == nil {
		f.imported = map[string]string{}
	}
	f.imported[name] = string(b)
	return err
}

// readArchive returns the manifest and the regular files of a backup.
func readArchive(t *testing.T, file string) (manifest, map[string]string) {
	t.Helper()
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	var man manifest
	files := map[string]string{}
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(tr)
		if hdr.Name == "manifest.json" {
			if err := json.Unmarshal(b, &man); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if hdr.Typeflag == tar.TypeReg {
			files[hdr.Name] = string(b)
		}
	}
	return man, files
}

func TestSiteBackupIncludesPostgres(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	www := t.TempDir()
	log := slog.New(slog.DiscardHandler)
	h := hosting.New(hosting.Paths{WWW: www}, nil, st, log, "")
	m := New(st, nil, h, t.TempDir(), log)

	site := &store.Site{Domain: "example.com", SysUser: "example_com"}
	if err := st.CreateSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(www, "example.com", "public"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(www, "example.com", "public", "index.php"), []byte("<?php"), 0o644)
	for _, name := range []string{"app", "app_queue"} {
		if err := st.CreateDatabase(ctx, &store.Database{Name: name, Username: name, Password: "x", SiteID: site.ID, Engine: store.EnginePostgres}); err != nil {
			t.Fatal(err)
		}
	}
	// Without PostgreSQL support the backup is refused, not made without them.
	if _, err := m.BackupSite(ctx, "example.com", 0, false, testLog{t}); err == nil {
		t.Fatal("backed up without the PostgreSQL databases")
	}

	pg := &fakePG{st: st}
	m.SetPostgres(pg)
	b, err := m.BackupSite(ctx, "example.com", 0, false, testLog{t})
	if err != nil {
		t.Fatal(err)
	}
	man, files := readArchive(t, b.File)
	if len(man.PGDatabases) != 2 || man.PGDatabases[0] != "app" || len(man.Databases) != 0 {
		t.Fatalf("manifest %+v", man)
	}
	if files["postgres/app.sql"] != "-- pg dump of app\nCREATE TABLE t (a int);\n" || files["postgres/app_queue.sql"] == "" ||
		files["files/public/index.php"] != "<?php" {
		t.Fatalf("files %v", files)
	}
}

func TestPostgresDatabaseBackupAndRestore(t *testing.T) {
	ctx := context.Background()
	m, st := newTestManager(t)
	pg := &fakePG{st: st}
	m.SetPostgres(pg)
	if err := st.CreateDatabase(ctx, &store.Database{Name: "app", Username: "app", Password: "x", Engine: store.EnginePostgres}); err != nil {
		t.Fatal(err)
	}
	if err := m.CheckTarget(ctx, KindDatabase, "app"); err != nil {
		t.Fatalf("a PostgreSQL database is not a backup target: %v", err)
	}
	if err := m.CheckTarget(ctx, KindDatabase, "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
	b, err := m.BackupDatabase(ctx, "app", 0, false, testLog{t})
	if err != nil {
		t.Fatal(err)
	}
	man, files := readArchive(t, b.File)
	if len(man.PGDatabases) != 1 || len(man.Databases) != 0 || files["postgres/app.sql"] == "" {
		t.Fatalf("%+v %v", man, files)
	}

	if err := m.Restore(ctx, b.ID, testLog{t}); err != nil {
		t.Fatal(err)
	}
	if len(pg.resets) != 1 || pg.resets[0] != "app" || pg.imported["app"] != files["postgres/app.sql"] || len(pg.created) != 0 {
		t.Fatalf("resets %v, imported %v, created %v", pg.resets, pg.imported, pg.created)
	}

	// A database deleted since is created again before the restore.
	d, _ := st.PGDatabaseByName(ctx, "app")
	if err := st.DeleteDatabase(ctx, d.ID); err != nil {
		t.Fatal(err)
	}
	if err := m.Restore(ctx, b.ID, testLog{t}); err != nil {
		t.Fatal(err)
	}
	if len(pg.created) != 1 || pg.created[0] != "app" || len(pg.resets) != 2 {
		t.Fatalf("created %v, resets %v", pg.created, pg.resets)
	}
}
