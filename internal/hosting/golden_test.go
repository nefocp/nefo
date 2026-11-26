package hosting

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/store"
)

// The golden files pin every Nginx file the panel generates, so a change to
// the templates or to the code that feeds them cannot alter an existing
// Nginx server by accident. Regenerate them only for an intended change:
//
//	go test ./internal/hosting -run Golden -update-golden
//
// The Manager-level cases use real paths, which Windows would write with
// backslashes: they run on Linux only (testdata/golden/nginx-linux).
var updateGolden = flag.Bool("update-golden", false, "rewrite the golden files")

func checkGolden(t *testing.T, dir, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", "golden", dir, name)
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: %v (run with -update-golden to create it)", name, err)
	}
	if string(want) != string(got) {
		t.Errorf("%s differs from its golden file.\n--- got:\n%s\n--- want:\n%s", name, got, want)
	}
}

// nginxVhostCases are the server block variants: every mode and option.
func nginxVhostCases() map[string]vhostData {
	php := func(d vhostData) vhostData { d.PHPSocket = "/run/php/panel-example_com.sock"; return d }
	ssl := func(d vhostData) vhostData {
		d.SSL = true
		d.CertFile = "/var/lib/nefo/certs/example.com/fullchain.pem"
		d.KeyFile = "/var/lib/nefo/certs/example.com/privkey.pem"
		return d
	}
	force := func(d vhostData) vhostData { d = ssl(d); d.ForceHTTPS = true; return d }
	newNginx := func(d vhostData) vhostData { d.nginxFeatures = featuresFor("1.26.3", false); return d }
	newNginxV6 := func(d vhostData) vhostData { d.nginxFeatures = featuresFor("1.26.3", true); return d }
	oldNginx := func(d vhostData) vhostData { d.nginxFeatures = featuresFor("1.18.0", true); return d }
	app := func(d vhostData) vhostData { d.ProxyPort = 3000; return d }
	id := func(d vhostData) vhostData { d.ID = 42; return d }

	base := id(baseVhost())
	cases := map[string]vhostData{
		"static":            base,
		"static-new-noipv6": newNginx(base),
		"static-new-ipv6":   newNginxV6(base),
		"static-old":        oldNginx(base),
		"php":               php(base),
		"php-ssl":           ssl(php(base)),
		"php-ssl-new":       newNginx(ssl(php(base))),
		"php-ssl-new-ipv6":  newNginxV6(ssl(php(base))),
		"php-force":         force(php(base)),
		"php-force-new":     newNginx(force(php(base))),
		"static-ssl":        ssl(base),
		"static-force":      force(base),
		"app":               app(base),
		"app-ssl":           ssl(app(base)),
		"app-force":         force(app(base)),
		"app-force-new":     newNginx(force(app(base))),
		"static-noaliases":  func() vhostData { d := base; d.Aliases = nil; return d }(),
		"static-many-aliases": func() vhostData {
			d := base
			d.Aliases = []string{"www.example.com", "*.example.net", "shop.example.org"}
			return d
		}(),
		"php-limits":        func() vhostData { d := php(base); d.UploadMB, d.TimeoutSec = 512, 630; return d }(),
		"app-limits":        func() vhostData { d := app(base); d.UploadMB = 200; return d }(),
		"suspended":         func() vhostData { d := php(base); d.Disabled = true; return d }(),
		"suspended-ssl":     func() vhostData { d := ssl(php(base)); d.Disabled = true; return d }(),
		"suspended-force":   func() vhostData { d := force(php(base)); d.Disabled = true; return d }(),
		"redirect":          func() vhostData { d := base; d.RedirectURL = "https://new.example.org"; return d }(),
		"redirect-path-ssl": func() vhostData { d := ssl(base); d.RedirectURL = "https://new.example.org:8443/sub/dir"; return d }(),
		"auth":              func() vhostData { d := php(base); d.AuthFile = "/etc/nginx/panel/auth/example.com.htpasswd"; return d }(),
		"auth-force": func() vhostData {
			d := force(php(base))
			d.AuthFile = "/etc/nginx/panel/auth/example.com.htpasswd"
			return d
		}(),
		"auth-app":        func() vhostData { d := app(base); d.AuthFile = "/etc/nginx/panel/auth/example.com.htpasswd"; return d }(),
		"waf-detect":      func() vhostData { d := php(base); d.WAF = WAFDetect; return d }(),
		"waf-block":       func() vhostData { d := php(base); d.WAF = WAFBlock; return d }(),
		"waf-block-force": func() vhostData { d := force(php(base)); d.WAF = WAFBlock; return d }(),
		"waf-detect-app":  func() vhostData { d := app(base); d.WAF = WAFDetect; return d }(),
		"ratelimit-php": func() vhostData {
			d := php(base)
			d.RateLimit, d.RateBurst, d.RateZone = 10, 50, "panel_rl_example_com"
			return d
		}(),
		"ratelimit-app": func() vhostData {
			d := app(base)
			d.RateLimit, d.RateBurst, d.RateZone = 5, 25, "panel_rl_example_com"
			return d
		}(),
		"ratelimit-static": func() vhostData {
			d := base
			d.RateLimit, d.RateBurst, d.RateZone = 5, 25, "panel_rl_example_com"
			return d
		}(),
		"blocked-static": func() vhostData {
			d := base
			d.BlockedIPs = []string{"203.0.113.7", "198.51.100.0/24", "2001:db8::/32"}
			return d
		}(),
		"blocked-force":    func() vhostData { d := force(php(base)); d.BlockedIPs = []string{"203.0.113.7"}; return d }(),
		"custom404-static": func() vhostData { d := base; d.Custom404 = true; return d }(),
		"custom404-php":    func() vhostData { d := php(base); d.Custom404 = true; return d }(),
		"hotlink-static":   func() vhostData { d := base; d.Hotlink = true; return d }(),
		"hotlink-php-allow": func() vhostData {
			d := php(base)
			d.Hotlink, d.HotlinkAllow = true, []string{"partner.example.org", "*.cdn.example.net"}
			return d
		}(),
		"custom-static":            func() vhostData { d := base; d.CustomFile = "/etc/nginx/panel/custom/example.com.conf"; return d }(),
		"custom-php":               func() vhostData { d := php(base); d.CustomFile = "/etc/nginx/panel/custom/example.com.conf"; return d }(),
		"custom-app":               func() vhostData { d := app(base); d.CustomFile = "/etc/nginx/panel/custom/example.com.conf"; return d }(),
		"githook-noid":             baseVhost(),
		"everything-php":           everythingVhost(php(force(base))),
		"everything-php-old":       everythingVhost(oldNginx(php(ssl(base)))),
		"everything-app":           everythingVhost(app(force(base))),
		"everything-static-new":    everythingVhost(newNginxV6(ssl(base))),
		"preset-default-static":    func() vhostData { d := base; d.RewriteRules = rewriteRules("", false); return d }(),
		"preset-default-php":       func() vhostData { d := php(base); d.RewriteRules = rewriteRules("", true); return d }(),
		"preset-php-only-fallback": func() vhostData { d := base; d.RewriteRules = rewriteRules("wordpress", false); return d }(),
	}
	for _, p := range RewritePresets {
		d := base
		if p.PHP {
			d = php(d)
		}
		d.RewriteRules = rewriteRules(p.ID, p.PHP)
		cases["preset-"+p.ID] = d
		h := d
		h.Hotlink, h.Custom404, h.CustomFile = true, true, "/etc/nginx/panel/custom/example.com.conf"
		cases["preset-"+p.ID+"-options"] = h
	}
	return cases
}

func everythingVhost(d vhostData) vhostData {
	d.AuthFile = "/etc/nginx/panel/auth/example.com.htpasswd"
	d.WAF = WAFDetect
	d.RateLimit, d.RateBurst, d.RateZone = 20, 100, "panel_rl_example_com"
	d.BlockedIPs = []string{"203.0.113.7", "2001:db8::/32"}
	d.Custom404, d.Hotlink, d.HotlinkAllow = true, true, []string{"partner.example.org"}
	d.CustomFile = "/etc/nginx/panel/custom/example.com.conf"
	d.UploadMB, d.TimeoutSec = 128, 90
	if d.PHPSocket != "" {
		d.RewriteRules = rewriteRules("wordpress", true)
	}
	return d
}

func TestNginxGoldenVhosts(t *testing.T) {
	cases := nginxVhostCases()
	names := make([]string, 0, len(cases))
	for n := range cases {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		out, err := renderVhost(cases[n])
		if err != nil {
			t.Fatalf("%s: %v", n, err)
		}
		checkGolden(t, "nginx", "vhost-"+n+".conf", out)
	}
}

func TestNginxGoldenShared(t *testing.T) {
	render1 := func(name string, f func() ([]byte, error)) {
		t.Helper()
		out, err := f()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		checkGolden(t, "nginx", name, out)
	}
	for _, c := range []struct {
		name    string
		version string
		ipv6    bool
		waf     bool
	}{
		{"hook-1.24-ipv6.conf", "1.24.0", true, false},
		{"hook-1.24-ipv6-waf.conf", "1.24.0", true, true},
		{"hook-1.18.conf", "1.18.0", true, false},
		{"hook-1.26-noipv6.conf", "1.26.3", false, false},
		{"hook-1.26-ipv6-waf.conf", "1.26.3", true, true},
		{"hook-unknown.conf", "", false, false},
	} {
		render1(c.name, func() ([]byte, error) {
			return render(hookTmpl, hookData{nginxFeatures: featuresFor(c.version, c.ipv6), SitesDir: "/etc/nginx/panel/sites", SnippetDir: "/etc/nginx/panel", WAF: c.waf})
		})
	}
	// Ubuntu's nginx.conf turns gzip on (for HTML only): the rest is added.
	render1("hook-1.24-gzip.conf", func() ([]byte, error) {
		return render(hookTmpl, hookData{nginxFeatures: featuresFor("1.24.0", true), SitesDir: "/etc/nginx/panel/sites",
			SnippetDir: "/etc/nginx/panel", Gzip: gzipDirectives(ubuntuNginxConf)})
	})
	render1("acme.conf", func() ([]byte, error) {
		return render(acmeSnippetTmpl, struct{ Webroot string }{"/var/lib/panel-acme"})
	})
	render1("ssl.conf", func() ([]byte, error) { return render(sslSnippetTmpl, nil) })
	pool := poolData{Domain: "example.com", User: "example_com", Root: "/var/www/example.com", Socket: "/run/php/panel-example_com.sock"}
	render1("pool-default.conf", func() ([]byte, error) { return renderPool(pool) })
	render1("pool-limits.conf", func() ([]byte, error) {
		p := pool
		p.MemoryMB, p.UploadMB, p.TimeSec = 1024, 512, 600
		return renderPool(p)
	})
	render1("pool-dynamic.conf", func() ([]byte, error) {
		p := pool
		p.PM, p.Workers, p.Allow = "dynamic", 12, []string{"proc_open", "exec"}
		return renderPool(p)
	})
	render1("pool-all-allowed.conf", func() ([]byte, error) {
		p := pool
		p.PM, p.Workers, p.Allow = "ondemand", 1, HardenedFunctions
		return renderPool(p)
	})
	render1("welcome.html", func() ([]byte, error) { return renderWelcome("example.com") })
	render1("custom.conf", func() ([]byte, error) {
		return customBytes(&store.Site{NginxCustom: "location /api/ {\r\n    return 204;\n}\n\n\n"}), nil
	})
}

// goldenRunner answers the commands the renderers ask about.
func goldenRunner(nginxVersion string) *fakeRunner {
	return &fakeRunner{outputs: map[string]fakeResult{
		"nginx":     {out: "nginx version: nginx/" + nginxVersion + " (Ubuntu)\n"},
		"apt-cache": {out: "php-fpm\n  Depends: php8.3-fpm\n"},
		"systemctl": {out: "active\n"},
	}}
}

// goldenManager uses the real Debian paths (they only appear in the output)
// with a PHP folder that claims 8.3 is installed.
func goldenManager(t *testing.T, nginxVersion string) *Manager {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	p := DefaultPaths("/var/lib/nefo")
	p.PHP = t.TempDir()
	for _, v := range []string{"8.3", "8.4"} {
		dir := filepath.Join(p.PHP, v, "fpm")
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "php-fpm.conf"), nil, 0o644)
	}
	m := New(p, goldenRunner(nginxVersion), st, slog.New(slog.NewTextHandler(io.Discard, nil)), "")
	m.now = func() time.Time { return time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC) }
	// Explicitly: the files of this machine (an Apache hook) must not decide.
	if err := m.setServerKind(context.Background(), ServerNginx); err != nil {
		t.Fatal(err)
	}
	return m
}

// goldenSites are stored sites in every shape vhostBytes handles.
func goldenSites() []*store.Site {
	return []*store.Site{
		{ID: 1, Domain: "static.example.com", SysUser: "static_example_com"},
		{ID: 2, Domain: "php.example.com", Aliases: []string{"www.php.example.com"}, SysUser: "php_example_com", PHPVersion: "8.3",
			PHPMemory: 256, PHPUpload: 64, PHPTime: 300},
		{ID: 3, Domain: "ssl.example.com", Aliases: []string{"*.ssl.example.com"}, SysUser: "ssl_example_com", PHPVersion: "8.4",
			SSL: true, ForceHTTPS: true, PHPMemory: 512, PHPUpload: 128, PHPTime: 120, PHPWorkers: 8, PHPPM: "dynamic", PHPAllow: "exec proc_open"},
		{ID: 4, Domain: "app.example.com", SysUser: "app_example_com", ProxyPort: 3000, AppCommand: "node server.js", SSL: true, PHPUpload: 64, PHPTime: 300},
		{ID: 5, Domain: "wp.example.com", SysUser: "wp_example_com", PHPVersion: "8.3", App: AppWordPress, Rewrite: "wordpress",
			PHPMemory: 256, PHPUpload: 64, PHPTime: 300, AuthUser: "guest", AuthHash: "$2a$10$abcdefghijklmnopqrstuuJ0.dV9W3cYyk1C1qQeYv0zZ7pZ0rH6",
			BlockedIPs: "203.0.113.7 198.51.100.0/24", RateLimit: 10, Custom404: true, Hotlink: true, HotlinkAllow: "partner.example.org",
			NginxCustom: "location /x/ {\n    return 204;\n}", WAF: WAFBlock},
		{ID: 6, Domain: "off.example.com", SysUser: "off_example_com", PHPVersion: "8.3", Disabled: true, PHPUpload: 64, PHPTime: 300},
		{ID: 7, Domain: "moved.example.com", SysUser: "moved_example_com", RedirectURL: "https://example.org/new", SSL: true},
		{ID: 8, Domain: "drupal.example.com", SysUser: "drupal_example_com", PHPVersion: "8.3", Rewrite: "drupal", PHPUpload: 64, PHPTime: 300},
		{ID: 9, Domain: "spa.example.com", SysUser: "spa_example_com", Rewrite: "spa"},
		{ID: 10, Domain: "nophp.example.com", SysUser: "nophp_example_com", Rewrite: "laravel"},
	}
}

func TestNginxGoldenManager(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("real paths: Linux only")
	}
	ctx := context.Background()
	for _, version := range []string{"1.24.0", "1.26.3"} {
		m := goldenManager(t, version)
		suffix := fmt.Sprintf("-%s-ipv6=%v", version, ipv6Enabled())
		for _, s := range goldenSites() {
			out, err := m.vhostBytes(ctx, s)
			if err != nil {
				t.Fatalf("%s: %v", s.Domain, err)
			}
			checkGolden(t, "nginx-linux", "vhost-"+s.Domain+suffix+".conf", out)
			if s.PHPVersion != "" {
				pool, err := m.poolBytes(s)
				if err != nil {
					t.Fatalf("pool %s: %v", s.Domain, err)
				}
				checkGolden(t, "nginx-linux", "pool-"+s.Domain+".conf", pool)
			}
		}
		base, err := m.baseFiles(ctx)
		if err != nil {
			t.Fatal(err)
		}
		paths := make([]string, 0, len(base))
		for p := range base {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		for _, p := range paths {
			checkGolden(t, "nginx-linux", "base"+suffix+"-"+filepath.Base(p), base[p])
		}
		checkGolden(t, "nginx-linux", "base"+suffix+"-paths.txt", []byte(fmt.Sprint(paths)))

		m.store.SetSetting(ctx, adminerPortKey, "46000")
		m.store.SetSetting(ctx, adminerTokenKey, "0123456789abcdef0123456789abcdef0123456789abcdef")
		conf, err := m.adminerConfBytes(ctx)
		if err != nil {
			t.Fatal(err)
		}
		checkGolden(t, "nginx-linux", "adminer.conf", conf)
		checkGolden(t, "nginx-linux", "adminer-path.txt", []byte(m.adminerConf()))

		waf := m.wafFiles(goldenSites())
		for p, data := range waf {
			checkGolden(t, "nginx-linux", "waf-"+filepath.Base(p), data)
		}
		files := []string{m.vhostFile("a.example.com"), m.customFile("a.example.com"), m.authFile("a.example.com"),
			m.web().hookFile(), strings.TrimPrefix(m.paths.poolFile("8.3", "u"), m.paths.PHP), m.paths.phpSocket("u"), m.wafDir()}
		checkGolden(t, "nginx-linux", "paths.txt", []byte(fmt.Sprint(files)))
	}
}
