package systools

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// What apt-get -s upgrade --with-new-pkgs prints on Ubuntu 22.04.
const ubuntuSim = `NOTE: This is only a simulation!
      apt-get needs root privileges for real execution.
      Keep also in mind that locking is deactivated,
      so don't depend on the relevance to the real current situation!
Reading package lists...
Building dependency tree...
Reading state information...
Calculating upgrade...
The following NEW packages will be installed:
  linux-image-5.15.0-92-generic
The following packages have been kept back:
  ubuntu-advantage-tools
The following upgrades have been deferred due to phasing:
  systemd systemd-sysv
The following packages will be upgraded:
  libssl3 linux-image-generic nginx openssl php8.3-fpm tzdata
6 upgraded, 1 newly installed, 0 to remove and 3 not upgraded.
Inst libssl3 [3.0.2-0ubuntu1.14] (3.0.2-0ubuntu1.15 Ubuntu:22.04/jammy-updates, Ubuntu:22.04/jammy-security [amd64])
Inst openssl [3.0.2-0ubuntu1.14] (3.0.2-0ubuntu1.15 Ubuntu:22.04/jammy-updates, Ubuntu:22.04/jammy-security [amd64])
Inst linux-image-5.15.0-92-generic (5.15.0-92.102 Ubuntu:22.04/jammy-updates, Ubuntu:22.04/jammy-security [amd64])
Inst linux-image-generic [5.15.0.91.88] (5.15.0.92.89 Ubuntu:22.04/jammy-updates, Ubuntu:22.04/jammy-security [amd64])
Inst nginx [1.18.0-6ubuntu14.4] (1.18.0-6ubuntu14.5 Ubuntu:22.04/jammy-updates [amd64]) []
Inst php8.3-fpm [8.3.6-1+ubuntu22.04.1+deb.sury.org+1] (8.3.7-1+ubuntu22.04.1+deb.sury.org+1 LP-PPA-ondrej-php:22.04/jammy [amd64])
Inst tzdata [2024a-0ubuntu0.22.04] (2024a-0ubuntu0.22.04.1 Ubuntu:22.04/jammy-updates [all])
Conf libssl3 (3.0.2-0ubuntu1.15 Ubuntu:22.04/jammy-updates, Ubuntu:22.04/jammy-security [amd64])
Conf openssl (3.0.2-0ubuntu1.15 Ubuntu:22.04/jammy-updates, Ubuntu:22.04/jammy-security [amd64])
`

// And on Debian 12.
const debianSim = `Reading package lists...
Calculating upgrade...
The following packages will be upgraded:
  libc6 libssl3 mariadb-server
Inst libc6 [2.36-9+deb12u3] (2.36-9+deb12u4 Debian:12.5/stable [amd64])
Inst libssl3 [3.0.11-1~deb12u1] (3.0.11-1~deb12u2 Debian-Security:12/stable-security [amd64])
Inst mariadb-server [1:10.11.4-1~deb12u1] (1:10.11.6-0+deb12u1 Debian:12.5/stable, Debian-Security:12/stable-security [amd64])
`

func TestParseSimulation(t *testing.T) {
	c := parseSimulation(ubuntuSim)
	if len(c.Packages) != 7 {
		t.Fatalf("packages = %+v", c.Packages)
	}
	want := map[string]Package{
		"libssl3":                       {Name: "libssl3", Current: "3.0.2-0ubuntu1.14", Version: "3.0.2-0ubuntu1.15", Origin: "jammy-security", Security: true},
		"linux-image-5.15.0-92-generic": {Name: "linux-image-5.15.0-92-generic", Current: "", Version: "5.15.0-92.102", Origin: "jammy-security", Security: true},
		"nginx":                         {Name: "nginx", Current: "1.18.0-6ubuntu14.4", Version: "1.18.0-6ubuntu14.5", Origin: "jammy-updates"},
		"php8.3-fpm":                    {Name: "php8.3-fpm", Current: "8.3.6-1+ubuntu22.04.1+deb.sury.org+1", Version: "8.3.7-1+ubuntu22.04.1+deb.sury.org+1", Origin: "jammy"},
		"tzdata":                        {Name: "tzdata", Current: "2024a-0ubuntu0.22.04", Version: "2024a-0ubuntu0.22.04.1", Origin: "jammy-updates"},
	}
	for _, p := range c.Packages {
		if w, ok := want[p.Name]; ok && p != w {
			t.Errorf("%s = %+v, want %+v", p.Name, p, w)
		}
	}
	if strings.Join(c.Held, " ") != "ubuntu-advantage-tools systemd systemd-sysv" {
		t.Errorf("held = %q", c.Held)
	}

	d := parseSimulation(debianSim)
	sec := 0
	for _, p := range d.Packages {
		if p.Security {
			sec++
		}
	}
	if len(d.Packages) != 3 || sec != 2 || d.Packages[2].Current != "1:10.11.4-1~deb12u1" || d.Packages[2].Origin != "stable-security" {
		t.Fatalf("debian = %+v", d.Packages)
	}
	if len(d.Held) != 0 {
		t.Fatalf("held = %q", d.Held)
	}
	if r := restartsFor(append(c.Packages, d.Packages...)); strings.Join(r, ",") != "Nginx,PHP,MariaDB" {
		t.Fatalf("restarts = %q", r)
	}
}

func TestCheckCachesAndPersists(t *testing.T) {
	e := newTest(t)
	ctx := context.Background()
	e.f.answers["apt-get -s upgrade"] = ubuntuSim
	if u := e.m.Updates(ctx); u.CheckedAt != nil || len(u.Packages) != 0 {
		t.Fatalf("before any check: %+v", u)
	}
	if checked, err := e.m.CheckIfStale(ctx, CheckEvery); err != nil || !checked {
		t.Fatalf("first check: %v %v", checked, err)
	}
	if !e.f.ran("APT update") || !e.f.ran("apt-get -s upgrade --with-new-pkgs") {
		t.Fatalf("calls = %q", e.f.calls)
	}
	u := e.m.Updates(ctx)
	if u.CheckedAt == nil || !u.CheckedAt.Equal(e.now) || len(u.Packages) != 7 || u.Security != 4 {
		t.Fatalf("updates = %+v", u)
	}

	// Within six hours, nothing runs; the page only reads the cache.
	e.f.reset()
	e.now = e.now.Add(5 * time.Hour)
	if checked, _ := e.m.CheckIfStale(ctx, CheckEvery); checked || len(e.f.calls) != 0 {
		t.Fatalf("fresh check ran: %q", e.f.calls)
	}

	// A new manager (the panel restarted) reads the last check from the settings.
	m2 := New(e.f, e.st)
	m2.p, m2.now = e.m.p, e.m.now
	if u := m2.Updates(ctx); u.CheckedAt == nil || len(u.Packages) != 7 {
		t.Fatalf("after restart: %+v", u)
	}
	if checked, _ := m2.CheckIfStale(ctx, CheckEvery); checked {
		t.Fatal("checked again after a restart")
	}
	e.now = e.now.Add(2 * time.Hour)
	if checked, err := m2.CheckIfStale(ctx, CheckEvery); !checked || err != nil {
		t.Fatalf("stale check: %v %v", checked, err)
	}

	// No check while another change holds the manager.
	e.now = e.now.Add(24 * time.Hour)
	e.m.mu.Lock()
	checked, _ := e.m.CheckIfStale(ctx, CheckEvery)
	e.m.mu.Unlock()
	if checked {
		t.Fatal("checked while busy")
	}
}

func TestUpgrade(t *testing.T) {
	e := newTest(t)
	ctx := context.Background()
	e.f.answers["apt-get -s upgrade"] = ubuntuSim
	var lg testLog
	n, err := e.m.Upgrade(ctx, true, &lg)
	if err != nil || n != 3 {
		t.Fatalf("security upgrade: %d %v", n, err)
	}
	// Only installed packages with a security origin, by name: the new
	// kernel comes with linux-image-generic.
	if !e.f.ran("APT install --only-upgrade libssl3 openssl linux-image-generic") || e.f.ran("APT upgrade") {
		t.Fatalf("calls = %q", e.f.calls)
	}

	e.f.reset()
	if n, err := e.m.Upgrade(ctx, false, &lg); err != nil || n != 7 || !e.f.ran("APT upgrade --with-new-pkgs") {
		t.Fatalf("full upgrade: %d %v %q", n, err, e.f.calls)
	}
	if !strings.Contains(lg.String(), "restart Nginx, PHP for a moment") {
		t.Fatalf("log = %s", lg.String())
	}

	// Nothing to install: no apt-get install at all.
	e.f.reset()
	e.f.answers["apt-get -s upgrade"] = "0 upgraded, 0 newly installed, 0 to remove and 0 not upgraded.\n"
	if n, err := e.m.Upgrade(ctx, true, &lg); err != nil || n != 0 || e.f.ran("APT install") {
		t.Fatalf("nothing: %d %v %q", n, err, e.f.calls)
	}
	if u := e.m.Updates(ctx); len(u.Packages) != 0 {
		t.Fatalf("cache not updated: %+v", u.Packages)
	}

	// A failed install still records what is left.
	e.f.answers["apt-get -s upgrade"] = debianSim
	e.f.fail["APT upgrade"] = "E: dpkg was interrupted"
	if _, err := e.m.Upgrade(ctx, false, &lg); err == nil {
		t.Fatal("failed apt-get upgrade succeeded")
	}
	if u := e.m.Updates(ctx); len(u.Packages) != 3 {
		t.Fatalf("after a failure: %+v", u.Packages)
	}
}

func TestReboot(t *testing.T) {
	e := newTest(t)
	ctx := context.Background()
	if r := e.m.Updates(ctx).Reboot; r.Required || r.Packages == nil {
		t.Fatalf("reboot = %+v", r)
	}
	e.write(t, e.m.p.RebootRequired, "*** System restart required ***\n")
	e.write(t, e.m.p.RebootPkgs, "linux-image-5.15.0-92-generic\nlibssl3\nlibssl3\n")
	if r := e.m.Updates(ctx).Reboot; !r.Required || strings.Join(r.Packages, " ") != "linux-image-5.15.0-92-generic libssl3" {
		t.Fatalf("reboot = %+v", r)
	}

	// Debian: no reboot-required file, but a newer kernel of the same flavour.
	os.Remove(e.m.p.RebootRequired)
	e.write(t, e.m.p.OSRelease, "6.1.0-17-amd64\n")
	for _, k := range []string{"vmlinuz-6.1.0-17-amd64", "vmlinuz-6.1.0-9-amd64", "vmlinuz-6.1.0-25-cloud-amd64"} {
		e.write(t, filepath.Join(e.m.p.Boot, k), "")
	}
	if r := e.m.Updates(ctx).Reboot; r.Required {
		t.Fatalf("older and other kernels: %+v", r)
	}
	e.write(t, filepath.Join(e.m.p.Boot, "vmlinuz-6.1.0-18-amd64"), "")
	if r := e.m.Updates(ctx).Reboot; !r.Required || r.Kernel != "6.1.0-18-amd64" {
		t.Fatalf("newer kernel: %+v", r)
	}
}

func TestCompareVersions(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"5.15.0-100-generic", "5.15.0-91-generic", 1},
		{"5.15.0-91-generic", "5.15.0-91-generic", 0},
		{"6.1.0-9-amd64", "6.1.0-17-amd64", -1},
		{"6.8.0", "6.8.0-1", -1},
	} {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestAutoUpdates(t *testing.T) {
	e := newTest(t)
	ctx := context.Background()
	// apt-config answers from the file, as apt would.
	e.f.fn["apt-config shell"] = func(string) (string, error) {
		b, _ := os.ReadFile(e.m.p.AutoUpgrades)
		out := ""
		for _, k := range []string{"Update-Package-Lists", "Unattended-Upgrade"} {
			for _, l := range strings.Split(string(b), "\n") {
				if mm := periodicRe.FindStringSubmatch(l); mm != nil && mm[1] == k {
					v := strings.Fields(l)[1]
					out += map[string]string{"Update-Package-Lists": "UL", "Unattended-Upgrade": "UU"}[k] + "='" + strings.Trim(v, `";`) + "'\n"
				}
			}
		}
		return out, nil
	}
	e.f.hook = func(line string) {
		if line == "APT install --no-install-recommends unattended-upgrades" {
			e.write(t, e.m.p.Unattended, "#!/usr/bin/python3\n")
		}
	}
	if a := e.m.Updates(ctx).Auto; a.Installed || a.Enabled || !e.m.NeedsInstall() {
		t.Fatalf("auto = %+v", a)
	}
	// Turning it off while it is not installed changes nothing.
	if err := e.m.SetAutoUpdates(ctx, false, &testLog{}); err != nil || exists(e.m.p.AutoUpgrades) {
		t.Fatalf("off without the package: %v", err)
	}

	e.write(t, e.m.p.AutoUpgrades, "// my settings\nAPT::Periodic::AutocleanInterval \"7\";\nAPT::Periodic::Unattended-Upgrade \"0\";\n")
	if err := e.m.SetAutoUpdates(ctx, true, &testLog{}); err != nil {
		t.Fatal(err)
	}
	if !e.f.ran("APT install --no-install-recommends unattended-upgrades") || !e.f.ran("systemctl enable --now apt-daily.timer apt-daily-upgrade.timer") {
		t.Fatalf("calls = %q", e.f.calls)
	}
	want := "// my settings\nAPT::Periodic::AutocleanInterval \"7\";\nAPT::Periodic::Unattended-Upgrade \"1\";\nAPT::Periodic::Update-Package-Lists \"1\";\n"
	if got := read(t, e.m.p.AutoUpgrades); got != want {
		t.Fatalf("20auto-upgrades =\n%s", got)
	}
	if a := e.m.Updates(ctx).Auto; !a.Installed || !a.Enabled {
		t.Fatalf("auto = %+v", a)
	}

	e.f.reset()
	if err := e.m.SetAutoUpdates(ctx, false, &testLog{}); err != nil {
		t.Fatal(err)
	}
	if e.f.ran("APT") || !strings.Contains(read(t, e.m.p.AutoUpgrades), "APT::Periodic::Unattended-Upgrade \"0\";") {
		t.Fatalf("off: %q\n%s", e.f.calls, read(t, e.m.p.AutoUpgrades))
	}
	if a := e.m.Updates(ctx).Auto; a.Enabled {
		t.Fatalf("still on: %+v", a)
	}

	// Another file turns it off again: the panel says so and puts its file back.
	before := read(t, e.m.p.AutoUpgrades)
	e.f.fn["apt-config shell"] = func(string) (string, error) { return "UL='1'\nUU='0'\n", nil }
	if err := e.m.SetAutoUpdates(ctx, true, &testLog{}); code(err) != "auto_updates_overridden" {
		t.Fatalf("overridden: %v", err)
	}
	if read(t, e.m.p.AutoUpgrades) != before {
		t.Fatal("file not restored")
	}
}

func TestSetPeriodicNewFile(t *testing.T) {
	got := setPeriodic("", map[string]string{"Update-Package-Lists": "1", "Unattended-Upgrade": "1"})
	want := "APT::Periodic::Update-Package-Lists \"1\";\nAPT::Periodic::Unattended-Upgrade \"1\";\n"
	if got != want {
		t.Fatalf("got %q", got)
	}
	if !slices.Equal(restartsFor(nil), []string{}) {
		t.Fatal("restarts of nothing")
	}
}

// Lists refreshed lately (Ubuntu's apt-daily, an administrator's apt
// update) are read as they are: the background check then takes no apt
// lock that an apt command typed at the same moment would trip over.
func TestBackgroundCheckUsesFreshLists(t *testing.T) {
	e := newTest(t)
	ctx := context.Background()
	e.f.answers["apt-get -s upgrade"] = ubuntuSim
	if err := os.MkdirAll(e.m.p.AptLists, 0o755); err != nil {
		t.Fatal(err)
	}
	rel := filepath.Join(e.m.p.AptLists, "archive.ubuntu.com_ubuntu_dists_noble-updates_InRelease")
	os.WriteFile(rel, nil, 0o644)
	os.WriteFile(filepath.Join(e.m.p.AptLists, "lock"), nil, 0o640)
	os.Chtimes(rel, e.now.Add(-time.Hour), e.now.Add(-time.Hour))
	if checked, err := e.m.CheckIfStale(ctx, CheckEvery); err != nil || !checked {
		t.Fatalf("check: %v %v", checked, err)
	}
	if e.f.ran("APT update") || !e.f.ran("apt-get -s upgrade --with-new-pkgs") {
		t.Fatalf("calls = %q", e.f.calls)
	}
	if u := e.m.Updates(ctx); len(u.Packages) != 7 {
		t.Fatalf("updates = %+v", u)
	}
	// Old lists: refreshed first.
	e.f.reset()
	e.now = e.now.Add(7 * time.Hour)
	if checked, err := e.m.CheckIfStale(ctx, CheckEvery); err != nil || !checked || !e.f.ran("APT update") {
		t.Fatalf("stale lists: %v %v %q", checked, err, e.f.calls)
	}
}
