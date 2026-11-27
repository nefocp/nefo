package software

import (
	"bufio"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Which PHP versions the Software page offers. PHPVersions are the ones
// the panel knows; the PHP repository (packages.sury.org, or the old
// ppa:ondrej/php) adds a new version for every release a few days after
// PHP publishes it, and then the panel offers it too, read from apt's lists
// of that repository: PHP 8.7 shows up by itself once the repository has it
// and apt's lists are refreshed (the panel's installs run apt-get update, and
// so does the system's daily apt job). When the repository is set up and
// its lists say it does not have a version the panel knows (the frozen PPA
// has no 8.6), that version is not offered. Before the repository is set up
// (the first PHP version from it adds it) there are no lists to read, and
// the known versions are offered. Installed versions and the
// distribution's are always listed; so are the old versions (5.6 to 7.3,
// under "Older versions"), whose availability legacyphp.go checks.

// PHPVersions are the versions the panel knows, oldest first. The first
// ones are LegacyPHPVersions, for old websites (see legacyphp.go).
var PHPVersions = []string{"5.6", "7.0", "7.1", "7.2", "7.3", "7.4", "8.0", "8.1", "8.2", "8.3", "8.4", "8.5", "8.6"}

// PHPVersionKnownOrNewer reports whether v is one of PHPVersions or a
// newer version (such as "8.7" or "9.0"), which the PHP repository may
// have by now.
func PHPVersionKnownOrNewer(v string) bool {
	return slices.Contains(PHPVersions, v) || (phpDirRe.MatchString(v) && phpNumber(v) > phpNumber(PHPVersions[len(PHPVersions)-1]))
}

// OSRelease returns ID and VERSION_CODENAME of /etc/os-release, such as
// "ubuntu" and "noble".
func OSRelease() (id, codename string) {
	b, _ := os.ReadFile(osReleaseFile)
	return osField(b, "ID"), osField(b, "VERSION_CODENAME")
}

// AptListsEmpty reports whether apt's package lists were never downloaded
// (or were deleted): the next "apt-get update" downloads all of them.
func AptListsEmpty() bool {
	m, _ := filepath.Glob(sysPath("/var/lib/apt/lists/*_Packages*"))
	return len(m) == 0
}

// PHPVersionListed reports whether v is a PHP version the panel knows,
// one the PHP repository's lists have, or an installed one: an ID such as
// "php8.7" names a Software page item.
func PHPVersionListed(v string) bool {
	if slices.Contains(PHPVersions, v) || (phpDirRe.MatchString(v) && phpInstalled(v)) {
		return true
	}
	repo, _ := repoPHPVersions()
	return repo[v]
}

// phpListGlobs find apt's lists of the PHP repositories (sury's own, and
// the PPA's on launchpadcontent.net or the older launchpad.net), for this
// architecture.
func phpListGlobs() []string {
	arch := debArch()
	return []string{
		sysPath("/var/lib/apt/lists/packages.sury.org_php_dists_*_main_binary-" + arch + "_Packages*"),
		sysPath("/var/lib/apt/lists/ppa.launchpadcontent.net_ondrej_php_ubuntu_dists_*_main_binary-" + arch + "_Packages*"),
		sysPath("/var/lib/apt/lists/ppa.launchpad.net_ondrej_php_ubuntu_dists_*_main_binary-" + arch + "_Packages*"),
	}
}

var phpFPMPkgRe = regexp.MustCompile(`^Package: php([5-9]\.[0-9]{1,2})-fpm$`)

// phpRepoCache keeps what the lists said until one of them changes.
var phpRepoCache struct {
	sync.Mutex
	key      string
	versions map[string]bool
}

// repoPHPVersions returns the PHP versions apt's lists of the PHP
// repository have (a php<v>-fpm package), and false when there are no
// such lists to read.
func repoPHPVersions() (map[string]bool, bool) {
	var files []string
	for _, g := range phpListGlobs() {
		m, _ := filepath.Glob(g)
		files = append(files, m...)
	}
	sort.Strings(files)
	key := ""
	for _, f := range files {
		fi, err := os.Stat(f)
		if err != nil {
			continue
		}
		key += f + "|" + fi.ModTime().Format(time.RFC3339Nano) + "|" + strconv.FormatInt(fi.Size(), 10) + "\n"
	}
	if key == "" {
		return nil, false
	}
	phpRepoCache.Lock()
	defer phpRepoCache.Unlock()
	if phpRepoCache.key == key {
		return phpRepoCache.versions, len(phpRepoCache.versions) > 0
	}
	versions := map[string]bool{}
	for _, f := range files {
		readPHPList(f, versions)
	}
	phpRepoCache.key, phpRepoCache.versions = key, versions
	return versions, len(versions) > 0
}

// readPHPList adds the php<v>-fpm packages of one list file (plain or
// gzip; other compressions are skipped).
func readPHPList(path string, versions map[string]bool) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	var r io.Reader = f
	switch {
	case strings.HasSuffix(path, "_Packages"):
	case strings.HasSuffix(path, "_Packages.gz"):
		zr, err := gzip.NewReader(f)
		if err != nil {
			return
		}
		r = zr
	default:
		return
	}
	sc := bufio.NewScanner(io.LimitReader(r, 64<<20))
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		if m := phpFPMPkgRe.FindStringSubmatch(sc.Text()); m != nil {
			versions[m[1]] = true
		}
	}
}

// OfferedPHPVersions returns the PHP versions the Software page lists,
// oldest first (see the top of this file).
func (m *Manager) OfferedPHPVersions(ctx context.Context) []string {
	return offeredPHP(m.distroPHP(ctx))
}

func offeredPHP(distro string) []string {
	repo, known := repoPHPVersions()
	all := slices.Clone(PHPVersions)
	for v := range repo {
		all = append(all, v)
	}
	all = append(all, installedPHP()...)
	if distro != "" && phpDirRe.MatchString(distro) {
		all = append(all, distro)
	}
	var out []string
	for _, v := range all {
		if slices.Contains(out, v) {
			continue
		}
		switch {
		case phpInstalled(v), v == distro, IsLegacyPHP(v):
		case known && !repo[v]:
			continue // the repository does not have it
		}
		out = append(out, v)
	}
	sort.Slice(out, func(a, b int) bool { return phpNumber(out[a]) < phpNumber(out[b]) })
	return out
}
