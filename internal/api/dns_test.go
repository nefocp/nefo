package api

import (
	"context"
	"testing"

	"github.com/nefocp/nefo/internal/hosting"
)

// The DNS routes need a session and a token, and check IDs before they ask
// Cloudflare anything (the hosting package tests the Cloudflare side).
func TestDNSRoutes(t *testing.T) {
	e := newEnv(t)
	if res, _ := e.do(t, "GET", "/api/dns/zones", nil, nil); res.StatusCode != 401 {
		t.Fatalf("without a session: %d", res.StatusCode)
	}
	e.login(t)
	if res, out := e.do(t, "GET", "/api/dns/zones", nil, nil); res.StatusCode != 400 || out["error"] != "cloudflare_not_configured" {
		t.Fatalf("without a token: %d %v", res.StatusCode, out)
	}
	if res, out := e.do(t, "POST", "/api/sites/1/dns/plan", map[string]any{}, nil); res.StatusCode != 404 {
		t.Fatalf("plan for a missing site: %d %v", res.StatusCode, out)
	}
	if err := e.store.SetSetting(context.Background(), hosting.CloudflareTokenKey, "token"); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ method, path string }{
		{"GET", "/api/dns/zones/not-an-id/records"},
		{"DELETE", "/api/dns/zones/0000000000000000000000000000c0de/records/x"},
		{"PUT", "/api/dns/zones/0000000000000000000000000000c0de/records/..%2f..%2fuser"},
	} {
		var body any
		if c.method == "PUT" {
			body = map[string]any{"type": "A", "name": "www", "content": "203.0.113.10"}
		}
		if res, out := e.do(t, c.method, c.path, body, nil); res.StatusCode != 404 {
			t.Errorf("%s %s: %d %v", c.method, c.path, res.StatusCode, out)
		}
	}
	if res, out := e.do(t, "POST", "/api/dns/zones/0000000000000000000000000000c0de/purge", map[string]any{"everything": true, "extra": 1}, nil); res.StatusCode != 400 {
		t.Fatalf("unknown field: %d %v", res.StatusCode, out)
	}
}
