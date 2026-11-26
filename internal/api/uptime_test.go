package api

import (
	"context"
	"testing"

	"github.com/nefocp/nefo/internal/store"
)

// Monitoring routes. The settings saved here keep monitoring off: turning
// it on checks the website at once, through DNS (see internal/uptime for
// the checks, against local servers).
func TestSiteMonitorRoutes(t *testing.T) {
	e := newEnv(t)
	e.login(t)
	site := &store.Site{Domain: "example.com", SysUser: "example_com"}
	if err := e.store.CreateSite(context.Background(), site); err != nil {
		t.Fatal(err)
	}
	path := "/api/sites/" + itoa(int(site.ID)) + "/monitor"

	res, out := e.do(t, "GET", path, nil, nil)
	settings, _ := out["settings"].(map[string]any)
	if res.StatusCode != 200 || settings["enabled"] != false || settings["interval"] != 5.0 || settings["path"] != "/" || out["status"] != nil {
		t.Fatalf("defaults: %d %v", res.StatusCode, out)
	}
	for body, code := range map[string]map[string]any{
		"invalid_monitor_interval": {"enabled": true, "interval": 3},
		"invalid_monitor_path":     {"enabled": true, "interval": 5, "path": "no-slash"},
		"invalid_monitor_keyword":  {"enabled": true, "interval": 5, "keyword": "a\nb"},
	} {
		if res, out := e.do(t, "PUT", path, code, nil); res.StatusCode != 400 || out["error"] != body {
			t.Errorf("%v: %d %v", code, res.StatusCode, out)
		}
	}
	res, out = e.do(t, "PUT", path, map[string]any{"enabled": false, "interval": 2, "path": "/health", "keyword": "OK", "strict_4xx": true}, nil)
	settings, _ = out["settings"].(map[string]any)
	if res.StatusCode != 200 || settings["interval"] != 2.0 || settings["path"] != "/health" || settings["keyword"] != "OK" || settings["strict_4xx"] != true {
		t.Fatalf("save: %d %v", res.StatusCode, out)
	}
	if res, out := e.do(t, "PUT", "/api/sites/999/monitor", map[string]any{"interval": 5}, nil); res.StatusCode != 404 {
		t.Fatalf("unknown site: %d %v", res.StatusCode, out)
	}
	res, out = e.do(t, "GET", "/api/uptime", nil, nil)
	if sites, ok := out["sites"].([]any); res.StatusCode != 200 || !ok || len(sites) != 0 {
		t.Fatalf("summary: %d %v", res.StatusCode, out)
	}
}
