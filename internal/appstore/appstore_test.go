package appstore

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestEmbeddedCatalog(t *testing.T) {
	c := Embedded()
	if len(c.Apps) < 25 {
		t.Fatalf("only %d apps", len(c.Apps))
	}
	for _, id := range []string{"open-webui", "ollama", "vaultwarden", "rustfs", "filebrowser", "postgresql", "mongodb",
		"code-server", "stirling-pdf", "it-tools", "directus", "wikijs", "umami", "adguard-home", "odoo",
		"woocommerce", "opencart", "moodle", "wordpress", "joomla", "nextcloud", "drupal", "laravel", "ghost", "uptime-kuma", "n8n", "gitea"} {
		if _, ok := c.App(id); !ok {
			t.Errorf("%s missing", id)
		}
	}
	for _, gone := range []string{"wg-easy", "minio", "strapi"} {
		if _, ok := c.App(gone); ok {
			t.Errorf("%s must not be in batch 1", gone)
		}
	}
	for _, a := range c.Apps {
		if a.Docker == nil {
			continue
		}
		for name, s := range a.Docker.Services {
			if s.Digest == "" {
				t.Errorf("%s/%s: the embedded catalog pins digests", a.ID, name)
			}
		}
	}
	for _, id := range []string{"open-webui", "mongodb", "directus", "n8n"} {
		if a, _ := c.App(id); a == nil || !a.SourceAvailable {
			t.Errorf("%s needs the source-available badge", id)
		}
	}
	if a, _ := c.App("ollama"); a.Resources.MinRAMMB != 3584 || a.Resources.MinFreeDiskMB != 12288 {
		t.Error("Ollama gate must be 3.5 GB RAM and 12 GB disk")
	}
}

func mustCatalog(t *testing.T, mutate func(*Catalog)) error {
	t.Helper()
	c := Embedded()
	mutate(c)
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Parse(b)
	return err
}

func TestValidatorRefuses(t *testing.T) {
	app := func(c *Catalog, id string) *App { a, _ := c.App(id); return a }
	cases := map[string]func(*Catalog){
		"floating tag": func(c *Catalog) { app(c, "vaultwarden").Docker.Services["app"].Image = "vaultwarden/server:latest" },
		"bare major":   func(c *Catalog) { app(c, "gitea").Docker.Services["app"].Image = "gitea/gitea:1" },
		"secret on argv": func(c *Catalog) {
			app(c, "odoo").Docker.Services["app"].Command = []string{"--pw", "${secret.db_password}"}
		},
		"secret in run argv":  func(c *Catalog) { app(c, "odoo").Docker.BeforeStart[0].Run.Argv = []string{"${secret.db_password}"} },
		"unknown placeholder": func(c *Catalog) { app(c, "vaultwarden").Docker.Services["app"].Env["X"] = "${hostname}" },
		"unknown secret":      func(c *Catalog) { app(c, "vaultwarden").Docker.Services["app"].Env["X"] = "${secret.nope}" },
		"malformed":           func(c *Catalog) { app(c, "vaultwarden").Docker.Services["app"].Env["X"] = "${domain" },
		"bad owner":           func(c *Catalog) { app(c, "vaultwarden").Docker.Services["app"].Volumes[0].Owner = "root" },
		"relative target":     func(c *Catalog) { app(c, "vaultwarden").Docker.Services["app"].Volumes[0].Target = "data" },
		"file escape":         func(c *Catalog) { app(c, "adguard-home").Docker.Services["app"].Files[0].Path = "../x" },
		"http download":       func(c *Catalog) { app(c, "joomla").PHP.Download.URL = "http://example.com/x.tgz" },
		"no sha":              func(c *Catalog) { app(c, "joomla").PHP.Download.SHA256 = "" },
		"unknown installer":   func(c *Catalog) { app(c, "joomla").PHP.Installer = "magento" },
		"duplicate":           func(c *Catalog) { c.Apps = append(c.Apps, c.Apps[0]) },
		"no vi":               func(c *Catalog) { delete(app(c, "umami").Description, "vi") },
		"unknown logo":        func(c *Catalog) { app(c, "umami").Logo = "nope" },
		"web on service":      func(c *Catalog) { app(c, "postgresql").Docker.Web = &Web{Service: "app", Port: 1, Health: "/"} },
		"bad schema":          func(c *Catalog) { c.Schema = 2 },
		"hash of hash": func(c *Catalog) {
			app(c, "umami").Docker.Secrets["admin_password_bcrypt"].From = "admin_password_bcrypt"
		},
	}
	for name, mutate := range cases {
		if err := mustCatalog(t, mutate); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if err := mustCatalog(t, func(*Catalog) {}); err != nil {
		t.Fatalf("unchanged catalog refused: %v", err)
	}
	// Unknown fields are refused too (a typo must not silently do nothing).
	if _, err := Parse([]byte(`{"schema":1,"catalog_version":1,"apps":[],"extra":1}`)); err == nil {
		t.Error("unknown field accepted")
	}
}

func testEnv(t *testing.T, secrets map[string]string) *Env {
	return &Env{Name: "vault_example_com", Domain: "vault.example.com", HTTPS: true, Port: 20001, SiteUID: 20001, SiteGID: 20001,
		AdminEmail: "owner@example.com", Secrets: secrets, Inputs: map[string]string{}, ProjectDir: "/var/lib/panel-apps/vault_example_com",
		SiteDir:    "/var/www/vault.example.com",
		ExtraPorts: map[int]int{9000: 20002}}
}

func render(t *testing.T, id string, bootstrap bool) (map[string]any, *Env) {
	t.Helper()
	a, _ := Embedded().App(id)
	sec, err := GenerateSecrets(a.Docker.Secrets, nil)
	if err != nil {
		t.Fatal(err)
	}
	e := testEnv(t, sec)
	e.Bootstrap = bootstrap
	b, err := Render(a, e)
	if err != nil {
		t.Fatalf("%s: %v", id, err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m, e
}

func svc(m map[string]any, name string) map[string]any {
	return m["services"].(map[string]any)[name].(map[string]any)
}

func TestRenderEveryDockerApp(t *testing.T) {
	for _, a := range Embedded().Apps {
		if a.Docker == nil {
			continue
		}
		for _, boot := range []bool{true, false} {
			m, _ := render(t, a.ID, boot)
			if m["name"] != "panelapp-vault_example_com" {
				t.Errorf("%s: project %v", a.ID, m["name"])
			}
			for name := range a.Docker.Services {
				s := svc(m, name)
				ports, _ := s["ports"].([]any)
				for _, p := range ports {
					if hip := p.(map[string]any)["host_ip"]; hip != "127.0.0.1" {
						t.Errorf("%s/%s: port on %v", a.ID, name, hip)
					}
				}
				if s["mem_limit"] == "" || s["restart"] != "unless-stopped" {
					t.Errorf("%s/%s: limits", a.ID, name)
				}
			}
		}
	}
}

func TestRenderVaultwarden(t *testing.T) {
	m, e := render(t, "vaultwarden", false)
	app := svc(m, "app")
	env := app["environment"].(map[string]any)
	if env["DOMAIN"] != "https://vault.example.com" || env["SIGNUPS_ALLOWED"] != "false" {
		t.Errorf("env %v", env)
	}
	// argon2 hashes are full of $: Compose would read them as variables.
	if tok := env["ADMIN_TOKEN"].(string); !strings.HasPrefix(tok, "$$argon2id$$v=19$$") {
		t.Errorf("ADMIN_TOKEN not escaped: %s", tok)
	}
	if app["user"] != "20001:20001" || app["container_name"] != "panel-vault_example_com" {
		t.Errorf("user/name %v %v", app["user"], app["container_name"])
	}
	if p := app["ports"].([]any)[0].(map[string]any); p["published"] != "20001" || p["target"] != float64(80) {
		t.Errorf("port %v", p)
	}
	v := app["volumes"].([]any)[0].(map[string]any)
	if v["source"] != e.DataDir()+"/data" || v["target"] != "/data" {
		t.Errorf("volume %v", v)
	}
}

func TestRenderBootstrapAndSecretFiles(t *testing.T) {
	with, _ := render(t, "wikijs", true)
	without, _ := render(t, "wikijs", false)
	if svc(with, "app")["environment"].(map[string]any)["ADMIN_PASS"] == nil {
		t.Error("bootstrap env missing on the first start")
	}
	if svc(without, "app")["environment"].(map[string]any)["ADMIN_PASS"] != nil {
		t.Error("bootstrap env kept after the first start")
	}
	db := svc(without, "db")
	if svc(without, "app")["depends_on"].(map[string]any)["db"].(map[string]any)["condition"] != "service_healthy" {
		t.Error("app must wait for the database")
	}
	if db["ports"] != nil {
		t.Error("the database sidecar must not publish a port")
	}
	found := false
	for _, v := range db["volumes"].([]any) {
		vm := v.(map[string]any)
		if vm["target"] == "/run/secrets/db_password" && vm["read_only"] == true {
			found = true
		}
	}
	if !found {
		t.Error("database password not mounted as a file")
	}
	if env := db["environment"].(map[string]any); env["POSTGRES_PASSWORD"] != nil {
		t.Error("password in the environment")
	}
	if svc(without, "db")["container_name"] != "panel-vault_example_com-db" {
		t.Error("sidecar name")
	}
}

func TestRenderSharedNetwork(t *testing.T) {
	m, _ := render(t, "ollama", false)
	nets := m["networks"].(map[string]any)
	if n := nets["panel-ai"].(map[string]any); n["external"] != true || n["name"] != "panel-ai" {
		t.Errorf("network %v", n)
	}
	sn := svc(m, "app")["networks"].(map[string]any)["panel-ai"].(map[string]any)
	if sn["aliases"].([]any)[0] != "ollama" {
		t.Errorf("alias %v", sn)
	}
}

func TestLayoutAndFiles(t *testing.T) {
	a, _ := Embedded().App("adguard-home")
	sec, _ := GenerateSecrets(a.Docker.Secrets, nil)
	e := testEnv(t, sec)
	dirs, files, err := Layout(a, e)
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) != 2 || dirs[0].Mode != 0o700 || dirs[0].UID != 0 {
		t.Errorf("dirs %+v", dirs)
	}
	if len(files) != 1 || files[0].Mode != 0o600 {
		t.Fatalf("files %+v", files)
	}
	text := string(files[0].Data)
	if !strings.Contains(text, "    - "+sec["client_id"]+"\n") || !strings.Contains(text, "password: "+sec["admin_password_bcrypt"]+"\n") {
		t.Errorf("config:\n%s", text)
	}
	if bcrypt.CompareHashAndPassword([]byte(sec["admin_password_bcrypt"]), []byte(sec["admin_password"])) != nil {
		t.Error("bcrypt of the admin password")
	}
	if strings.Contains(text, "${") {
		t.Error("placeholder left")
	}
}

func TestExpandRefusesBadValues(t *testing.T) {
	e := testEnv(t, map[string]string{"x": "ok'; DROP"})
	if _, err := e.Expand("${secret.x}"); err == nil {
		t.Error("a secret with a quote was accepted")
	}
	e.AdminEmail = "a@b.c\nX: y"
	if _, err := e.Expand("${admin_email}"); err == nil {
		t.Error("an e-mail with a line break was accepted")
	}
	e.Domain = ""
	if _, err := e.Expand("${url}"); err == nil {
		t.Error("url without a site")
	}
}

func TestGenerateSecrets(t *testing.T) {
	specs := map[string]*SecretSpec{
		"a": {Gen: "random", Length: 20}, "b": {Gen: "hex", Length: 64}, "c": {Gen: "argon2id", From: "a"},
		"d": {Gen: "random", Chars: "lower-alnum", Length: 24},
	}
	s, err := GenerateSecrets(specs, map[string]string{"a": "keepthisvalue123", "stale": "x"})
	if err != nil {
		t.Fatal(err)
	}
	if s["a"] != "keepthisvalue123" || s["stale"] != "" {
		t.Errorf("kept %v", s)
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(s["b"]) || !regexp.MustCompile(`^[a-z0-9]{24}$`).MatchString(s["d"]) {
		t.Errorf("formats %v", s)
	}
	if !regexp.MustCompile(`^\$argon2id\$v=19\$m=65536,t=3,p=4\$[A-Za-z0-9+/]{22}\$[A-Za-z0-9+/]{43}$`).MatchString(s["c"]) {
		t.Errorf("argon2 %s", s["c"])
	}
}

func TestCheckResources(t *testing.T) {
	c := Embedded()
	ollama, _ := c.App("ollama")
	r := Check(ollama, System{MemTotalMB: 1961, MemAvailableMB: 1400, FreeDiskMB: 42000, Arch: "amd64"})
	if r.OK() || r.Refusals[0].Code != "app_needs_ram" {
		t.Errorf("2 GB server: %+v", r)
	}
	r = Check(ollama, System{MemTotalMB: 8000, MemAvailableMB: 7000, FreeDiskMB: 10000, Arch: "amd64"})
	if r.OK() || r.Refusals[0].Code != "app_needs_disk" {
		t.Errorf("10 GB disk: %+v", r)
	}
	r = Check(ollama, System{MemTotalMB: 4096, MemAvailableMB: 3500, FreeDiskMB: 20000, Arch: "amd64"})
	if !r.OK() || r.MemoryLimitMB != 3072 || len(r.Warnings) == 0 || r.Warnings[0].Code != "app_ram_below_recommended" {
		t.Errorf("4 GB: %+v", r)
	}
	mongo, _ := c.App("mongodb")
	r = Check(mongo, System{MemTotalMB: 4096, MemAvailableMB: 3000, FreeDiskMB: 20000, Arch: "amd64", CPUFlags: []string{"sse4_2"}})
	if r.OK() || r.Refusals[0].Code != "app_needs_cpu" {
		t.Errorf("no AVX: %+v", r)
	}
	vw, _ := c.App("vaultwarden")
	r = Check(vw, System{MemTotalMB: 1961, MemAvailableMB: 200, FreeDiskMB: 42000, Arch: "amd64", RunningLimitsMB: 3000})
	codes := ""
	for _, w := range r.Warnings {
		codes += w.Code + " "
	}
	if !r.OK() || !strings.Contains(codes, "app_low_available_memory") || !strings.Contains(codes, "app_memory_overcommit") {
		t.Errorf("warnings %+v", r)
	}
	if cpuFlags("processor: 0\nflags\t\t: fpu avx avx2\n")[2] != "avx2" || cpuFlags("Features\t: fp asimd atomics\n")[2] != "atomics" {
		t.Error("cpuinfo")
	}
}

func TestPickDownload(t *testing.T) {
	m, _ := Embedded().App("moodle")
	if _, v, _ := m.PHP.PickDownload("10.11.14-MariaDB"); v != "5.2.5" {
		t.Errorf("MariaDB 10.11 got %s", v)
	}
	if _, v, _ := m.PHP.PickDownload("11.8.3-MariaDB"); v != "5.3.1" {
		t.Errorf("MariaDB 11.8 got %s", v)
	}
	if _, _, ok := m.PHP.PickDownload("10.6.1"); ok {
		t.Error("MariaDB 10.6 must have no variant")
	}
	if _, v, _ := m.PHP.PickDownload(""); v != "5.2.5" {
		t.Errorf("unknown MariaDB got %s", v)
	}
}

func TestLogosAreSanitized(t *testing.T) {
	bad := regexp.MustCompile(`(?i)<(script|style|foreignobject|image|a|text|iframe)\b|\son[a-z]+=|javascript:|data:|href="[^#]`)
	keys := Logos()
	if len(keys) < 30 {
		t.Fatalf("%d logos", len(keys))
	}
	for _, k := range keys {
		b, _ := Logo(k)
		if !strings.HasPrefix(string(b), "<svg") || bad.Match(b) {
			t.Errorf("%s.svg is not plain shapes", k)
		}
	}
	if _, ok := Logo("../catalog"); ok {
		t.Error("path in logo key")
	}
}

func TestRemoteCatalog(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	verify := func(data, sig []byte) error {
		if !ed25519.Verify(pub, data, sig) {
			return errors.New("bad signature")
		}
		return nil
	}
	c := Embedded()
	c.CatalogVersion++
	newer, _ := json.Marshal(c)
	files := map[string][]byte{"/appstore-catalog.json": newer, "/appstore-catalog.json.sig": ed25519.Sign(priv, newer)}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Write(b)
	}))
	defer srv.Close()
	defer setCurrent(Embedded(), "embedded", nil)

	off := &Remote{Base: srv.URL, CacheDir: t.TempDir()}
	if _, err := off.Refresh(context.Background()); !errors.Is(err, ErrRemoteDisabled) {
		t.Fatalf("without a key: %v", err)
	}
	dir := t.TempDir()
	r := &Remote{Base: srv.URL, CacheDir: dir, Verify: verify}
	got, err := r.Refresh(context.Background())
	if err != nil || got.CatalogVersion != c.CatalogVersion || Source() != "remote" {
		t.Fatalf("refresh: %v", err)
	}
	// The same file again is not newer (no replay).
	if _, err := r.Refresh(context.Background()); !IsNotNewer(err) {
		t.Errorf("replay: %v", err)
	}
	// But it loads from the cache at start-up.
	setCurrent(Embedded(), "embedded", nil)
	if err := r.LoadCache(); err != nil || Current().CatalogVersion != c.CatalogVersion {
		t.Errorf("cache: %v", err)
	}
	// A wrong signature is refused, and so is a tampered cache.
	files["/appstore-catalog.json.sig"] = make([]byte, 64)
	c.CatalogVersion++
	files["/appstore-catalog.json"], _ = json.Marshal(c)
	if _, err := r.Refresh(context.Background()); err == nil || IsNotNewer(err) {
		t.Errorf("bad signature: %v", err)
	}
	os.WriteFile(filepath.Join(dir, "appstore-catalog.json"), []byte(`{"schema":1}`), 0o600)
	if err := r.LoadCache(); err == nil {
		t.Error("tampered cache accepted")
	}
}
