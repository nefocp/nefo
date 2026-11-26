package software

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/fsutil"
)

// The PHP versions other than the distribution's come from Ondřej Surý's
// repository packages.sury.org/php, on Ubuntu and Debian alike. It used to
// be ppa:ondrej/php on Ubuntu, but the PPA is frozen: "This PPA is frozen
// and no longer receives updates. The PHP packages for Ubuntu are now
// published at https://packages.sury.org/php/"
// (https://launchpad.net/~ondrej/+archive/ubuntu/php). Servers that still
// have the PPA are moved over once at start-up (MigratePHPRepository).

const (
	// suryFingerprint is the repository's signing key, "DEB.SURY.ORG
	// Automatic Signing Key <deb@sury.org>" (rsa3072, 2019-03-18), which
	// signs the InRelease files of every release it serves. The key is
	// downloaded (so a later expiry date comes with it) and refused unless
	// it is this one and only this one.
	suryFingerprint = "15058500A0235D97F5D10063B188E2B695BD4743"
	// surySourceURL is the repository in the apt source; suryKeyring the
	// key apt checks it with (signed-by), suryList the source file.
	surySourceURL = "https://packages.sury.org/php/"
	suryKeyring   = "/usr/share/keyrings/sury-php.gpg"
	suryList      = "/etc/apt/sources.list.d/sury-php.list"
	// ppaKeyFile is the key add-apt-repository put in trusted.gpg.d for
	// the PPA on Ubuntu 22.04 (24.04 embeds it in the .sources file).
	ppaKeyFile = "/etc/apt/trusted.gpg.d/ondrej-ubuntu-php.gpg"
)

var (
	// suryBase is where the key and the release files are downloaded from;
	// tests serve them on 127.0.0.1.
	suryBase = surySourceURL
	// sysRoot prefixes the system paths above; tests use a temporary folder.
	sysRoot = "/"
	// suryCodenames are the releases packages.sury.org/php serves that the
	// panel supports (https://packages.sury.org/php/dists/, 2026-11-26).
	suryCodenames = []string{"jammy", "noble", "resolute", "bookworm", "trixie"}

	codenameRe = regexp.MustCompile(`^[a-z]{2,20}$`)
	ppaURIRe   = regexp.MustCompile(`^(https?://)?ppa\.(launchpad|launchpadcontent)\.net/ondrej/php(/|$)`)
)

func sysPath(p string) string { return filepath.Join(sysRoot, p) }

// pgpFingerprints returns the fingerprints of the primary keys in a binary
// OpenPGP keyring (v4 keys: the SHA-1 of the public key packet, RFC 4880
// 12.2).
func pgpFingerprints(b []byte) ([]string, error) {
	var fps []string
	for len(b) > 0 {
		hdr := b[0]
		if hdr&0x80 == 0 {
			return nil, errors.New("not an OpenPGP keyring")
		}
		var tag, n, hl int
		if hdr&0x40 != 0 { // new format
			tag = int(hdr & 0x3f)
			if len(b) < 2 {
				return nil, errors.New("truncated keyring")
			}
			switch l := int(b[1]); {
			case l < 192:
				n, hl = l, 2
			case l < 224:
				if len(b) < 3 {
					return nil, errors.New("truncated keyring")
				}
				n, hl = (l-192)<<8+int(b[2])+192, 3
			case l == 255:
				if len(b) < 6 {
					return nil, errors.New("truncated keyring")
				}
				n, hl = int(binary.BigEndian.Uint32(b[2:6])), 6
			default:
				return nil, errors.New("partial packet lengths are not allowed in a keyring")
			}
		} else { // old format
			tag = int(hdr>>2) & 0x0f
			switch hdr & 3 {
			case 0:
				if len(b) < 2 {
					return nil, errors.New("truncated keyring")
				}
				n, hl = int(b[1]), 2
			case 1:
				if len(b) < 3 {
					return nil, errors.New("truncated keyring")
				}
				n, hl = int(binary.BigEndian.Uint16(b[1:3])), 3
			case 2:
				if len(b) < 5 {
					return nil, errors.New("truncated keyring")
				}
				n, hl = int(binary.BigEndian.Uint32(b[1:5])), 5
			default:
				return nil, errors.New("indeterminate packet length")
			}
		}
		if n < 0 || hl+n > len(b) {
			return nil, errors.New("truncated keyring")
		}
		body := b[hl : hl+n]
		switch tag {
		case 5, 7: // secret keys have no place here
			return nil, errors.New("the keyring holds a secret key")
		case 6:
			if n == 0 || body[0] != 4 || n > 0xffff {
				return nil, errors.New("not a version 4 public key")
			}
			h := sha1.New()
			h.Write([]byte{0x99, byte(n >> 8), byte(n)})
			h.Write(body)
			fps = append(fps, strings.ToUpper(hex.EncodeToString(h.Sum(nil))))
		}
		b = b[hl+n:]
	}
	return fps, nil
}

// checkSuryKey accepts a keyring holding the pinned key and no other.
func checkSuryKey(b []byte) error {
	fps, err := pgpFingerprints(b)
	if err != nil {
		return err
	}
	if len(fps) == 0 {
		return errors.New("no key in it")
	}
	for _, fp := range fps {
		if fp != suryFingerprint {
			return fmt.Errorf("unexpected key %s (expected %s)", fp, suryFingerprint)
		}
	}
	return nil
}

// ensureSuryKey makes suryKeyring hold the pinned key. It downloads the key
// (a key whose expiry date was extended replaces the old file); when that
// fails, a valid key already there is kept.
func ensureSuryKey(ctx context.Context) error {
	old, oldErr := os.ReadFile(sysPath(suryKeyring))
	if oldErr == nil && checkSuryKey(old) != nil {
		oldErr = errors.New("invalid")
	}
	key, err := download(ctx, suryBase+"apt.gpg", 64<<10)
	if err == nil {
		if kerr := checkSuryKey(key); kerr != nil {
			err = fmt.Errorf("the key at %sapt.gpg: %w", suryBase, kerr)
		}
	}
	if err != nil {
		if oldErr == nil {
			return nil
		}
		return err
	}
	if oldErr == nil && string(old) == string(key) {
		return nil
	}
	return fsutil.WriteFileAtomic(sysPath(suryKeyring), key, 0o644)
}

// surySourceFor checks that packages.sury.org/php serves a release, and
// writes its apt source.
func surySourceFor(ctx context.Context, codename string) error {
	if !codenameRe.MatchString(codename) {
		return fmt.Errorf("unknown release %q", codename)
	}
	// A source apt can't fetch would break every apt-get update.
	if _, err := download(ctx, suryBase+"dists/"+codename+"/InRelease", 4<<20); err != nil {
		return fmt.Errorf("packages.sury.org/php does not serve %s: %w", codename, err)
	}
	src := fmt.Sprintf("# Added by the server panel: PHP versions from packages.sury.org.\ndeb [signed-by=%s] %s %s main\n", suryKeyring, surySourceURL, codename)
	return fsutil.WriteFileAtomic(sysPath(suryList), []byte(src), 0o644)
}

// aptSource is one apt source file and the repositories it lists.
type aptSource struct {
	path string
	uris []string
}

// aptSources reads sources.list and the files in sources.list.d: one-line
// (.list) and deb822 (.sources) entries that are not commented out.
func aptSources() []aptSource {
	files := []string{sysPath("/etc/apt/sources.list")}
	for _, pat := range []string{"*.list", "*.sources"} {
		m, _ := filepath.Glob(filepath.Join(sysPath("/etc/apt/sources.list.d"), pat))
		files = append(files, m...)
	}
	var out []aptSource
	for _, f := range files {
		fh, err := os.Open(f)
		if err != nil {
			continue
		}
		src := aptSource{path: f}
		sc := bufio.NewScanner(io.LimitReader(fh, 1<<20))
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || line[0] == '#' {
				continue
			}
			if strings.HasSuffix(f, ".sources") {
				if v, ok := cutField(line, "URIs:"); ok {
					src.uris = append(src.uris, strings.Fields(v)...)
				}
				continue
			}
			f := strings.Fields(line)
			if len(f) < 2 || (f[0] != "deb" && f[0] != "deb-src") {
				continue
			}
			i := 1
			if strings.HasPrefix(f[1], "[") { // [options]
				for i < len(f) && !strings.HasSuffix(f[i], "]") {
					i++
				}
				i++
			}
			if i < len(f) {
				src.uris = append(src.uris, f[i])
			}
		}
		fh.Close()
		out = append(out, src)
	}
	return out
}

func cutField(line, name string) (string, bool) {
	if len(line) >= len(name) && strings.EqualFold(line[:len(name)], name) {
		return strings.TrimSpace(line[len(name):]), true
	}
	return "", false
}

func isSuryURI(u string) bool {
	return strings.HasPrefix(strings.TrimSuffix(u, "/")+"/", "https://packages.sury.org/php/") ||
		strings.HasPrefix(strings.TrimSuffix(u, "/")+"/", "http://packages.sury.org/php/")
}

// phpSources returns whether a packages.sury.org/php source exists, the
// source files that list only the ondrej/php PPA, and the files that list
// it next to other repositories (which the panel does not edit).
func phpSources() (sury bool, ppa, mixed []string) {
	for _, s := range aptSources() {
		n := 0
		for _, u := range s.uris {
			if isSuryURI(u) {
				sury = true
			}
			if ppaURIRe.MatchString(u) {
				n++
			}
		}
		switch {
		case n == 0:
		case n == len(s.uris) && filepath.Dir(s.path) == sysPath("/etc/apt/sources.list.d"):
			ppa = append(ppa, s.path)
		default:
			mixed = append(mixed, s.path)
		}
	}
	return sury, ppa, mixed
}

// addSury sets packages.sury.org/php up for a release: the pinned key and
// the source, unless a source for it exists already (the panel's, or one
// from sury's own instructions with its debsuryorg keyring: a second one
// with another key would make apt refuse both).
func addSury(ctx context.Context, codename string, lg Logger) error {
	if sury, _, _ := phpSources(); sury {
		return nil
	}
	if !slices.Contains(suryCodenames, codename) {
		return fmt.Errorf("packages.sury.org/php has no packages for %q", codename)
	}
	lg.Logf("Adding the PHP repository packages.sury.org/php (key %s)", suryFingerprint)
	if err := ensureSuryKey(ctx); err != nil {
		return err
	}
	return surySourceFor(ctx, codename)
}

// MigratePHPRepository moves an Ubuntu server from the frozen
// ppa:ondrej/php to packages.sury.org/php, as the PPA's notice says: the
// sury source is added with the pinned key, then the PPA's source files,
// and the key add-apt-repository put in trusted.gpg.d for it, are moved
// into backupDir. Nothing is installed, removed or downgraded, and
// apt-get update does not run (another program may be using apt at
// start-up); the next update reads the new source, and the installed PHP
// packages update from it. It does nothing on Debian, without the PPA, and
// once done. If the new source can't be set up, the PPA stays. It returns
// what it changed, for the log.
func (m *Manager) MigratePHPRepository(ctx context.Context, backupDir string) (string, error) {
	b, _ := os.ReadFile(osReleaseFile)
	if osField(b, "ID") != "ubuntu" {
		return "", nil
	}
	sury, ppa, mixed := phpSources()
	if len(ppa) == 0 {
		if len(mixed) > 0 {
			return "", fmt.Errorf("the frozen ppa:ondrej/php is listed in %s next to other repositories; replace it with packages.sury.org/php by hand", strings.Join(mixed, ", "))
		}
		return "", nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	added := ""
	if !sury {
		codename := osField(b, "VERSION_CODENAME")
		if err := addSury(ctx, codename, discardLogger{}); err != nil {
			return "", fmt.Errorf("the frozen ppa:ondrej/php stays, because packages.sury.org/php could not be added: %w", err)
		}
		added = "added packages.sury.org/php (" + suryList + "), "
	}
	moved := append([]string{}, ppa...)
	if fi, err := os.Stat(sysPath(ppaKeyFile)); err == nil && fi.Mode().IsRegular() {
		moved = append(moved, sysPath(ppaKeyFile))
	}
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return "", err
	}
	stamp := time.Now().UTC().Format("20060102-150405")
	for _, p := range moved {
		if err := moveFile(p, filepath.Join(backupDir, filepath.Base(p)+"."+stamp)); err != nil {
			return "", err
		}
	}
	msg := added + "moved the frozen ppa:ondrej/php (" + strings.Join(moved, ", ") + ") to " + backupDir +
		". Installed PHP packages are unchanged and update from packages.sury.org after the next apt-get update."
	if len(mixed) > 0 {
		msg += " Still listed next to other repositories in " + strings.Join(mixed, ", ") + ": edit by hand."
	}
	return msg, nil
}

// moveFile renames, or copies and deletes across file systems.
func moveFile(from, to string) error {
	if err := os.Rename(from, to); err == nil {
		return nil
	}
	b, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	if err := os.WriteFile(to, b, 0o600); err != nil {
		return err
	}
	return os.Remove(from)
}

// suryListsMissing reports whether apt has not fetched packages.sury.org's
// lists since the source was added (by the migration at start-up).
func suryListsMissing() bool {
	if sury, _, _ := phpSources(); !sury {
		return false
	}
	m, _ := filepath.Glob(sysPath("/var/lib/apt/lists/packages.sury.org_php_*"))
	return len(m) == 0
}

type discardLogger struct{}

func (discardLogger) Write(p []byte) (int, error)     { return len(p), nil }
func (discardLogger) Logf(format string, args ...any) {}
