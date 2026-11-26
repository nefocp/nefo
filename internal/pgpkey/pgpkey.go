// Package pgpkey reads the fingerprint of an OpenPGP public key file, so a
// package repository's signing key can be checked against a fingerprint
// pinned in the code before apt is told to trust it (signed-by).
package pgpkey

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Fingerprint returns the fingerprint of the only primary key in a binary
// OpenPGP key file (version 4 keys). A file with secret keys, more than one
// primary key or packets it does not expect in a public key is refused: apt
// would trust every key in it.
func Fingerprint(b []byte) (string, error) {
	var fp string
	primaries := 0
	for len(b) > 0 {
		tag, body, rest, err := packet(b)
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

// packet splits the first packet off b (RFC 4880, section 4.2).
func packet(b []byte) (tag int, body, rest []byte, err error) {
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

// Expiry returns when the primary key of a binary key file expires, as its
// newest self-signature on a user ID says; never is true for a key without
// an expiry. The signatures are not verified: this only helps choose the
// newer of two copies of a key whose fingerprint was checked, and apt
// verifies them itself.
func Expiry(b []byte) (expires time.Time, never bool, err error) {
	fp, err := Fingerprint(b)
	if err != nil {
		return time.Time{}, false, err
	}
	fpBytes, _ := hex.DecodeString(fp)
	var created time.Time
	var newest time.Time
	var expiry uint32
	found, inUID := false, false
	for len(b) > 0 {
		tag, body, rest, err := packet(b)
		if err != nil {
			return time.Time{}, false, err
		}
		b = rest
		switch tag {
		case 6:
			if len(body) < 5 {
				return time.Time{}, false, errors.New("short key packet")
			}
			created = time.Unix(int64(binary.BigEndian.Uint32(body[1:5])), 0)
		case 13:
			inUID = true
		case 14, 17:
			inUID = false
		case 2:
			if !inUID {
				continue
			}
			s, ok := parseSig(body)
			if !ok || s.typ < 0x10 || s.typ > 0x13 || !s.issuedBy(fpBytes) {
				continue
			}
			if !found || s.created.After(newest) {
				found, newest, expiry = true, s.created, s.keyExpiry
			}
		}
	}
	if !found {
		return time.Time{}, false, errors.New("no self-signature on a user ID")
	}
	if expiry == 0 {
		return time.Time{}, true, nil
	}
	return created.Add(time.Duration(expiry) * time.Second), false, nil
}

// sig is what Expiry needs of a version 4 signature packet.
type sig struct {
	typ       byte
	created   time.Time
	keyExpiry uint32 // seconds after the key's creation, 0 for none
	issuerFP  []byte
	issuerID  []byte
}

func (s sig) issuedBy(fp []byte) bool {
	if len(s.issuerFP) == 21 && s.issuerFP[0] == 4 {
		return bytes.Equal(s.issuerFP[1:], fp)
	}
	return len(s.issuerID) == 8 && len(fp) == 20 && bytes.Equal(s.issuerID, fp[12:])
}

// parseSig reads a version 4 signature's subpackets (RFC 4880, 5.2.3).
// Only the hashed area counts for the dates; the issuer may be in either.
func parseSig(b []byte) (sig, bool) {
	var s sig
	if len(b) < 6 || b[0] != 4 {
		return s, false
	}
	s.typ = b[1]
	n := int(binary.BigEndian.Uint16(b[4:6]))
	if 6+n+2 > len(b) {
		return s, false
	}
	hashed, rest := b[6:6+n], b[6+n:]
	m := int(binary.BigEndian.Uint16(rest[0:2]))
	if 2+m > len(rest) {
		return s, false
	}
	unhashed := rest[2 : 2+m]
	for i, area := range [][]byte{hashed, unhashed} {
		for len(area) > 0 {
			var l, h int
			switch c := area[0]; {
			case c < 192:
				l, h = int(c), 1
			case c < 255:
				if len(area) < 2 {
					return s, false
				}
				l, h = (int(c)-192)<<8+int(area[1])+192, 2
			default:
				if len(area) < 5 {
					return s, false
				}
				l, h = int(binary.BigEndian.Uint32(area[1:5])), 5
			}
			if l < 1 || h+l > len(area) {
				return s, false
			}
			typ, data := area[h]&0x7f, area[h+1:h+l]
			area = area[h+l:]
			switch {
			case typ == 2 && i == 0 && len(data) == 4:
				s.created = time.Unix(int64(binary.BigEndian.Uint32(data)), 0)
			case typ == 9 && i == 0 && len(data) == 4:
				s.keyExpiry = binary.BigEndian.Uint32(data)
			case typ == 16 && len(data) == 8:
				s.issuerID = data
			case typ == 33 && len(data) == 21:
				s.issuerFP = data
			}
		}
	}
	return s, true
}

const (
	armorBegin = "-----BEGIN PGP PUBLIC KEY BLOCK-----"
	armorEnd   = "-----END PGP PUBLIC KEY BLOCK-----"
)

// Dearmor turns an ASCII-armored public key block (RFC 4880, section 6.2)
// into the binary key file apt reads. Exactly one block is accepted, and its
// checksum, when present, must match.
func Dearmor(b []byte) ([]byte, error) {
	if bytes.Count(b, []byte(armorBegin)) != 1 {
		return nil, errors.New("not one armored public key block")
	}
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 4096), 64<<10)
	const (
		before = iota
		headers
		data
		done
	)
	state := before
	var body strings.Builder
	crc := ""
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), " \t\r")
		switch state {
		case before:
			if line == armorBegin {
				state = headers
			}
		case headers:
			if line == "" {
				state = data
			} else if !strings.Contains(line, ": ") {
				// No headers at all: this line is data already.
				state = data
				body.WriteString(line)
			}
		case data:
			switch {
			case line == armorEnd:
				state = done
			case strings.HasPrefix(line, "="):
				crc = strings.TrimPrefix(line, "=")
			case crc != "":
				return nil, errors.New("data after the armor checksum")
			default:
				body.WriteString(line)
			}
		}
		if state == done {
			break
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if state != done {
		return nil, errors.New("the armored key block is not complete")
	}
	out, err := base64.StdEncoding.DecodeString(body.String())
	if err != nil {
		return nil, fmt.Errorf("the armored key is not valid base64: %w", err)
	}
	if crc != "" {
		want, err := base64.StdEncoding.DecodeString(crc)
		if err != nil || len(want) != 3 {
			return nil, errors.New("the armor checksum is not valid")
		}
		got := crc24(out)
		if byte(got>>16) != want[0] || byte(got>>8) != want[1] || byte(got) != want[2] {
			return nil, errors.New("the armor checksum does not match")
		}
	}
	return out, nil
}

// crc24 is the armor checksum (RFC 4880, section 6.1).
func crc24(b []byte) uint32 {
	crc := uint32(0xB704CE)
	for _, c := range b {
		crc ^= uint32(c) << 16
		for range 8 {
			crc <<= 1
			if crc&0x1000000 != 0 {
				crc ^= 0x1864CFB
			}
		}
	}
	return crc & 0xFFFFFF
}
