package database

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/pgpkey"
	"github.com/nefocp/nefo/internal/usererr"
)

// MariaDB from MariaDB's own apt repository (dlm.mariadb.com), next to the
// distribution's MariaDB, which stays the default: the distribution's gets
// its security fixes with the system's updates and needs no other
// repository. MariaDB's repository has one URL per series, so a server on
// 11.4 stays on 11.4 and only gets its point releases. As for MySQL, the
// panel writes the repository itself, with the signing key downloaded over
// HTTPS, accepted only if it is exactly the pinned key, and trusted for this
// repository only (signed-by).
const (
	// mariadbKeyFingerprint is "MariaDB Signing Key
	// <signing-key@mariadb.org>", RSA 4096 from 2016-03-30 (key ID
	// F1656F24C74CD1D8), which signs the InRelease files of the 11.4, 11.8
	// and 12.3 repositories (checked in November 2026, 12.3 on all five
	// releases). Source:
	// https://mariadb.com/kb/en/gpg/, which gives this fingerprint;
	// keyserver.ubuntu.com has the same key. It has no expiry date.
	mariadbKeyFingerprint = "177F4010FE56CA3336300305F1656F24C74CD1D8"
	// mariadbKeyURL is that key alone (armored). MariaDB's keyring files
	// (mariadb-keyring-2019.gpg and -2025.gpg) also hold its MaxScale and
	// Enterprise keys, which apt would then trust for this repository too.
	mariadbKeyURL       = "https://supplychain.mariadb.com/MariaDB-Server-GPG-KEY"
	mariadbKeyserverURL = "https://keyserver.ubuntu.com/pks/lookup?op=get&options=mr&search=0x" + mariadbKeyFingerprint
	mariadbRepoURL      = "https://dlm.mariadb.com/repo/mariadb-server/"
	mariadbKeyring      = "/etc/apt/keyrings/panel-mariadb.gpg"
	mariadbRepoList     = "/etc/apt/sources.list.d/panel-mariadb.list"
)

// MariaDBSeries is a long-term support series of MariaDB that the panel
// installs from MariaDB's repository.
type MariaDBSeries struct {
	Series string
	// Until is the end of MariaDB's support for the series.
	Until string
	// Releases are the codenames the repository has packages for (amd64
	// and arm64 on all of them).
	Releases []string
}

// mariadbSeries are the LTS series offered, oldest first, with the
// releases their repositories have (dists/<codename>/Release under
// https://dlm.mariadb.com/repo/mariadb-server/<series>/repo/<ubuntu or
// debian>/, November 2026: 11.4.14, 11.8.10 and 12.3.4). The end-of-support
// dates are MariaDB's (https://downloads.mariadb.org/rest-api/mariadb/).
// 12.3 uses the same package, program and service names, root's socket
// sign-in and mariadb-upgrade; Galera moved to an optional package
// (mariadb-server-galera), so it installs less.
var mariadbSeries = []MariaDBSeries{
	{Series: "11.4", Until: "2029-05-29", Releases: []string{"jammy", "noble", "bookworm"}},
	{Series: "11.8", Until: "2028-06-04", Releases: []string{"jammy", "noble", "resolute", "bookworm", "trixie"}},
	{Series: "12.3", Until: "2029-06-12", Releases: []string{"jammy", "noble", "resolute", "bookworm", "trixie"}},
}

// distroMariaDB is the series of each release's own MariaDB packages
// (apt-cache policy mariadb-server, November 2026).
var distroMariaDB = map[string]string{
	"jammy": "10.6", "noble": "10.11", "resolute": "11.8",
	"bookworm": "10.11", "trixie": "11.8",
}

// MariaDBSeriesList returns the series offered from MariaDB's repository.
func MariaDBSeriesList() []MariaDBSeries { return mariadbSeries }

func seriesInfo(series string) (MariaDBSeries, bool) {
	for _, s := range mariadbSeries {
		if s.Series == series {
			return s, true
		}
	}
	return MariaDBSeries{}, false
}

// SeriesOf returns the series of a version: "11.4" for "11.4.14" or
// "1:11.4.14+maria~ubu2404", "" when it has none.
func SeriesOf(version string) string {
	if _, v, ok := strings.Cut(version, ":"); ok {
		version = v
	}
	mm := regexp.MustCompile(`^([0-9]+)\.([0-9]+)`).FindStringSubmatch(version)
	if mm == nil {
		return ""
	}
	return mm[1] + "." + mm[2]
}

// CompareSeries orders "10.6" < "10.11" < "11.4"; "" is lowest.
func CompareSeries(a, b string) int { return compareSeries(a, b) }

func compareSeries(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < 2; i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			return x - y
		}
	}
	return 0
}

// MariaDBChoice is one way to install MariaDB on this server.
type MariaDBChoice struct {
	// ID is the Software page's: "mariadb" for the distribution's, or
	// "mariadb-11.4" for a series from MariaDB's repository.
	ID string
	// Series is "10.11" or "11.4"; "" when the distribution's is unknown.
	Series string
	// Repo means it comes from MariaDB's repository.
	Repo  bool
	Until string // end of MariaDB's support for a series of its repository
	// Unavailable is why it can't be installed on this server (a usererr),
	// nil when it can.
	Unavailable error
}

// SeriesID is the Software page's ID of a series of MariaDB's repository.
func SeriesID(series string) string { return "mariadb-" + series }

// ParseMariaDBID returns the series of a Software page ID: "" for
// "mariadb", "11.4" for "mariadb-11.4"; ok is false for anything else.
func ParseMariaDBID(id string) (series string, ok bool) {
	if id == "mariadb" {
		return "", true
	}
	s, found := strings.CutPrefix(id, "mariadb-")
	if !found {
		return "", false
	}
	if _, ok := seriesInfo(s); !ok {
		return "", false
	}
	return s, true
}

// DistroMariaDBSeries is the series of the distribution's MariaDB, "" when
// the release is not one the panel knows.
func (m *Manager) DistroMariaDBSeries() string {
	_, codename := m.repo.osRelease()
	return distroMariaDB[codename]
}

// MariaDBChoices lists the ways to install MariaDB here: the
// distribution's, then each series of MariaDB's repository, with why it is
// not offered. A series is offered when the repository has packages for
// this release and it is newer than the distribution's own.
func (m *Manager) MariaDBChoices() []MariaDBChoice {
	distro := m.DistroMariaDBSeries()
	out := []MariaDBChoice{{ID: "mariadb", Series: distro}}
	if !m.supported() {
		out[0].Unavailable = usererr.New("unsupported")
	}
	for _, s := range mariadbSeries {
		c := MariaDBChoice{ID: SeriesID(s.Series), Series: s.Series, Repo: true, Until: s.Until, Unavailable: m.seriesAvailable(s.Series)}
		if c.Unavailable == nil && distro != "" && compareSeries(s.Series, distro) <= 0 {
			c.Unavailable = usererr.New("mariadb_series_not_newer", distro)
		}
		out = append(out, c)
	}
	return out
}

// SeriesAvailable returns why MariaDB's repository can't provide series on
// this server (a usererr), or nil.
func (m *Manager) SeriesAvailable(series string) error { return m.seriesAvailable(series) }

func (m *Manager) seriesAvailable(series string) error {
	if !m.supported() {
		return usererr.New("unsupported")
	}
	s, ok := seriesInfo(series)
	if !ok {
		return usererr.New("unknown_software", "mariadb-"+series)
	}
	id, codename := m.repo.osRelease()
	release := strings.TrimSpace(id + " " + codename)
	if dist, ok := mysqlReleases[codename]; !ok || dist != id || !containsString(s.Releases, codename) {
		if release == "" {
			release = "?"
		}
		return usererr.New("mariadb_series_unavailable", series+" ("+release+")")
	}
	if a := m.repo.arch(); a != "amd64" && a != "arm64" {
		return usererr.New("mariadb_series_unavailable", series+" ("+a+")")
	}
	return nil
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// CheckInstallMariaDB returns why MariaDB of series ("" for the
// distribution's) cannot be installed now, or nil.
func (m *Manager) CheckInstallMariaDB(ctx context.Context, series string) error {
	if !m.supported() {
		return usererr.New("unsupported")
	}
	// Installing again what is installed repairs it, as before; another
	// series is an upgrade (or a downgrade), not an install.
	if m.detected() == MariaDB {
		if cur := m.MariaDBRepoSeries(); cur != series {
			name := "MariaDB"
			if cur != "" {
				name += " " + cur
			}
			return usererr.New("mariadb_installed", name)
		}
	}
	if series != "" {
		for _, c := range m.MariaDBChoices() {
			if c.ID == SeriesID(series) && c.Unavailable != nil {
				return c.Unavailable
			}
		}
		if err := m.seriesAvailable(series); err != nil {
			return err
		}
	}
	if err := m.checkInstall(ctx, MariaDB); err != nil {
		return err
	}
	return m.checkDataDirSeries(series)
}

// checkDataDirSeries refuses to install a MariaDB older than the one whose
// data the data directory keeps: it could not read the upgraded files.
func (m *Manager) checkDataDirSeries(series string) error {
	if m.dataDirEngine() != MariaDB {
		return nil
	}
	target := series
	if target == "" {
		target = m.DistroMariaDBSeries()
	}
	have := m.dataDirSeries()
	if have == "" || target == "" || compareSeries(have, target) <= 0 {
		return nil
	}
	return usererr.New("mariadb_datadir_newer", have)
}

// dataDirSeries reads the series of the MariaDB that last upgraded the
// data directory, from the file mariadb-upgrade leaves in it.
func (m *Manager) dataDirSeries() string {
	for _, n := range []string{"mariadb_upgrade_info", "mysql_upgrade_info"} {
		b, err := os.ReadFile(filepath.Join(m.dataDirPath(), n))
		if err == nil {
			return SeriesOf(strings.TrimSpace(string(b)))
		}
	}
	// Else the InnoDB redo log, whose header names the version that made
	// it ("MariaDB 11.8.10"), since MariaDB 10.8.
	f, err := os.Open(filepath.Join(m.dataDirPath(), "ib_logfile0"))
	if err != nil {
		return ""
	}
	defer f.Close()
	head := make([]byte, 512)
	n, _ := f.Read(head)
	if mm := regexp.MustCompile(`MariaDB ([0-9]+\.[0-9]+)\.[0-9]`).FindSubmatch(head[:n]); mm != nil {
		return string(mm[1])
	}
	return ""
}

// mariaRepoPaths are the panel's files for MariaDB's repository.
func (m *Manager) mariaRepoPaths() (keyring, list string) {
	if m.mariaKeyring != "" {
		return m.mariaKeyring, m.mariaList
	}
	return mariadbKeyring, mariadbRepoList
}

// MariaDBRepoSeries returns the series of the panel's MariaDB repository,
// "" when it has none.
func (m *Manager) MariaDBRepoSeries() string {
	_, list := m.mariaRepoPaths()
	b, err := os.ReadFile(list)
	if err != nil {
		return ""
	}
	mm := regexp.MustCompile(`(?m)^deb .*/mariadb-server/([0-9]+\.[0-9]+)/repo/`).FindSubmatch(b)
	if mm == nil {
		return ""
	}
	return string(mm[1])
}

// addMariaDBRepo writes the pinned key and the source list of a series.
func (m *Manager) addMariaDBRepo(ctx context.Context, series string) error {
	if err := m.seriesAvailable(series); err != nil {
		return err
	}
	id, codename := m.repo.osRelease()
	urls := m.mariaKeyURLs
	if len(urls) == 0 {
		urls = []string{mariadbKeyURL, mariadbKeyserverURL}
	}
	var key []byte
	var err error
	for _, u := range urls {
		if key, err = m.fetchPinnedKey(ctx, u, mariadbKeyFingerprint, "MariaDB"); err == nil {
			break
		}
	}
	if err != nil {
		return err
	}
	keyring, list := m.mariaRepoPaths()
	if err := fsutil.MkdirAll(filepath.Dir(keyring), 0o755); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(keyring, key, 0o644); err != nil {
		return err
	}
	if err := fsutil.MkdirAll(filepath.Dir(list), 0o755); err != nil {
		return err
	}
	src := fmt.Sprintf("# Managed by the panel: MariaDB %s from MariaDB's repository.\n"+
		"deb [signed-by=%s] %s%s/repo/%s %s main\n", series, keyring, mariadbRepoURL, series, id, codename)
	return fsutil.WriteFileAtomic(list, []byte(src), 0o644)
}

// removeMariaDBRepo deletes the panel's MariaDB repository and key, and
// says so in lg (which may be nil).
func (m *Manager) removeMariaDBRepo(lg Logger) {
	keyring, list := m.mariaRepoPaths()
	for _, f := range []string{list, keyring} {
		if err := os.Remove(f); err == nil && lg != nil {
			lg.Logf("Removed %s", f)
		}
	}
}

// fetchPinnedKey downloads a signing key (armored or binary) and returns it
// in the binary form apt reads, only if it is the key with fingerprint.
func (m *Manager) fetchPinnedKey(ctx context.Context, url, fingerprint, name string) ([]byte, error) {
	b, err := httpGet(ctx, m.repo.client, url, 256<<10)
	if err != nil {
		return nil, err
	}
	if bytes.Contains(b, []byte("-----BEGIN PGP")) {
		if b, err = pgpkey.Dearmor(b); err != nil {
			return nil, fmt.Errorf("%s's signing key could not be read: %w", name, err)
		}
	}
	fp, err := pgpkey.Fingerprint(b)
	if err != nil {
		return nil, fmt.Errorf("%s's signing key could not be read: %w", name, err)
	}
	if fp != fingerprint {
		return nil, fmt.Errorf("%s's signing key has the fingerprint %s, not the expected %s: not installed", name, fp, fingerprint)
	}
	return b, nil
}

// candidate returns apt's candidate version of a package ("" for none).
func (m *Manager) candidate(ctx context.Context, pkg string) string {
	out, _ := m.run.Output(ctx, "apt-cache", "policy", pkg)
	return parseCandidate(out)
}

func parseCandidate(out string) string {
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		if v, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "Candidate:"); ok {
			v = strings.TrimSpace(v)
			if v == "(none)" {
				return ""
			}
			return v
		}
	}
	return ""
}

// checkCandidate makes sure apt will install series from MariaDB's
// repository, not the distribution's packages (if MariaDB's repository
// could not be read, apt-get update only warns).
func (m *Manager) checkCandidate(ctx context.Context, series string) error {
	v := m.candidate(ctx, "mariadb-server")
	if SeriesOf(v) != series || !strings.Contains(v, "maria") {
		if v == "" {
			v = "none"
		}
		return fmt.Errorf("apt would not install MariaDB %s from MariaDB's repository (its candidate is %s): check that this server reaches dlm.mariadb.com", series, v)
	}
	return nil
}
