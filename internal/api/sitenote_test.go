package api

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/store"
)

func TestSiteNote(t *testing.T) {
	e := newEnv(t)
	e.login(t)
	site := &store.Site{Domain: "note.example", SysUser: "note_example"}
	if err := e.store.CreateSite(context.Background(), site); err != nil {
		t.Fatal(err)
	}
	path := "/api/sites/" + strconv.FormatInt(site.ID, 10) + "/note"

	res, out := e.do(t, "PUT", path, map[string]string{"note": "  Khách hàng A · hết hạn 12/2027  "}, nil)
	if res.StatusCode != 200 || out["note"] != "Khách hàng A · hết hạn 12/2027" {
		t.Fatalf("set note: %d %v", res.StatusCode, out)
	}
	got, _ := e.store.SiteByID(context.Background(), site.ID)
	if got.Note != "Khách hàng A · hết hạn 12/2027" {
		t.Errorf("stored note %q", got.Note)
	}
	for _, bad := range []string{strings.Repeat("é", maxSiteNote+1), "two\nlines", "tab\there", "nul\x00", "sep x"} {
		if res, out := e.do(t, "PUT", path, map[string]string{"note": bad}, nil); res.StatusCode != 400 || out["error"] != "invalid_note" {
			t.Errorf("note %q: %d %v, want 400 invalid_note", bad, res.StatusCode, out)
		}
	}
	if res, _ := e.do(t, "PUT", path, map[string]string{"note": strings.Repeat("é", maxSiteNote)}, nil); res.StatusCode != 200 {
		t.Errorf("a note of exactly %d characters: %d", maxSiteNote, res.StatusCode)
	}
	if res, out := e.do(t, "PUT", path, map[string]string{"note": ""}, nil); res.StatusCode != 200 || out["note"] != "" {
		t.Errorf("clear note: %d %v", res.StatusCode, out)
	}
	if res, _ := e.do(t, "PUT", "/api/sites/9999/note", map[string]string{"note": "x"}, nil); res.StatusCode != 404 {
		t.Errorf("missing site: %d, want 404", res.StatusCode)
	}
	if res, _ := e.do(t, "PUT", path, map[string]string{"note": "x"}, map[string]string{"X-Requested-With": ""}); res.StatusCode == 200 {
		t.Error("PUT without X-Requested-With was accepted")
	}
}
