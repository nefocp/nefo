package wptoolkit

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/nefocp/nefo/internal/appstore"
	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/usererr"
)

// wp-cli itself: installed on first use, and on the Software page's Tools
// group, from its latest release on GitHub. The release publishes
// wp-cli-<version>.phar.sha512 next to the phar; both come over HTTPS, and a
// phar that does not match is refused, leaving the installed one in place.
// Nothing is pinned, so the panel never keeps installing an old release.
// Next to the phar the panel records the release and the phar's SHA-512
// (<phar>.json), which is how it knows the file is the one it installed.
var (
	// https://docs.github.com/en/rest/releases/releases#get-the-latest-release
	wpcliLatestURL = "https://api.github.com/repos/wp-cli/wp-cli/releases/latest"
	wpcliAssetURL  = "https://github.com/wp-cli/wp-cli/releases/download/"
)

// pinnedSHA512 is wp-cli 2.12.0's published SHA-512: the release earlier
// panels pinned, which they installed without a record.
const (
	pinnedVersion = "2.12.0"
	pinnedSHA512  = "be928f6b8ca1e8dfb9d2f4b75a13aa4aee0896f8a9a0a1c45cd5d2c98605e6172e6d014dda2e27f88c98befc16c040cbb2bd1bfa121510ea5cdf5f6a30fe8832"
)

var (
	releaseTagRe = regexp.MustCompile(`^v([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3})$`)
	sha512Re     = regexp.MustCompile(`^[0-9a-f]{128}$`)
)

// cliRecord is what the panel writes next to the wp-cli it installed.
type cliRecord struct {
	Version string `json:"version"`
	SHA512  string `json:"sha512"`
}

func (t *Toolkit) recordFile() string { return t.phar + ".json" }

// pharSum returns the installed phar's SHA-512, computed again only when
// the file's size or time changed.
func (t *Toolkit) pharSum() (string, error) {
	fi, err := os.Stat(t.phar)
	if err != nil {
		return "", err
	}
	if !fi.Mode().IsRegular() || fi.Size() > wpcliMaxSize {
		return "", errors.New("wp-cli is not a regular file of the expected size")
	}
	key := fmt.Sprintf("%d %d", fi.Size(), fi.ModTime().UnixNano())
	t.sumMu.Lock()
	defer t.sumMu.Unlock()
	if t.sumKey == key {
		return t.sum, nil
	}
	sum, err := fileSHA512(t.phar)
	if err != nil {
		return "", err
	}
	t.sumKey, t.sum = key, sum
	return sum, nil
}

// CLIVersion returns the installed wp-cli's version, or "" when wp-cli is
// missing or is not a file the panel installed.
func (t *Toolkit) CLIVersion() string {
	sum, err := t.pharSum()
	if err != nil {
		return ""
	}
	if sum == pinnedSHA512 {
		return pinnedVersion
	}
	b, err := os.ReadFile(t.recordFile())
	if err != nil {
		return ""
	}
	var r cliRecord
	if json.Unmarshal(b, &r) != nil || r.SHA512 != sum || releaseTagRe.FindString("v"+r.Version) == "" {
		return ""
	}
	return r.Version
}

// InstallCLI installs the latest wp-cli; UpdateCLI is the same.
func (t *Toolkit) InstallCLI(ctx context.Context, lg Logger) error { return t.UpdateCLI(ctx, lg) }

// UpdateCLI installs the latest wp-cli release unless it is installed.
func (t *Toolkit) UpdateCLI(ctx context.Context, lg Logger) error {
	t.installMu.Lock()
	defer t.installMu.Unlock()
	if err := t.installLatest(ctx, lg); err != nil {
		return err
	}
	t.installed = true
	return nil
}

// latestRelease returns the version of wp-cli's latest release, which must
// carry the phar and its .sha512.
func (t *Toolkit) latestRelease(ctx context.Context) (string, error) {
	b, err := fetch(ctx, t.latestURL, 1<<20)
	if err != nil {
		return "", err
	}
	var rel struct {
		Tag        string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		Assets     []struct {
			Name string `json:"name"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(b, &rel); err != nil {
		return "", fmt.Errorf("the latest wp-cli release: %w", err)
	}
	m := releaseTagRe.FindStringSubmatch(rel.Tag)
	if m == nil || rel.Draft || rel.Prerelease {
		return "", fmt.Errorf("the latest wp-cli release has an unexpected tag %q", rel.Tag)
	}
	found := 0
	for _, a := range rel.Assets {
		if a.Name == "wp-cli-"+m[1]+".phar" || a.Name == "wp-cli-"+m[1]+".phar.sha512" {
			found++
		}
	}
	if found != 2 {
		return "", fmt.Errorf("wp-cli %s has no phar with a SHA-512 checksum", m[1])
	}
	return m[1], nil
}

// installLatest downloads the latest release, checks it against its
// published SHA-512 and puts it in place in one step (the caller holds
// installMu).
func (t *Toolkit) installLatest(ctx context.Context, lg Logger) error {
	lg.Logf("Looking up the latest wp-cli release on GitHub")
	ver, err := t.latestRelease(ctx)
	if err != nil {
		return err
	}
	if cur := t.CLIVersion(); cur == ver {
		lg.Logf("wp-cli %s, the latest release, is already installed.", ver)
		return nil
	}
	base := t.assetURL + "v" + ver + "/wp-cli-" + ver + ".phar"
	sums, err := fetch(ctx, base+".sha512", 1024)
	if err != nil {
		return err
	}
	want := ""
	if f := strings.Fields(string(sums)); len(f) >= 1 && sha512Re.MatchString(f[0]) {
		want = f[0]
	}
	if want == "" {
		return errors.New("wp-cli's published .sha512 is not a SHA-512 checksum")
	}
	lg.Logf("Downloading wp-cli %s", ver)
	data, err := fetch(ctx, base, wpcliMaxSize)
	if err != nil {
		return err
	}
	h := sha512.Sum512(data)
	if hex.EncodeToString(h[:]) != want {
		return errors.New("the wp-cli download does not match its published SHA-512 checksum: nothing was changed")
	}
	if !bytes.HasPrefix(data, []byte("#!/usr/bin/env php")) {
		return errors.New("the wp-cli download is not a PHP archive: nothing was changed")
	}
	dir := filepath.Dir(t.phar)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// The panel's umask is 077; site users must be able to read wp-cli.
	if err := os.Chmod(dir, 0o755); err != nil {
		return err
	}
	rec, err := json.Marshal(cliRecord{Version: ver, SHA512: want})
	if err != nil {
		return err
	}
	// The phar first: if the record were left behind, the next use would
	// only download the release again.
	if err := fsutil.WriteFileAtomic(t.phar, data, 0o644); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(t.recordFile(), rec, 0o644); err != nil {
		return err
	}
	lg.Logf("wp-cli %s is installed in %s.", ver, t.phar)
	return nil
}

// RemoveCLI deletes wp-cli. Using the WordPress toolkit installs it again.
func (t *Toolkit) RemoveCLI(lg Logger) error {
	t.installMu.Lock()
	defer t.installMu.Unlock()
	for _, p := range []string{t.phar, t.recordFile()} {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	t.installed = false
	lg.Logf("wp-cli is removed.")
	return nil
}

// CLIBlocker returns why wp-cli can't be removed (a usererr), or nil:
// the toolkit runs it for every WordPress website.
func CLIBlocker(wordpressSites int) error {
	if wordpressSites > 0 {
		return usererr.New("wpcli_in_use", strconv.Itoa(wordpressSites))
	}
	return nil
}

// coreMinPHP is the PHP version current WordPress releases need: the app
// store's minimum for WordPress, which follows them (7.4 for WordPress 7.0
// and 7.1, 7.2.24 for 6.6 to 6.9; wordpress.org's update API says so per
// release). wp-cli's "core update" does not check it, and WordPress would
// stop working on an older PHP, so Update leaves the core alone there.
func coreMinPHP() string {
	if a, ok := appstore.Lookup("wordpress"); ok && a.PHP != nil && a.PHP.MinPHP != "" {
		return a.PHP.MinPHP
	}
	return "7.4"
}

type discardLog struct{}

func (discardLog) Write(p []byte) (int, error)     { return len(p), nil }
func (discardLog) Logf(format string, args ...any) {}
