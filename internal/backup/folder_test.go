package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/store"
)

func TestCheckFolder(t *testing.T) {
	m, _ := newTestManager(t)
	if runtime.GOOS != "windows" {
		m.ProtectFolders("/srv/panel-data")
		for _, p := range []string{"/", "/proc", "/proc/1", "/sys/kernel", "/dev", "/run/php", "/var/lib/nefo",
			"/var/lib/nefo/certs", "/var/lib/panel-sftp/u1", "/var/lib", "/var", "/srv", "/srv/panel-data/x"} {
			if err := m.checkFolder(p); userCode(err) != "folder_protected" {
				t.Errorf("%s: %v", p, err)
			}
		}
		for _, p := range []string{"etc/nginx", "/etc/nginx/", "/etc/../etc", "", "/etc/\nx"} {
			if err := m.checkFolder(p); userCode(err) != "invalid_folder" {
				t.Errorf("%q: %v", p, err)
			}
		}
		for _, p := range []string{"/etc/nginx", "/opt/app", "/var/www/example.com", "/home", "/etc"} {
			if err := m.checkFolder(p); err != nil {
				t.Errorf("%s refused: %v", p, err)
			}
		}
		link := filepath.Join(t.TempDir(), "sneaky")
		if err := os.Symlink("/proc", link); err == nil {
			if err := m.checkFolder(link); userCode(err) != "folder_protected" {
				t.Errorf("symlink to /proc: %v", err)
			}
		}
	}
	// The backup folder, folders inside it and folders containing it.
	for _, p := range []string{m.dir, filepath.Join(m.dir, "site"), filepath.Dir(m.dir)} {
		if err := m.checkFolder(p); userCode(err) != "folder_protected" {
			t.Errorf("%s: %v", p, err)
		}
	}
	other := t.TempDir()
	if err := m.CheckTarget(context.Background(), KindFolder, other); err != nil {
		t.Errorf("%s refused: %v", other, err)
	}
	if err := m.CheckTarget(context.Background(), KindFolder, filepath.Join(other, "missing")); userCode(err) != "folder_missing" {
		t.Errorf("missing folder: %v", err)
	}
}

func TestFolderSlug(t *testing.T) {
	if s := storageDir(KindFolder, "/etc/nginx"); !strings.HasPrefix(s, "etc_nginx-") || len(s) != len("etc_nginx-")+8 {
		t.Fatal(s)
	}
	if storageDir(KindFolder, "/a/b_c") == storageDir(KindFolder, "/a_b/c") {
		t.Fatal("different folders share a storage folder")
	}
	if s := folderSlug("/.."); s != "folder" {
		t.Fatal(s)
	}
	long := "/var/www/example.com/public/wp-content/uploads/2026/11/some-very-long-folder-name"
	if s := folderSlug(long); len(s) > 60 || !strings.HasSuffix(s, "some-very-long-folder-name") {
		t.Fatal(s)
	}
	if storageDir(KindSite, "example.com") != "example.com" {
		t.Fatal("site storage folder changed")
	}
}

// makeTree writes a small folder to back up.
func makeTree(t *testing.T, dir string) {
	t.Helper()
	files := map[string]string{"config.ini": "a=1\n", "sub/deep/data.txt": "deep", "bin/run.sh": "#!/bin/sh\necho hi\n"}
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	os.Mkdir(filepath.Join(dir, "empty"), 0o755)
	old := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	os.Chtimes(filepath.Join(dir, "config.ini"), old, old)
	if runtime.GOOS != "windows" {
		os.Chmod(filepath.Join(dir, "config.ini"), 0o640)
		os.Chmod(filepath.Join(dir, "sub/deep/data.txt"), 0o600)
		os.Chmod(filepath.Join(dir, "bin/run.sh"), 0o755)
		os.Chmod(filepath.Join(dir, "sub"), 0o750)
		os.Chmod(dir, 0o751)
		os.Symlink("sub/deep", filepath.Join(dir, "current"))
		os.Symlink("/etc/hostname", filepath.Join(dir, "outside"))
	}
}

func checkTree(t *testing.T, dir string) {
	t.Helper()
	for name, body := range map[string]string{"config.ini": "a=1\n", "sub/deep/data.txt": "deep", "bin/run.sh": "#!/bin/sh\necho hi\n"} {
		if b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(name))); err != nil || string(b) != body {
			t.Errorf("%s = %q, %v", name, b, err)
		}
	}
	if fi, err := os.Stat(filepath.Join(dir, "empty")); err != nil || !fi.IsDir() {
		t.Errorf("empty folder: %v", err)
	}
	if fi, err := os.Stat(filepath.Join(dir, "config.ini")); err != nil || !fi.ModTime().Equal(time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)) {
		t.Errorf("modification time not restored: %v", fi.ModTime())
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if isRestoreLeftover(e.Name()) {
			t.Errorf("leftover %s", e.Name())
		}
	}
	if runtime.GOOS == "windows" {
		return
	}
	for name, want := range map[string]fs.FileMode{"config.ini": 0o640, "sub/deep/data.txt": 0o600, "bin/run.sh": 0o755, "sub": 0o750 | fs.ModeDir, ".": 0o751 | fs.ModeDir} {
		if fi, err := os.Lstat(filepath.Join(dir, name)); err != nil || fi.Mode() != want {
			t.Errorf("%s mode = %v, want %v (%v)", name, fi.Mode(), want, err)
		}
	}
	if l, err := os.Readlink(filepath.Join(dir, "current")); err != nil || l != "sub/deep" {
		t.Errorf("symlink = %q, %v", l, err)
	}
	if l, err := os.Readlink(filepath.Join(dir, "outside")); err != nil || l != "/etc/hostname" {
		t.Errorf("symlink outside = %q, %v", l, err)
	}
}

func archiveNames(t *testing.T, file string) []string {
	t.Helper()
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	tr := tar.NewReader(gz)
	var names []string
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return names
		}
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, h.Name)
	}
}

func TestFolderBackupRestoreRetentionOverSFTP(t *testing.T) {
	srv := newTestSFTPServer(t)
	m, st := newTestManager(t)
	ctx := context.Background()
	lg := testLog{t}
	if _, err := m.SaveSFTP(ctx, srv.config()); err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 11, 25, 3, 0, 0, 0, time.Local)
	m.now = func() time.Time { clock = clock.Add(time.Minute); return clock }

	dir := filepath.Join(t.TempDir(), "app")
	makeTree(t, dir)
	sc := &store.Schedule{Kind: KindFolder, Target: dir, Frequency: "daily", Hour: 3, Keep: 1, Remote: true}
	if err := m.CheckSchedule(ctx, sc); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateSchedule(ctx, sc); err != nil {
		t.Fatal(err)
	}

	b1, err := m.Backup(ctx, KindFolder, dir, sc.ID, true, lg)
	if err != nil {
		t.Fatal(err)
	}
	if got := Remotes(b1); !slices.Equal(got, []string{RemoteSFTP}) {
		t.Fatalf("remotes = %v", got)
	}
	onServer := func(b *store.Backup) string {
		return filepath.Join(srv.dir, "backups", "panel", filepath.FromSlash(remoteKey(b)))
	}
	if _, err := os.Stat(onServer(b1)); err != nil {
		t.Fatalf("not uploaded: %v", err)
	}
	if !strings.Contains(filepath.Base(b1.File), "app_20261125-") {
		t.Fatalf("file name %s", b1.File)
	}
	names := archiveNames(t, b1.File)
	if names[0] != "manifest.json" || names[1] != "files/" || !slices.Contains(names, "files/sub/deep/data.txt") || !slices.Contains(names, "files/empty/") {
		t.Fatalf("archive: %v", names)
	}

	// Break the folder, drop the local copy: the restore downloads from SFTP.
	os.WriteFile(filepath.Join(dir, "config.ini"), []byte("broken"), 0o644)
	os.RemoveAll(filepath.Join(dir, "sub"))
	os.WriteFile(filepath.Join(dir, "intruder.php"), []byte("x"), 0o644)
	if err := os.Remove(b1.File); err != nil {
		t.Fatal(err)
	}
	if err := m.Restore(ctx, b1.ID, lg); err != nil {
		t.Fatal(err)
	}
	checkTree(t, dir)
	if _, err := os.Stat(filepath.Join(dir, "intruder.php")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("file added after the backup survived the restore: %v", err)
	}

	// The folder was deleted: the restore creates it again.
	os.RemoveAll(dir)
	if err := m.Restore(ctx, b1.ID, lg); err != nil {
		t.Fatal(err)
	}
	checkTree(t, dir)

	// Retention keeps 1: the older copy disappears from the SFTP server too.
	b2, err := m.Backup(ctx, KindFolder, dir, sc.ID, true, lg)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Prune(ctx, sc, lg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(onServer(b1)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("old copy still on the SFTP server: %v", err)
	}
	if _, err := st.BackupByID(ctx, b1.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("old backup still listed: %v", err)
	}
	if _, err := os.Stat(onServer(b2)); err != nil {
		t.Errorf("new copy missing on the SFTP server: %v", err)
	}
	if err := m.Delete(ctx, b2.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(onServer(b2)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("deleted backup still on the SFTP server: %v", err)
	}
}

// fakeS3 keeps objects in memory and can be told to fail uploads.
type fakeS3 struct {
	mu        sync.Mutex
	objects   map[string]string
	failPuts  bool
	srv       *httptest.Server
	prevMaker func(S3Config) remote
}

func newFakeS3(t *testing.T) *fakeS3 {
	f := &fakeS3{objects: map[string]string{}}
	f.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.Method {
		case http.MethodPut:
			if f.failPuts {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			b, _ := io.ReadAll(r.Body)
			// The signed checksum must be the body's, as S3 checks it.
			if sum := sha256.Sum256(b); r.Header.Get("X-Amz-Content-Sha256") != hex.EncodeToString(sum[:]) {
				w.WriteHeader(http.StatusBadRequest)
				io.WriteString(w, `<Error><Code>XAmzContentSHA256Mismatch</Code><Message>checksum</Message></Error>`)
				return
			}
			f.objects[r.URL.Path] = string(b)
		case http.MethodGet:
			if r.URL.Query().Get("list-type") == "2" {
				f.list(w, r)
				return
			}
			v, ok := f.objects[r.URL.Path]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			io.WriteString(w, v)
		case http.MethodDelete:
			delete(f.objects, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	f.prevMaker = newS3Remote
	newS3Remote = func(c S3Config) remote {
		cl := newS3(c)
		cl.http = f.srv.Client()
		return cl
	}
	t.Cleanup(func() { newS3Remote = f.prevMaker; f.srv.Close() })
	return f
}

// list answers ListObjectsV2 with pages of two objects.
func (f *fakeS3) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	prefix := r.URL.Path + "/" + q.Get("prefix")
	var keys []string
	for k := range f.objects {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	start := 0
	if t := q.Get("continuation-token"); t != "" {
		start = slices.Index(keys, t)
	}
	end := min(start+2, len(keys))
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><ListBucketResult>`)
	for _, k := range keys[start:end] {
		fmt.Fprintf(&b, `<Contents><Key>%s</Key><Size>%d</Size><LastModified>2026-11-27T03:00:00.000Z</LastModified></Contents>`,
			strings.TrimPrefix(k, r.URL.Path+"/"), len(f.objects[k]))
	}
	if end < len(keys) {
		fmt.Fprintf(&b, `<IsTruncated>true</IsTruncated><NextContinuationToken>%s</NextContinuationToken>`, keys[end])
	} else {
		b.WriteString(`<IsTruncated>false</IsTruncated>`)
	}
	b.WriteString(`</ListBucketResult>`)
	io.WriteString(w, b.String())
}

func (f *fakeS3) set(fn func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn()
}

func TestBothRemotes(t *testing.T) {
	s3 := newFakeS3(t)
	srv := newTestSFTPServer(t)
	m, st := newTestManager(t)
	ctx := context.Background()
	lg := testLog{t}
	if err := m.SaveStorage(ctx, S3Config{Endpoint: s3.srv.URL, Bucket: "bkt", AccessKey: "AK", SecretKey: "SK"}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.SaveSFTP(ctx, srv.config()); err != nil {
		t.Fatal(err)
	}
	if names, _ := m.ConfiguredRemotes(ctx); !slices.Equal(names, []string{RemoteS3, RemoteSFTP}) {
		t.Fatalf("configured = %v", names)
	}
	clock := time.Date(2026, 11, 25, 3, 0, 0, 0, time.Local)
	m.now = func() time.Time { clock = clock.Add(time.Minute); return clock }
	dir := filepath.Join(t.TempDir(), "app")
	makeTree(t, dir)

	b, err := m.BackupFolder(ctx, dir, 0, true, lg)
	if err != nil {
		t.Fatal(err)
	}
	if got := Remotes(b); !slices.Equal(got, []string{RemoteS3, RemoteSFTP}) {
		t.Fatalf("remotes = %v", got)
	}
	s3Key := "/bkt/" + remoteKey(b)
	if _, ok := s3.objects[s3Key]; !ok {
		t.Fatalf("not in S3: %v", s3.objects)
	}

	// The S3 copy is lost and the local file too: the SFTP copy is used.
	s3.set(func() { delete(s3.objects, s3Key) })
	os.Remove(b.File)
	os.RemoveAll(dir)
	if err := m.Restore(ctx, b.ID, lg); err != nil {
		t.Fatal(err)
	}
	checkTree(t, dir)

	// S3 is down: the backup still reaches SFTP, and the error names S3.
	s3.set(func() { s3.failPuts = true })
	b2, err := m.BackupFolder(ctx, dir, 0, true, lg)
	if err == nil || !strings.Contains(err.Error(), "S3") || b2 == nil {
		t.Fatalf("partial upload: %v", err)
	}
	if got := Remotes(b2); !slices.Equal(got, []string{RemoteSFTP}) {
		t.Fatalf("remotes = %v", got)
	}
	// Uploading again later adds only the missing copy.
	s3.set(func() { s3.failPuts = false })
	if err := m.Upload(ctx, b2.ID, lg); err != nil {
		t.Fatal(err)
	}
	b2, _ = st.BackupByID(ctx, b2.ID)
	if got := Remotes(b2); !slices.Equal(got, []string{RemoteSFTP, RemoteS3}) {
		t.Fatalf("remotes after upload = %v (%q)", got, b2.RemoteKey)
	}

	// Deleting removes both copies.
	if err := m.Delete(ctx, b2.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := s3.objects["/bkt/"+remoteKey(b2)]; ok {
		t.Error("S3 copy left")
	}
	if _, err := os.Stat(filepath.Join(srv.dir, "backups", "panel", filepath.FromSlash(remoteKey(b2)))); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("SFTP copy left: %v", err)
	}

	// The S3 view does not contain the secret key; SFTP has its own.
	v, _ := m.Storage(ctx)
	vb, _ := json.Marshal(v)
	if strings.Contains(string(vb), "SK") {
		t.Fatalf("S3 view: %s", vb)
	}
}
