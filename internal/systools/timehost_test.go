package systools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const timedatectlShow = `Timezone=Asia/Ho_Chi_Minh
LocalRTC=no
CanNTP=yes
NTP=yes
NTPSynchronized=yes
TimeUSec=Thu 2026-11-26 15:00:00 +07
RTCTimeUSec=Thu 2026-11-26 08:00:00 +07
`

func TestTime(t *testing.T) {
	e := newTest(t)
	ctx := context.Background()
	e.f.answers["timedatectl show"] = timedatectlShow
	ti := e.m.Time(ctx)
	if !ti.Available || ti.Timezone != "Asia/Ho_Chi_Minh" || !ti.NTP || !ti.Synced || !ti.CanNTP || ti.LocalRTC || ti.Hostname != "old-name" {
		t.Fatalf("time = %+v", ti)
	}
	// The clock in the server's zone (when this machine has the zone data).
	if ti.Time != "2026-11-26T15:00:00+07:00" && ti.Time != "2026-11-26T08:00:00Z" {
		t.Fatalf("time = %q", ti.Time)
	}
}

func TestSetTimezoneAndNTP(t *testing.T) {
	e := newTest(t)
	ctx := context.Background()
	e.f.answers["timedatectl list-timezones"] = "Africa/Abidjan\nAmerica/Argentina/Buenos_Aires\nAsia/Ho_Chi_Minh\nEurope/Berlin\nUTC\n"
	e.f.answers["timedatectl show"] = "Timezone=UTC\nCanNTP=no\nNTP=no\n"
	for _, bad := range []string{"", "Asia/Ho Chi Minh", "../../etc/passwd", "-u", "Mars/Olympus"} {
		if err := e.m.SetTimezone(ctx, bad); code(err) != "timezone_invalid" {
			t.Errorf("%q: %v", bad, err)
		}
	}
	if e.f.ran("timedatectl set-timezone") {
		t.Fatalf("calls = %q", e.f.calls)
	}
	if err := e.m.SetTimezone(ctx, "America/Argentina/Buenos_Aires"); err != nil || !e.f.ran("timedatectl set-timezone America/Argentina/Buenos_Aires") ||
		!e.f.ran("systemctl try-restart cron.service") {
		t.Fatalf("set: %v %q", err, e.f.calls)
	}
	// The list is read once.
	e.f.reset()
	e.m.SetTimezone(ctx, "UTC")
	if e.f.ran("timedatectl list-timezones") {
		t.Fatal("list read again")
	}

	if err := e.m.SetNTP(ctx, true); code(err) != "ntp_unavailable" {
		t.Fatalf("no NTP service: %v", err)
	}
	if err := e.m.SetNTP(ctx, false); err != nil || !e.f.ran("timedatectl set-ntp false") {
		t.Fatalf("off: %v", err)
	}
	e.f.answers["timedatectl show"] = timedatectlShow
	if err := e.m.SetNTP(ctx, true); err != nil || !e.f.ran("timedatectl set-ntp true") {
		t.Fatalf("on: %v", err)
	}
}

func TestValidHostname(t *testing.T) {
	good := map[string]string{"web1": "web1", " Web-1.Example.COM ": "web-1.example.com", "a": "a", "1and1": "1and1"}
	for in, want := range good {
		if got, err := ValidHostname(in); err != nil || got != want {
			t.Errorf("%q = %q, %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "-web", "web-", "web_1", "web..x", "web.", "a b", "192.0.2.1", "localhost",
		"x$(id)", string(make([]byte, 65)), "a.bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"} {
		if _, err := ValidHostname(bad); code(err) != "hostname_invalid" {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestSetHostname(t *testing.T) {
	e := newTest(t)
	ctx := context.Background()
	e.write(t, e.m.p.Hosts, "127.0.0.1 localhost\n127.0.1.1 old-name old-name\n\n# IPv6\n::1 localhost ip6-localhost\n")
	if err := e.m.SetHostname(ctx, "Web1.Example.com"); err != nil {
		t.Fatal(err)
	}
	if !e.f.ran("hostnamectl set-hostname web1.example.com") {
		t.Fatalf("calls = %q", e.f.calls)
	}
	want := "127.0.0.1 localhost\n127.0.1.1\tweb1.example.com web1\n\n# IPv6\n::1 localhost ip6-localhost\n"
	if got := read(t, e.m.p.Hosts); got != want {
		t.Fatalf("hosts =\n%s", got)
	}
	// No cloud-init here: no file for it.
	if exists(filepath.Join(e.m.p.CloudCfgDir, "99-panel-hostname.cfg")) {
		t.Fatal("cloud-init file without cloud-init")
	}

	// A hosts file without a 127.0.1.1 line gets one; cloud-init keeps the name.
	e.write(t, e.m.p.Hosts, "127.0.0.1 localhost\n::1 localhost\n")
	os.MkdirAll(e.m.p.CloudCfgDir, 0o755)
	if err := e.m.SetHostname(ctx, "db"); err != nil {
		t.Fatal(err)
	}
	if got := read(t, e.m.p.Hosts); got != "127.0.0.1 localhost\n127.0.1.1\tdb\n::1 localhost\n" {
		t.Fatalf("hosts =\n%s", got)
	}
	if got := read(t, filepath.Join(e.m.p.CloudCfgDir, "99-panel-hostname.cfg")); !strings.Contains(got, "preserve_hostname: true") {
		t.Fatalf("cloud-init = %q", got)
	}

	// hostnamectl fails (no D-Bus): /etc/hosts stays as it was.
	e.f.fail["hostnamectl"] = "Failed to connect to bus: No such file or directory"
	before := read(t, e.m.p.Hosts)
	if err := e.m.SetHostname(ctx, "other"); err == nil || errors.Is(err, os.ErrNotExist) {
		t.Fatalf("hostnamectl failure: %v", err)
	}
	if read(t, e.m.p.Hosts) != before {
		t.Fatal("hosts changed")
	}
	if err := e.m.SetHostname(ctx, "bad name"); code(err) != "hostname_invalid" {
		t.Fatalf("bad name: %v", err)
	}
}
