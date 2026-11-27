package backup

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	mrand "math/rand/v2"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/usererr"
)

// Cheap parameters for tests: scrypt N = 2^10, chunks of 1 KiB.
var testEncParams = encParams{logN: 10, r: 8, p: 1, logChunk: 10}

func encryptAll(t *testing.T, plain []byte, pass string) []byte {
	t.Helper()
	e, err := newEncReaderParams(bytes.NewReader(plain), int64(len(plain)), pass, testEncParams)
	if err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(io.NewSectionReader(e, 0, e.Size()))
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(out)) != e.Size() || e.Size() != encryptedSize(int64(len(plain)), testEncParams.logChunk) {
		t.Fatalf("size %d, Size() %d", len(out), e.Size())
	}
	return out
}

// decryptAll writes sealed to a restoreWriter in pieces of random sizes.
func decryptAll(sealed []byte, pass ...string) ([]byte, error) {
	var out bytes.Buffer
	rw := newRestoreWriter(&out, pass)
	for len(sealed) > 0 {
		n := min(len(sealed), 1+mrand.IntN(3000))
		if _, err := rw.Write(sealed[:n]); err != nil {
			return nil, err
		}
		sealed = sealed[n:]
	}
	if err := rw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	rand.Read(b)
	return b
}

func TestEncryptDecryptSizes(t *testing.T) {
	c := testEncParams.chunk()
	for _, n := range []int{0, 1, c - 1, c, c + 1, 3 * c, 3*c + 17, 10_000} {
		plain := randomBytes(n)
		sealed := encryptAll(t, plain, "correct horse battery")
		if !bytes.HasPrefix(sealed, []byte("NEFOENC\x01")) {
			t.Fatalf("%d: header %q", n, sealed[:8])
		}
		if n > 64 && bytes.Contains(sealed, plain[:64]) {
			t.Fatalf("%d: plaintext visible in the encrypted copy", n)
		}
		got, err := decryptAll(sealed, "wrong one", "correct horse battery")
		if err != nil || !bytes.Equal(got, plain) {
			t.Fatalf("%d bytes: %v (got %d bytes)", n, err, len(got))
		}
		// Byte by byte, as a slow download would.
		var out bytes.Buffer
		rw := newRestoreWriter(&out, []string{"correct horse battery"})
		for i := range sealed {
			if _, err := rw.Write(sealed[i : i+1]); err != nil {
				t.Fatalf("%d: byte %d: %v", n, i, err)
			}
		}
		if err := rw.Close(); err != nil || !bytes.Equal(out.Bytes(), plain) || !rw.Encrypted() {
			t.Fatalf("%d bytes one at a time: %v", n, err)
		}
	}
}

func TestEncReaderAtAnyOffset(t *testing.T) {
	plain := randomBytes(5000)
	e, err := newEncReaderParams(bytes.NewReader(plain), int64(len(plain)), "correct horse battery", testEncParams)
	if err != nil {
		t.Fatal(err)
	}
	whole := make([]byte, e.Size())
	if n, err := e.ReadAt(whole, 0); n != len(whole) || err != nil && err != io.EOF {
		t.Fatal(n, err)
	}
	for i := 0; i < 300; i++ {
		off := mrand.Int64N(e.Size())
		n := 1 + mrand.IntN(2500)
		buf := make([]byte, n)
		got, err := e.ReadAt(buf, off)
		want := whole[off:min(off+int64(n), e.Size())]
		if got != len(want) || !bytes.Equal(buf[:got], want) {
			t.Fatalf("ReadAt(%d, %d) = %d %v", n, off, got, err)
		}
		if got < n && err != io.EOF {
			t.Fatalf("short read without EOF: %v", err)
		}
	}
	// A backup file that shrank is an error, not a short copy.
	e2, _ := newEncReaderParams(bytes.NewReader(plain[:3000]), int64(len(plain)), "correct horse battery", testEncParams)
	if _, err := io.ReadAll(io.NewSectionReader(e2, 0, e2.Size())); err == nil || !strings.Contains(err.Error(), "shorter") {
		t.Fatalf("shrunk file: %v", err)
	}
}

func TestDecryptErrors(t *testing.T) {
	plain := randomBytes(4000) // 3 full chunks and a short one
	sealed := encryptAll(t, plain, "correct horse battery")
	step := testEncParams.chunk() + encTagSize

	if _, err := decryptAll(sealed, "wrong passphrase!"); !errors.Is(err, ErrWrongPassphrase) {
		t.Errorf("wrong passphrase: %v", err)
	}
	if _, err := decryptAll(sealed); !errors.Is(err, ErrNeedPassphrase) {
		t.Errorf("no passphrase: %v", err)
	}

	flip := func(i int) []byte {
		b := bytes.Clone(sealed)
		b[i] ^= 0x40
		return b
	}
	if _, err := decryptAll(flip(encHeaderSize+step+100), "correct horse battery"); !errors.Is(err, ErrDamaged) {
		t.Errorf("changed byte in a chunk: %v", err)
	}
	if _, err := decryptAll(flip(len(sealed)-1), "correct horse battery"); !errors.Is(err, ErrDamaged) {
		t.Errorf("changed tag of the last chunk: %v", err)
	}
	if _, err := decryptAll(flip(20), "correct horse battery"); err == nil || !strings.Contains(err.Error(), "header") {
		t.Errorf("changed salt: %v", err)
	}
	// A header changed with its CRC fixed: the key check fails, as for a
	// wrong passphrase; a changed field the CRC covers but nobody signs
	// is still caught by the chunks' additional data.
	b := flip(50)
	binary.BigEndian.PutUint32(b[60:], crc32.ChecksumIEEE(b[:60]))
	if _, err := decryptAll(b, "correct horse battery"); !errors.Is(err, ErrWrongPassphrase) {
		t.Errorf("changed key check: %v", err)
	}

	// Cut at a chunk boundary: the last chunk is missing.
	for _, n := range []int{encHeaderSize + step, encHeaderSize + 2*step, encHeaderSize + 3*step} {
		if _, err := decryptAll(sealed[:n], "correct horse battery"); !errors.Is(err, ErrTruncated) {
			t.Errorf("cut at %d: %v", n, err)
		}
	}
	// Cut inside a chunk, or inside the header.
	if _, err := decryptAll(sealed[:encHeaderSize+step+500], "correct horse battery"); !errors.Is(err, ErrDamaged) {
		t.Errorf("cut inside a chunk: %v", err)
	}
	if _, err := decryptAll(sealed[:40], "correct horse battery"); !errors.Is(err, ErrTruncated) {
		t.Errorf("cut inside the header: %v", err)
	}
	if _, err := decryptAll(sealed[:encHeaderSize+5], "correct horse battery"); !errors.Is(err, ErrTruncated) {
		t.Errorf("cut after the header: %v", err)
	}
	// Data added at the end.
	if _, err := decryptAll(append(bytes.Clone(sealed), 0), "correct horse battery"); !errors.Is(err, ErrDamaged) {
		t.Errorf("byte added: %v", err)
	}
	// Two chunks swapped.
	sw := bytes.Clone(sealed)
	copy(sw[encHeaderSize:], sealed[encHeaderSize+step:encHeaderSize+2*step])
	copy(sw[encHeaderSize+step:], sealed[encHeaderSize:encHeaderSize+step])
	if _, err := decryptAll(sw, "correct horse battery"); !errors.Is(err, ErrDamaged) {
		t.Errorf("chunks swapped: %v", err)
	}
	// A chunk of another file with the same passphrase.
	other := encryptAll(t, plain, "correct horse battery")
	mix := bytes.Clone(sealed)
	copy(mix[encHeaderSize:], other[encHeaderSize:encHeaderSize+step])
	if _, err := decryptAll(mix, "correct horse battery"); !errors.Is(err, ErrDamaged) {
		t.Errorf("chunk from another file: %v", err)
	}

	// A header asking for too much memory is refused before scrypt runs.
	big := bytes.Clone(sealed)
	big[9] = 30
	binary.BigEndian.PutUint32(big[60:], crc32.ChecksumIEEE(big[:60]))
	if _, err := decryptAll(big, "correct horse battery"); err == nil || !strings.Contains(err.Error(), "settings") {
		t.Errorf("huge scrypt cost: %v", err)
	}
	newer := bytes.Clone(sealed)
	newer[7] = 2
	binary.BigEndian.PutUint32(newer[60:], crc32.ChecksumIEEE(newer[:60]))
	if _, err := decryptAll(newer, "correct horse battery"); err == nil || !strings.Contains(err.Error(), "update the panel") {
		t.Errorf("format version 2: %v", err)
	}
}

func TestRestoreWriterPassesPlainArchives(t *testing.T) {
	for _, plain := range [][]byte{
		{0x1f, 0x8b, 8, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		[]byte("NEF"), // shorter than the magic
		{},
		[]byte("NEFOENX and more"),
	} {
		got, err := decryptAll(plain)
		if err != nil || !bytes.Equal(got, plain) {
			t.Errorf("%q: %q %v", plain, got, err)
		}
	}
}

func TestDecryptBackupDefaultParams(t *testing.T) {
	plain := randomBytes(200_000)
	e, err := newEncReader(bytes.NewReader(plain), int64(len(plain)), "a long enough passphrase")
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	enc, err := DecryptBackup(io.NewSectionReader(e, 0, e.Size()), &out, "a long enough passphrase")
	if err != nil || !enc || !bytes.Equal(out.Bytes(), plain) {
		t.Fatalf("encrypted %v, err %v", enc, err)
	}
	out.Reset()
	if enc, err := DecryptBackup(bytes.NewReader(plain), &out, ""); err != nil || enc || !bytes.Equal(out.Bytes(), plain) {
		t.Fatalf("plain: %v %v", enc, err)
	}
	if _, err := DecryptBackup(io.NewSectionReader(e, 0, e.Size()), io.Discard, "another passphrase"); !errors.Is(err, ErrWrongPassphrase) {
		t.Fatalf("wrong passphrase: %v", err)
	}
}

func TestMergeEncryption(t *testing.T) {
	code := func(err error) string {
		var ue *usererr.Error
		if errors.As(err, &ue) {
			return ue.Code
		}
		return ""
	}
	n := Encryption{Encrypt: true}
	if err := mergeEncryption(&n, Encryption{}); code(err) != "backup_passphrase_missing" {
		t.Fatalf("no passphrase: %v", err)
	}
	n = Encryption{Encrypt: true, EncryptPassphrase: "short"}
	if err := mergeEncryption(&n, Encryption{}); code(err) != "backup_passphrase_invalid" {
		t.Fatalf("short: %v", err)
	}
	n = Encryption{Encrypt: true, EncryptPassphrase: "twelve chars\nwith newline"}
	if err := mergeEncryption(&n, Encryption{}); code(err) != "backup_passphrase_invalid" {
		t.Fatalf("newline: %v", err)
	}
	// Twelve characters, not bytes: Vietnamese passphrases count right.
	n = Encryption{Encrypt: true, EncryptPassphrase: "mật khẩu dài"}
	if err := mergeEncryption(&n, Encryption{}); err != nil {
		t.Fatalf("12 runes: %v", err)
	}

	old := Encryption{Encrypt: true, EncryptPassphrase: "first passphrase"}
	n = Encryption{Encrypt: true}
	if err := mergeEncryption(&n, old); err != nil || n.EncryptPassphrase != "first passphrase" || len(n.PreviousPassphrases) != 0 {
		t.Fatalf("kept: %+v %v", n, err)
	}
	n = Encryption{Encrypt: true, EncryptPassphrase: "second passphrase", PreviousPassphrases: []string{"injected"}}
	if err := mergeEncryption(&n, old); err != nil || n.EncryptPassphrase != "second passphrase" ||
		len(n.PreviousPassphrases) != 1 || n.PreviousPassphrases[0] != "first passphrase" {
		t.Fatalf("changed: %+v %v", n, err)
	}
	if got := n.passphrases("given one"); strings.Join(got, ",") != "given one,second passphrase,first passphrase" {
		t.Fatalf("passphrases = %v", got)
	}
	// Turned off: the passphrase is kept for restores only.
	off := Encryption{Encrypt: false, EncryptPassphrase: "ignored"}
	if err := mergeEncryption(&off, n); err != nil || off.EncryptPassphrase != "" ||
		strings.Join(off.PreviousPassphrases, ",") != "second passphrase,first passphrase" {
		t.Fatalf("off: %+v %v", off, err)
	}
	// Back on with the first one again: it is no longer a previous one.
	on := Encryption{Encrypt: true, EncryptPassphrase: "first passphrase"}
	if err := mergeEncryption(&on, off); err != nil || strings.Join(on.PreviousPassphrases, ",") != "second passphrase" {
		t.Fatalf("on again: %+v %v", on, err)
	}
}
