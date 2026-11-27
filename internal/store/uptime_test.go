package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSiteMonitor(t *testing.T) {
	ctx := context.Background()
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	site := &Site{Domain: "example.com", SysUser: "example_com"}
	if err := st.CreateSite(ctx, site); err != nil {
		t.Fatal(err)
	}

	// Never saved: off, every 5 minutes, "/".
	m, err := st.SiteMonitor(ctx, site.ID)
	if err != nil || m.Enabled || m.Interval != 5 || m.Path != "/" {
		t.Fatalf("defaults: %+v, %v", m, err)
	}
	m.Enabled, m.Interval, m.Path, m.Keyword, m.Strict4xx = true, 2, "/health?x=1", "Welcome", true
	if err := st.SaveSiteMonitor(ctx, m); err != nil {
		t.Fatal(err)
	}
	expiry, warned := time.Unix(1_900_000_000, 0), time.Unix(1_800_000_000, 0)
	if err := st.SetMonitorCert(ctx, site.ID, expiry, warned); err != nil {
		t.Fatal(err)
	}
	// Saving the settings again keeps the certificate fields.
	m.Interval = 10
	if err := st.SaveSiteMonitor(ctx, m); err != nil {
		t.Fatal(err)
	}
	got, err := st.SiteMonitor(ctx, site.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := SiteMonitor{SiteID: site.ID, Enabled: true, Interval: 10, Path: "/health?x=1", Keyword: "Welcome", Strict4xx: true,
		CertNotAfter: expiry, CertWarnedAt: warned}
	if *got != want {
		t.Fatalf("got %+v, want %+v", *got, want)
	}
	if all, err := st.SiteMonitors(ctx); err != nil || len(all) != 1 || all[0].SiteID != site.ID {
		t.Fatalf("SiteMonitors: %+v, %v", all, err)
	}
	if err := st.SaveSiteMonitor(ctx, &SiteMonitor{SiteID: 999, Interval: 5, Path: "/"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown site: %v", err)
	}
}

func TestUptimeChecks(t *testing.T) {
	ctx := context.Background()
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	site := &Site{Domain: "example.com", SysUser: "example_com"}
	other := &Site{Domain: "other.com", SysUser: "other_com"}
	for _, s := range []*Site{site, other} {
		if err := st.CreateSite(ctx, s); err != nil {
			t.Fatal(err)
		}
		if err := st.SaveSiteMonitor(ctx, &SiteMonitor{SiteID: s.ID, Enabled: true, Interval: 5, Path: "/"}); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Unix(1_800_000_000, 0)
	add := func(siteID int64, ago time.Duration, ok bool, code, ms int, e string) {
		t.Helper()
		if err := st.AddUptimeCheck(ctx, UptimeCheck{SiteID: siteID, Time: now.Add(-ago), OK: ok, Code: code, MS: ms, Error: e}); err != nil {
			t.Fatal(err)
		}
	}
	add(site.ID, 8*24*time.Hour, true, 200, 999, "") // older than a week
	add(site.ID, 30*time.Hour, false, 502, 40, "HTTP 502")
	add(site.ID, 20*time.Minute, true, 200, 100, "")
	add(site.ID, 10*time.Minute, false, 0, 10000, "timed out")
	add(site.ID, 0, true, 200, 300, "")
	add(other.ID, 0, false, 500, 5, "HTTP 500")

	day, err := st.UptimeStats(ctx, site.ID, now.Add(-24*time.Hour))
	if err != nil || day != (UptimeStats{Checks: 3, Up: 2, AvgMS: 200}) {
		t.Fatalf("24 h: %+v, %v", day, err)
	}
	week, err := st.UptimeStats(ctx, site.ID, now.Add(-7*24*time.Hour))
	if err != nil || week.Checks != 4 || week.Up != 2 {
		t.Fatalf("7 d: %+v, %v", week, err)
	}
	if none, err := st.UptimeStats(ctx, 999, now.Add(-time.Hour)); err != nil || none != (UptimeStats{}) {
		t.Fatalf("no checks: %+v, %v", none, err)
	}

	fails, err := st.UptimeChecks(ctx, site.ID, now.Add(-7*24*time.Hour), 10, true)
	if err != nil || len(fails) != 2 || fails[0].Error != "timed out" || fails[1].Code != 502 {
		t.Fatalf("failures: %+v, %v", fails, err)
	}
	last, err := st.UptimeChecks(ctx, site.ID, time.Time{}, 2, false)
	if err != nil || len(last) != 2 || !last[0].OK || last[0].MS != 300 || last[1].OK || !last[0].Time.Equal(now) {
		t.Fatalf("newest: %+v, %v", last, err)
	}

	// The newest check succeeded: no failures in a row.
	if n, since, err := st.UptimeStreak(ctx, site.ID); err != nil || n != 0 || !since.IsZero() {
		t.Fatalf("streak: %d %v %v", n, since, err)
	}
	add(site.ID, -time.Minute, false, 502, 30, "HTTP 502")
	add(site.ID, -2*time.Minute, false, 0, 10000, "timed out")
	if n, since, err := st.UptimeStreak(ctx, site.ID); err != nil || n != 2 || !since.Equal(now.Add(time.Minute)) {
		t.Fatalf("streak: %d %v %v", n, since, err)
	}
	if n, _, err := st.UptimeStreak(ctx, other.ID); err != nil || n != 1 { // never up
		t.Fatalf("streak of other: %d %v", n, err)
	}
	if _, err := st.db.Exec(`DELETE FROM uptime_checks WHERE ts > ?`, now.Unix()); err != nil {
		t.Fatal(err)
	}

	// Response times of the successful checks, in hourly buckets.
	pts, err := st.UptimeSeries(ctx, site.ID, now.Add(-24*time.Hour), time.Hour)
	// now is on the hour: the two successful checks fall in two buckets.
	if err != nil || len(pts) != 2 || pts[0].MS != 100 || pts[1].MS != 300 || !pts[1].Time.Equal(now) {
		t.Fatalf("series: %+v, %v", pts, err)
	}

	// Pruning keeps the last 7 days.
	if err := st.PurgeUptimeBefore(ctx, now.Add(-7*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if all, _ := st.UptimeChecks(ctx, site.ID, time.Time{}, 100, false); len(all) != 4 {
		t.Fatalf("after purge: %d checks", len(all))
	}

	// Deleting a website deletes its history and settings.
	if err := st.DeleteSite(ctx, site.ID); err != nil {
		t.Fatal(err)
	}
	if all, _ := st.UptimeChecks(ctx, site.ID, time.Time{}, 100, false); len(all) != 0 {
		t.Fatalf("after delete: %d checks", len(all))
	}
	if ms, _ := st.SiteMonitors(ctx); len(ms) != 1 || ms[0].SiteID != other.ID {
		t.Fatalf("monitors after delete: %+v", ms)
	}
	if all, _ := st.UptimeChecks(ctx, other.ID, time.Time{}, 100, false); len(all) != 1 {
		t.Fatalf("other site: %d checks", len(all))
	}
}

// A database from before migration 22 gets the monitoring tables, and its
// websites are not monitored.
func TestUptimeMigration(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	site := &Site{Domain: "example.com", SysUser: "example_com"}
	if err := st.CreateSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`DROP TABLE api_keys; DROP TABLE uptime_checks; DROP TABLE site_monitors;
		ALTER TABLE databases DROP COLUMN engine; ALTER TABLE sites DROP COLUMN app_memory; ALTER TABLE sites DROP COLUMN proxy_url; ALTER TABLE sites DROP COLUMN proxy_opts; PRAGMA user_version = 21`); err != nil {
		t.Fatal(err)
	}
	st.Close()
	if st, err = Open(dir); err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	var v int
	if err := st.db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil || v != len(migrations) {
		t.Fatalf("user_version %d, %v", v, err)
	}
	if ms, err := st.SiteMonitors(ctx); err != nil || len(ms) != 0 {
		t.Fatalf("monitors: %+v, %v", ms, err)
	}
	m, err := st.SiteMonitor(ctx, site.ID)
	if err != nil || m.Enabled {
		t.Fatalf("monitor: %+v, %v", m, err)
	}
	m.Enabled = true
	if err := st.SaveSiteMonitor(ctx, m); err != nil {
		t.Fatal(err)
	}
	if err := st.AddUptimeCheck(ctx, UptimeCheck{SiteID: site.ID, Time: time.Now(), OK: true, Code: 200, MS: 12}); err != nil {
		t.Fatal(err)
	}
}
