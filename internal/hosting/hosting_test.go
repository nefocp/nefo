package hosting

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

func baseVhost() vhostData {
	return vhostData{
		nginxFeatures: featuresFor("1.24.0", true),
		Domain:        "example.com",
		Aliases:       []string{"www.example.com"},
		Public:        "/var/www/example.com/public",
		LogDir:        "/var/log/nginx",
		SnippetDir:    "/etc/nginx/panel",
	}
}

func mustContain(t *testing.T, text string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if !strings.Contains(text, p) {
			t.Errorf("missing %q in:\n%s", p, text)
		}
	}
}

func mustNotContain(t *testing.T, text string, parts ...string) {
	t.Helper()
	for _, p := range parts {
		if strings.Contains(text, p) {
			t.Errorf("unexpected %q in:\n%s", p, text)
		}
	}
}

func TestVhostStatic(t *testing.T) {
	out, err := renderVhost(baseVhost())
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	mustContain(t, s,
		"server_name example.com www.example.com;",
		"listen [::]:80;",
		"root /var/www/example.com/public;",
		"index index.html index.htm;",
		"include /etc/nginx/panel/acme.conf;",
		"try_files $uri $uri/ =404;",
		"location ~ /\\.(?!well-known/)",
	)
	mustNotContain(t, s, "fastcgi_pass", "listen 443", "return 301")
	if strings.Count(s, "{") != strings.Count(s, "}") {
		t.Errorf("unbalanced braces:\n%s", s)
	}
}

func TestVhostPHPWithSSL(t *testing.T) {
	d := baseVhost()
	d.PHPSocket = "/run/php/panel-example_com.sock"
	d.SSL = true
	d.CertFile = "/var/lib/nefo/certs/example.com/fullchain.pem"
	d.KeyFile = "/var/lib/nefo/certs/example.com/privkey.pem"
	out, err := renderVhost(d)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	mustContain(t, s,
		"fastcgi_pass unix:/run/php/panel-example_com.sock;",
		"index index.php index.html index.htm;",
		"try_files $uri $uri/ /index.php?$args;",
		"listen 443 ssl http2;", // Nginx 1.24 has no "http2 on"
		"ssl_certificate     /var/lib/nefo/certs/example.com/fullchain.pem;",
		"include /etc/nginx/panel/ssl.conf;",
	)
	mustNotContain(t, s, "return 301", "http2 on;")
	// Without forced HTTPS the site is served on both ports.
	if n := strings.Count(s, "root /var/www/example.com/public;"); n != 2 {
		t.Errorf("site body appears %d times, want 2", n)
	}
	if strings.Count(s, "{") != strings.Count(s, "}") {
		t.Errorf("unbalanced braces:\n%s", s)
	}
}

func TestVhostGitHook(t *testing.T) {
	d := baseVhost()
	d.ID = 7
	d.SSL, d.ForceHTTPS, d.WAF, d.AuthFile = true, true, WAFBlock, "/etc/nginx/panel/auth/x"
	d.CertFile, d.KeyFile = "/c/fullchain.pem", "/c/privkey.pem"
	out, err := renderVhost(d)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	// Once on port 80 (a webhook saved with the http:// URL; GitHub does not
	// follow redirects) and once on 443.
	if n := strings.Count(s, "location = /.panel/deploy/7 {"); n != 2 {
		t.Errorf("webhook location appears %d times, want 2:\n%s", n, s)
	}
	mustContain(t, s, "proxy_pass http://127.0.0.1:7809/git/7;", "auth_basic off;", "modsecurity off;", "client_max_body_size 1m;")
	if strings.Index(s, "location = /.panel/deploy/7") > strings.Index(s, "return 301 https") {
		t.Error("the webhook would be redirected on port 80")
	}
	d.WAF, d.SSL, d.ForceHTTPS = WAFOff, false, false
	out, _ = renderVhost(d)
	mustNotContain(t, string(out), "modsecurity off;")
}

func TestVhostForceHTTPSAndNewNginx(t *testing.T) {
	d := baseVhost()
	d.nginxFeatures = featuresFor("1.26.3", false)
	d.SSL, d.ForceHTTPS = true, true
	d.CertFile, d.KeyFile = "/c/fullchain.pem", "/c/privkey.pem"
	out, err := renderVhost(d)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	mustContain(t, s, "return 301 https://$host$request_uri;", "listen 443 ssl;", "http2 on;")
	mustNotContain(t, s, "listen [::]", "ssl http2")
	if n := strings.Count(s, "root /var/www/example.com/public;"); n != 1 {
		t.Errorf("site body appears %d times, want 1 (HTTPS only)", n)
	}
	// The ACME location must stay reachable over HTTP, before the redirect.
	if strings.Index(s, "acme.conf") > strings.Index(s, "return 301") {
		t.Error("ACME challenges would be redirected")
	}
}

func TestRenderRefusesInjection(t *testing.T) {
	for _, bad := range []string{"example.com;", "a.com\n}", "exa mple.com", "$host.com"} {
		d := baseVhost()
		d.Domain = bad
		if _, err := renderVhost(d); err == nil {
			t.Errorf("domain %q rendered", bad)
		}
	}
	d := baseVhost()
	d.Aliases = []string{"ok.com", "evil.com; include /etc/passwd"}
	if _, err := renderVhost(d); err == nil {
		t.Error("bad alias rendered")
	}
	d = baseVhost()
	d.PHPSocket = "/run/php/x.sock; deny all"
	if _, err := renderVhost(d); err == nil {
		t.Error("bad socket path rendered")
	}
	d = baseVhost()
	d.Public = "/var/www/../../etc"
	if _, err := renderVhost(d); err == nil {
		t.Error("path with .. rendered")
	}
	if _, err := renderPool(poolData{Domain: "example.com", User: "root\nuser = x", Root: "/var/www/example.com", Socket: "/run/php/a.sock"}); err == nil {
		t.Error("bad pool user rendered")
	}
}

func TestPool(t *testing.T) {
	out, err := renderPool(poolData{Domain: "example.com", User: "example_com", Root: "/var/www/example.com", Socket: "/run/php/panel-example_com.sock"})
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, string(out),
		"[example_com]",
		"user = example_com",
		"listen = /run/php/panel-example_com.sock",
		"pm = ondemand",
		"php_admin_value[open_basedir] = /var/www/example.com/",
		"php_admin_value[session.save_path] = /var/www/example.com/tmp",
	)
}

func TestHook(t *testing.T) {
	out, err := render(hookTmpl, hookData{nginxFeatures: featuresFor("1.24.0", true), SitesDir: "/etc/nginx/panel/sites", SnippetDir: "/etc/nginx/panel"})
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, string(out), "return 444;", "ssl_reject_handshake on;", "listen 443 ssl http2 default_server;",
		"include /etc/nginx/panel/sites/*.conf;", "include /etc/nginx/panel/acme.conf;")
	// Nginx 1.18 (Ubuntu 22.04) has no ssl_reject_handshake.
	out, _ = render(hookTmpl, hookData{nginxFeatures: featuresFor("1.18.0", true), SitesDir: "/s"})
	mustNotContain(t, string(out), "ssl_reject_handshake", "listen 443")
}

func TestWelcomeEscapes(t *testing.T) {
	out, err := renderWelcome("<script>.com")
	if err != nil {
		t.Fatal(err)
	}
	mustNotContain(t, string(out), "<script>")
}

func TestVersionAtLeast(t *testing.T) {
	cases := []struct {
		have, want string
		ok         bool
	}{
		{"1.24.0", "1.25.1", false},
		{"1.25.1", "1.25.1", true},
		{"1.26.0", "1.25.1", true},
		{"1.18.0", "1.19.4", false},
		{"2.0", "1.25.1", true},
		{"", "1.0", false},
	}
	for _, c := range cases {
		if got := versionAtLeast(c.have, c.want); got != c.ok {
			t.Errorf("versionAtLeast(%q, %q) = %v", c.have, c.want, got)
		}
	}
}

func TestPHPPackagesOPcache(t *testing.T) {
	// OPcache is part of PHP itself from 8.5: Ubuntu 26.04 and
	// packages.sury.org have no php8.5-opcache, and one missing package
	// fails the whole apt-get install.
	for v, want := range map[string]bool{"8.1": true, "8.4": true, "8.5": false, "8.10": false} {
		pkgs := phpPackages(v)
		if got := slices.Contains(pkgs, "php"+v+"-opcache"); got != want {
			t.Errorf("PHP %s: opcache package %v, want %v (%v)", v, got, want, pkgs)
		}
		if !slices.Contains(pkgs, "php"+v+"-fpm") || !slices.Contains(pkgs, "php"+v+"-mysql") {
			t.Errorf("PHP %s: %v", v, pkgs)
		}
	}
}

func TestUserNames(t *testing.T) {
	cases := map[string]string{
		"example.com":                            "example_com",
		"shop.my-site.vn":                        "shop_my-site_vn",
		"1panel.com":                             "w1panel_com",
		"xn--bcher-kva.de":                       "xn--bcher-kva_de",
		"a-very-long-subdomain-name.example.com": "a-very-long-subdomain-name_e",
	}
	for domain, want := range cases {
		if got := userBase(domain); got != want {
			t.Errorf("userBase(%q) = %q, want %q", domain, got, want)
		}
	}
	taken := map[string]bool{"example_com": true, "example_com_2": true}
	got, err := pickName("example_com", func(n string) bool { return taken[n] })
	if err != nil || got != "example_com_3" {
		t.Fatalf("pickName = %q, %v", got, err)
	}
	if _, err := pickName("x", func(string) bool { return true }); err == nil {
		t.Fatal("pickName found a name although all are taken")
	}
}

func TestRenewalDue(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	site := func(start time.Time, life time.Duration) *store.Site {
		return &store.Site{CertNotBefore: start, CertNotAfter: start.Add(life)}
	}
	if renewalDue(site(now.Add(-50*day), 90*day), now) {
		t.Error("90-day cert with 40 days left is due")
	}
	if !renewalDue(site(now.Add(-61*day), 90*day), now) {
		t.Error("90-day cert with 29 days left is not due")
	}
	if !renewalDue(site(now.Add(-31*day), 45*day), now) {
		t.Error("45-day cert with 14 days left is not due")
	}
	if renewalDue(site(now.Add(-20*day), 45*day), now) {
		t.Error("45-day cert with 25 days left is due")
	}
}

func TestCustomReminderKeepsRealErrors(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return now }
	for domain, ssle := range map[string]string{
		"old.example": legacyCustomReminder + "2026-01-10: install a new one, or switch to Let's Encrypt",
		"le.example":  "start the order: rate limited",
	} {
		site := &store.Site{Domain: domain, SysUser: strings.ReplaceAll(domain, ".", "_"), SSL: true, SSLSource: SSLCustom, SSLError: ssle}
		if err := m.store.CreateSite(ctx, site); err != nil {
			t.Fatal(err)
		}
		if err := m.store.UpdateSite(ctx, site); err != nil {
			t.Fatal(err)
		}
		m.recordCustomReminder(ctx, site.ID)
		site, err := m.store.SiteByID(ctx, site.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !site.SSLAttemptAt.Equal(now) {
			t.Errorf("%s: reminder time = %v", domain, site.SSLAttemptAt)
		}
		if want := map[string]string{"old.example": "", "le.example": ssle}[domain]; site.SSLError != want {
			t.Errorf("%s: certificate error = %q, want %q", domain, site.SSLError, want)
		}
	}
}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	root := t.TempDir()
	p := DefaultPaths(filepath.Join(root, "data"))
	p.PHP = filepath.Join(root, "php")
	p.GeoIP = filepath.Join(root, "geoip")
	return New(p, nil, st, slog.New(slog.NewTextHandler(io.Discard, nil)), "")
}

func TestCheckNames(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()
	domain, aliases, err := m.checkNamesT(ctx, " Example.COM. ", []string{"www.example.com", "", "WWW.example.com", "example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if domain != "example.com" || len(aliases) != 1 || aliases[0] != "www.example.com" {
		t.Fatalf("got %q %q", domain, aliases)
	}
	m.store.CreateSite(ctx, &store.Site{Domain: "example.com", Aliases: []string{"www.example.com"}, SysUser: "example_com"})
	var ie *usererr.Error
	if _, _, err := m.checkNamesT(ctx, "www.example.com", nil); !errors.As(err, &ie) || ie.Code != "domain_exists" {
		t.Fatalf("alias of another site accepted: %v", err)
	}
	if _, _, err := m.checkNamesT(ctx, "bad_domain.com", nil); !errors.As(err, &ie) || ie.Code != "invalid_domain" {
		t.Fatalf("err = %v", err)
	}
	if _, _, err := m.checkNamesT(ctx, "ok.com", []string{"x;y.com"}); !errors.As(err, &ie) || ie.Code != "invalid_alias" {
		t.Fatalf("err = %v", err)
	}
}

func TestPHPVersions(t *testing.T) {
	m := newTestManager(t)
	for _, v := range []string{"8.3", "7.4", "8.10", "junk"} {
		dir := filepath.Join(m.paths.PHP, v, "fpm")
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "php-fpm.conf"), nil, 0o644)
	}
	got := strings.Join(m.phpVersions(), ",")
	if got != "7.4,8.3,8.10" {
		t.Fatalf("phpVersions() = %s", got)
	}
	if !m.phpInstalled("8.3") || m.phpInstalled("8.2") {
		t.Fatal("phpInstalled is wrong")
	}
}

func (m *Manager) checkNamesT(ctx context.Context, d string, a []string) (string, []string, error) {
	return m.checkNames(ctx, d, a, 0)
}

func TestTail(t *testing.T) {
	f, _ := os.CreateTemp(t.TempDir(), "log")
	defer f.Close()
	for i := 1; i <= 500; i++ {
		fmt.Fprintf(f, "line %d\n", i)
	}
	got, err := tail(f, 3)
	if err != nil || got != "line 498\nline 499\nline 500" {
		t.Fatalf("tail = %q, %v", got, err)
	}
	empty, _ := os.CreateTemp(t.TempDir(), "empty")
	defer empty.Close()
	if got, _ := tail(empty, 10); got != "" {
		t.Fatalf("empty tail = %q", got)
	}
}

func TestTemplateLimits(t *testing.T) {
	d := baseVhost()
	d.PHPSocket, d.UploadMB, d.TimeoutSec = "/run/php/x.sock", 512, 630
	out, _ := renderVhost(d)
	mustContain(t, string(out), "client_max_body_size 512m;", "fastcgi_read_timeout 630s;")
	pool, _ := renderPool(poolData{Domain: "example.com", User: "u", Root: "/var/www/example.com", Socket: "/run/php/x.sock",
		MemoryMB: 1024, UploadMB: 512, TimeSec: 600})
	mustContain(t, string(pool), "php_value[memory_limit] = 1024M", "php_value[upload_max_filesize] = 512M",
		"php_value[post_max_size] = 512M", "php_value[max_execution_time] = 600")
}

func TestVhostProxy(t *testing.T) {
	d := baseVhost()
	d.ProxyPort = 3000
	out, err := renderVhost(d)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	mustContain(t, s, "proxy_pass http://127.0.0.1:3000;", "proxy_set_header Upgrade $http_upgrade;",
		"proxy_set_header Connection $panel_connection_upgrade;", "include /etc/nginx/panel/acme.conf;")
	mustNotContain(t, s, "root /var/www", "fastcgi_pass")
	if strings.Count(s, "{") != strings.Count(s, "}") {
		t.Errorf("unbalanced braces:\n%s", s)
	}
	hook, _ := render(hookTmpl, hookData{nginxFeatures: featuresFor("1.24.0", true), SitesDir: "/s", SnippetDir: "/p"})
	mustContain(t, string(hook), "map $http_upgrade $panel_connection_upgrade")
}

func TestVhostAccess(t *testing.T) {
	d := baseVhost()
	d.Disabled = true
	out, _ := renderVhost(d)
	mustContain(t, string(out), "return 503", "include /etc/nginx/panel/acme.conf;")
	mustNotContain(t, string(out), "root /var/www")

	d = baseVhost()
	d.RedirectURL = "https://new.example.com"
	out, _ = renderVhost(d)
	mustContain(t, string(out), "return 301 https://new.example.com$request_uri;")

	d = baseVhost()
	d.AuthFile = "/etc/nginx/panel/auth/example.com.htpasswd"
	out, _ = renderVhost(d)
	mustContain(t, string(out), `auth_basic "Restricted";`, "auth_basic_user_file /etc/nginx/panel/auth/example.com.htpasswd;", "root /var/www")

	for _, bad := range []string{"https://x.com/a b", "https://x.com/;return 200", "javascript:alert(1)", "https://x.com/$host", `https://x.com/"`} {
		d = baseVhost()
		d.RedirectURL = bad
		if _, err := renderVhost(d); err == nil {
			t.Errorf("redirect %q rendered", bad)
		}
	}
	acme, _ := render(acmeSnippetTmpl, struct{ Webroot string }{"/w"})
	mustContain(t, string(acme), "auth_basic off;")
}

func TestPoolWorkersAndFunctions(t *testing.T) {
	base := poolData{Domain: "example.com", User: "u", Root: "/var/www/example.com", Socket: "/run/php/x.sock"}
	out, err := renderPool(base)
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, string(out), "pm = ondemand", "pm.max_children = 5",
		"php_admin_value[disable_functions] = exec,passthru,shell_exec,system,proc_open,popen,pcntl_exec")
	d := base
	d.PM, d.Workers, d.Allow = "dynamic", 12, []string{"proc_open", "exec"}
	out, err = renderPool(d)
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, string(out), "pm = dynamic", "pm.max_children = 12", "pm.start_servers = 2", "pm.max_spare_servers = 6",
		"php_admin_value[disable_functions] = passthru,shell_exec,system,popen,pcntl_exec")
	all := base
	all.Allow = HardenedFunctions
	out, _ = renderPool(all)
	mustNotContain(t, string(out), "disable_functions")
	for _, bad := range []poolData{{PM: "static"}, {Workers: 500}, {Allow: []string{"exec\nphp_admin_value[open_basedir]=/"}}, {Allow: []string{"eval"}}} {
		b := base
		if bad.PM != "" {
			b.PM = bad.PM
		}
		if bad.Workers != 0 {
			b.Workers = bad.Workers
		}
		b.Allow = bad.Allow
		if _, err := renderPool(b); err == nil {
			t.Errorf("rendered %+v", bad)
		}
	}
}

func TestSiteOptionsVhost(t *testing.T) {
	d := baseVhost()
	d.PHPSocket, d.Custom404, d.Hotlink, d.HotlinkAllow = "/run/php/x.sock", true, true, []string{"partner.example.org", "*.cdn.example.net"}
	d.RewriteRules = rewriteRules("drupal", true)
	out, err := renderVhost(d)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	mustContain(t, s, "error_page 404 /404.html;",
		`valid_referers none blocked server_names partner.example.org *.cdn.example.net ~\.google\.`,
		`location ~* \.(?:jpe?g|png`,
		"if ($invalid_referer) {", "try_files $uri /index.php?$args;")
	// After the preset (Drupal builds image styles in PHP), before PHP.
	hot := strings.Index(s, "valid_referers")
	if styles := strings.Index(s, "files/styles/"); styles > hot {
		t.Fatal("hotlink location comes before the preset's rules")
	}
	if php := strings.Index(s, `location ~ [^/]\.php(/|$)`); php < hot {
		t.Fatal("hotlink location comes after the PHP location")
	}
	d.HotlinkAllow = []string{"evil.com; return 200"}
	if _, err := renderVhost(d); err == nil {
		t.Fatal("rendered an unsafe hotlink name")
	}
}

func TestWAFPackaged(t *testing.T) {
	dir := t.TempDir()
	old := aptLists
	defer func() { aptLists = old }()
	aptLists = filepath.Join(dir, "*_Packages*")
	ctx := context.Background()
	f := &fakeRunner{outputs: map[string]fakeResult{"apt-cache": {out: ""}}}
	m := &Manager{run: f}
	// Without apt's lists, apt can't tell: the install reports the error.
	if !m.WAFPackaged(ctx, ServerNginx) {
		t.Fatal("no lists: should say yes")
	}
	if err := os.WriteFile(filepath.Join(dir, "archive.ubuntu.com_dists_jammy_main_binary-amd64_Packages"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	// A package apt doesn't know (Ubuntu 22.04's Nginx module).
	if m.WAFPackaged(ctx, ServerNginx) {
		t.Fatal("unknown package: should say no")
	}
	// The answer is kept; another web server is asked on its own.
	f.outputs["apt-cache"] = fakeResult{out: "libapache2-mod-security2:\n  Installed: (none)\n  Candidate: 2.9.5-1\n"}
	if m.WAFPackaged(ctx, ServerNginx) {
		t.Fatal("the answer for Nginx was not kept")
	}
	if !m.WAFPackaged(ctx, ServerApache) {
		t.Fatal("Apache's packages are there")
	}
	f.outputs["apt-cache"] = fakeResult{out: "ols-modsecurity:\n  Installed: (none)\n  Candidate: (none)\n"}
	if m.WAFPackaged(ctx, ServerOLS) {
		t.Fatal("Candidate (none): should say no")
	}
	if got := m.ServerTitle(ServerNginx); got != "Nginx" {
		t.Fatalf("title %q", got)
	}
}
