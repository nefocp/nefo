package hosting

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

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
// binary OpenPGP key file (version 4 keys). A file with secret keys, more
// than one primary key or packets it does not expect in a public key is
// refused: apt would trust every key in it.
func openPGPFingerprint(b []byte) (string, error) {
	var fp string
	primaries := 0
	for len(b) > 0 {
		tag, body, rest, err := openPGPPacket(b)
		if err != nil {
			return "", err
		}
		b = rest
		switch tag {
		case 6: // public key
			primaries++
			if len(body) == 0 || body[0] != 4 {
				return "", errors.New("not a version 4 key")
			}
			h := sha1.New()
			h.Write([]byte{0x99, byte(len(body) >> 8), byte(len(body))})
			h.Write(body)
			fp = strings.ToUpper(hex.EncodeToString(h.Sum(nil)))
		case 2, 12, 13, 14, 17: // signature, trust, user ID, public subkey, user attribute
		default:
			return "", fmt.Errorf("unexpected OpenPGP packet %d", tag)
		}
	}
	if primaries != 1 {
		return "", fmt.Errorf("%d primary keys instead of 1", primaries)
	}
	return fp, nil
}

// openPGPPacket splits the first packet off b (RFC 4880, section 4.2).
func openPGPPacket(b []byte) (tag int, body, rest []byte, err error) {
	if len(b) < 2 || b[0]&0x80 == 0 {
		return 0, nil, nil, errors.New("not an OpenPGP packet (armored keys are not accepted)")
	}
	var n, hdr int
	if b[0]&0x40 != 0 { // new format
		tag = int(b[0] & 0x3f)
		switch l := b[1]; {
		case l < 192:
			n, hdr = int(l), 2
		case l < 224:
			if len(b) < 3 {
				return 0, nil, nil, errors.New("truncated packet")
			}
			n, hdr = (int(l)-192)<<8+int(b[2])+192, 3
		case l == 255:
			if len(b) < 6 {
				return 0, nil, nil, errors.New("truncated packet")
			}
			n, hdr = int(binary.BigEndian.Uint32(b[2:6])), 6
		default:
			return 0, nil, nil, errors.New("partial packet lengths are not accepted in a key")
		}
	} else { // old format
		tag = int(b[0]>>2) & 0x0f
		switch b[0] & 3 {
		case 0:
			n, hdr = int(b[1]), 2
		case 1:
			if len(b) < 3 {
				return 0, nil, nil, errors.New("truncated packet")
			}
			n, hdr = int(binary.BigEndian.Uint16(b[1:3])), 3
		case 2:
			if len(b) < 5 {
				return 0, nil, nil, errors.New("truncated packet")
			}
			n, hdr = int(binary.BigEndian.Uint32(b[1:5])), 5
		default:
			return 0, nil, nil, errors.New("indeterminate packet length")
		}
	}
	if n < 0 || hdr+n > len(b) {
		return 0, nil, nil, errors.New("truncated packet")
	}
	return tag, b[hdr : hdr+n], b[hdr+n:], nil
}
