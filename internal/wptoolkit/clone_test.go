package wptoolkit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// A hand-edited wp-config.php in the style of wp-config-sample.php.
const sampleConfig = `<?php
/** The name of the database for WordPress */
define( "DB_NAME", "wp_example_com" );
// define( 'DB_NAME', 'old_name' );
define('DB_USER','wp_example_com');
define( 'DB_PASSWORD', 'it\'s-old' );
define( 'DB_HOST', 'localhost:/run/mysqld/mysqld.sock' );
define( 'WP_HOME', 'https://example.com' );
define( 'WP_SITEURL', 'https://example.com/wp' );
define( 'FORCE_SSL_ADMIN', true );
define( 'WP_DEBUG_LOG', '/var/www/example.com/logs/wp-debug.log' );
$table_prefix = 'wp_';
/* That's all, stop editing! Happy publishing. */
require_once ABSPATH . 'wp-settings.php';
`

var stagingDB = &store.Database{ID: 9, Name: "wp_staging_example_com", Username: "wp_staging_example_com", Password: "New-Pass.word123"}

func TestRewriteConfig(t *testing.T) {
	out, err := rewriteConfig(sampleConfig, stagingDB, []string{
		"/var/www/example.com/", "/var/www/staging.example.com/", "https://example.com", "http://staging.example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"define( 'DB_NAME', 'wp_staging_example_com' );\n",
		"// define( 'DB_NAME', 'old_name' );\n", // comments stay
		"define( 'DB_USER', 'wp_staging_example_com' );\n",
		"define( 'DB_PASSWORD', 'New-Pass.word123' );\n",
		"define( 'DB_HOST', 'localhost' );\n",
		"define( 'WP_HOME', 'http://staging.example.com' );\n",
		"define( 'WP_SITEURL', 'http://staging.example.com/wp' );\n",
		"define( 'FORCE_SSL_ADMIN', false );\n",
		"define( 'WP_DEBUG_LOG', '/var/www/staging.example.com/logs/wp-debug.log' );\n",
		"$table_prefix = 'wp_';\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "it\\'s-old") || strings.Contains(out, "https://example.com") {
		t.Fatalf("old values left:\n%s", out)
	}

	// The panel's own wp-config.php.
	out, err = rewriteConfig(panelConfig, stagingDB, nil)
	if err != nil || !strings.Contains(out, "define( 'DB_NAME', 'wp_staging_example_com' );\ndefine( 'DB_USER', 'wp_staging_example_com' );\ndefine( 'DB_PASSWORD', 'New-Pass.word123' );\ndefine( 'DB_HOST', 'localhost' );") {
		t.Fatalf("panel config: %v\n%s", err, out)
	}

	// Values the panel cannot rewrite safely.
	var ue *usererr.Error
	for _, cfg := range []string{
		strings.Replace(panelConfig, "define( 'DB_PASSWORD', 'Abc123-secret.pw' );", "define( 'DB_PASSWORD', getenv('DB_PASSWORD') );", 1),
		strings.Replace(panelConfig, "define( 'DB_USER',", "// define( 'DB_USER',", 1),
	} {
		if _, err := rewriteConfig(cfg, stagingDB, nil); !errors.As(err, &ue) || ue.Code != "wp_config_unreadable" {
			t.Errorf("err = %v", err)
		}
	}
	if _, err := rewriteConfig(panelConfig, &store.Database{Name: "x", Username: "x", Password: "it's"}, nil); err == nil {
		t.Error("accepted a password with a quote")
	}
}

func TestParseCloneSource(t *testing.T) {
	src, err := parseCloneSource(sampleConfig)
	if err != nil || src.DBName != "wp_example_com" {
		t.Fatalf("src = %+v, %v", src, err)
	}
	var ue *usererr.Error
	for cfg, code := range map[string]string{
		strings.Replace(panelConfig, "'localhost'", "'db.example.net'", 1):                       "wp_remote_db",
		strings.Replace(panelConfig, "'localhost'", "'127.0.0.1:3307'", 1):                       "wp_remote_db",
		panelConfig + "define( 'MULTISITE', true );\n":                                           "wp_multisite",
		strings.Replace(panelConfig, "'DB_NAME', 'wp_example_com'", "'DB_NAME', 'My-Shop'", 1):   "wp_db_unsupported",
		strings.Replace(panelConfig, "define( 'DB_NAME', 'wp_example_com' );", "", 1):            "wp_config_unreadable",
		strings.Replace(panelConfig, "'DB_NAME', 'wp_example_com'", "'DB_NAME', $_ENV['DB']", 1): "wp_config_unreadable",
	} {
		if _, err := parseCloneSource(cfg); !errors.As(err, &ue) || ue.Code != code {
			t.Errorf("want %s, got %v", code, err)
		}
	}
	for _, h := range []string{"localhost", "127.0.0.1", "localhost:3306", "LOCALHOST", "localhost:/var/run/mysqld/mysqld.sock"} {
		if !localDBHost(h) {
			t.Errorf("%q is local", h)
		}
	}
}

// cloneFixture is a live site with files, and a fake panel to create the
// staging site in.
type cloneFixture struct {
	k      *Toolkit
	r      *fakeRunner
	live   *store.Site
	steps  []string
	failOn string // a wp-cli command that fails
	host   CloneHost
}

func newCloneFixture(t *testing.T) *cloneFixture {
	f := &cloneFixture{r: &fakeRunner{}}
	f.k, _ = newKit(t, f.r)
	f.live = newSite(t, f.k, "example.com", "example_com")
	f.live.Rewrite = "wordpress"
	public := f.k.public(f.live)
	cfg := strings.Replace(panelConfig, "define( 'WP_DEBUG', false );",
		"define( 'WP_DEBUG_LOG', '"+f.k.root("example.com")+"/logs/wp-debug.log' );", 1)
	writeFile(t, filepath.Join(public, "wp-config.php"), cfg)
	writeFile(t, filepath.Join(public, "index.php"), "<?php require __DIR__ . '/wp-blog-header.php';")
	writeFile(t, filepath.Join(public, "wp-content", "uploads", "2026", "11", "photo.jpg"), "JPEG")
	writeFile(t, filepath.Join(public, "wp-content", "object-cache.php"), "<?php // redis")
	writeFile(t, filepath.Join(public, ".user.ini"), "auto_prepend_file = '"+f.k.root("example.com")+"/public/wordfence-waf.php'\n")
	f.r.answer = func(c call) (string, error) {
		w := c.wpLine()
		if f.failOn != "" && strings.HasPrefix(w, f.failOn) {
			return "", cliFail("Something broke.")
		}
		switch w {
		case "option get home --format=json":
			return `"https:\/\/example.com"`, nil
		case "option get siteurl --format=json":
			return `"https:\/\/example.com"`, nil
		}
		return "", nil
	}
	f.host = CloneHost{
		CreateSite: func(ctx context.Context, domain, php string) (*store.Site, error) {
			f.steps = append(f.steps, "create site "+domain+" php "+php)
			s := newSite(t, f.k, domain, "staging_example_com")
			writeFile(t, filepath.Join(f.k.public(s), "index.html"), "placeholder")
			return s, nil
		},
		DeleteSite: func(ctx context.Context, s *store.Site) error {
			f.steps = append(f.steps, "delete site "+s.Domain)
			return os.RemoveAll(f.k.root(s.Domain))
		},
		CreateDB: func(ctx context.Context, s *store.Site) (*store.Database, error) {
			f.steps = append(f.steps, "create db for "+s.Domain)
			return stagingDB, nil
		},
		DeleteDB: func(ctx context.Context, db *store.Database) error {
			f.steps = append(f.steps, "delete db "+db.Name)
			return nil
		},
		CopyDB: func(ctx context.Context, from, to string) error {
			f.steps = append(f.steps, "copy db "+from+" to "+to)
			return nil
		},
		MarkWordPress: func(ctx context.Context, from, to *store.Site, lg Logger) error {
			f.steps = append(f.steps, "mark "+to.Domain+" like "+from.Domain)
			return nil
		},
	}
	return f
}

func TestClone(t *testing.T) {
	f := newCloneFixture(t)
	public := f.k.public(f.live)
	linked := os.Symlink(filepath.Join(public, "wp-config.php"), filepath.Join(public, "config-link.php")) == nil

	lg := &testLog{}
	to, err := f.k.Clone(context.Background(), f.live, "staging.example.com", f.host, lg)
	if err != nil {
		t.Fatalf("%v\n%s", err, lg.String())
	}
	if to.Domain != "staging.example.com" {
		t.Fatalf("to = %+v", to)
	}
	if !slices.Equal(f.steps, []string{
		"create site staging.example.com php 8.3", "create db for staging.example.com",
		"copy db wp_example_com to wp_staging_example_com", "mark staging.example.com like example.com",
	}) {
		t.Fatalf("steps = %q", f.steps)
	}

	dst := f.k.public(to)
	if readFile(t, filepath.Join(dst, "wp-content", "uploads", "2026", "11", "photo.jpg")) != "JPEG" {
		t.Fatal("upload not copied")
	}
	for _, gone := range []string{"index.html", "wp-content/object-cache.php", "config-link.php"} {
		if _, err := os.Lstat(filepath.Join(dst, gone)); !os.IsNotExist(err) {
			t.Errorf("%s: %v", gone, err)
		}
	}
	cfg := readFile(t, filepath.Join(dst, "wp-config.php"))
	newRoot := f.k.root("staging.example.com")
	if !strings.Contains(cfg, "define( 'DB_NAME', 'wp_staging_example_com' );") ||
		!strings.Contains(cfg, "define( 'DB_PASSWORD', 'New-Pass.word123' );") ||
		!strings.Contains(cfg, newRoot+"/logs/wp-debug.log") || strings.Contains(cfg, "Abc123-secret.pw") {
		t.Fatalf("wp-config.php:\n%s", cfg)
	}
	if ini := readFile(t, filepath.Join(dst, ".user.ini")); !strings.Contains(ini, newRoot+"/public/wordfence-waf.php") {
		t.Fatalf(".user.ini = %q", ini)
	}

	wantLines(t, f.r.wpLines(), []string{
		"option get home --format=json",
		"option get siteurl --format=json",
		"search-replace https://example.com http://staging.example.com --all-tables --precise --report-changed-only",
		`search-replace https:\/\/example.com http:\/\/staging.example.com --all-tables --precise --report-changed-only`,
		"option update blog_public 0",
	})
	// The live site is only read, as its own user; the copy is changed as
	// the new site's user.
	for _, c := range f.r.calls {
		want := "staging_example_com"
		if strings.HasPrefix(c.wpLine(), "option get") {
			want = "example_com"
		}
		if c.user() != want {
			t.Errorf("%q ran as %q", c.wpLine(), c.user())
		}
	}
	if linked && !strings.Contains(lg.String(), "config-link.php") {
		t.Errorf("the left-out link is not in the log:\n%s", lg.String())
	}
}

func TestCloneFailureCleansUp(t *testing.T) {
	f := newCloneFixture(t)
	f.failOn = "search-replace"
	lg := &testLog{}
	to, err := f.k.Clone(context.Background(), f.live, "staging.example.com", f.host, lg)
	if err == nil || to != nil || !strings.Contains(err.Error(), "Something broke.") {
		t.Fatalf("to = %v, err = %v", to, err)
	}
	// The database goes first, then the site with its files.
	if n := len(f.steps); n < 2 || f.steps[n-2] != "delete db wp_staging_example_com" || f.steps[n-1] != "delete site staging.example.com" {
		t.Fatalf("steps = %q", f.steps)
	}
	if _, err := os.Stat(f.k.root("staging.example.com")); !os.IsNotExist(err) {
		t.Fatalf("staging folder left: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f.k.public(f.live), "wp-config.php")); err != nil {
		t.Fatal("the live site was touched")
	}

	// A live site that cannot be copied stops before anything is created.
	f = newCloneFixture(t)
	writeFile(t, filepath.Join(f.k.public(f.live), "wp-config.php"), panelConfig+"define( 'MULTISITE', true );\n")
	var ue *usererr.Error
	if _, err := f.k.Clone(context.Background(), f.live, "staging.example.com", f.host, lg); !errors.As(err, &ue) || ue.Code != "wp_multisite" {
		t.Fatalf("err = %v", err)
	}
	if len(f.steps) != 0 {
		t.Fatalf("steps = %q", f.steps)
	}
}
