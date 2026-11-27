package backup

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/store"
)

// A folder backup goes to S3, SFTP and Google Drive, each encrypting with
// its own passphrase, and comes back from each of them.
func TestEncryptedCopiesOnEveryStorage(t *testing.T) {
	s3 := newFakeS3(t)
	sftpSrv := newTestSFTPServer(t)
	drive := useFakeDrive(t)
	m, st := newTestManager(t)
	ctx := context.Background()
	lg := testLog{t}

	const s3Pass, sftpPass, drivePass = "s3 passphrase 2026", "sftp passphrase 2026", "drive passphrase 2026"
	if err := m.SaveStorage(ctx, S3Config{Endpoint: s3.srv.URL, Bucket: "bkt", AccessKey: "AK", SecretKey: "SK",
		Encryption: Encryption{Encrypt: true, EncryptPassphrase: s3Pass}}); err != nil {
		t.Fatal(err)
	}
	sc := sftpSrv.config()
	sc.Encryption = Encryption{Encrypt: true, EncryptPassphrase: sftpPass}
	if _, err := m.SaveSFTP(ctx, sc); err != nil {
		t.Fatal(err)
	}
	drive.connect(t, m, DriveConfig{Folder: "Backups", Encryption: Encryption{Encrypt: true, EncryptPassphrase: drivePass}})

	// No passphrase reaches the UI.
	s3v, _ := m.Storage(ctx)
	sftpv, _ := m.SFTPStorage(ctx)
	drv, _ := m.DriveStorage(ctx)
	if !s3v.Encrypt || s3v.EncryptPassphrase != "" || !sftpv.Encrypt || !drv.Encrypt {
		t.Fatalf("views: %+v %+v %+v", s3v, sftpv, drv)
	}

	clock := time.Date(2026, 11, 27, 3, 0, 0, 0, time.Local)
	m.now = func() time.Time { clock = clock.Add(time.Minute); return clock }
	dir := filepath.Join(t.TempDir(), "app")
	makeTree(t, dir)
	big := randomBytes(300_000) // several chunks, does not compress
	os.WriteFile(filepath.Join(dir, "big.bin"), big, 0o644)

	b, err := m.BackupFolder(ctx, dir, 0, true, lg)
	if err != nil {
		t.Fatal(err)
	}
	if got := EncryptedRemotes(b); !slices.Equal(got, []string{RemoteS3, RemoteSFTP, RemoteDrive}) {
		t.Fatalf("encrypted copies: %v (%q)", got, b.RemoteKey)
	}
	plain, _ := os.ReadFile(b.File)
	key := remoteKey(b) + encSuffix
	s3obj := []byte(s3.objects["/bkt/"+key])
	sftpFile, _ := os.ReadFile(filepath.Join(sftpSrv.dir, "backups", "panel", filepath.FromSlash(key)))
	driveFile := drive.byPath("Backups/" + key)
	if driveFile == nil {
		t.Fatal("not in Google Drive")
	}
	for name, copy := range map[string][]byte{"S3": s3obj, "SFTP": sftpFile, "Drive": driveFile.data} {
		if !bytes.HasPrefix(copy, []byte("NEFOENC\x01")) || int64(len(copy)) != encryptedSize(b.Size, encLogChunk) {
			t.Fatalf("%s copy: %d bytes, starts %q", name, len(copy), copy[:min(8, len(copy))])
		}
		if bytes.Contains(copy, plain[:200]) || bytes.Contains(copy, []byte{0x1f, 0x8b, 8}) && bytes.Index(copy, []byte{0x1f, 0x8b, 8}) < 64 {
			t.Fatalf("%s copy is readable", name)
		}
	}
	if bytes.Equal(s3obj[64:200], sftpFile[64:200]) {
		t.Fatal("two copies encrypted alike")
	}

	restore := func(opt RestoreOptions) error {
		t.Helper()
		os.RemoveAll(dir)
		return m.RestoreWith(ctx, b.ID, opt, lg)
	}
	checkBig := func() {
		t.Helper()
		checkTree(t, dir)
		if got, _ := os.ReadFile(filepath.Join(dir, "big.bin")); !bytes.Equal(got, big) {
			t.Fatal("big.bin not restored")
		}
	}

	// The local file is gone: each storage in turn gives it back.
	os.Remove(b.File)
	if err := restore(RestoreOptions{}); err != nil {
		t.Fatal(err)
	}
	checkBig()
	s3.set(func() { delete(s3.objects, "/bkt/"+key) })
	if err := restore(RestoreOptions{}); err != nil {
		t.Fatal(err)
	}
	checkBig()
	os.Remove(filepath.Join(sftpSrv.dir, "backups", "panel", filepath.FromSlash(key)))
	if err := restore(RestoreOptions{}); err != nil {
		t.Fatal(err)
	}
	checkBig()

	// A new Drive passphrase: the old copy still restores (the old
	// passphrase is remembered), the next upload uses the new one.
	if _, err := m.SaveDrive(ctx, DriveConfig{Folder: "Backups", Encryption: Encryption{Encrypt: true, EncryptPassphrase: "a new drive passphrase"}}); err != nil {
		t.Fatal(err)
	}
	if err := restore(RestoreOptions{}); err != nil {
		t.Fatalf("restore after a passphrase change: %v", err)
	}
	checkBig()

	// A fresh setup with another passphrase (as on a new server): the copy
	// does not open, and the folder is left alone; with the passphrase
	// given for this restore it does.
	if err := m.RemoveDrive(ctx); err != nil {
		t.Fatal(err)
	}
	drive.set(func() { drive.revoked = false })
	drive.connect(t, m, DriveConfig{Folder: "Backups", Encryption: Encryption{Encrypt: true, EncryptPassphrase: "yet another passphrase"}})
	makeTree(t, dir)
	marker := filepath.Join(dir, "marker")
	os.WriteFile(marker, []byte("still here"), 0o644)
	err = m.RestoreWith(ctx, b.ID, RestoreOptions{}, lg)
	if !errors.Is(err, ErrWrongPassphrase) || !strings.Contains(err.Error(), "Google Drive") {
		t.Fatalf("wrong passphrase: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("a failed restore changed the folder")
	}
	if err := restore(RestoreOptions{Passphrase: drivePass}); err != nil {
		t.Fatalf("with the old passphrase given: %v", err)
	}
	checkBig()

	// A damaged copy is refused before anything changes.
	drive.set(func() { f := drive.files[drive.byPathLocked("Backups/"+key).id]; f.data[70_000] ^= 1 })
	os.WriteFile(marker, []byte("still here"), 0o644)
	err = m.RestoreWith(ctx, b.ID, RestoreOptions{Passphrase: drivePass}, lg)
	if !errors.Is(err, ErrDamaged) {
		t.Fatalf("damaged copy: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("restoring a damaged copy changed the folder")
	}

	// Encryption off on SFTP: the next copy there is the plain archive.
	sc.Encryption = Encryption{}
	sc.Password = ""
	if _, err := m.SaveSFTP(ctx, sc); err != nil {
		t.Fatal(err)
	}
	b2, err := m.BackupFolder(ctx, dir, 0, true, lg)
	if err != nil {
		t.Fatal(err)
	}
	if got := EncryptedRemotes(b2); !slices.Equal(got, []string{RemoteS3, RemoteDrive}) {
		t.Fatalf("encrypted copies: %v", got)
	}
	sftpPlain, _ := os.ReadFile(filepath.Join(sftpSrv.dir, "backups", "panel", filepath.FromSlash(remoteKey(b2))))
	if local, _ := os.ReadFile(b2.File); !bytes.Equal(sftpPlain, local) {
		t.Fatal("unencrypted SFTP copy differs from the backup")
	}

	// Deleting removes the encrypted copies too.
	if err := m.Delete(ctx, b2.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := s3.objects["/bkt/"+remoteKey(b2)+encSuffix]; ok {
		t.Error("S3 copy left")
	}
	if drive.byPath("Backups/"+remoteKey(b2)+encSuffix) != nil {
		t.Error("Drive copy left")
	}
	if _, err := st.BackupByID(ctx, b2.ID); !errors.Is(err, store.ErrNotFound) {
		t.Error("backup still listed")
	}
}

// byPathLocked is byPath for a caller that holds f.mu.
func (f *fakeDrive) byPathLocked(p string) *fakeFile {
	f.mu.Unlock()
	defer f.mu.Lock()
	return f.byPath(p)
}
