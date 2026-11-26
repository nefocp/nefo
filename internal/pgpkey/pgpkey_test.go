package pgpkey

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mysqlFingerprint is the MySQL Release Engineering key in
// testdata/RPM-GPG-KEY-mysql-2025 (https://repo.mysql.com/RPM-GPG-KEY-mysql-2025).
const mysqlFingerprint = "BCA43417C3B485DD128EC6D4B7B3B788A8D3785C"

func TestDearmorAndFingerprint(t *testing.T) {
	armored, err := os.ReadFile(filepath.Join("testdata", "RPM-GPG-KEY-mysql-2025"))
	if err != nil {
		t.Fatal(err)
	}
	key, err := Dearmor(armored)
	if err != nil {
		t.Fatal(err)
	}
	fp, err := Fingerprint(key)
	if err != nil || fp != mysqlFingerprint {
		t.Fatalf("fingerprint %s, %v", fp, err)
	}
	// CRLF line ends are fine too.
	if k2, err := Dearmor(bytes.ReplaceAll(armored, []byte("\n"), []byte("\r\n"))); err != nil || !bytes.Equal(k2, key) {
		t.Fatalf("CRLF: %v", err)
	}
	// The binary key itself is not armored.
	if _, err := Dearmor(key); err == nil {
		t.Error("binary key accepted as armored")
	}
}

func TestDearmorRefusesDamage(t *testing.T) {
	armored, err := os.ReadFile(filepath.Join("testdata", "RPM-GPG-KEY-mysql-2025"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(armored)
	lines := strings.Split(s, "\n")
	// Change one character of the base64 data: the checksum no longer matches.
	flip := lines[3]
	c := byte('A')
	if flip[10] == 'A' {
		c = 'B'
	}
	lines[3] = flip[:10] + string(c) + flip[11:]
	for name, bad := range map[string]string{
		"checksum":  strings.Join(lines, "\n"),
		"two keys":  s + s,
		"truncated": s[:len(s)/2],
		"no armor":  "hello",
		"no end":    strings.Replace(s, armorEnd, "", 1),
	} {
		if _, err := Dearmor([]byte(bad)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestFingerprintRefuses(t *testing.T) {
	armored, _ := os.ReadFile(filepath.Join("testdata", "RPM-GPG-KEY-mysql-2025"))
	key, err := Dearmor(armored)
	if err != nil {
		t.Fatal(err)
	}
	for name, bad := range map[string][]byte{
		"two keys":  append(append([]byte{}, key...), key...),
		"armored":   []byte(armorBegin + "\n"),
		"truncated": key[:len(key)-10],
		"secret":    {0x95, 0x00, 0x01, 0x04},
		"empty":     nil,
	} {
		if _, err := Fingerprint(bad); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestCRC24(t *testing.T) {
	// The CRC-24 of the empty input is its initial value.
	if got := crc24(nil); got != 0xB704CE {
		t.Fatalf("crc24(nil) = %x", got)
	}
}

func TestExpiry(t *testing.T) {
	for file, want := range map[string]string{
		// The same key before and after Oracle renewed it in October 2025.
		"RPM-GPG-KEY-mysql-2023": "2025-10-22",
		"RPM-GPG-KEY-mysql-2025": "2027-10-23",
	} {
		armored, err := os.ReadFile(filepath.Join("testdata", file))
		if err != nil {
			t.Fatal(err)
		}
		key, err := Dearmor(armored)
		if err != nil {
			t.Fatal(err)
		}
		exp, never, err := Expiry(key)
		if err != nil || never || exp.UTC().Format("2006-01-02") != want {
			t.Errorf("%s: %v %v %v, want %s", file, exp, never, err, want)
		}
	}
	if _, _, err := Expiry([]byte("junk")); err == nil {
		t.Error("junk accepted")
	}
}
