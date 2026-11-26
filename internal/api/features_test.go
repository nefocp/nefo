package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/store"
)

func TestFeatures(t *testing.T) {
	env := newEnv(t)
	ctx := context.Background()
	get := func() map[string]any {
		t.Helper()
		w := httptest.NewRecorder()
		env.srv.handleFeatures(w, httptest.NewRequest(http.MethodGet, "/api/features", nil), &authed{user: &store.User{Username: "admin"}})
		if w.Code != http.StatusOK {
			t.Fatalf("status %d: %s", w.Code, w.Body)
		}
		var f map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &f); err != nil {
			t.Fatal(err)
		}
		return f
	}
	f := get()
	for _, k := range []string{"docker", "redis", "mariadb", "node", "python", "wordpress_sites"} {
		if _, ok := f[k].(bool); !ok {
			t.Errorf("%s missing or not a boolean: %v", k, f)
		}
	}
	if _, ok := f["web_server"].(string); !ok || f["wordpress_sites"] != false {
		t.Fatalf("features %v", f)
	}
	site := &store.Site{Domain: "blog.test", SysUser: "blogtest", PHPVersion: "8.3"}
	if err := env.store.CreateSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	site.App = "wordpress"
	if err := env.store.UpdateSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	if f := get(); f["wordpress_sites"] != true {
		t.Fatalf("a WordPress site is not seen: %v", f)
	}
}

func TestSoftwareRemoveRefusals(t *testing.T) {
	env := newEnv(t)
	a := &authed{user: &store.User{Username: "admin"}}
	post := func(id, body string) (int, map[string]string) {
		t.Helper()
		r := httptest.NewRequest(http.MethodPost, "/api/software/"+id+"/remove", strings.NewReader(body))
		r.SetPathValue("id", id)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		env.srv.handleSoftwareRemove(w, r, a)
		var res map[string]string
		json.Unmarshal(w.Body.Bytes(), &res)
		return w.Code, res
	}
	if code, res := post("nginx", "{}"); code != http.StatusBadRequest || res["error"] != "unknown_software" {
		t.Fatalf("unknown item: %d %v", code, res)
	}
	if code, res := post("docker", `{"delete_data": true, "wipe": true}`); code != http.StatusBadRequest || res["error"] != "bad_request" {
		t.Fatalf("unknown field: %d %v", code, res)
	}
	// No job may start for a refused removal.
	if jobs := env.srv.jobs.Recent(); len(jobs) != 0 {
		t.Fatalf("jobs started: %v", jobs)
	}
	for id, want := range map[string]string{"php8.3": "PHP 8.3", "mariadb": "MariaDB", "nodejs": "Node.js"} {
		if name, ok := softwareName(id); !ok || name != want {
			t.Errorf("%s: %q %v", id, name, ok)
		}
	}
	if _, ok := softwareName("php9.9"); ok {
		t.Error("php9.9 accepted")
	}
}
