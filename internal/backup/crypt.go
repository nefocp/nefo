package backup

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"runtime/debug"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/crypto/scrypt"

	"github.com/nefocp/nefo/internal/usererr"
)

// Encrypted remote copies
//
// A storage can encrypt the backups it receives with a passphrase. The
// copy is then a file of this format (version 1), named like the backup
// with ".enc" added. All integers are big-endian.
//
//	Header, 64 bytes
//	   0   7  magic: the ASCII letters "NEFOENC"
//	   7   1  format version: 1
//	   8   1  key derivation: 1 = scrypt
//	   9   1  scrypt cost: log2(N), 16 by default
//	  10   1  scrypt r: 8
//	  11   1  scrypt p: 1
//	  12   1  cipher: 1 = AES-256-GCM
//	  13   1  log2(chunk size): 16, for chunks of 64 KiB
//	  14   2  zero
//	  16  32  salt: random, new for every file
//	  48  12  key check: bytes 32 to 43 of the scrypt output
//	  60   4  CRC-32 (IEEE) of bytes 0 to 59
//
//	Chunks
//	  scrypt(passphrase, salt, N, r, p) gives 44 bytes: the first 32 are the
//	  AES-256 key, the last 12 the key check above, which tells a wrong
//	  passphrase from a damaged file (the CRC tells a damaged header). The archive is cut into chunks of the
//	  chunk size; the last chunk is shorter, possibly empty, so a file always
//	  has size/chunk + 1 chunks. Each chunk is sealed with AES-256-GCM:
//	    nonce: the chunk's number from 0 (8 bytes), 3 zero bytes, then 1 for
//	           the last chunk and 0 for the others
//	    additional data: the 64-byte header
//	  and written as ciphertext followed by the 16-byte tag. A file cut at a
//	  chunk boundary lacks its last chunk (the one marked last), so a
//	  truncated file is detected, as are reordered, changed or added chunks.
//
// The file's size is 64 + size + 16 × (size/chunk + 1). Nothing but the
// passphrase is needed to decrypt it: no key is kept on the server, so a
// copy can be restored on any server, or with "nefo decrypt-backup".

const (
	encMagic      = "NEFOENC"
	encVersion    = 1
	encHeaderSize = 64
	encCheckSize  = 12 // of the key check, at offset 48
	encTagSize    = 16
	encSuffix     = ".enc" // added to the remote key of an encrypted copy

	encKDFScrypt = 1
	encAEADGCM   = 1

	// Defaults for new files: scrypt with N = 2^16 needs 64 MiB for a
	// moment (128 × N × r bytes), about a quarter of a second.
	encLogN      = 16
	encR         = 8
	encP         = 1
	encLogChunk  = 16
	encMaxLogN   = 20 // a file asking for more (1 GiB) is refused
	encMaxMemory = 1 << 30

	// Passphrase limits for new settings.
	minPassphrase = 12
	maxPassphrase = 1024
)

// Errors of decryption. Their messages are read in job logs.
var (
	ErrWrongPassphrase = errors.New("this copy is encrypted and the passphrase does not unlock it. " +
		"If the storage's passphrase was changed after the upload, enter the old one when you restore")
	ErrNeedPassphrase = errors.New("this copy is encrypted: enter its passphrase when you restore")
	ErrDamaged        = errors.New("the encrypted copy is damaged or was changed: a part of it does not decrypt")
	ErrTruncated      = errors.New("the encrypted copy is incomplete: the file ends before its last part")
)

// Encryption is the part of a storage's settings that says whether copies
// are encrypted before they are uploaded.
type Encryption struct {
	Encrypt bool `json:"encrypt"`
	// EncryptPassphrase encrypts new uploads. It is never sent to the UI.
	EncryptPassphrase string `json:"encrypt_passphrase,omitempty"`
	// PreviousPassphrases were used before the passphrase was changed (or
	// encryption turned off): files uploaded then keep them, so restores on
	// this server try them too. Newest first.
	PreviousPassphrases []string `json:"previous_passphrases,omitempty"`
}

const maxPreviousPassphrases = 10

// mergeEncryption fills the new settings from the stored ones: an empty
// passphrase keeps the stored one, and a passphrase that is replaced or no
// longer used is remembered for restores.
func mergeEncryption(n *Encryption, old Encryption) error {
	n.PreviousPassphrases = old.PreviousPassphrases
	if n.Encrypt && n.EncryptPassphrase == "" && old.Encrypt {
		n.EncryptPassphrase = old.EncryptPassphrase
	}
	if !n.Encrypt {
		n.EncryptPassphrase = ""
	}
	if n.Encrypt {
		p := n.EncryptPassphrase
		if p == "" {
			return usererr.New("backup_passphrase_missing")
		}
		if utf8.RuneCountInString(p) < minPassphrase || len(p) > maxPassphrase || !utf8.ValidString(p) ||
			strings.ContainsAny(p, "\x00\r\n") {
			return usererr.New("backup_passphrase_invalid")
		}
	}
	if old.Encrypt && old.EncryptPassphrase != "" && old.EncryptPassphrase != n.EncryptPassphrase {
		n.PreviousPassphrases = append([]string{old.EncryptPassphrase}, n.PreviousPassphrases...)
	}
	// The current one need not be in the list, and each one only once.
	var prev []string
	for _, p := range n.PreviousPassphrases {
		if p != "" && p != n.EncryptPassphrase && !containsString(prev, p) && len(prev) < maxPreviousPassphrases {
			prev = append(prev, p)
		}
	}
	n.PreviousPassphrases = prev
	return nil
}

// passphrases lists what to try on an encrypted copy: the one given for
// this restore first, then the storage's current and earlier ones.
func (e Encryption) passphrases(given string) []string {
	var out []string
	for _, p := range append([]string{given, e.EncryptPassphrase}, e.PreviousPassphrases...) {
		if p != "" && !containsString(out, p) {
			out = append(out, p)
		}
	}
	return out
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// encParams are the header fields that choose the algorithms.
type encParams struct {
	logN, r, p, logChunk int
}

var defaultEncParams = encParams{encLogN, encR, encP, encLogChunk}

func (pr encParams) chunk() int { return 1 << pr.logChunk }

// encryptedSize is the size of the encrypted copy of size bytes.
func encryptedSize(size int64, logChunk int) int64 {
	return encHeaderSize + size + encTagSize*(size>>logChunk+1)
}

// deriveKey runs scrypt and returns the AES key and the key check.
func deriveKey(passphrase string, salt []byte, pr encParams) (key, check []byte, err error) {
	dk, err := scrypt.Key([]byte(passphrase), salt, 1<<pr.logN, pr.r, pr.p, 32+encCheckSize)
	// scrypt's work memory is garbage now: give it back to the system
	// at once, so the panel's memory use does not stay up after a backup.
	debug.FreeOSMemory()
	if err != nil {
		return nil, nil, err
	}
	return dk[:32], dk[32:], nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	b, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(b)
}

func chunkNonce(i uint64, last bool) []byte {
	n := make([]byte, 12)
	binary.BigEndian.PutUint64(n, i)
	if last {
		n[11] = 1
	}
	return n
}

// parseHeader checks a header's fixed fields and returns its parameters.
func parseHeader(h []byte) (encParams, error) {
	if len(h) < encHeaderSize || string(h[:7]) != encMagic {
		return encParams{}, errors.New("not an encrypted backup")
	}
	if crc32.ChecksumIEEE(h[:60]) != binary.BigEndian.Uint32(h[60:64]) {
		return encParams{}, errors.New("the header of the encrypted copy is damaged")
	}
	if h[7] != encVersion {
		return encParams{}, fmt.Errorf("the encrypted copy has format version %d, which this version of the panel cannot read: update the panel", h[7])
	}
	pr := encParams{logN: int(h[9]), r: int(h[10]), p: int(h[11]), logChunk: int(h[13])}
	if h[8] != encKDFScrypt || h[12] != encAEADGCM || h[14] != 0 || h[15] != 0 ||
		pr.logN < 10 || pr.logN > encMaxLogN || pr.r < 1 || pr.r > 32 || pr.p < 1 || pr.p > 16 ||
		int64(128)<<pr.logN*int64(pr.r) > encMaxMemory || pr.logChunk < 10 || pr.logChunk > 24 {
		return encParams{}, errors.New("the header of the encrypted copy is damaged or uses settings this panel does not know")
	}
	return pr, nil
}

// ---- encrypting ----

// encReader is the encrypted copy of a plaintext of known size, readable at
// any offset: an interrupted upload resumes from where it stopped, and an
// upload that needs the copy's checksum first reads it twice. Encryption is
// deterministic once the salt is chosen, so every read gives the same bytes.
type encReader struct {
	src    io.ReaderAt
	plain  int64
	size   int64
	header []byte
	aead   cipher.AEAD
	chunk  int

	mu     sync.Mutex
	cached int64 // number of the chunk in sealed, -1 for none
	sealed []byte
	pbuf   []byte
}

// newEncReader derives the key (a quarter of a second) and returns the
// encrypted view of src.
func newEncReader(src io.ReaderAt, size int64, passphrase string) (*encReader, error) {
	return newEncReaderParams(src, size, passphrase, defaultEncParams)
}

func newEncReaderParams(src io.ReaderAt, size int64, passphrase string, pr encParams) (*encReader, error) {
	if passphrase == "" {
		return nil, errors.New("no encryption passphrase is set")
	}
	h := make([]byte, encHeaderSize)
	copy(h, encMagic)
	h[7], h[8], h[9], h[10], h[11], h[12], h[13] = encVersion, encKDFScrypt, byte(pr.logN), byte(pr.r), byte(pr.p), encAEADGCM, byte(pr.logChunk)
	if _, err := rand.Read(h[16:48]); err != nil {
		return nil, err
	}
	key, check, err := deriveKey(passphrase, h[16:48], pr)
	if err != nil {
		return nil, err
	}
	copy(h[48:60], check)
	binary.BigEndian.PutUint32(h[60:64], crc32.ChecksumIEEE(h[:60]))
	aead, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	c := pr.chunk()
	return &encReader{src: src, plain: size, size: encryptedSize(size, pr.logChunk), header: h, aead: aead,
		chunk: c, cached: -1, pbuf: make([]byte, c), sealed: make([]byte, 0, c+encTagSize)}, nil
}

// Size is the size of the encrypted copy.
func (e *encReader) Size() int64 { return e.size }

// sealChunk encrypts chunk i into e.sealed. The caller holds e.mu.
func (e *encReader) sealChunk(i int64) error {
	if e.cached == i {
		return nil
	}
	last := i == e.plain/int64(e.chunk)
	n := e.chunk
	if last {
		n = int(e.plain % int64(e.chunk))
	}
	if n > 0 {
		got, err := e.src.ReadAt(e.pbuf[:n], i*int64(e.chunk))
		if got < n {
			if err == nil || errors.Is(err, io.EOF) {
				err = errors.New("the backup file is shorter than recorded: it changed after it was made")
			}
			e.cached = -1
			return err
		}
	}
	e.sealed = e.aead.Seal(e.sealed[:0], chunkNonce(uint64(i), last), e.pbuf[:n], e.header)
	e.cached = i
	return nil
}

// ReadAt reads the encrypted copy at off.
func (e *encReader) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, errors.New("negative offset")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for n < len(p) {
		pos := off + int64(n)
		if pos >= e.size {
			return n, io.EOF
		}
		if pos < encHeaderSize {
			n += copy(p[n:], e.header[pos:])
			continue
		}
		step := int64(e.chunk + encTagSize)
		i := (pos - encHeaderSize) / step
		if err := e.sealChunk(i); err != nil {
			return n, err
		}
		n += copy(p[n:], e.sealed[(pos-encHeaderSize)-i*step:])
	}
	return n, nil
}

// ---- decrypting ----

// decryptWriter decrypts an encrypted copy written to it and writes the
// plaintext to w. Close checks that the copy was complete.
type decryptWriter struct {
	w           io.Writer
	passphrases []string
	header      []byte
	aead        cipher.AEAD
	chunk       int
	buf         []byte
	next        uint64
	err         error
}

func newDecryptWriter(w io.Writer, passphrases []string) *decryptWriter {
	return &decryptWriter{w: w, passphrases: passphrases}
}

func (d *decryptWriter) fail(err error) error {
	if d.err == nil {
		d.err = err
	}
	return d.err
}

func (d *decryptWriter) Write(p []byte) (int, error) {
	if d.err != nil {
		return 0, d.err
	}
	d.buf = append(d.buf, p...)
	if d.aead == nil {
		if len(d.buf) < encHeaderSize {
			return len(p), nil
		}
		if err := d.start(); err != nil {
			return 0, d.fail(err)
		}
	}
	// A full chunk is decrypted once more data follows it: only then is it
	// known not to be the last one.
	step := d.chunk + encTagSize
	off := 0
	for len(d.buf)-off > step {
		if err := d.open(d.buf[off:off+step], false); err != nil {
			return 0, d.fail(err)
		}
		off += step
	}
	if off > 0 {
		d.buf = append(d.buf[:0], d.buf[off:]...)
	}
	return len(p), nil
}

// start reads the header and finds the passphrase that unlocks the copy.
func (d *decryptWriter) start() error {
	pr, err := parseHeader(d.buf)
	if err != nil {
		return err
	}
	h := append([]byte(nil), d.buf[:encHeaderSize]...)
	d.chunk = pr.chunk() // known even when no passphrase fits: it gives the archive's size
	if len(d.passphrases) == 0 {
		return ErrNeedPassphrase
	}
	for _, pass := range d.passphrases {
		key, check, err := deriveKey(pass, h[16:48], pr)
		if err != nil {
			return err
		}
		if subtle.ConstantTimeCompare(check, h[48:60]) == 1 {
			if d.aead, err = newGCM(key); err != nil {
				return err
			}
			d.header = h
			d.buf = append(d.buf[:0], d.buf[encHeaderSize:]...)
			return nil
		}
	}
	return ErrWrongPassphrase
}

func (d *decryptWriter) open(sealed []byte, last bool) error {
	plain, err := d.aead.Open(nil, chunkNonce(d.next, last), sealed, d.header)
	if err != nil {
		return ErrDamaged
	}
	d.next++
	if len(plain) > 0 {
		if _, err := d.w.Write(plain); err != nil {
			return err
		}
	}
	return nil
}

// Close decrypts the last chunk. An error means the copy is unusable.
func (d *decryptWriter) Close() error {
	if d.err != nil {
		return d.err
	}
	if d.aead == nil {
		if len(d.buf) >= encHeaderSize { // never: Write starts as soon as the header is there
			return d.fail(errors.New("internal error: header not read"))
		}
		return d.fail(ErrTruncated)
	}
	// The last chunk is shorter than a full one; a full one at the end
	// means the last chunk is missing.
	if len(d.buf) < encTagSize || len(d.buf) >= d.chunk+encTagSize {
		return d.fail(ErrTruncated)
	}
	if err := d.open(d.buf, true); err != nil {
		return d.fail(err)
	}
	d.buf = nil
	d.err = errors.New("already closed")
	return nil
}

// restoreWriter writes a downloaded copy to w: as it is when it is a plain
// archive, decrypted when it starts with the header of an encrypted copy.
type restoreWriter struct {
	w           io.Writer
	passphrases []string
	head        []byte
	dec         *decryptWriter // set once the copy is known to be encrypted
	plain       bool
	err         error
}

func newRestoreWriter(w io.Writer, passphrases []string) *restoreWriter {
	return &restoreWriter{w: w, passphrases: passphrases}
}

// Encrypted tells whether the copy was encrypted (once 7 bytes were written).
func (r *restoreWriter) Encrypted() bool { return r.dec != nil }

func (r *restoreWriter) Write(p []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	switch {
	case r.dec != nil:
		n, err := r.dec.Write(p)
		r.err = err
		return n, err
	case r.plain:
		n, err := r.w.Write(p)
		r.err = err
		return n, err
	}
	need := len(encMagic) - len(r.head)
	if len(p) < need {
		r.head = append(r.head, p...)
		return len(p), nil
	}
	r.head = append(r.head, p[:need]...)
	if err := r.decide(); err != nil {
		return 0, err
	}
	if _, err := r.Write(p[need:]); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (r *restoreWriter) decide() error {
	head := r.head
	r.head = nil
	if bytes.Equal(head, []byte(encMagic)) {
		r.dec = newDecryptWriter(r.w, r.passphrases)
		_, r.err = r.dec.Write(head)
		return r.err
	}
	r.plain = true
	_, r.err = r.w.Write(head)
	return r.err
}

// Close finishes the copy: for an encrypted one, it checks that nothing is
// missing at the end.
func (r *restoreWriter) Close() error {
	if r.err != nil {
		return r.err
	}
	if r.dec == nil && !r.plain {
		if err := r.decide(); err != nil {
			return err
		}
	}
	if r.dec != nil {
		r.err = r.dec.Close()
		return r.err
	}
	return nil
}

// DecryptBackup decrypts an encrypted copy read from r into w; a plain
// archive is copied as it is. It reports whether r was encrypted.
func DecryptBackup(r io.Reader, w io.Writer, passphrase string) (encrypted bool, err error) {
	var pass []string
	if passphrase != "" {
		pass = []string{passphrase}
	}
	rw := newRestoreWriter(w, pass)
	if _, err := io.Copy(rw, r); err != nil {
		if rw.err != nil {
			return rw.Encrypted(), rw.err
		}
		return rw.Encrypted(), err
	}
	return rw.Encrypted(), rw.Close()
}
