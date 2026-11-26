package api

import (
	"net/http"
	"testing"
)

func TestUISummaryEndpoints(t *testing.T) {
	e := newEnv(t)
	if res, _ := e.do(t, http.MethodGet, "/api/jobs/summary", nil, nil); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("job summary without a session: %d, want 401", res.StatusCode)
	}
	e.login(t)

	res, body := e.do(t, http.MethodGet, "/api/jobs/summary", nil, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("job summary: %d", res.StatusCode)
	}
	if jobs, ok := body["jobs"].([]any); !ok || len(jobs) != 0 {
		t.Errorf("job summary of a new server = %v, want an empty list", body["jobs"])
	}

	res, body = e.do(t, http.MethodGet, "/api/system/addresses", nil, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("addresses: %d", res.StatusCode)
	}
	if _, ok := body["addresses"].([]any); !ok {
		t.Errorf("addresses = %v, want a list", body["addresses"])
	}
	if _, ok := body["primary"].(string); !ok {
		t.Errorf("primary = %v, want a string", body["primary"])
	}
}
