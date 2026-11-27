package api

import (
	"context"
	"testing"

	"github.com/nefocp/nefo/internal/hosting"
)

// The e-mail helper's routes need a session and check the zone ID and the
// request before they ask Cloudflare anything (the hosting package tests
// the plans against a fake Cloudflare).
func TestMailDNSRoutes(t *testing.T) {
	e := newEnv(t)
	if res, _ := e.do(t, "POST", "/api/dns/zones/0000000000000000000000000000c0de/mail/plan", map[string]any{"provider": "google"}, nil); res.StatusCode != 401 {
		t.Fatalf("without a session: %d", res.StatusCode)
	}
	e.login(t)
	if res, out := e.do(t, "POST", "/api/dns/zones/0000000000000000000000000000c0de/mail/plan", map[string]any{"provider": "google"}, nil); res.StatusCode != 400 || out["error"] != "cloudflare_not_configured" {
		t.Fatalf("without a token: %d %v", res.StatusCode, out)
	}
	if err := e.store.SetSetting(context.Background(), hosting.CloudflareTokenKey, "token"); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/dns/zones/not-an-id/mail/plan", "/api/dns/zones/..%2fuser/mail/apply"} {
		if res, out := e.do(t, "POST", path, map[string]any{"provider": "google", "hash": "x"}, nil); res.StatusCode != 404 && res.StatusCode != 400 {
			t.Errorf("%s: %d %v", path, res.StatusCode, out)
		}
	}
	if res, out := e.do(t, "POST", "/api/dns/zones/not-an-id/mail/plan", map[string]any{"provider": "google"}, nil); res.StatusCode != 404 {
		t.Errorf("bad zone id: %d %v", res.StatusCode, out)
	}
	if res, out := e.do(t, "POST", "/api/dns/zones/0000000000000000000000000000c0de/mail/plan", map[string]any{"provider": "google", "extra": 1}, nil); res.StatusCode != 400 {
		t.Fatalf("unknown field: %d %v", res.StatusCode, out)
	}
}
