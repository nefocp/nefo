package cron

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

func TestValidSchedule(t *testing.T) {
	for _, s := range []string{"* * * * *", "*/5 * * * *", "0 3 * * 1-5", "15,45 8-18 * jan-jun mon", "@daily", "0 0 1 */2 *"} {
		if !ValidSchedule(s) {
			t.Errorf("%q refused", s)
		}
	}
	for _, s := range []string{"", "* * * *", "60 * * * *", "* 24 * * *", "* * * * * root", "*;rm * * * *", "@sometimes", "* * * * *\n* * * * * root x"} {
		if ValidSchedule(s) {
			t.Errorf("%q accepted", s)
		}
	}
}

func newTest(t *testing.T) *Manager {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	dir := t.TempDir()
	m := New(st, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), func(context.Context) (map[string][2]int, error) {
		return map[string][2]int{"root": {0, 0}, "shop_com": {999, 999}}, nil
	})
	m.file, m.logDir = filepath.Join(dir, "panel"), filepath.Join(dir, "logs")
	m.chown = func(string, int, int) error { return nil }
	return m
}

func TestSyncWritesCronFile(t *testing.T) {
	m := newTest(t)
	ctx := context.Background()
	jobs := []*store.CronJob{
		{Name: "WordPress cron", Schedule: "*/5  *  * * *", Command: "php8.3 /var/www/shop.com/public/wp-cron.php", RunAs: "shop_com", Enabled: true},
		{Name: "Dated dump", Schedule: "@daily", Command: "date +%Y-%m-%d > /root/d.txt", RunAs: "root", Enabled: true},
		{Name: "Off", Schedule: "0 1 * * *", Command: "echo off", RunAs: "root", Enabled: false},
	}
	for _, j := range jobs {
		if err := m.Save(ctx, j); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := os.ReadFile(m.file)
	s := string(b)
	for _, want := range []string{
		"*/5 * * * * shop_com ( php8.3 /var/www/shop.com/public/wp-cron.php ) >> " + filepath.Join(m.logDir, "1.log") + " 2>&1",
		`@daily root ( date +\%Y-\%m-\%d > /root/d.txt )`,
		"# (disabled) 0 1 * * * root ( echo off )",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
}

func TestCheckRefusesBadJobs(t *testing.T) {
	m := newTest(t)
	ctx := context.Background()
	cases := map[string]*store.CronJob{
		"invalid_cron_command":  {Name: "x", Schedule: "@daily", Command: "echo 1\n* * * * * root evil", RunAs: "root"},
		"invalid_cron_schedule": {Name: "x", Schedule: "* * * *", Command: "echo", RunAs: "root"},
		"invalid_cron_user":     {Name: "x", Schedule: "@daily", Command: "echo", RunAs: "www-data"},
		"invalid_cron_name":     {Name: "", Schedule: "@daily", Command: "echo", RunAs: "root"},
	}
	for code, j := range cases {
		var ue *usererr.Error
		if err := m.Check(ctx, j); !errors.As(err, &ue) || ue.Code != code {
			t.Errorf("%s: err = %v", code, err)
		}
	}
}
