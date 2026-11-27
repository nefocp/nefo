package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/store"
)

func TestPlainSizeOf(t *testing.T) {
	for _, n := range []int64{0, 1, 65535, 65536, 65537, 300_000} {
		got, ok := plainSizeOf(encryptedSize(n, 16), 1<<16)
		if !ok || got != n {
			t.Errorf("%d: %d %v", n, got, ok)
		}
	}
	if _, ok := plainSizeOf(encHeaderSize+encTagSize+65536, 1<<16); ok {
		t.Error("impossible size accepted")
	}
}

func TestParseFoundKey(t *testing.T) {
	kind, dir, name, enc, ok := parseFoundKey("site/example.com/example.com_20261127-030000.tar.gz.enc")
	if !ok || kind != KindSite || dir != "example.com" || name != "example.com_20261127-030000.tar.gz" || !enc {
		t.Fatal(kind, dir, name, enc, ok)
	}
	for _, k := range []string{".panel-test", "site/a.com/notes.txt", "site/a.com/b/c.tar.gz", "other/a/b.tar.gz", "site/a.com/.x.tar.gz", "site//x.tar.gz"} {
		if _, _, _, _, ok := parseFoundKey(k); ok {
			t.Errorf("%q accepted", k)
		}
	}
}

// fakeArchive is a .tar.gz with the given manifest first.
func fakeArchive(t *testing.T, man manifest) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	b, _ := json.Marshal(man)
	addBytes(tw, "manifest.json", b, time.Now())
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

// A "new server": the backup list is empty, the storages are set up again,
// and the backups on them are found, imported and restored.
func TestFoundOnStorages(t *testing.T) {
	s3 := newFakeS3(t)
	sftpSrv := newTestSFTPServer(t)
	drive := useFakeDrive(t)
	m, st := newTestManager(t)
	ctx := context.Background()
	lg := testLog{t}
	const s3Pass, drivePass = "s3 passphrase 2026", "drive passphrase 2026"
	saveS3 := func(pass string) {
		t.Helper()
		if err := m.SaveStorage(ctx, S3Config{Endpoint: s3.srv.URL, Bucket: "bkt", Prefix: "panel", AccessKey: "AK", SecretKey: "SK",
			Encryption: Encryption{Encrypt: true, EncryptPassphrase: pass}}); err != nil {
			t.Fatal(err)
		}
	}
	saveS3(s3Pass)
	if _, err := m.SaveSFTP(ctx, sftpSrv.config()); err != nil {
		t.Fatal(err)
	}
	drive.connect(t, m, DriveConfig{Folder: "Backups", Encryption: Encryption{Encrypt: true, EncryptPassphrase: drivePass}})

	if _, err := m.ImportFound(ctx, RemoteS3, []string{"x"}); userCode(err) != "backup_scan_first" {
		t.Fatalf("import before a scan: %v", err)
	}

	clock := time.Date(2026, 11, 27, 3, 0, 0, 0, time.Local)
	m.now = func() time.Time { clock = clock.Add(time.Minute); return clock }
	dir := filepath.Join(t.TempDir(), "app")
	makeTree(t, dir)
	big := randomBytes(300_000)
	os.WriteFile(filepath.Join(dir, "big.bin"), big, 0o644)
	b1, err := m.BackupFolder(ctx, dir, 0, true, lg)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := m.BackupFolder(ctx, dir, 0, true, lg)
	if err != nil {
		t.Fatal(err)
	}
	// Things that are not backups, or not where they claim to be.
	os.MkdirAll(filepath.Join(sftpSrv.dir, "backups", "panel", "site", "evil.example"), 0o755)
	os.WriteFile(filepath.Join(sftpSrv.dir, "backups", "panel", "site", "evil.example", "evil.example_20261127-010000.tar.gz"),
		fakeArchive(t, manifest{Format: 1, Kind: KindSite, Domain: "other.example"}), 0o600)
	os.WriteFile(filepath.Join(sftpSrv.dir, "backups", "panel", "site", "evil.example", "junk_20261127-010000.tar.gz"), []byte("not gzip"), 0o600)
	os.WriteFile(filepath.Join(sftpSrv.dir, "backups", "panel", "site", "evil.example", "notes.txt"), []byte("x"), 0o600)

	// Nothing is found while the panel knows every file.
	res, err := m.ScanRemote(ctx, RemoteS3, "", lg)
	if err != nil || len(res.Found) != 0 || res.Known != 2 {
		t.Fatalf("scan of a known storage: %+v %v", res, err)
	}
	if l := m.FoundOn(RemoteS3); l == nil || l.Found == nil {
		t.Fatal("an empty scan must give an empty list, not none")
	}

	// The panel forgets its backups and their local files, and S3 is set
	// up again with another passphrase.
	for _, b := range []*store.Backup{b1, b2} {
		os.Remove(b.File)
		if err := st.DeleteBackup(ctx, b.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.RemoveStorage(ctx); err != nil {
		t.Fatal(err)
	}
	saveS3("a passphrase of the new server")

	res, err = m.ScanRemote(ctx, RemoteS3, "", lg)
	if err != nil || len(res.Found) != 2 || res.Known != 0 {
		t.Fatalf("scan: %+v %v", res, err)
	}
	for _, f := range res.Found {
		if f.Readable || !f.Encrypted || !strings.Contains(f.Error, "passphrase does not unlock") || f.ErrorCode != "wrong_passphrase" || f.Kind != KindFolder || f.Size != b1.Size {
			t.Fatalf("without the passphrase: %+v", f)
		}
	}
	if _, err := m.ImportFound(ctx, RemoteS3, []string{res.Found[0].Key}); userCode(err) != "backup_unreadable" {
		t.Fatalf("import of an unreadable copy: %v", err)
	}
	res, err = m.ScanRemote(ctx, RemoteS3, s3Pass, lg)
	if err != nil || len(res.Found) != 2 {
		t.Fatal(err)
	}
	f := res.Found[1] // newest first: b1 is the second
	if !f.Readable || f.Target != dir || f.CreatedAt == nil || !f.CreatedAt.Equal(b1.CreatedAt) ||
		f.Name != filepath.Base(b1.File) || f.Size != b1.Size || f.Key != remoteKey(b1)+encSuffix {
		t.Fatalf("found: %+v (want %s, %v)", f, filepath.Base(b1.File), b1.CreatedAt)
	}
	if _, err := m.ImportFound(ctx, RemoteS3, []string{"folder/x/y.tar.gz"}); userCode(err) != "backup_not_found_on_storage" {
		t.Fatalf("unknown key: %v", err)
	}
	ids, err := m.ImportFound(ctx, RemoteS3, []string{f.Key})
	if err != nil || len(ids) != 1 {
		t.Fatal(ids, err)
	}
	ib, err := st.BackupByID(ctx, ids[0])
	if err != nil || ib.File != b1.File || ib.Size != b1.Size || ib.SHA256 != "" || ib.Target != dir || ib.RemoteKey != f.Key || ib.ScheduleID != 0 {
		t.Fatalf("imported: %+v %v", ib, err)
	}
	if res, _ := m.ScanRemote(ctx, RemoteS3, s3Pass, lg); len(res.Found) != 1 || res.Known != 1 {
		t.Fatalf("after import: %+v", res)
	}

	// The same backup on SFTP (not encrypted there) joins the record.
	res, err = m.ScanRemote(ctx, RemoteSFTP, "", lg)
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, f := range res.Found {
		switch {
		case strings.HasPrefix(f.Key, "site/evil.example/evil"):
			if f.Readable || !strings.Contains(f.Error, "does not match") || f.ErrorCode != "mismatch" {
				t.Errorf("manifest of another site: %+v", f)
			}
		case strings.HasPrefix(f.Key, "site/evil.example/junk"):
			if f.Readable || !strings.Contains(f.Error, "not a backup archive") || f.ErrorCode != "not_backup" {
				t.Errorf("junk: %+v", f)
			}
		case f.Readable && !f.Encrypted && f.Target == dir:
			keys = append(keys, f.Key)
		default:
			t.Errorf("unexpected: %+v", f)
		}
	}
	if len(res.Found) != 4 || len(keys) != 2 {
		t.Fatalf("SFTP scan: %+v", res.Found)
	}
	ids2, err := m.ImportFound(ctx, RemoteSFTP, keys)
	if err != nil || len(ids2) != 2 || !slices.Contains(ids2, ids[0]) {
		t.Fatalf("SFTP import: %v %v", ids2, err)
	}
	ib, _ = st.BackupByID(ctx, ids[0])
	if got := Remotes(ib); !slices.Equal(got, []string{RemoteS3, RemoteSFTP}) {
		t.Fatalf("locations: %q", ib.RemoteKey)
	}

	// Restore: the copy is checked through, restored, its checksum kept.
	os.RemoveAll(dir)
	if err := m.Restore(ctx, ids[0], lg); err != nil {
		t.Fatal(err)
	}
	checkTree(t, dir)
	if got, _ := os.ReadFile(filepath.Join(dir, "big.bin")); !bytes.Equal(got, big) {
		t.Fatal("big.bin")
	}
	if ib, _ = st.BackupByID(ctx, ids[0]); ib.SHA256 != b1.SHA256 {
		t.Fatalf("checksum %q, want %q", ib.SHA256, b1.SHA256)
	}

	// A damaged plain copy without a recorded checksum is refused before
	// the folder changes (b2 is only on SFTP and S3; S3's copy is dropped).
	other := ids2[0]
	if other == ids[0] {
		other = ids2[1]
	}
	ob, _ := st.BackupByID(ctx, other)
	p := filepath.Join(sftpSrv.dir, "backups", "panel", filepath.FromSlash(remoteKey(ob)))
	data, _ := os.ReadFile(p)
	data[len(data)/2] ^= 0xff
	os.WriteFile(p, data, 0o600)
	os.WriteFile(filepath.Join(dir, "marker"), []byte("x"), 0o644)
	if err := m.Restore(ctx, other, lg); err == nil || !strings.Contains(err.Error(), "damaged") {
		t.Fatalf("damaged plain copy: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "marker")); err != nil {
		t.Fatal("the folder changed")
	}

	// Google Drive lists its folders too. Both copies there are unknown:
	// the records only name S3 and SFTP.
	res, err = m.ScanRemote(ctx, RemoteDrive, "", lg)
	if err != nil || len(res.Found) != 2 || res.Known != 0 {
		t.Fatalf("Drive scan: %+v %v", res, err)
	}
	for _, f := range res.Found {
		if !f.Readable || !f.Encrypted || f.Target != dir {
			t.Fatalf("Drive: %+v", f)
		}
	}
	if _, err := m.ScanRemote(ctx, "nope", "", lg); err == nil {
		t.Fatal("unknown storage accepted")
	}
	if l := m.FoundOn(RemoteDrive); l == nil || len(l.Found) != 2 {
		t.Fatalf("FoundOn: %+v", l)
	}
}
