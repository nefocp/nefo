package hosting

import (
	"bytes"
	"context"
	"runtime"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/software"
	"github.com/nefocp/nefo/internal/store"
)

// Every PHP version the panel offers gets a pool its PHP-FPM accepts, with
// every option the panel can turn on; the output does not depend on the
// version, so the pools of 7.4 and later stay byte for byte what they were
// (the golden files).
func TestPoolForEveryPHPVersion(t *testing.T) {
	variants := []poolData{
		{},
		{PM: "dynamic", Workers: 12, MaxInputVars: 5000},
		{Allow: HardenedFunctions, MemoryMB: 512, UploadMB: 128, TimeSec: 600},
	}
	for _, v := range software.PHPVersions {
		for i, d := range variants {
			d.Domain, d.User, d.Root, d.Socket = "example.com", "example_com", "/var/www/example.com", "/run/php/panel-example_com.sock"
			plain, err := renderPool(d)
			if err != nil {
				t.Fatalf("variant %d: %v", i, err)
			}
			d.PHP = v
			out, err := renderPool(d)
			if err != nil {
				t.Fatalf("PHP %s, variant %d: %v", v, i, err)
			}
			if !bytes.Equal(out, plain) {
				t.Fatalf("PHP %s, variant %d: the pool differs from the version-less one:\n%s", v, i, out)
			}
			if err := checkPool(out, v); err != nil {
				t.Fatalf("PHP %s: %v", v, err)
			}
		}
	}
}

func TestPoolDirectivesOldFPM(t *testing.T) {
	line := func(s string) []byte { return []byte("[u]\nuser = u\n" + s + "\n") }
	for _, c := range []struct {
		line, php string
		ok        bool
	}{
		{"catch_workers_output = yes", "5.6", true},
		{"clear_env = no", "5.6", true},
		{"decorate_workers_output = no", "7.2", false},
		{"decorate_workers_output = no", "7.3", true},
		{"request_slowlog_trace_depth = 20", "7.1", false},
		{"pm.max_spawn_rate = 32", "8.0", false},
		{"pm.max_spawn_rate = 32", "8.1", true},
		{"pm.status_listen = 127.0.0.1:9001", "7.4", false},
		{"php_admin_value[sys_temp_dir] = /tmp", "5.6", true},
		{"php_admin_value[opcache.jit] = 1255", "8.3", false}, // not listed: refused for every version
		{"php_value[memory_limit] = 128M", "", true},
		{"env[PATH] = /usr/bin", "5.6", true},
		{"no_such_directive = 1", "8.5", false},
		{"listen", "8.3", false},
	} {
		err := checkPool(line(c.line), c.php)
		if (err == nil) != c.ok {
			t.Errorf("%q on PHP %q: %v", c.line, c.php, err)
		}
	}
	// A version that is not one is refused.
	if _, err := renderPool(poolData{Domain: "example.com", User: "u", Root: "/var/www/example.com", Socket: "/run/php/x.sock", PHP: "8.3\nuser = root"}); err == nil {
		t.Fatal("rendered for a bad version")
	}
}

func TestPoolBytesUsesSitePHP(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("real paths: Linux only")
	}
	m := newTestManager(t)
	out, err := m.poolBytes(&store.Site{Domain: "old.example.com", SysUser: "old_example_com", PHPVersion: "5.6"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "[old_example_com]") || checkPool(out, "5.6") != nil {
		t.Fatalf("pool:\n%s", out)
	}
	if _, err := m.poolBytes(&store.Site{Domain: "x.example.com", SysUser: "x_example_com", PHPVersion: "8.3; rm"}); err == nil {
		t.Fatal("a pool for a bad version")
	}
}

func TestDistroPHPSkipsThePHPRepository(t *testing.T) {
	// Ubuntu 24.04 with packages.sury.org added: apt's candidate php-fpm is
	// the repository's, for PHP 8.6; the distribution's is 8.3.
	sury := "Package: php-fpm\nVersion: 2:8.6+106~exp1+debsuryorg24.04.1+1\nDepends: php8.6-fpm\n\n" +
		"Package: php-fpm\nVersion: 2:8.3+93ubuntu2\nDepends: php8.3-fpm\n\n"
	ppa := "Package: php-fpm\nVersion: 2:8.4+96+ubuntu22.04.1+deb.sury.org+1\nDepends: php8.4-fpm\n\n" +
		"Package: php-fpm\nVersion: 2:8.1+92ubuntu1\nDepends: php8.1-fpm\n"
	for _, c := range []struct{ out, want string }{
		{sury, "8.3"},
		{ppa, "8.1"},
		{"Package: php-fpm\nVersion: 2:8.2+93\nDepends: php8.2-fpm\n", "8.2"},              // Debian 12 alone
		{"Package: php-fpm\nVersion: 2:8.6+106+debsuryorg12+1\nDepends: php8.6-fpm\n", ""}, // only the repository's lists
		{"", ""},
	} {
		if got := distroPHPFrom(c.out, nil); got != c.want {
			t.Errorf("%q: got %q, want %q", c.out, got, c.want)
		}
	}
	// An older build of the repository, without "sury" in its version:
	// madison says where it comes from.
	old := "Package: php-fpm\nVersion: 2:93+0~20240105.50+debian12~1.gbp1a2b3c\nDepends: php8.3-fpm\n\n" +
		"Package: php-fpm\nVersion: 2:8.2+93\nDepends: php8.2-fpm\n"
	madison := "   php-fpm | 2:93+0~20240105.50+debian12~1.gbp1a2b3c | https://packages.sury.org/php bookworm/main amd64 Packages\n" +
		"   php-fpm |    2:8.2+93 | http://deb.debian.org/debian bookworm/main amd64 Packages\n"
	if got := distroPHPFrom(old, phpRepoVersions(madison)); got != "8.2" {
		t.Errorf("old repository build: %q", got)
	}
	ppaMadison := "   php-fpm | 2:8.4+96+ubuntu22.04.1+deb.sury.org+1 | https://ppa.launchpadcontent.net/ondrej/php/ubuntu jammy/main amd64 Packages\n"
	if !phpRepoVersions(ppaMadison)["2:8.4+96+ubuntu22.04.1+deb.sury.org+1"] {
		t.Error("the PPA's version")
	}
	m := &Manager{run: &fakeRunner{outputs: map[string]fakeResult{"apt-cache": {out: sury}}}}
	if got := m.distroPHP(context.Background()); got != "8.3" {
		t.Fatalf("distroPHP %q", got)
	}
}
