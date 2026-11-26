package hosting

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/nefocp/nefo/internal/pgpkey"
	"github.com/nefocp/nefo/internal/version"
)

// OpenLiteSpeed comes from LiteSpeed's apt repository. The panel adds it
// itself instead of running LiteSpeed's setup script: the signing key is
// downloaded over HTTPS, accepted only if it is exactly the pinned key, and
// trusted for this repository only (signed-by), not for all of apt.
const (
	olsRepoURL = "https://rpms.litespeedtech.com/debian/"
	olsKeyURL  = "https://rpms.litespeedtech.com/debian/lst_repo.gpg"
	// olsKeyFingerprint is "LiteSpeed Technologies Inc (LiteSpeed
	// Technologies Inc GnuPG Key) <info@litespeedtech.com>", RSA 4096 from
	// 2016, which signs the Debian and Ubuntu repositories. It is the key
	// LiteSpeed's setup script (https://repo.litespeed.sh) installs from
	// olsKeyURL, and the one keyserver.ubuntu.com and keys.openpgp.org
	// publish under this fingerprint.
	olsKeyFingerprint = "3E892522DB44E1B063D366C5011AA62DEDA1F085"
	olsKeyring        = "/etc/apt/keyrings/panel-litespeed.gpg"
	olsRepoList       = "/etc/apt/sources.list.d/panel-litespeed.list"
)

// olsCodenames are the releases LiteSpeed's repository has packages for.
var olsCodenames = []string{"focal", "jammy", "noble", "resolute", "buster", "bullseye", "bookworm", "trixie"}

var codenameRe = regexp.MustCompile(`^[a-z]{2,20}$`)

// OLSRelease reports whether LiteSpeed's repository has OpenLiteSpeed for
// this release of the operating system (addRepo also needs an amd64 or
// arm64 CPU).
func OLSRelease() bool { return slices.Contains(olsCodenames, osCodename()) }

// addRepo adds LiteSpeed's repository with its pinned key, and the systemd
// drop-in that keeps the package from starting its demo configuration.
func (o olsServer) addRepo(ctx context.Context, lg Logger) error {
	codename := osCodename()
	if !slices.Contains(olsCodenames, codename) {
		return fmt.Errorf("LiteSpeed's repository has no OpenLiteSpeed packages for this release (%q)", codename)
	}
	arch, _ := o.m.run.Output(ctx, "dpkg", "--print-architecture")
	if a := strings.TrimSpace(arch); a != "amd64" && a != "arm64" {
		return fmt.Errorf("LiteSpeed's repository has no OpenLiteSpeed packages for the %q architecture", a)
	}
	lg.Logf("Adding LiteSpeed's package repository (signing key %s)", olsKeyFingerprint)
	key, err := fetch(ctx, olsKeyURL, 64<<10)
	if err != nil {
		return err
	}
	fp, err := openPGPFingerprint(key)
	if err != nil {
		return fmt.Errorf("LiteSpeed's signing key could not be read: %w", err)
	}
	if fp != olsKeyFingerprint {
		return fmt.Errorf("LiteSpeed's signing key has the fingerprint %s, not the expected %s: not installed", fp, olsKeyFingerprint)
	}
	if err := mkdir("/etc/apt/keyrings", 0o755); err != nil {
		return err
	}
	if err := writeFile(olsKeyring, key, 0o644); err != nil {
		return err
	}
	list := fmt.Sprintf("# Managed by %s: OpenLiteSpeed, from LiteSpeed's repository.\ndeb [signed-by=%s] %s %s main\n",
		version.AppName, olsKeyring, olsRepoURL, codename)
	if err := writeFile(olsRepoList, []byte(list), 0o644); err != nil {
		return err
	}
	return o.writeDropIn(ctx)
}

// osCodename reads VERSION_CODENAME from /etc/os-release.
func osCodename() string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "VERSION_CODENAME="); ok {
			v = strings.Trim(v, `"'`)
			if codenameRe.MatchString(v) {
				return v
			}
		}
	}
	return ""
}

// openPGPFingerprint returns the fingerprint of the only primary key in a
// binary OpenPGP key file (see pgpkey.Fingerprint).
func openPGPFingerprint(b []byte) (string, error) { return pgpkey.Fingerprint(b) }
