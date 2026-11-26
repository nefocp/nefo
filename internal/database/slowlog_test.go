package database

import (
	"bytes"
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseSlowLog(t *testing.T) {
	f, err := os.Open("testdata/mariadb-slow.log")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got := parseSlowLog(f, false, SlowQueryLimit)
	if len(got) != 4 {
		t.Fatalf("got %d entries: %+v", len(got), got)
	}
	// Newest first.
	wp, prep, shop, root := got[0], got[1], got[2], got[3]

	if root.User != "root@localhost" || root.Database != "" || root.Query != "SELECT SLEEP(0.3)" ||
		root.QueryTime != 0.300321 || root.RowsSent != 1 || root.RowsExamined != 0 || root.Time != "2025-11-25T14:03:07Z" {
		t.Errorf("root entry: %+v", root)
	}
	wantShop := "SELECT o.id, o.total\nFROM orders o\nWHERE o.status = 'open'\nORDER BY o.created_at DESC LIMIT 20"
	if shop.User != "shop@198.51.100.7" || shop.Database != "shop" || shop.Query != wantShop ||
		shop.QueryTime != 1.204518 || shop.LockTime != 0.000103 || shop.RowsExamined != 184220 || shop.RowsSent != 20 {
		t.Errorf("shop entry: %+v", shop)
	}
	// The server header after this entry is not part of its query.
	if prep.Query != "# administrator command: Prepare" || prep.User != "shop@localhost" || prep.Time != "2025-11-25T14:05:41Z" {
		t.Errorf("admin command entry: %+v", prep)
	}
	if wp.Database != "wp_example_com" || wp.RowsExamined != 1532 || wp.QueryTime != 2.500113 ||
		wp.Time != "2025-11-26T09:05:01Z" || !strings.HasPrefix(wp.Query, "UPDATE wp_options") || wp.Truncated {
		t.Errorf("wp entry: %+v", wp)
	}

	if got := parseSlowLog(strings.NewReader(sampleSlowLog(t)), false, 2); len(got) != 2 || got[0].Database != "wp_example_com" {
		t.Errorf("limit 2: %+v", got)
	}
}

func sampleSlowLog(t *testing.T) string {
	b, err := os.ReadFile("testdata/mariadb-slow.log")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestParseSlowLogMidFileAndLongQueries(t *testing.T) {
	sample := sampleSlowLog(t)
	// Reading from the middle of a file: the partial entry is skipped.
	cut := sample[strings.Index(sample, "FROM orders o"):]
	got := parseSlowLog(strings.NewReader(cut), true, 10)
	if len(got) != 2 || got[1].User != "shop@localhost" {
		t.Fatalf("mid-file: %+v", got)
	}

	long := strings.Repeat("é", slowQueryMax+500)
	log := "# Time: 251125 14:03:07\n# User@Host: a[a] @ localhost []\n# Query_time: 3.0  Lock_time: 0.0  Rows_sent: 0  Rows_examined: 0\n" +
		"SET timestamp=1764079387;\nINSERT INTO t VALUES ('" + long + "'),\n('" + long + "');\n" +
		"# User@Host: b[b] @ localhost []\nSET timestamp=1764079388;\nSELECT 1;\n"
	got = parseSlowLog(strings.NewReader(log), false, 10)
	if len(got) != 2 {
		t.Fatalf("got %+v", got)
	}
	if q := got[1]; !q.Truncated || len([]rune(q.Query)) != slowQueryMax || !strings.HasPrefix(q.Query, "INSERT INTO t") {
		t.Errorf("long query: truncated=%v runes=%d", q.Truncated, len([]rune(q.Query)))
	}
	if got[0].Query != "SELECT 1" || got[0].User != "b@localhost" {
		t.Errorf("entry after the long one: %+v", got[0])
	}

	// A single line longer than the scanner buffer does not stop parsing.
	huge := "# User@Host: c[c] @ localhost []\nSET timestamp=1764079387;\nINSERT INTO t VALUES ('" +
		strings.Repeat("x", 3<<20) + "');\n# User@Host: d[d] @ localhost []\nSET timestamp=1764079388;\nSELECT 2;\n"
	got = parseSlowLog(strings.NewReader(huge), false, 10)
	if len(got) != 2 || got[0].Query != "SELECT 2" || !got[1].Truncated {
		t.Fatalf("huge line: %d entries", len(got))
	}
}

func TestParseSlowTime(t *testing.T) {
	for in, want := range map[string]string{"251125 14:03:07": "2025-11-25 14:03:07", "251126  9:05:01": "2025-11-26 09:05:01"} {
		got, ok := parseSlowTime(in)
		if !ok || got.Format(time.DateTime) != want {
			t.Errorf("%q: %v %v", in, got, ok)
		}
	}
	if _, ok := parseSlowTime("yesterday"); ok {
		t.Error("parsed nonsense")
	}
}

func TestFormatThreshold(t *testing.T) {
	ok := map[float64]string{0.1: "0.1", 1: "1", 60: "60", 1.23456: "1.235", 2.5: "2.5"}
	for in, want := range ok {
		if got, err := formatThreshold(in); err != nil || got != want {
			t.Errorf("%v: %q %v", in, got, err)
		}
	}
	for _, in := range []float64{0, 0.05, 0.0999, 60.01, -1, math.NaN(), math.Inf(1)} {
		if _, err := formatThreshold(in); !isCode(err, "invalid_slow_threshold") {
			t.Errorf("%v: err = %v", in, err)
		}
	}
}

func TestSetSlowLog(t *testing.T) {
	m, f := newTest(t)
	ctx := context.Background()
	if err := m.SetSlowLog(ctx, true, 0.5); err != nil {
		t.Fatal(err)
	}
	want := "SET GLOBAL long_query_time = 0.5;\nSET GLOBAL slow_query_log_file = '/var/log/mysql/mariadb-slow.log';\nSET GLOBAL slow_query_log = ON;\n"
	if q := f.sql[len(f.sql)-1]; q != want {
		t.Errorf("SQL:\n%s", q)
	}
	if len(f.cmds) != 1 || f.cmds[0] != "install -d -o mysql -g adm -m 2750 /var/log/mysql" {
		t.Errorf("commands: %q", f.cmds)
	}
	b, _ := os.ReadFile(m.slowConf)
	if !strings.Contains(string(b), "[mysqld]\nslow_query_log = 1\nslow_query_log_file = /var/log/mysql/mariadb-slow.log\nlong_query_time = 0.5\n") {
		t.Errorf("drop-in:\n%s", b)
	}
	if fi, err := os.Stat(m.slowConf); err != nil || (fi.Mode().Perm() != 0o644 && os.PathSeparator == '/') {
		t.Errorf("drop-in mode: %v %v", fi.Mode(), err)
	}

	if err := m.SetSlowLog(ctx, false, 0.5); err != nil {
		t.Fatal(err)
	}
	if q := f.sql[len(f.sql)-1]; !strings.HasSuffix(q, "SET GLOBAL slow_query_log = OFF;\n") {
		t.Errorf("SQL:\n%s", q)
	}
	b, _ = os.ReadFile(m.slowConf)
	if on, th := parseSlowConf(string(b)); on || th != 0.5 {
		t.Errorf("drop-in after off: %v %v\n%s", on, th, b)
	}

	n := len(f.sql)
	if err := m.SetSlowLog(ctx, true, 0.01); !isCode(err, "invalid_slow_threshold") {
		t.Errorf("0.01 s: %v", err)
	}
	if len(f.sql) != n {
		t.Error("SQL ran for an invalid threshold")
	}

	f.answer["@@global.slow_query_log"] = "1\t0.500000\t/var/log/mysql/mariadb-slow.log\n"
	st, err := m.SlowLogStatus(ctx)
	if err != nil || !st.Enabled || st.Threshold != 0.5 || !st.Running {
		t.Errorf("status: %+v %v", st, err)
	}
	f.answer["@@global.slow_query_log"] = "1\t2.000000\tweb-1-slow.log\n"
	if st, _ := m.SlowLogStatus(ctx); st.Enabled {
		t.Error("a log in another file counts as on")
	}
}

func TestSlowQueriesAndClear(t *testing.T) {
	m, f := newTest(t)
	ctx := context.Background()
	if got, err := m.SlowQueries(ctx, 10); err != nil || len(got) != 0 {
		t.Fatalf("no file: %v %v", got, err)
	}
	if err := m.ClearSlowLog(ctx); err != nil {
		t.Fatalf("clear without a file: %v", err)
	}
	if err := os.WriteFile(m.slowLog, []byte(sampleSlowLog(t)), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := m.SlowQueries(ctx, 10)
	if err != nil || len(got) != 4 {
		t.Fatalf("got %d, %v", len(got), err)
	}
	if err := m.ClearSlowLog(ctx); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(m.slowLog); fi.Size() != 0 {
		t.Errorf("size after clear: %d", fi.Size())
	}
	if q := f.sql[len(f.sql)-1]; q != "FLUSH SLOW LOGS;" {
		t.Errorf("log not reopened: %q", q)
	}

	// A link planted in MariaDB's log directory is neither read nor emptied.
	target := filepath.Join(t.TempDir(), "secret")
	os.WriteFile(target, []byte("keep me"), 0o600)
	os.Remove(m.slowLog)
	if err := os.Symlink(target, m.slowLog); err != nil {
		t.Skip("symlinks not available:", err)
	}
	if _, err := m.SlowQueries(ctx, 10); err == nil {
		t.Error("read through a symlink")
	}
	if err := m.ClearSlowLog(ctx); err == nil {
		t.Error("cleared through a symlink")
	}
	if b, _ := os.ReadFile(target); !bytes.Equal(b, []byte("keep me")) {
		t.Errorf("target changed: %q", b)
	}
}
