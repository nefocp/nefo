package logstats

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	_ "time/tzdata" // for the daylight saving time test on any system
)

const (
	chrome    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	safari    = "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Safari/605.1.15"
	firefox   = "Mozilla/5.0 (X11; Linux x86_64; rv:133.0) Gecko/20100101 Firefox/133.0"
	googlebot = "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)"
	curlUA    = "curl/8.5.0"
)

// logLine formats a line in Nginx's combined format. ref and ua "" are
// written as "-".
func logLine(ip string, t time.Time, request string, status, size int, ref, ua string) string {
	if ref == "" {
		ref = "-"
	}
	if ua == "" {
		ua = "-"
	}
	return fmt.Sprintf("%s - - [%s] \"%s\" %d %d \"%s\" \"%s\"\n",
		ip, t.Format("02/Jan/2006:15:04:05 -0700"), request, status, size, ref, ua)
}

func get(ip string, t time.Time, path string, status, size int, ref, ua string) string {
	return logLine(ip, t, "GET "+path+" HTTP/1.1", status, size, ref, ua)
}

func writeLog(t *testing.T, path string, lines []string, mtime time.Time) {
	t.Helper()
	data := []byte(strings.Join(lines, ""))
	if strings.HasSuffix(path, ".gz") {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		zw.Write(data)
		zw.Close()
		data = buf.Bytes()
	}
	writeRaw(t, path, data, mtime)
}

// writeRaw writes a file and sets its modification time, by default to a
// fixed time within the tests' periods (not the clock of the machine).
func writeRaw(t testing.TB, path string, data []byte, mtime time.Time) {
	t.Helper()
	if mtime.IsZero() {
		mtime = at("2026-11-25 15:00:00", utc)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

var utc = time.UTC

func at(s string, loc *time.Location) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04:05", s, loc)
	if err != nil {
		panic(err)
	}
	return t
}

func findItem(items []Item, name string) *Item {
	for i := range items {
		if items[i].Name == name {
			return &items[i]
		}
	}
	return nil
}

func TestComputeRotatedLogs(t *testing.T) {
	dir := t.TempDir()
	now := at("2026-11-25 15:30:00", utc)
	names := []string{"example.com", "www.example.com", "*.example.com"}

	// Today, in the current log.
	writeLog(t, filepath.Join(dir, "example.com.access.log"), []string{
		get("192.0.2.1", at("2026-11-25 00:00:00", utc), "/", 200, 1000, "", chrome),
		get("192.0.2.1", at("2026-11-25 08:00:00", utc), "/blog/?page=2", 200, 2000, "https://www.example.com/", chrome),
		get("192.0.2.2", at("2026-11-25 09:00:00", utc), "/blog/", 200, 3000, "https://news.example.org/item?id=1", safari),
		get("192.0.2.3", at("2026-11-25 10:00:00", utc), "/missing", 404, 100, "", googlebot),
		get("192.0.2.3", at("2026-11-25 10:00:01", utc), "/.well-known/acme-challenge/token", 200, 87, "", ""),
		logLine("192.0.2.4", at("2026-11-25 11:00:00", utc), "POST /login HTTP/1.1", 500, 50, "https://Search.Example.NET:443/q", curlUA),
		"this is not a log line\n",
		"\n",
		get("2001:db8::1", at("2026-11-25 12:00:00", utc), "/", 304, 0, "https://shop.example.com/", firefox),
	}, at("2026-11-25 12:00:00", utc))
	// Yesterday, in the copy rotated this morning (not compressed: delaycompress).
	writeLog(t, filepath.Join(dir, "example.com.access.log.1"), []string{
		get("192.0.2.1", at("2026-11-24 06:00:00", utc), "/", 200, 1000, "", chrome),
		get("198.51.100.7", at("2026-11-24 23:59:59", utc), "/blog/", 301, 10, "https://news.example.org/", ""),
	}, at("2026-11-24 23:59:59", utc))
	// Two days ago, compressed; one line is older than the period.
	writeLog(t, filepath.Join(dir, "example.com.access.log.2.gz"), []string{
		get("203.0.113.9", at("2026-11-18 23:59:59", utc), "/old", 200, 5, "", chrome),
		get("203.0.113.9", at("2026-11-23 10:00:00", utc), "/blog/", 200, 3000, "", chrome),
	}, at("2026-11-23 10:00:00", utc))
	// Older than the period: skipped without being read, even though this
	// (made-up) line is recent.
	writeLog(t, filepath.Join(dir, "example.com.access.log.3.gz"), []string{
		get("203.0.113.66", at("2026-11-25 01:00:00", utc), "/never", 200, 5, "", chrome),
	}, at("2026-11-10 10:00:00", utc))
	// Other sites' logs and stray files are ignored.
	writeLog(t, filepath.Join(dir, "other.com.access.log"), []string{
		get("203.0.113.5", at("2026-11-25 10:00:00", utc), "/", 200, 5, "", chrome),
	}, time.Time{})
	writeLog(t, filepath.Join(dir, "example.com.access.log.1.bak"), []string{
		get("203.0.113.5", at("2026-11-25 10:00:00", utc), "/", 200, 5, "", chrome),
	}, time.Time{})
	writeLog(t, filepath.Join(dir, "example.com.error.log"), []string{"2026/11/25 10:00:00 [error] 1#1: oops\n"}, time.Time{})

	r, err := Compute(dir, "example.com", names, 7, now)
	if err != nil {
		t.Fatal(err)
	}
	tot := r.Totals
	want := Counts{Requests: 9, Visitors: 7, Bytes: 1000 + 2000 + 3000 + 100 + 50 + 0 + 1000 + 10 + 3000,
		Status2xx: 5, Status3xx: 2, Status4xx: 1, Status5xx: 1, Bots: 3}
	if tot != want {
		t.Errorf("totals = %+v, want %+v", tot, want)
	}
	if r.Files != 4 || r.Malformed != 1 || r.Truncated || r.Approximate {
		t.Errorf("files %d malformed %d truncated %v approximate %v", r.Files, r.Malformed, r.Truncated, r.Approximate)
	}
	// 7 lines today + 1 malformed, 2 yesterday, 2 in the .2.gz copy.
	if r.Lines != 12 {
		t.Errorf("lines = %d, want 12", r.Lines)
	}
	if r.LogsSince != nil {
		t.Errorf("logs_since = %v, want none: the logs cover the period", r.LogsSince)
	}
	if len(r.Daily) != 7 || !r.Daily[0].T.Equal(at("2026-11-19 00:00:00", utc)) || !r.Daily[6].T.Equal(at("2026-11-25 00:00:00", utc)) {
		t.Fatalf("daily buckets: %d, first %v", len(r.Daily), r.Daily[0].T)
	}
	if !r.From.Equal(r.Daily[0].T) || !r.To.Equal(now) {
		t.Errorf("period %v - %v", r.From, r.To)
	}
	today, yesterday, twoAgo := r.Daily[6], r.Daily[5], r.Daily[4]
	if today.Requests != 6 || today.Visitors != 5 || today.Status4xx != 1 || today.Status5xx != 1 || today.Bots != 2 {
		t.Errorf("today = %+v", today.Counts)
	}
	if yesterday.Requests != 2 || yesterday.Visitors != 2 || yesterday.Status3xx != 1 || yesterday.Bots != 1 {
		t.Errorf("yesterday = %+v", yesterday.Counts)
	}
	if twoAgo.Requests != 1 || twoAgo.Visitors != 1 || twoAgo.Bytes != 3000 {
		t.Errorf("two days ago = %+v", twoAgo.Counts)
	}
	if r.Hourly != nil {
		t.Error("hourly buckets for a 7-day period")
	}

	if p := r.Paths[0]; p.Name != "/blog/" || p.Requests != 4 || p.Bytes != 2000+3000+10+3000 {
		t.Errorf("top path = %+v", p)
	}
	if findItem(r.Paths, "/.well-known/acme-challenge/token") != nil || findItem(r.Paths, "/never") != nil || findItem(r.Paths, "/old") != nil {
		t.Errorf("paths = %+v", r.Paths)
	}
	if p := findItem(r.Paths, "/login"); p == nil || p.Requests != 1 {
		t.Errorf("paths = %+v", r.Paths)
	}
	if ip := r.IPs[0]; ip.Name != "192.0.2.1" || ip.Requests != 3 || ip.Bytes != 4000 {
		t.Errorf("top IP = %+v", ip)
	}
	if findItem(r.IPs, "2001:db8::1") == nil {
		t.Errorf("IPv6 visitor missing: %+v", r.IPs)
	}
	// The site's own names (www, and *.example.com) are not referrers.
	if len(r.Referrers) != 2 || r.Referrers[0] != (Item{"news.example.org", 2, 3010}) || r.Referrers[1].Name != "search.example.net" {
		t.Errorf("referrers = %+v", r.Referrers)
	}
	if b := r.Browsers[0]; b.Name != "Chrome" || b.Requests != 4 {
		t.Errorf("browsers = %+v", r.Browsers)
	}
	if findItem(r.Browsers, "Safari") == nil || findItem(r.Browsers, "Firefox") == nil {
		t.Errorf("browsers = %+v", r.Browsers)
	}
	for _, bot := range []string{"Googlebot", "curl", NoAgent} {
		if b := findItem(r.Bots, bot); b == nil || b.Requests != 1 {
			t.Errorf("bot %q: bots = %+v", bot, r.Bots)
		}
	}
}

func TestDayBoundariesAndTimeZones(t *testing.T) {
	dir := t.TempDir()
	plus7 := time.FixedZone("+07", 7*3600)
	writeLog(t, filepath.Join(dir, "example.com.access.log"), []string{
		// Nov 24 17:30 UTC, written by a server in UTC+7.
		get("192.0.2.1", at("2026-11-25 00:30:00", plus7), "/a", 200, 1, "", chrome),
		// Either side of midnight UTC.
		get("192.0.2.2", at("2026-11-24 23:59:59", utc), "/b", 200, 1, "", chrome),
		get("192.0.2.2", at("2026-11-25 00:00:00", utc), "/c", 200, 1, "", chrome),
		// Nov 25 03:00 UTC, written with a negative offset.
		get("192.0.2.3", at("2026-11-24 22:00:00", time.FixedZone("-05", -5*3600)), "/d", 200, 1, "", chrome),
	}, time.Time{})

	// Days are cut at midnight UTC.
	r, err := Compute(dir, "example.com", nil, 7, at("2026-11-25 15:30:00", utc))
	if err != nil {
		t.Fatal(err)
	}
	if d := r.Daily; d[5].Requests != 2 || d[6].Requests != 2 || d[5].Visitors != 2 || d[6].Visitors != 2 {
		t.Errorf("UTC days: yesterday %+v, today %+v", d[5].Counts, d[6].Counts)
	}

	// Days are cut at midnight in UTC+7: Nov 25 starts at Nov 24 17:00 UTC.
	r, err = Compute(dir, "example.com", nil, 7, at("2026-11-25 22:30:00", plus7))
	if err != nil {
		t.Fatal(err)
	}
	if d := r.Daily; d[6].Requests != 4 || d[5].Requests != 0 {
		t.Errorf("UTC+7 days: yesterday %+v, today %+v", d[5].Counts, d[6].Counts)
	}
	if _, off := r.Daily[6].T.Zone(); off != 7*3600 || r.Daily[6].T.Hour() != 0 {
		t.Errorf("bucket starts at %v", r.Daily[6].T)
	}
	if r.LogsSince == nil || !r.LogsSince.Equal(at("2026-11-24 17:30:00", utc)) {
		t.Errorf("logs_since = %v", r.LogsSince)
	}

	// A one-day period: only today's lines.
	r, err = Compute(dir, "example.com", nil, 1, at("2026-11-25 15:30:00", utc))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Daily) != 1 || r.Totals.Requests != 2 {
		t.Errorf("one day: %d buckets, %+v", len(r.Daily), r.Totals)
	}
}

func TestDaylightSavingTime(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	// Clocks went forward at 02:00 on 29 March 2026: that day has 23 hours.
	writeLog(t, filepath.Join(dir, "example.com.access.log"), []string{
		get("192.0.2.1", at("2026-03-28 23:30:00", berlin), "/", 200, 1, "", chrome),
		get("192.0.2.1", at("2026-03-29 00:30:00", berlin), "/", 200, 1, "", chrome),
		get("192.0.2.1", at("2026-03-29 23:30:00", berlin), "/", 200, 1, "", chrome),
		get("192.0.2.1", at("2026-03-30 00:10:00", berlin), "/", 200, 1, "", chrome),
	}, time.Time{})
	r, err := Compute(dir, "example.com", nil, 7, at("2026-03-30 12:00:00", berlin))
	if err != nil {
		t.Fatal(err)
	}
	d := r.Daily
	if d[4].Requests != 1 || d[5].Requests != 2 || d[6].Requests != 1 {
		t.Errorf("28th %d, 29th %d, 30th %d requests", d[4].Requests, d[5].Requests, d[6].Requests)
	}
	if got := d[6].T.Sub(d[5].T); got != 23*time.Hour {
		t.Errorf("29 March lasted %v", got)
	}
}

func TestHourly(t *testing.T) {
	dir := t.TempDir()
	writeLog(t, filepath.Join(dir, "example.com.access.log"), []string{
		get("192.0.2.1", at("2026-11-24 23:59:00", utc), "/", 200, 1, "", chrome),
		get("192.0.2.1", at("2026-11-25 00:05:00", utc), "/", 200, 1, "", chrome),
		get("192.0.2.1", at("2026-11-25 09:59:59", utc), "/", 200, 1, "", chrome),
		get("192.0.2.1", at("2026-11-25 10:00:00", utc), "/", 200, 1, "", chrome),
		get("192.0.2.2", at("2026-11-25 10:30:00", utc), "/", 200, 1, "", chrome),
		get("192.0.2.1", at("2026-11-25 15:29:00", utc), "/", 200, 1, "", chrome),
	}, time.Time{})
	r, err := Compute(dir, "example.com", nil, 1, at("2026-11-25 15:30:00", utc))
	if err != nil {
		t.Fatal(err)
	}
	h := r.Hourly
	if len(h) != 16 || !h[15].T.Equal(at("2026-11-25 15:00:00", utc)) {
		t.Fatalf("%d hourly buckets, last %v", len(h), h[len(h)-1].T)
	}
	if h[0].Requests != 1 || h[9].Requests != 1 || h[10].Requests != 2 || h[10].Visitors != 2 || h[15].Visitors != 1 || h[3].Requests != 0 {
		t.Errorf("hourly = %+v", h)
	}
	if r.Totals.Requests != 5 || r.Totals.Visitors != 2 || r.Daily[0].Visitors != 2 {
		t.Errorf("totals = %+v", r.Totals)
	}
}

func TestLineCap(t *testing.T) {
	dir := t.TempDir()
	now := at("2026-11-25 15:30:00", utc)
	var cur, old []string
	for i := range 20 {
		cur = append(cur, get("192.0.2.1", at("2026-11-25 10:00:00", utc).Add(time.Duration(i)*time.Second), "/new", 200, 1, "", chrome))
		old = append(old, get("192.0.2.2", at("2026-11-24 10:00:00", utc).Add(time.Duration(i)*time.Second), "/old", 200, 1, "", chrome))
	}
	writeLog(t, filepath.Join(dir, "example.com.access.log"), cur, time.Time{})
	writeLog(t, filepath.Join(dir, "example.com.access.log.1"), old, at("2026-11-24 10:00:20", utc))

	// The newest file is read first: the cap drops the oldest lines.
	r, err := compute(dir, "example.com", nil, 7, now, 30)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Truncated || r.Lines != 30 || r.Daily[6].Requests != 20 || r.Daily[5].Requests != 10 {
		t.Errorf("truncated %v, lines %d, today %d, yesterday %d", r.Truncated, r.Lines, r.Daily[6].Requests, r.Daily[5].Requests)
	}
	if r.LogsSince == nil || !r.LogsSince.Equal(at("2026-11-24 10:00:00", utc)) {
		t.Errorf("logs_since = %v", r.LogsSince)
	}
	// Exactly at the cap: nothing is left out.
	for _, max := range []int64{20, 40} {
		r, err = compute(dir, "example.com", nil, 7, now, max)
		if err != nil {
			t.Fatal(err)
		}
		if r.Truncated != (max == 20) || r.Lines != max {
			t.Errorf("cap %d: truncated %v, lines %d", max, r.Truncated, r.Lines)
		}
	}
}

func TestMalformedLines(t *testing.T) {
	dir := t.TempDir()
	ts := at("2026-11-25 10:00:00", utc)
	good := get("192.0.2.1", ts, "/", 200, 10, "", chrome)
	stamp := ts.Format("02/Jan/2006:15:04:05 -0700")
	writeLog(t, filepath.Join(dir, "example.com.access.log"), []string{
		good,
		"garbage\n",
		"192.0.2.1 - - [25/Nov/2026:10:00:00] \"GET / HTTP/1.1\" 200 1 \"-\" \"-\"\n",         // no offset
		"192.0.2.1 - - [25/Foo/2026:10:00:00 +0000] \"GET / HTTP/1.1\" 200 1 \"-\" \"-\"\n",   // bad month
		"192.0.2.1 - - [" + stamp + "] \"GET / HTTP/1.1\" 2000 1 \"-\" \"-\"\n",               // bad status
		"192.0.2.1 - - [" + stamp + "] \"GET / HTTP/1.1\" 200 x \"-\" \"-\"\n",                // bad size
		"192.0.2.1 - - [" + stamp + "] \"GET / HTTP/1.1 200 1 \"-\" \"-\"\n",                  // unbalanced quotes
		"not-an-ip - - [" + stamp + "] \"GET / HTTP/1.1\" 200 1 \"-\" \"-\"\n",                // bad address
		"192.0.2.1 - - [" + stamp + "] \"GET /" + strings.Repeat("a", 100<<10) + "\" 200 1\n", // too long
		"\r\n",
		good,
		// A TLS handshake sent to the HTTP port is a real (400) request.
		"192.0.2.9 - - [" + stamp + "] \"\\x16\\x03\\x01\\x02\\x00\\x01\" 400 157 \"-\" \"-\"\n",
		// The "common" format, without referrer and user agent.
		"192.0.2.1 - - [" + stamp + "] \"GET /plain HTTP/1.0\" 200 5\n",
		strings.TrimSuffix(good, "\n"), // the last line has no newline yet
	}, time.Time{})
	r, err := Compute(dir, "example.com", nil, 7, at("2026-11-25 15:30:00", utc))
	if err != nil {
		t.Fatal(err)
	}
	if r.Malformed != 8 || r.Lines != 13 || r.Totals.Requests != 5 || r.Totals.Status4xx != 1 {
		t.Errorf("malformed %d, lines %d, totals %+v", r.Malformed, r.Lines, r.Totals)
	}
	if p := findItem(r.Paths, "/"); p == nil || p.Requests != 3 {
		t.Errorf("paths = %+v", r.Paths)
	}
}

func TestDamagedGzip(t *testing.T) {
	dir := t.TempDir()
	var lines []string
	for i := range 200 {
		lines = append(lines, get(fmt.Sprintf("192.0.2.%d", i%250), at("2026-11-24 10:00:00", utc), fmt.Sprintf("/p%d", i), 200, 1, "", chrome))
	}
	p := filepath.Join(dir, "example.com.access.log.2.gz")
	writeLog(t, p, lines, time.Time{})
	data, _ := os.ReadFile(p)
	writeRaw(t, p, data[:len(data)/2], time.Time{}) // cut in the middle
	writeRaw(t, filepath.Join(dir, "example.com.access.log.3.gz"), []byte("not gzip at all\n"), time.Time{})
	writeRaw(t, filepath.Join(dir, "example.com.access.log.4.gz"), []byte{0x1f, 0x8b, 0, 0}, time.Time{})

	r, err := Compute(dir, "example.com", nil, 7, at("2026-11-25 15:30:00", utc))
	if err != nil {
		t.Fatal(err)
	}
	if r.Files != 3 || r.Totals.Requests == 0 || r.Totals.Requests >= 200 || r.Malformed < 2 {
		t.Errorf("files %d, requests %d, malformed %d", r.Files, r.Totals.Requests, r.Malformed)
	}
}

func TestNoLogs(t *testing.T) {
	r, err := Compute(filepath.Join(t.TempDir(), "missing"), "example.com", nil, 30, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if r.Files != 0 || r.Totals != (Counts{}) || len(r.Daily) != 30 || r.LogsSince != nil {
		t.Errorf("report = %+v", r)
	}
	js, _ := json.Marshal(r)
	for _, s := range []string{`"paths":[]`, `"ips":[]`, `"referrers":[]`, `"browsers":[]`, `"bots":[]`} {
		if !bytes.Contains(js, []byte(s)) {
			t.Errorf("JSON lacks %s: %s", s, js)
		}
	}
	if bytes.Contains(js, []byte(`"hourly"`)) {
		t.Errorf("JSON has hourly buckets: %s", js)
	}
}

func TestInvalidArguments(t *testing.T) {
	dir := t.TempDir()
	for _, days := range []int{0, -1, 32} {
		if _, err := Compute(dir, "example.com", nil, days, time.Now()); err == nil {
			t.Errorf("days %d accepted", days)
		}
	}
	for _, d := range []string{"", "../etc/passwd", `a\b`, ".hidden", "a/b"} {
		if _, err := Compute(dir, d, nil, 7, time.Now()); err == nil {
			t.Errorf("domain %q accepted", d)
		}
	}
}

func TestSymlinksAreNotFollowed(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.log")
	writeLog(t, outside, []string{get("192.0.2.1", at("2026-11-25 10:00:00", utc), "/secret", 200, 1, "", chrome)}, time.Time{})
	if err := os.Symlink(outside, filepath.Join(dir, "example.com.access.log.1")); err != nil {
		t.Skip("symbolic links are not available:", err)
	}
	writeLog(t, filepath.Join(dir, "example.com.access.log"), []string{get("192.0.2.1", at("2026-11-25 10:00:00", utc), "/", 200, 1, "", chrome)}, time.Time{})
	r, err := Compute(dir, "example.com", nil, 7, at("2026-11-25 15:30:00", utc))
	if err != nil {
		t.Fatal(err)
	}
	if r.Totals.Requests != 1 || findItem(r.Paths, "/secret") != nil {
		t.Errorf("the symlink was read: %+v", r.Paths)
	}
	// Opening a symlink directly fails too.
	if f, err := openLog(filepath.Join(dir, "example.com.access.log.1")); err == nil {
		f.Close()
		t.Error("openLog followed a symlink")
	}
}

func TestParseLine(t *testing.T) {
	stamp := "25/Nov/2026:10:00:00 +0700"
	unix := at("2026-11-25 03:00:00", utc).Unix()
	tests := []struct {
		line   string
		ok     bool
		path   string
		status int
		size   int64
		ref    string
		agent  string
	}{
		{`192.0.2.1 - - [` + stamp + `] "GET /a/b?x=1 HTTP/2.0" 200 512 "https://example.org/" "curl/8.5.0"`, true, "/a/b", 200, 512, "https://example.org/", "curl/8.5.0"},
		{`2001:db8::2 - alice [` + stamp + `] "POST /login HTTP/1.1" 302 0 "-" "-"`, true, "/login", 302, 0, "", ""},
		{`192.0.2.1 - - [` + stamp + `] "GET http://example.com/x?y HTTP/1.1" 200 1 "-" "a"`, true, "/x", 200, 1, "", "a"},
		{`192.0.2.1 - - [` + stamp + `] "-" 400 0 "-" "-"`, true, "", 400, 0, "", ""},
		{`192.0.2.1 - - [` + stamp + `] "GET / HTTP/1.1" 499 - "-" "x\x22y"`, true, "/", 499, 0, "", `x\x22y`},
		{`192.0.2.1 - - [` + stamp + `] "GET / HTTP/1.1" 200 1 "-" "a" "198.51.100.1"`, true, "/", 200, 1, "", "a"},
		{`192.0.2.1 - - [` + stamp + `] "GET / HTTP/1.1" 200 1 "-"`, false, "", 0, 0, "", ""},
		{`192.0.2.1 - - [` + stamp + `] "GET / HTTP/1.1" 099 1 "-" "-"`, false, "", 0, 0, "", ""},
		{`192.0.2.1 - - [32/Nov/2026:10:00:00 +0700] "GET / HTTP/1.1" 200 1 "-" "-"`, false, "", 0, 0, "", ""},
		{`192.0.2.1 - - [25/Nov/2026:24:00:00 +0700] "GET / HTTP/1.1" 200 1 "-" "-"`, false, "", 0, 0, "", ""},
		{``, false, "", 0, 0, "", ""},
	}
	for _, tt := range tests {
		var e entry
		ok := parseLine([]byte(tt.line), &e)
		if ok != tt.ok {
			t.Errorf("%s: ok = %v", tt.line, ok)
			continue
		}
		if !ok {
			continue
		}
		if string(e.path) != tt.path || e.status != tt.status || e.bytes != tt.size || string(e.ref) != tt.ref || string(e.agent) != tt.agent || e.unix != unix {
			t.Errorf("%s: got path %q status %d size %d ref %q agent %q time %d", tt.line, e.path, e.status, e.bytes, e.ref, e.agent, e.unix)
		}
	}
}

func TestClassify(t *testing.T) {
	tests := []struct {
		ua   string
		name string
		bot  bool
	}{
		{chrome, "Chrome", false},
		{safari, "Safari", false},
		{firefox, "Firefox", false},
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 Edg/131.0.0.0", "Edge", false},
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 18_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/131.0 Mobile/15E148 Safari/604.1", "Chrome", false},
		{"Mozilla/5.0 (Linux; Android 14; SM-S921B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/26.0 Chrome/122.0 Mobile Safari/537.36", "Samsung Internet", false},
		{"Mozilla/5.0 (Windows NT 10.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0 Safari/537.36 OPR/115.0", "Opera", false},
		{googlebot, "Googlebot", true},
		{"Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)", "Bingbot", true},
		{"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; ClaudeBot/1.0; +claudebot@anthropic.com)", "ClaudeBot", true},
		{curlUA, "curl", true},
		{"Wget/1.21.4", "Wget", true},
		{"python-requests/2.32.3", "Python Requests", true},
		{"Go-http-client/1.1", "Go HTTP client", true},
		{"Mozilla/5.0 (l9scan/2.0.1; +https://leakix.net)", "LeakIX", true},
		{"Mozilla/5.0 (compatible; ExampleBot/1.2; +https://bots.example.org/info)", "ExampleBot", true},
		{"Mozilla/5.0 (compatible; Example-Spider/3)", "Example-Spider", true},
		{"Mozilla/5.0 (compatible; +http://example.org/crawler.html)", OtherBot, true},
		{"Mozilla/5.0 (compatible; ExampleEngine/1.0; +https://engine.example.net/)", "ExampleEngine", true},
		{"Mozilla/5.0 (compatible; example-ct-watch/2.1)", "example-ct-watch", true},
		{"ExampleSiteMonitor/1.0", "ExampleSiteMonitor", true},
		{"Example Global Inventory Crawler", "Example", true},
		{"Mozilla/5.0 (compatible; Barkrowler/0.9; +https://crawl.example.org/)", "Barkrowler", true},
		{"Mozilla/4.0 (compatible; MSIE 8.0; Windows NT 6.1; Trident/4.0)", "Internet Explorer", false},
		{"Opera/9.80 (Windows NT 6.1) Presto/2.12.388 Version/12.18", "Opera", false},
		{"Mozilla/5.0 (X11; Linux x86_64)", OtherAgent, false},
		{"", NoAgent, true},
	}
	var lower []byte
	for _, tt := range tests {
		name, bot := classify([]byte(tt.ua), &lower)
		if name != tt.name || bot != tt.bot {
			t.Errorf("%q = %q, %v; want %q, %v", tt.ua, name, bot, tt.name, tt.bot)
		}
	}
}

func TestRefHost(t *testing.T) {
	tests := map[string]string{
		"https://Example.ORG/path?q=1":     "example.org",
		"http://user:pw@example.org:8080/": "example.org",
		"https://[2001:db8::1]:8443/x":     "2001:db8::1",
		"https://example.org.":             "example.org",
		"android-app://com.example.app/":   "com.example.app",
		"example.org/no-scheme":            "",
		"https:///nohost":                  "",
		"":                                 "",
	}
	for in, want := range tests {
		if got := string(refHost([]byte(in))); got != want {
			t.Errorf("refHost(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCounterPrune(t *testing.T) {
	c := newCounter(10)
	for range 5 {
		c.add([]byte("/popular"), 1)
	}
	for i := range 100 {
		c.add(fmt.Appendf(nil, "/rare%d", i), 1)
	}
	if !c.pruned || len(c.m) > 10 {
		t.Fatalf("pruned %v, %d values", c.pruned, len(c.m))
	}
	if top := c.top(1); top[0].Name != "/popular" || top[0].Requests != 5 {
		t.Errorf("top = %+v", top)
	}
}

func TestCache(t *testing.T) {
	c := NewCache(time.Minute)
	clock := at("2026-11-25 10:00:00", utc)
	c.now = func() time.Time { return clock }
	var calls atomic.Int32
	release := make(chan struct{})
	compute := func() (*Report, error) {
		calls.Add(1)
		<-release
		return &Report{Days: 7}, nil
	}

	// Concurrent requests share one computation.
	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() {
			if r, err := c.Get(context.Background(), "1/7", compute); err != nil || r.Days != 7 {
				t.Errorf("Get = %v, %v", r, err)
			}
		})
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	if n := calls.Load(); n != 1 {
		t.Errorf("%d computations, want 1", n)
	}

	c.Get(context.Background(), "1/7", compute)
	c.Get(context.Background(), "1/30", compute)
	if n := calls.Load(); n != 2 {
		t.Errorf("%d computations after a cached and a new key, want 2", n)
	}
	clock = clock.Add(61 * time.Second)
	c.Get(context.Background(), "1/7", compute)
	if n := calls.Load(); n != 3 {
		t.Errorf("%d computations after expiry, want 3", n)
	}

	// Errors are not cached.
	fail := func() (*Report, error) { calls.Add(1); return nil, errors.New("boom") }
	for range 2 {
		if _, err := c.Get(context.Background(), "2/7", fail); err == nil {
			t.Error("no error")
		}
	}
	if n := calls.Load(); n != 5 {
		t.Errorf("%d computations, want 5", n)
	}

	// A panic does not leave waiters hanging.
	func() {
		defer func() { recover() }()
		c.Get(context.Background(), "3/7", func() (*Report, error) { panic("oops") })
	}()
	if r, err := c.Get(context.Background(), "3/7", func() (*Report, error) { return &Report{Days: 1}, nil }); err != nil || r.Days != 1 {
		t.Errorf("after a panic: %v, %v", r, err)
	}
}

func BenchmarkCompute(b *testing.B) {
	dir := b.TempDir()
	now := at("2026-11-25 15:30:00", utc)
	var sb strings.Builder
	uas := []string{chrome, safari, firefox, googlebot, curlUA}
	for i := range 200_000 {
		ts := now.Add(-time.Duration(i) * 2 * time.Second)
		sb.WriteString(get(fmt.Sprintf("198.51.100.%d", i%200), ts, fmt.Sprintf("/page/%d?x=%d", i%500, i), 200, 1500, "https://www.example.org/", uas[i%len(uas)]))
	}
	writeRaw(b, filepath.Join(dir, "example.com.access.log"), []byte(sb.String()), now)
	b.ResetTimer()
	for range b.N {
		if _, err := Compute(dir, "example.com", []string{"example.com"}, 7, now); err != nil {
			b.Fatal(err)
		}
	}
}

// The panel's own website checks (internal/uptime) are not visits.
func TestComputeSkipsMonitorChecks(t *testing.T) {
	dir := t.TempDir()
	now := at("2026-11-25 15:30:00", utc)
	writeLog(t, filepath.Join(dir, "example.com.access.log"), []string{
		get("192.0.2.1", at("2026-11-25 08:00:00", utc), "/", 200, 1000, "", chrome),
		get("203.0.113.1", at("2026-11-25 08:05:00", utc), "/", 200, 1000, "", "Nefo-Monitor/1.0"),
		get("203.0.113.1", at("2026-11-25 08:10:00", utc), "/health", 502, 10, "", "Nefo-Monitor/1.0"),
	}, at("2026-11-25 12:00:00", utc))
	r, err := Compute(dir, "example.com", []string{"example.com"}, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	if r.Totals.Requests != 1 || r.Totals.Bots != 0 || r.Totals.Status5xx != 0 || findItem(r.Paths, "/health") != nil {
		t.Fatalf("totals = %+v, paths = %+v", r.Totals, r.Paths)
	}
}
