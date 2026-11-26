package hosting

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// apacheCases are nginxVhostCases with Apache's paths.
func apacheCases() map[string]vhostData {
	cases := nginxVhostCases()
	for n, d := range cases {
		if d.CacheZone != "" { // Nginx only
			delete(cases, n)
			continue
		}
		d.LogDir, d.SnippetDir, d.EmptyRoot = "/var/log/apache2", "/etc/apache2/panel", "/var/lib/panel-acme"
		if d.AuthFile != "" {
			d.AuthFile = "/etc/apache2/panel/auth/example.com.htpasswd"
		}
		if d.CustomFile != "" {
			d.CustomFile = "/etc/apache2/panel/custom/example.com.conf"
		}
		cases[n] = d
	}
	return cases
}

// presetOf returns the preset ID a case was built with (its Nginx rules).
func presetOf(d vhostData) string {
	for _, p := range RewritePresets {
		if p.Rules == d.RewriteRules {
			return p.ID
		}
	}
	return ""
}

func renderApacheCase(t *testing.T, d vhostData) string {
	t.Helper()
	out, err := renderApacheVhost(d, presetOf(d))
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestApacheGoldenVhosts(t *testing.T) {
	cases := apacheCases()
	names := make([]string, 0, len(cases))
	for n := range cases {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		d := cases[n]
		out, err := renderApacheVhost(d, presetOf(d))
		if err != nil {
			t.Fatalf("%s: %v", n, err)
		}
		s := string(out)
		if strings.Count(s, "<VirtualHost") != strings.Count(s, "</VirtualHost>") ||
			strings.Count(s, "<Location") != strings.Count(s, "</Location") ||
			strings.Count(s, "<Directory") != strings.Count(s, "</Directory>") {
			t.Errorf("%s: unbalanced sections:\n%s", n, s)
		}
		if strings.Contains(s, "limit_req") || strings.Contains(s, "location ") {
			t.Errorf("%s: Nginx syntax in the Apache file:\n%s", n, s)
		}
		checkGolden(t, "apache", "vhost-"+n+".conf", out)
	}
}

func TestApacheVhostPHP(t *testing.T) {
	d := apacheCases()["php"]
	s := renderApacheCase(t, d)
	mustContain(t, s,
		"<VirtualHost *:80>",
		"ServerName example.com",
		"ServerAlias www.example.com",
		"DocumentRoot /var/www/example.com/public",
		"Options SymLinksIfOwnerMatch",
		"AllowOverride AuthConfig FileInfo Indexes Limit Nonfatal=All",
		"DirectoryIndex index.php index.html index.htm",
		`<FilesMatch "\.php$">`,
		`SetHandler "proxy:unix:/run/php/panel-example_com.sock|fcgi://localhost"`,
		`ProxyFCGISetEnvIf "note('uds_path') != '/run/php/panel-example_com.sock'" SCRIPT_FILENAME /nonexistent`,
		`ProxyFCGISetEnvIf "true" !PHP_ADMIN_VALUE`,
		"FallbackResource /index.php",
		"CustomLog /var/log/apache2/example.com.access.log panel_combined",
		"Include /etc/apache2/panel/acme.conf",
		"ProxyPass /.panel/deploy/42 http://127.0.0.1:7809/git/42 timeout=30",
		`RewriteCond expr "%{HTTP:Content-Length} -gt 67108864"`,
		`<LocationMatch "/\.(?!well-known/)">`,
	)
	mustNotContain(t, s, "<VirtualHost *:443>", "SSLEngine", "AuthType", "SecRuleEngine", "FollowSymLinks")
	// The panel's access rules come after .htaccess: in <Location>, never
	// only in <Directory>.
	if strings.Index(s, "<Location />") < 0 {
		t.Fatal("no <Location /> block")
	}
	// The webhook is opened after the dot-file rule that would hide it.
	if strings.Index(s, "<Location /.panel/deploy/42>") < strings.Index(s, "<LocationMatch") {
		t.Fatal("the dot-file rule comes after the webhook location")
	}
	// The webhook's exemption comes first among the rewrite rules.
	if strings.Index(s, `RewriteRule ^/\.panel/deploy/42$ - [L]`) > strings.Index(s, `RewriteRule ^ - [R=413,L]`) {
		t.Fatal("the body limit would apply to the webhook")
	}
}

func TestApacheVhostStaticAndApp(t *testing.T) {
	s := renderApacheCase(t, apacheCases()["static"])
	mustContain(t, s, "DirectoryIndex index.html index.htm", `ProxyFCGISetEnvIf "true" SCRIPT_FILENAME /nonexistent`)
	mustNotContain(t, s, "FilesMatch", "FallbackResource")

	s = renderApacheCase(t, apacheCases()["app"])
	mustContain(t, s,
		"DocumentRoot /var/lib/panel-acme",
		"ProxyPreserveHost On",
		"ProxyPass /.well-known/acme-challenge/ !",
		"ProxyPass / http://127.0.0.1:3000/ upgrade=websocket timeout=300",
		"ProxyPassReverse / http://127.0.0.1:3000/",
		"RequestHeader set X-Forwarded-Proto expr=%{REQUEST_SCHEME}",
		"RequestHeader set X-Real-IP expr=%{REMOTE_ADDR}")
	mustNotContain(t, s, "<Directory /var/www", "DocumentRoot /var/www")
	// The webhook is proxied before the app takes everything.
	if strings.Index(s, "ProxyPass /.panel/deploy/42") > strings.Index(s, "ProxyPass / http") {
		t.Fatal("ProxyPass / comes before the webhook")
	}
}

func TestApacheVhostSSL(t *testing.T) {
	s := renderApacheCase(t, apacheCases()["php-force"])
	mustContain(t, s, "<VirtualHost *:443>", "SSLEngine on",
		"SSLCertificateFile /var/lib/nefo/certs/example.com/fullchain.pem",
		"SSLCertificateKeyFile /var/lib/nefo/certs/example.com/privkey.pem",
		`RewriteCond %{THE_REQUEST} ^\S+\s+(/[^\s?]*)`,
		"RewriteRule ^ https://%{SERVER_NAME}%1 [R=301,L,NE]")
	// Port 80: ACME and the webhook, then the redirect; no site files.
	port80 := s[:strings.Index(s, "<VirtualHost *:443>")]
	mustContain(t, port80, "Include /etc/apache2/panel/acme.conf", "ProxyPass /.panel/deploy/42")
	mustNotContain(t, port80, "DocumentRoot /var/www", "FilesMatch")
	if strings.Index(port80, "acme-challenge/ - [L]") > strings.Index(port80, "R=301") {
		t.Fatal("ACME challenges would be redirected")
	}
	if n := strings.Count(s, "DocumentRoot /var/www/example.com/public"); n != 1 {
		t.Fatalf("site body appears %d times, want 1", n)
	}
	s = renderApacheCase(t, apacheCases()["php-ssl"])
	if n := strings.Count(s, "DocumentRoot /var/www/example.com/public"); n != 2 {
		t.Fatalf("site body appears %d times, want 2 (HTTP and HTTPS)", n)
	}
}

func TestApacheVhostAccess(t *testing.T) {
	c := apacheCases()
	s := renderApacheCase(t, c["suspended"])
	mustContain(t, s, `ErrorDocument 503 "This website is temporarily unavailable."`, "RewriteRule ^ - [R=503,L]",
		"DocumentRoot /var/lib/panel-acme")
	mustNotContain(t, s, "<Directory /var/www", "FallbackResource")
	if strings.Index(s, "R=503") < strings.Index(s, "acme-challenge/ - [L]") {
		t.Fatal("ACME challenges would get 503")
	}

	s = renderApacheCase(t, c["redirect-path-ssl"])
	mustContain(t, s, "RewriteRule ^ https://new.example.org:8443/sub/dir%1 [R=301,L,NE]",
		"RewriteRule ^/?(.*)$ https://new.example.org:8443/sub/dir/$1 [R=301,L]")
	d := c["redirect"]
	d.RedirectURL = "https://new.example.org/a%2Fb"
	s = renderApacheCase(t, d)
	mustContain(t, s, `RewriteRule ^ https://new.example.org/a\%2Fb%1 [R=301,L,NE]`)

	s = renderApacheCase(t, c["auth"])
	mustContain(t, s, "AuthType Basic", `AuthName "Restricted"`, "AuthUserFile /etc/apache2/panel/auth/example.com.htpasswd",
		"AuthMerging And", "Satisfy All", "Require valid-user", "<Location /.well-known/acme-challenge/>")
	loc := s[strings.Index(s, "<Location />"):]
	if strings.Index(loc, "AuthType") > strings.Index(loc, "</Location>") {
		t.Fatal("the password is not in the <Location /> block")
	}

	s = renderApacheCase(t, c["blocked-static"])
	mustContain(t, s, "Require all granted\n            Require not ip 203.0.113.7 198.51.100.0/24 2001:db8::/32")
	// The webhook location also refuses them.
	hook := s[strings.Index(s, "<Location /.panel/deploy/42>"):]
	mustContain(t, hook[:strings.Index(hook, "</Location>")], "Require not ip 203.0.113.7")
}

func TestApacheVhostFirewallAndOptions(t *testing.T) {
	c := apacheCases()
	s := renderApacheCase(t, c["waf-detect"])
	mustContain(t, s, "SecRuleEngine DetectionOnly", "SecRuleEngine Off")
	// The engine is set for the virtual host, so phase 1 runs.
	if strings.Index(s, "SecRuleEngine DetectionOnly") > strings.Index(s, "<Location />") {
		t.Fatal("SecRuleEngine inside <Location />: phase 1 rules would not run")
	}
	s = renderApacheCase(t, c["waf-block"])
	mustContain(t, s, "SecRuleEngine On")
	// No rate limit on Apache.
	for _, n := range []string{"ratelimit-php", "ratelimit-app", "everything-php"} {
		d := c[n]
		a := apacheServer{}
		out, err := a.vhost(context.Background(), d, presetOf(d))
		if err != nil {
			t.Fatal(err)
		}
		mustNotContain(t, string(out), "limit_req", "panel_rl")
	}

	s = renderApacheCase(t, c["hotlink-php-allow"])
	mustContain(t, s,
		`RewriteCond %{HTTP_REFERER} !^https?://(?:example\.com|www\.example\.com|partner\.example\.org|[a-z0-9-]+\.cdn\.example\.net)(?::[0-9]+)?(?:[/?#]|$) [NC]`,
		`RewriteRule \.(?:jpe?g|png|gif|webp|avif|svg|ico|bmp|mp4|webm|mp3|ogg|pdf)$ - [F,NC]`)
	s = renderApacheCase(t, c["custom404-static"])
	mustContain(t, s, "ErrorDocument 404 /404.html")
	s = renderApacheCase(t, c["custom-php"])
	mustContain(t, s, "Include /etc/apache2/panel/custom/example.com.conf")
	// The administrator's directives come last.
	if strings.LastIndex(s, "Include /etc/apache2/panel/custom/") < strings.LastIndex(s, "</Directory>") {
		t.Fatal("custom directives before the site's directory block")
	}
}

func TestApachePresets(t *testing.T) {
	for _, p := range RewritePresets {
		if _, ok := apachePresetRules[p.ID]; !ok {
			t.Errorf("preset %s has no Apache rules", p.ID)
		}
	}
	d := apacheCases()["php"]
	s := renderApacheCase(t, func() vhostData { d.RewriteRules = rewriteRules("wordpress", true); return d }())
	mustContain(t, s, `RewriteRule ^/wp-content/uploads/.*\.php$ - [F,NC]`, "FallbackResource /index.php")
	// Protective rules run at the virtual host level, before .htaccess:
	// outside the <Directory> block.
	dir := s[strings.Index(s, "<Directory /var/www"):]
	mustNotContain(t, dir[:strings.Index(dir, "</Directory>")], "wp-content")
	// A PHP preset on a static site falls back to the default rules.
	if r := apacheRewriteRules("laravel", false); r.Dir != "" || r.Server != "" {
		t.Fatalf("PHP preset on a static site: %+v", r)
	}
	if r := apacheRewriteRules("spa", false); !strings.Contains(r.Dir, "/index.html") {
		t.Fatalf("spa: %+v", r)
	}
	if r := apacheRewriteRules("static-clean", false); !strings.Contains(r.Dir, "RewriteCond %{REQUEST_FILENAME}.html -f") ||
		!strings.Contains(r.Dir, "RewriteRule ^ %{REQUEST_URI}.html [L]") || r.Server != "" {
		t.Fatalf("static-clean: %+v", r)
	}
	if _, err := renderApacheVhost(apacheCases()["static"], "nope"); err == nil {
		t.Fatal("unknown preset rendered")
	}
	list := apachePresetList()
	if len(list) != len(RewritePresets) || !strings.Contains(list[0].Rules, "FallbackResource") {
		t.Fatalf("preset list: %+v", list[0])
	}
}

func TestApacheRefusesInjection(t *testing.T) {
	for _, bad := range []func(*vhostData){
		func(d *vhostData) { d.Domain = "example.com\n</VirtualHost>" },
		func(d *vhostData) { d.Aliases = []string{"a.com b.com"} },
		func(d *vhostData) { d.PHPSocket = `/run/php/x.sock|fcgi://evil" "` },
		func(d *vhostData) { d.RedirectURL = "https://x.com/ [L]" },
		func(d *vhostData) { d.BlockedIPs = []string{"1.2.3.4 all"} },
		func(d *vhostData) { d.EmptyRoot = "" },
		func(d *vhostData) { d.CustomFile = "/etc/x.conf\nInclude /etc/shadow" },
		func(d *vhostData) { d.HotlinkAllow = []string{"x.com)|(.*"} },
	} {
		d := apacheCases()["php"]
		bad(&d)
		if _, err := renderApacheVhost(d, ""); err == nil {
			t.Errorf("rendered %+v", d)
		}
	}
}

func TestApacheShared(t *testing.T) {
	m := goldenManagerApache(t)
	a := apacheServer{m}
	hook, err := render(apacheHookTmpl, apacheHookData{WWW: "/var/www", EmptyRoot: "/var/lib/panel-acme", SnippetDir: "/etc/apache2/panel", SitesDir: "/etc/apache2/panel/sites"})
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, string(hook),
		`LogFormat "%a %l %u %t \"%r\" %>s %b \"%{Referer}i\" \"%{User-Agent}i\"" panel_combined`,
		"<Directory /var/www/>", "Require all denied", "ServerName default.invalid",
		"SSLCertificateFile /etc/apache2/panel/default.crt", "IncludeOptional /etc/apache2/panel/sites/*.conf")
	checkGolden(t, "apache", "hook.conf", hook)
	acme, _ := render(apacheACMETmpl, struct{ Webroot string }{"/var/lib/panel-acme"})
	mustContain(t, string(acme), "Alias /.well-known/acme-challenge/ /var/lib/panel-acme/.well-known/acme-challenge/")
	checkGolden(t, "apache", "acme.conf", acme)
	conf := a.adminerConf("46000", "0123456789abcdef0123456789abcdef", "8.3")
	mustContain(t, string(conf), "Listen 127.0.0.1:46000", "<VirtualHost 127.0.0.1:46000>",
		"RewriteCond %{HTTP:X-Panel-Adminer} !=0123456789abcdef0123456789abcdef", "RequestHeader unset X-Panel-Adminer",
		`SetHandler "proxy:unix:/run/php/php8.3-fpm.sock|fcgi://localhost"`)
	checkGolden(t, "apache", "adminer.conf", conf)
	for _, bad := range [][3]string{{"46000", "x\nListen 80", "8.3"}, {"80 443", "abcdefabcdefabcd", "8.3"}, {"46000", "abcdefabcdefabcd", "8.3;"}} {
		if a.adminerConf(bad[0], bad[1], bad[2]) != nil {
			t.Errorf("adminer rendered %q", bad)
		}
	}
	custom := a.customBytes(&store.Site{NginxCustom: "Header set X-A b\n\n"})
	if string(custom) != "# Managed by Nefo: edit in Websites → Apache configuration.\nHeader set X-A b\n" {
		t.Fatalf("custom = %q", custom)
	}
	if runtime.GOOS == "windows" {
		return // the paths below are Linux paths
	}
	waf := a.wafFiles([]*store.Site{{Domain: "wp.example.com", App: AppWordPress}})
	main := string(waf[filepath.Join(wafDirOf(a), "main.conf")])
	mustContain(t, main, "SecRuleEngine Off", "SecAuditLogFormat JSON", "SecAuditLog /var/log/panel-waf/audit.log",
		`wp\.example\.com`, "crs_exclusions_wordpress", "SecUnicodeMapFile /etc/modsecurity/unicode.mapping 20127")
	mustNotContain(t, main, "Include /usr/share/modsecurity-crs") // security2.conf loads it
	mustContain(t, string(waf[apacheWAFInclude]), "IncludeOptional "+filepath.Join(wafDirOf(a), "main.conf"))
}

func TestApacheTestError(t *testing.T) {
	out := "AH00558: apache2: Could not reliably determine the server's fully qualified domain name\n" +
		"AH00526: Syntax error on line 2 of /etc/apache2/panel/custom/a.com.conf:\n" +
		"Invalid command 'Foo', perhaps misspelled or defined by a module not included in the server configuration\n" +
		"Action '-t' failed.\nThe Apache error log may have more information.\n"
	want := "AH00526: Syntax error on line 2 of /etc/apache2/panel/custom/a.com.conf:\n" +
		"Invalid command 'Foo', perhaps misspelled or defined by a module not included in the server configuration"
	if got := apacheTestError(out); got != want {
		t.Fatalf("got %q", got)
	}
}

func TestParseAuditModSecurity2(t *testing.T) {
	line := `{"transaction":{"time":"24/Nov/2026:23:30:32.272338 +0000","transaction_id":"awYe","remote_address":"203.0.113.9","remote_port":38222,"local_address":"127.0.0.1","local_port":80},` +
		`"request":{"request_line":"GET /?id=1%27%20OR%20%271%27=%271 HTTP/1.1","headers":{"Host":"ua.test","User-Agent":"curl/8.5.0"}},` +
		`"response":{"protocol":"HTTP/1.1","status":403,"headers":{}},` +
		`"audit_data":{"messages":["Warning. detected SQLi using libinjection [file \"/usr/share/modsecurity-crs/rules/REQUEST-942-APPLICATION-ATTACK-SQLI.conf\"] [line \"66\"] [id \"942100\"] [msg \"SQL Injection Attack Detected via libinjection\"] [data \"Matched Data: s&sos found within ARGS:id: 1' OR '1'='1\"]",` +
		`"Access denied with code 403 (phase 2). [id \"949110\"] [msg \"Inbound Anomaly Score Exceeded (Total Score: 5)\"]",` +
		`"Warning. [id \"941100\"] [msg \"XSS with a \\\"quote\\\"\"]"]}}`
	other := strings.Replace(line, `"Host":"ua.test"`, `"Host":"other.test"`, 1)
	events := parseAudit(strings.NewReader(other+"\n"+line+"\n"), []string{"ua.test"}, 10)
	if len(events) != 1 {
		t.Fatalf("events = %+v", events)
	}
	e := events[0]
	if e.IP != "203.0.113.9" || e.Method != "GET" || e.URI != "/?id=1%27%20OR%20%271%27=%271" || e.Status != 403 ||
		e.Host != "ua.test" || e.Time != "24/Nov/2026:23:30:32 +0000" {
		t.Fatalf("event = %+v", e)
	}
	if len(e.Rules) != 2 || e.Rules[0].ID != "942100" || e.Rules[0].Message != "SQL Injection Attack Detected via libinjection" ||
		e.Rules[1].Message != `XSS with a "quote"` {
		t.Fatalf("rules = %+v", e.Rules)
	}
}

func TestUFWWebsiteRules(t *testing.T) {
	out := `Status: active

     To                         Action      From
     --                         ------      ----
[ 1] 22/tcp                     ALLOW IN    Anywhere
[ 2] 80/tcp                     ALLOW IN    Anywhere                   # Websites
[ 3] 443/tcp                    ALLOW IN    Anywhere                   # Websites
[ 4] 8080/tcp                   ALLOW IN    Anywhere                   # Panel
[10] 80/tcp (v6)                ALLOW IN    Anywhere (v6)              # Websites
[11] 443/tcp (v6)               ALLOW IN    Anywhere (v6)              # Websites
`
	got := fmt.Sprint(ufwWebsiteRules(out))
	if got != "[2 3 10 11]" {
		t.Fatalf("rules = %s", got)
	}
}

// goldenManagerApache is a manager whose store says it runs Apache, with
// the panel's folders in a temporary directory.
func goldenManagerApache(t *testing.T) *Manager {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	p := DefaultPaths("/var/lib/nefo")
	m := New(p, goldenRunner("1.24.0"), st, slog.New(slog.NewTextHandler(io.Discard, nil)), "")
	if err := m.setServerKind(context.Background(), ServerApache); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestServerKind(t *testing.T) {
	ctx := context.Background()
	newM := func(r *fakeRunner) *Manager {
		st, err := store.Open(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { st.Close() })
		root := t.TempDir()
		p := DefaultPaths(filepath.Join(root, "data"))
		p.Nginx, p.Apache, p.WWW = filepath.Join(root, "nginx"), filepath.Join(root, "apache2"), filepath.Join(root, "www")
		if r == nil {
			r = &fakeRunner{outputs: map[string]fakeResult{"nginx": {err: errors.New("not found")}, "apache2": {err: errors.New("not found")}}}
		}
		return New(p, r, st, slog.New(slog.NewTextHandler(io.Discard, nil)), "")
	}

	// Nothing installed: no server, Nginx's paths for the "not ready" checks.
	m := newM(nil)
	if k := m.WebServer(); k != "" || m.web().kind() != ServerNginx {
		t.Fatalf("fresh: %q %q", k, m.web().kind())
	}

	// An existing panel: Nginx's hook file and no setting.
	m = newM(nil)
	os.MkdirAll(filepath.Join(m.paths.Nginx, "conf.d"), 0o755)
	os.WriteFile(filepath.Join(m.paths.Nginx, "conf.d", "panel.conf"), nil, 0o644)
	if k := m.WebServer(); k != ServerNginx {
		t.Fatalf("old panel: %q", k)
	}
	if v, _ := m.store.Setting(ctx, webServerKey); v != "" {
		t.Fatalf("detection wrote the setting %q", v)
	}
	if got := m.vhostFile("a.com"); got != filepath.Join(m.paths.Nginx, "panel", "sites", "a.com.conf") {
		t.Fatalf("vhost file %s", got)
	}

	// The setting wins over the files.
	m = newM(nil)
	m.store.SetSetting(ctx, webServerKey, ServerApache)
	if k := m.WebServer(); k != ServerApache || m.LogDir() != m.paths.ApacheLogs {
		t.Fatalf("setting: %q %q", k, m.LogDir())
	}
	if got := m.customFile("a.com"); got != filepath.Join(m.paths.Apache, "panel", "custom", "a.com.conf") {
		t.Fatalf("custom file %s", got)
	}

	// A conflict: Nginx's binary answers, so Apache may not be installed.
	if runtime.GOOS == "linux" {
		if _, err := exec.LookPath("apt-get"); err == nil {
			m = newM(&fakeRunner{outputs: map[string]fakeResult{"nginx": {out: "nginx version: nginx/1.24.0"}, "apache2": {err: errors.New("no")}}})
			var ce *ConflictError
			if _, err := m.CheckInstall(ctx, ServerApache); !errors.As(err, &ce) || ce.Code != "web_server_conflict" || ce.Detail != ServerNginx {
				t.Fatalf("conflict: %v", err)
			}
			if k, err := m.CheckInstall(ctx, ""); err != nil || k != ServerNginx {
				t.Fatalf("nginx again: %q %v", k, err)
			}
			var ue *usererr.Error
			if _, err := m.CheckInstall(ctx, "caddy"); !errors.As(err, &ue) || ue.Code != "invalid_web_server" {
				t.Fatalf("caddy: %v", err)
			}
			// Removal: refused while a site file exists on disk.
			os.MkdirAll(filepath.Join(m.paths.Nginx, "panel", "sites"), 0o755)
			os.WriteFile(filepath.Join(m.paths.Nginx, "panel", "sites", "_adminer.conf"), nil, 0o600)
			if _, err := m.CheckRemove(ctx, ServerNginx); err != nil {
				t.Fatalf("adminer counted as a site: %v", err)
			}
			os.WriteFile(filepath.Join(m.paths.Nginx, "panel", "sites", "ghost.test.conf"), nil, 0o644)
			if _, err := m.CheckRemove(ctx, ServerNginx); !errors.As(err, &ce) || ce.Code != "web_server_in_use" || ce.Detail != "ghost.test" {
				t.Fatalf("in use: %v", err)
			}
			if _, err := m.CheckRemove(ctx, ServerApache); !errors.As(err, &ue) || ue.Code != "web_server_not_installed" {
				t.Fatalf("apache not installed: %v", err)
			}
		}
	}
}

// TestApacheConfigtest checks every rendered variant with the installed
// Apache. It needs Apache and runs only with NEFO_APACHE_CONFIGTEST=1 (in
// the test sandbox).
func TestApacheConfigtest(t *testing.T) {
	if os.Getenv("NEFO_APACHE_CONFIGTEST") != "1" {
		t.Skip("set NEFO_APACHE_CONFIGTEST=1 on a server with Apache")
	}
	dir := t.TempDir()
	os.Chmod(dir, 0o755)
	certs := filepath.Join(dir, "certs")
	os.MkdirAll(certs, 0o755)
	c, k, err := selfSigned("example.com", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(certs, "fullchain.pem"), c, 0o600)
	os.WriteFile(filepath.Join(certs, "privkey.pem"), k, 0o600)
	os.WriteFile(filepath.Join(dir, "custom.conf"), []byte("Header set X-Test yes\n"), 0o644)
	acme, _ := render(apacheACMETmpl, struct{ Webroot string }{"/var/lib/panel-acme"})
	os.WriteFile(filepath.Join(dir, "acme.conf"), acme, 0o644)
	vhosts := filepath.Join(dir, "vhosts")
	os.MkdirAll(vhosts, 0o755)
	for n, d := range apacheCases() {
		d.SnippetDir = dir
		if d.CertFile != "" {
			d.CertFile, d.KeyFile = filepath.Join(certs, "fullchain.pem"), filepath.Join(certs, "privkey.pem")
		}
		if d.CustomFile != "" {
			d.CustomFile = filepath.Join(dir, "custom.conf")
		}
		out, err := renderApacheVhost(d, presetOf(d))
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(vhosts, n+".conf"), out, 0o644)
	}
	cmd := exec.Command("sh", "-c", `. /etc/apache2/envvars && apache2 -t -c "IncludeOptional $0/*.conf"`, vhosts)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("apache2 -t: %v\n%s", err, out)
	}
	t.Logf("%d variants: %s", len(apacheCases()), strings.TrimSpace(string(out)))
}
