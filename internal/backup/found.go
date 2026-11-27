package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// Backups found on a storage
//
// A new server (or one whose panel lost its list) can recover the backups
// that an earlier panel uploaded: ScanRemote lists the archive files on a
// storage that this panel does not know, and reads the manifest.json at
// the start of each one (decrypting the first chunk of an encrypted copy),
// without downloading the rest. ImportFound adds the chosen ones to the
// backup list as backups kept only on that storage; they are restored like
// any other (a website that does not exist here is created again).

// treeLister is a remote that can list every file under a prefix, at any
// depth. Names are keys relative to the storage's root ("site/a.com/x").
type treeLister interface {
	ListTree(ctx context.Context, prefix string) ([]RemoteFile, error)
}

// maxFound is the most files a scan reads.
const maxFound = 2000

// FoundBackup is a backup file on a storage that this panel does not list.
type FoundBackup struct {
	Remote     string    `json:"remote"`
	Key        string    `json:"key"`  // in the storage, as remote_key keeps it
	Kind       string    `json:"kind"` // site, database, folder, app
	Target     string    `json:"target"`
	Name       string    `json:"name"`        // the archive's file name, without .enc
	Size       int64     `json:"size"`        // of the archive (decrypted)
	StoredSize int64     `json:"stored_size"` // of the file on the storage
	Modified   time.Time `json:"modified"`
	Encrypted  bool      `json:"encrypted"`
	// Readable means the manifest was read; otherwise Error says why
	// (a passphrase is needed, it is wrong, the file is damaged…).
	Readable bool   `json:"readable"`
	Error    string `json:"error,omitempty"`
	// ErrorCode names the usual reasons, for the UI to translate:
	// need_passphrase, wrong_passphrase, damaged, not_backup, mismatch.
	ErrorCode    string     `json:"error_code,omitempty"`
	CreatedAt    *time.Time `json:"created_at,omitempty"`
	PHPVersion   string     `json:"php_version,omitempty"`
	App          string     `json:"app,omitempty"`
	Databases    []string   `json:"databases"`
	PanelVersion string     `json:"panel_version,omitempty"`
	// Imported is set once the file is in the backup list.
	Imported bool `json:"imported,omitempty"`
}

// FoundList is the result of the last scan of a storage.
type FoundList struct {
	Remote    string        `json:"remote"`
	ScannedAt time.Time     `json:"scanned_at"`
	Found     []FoundBackup `json:"found"`
	// Known counts the archive files that are in the backup list already.
	Known int `json:"known"`
}

// foundScans keeps the last scan of each storage, for ImportFound.
type foundScans struct {
	mu   sync.Mutex
	last map[string]*FoundList
}

// errNotBackup is a file that is not a backup archive made by the panel.
var errNotBackup = errors.New("this file is not a backup archive made by the panel")

// errHeadRead stops a download once the manifest has been read.
var errHeadRead = errors.New("manifest read")

// headInfo is what the start of an archive tells.
type headInfo struct {
	man       *manifest
	encrypted bool
	chunk     int // of an encrypted copy
}

// readHead reads manifest.json at the start of a stored archive, plain or
// encrypted, and stops the download there.
func readHead(ctx context.Context, r remote, key string, passphrases []string) (*headInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	pr, pw := io.Pipe()
	type result struct {
		man *manifest
		err error
	}
	done := make(chan result, 1)
	go func() {
		man, err := func() (*manifest, error) {
			gz, err := gzip.NewReader(pr)
			if err != nil {
				return nil, errNotBackup
			}
			tr := tar.NewReader(gz)
			hdr, err := tr.Next()
			if err != nil || hdr.Name != "manifest.json" {
				return nil, errNotBackup
			}
			var man manifest
			if err := json.NewDecoder(io.LimitReader(tr, 1<<20)).Decode(&man); err != nil {
				return nil, fmt.Errorf("read manifest.json: %w", err)
			}
			return &man, nil
		}()
		pr.CloseWithError(errHeadRead) // the download stops at its next write
		done <- result{man, err}
	}()
	rw := newRestoreWriter(pw, passphrases)
	gerr := r.Get(ctx, key, rw)
	if gerr == nil {
		gerr = rw.Close()
	}
	if gerr == nil {
		gerr = io.EOF
	}
	pw.CloseWithError(gerr)
	res := <-done
	info := &headInfo{man: res.man, encrypted: rw.Encrypted()}
	if rw.dec != nil {
		info.chunk = rw.dec.chunk
	}
	if res.err == nil {
		return info, nil
	}
	// The decryption's own error says more than "not a backup archive".
	if rw.err != nil && !errors.Is(rw.err, errHeadRead) {
		return info, rw.err
	}
	if !errors.Is(gerr, io.EOF) && !errors.Is(gerr, errHeadRead) && !strings.Contains(gerr.Error(), errHeadRead.Error()) {
		return info, gerr
	}
	return info, res.err
}

// plainSizeOf is the size of the archive inside an encrypted copy of
// stored bytes, or false when that size is impossible.
func plainSizeOf(stored int64, chunk int) (int64, bool) {
	if chunk <= 0 {
		return 0, false
	}
	m := stored - encHeaderSize - encTagSize
	if m < 0 {
		return 0, false
	}
	step := int64(chunk + encTagSize)
	full, last := m/step, m%step
	if last >= int64(chunk) {
		return 0, false
	}
	return full*int64(chunk) + last, true
}

// parseFoundKey splits "site/example.com/example.com_….tar.gz[.enc]".
func parseFoundKey(key string) (kind, dir, name string, encrypted, ok bool) {
	parts := strings.Split(key, "/")
	if len(parts) != 3 {
		return "", "", "", false, false
	}
	kind, dir, name = parts[0], parts[1], parts[2]
	name, encrypted = strings.CutSuffix(name, encSuffix)
	switch kind {
	case KindSite, KindDatabase, KindFolder, KindApp:
	default:
		return "", "", "", false, false
	}
	if dir == "" || !strings.HasSuffix(name, ".tar.gz") || len(name) <= len(".tar.gz") || strings.HasPrefix(name, ".") {
		return "", "", "", false, false
	}
	return kind, dir, name, encrypted, true
}

// targetOf is the target a manifest names for its kind of backup.
func targetOf(kind string, man *manifest) string {
	switch kind {
	case KindSite:
		return man.Domain
	case KindDatabase:
		if len(man.Databases) > 0 {
			return man.Databases[0]
		}
		if len(man.PGDatabases) > 0 {
			return man.PGDatabases[0]
		}
	case KindFolder:
		return man.Path
	case KindApp:
		return path.Base(man.Path)
	}
	return ""
}

// ScanRemote lists the backup archives on a storage that are not in the
// backup list, and reads their manifests. passphrase (optional) is tried
// first on encrypted copies, then the storage's own passphrases.
func (m *Manager) ScanRemote(ctx context.Context, name, passphrase string, lg Logger) (*FoundList, error) {
	r, enc, err := m.openRemote(ctx, name)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, usererr.New("storage_not_configured")
	}
	defer r.Close()
	tl, ok := r.(treeLister)
	if !ok {
		return nil, fmt.Errorf("%s storage cannot list its files", remoteLabel(name))
	}
	known := map[string]bool{}
	backups, err := m.store.ListBackups(ctx)
	if err != nil {
		return nil, err
	}
	for _, b := range backups {
		for _, l := range parseLocations(b.RemoteKey) {
			if l.remote == name {
				known[l.key] = true
			}
		}
	}
	lg.Logf("Listing the files on %s", remoteLabel(name))
	var files []RemoteFile
	for _, kind := range []string{KindSite, KindDatabase, KindFolder, KindApp} {
		f, err := tl.ListTree(ctx, kind)
		if err != nil {
			return nil, fmt.Errorf("list the files on %s: %w", remoteLabel(name), err)
		}
		files = append(files, f...)
	}
	res := &FoundList{Remote: name, ScannedAt: m.now(), Found: []FoundBackup{}}
	var todo []RemoteFile
	for _, f := range files {
		if _, _, _, _, ok := parseFoundKey(f.Name); !ok {
			continue
		}
		if known[f.Name] {
			res.Known++
			continue
		}
		todo = append(todo, f)
	}
	sort.Slice(todo, func(i, j int) bool { return todo[i].Name < todo[j].Name })
	if len(todo) > maxFound {
		lg.Logf("%d archives found: only the first %d are read", len(todo), maxFound)
		todo = todo[:maxFound]
	}
	lg.Logf("%d archives that are not in the backup list (%d are)", len(todo), res.Known)
	pass := enc.passphrases(passphrase)
	for i, f := range todo {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		fb := m.readFound(ctx, r, name, f, pass)
		if (i+1)%20 == 0 || i == len(todo)-1 {
			lg.Logf("Read %d of %d", i+1, len(todo))
		}
		res.Found = append(res.Found, fb)
	}
	// Newest first, as in the backup list.
	sort.SliceStable(res.Found, func(i, j int) bool { return foundTime(res.Found[i]).After(foundTime(res.Found[j])) })
	m.found.mu.Lock()
	if m.found.last == nil {
		m.found.last = map[string]*FoundList{}
	}
	m.found.last[name] = res
	m.found.mu.Unlock()
	return res, nil
}

func foundTime(f FoundBackup) time.Time {
	if f.CreatedAt != nil {
		return *f.CreatedAt
	}
	return f.Modified
}

// readFound reads one archive's manifest and checks that it matches the
// archive's place on the storage.
func (m *Manager) readFound(ctx context.Context, r remote, name string, f RemoteFile, pass []string) FoundBackup {
	kind, dir, file, encrypted, _ := parseFoundKey(f.Name)
	fb := FoundBackup{Remote: name, Key: f.Name, Kind: kind, Name: file, StoredSize: f.Size, Size: f.Size,
		Modified: f.ModTime, Encrypted: encrypted, Databases: []string{}}
	if kind != KindFolder {
		fb.Target = dir // until the manifest tells
	}
	info, err := readHead(ctx, r, f.Name, pass)
	if info != nil && info.encrypted {
		fb.Encrypted = true
		if n, ok := plainSizeOf(f.Size, info.chunk); ok {
			fb.Size = n
		}
	}
	if err != nil {
		fb.Error, fb.ErrorCode = err.Error(), foundErrorCode(err)
		return fb
	}
	man := info.man
	target := targetOf(kind, man)
	if man.Kind != kind || checkTarget(kind, target) != nil || storageDir(kind, target) != dir {
		fb.Error, fb.ErrorCode = "the archive's manifest does not match its place on the storage", "mismatch"
		return fb
	}
	fb.Readable, fb.Target = true, target
	t := man.CreatedAt
	fb.CreatedAt, fb.PHPVersion, fb.App, fb.PanelVersion = &t, man.PHPVersion, man.App, man.PanelVersion
	fb.Databases = append(append(fb.Databases, man.Databases...), man.PGDatabases...)
	return fb
}

func foundErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrNeedPassphrase):
		return "need_passphrase"
	case errors.Is(err, ErrWrongPassphrase):
		return "wrong_passphrase"
	case errors.Is(err, ErrDamaged), errors.Is(err, ErrTruncated), strings.Contains(err.Error(), "header of the encrypted copy"):
		return "damaged"
	case errors.Is(err, errNotBackup):
		return "not_backup"
	}
	return ""
}

// FoundOn returns the last scan of a storage, or nil.
func (m *Manager) FoundOn(name string) *FoundList {
	m.found.mu.Lock()
	defer m.found.mu.Unlock()
	if l := m.found.last[name]; l != nil {
		c := *l
		c.Found = append([]FoundBackup{}, l.Found...) // [] rather than null when nothing was found
		return &c
	}
	return nil
}

// ImportFound adds archives of the last scan of a storage to the backup
// list, as backups kept only there. An archive whose backup is in the list
// already (the same file name, uploaded to another storage) is added to
// that backup as one more copy. It returns the backups' IDs, in the order
// of keys.
func (m *Manager) ImportFound(ctx context.Context, name string, keys []string) ([]int64, error) {
	m.found.mu.Lock()
	scan := m.found.last[name]
	m.found.mu.Unlock()
	if scan == nil {
		return nil, usererr.New("backup_scan_first")
	}
	byKey := map[string]*FoundBackup{}
	for i := range scan.Found {
		byKey[scan.Found[i].Key] = &scan.Found[i]
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	backups, err := m.store.ListBackups(ctx)
	if err != nil {
		return nil, err
	}
	byFile := map[string]*store.Backup{}
	for _, b := range backups {
		byFile[b.File] = b
	}
	ids := []int64{}
	for _, key := range keys {
		fb := byKey[key]
		if fb == nil {
			return nil, usererr.New("backup_not_found_on_storage", key)
		}
		if !fb.Readable {
			return nil, usererr.New("backup_unreadable", fb.Name)
		}
		kind, dir, file, _, _ := parseFoundKey(key)
		local := filepath.Join(m.dir, kind, dir, file)
		if b := byFile[local]; b != nil {
			locs := parseLocations(b.RemoteKey)
			if !hasLocation(locs, name) {
				locs = append(locs, location{name, key})
				b.RemoteKey = formatLocations(locs)
				if err := m.store.SetBackupRemote(ctx, b.ID, b.RemoteKey); err != nil {
					return nil, err
				}
			}
			ids = append(ids, b.ID)
			fb.Imported = true
			continue
		}
		b := &store.Backup{Kind: kind, Target: fb.Target, File: local, Size: fb.Size,
			RemoteKey: formatLocations([]location{{name, key}}), CreatedAt: *fb.CreatedAt}
		if kind == KindApp {
			b.AppID = fb.App
		}
		// No checksum is known: the restore checks the archive through
		// once before it changes anything, and records it then.
		if err := m.store.CreateBackup(ctx, b); err != nil {
			return nil, err
		}
		byFile[local] = b
		ids = append(ids, b.ID)
		fb.Imported = true
	}
	return ids, nil
}

// checkArchive reads a whole archive (gzip and tar), so that a damaged one
// is found before a restore changes anything.
func checkArchive(r io.Reader) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		_, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if _, err := io.Copy(io.Discard, tr); err != nil {
			return err
		}
	}
	// The end of the gzip stream holds its checksum.
	_, err = io.Copy(io.Discard, gz)
	return err
}
