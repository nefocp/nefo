package appstore

import (
	"encoding/json"
	"strings"
	"testing"
)

// catalogMap is the embedded catalog as plain JSON values, to build
// catalogs a newer panel could publish.
func catalogMap(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(embedded, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func appMap(t *testing.T, m map[string]any, id string) map[string]any {
	t.Helper()
	for _, a := range m["apps"].([]any) {
		if am := a.(map[string]any); am["id"] == id {
			return am
		}
	}
	t.Fatalf("no app %s", id)
	return nil
}

func clone(t *testing.T, v map[string]any) map[string]any {
	t.Helper()
	b, _ := json.Marshal(v)
	var out map[string]any
	json.Unmarshal(b, &out)
	return out
}

// A downloaded catalog with recipe features this panel does not know: the
// apps that use them are left out (or keep the embedded entry), the others
// are taken, and the file as a whole is not refused.
func TestParseRemoteLeavesOutUnknownFeatures(t *testing.T) {
	m := catalogMap(t)
	m["catalog_version"] = 2099010101
	m["future_top_level"] = map[string]any{"x": 1} // ignored
	apps := m["apps"].([]any)

	// An existing app whose recipe gained a field this panel does not know.
	memos := appMap(t, m, "memos")
	memos["docker"].(map[string]any)["sandbox"] = map[string]any{"profile": "strict"}
	memos["version"] = "99.0.0"
	// A new app with a set-up step of a new kind.
	novel := clone(t, appMap(t, m, "ntfy"))
	novel["id"] = "novel"
	novel["docker"].(map[string]any)["after_start"] = []any{map[string]any{"grpc": map[string]any{"method": "Init"}}}
	// A new app that needs a newer panel.
	later := clone(t, appMap(t, m, "ntfy"))
	later["id"] = "later"
	later["min_panel"] = "99.0.0"
	// A new app with an unknown category value (refused by the validator).
	odd := clone(t, appMap(t, m, "ntfy"))
	odd["id"] = "odd"
	odd["category"] = "gardening"
	// A new app this panel can install.
	fine := clone(t, appMap(t, m, "ntfy"))
	fine["id"] = "ntfy-two"
	m["apps"] = append(apps, novel, later, odd, fine)
	data, _ := json.Marshal(m)

	// Panels up to 0.23 parsed the whole file strictly: it was refused as a
	// whole, and they kept their own catalog.
	if _, err := Parse(data); err == nil {
		t.Fatal("strict parse accepted unknown fields")
	}

	c, skipped, err := ParseRemote(data, "0.24.0")
	if err != nil {
		t.Fatal(err)
	}
	reasons := map[string]string{}
	for _, s := range skipped {
		reasons[s.ID] = s.Reason
	}
	for _, id := range []string{"memos", "novel", "later", "odd"} {
		if reasons[id] == "" {
			t.Errorf("%s was not left out (%v)", id, reasons)
		}
	}
	if !strings.Contains(reasons["later"], "99.0.0") || !strings.Contains(reasons["memos"], "sandbox") {
		t.Errorf("reasons: %v", reasons)
	}
	if _, ok := c.App("ntfy-two"); !ok {
		t.Error("a valid new app was dropped")
	}
	for _, id := range []string{"novel", "later", "odd"} {
		if _, ok := c.App(id); ok {
			t.Errorf("%s should not be installable", id)
		}
	}
	// Memos stays, as this panel knows it.
	if a, ok := c.App("memos"); !ok || a.Version != "26.11.2" {
		t.Errorf("memos: %v %v", ok, a)
	}
	if len(c.Apps) != len(Embedded().Apps)+1 {
		t.Errorf("%d apps", len(c.Apps))
	}
	// A dev build (version "dev") takes apps whatever their min_panel.
	if c, _, _ := ParseRemote(data, "dev"); c != nil {
		if _, ok := c.App("later"); !ok {
			t.Error("dev build left out a min_panel app")
		}
	}
	// The frame itself is still checked.
	for _, bad := range []string{`{"schema":2,"catalog_version":5,"apps":[]}`, `{"schema":1,"catalog_version":0,"apps":[]}`,
		`{"schema":1,"catalog_version":5,"apps":[{"id":"x"}]}`, `not json`} {
		if _, _, err := ParseRemote([]byte(bad), "0.24.0"); err == nil {
			t.Errorf("accepted %s", bad)
		}
	}
}

func TestStepValues(t *testing.T) {
	a, _ := Embedded().App("memos")
	b, _ := json.Marshal(a)
	var cp App
	json.Unmarshal(b, &cp)
	if err := cp.validate(); err != nil {
		t.Fatal(err)
	}
	// ${step.token} used before a step saved it.
	steps := cp.Docker.AfterStart
	cp.Docker.AfterStart = []Step{steps[2], steps[1]}
	if err := cp.validate(); err == nil || !strings.Contains(err.Error(), "step.token") {
		t.Errorf("use before save: %v", err)
	}
	// Outside an HTTP step.
	json.Unmarshal(b, &cp)
	cp.Docker.Services["app"].Env["X"] = "${step.token}"
	if err := cp.validate(); err == nil {
		t.Error("${step.*} in env accepted")
	}
	// Bad header names and JSON paths.
	json.Unmarshal(b, &cp)
	cp.Docker.AfterStart[2].HTTP.Headers["Bad Header"] = "x"
	if err := cp.validate(); err == nil {
		t.Error("bad header accepted")
	}
	json.Unmarshal(b, &cp)
	cp.Docker.AfterStart[1].HTTP.Save["token"] = "a..b"
	if err := cp.validate(); err == nil {
		t.Error("bad save path accepted")
	}
	json.Unmarshal(b, &cp)
	cp.Docker.AfterStart[0].HTTP.Method = "DELETE"
	if err := cp.validate(); err == nil {
		t.Error("DELETE accepted")
	}
	// Step values are checked when they are used.
	e := &Env{Steps: map[string]string{"token": "eyJhbGciOi.J9-_x"}}
	if v, err := e.Expand("Bearer ${step.token}"); err != nil || v != "Bearer eyJhbGciOi.J9-_x" {
		t.Errorf("expand: %q %v", v, err)
	}
	e.Steps["token"] = "a b"
	if _, err := e.Expand("${step.token}"); err == nil {
		t.Error("a step value with a space was used")
	}
}

func TestSiteVolumes(t *testing.T) {
	m, e := render(t, "navidrome", true)
	app := svc(m, "app")
	var music map[string]any
	for _, v := range app["volumes"].([]any) {
		if vm := v.(map[string]any); vm["target"] == "/music" {
			music = vm
		}
	}
	if music == nil || music["source"] != "/var/www/vault.example.com/music" || music["read_only"] != true {
		t.Fatalf("music volume: %v", music)
	}
	a, _ := Embedded().App("navidrome")
	dirs, _, err := Layout(a, e)
	if err != nil {
		t.Fatal(err)
	}
	site := 0
	for _, d := range dirs {
		if d.Site {
			site++
			if d.Path != "/var/www/vault.example.com/music" || d.UID != 20001 {
				t.Errorf("site dir %+v", d)
			}
		}
	}
	if site != 1 {
		t.Errorf("%d site dirs", site)
	}
	// Without a website there is no site folder.
	e.SiteDir = ""
	if _, err := Render(a, e); err == nil {
		t.Error("rendered a site volume without a site folder")
	}
	// Only a container that runs as the site's user may mount one.
	b, _ := json.Marshal(a)
	var cp App
	json.Unmarshal(b, &cp)
	cp.Docker.Services["app"].User = ""
	if err := cp.validate(); err == nil {
		t.Error("site volume in a root container accepted")
	}
	json.Unmarshal(b, &cp)
	cp.Docker.Services["app"].Volumes[1].Owner = "0:0"
	if err := cp.validate(); err == nil {
		t.Error("site volume with another owner accepted")
	}
}

func TestBootstrapOverridesEnv(t *testing.T) {
	for _, boot := range []bool{true, false} {
		m, _ := render(t, "vikunja", boot)
		env := svc(m, "app")["environment"].(map[string]any)
		want := map[bool]string{true: "true", false: "false"}[boot]
		if env["VIKUNJA_SERVICE_ENABLEREGISTRATION"] != want {
			t.Errorf("bootstrap %v: registration %v", boot, env["VIKUNJA_SERVICE_ENABLEREGISTRATION"])
		}
	}
	// ntfy's admin hash: every $ doubled for Compose.
	m, e := render(t, "ntfy", false)
	users := svc(m, "app")["environment"].(map[string]any)["NTFY_AUTH_USERS"].(string)
	if !strings.HasPrefix(users, "admin:$$2a$$10$$") || !strings.HasSuffix(users, ":admin") ||
		strings.Count(users, "$$") != strings.Count(e.Secrets["admin_password_bcrypt"], "$") {
		t.Errorf("NTFY_AUTH_USERS %q", users)
	}
}

func TestBatch2Catalog(t *testing.T) {
	c := Embedded()
	if n := len(c.Apps); n != 42 {
		t.Errorf("%d apps", n)
	}
	for _, id := range []string{"grafana", "ntfy", "gotify", "memos", "linkding", "navidrome", "vikunja", "actual",
		"changedetection", "healthchecks", "listmonk", "kanboard", "freshrss", "jellyfin", "mealie"} {
		a, ok := c.App(id)
		if !ok {
			t.Errorf("%s missing", id)
			continue
		}
		if a.Docker != nil {
			for name, s := range a.Docker.Services {
				if s.Digest == "" {
					t.Errorf("%s/%s: no digest", id, name)
				}
			}
		}
	}
	// No secret reaches a command line, in the new recipes either.
	for _, a := range c.Apps {
		if a.Docker == nil {
			continue
		}
		for _, list := range [][]Step{a.Docker.BeforeStart, a.Docker.AfterStart} {
			for _, st := range list {
				if e := st.Exec; e != nil && strings.Contains(strings.Join(e.Argv, " "), "${secret.") {
					t.Errorf("%s: secret on a command line", a.ID)
				}
			}
		}
	}
}
