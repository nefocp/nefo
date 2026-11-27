package files

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/secure"
	"github.com/nefocp/nefo/internal/usererr"
)

// The trash keeps deleted items, one folder per scope:
//
//	/var/lib/panel-trash/<scope>/<entry id>/meta.json  what it was and where
//	/var/lib/panel-trash/<scope>/<entry id>/payload    the item itself
//
// <scope> is the website's domain, or "_server" for the rest of the file
// system. Every folder of the trash is owned by root with mode 0700, so a
// website's user cannot reach its deleted files. The items keep their own
// owner and mode, and are moved back on restore.
const (
	// DefaultTrashDir is where deleted items wait.
	DefaultTrashDir = "/var/lib/panel-trash"
	// TrashMaxAge is how long items stay in the trash; PurgeTrash removes
	// older ones.
	TrashMaxAge = 30 * 24 * time.Hour

	serverScopeID = "_server"
	trashPayload  = "payload"
	trashMetaFile = "meta.json"
	maxMetaSize   = 64 << 10
	maxTrashList  = 1000
	// Moving to the trash across file systems means copying; larger items
	// must be deleted permanently instead.
	maxTrashCopy = 1 << 30
)

var (
	scopeIDPattern = regexp.MustCompile(`^(_server|[a-z0-9][a-z0-9._-]{0,252})$`)
	entryIDPattern = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)
)

// trashMeta is stored next to each item. It is checked on every read, as
// if anyone could have written it.
type trashMeta struct {
	Path      string    `json:"path"`  // relative to the scope's folder, with /
	Scope     string    `json:"scope"` // must match the folder it is in
	Type      string    `json:"type"`
	UID       int       `json:"uid"`
	GID       int       `json:"gid"`
	Mode      string    `json:"mode"`
	Size      int64     `json:"size"`
	DeletedAt time.Time `json:"deleted_at"`
	// The scope's owner when the item was deleted: if the website has been
	// created again since, its new user takes over the restored files.
	ScopeUID int `json:"scope_uid"`
	ScopeGID int `json:"scope_gid"`
}

// TrashEntry is one deleted item.
type TrashEntry struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"` // where it was, and goes back to
	Type      string    `json:"type"`
	Size      int64     `json:"size"`
	Mode      string    `json:"mode"`
	Owner     string    `json:"owner"`
	Group     string    `json:"group"`
	DeletedAt time.Time `json:"deleted_at"`
	Occupied  bool      `json:"occupied"` // something is at Path now: restore is refused
}

// TrashListing is the trash of one scope.
type TrashListing struct {
	Site       string       `json:"site,omitempty"`
	Root       string       `json:"root"`
	Entries    []TrashEntry `json:"entries"`
	Count      int          `json:"count"`
	TotalSize  int64        `json:"total_size"`
	Truncated  bool         `json:"truncated"`
	MaxAgeDays int          `json:"max_age_days"`
}

// scopeID names a scope's trash folder.
func scopeID(s Scope) (string, error) {
	id := s.Site
	if id == "" {
		if s.Root != "/" {
			return "", fmt.Errorf("no trash for folder %s", s.Root)
		}
		id = serverScopeID
	}
	if !scopeIDPattern.MatchString(id) || id == "." || id == ".." {
		return "", fmt.Errorf("invalid trash scope %q", id)
	}
	return id, nil
}

// openBin opens the trash folder of a scope, creating it if asked. It
// returns nil, nil when it does not exist and create is false.
func (m *Manager) openBin(id string, create bool) (*os.Root, error) {
	dir := filepath.Join(m.trashDir, id)
	if create {
		if err := os.MkdirAll(m.trashDir, 0o700); err != nil {
			return nil, err
		}
		if err := os.Chmod(m.trashDir, 0o700); err != nil {
			return nil, err
		}
		if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
	}
	fi, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) && !create {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("%s is not a folder", dir)
	}
	if create {
		if err := os.Chmod(dir, 0o700); err != nil {
			return nil, err
		}
	}
	return os.OpenRoot(dir)
}

// trashItem moves t's item to the trash.
func (m *Manager) trashItem(t *target) error {
	sid, err := scopeID(t.scope)
	if err != nil {
		return err
	}
	m.trashMu.Lock()
	defer m.trashMu.Unlock()
	bin, err := m.openBin(sid, true)
	if err != nil {
		return err
	}
	defer bin.Close()
	_, err = moveToTrash(t.root, t.rel, t.scope, sid, bin, time.Now())
	if errors.Is(err, errTrashTooLarge) {
		return usererr.New("trash_too_large", t.abs)
	}
	return mapErr(err, t.abs)
}

// moveToTrash moves item rel of root (a scope's folder) into bin and
// returns the new entry's id.
func moveToTrash(root *os.Root, rel string, scope Scope, sid string, bin *os.Root, now time.Time) (string, error) {
	fi, err := root.Lstat(rel)
	if err != nil {
		return "", err
	}
	id, err := newEntry(bin, now)
	if err != nil {
		return "", err
	}
	uid, gid := fsutil.Owner(fi)
	meta := &trashMeta{Path: rel, Scope: sid, Type: entryType(fi.Mode()), UID: uid, GID: gid,
		Mode: modeString(fi.Mode()), Size: -1, DeletedAt: now.UTC(), ScopeUID: scope.UID, ScopeGID: scope.GID}
	if fi.Mode().IsRegular() {
		meta.Size = fi.Size()
	}
	// The description goes first, so an item is never in the trash
	// without one.
	if err := writeMeta(bin, id, meta); err != nil {
		bin.RemoveAll(id)
		return "", err
	}
	if err := moveOut(root, rel, bin, id); err != nil {
		if !errors.Is(err, errSourceLeft) {
			bin.RemoveAll(id)
		}
		return "", err
	}
	if meta.Type == "dir" {
		meta.Size = treeSize(bin, path.Join(id, trashPayload))
		writeMeta(bin, id, meta) // the size is only shown, so a failure does not matter
	}
	return id, nil
}

// moveEntry is renameEntry; tests replace it to try the copy used across
// file systems.
var moveEntry = renameEntry

// errTrashTooLarge refuses to copy a large item to the trash on another
// file system.
var errTrashTooLarge = errors.New("too large to copy to the trash")

// errSourceLeft means an item was copied to the trash but could not be
// fully removed from its place; the copy is kept so nothing is lost.
var errSourceLeft = errors.New("the item was copied to the trash but some of it could not be removed")

// moveOut moves item rel of root to <id>/payload in bin. The rename works
// on open folders, so a symbolic link planted in a parent cannot redirect
// it; across file systems the item is copied, then removed.
func moveOut(root *os.Root, rel string, bin *os.Root, id string) error {
	parent, err := openDir(root, path.Dir(rel))
	if err != nil {
		return err
	}
	defer parent.Close()
	dst, err := openDir(bin, id)
	if err != nil {
		return err
	}
	defer dst.Close()
	err = moveEntry(parent, path.Base(rel), dst, trashPayload)
	if err == nil || !isCrossDevice(err) {
		return err
	}
	if treeSize(root, rel) > maxTrashCopy {
		return errTrashTooLarge
	}
	if err := copyTree(root, rel, bin, path.Join(id, trashPayload)); err != nil {
		return err
	}
	if err := root.RemoveAll(rel); err != nil {
		return fmt.Errorf("%w: %v", errSourceLeft, err)
	}
	return nil
}

// openDir opens folder rel of root and checks that it is one. It never
// waits on a FIFO put in the folder's place.
func openDir(root *os.Root, rel string) (*os.File, error) {
	f, err := root.OpenFile(rel, os.O_RDONLY|openDirectory, 0)
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if !fi.IsDir() {
		f.Close()
		return nil, usererr.New("restore_parent_not_folder", rel)
	}
	return f, nil
}

func newEntry(bin *os.Root, now time.Time) (string, error) {
	for range 10 {
		id := now.UTC().Format("20060102-150405") + "-" + secure.String(8, secure.LowerAlnum)
		err := bin.Mkdir(id, 0o700)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		return id, bin.Chmod(id, 0o700)
	}
	return "", errors.New("could not create a trash entry")
}

func writeMeta(bin *os.Root, id string, meta *trashMeta) error {
	b, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	tmp := path.Join(id, ".meta.tmp")
	if err := bin.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := bin.Chmod(tmp, 0o600); err != nil {
		return err
	}
	return bin.Rename(tmp, path.Join(id, trashMetaFile))
}

// readMeta reads and checks the description of entry id of scope sid.
func readMeta(bin *os.Root, sid, id string) (*trashMeta, error) {
	if !entryIDPattern.MatchString(id) {
		return nil, usererr.New("trash_not_found", id)
	}
	fi, err := bin.Lstat(path.Join(id, trashMetaFile))
	if errors.Is(err, fs.ErrNotExist) {
		if _, err := bin.Lstat(id); errors.Is(err, fs.ErrNotExist) {
			return nil, usererr.New("trash_not_found", id)
		}
	}
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > maxMetaSize {
		return nil, usererr.New("trash_damaged", id)
	}
	b, err := bin.ReadFile(path.Join(id, trashMetaFile))
	if err != nil {
		return nil, usererr.New("trash_damaged", id)
	}
	var meta trashMeta
	if err := json.Unmarshal(b, &meta); err != nil || meta.Scope != sid || !validRel(meta.Path) {
		return nil, usererr.New("trash_damaged", id)
	}
	if _, err := bin.Lstat(path.Join(id, trashPayload)); err != nil {
		return nil, usererr.New("trash_damaged", id)
	}
	return &meta, nil
}

// validRel accepts a clean path relative to a scope: no "..", no leading
// slash, not the scope's folder itself.
func validRel(p string) bool {
	return p != "" && p != "." && len(p) <= 4096 && fs.ValidPath(p) && !strings.ContainsRune(p, 0)
}

// ListTrash returns the trash of the scope that holds path p, newest first.
func (m *Manager) ListTrash(ctx context.Context, p string) (*TrashListing, error) {
	t, err := m.open(ctx, p)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	sid, err := scopeID(t.scope)
	if err != nil {
		return nil, err
	}
	l := &TrashListing{Site: t.scope.Site, Root: t.scope.Root, Entries: []TrashEntry{}, MaxAgeDays: int(TrashMaxAge / (24 * time.Hour))}
	bin, err := m.openBin(sid, false)
	if err != nil || bin == nil {
		return l, err
	}
	defer bin.Close()
	err = listBin(bin, sid, t.root, t.scope.Root, l)
	return l, err
}

func listBin(bin *os.Root, sid string, root *os.Root, base string, l *TrashListing) error {
	d, err := bin.Open(".")
	if err != nil {
		return err
	}
	des, err := d.ReadDir(-1)
	d.Close()
	if err != nil {
		return err
	}
	names := newNameCache()
	for _, de := range des {
		if !de.IsDir() {
			continue
		}
		meta, err := readMeta(bin, sid, de.Name())
		if err != nil {
			continue // damaged or half-written: only Empty and the purge remove it
		}
		e := TrashEntry{ID: de.Name(), Path: path.Join(base, meta.Path), Type: meta.Type, Size: max(meta.Size, 0),
			Mode: meta.Mode, Owner: names.user(meta.UID), Group: names.group(meta.GID), DeletedAt: meta.DeletedAt}
		if _, err := root.Lstat(meta.Path); err == nil {
			e.Occupied = true
		}
		l.Entries = append(l.Entries, e)
		l.TotalSize += e.Size
	}
	sort.Slice(l.Entries, func(i, j int) bool { return l.Entries[i].DeletedAt.After(l.Entries[j].DeletedAt) })
	l.Count = len(l.Entries)
	if len(l.Entries) > maxTrashList {
		l.Entries, l.Truncated = l.Entries[:maxTrashList], true
	}
	return nil
}

// RestoreTrash puts entry id of the trash of p's scope back where it was
// and returns that path. Missing parent folders are created with the
// owner and mode of the scope's folder; an item in the way stops it.
func (m *Manager) RestoreTrash(ctx context.Context, p, id string) (string, error) {
	t, err := m.open(ctx, p)
	if err != nil {
		return "", err
	}
	defer t.Close()
	sid, err := scopeID(t.scope)
	if err != nil {
		return "", err
	}
	m.trashMu.Lock()
	defer m.trashMu.Unlock()
	bin, err := m.openBin(sid, false)
	if err != nil {
		return "", err
	}
	if bin == nil {
		return "", usererr.New("trash_not_found", id)
	}
	defer bin.Close()
	return restoreEntry(t.root, t.scope, sid, bin, id, m.trashDir)
}

// restoreEntry moves entry id of bin back into root. Nothing is put back
// inside trashDir.
func restoreEntry(root *os.Root, scope Scope, sid string, bin *os.Root, id, trashDir string) (string, error) {
	meta, err := readMeta(bin, sid, id)
	if err != nil {
		return "", err
	}
	rel := meta.Path
	abs := path.Join(scope.Root, rel)
	if protected[abs] || abs == trashDir || strings.HasPrefix(abs, trashDir+"/") {
		return "", usererr.New("path_protected", abs)
	}
	if scope.Guard != nil {
		if err := scope.Guard(rel, false); err != nil {
			return "", err
		}
	}
	// "Not a folder" means a parent is a file: mkdirParents says so.
	if _, err := root.Lstat(rel); err == nil {
		return "", usererr.New("restore_exists", abs)
	} else if !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, syscall.ENOTDIR) {
		return "", mapErr(err, abs)
	}
	if err := mkdirParents(root, scope.Root, path.Dir(rel)); err != nil {
		return "", mapErr(err, abs)
	}
	parent, err := openDir(root, path.Dir(rel))
	if err != nil {
		return "", mapErr(err, abs)
	}
	defer parent.Close()
	src, err := openDir(bin, id)
	if err != nil {
		return "", err
	}
	defer src.Close()
	// The check above leaves a short gap in which the website's user could
	// put something at rel, which the rename would replace; that can only
	// lose what that user just put there.
	err = moveEntry(src, trashPayload, parent, path.Base(rel))
	if err != nil && isCrossDevice(err) {
		err = copyTree(bin, path.Join(id, trashPayload), root, rel)
		if err != nil && !errors.Is(err, fs.ErrExist) {
			root.RemoveAll(rel) // the part copied so far
		}
	}
	if err != nil {
		return "", mapErr(err, abs)
	}
	if scope.Site != "" && meta.ScopeUID >= 0 && scope.UID >= 0 && (meta.ScopeUID != scope.UID || meta.ScopeGID != scope.GID) {
		reown(root, rel, meta.ScopeUID, meta.ScopeGID, scope.UID, scope.GID)
	}
	bin.RemoveAll(id)
	return abs, nil
}

// mkdirParents creates the missing folders of dir, one by one, with the
// owner and mode of the scope's folder. Existing links are followed only
// inside the scope.
func mkdirParents(root *os.Root, base, dir string) error {
	if dir == "." {
		return nil
	}
	top, err := root.Stat(".")
	if err != nil {
		return err
	}
	mode := fileMode(top)
	uid, gid := fsutil.Owner(top)
	cur := ""
	for _, part := range strings.Split(dir, "/") {
		cur = path.Join(cur, part)
		fi, err := root.Lstat(cur)
		switch {
		case err == nil && fi.Mode()&fs.ModeSymlink != 0:
			st, err := root.Stat(cur)
			if err != nil {
				return err
			}
			if !st.IsDir() {
				return usererr.New("restore_parent_not_folder", path.Join(base, cur))
			}
			continue
		case err == nil && fi.IsDir():
			continue
		case err == nil:
			return usererr.New("restore_parent_not_folder", path.Join(base, cur))
		case !errors.Is(err, fs.ErrNotExist):
			return err
		}
		if err := root.Mkdir(cur, 0o700); err != nil {
			return err
		}
		if uid >= 0 {
			if err := root.Lchown(cur, uid, gid); err != nil {
				return err
			}
		}
		if err := root.Chmod(cur, mode); err != nil {
			return err
		}
	}
	return nil
}

// reown gives the restored items of a website created again since they
// were deleted to its new user: items owned by the old user (or group)
// get the new one.
func reown(root *os.Root, rel string, oldUID, oldGID, uid, gid int) {
	one := func(p string) {
		fi, err := root.Lstat(p)
		if err != nil {
			return
		}
		fu, fg := fsutil.Owner(fi)
		nu, ng := fu, fg
		if fu == oldUID {
			nu = uid
		}
		if fg == oldGID {
			ng = gid
		}
		if nu == fu && ng == fg {
			return
		}
		root.Lchown(p, nu, ng)
		if fi.Mode().IsRegular() && fi.Mode()&(fs.ModeSetuid|fs.ModeSetgid) != 0 {
			root.Chmod(p, fileMode(fi)) // chown clears setuid and setgid
		}
	}
	// A restored link is changed itself, never followed.
	one(rel)
	if fi, err := root.Lstat(rel); err != nil || !fi.IsDir() {
		return
	}
	walkRoot(root, rel, func(p string, _ fs.DirEntry) error {
		one(p)
		return nil
	})
}

// DeleteTrash removes entry id of the trash of p's scope for good.
func (m *Manager) DeleteTrash(ctx context.Context, p, id string) error {
	t, err := m.open(ctx, p)
	if err != nil {
		return err
	}
	defer t.Close()
	sid, err := scopeID(t.scope)
	if err != nil {
		return err
	}
	if !entryIDPattern.MatchString(id) {
		return usererr.New("trash_not_found", id)
	}
	m.trashMu.Lock()
	defer m.trashMu.Unlock()
	bin, err := m.openBin(sid, false)
	if err != nil {
		return err
	}
	if bin == nil {
		return usererr.New("trash_not_found", id)
	}
	defer bin.Close()
	if fi, err := bin.Lstat(id); err != nil || !fi.IsDir() {
		return usererr.New("trash_not_found", id)
	}
	return bin.RemoveAll(id)
}

// EmptyTrash removes everything in the trash of p's scope and returns the
// number of entries removed.
func (m *Manager) EmptyTrash(ctx context.Context, p string) (int, error) {
	t, err := m.open(ctx, p)
	if err != nil {
		return 0, err
	}
	defer t.Close()
	sid, err := scopeID(t.scope)
	if err != nil {
		return 0, err
	}
	m.trashMu.Lock()
	defer m.trashMu.Unlock()
	return removeEntries(ctx, filepath.Join(m.trashDir, sid), func(string, time.Time) bool { return true })
}

// PurgeTrash removes the items deleted more than olderThan ago from every
// scope's trash and returns how many it removed. It is meant to run once a
// day with TrashMaxAge.
func (m *Manager) PurgeTrash(ctx context.Context, olderThan time.Duration) (int, error) {
	m.trashMu.Lock()
	defer m.trashMu.Unlock()
	des, err := os.ReadDir(m.trashDir)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	cutoff := time.Now().Add(-olderThan)
	total := 0
	var errs []error
	for _, de := range des {
		if !de.IsDir() || !scopeIDPattern.MatchString(de.Name()) {
			continue
		}
		dir := filepath.Join(m.trashDir, de.Name())
		n, err := removeEntries(ctx, dir, func(_ string, deleted time.Time) bool { return deleted.Before(cutoff) })
		total += n
		if err != nil {
			errs = append(errs, err)
		}
		os.Remove(dir) // only succeeds once it is empty
	}
	return total, errors.Join(errs...)
}

// DropSiteTrash removes the trash of a website, for when the website is
// deleted.
func (m *Manager) DropSiteTrash(domain string) error {
	if !scopeIDPattern.MatchString(domain) || domain == serverScopeID {
		return fmt.Errorf("invalid trash scope %q", domain)
	}
	m.trashMu.Lock()
	defer m.trashMu.Unlock()
	return os.RemoveAll(filepath.Join(m.trashDir, domain))
}

// removeEntries removes the entries of the trash folder dir that remove
// accepts, given their deletion time (from meta.json, or the folder's
// modification time when that is unreadable).
func removeEntries(ctx context.Context, dir string, remove func(id string, deleted time.Time) bool) (int, error) {
	bin, err := os.OpenRoot(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer bin.Close()
	d, err := bin.Open(".")
	if err != nil {
		return 0, err
	}
	des, err := d.ReadDir(-1)
	d.Close()
	if err != nil {
		return 0, err
	}
	sid := filepath.Base(dir)
	n := 0
	for _, de := range des {
		if err := ctx.Err(); err != nil {
			return n, err
		}
		name := de.Name()
		var deleted time.Time
		if meta, err := readMeta(bin, sid, name); err == nil {
			deleted = meta.DeletedAt
		} else if fi, err := de.Info(); err == nil {
			deleted = fi.ModTime()
		}
		if !remove(name, deleted) {
			continue
		}
		if err := bin.RemoveAll(name); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// treeSize adds up the sizes of the files under rel, without following
// symbolic links.
func treeSize(root *os.Root, rel string) int64 {
	fi, err := root.Lstat(rel)
	if err != nil {
		return 0
	}
	if !fi.IsDir() {
		if fi.Mode().IsRegular() {
			return fi.Size()
		}
		return 0
	}
	var n int64
	walkRoot(root, rel, func(p string, d fs.DirEntry) error {
		if d.Type().IsRegular() {
			if info, err := root.Lstat(p); err == nil && info.Mode().IsRegular() {
				n += info.Size()
			}
		}
		return nil
	})
	return n
}

// walkRoot calls fn for every item under folder rel of root (not rel
// itself), in name order, without following symbolic links. Folders are
// opened with openDir, so one replaced by a FIFO cannot block the walk, and
// unreadable sub-folders are skipped. fn may return fs.SkipDir for a folder
// or fs.SkipAll; any other error stops the walk and is returned.
func walkRoot(root *os.Root, rel string, fn func(p string, d fs.DirEntry) error) error {
	err := walkDir(root, rel, fn, true)
	if errors.Is(err, fs.SkipAll) {
		return nil
	}
	return err
}

func walkDir(root *os.Root, dir string, fn func(string, fs.DirEntry) error, top bool) error {
	f, err := openDir(root, dir)
	if err != nil {
		if top {
			return err
		}
		return nil
	}
	des, err := f.ReadDir(-1)
	f.Close()
	if err != nil && top {
		return err
	}
	sort.Slice(des, func(i, j int) bool { return des[i].Name() < des[j].Name() })
	for _, d := range des {
		p := path.Join(dir, d.Name())
		err := fn(p, d)
		if errors.Is(err, fs.SkipDir) {
			continue
		}
		if err != nil {
			return err
		}
		if d.IsDir() {
			if err := walkDir(root, p, fn, false); err != nil {
				return err
			}
		}
	}
	return nil
}

// copyTree copies item sp of src to dp of dst (which must not exist), with
// modes, owners and modification times. Symbolic links are copied as
// links; sockets and devices are left out. Both sides are os.Roots, so
// nothing outside them is read or written.
func copyTree(src *os.Root, sp string, dst *os.Root, dp string) error {
	fi, err := src.Lstat(sp)
	if err != nil {
		return err
	}
	uid, gid := fsutil.Owner(fi)
	switch {
	case fi.IsDir():
		if err := dst.Mkdir(dp, 0o700); err != nil {
			return err
		}
		d, err := openDir(src, sp)
		if err != nil {
			return err
		}
		des, err := d.ReadDir(-1)
		d.Close()
		if err != nil {
			return err
		}
		for _, de := range des {
			if err := copyTree(src, path.Join(sp, de.Name()), dst, path.Join(dp, de.Name())); err != nil {
				return err
			}
		}
		if uid >= 0 {
			if err := dst.Lchown(dp, uid, gid); err != nil {
				return err
			}
		}
		if err := dst.Chmod(dp, fileMode(fi)); err != nil {
			return err
		}
		dst.Chtimes(dp, fi.ModTime(), fi.ModTime())
		return nil
	case fi.Mode().IsRegular():
		in, err := openRegular(src, sp)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := dst.OpenFile(dp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, in)
		if err == nil && uid >= 0 {
			err = out.Chown(uid, gid)
		}
		if err == nil {
			err = out.Chmod(fileMode(fi))
		}
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
		dst.Chtimes(dp, fi.ModTime(), fi.ModTime())
		return nil
	case fi.Mode()&fs.ModeSymlink != 0:
		target, err := src.Readlink(sp)
		if err != nil {
			return err
		}
		if err := dst.Symlink(target, dp); err != nil {
			return err
		}
		if uid >= 0 {
			return dst.Lchown(dp, uid, gid)
		}
		return nil
	}
	return nil
}

// fileMode keeps the permission and setuid, setgid and sticky bits.
func fileMode(fi fs.FileInfo) fs.FileMode {
	return fi.Mode() & (fs.ModePerm | fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky)
}

func modeString(m fs.FileMode) string {
	return fmt.Sprintf("%04o", m.Perm()|specialBits(m))
}
