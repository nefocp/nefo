package software

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/store"
)

// osRelease points osReleaseFile and legacyArch at a system for one test.
func osRelease(t *testing.T, id, versionID, codename, arch string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "os-release")
	name := map[string]string{"ubuntu": "Ubuntu", "debian": "Debian GNU/Linux"}[id]
	body := "NAME=\"" + name + "\"\nID=" + id + "\nVERSION_ID=\"" + versionID + "\"\nVERSION_CODENAME=" + codename + "\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	oldFile, oldArch := osReleaseFile, legacyArch
	osReleaseFile, legacyArch = p, arch
	t.Cleanup(func() { osReleaseFile, legacyArch = oldFile, oldArch })
}

func installPHPForTest(t *testing.T, versions ...string) {
	t.Helper()
	for _, v := range versions {
		touch(t, filepath.Join(phpEtc, v, "fpm", "php-fpm.conf"))
	}
}

func TestLegacyVersionsListed(t *testing.T) {
	if !slices.Equal(PHPVersions[:len(LegacyPHPVersions)], LegacyPHPVersions) {
		t.Fatalf("PHPVersions %v must start with the old versions %v, oldest first", PHPVersions, LegacyPHPVersions)
	}
	for _, v := range PHPVersions {
		if IsLegacyPHP(v) != !phpAtLeast(v, "7.4") {
			t.Errorf("PHP %s: legacy %v", v, IsLegacyPHP(v))
		}
	}
}

func TestPHPExtensionsPerVersion(t *testing.T) {
	// json is a package of its own before PHP 8.0; mcrypt comes with 5.6.
	for v, want := range map[string]string{
		"5.6": "fpm cli mysql curl gd mbstring xml zip intl bcmath opcache json",
		"7.0": "fpm cli mysql curl gd mbstring xml zip intl bcmath opcache json",
		"7.3": "fpm cli mysql curl gd mbstring xml zip intl bcmath opcache json",
		"7.4": "fpm cli mysql curl gd mbstring xml zip intl bcmath opcache json",
		"8.0": "fpm cli mysql curl gd mbstring xml zip intl bcmath opcache",
		"8.5": "fpm cli mysql curl gd mbstring xml zip intl bcmath opcache",
	} {
		if got := strings.Join(phpExtensionsFor(v), " "); got != want {
			t.Errorf("PHP %s: %s", v, got)
		}
	}
	if !slices.Equal(phpExtrasFor("5.6"), []string{"mcrypt"}) || phpExtrasFor("7.0") != nil || phpExtrasFor("8.3") != nil {
		t.Error("mcrypt is for PHP 5.6 only")
	}
	// The base list itself is unchanged (the hosting package installs the
	// same set with the distribution's PHP).
	if strings.Join(PHPExtensions, " ") != "fpm cli mysql curl gd mbstring xml zip intl bcmath opcache" {
		t.Error("PHPExtensions changed")
	}
}

// Every php.ini key the panel writes (99-panel.ini) exists in PHP 5.6, the
// oldest version offered: first versions from the php.ini directives list,
// https://www.php.net/manual/en/ini.list.php. A key for newer versions only
// (opcache.jit is 8.0+) must come with a per-version rule first.
func TestIniRulesOldPHP(t *testing.T) {
	since := map[string]string{
		"max_input_vars": "5.3", "max_input_time": "4.0", "date.timezone": "5.1", "display_errors": "4.0",
		"short_open_tag": "4.0", "allow_url_fopen": "4.0", "expose_php": "4.0", "default_socket_timeout": "4.3",
		"session.gc_maxlifetime": "4.0", "realpath_cache_size": "5.1",
		"opcache.enable": "5.5", "opcache.memory_consumption": "5.5", "opcache.max_accelerated_files": "5.5",
		"opcache.validate_timestamps": "5.5", "opcache.revalidate_freq": "5.5",
	}
	for k := range iniRules {
		v, ok := since[k]
		if !ok {
			t.Errorf("%s: which PHP versions have it? Add it here, and only offer it where it exists", k)
			continue
		}
		if !phpAtLeast(LegacyPHPVersions[0], v) {
			t.Errorf("%s needs PHP %s", k, v)
		}
	}
}

func TestLegacyOffered(t *testing.T) {
	sandbox(t)
	for _, c := range []struct {
		id, ver, codename, arch, want string
	}{
		{"ubuntu", "22.04", "jammy", "amd64", ""},
		{"ubuntu", "24.04", "noble", "arm64", ""},
		{"ubuntu", "26.04", "resolute", "amd64", ""},
		{"debian", "12", "bookworm", "amd64", ""},
		{"debian", "13", "trixie", "arm64", ""},
		{"debian", "13", "trixie", "riscv64", "php_not_published:Debian GNU/Linux 13 (riscv64), packages.sury.org"},
		{"fedora", "41", "", "amd64", "unsupported:"},
	} {
		osRelease(t, c.id, c.ver, c.codename, c.arch)
		code, detail := legacyOffered()
		if got := code + ":" + detail; (c.want == "" && code != "") || (c.want != "" && got != c.want) {
			t.Errorf("%s %s %s: %q", c.id, c.codename, c.arch, got)
		}
	}
	// A 26.04 server still on the frozen PPA (the move to sury failed):
	// the PPA has nothing for it.
	ppaFiles(t)
	osRelease(t, "ubuntu", "26.04", "resolute", "amd64")
	if code, detail := legacyOffered(); code+":"+detail != "php_not_published:Ubuntu 26.04, ppa:ondrej/php" {
		t.Errorf("26.04 on the PPA: %s %s", code, detail)
	}
}

func TestCatalogMarksOldVersions(t *testing.T) {
	sandbox(t)
	m, _, _ := newTestManager(t)
	ctx := context.Background()
	osRelease(t, "ubuntu", "24.04", "noble", "amd64")
	byID := func() map[string]Item {
		items, err := m.Catalog(ctx)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]Item{}
		for _, it := range items {
			out[it.ID] = it
		}
		return out
	}
	// No supported PHP yet: an old one would be the server's only PHP.
	items := byID()
	if it := items["php5.6"]; !it.EOL || it.NotOffered != "php_needs_supported" || !it.ThirdParty {
		t.Fatalf("php5.6 without a supported version: %+v", it)
	}
	if it := items["php8.3"]; it.EOL || it.NotOffered != "" {
		t.Fatalf("php8.3: %+v", it)
	}
	installPHPForTest(t, "8.3")
	if it := byID()["php7.3"]; !it.EOL || it.NotOffered != "" {
		t.Fatalf("php7.3 with PHP 8.3 installed: %+v", it)
	}
	if it := byID()["php7.4"]; it.Legacy || !it.EOL || it.NotOffered != "" {
		t.Fatalf("7.4 is past its date but not in the old group: %+v", it)
	}
	// From sury, Ubuntu 26.04 has them too; still on the frozen PPA, it has not.
	osRelease(t, "ubuntu", "26.04", "resolute", "amd64")
	if it := byID()["php7.0"]; it.NotOffered != "" {
		t.Fatalf("php7.0 on 26.04: %+v", it)
	}
	ppa := ppaFiles(t)
	if it := byID()["php7.0"]; it.NotOffered != "php_not_published" || !strings.Contains(it.NotOfferedDetail, "26.04") {
		t.Fatalf("php7.0 on 26.04 with the PPA: %+v", it)
	}
	for _, p := range ppa {
		os.Remove(p)
	}
	// Installed next to an old version, the last supported one stays.
	osRelease(t, "ubuntu", "24.04", "noble", "amd64")
	installPHPForTest(t, "5.6")
	items = byID()
	if it := items["php8.3"]; it.Removable || it.Blocked != "php_last_supported" {
		t.Fatalf("php8.3 next to 5.6: %+v", it)
	}
	if it := items["php5.6"]; !it.Removable || !it.EOL {
		t.Fatalf("php5.6: %+v", it)
	}
}

func TestCheckInstallNeedsConfirmation(t *testing.T) {
	sandbox(t)
	m, _, _ := newTestManager(t)
	ctx := context.Background()
	osRelease(t, "debian", "12", "bookworm", "amd64")
	if err := m.CheckInstall(ctx, "php5.6", true); code(err) != "php_needs_supported:5.6" {
		t.Fatalf("no supported PHP: %v", err)
	}
	installPHPForTest(t, "8.2")
	if err := m.CheckInstall(ctx, "php5.6", false); code(err) != "php_eol_confirm:5.6" {
		t.Fatalf("not confirmed: %v", err)
	}
	if err := m.CheckInstall(ctx, "php7.3", true); err != nil {
		t.Fatalf("confirmed: %v", err)
	}
	// Supported versions and other items need no confirmation.
	for _, id := range []string{"php7.4", "php8.4", "redis", "composer"} {
		if err := m.CheckInstall(ctx, id, false); err != nil {
			t.Errorf("%s: %v", id, err)
		}
	}
	osRelease(t, "ubuntu", "26.04", "resolute", "arm64")
	if err := m.CheckInstall(ctx, "php7.2", true); err != nil {
		t.Fatalf("26.04 from sury: %v", err)
	}
	ppaFiles(t) // still on the frozen PPA
	if err := m.CheckInstall(ctx, "php7.2", true); !strings.HasPrefix(code(err), "php_not_published:") {
		t.Fatalf("26.04: %v", err)
	}
	// Install checks the same, even without the API in front.
	if err := m.Install(ctx, "php7.2", &nopLog{}); !strings.HasPrefix(code(err), "php_not_published:") {
		t.Fatalf("Install on 26.04: %v", err)
	}
}

func TestOldVersionIsNeverTheDefault(t *testing.T) {
	sandbox(t)
	m, f, st := newTestManager(t)
	ctx := context.Background()
	installPHPForTest(t, "8.3", "5.6", "7.1")
	addSite(t, st, &store.Site{Domain: "old.test", PHPVersion: "5.6"})
	// 8.3 is the only supported version: it stays while old ones are there.
	if err := m.CheckRemove(ctx, "php8.3", RemoveOptions{}); code(err) != "php_last_supported:8.3" {
		t.Fatalf("remove 8.3: %v", err)
	}
	if err := m.Remove(ctx, "php8.3", RemoveOptions{}, &nopLog{}); code(err) != "php_last_supported:8.3" || len(f.apt) != 0 {
		t.Fatalf("Remove 8.3: %v, apt %q", err, f.apt)
	}
	if err := m.CheckRemove(ctx, "php5.6", RemoveOptions{}); code(err) != "php_in_use:5.6" {
		t.Fatalf("remove 5.6: %v", err)
	}
	if err := m.CheckRemove(ctx, "php7.1", RemoveOptions{}); err != nil {
		t.Fatalf("remove 7.1: %v", err)
	}
	// With a second supported version, either may go.
	installPHPForTest(t, "8.4")
	if err := m.CheckRemove(ctx, "php8.3", RemoveOptions{}); err != nil {
		t.Fatalf("remove 8.3 next to 8.4: %v", err)
	}
	if code(lastSupportedBlocker("7.3", 0)) != "" || code(lastSupportedBlocker("8.1", 1)) != "php_last_supported:8.1" || lastSupportedBlocker("8.1", 2) != nil {
		t.Error("lastSupportedBlocker")
	}
}

func TestInstallOldPHP(t *testing.T) {
	sandbox(t)
	m, f, _ := newTestManager(t)
	ctx := context.Background()
	osRelease(t, "ubuntu", "24.04", "noble", "amd64")
	newFakeSury(t)
	installPHPForTest(t, "8.3")
	touch(t, redisBin)
	policy := "php5.6-fpm:\n  Installed: (none)\n  Candidate: 5.6.40-110+debsuryorg24.04.1+1\nphp5.6-mcrypt:\n  Installed: (none)\n  Candidate: 5.6.40-110\n"
	f.answers["apt-cache policy php5.6-fpm"] = policy
	if err := m.Install(ctx, "php5.6", &nopLog{}); err != nil {
		t.Fatal(err)
	}
	want := "update|install --no-install-recommends php5.6-fpm php5.6-cli php5.6-mysql php5.6-curl php5.6-gd php5.6-mbstring " +
		"php5.6-xml php5.6-zip php5.6-intl php5.6-bcmath php5.6-opcache php5.6-json php5.6-mcrypt php5.6-redis"
	if got := strings.Join(f.apt, "|"); got != want {
		t.Fatalf("apt:\n%s\nwant\n%s", got, want)
	}
	// Where the repository has no mcrypt, PHP 5.6 comes without it.
	f.apt = nil
	f.answers["apt-cache policy php5.6-fpm"] = "php5.6-fpm:\n  Candidate: 5.6.40-110\nphp5.6-mcrypt:\n  Candidate: (none)\n"
	if err := m.Install(ctx, "php5.6", &nopLog{}); err != nil || strings.Contains(strings.Join(f.apt, " "), "mcrypt") {
		t.Fatalf("without mcrypt: %v %q", err, f.apt)
	}
	// A version the repository does not have is refused before apt installs anything.
	f.apt = nil
	f.answers["apt-cache policy php7.0-fpm"] = "php7.0-fpm:\n  Installed: (none)\n  Candidate: (none)\n"
	if err := m.Install(ctx, "php7.0", &nopLog{}); code(err) != "php_not_published:Ubuntu 24.04, packages.sury.org" {
		t.Fatalf("unpublished: %v", err)
	}
	if strings.Join(f.apt, "|") != "update" {
		t.Fatalf("apt ran %q", f.apt)
	}
}
