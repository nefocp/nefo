package api

import (
	"net/http"
	"testing"
)

// The System page's endpoints need a session, and refuse bad input before
// anything runs (nothing here touches the system: the cases are refused by
// the checks, or because the machine is not a Debian or Ubuntu server).
func TestSystemToolsEndpoints(t *testing.T) {
	e := newEnv(t)
	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/api/system/updates"},
		{http.MethodPost, "/api/system/updates/install"},
		{http.MethodPost, "/api/system/swap"},
		{http.MethodPut, "/api/system/hostname"},
		{http.MethodPut, "/api/system/network/bbr"},
		{http.MethodPost, "/api/system/cleanup/apt"},
	} {
		if res, _ := e.do(t, c.method, c.path, map[string]any{}, nil); res.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s %s without a session: %d", c.method, c.path, res.StatusCode)
		}
	}
	e.login(t)
	if res, _ := e.do(t, http.MethodPut, "/api/system/hostname", map[string]any{"name": "x"}, nil); res.StatusCode != http.StatusBadRequest {
		t.Errorf("unknown field: %d", res.StatusCode)
	}
	refused := func(path string, body any, code string) {
		t.Helper()
		want := code
		if !e.srv.hosting.Supported() {
			want = "unsupported"
		}
		if res, out := e.do(t, http.MethodPost, path, body, nil); out["error"] != want {
			t.Errorf("POST %s: %d %v, want %s", path, res.StatusCode, out, want)
		}
	}
	refused("/api/system/swap", map[string]any{"size_mb": 3000}, "swap_size_invalid")
	refused("/api/system/cleanup/everything", map[string]any{}, "cleanup_unknown")
}
