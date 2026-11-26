// Package files is the panel's file manager.
//
// The panel runs as root, so every operation is done through an os.Root:
// paths cannot climb out with "..", and symlinks cannot point outside it.
// Inside a website's folder the root is that folder, so a hacked site that
// plants a symlink to /etc cannot trick the panel into touching system files,
// and new files belong to the site's user so its PHP can use them.
package files

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/netip"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/usererr"
)

const (
	// MaxEditSize is the largest file the editor opens.
	MaxEditSize = 2 << 20
	maxEntries  = 5000
)

// Scope is the folder an operation is confined to and who owns new files.
type Scope struct {
	Root string // absolute directory
	UID  int    // owner of new files and folders
	GID  int
	Site string // domain when Root is a website's folder
}

// Resolver finds the scope of an absolute path.
type Resolver func(ctx context.Context, abs string) (Scope, error)

// Manager performs file operations.
type Manager struct {
	resolve  Resolver
	trashDir string
	trashMu  sync.Mutex // one trash change at a time
	// checkAddr decides which addresses downloads may connect to.
	checkAddr func(netip.AddrPort) error
}

// New returns a Manager.
func New(resolve Resolver) *Manager {
	return &Manager{resolve: resolve, trashDir: DefaultTrashDir, checkAddr: publicAddr}
}

// ErrUnsupported is returned on systems other than Linux.
var ErrUnsupported = errors.New("the file manager needs Linux")

// protected are folders that must never be deleted, renamed or replaced.
var protected = map[string]bool{
	"/": true, "/bin": true, "/boot": true, "/dev": true, "/etc": true, "/home": true, "/lib": true,
	"/lib64": true, "/opt": true, "/proc": true, "/root": true, "/run": true, "/sbin": true, "/srv": true,
	"/sys": true, "/tmp": true, "/usr": true, "/var": true, "/var/www": true, "/var/lib": true,
	"/var/lib/nefo": true, "/var/log": true, "/etc/nefo": true,
}

// Clean checks that p is an absolute path and returns it cleaned.
func Clean(p string) (string, error) {
	if p == "" || !strings.HasPrefix(p, "/") || strings.ContainsRune(p, 0) || len(p) > 4096 {
		return "", usererr.New("invalid_path", p)
	}
	return path.Clean(p), nil
}

// target is a resolved path: an os.Root, the path relative to it and the scope.
type target struct {
	root  *os.Root
	rel   string
	abs   string
	scope Scope
}

func (t *target) Close() { t.root.Close() }

func (m *Manager) open(ctx context.Context, p string) (*target, error) {
	if runtime.GOOS != "linux" {
		return nil, ErrUnsupported
	}
	abs, err := Clean(p)
	if err != nil {
		return nil, err
	}
	scope, err := m.resolve(ctx, abs)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(scope.Root, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		return nil, usererr.New("invalid_path", abs)
	}
	root, err := os.OpenRoot(scope.Root)
	if err != nil {
		return nil, err
	}
	return &target{root: root, rel: rel, abs: abs, scope: scope}, nil
}

// mapErr turns common file system errors into messages the UI translates.
func mapErr(err error, p string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, fs.ErrNotExist):
		return usererr.New("path_not_found", p)
	case errors.Is(err, fs.ErrExist):
		return usererr.New("path_exists", p)
	case errors.Is(err, fs.ErrPermission):
		return usererr.New("permission_denied", p)
	case strings.Contains(err.Error(), "escapes from parent"):
		return usererr.New("path_escapes", p)
	}
	return err
}

// Entry is one item of a folder listing.
type Entry struct {
	Name      string    `json:"name"`
	Type      string    `json:"type"` // dir, file, link or other
	Size      int64     `json:"size"`
	Mode      string    `json:"mode"`
	Owner     string    `json:"owner"`
	Group     string    `json:"group"`
	Modified  time.Time `json:"modified"`
	Target    string    `json:"target,omitempty"`
	TargetDir bool      `json:"target_dir,omitempty"`
}

// Listing is the content of a folder.
type Listing struct {
	Path      string  `json:"path"`
	Site      string  `json:"site,omitempty"`
	Entries   []Entry `json:"entries"`
	Truncated bool    `json:"truncated"`
}

func entryType(m fs.FileMode) string {
	switch {
	case m.IsDir():
		return "dir"
	case m.IsRegular():
		return "file"
	case m&fs.ModeSymlink != 0:
		return "link"
	}
	return "other"
}

// List returns the entries of a folder: folders first, then by name.
func (m *Manager) List(ctx context.Context, p string) (*Listing, error) {
	t, err := m.open(ctx, p)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	f, err := t.root.Open(t.rel)
	if err != nil {
		return nil, mapErr(err, t.abs)
	}
	defer f.Close()
	des, err := f.ReadDir(-1)
	if err != nil {
		return nil, mapErr(err, t.abs)
	}
	l := &Listing{Path: t.abs, Site: t.scope.Site, Entries: []Entry{}}
	if len(des) > maxEntries {
		des, l.Truncated = des[:maxEntries], true
	}
	names := newNameCache()
	for _, de := range des {
		rel := path.Join(t.rel, de.Name())
		fi, err := t.root.Lstat(rel)
		if err != nil {
			continue
		}
		uid, gid := fsutil.Owner(fi)
		e := Entry{
			Name:     de.Name(),
			Type:     entryType(fi.Mode()),
			Size:     fi.Size(),
			Mode:     fmt.Sprintf("%04o", fi.Mode().Perm()|specialBits(fi.Mode())),
			Owner:    names.user(uid),
			Group:    names.group(gid),
			Modified: fi.ModTime(),
		}
		if e.Type == "link" {
			e.Target, _ = t.root.Readlink(rel)
			if st, err := t.root.Stat(rel); err == nil {
				e.TargetDir = st.IsDir()
			}
		}
		l.Entries = append(l.Entries, e)
	}
	sort.Slice(l.Entries, func(i, j int) bool {
		a, b := l.Entries[i], l.Entries[j]
		ad, bd := a.Type == "dir" || a.TargetDir, b.Type == "dir" || b.TargetDir
		if ad != bd {
			return ad
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	return l, nil
}

func specialBits(m fs.FileMode) fs.FileMode {
	var s fs.FileMode
	if m&fs.ModeSetuid != 0 {
		s |= 0o4000
	}
	if m&fs.ModeSetgid != 0 {
		s |= 0o2000
	}
	if m&fs.ModeSticky != 0 {
		s |= 0o1000
	}
	return s
}

// ReadText returns the content of a text file for the editor.
func (m *Manager) ReadText(ctx context.Context, p string) (string, time.Time, error) {
	t, err := m.open(ctx, p)
	if err != nil {
		return "", time.Time{}, err
	}
	defer t.Close()
	fi, err := t.root.Stat(t.rel)
	if err != nil {
		return "", time.Time{}, mapErr(err, t.abs)
	}
	if !fi.Mode().IsRegular() {
		return "", time.Time{}, usererr.New("not_a_file", t.abs)
	}
	if fi.Size() > MaxEditSize {
		return "", time.Time{}, usererr.New("file_too_large", t.abs)
	}
	b, err := t.root.ReadFile(t.rel)
	if err != nil {
		return "", time.Time{}, mapErr(err, t.abs)
	}
	if isBinary(b) {
		return "", time.Time{}, usererr.New("binary_file", t.abs)
	}
	return string(b), fi.ModTime(), nil
}

// isBinary reports whether b looks like a binary file: a NUL byte in the
// first 8 KB.
func isBinary(b []byte) bool {
	if len(b) > 8<<10 {
		b = b[:8<<10]
	}
	for _, c := range b {
		if c == 0 {
			return true
		}
	}
	return false
}

// WriteText saves a text file. An existing file keeps its mode and owner; a
// new one gets mode 0644 and the scope's owner.
func (m *Manager) WriteText(ctx context.Context, p, content string, create bool) error {
	if len(content) > MaxEditSize {
		return usererr.New("file_too_large", p)
	}
	t, err := m.open(ctx, p)
	if err != nil {
		return err
	}
	defer t.Close()
	fi, err := t.root.Lstat(t.rel)
	switch {
	case err == nil && create:
		return usererr.New("path_exists", t.abs)
	case err == nil && !fi.Mode().IsRegular():
		return usererr.New("not_a_file", t.abs)
	case err == nil:
		return mapErr(t.root.WriteFile(t.rel, []byte(content), fi.Mode().Perm()), t.abs)
	case !errors.Is(err, fs.ErrNotExist):
		return mapErr(err, t.abs)
	case !create:
		return usererr.New("path_not_found", t.abs)
	}
	if err := t.root.WriteFile(t.rel, []byte(content), 0o644); err != nil {
		return mapErr(err, t.abs)
	}
	return t.own(t.rel, 0o644)
}

// own sets the mode explicitly (the panel runs with umask 077) and gives a
// new item to the scope's owner.
func (t *target) own(rel string, mode fs.FileMode) error {
	if err := t.root.Chmod(rel, mode); err != nil {
		return err
	}
	return t.root.Lchown(rel, t.scope.UID, t.scope.GID)
}

// Mkdir creates a folder.
func (m *Manager) Mkdir(ctx context.Context, p string) error {
	t, err := m.open(ctx, p)
	if err != nil {
		return err
	}
	defer t.Close()
	if err := t.root.Mkdir(t.rel, 0o755); err != nil {
		return mapErr(err, t.abs)
	}
	return t.own(t.rel, 0o755)
}

func (t *target) checkChangeable() error {
	if protected[t.abs] || t.rel == "." {
		return usererr.New("path_protected", t.abs)
	}
	return nil
}

// Rename moves or renames an item within the same scope.
func (m *Manager) Rename(ctx context.Context, from, to string) error {
	src, err := m.open(ctx, from)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := m.open(ctx, to)
	if err != nil {
		return err
	}
	defer dst.Close()
	if src.scope.Root != dst.scope.Root {
		return usererr.New("cross_scope")
	}
	if err := src.checkChangeable(); err != nil {
		return err
	}
	if err := dst.checkChangeable(); err != nil {
		return err
	}
	if _, err := src.root.Lstat(dst.rel); err == nil {
		return usererr.New("path_exists", dst.abs)
	}
	return mapErr(src.root.Rename(src.rel, dst.rel), src.abs)
}

// Delete moves files and folders to the trash, where they can be restored
// for 30 days. Items already inside the trash folder are deleted for good.
func (m *Manager) Delete(ctx context.Context, paths []string) error {
	for _, p := range paths {
		if err := m.deleteOne(ctx, p, false); err != nil {
			return err
		}
	}
	return nil
}

// DeletePermanently removes files and folders (with their content) at once,
// without going through the trash.
func (m *Manager) DeletePermanently(ctx context.Context, paths []string) error {
	for _, p := range paths {
		if err := m.deleteOne(ctx, p, true); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) deleteOne(ctx context.Context, p string, permanent bool) error {
	t, err := m.open(ctx, p)
	if err != nil {
		return err
	}
	defer t.Close()
	if err := t.checkChangeable(); err != nil {
		return err
	}
	// The trash folder itself, and the folders that hold it, stay.
	if t.abs == m.trashDir || strings.HasPrefix(m.trashDir, t.abs+"/") {
		return usererr.New("path_protected", t.abs)
	}
	if _, err := t.root.Lstat(t.rel); err != nil {
		return mapErr(err, p)
	}
	if permanent || strings.HasPrefix(t.abs, m.trashDir+"/") {
		return mapErr(t.root.RemoveAll(t.rel), p)
	}
	return m.trashItem(t)
}

// Chmod changes the permissions of an item. Only the usual permission bits
// (up to 0777) are accepted: no setuid.
func (m *Manager) Chmod(ctx context.Context, p, mode string) error {
	v, err := strconv.ParseUint(mode, 8, 32)
	if err != nil || v > 0o777 {
		return usererr.New("invalid_mode", mode)
	}
	t, err := m.open(ctx, p)
	if err != nil {
		return err
	}
	defer t.Close()
	if err := t.checkChangeable(); err != nil {
		return err
	}
	return mapErr(t.root.Chmod(t.rel, fs.FileMode(v)), t.abs)
}

// ValidName checks a single file name, such as an uploaded file's.
func ValidName(name string) bool {
	return name != "" && name != "." && name != ".." && len(name) <= 255 &&
		!strings.ContainsAny(name, "/\x00")
}

// Upload writes r to a new file name in folder dir.
func (m *Manager) Upload(ctx context.Context, dir, name string, r io.Reader, overwrite bool) (int64, error) {
	if !ValidName(name) {
		return 0, usererr.New("invalid_name", name)
	}
	t, err := m.open(ctx, path.Join(dir, name))
	if err != nil {
		return 0, err
	}
	defer t.Close()
	flags := os.O_WRONLY | os.O_CREATE | os.O_EXCL
	if overwrite {
		if fi, err := t.root.Lstat(t.rel); err == nil && !fi.Mode().IsRegular() {
			return 0, usererr.New("not_a_file", t.abs)
		}
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}
	f, err := t.root.OpenFile(t.rel, flags, 0o644)
	if err != nil {
		return 0, mapErr(err, t.abs)
	}
	n, err := io.Copy(f, r)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		t.root.Remove(t.rel)
		return n, err
	}
	return n, t.own(t.rel, 0o644)
}

// OpenFile opens a regular file for download.
func (m *Manager) OpenFile(ctx context.Context, p string) (*os.File, fs.FileInfo, error) {
	t, err := m.open(ctx, p)
	if err != nil {
		return nil, nil, err
	}
	defer t.Close()
	f, err := t.root.Open(t.rel)
	if err != nil {
		return nil, nil, mapErr(err, t.abs)
	}
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		f.Close()
		return nil, nil, usererr.New("not_a_file", t.abs)
	}
	return f, fi, nil
}
