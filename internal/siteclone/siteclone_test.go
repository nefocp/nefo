package siteclone

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

type testLog struct{ bytes.Buffer }

func (l *testLog) Logf(format string, args ...any) { fmt.Fprintf(&l.Buffer, format+"\n", args...) }

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// fixture is a PHP website with files and a database, and a fake panel to
// copy it in.
type fixture struct {
	base      string
	from      *store.Site
	dbs       []*store.Database
	steps     []string
	failCopy  bool
	directive string
	host      Host
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{base: t.TempDir()}
	root := func(domain string) string { return filepath.Join(f.base, "www", domain) }
	old := root("shop.example.com")
	f.from = &store.Site{ID: 1, Domain: "shop.example.com", SysUser: "shop_example_com", PHPVersion: "8.3", App: "laravel",
		Rewrite: "laravel", NginxCustom: "location /files { alias " + old + "/storage/files; }\n# " + old + ".au is another site\n"}
	f.dbs = []*store.Database{{ID: 4, Name: "shop_db", Username: "shop_db", SiteID: 1}}
	for _, d := range []string{"public", "logs", "tmp"} {
		if err := os.MkdirAll(filepath.Join(old, d), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(old, "public", "index.php"), "<?php require '../vendor/autoload.php';")
	writeFile(t, filepath.Join(old, "public", ".user.ini"), "auto_prepend_file = '"+old+"/public/prepend.php'\n")
	writeFile(t, filepath.Join(old, ".env"), "DB_DATABASE=shop_db\n")
	writeFile(t, filepath.Join(old, "vendor", "autoload.php"), "<?php")
	writeFile(t, filepath.Join(old, "logs", "php-error.log"), "an error")
	writeFile(t, filepath.Join(old, "tmp", "sess_abc"), "a session")
	writeFile(t, filepath.Join(old, ".ssh", "id_ed25519"), "a deploy key")

	f.host = Host{
		Root:      root,
		LookupIDs: func(string) (int, int, error) { return -1, -1, nil },
		CreateSite: func(ctx context.Context, from *store.Site, domain string) (*store.Site, error) {
			f.steps = append(f.steps, "create site "+domain+" php "+from.PHPVersion)
			for _, d := range []string{"public", "logs", "tmp"} {
				if err := os.MkdirAll(filepath.Join(root(domain), d), 0o750); err != nil {
					return nil, err
				}
			}
			writeFile(t, filepath.Join(root(domain), "public", "index.html"), "welcome")
			return &store.Site{ID: 2, Domain: domain, SysUser: "copy_example_com", PHPVersion: from.PHPVersion}, nil
		},
		DeleteSite: func(ctx context.Context, s *store.Site) error {
			f.steps = append(f.steps, "delete site "+s.Domain)
			return os.RemoveAll(root(s.Domain))
		},
		CopySettings: func(ctx context.Context, from, to *store.Site, directives string, lg Logger) error {
			f.steps = append(f.steps, "settings of "+from.Domain+" to "+to.Domain)
			f.directive = directives
			return nil
		},
		CreateDB: func(ctx context.Context, s *store.Site, from *store.Database) (*store.Database, error) {
			f.steps = append(f.steps, "create db for "+s.Domain+" from "+from.Name)
			return &store.Database{ID: 5, Name: "laravel_copy_example_com", Username: "laravel_copy_example_com", SiteID: s.ID}, nil
		},
		DeleteDB: func(ctx context.Context, db *store.Database) error {
			f.steps = append(f.steps, "delete db "+db.Name)
			return nil
		},
		CopyDB: func(ctx context.Context, from, to string) error {
			if f.failCopy {
				return errors.New("ERROR 1046")
			}
			f.steps = append(f.steps, "copy db "+from+" to "+to)
			return nil
		},
	}
	return f
}

func TestClone(t *testing.T) {
	f := newFixture(t)
	lg := &testLog{}
	res, err := Clone(context.Background(), f.from, "copy.example.com", f.dbs, f.host, lg)
	if err != nil {
		t.Fatalf("%v\n%s", err, lg.String())
	}
	if !slices.Equal(f.steps, []string{
		"create site copy.example.com php 8.3",
		"settings of shop.example.com to copy.example.com",
		"create db for copy.example.com from shop_db",
		"copy db shop_db to laravel_copy_example_com",
	}) {
		t.Fatalf("steps = %q", f.steps)
	}
	if res.Site.Domain != "copy.example.com" || len(res.Databases) != 1 || res.Databases[0].From != "shop_db" || res.Databases[0].To.Name != "laravel_copy_example_com" {
		t.Fatalf("res = %+v", res)
	}

	old, dst := f.host.Root("shop.example.com"), f.host.Root("copy.example.com")
	// The whole site folder, but not its logs, temporary files or deploy key.
	for p, want := range map[string]string{
		"public/index.php":    "<?php require '../vendor/autoload.php';",
		".env":                "DB_DATABASE=shop_db\n",
		"vendor/autoload.php": "<?php",
		"public/.user.ini":    "auto_prepend_file = '" + dst + "/public/prepend.php'\n",
	} {
		if got := readFile(t, filepath.Join(dst, p)); got != want {
			t.Errorf("%s = %q, want %q", p, got, want)
		}
	}
	for _, gone := range []string{"public/index.html", "logs/php-error.log", "tmp/sess_abc", ".ssh"} {
		if _, err := os.Lstat(filepath.Join(dst, gone)); !os.IsNotExist(err) {
			t.Errorf("%s: %v", gone, err)
		}
	}
	// The directives name the new folder, not the old one.
	if want := "location /files { alias " + dst + "/storage/files; }\n# " + old + ".au is another site\n"; f.directive != want {
		t.Fatalf("directives = %q, want %q", f.directive, want)
	}
	// The log says which database the copy uses.
	if !strings.Contains(lg.String(), "still name shop_db") || !strings.Contains(lg.String(), "laravel_copy_example_com") {
		t.Fatalf("log:\n%s", lg.String())
	}
}

func TestCloneWithoutDatabases(t *testing.T) {
	f := newFixture(t)
	f.from.PHPVersion, f.from.App, f.from.Rewrite = "", "", ""
	if _, err := Clone(context.Background(), f.from, "copy.example.com", nil, f.host, &testLog{}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(f.steps, []string{"create site copy.example.com php ", "settings of shop.example.com to copy.example.com"}) {
		t.Fatalf("steps = %q", f.steps)
	}
}

func TestCloneFailureCleansUp(t *testing.T) {
	f := newFixture(t)
	f.failCopy = true
	res, err := Clone(context.Background(), f.from, "copy.example.com", f.dbs, f.host, &testLog{})
	if err == nil || res != nil || !strings.Contains(err.Error(), "ERROR 1046") {
		t.Fatalf("res = %v, err = %v", res, err)
	}
	if n := len(f.steps); n < 2 || f.steps[n-2] != "delete db laravel_copy_example_com" || f.steps[n-1] != "delete site copy.example.com" {
		t.Fatalf("steps = %q", f.steps)
	}
	if _, err := os.Stat(f.host.Root("copy.example.com")); !os.IsNotExist(err) {
		t.Fatalf("the copy's folder is left: %v", err)
	}
	if readFile(t, filepath.Join(f.host.Root("shop.example.com"), ".env")) != "DB_DATABASE=shop_db\n" {
		t.Fatal("the source changed")
	}
}

func TestCloneRefusesApps(t *testing.T) {
	var ue *usererr.Error
	for _, site := range []*store.Site{
		{Domain: "node.example.com", SysUser: "node_example_com", ProxyPort: 3000},
		{Domain: "ghost.example.com", SysUser: "ghost_example_com", App: "docker:ghost"},
	} {
		f := newFixture(t)
		if _, err := Clone(context.Background(), site, "copy.example.com", nil, f.host, &testLog{}); !errors.As(err, &ue) || ue.Code != "clone_app_site" {
			t.Errorf("%s: err = %v", site.Domain, err)
		}
		if len(f.steps) != 0 {
			t.Errorf("steps = %q", f.steps)
		}
	}
	if err := Check(&store.Site{Domain: "x.example.com", SysUser: "x_example_com", PHPVersion: "8.3"}); err != nil {
		t.Errorf("PHP site: %v", err)
	}
}

func TestReplaceRoot(t *testing.T) {
	for in, want := range map[string]string{
		"/var/www/a.com/x":                "/var/www/b.com/x",
		"root /var/www/a.com;":            "root /var/www/b.com;",
		"/var/www/a.com":                  "/var/www/b.com",
		"/var/www/a.com.au/x":             "/var/www/a.com.au/x",
		"/var/www/a.com_2 /var/www/a.com": "/var/www/a.com_2 /var/www/b.com",
	} {
		if got := ReplaceRoot(in, "/var/www/a.com", "/var/www/b.com"); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}
