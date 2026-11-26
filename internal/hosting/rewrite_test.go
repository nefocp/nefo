package hosting

import (
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/validate"
)

func TestRewritePresets(t *testing.T) {
	d := baseVhost()
	d.PHPSocket = "/run/php/x.sock"
	d.RewriteRules = rewriteRules("wordpress", true)
	out, err := renderVhost(d)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	// Regex locations match in order: the uploads rule must come before the
	// PHP location, or uploaded .php files would run.
	up := strings.Index(s, `location ~* ^/wp-content/uploads/.*\.php$`)
	php := strings.Index(s, `location ~ [^/]\.php(/|$)`)
	if up < 0 || php < 0 || up > php {
		t.Fatalf("uploads rule at %d, PHP location at %d:\n%s", up, php, s)
	}
	mustContain(t, s, `fastcgi_split_path_info ^(.+?\.php)(/.*)$;`, "try_files $fastcgi_script_name =404;", "fastcgi_param PATH_INFO $path_info;")
	if strings.Count(s, "location / {") != 1 { // one server block: no SSL here
		t.Fatalf("want exactly one location /:\n%s", s)
	}

	// Every preset renders, and Nginx-significant text stays inside rules
	// the panel wrote itself.
	for _, p := range RewritePresets {
		d := baseVhost()
		if p.PHP {
			d.PHPSocket = "/run/php/x.sock"
		}
		d.RewriteRules = rewriteRules(p.ID, p.PHP)
		if _, err := renderVhost(d); err != nil {
			t.Errorf("%s: %v", p.ID, err)
		}
		if strings.Count(p.Rules, "{") != strings.Count(p.Rules, "}") {
			t.Errorf("%s: unbalanced braces", p.ID)
		}
	}

	// A PHP preset on a static site falls back to the static rules.
	if got := rewriteRules("laravel", false); got != defaultStaticRules {
		t.Fatalf("laravel without PHP = %q", got)
	}
	if got := rewriteRules("spa", false); !strings.Contains(got, "/index.html;") {
		t.Fatalf("spa = %q", got)
	}
	// Clean URLs: /docs/install is docs/install.html, /docs/ docs/index.html
	// (the index), and a missing page stays a 404. PHP sites may use it too.
	for _, php := range []bool{false, true} {
		if got := rewriteRules("static-clean", php); !strings.Contains(got, "try_files $uri $uri.html $uri/ =404;") {
			t.Fatalf("static-clean (php %v) = %q", php, got)
		}
	}
	if got := rewriteRules("nope", true); got != defaultPHPRules {
		t.Fatalf("unknown preset = %q", got)
	}

	// Rules that are not the panel's own never reach the template.
	d = baseVhost()
	d.RewriteRules = "    location / { return 200; }"
	if _, err := renderVhost(d); err == nil {
		t.Fatal("rendered unknown rules")
	}
}

func TestCustomInclude(t *testing.T) {
	d := baseVhost()
	d.CustomFile = "/etc/nginx/panel/custom/example.com.conf"
	out, err := renderVhost(d)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	inc := strings.Index(s, "include /etc/nginx/panel/custom/example.com.conf;")
	loc := strings.Index(s, "location / {")
	if inc < 0 || inc > loc {
		t.Fatalf("custom include must come before the rules:\n%s", s)
	}
	d.CustomFile = "/etc/x.conf; deny all"
	if _, err := renderVhost(d); err == nil {
		t.Fatal("accepted an unsafe custom path")
	}
	// App sites get the include too.
	d = baseVhost()
	d.ProxyPort, d.CustomFile = 3000, "/etc/nginx/panel/custom/example.com.conf"
	out, _ = renderVhost(d)
	mustContain(t, string(out), "include /etc/nginx/panel/custom/example.com.conf;")
}

func TestWildcardAlias(t *testing.T) {
	for name, ok := range map[string]bool{
		"*.example.com": true, "example.com": true, "*.com": false, "a.*.example.com": false,
		"**.example.com": false, "*example.com": false, "*": false,
	} {
		if (validate.Alias(name) == nil) != ok {
			t.Errorf("Alias(%q) ok = %v", name, !ok)
		}
	}
	d := baseVhost()
	d.Aliases = []string{"*.example.com"}
	out, err := renderVhost(d)
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, string(out), "server_name example.com *.example.com;")
	d = baseVhost()
	d.Domain = "*.example.com"
	if _, err := renderVhost(d); err == nil {
		t.Fatal("a wildcard main domain was accepted")
	}
}

func TestNginxTestError(t *testing.T) {
	out := "nginx: [emerg] unknown directive \"foo\" in /etc/nginx/panel/custom/a.com.conf:2\nnginx: configuration file /etc/nginx/nginx.conf test failed\n"
	if got := nginxTestError(out); got != `[emerg] unknown directive "foo" in /etc/nginx/panel/custom/a.com.conf:2` {
		t.Fatalf("got %q", got)
	}
}
