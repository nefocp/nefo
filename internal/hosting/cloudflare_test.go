package hosting

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeCloudflare is a tiny Cloudflare API with one zone, example.com.
type fakeCloudflare struct {
	mu      sync.Mutex
	records map[string]map[string]string // id → name, content
	auth    []string
	queries []string
}

func (f *fakeCloudflare) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.auth = append(f.auth, r.Header.Get("Authorization"))
	ok := func(result any) {
		json.NewEncoder(w).Encode(map[string]any{"success": true, "errors": []any{}, "result": result})
	}
	if r.Header.Get("Authorization") != "Bearer good-token" {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"success": false, "errors": []any{map[string]any{"code": 1000, "message": "Invalid API Token"}}})
		return
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/user/tokens/verify":
		ok(map[string]any{"status": "active"})
	case r.Method == http.MethodGet && r.URL.Path == "/zones":
		name := r.URL.Query().Get("name")
		f.queries = append(f.queries, name)
		if name == "example.com" || name == "" {
			ok([]any{map[string]any{"id": "zone1", "name": "example.com", "name_servers": []string{"ns1.test", "ns2.test"}}})
		} else {
			ok([]any{})
		}
	case r.Method == http.MethodPost && r.URL.Path == "/zones/zone1/dns_records":
		var in map[string]any
		json.NewDecoder(r.Body).Decode(&in)
		id := "rec" + string(rune('0'+len(f.records)))
		f.records[id] = map[string]string{"name": in["name"].(string), "content": in["content"].(string)}
		ok(map[string]any{"id": id})
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/zones/zone1/dns_records/"):
		delete(f.records, strings.TrimPrefix(r.URL.Path, "/zones/zone1/dns_records/"))
		ok(map[string]any{})
	default:
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"success":false,"errors":[{"code":7003,"message":"not found"}]}`)
	}
}

func (f *fakeCloudflare) txt(name string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, r := range f.records {
		if r["name"] == name {
			out = append(out, r["content"])
		}
	}
	return out
}

func TestCloudflare(t *testing.T) {
	fake := &fakeCloudflare{records: map[string]map[string]string{}}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	oldAPI, oldLookup := cloudflareAPI, lookupTXT
	defer func() { cloudflareAPI, lookupTXT = oldAPI, oldLookup }()
	cloudflareAPI = srv.URL
	var asked []string
	lookupTXT = func(_ context.Context, server, name string) ([]string, error) {
		asked = append(asked, server)
		return fake.txt(name), nil
	}
	ctx := context.Background()

	if err := newCloudflare("bad-token").Verify(ctx); err == nil || !strings.Contains(err.Error(), "Invalid API Token (1000)") {
		t.Fatalf("bad token: %v", err)
	}
	cf := newCloudflare("good-token")
	if err := cf.Verify(ctx); err != nil {
		t.Fatal(err)
	}

	// shop.example.com lives in the example.com zone.
	cleanup, err := cf.Present(ctx, "_acme-challenge.shop.example.com", "v1", nopLogger{})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(fake.queries, ","); got != "shop.example.com,example.com" {
		t.Fatalf("zone lookups = %s", got)
	}
	if got := fake.txt("_acme-challenge.shop.example.com"); len(got) != 1 || got[0] != "v1" {
		t.Fatalf("records = %v", got)
	}
	if strings.Join(asked, ",") != "ns1.test,ns2.test" {
		t.Fatalf("asked %v", asked)
	}
	// "*.example.com" and "example.com" share one record name: both stay
	// until each is cleaned up.
	c2, err := cf.Present(ctx, "_acme-challenge.example.com", "v2", nopLogger{})
	if err != nil {
		t.Fatal(err)
	}
	c3, err := cf.Present(ctx, "_acme-challenge.example.com", "v3", nopLogger{})
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.txt("_acme-challenge.example.com")) != 2 {
		t.Fatal("want two TXT records")
	}
	cleanup()
	c2()
	c3()
	if len(fake.records) != 0 {
		t.Fatalf("records left: %v", fake.records)
	}

	if _, err := cf.Present(ctx, "_acme-challenge.example.org", "v", nopLogger{}); err == nil {
		t.Fatal("found a zone for example.org")
	} else if !strings.Contains(err.Error(), "holds _acme-challenge.example.org") && !strings.Contains(err.Error(), "holds example.org") {
		t.Fatalf("no zone: %v", err)
	}
}

func TestWaitTXT(t *testing.T) {
	old := lookupTXT
	defer func() { lookupTXT = old }()
	calls := 0
	lookupTXT = func(_ context.Context, server, name string) ([]string, error) {
		calls++
		if calls >= 3 {
			return []string{"other", "want"}, nil
		}
		return nil, nil
	}
	start := time.Now()
	if err := waitTXT(context.Background(), []string{"ns"}, "_acme-challenge.x.com", "want", time.Minute); err != nil {
		t.Fatal(err)
	}
	if calls != 3 || time.Since(start) < 5*time.Second {
		t.Fatalf("calls = %d after %v", calls, time.Since(start))
	}
	lookupTXT = func(context.Context, string, string) ([]string, error) { return nil, nil }
	if err := waitTXT(context.Background(), []string{"ns"}, "n", "want", time.Millisecond); err == nil {
		t.Fatal("no timeout")
	}
}

type nopLogger struct{}

func (nopLogger) Write(p []byte) (int, error) { return len(p), nil }
func (nopLogger) Logf(string, ...any)         {}
