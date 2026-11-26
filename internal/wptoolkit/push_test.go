package wptoolkit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// pushFixture is a live site and its staging copy, each with files, and a
// fake panel that records every step in order.
type pushFixture struct {
	k             *Toolkit
	r             *fakeRunner
	live, staging *store.Site
	steps         []string
	failOn        string // a wp-cli command that fails
	failHost      string // a host step that fails
	liveStyle     string // the live theme's style.css at the backup
	inMaintenance bool   // the live site is in maintenance mode before the push
	host          PushHost
}

func newPushFixture(t *testing.T) *pushFixture {
	f := &pushFixture{r: &fakeRunner{}, liveStyle: "old"}
	f.k, _ = newKit(t, f.r)
	f.live = newSite(t, f.k, "example.com", "example_com")
	f.staging = newSite(t, f.k, "staging.example.com", "staging_example_com")
	f.staging.ID = 99
	live, staging := f.k.public(f.live), f.k.public(f.staging)
	liveRoot, stagingRoot := f.k.root("example.com"), f.k.root("staging.example.com")

	writeFile(t, filepath.Join(live, "wp-config.php"), panelConfig)
	writeFile(t, filepath.Join(live, "index.php"), "<?php // index")
	writeFile(t, filepath.Join(live, "wp-content", "themes", "shop", "style.css"), "old")
	writeFile(t, filepath.Join(live, "wp-content", "uploads", "2026", "only-live.jpg"), "JPEG")
	writeFile(t, filepath.Join(live, "wp-content", "object-cache.php"), "<?php // live redis")
	writeFile(t, filepath.Join(live, ".user.ini"), "auto_prepend_file = '"+liveRoot+"/public/wordfence-waf.php'\n")
	// The live files are older: a file of the same size and time counts as
	// unchanged, and files written within one clock tick share their time.
	hourAgo := time.Now().Add(-time.Hour)
	if err := filepath.Walk(live, func(p string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chtimes(p, hourAgo, hourAgo)
	}); err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(staging, "wp-config.php"), strings.ReplaceAll(panelConfig, "wp_example_com", "wp_staging_example_com"))
	writeFile(t, filepath.Join(staging, "index.php"), "<?php // index")
	writeFile(t, filepath.Join(staging, "wp-content", "themes", "shop", "style.css"), "new")
	writeFile(t, filepath.Join(staging, "wp-content", "uploads", "2026", "new.jpg"), "PNG")
	writeFile(t, filepath.Join(staging, "wp-content", "advanced-cache.php"), "<?php // staging page cache")
	writeFile(t, filepath.Join(staging, ".user.ini"), "auto_prepend_file = '"+stagingRoot+"/public/wordfence-waf.php'\n")

	maintenance := fakeMaintenance(live)
	f.r.answer = func(c call) (string, error) {
		w := c.wpLine()
		if f.failOn != "" && strings.HasPrefix(w, f.failOn) {
			f.steps = append(f.steps, "wp "+w+" (fails)")
			return "", cliFail("Something broke.")
		}
		switch {
		case strings.HasPrefix(w, "option get home"), strings.HasPrefix(w, "option get siteurl"):
			if c.user() == "staging_example_com" {
				return `"http:\/\/staging.example.com"`, nil
			}
			return `"https:\/\/example.com"`, nil
		case w == "option get blog_public":
			return "1\n", nil
		case strings.HasPrefix(w, "maintenance-mode"):
			f.steps = append(f.steps, "wp "+w)
			return maintenance(c)
		}
		f.steps = append(f.steps, "wp "+w)
		return "", nil
	}
	style := filepath.Join(live, "wp-content", "themes", "shop", "style.css")
	f.host = PushHost{
		Backup: func(ctx context.Context, s *store.Site, db string, lg Logger) (string, error) {
			if f.failHost == "backup" {
				return "", errors.New("disk full")
			}
			if readFile(t, style) != f.liveStyle || f.k.maintenanceOn(s) != f.inMaintenance {
				t.Error("the live site changed before its backup")
			}
			f.steps = append(f.steps, "backup "+s.Domain+" "+db)
			return "backup #7", nil
		},
		ResetDB: func(ctx context.Context, name string) error {
			if b, _ := os.ReadFile(style); string(b) == "new" {
				f.steps = append(f.steps, "files copied")
			}
			f.steps = append(f.steps, "reset "+name)
			return nil
		},
		CopyDB: func(ctx context.Context, from, to string) error {
			if f.failHost == "copy" {
				return errors.New("import into " + to + ": ERROR 1064")
			}
			f.steps = append(f.steps, "copy "+from+" to "+to)
			return nil
		},
	}
	return f
}

func TestPush(t *testing.T) {
	f := newPushFixture(t)
	live := f.k.public(f.live)
	linked := os.Symlink("/srv/shared-uploads", filepath.Join(live, "wp-content", "shared")) == nil

	lg := &testLog{}
	if err := f.k.Push(context.Background(), f.staging, f.live, PushOptions{Files: true, Database: true}, f.host, lg); err != nil {
		t.Fatalf("%v\n%s", err, lg.String())
	}
	stagingRoot, liveRoot := f.k.root("staging.example.com"), f.k.root("example.com")
	sr := func(from, to string) string {
		return "wp search-replace " + from + " " + to + " --all-tables --precise --report-changed-only"
	}
	want := []string{
		"backup example.com wp_example_com",
		"wp maintenance-mode activate --force",
		"files copied",
		"reset wp_example_com",
		"copy wp_staging_example_com to wp_example_com",
		sr("http://staging.example.com", "https://example.com"),
		sr(`http:\/\/staging.example.com`, `https:\/\/example.com`),
		sr("https://staging.example.com", "https://example.com"),
		sr(`https:\/\/staging.example.com`, `https:\/\/example.com`),
		sr(stagingRoot+"/", liveRoot+"/"),
		"wp option update home https://example.com",
		"wp option update siteurl https://example.com",
		"wp option update blog_public 1",
		"wp cache flush",
		"wp option delete rewrite_rules",
		"wp maintenance-mode deactivate",
	}
	if !slices.Equal(f.steps, want) {
		t.Fatalf("steps:\n  %s\nwant:\n  %s", strings.Join(f.steps, "\n  "), strings.Join(want, "\n  "))
	}

	// Files: added, updated and removed; the live site's own files kept.
	for p, content := range map[string]string{
		"wp-content/themes/shop/style.css": "new",
		"wp-content/uploads/2026/new.jpg":  "PNG",
		"wp-config.php":                    panelConfig,
		"wp-content/object-cache.php":      "<?php // live redis",
		".user.ini":                        "auto_prepend_file = '" + liveRoot + "/public/wordfence-waf.php'\n",
	} {
		if got := readFile(t, filepath.Join(live, p)); got != content {
			t.Errorf("%s = %q, want %q", p, got, content)
		}
	}
	for _, gone := range []string{"wp-content/uploads/2026/only-live.jpg", "wp-content/advanced-cache.php", ".maintenance"} {
		if _, err := os.Lstat(filepath.Join(live, gone)); !os.IsNotExist(err) {
			t.Errorf("%s: %v", gone, err)
		}
	}
	if linked {
		if fi, err := os.Lstat(filepath.Join(live, "wp-content", "shared")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
			t.Errorf("the live site's link was removed: %v", err)
		}
	}
	// The staging copy is only read, as its own user; the live site is
	// changed as the live site's user.
	for _, c := range f.r.calls {
		want := "example_com"
		if c.user() == "staging_example_com" && strings.HasPrefix(c.wpLine(), "option get") {
			continue
		}
		if c.user() != want {
			t.Errorf("%q ran as %q", c.wpLine(), c.user())
		}
	}
	if readFile(t, filepath.Join(f.k.public(f.staging), "wp-content", "themes", "shop", "style.css")) != "new" {
		t.Error("the staging copy changed")
	}
}

func TestPushFilesOnly(t *testing.T) {
	f := newPushFixture(t)
	lg := &testLog{}
	if err := f.k.Push(context.Background(), f.staging, f.live, PushOptions{Files: true}, f.host, lg); err != nil {
		t.Fatalf("%v\n%s", err, lg.String())
	}
	want := []string{"backup example.com ", "wp maintenance-mode activate --force", "wp maintenance-mode deactivate"}
	if !slices.Equal(f.steps, want) {
		t.Fatalf("steps = %q", f.steps)
	}
	if got := readFile(t, filepath.Join(f.k.public(f.live), "wp-content", "themes", "shop", "style.css")); got != "new" {
		t.Fatalf("style.css = %q", got)
	}
	// A second push finds the files unchanged.
	lg, f.liveStyle = &testLog{}, "new"
	if err := f.k.Push(context.Background(), f.staging, f.live, PushOptions{Files: true}, f.host, lg); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(lg.String(), "Copied 1 files (0 MB), 3 unchanged, 0 removed") {
		t.Fatalf("log:\n%s", lg.String())
	}
}

func TestPushDatabaseOnly(t *testing.T) {
	f := newPushFixture(t)
	if err := f.k.Push(context.Background(), f.staging, f.live, PushOptions{Database: true}, f.host, &testLog{}); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(f.steps, "files copied") || !slices.Contains(f.steps, "reset wp_example_com") {
		t.Fatalf("steps = %q", f.steps)
	}
	if got := readFile(t, filepath.Join(f.k.public(f.live), "wp-content", "themes", "shop", "style.css")); got != "old" {
		t.Fatalf("the files were pushed: %q", got)
	}
}

func TestPushFailures(t *testing.T) {
	// A failed backup stops the push before anything changes.
	f := newPushFixture(t)
	f.failHost = "backup"
	err := f.k.Push(context.Background(), f.staging, f.live, PushOptions{Files: true, Database: true}, f.host, &testLog{})
	if err == nil || !strings.Contains(err.Error(), "nothing was pushed") || len(f.steps) != 0 {
		t.Fatalf("err = %v, steps = %q", err, f.steps)
	}
	if readFile(t, filepath.Join(f.k.public(f.live), "wp-content", "themes", "shop", "style.css")) != "old" {
		t.Fatal("files were pushed")
	}

	// A failed search-replace still turns maintenance mode off and points
	// at the backup.
	f = newPushFixture(t)
	f.failOn = "search-replace"
	err = f.k.Push(context.Background(), f.staging, f.live, PushOptions{Files: true, Database: true}, f.host, &testLog{})
	if err == nil || !strings.Contains(err.Error(), "Something broke.") || !strings.Contains(err.Error(), "backup #7") {
		t.Fatalf("err = %v", err)
	}
	if n := len(f.steps); f.steps[n-1] != "wp maintenance-mode deactivate" || f.k.maintenanceOn(f.live) {
		t.Fatalf("steps = %q", f.steps)
	}

	// A failed import, with a live database wp-cli cannot use any more:
	// maintenance mode goes with its file.
	f = newPushFixture(t)
	f.failHost = "copy"
	f.failOn = "maintenance-mode deactivate"
	err = f.k.Push(context.Background(), f.staging, f.live, PushOptions{Database: true}, f.host, &testLog{})
	if err == nil || !strings.Contains(err.Error(), "ERROR 1064") || !strings.Contains(err.Error(), "backup #7") {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.k.public(f.live), ".maintenance")); !os.IsNotExist(err) {
		t.Fatalf(".maintenance left: %v (steps %q)", err, f.steps)
	}
	if slices.ContainsFunc(f.steps, func(s string) bool { return strings.HasPrefix(s, "wp search-replace") }) {
		t.Fatalf("steps = %q", f.steps)
	}

	// A live site wp-cli cannot load still gets its maintenance page.
	f = newPushFixture(t)
	f.failOn = "maintenance-mode activate"
	if err := f.k.Push(context.Background(), f.staging, f.live, PushOptions{Files: true}, f.host, &testLog{}); err != nil {
		t.Fatal(err)
	}
	if want := []string{"backup example.com ", "wp maintenance-mode activate --force (fails)", "wp maintenance-mode deactivate"}; !slices.Equal(f.steps, want) || f.k.maintenanceOn(f.live) {
		t.Fatalf("steps = %q", f.steps)
	}

	// A live database WordPress cannot use: the panel knows the address.
	f = newPushFixture(t)
	f.live.SSL = true
	answer := f.r.answer
	f.r.answer = func(c call) (string, error) {
		if c.user() == "example_com" && strings.HasPrefix(c.wpLine(), "option get") {
			return "", cliFail("Error establishing a database connection.")
		}
		return answer(c)
	}
	if err := f.k.Push(context.Background(), f.staging, f.live, PushOptions{Database: true}, f.host, &testLog{}); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(f.steps, "wp option update home https://example.com") || !slices.Contains(f.steps, "wp option update blog_public 1") {
		t.Fatalf("steps = %q", f.steps)
	}

	// A site that was in maintenance mode before stays in it.
	f = newPushFixture(t)
	f.inMaintenance = true
	writeFile(t, filepath.Join(f.k.public(f.live), ".maintenance"), maintenanceFile)
	if err := f.k.Push(context.Background(), f.staging, f.live, PushOptions{Files: true}, f.host, &testLog{}); err != nil {
		t.Fatal(err)
	}
	if !f.k.maintenanceOn(f.live) || slices.ContainsFunc(f.steps, func(s string) bool { return strings.HasPrefix(s, "wp maintenance-mode") }) {
		t.Fatalf("steps = %q", f.steps)
	}
}

func TestPreparePush(t *testing.T) {
	var ue *usererr.Error
	f := newPushFixture(t)
	if _, err := f.k.PreparePush(f.staging, f.live, PushOptions{}); !errors.As(err, &ue) || ue.Code != "wp_push_nothing" {
		t.Errorf("err = %v", err)
	}
	if _, err := f.k.PreparePush(f.live, f.live, PushOptions{Files: true}); err == nil {
		t.Error("pushed a site onto itself")
	}
	plan, err := f.k.PreparePush(f.staging, f.live, PushOptions{Database: true})
	if err != nil || plan.LiveDB != "wp_example_com" || plan.StagingDB != "wp_staging_example_com" {
		t.Fatalf("plan = %+v, %v", plan, err)
	}

	stagingCfg := filepath.Join(f.k.public(f.staging), "wp-config.php")
	for cfg, code := range map[string]string{
		panelConfig: "wp_push_same_db",
		strings.ReplaceAll(strings.ReplaceAll(panelConfig, "wp_example_com", "wp_staging_example_com"), "$table_prefix = 'wp_';", "$table_prefix = 'wpx_';"): "wp_push_prefix",
		strings.ReplaceAll(panelConfig, "wp_example_com", "wp_staging_example_com") + "define( 'MULTISITE', true );\n":                                       "wp_multisite",
	} {
		writeFile(t, stagingCfg, cfg)
		if _, err := f.k.PreparePush(f.staging, f.live, PushOptions{Files: true, Database: true}); !errors.As(err, &ue) || ue.Code != code {
			t.Errorf("want %s, got %v", code, err)
		}
		// The files alone do not need the database settings.
		if _, err := f.k.PreparePush(f.staging, f.live, PushOptions{Files: true}); err != nil {
			t.Errorf("files only: %v", err)
		}
	}
}
