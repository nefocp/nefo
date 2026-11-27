package hosting

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/appstore"
	"github.com/nefocp/nefo/internal/store"
)

// HTTP set-up steps: nested JSON bodies with placeholders, headers (Host
// too), and values saved from one answer for the next request (Memos signs
// in, then changes a setting with the token).
func TestHTTPStepsSaveAndHeaders(t *testing.T) {
	var got []map[string]any
	var hosts, auths []string
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var body map[string]any
		json.Unmarshal(b, &body)
		got = append(got, body)
		hosts = append(hosts, r.Host)
		auths = append(auths, r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/signin":
			w.Write([]byte(`{"user":{"id":7},"accessToken":"eyJ0.abc-_def"}`))
		case "/settings":
			if r.Method != http.MethodPatch {
				w.WriteHeader(405)
			}
		}
	}))
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv.Listener = l
	srv.Start()
	defer srv.Close()
	port := l.Addr().(*net.TCPAddr).Port

	inst := &store.App{Name: "memos_example_com", Port: port}
	env := &appstore.Env{Name: inst.Name, Domain: "memos.example.com", Secrets: map[string]string{"admin_password": "Secret123"}}
	m := &Manager{}
	ctx := context.Background()
	steps := []*appstore.HTTPStep{
		{Method: "POST", Path: "/signin", Expect: []int{200}, Save: map[string]string{"token": "accessToken", "uid": "user.id"},
			JSON: map[string]any{"passwordCredentials": map[string]any{"username": "admin", "password": "${secret.admin_password}"}}},
		{Method: "PATCH", Path: "/settings", Expect: []int{200},
			Headers: map[string]string{"Authorization": "Bearer ${step.token}", "Host": "${domain}"},
			JSON:    map[string]any{"general": map[string]any{"closed": true, "n": 3.0, "list": []any{"${step.uid}"}}}},
	}
	for _, st := range steps {
		if err := m.httpStep(ctx, inst, env, st, discardLogger{}); err != nil {
			t.Fatal(err)
		}
	}
	if pw := got[0]["passwordCredentials"].(map[string]any)["password"]; pw != "Secret123" {
		t.Errorf("nested placeholder: %v", pw)
	}
	g := got[1]["general"].(map[string]any)
	if g["closed"] != true || g["n"] != 3.0 || g["list"].([]any)[0] != "7" {
		t.Errorf("JSON body: %v", g)
	}
	if auths[1] != "Bearer eyJ0.abc-_def" || hosts[1] != "memos.example.com" || hosts[0] != "127.0.0.1:"+strconv.Itoa(port) {
		t.Errorf("headers: %v %v", auths, hosts)
	}
	// A value the answer does not have fails the step.
	bad := &appstore.HTTPStep{Method: "POST", Path: "/signin", Expect: []int{200}, Save: map[string]string{"x": "missing.path"}}
	if err := m.httpStep(ctx, inst, env, bad, discardLogger{}); err == nil {
		t.Error("missing JSON path accepted")
	}
}

// A site volume is created inside the website's folder without following a
// link the site's user put there.
func TestSiteVolumeDir(t *testing.T) {
	site := t.TempDir()
	d := appstore.Dir{Path: filepath.ToSlash(filepath.Join(site, "music")), UID: -1, GID: -1, Mode: 0o750, Site: true}
	d.Path = filepath.ToSlash(site) + "/music"
	if err := siteVolumeDir(filepath.ToSlash(site), d); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(filepath.Join(site, "music")); err != nil || !fi.IsDir() {
		t.Fatalf("music: %v", err)
	}
	// An existing folder keeps its files.
	os.WriteFile(filepath.Join(site, "music", "a.mp3"), []byte("x"), 0o644)
	if err := siteVolumeDir(filepath.ToSlash(site), d); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(site, "music", "a.mp3")); err != nil {
		t.Error("existing files gone")
	}
	// A file in the way, or a path outside the site, is refused.
	os.WriteFile(filepath.Join(site, "media"), []byte("x"), 0o644)
	if err := siteVolumeDir(filepath.ToSlash(site), appstore.Dir{Path: filepath.ToSlash(site) + "/media", Mode: 0o750, UID: -1}); err == nil {
		t.Error("a file was taken for the folder")
	}
	if err := siteVolumeDir(filepath.ToSlash(site), appstore.Dir{Path: "/etc/x", Mode: 0o750, UID: -1}); err == nil {
		t.Error("path outside the site accepted")
	}
	if runtime.GOOS == "windows" {
		return
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(site, "video")); err != nil {
		t.Fatal(err)
	}
	if err := siteVolumeDir(filepath.ToSlash(site), appstore.Dir{Path: filepath.ToSlash(site) + "/video", Mode: 0o777, UID: -1}); err == nil {
		t.Error("a link was followed")
	}
	if fi, _ := os.Stat(outside); fi.Mode().Perm() == 0o777 {
		t.Error("the link's target was changed")
	}
}

// An app still loading (503) is asked again; another error answer is not.
func TestHTTPStepRetriesWhileLoading(t *testing.T) {
	defer func(f, w time.Duration) { stepRetryFor, stepRetryWait = f, w }(stepRetryFor, stepRetryWait)
	stepRetryFor, stepRetryWait = 5*time.Second, 10*time.Millisecond
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		b, _ := io.ReadAll(r.Body)
		switch {
		case r.URL.Path == "/bad":
			w.WriteHeader(500)
		case calls < 3:
			w.WriteHeader(503)
		case string(b) != `{"a":"x"}`:
			w.WriteHeader(400) // the body must be sent again on every try
		}
	}))
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port
	inst := &store.App{Name: "jf_example_com", Port: port}
	env := &appstore.Env{Name: inst.Name}
	m := &Manager{}
	st := &appstore.HTTPStep{Method: "POST", Path: "/startup", JSON: map[string]any{"a": "x"}, Expect: []int{200}}
	if err := m.httpStep(context.Background(), inst, env, st, discardLogger{}); err != nil || calls != 3 {
		t.Fatalf("after %d calls: %v", calls, err)
	}
	calls = 0
	st.Path = "/bad"
	if err := m.httpStep(context.Background(), inst, env, st, discardLogger{}); err == nil || calls != 1 {
		t.Errorf("500 retried or accepted (%d calls): %v", calls, err)
	}
}
