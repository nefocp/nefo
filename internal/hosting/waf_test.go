package hosting

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/store"
)

func TestFirewallVhost(t *testing.T) {
	d := baseVhost()
	d.PHPSocket = "/run/php/x.sock"
	d.WAF, d.RateLimit, d.RateBurst, d.RateZone = WAFDetect, 10, 50, "panel_rl_example_com"
	d.BlockedIPs = []string{"203.0.113.7", "198.51.100.0/24", "2001:db8::/32"}
	out, err := renderVhost(d)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	mustContain(t, s,
		"limit_req_zone $binary_remote_addr zone=panel_rl_example_com:1m rate=10r/s;",
		"deny 203.0.113.7;", "deny 198.51.100.0/24;", "deny 2001:db8::/32;",
		"modsecurity on;", "modsecurity_rules 'SecRuleEngine DetectionOnly';",
		"limit_req zone=panel_rl_example_com burst=50 nodelay;", "limit_req_status 429;")
	// The zone is defined at http level, before the server block.
	if strings.Index(s, "limit_req_zone") > strings.Index(s, "server {") {
		t.Fatal("limit_req_zone inside the server block")
	}
	// The limit applies to PHP only: it sits inside the PHP location.
	php := strings.Index(s, `location ~ [^/]\.php(/|$)`)
	if lr := strings.Index(s, "limit_req zone="); lr < php {
		t.Fatal("limit_req outside the PHP location")
	}

	d.WAF = WAFBlock
	out, _ = renderVhost(d)
	if strings.Contains(string(out), "DetectionOnly") {
		t.Fatal("block mode rendered as detection only")
	}

	// App sites: the limit goes into the proxy location.
	a := baseVhost()
	a.ProxyPort, a.RateLimit, a.RateBurst, a.RateZone = 3000, 5, 25, "panel_rl_x"
	out, _ = renderVhost(a)
	mustContain(t, string(out), "limit_req zone=panel_rl_x burst=25 nodelay;")

	for _, bad := range []func(*vhostData){
		func(d *vhostData) { d.WAF = "on; include /etc/passwd" },
		func(d *vhostData) { d.BlockedIPs = []string{"1.2.3.4; allow all"} },
		func(d *vhostData) { d.BlockedIPs = []string{"10.0.0.1/8"} }, // not canonical
		func(d *vhostData) { d.RateLimit, d.RateZone = 5, "x;y" },
		func(d *vhostData) { d.RateLimit = 5000 },
	} {
		d := baseVhost()
		d.RateZone = "panel_rl_ok"
		bad(&d)
		if _, err := renderVhost(d); err == nil {
			t.Errorf("rendered %+v", d)
		}
	}
}

func TestNormalizeIPs(t *testing.T) {
	got, err := normalizeIPs([]string{" 10.0.0.1/8 ", "203.0.113.7", "", "203.0.113.7", "2001:DB8::1"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != "10.0.0.0/8,203.0.113.7,2001:db8::1" {
		t.Fatalf("got %v", got)
	}
	for _, bad := range []string{"1.2.3", "1.2.3.4/33", "example.com", "1.2.3.4;"} {
		if _, err := normalizeIPs([]string{bad}); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestWAFExclusions(t *testing.T) {
	m := &Manager{paths: DefaultPaths(t.TempDir())}
	files := m.wafFiles([]*store.Site{
		{ID: 1, Domain: "blog.example.com", Aliases: []string{"*.blog.example.com"}, App: AppWordPress},
		{ID: 2, Domain: "shop.example.org", Rewrite: "drupal"},
		{ID: 3, Domain: "plain.example.net"},
	})
	main := string(files[filepath.Join(m.wafDir(), "main.conf")])
	mustContain(t, main,
		"SecRuleEngine On", "SecRequestBodyLimitAction ProcessPartial", "SecAuditLogFormat JSON",
		"SecAuditLog "+wafLog(),
		"setvar:tx.crs_exclusions_wordpress=1", "setvar:tx.crs_exclusions_drupal=1",
		"Include /usr/share/modsecurity-crs/rules/*.conf", "logdata:'%{reqbody_error_msg}'")
	if strings.Contains(main, "plain") {
		t.Fatal("a site without an app got exclusions")
	}
	// Exclusions are set before the rules run.
	if strings.Index(main, "crs_exclusions_wordpress") > strings.Index(main, "Include /usr/share/modsecurity-crs/rules") {
		t.Fatal("exclusions come after the rules")
	}
	http := string(files[filepath.Join(m.wafDir(), "http.conf")])
	mustContain(t, http, "modsecurity off;", "modsecurity_rules_file "+filepath.Join(m.wafDir(), "main.conf")+";")

	re := regexp.MustCompile("^(?:" + hostRegex([]string{"blog.example.com", "*.blog.example.com"}) + ")(?::[0-9]+)?$")
	for host, ok := range map[string]bool{
		"blog.example.com": true, "blog.example.com:443": true, "a.blog.example.com": true,
		"blogxexample.com": false, "a.b.blog.example.com": false, "evil.com": false,
	} {
		if re.MatchString(host) != ok {
			t.Errorf("%s matched = %v", host, !ok)
		}
	}
}

func TestParseAudit(t *testing.T) {
	log := `{"transaction":{"client_ip":"203.0.113.9","time_stamp":"Tue Nov 25 10:00:00 2026","request":{"method":"GET","uri":"/?id=1' OR 1=1","headers":{"Host":"shop.example.org"}},"response":{"http_code":403},"messages":[{"message":"SQL Injection Attack Detected via libinjection","details":{"ruleId":"942100"}},{"message":"Inbound Anomaly Score Exceeded (Total Score: 5)","details":{"ruleId":"949110"}}]}}
not json
{"transaction":{"client_ip":"203.0.113.10","time_stamp":"Tue Nov 25 10:01:00 2026","request":{"method":"GET","uri":"/x","headers":{"host":"OTHER.example"}},"response":{"http_code":403},"messages":[]}}
{"transaction":{"client_ip":"203.0.113.11","time_stamp":"Tue Nov 25 10:02:00 2026","request":{"method":"POST","uri":"/login","headers":{"Host":"a.shop.example.org:443"}},"response":{"http_code":200},"messages":[{"message":"XSS Attack Detected via libinjection","details":{"ruleId":"941100"}}]}}
`
	ev := parseAudit(strings.NewReader(log), []string{"shop.example.org", "*.shop.example.org"}, 10)
	if len(ev) != 2 {
		t.Fatalf("got %d events: %+v", len(ev), ev)
	}
	// Newest first, other hosts left out, the anomaly total dropped.
	if ev[0].IP != "203.0.113.11" || ev[0].Status != 200 || ev[0].Rules[0].ID != "941100" {
		t.Fatalf("event 0 = %+v", ev[0])
	}
	if ev[1].URI != "/?id=1' OR 1=1" || len(ev[1].Rules) != 1 || ev[1].Rules[0].ID != "942100" {
		t.Fatalf("event 1 = %+v", ev[1])
	}
	if len(parseAudit(strings.NewReader(log), []string{"shop.example.org", "*.shop.example.org"}, 1)) != 1 {
		t.Fatal("limit not applied")
	}
}
