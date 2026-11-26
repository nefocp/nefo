package api

import (
	"bytes"
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/auth"
	"github.com/nefocp/nefo/internal/store"
)

// createKey makes a key through the signed-in session and returns it.
func (e *testEnv) createKey(t *testing.T, body map[string]any) (string, int64) {
	t.Helper()
	res, out := e.do(t, "POST", "/api/settings/api-keys", body, nil)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create key %v: %d %v", body, res.StatusCode, out)
	}
	token, _ := out["token"].(string)
	id, _ := out["key"].(map[string]any)["id"].(float64)
	return token, int64(id)
}

// keyDo sends a request the way a script does: the key in the header, no
// cookie, no X-Requested-With.
func (e *testEnv) keyDo(t *testing.T, method, path, key string, body any, headers map[string]string) (*http.Response, map[string]any) {
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
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out map[string]any
	raw, _ := io.ReadAll(res.Body)
	_ = json.Unmarshal(raw, &out)
	return res, out
}

// lockedBuffer collects the service log.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func (e *testEnv) captureLog() *lockedBuffer {
	buf := &lockedBuffer{}
	e.srv.log = slog.New(slog.NewTextHandler(buf, nil))
	return buf
}

func (e *testEnv) newSite(t *testing.T) string {
	t.Helper()
	site := &store.Site{Domain: "key.example", SysUser: "key_example"}
	if err := e.store.CreateSite(context.Background(), site); err != nil {
		t.Fatal(err)
	}
	return "/api/sites/" + strconv.FormatInt(site.ID, 10) + "/note"
}

func TestAPIKeyCreate(t *testing.T) {
	e := newEnv(t)
	e.login(t)
	token, id := e.createKey(t, map[string]any{"name": " deploy-bot ", "access": "full",
		"addresses": []string{"127.0.0.1", " 10.1.2.3/8", "", "2001:db8::/32", "::ffff:192.0.2.1"}, "expires_days": 90})
	if !strings.HasPrefix(token, "nefo_") || len(token) != apiKeyLen || !validAPIKeyFormat(token) {
		t.Fatalf("key %q", token)
	}
	res, out := e.do(t, "GET", "/api/settings/api-keys", nil, nil)
	keys, _ := out["keys"].([]any)
	if res.StatusCode != 200 || len(keys) != 1 || out["max"] != float64(maxAPIKeys) {
		t.Fatalf("list: %d %v", res.StatusCode, out)
	}
	k := keys[0].(map[string]any)
	if k["id"] != float64(id) || k["name"] != "deploy-bot" || k["access"] != "full" || k["prefix"] != token[:apiKeyShown] ||
		k["expires_at"] == nil || k["last_used_at"] != nil || k["expired"] != false {
		t.Fatalf("key %v", k)
	}
	if got := k["addresses"].([]any); len(got) != 4 || got[1] != "10.0.0.0/8" || got[3] != "192.0.2.1" {
		t.Fatalf("addresses %v", got)
	}
	// Neither the key nor its hash is ever shown again.
	stored, _ := e.store.APIKeyByID(context.Background(), id)
	raw, _ := json.Marshal(out)
	if strings.Contains(string(raw), token[apiKeyShown:]) || strings.Contains(string(raw), stored.Hash) {
		t.Fatalf("the list shows the key: %s", raw)
	}
	if stored.Hash != auth.HashToken(token) || strings.Contains(stored.Hash, token[apiKeyShown:]) {
		t.Fatal("stored hash")
	}
	two, _ := e.createKey(t, map[string]any{"name": "Two", "access": "read", "expires_days": 0})
	if two == token || two[:apiKeyShown] == token[:apiKeyShown] {
		t.Fatal("two keys alike")
	}

	for _, c := range []struct {
		body map[string]any
		code string
	}{
		{map[string]any{"name": "DEPLOY-bot", "access": "read"}, "api_key_exists"},
		{map[string]any{"name": "", "access": "read"}, "invalid_key_name"},
		{map[string]any{"name": "a=b", "access": "read"}, "invalid_key_name"},
		{map[string]any{"name": `x"y`, "access": "read"}, "invalid_key_name"},
		{map[string]any{"name": strings.Repeat("a", maxAPIKeyName+1), "access": "read"}, "invalid_key_name"},
		{map[string]any{"name": "ok", "access": "admin"}, "bad_request"},
		{map[string]any{"name": "ok", "access": "read", "expires_days": 10}, "bad_request"},
		{map[string]any{"name": "ok", "access": "read", "addresses": []string{"example.com"}}, "invalid_ip"},
		{map[string]any{"name": "ok", "access": "read", "addresses": []string{"fe80::1%eth0"}}, "invalid_ip"},
		{map[string]any{"name": "ok", "access": "read", "addresses": []string{"10.0.0.0/33"}}, "invalid_ip"},
	} {
		if res, out := e.do(t, "POST", "/api/settings/api-keys", c.body, nil); res.StatusCode != 400 || out["error"] != c.code {
			t.Errorf("%v: %d %v, want %s", c.body, res.StatusCode, out, c.code)
		}
	}
	many := make([]string, maxAPIKeyAddresses+1)
	for i := range many {
		many[i] = "10.0.0." + itoa(i+1)
	}
	if res, out := e.do(t, "POST", "/api/settings/api-keys", map[string]any{"name": "ok", "access": "read", "addresses": many}, nil); res.StatusCode != 400 || out["error"] != "too_many_key_addresses" {
		t.Errorf("too many addresses: %d %v", res.StatusCode, out)
	}
	for i := 2; i < maxAPIKeys; i++ {
		e.createKey(t, map[string]any{"name": "k" + itoa(i), "access": "read"})
	}
	if res, out := e.do(t, "POST", "/api/settings/api-keys", map[string]any{"name": "one more", "access": "read"}, nil); res.StatusCode != 400 || out["error"] != "too_many_api_keys" {
		t.Errorf("over the limit: %d %v", res.StatusCode, out)
	}
	// Managing keys needs the session's X-Requested-With, like any change.
	if res, _ := e.do(t, "POST", "/api/settings/api-keys", map[string]any{"name": "csrf", "access": "read"}, map[string]string{"X-Requested-With": ""}); res.StatusCode != 403 {
		t.Errorf("create without X-Requested-With: %d", res.StatusCode)
	}
}

func TestAPIKeyAccess(t *testing.T) {
	e := newEnv(t)
	e.login(t)
	note := e.newSite(t)
	ro, _ := e.createKey(t, map[string]any{"name": "monitor", "access": "read"})
	full, _ := e.createKey(t, map[string]any{"name": "deploy-bot", "access": "full", "addresses": []string{"127.0.0.0/8", "::1"}})

	// A read-only key reads, and changes nothing.
	if res, out := e.keyDo(t, "GET", "/api/audit", ro, nil, nil); res.StatusCode != 200 || out["entries"] == nil {
		t.Fatalf("read-only GET: %d %v", res.StatusCode, out)
	}
	if res, _ := e.keyDo(t, "HEAD", "/api/audit", ro, nil, nil); res.StatusCode != 200 {
		t.Fatalf("read-only HEAD: %d", res.StatusCode)
	}
	// (Routes whose handler would do no harm if the check were missing.)
	for _, c := range []struct{ method, path string }{{"PUT", note}, {"POST", "/api/alerts/test"}, {"DELETE", "/api/backups/999"}} {
		res, out := e.keyDo(t, c.method, c.path, ro, map[string]string{}, nil)
		if res.StatusCode != 403 || out["error"] != "api_key_read_only" {
			t.Errorf("read-only %s %s: %d %v", c.method, c.path, res.StatusCode, out)
		}
	}
	// A full key changes things without X-Requested-With.
	res, out := e.keyDo(t, "PUT", note, full, map[string]string{"note": "from CI"}, nil)
	if res.StatusCode != 200 || out["note"] != "from CI" {
		t.Fatalf("full PUT: %d %v", res.StatusCode, out)
	}
	// "bearer" in any case.
	if res, _ := e.keyDo(t, "GET", "/api/audit", "", nil, map[string]string{"Authorization": "bearer " + full}); res.StatusCode != 200 {
		t.Errorf("lowercase bearer: %d", res.StatusCode)
	}
	// A session still needs X-Requested-With.
	if res, _ := e.do(t, "PUT", note, map[string]string{"note": "x"}, map[string]string{"X-Requested-With": ""}); res.StatusCode != 403 {
		t.Errorf("session PUT without X-Requested-With: %d", res.StatusCode)
	}
	// A browser on another site is still refused, key or not.
	res, out = e.keyDo(t, "PUT", note, full, map[string]string{"note": "x"}, map[string]string{"Origin": "https://evil.example", "Sec-Fetch-Site": "cross-site"})
	if res.StatusCode != 403 || out["error"] == "api_key_read_only" {
		t.Errorf("cross-site with a key: %d %v", res.StatusCode, out)
	}
	// A key never signs in.
	res, out = e.keyDo(t, "POST", "/api/auth/login", full, map[string]string{"username": "admin", "password": testPassword}, nil)
	if res.StatusCode != 403 || out["error"] != "api_key_forbidden" || len(res.Cookies()) != 0 {
		t.Errorf("login with a key: %d %v %v", res.StatusCode, out, res.Cookies())
	}

	// Changes are audited as the user, with the key's name.
	entries, _ := e.store.RecentAudit(context.Background(), 50)
	var seen, refused, created bool
	for _, a := range entries {
		switch {
		case a.Action == "api_request" && a.Username == "admin (key deploy-bot)" && a.Detail == "PUT "+note && a.IP == "127.0.0.1":
			seen = true
		case a.Action == "api_key_refused" && a.Username == "admin (key monitor)" && a.Detail == "PUT "+note:
			refused = true
		case a.Action == "api_key_created" && a.Username == "admin" && strings.HasPrefix(a.Detail, "deploy-bot (full, 127.0.0.0/8 ::1"):
			created = true
		case a.Action == "api_request" && a.Username == "admin (key monitor)":
			t.Errorf("a read was audited: %+v", a)
		}
	}
	if !seen || !refused || !created {
		t.Errorf("audit: request %v, refused %v, created %v: %+v", seen, refused, created, entries)
	}
}

// A request with a key is never a session, and the key is read from the
// Authorization header only.
func TestAPIKeyNotASession(t *testing.T) {
	e := newEnv(t)
	e.login(t)
	full, _ := e.createKey(t, map[string]any{"name": "ci", "access": "full"})

	// A key with the session cookie still gets the key's answer.
	if res, out := e.do(t, "GET", "/api/auth/me", nil, map[string]string{"Authorization": "Bearer " + full}); res.StatusCode != 403 || out["error"] != "api_key_forbidden" {
		t.Errorf("key and cookie on a session-only route: %d %v", res.StatusCode, out)
	}
	if res, out := e.do(t, "GET", "/api/audit", nil, map[string]string{"Authorization": "Bearer nefo_wrong"}); res.StatusCode != 401 || out["error"] != "invalid_api_key" {
		t.Errorf("bad key and a good cookie: %d %v", res.StatusCode, out)
	}
	// The cookie session is untouched by that.
	if res, _ := e.do(t, "GET", "/api/auth/me", nil, nil); res.StatusCode != 200 {
		t.Errorf("session after a bad key: %d", res.StatusCode)
	}
	// Anywhere else, a key is nothing.
	for _, c := range []struct {
		path string
		hdr  map[string]string
	}{
		{"/api/audit?key=" + full, nil},
		{"/api/audit?access_token=" + full, nil},
		{"/api/audit", map[string]string{"Cookie": auth.CookieName + "=" + full}},
		{"/api/audit", map[string]string{"X-API-Key": full}},
		{"/api/audit", map[string]string{"Authorization": "Token " + full}},
		{"/api/audit", map[string]string{"Authorization": "Basic " + full}},
	} {
		if res, out := e.keyDo(t, "GET", c.path, "", nil, c.hdr); res.StatusCode != 401 || out["error"] != "unauthorized" {
			t.Errorf("%s %v: %d %v", c.path, c.hdr, res.StatusCode, out)
		}
	}
	// Two Bearer headers: neither counts.
	req, _ := http.NewRequest("GET", e.base+"/api/audit", nil)
	req.Header.Add("Authorization", "Bearer "+full)
	req.Header.Add("Authorization", "Bearer "+full)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 401 {
		t.Errorf("two Bearer headers: %d", res.StatusCode)
	}
}

func TestAPIKeyRejected(t *testing.T) {
	e := newEnv(t)
	now := time.Unix(1_900_000_000, 0)
	e.srv.now = func() time.Time { return now }
	e.login(t)
	expiring, _ := e.createKey(t, map[string]any{"name": "expiring", "access": "full", "expires_days": 30})
	elsewhere, _ := e.createKey(t, map[string]any{"name": "elsewhere", "access": "full", "addresses": []string{"10.0.0.0/8", "2001:db8::1"}})
	revoked, revokedID := e.createKey(t, map[string]any{"name": "revoked", "access": "full"})
	good, _ := e.createKey(t, map[string]any{"name": "good", "access": "read"})

	if res, _ := e.keyDo(t, "GET", "/api/audit", expiring, nil, nil); res.StatusCode != 200 {
		t.Fatalf("before expiry: %d", res.StatusCode)
	}
	if res, out := e.do(t, "DELETE", "/api/settings/api-keys/"+itoa(int(revokedID)), nil, nil); res.StatusCode != 200 {
		t.Fatalf("revoke: %d %v", res.StatusCode, out)
	}
	if res, _ := e.do(t, "DELETE", "/api/settings/api-keys/"+itoa(int(revokedID)), nil, nil); res.StatusCode != 404 {
		t.Errorf("revoke twice: %d", res.StatusCode)
	}
	now = now.AddDate(0, 0, 30)

	forged := "nefo_" + strings.Repeat("A", apiKeyLen-len("nefo_"))
	for name, key := range map[string]string{"expired": expiring, "wrong address": elsewhere, "revoked": revoked,
		"unknown": forged, "malformed": "nefo_short", "empty": "", "a session token": "abc"} {
		e.srv.limiter.Reset(limiterKey("127.0.0.1"))
		res, out := e.keyDo(t, "GET", "/api/audit", "", nil, map[string]string{"Authorization": "Bearer " + key})
		if res.StatusCode != 401 || len(out) != 1 || out["error"] != "invalid_api_key" || res.Header.Get("WWW-Authenticate") != "Bearer" {
			t.Errorf("%s key: %d %v", name, res.StatusCode, out)
		}
	}
	e.srv.limiter.Reset(limiterKey("127.0.0.1"))
	if res, _ := e.keyDo(t, "GET", "/api/audit", good, nil, nil); res.StatusCode != 200 {
		t.Errorf("good key: %d", res.StatusCode)
	}
	// Each failure is a failed sign-in in the audit log, which says why.
	entries, _ := e.store.RecentAudit(context.Background(), 100)
	why := map[string]string{}
	for _, a := range entries {
		if a.Action == "login_failed" {
			why[a.Username] = a.Detail
		}
	}
	if why["admin (key expiring)"] != "API key expired" || why["admin (key elsewhere)"] != "API key not allowed from this address" ||
		why[""] != "unknown API key" {
		t.Errorf("audit: %v", why)
	}
	for _, a := range entries {
		if a.Action == "api_key_revoked" && (a.Username != "admin" || a.Detail != "revoked") {
			t.Errorf("revoke audit %+v", a)
		}
	}
}

func TestAPIKeyRateLimit(t *testing.T) {
	e := newEnv(t)
	e.login(t)
	logs := e.captureLog()
	good, _ := e.createKey(t, map[string]any{"name": "good", "access": "full"})
	for i := 0; i < 5; i++ {
		if res, _ := e.keyDo(t, "GET", "/api/audit", "nefo_guess"+itoa(i), nil, nil); res.StatusCode != 401 {
			t.Fatalf("guess %d: %d", i, res.StatusCode)
		}
	}
	res, out := e.keyDo(t, "GET", "/api/audit", good, nil, nil)
	if res.StatusCode != 429 || out["error"] != "too_many_attempts" || res.Header.Get("Retry-After") == "" {
		t.Fatalf("after 5 bad keys, a good key: %d %v", res.StatusCode, out)
	}
	// It is the sign-in limit: the password is locked out too.
	if res, _ := e.do(t, "POST", "/api/auth/login", map[string]string{"username": "admin", "password": testPassword}, nil); res.StatusCode != 429 {
		t.Errorf("login after 5 bad keys: %d", res.StatusCode)
	}
	// fail2ban's panel jail sees each of them.
	f2b := regexp.MustCompile(`^.*msg=audit .* action=(?:login_failed|login_blocked) ip=(\S+)$`)
	failed, blocked := 0, 0
	for _, line := range strings.Split(logs.String(), "\n") {
		m := f2b.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if m[1] != "127.0.0.1" {
			t.Errorf("jail would ban %q: %s", m[1], line)
		}
		if strings.Contains(line, "action=login_failed") {
			failed++
		} else {
			blocked++
		}
	}
	if failed != 5 || blocked != 2 {
		t.Errorf("jail matches: %d failed, %d blocked\n%s", failed, blocked, logs)
	}
}

// The key never reaches the service log or the audit log.
func TestAPIKeyNotLogged(t *testing.T) {
	e := newEnv(t)
	e.login(t)
	logs := e.captureLog()
	note := e.newSite(t)
	full, id := e.createKey(t, map[string]any{"name": "ci", "access": "full"})
	ro, _ := e.createKey(t, map[string]any{"name": "ro", "access": "read"})
	e.keyDo(t, "PUT", note, full, map[string]string{"note": "x"}, nil)
	e.keyDo(t, "PUT", note, ro, map[string]string{"note": "x"}, nil)
	e.keyDo(t, "GET", "/api/terminal", full, nil, nil)
	near := full[:len(full)-1] + "A" // a near miss
	if near == full {
		near = full[:len(full)-1] + "B"
	}
	e.keyDo(t, "GET", "/api/audit", near, nil, nil)
	e.do(t, "DELETE", "/api/settings/api-keys/"+itoa(int(id)), nil, nil)
	e.keyDo(t, "GET", "/api/audit", full, nil, nil) // revoked

	entries, _ := e.store.RecentAudit(context.Background(), 100)
	raw, _ := json.Marshal(entries)
	for _, secret := range []string{full[apiKeyShown:], ro[apiKeyShown:], full[len(apiKeyPrefix) : apiKeyShown+4], auth.HashToken(full)} {
		if strings.Contains(logs.String(), secret) || strings.Contains(string(raw), secret) {
			t.Fatalf("a key is in the logs:\n%s\n%s", logs, raw)
		}
	}
	if len(entries) < 6 {
		t.Fatalf("audit: %s", raw)
	}
}

func TestAPIKeyLastUsed(t *testing.T) {
	e := newEnv(t)
	now := time.Unix(1_900_000_000, 0)
	e.srv.now = func() time.Time { return now }
	e.login(t)
	key, id := e.createKey(t, map[string]any{"name": "ci", "access": "read"})
	lastUsed := func() time.Time {
		k, err := e.store.APIKeyByID(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		return k.LastUsedAt
	}
	if !lastUsed().IsZero() {
		t.Fatal("used before use")
	}
	first := now
	for i := 0; i < 5; i++ {
		e.keyDo(t, "GET", "/api/audit", key, nil, nil)
		now = now.Add(10 * time.Second)
	}
	if got := lastUsed(); !got.Equal(first) {
		t.Fatalf("last used %v, want %v (once a minute)", got, first)
	}
	now = first.Add(time.Minute)
	e.keyDo(t, "GET", "/api/audit", key, nil, nil)
	if got := lastUsed(); !got.Equal(now) {
		t.Fatalf("last used %v, want %v", got, now)
	}
	res, out := e.do(t, "GET", "/api/settings/api-keys", nil, nil)
	k := out["keys"].([]any)[0].(map[string]any)
	if res.StatusCode != 200 || k["last_used_ip"] != "127.0.0.1" || k["last_used_at"] == nil {
		t.Fatalf("list: %v", k)
	}
}

// ---- the routes an API key must never reach ----

// apiRoute is one mux.Handle or mux.HandleFunc call of the package.
type apiRoute struct {
	pattern string
	wrapper string // "requireAuth", "sessionOnly", or "" for none
	handler string // the method that serves it, when it is one
}

// keyForbiddenOps are what makes a route session-only when its handler
// reaches them: shells over WebSocket, the Adminer proxy, the credentials
// and sessions of the account, and the API keys themselves. Some do not
// exist yet; they catch the obvious names of future ones.
var keyForbiddenOps = map[string]string{
	"ws.Upgrade":               "opens a WebSocket (a shell)",
	"httputil.ReverseProxy":    "proxies to another program",
	"store.SetPassword":        "changes the password",
	"store.SetTOTP":            "changes two-factor sign-in",
	"store.CreateSession":      "starts a session",
	"store.DeleteSession":      "signs out a session",
	"store.DeleteSessions":     "signs out sessions",
	"store.DeleteUserSessions": "signs out sessions",
	"store.PurgeUserSessions":  "signs out sessions",
	"store.SetUsername":        "changes the username",
	"store.RenameUser":         "changes the username",
	"store.CreateUser":         "creates an account",
	"store.DeleteUser":         "deletes an account",
	"store.CreateAPIKey":       "manages API keys",
	"store.DeleteAPIKey":       "manages API keys",
	"store.APIKeys":            "manages API keys",
	"store.APIKeyByID":         "manages API keys",
	"terminal.Start":           "starts a shell",
}

// sessionOnlyByPath are the paths that are session-only whatever their
// handler does.
func sessionOnlyByPath(path string) string {
	switch {
	case strings.HasPrefix(path, "/api/auth/"):
		return "is about the account's sign-in"
	case strings.Contains(path, "terminal"), strings.Contains(path, "/shell"), strings.Contains(path, "/exec"):
		return "is a shell"
	case strings.HasPrefix(path, "/adminer"), strings.HasPrefix(path, "/phpmyadmin"):
		return "is a database tool behind the sign-in"
	case strings.Contains(path, "api-keys"), strings.Contains(path, "apikeys"), strings.Contains(path, "/sessions"):
		return "manages keys or sessions"
	}
	return ""
}

func parseAPIPackage(t *testing.T) []*ast.File {
	t.Helper()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, p, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	return files
}

// callGraph lists what each function of the package refers to: "s.name"
// for methods of the Server, "name" for functions, "pkg.Name" for other
// packages and "store.Name" for methods of the store.
func callGraph(files []*ast.File) map[string][]string {
	g := map[string][]string{}
	for _, f := range files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			key, recv := fd.Name.Name, ""
			if fd.Recv != nil && len(fd.Recv.List) == 1 && len(fd.Recv.List[0].Names) == 1 {
				recv = fd.Recv.List[0].Names[0].Name
				key = "s." + fd.Name.Name
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.SelectorExpr:
					switch inner := x.X.(type) {
					case *ast.Ident:
						if inner.Name == recv {
							g[key] = append(g[key], "s."+x.Sel.Name)
						} else {
							g[key] = append(g[key], inner.Name+"."+x.Sel.Name)
						}
					case *ast.SelectorExpr:
						if inner.Sel.Name == "store" {
							g[key] = append(g[key], "store."+x.Sel.Name)
						}
					}
				case *ast.Ident:
					g[key] = append(g[key], x.Name)
				}
				return true
			})
		}
	}
	return g
}

// reaches returns the first forbidden operation that fn leads to.
func reaches(g map[string][]string, fn string, seen map[string]bool) string {
	if why, ok := keyForbiddenOps[fn]; ok {
		return fn + " (" + why + ")"
	}
	if seen[fn] {
		return ""
	}
	seen[fn] = true
	for _, c := range g[fn] {
		if why := reaches(g, c, seen); why != "" {
			return why
		}
	}
	return ""
}

func collectRoutes(t *testing.T, files []*ast.File) []apiRoute {
	t.Helper()
	var routes []apiRoute
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (sel.Sel.Name != "Handle" && sel.Sel.Name != "HandleFunc") {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); !ok || id.Name != "mux" {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				t.Errorf("a route with a computed pattern: %s", call.Args[0])
				return true
			}
			rt := apiRoute{}
			rt.pattern, _ = strconv.Unquote(lit.Value)
			h := call.Args[1]
			if c, ok := h.(*ast.CallExpr); ok {
				if fs, ok := c.Fun.(*ast.SelectorExpr); ok && len(c.Args) == 1 &&
					(fs.Sel.Name == "requireAuth" || fs.Sel.Name == "sessionOnly") {
					rt.wrapper, h = fs.Sel.Name, c.Args[0]
				}
			}
			switch x := h.(type) {
			case *ast.SelectorExpr:
				rt.handler = "s." + x.Sel.Name
			case *ast.CallExpr:
				if fs, ok := x.Fun.(*ast.SelectorExpr); ok {
					rt.handler = "s." + fs.Sel.Name
				}
			}
			routes = append(routes, rt)
			return true
		})
	}
	return routes
}

func splitPattern(p string) (method, path string) {
	if m, rest, ok := strings.Cut(p, " "); ok {
		return m, rest
	}
	return "GET", p
}

var pathWildcard = regexp.MustCompile(`\{[^}]*\}`)

// TestSessionOnlyRoutes walks every route of the package: those that lead
// to a shell, Adminer, the account's credentials and sessions or the API
// keys must be wrapped in s.sessionOnly, and each of those answers even a
// full key with 403.
func TestSessionOnlyRoutes(t *testing.T) {
	files := parseAPIPackage(t)
	routes := collectRoutes(t, files)
	if len(routes) < 100 {
		t.Fatalf("found %d routes: the test no longer understands how routes are registered", len(routes))
	}
	g := callGraph(files)
	// The handlers' calls are found, whatever their path says…
	for h, op := range map[string]string{"s.handleTerminal": "ws.Upgrade", "s.handleContainerTerminal": "ws.Upgrade",
		"s.handleAdminer": "httputil.ReverseProxy", "s.handleChangePassword": "store.SetPassword",
		"s.handleTOTPDisable": "store.SetTOTP", "s.handleLogout": "store.DeleteSession",
		"s.handleCreateAPIKey": "store.CreateAPIKey", "s.handleLogin": "store.CreateSession"} {
		if got := reaches(g, h, map[string]bool{}); !strings.HasPrefix(got, op+" ") {
			t.Errorf("%s reaches %q, want %s", h, got, op)
		}
	}
	// …and look-alikes are not: a database user's password is not the panel's.
	for _, h := range []string{"s.handleDBPassword", "s.handleWPPassword", "s.handleSiteNote", "s.handleListSites", "s.handleDockerLogs"} {
		if got := reaches(g, h, map[string]bool{}); got != "" {
			t.Errorf("%s reaches %s", h, got)
		}
	}
	var only []apiRoute
	patterns := map[string]bool{}
	for _, rt := range routes {
		patterns[rt.pattern] = true
		_, path := splitPattern(rt.pattern)
		why := sessionOnlyByPath(path)
		if why == "" && rt.handler != "" {
			why = reaches(g, rt.handler, map[string]bool{})
		}
		switch {
		case rt.wrapper == "sessionOnly", rt.wrapper == "requireAuth" && sessionOnlyPath(path):
			only = append(only, rt) // by its wrapper, or under sessionOnlyPrefixes
		case rt.wrapper == "requireAuth" && why != "":
			t.Errorf("%s: its handler %s %s: register it with s.sessionOnly, not s.requireAuth", rt.pattern, rt.handler, why)
		}
	}
	for _, p := range []string{
		"POST /api/auth/logout", "GET /api/auth/me", "POST /api/auth/password",
		"POST /api/auth/2fa/setup", "POST /api/auth/2fa/enable", "POST /api/auth/2fa/disable",
		"GET /api/settings/api-keys", "POST /api/settings/api-keys", "DELETE /api/settings/api-keys/{id}",
		"GET /api/terminal", "GET /api/docker/containers/{id}/terminal", "/adminer/",
	} {
		found := false
		for _, rt := range only {
			found = found || rt.pattern == p
		}
		if !found {
			t.Errorf("%s is not session-only (registered: %v)", p, patterns[p])
		}
	}

	// Each answers a full key with 403 before its handler runs.
	e := newEnv(t)
	e.login(t)
	full, _ := e.createKey(t, map[string]any{"name": "full", "access": "full"})
	for _, rt := range only {
		method, path := splitPattern(rt.pattern)
		path = pathWildcard.ReplaceAllString(path, "1")
		var body any
		if method != "GET" && method != "HEAD" {
			body = map[string]any{}
		}
		var hdr map[string]string
		if method == "GET" { // as a WebSocket handshake would
			hdr = map[string]string{"Connection": "Upgrade", "Upgrade": "websocket", "Sec-WebSocket-Version": "13",
				"Sec-WebSocket-Key": "dGhlIHNhbXBsZSBub25jZQ==", "Origin": e.ts.URL}
		}
		res, out := e.keyDo(t, method, path, full, body, hdr)
		if res.StatusCode != 403 || out["error"] != "api_key_forbidden" {
			t.Errorf("%s with a full key: %d %v", rt.pattern, res.StatusCode, out)
		}
	}
	// Still there for the session.
	if res, _ := e.do(t, "GET", "/api/settings/api-keys", nil, nil); res.StatusCode != 200 {
		t.Errorf("session: %d", res.StatusCode)
	}
	// And the refusals are audited.
	entries, _ := e.store.RecentAudit(context.Background(), 100)
	n := 0
	for _, a := range entries {
		if a.Action == "api_key_refused" && a.Username == "admin (key full)" {
			n++
		}
	}
	if n != len(only) {
		t.Errorf("%d refusals audited, want %d", n, len(only))
	}
}

// The paths under sessionOnlyPrefixes refuse a key even when their route
// uses s.requireAuth, such as a database tool proxied like Adminer.
func TestSessionOnlyPrefixes(t *testing.T) {
	e := newEnv(t)
	e.login(t)
	full, _ := e.createKey(t, map[string]any{"name": "full", "access": "full"})
	called := false
	h := e.srv.requireAuth(func(w http.ResponseWriter, r *http.Request, a *authed) {
		called = true
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	for _, p := range []string{"/phpmyadmin/", "/phpmyadmin", "/phpmyadmin/index.php?route=/sql", "/adminer/",
		"/api/auth/sessions", "/api/settings/api-keys/7"} {
		req := httptest.NewRequest("GET", p, nil)
		req.Header.Set("Authorization", "Bearer "+full)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != 403 || !strings.Contains(rec.Body.String(), "api_key_forbidden") || called {
			t.Errorf("%s: %d %s (handler ran: %v)", p, rec.Code, rec.Body, called)
		}
	}
	// Other paths reach the handler.
	req := httptest.NewRequest("GET", "/api/phpmyadmin-status", nil)
	req.Header.Set("Authorization", "Bearer "+full)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 || !called {
		t.Errorf("an ordinary path: %d %s", rec.Code, rec.Body)
	}
}

// A read-only key can't read passwords, secrets, files or whole databases;
// a full key can. Every refused pattern must be a real route, so a renamed
// route can't silently fall out of the list.
func TestReadOnlyRefusedReads(t *testing.T) {
	registered := map[string]bool{}
	for _, rt := range collectRoutes(t, parseAPIPackage(t)) {
		registered[rt.pattern] = true
	}
	for p := range readOnlyRefused {
		if !registered[p] {
			t.Errorf("readOnlyRefused has %q, which is not a route", p)
		}
	}

	e := newEnv(t)
	e.login(t)
	ro, _ := e.createKey(t, map[string]any{"name": "monitor", "access": "read"})
	full, _ := e.createKey(t, map[string]any{"name": "full", "access": "full"})
	for _, path := range []string{"/api/databases/1/credentials", "/api/databases/1/export", "/api/files/download?path=/etc/shadow",
		"/api/files/content?path=/etc/shadow", "/api/backups/1/download", "/api/apps/installed/x/credentials"} {
		res, out := e.keyDo(t, "GET", path, ro, nil, nil)
		if res.StatusCode != 403 || out["error"] != "api_key_read_only" {
			t.Errorf("read-only GET %s: %d %v", path, res.StatusCode, out)
		}
		if res, out := e.keyDo(t, "GET", path, full, nil, nil); res.StatusCode == 403 {
			t.Errorf("full GET %s: %d %v", path, res.StatusCode, out)
		}
	}
	// Other reads still work.
	if res, out := e.keyDo(t, "GET", "/api/sites", ro, nil, nil); res.StatusCode != 200 {
		t.Errorf("read-only GET /api/sites: %d %v", res.StatusCode, out)
	}
}

func TestBearerToken(t *testing.T) {
	for _, c := range []struct {
		hdr   []string
		token string
		ok    bool
	}{
		{nil, "", false},
		{[]string{"Bearer nefo_x"}, "nefo_x", true},
		{[]string{"BEARER   nefo_x  "}, "nefo_x", true},
		{[]string{"Bearer"}, "", true},
		{[]string{"Basic dXNlcjpwdw=="}, "", false},
		{[]string{"Basic dXNlcjpwdw==", "Bearer nefo_x"}, "nefo_x", true},
		{[]string{"Bearer nefo_x", "Bearer nefo_y"}, "", true},
	} {
		r, _ := http.NewRequest("GET", "/", nil)
		for _, h := range c.hdr {
			r.Header.Add("Authorization", h)
		}
		if token, ok := bearerToken(r); token != c.token || ok != c.ok {
			t.Errorf("%q: %q %v", c.hdr, token, ok)
		}
	}
}

func TestKeyAddresses(t *testing.T) {
	got, err := parseKeyAddresses([]string{"203.0.113.9", "203.0.113.9", "198.51.100.77/24", "2001:DB8::1", "2001:db8:1::/48", "::ffff:10.0.0.1"})
	want := []string{"203.0.113.9", "198.51.100.0/24", "2001:db8::1", "2001:db8:1::/48", "10.0.0.1"}
	if err != nil || strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("%v, %v", got, err)
	}
	for _, c := range []struct {
		ip string
		ok bool
	}{
		{"203.0.113.9", true}, {"203.0.113.10", false}, {"198.51.100.1", true}, {"198.51.101.1", false},
		{"2001:db8::1", true}, {"2001:db8::2", false}, {"2001:db8:1:ffff::5", true}, {"::ffff:10.0.0.1", true},
		{"::ffff:203.0.113.9", true}, {"10.0.0.2", false}, {"not an ip", false}, {"fe80::1%eth0", false},
	} {
		if addressAllowed(got, c.ip) != c.ok {
			t.Errorf("%s: %v", c.ip, !c.ok)
		}
	}
	if !addressAllowed(nil, "192.0.2.1") {
		t.Error("an empty list allows any address")
	}
}
