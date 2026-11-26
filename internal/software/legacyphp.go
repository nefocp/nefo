package software

import (
	"context"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/usererr"
)

// Older PHP versions (5.6 to 7.3) are offered for websites moved from
// another panel (cPanel, aaPanel…) that still need one. PHP stopped their
// security updates years ago (7.3 in December 2021, 5.6 in December 2018:
// https://www.php.net/eol.php), so the panel marks them, installs one only
// after the administrator accepted that risk, and never lets one be the
// server's only PHP: the web server's setup, Adminer, Composer and the
// panel's own tools always have a supported version.

// LegacyPHPVersions are the old versions; PHPVersions lists them too.
var LegacyPHPVersions = []string{"5.6", "7.0", "7.1", "7.2", "7.3"}

// IsLegacyPHP reports whether v is one of LegacyPHPVersions.
func IsLegacyPHP(v string) bool { return slices.Contains(LegacyPHPVersions, v) }

// legacyPHPReleases are the releases each PHP repository publishes PHP 5.6
// to 7.3 for, on amd64 and arm64, as their package lists showed on
// 2026-11-26:
//   - packages.sury.org: https://packages.sury.org/php/dists/<codename>/main/binary-<arch>/Packages
//     has php5.6 to php8.6 (and every extension the panel installs) for
//     jammy, noble, resolute (Ubuntu 22.04, 24.04, 26.04), bookworm and
//     trixie (Debian 12, 13).
//   - ppa:ondrej/php, on servers the start-up move has not switched yet:
//     https://launchpad.net/~ondrej/+archive/ubuntu/php has them for jammy
//     and noble only; it is frozen and has nothing for 26.04.
//
// The install asks apt as well, so a release that stops publishing one is
// refused with a reason, never half installed.
var legacyPHPReleases = map[string][]string{
	"packages.sury.org": {"jammy", "noble", "resolute", "bookworm", "trixie"},
	"ppa:ondrej/php":    {"jammy", "noble"},
}

// phpSecurityEnd is the last day of upstream security support of each
// version (https://www.php.net/supported-versions.php and
// https://www.php.net/eol.php, 2026-11-26).
var phpSecurityEnd = map[string]string{
	"5.6": "2018-12-31", "7.0": "2019-01-10", "7.1": "2019-12-01", "7.2": "2020-11-30", "7.3": "2021-12-06",
	"7.4": "2022-11-28", "8.0": "2023-11-26", "8.1": "2025-12-31", "8.2": "2026-12-31", "8.3": "2027-12-31",
	"8.4": "2028-12-31", "8.5": "2029-12-31",
}

// phpNow is the clock for end-of-life marks; tests set it.
var phpNow = time.Now

// PHPSecurityUntil returns the last day of version v's upstream security
// support, such as "2026-12-31", or "".
func PHPSecurityUntil(v string) string { return phpSecurityEnd[v] }

// PHPEndOfLife reports whether PHP v gets no security updates on this
// server: its upstream security support ended, and it is not the
// distribution's own version (distro), which the distribution's security
// team keeps patching for the release's lifetime. Old repositories'
// versions past that date get nothing.
func PHPEndOfLife(v, distro string, now time.Time) bool {
	if v == distro {
		return false
	}
	end, err := time.Parse("2006-01-02", phpSecurityEnd[v])
	if err != nil {
		return IsLegacyPHP(v)
	}
	return !now.Before(end.AddDate(0, 0, 1)) // the whole last day still counts
}

// osReleaseFile is read for the distribution; tests point it elsewhere.
var osReleaseFile = "/etc/os-release"

// legacyArch is runtime.GOARCH; tests change it.
var legacyArch = runtime.GOARCH

// legacyOffered returns why the old PHP versions can't be installed on this
// server (a usererr code and its detail), or "" when they can.
func legacyOffered() (code, detail string) {
	b, _ := os.ReadFile(osReleaseFile)
	id, codename := osField(b, "ID"), osField(b, "VERSION_CODENAME")
	name := osName(b)
	repo := ""
	if id == "ubuntu" || id == "debian" {
		repo = PHPRepository()
	}
	if repo == "" {
		return "unsupported", ""
	}
	if legacyArch != "amd64" && legacyArch != "arm64" {
		return "php_not_published", name + " (" + legacyArch + "), " + repo
	}
	if !slices.Contains(legacyPHPReleases[repo], codename) {
		return "php_not_published", name + ", " + repo
	}
	return "", ""
}

// osName is the system's name for messages, such as "Ubuntu 24.04".
func osName(osRelease []byte) string {
	return strings.TrimSpace(osField(osRelease, "NAME") + " " + osField(osRelease, "VERSION_ID"))
}

// phpAtLeast compares PHP versions such as "7.4" and "8.0".
func phpAtLeast(have, want string) bool {
	h, w := phpNumber(have), phpNumber(want)
	return h >= w
}

// phpNumber turns "7.4" into 70400, the form of PHP_VERSION_ID without the
// release number.
func phpNumber(v string) int {
	major, minor, _ := strings.Cut(v, ".")
	a, _ := strconv.Atoi(major)
	b, _ := strconv.Atoi(minor)
	return a*10000 + b*100
}

// phpExtensionsFor returns the extensions installed with PHP version v:
// PHPExtensions, and json before PHP 8.0, where it is a package of its own
// (PHP 8 has it built in; php8.x-json is only a virtual package).
func phpExtensionsFor(v string) []string {
	exts := slices.Clone(PHPExtensions)
	if !phpAtLeast(v, "8.0") {
		exts = append(exts, "json")
	}
	return exts
}

// phpExtrasFor are extensions installed with a version when the repository
// has a package for it: mcrypt for PHP 5.6, whose old applications (Magento
// 1, early Laravel) often need it. PHP 7.2 dropped it from PHP itself.
func phpExtrasFor(v string) []string {
	if v == "5.6" {
		return []string{"mcrypt"}
	}
	return nil
}

// supportedPHPInstalled counts the installed PHP versions that still get
// security updates (not LegacyPHPVersions).
func supportedPHPInstalled() int {
	n := 0
	for _, v := range PHPVersions {
		if !IsLegacyPHP(v) && phpInstalled(v) {
			n++
		}
	}
	return n
}

// lastSupportedBlocker keeps the last supported version while old versions
// are installed next to it: an old version must not become the server's
// only PHP.
func lastSupportedBlocker(v string, supported int) error {
	if !IsLegacyPHP(v) && supported <= 1 {
		return usererr.New("php_last_supported", v)
	}
	return nil
}

// legacyInstallBlocker returns why an old PHP version can't be installed
// now, or nil (the administrator's consent is checked by CheckInstall).
func legacyInstallBlocker(v string) error {
	if code, detail := legacyOffered(); code != "" {
		return usererr.New(code, detail)
	}
	if supportedPHPInstalled() == 0 {
		return usererr.New("php_needs_supported", v)
	}
	return nil
}

// CheckInstall returns why an item of the Software page can't be installed
// now (a usererr), or nil. An old PHP version needs acceptEOL: the
// administrator confirmed that it gets no security updates. It must also be
// published for this system, and a supported version must be installed
// first, so that an old one never is the server's default PHP.
func (m *Manager) CheckInstall(ctx context.Context, id string, acceptEOL bool) error {
	v, ok := strings.CutPrefix(id, "php")
	if !ok || !IsLegacyPHP(v) {
		return nil
	}
	if err := legacyInstallBlocker(v); err != nil {
		return err
	}
	if !acceptEOL {
		return usererr.New("php_eol_confirm", v)
	}
	return nil
}
