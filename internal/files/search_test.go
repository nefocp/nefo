package files

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// searchTree writes files (path relative to a new folder, with /) and
// returns the folder opened as an os.Root.
func searchTree(t *testing.T, files map[string]string) (*os.Root, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "site")
	for p, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(p))
		os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	os.MkdirAll(dir, 0o755)
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	return root, dir
}

func hitPaths(r *SearchResult) string {
	var ps []string
	for _, h := range r.Hits {
		ps = append(ps, strings.TrimPrefix(h.Path, "/site/"))
	}
	return strings.Join(ps, ",")
}

func TestSearch(t *testing.T) {
	root, _ := searchTree(t, map[string]string{
		"a/Readme.MD":      "Hello World\nsecond line with Needle here\n",
		"a/b/needle.txt":   "nothing to see",
		"c/data.bin":       "needle\x00\x01\x02",
		"c/big.txt":        strings.Repeat("x", 100) + " needle",
		"vi.txt":           "Xin chào\nTHẾ GIỚI rộng lớn",
		"wp-config.php":    "define('DB_PASSWORD', 'secret');",
		"a/NEEDLES/x.html": "<p>hi</p>",
	})
	ctx := context.Background()
	lim := defaultSearchLimits
	lim.fileSize = 64

	r, err := search(ctx, root, ".", "/site", "needle", "", lim)
	if err != nil {
		t.Fatal(err)
	}
	if got := hitPaths(r); got != "a/NEEDLES,a/b/needle.txt" {
		t.Errorf("name search = %s", got)
	}
	if r.Hits[0].Type != "dir" || r.Hits[1].Type != "file" || r.Hits[1].Size != 14 {
		t.Errorf("hits = %+v", r.Hits)
	}

	r, _ = search(ctx, root, ".", "/site", "", "NEEDLE", lim)
	if got := hitPaths(r); got != "a/Readme.MD" {
		t.Errorf("content search = %s", got)
	}
	if r.Skipped != 2 { // the binary file and the one over the size limit
		t.Errorf("skipped = %d", r.Skipped)
	}
	if h := r.Hits[0]; h.Line != 2 || h.Excerpt != "second line with Needle here" {
		t.Errorf("hit = %+v", h)
	}

	r, _ = search(ctx, root, ".", "/site", "", "thế giới", lim)
	if got := hitPaths(r); got != "vi.txt" || r.Hits[0].Line != 2 {
		t.Errorf("unicode content search = %s %+v", got, r.Hits)
	}

	// Name and content together.
	r, _ = search(ctx, root, ".", "/site", ".php", "db_password", lim)
	if got := hitPaths(r); got != "wp-config.php" {
		t.Errorf("name and content = %s", got)
	}
	r, _ = search(ctx, root, ".", "/site", ".html", "db_password", lim)
	if len(r.Hits) != 0 {
		t.Errorf("name and content mismatch = %s", hitPaths(r))
	}

	// Inside a sub-folder only.
	r, _ = search(ctx, root, "a", "/site", "html", "", lim)
	if got := hitPaths(r); got != "a/NEEDLES/x.html" {
		t.Errorf("sub-folder search = %s", got)
	}
}

func TestSearchLimits(t *testing.T) {
	files := map[string]string{}
	for i := range 60 {
		files[fmt.Sprintf("d%02d/file%02d.txt", i%6, i)] = "match me"
	}
	root, _ := searchTree(t, files)
	ctx := context.Background()

	lim := defaultSearchLimits
	lim.walk = 10
	r, _ := search(ctx, root, ".", "/site", "file", "", lim)
	if r.Walked != 10 || !r.Truncated || len(r.Hits) > 10 {
		t.Errorf("walk limit: walked %d, %d hits, truncated %v", r.Walked, len(r.Hits), r.Truncated)
	}

	lim = defaultSearchLimits
	lim.hits = 5
	r, _ = search(ctx, root, ".", "/site", "", "MATCH", lim)
	if len(r.Hits) != 5 || !r.Truncated {
		t.Errorf("hit limit: %d hits, truncated %v", len(r.Hits), r.Truncated)
	}

	lim = defaultSearchLimits
	r, _ = search(ctx, root, ".", "/site", "file", "", lim)
	if len(r.Hits) != 60 || r.Truncated {
		t.Errorf("no limit: %d hits, truncated %v", len(r.Hits), r.Truncated)
	}

	lim.budget = -time.Second
	r, _ = search(ctx, root, ".", "/site", "file", "", lim)
	if !r.Truncated || len(r.Hits) != 0 {
		t.Errorf("time budget: %d hits, truncated %v", len(r.Hits), r.Truncated)
	}

	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := search(cctx, root, ".", "/site", "file", "", defaultSearchLimits); err == nil {
		t.Error("cancelled search succeeded")
	}
}

func TestSearchDoesNotFollowLinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need privileges on Windows")
	}
	root, dir := searchTree(t, map[string]string{"inside.txt": "needle"})
	outside := filepath.Join(filepath.Dir(dir), "outside")
	os.Mkdir(outside, 0o755)
	os.WriteFile(filepath.Join(outside, "needle-secret.txt"), []byte("needle"), 0o644)
	os.Symlink(outside, filepath.Join(dir, "needle-link"))
	os.Symlink(filepath.Join(outside, "needle-secret.txt"), filepath.Join(dir, "file-link.txt"))

	r, err := search(context.Background(), root, ".", "/site", "needle", "", defaultSearchLimits)
	if err != nil {
		t.Fatal(err)
	}
	if got := hitPaths(r); got != "needle-link" || r.Hits[0].Type != "link" {
		t.Errorf("name search = %s %+v", got, r.Hits)
	}
	r, _ = search(context.Background(), root, ".", "/site", "", "needle", defaultSearchLimits)
	if got := hitPaths(r); got != "inside.txt" {
		t.Errorf("content search followed a link: %s", got)
	}
}

func TestExcerpt(t *testing.T) {
	line := strings.Repeat("a", 300) + "NEEDLE" + strings.Repeat("é", 200)
	b := []byte("first\n" + line + "\nlast")
	start := strings.Index(string(b), "NEEDLE")
	n, ex := lineAndExcerpt(b, start, start+6)
	if n != 2 || !strings.Contains(ex, "NEEDLE") || !strings.HasPrefix(ex, "…") || !strings.HasSuffix(ex, "…") {
		t.Fatalf("line %d excerpt %q", n, ex)
	}
	if len(ex) > excerptWidth+8 || strings.ContainsRune(ex, '�') {
		t.Fatalf("excerpt %q is %d bytes or splits a character", ex, len(ex))
	}
	if _, ex := lineAndExcerpt([]byte("\tshort needle line\r"), 7, 13); ex != "short needle line" {
		t.Fatalf("short excerpt %q", ex)
	}
}

func TestManagerSearch(t *testing.T) {
	m, site := newTest(t)
	ctx := context.Background()
	os.MkdirAll(site+"/public/wp-content", 0o755)
	os.WriteFile(site+"/public/wp-content/evil.php", []byte("<?php eval($_POST['x']);"), 0o644)
	r, err := m.Search(ctx, site+"/public", "", "EVAL(")
	if err != nil || len(r.Hits) != 1 || r.Hits[0].Path != site+"/public/wp-content/evil.php" || r.Path != site+"/public" {
		t.Fatalf("Search = %+v, %v", r, err)
	}
	if _, err := m.Search(ctx, site, " ", " "); code(err) != "search_empty" {
		t.Errorf("empty search: %v", err)
	}
	if _, err := m.Search(ctx, site+"/public/wp-content/evil.php", "x", ""); code(err) != "not_a_folder" {
		t.Errorf("search in a file: %v", err)
	}
}

func TestSearchSkipsPseudoFileSystems(t *testing.T) {
	root, _ := searchTree(t, map[string]string{"proc/kmsg": "x", "sys/kernel/x": "x", "etc/kmsg.conf": "x", "srv/empty": ""})
	r, err := search(context.Background(), root, ".", "/", "kmsg", "", defaultSearchLimits)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Hits) != 1 || r.Hits[0].Path != "/etc/kmsg.conf" {
		t.Errorf("hits = %+v", r.Hits)
	}
	// Empty files cannot hold the text and are not read.
	r, _ = search(context.Background(), root, ".", "/", "", "x", defaultSearchLimits)
	if len(r.Hits) != 1 || r.Skipped != 0 {
		t.Errorf("content hits = %+v, skipped %d", r.Hits, r.Skipped)
	}
}
