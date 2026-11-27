package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/store"
)

func readManifest(t *testing.T, file string) manifest {
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
	if hdr, err := tr.Next(); err != nil || hdr.Name != "manifest.json" {
		t.Fatalf("first entry: %v %v", hdr, err)
	}
	var man manifest
	if err := json.NewDecoder(tr).Decode(&man); err != nil {
		t.Fatal(err)
	}
	return man
}

// The backup made before a website is deleted is named for it and is
// never pruned by the website's schedule.
func TestBackupBeforeDeleteIsKept(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	paths := hosting.DefaultPaths(t.TempDir())
	paths.WWW = t.TempDir()
	h := hosting.New(paths, run.System{}, st, slog.New(slog.DiscardHandler), "")
	m := New(st, nil, h, t.TempDir(), slog.New(slog.DiscardHandler))
	clock := time.Date(2026, 11, 27, 3, 0, 0, 0, time.Local)
	m.now = func() time.Time { clock = clock.Add(time.Minute); return clock }
	lg := testLog{t}

	site := &store.Site{Domain: "gone.example.com", SysUser: "gone_example_com"}
	if err := st.CreateSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	makeTree(t, filepath.Join(paths.WWW, site.Domain, "public"))
	sc := &store.Schedule{Kind: KindSite, Target: site.Domain, Frequency: "daily", Hour: 3, Keep: 1}
	if err := st.CreateSchedule(ctx, sc); err != nil {
		t.Fatal(err)
	}
	list, err := m.BackupBeforeDelete(ctx, site.Domain, lg)
	if err != nil || len(list) != 1 {
		t.Fatalf("%v %v", list, err)
	}
	safety := list[0]
	if name := filepath.Base(safety.File); !strings.HasPrefix(name, "gone.example.com_before-delete_20261127-") || safety.ScheduleID != 0 {
		t.Fatalf("backup %s, schedule %d", name, safety.ScheduleID)
	}
	if man := readManifest(t, safety.File); man.Label != LabelBeforeDelete || man.Domain != site.Domain {
		t.Fatalf("manifest %+v", man)
	}
	// Two scheduled backups and the pruning that keeps one.
	for range 2 {
		if _, err := m.Backup(ctx, KindSite, site.Domain, sc.ID, false, lg); err != nil {
			t.Fatal(err)
		}
		if err := m.Prune(ctx, sc, lg); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.BackupByID(ctx, safety.ID); err != nil {
		t.Fatalf("the backup before deletion was pruned: %v", err)
	}
	if _, err := os.Stat(safety.File); err != nil {
		t.Fatal(err)
	}
	if n, _ := st.BackupsFor(ctx, KindSite, site.Domain, sc.ID); len(n) != 1 {
		t.Fatalf("scheduled backups kept: %d", len(n))
	}
	if man := readManifest(t, mustLatest(t, st, sc.ID).File); man.Label != "" || man.DBPasswords != nil {
		t.Fatalf("scheduled manifest %+v", man)
	}
}

func mustLatest(t *testing.T, st *store.Store, sc int64) *store.Backup {
	t.Helper()
	list, err := st.BackupsFor(context.Background(), KindSite, "gone.example.com", sc)
	if err != nil || len(list) == 0 {
		t.Fatal(err)
	}
	return list[0]
}
