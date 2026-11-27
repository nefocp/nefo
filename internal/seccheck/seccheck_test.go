package seccheck

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/docker"
	"github.com/nefocp/nefo/internal/procs"
	"github.com/nefocp/nefo/internal/security"
	"github.com/nefocp/nefo/internal/systools"
)

var now = time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)

func write(t *testing.T, path, text string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

// goodFacts is a well set up server: every check passes.
func goodFacts(dir string) *Facts {
	checked := now.Add(-time.Hour)
	return &Facts{
		SSH:      &security.SSH{Port: 2222, PasswordAuth: false, PermitRootLogin: "prohibit-password", Logins: []security.Login{{User: "root", Key: true}}},
		Firewall: security.Firewall{Installed: true, Active: true, Rules: []security.Rule{{To: "2222/tcp", Action: "ALLOW", From: "Anywhere"}}},
		Fail2ban: security.Fail2ban{Installed: true, Running: true, Jails: []security.Jail{{Name: "sshd"}, {Name: "panel"}}},
		Updates:  &systools.Updates{CheckedAt: &checked, Auto: systools.Auto{Installed: true, Enabled: true}},
		Ports: []procs.Port{
			{Proto: "tcp", Address: "127.0.0.1", Port: 3306, Process: "mariadbd", Local: true},
			{Proto: "tcp", Address: "127.0.0.1", Port: 6379, Process: "redis-server", Local: true},
			{Proto: "tcp", Address: "0.0.0.0", Port: 80, Process: "nginx"},
		},
		Docker: []docker.Container{{Name: "db", State: "running", Ports: "127.0.0.1:5432->5432/tcp"}},
		Panel:  Panel{TLS: true, Users: []User{{Name: "admin", TOTP: true}}},
		Sites: []Site{{ID: 1, Domain: "example.com", Dir: filepath.Join(dir, "www", "example.com"), SSL: true, ForceHTTPS: true,
			CertExpiry: now.Add(60 * 24 * time.Hour)}},
		WordPress: map[string]WordPress{"example.com": {CheckedAt: now.Add(-time.Hour)}},
	}
}

func goodEnv(t *testing.T, dir string) Env {
	write(t, filepath.Join(dir, "passwd"), "root:x:0:0:root:/root:/bin/bash\nwww-data:x:33:33::/var/www:/usr/sbin/nologin\n", 0o644)
	write(t, filepath.Join(dir, "shadow"), "root:$y$j9T$abc:19000:0:99999:7:::\nwww-data:*:19000:0:99999:7:::\n", 0o600)
	write(t, filepath.Join(dir, "php", "8.3", "fpm", "php.ini"), "[PHP]\nexpose_php = On\ndisplay_errors = Off\n", 0o644)
	write(t, filepath.Join(dir, "php", "8.3", "fpm", "conf.d", "99-panel.ini"), "expose_php = Off\n", 0o644)
	write(t, filepath.Join(dir, "redis.conf"), "bind 127.0.0.1 ::1\nrequirepass \"s3cret-value\"\n", 0o640)
	write(t, filepath.Join(dir, "www", "example.com", "public", "index.php"), "<?php echo 1;", 0o644)
	return Env{Now: now, Passwd: filepath.Join(dir, "passwd"), Shadow: filepath.Join(dir, "shadow"), PHPDir: filepath.Join(dir, "php"),
		RedisConf: filepath.Join(dir, "redis.conf"), Probe: func(context.Context, string, string) (int, error) { return 404, nil }}
}

func ids(r *Report) []string {
	var out []string
	for _, f := range r.Findings {
		out = append(out, f.ID+"="+f.Severity)
	}
	sort.Strings(out)
	return out
}

func TestAllPass(t *testing.T) {
	dir := t.TempDir()
	rep := Run(context.Background(), goodFacts(dir), goodEnv(t, dir))
	if len(rep.Findings) != 0 {
		t.Fatalf("findings on a good server: %v", ids(rep))
	}
	skipped := 0
	if !unixPerms {
		skipped = 1 // site_permissions
	}
	if len(rep.Passed) != len(checks)-skipped || len(rep.Skipped) != skipped {
		t.Fatalf("passed %v, skipped %v", rep.Passed, rep.Skipped)
	}
	if n := len(Keys()); n < 20 {
		t.Fatalf("only %d checks", n)
	}
}

func TestSkipped(t *testing.T) {
	dir := t.TempDir()
	env := goodEnv(t, dir)
	env.Shadow = filepath.Join(dir, "missing")
	env.PHPDir = filepath.Join(dir, "missing")
	env.RedisConf = filepath.Join(dir, "missing")
	f := &Facts{Panel: Panel{TLS: true, Users: []User{{Name: "admin", TOTP: true}}}, Firewall: security.Firewall{Installed: true, Active: true},
		Fail2ban: security.Fail2ban{Installed: true, Running: true}}
	rep := Run(context.Background(), f, env)
	want := []string{"ssh_root", "ssh_password", "ssh_port", "os_updates", "os_auto_updates", "os_reboot", "accounts_empty_password",
		"public_services", "redis_password", "docker_ports", "site_https", "site_certs", "site_files", "site_permissions",
		"php_display_errors", "php_expose", "wordpress"}
	if !slices.Equal(rep.Skipped, want) {
		t.Fatalf("skipped %v", rep.Skipped)
	}
	if len(rep.Findings) != 0 {
		t.Fatalf("findings %v", ids(rep))
	}
}

func TestBadServer(t *testing.T) {
	dir := t.TempDir()
	env := goodEnv(t, dir)
	f := goodFacts(dir)
	f.SSH = &security.SSH{Port: 22, PasswordAuth: true, PermitRootLogin: "yes",
		Logins: []security.Login{{User: "root", Password: true}, {User: "deploy", Key: true, Password: true}}}
	f.Firewall = security.Firewall{Installed: true, Active: false}
	f.Fail2ban = security.Fail2ban{Installed: true, Running: true, Jails: []security.Jail{{Name: "panel"}}}
	f.Panel = Panel{TLS: false, Users: []User{{Name: "admin"}, {Name: "ops", TOTP: true}}}
	old := now.Add(-30 * 24 * time.Hour)
	f.Updates = &systools.Updates{CheckedAt: &old, Security: 3, Reboot: systools.Reboot{Required: true}}
	f.Ports = append(f.Ports,
		procs.Port{Proto: "tcp", Address: "0.0.0.0", Port: 6380, Process: "redis-server"},
		procs.Port{Proto: "tcp", Address: "::", Port: 6380, Process: "redis-server"},
		procs.Port{Proto: "tcp", Address: "10.0.0.5", Port: 5432, Process: "postgres"},
		procs.Port{Proto: "udp", Address: "0.0.0.0", Port: 11211, Process: "memcached"},
	)
	f.Docker = []docker.Container{
		{Name: "web", State: "running", Ports: "0.0.0.0:8080->80/tcp, [::]:8080->80/tcp, 0.0.0.0:8443->443/tcp"},
		{Name: "old", State: "exited", Ports: "0.0.0.0:9000->9000/tcp"},
	}
	f.Sites = append(f.Sites,
		Site{ID: 2, Domain: "plain.example", Dir: filepath.Join(dir, "www", "plain.example")},
		Site{ID: 3, Domain: "noredir.example", Dir: filepath.Join(dir, "www", "noredir.example"), SSL: true, CertExpiry: now.Add(-time.Hour)},
		Site{ID: 4, Domain: "soon.example", Dir: filepath.Join(dir, "www", "soon.example"), SSL: true, ForceHTTPS: true, CertExpiry: now.Add(5 * 24 * time.Hour), CustomCert: true},
		Site{ID: 5, Domain: "off.example", Dir: filepath.Join(dir, "www", "off.example"), Disabled: true},
	)
	f.WordPress = map[string]WordPress{"example.com": {CheckedAt: now, CoreUpdate: "6.9", PluginUpdates: 2, Debug: true}, "gone.example": {PluginUpdates: 1}}
	write(t, env.Passwd, "root:x:0:0:root:/root:/bin/bash\ntoor:x:0:0::/root:/bin/sh\n", 0o644)
	write(t, env.Shadow, "root:$y$j9T$abc:19000::::::\nguest::19000::::::\n", 0o600)
	write(t, env.RedisConf, "bind 0.0.0.0\n# requirepass x\n", 0o640)
	write(t, filepath.Join(env.PHPDir, "8.3", "fpm", "php.ini"), "display_errors = stdout ; shown\n", 0o644)
	write(t, filepath.Join(env.PHPDir, "8.3", "fpm", "conf.d", "99-panel.ini"), "max_input_vars = 3000\n", 0o644)
	write(t, filepath.Join(env.PHPDir, "8.2", "fpm", "php.ini"), "display_errors = Off\nexpose_php = Off\n", 0o644)
	write(t, filepath.Join(env.PHPDir, "8.2", "fpm", "pool.d", "panel-shop.conf"), "[shop]\nphp_admin_flag[display_errors] = on\n", 0o644)

	if unixPerms {
		write(t, filepath.Join(dir, "www", "example.com", "public", "upload.php"), "x", 0o666)
	}
	rep := Run(context.Background(), f, env)
	want := []string{
		"docker_public:web=warning",
		"empty_password=critical",
		"fail2ban_no_sshd=warning",
		"firewall_off=warning",
		"os_auto_updates_off=warning",
		"os_reboot=info",
		"os_updates_unknown=info",
		"panel_2fa=critical",
		"panel_http=critical",
		"php_display_errors:8.2=warning",
		"php_display_errors:8.3=warning",
		"php_expose:8.3=info",
		"redis_no_password=critical",
		"service_public:memcached:11211/udp=critical",
		"service_public:postgres:5432/tcp=info",
		"service_public:redis-server:6380/tcp=critical",
		"site_cert_expired:noredir.example=critical",
		"site_cert_expiring:soon.example=warning",
		"site_no_https:plain.example=warning",
		"site_no_redirect:noredir.example=info",
		"site_world_writable:example.com=warning",
		"ssh_password=warning",
		"ssh_port_22=info",
		"ssh_root_password=critical",
		"uid0=critical",
		"wp_debug:example.com=warning",
		"wp_updates:example.com=warning",
	}
	if !unixPerms {
		want = slices.DeleteFunc(want, func(s string) bool { return strings.HasPrefix(s, "site_world_writable") })
	}
	if got := ids(rep); !slices.Equal(got, want) {
		t.Fatalf("findings:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if rep.Findings[0].Severity != Critical || rep.Findings[len(rep.Findings)-1].Severity != Info {
		t.Fatal("not sorted by severity")
	}
	by := map[string]Finding{}
	for _, x := range rep.Findings {
		by[x.ID] = x
	}
	if p := by["service_public:redis-server:6380/tcp"].Params; p["address"] != "0.0.0.0, ::" || p["exposure"] != "open" || p["service"] != "Redis" {
		t.Fatalf("redis params %v", p)
	}
	if p := by["docker_public:web"].Params; p["ports"] != "8080, 8443" {
		t.Fatalf("docker ports %v", p)
	}
	if p := by["php_display_errors:8.2"].Params; p["scope"] != "shop" {
		t.Fatalf("pool scope %v", p)
	}
	if p := by["php_display_errors:8.3"].Params; p["scope"] != "all" {
		t.Fatalf("global scope %v", p)
	}
	if p := by["ssh_password"].Params; p["users"] != "deploy" {
		t.Fatalf("ssh users %v", p)
	}
	if p := by["uid0"].Params; p["users"] != "toor" {
		t.Fatalf("uid0 %v", p)
	}
	if p := by["empty_password"].Params; p["users"] != "guest" {
		t.Fatalf("empty %v", p)
	}
	if x := by["site_cert_expiring:soon.example"]; x.Extra["days"] != "5" || x.Params["custom"] != "true" {
		t.Fatalf("expiring %+v", x)
	}
	if fix := by["site_no_https:plain.example"].Fix; fix.Page != "websites" || fix.Query["site"] != "2" || fix.Query["tab"] != "ssl" {
		t.Fatalf("fix %+v", fix)
	}
	if fix := by["php_expose:8.3"].Fix; fix.Page != "software" || fix.Query["php"] != "8.3" {
		t.Fatalf("php fix %+v", fix)
	}
}

func TestExposure(t *testing.T) {
	dir := t.TempDir()
	env := goodEnv(t, dir)
	f := goodFacts(dir)
	f.Ports = []procs.Port{{Proto: "tcp", Address: "0.0.0.0", Port: 3306, Process: "mariadbd"}}
	sev := func() (string, string) {
		rep := Run(context.Background(), f, env)
		for _, x := range rep.Findings {
			if x.Check == "service_public" {
				return x.Severity, x.Params["exposure"]
			}
		}
		return "", ""
	}
	// ufw on, no rule for 3306: blocked.
	if s, e := sev(); s != Info || e != "blocked" {
		t.Fatalf("blocked: %s %s", s, e)
	}
	f.Firewall.Rules = append(f.Firewall.Rules, security.Rule{To: "3306/tcp", Action: "ALLOW", From: "198.51.100.7"})
	if s, e := sev(); s != Info || e != "limited" {
		t.Fatalf("limited: %s %s", s, e)
	}
	f.Firewall.Rules = append(f.Firewall.Rules, security.Rule{To: "3306/tcp (v6)", Action: "ALLOW", From: "Anywhere (v6)", V6: true})
	if s, e := sev(); s != Critical || e != "open" {
		t.Fatalf("open: %s %s", s, e)
	}
	f.Firewall = security.Firewall{Installed: true, Active: false}
	if s, e := sev(); s != Critical || e != "open" {
		t.Fatalf("ufw off: %s %s", s, e)
	}
}

func TestSiteFiles(t *testing.T) {
	dir := t.TempDir()
	env := goodEnv(t, dir)
	pub := filepath.Join(dir, "www", "example.com", "public")
	write(t, filepath.Join(pub, ".env"), "DB_PASSWORD=x\n", 0o644)
	write(t, filepath.Join(pub, ".git", "HEAD"), "ref: refs/heads/main\n", 0o644)
	write(t, filepath.Join(pub, "backup-2026.sql.gz"), "x", 0o644)
	write(t, filepath.Join(pub, "example-site.zip"), "x", 0o644)
	write(t, filepath.Join(pub, "brochure.zip"), "x", 0o644)  // a normal download
	write(t, filepath.Join(pub, "wp-config.php"), "x", 0o644) // served as PHP, never as text
	write(t, filepath.Join(pub, "style.css.bak"), "x", 0o644) // a copy
	var asked []string
	// The web server hides dot files (like the panel's Nginx) and serves the rest.
	env.Probe = func(_ context.Context, domain, path string) (int, error) {
		asked = append(asked, domain+path)
		if strings.HasPrefix(path, "/.") {
			return 403, nil
		}
		return 206, nil
	}
	f := goodFacts(dir)
	rep := Run(context.Background(), f, env)
	sort.Strings(asked)
	wantAsked := []string{"example.com/.env", "example.com/.git/HEAD", "example.com/backup-2026.sql.gz", "example.com/example-site.zip", "example.com/style.css.bak"}
	if !slices.Equal(asked, wantAsked) {
		t.Fatalf("asked %v", asked)
	}
	if len(rep.Findings) != 1 || rep.Findings[0].ID != "site_exposed:example.com" || rep.Findings[0].Severity != Critical {
		t.Fatalf("findings %v", ids(rep))
	}
	if got := rep.Findings[0].Params["files"]; got != "/backup-2026.sql.gz, /example-site.zip, /style.css.bak" {
		t.Fatalf("files %q", got)
	}
	if fix := rep.Findings[0].Fix; fix.Page != "files" || fix.Query["path"] != pub {
		t.Fatalf("fix %+v", fix)
	}

	// Only archives and copies: a warning. Every probe counts toward the limit.
	os.Remove(filepath.Join(pub, "backup-2026.sql.gz"))
	env.Probe = func(_ context.Context, _, path string) (int, error) { return 200, nil } // dot files served too (Apache without rules)
	rep = Run(context.Background(), f, env)
	if len(rep.Findings) != 1 || rep.Findings[0].Severity != Critical || !strings.Contains(rep.Findings[0].Params["files"], "/.env") {
		t.Fatalf("dot files served: %v %v", ids(rep), rep.Findings)
	}
	os.Remove(filepath.Join(pub, ".env"))
	os.RemoveAll(filepath.Join(pub, ".git"))
	rep = Run(context.Background(), f, env)
	if len(rep.Findings) != 1 || rep.Findings[0].Severity != Warning {
		t.Fatalf("archives: %v", ids(rep))
	}
	env.ProbeLimit = 1
	rep = Run(context.Background(), f, env)
	if !slices.Contains(rep.Partial, "site_files") {
		t.Fatalf("partial %v", rep.Partial)
	}
	// A site behind a password, a suspended site: not probed.
	f.Sites[0].Password = true
	env.ProbeLimit = 0
	rep = Run(context.Background(), f, env)
	if len(rep.Findings) != 0 {
		t.Fatalf("password site probed: %v", ids(rep))
	}
}

func TestWorldWritable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permissions")
	}
	dir := t.TempDir()
	env := goodEnv(t, dir)
	site := filepath.Join(dir, "www", "example.com")
	write(t, filepath.Join(site, "public", "upload.php"), "x", 0o666)
	write(t, filepath.Join(site, "public", "a", "b.txt"), "x", 0o646)
	write(t, filepath.Join(site, "public", "ok.txt"), "x", 0o644)
	os.MkdirAll(filepath.Join(site, "tmp"), 0o755)
	os.Chmod(filepath.Join(site, "tmp"), 0o777|os.ModeSticky) // sticky: fine
	os.MkdirAll(filepath.Join(site, "cache"), 0o755)
	os.Chmod(filepath.Join(site, "cache"), 0o777)
	os.Symlink("/etc", filepath.Join(site, "public", "etc")) // never followed
	rep := Run(context.Background(), goodFacts(dir), env)
	if len(rep.Findings) != 1 || rep.Findings[0].ID != "site_world_writable:example.com" {
		t.Fatalf("findings %v", ids(rep))
	}
	x := rep.Findings[0]
	if x.Params["count"] != "3" || x.Extra["examples"] != "cache, public/a/b.txt, public/upload.php" {
		t.Fatalf("got %v %v", x.Params, x.Extra)
	}
	env.WalkLimit = 3
	rep = Run(context.Background(), goodFacts(dir), env)
	if !slices.Contains(rep.Partial, "site_permissions") {
		t.Fatalf("partial %v", rep.Partial)
	}
}

func TestRedisPassword(t *testing.T) {
	for conf, want := range map[string]bool{
		"requirepass foo\n":                        true,
		"requirepass \"\"\n":                       false,
		"# requirepass foo\n":                      false,
		"user default on >secret ~* +@all\n":       true,
		"user default on nopass ~* +@all\n":        false,
		"requirepass a\nuser default on nopass\n":  false,
		"user other on >x\n":                       false,
		"bind 0.0.0.0\nprotected-mode no\n":        false,
		"requirepass 'with space'\nport 6379\n":    true,
		"REQUIREPASS upper\n":                      true,
		"  requirepass   indented  # comment ok\n": true,
	} {
		if got := redisPassword(conf); got != want {
			t.Errorf("%q: %v", conf, got)
		}
	}
}

func TestFileKind(t *testing.T) {
	for name, want := range map[string]string{
		".env": "secret", ".env.production": "secret", ".htpasswd": "secret", "dump.sql": "secret", "db.sqlite": "secret",
		"wp-config.php": "", "wp-config-sample.php": "", "wp-config.php.bak": "secret", "wp-config.old": "secret",
		"index.php": "", "logo.png": "", "index.php~": "backup", "config.php.save": "backup", "phpinfo.php": "backup",
		"site-backup.tar.gz": "backup", "example.zip": "backup", "wordpress-6.8.zip": "backup", "manual.zip": "", "fonts.tar.gz": "",
	} {
		if got := fileKind(name, "example.com", false); got != want {
			t.Errorf("%s: %q, want %q", name, got, want)
		}
	}
	if fileKind(".git", "example.com", true) != "secret" || fileKind("assets", "example.com", true) != "" {
		t.Error("folders")
	}
}

func TestPublishedPorts(t *testing.T) {
	for in, want := range map[string]string{
		"0.0.0.0:8080->80/tcp, :::8080->80/tcp":       "8080",
		"0.0.0.0:53->53/udp, 0.0.0.0:53->53/tcp":      "53",
		"127.0.0.1:5432->5432/tcp, 80/tcp":            "",
		"[::]:9443->9443/tcp, 0.0.0.0:9443->9443/tcp": "9443",
		"10.0.0.2:80->80/tcp":                         "",
		"":                                            "",
	} {
		if got := strings.Join(publishedPorts(in), ", "); got != want {
			t.Errorf("%q: %q", in, got)
		}
	}
}

// memStore is the settings table in memory.
type memStore map[string]string

func (m memStore) Setting(_ context.Context, k string) (string, error) { return m[k], nil }
func (m memStore) SetSetting(_ context.Context, k, v string) error {
	if v == "" {
		delete(m, k)
	} else {
		m[k] = v
	}
	return nil
}

func TestService(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	env := goodEnv(t, dir)
	st := memStore{}
	svc := NewService(st)
	if rep, err := svc.Last(ctx); rep != nil || err != nil {
		t.Fatalf("before the first run: %v %v", rep, err)
	}
	if set, _ := svc.Settings(ctx); !set.Daily || set.Notify {
		t.Fatalf("default settings %+v", set)
	}
	if !svc.Due(ctx, now) {
		t.Fatal("not due before the first run")
	}
	f := goodFacts(dir)
	f.Firewall.Active = false
	f.SSH.Port = 22
	facts := func(context.Context) *Facts { return f }

	rep, fresh, err := svc.Run(ctx, facts, env)
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(rep); !slices.Equal(got, []string{"firewall_off=warning", "ssh_port_22=info"}) {
		t.Fatalf("findings %v", got)
	}
	if len(fresh) != 1 || fresh[0].ID != "firewall_off" {
		t.Fatalf("fresh %v", fresh) // info findings are never news
	}
	if svc.Due(ctx, now.Add(time.Hour)) || !svc.Due(ctx, now.Add(24*time.Hour)) {
		t.Fatal("due")
	}

	// Accepted: hidden and counted apart, the same next run, nothing new.
	rep, err = svc.Accept(ctx, "firewall_off", true)
	if err != nil || !rep.Findings[0].Accepted || rep.Counts() != (Counts{Info: 1, Accepted: 1}) {
		t.Fatalf("accept: %v %+v", err, rep.Counts())
	}
	rep, fresh, _ = svc.Run(ctx, facts, env)
	if c := rep.Counts(); c != (Counts{Info: 1, Accepted: 1}) || len(fresh) != 0 {
		t.Fatalf("after accept: %+v %v", c, fresh)
	}
	last, _ := svc.Last(ctx)
	if !last.Findings[0].Accepted {
		t.Fatal("Last lost the accepted mark")
	}
	if _, err := svc.Accept(ctx, "nope", true); err == nil {
		t.Fatal("accepted an unknown finding")
	}

	// The facts change: the finding shows again, and is news.
	f.Fail2ban.Running = false
	f.Firewall.Installed = false
	rep, fresh, _ = svc.Run(ctx, facts, env)
	if c := rep.Counts(); c.Warning != 2 || c.Accepted != 0 {
		t.Fatalf("changed: %+v %v", c, ids(rep))
	}
	if len(fresh) != 2 {
		t.Fatalf("fresh %v", fresh)
	}

	// Accept, fix, break again: shows again.
	svc.Accept(ctx, "fail2ban_stopped", true)
	f.Fail2ban.Running = true
	svc.Run(ctx, facts, env)
	f.Fail2ban.Running = false
	rep, _, _ = svc.Run(ctx, facts, env)
	for _, x := range rep.Findings {
		if x.ID == "fail2ban_stopped" && x.Accepted {
			t.Fatal("a problem that came back stayed hidden")
		}
	}

	// Showing again.
	svc.Accept(ctx, "ssh_port_22", true)
	rep, _ = svc.Accept(ctx, "ssh_port_22", false)
	if rep.Counts().Accepted != 0 {
		t.Fatal("not shown again")
	}

	// One run at a time.
	block := make(chan struct{})
	started := make(chan struct{})
	go svc.Run(ctx, func(context.Context) *Facts { close(started); <-block; return f }, env)
	<-started
	if _, _, err := svc.Run(ctx, facts, env); err != ErrRunning || !svc.Running() {
		t.Fatalf("second run: %v", err)
	}
	close(block)
	for svc.Running() {
		time.Sleep(time.Millisecond)
	}
	svc.SaveSettings(ctx, Settings{Daily: false, Notify: true})
	if set, _ := svc.Settings(ctx); set.Daily || !set.Notify || svc.Due(ctx, now.Add(48*time.Hour)) {
		t.Fatalf("settings %+v", set)
	}
}

func TestCancelled(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rep := Run(ctx, goodFacts(dir), goodEnv(t, dir))
	if len(rep.Partial) != len(checks) {
		t.Fatalf("partial %v", rep.Partial)
	}
}
