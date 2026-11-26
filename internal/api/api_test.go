package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/auth"
	"github.com/nefocp/nefo/internal/config"
	"github.com/nefocp/nefo/internal/store"
)

const testPassword = "correct horse battery"

type testEnv struct {
	ts     *httptest.Server
	base   string // URL including the entrance, without trailing slash
	client *http.Client
	store  *store.Store
	srv    *Server
}

func newEnv(t *testing.T) *testEnv {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	hash, err := auth.HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser(context.Background(), "admin", hash); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Listen: "127.0.0.1:0", Entrance: "secretpath", DataDir: dir}
	srv := New(cfg, st, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	jar, _ := cookiejar.New(nil)
	return &testEnv{ts: ts, base: ts.URL + "/secretpath", client: &http.Client{
		Jar:           jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}, store: st, srv: srv}
}

func (e *testEnv) do(t *testing.T, method, path string, body any, headers map[string]string) (*http.Response, map[string]any) {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, e.base+path, rd)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if method != http.MethodGet {
		req.Header.Set("X-Requested-With", "panel")
	}
	for k, v := range headers {
		if v == "" {
			req.Header.Del(k)
		} else {
			req.Header.Set(k, v)
		}
	}
	res, err := e.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	raw, _ := io.ReadAll(res.Body)
	_ = json.Unmarshal(raw, &out)
	return res, out
}

func (e *testEnv) login(t *testing.T) {
	t.Helper()
	res, out := e.do(t, "POST", "/api/auth/login", map[string]string{"username": "admin", "password": testPassword}, nil)
	if res.StatusCode != 200 {
		t.Fatalf("login: %d %v", res.StatusCode, out)
	}
}

func TestEntranceHidesPanel(t *testing.T) {
	e := newEnv(t)
	for _, p := range []string{"/", "/api/auth/me", "/index.html", "/secretpat", "/secretpathx/"} {
		res, err := e.client.Get(e.ts.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != 404 {
			t.Errorf("GET %s = %d, want 404", p, res.StatusCode)
		}
		if strings.Contains(strings.ToLower(string(body)), "nefo") {
			t.Errorf("GET %s leaks the product name: %q", p, body)
		}
	}
	res, err := e.client.Get(e.base)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusFound || res.Header.Get("Location") != "/secretpath/" {
		t.Errorf("GET entrance without slash = %d %s", res.StatusCode, res.Header.Get("Location"))
	}
}

func TestLoginFlow(t *testing.T) {
	e := newEnv(t)

	if res, _ := e.do(t, "GET", "/api/auth/me", nil, nil); res.StatusCode != 401 {
		t.Fatalf("me before login = %d", res.StatusCode)
	}
	res, out := e.do(t, "POST", "/api/auth/login", map[string]string{"username": "admin", "password": "nope"}, nil)
	if res.StatusCode != 401 || out["error"] != "invalid_credentials" {
		t.Fatalf("wrong password: %d %v", res.StatusCode, out)
	}
	res, out = e.do(t, "POST", "/api/auth/login", map[string]string{"username": "ghost", "password": "nope"}, nil)
	if res.StatusCode != 401 || out["error"] != "invalid_credentials" {
		t.Fatalf("unknown user: %d %v", res.StatusCode, out)
	}

	e.login(t)
	res, out = e.do(t, "GET", "/api/auth/me", nil, nil)
	if res.StatusCode != 200 || out["username"] != "admin" || out["totp_enabled"] != false {
		t.Fatalf("me: %d %v", res.StatusCode, out)
	}
	if cc := res.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("API Cache-Control = %q", cc)
	}
	if res, _ := e.do(t, "GET", "/api/system/stats", nil, nil); res.StatusCode != 200 {
		t.Errorf("stats = %d", res.StatusCode)
	}

	if res, _ := e.do(t, "POST", "/api/auth/logout", map[string]string{}, nil); res.StatusCode != 200 {
		t.Fatalf("logout = %d", res.StatusCode)
	}
	if res, _ := e.do(t, "GET", "/api/auth/me", nil, nil); res.StatusCode != 401 {
		t.Fatalf("me after logout = %d", res.StatusCode)
	}
}

func TestCSRFProtection(t *testing.T) {
	e := newEnv(t)
	creds := map[string]string{"username": "admin", "password": testPassword}

	res, _ := e.do(t, "POST", "/api/auth/login", creds, map[string]string{"X-Requested-With": ""})
	if res.StatusCode != 403 {
		t.Errorf("without custom header = %d, want 403", res.StatusCode)
	}
	res, _ = e.do(t, "POST", "/api/auth/login", creds, map[string]string{"Origin": "https://evil.example", "Sec-Fetch-Site": "cross-site"})
	if res.StatusCode != 403 {
		t.Errorf("cross-site request = %d, want 403", res.StatusCode)
	}
	res, _ = e.do(t, "POST", "/api/auth/login", creds, map[string]string{"Content-Type": "text/plain"})
	if res.StatusCode != 415 {
		t.Errorf("text/plain body = %d, want 415", res.StatusCode)
	}
}

func TestLoginRateLimit(t *testing.T) {
	e := newEnv(t)
	bad := map[string]string{"username": "admin", "password": "wrong"}
	for i := 0; i < 5; i++ {
		e.do(t, "POST", "/api/auth/login", bad, nil)
	}
	res, out := e.do(t, "POST", "/api/auth/login", map[string]string{"username": "admin", "password": testPassword}, nil)
	if res.StatusCode != 429 || out["error"] != "too_many_attempts" {
		t.Fatalf("after 5 failures, correct password = %d %v; want 429", res.StatusCode, out)
	}
	if res.Header.Get("Retry-After") == "" {
		t.Error("missing Retry-After")
	}
}

func TestTOTPFlow(t *testing.T) {
	e := newEnv(t)
	// Freeze the clock (before login, so the session is valid at that time).
	now := time.Unix(1_900_000_000, 0)
	e.srv.now = func() time.Time { return now }
	e.login(t)

	res, out := e.do(t, "POST", "/api/auth/2fa/setup", map[string]string{}, nil)
	if res.StatusCode != 200 {
		t.Fatalf("setup = %d %v", res.StatusCode, out)
	}
	secret, _ := out["secret"].(string)
	if secret == "" || !strings.HasPrefix(out["uri"].(string), "otpauth://totp/") {
		t.Fatalf("setup response %v", out)
	}

	// Enable with the previous time step's code and log in with the current
	// one, because each code is accepted only once.
	prev := codeAt(t, secret, now.Add(-30*time.Second))
	cur := codeAt(t, secret, now)
	next := codeAt(t, secret, now.Add(30*time.Second))
	wrong := "000000"
	for wrong == prev || wrong == cur || wrong == next {
		wrong = pad6(atoiOrZero(wrong) + 1)
	}

	if res, out := e.do(t, "POST", "/api/auth/2fa/enable", map[string]string{"code": wrong}, nil); res.StatusCode != 403 || out["error"] != "invalid_totp" {
		t.Fatalf("enable with wrong code = %d %v", res.StatusCode, out)
	}
	if res, out := e.do(t, "POST", "/api/auth/2fa/enable", map[string]string{"code": prev}, nil); res.StatusCode != 200 || out["totp_enabled"] != true {
		t.Fatalf("enable = %d %v", res.StatusCode, out)
	}

	e.do(t, "POST", "/api/auth/logout", map[string]string{}, nil)
	creds := map[string]string{"username": "admin", "password": testPassword}
	if res, out := e.do(t, "POST", "/api/auth/login", creds, nil); res.StatusCode != 401 || out["error"] != "totp_required" {
		t.Fatalf("login without code = %d %v", res.StatusCode, out)
	}
	creds["code"] = prev // already used
	if res, out := e.do(t, "POST", "/api/auth/login", creds, nil); res.StatusCode != 401 || out["error"] != "invalid_totp" {
		t.Fatalf("replayed code = %d %v", res.StatusCode, out)
	}
	creds["code"] = cur
	if res, out := e.do(t, "POST", "/api/auth/login", creds, nil); res.StatusCode != 200 {
		t.Fatalf("login with code = %d %v", res.StatusCode, out)
	}
}

func TestChangePasswordSignsOut(t *testing.T) {
	e := newEnv(t)
	e.login(t)
	res, out := e.do(t, "POST", "/api/auth/password", map[string]string{"current_password": "wrong", "new_password": "a new long password"}, nil)
	if res.StatusCode != 403 || out["error"] != "invalid_password" {
		t.Fatalf("wrong current password = %d %v", res.StatusCode, out)
	}
	res, out = e.do(t, "POST", "/api/auth/password", map[string]string{"current_password": testPassword, "new_password": "short"}, nil)
	if res.StatusCode != 400 || out["error"] != "weak_password" {
		t.Fatalf("short password = %d %v", res.StatusCode, out)
	}
	res, out = e.do(t, "POST", "/api/auth/password", map[string]string{"current_password": testPassword, "new_password": "a new long password"}, nil)
	if res.StatusCode != 200 {
		t.Fatalf("change = %d %v", res.StatusCode, out)
	}
	if res, _ := e.do(t, "GET", "/api/auth/me", nil, nil); res.StatusCode != 401 {
		t.Errorf("still signed in after password change: %d", res.StatusCode)
	}
}

func codeAt(t *testing.T, secret string, at time.Time) string {
	t.Helper()
	c, err := auth.GenerateTOTP(secret, at)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func pad6(i int) string {
	s := "000000" + itoa(i)
	return s[len(s)-6:]
}

func atoiOrZero(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
