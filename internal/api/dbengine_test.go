package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/database"
	"github.com/nefocp/nefo/internal/hosting"
)

// phpMyAdmin is reached through the panel's sign-in only, has its own CSP
// and CSRF tokens, and answers 404 while it is not installed.
func TestPhpMyAdminRoute(t *testing.T) {
	e := newEnv(t)
	if res, _ := e.do(t, "GET", "/phpmyadmin/", nil, nil); res.StatusCode != 401 {
		t.Fatalf("without a session: %d", res.StatusCode)
	}
	e.login(t)
	res, _ := e.do(t, "GET", "/phpmyadmin/index.php?route=/", nil, nil)
	if res.StatusCode != 404 {
		t.Fatalf("not installed: %d", res.StatusCode)
	}
	if csp := res.Header.Get("Content-Security-Policy"); csp != "" {
		t.Errorf("the panel's CSP on phpMyAdmin: %q", csp)
	}
	if res.Header.Get("X-Frame-Options") != "DENY" {
		t.Error("no X-Frame-Options")
	}
	// phpMyAdmin's forms cannot send the panel's CSRF header.
	res, out := e.do(t, "POST", "/phpmyadmin/index.php?route=/", nil, map[string]string{"X-Requested-With": ""})
	if res.StatusCode == 403 || out["error"] == "csrf" {
		t.Fatalf("form post refused: %d %v", res.StatusCode, out)
	}
	// The panel's API keeps both.
	res, out = e.do(t, "POST", "/api/databases", map[string]string{}, map[string]string{"X-Requested-With": ""})
	if res.StatusCode != 403 || out["error"] != "csrf" {
		t.Fatalf("API post without the header: %d %v", res.StatusCode, out)
	}
	if res.Header.Get("Content-Security-Policy") == "" {
		t.Error("no CSP on the API")
	}
}

// The proxy passes the full path, the tool's token and the tool's own
// cookies, never the panel's session or the other tool's token.
func TestProxyDBTool(t *testing.T) {
	e := newEnv(t)
	var got *http.Request
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Clone(r.Context())
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		w.Header().Set("Server", "nginx/1.24.0")
		io.WriteString(w, "phpMyAdmin")
	}))
	defer backend.Close()
	addr := strings.TrimPrefix(backend.URL, "http://")
	const token = "0123456789abcdef0123456789abcdef"

	req := httptest.NewRequest("POST", "/phpmyadmin/index.php?route=/sql&db=shop", strings.NewReader("token=x"))
	req.Host = "panel.example:8443"
	req.AddCookie(&http.Cookie{Name: "panel_session", Value: "secret"})
	req.AddCookie(&http.Cookie{Name: "pmaUser-1", Value: "enc"})
	req.Header.Set(hosting.AdminerTokenHeader, "forged")
	req.Header.Set(hosting.PhpMyAdminTokenHeader, "forged")
	req.Header.Set(hosting.AdminerHTTPSHeader, "on") // the panel decides that
	rec := httptest.NewRecorder()
	e.srv.proxyDBTool(rec, req, "phpMyAdmin", addr, hosting.PhpMyAdminTokenHeader, token)

	if rec.Code != 200 || rec.Body.String() != "phpMyAdmin" || got == nil {
		t.Fatalf("proxy: %d %q", rec.Code, rec.Body)
	}
	if got.URL.Path != "/secretpath/phpmyadmin/index.php" || got.URL.RawQuery != "route=/sql&db=shop" {
		t.Errorf("forwarded %s?%s", got.URL.Path, got.URL.RawQuery)
	}
	if got.Host != "panel.example:8443" {
		t.Errorf("host %q", got.Host)
	}
	if got.Header.Get(hosting.PhpMyAdminTokenHeader) != token || got.Header.Get(hosting.AdminerTokenHeader) != "" {
		t.Errorf("token headers: %q", got.Header)
	}
	if got.Header.Get(hosting.AdminerHTTPSHeader) != "" {
		t.Error("HTTPS header passed on for a plain HTTP request")
	}
	if c := got.Header.Get("Cookie"); strings.Contains(c, "panel_") || !strings.Contains(c, "pmaUser-1=enc") {
		t.Errorf("cookies %q", c)
	}
	h := rec.Header()
	if h.Get("Server") != "" || h.Get("X-Frame-Options") != "" || h.Get("Content-Security-Policy") != "default-src 'self'" {
		t.Errorf("response headers %q", h)
	}

	// Not installed: 404, nothing forwarded.
	got = nil
	rec = httptest.NewRecorder()
	e.srv.proxyDBTool(rec, httptest.NewRequest("GET", "/phpmyadmin/", nil), "phpMyAdmin", "", hosting.PhpMyAdminTokenHeader, "")
	if rec.Code != 404 || got != nil {
		t.Fatalf("not installed: %d", rec.Code)
	}
}

func TestDBEngineAPI(t *testing.T) {
	e := newEnv(t)
	e.login(t)
	res, out := e.do(t, "GET", "/api/features", nil, nil)
	if _, ok := out["db_engine"]; res.StatusCode != 200 || !ok {
		t.Fatalf("features: %d %v", res.StatusCode, out)
	}
	if res, out := e.do(t, "POST", "/api/db/install", map[string]string{"engine": "oracle"}, nil); res.StatusCode == 202 || out["error"] != "unknown_software" {
		t.Fatalf("unknown engine: %d %v", res.StatusCode, out)
	}
	res, out = e.do(t, "GET", "/api/db/status", nil, nil)
	if _, ok := out["mysql_unavailable"]; res.StatusCode != 200 || !ok {
		t.Fatalf("status: %d %v", res.StatusCode, out)
	}
	if name, ok := softwareName("mysql"); !ok || name != "MySQL" {
		t.Error("mysql is not a Software page item")
	}
	if name, ok := softwareName("phpmyadmin"); !ok || name != "phpMyAdmin" {
		t.Error("phpmyadmin is not a Software page item")
	}
}

func TestAppDBServer(t *testing.T) {
	if v, e := appDBServer(database.Status{Installed: true, Engine: database.MySQL, Version: "8.4.13"}); v != "" || e != "mysql" {
		t.Errorf("MySQL: %q %q", v, e)
	}
	if v, e := appDBServer(database.Status{Installed: true, Engine: database.MariaDB, Version: "10.11.14"}); v != "10.11.14" || e != "mariadb" {
		t.Errorf("MariaDB: %q %q", v, e)
	}
}
