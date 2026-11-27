package software

import (
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeList(t *testing.T, name string, pkgs ...string) string {
	t.Helper()
	var b strings.Builder
	for _, p := range pkgs {
		b.WriteString("Package: " + p + "\nVersion: 1.0\nArchitecture: amd64\n\n")
	}
	p := sysPath("/var/lib/apt/lists/" + name)
	data := []byte(b.String())
	if strings.HasSuffix(name, ".gz") {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		zw.Write(data)
		zw.Close()
		data = buf.Bytes()
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestOfferedPHPVersions(t *testing.T) {
	sandbox(t)
	oldArch := legacyArch
	legacyArch = "amd64"
	defer func() { legacyArch = oldArch }()
	ctx := context.Background()
	m, _, _ := newTestManager(t) // the distribution's PHP is 8.3

	// No PHP repository yet: the versions the panel knows.
	if got := strings.Join(m.OfferedPHPVersions(ctx), " "); got != strings.Join(PHPVersions, " ") {
		t.Fatalf("without lists: %s", got)
	}

	// sury's list: 8.7 appears by itself; 8.0, which it does not have,
	// goes, unless it is installed; the old versions stay (legacyphp.go
	// says whether they can be installed).
	writeList(t, "packages.sury.org_php_dists_noble_main_binary-amd64_Packages",
		"php8.1-fpm", "php8.2-fpm", "php8.4-fpm", "php8.4-cli", "php8.5-fpm", "php8.6-fpm", "php8.7-fpm", "php8.7-opcache", "php-common", "php7.4-fpm")
	writeList(t, "packages.sury.org_php_dists_noble_main_binary-arm64_Packages", "php9.1-fpm") // another architecture
	if got := strings.Join(m.OfferedPHPVersions(ctx), " "); got != "5.6 7.0 7.1 7.2 7.3 7.4 8.1 8.2 8.3 8.4 8.5 8.6 8.7" {
		t.Fatalf("with sury: %s", got)
	}
	touch(t, filepath.Join(phpEtc, "8.0", "fpm", "php-fpm.conf"))
	if got := strings.Join(m.OfferedPHPVersions(ctx), " "); !strings.Contains(got, " 8.0 ") {
		t.Fatalf("installed 8.0 is not listed: %s", got)
	}

	// A gzip list (Acquire::GzipIndexes) of the old PPA: read the same way.
	os.Remove(sysPath("/var/lib/apt/lists/packages.sury.org_php_dists_noble_main_binary-amd64_Packages"))
	writeList(t, "ppa.launchpadcontent.net_ondrej_php_ubuntu_dists_noble_main_binary-amd64_Packages.gz", "php8.4-fpm", "php8.5-fpm")
	if got := strings.Join(m.OfferedPHPVersions(ctx), " "); got != "5.6 7.0 7.1 7.2 7.3 8.0 8.3 8.4 8.5" {
		t.Fatalf("with the PPA: %s", got)
	}

	// The Software page lists them; 8.7 can be installed.
	items, err := m.Catalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var php []string
	for _, it := range items {
		if it.Kind == "php" {
			php = append(php, it.Version)
		}
	}
	if strings.Join(php, " ") != "5.6 7.0 7.1 7.2 7.3 8.0 8.3 8.4 8.5" {
		t.Fatalf("catalog: %v", php)
	}
}

func TestPHPVersionKnownOrNewer(t *testing.T) {
	for v, want := range map[string]bool{"8.3": true, "5.6": true, "8.6": true, "8.7": true, "9.0": true, "5.2": false, "8.3;rm": false, "": false} {
		if got := PHPVersionKnownOrNewer(v); got != want {
			t.Errorf("%q: %v", v, got)
		}
	}
}
