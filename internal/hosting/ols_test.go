package hosting

import (
	"context"
	"errors"
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

const olsErrorDir = "/usr/local/lsws/panel-errors"

// olsCases are nginxVhostCases with OpenLiteSpeed's paths, and the sites'
// .htaccess rules in a few shapes.
func olsCases() map[string]olsVhost {
	cases := map[string]olsVhost{}
	for n, d := range nginxVhostCases() {
		d.LogDir, d.SnippetDir, d.EmptyRoot = "/var/log/openlitespeed", "/usr/local/lsws/panel", "/var/lib/panel-acme"
		d.User = "example_com"
		if d.AuthFile != "" {
			d.AuthFile = "/usr/local/lsws/panel/auth/example.com.htpasswd"
		}
		if d.CustomFile != "" {
			d.CustomFile = "/usr/local/lsws/panel/custom/example.com.conf"
		}
		cases[n] = olsVhost{vhostData: d, ErrorDir: olsErrorDir}
	}
	wp := sanitizeHtaccess([]byte(wordpressHtaccess))
	sub := olsHtContext{URI: "/blog/", Location: "/var/www/example.com/public/blog/",
		Rules: "RewriteEngine on\nRewriteBase /blog/\nRewriteRule ^old$ new.php [L]"}
	for _, n := range []string{"php", "static", "preset-wordpress", "preset-opencart", "preset-spa", "preset-static-clean", "everything-php"} {
		c := cases[n]
		c.Htaccess = olsHtaccess{Root: wp, Contexts: []olsHtContext{sub}}
		cases["htaccess-"+n] = c
	}
	php := cases["php"]
	php.PHPWorkers = 12
	cases["php-workers"] = php
	return cases
}

const wordpressHtaccess = `# BEGIN LSCACHE
<IfModule LiteSpeed>
RewriteEngine on
CacheLookup on
RewriteRule .* - [E=Cache-Control:no-autoflush]
RewriteRule \.litespeed_conf\.dat - [F,L]
RewriteRule favicon\.ico$ - [E=cache-control:max-age=86400]
</IfModule>
# END LSCACHE
# BEGIN WordPress
<IfModule mod_rewrite.c>
RewriteEngine On
RewriteRule .* - [E=HTTP_AUTHORIZATION:%{HTTP:Authorization}]
RewriteBase /
RewriteRule ^index\.php$ - [L]
RewriteCond %{REQUEST_FILENAME} !-f
RewriteCond %{REQUEST_FILENAME} !-d
RewriteRule . /index.php [L]
</IfModule>
# END WordPress
`

func renderOLSCase(t *testing.T, v olsVhost) string {
	t.Helper()
	out, err := renderOLSVhost(v, presetOf(v.vhostData))
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestOLSGoldenVhosts(t *testing.T) {
	cases := olsCases()
	names := make([]string, 0, len(cases))
	for n := range cases {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		s := renderOLSCase(t, cases[n])
		if strings.Count(s, "{") != strings.Count(s, "}") {
			t.Errorf("%s: unbalanced braces:\n%s", n, s)
		}
		if strings.Count(s, "<<<END_rules") != strings.Count(s, "\nEND_rules\n") {
			t.Errorf("%s: unbalanced rules blocks:\n%s", n, s)
		}
		if strings.Contains(s, "limit_req") || strings.Contains(s, "location /") || strings.Contains(s, "location ~") || strings.Contains(s, "<VirtualHost") {
			t.Errorf("%s: another server's syntax:\n%s", n, s)
		}
		checkGolden(t, "ols", "vhost-"+n+".conf", []byte(s))
	}
}

func TestOLSVhostPHP(t *testing.T) {
	s := renderOLSCase(t, olsCases()["php"])
	mustContain(t, s,
		"virtualHost example.com {",
		"vhRoot                  /var/www/example.com/",
		"vhDomain                example.com",
		"vhAliases               www.example.com",
		"listeners               http\n",
		"user                    example_com",
		"allowSymbolLink         2",
		"forceStrictOwnership    1",
		"docRoot                 /var/www/example.com/public/",
		"address               uds://run/php/panel-example_com.sock",
		"maxConns              5",
		"persistConn           0",
		"autoStart             0",
		"add                   fcgi:php_example_com php",
		"indexFiles            index.php, index.html, index.htm",
		"autoLoadHtaccess      0",
		"accesslog /var/log/openlitespeed/example.com.access.log {",
		`logFormat             %h %l %u %t "%r" %>s %b "%{Referer}i" "%{User-Agent}i"`,
		"RewriteRule ^/\\.panel/deploy/42$ http://panel_githook/git/42 [P,L]",
		"RewriteCond %{HTTP:Content-Length} -gt67108864",
		"RewriteRule /\\.(?!well-known/) - [F]",
		// The front controller follows the site's own rules.
		"RewriteRule ^ /index.php [L]",
	)
	mustNotContain(t, s, "vhssl", "realm", "mod_security", "accessControl", "lsapi", "https\n")
	// The webhook and Let's Encrypt pass before every other panel rule.
	rules := s[strings.LastIndex(s, "  rewrite {"):]
	if strings.Index(rules, "acme-challenge/ - [L]") > strings.Index(rules, "[R=413,L]") ||
		strings.Index(rules, "panel_githook") > strings.Index(rules, "-gt67108864") {
		t.Fatalf("rule order:\n%s", rules)
	}
	if n := strings.Count(s, "\n}\n"); n != 1 {
		t.Fatalf("%d top-level blocks", n)
	}
	s = renderOLSCase(t, olsCases()["php-workers"])
	mustContain(t, s, "maxConns              12")
}

func TestOLSVhostStaticAndApp(t *testing.T) {
	c := olsCases()
	s := renderOLSCase(t, c["static"])
	mustContain(t, s, "indexFiles            index.html, index.htm", "context / {")
	mustNotContain(t, s, "extprocessor", "scripthandler", "index.php")

	s = renderOLSCase(t, c["app"])
	mustContain(t, s,
		"docRoot                 /var/lib/panel-acme/",
		"type                  proxy",
		"address               127.0.0.1:3000",
		"handler               app_example_com",
		"websocket / {",
		"RequestHeader unset X-Real-IP",
		"context /.well-known/acme-challenge/ {",
		"RewriteCond %{HTTP:Content-Length} -gt67108864")
	mustNotContain(t, s, "docRoot                 /var/www", "fcgi", "RewriteRule /\\.(?!well-known/)")
}

func TestOLSVhostSSL(t *testing.T) {
	c := olsCases()
	s := renderOLSCase(t, c["php-force"])
	mustContain(t, s, "listeners               http, https", "vhssl {",
		"certFile              /var/lib/nefo/certs/example.com/fullchain.pem",
		"keyFile               /var/lib/nefo/certs/example.com/privkey.pem",
		"RewriteCond %{HTTPS} !on",
		"RewriteRule ^ https://%{SERVER_NAME}%1 [R=301,L,NE]")
	rules := s[strings.LastIndex(s, "  rewrite {"):]
	if strings.Index(rules, "panel_githook") > strings.Index(rules, "https://%{SERVER_NAME}") {
		t.Fatal("the webhook would be redirected to HTTPS")
	}
	s = renderOLSCase(t, c["php-ssl"])
	mustContain(t, s, "vhssl {")
	mustNotContain(t, s, "RewriteCond %{HTTPS} !on")
}

func TestOLSVhostAccess(t *testing.T) {
	c := olsCases()
	s := renderOLSCase(t, c["suspended"])
	mustContain(t, s, "RewriteRule ^ - [R=503,L]", "url                   /.panel-error/503.html",
		"location              /usr/local/lsws/panel-errors/", "RewriteRule ^/\\.panel-error/ - [L]")
	mustNotContain(t, s, "fcgi", "context / {")

	s = renderOLSCase(t, c["redirect-path-ssl"])
	mustContain(t, s, "RewriteRule ^ https://new.example.org:8443/sub/dir%1 [R=301,L,NE]")

	s = renderOLSCase(t, c["auth"])
	mustContain(t, s, "realm site {", "location            /usr/local/lsws/panel/auth/example.com.htpasswd",
		"realm                 site", "required              valid-user")
	root := s[strings.Index(s, "  context / {"):]
	if strings.Index(root, "realm                 site") > strings.Index(root, "\n  }") {
		t.Fatal("the password is not in the root context")
	}
	s = renderOLSCase(t, c["blocked-static"])
	mustContain(t, s, "accessControl {", "deny                  203.0.113.7, 198.51.100.0/24, 2001:db8::/32")
}

func TestOLSVhostFirewallAndOptions(t *testing.T) {
	c := olsCases()
	s := renderOLSCase(t, c["waf-detect"])
	mustContain(t, s, "module mod_security {", "ls_enabled            1", "modsecurity_rules     SecRuleEngine DetectionOnly")
	s = renderOLSCase(t, c["waf-block"])
	mustContain(t, s, "modsecurity_rules     SecRuleEngine On")
	for _, n := range []string{"ratelimit-php", "ratelimit-app", "everything-php"} {
		if runtime.GOOS == "windows" {
			break // vhost uses the real (Linux) paths
		}
		v := c[n]
		out, err := olsServer{goldenManagerApache(t)}.vhost(context.Background(), v.vhostData, presetOf(v.vhostData))
		if err != nil {
			t.Fatal(err)
		}
		mustNotContain(t, string(out), "limit_req", "panel_rl", "dynReqPerSec")
	}
	s = renderOLSCase(t, c["hotlink-php-allow"])
	mustContain(t, s, `RewriteCond %{HTTP_REFERER} !^https?://(?:example\.com|www\.example\.com|partner\.example\.org|[a-z0-9-]+\.cdn\.example\.net)(?::[0-9]+)?(?:[/?#]|$) [NC]`)
	s = renderOLSCase(t, c["custom404-static"])
	mustContain(t, s, "errorpage 404 {", "url                   /404.html")
	s = renderOLSCase(t, c["custom-php"])
	mustContain(t, s, "include                 /usr/local/lsws/panel/custom/example.com.conf\n}\n")
}

func TestOLSHtaccessContexts(t *testing.T) {
	c := olsCases()
	s := renderOLSCase(t, c["htaccess-php"])
	root := s[strings.Index(s, "  context / {"):]
	root = root[:strings.Index(root, "\n  }\n")]
	// The site's rules first, then the front controller.
	if i, j := strings.Index(root, "RewriteRule . /index.php [L]"), strings.Index(root, "RewriteRule ^ /index.php [L]"); i < 0 || j < i {
		t.Fatalf("root context:\n%s", root)
	}
	mustContain(t, s, "context /blog/ {", "location              /var/www/example.com/public/blog/",
		"RewriteRule ^old$ new.php [L]", "RewriteRule .* - [E=HTTP_AUTHORIZATION:%{HTTP:Authorization}]",
		"RewriteRule favicon\\.ico$ - [E=cache-control:max-age=86400]")
	mustNotContain(t, s, "CacheLookup")
	s = renderOLSCase(t, c["htaccess-static"])
	mustNotContain(t, s, "/index.php [L]\nEND_rules")
	s = renderOLSCase(t, c["htaccess-preset-opencart"])
	blog := s[strings.Index(s, "context /blog/"):]
	mustNotContain(t, blog[:strings.Index(blog, "\n  }\n")], "_route_")
	// A context that is not below the site's folder, or rules that would end
	// the block, are refused.
	for _, bad := range []olsHtContext{
		{URI: "/x y/", Location: "/var/www/example.com/public/x y/", Rules: "RewriteEngine on"},
		{URI: "/x/", Location: "/etc/", Rules: "RewriteEngine on"},
		{URI: "/x/", Location: "/var/www/example.com/public/x/", Rules: "RewriteEngine on\nEND_rules\n}"},
		{URI: "/", Location: "/var/www/example.com/public/", Rules: "RewriteEngine on"},
	} {
		v := c["php"]
		v.Htaccess.Contexts = []olsHtContext{bad}
		if _, err := renderOLSVhost(v, ""); err == nil {
			t.Errorf("rendered %+v", bad)
		}
	}
}

func TestOLSRefusesInjection(t *testing.T) {
	for _, bad := range []func(*olsVhost){
		func(v *olsVhost) { v.Domain = "example.com {\n" },
		func(v *olsVhost) { v.Aliases = []string{"a.com b.com"} },
		func(v *olsVhost) { v.PHPSocket = "/run/php/x.sock\n}" },
		func(v *olsVhost) { v.RedirectURL = "https://x.com/ [L]" },
		func(v *olsVhost) { v.BlockedIPs = []string{"1.2.3.4, ALL"} },
		func(v *olsVhost) { v.User = "root;x" },
		func(v *olsVhost) { v.User = "" },
		func(v *olsVhost) { v.ErrorDir = "/x y" },
		func(v *olsVhost) { v.CustomFile = "/etc/x.conf\ninclude /etc/shadow" },
		func(v *olsVhost) { v.Htaccess.Root = "RewriteEngine on\nEND_rules" },
	} {
		v := olsCases()["php"]
		bad(&v)
		if _, err := renderOLSVhost(v, ""); err == nil {
			t.Errorf("rendered %+v", v)
		}
	}
}

func TestSanitizeHtaccess(t *testing.T) {
	got := sanitizeHtaccess([]byte(strings.ReplaceAll(wordpressHtaccess, "\n", "\r\n")))
	want := "<IfModule LiteSpeed>\nRewriteEngine on\nRewriteRule .* - [E=Cache-Control:no-autoflush]\n" +
		"RewriteRule \\.litespeed_conf\\.dat - [F,L]\nRewriteRule favicon\\.ico$ - [E=cache-control:max-age=86400]\n</IfModule>\n" +
		"<IfModule mod_rewrite.c>\nRewriteEngine on\nRewriteRule .* - [E=HTTP_AUTHORIZATION:%{HTTP:Authorization}]\nRewriteBase /\n" +
		"RewriteRule ^index\\.php$ - [L]\nRewriteCond %{REQUEST_FILENAME} !-f\nRewriteCond %{REQUEST_FILENAME} !-d\nRewriteRule . /index.php [L]\n</IfModule>"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
	for _, bad := range []string{
		"RewriteRule .* - [E=PHP_ADMIN_VALUE:extension=/tmp/x.so]",
		"RewriteRule .* - [L,env=php_value:auto_prepend_file=/etc/passwd]",
		`RewriteRule .* - [E="PHP_VALUE:x=1"]`,
		"RewriteRule .* - [E=SCRIPT_FILENAME:/var/www/other/public/x.php]",
		"RewriteRule .* - [E=%1:extension=/tmp/x.so]",
		"RewriteRule .* - [E=%{HTTP:X-N}:x]",
		"RewriteRule ^(.*)$ http://app_other/$1 [P]",
		"RewriteRule ^(.*)$ http://app_other/$1 [L,proxy]",
		"RewriteRule a b [L] extra",
		`RewriteRule "a b`,
		"RewriteFile /etc/passwd",
		"RewriteMap x txt:/etc/passwd",
		"RewriteBase /a b",
		"END_rules",
		"}",
		"Options +FollowSymLinks",
		"php_value upload_max_filesize 64M",
		"<Files wp-config.php>",
		"SetHandler proxy:unix:/run/php/php8.3-fpm.sock|fcgi://localhost",
		"AddHandler application/x-httpd-php .jpg",
	} {
		if out := sanitizeHtaccess([]byte(bad)); out != "" {
			t.Errorf("kept %q as %q", bad, out)
		}
	}
	for _, ok := range []string{
		"RewriteRule ^c\\.php$ - [E=cache-control:max-age=120]",
		"RewriteRule .* - [E=!HTTP_AUTHORIZATION]",
		"RewriteRule ^x$ /y.php [T=application/x-httpd-php,L]",
		"RewriteCond %{HTTPS} !=on\nRewriteRule ^ https://example.com/ [R=301,L]",
		"CacheKeyModify -qs:fbclid",
		"RewriteRule ^(.+)$ index.php?_route_=$1 [L,QSA]",
		`RewriteRule "^a b$" /c [L]`,
	} {
		if out := sanitizeHtaccess([]byte(ok)); out != ok {
			t.Errorf("dropped %q (%q)", ok, out)
		}
	}
	// A dropped rule takes its conditions with it.
	if out := sanitizeHtaccess([]byte("RewriteCond %{HTTP:X-N} (.+)\nRewriteRule .* - [E=%1:x]\nRewriteRule ^a$ b [L]\nRewriteCond %{HTTPS} on\n")); out != "RewriteRule ^a$ b [L]" {
		t.Errorf("conditions of a dropped rule: %q", out)
	}
	if sanitizeHtaccess(make([]byte, maxHtaccessSize+1)) != "" {
		t.Error("kept a file over the size limit")
	}
}

func TestScanHtaccess(t *testing.T) {
	public := filepath.Join(t.TempDir(), "public")
	write := func(rel, data string) {
		p := filepath.Join(public, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".htaccess", "RewriteEngine On\nRewriteRule ^hi$ hello.php [L]\n")
	write("blog/.htaccess", "RewriteEngine On\nRewriteRule ^a$ b [L]\n")
	write("blog/deep/.htaccess", "Options -Indexes\n") // no rewrite rules: no context
	write(".git/.htaccess", "RewriteEngine On\n")
	write("bad name/.htaccess", "RewriteEngine On\n")
	write("other/real.txt", "x")
	if runtime.GOOS != "windows" {
		os.Symlink(filepath.Join(public, "other", "real.txt"), filepath.Join(public, "other", ".htaccess"))
		os.Symlink(filepath.Join(public, "blog"), filepath.Join(public, "linked"))
	}
	m := goldenManagerApache(t)
	ht := m.scanHtaccess(7, public)
	if ht.Root != "RewriteEngine on\nRewriteRule ^hi$ hello.php [L]" {
		t.Fatalf("root = %q", ht.Root)
	}
	if len(ht.Contexts) != 1 || ht.Contexts[0].URI != "/blog/" || ht.Contexts[0].Location != public+"/blog/" {
		t.Fatalf("contexts = %+v", ht.Contexts)
	}
	st := m.htaccess[7]
	if _, ok := st.files["blog/deep/.htaccess"]; !ok || len(st.files) != 3 {
		t.Fatalf("stamps = %+v", st.files)
	}
	// A missing folder gives no rules.
	if ht := m.scanHtaccess(8, filepath.Join(public, "nope")); ht.Root != "" || len(ht.Contexts) != 0 {
		t.Fatalf("missing folder: %+v", ht)
	}
}

func TestOLSPresets(t *testing.T) {
	for _, p := range RewritePresets {
		if _, ok := olsPresetDir[p.ID]; !ok {
			t.Errorf("preset %s has no OpenLiteSpeed rules", p.ID)
		}
	}
	if r := olsRewriteRules("wordpress", true); !strings.Contains(r.Server, `RewriteRule ^/wp-content/uploads/.*\.php$ - [F,NC]`) ||
		strings.Contains(r.Server, "#") || !strings.Contains(r.Dir, "/index.php") {
		t.Fatalf("wordpress: %+v", r)
	}
	if r := olsRewriteRules("laravel", false); r.Dir != "" || r.Server != "" {
		t.Fatalf("PHP preset on a static site: %+v", r)
	}
	if r := olsRewriteRules("", false); r.Dir != "" {
		t.Fatalf("static default: %+v", r)
	}
	// Clean URLs use the whole path, so folders with their own .htaccess
	// get them too.
	if r := olsRewriteRules("static-clean", false); !strings.Contains(r.Dir, "RewriteCond %{REQUEST_FILENAME}.html -f") ||
		!strings.Contains(r.Dir, "RewriteRule ^ %{REQUEST_URI}.html [L]") || !r.Nested || r.Server != "" {
		t.Fatalf("static-clean: %+v", r)
	}
	list := olsPresetList()
	if len(list) != len(RewritePresets) || !strings.Contains(list[0].Rules, "RewriteRule ^ /index.php [L]") {
		t.Fatalf("preset list: %+v", list[0])
	}
}

func TestOpenPGPFingerprint(t *testing.T) {
	key, err := os.ReadFile(filepath.Join("testdata", "ols", "lst_repo.gpg"))
	if err != nil {
		t.Fatal(err)
	}
	fp, err := openPGPFingerprint(key)
	if err != nil || fp != olsKeyFingerprint {
		t.Fatalf("fingerprint %s, %v", fp, err)
	}
	old, _ := os.ReadFile(filepath.Join("testdata", "ols", "lst_debian_repo.gpg"))
	if fp, err := openPGPFingerprint(old); err != nil || fp == olsKeyFingerprint {
		t.Fatalf("old key: %s %v", fp, err)
	}
	for name, bad := range map[string][]byte{
		"two keys":  append(append([]byte{}, key...), old...),
		"armored":   []byte("-----BEGIN PGP PUBLIC KEY BLOCK-----\n"),
		"truncated": key[:len(key)-10],
		"secret":    {0x95, 0x00, 0x01, 0x04},
		"empty":     nil,
	} {
		if _, err := openPGPFingerprint(bad); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestOLSTestProblems(t *testing.T) {
	out := `2026-11-25 01:36:40.882319 [WARN] [PlainConf] [virtualhost:t2.test] Not support [thisisnotadirective on] in file /usr/local/lsws/panel/custom/t2.test.conf:1
2026-11-25 01:36:40.882320 [WARN] [PlainConf] [virtualhost:t2.test] Not support [newkey 1] in file /usr/local/lsws/panel/sites/t2.test.conf:9
2026-11-25 01:36:41.474104 [WARN] [PlainConf] [virtualhost:t2.test] parseFile find '{' and '}' do not match in the end of file /usr/local/lsws/conf/httpd_config.conf, rootTag httpServerConfig.
2026-11-25 01:36:41.481963 [ERROR] [9168] [config:server:vhosts:vhost:t2.test:context:/api/] path is not accessible: /nonexistent/
2026-11-25 01:36:41.481964 [ERROR] [9168] [config:server:vhosts:vhost:t2.test:context:/blog/] path is not accessible: /var/www/t2.test/public/blog/
2026-11-25 01:25:38.193235 [ERROR] [8453] [(null)] [REWRITE] invalid rewrite condition while parsing: RewriteCond x
2026-11-25 01:11:18.137377 [WARN] [7578] [config:server:vhosts:vhost:_default] Uid of /var/lib/panel-acme/ is 0, smaller than minimum requirement 100, use server uid!
`
	got := olsTestProblems(out, "/var/www", "/usr/local/lsws/panel/custom")
	want := []string{
		"[WARN] [PlainConf] [virtualhost:t2.test] Not support [thisisnotadirective on] in file /usr/local/lsws/panel/custom/t2.test.conf:1",
		"[WARN] [PlainConf] [virtualhost:t2.test] parseFile find '{' and '}' do not match in the end of file /usr/local/lsws/conf/httpd_config.conf, rootTag httpServerConfig.",
		"[ERROR] [config:server:vhosts:vhost:t2.test:context:/api/] path is not accessible: /nonexistent/",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got:\n%s", strings.Join(got, "\n"))
	}
	if p := olsTestProblems("", "/var/www", "/x"); len(p) != 0 {
		t.Fatalf("clean: %v", p)
	}
}

func TestOLSShared(t *testing.T) {
	m := goldenManagerApache(t)
	o := olsServer{m}
	main, _ := render(olsMainTmpl, struct{ Hook string }{"/usr/local/lsws/panel/server.conf"})
	// The package's upgrade script greps the file for these words.
	if n := strings.Count(string(main), "user"); n != 1 {
		t.Fatalf("%d lines with \"user\":\n%s", n, main)
	}
	if n := strings.Count(string(main), "group"); n != 1 {
		t.Fatalf("%d lines with \"group\":\n%s", n, main)
	}
	checkGolden(t, "ols", "httpd_config.conf", main)
	for _, c := range []struct {
		name      string
		ipv6, waf bool
	}{{"server-ipv6.conf", true, false}, {"server-noipv6-waf.conf", false, true}} {
		hook, err := render(olsHookTmpl, olsHookData{Home: "/usr/local/lsws", SnippetDir: "/usr/local/lsws/panel",
			SitesDir: "/usr/local/lsws/panel/sites", EmptyRoot: "/var/lib/panel-acme", DataDir: "/var/lib/nefo",
			Hook: hookPort, IPv6: c.ipv6, WAF: c.waf})
		if err != nil {
			t.Fatal(err)
		}
		s := string(hook)
		mustContain(t, s, "disableWebAdmin           1", "followSymbolLink        2", "forceStrictOwnership    1",
			"include                   /usr/local/lsws/panel/sites/*.conf", "map                     _default *",
			"enableCache             0", "dir                     /usr/local/lsws/panel/*", "address                 127.0.0.1:7809")
		mustNotContain(t, s, ":7080", "lsphp", "scriptHandler", "lsapi")
		if strings.Count(s, "{") != strings.Count(s, "}") {
			t.Fatalf("unbalanced:\n%s", s)
		}
		if c.waf != strings.Contains(s, "module mod_security") {
			t.Fatalf("waf %v:\n%s", c.waf, s)
		}
		checkGolden(t, "ols", c.name, hook)
	}
	conf := o.adminerConf("46000", "0123456789abcdef0123456789abcdef", "8.3")
	mustContain(t, string(conf), "address                 127.0.0.1:46000", "map                     _adminer *",
		"RewriteCond %{HTTP:X-Panel-Adminer} !=0123456789abcdef0123456789abcdef", "uds://run/php/php8.3-fpm.sock",
		"[E=HTTPS:on,L]")
	checkGolden(t, "ols", "adminer.conf", conf)
	for _, bad := range [][3]string{{"46000", "x\n}", "8.3"}, {"80 443", "abcdefabcdefabcd", "8.3"}, {"46000", "abcdefabcdefabcd", "8.3;"}} {
		if o.adminerConf(bad[0], bad[1], bad[2]) != nil {
			t.Errorf("adminer rendered %q", bad)
		}
	}
	custom := o.customBytes(&store.Site{NginxCustom: "context /api/ {\n}\n\n"})
	if string(custom) != "# Managed by Nefo: edit in Websites → OpenLiteSpeed configuration.\ncontext /api/ {\n}\n" {
		t.Fatalf("custom = %q", custom)
	}
	if runtime.GOOS == "windows" {
		return // the paths below are Linux paths
	}
	waf := string(o.wafFiles([]*store.Site{{Domain: "wp.example.com", App: AppWordPress}})[filepath.Join(wafDirOf(o), "main.conf")])
	mustContain(t, waf, "SecAuditLogFormat JSON", "SecAuditLog /var/log/panel-waf/audit.log", `wp\.example\.com`,
		"Include /usr/share/modsecurity-crs/rules/*.conf", "ctl:ruleEngine=Off", "SecRequestBodyLimitAction ProcessPartial")
	mustNotContain(t, waf, "SecRuleEngine On", "SecRuleEngine Off", "SecUnicodeMapFile")
}

func TestUFWWebsiteRulesUDP(t *testing.T) {
	out := `Status: active

     To                         Action      From
     --                         ------      ----
[ 1] 22/tcp                     ALLOW IN    Anywhere
[ 2] 80/tcp                     ALLOW IN    Anywhere                   # Websites
[ 3] 443/tcp                    ALLOW IN    Anywhere                   # Websites
[ 4] 443/udp                    ALLOW IN    Anywhere                   # Websites
[ 5] 80/tcp (v6)                ALLOW IN    Anywhere (v6)              # Websites
[ 6] 443/tcp (v6)               ALLOW IN    Anywhere (v6)              # Websites
[ 7] 443/udp (v6)               ALLOW IN    Anywhere (v6)              # Websites
`
	if got := ufwWebsiteRules(out); len(got) != 6 || got[2] != 4 || got[5] != 7 {
		t.Fatalf("rules = %v", got)
	}
}

func TestServerKindOLS(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	root := t.TempDir()
	p := DefaultPaths(filepath.Join(root, "data"))
	p.Nginx, p.Apache, p.OLS, p.WWW = filepath.Join(root, "nginx"), filepath.Join(root, "apache2"), filepath.Join(root, "lsws"), filepath.Join(root, "www")
	p.OLSLogs = filepath.Join(root, "ols-logs")
	r := &fakeRunner{outputs: map[string]fakeResult{"nginx": {err: errors.New("no")}, "apache2": {err: errors.New("no")}}}
	m := New(p, r, st, slog.New(slog.NewTextHandler(io.Discard, nil)), "")

	// The hook file alone (an older panel never had OpenLiteSpeed, but the
	// detection is the same).
	os.MkdirAll(filepath.Join(p.OLS, "panel"), 0o755)
	os.WriteFile(filepath.Join(p.OLS, "panel", "server.conf"), nil, 0o644)
	if k := m.WebServer(); k != ServerOLS || m.LogDir() != p.OLSLogs {
		t.Fatalf("detected %q %q", k, m.LogDir())
	}
	if got := m.vhostFile("a.com"); got != filepath.Join(p.OLS, "panel", "sites", "a.com.conf") {
		t.Fatalf("vhost file %s", got)
	}
	if got := m.authFile("a.com"); got != filepath.Join(p.OLS, "panel", "auth", "a.com.htpasswd") {
		t.Fatalf("auth file %s", got)
	}
	if info := m.server(ServerOLS).info(ctx); info.Installed {
		t.Fatal("installed without its program")
	}
	if _, err := validKind(ServerOLS); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "linux" {
		if _, err := exec.LookPath("apt-get"); err == nil {
			// Nginx is refused while OpenLiteSpeed's hook file exists.
			var ce *ConflictError
			if _, err := m.CheckInstall(ctx, ServerNginx); !errors.As(err, &ce) || ce.Detail != ServerOLS {
				t.Fatalf("conflict: %v", err)
			}
			var ue *usererr.Error
			if _, err := m.CheckRemove(ctx, "litespeed"); !errors.As(err, &ue) || ue.Code != "invalid_web_server" {
				t.Fatalf("bad name: %v", err)
			}
		}
	}
}

func TestProcNetAddr(t *testing.T) {
	for in, want := range map[string]string{
		"0100007F:1F90":                         "127.0.0.1:8080",
		"00000000:0050":                         "0.0.0.0:80",
		"00000000000000000000000000000000:01BB": ":::443",
		"00000000000000000000000001000000:0050": "::1:80",
		"zz:0050":                               "",
	} {
		if got := procNetAddr(in); got != want {
			t.Errorf("%s: %q, want %q", in, got, want)
		}
	}
}

func TestPHPModuleGuard(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks")
	}
	root := t.TempDir()
	php, reg := filepath.Join(root, "php"), filepath.Join(root, "modules")
	confd := filepath.Join(php, "8.3", "fpm", "conf.d")
	os.MkdirAll(confd, 0o755)
	os.Symlink("/etc/php/8.3/mods-available/posix.ini", filepath.Join(confd, "20-posix.ini"))
	os.Symlink("/etc/php/8.3/mods-available/mysqli.ini", filepath.Join(confd, "20-mysqli.ini"))
	os.WriteFile(filepath.Join(confd, "99-panel.ini"), []byte("x=1\n"), 0o644)
	os.MkdirAll(filepath.Join(reg, "registry"), 0o755)
	os.WriteFile(filepath.Join(reg, "registry", "posix"), nil, 0o644)
	st := snapshotPHPModules(php, reg)
	// What the lsphp purge does: links gone, modules disabled by admin.
	os.Remove(filepath.Join(confd, "20-posix.ini"))
	os.Remove(filepath.Join(confd, "20-mysqli.ini"))
	os.MkdirAll(filepath.Join(reg, "8.3", "fpm", "disabled_by_admin"), 0o755)
	os.WriteFile(filepath.Join(reg, "8.3", "fpm", "disabled_by_admin", "posix"), nil, 0o644)
	os.WriteFile(filepath.Join(reg, "registry", "ALL"), nil, 0o644)
	if n := st.restore(php, reg); n != 4 {
		t.Fatalf("restored %d", n)
	}
	if target, err := os.Readlink(filepath.Join(confd, "20-posix.ini")); err != nil || target != "/etc/php/8.3/mods-available/posix.ini" {
		t.Fatalf("posix link: %q %v", target, err)
	}
	if fileExists(filepath.Join(reg, "8.3", "fpm", "disabled_by_admin", "posix")) || fileExists(filepath.Join(reg, "registry", "ALL")) {
		t.Fatal("registry marks left")
	}
	if b, _ := os.ReadFile(filepath.Join(confd, "99-panel.ini")); string(b) != "x=1\n" {
		t.Fatal("regular file changed")
	}
	if n := st.restore(php, reg); n != 0 {
		t.Fatalf("second restore changed %d", n)
	}
}

func TestTryLockWaitsForWatcher(t *testing.T) {
	m := goldenManagerApache(t)
	m.ops.Lock()
	if err := m.tryLock(); err != ErrBusy {
		t.Fatalf("a job holds the lock: %v", err)
	}
	m.htaccessBusy.Store(true)
	go func() {
		time.Sleep(100 * time.Millisecond)
		m.ops.Unlock()
		m.htaccessBusy.Store(false)
	}()
	if err := m.tryLock(); err != nil {
		t.Fatalf("watcher held the lock: %v", err)
	}
	m.ops.Unlock()
}
