package postgres

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/pgpkey"
	"github.com/nefocp/nefo/internal/usererr"
)

// Other major versions come from the PostgreSQL project's own apt
// repository, apt.postgresql.org ("PGDG"), which builds every supported
// major version for each Debian and Ubuntu release. The panel adds the
// repository itself, as it does for MySQL: the signing key is downloaded
// over HTTPS, accepted only if it is exactly the pinned key, and trusted
// for this repository only (signed-by), not for all of apt. The
// distribution's own version stays the default.
//
// One major version runs at a time. With the repository in place the
// panel installs postgresql-<major> by name, never the "postgresql"
// metapackage, whose PGDG build always depends on the newest major and
// would bring a second server with the next "apt upgrade".
const (
	// pgdgFingerprint is "PostgreSQL Debian Repository" (RSA 4096,
	// 2011-10-13, key ID 7FCC7D46ACCC4CF8, no expiry), which signs the
	// repository's Release files. Source: https://wiki.postgresql.org/wiki/Apt
	// and the copy postgresql-common ships (pgdg/apt.postgresql.org.asc).
	pgdgFingerprint = "B97B0AFCAA1A47F044F244A07FCC7D46ACCC4CF8"
	pgdgKeyURL      = "https://www.postgresql.org/media/keys/ACCC4CF8.asc"
	pgdgRepoURL     = "https://apt.postgresql.org/pub/repos/apt"
	pgdgKeyring     = "/etc/apt/keyrings/panel-pgdg.gpg"
	pgdgList        = "/etc/apt/sources.list.d/panel-pgdg.list"
)

// pgRelease is one of the systems the panel supports, with its own
// PostgreSQL major version (the "postgresql" metapackage's dependency).
type pgRelease struct {
	id     string // ubuntu or debian
	name   string // for people: "Ubuntu 24.04"
	distro string // the distribution's major
	pgdg   bool   // apt.postgresql.org serves <codename>-pgdg
}

// pgReleases, by codename: checked on 2026-11-27 against
// packages.ubuntu.com and packages.debian.org (the metapackage), and
// https://apt.postgresql.org/pub/repos/apt/dists/ (jammy-pgdg, noble-pgdg,
// resolute-pgdg, bookworm-pgdg and trixie-pgdg all carry postgresql-10 to
// postgresql-19 for amd64 and arm64 in "main").
var pgReleases = map[string]pgRelease{
	"jammy":    {"ubuntu", "Ubuntu 22.04", "14", true},
	"noble":    {"ubuntu", "Ubuntu 24.04", "16", true},
	"resolute": {"ubuntu", "Ubuntu 26.04", "18", true},
	"bookworm": {"debian", "Debian 12", "15", true},
	"trixie":   {"debian", "Debian 13", "17", true},
}

// pgdgMajors are the major versions the panel offers from PGDG: those the
// PostgreSQL project still supports (five years each; 14 ended on
// 2026-11-12, 15 ends in November 2027, 19 in November 2031). PGDG also
// builds older ones, which get no security fixes.
var pgdgMajors = []string{"15", "16", "17", "18", "19"}

// pgdgRepo is where the repository's files go and what the system is;
// tests replace it.
type pgdgRepo struct {
	keyURL    string
	keyring   string
	list      string
	osRelease func() (id, codename string)
	arch      func() string
	client    *http.Client
}

func defaultPGDGRepo() pgdgRepo {
	return pgdgRepo{
		keyURL: pgdgKeyURL, keyring: pgdgKeyring, list: pgdgList,
		osRelease: func() (string, string) { return readOSRelease("/etc/os-release") },
		arch:      func() string { return runtime.GOARCH },
		client:    &http.Client{Timeout: 2 * time.Minute},
	}
}

// readOSRelease returns ID and VERSION_CODENAME from an os-release file.
func readOSRelease(path string) (id, codename string) {
	f, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), "=")
		if !ok {
			continue
		}
		v = strings.Trim(v, `"'`)
		switch k {
		case "ID":
			id = v
		case "VERSION_CODENAME":
			codename = v
		}
	}
	return id, codename
}

// release returns this system's entry, and its name for messages.
func (m *Manager) release() (pgRelease, string, bool) {
	id, codename := m.pgdg.osRelease()
	name := strings.TrimSpace(id + " " + codename)
	if name == "" {
		name = "?"
	}
	r, ok := pgReleases[codename]
	if !ok || r.id != id {
		return pgRelease{}, name, false
	}
	return r, r.name, true
}

// pgdgAvailable returns why PGDG cannot be used here (a usererr), or nil.
func (m *Manager) pgdgAvailable() error {
	r, name, ok := m.release()
	if !ok || !r.pgdg {
		return usererr.New("pg_pgdg_unsupported_os", name)
	}
	if a := m.pgdg.arch(); a != "amd64" && a != "arm64" {
		return usererr.New("pg_pgdg_unsupported_arch", a)
	}
	return nil
}

// Versions is what the Software page offers when PostgreSQL is installed,
// and what an installed server can be upgraded to.
type Versions struct {
	Release string `json:"release"` // "ubuntu noble"
	// Distribution is the distribution's own major version ("" when the
	// system is not one the panel knows), the default.
	Distribution string `json:"distribution"`
	// PGDG are the major versions offered from apt.postgresql.org;
	// PGDGUnavailable is why there are none (an error code with
	// PGDGDetail).
	PGDG            []string `json:"pgdg"`
	PGDGUnavailable string   `json:"pgdg_unavailable,omitempty"`
	PGDGDetail      string   `json:"pgdg_detail,omitempty"`
	// Installed is the major version that runs, with its Source
	// ("distribution" or "pgdg").
	Installed string `json:"installed,omitempty"`
	Source    string `json:"source,omitempty"`
	// Upgrades are the newer major versions the cluster can move to;
	// UpgradeBlocked is why it can't now (an error code with
	// UpgradeDetail).
	Upgrades       []string `json:"upgrades"`
	UpgradeBlocked string   `json:"upgrade_blocked,omitempty"`
	UpgradeDetail  string   `json:"upgrade_detail,omitempty"`
}

// Versions returns the install and upgrade choices. It reads files and
// runs dpkg-query and "postgres --version" at most, nothing slow.
func (m *Manager) Versions(ctx context.Context) Versions {
	v := Versions{PGDG: []string{}, Upgrades: []string{}}
	r, name, known := m.release()
	v.Release = name
	if known {
		v.Distribution = r.distro
	}
	if err := m.pgdgAvailable(); err != nil {
		v.PGDGUnavailable, v.PGDGDetail = codeOf(err)
	} else {
		for _, mj := range pgdgMajors {
			if mj != v.Distribution {
				v.PGDG = append(v.PGDG, mj)
			}
		}
	}
	st := m.Status(ctx)
	if !st.Installed {
		return v
	}
	v.Installed, v.Source = st.Major, st.Source
	cur, _ := strconv.Atoi(st.Major)
	if v.PGDGUnavailable == "" {
		for _, mj := range pgdgMajors {
			if n, _ := strconv.Atoi(mj); n > cur {
				v.Upgrades = append(v.Upgrades, mj)
			}
		}
	}
	if len(v.Upgrades) > 0 {
		if err := m.checkUpgradable(ctx); err != nil {
			v.UpgradeBlocked, v.UpgradeDetail = codeOf(err)
		}
	}
	return v
}

func codeOf(err error) (string, string) {
	if ue, ok := err.(*usererr.Error); ok {
		return ue.Code, ue.Detail
	}
	return "pg_failed", err.Error()
}

// checkMajor checks a major version asked for at install: "" (the
// distribution's) or one of the PGDG choices.
func (m *Manager) checkMajor(major string) error {
	if r, _, _ := m.release(); major == "" || r.distro == major {
		return nil // the distribution's own: installed from the distribution
	}
	if !slices.Contains(pgdgMajors, major) {
		return usererr.New("pg_version_unknown", major)
	}
	return m.pgdgAvailable()
}

// usePGDG reports whether major (checked) comes from PGDG.
func (m *Manager) usePGDG(major string) bool {
	if major == "" {
		return false
	}
	r, _, _ := m.release()
	return r.distro != major
}

// addPGDGRepo writes the pinned key and the repository's source list.
func (m *Manager) addPGDGRepo(ctx context.Context) error {
	if err := m.pgdgAvailable(); err != nil {
		return err
	}
	_, codename := m.pgdg.osRelease()
	key, err := m.fetchPGDGKey(ctx)
	if err != nil {
		return err
	}
	if err := fsutil.MkdirAll(filepath.Dir(m.pgdg.keyring), 0o755); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(m.pgdg.keyring, key, 0o644); err != nil {
		return err
	}
	list := fmt.Sprintf("# Managed by Nefo: PostgreSQL from the PostgreSQL project's repository (apt.postgresql.org).\n"+
		"# Removed with PostgreSQL on the Software page.\n"+
		"deb [arch=%s signed-by=%s] %s %s-pgdg main\n", m.pgdg.arch(), m.pgdg.keyring, pgdgRepoURL, codename)
	return fsutil.WriteFileAtomic(m.pgdg.list, []byte(list), 0o644)
}

// removePGDGRepo removes the repository's files, if they are there.
func (m *Manager) removePGDGRepo(lg Logger) error {
	for _, p := range []string{m.pgdg.list, m.pgdg.keyring} {
		if err := os.Remove(p); err == nil {
			lg.Logf("Removed %s", p)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// hasPGDGRepo reports whether the panel's PGDG source list is in place.
func (m *Manager) hasPGDGRepo() bool { return fsutil.Exists(m.pgdg.list) }

// fetchPGDGKey downloads the repository's signing key (armored or binary)
// and returns it in the binary form apt reads, only if it is the pinned key.
func (m *Manager) fetchPGDGKey(ctx context.Context) ([]byte, error) {
	b, err := httpGet(ctx, m.pgdg.client, m.pgdg.keyURL, 256<<10)
	if err != nil {
		return nil, err
	}
	if bytes.Contains(b, []byte("-----BEGIN PGP")) {
		if b, err = pgpkey.Dearmor(b); err != nil {
			return nil, fmt.Errorf("the PostgreSQL repository's signing key could not be read: %w", err)
		}
	}
	fp, err := pgpkey.Fingerprint(b)
	if err != nil {
		return nil, fmt.Errorf("the PostgreSQL repository's signing key could not be read: %w", err)
	}
	if fp != pgdgFingerprint {
		return nil, fmt.Errorf("the PostgreSQL repository's signing key has the fingerprint %s, not the expected %s: not installed", fp, pgdgFingerprint)
	}
	return b, nil
}

func httpGet(ctx context.Context, c *http.Client, url string, max int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err == nil && int64(len(b)) > max {
		err = fmt.Errorf("download %s: larger than expected", url)
	}
	return b, err
}
