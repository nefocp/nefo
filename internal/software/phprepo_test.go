package software

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// suryKey is packages.sury.org/php/apt.gpg as published on 2026-11-26.
func suryKey(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/sury-apt.gpg")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// fakeSury serves the key and the releases' InRelease like
// packages.sury.org/php, on 127.0.0.1.
type fakeSury struct {
	key      []byte
	releases []string
	fetches  int
}

func newFakeSury(t *testing.T) *fakeSury {
	t.Helper()
	s := &fakeSury{key: suryKey(t), releases: slices.Clone(suryCodenames)}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.fetches++
		if r.URL.Path == "/php/apt.gpg" {
			w.Write(s.key)
			return
		}
		for _, c := range s.releases {
			if r.URL.Path == "/php/dists/"+c+"/InRelease" {
				w.Write([]byte("-----BEGIN PGP SIGNED MESSAGE-----\nCodename: " + c + "\n"))
				return
			}
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	suryBase = srv.URL + "/php/"
	return s
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// otherKey is a made-up v4 public key packet (RSA), with another
// fingerprint.
var otherKey = []byte{0x99, 0x00, 0x0c, 4, 0x5c, 0x8f, 0x00, 0x00, 1, 0x00, 0x08, 0xc5, 0x00, 0x02, 0x03}

func TestSuryKeyPinned(t *testing.T) {
	key := suryKey(t)
	fps, err := pgpFingerprints(key)
	if err != nil || !slices.Equal(fps, []string{suryFingerprint}) {
		t.Fatalf("fingerprints %v, %v", fps, err)
	}
	if err := checkSuryKey(key); err != nil {
		t.Fatal(err)
	}
	// Another key next to it would be trusted for the repository too.
	if err := checkSuryKey(append(slices.Clone(key), otherKey...)); err == nil || !strings.Contains(err.Error(), "unexpected key") {
		t.Fatalf("extra key: %v", err)
	}
	for name, b := range map[string][]byte{
		"other key": otherKey, "html": []byte("<html>not found</html>"), "empty": nil, "truncated": key[:100],
		"armored":    []byte("-----BEGIN PGP PUBLIC KEY BLOCK-----\n"),
		"secret key": {0x95, 0x00, 0x01, 4},
	} {
		if checkSuryKey(b) == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestNewInstallsUseSury(t *testing.T) {
	for _, c := range []struct{ id, ver, codename string }{
		{"ubuntu", "22.04", "jammy"}, {"ubuntu", "24.04", "noble"}, {"ubuntu", "26.04", "resolute"},
		{"debian", "12", "bookworm"}, {"debian", "13", "trixie"},
	} {
		sandbox(t)
		m, f, _ := newTestManager(t)
		osRelease(t, c.id, c.ver, c.codename, "amd64")
		newFakeSury(t)
		if err := m.addPHPRepository(context.Background(), &nopLog{}); err != nil {
			t.Fatalf("%s: %v", c.codename, err)
		}
		want := "deb [signed-by=/usr/share/keyrings/sury-php.gpg] https://packages.sury.org/php/ " + c.codename + " main\n"
		if src := readFile(t, sysPath(suryList)); !strings.HasSuffix(src, want) {
			t.Fatalf("%s: source %q", c.codename, src)
		}
		if readFile(t, sysPath(suryKeyring)) != string(suryKey(t)) {
			t.Fatalf("%s: keyring", c.codename)
		}
		// No PPA, no add-apt-repository, no other command.
		if len(f.cmds) != 0 || len(f.apt) != 0 {
			t.Fatalf("%s: ran %q %q", c.codename, f.cmds, f.apt)
		}
		if PHPRepository() != "packages.sury.org" {
			t.Fatalf("%s: repository %q", c.codename, PHPRepository())
		}
		if code, detail := legacyOffered(); code != "" {
			t.Fatalf("%s: old versions not offered: %s %s", c.codename, code, detail)
		}
	}
}

func TestSuryRefusals(t *testing.T) {
	sandbox(t)
	m, _, _ := newTestManager(t)
	ctx := context.Background()
	osRelease(t, "ubuntu", "24.04", "noble", "amd64")
	s := newFakeSury(t)

	// A key that is not the pinned one: nothing written.
	s.key = append(suryKey(t), otherKey...)
	if err := m.addPHPRepository(ctx, &nopLog{}); err == nil || exists(sysPath(suryList)) || exists(sysPath(suryKeyring)) {
		t.Fatalf("wrong key: %v", err)
	}
	// A release the repository does not serve: no source that would break apt-get update.
	s.key, s.releases = suryKey(t), []string{"jammy"}
	if err := m.addPHPRepository(ctx, &nopLog{}); err == nil || exists(sysPath(suryList)) {
		t.Fatalf("release not served: %v", err)
	}
	osRelease(t, "ubuntu", "25.10", "questing", "amd64")
	if err := m.addPHPRepository(ctx, &nopLog{}); err == nil || exists(sysPath(suryList)) {
		t.Fatalf("unsupported release: %v", err)
	}
	osRelease(t, "fedora", "41", "", "amd64")
	if err := m.addPHPRepository(ctx, &nopLog{}); code(err) != "unsupported:" {
		t.Fatalf("fedora: %v", err)
	}

	// Set up by sury's own instructions (its debsuryorg keyring): no second
	// source with another key, which would make apt refuse both.
	osRelease(t, "debian", "12", "bookworm", "amd64")
	os.Remove(sysPath(suryKeyring))
	s.releases = slices.Clone(suryCodenames)
	own := filepath.Join(sysPath("/etc/apt/sources.list.d"), "php.list")
	os.WriteFile(own, []byte("deb [signed-by=/usr/share/keyrings/debsuryorg-archive-keyring.gpg] https://packages.sury.org/php/ bookworm main\n"), 0o644)
	if err := m.addPHPRepository(ctx, &nopLog{}); err != nil || exists(sysPath(suryList)) || exists(sysPath(suryKeyring)) {
		t.Fatalf("existing sury source: %v", err)
	}
	os.Remove(own)

	// The panel's own key is refreshed when the repository is used again,
	// and kept when the download fails.
	if err := m.addPHPRepository(ctx, &nopLog{}); err != nil {
		t.Fatal(err)
	}
	suryBase = "http://127.0.0.1:1/"
	lg := &nopLog{}
	if err := m.addPHPRepository(ctx, lg); err != nil || readFile(t, sysPath(suryKeyring)) != string(suryKey(t)) {
		t.Fatalf("offline: %v", err)
	}
}

// ppaFiles writes what add-apt-repository leaves for ppa:ondrej/php: a
// deb822 file with the key inside (24.04), a one-line file (22.04), and the
// key in trusted.gpg.d (22.04).
func ppaFiles(t *testing.T) []string {
	t.Helper()
	d := sysPath("/etc/apt/sources.list.d")
	files := map[string]string{
		filepath.Join(d, "ondrej-ubuntu-php-noble.sources"): "Types: deb\nURIs: https://ppa.launchpadcontent.net/ondrej/php/ubuntu/\nSuites: noble\nComponents: main\nSigned-By: -----BEGIN PGP PUBLIC KEY BLOCK-----\n .\n mQINBGYo\n -----END PGP PUBLIC KEY BLOCK-----\n",
		filepath.Join(d, "ondrej-ubuntu-php-jammy.list"):    "deb https://ppa.launchpadcontent.net/ondrej/php/ubuntu/ jammy main\n# deb-src https://ppa.launchpadcontent.net/ondrej/php/ubuntu/ jammy main\n",
		sysPath(ppaKeyFile): "key",
	}
	var out []string
	for p, body := range files {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		out = append(out, p)
	}
	return out
}

func TestMigratePPA(t *testing.T) {
	dir := sandbox(t)
	m, f, _ := newTestManager(t)
	ctx := context.Background()
	backup := filepath.Join(dir, "var/lib/nefo/apt-backup")

	// Debian, and Ubuntu without the PPA: nothing to do.
	osRelease(t, "debian", "12", "bookworm", "amd64")
	if msg, err := m.MigratePHPRepository(ctx, backup); msg != "" || err != nil {
		t.Fatalf("debian: %q %v", msg, err)
	}
	osRelease(t, "ubuntu", "24.04", "noble", "amd64")
	if msg, err := m.MigratePHPRepository(ctx, backup); msg != "" || err != nil || exists(backup) {
		t.Fatalf("no PPA: %q %v", msg, err)
	}

	files := ppaFiles(t)
	keep := filepath.Join(sysPath("/etc/apt/sources.list.d"), "ondrej-ubuntu-nginx-noble.list")
	os.WriteFile(keep, []byte("deb https://ppa.launchpadcontent.net/ondrej/nginx/ubuntu/ noble main\n"), 0o644)
	if PHPRepository() != "ppa:ondrej/php" {
		t.Fatalf("before: %q", PHPRepository())
	}

	// The key can't be fetched: the PPA stays.
	if msg, err := m.MigratePHPRepository(ctx, backup); err == nil || msg != "" || !strings.Contains(err.Error(), "stays") {
		t.Fatalf("offline: %q %v", msg, err)
	}
	for _, p := range files {
		if !exists(p) {
			t.Fatalf("%s moved although packages.sury.org could not be added", p)
		}
	}
	if exists(sysPath(suryList)) {
		t.Fatal("half a source")
	}

	newFakeSury(t)
	msg, err := m.MigratePHPRepository(ctx, backup)
	if err != nil || !strings.Contains(msg, "packages.sury.org") {
		t.Fatalf("migrate: %q %v", msg, err)
	}
	if !strings.HasSuffix(readFile(t, sysPath(suryList)), "https://packages.sury.org/php/ noble main\n") || readFile(t, sysPath(suryKeyring)) != string(suryKey(t)) {
		t.Fatal("sury source or key missing")
	}
	for _, p := range files {
		if exists(p) {
			t.Fatalf("%s still there", p)
		}
		if m, _ := filepath.Glob(filepath.Join(backup, filepath.Base(p)+".*")); len(m) != 1 {
			t.Fatalf("%s not in the backup: %v", p, m)
		}
	}
	if !exists(keep) {
		t.Fatal("another PPA of the same owner was moved")
	}
	// Nothing installed, removed or updated.
	if len(f.cmds) != 0 || len(f.apt) != 0 {
		t.Fatalf("ran %q %q", f.cmds, f.apt)
	}
	if PHPRepository() != "packages.sury.org" {
		t.Fatalf("after: %q", PHPRepository())
	}
	// Once only.
	if msg, err := m.MigratePHPRepository(ctx, backup); msg != "" || err != nil {
		t.Fatalf("again: %q %v", msg, err)
	}
	// Until apt-get update fetched the new lists, an extension install runs it.
	if !suryListsMissing() {
		t.Fatal("lists not missing")
	}
	os.WriteFile(sysPath("/var/lib/apt/lists/packages.sury.org_php_dists_noble_InRelease"), nil, 0o644)
	if suryListsMissing() {
		t.Fatal("lists missing")
	}
}

func TestMigratePPAKeepsExistingSury(t *testing.T) {
	dir := sandbox(t)
	m, _, _ := newTestManager(t)
	ctx := context.Background()
	osRelease(t, "ubuntu", "22.04", "jammy", "arm64")
	// The administrator followed the PPA's notice half way: sury's package
	// added its source; the PPA is still there.
	own := filepath.Join(sysPath("/etc/apt/sources.list.d"), "php.sources")
	os.WriteFile(own, []byte("Types: deb\nURIs: https://packages.sury.org/php/\nSuites: jammy\nComponents: main\nSigned-By: /usr/share/keyrings/debsuryorg-archive-keyring.gpg\n"), 0o644)
	files := ppaFiles(t)
	msg, err := m.MigratePHPRepository(ctx, filepath.Join(dir, "backup"))
	if err != nil || msg == "" || strings.Contains(msg, "added") {
		t.Fatalf("%q %v", msg, err)
	}
	if exists(sysPath(suryList)) || !exists(own) || exists(files[0]) {
		t.Fatal("added a second sury source, or left the PPA")
	}

	// The PPA in sources.list itself, next to Ubuntu's own: not edited.
	os.WriteFile(sysPath("/etc/apt/sources.list"), []byte("deb http://archive.ubuntu.com/ubuntu jammy main\ndeb http://ppa.launchpad.net/ondrej/php/ubuntu jammy main\n"), 0o644)
	if msg, err := m.MigratePHPRepository(ctx, filepath.Join(dir, "backup")); msg != "" || err == nil || !strings.Contains(err.Error(), "by hand") {
		t.Fatalf("mixed: %q %v", msg, err)
	}
	if !strings.Contains(readFile(t, sysPath("/etc/apt/sources.list")), "ondrej/php") {
		t.Fatal("sources.list edited")
	}
}

func TestInstallExtensionFetchesNewLists(t *testing.T) {
	sandbox(t)
	m, f, _ := newTestManager(t)
	installPHPForTest(t, "8.3")
	os.WriteFile(sysPath(suryList), []byte("deb [signed-by=/usr/share/keyrings/sury-php.gpg] https://packages.sury.org/php/ noble main\n"), 0o644)
	if err := m.InstallExtension(context.Background(), "8.3", "imagick", &nopLog{}); err != nil {
		t.Fatal(err)
	}
	if strings.Join(f.apt, "|") != "update|install -y --no-install-recommends php8.3-imagick" {
		t.Fatalf("apt %q", f.apt)
	}
	f.apt = nil
	os.WriteFile(sysPath("/var/lib/apt/lists/packages.sury.org_php_dists_noble_main_binary-amd64_Packages"), nil, 0o644)
	if err := m.InstallExtension(context.Background(), "8.3", "imagick", &nopLog{}); err != nil || strings.Join(f.apt, "|") != "install -y --no-install-recommends php8.3-imagick" {
		t.Fatalf("apt %q %v", f.apt, err)
	}
}

func TestPHPEndOfLifeByDate(t *testing.T) {
	day := func(s string) time.Time { d, _ := time.Parse("2006-01-02 15:04", s); return d }
	for _, c := range []struct {
		v, distro, now string
		eol            bool
	}{
		{"8.2", "8.3", "2026-12-31 23:59", false},
		{"8.2", "8.3", "2027-01-01 00:00", true},  // the badge comes by itself in January
		{"8.2", "8.2", "2027-06-01 00:00", false}, // Debian 12's own PHP: its security team patches it
		{"8.1", "8.1", "2026-11-26 12:00", false}, // Ubuntu 22.04's own PHP
		{"8.1", "8.3", "2026-11-26 12:00", true},
		{"7.4", "8.3", "2026-11-26 12:00", true},
		{"8.0", "8.4", "2026-11-26 12:00", true},
		{"8.3", "8.4", "2026-11-26 12:00", false},
		{"8.5", "8.3", "2029-12-31 12:00", false},
		{"5.6", "8.3", "2026-11-26 12:00", true},
	} {
		if got := PHPEndOfLife(c.v, c.distro, day(c.now)); got != c.eol {
			t.Errorf("PHP %s (distribution %s) on %s: %v", c.v, c.distro, c.now, got)
		}
	}
	for _, v := range PHPVersions {
		if _, err := time.Parse("2006-01-02", PHPSecurityUntil(v)); err != nil {
			t.Errorf("PHP %s has no end date", v)
		}
	}
}

func TestCatalogEndOfLifeByDate(t *testing.T) {
	sandbox(t)
	m, _, _ := newTestManager(t) // the distribution's PHP is 8.3
	osRelease(t, "ubuntu", "24.04", "noble", "amd64")
	installPHPForTest(t, "8.3", "8.2")
	old := phpNow
	t.Cleanup(func() { phpNow = old })
	items := func() map[string]Item {
		list, err := m.Catalog(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]Item{}
		for _, it := range list {
			out[it.ID] = it
		}
		return out
	}
	phpNow = func() time.Time { return time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC) }
	it := items()
	if it["php8.2"].EOL || it["php8.2"].SecurityUntil != "2026-12-31" || it["php8.3"].SecurityUntil != "" {
		t.Fatalf("December: %+v %+v", it["php8.2"], it["php8.3"])
	}
	if !it["php8.1"].EOL || it["php8.1"].Legacy || !it["php7.3"].EOL || !it["php7.3"].Legacy {
		t.Fatalf("8.1 %+v, 7.3 %+v", it["php8.1"], it["php7.3"])
	}
	phpNow = func() time.Time { return time.Date(2027, 1, 2, 0, 0, 0, 0, time.UTC) }
	if it := items(); !it["php8.2"].EOL || it["php8.2"].Legacy || it["php8.3"].EOL {
		t.Fatalf("January: %+v %+v", it["php8.2"], it["php8.3"])
	}
	// Past its date but not old: no confirmation needed.
	if err := m.CheckInstall(context.Background(), "php8.1", false); err != nil {
		t.Fatal(err)
	}
}
