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

func TestSoftwareToolsAndOldPHPNames(t *testing.T) {
	for id, want := range map[string]string{"php5.6": "PHP 5.6", "php7.3": "PHP 7.3", "composer": "Composer", "wpcli": "WP-CLI"} {
		if name, ok := softwareName(id); !ok || name != want {
			t.Errorf("%s: %q %v", id, name, ok)
		}
	}
	if _, ok := softwareName("php5.5"); ok {
		t.Error("php5.5 accepted")
	}

	env := newEnv(t)
	ctx := context.Background()
	items := env.srv.toolItems(ctx)
	if len(items) != 2 || items[0].ID != "composer" || items[1].ID != "wpcli" || items[0].Group != "tools" || items[1].Group != "tools" {
		t.Fatalf("tools %+v", items)
	}

	// WordPress websites are counted for WP-CLI's guard.
	site := &store.Site{Domain: "blog.test", SysUser: "blogtest", PHPVersion: "7.0"}
	if err := env.store.CreateSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	site.App = "wordpress"
	if err := env.store.UpdateSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	if n, err := env.srv.wordpressSites(ctx); err != nil || n != 1 {
		t.Fatalf("wordpress sites: %d %v", n, err)
	}
}

func TestSoftwareUpdateAndInstallRefusals(t *testing.T) {
	env := newEnv(t)
	a := &authed{user: &store.User{Username: "admin"}}
	call := func(h func(http.ResponseWriter, *http.Request, *authed), action, id, body string) (int, map[string]string) {
		t.Helper()
		r := httptest.NewRequest(http.MethodPost, "/api/software/"+id+"/"+action, strings.NewReader(body))
		r.SetPathValue("id", id)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h(w, r, a)
		var res map[string]string
		json.Unmarshal(w.Body.Bytes(), &res)
		return w.Code, res
	}
	// Only the tools have an update.
	for _, id := range []string{"redis", "php8.3", "nginx"} {
		if code, res := call(env.srv.handleSoftwareUpdate, "update", id, "{}"); code != http.StatusBadRequest || res["error"] != "unknown_software" {
			t.Errorf("update %s: %d %v", id, code, res)
		}
	}
	// The confirmation is a boolean; anything else in the body is refused.
	if code, res := call(env.srv.handleSoftwareInstall, "install", "php5.6", `{"accept_eol": "yes"}`); code != http.StatusBadRequest || res["error"] != "bad_request" {
		t.Fatalf("bad confirmation: %d %v", code, res)
	}
	if code, res := call(env.srv.handleSoftwareInstall, "install", "php5.6", `{"accept_eol": true, "force": true}`); code != http.StatusBadRequest || res["error"] != "bad_request" {
		t.Fatalf("unknown field: %d %v", code, res)
	}
	if jobs := env.srv.jobs.Recent(); len(jobs) != 0 {
		t.Fatalf("jobs started: %v", jobs)
	}
}
