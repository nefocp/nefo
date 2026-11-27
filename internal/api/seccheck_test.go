package api

import (
	"context"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/seccheck"
	"github.com/nefocp/nefo/internal/security"
	"github.com/nefocp/nefo/internal/wptoolkit"
)

func TestSecurityCheckAPI(t *testing.T) {
	e := newEnv(t)
	e.login(t)
	dir := t.TempDir()
	// The server's facts never come from this machine.
	facts := &seccheck.Facts{
		SSH:      &security.SSH{Port: 22, PasswordAuth: true, PermitRootLogin: "yes", Logins: []security.Login{{User: "root", Password: true}}},
		Firewall: security.Firewall{Installed: true, Active: true},
		Fail2ban: security.Fail2ban{Installed: true, Running: true, Jails: []security.Jail{{Name: "sshd"}}},
		Panel:    seccheck.Panel{TLS: true, Users: []seccheck.User{{Name: "admin", TOTP: true}}},
	}
	secFactsHooks.Store(e.srv, func(context.Context) *seccheck.Facts { return facts })
	secEnvHooks.Store(e.srv, func() seccheck.Env {
		return seccheck.Env{Passwd: dir + "/passwd", Shadow: dir + "/shadow", PHPDir: dir, RedisConf: dir + "/redis.conf"}
	})
	t.Cleanup(func() { secFactsHooks.Delete(e.srv); secEnvHooks.Delete(e.srv) })

	res, out := e.do(t, "GET", "/api/security/check", nil, nil)
	if res.StatusCode != 200 || out["report"] != nil || out["settings"].(map[string]any)["daily"] != true {
		t.Fatalf("before the first run: %d %v", res.StatusCode, out)
	}
	res, out = e.do(t, "POST", "/api/security/check", map[string]any{}, nil)
	if res.StatusCode != 200 {
		t.Fatalf("run: %d %v", res.StatusCode, out)
	}
	counts := out["counts"].(map[string]any)
	if counts["critical"] != 1.0 || counts["info"] != 1.0 {
		t.Fatalf("counts %v", counts)
	}
	res, out = e.do(t, "POST", "/api/security/check/accept", map[string]any{"id": "ssh_root_password", "accepted": true}, nil)
	if res.StatusCode != 200 || out["counts"].(map[string]any)["accepted"] != 1.0 || out["counts"].(map[string]any)["critical"] != 0.0 {
		t.Fatalf("accept: %d %v", res.StatusCode, out)
	}
	res, out = e.do(t, "POST", "/api/security/check/accept", map[string]any{"id": "nope", "accepted": true}, nil)
	if res.StatusCode != 400 || out["error"] != "security_finding_gone" {
		t.Fatalf("unknown finding: %d %v", res.StatusCode, out)
	}
	res, out = e.do(t, "PUT", "/api/security/check/settings", map[string]any{"daily": false, "notify": true}, nil)
	if res.StatusCode != 200 || out["settings"].(map[string]any)["daily"] != false {
		t.Fatalf("settings: %d %v", res.StatusCode, out)
	}
	res, out = e.do(t, "GET", "/api/security/check", nil, nil)
	if res.StatusCode != 200 || out["counts"].(map[string]any)["accepted"] != 1.0 || out["settings"].(map[string]any)["notify"] != true {
		t.Fatalf("read back: %v", out)
	}
}

func TestWPLastChecks(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	s := e.srv
	if s.wpLastChecks(ctx) != nil {
		t.Fatal("checks before any")
	}
	s.rememberWPCheck(ctx, "blog.example.com", &wptoolkit.Status{CoreUpdate: "6.9", PluginUpdates: 2})
	s.rememberWPCheck(ctx, "broken.example.com", &wptoolkit.Status{Error: "database down"})
	m := s.loadWPChecks(ctx)
	if len(m) != 1 || m["blog.example.com"].PluginUpdates != 2 || time.Since(m["blog.example.com"].CheckedAt) > time.Minute {
		t.Fatalf("remembered %v", m)
	}
	// Not a WordPress site of the panel (no such site): left out.
	if s.wpLastChecks(ctx) != nil {
		t.Fatal("a check of an unknown site")
	}
	s.rememberWPCheck(ctx, "blog.example.com", nil)
	if len(s.loadWPChecks(ctx)) != 0 {
		t.Fatal("not forgotten")
	}
}

func TestSecCheckAlert(t *testing.T) {
	text := secCheckAlert([]seccheck.Finding{
		{Check: "ssh_root_password", Severity: seccheck.Critical},
		{Check: "site_no_https", Severity: seccheck.Warning, Params: map[string]string{"domain": "example.com"}},
	})
	want := "🛡 The daily security check found 2 new problem(s):\n- [critical] root can sign in over SSH with a password\n- [warning] example.com has no HTTPS\nSee Security check in the panel."
	if text != want {
		t.Fatalf("got %q", text)
	}
}
