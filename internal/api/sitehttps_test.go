package api

import (
	"context"
	"strconv"
	"testing"

	"github.com/nefocp/nefo/internal/store"
)

// The routes refuse before changing anything: on a test machine the panel
// either cannot manage websites (409 unsupported) or the site lacks what
// the setting needs (400).
func TestSiteHTTPSRoutes(t *testing.T) {
	e := newEnv(t)
	e.login(t)
	site := &store.Site{Domain: "plain.example", SysUser: "plain_example"}
	if err := e.store.CreateSite(context.Background(), site); err != nil {
		t.Fatal(err)
	}
	base := "/api/sites/" + strconv.FormatInt(site.ID, 10)

	res, out := e.do(t, "GET", "/api/sites", nil, nil)
	sites, _ := out["sites"].([]any)
	if res.StatusCode != 200 || len(sites) != 1 {
		t.Fatalf("list: %d %v", res.StatusCode, out)
	}
	for _, k := range []string{"hsts", "hsts_subdomains", "page_cache"} {
		if v, ok := sites[0].(map[string]any)[k]; !ok || v != false {
			t.Errorf("site JSON %s = %v (present %v)", k, v, ok)
		}
	}

	for _, c := range []struct {
		method, path string
		body         any
		code         string
	}{
		{"PUT", base + "/hsts", map[string]bool{"enabled": true}, "hsts_needs_redirect"},
		{"PUT", base + "/page-cache", map[string]bool{"enabled": true}, ""}, // nginx_only or needs_php
		{"POST", base + "/page-cache/purge", map[string]bool{}, "page_cache_off"},
	} {
		res, out := e.do(t, c.method, c.path, c.body, nil)
		switch {
		case res.StatusCode == 409 && out["error"] == "unsupported":
		case res.StatusCode == 400 && (c.code == "" || out["error"] == c.code):
		default:
			t.Errorf("%s %s: %d %v", c.method, c.path, res.StatusCode, out)
		}
	}
	for _, c := range [][2]string{{"DELETE", "/api/sites/x/ssl"}, {"PUT", "/api/sites/0/hsts"}, {"PUT", "/api/sites/x/page-cache"}, {"POST", "/api/sites/x/page-cache/purge"}} {
		if res, _ := e.do(t, c[0], c[1], map[string]bool{}, nil); res.StatusCode != 404 {
			t.Errorf("%s %s: %d, want 404", c[0], c[1], res.StatusCode)
		}
	}
}
