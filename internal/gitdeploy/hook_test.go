package gitdeploy

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/jobs"
	"github.com/nefocp/nefo/internal/store"
)

const testSecret = "3f6c1d0e9a8b7c6d5e4f3a2b1c0d9e8f7a6b5c4d3e2f1a0b9c8d7e6f5a4b3c2d"

type hookStore map[int64]*store.GitDeploy

func (h hookStore) GitDeploy(_ context.Context, id int64) (*store.GitDeploy, error) {
	if g, ok := h[id]; ok {
		return g, nil
	}
	return nil, store.ErrNotFound
}

// deployRecorder is a DeployFunc that records its triggers and answers
// with the errors in errs, then nil.
type deployRecorder struct {
	mu    sync.Mutex
	calls []Trigger
	errs  []error
}

func (d *deployRecorder) deploy(t Trigger) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, t)
	if len(d.errs) > 0 {
		err := d.errs[0]
		d.errs = d.errs[1:]
		return err
	}
	return nil
}

func (d *deployRecorder) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.calls)
}

func newTestHook() (*HookHandler, *deployRecorder) {
	st := hookStore{
		7: {SiteID: 7, Repo: "https://github.com/o/r.git", Branch: "main", Target: "public", HookSecret: testSecret},
		8: {SiteID: 8, Repo: "https://github.com/o/r.git", Branch: "main", Target: "public"}, // no secret
	}
	d := &deployRecorder{}
	return NewHookHandler(st, d.deploy, nil), d
}

func sign(secret, body string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(body))
	return hex.EncodeToString(m.Sum(nil))
}

const pushMain = `{"ref":"refs/heads/main","after":"1111111111111111111111111111111111111111","deleted":false}`

func hookRequest(t *testing.T, h http.Handler, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:50000"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Real-IP", "140.82.115.1")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func github(body string) map[string]string {
	return map[string]string{"X-GitHub-Event": "push", "X-Hub-Signature-256": "sha256=" + sign(testSecret, body)}
}

func TestHookProviders(t *testing.T) {
	cases := []struct {
		name     string
		headers  map[string]string
		provider string
	}{
		{"github", github(pushMain), "github"},
		{"github upper-case hex", map[string]string{"X-GitHub-Event": "push", "X-Hub-Signature-256": "sha256=" + strings.ToUpper(sign(testSecret, pushMain))}, "github"},
		{"gitlab", map[string]string{"X-Gitlab-Event": "Push Hook", "X-Gitlab-Token": testSecret}, "gitlab"},
		{"gitea", map[string]string{"X-Gitea-Event": "push", "X-Gitea-Signature": sign(testSecret, pushMain)}, "gitea"},
		{"gitea with github's header too", map[string]string{"X-Gitea-Event": "push", "X-GitHub-Event": "push",
			"X-Gitea-Signature": sign(testSecret, pushMain), "X-Hub-Signature-256": "sha256=" + sign(testSecret, pushMain)}, "gitea"},
		{"forgejo", map[string]string{"X-Forgejo-Event": "push", "X-Forgejo-Signature": sign(testSecret, pushMain)}, "gitea"},
	}
	for _, c := range cases {
		h, d := newTestHook()
		w := hookRequest(t, h, "/git/7", pushMain, c.headers)
		if w.Code != http.StatusAccepted || d.count() != 1 {
			t.Errorf("%s: code %d, %d deploys: %s", c.name, w.Code, d.count(), w.Body)
			continue
		}
		got := d.calls[0]
		want := Trigger{SiteID: 7, Provider: c.provider, Ref: "refs/heads/main", Commit: strings.Repeat("1", 40), IP: "140.82.115.1"}
		if got != want {
			t.Errorf("%s: trigger %+v, want %+v", c.name, got, want)
		}
	}
}

func TestHookRejects(t *testing.T) {
	other := "0000000000000000000000000000000000000000000000000000000000000000"
	cases := []struct {
		name    string
		path    string
		body    string
		headers map[string]string
		code    int
	}{
		{"unknown site", "/git/99", pushMain, github(pushMain), http.StatusNotFound},
		{"site without secret", "/git/8", pushMain, github(pushMain), http.StatusNotFound},
		{"bad path", "/git/7x", pushMain, github(pushMain), http.StatusNotFound},
		{"leading zero", "/git/07", pushMain, github(pushMain), http.StatusNotFound},
		{"other path", "/other/7", pushMain, github(pushMain), http.StatusNotFound},
		{"no signature", "/git/7", pushMain, map[string]string{"X-GitHub-Event": "push"}, http.StatusUnauthorized},
		{"github wrong secret", "/git/7", pushMain, map[string]string{"X-GitHub-Event": "push", "X-Hub-Signature-256": "sha256=" + sign(other, pushMain)}, http.StatusUnauthorized},
		{"github signature of another body", "/git/7", pushMain, map[string]string{"X-GitHub-Event": "push", "X-Hub-Signature-256": "sha256=" + sign(testSecret, pushMain+" ")}, http.StatusUnauthorized},
		{"github without sha256= prefix", "/git/7", pushMain, map[string]string{"X-GitHub-Event": "push", "X-Hub-Signature-256": sign(testSecret, pushMain)}, http.StatusUnauthorized},
		{"github not hex", "/git/7", pushMain, map[string]string{"X-GitHub-Event": "push", "X-Hub-Signature-256": "sha256=zz"}, http.StatusUnauthorized},
		{"gitlab wrong token", "/git/7", pushMain, map[string]string{"X-Gitlab-Event": "Push Hook", "X-Gitlab-Token": other}, http.StatusUnauthorized},
		{"gitlab token prefix", "/git/7", pushMain, map[string]string{"X-Gitlab-Event": "Push Hook", "X-Gitlab-Token": testSecret[:32]}, http.StatusUnauthorized},
		{"gitea wrong signature", "/git/7", pushMain, map[string]string{"X-Gitea-Event": "push", "X-Gitea-Signature": sign(other, pushMain)}, http.StatusUnauthorized},
		// A valid GitLab token does not rescue a bad GitHub signature.
		{"bad github signature with gitlab token", "/git/7", pushMain, map[string]string{"X-Hub-Signature-256": "sha256=" + sign(other, pushMain), "X-Gitlab-Token": testSecret}, http.StatusUnauthorized},
		{"not JSON", "/git/7", "hello", github("hello"), http.StatusBadRequest},
	}
	for _, c := range cases {
		h, d := newTestHook()
		w := hookRequest(t, h, c.path, c.body, c.headers)
		if w.Code != c.code || d.count() != 0 {
			t.Errorf("%s: code %d (want %d), %d deploys: %s", c.name, w.Code, c.code, d.count(), w.Body)
		}
	}

	h, d := newTestHook()
	req := httptest.NewRequest(http.MethodGet, "/git/7", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed || d.count() != 0 {
		t.Errorf("GET: code %d", w.Code)
	}

	big := `{"ref":"refs/heads/main","x":"` + strings.Repeat("a", 1<<20) + `"}`
	w = hookRequest(t, h, "/git/7", big, github(big))
	if w.Code != http.StatusRequestEntityTooLarge || d.count() != 0 {
		t.Errorf("1 MB body: code %d", w.Code)
	}
}

func TestHookBranchFilter(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		headers func(string) map[string]string
		code    int
		deploys int
	}{
		{"configured branch", pushMain, github, http.StatusAccepted, 1},
		{"other branch", `{"ref":"refs/heads/dev","after":"2222222222222222222222222222222222222222"}`, github, http.StatusOK, 0},
		{"branch with the same suffix", `{"ref":"refs/heads/feature/main"}`, github, http.StatusOK, 0},
		{"tag named like the branch", `{"ref":"refs/tags/main"}`, github, http.StatusOK, 0},
		{"deleted on github", `{"ref":"refs/heads/main","after":"0000000000000000000000000000000000000000","deleted":true}`, github, http.StatusOK, 0},
		{"deleted on gitlab", `{"ref":"refs/heads/main","after":"0000000000000000000000000000000000000000"}`,
			func(b string) map[string]string {
				return map[string]string{"X-Gitlab-Event": "Push Hook", "X-Gitlab-Token": testSecret}
			}, http.StatusOK, 0},
		{"github ping", `{"zen":"Keep it logically awesome.","hook_id":1}`,
			func(b string) map[string]string {
				return map[string]string{"X-GitHub-Event": "ping", "X-Hub-Signature-256": "sha256=" + sign(testSecret, b)}
			}, http.StatusOK, 0},
		{"github release event", `{"action":"published"}`,
			func(b string) map[string]string {
				return map[string]string{"X-GitHub-Event": "release", "X-Hub-Signature-256": "sha256=" + sign(testSecret, b)}
			}, http.StatusOK, 0},
		{"gitlab tag push", `{"ref":"refs/heads/main"}`,
			func(b string) map[string]string {
				return map[string]string{"X-Gitlab-Event": "Tag Push Hook", "X-Gitlab-Token": testSecret}
			}, http.StatusOK, 0},
		{"no event header", pushMain,
			func(b string) map[string]string {
				return map[string]string{"X-Hub-Signature-256": "sha256=" + sign(testSecret, b)}
			}, http.StatusAccepted, 1},
	}
	for _, c := range cases {
		h, d := newTestHook()
		w := hookRequest(t, h, "/git/7", c.body, c.headers(c.body))
		if w.Code != c.code || d.count() != c.deploys {
			t.Errorf("%s: code %d (want %d), %d deploys (want %d): %s", c.name, w.Code, c.code, d.count(), c.deploys, w.Body)
		}
	}
}

func TestHookFormPayload(t *testing.T) {
	h, d := newTestHook()
	body := "payload=" + url.QueryEscape(pushMain)
	req := httptest.NewRequest(http.MethodPost, "/git/7", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", "sha256="+sign(testSecret, body))
	req.RemoteAddr = "192.0.2.10:4444" // not through Nginx: X-Real-IP is not trusted
	req.Header.Set("X-Real-IP", "10.0.0.1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted || d.count() != 1 || d.calls[0].IP != "192.0.2.10" {
		t.Fatalf("form payload: code %d, calls %+v", w.Code, d.calls)
	}
}

func TestHookBusyRetries(t *testing.T) {
	h, d := newTestHook()
	h.retryEvery, h.retryFor = 50*time.Millisecond, 5*time.Second
	busy := fmt.Errorf("start: %w", jobs.ErrBusy)
	d.errs = []error{busy, busy, busy}

	w := hookRequest(t, h, "/git/7", pushMain, github(pushMain))
	if w.Code != http.StatusAccepted || !strings.Contains(w.Body.String(), "queued") {
		t.Fatalf("busy: code %d: %s", w.Code, w.Body)
	}
	// A second push while one is waiting does not queue another deploy.
	hookRequest(t, h, "/git/7", pushMain, github(pushMain))
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		h.mu.Lock()
		waiting := h.pending[7]
		h.mu.Unlock()
		if !waiting {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	// 1st push (busy), 2nd push (busy), then retries: busy, nil.
	if n := d.count(); n != 4 {
		t.Fatalf("%d deploy attempts, want 4", n)
	}
	time.Sleep(150 * time.Millisecond)
	if n := d.count(); n != 4 {
		t.Fatalf("kept retrying after the deploy started: %d attempts", n)
	}

	// Another error is not retried.
	h2, d2 := newTestHook()
	h2.retryEvery = time.Millisecond
	d2.errs = []error{fmt.Errorf("site deleted")}
	w = hookRequest(t, h2, "/git/7", pushMain, github(pushMain))
	time.Sleep(20 * time.Millisecond)
	if w.Code != http.StatusInternalServerError || d2.count() != 1 {
		t.Fatalf("other error: code %d, %d attempts", w.Code, d2.count())
	}
}

func TestVerifyEmptySecret(t *testing.T) {
	h := http.Header{}
	h.Set("X-Gitlab-Token", "")
	h.Set("X-Hub-Signature-256", "sha256="+sign("", "x"))
	if _, ok := verify(h, []byte("x"), ""); ok {
		t.Fatal("an empty secret verified")
	}
}

func TestListenAndServe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	h, _ := newTestHook()
	done := make(chan error, 1)
	go func() { done <- ListenAndServe(ctx, "127.0.0.1:0", h) }()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("ListenAndServe did not stop")
	}
}

func TestRemoteIP(t *testing.T) {
	for _, c := range []struct {
		peer, realIP, xff, want string
	}{
		{"127.0.0.1:5000", "140.82.115.1", "", "140.82.115.1"},           // Nginx, Apache
		{"127.0.0.1:5000", "", "10.9.9.9, 140.82.115.2", "140.82.115.2"}, // OpenLiteSpeed appends the sender
		{"127.0.0.1:5000", "", "", "127.0.0.1"},                          // a local sender
		{"192.0.2.10:4444", "10.0.0.1", "10.0.0.2", "192.0.2.10"},        // not through the web server
		{"127.0.0.1:5000", "not-an-ip", "also-not", "127.0.0.1"},
	} {
		req := httptest.NewRequest(http.MethodPost, "/git/7", nil)
		req.RemoteAddr = c.peer
		if c.realIP != "" {
			req.Header.Set("X-Real-IP", c.realIP)
		}
		if c.xff != "" {
			req.Header.Set("X-Forwarded-For", c.xff)
		}
		if got := remoteIP(req); got != c.want {
			t.Errorf("%+v: got %s", c, got)
		}
	}
}
