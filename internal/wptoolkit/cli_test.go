package wptoolkit

import (
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/usererr"
)

// fakeGitHub answers like GitHub: the latest release, the phar and its
// .sha512, on 127.0.0.1.
type fakeGitHub struct {
	version   string
	phar      []byte
	sum       string // the published .sha512
	tag       string // the release's tag; "" means "v"+version
	downloads int    // phar downloads
}

func newFakeGitHub(t *testing.T, k *Toolkit, version string, phar []byte, sum string) *fakeGitHub {
	t.Helper()
	gh := &fakeGitHub{version: version, phar: phar, sum: sum}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		base := "/download/v" + gh.version + "/wp-cli-" + gh.version + ".phar"
		switch r.URL.Path {
		case "/repos/wp-cli/wp-cli/releases/latest":
			tag := gh.tag
			if tag == "" {
				tag = "v" + gh.version
			}
			json.NewEncoder(w).Encode(map[string]any{"tag_name": tag, "draft": false, "prerelease": false,
				"assets": []map[string]string{{"name": "wp-cli-" + gh.version + ".phar"}, {"name": "wp-cli-" + gh.version + ".phar.sha512"},
					{"name": "wp-cli-" + gh.version + ".phar.md5"}}})
		case base:
			gh.downloads++
			w.Write(gh.phar)
		case base + ".sha512":
			w.Write([]byte(gh.sum + "\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	k.latestURL, k.assetURL = srv.URL+"/repos/wp-cli/wp-cli/releases/latest", srv.URL+"/download/"
	return gh
}

func sha512Hex(b []byte) string {
	s := sha512.Sum512(b)
	return hex.EncodeToString(s[:])
}

func TestUpdateCLI(t *testing.T) {
	k, base := newKit(t, &fakeRunner{})
	k.phar = filepath.Join(base, "lib", "panel", "wp-cli.phar")
	old := []byte("#!/usr/bin/env php\nwp-cli 2.12.0")
	writeFile(t, k.phar, string(old))
	if k.CLIVersion() != "" {
		t.Fatal("a phar without a record has a version")
	}
	next := []byte("#!/usr/bin/env php\nwp-cli 2.13.0")
	gh := newFakeGitHub(t, k, "2.13.0", next, sha512Hex(next))
	ctx := context.Background()

	// A checksum that does not match: refused, the installed file stays.
	gh.sum = sha512Hex([]byte("something else"))
	lg := &testLog{}
	if err := k.UpdateCLI(ctx, lg); err == nil || !strings.Contains(err.Error(), "does not match its published SHA-512") {
		t.Fatalf("mismatch: %v", err)
	}
	if readFile(t, k.phar) != string(old) || fileExists(k.recordFile()) {
		t.Fatal("a download that does not match was installed")
	}
	// A checksum file that is not one, or a file that is no phar: refused.
	gh.sum = "<html>"
	if err := k.UpdateCLI(ctx, lg); err == nil || !strings.Contains(err.Error(), "not a SHA-512") {
		t.Fatalf("bad checksum file: %v", err)
	}
	gh.phar, gh.sum = []byte("<html>"), sha512Hex([]byte("<html>"))
	if err := k.UpdateCLI(ctx, lg); err == nil || !strings.Contains(err.Error(), "not a PHP archive") {
		t.Fatalf("not a phar: %v", err)
	}
	// A release tag that is not a version is refused before any download.
	gh.phar, gh.sum, gh.tag, gh.downloads = next, sha512Hex(next), "nightly", 0
	if err := k.UpdateCLI(ctx, lg); err == nil || gh.downloads != 0 {
		t.Fatalf("tag: %v, %d downloads", err, gh.downloads)
	}

	// The right one replaces the file in one step (no temporary file left)
	// and records the release.
	gh.tag = ""
	if err := k.UpdateCLI(ctx, lg); err != nil {
		t.Fatal(err)
	}
	if readFile(t, k.phar) != string(next) || k.CLIVersion() != "2.13.0" {
		t.Fatalf("installed %q, version %q", readFile(t, k.phar), k.CLIVersion())
	}
	if fi, err := os.Stat(k.phar); err != nil || fi.Mode().Perm() != 0o644 && os.PathSeparator == '/' {
		t.Fatalf("mode: %v %v", fi.Mode(), err)
	}
	entries, _ := os.ReadDir(filepath.Dir(k.phar))
	if len(entries) != 2 {
		t.Fatalf("left in the folder: %v", entries)
	}
	// Already the latest: nothing downloaded.
	gh.downloads = 0
	lg.Reset()
	if err := k.UpdateCLI(ctx, lg); err != nil || gh.downloads != 0 || !strings.Contains(lg.String(), "already installed") {
		t.Fatalf("again: %v, %d downloads, %q", err, gh.downloads, lg.String())
	}
	// A record that no longer matches the file means an unknown file.
	writeFile(t, k.phar, "#!/usr/bin/env php\nchanged by hand")
	if k.CLIVersion() != "" {
		t.Fatal("a changed phar kept its version")
	}

	if err := k.RemoveCLI(lg); err != nil || k.Installed() || fileExists(k.recordFile()) {
		t.Fatalf("remove: %v", err)
	}
	if err := k.RemoveCLI(lg); err != nil {
		t.Fatalf("remove again: %v", err)
	}
}

func TestPinnedCLIRecognised(t *testing.T) {
	k, _ := newKit(t, &fakeRunner{})
	os.Remove(k.recordFile())
	// The phar of earlier panels is not downloaded again: its checksum is
	// wp-cli 2.12.0's.
	if k.CLIVersion() != "" {
		t.Fatal("version without a record")
	}
	k.sumKey, k.sum = "", ""
	if sum, _ := k.pharSum(); sum == "" {
		t.Fatal("no checksum")
	}
	k.sumMu.Lock()
	k.sum = pinnedSHA512 // the cached checksum stands for the file's
	k.sumMu.Unlock()
	if v := k.CLIVersion(); v != pinnedVersion {
		t.Fatalf("pinned version %q", v)
	}
	if err := k.Ensure(context.Background()); err != nil {
		t.Fatalf("Ensure downloaded: %v", err)
	}
}

func TestCLIBlocker(t *testing.T) {
	var ue *usererr.Error
	if err := CLIBlocker(2); !errors.As(err, &ue) || ue.Code != "wpcli_in_use" || ue.Detail != "2" {
		t.Fatalf("%v", err)
	}
	if CLIBlocker(0) != nil {
		t.Fatal("blocked without WordPress websites")
	}
}

// Old PHP versions: the toolkit leaves WordPress itself alone there, since
// the newest WordPress would not run, and still updates plugins and themes.
func TestUpdateOldPHPSkipsCore(t *testing.T) {
	r := &fakeRunner{}
	k, _ := newKit(t, r)
	site := newSite(t, k, "old.example.com", "old_example_com")
	site.PHPVersion = "7.0"
	r.answer = func(c call) (string, error) {
		if w := c.wp(); len(w) == 3 && w[0] == "db" && w[1] == "export" {
			return "", os.WriteFile(w[2], []byte("-- dump"), 0o644)
		}
		return "", nil
	}
	lg := &testLog{}
	err := k.Update(context.Background(), site, lg)
	if err == nil || !strings.Contains(err.Error(), "WordPress itself was not updated: it needs PHP "+coreMinPHP()) {
		t.Fatalf("err = %v", err)
	}
	for _, l := range r.wpLines() {
		if strings.HasPrefix(l, "core update") {
			t.Fatalf("ran %q on PHP 7.0", l)
		}
	}
	if lines := strings.Join(r.wpLines(), "|"); !strings.Contains(lines, "plugin update --all|theme update --all") {
		t.Fatalf("commands %q", lines)
	}
	if min := coreMinPHP(); compareVersions(min, "7.2") < 0 {
		t.Fatalf("WordPress's minimum PHP %q is lower than WordPress has needed since 6.6 (7.2.24)", min)
	}
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }
