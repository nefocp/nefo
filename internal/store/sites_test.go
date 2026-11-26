package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSites(t *testing.T) {
	ctx := context.Background()
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	site := &Site{Domain: "example.com", Aliases: []string{"www.example.com"}, SysUser: "example_com", PHPVersion: "8.3"}
	if err := st.CreateSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	if site.ID == 0 {
		t.Fatal("ID not set")
	}
	if err := st.CreateSite(ctx, &Site{Domain: "example.com", SysUser: "other"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate domain: err = %v, want ErrConflict", err)
	}
	if err := st.CreateSite(ctx, &Site{Domain: "other.com", SysUser: "example_com"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate user: err = %v, want ErrConflict", err)
	}
	if taken, _ := st.SysUserTaken(ctx, "example_com"); !taken {
		t.Fatal("SysUserTaken = false")
	}

	expires := time.Unix(1_900_000_000, 0)
	site.SSL, site.ForceHTTPS, site.CertNotAfter = true, true, expires
	if err := st.UpdateSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	got, err := st.SiteByID(ctx, site.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.SSL || !got.ForceHTTPS || !got.CertNotAfter.Equal(expires) || !got.CertNotBefore.IsZero() {
		t.Fatalf("after update: %+v", got)
	}
	if len(got.Aliases) != 1 || got.Aliases[0] != "www.example.com" {
		t.Fatalf("aliases = %q", got.Aliases)
	}
	if got.HSTS || got.HSTSSubdomains || got.PageCache {
		t.Fatalf("new site has HSTS or page cache: %+v", got)
	}
	got.HSTS, got.HSTSSubdomains, got.PageCache = true, true, true
	if err := st.UpdateSite(ctx, got); err != nil {
		t.Fatal(err)
	}
	if again, _ := st.SiteByID(ctx, site.ID); !again.HSTS || !again.HSTSSubdomains || !again.PageCache {
		t.Fatalf("HSTS and page cache not saved: %+v", again)
	}
	if names := got.Names(); len(names) != 2 || names[0] != "example.com" {
		t.Fatalf("Names() = %q", names)
	}

	list, err := st.ListSites(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListSites = %v, %v", list, err)
	}
	if err := st.DeleteSite(ctx, site.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SiteByID(ctx, site.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete: err = %v", err)
	}
}

func TestSiteNote(t *testing.T) {
	ctx := context.Background()
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	site := &Site{Domain: "note.example", SysUser: "note_example"}
	if err := st.CreateSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	if err := st.SetSiteNote(ctx, site.ID, "Khách hàng A · hết hạn 12/2027"); err != nil {
		t.Fatal(err)
	}
	// A settings change made from an older copy of the site keeps the note.
	site.PHPVersion = "8.4"
	if err := st.UpdateSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	got, err := st.SiteByID(ctx, site.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Note != "Khách hàng A · hết hạn 12/2027" || got.PHPVersion != "8.4" {
		t.Errorf("note %q php %q", got.Note, got.PHPVersion)
	}
	if err := st.SetSiteNote(ctx, 9999, "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("SetSiteNote on a missing site: err = %v, want ErrNotFound", err)
	}
}

func TestMetricsNetwork(t *testing.T) {
	ctx := context.Background()
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	base := time.Unix(1_800_000_000, 0) // a multiple of 120 s
	f := func(v float64) *float64 { return &v }
	// Two samples with traffic and one without (taken before it was recorded)
	// in the first bucket; one without in the second.
	for i, m := range []Metric{{NetRx: f(1000), NetTx: f(100)}, {NetRx: f(3000), NetTx: f(300)}, {}, {}} {
		m.Time = base.Add(time.Duration(i) * 40 * time.Second)
		if err := st.AddMetric(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	got, err := st.Metrics(ctx, base, 2*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].NetRx == nil || *got[0].NetRx != 2000 || *got[0].NetTx != 200 || got[1].NetRx != nil {
		t.Errorf("Metrics = %+v, want rx 2000 tx 200, then no traffic", got)
	}
	rx, tx, err := st.NetTotals(ctx, base)
	if err != nil || rx != 4000*60 || tx != 400*60 {
		t.Errorf("NetTotals = %v, %v, %v; want 240000, 24000", rx, tx, err)
	}
	if rx, tx, err := st.NetTotals(ctx, base.Add(time.Hour)); err != nil || rx != 0 || tx != 0 {
		t.Errorf("NetTotals with no samples = %v, %v, %v", rx, tx, err)
	}
}

// A site created suspended (an import, a Docker app) is suspended in the
// store too, not only in its web server configuration.
func TestCreateSiteSuspended(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	site := &Site{Domain: "closed.example", SysUser: "closed_example", Disabled: true}
	if err := st.CreateSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	got, err := st.SiteByID(ctx, site.ID)
	if err != nil || !got.Disabled {
		t.Fatalf("disabled %v, %v", got != nil && got.Disabled, err)
	}
}
