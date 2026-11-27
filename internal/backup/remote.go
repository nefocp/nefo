package backup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// Kinds of remote storage. S3, SFTP and Google Drive can all be set up at
// the same time: "also upload to remote storage" then copies a backup to
// each of them.
const (
	RemoteS3    = "s3"
	RemoteSFTP  = "sftp"
	RemoteDrive = "gdrive"
)

// remoteNames lists every kind of remote storage, in upload order.
var remoteNames = []string{RemoteS3, RemoteSFTP, RemoteDrive}

// RemoteNames lists every kind of remote storage.
func RemoteNames() []string { return append([]string(nil), remoteNames...) }

// remoteSettings are the settings that hold each storage's configuration.
var remoteSettings = map[string]string{RemoteS3: s3SettingKey, RemoteSFTP: sftpSettingKey, RemoteDrive: driveSettingKey}

// remote is a place backups are copied to.
type remote interface {
	// Put stores a file whose size and SHA-256 are known.
	Put(ctx context.Context, key string, r io.Reader, size int64, sha256hex string) error
	// Get writes a stored file to w.
	Get(ctx context.Context, key string, w io.Writer) error
	// Delete removes a stored file. A file that is already gone is not an error.
	Delete(ctx context.Context, key string) error
	// Describe names where key is stored, for job logs.
	Describe(key string) string
	Close() error
}

// wantsSHA256 is a remote that needs the checksum of what it receives before
// the upload starts (S3 signs it). An encrypted copy then needs a first
// pass to compute it.
type wantsSHA256 interface {
	needsSHA256() bool
}

// lister is a remote that can list the files under a prefix.
type lister interface {
	List(ctx context.Context, prefix string) ([]RemoteFile, error)
}

// RemoteFile is a file in remote storage.
type RemoteFile struct {
	Name    string
	Size    int64
	ModTime time.Time
}

// location is a copy of a backup in one remote storage.
type location struct {
	remote string
	key    string
}

// The remote_key column lists every copy of a backup, separated by spaces.
// S3 keys are stored bare, as they were before SFTP existed; SFTP and
// Google Drive keys get the prefix "sftp:" or "gdrive:". Keys never contain
// spaces: they are made of the backup kind, a domain, database name or
// folder slug, and a generated file name, with ".enc" at the end for an
// encrypted copy.
func parseLocations(s string) []location {
	var out []location
	for _, f := range strings.Fields(s) {
		if k, ok := strings.CutPrefix(f, RemoteSFTP+":"); ok {
			out = append(out, location{RemoteSFTP, k})
		} else if k, ok := strings.CutPrefix(f, RemoteDrive+":"); ok {
			out = append(out, location{RemoteDrive, k})
		} else {
			out = append(out, location{RemoteS3, strings.TrimPrefix(f, RemoteS3+":")})
		}
	}
	return out
}

// encrypted tells whether the copy was encrypted before upload.
func (l location) encrypted() bool { return strings.HasSuffix(l.key, encSuffix) }

func formatLocations(locs []location) string {
	parts := make([]string, 0, len(locs))
	for _, l := range locs {
		if l.remote == RemoteS3 {
			parts = append(parts, l.key)
		} else {
			parts = append(parts, l.remote+":"+l.key)
		}
	}
	return strings.Join(parts, " ")
}

func hasLocation(locs []location, name string) bool {
	for _, l := range locs {
		if l.remote == name {
			return true
		}
	}
	return false
}

// Remotes lists the remote storages that hold a copy of b ("s3", "sftp",
// "gdrive").
func Remotes(b *store.Backup) []string {
	out := []string{}
	for _, l := range parseLocations(b.RemoteKey) {
		out = append(out, l.remote)
	}
	return out
}

// EncryptedRemotes lists the remote storages whose copy of b is encrypted.
func EncryptedRemotes(b *store.Backup) []string {
	out := []string{}
	for _, l := range parseLocations(b.RemoteKey) {
		if l.encrypted() {
			out = append(out, l.remote)
		}
	}
	return out
}

// RemoteLabel names a kind of storage for people: "S3", "SFTP", "Google Drive".
func RemoteLabel(name string) string { return remoteLabel(name) }

func remoteLabel(name string) string {
	switch name {
	case RemoteSFTP:
		return "SFTP"
	case RemoteDrive:
		return "Google Drive"
	}
	return "S3"
}

// remoteKey is where a backup is stored in remote storage, relative to the
// bucket prefix or the SFTP folder.
func remoteKey(b *store.Backup) string {
	return b.Kind + "/" + storageDir(b.Kind, b.Target) + "/" + filepath.Base(b.File)
}

// ConfiguredRemotes lists the remote storages that are set up.
func (m *Manager) ConfiguredRemotes(ctx context.Context) ([]string, error) {
	out := []string{}
	for _, name := range remoteNames {
		v, err := m.store.Setting(ctx, remoteSettings[name])
		if err != nil {
			return nil, err
		}
		if v != "" {
			out = append(out, name)
		}
	}
	return out, nil
}

// newS3Remote makes the S3 client; tests replace it.
var newS3Remote = func(c S3Config) remote { return newS3(c) }

// openRemote connects to a remote storage and returns its encryption
// settings. It returns a nil remote when that storage is not set up.
func (m *Manager) openRemote(ctx context.Context, name string) (remote, Encryption, error) {
	switch name {
	case RemoteS3:
		c, err := m.storageWithSecret(ctx)
		if err != nil || c == nil {
			return nil, Encryption{}, err
		}
		return newS3Remote(*c), c.Encryption, nil
	case RemoteSFTP:
		c, err := m.sftpWithSecret(ctx)
		if err != nil || c == nil {
			return nil, Encryption{}, err
		}
		cl, err := dialSFTP(ctx, *c, false)
		if err != nil {
			return nil, Encryption{}, err
		}
		return cl, c.Encryption, nil
	case RemoteDrive:
		c, err := m.driveWithSecret(ctx)
		if err != nil || c == nil {
			return nil, Encryption{}, err
		}
		return newDriveClient(*c), c.Encryption, nil
	}
	return nil, Encryption{}, fmt.Errorf("unknown remote storage %q", name)
}

// upload copies a backup to every configured remote storage that does not
// have it yet. A failure on one storage does not stop the others.
func (m *Manager) upload(ctx context.Context, b *store.Backup, lg Logger) error {
	names, err := m.ConfiguredRemotes(ctx)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return usererr.New("storage_not_configured")
	}
	locs := parseLocations(b.RemoteKey)
	key := remoteKey(b)
	var errs []error
	done := 0
	for _, name := range names {
		if hasLocation(locs, name) {
			continue
		}
		stored, err := m.uploadTo(ctx, name, b, key, lg)
		if err != nil {
			lg.Logf("Upload to %s failed: %v", remoteLabel(name), err)
			errs = append(errs, fmt.Errorf("%s: %w", remoteLabel(name), err))
			continue
		}
		locs = append(locs, location{name, stored})
		b.RemoteKey = formatLocations(locs)
		if err := m.store.SetBackupRemote(context.WithoutCancel(ctx), b.ID, b.RemoteKey); err != nil {
			return err
		}
		done++
	}
	if done == 0 && len(errs) == 0 {
		lg.Logf("The backup is already in every remote storage")
	}
	return errors.Join(errs...)
}

// uploadTo copies a backup to one storage, encrypted if that storage asks
// for it, and returns the key it is stored under.
func (m *Manager) uploadTo(ctx context.Context, name string, b *store.Backup, key string, lg Logger) (string, error) {
	r, enc, err := m.openRemote(ctx, name)
	if err != nil {
		return "", err
	}
	if r == nil {
		return "", usererr.New("storage_not_configured")
	}
	defer r.Close()
	f, err := os.Open(b.File)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if !enc.Encrypt {
		lg.Logf("Uploading to %s", r.Describe(key))
		return key, r.Put(ctx, key, f, b.Size, b.SHA256)
	}
	key += encSuffix
	e, err := newEncReader(f, b.Size, enc.EncryptPassphrase)
	if err != nil {
		return "", err
	}
	sum := ""
	if w, ok := r.(wantsSHA256); ok && w.needsSHA256() {
		lg.Logf("Encrypting the backup to compute the checksum of the encrypted copy")
		if sum, err = sha256Of(ctx, io.NewSectionReader(e, 0, e.Size())); err != nil {
			return "", err
		}
	}
	lg.Logf("Uploading an encrypted copy to %s", r.Describe(key))
	return key, r.Put(ctx, key, io.NewSectionReader(e, 0, e.Size()), e.Size(), sum)
}

// sha256Of reads r to its end and returns its SHA-256.
func sha256Of(ctx context.Context, r io.Reader) (string, error) {
	h := sha256.New()
	buf := make([]byte, 256<<10)
	for {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		n, err := r.Read(buf)
		h.Write(buf[:n])
		if err == io.EOF {
			return hex.EncodeToString(h.Sum(nil)), nil
		}
		if err != nil {
			return "", err
		}
	}
}

// download fetches a backup kept only in remote storage into a private
// temporary file, trying each copy in turn. An encrypted copy is decrypted
// on the way, with passphrase (if given) or the storage's passphrases.
func (m *Manager) download(ctx context.Context, b *store.Backup, passphrase string, lg Logger) (*os.File, func(), error) {
	locs := parseLocations(b.RemoteKey)
	if len(locs) == 0 {
		return nil, nil, usererr.New("backup_file_missing", b.File)
	}
	if err := mkdirPrivate(m.dir); err != nil {
		return nil, nil, err
	}
	var errs []error
	for _, l := range locs {
		tmp, err := m.downloadFrom(ctx, l, passphrase, lg)
		if err == nil {
			return tmp, func() { tmp.Close(); os.Remove(tmp.Name()) }, nil
		}
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		lg.Logf("Download from %s failed: %v", remoteLabel(l.remote), err)
		errs = append(errs, fmt.Errorf("%s: %w", remoteLabel(l.remote), err))
	}
	return nil, nil, fmt.Errorf("the backup file is not on this server and could not be downloaded: %w", errors.Join(errs...))
}

func (m *Manager) downloadFrom(ctx context.Context, l location, passphrase string, lg Logger) (*os.File, error) {
	r, enc, err := m.openRemote(ctx, l.remote)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, fmt.Errorf("this remote storage is not set up any more")
	}
	defer r.Close()
	lg.Logf("Downloading the backup from %s", r.Describe(l.key))
	tmp, err := os.CreateTemp(m.dir, ".download-*.tar.gz")
	if err != nil {
		return nil, err
	}
	// The copy is decrypted as it arrives when it is an encrypted one,
	// whatever the storage's settings are now.
	rw := newRestoreWriter(tmp, enc.passphrases(passphrase))
	err = r.Get(ctx, l.key, rw)
	if rw.err != nil {
		err = rw.err // the decryption's own message, not the download's
	}
	if err == nil {
		err = rw.Close()
	}
	if err == nil && rw.Encrypted() {
		lg.Logf("The copy was encrypted and is decrypted")
	}
	if err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, err
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, err
	}
	return tmp, nil
}

// removeRemote deletes every remote copy of a backup. Failures are logged:
// the backup is forgotten anyway, as it was before SFTP existed.
func (m *Manager) removeRemote(ctx context.Context, b *store.Backup, lg Logger) {
	for _, l := range parseLocations(b.RemoteKey) {
		r, _, err := m.openRemote(ctx, l.remote)
		if err == nil && r != nil {
			err = r.Delete(ctx, l.key)
			r.Close()
		}
		if err != nil {
			m.log.Warn("delete remote backup", "remote", l.remote, "key", l.key, "err", err)
			if lg != nil {
				lg.Logf("Warning: could not delete the copy on %s: %v", remoteLabel(l.remote), err)
			}
		}
	}
}

// testStorage writes, lists (when the storage can), reads back and deletes a
// small file, encrypted when the storage encrypts its copies.
func testStorage(ctx context.Context, r remote, enc Encryption) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	body := []byte("panel storage test " + time.Now().UTC().Format(time.RFC3339))
	key := ".panel-test"
	var put io.Reader = strings.NewReader(string(body))
	size := int64(len(body))
	sum := sha256.Sum256(body)
	if enc.Encrypt {
		e, err := newEncReader(strings.NewReader(string(body)), size, enc.EncryptPassphrase)
		if err != nil {
			return err
		}
		key, size = key+encSuffix, e.Size()
		sealed := make([]byte, size)
		if _, err := e.ReadAt(sealed, 0); err != nil && err != io.EOF {
			return err
		}
		put, sum = bytes.NewReader(sealed), sha256.Sum256(sealed)
	}
	if err := r.Put(ctx, key, put, size, hex.EncodeToString(sum[:])); err != nil {
		return err
	}
	if l, ok := r.(lister); ok {
		files, err := l.List(ctx, "")
		if err != nil {
			return err
		}
		found := false
		for _, f := range files {
			found = found || f.Name == key && f.Size == size
		}
		if !found {
			return errors.New("the test file was written but is not listed")
		}
	}
	var got strings.Builder
	rw := newRestoreWriter(&got, enc.passphrases(""))
	err := r.Get(ctx, key, rw)
	if err == nil {
		err = rw.Close()
	}
	if err != nil {
		return err
	}
	if got.String() != string(body) {
		return errors.New("the test file read back differently")
	}
	return r.Delete(ctx, key)
}
