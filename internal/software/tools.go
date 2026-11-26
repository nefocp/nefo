package software

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/nefocp/nefo/internal/appstore"
	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/software/apt"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// Composer, on the Software page's Tools group. The distribution's
// "composer" package is the baseline: apt keeps it patched. Update installs
// the latest stable release from getcomposer.org into /usr/local/bin,
// checked against the SHA-256 checksum getcomposer.org publishes for it. That
// one comes first in PATH, so "composer" in a shell, Git deploy scripts and
// the panel's own installs of Laravel and Drupal all run the same Composer
// (see ComposerPath).

// Where Composer lives and where its releases come from; tests change them.
var (
	composerPhar = "/usr/local/bin/composer"
	composerApt  = "/usr/bin/composer"
	// composerSite publishes the releases: /versions lists them
	// (https://getcomposer.org/versions), and each release has
	// /download/<version>/composer.phar and composer.phar.sha256sum next
	// to it ("<sha256>  composer.phar").
	composerSite = "https://getcomposer.org"
)

const composerMaxSize = 32 << 20

// ComposerPath returns the Composer the panel runs: /usr/local/bin/composer
// when it exists, as "composer" in a shell finds it first in PATH, else the
// distribution's /usr/bin/composer, else "".
func ComposerPath() string {
	for _, p := range []string{composerPhar, composerApt} {
		if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() {
			return p
		}
	}
	return ""
}

// ComposerItem is Composer's card: installed or not, its version and where
// it comes from, and what uses it.
func (m *Manager) ComposerItem(ctx context.Context) Item {
	it := Item{ID: "composer", Kind: "composer", Group: "tools"}
	path := ComposerPath()
	if path == "" {
		return it
	}
	it.Installed, it.Updatable = true, true
	if path == composerPhar {
		it.Source, it.Version = "official", pharVersion(path)
	} else {
		it.Source, it.Version = "apt", m.aptVersion(ctx, "composer")
	}
	sites, err := m.store.ListSites(ctx)
	if err != nil {
		it.Block(err)
		return it
	}
	apps, deploys := m.composerUsers(ctx, sites)
	it.Usage = map[string]int{"apps": apps, "deploys": deploys}
	it.Block(composerBlocker(deploys))
	return it
}

// composerUsers counts the websites whose app was built with Composer
// (Laravel, Drupal: they only need it to install) and the Git deploy
// scripts that run composer (they need it at every deploy).
func (m *Manager) composerUsers(ctx context.Context, sites []*store.Site) (apps, deploys int) {
	for _, s := range sites {
		if a, ok := appstore.Lookup(s.App); ok && a.PHP != nil && a.PHP.Composer != nil {
			apps++
		}
		if g, err := m.store.GitDeploy(ctx, s.ID); err == nil && composerCallRe.MatchString(g.Script) {
			deploys++
		}
	}
	return apps, deploys
}

// composerCallRe finds a composer command in a deploy script, but not
// composer.json or a project's own composer.phar.
var composerCallRe = regexp.MustCompile(`(?m)(^|[\s;&|(/])composer(\s|$)`)

// composerBlocker: Git deploy scripts run composer at every deploy, so it
// stays while one does. Websites built with Composer (Laravel, Drupal) have
// their vendor folder already; installing such an app again brings
// Composer back from the distribution.
func composerBlocker(deploys int) error {
	if deploys > 0 {
		return usererr.New("composer_in_use", strconv.Itoa(deploys))
	}
	return nil
}

// aptVersion returns an installed package's upstream version, such as
// "2.7.1" for "2.7.1-2" or "1:2.2.6-2ubuntu1".
func (m *Manager) aptVersion(ctx context.Context, pkg string) string {
	out, err := m.run.Output(ctx, "dpkg-query", "-W", "-f=${Version}", pkg)
	if err != nil {
		return ""
	}
	v := strings.TrimSpace(out)
	if _, after, ok := strings.Cut(v, ":"); ok {
		v = after
	}
	return firstMatch(`^([0-9]+(?:\.[0-9]+)*)`, v)
}

// pharVersionRe finds Composer's version in its phar, which stores its
// files uncompressed: "public const VERSION = '2.11.3';" in Composer.php.
var pharVersionRe = regexp.MustCompile(`const VERSION = '([0-9]+\.[0-9]+\.[0-9]+)';\s*public const BRANCH_ALIAS_VERSION`)

var pharVersions sync.Map // path + size + mtime → version

// pharVersion returns the version of the Composer phar at path, or "".
func pharVersion(path string) string {
	fi, err := os.Stat(path)
	if err != nil || fi.Size() > composerMaxSize {
		return ""
	}
	key := fmt.Sprintf("%s %d %d", path, fi.Size(), fi.ModTime().UnixNano())
	if v, ok := pharVersions.Load(key); ok {
		return v.(string)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	v := ""
	if mm := pharVersionRe.FindSubmatch(b); mm != nil {
		v = string(mm[1])
	}
	pharVersions.Store(key, v)
	return v
}

// installComposer installs the distribution's package, with the PHP
// command line it needs.
func (m *Manager) installComposer(ctx context.Context, lg Logger) error {
	if err := run.Apt(ctx, m.run, lg, "update"); err != nil {
		return err
	}
	lg.Logf("Installing Composer from the distribution's packages")
	if err := run.Apt(ctx, m.run, lg, "install", "--no-install-recommends", "composer"); err != nil {
		return err
	}
	lg.Logf("Composer %s is installed in %s. Update installs the latest release from getcomposer.org.", m.aptVersion(ctx, "composer"), composerApt)
	return nil
}

// UpdateComposer installs the latest stable Composer from getcomposer.org
// into /usr/local/bin/composer, checked against its published SHA-256. A
// download that does not match is refused and nothing changes. The file is
// replaced in one step, so a Composer that is running keeps working.
func (m *Manager) UpdateComposer(ctx context.Context, lg Logger) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	lg.Logf("Looking up the latest stable Composer on getcomposer.org")
	b, err := download(ctx, composerSite+"/versions", 1<<20)
	if err != nil {
		return err
	}
	var list struct {
		Stable []struct {
			Path    string `json:"path"`
			Version string `json:"version"`
			MinPHP  int    `json:"min-php"`
		} `json:"stable"`
	}
	if err := json.Unmarshal(b, &list); err != nil || len(list.Stable) == 0 {
		return errors.New("getcomposer.org/versions has no stable release")
	}
	rel := list.Stable[0]
	if !regexp.MustCompile(`^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$`).MatchString(rel.Version) || rel.Path != "/download/"+rel.Version+"/composer.phar" {
		return fmt.Errorf("getcomposer.org/versions lists an unexpected release %q at %q", rel.Version, rel.Path)
	}
	if php := newestPHP(); php == "" || phpNumber(php)+99 < rel.MinPHP {
		return usererr.New("composer_needs_php", fmt.Sprintf("%d.%d.%d", rel.MinPHP/10000, rel.MinPHP/100%100, rel.MinPHP%100))
	}
	sums, err := download(ctx, composerSite+rel.Path+".sha256sum", 4096)
	if err != nil {
		return err
	}
	want := ""
	if f := strings.Fields(string(sums)); len(f) >= 1 && regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(f[0]) && (len(f) == 1 || f[1] == "composer.phar") {
		want = f[0]
	}
	if want == "" {
		return errors.New("composer.phar.sha256sum on getcomposer.org is not a SHA-256 checksum")
	}
	if cur, err := fileSHA256(composerPhar); err == nil && cur == want {
		lg.Logf("Composer %s, the latest stable release, is already installed in %s.", rel.Version, composerPhar)
		return nil
	}
	lg.Logf("Downloading Composer %s", rel.Version)
	if err := installVerified(ctx, composerSite+rel.Path, composerMaxSize, want, composerPhar, 0o755); err != nil {
		return err
	}
	if got := pharVersion(composerPhar); got != rel.Version {
		lg.Logf("Warning: the installed file reports Composer %q", got)
	}
	lg.Logf("Composer %s is installed in %s. \"composer\" and the panel use it from now on; the distribution's package stays for its security updates.", rel.Version, composerPhar)
	return nil
}

// newestPHP returns the newest installed PHP version, or "".
func newestPHP() string {
	for i := len(PHPVersions) - 1; i >= 0; i-- {
		if phpInstalled(PHPVersions[i]) {
			return PHPVersions[i]
		}
	}
	return ""
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// installVerified downloads url into a temporary file next to dest while
// hashing it, and renames it over dest only when its SHA-256 is want: dest
// is either the old file or the new one, never a partial or unverified one.
func installVerified(ctx context.Context, url string, max int64, want, dest string, mode os.FileMode) error {
	dir := filepath.Dir(dest)
	if err := fsutil.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".panel-download-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after the rename
	h := sha256.New()
	err = downloadTo(ctx, url, max, io.MultiWriter(tmp, h))
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if hex.EncodeToString(h.Sum(nil)) != want {
		return errors.New("the download does not match its published SHA-256 checksum: nothing was changed")
	}
	if err := os.Chmod(tmp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dest)
}

// removeComposer removes the phar in /usr/local/bin and the distribution's
// package.
func (m *Manager) removeComposer(ctx context.Context, lg Logger) error {
	pkgs, err := apt.Installed(ctx, m.run, false, "composer")
	if err != nil {
		return err
	}
	if err := apt.Check(ctx, m.run, false, pkgs...); err != nil {
		return err
	}
	if fi, err := os.Stat(composerPhar); err == nil && fi.Mode().IsRegular() {
		lg.Logf("Removing %s", composerPhar)
		if err := os.Remove(composerPhar); err != nil {
			return err
		}
	}
	if len(pkgs) > 0 {
		lg.Logf("Removing the distribution's Composer package")
		if err := apt.Remove(ctx, m.run, lg, false, pkgs...); err != nil {
			return err
		}
	}
	lg.Logf("Composer is removed. Installing Laravel or Drupal from the App store installs it again.")
	return nil
}
