package database

import (
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestRotateSlowLog(t *testing.T) {
	m, f := newTest(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 27, 6, 30, 0, 0, time.UTC)
	// No server, or no log: nothing to do.
	if done, err := m.RotateSlowLog(ctx, now); done || err != nil {
		t.Fatalf("without a server: %v %v", done, err)
	}
	m.detect = func() Engine { return MariaDB }
	if done, err := m.RotateSlowLog(ctx, now); done || err != nil {
		t.Fatalf("without a log: %v %v", done, err)
	}
	// A small, recent log stays.
	recent := "# Time: 261126 10:00:00\n# User@Host: shop[shop] @ localhost []\n# Query_time: 1.0\nSET timestamp=1764151200;\nSELECT 1;\n"
	os.WriteFile(m.slowLog, []byte(recent), 0o640)
	if done, err := m.RotateSlowLog(ctx, now); done || err != nil {
		t.Fatalf("small and recent: %v %v", done, err)
	}
	// Past 50 MB it goes, and the server reopens its log.
	big := strings.Repeat("SELECT 'x';\n", (SlowRotateMB<<20)/12+1)
	os.WriteFile(m.slowLog, []byte(recent+big), 0o640)
	done, err := m.RotateSlowLog(ctx, now)
	if !done || err != nil {
		t.Fatalf("big: %v %v", done, err)
	}
	rotated := m.slowLog + "-20261127-063000.gz"
	gz, err := os.Open(rotated)
	if err != nil {
		t.Fatal(err)
	}
	zr, _ := gzip.NewReader(gz)
	b, _ := io.ReadAll(zr)
	gz.Close()
	if len(b) != len(recent)+len(big) {
		t.Errorf("rotated log has %d bytes", len(b))
	}
	if fileExists(m.slowLog) || fileExists(m.slowLog+"-20261127-063000") {
		t.Error("the log was not moved away, or the uncompressed copy stayed")
	}
	if !slices.Contains(f.sql, "FLUSH SLOW LOGS;") {
		t.Errorf("no FLUSH SLOW LOGS: %q", f.sql)
	}
	// A week-old first entry rotates a small log; four old ones are kept.
	for _, d := range []string{"20250101", "20250102", "20250103", "20250104"} {
		os.WriteFile(m.slowLog+"-"+d+"-000000.gz", []byte("x"), 0o640)
	}
	old := "# Time: 261101  9:00:00\n# User@Host: shop[shop] @ localhost []\n# Query_time: 1.0\nSELECT 1;\n"
	os.WriteFile(m.slowLog, []byte(old), 0o640)
	if done, err := m.RotateSlowLog(ctx, now.Add(time.Hour)); !done || err != nil {
		t.Fatalf("old: %v %v", done, err)
	}
	got, _ := filepath.Glob(m.slowLog + "-*.gz")
	want := []string{m.slowLog + "-20250103-000000.gz", m.slowLog + "-20250104-000000.gz", rotated, m.slowLog + "-20261127-073000.gz"}
	if !slices.Equal(got, want) {
		t.Errorf("kept:\n%s", strings.Join(got, "\n"))
	}
	r := m.slowRotation()
	if r.Files != 4 || r.MB != 50 || r.Days != 7 || r.Keep != 4 {
		t.Errorf("rotation %+v", r)
	}
	// A link in place of the log is refused (the directory is the mysql user's).
	os.Symlink(filepath.Join(t.TempDir(), "elsewhere"), m.slowLog)
	if done, _ := m.RotateSlowLog(ctx, now); done {
		t.Error("rotated a symlink")
	}
}
