package hosting

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// The file lock (other panels call it tamper protection) makes a website's
// code read-only for its PHP and app, and for its SFTP account, which all
// run as the site's user: a hacked plugin cannot change the site's files or
// add new ones, except in the writable folders (uploads, caches).
//
// It works with owners and modes only. Locking walks the site's folder,
// skipping the writable folders and tmp/ and logs/, and gives every entry
// the site's user owns to root, without write permission for the group or
// others. Who could read what stays the same: an entry the site's group
// could read keeps the site's group (the web server, www-data, is in it);
// one only its owner could read (wp-config.php is 0600) gets the site's
// lock group, panel-lock-<uid>, which holds the site's user alone. Set-user
// and set-group bits of files go. Links get the owner their target gets,
// as the web servers follow a link only to a target of the same owner.
//
// What locking changed is written down (lockManifest) before each change,
// so unlocking puts back exactly the owner, group and mode each entry had,
// even after a crash halfway.
//
// The panel's own operations on the site's code unlock it while they run
// and lock it again (WithUnlocked): WordPress updates and other toolkit
// actions, a staging copy pushed to it, Git deploys, backup restores.
// Changes by hand are refused outside the writable folders: the file
// manager (CheckWritable), app installs and imports into the site.

const (
	maxLockEntries = 2_000_000
	// The daily check of the PHP files reads at most this much per site.
	scanMaxFiles = 200_000
	scanMaxBytes = 2 << 30
	scanEvery    = 23 * time.Hour
	maxWritable  = 50
)

// LockPresets are the folders applications write to while they run,
// relative to the site's folder. A path may use * for one folder name.
var LockPresets = map[string][]string{
	"wordpress":   {"public/wp-content/uploads", "public/wp-content/cache", "public/wp-content/upgrade"},
	"joomla":      {"public/images", "public/media", "public/cache", "public/administrator/cache", "public/administrator/logs", "public/tmp"},
	"laravel":     {"storage", "bootstrap/cache"},
	"symfony":     {"var"},
	"codeigniter": {"writable"},
	"drupal":      {"public/sites/*/files"},
	"nextcloud":   {"data", "public/config"},
	"opencart":    {"storage", "public/image/cache", "public/image/catalog"},
	"moodle":      {"moodledata"},
}

// lockPreset returns the preset that fits a site: its app, else its
// rewrite preset.
func lockPreset(site *store.Site) string {
	for _, k := range []string{site.App, site.Rewrite} {
		if k == AppWordPress || k == "woocommerce" {
			return "wordpress"
		}
		if _, ok := LockPresets[k]; ok {
			return k
		}
	}
	return ""
}

var writableRe = regexp.MustCompile(`^[A-Za-z0-9._@+*~-]+(/[A-Za-z0-9._@+*~-]+)*$`)

// normalizeWritable checks folders relative to the site's folder.
func normalizeWritable(in []string) ([]string, error) {
	out := []string{}
	for _, w := range in {
		w = strings.Trim(strings.TrimSpace(strings.ReplaceAll(w, "\\", "/")), "/")
		if w == "" {
			continue
		}
		if !writableRe.MatchString(w) || slices.Contains(strings.Split(w, "/"), "..") ||
			slices.Contains(strings.Split(w, "/"), ".") || path.Clean(w) != w {
			return nil, usererr.New("invalid_writable", w)
		}
		if !slices.Contains(out, w) {
			out = append(out, w)
		}
	}
	if len(out) > maxWritable {
		return nil, usererr.New("too_many_writable")
	}
	return out, nil
}

// lockAlways are the site's own folders that always stay writable: PHP's
// temporary files and sessions, and its error log.
var lockAlways = []string{"tmp", "logs"}

func (m *Manager) lockManifest(id int64) string {
	return filepath.Join(m.paths.FileLock, strconv.FormatInt(id, 10)+".lock")
}
func (m *Manager) lockBaseline(id int64) string {
	return filepath.Join(m.paths.FileLock, strconv.FormatInt(id, 10)+".php.gz")
}
func (m *Manager) lockScanFile(id int64) string {
	return filepath.Join(m.paths.FileLock, strconv.FormatInt(id, 10)+".scan.json")
}

func lockGroupName(uid int) string { return "panel-lock-" + strconv.Itoa(uid) }

// ensureLockGroup creates the site's lock group and puts its user in it.
// It reports whether the group is new: running PHP workers do not have it
// yet.
func (m *Manager) ensureLockGroup(ctx context.Context, site *store.Site, uid int) (int, bool, error) {
	name := lockGroupName(uid)
	created := false
	g, err := user.LookupGroup(name)
	if err != nil {
		if _, err := m.run.Output(ctx, "groupadd", "--system", name); err != nil {
			return 0, false, err
		}
		if g, err = user.LookupGroup(name); err != nil {
			return 0, false, err
		}
		created = true
	}
	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		return 0, false, err
	}
	u, err := user.Lookup(site.SysUser)
	if err != nil {
		return 0, false, err
	}
	ids, _ := u.GroupIds()
	if !slices.Contains(ids, g.Gid) {
		if _, err := m.run.Output(ctx, "usermod", "--append", "--groups", name, site.SysUser); err != nil {
			return 0, false, err
		}
		created = true
	}
	return gid, created, nil
}

// removeLockGroup deletes a site's lock group (the site is being deleted).
func (m *Manager) removeLockGroup(ctx context.Context, uid int) {
	if _, err := user.LookupGroup(lockGroupName(uid)); err == nil {
		m.run.Output(ctx, "groupdel", lockGroupName(uid))
	}
}

// lockEntry is the state of one entry before the lock, to put back.
type lockEntry struct {
	Mode     os.FileMode
	UID, GID int
}

// lockedState returns what locking makes of an entry, given the site's
// user and groups; ok is false when it stays as it is.
func lockedState(fi fs.FileInfo, uid, gid int, siteUID, siteGID, lockGID int) (newUID, newGID int, mode os.FileMode, ok bool) {
	m := fi.Mode()
	perm := m.Perm()
	u, g, o := perm>>6&7, perm>>3&7, perm&7
	special := os.FileMode(0)
	if m.IsDir() {
		special = m & (os.ModeSetgid | os.ModeSticky)
	}
	if uid != siteUID {
		// Not the site's: only write access through the site's groups or
		// for everyone goes.
		g2 := g
		if gid == siteGID || gid == lockGID {
			g2 = g &^ 2
		}
		mode = os.FileMode(u<<6|g2<<3|o&^2) | special
		if mode == m&(os.ModePerm|os.ModeSetgid|os.ModeSticky) && m&os.ModeSetuid == 0 && (m.IsDir() || m&os.ModeSetgid == 0) {
			return uid, gid, m, false
		}
		return uid, gid, mode, true
	}
	ur, gr := u&^2, g&^2
	newGID = siteGID
	if gr == 0 && ur != 0 {
		// Only its owner could read it: the lock group keeps it so.
		newGID, gr = lockGID, ur
	} else {
		gr |= ur
	}
	return 0, newGID, os.FileMode(u<<6|gr<<3|o&^2) | special, true
}

// lockSiteMutex serialises locking and unlocking one site.
func (m *Manager) lockSiteMutex(id int64) *sync.Mutex {
	mu, _ := m.lockSite.LoadOrStore(id, &sync.Mutex{})
	return mu.(*sync.Mutex)
}

func (m *Manager) setLockBusy(id int64, delta int) {
	m.lockMu.Lock()
	defer m.lockMu.Unlock()
	if m.lockBusy == nil {
		m.lockBusy = map[int64]int{}
	}
	m.lockBusy[id] += delta
	if m.lockBusy[id] <= 0 {
		delete(m.lockBusy, id)
	}
}

func (m *Manager) lockIsBusy(id int64) bool {
	m.lockMu.Lock()
	defer m.lockMu.Unlock()
	return m.lockBusy[id] > 0
}

// writableDirs expands a site's writable folders (and tmp, logs) to the
// existing real folders, relative to the site's folder. Missing ones whose
// parent folder exists are created for the site's user when create is set.
// It returns the folders and the patterns that matched nothing.
func writableDirs(root string, patterns []string, uid, gid int, create bool) (map[string]bool, []string) {
	dirs := map[string]bool{}
	var missing []string
	for _, p := range append(slices.Clone(lockAlways), patterns...) {
		matches, _ := filepath.Glob(filepath.Join(root, filepath.FromSlash(p)))
		if len(matches) == 0 && create && !strings.Contains(p, "*") && realDirWithin(root, path.Dir(p)) {
			full := filepath.Join(root, filepath.FromSlash(p))
			if err := os.Mkdir(full, 0o750); err == nil {
				os.Chown(full, uid, gid)
				os.Chmod(full, 0o750)
				matches = []string{full}
			}
		}
		found := false
		for _, mm := range matches {
			rel, err := filepath.Rel(root, mm)
			if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
				continue
			}
			rel = filepath.ToSlash(rel)
			if !realDirWithin(root, rel) {
				continue
			}
			dirs[rel] = true
			found = true
		}
		if !found && !slices.Contains(lockAlways, p) {
			missing = append(missing, p)
		}
	}
	return dirs, missing
}

// realDirWithin reports whether rel (slash-separated, relative to root) is
// a folder reached without following a link: each step is a real folder.
func realDirWithin(root, rel string) bool {
	if rel == "." || rel == "" {
		return true
	}
	cur := root
	for _, part := range strings.Split(rel, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
		cur = filepath.Join(cur, part)
		st, err := os.Lstat(cur)
		if err != nil || !st.IsDir() {
			return false
		}
	}
	return true
}

// inWritable reports whether rel (slash-separated) is one of dirs or below one.
func inWritable(rel string, dirs map[string]bool) bool {
	for d := range dirs {
		if rel == d || strings.HasPrefix(rel, d+"/") {
			return true
		}
	}
	return false
}

// lockResult sums up a lock.
type lockResult struct {
	Entries int
	Missing []string
}

// lockTree locks a site's files (see the top of this file). If it fails
// halfway, what it changed is put back.
func (m *Manager) lockTree(ctx context.Context, site *store.Site, p *store.SiteProtect) (*lockResult, error) {
	root := m.paths.siteRoot(site.Domain)
	uid, gid, err := lookupIDs(site.SysUser)
	if err != nil {
		return nil, err
	}
	if uid == 0 || gid == 0 {
		return nil, fmt.Errorf("refusing to lock the files of %s: its user is root", site.Domain)
	}
	lockGID, newGroup, err := m.ensureLockGroup(ctx, site, uid)
	if err != nil {
		return nil, fmt.Errorf("create the lock group: %w", err)
	}
	if newGroup {
		// PHP-FPM gives new workers the user's groups: start new ones.
		if site.PHPVersion != "" && m.phpInstalled(site.PHPVersion) {
			if err := m.reloadPHP(ctx, site.PHPVersion); err != nil {
				m.log.Warn("file lock: reload PHP", "domain", site.Domain, "err", err)
			}
		}
		if site.AppCommand != "" {
			m.run.Output(ctx, "systemctl", "try-restart", appUnit(site.SysUser))
		}
	}
	dirs, missing := writableDirs(root, p.LockWritable, uid, gid, true)
	if err := fsutil.MkdirAll(m.paths.FileLock, 0o700); err != nil {
		return nil, err
	}
	man := m.lockManifest(site.ID)
	if fileExists(man) {
		// Still locked (or a lock that stopped halfway): undo it first.
		if err := m.unlockTree(ctx, site); err != nil {
			return nil, err
		}
	}
	f, err := os.OpenFile(man, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	w := bufio.NewWriter(f)
	fmt.Fprintf(w, "nefo-filelock 1\nroot %q\nids %d %d %d\n", root, uid, gid, lockGID)
	wdirs := make([]string, 0, len(dirs))
	for d := range dirs {
		wdirs = append(wdirs, d)
	}
	sort.Strings(wdirs)
	for _, d := range wdirs {
		fmt.Fprintf(w, "writable %q\n", d)
	}
	n := 0
	walkErr := filepath.WalkDir(root, func(full string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, full)
		rel = filepath.ToSlash(rel)
		if rel != "." && dirs[rel] {
			return fs.SkipDir
		}
		if n++; n > maxLockEntries {
			return usererr.New("lock_too_many_files", strconv.Itoa(maxLockEntries))
		}
		fi, err := os.Lstat(full)
		if err != nil {
			return err
		}
		ou, og := fsutil.Owner(fi)
		if fi.Mode()&os.ModeSymlink != 0 {
			if ou != uid || linkTargetWritable(root, full, dirs) {
				return nil
			}
			// Before the change: how to put it back.
			fmt.Fprintf(w, "x %d %d %d %q\n", uint32(fi.Mode()), ou, og, rel)
			if err := w.Flush(); err != nil {
				return err
			}
			return os.Lchown(full, 0, -1)
		}
		if !fi.IsDir() && !fi.Mode().IsRegular() {
			return nil // devices, sockets and FIFOs are left alone
		}
		nu, ng, mode, change := lockedState(fi, ou, og, uid, gid, lockGID)
		if !change {
			return nil
		}
		if !(ou == uid && og == gid && mode == fi.Mode()&(os.ModePerm|os.ModeSetgid|os.ModeSticky)) {
			fmt.Fprintf(w, "x %d %d %d %q\n", uint32(fi.Mode()), ou, og, rel)
			if err := w.Flush(); err != nil {
				return err
			}
		}
		if err := os.Lchown(full, nu, ng); err != nil {
			return err
		}
		return os.Chmod(full, mode)
	})
	if walkErr == nil {
		fmt.Fprintf(w, "done %d\n", n)
		walkErr = w.Flush()
	}
	if walkErr == nil {
		walkErr = f.Sync()
	}
	f.Close()
	if walkErr != nil {
		bg := context.WithoutCancel(ctx)
		if err := m.unlockTree(bg, site); err != nil {
			m.log.Warn("file lock: undo a failed lock", "domain", site.Domain, "err", err)
		}
		return nil, walkErr
	}
	if p.LockAlert {
		if err := m.writeBaseline(ctx, site); err != nil {
			m.log.Warn("file lock: PHP files snapshot", "domain", site.Domain, "err", err)
		}
	} else {
		removeIfExists(m.lockBaseline(site.ID))
	}
	removeIfExists(m.lockScanFile(site.ID))
	return &lockResult{Entries: n, Missing: missing}, nil
}

// linkTargetWritable reports whether a link points into a writable folder
// or out of the site's folder: then it keeps the site's user, like its
// target.
func linkTargetWritable(root, link string, dirs map[string]bool) bool {
	t, err := os.Readlink(link)
	if err != nil {
		return true
	}
	if !filepath.IsAbs(t) {
		t = filepath.Join(filepath.Dir(link), t)
	}
	rel, err := filepath.Rel(root, filepath.Clean(t))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return true
	}
	return inWritable(filepath.ToSlash(rel), dirs)
}

// manifest is a parsed lockManifest file.
type manifest struct {
	root          string
	uid, gid, lck int
	writable      map[string]bool
	entries       map[string]lockEntry
	done          bool
}

func readManifest(file string) (*manifest, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	mf := &manifest{writable: map[string]bool{}, entries: map[string]lockEntry{}}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	first := true
	for sc.Scan() {
		line := sc.Text()
		if first {
			if line != "nefo-filelock 1" {
				return nil, errors.New("not a file lock record")
			}
			first = false
			continue
		}
		kind, rest, _ := strings.Cut(line, " ")
		switch kind {
		case "root":
			mf.root, err = strconv.Unquote(rest)
		case "ids":
			_, err = fmt.Sscan(rest, &mf.uid, &mf.gid, &mf.lck)
		case "writable":
			var d string
			if d, err = strconv.Unquote(rest); err == nil {
				mf.writable[d] = true
			}
		case "x":
			var mode uint32
			var e lockEntry
			f := strings.SplitN(rest, " ", 4)
			if len(f) != 4 {
				return nil, errors.New("damaged file lock record")
			}
			if _, err = fmt.Sscan(f[0]+" "+f[1]+" "+f[2], &mode, &e.UID, &e.GID); err == nil {
				var rel string
				if rel, err = strconv.Unquote(f[3]); err == nil {
					e.Mode = os.FileMode(mode)
					mf.entries[rel] = e
				}
			}
		case "done":
			mf.done = true
		}
		if err != nil {
			return nil, fmt.Errorf("damaged file lock record: %w", err)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if first || mf.root == "" {
		return nil, errors.New("empty file lock record")
	}
	return mf, nil
}

// unlockTree puts back what lockTree changed. Entries are restored
// children first, so their folders stay root's until they are done and
// nothing in them can be swapped for a link meanwhile.
func (m *Manager) unlockTree(ctx context.Context, site *store.Site) error {
	root := m.paths.siteRoot(site.Domain)
	uid, gid, err := lookupIDs(site.SysUser)
	if err != nil {
		return err
	}
	mf, err := readManifest(m.lockManifest(site.ID))
	if errors.Is(err, fs.ErrNotExist) {
		mf = nil // not locked by the panel: give root's entries of the site's groups back
	} else if err != nil {
		m.log.Warn("file lock record", "domain", site.Domain, "err", err)
		mf = nil
	}
	lockGID := -1
	if g, err := user.LookupGroup(lockGroupName(uid)); err == nil {
		lockGID, _ = strconv.Atoi(g.Gid)
	}
	dirs := map[string]bool{}
	entries := map[string]lockEntry{}
	if mf != nil {
		if mf.root != root || mf.uid != uid {
			return fmt.Errorf("the file lock record of %s is for another folder or user", site.Domain)
		}
		dirs, entries = mf.writable, mf.entries
	} else {
		for _, d := range lockAlways {
			dirs[d] = true
		}
	}
	type item struct {
		full, rel string
		link      bool
	}
	var list []item
	err = filepath.WalkDir(root, func(full string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		rel, _ := filepath.Rel(root, full)
		rel = filepath.ToSlash(rel)
		if rel != "." && dirs[rel] {
			return fs.SkipDir
		}
		list = append(list, item{full, rel, d.Type()&fs.ModeSymlink != 0})
		return nil
	})
	if err != nil {
		return err
	}
	for i := len(list) - 1; i >= 0; i-- {
		it := list[i]
		fi, err := os.Lstat(it.full)
		if err != nil {
			continue
		}
		link := fi.Mode()&os.ModeSymlink != 0
		ou, og := fsutil.Owner(fi)
		if e, ok := entries[it.rel]; ok {
			if (e.Mode&os.ModeSymlink != 0) != link || e.Mode.IsDir() != fi.IsDir() {
				continue // replaced since (by root): leave it
			}
			if err := os.Lchown(it.full, e.UID, e.GID); err != nil {
				return err
			}
			if !link {
				if err := os.Chmod(it.full, e.Mode&(os.ModePerm|os.ModeSetuid|os.ModeSetgid|os.ModeSticky)); err != nil {
					return err
				}
			}
			continue
		}
		if ou == 0 && (og == gid || (lockGID >= 0 && og == lockGID)) {
			if err := os.Lchown(it.full, uid, gid); err != nil {
				return err
			}
			if mf == nil && !link && og == lockGID {
				// Without the record: what only its owner could read.
				os.Chmod(it.full, fi.Mode().Perm()&^0o077|os.FileMode(0o600))
			}
		}
	}
	removeIfExists(m.lockManifest(site.ID))
	return nil
}

// ---- settings ----

// FileLock are a site's file lock settings.
type FileLock struct {
	On       bool     `json:"on"`
	Writable []string `json:"writable"`
	Alert    bool     `json:"alert"`
}

// FileChange is a PHP file that differs from the snapshot of the lock.
type FileChange struct {
	Path string `json:"path"`
	Kind string `json:"kind"` // changed, added or removed
}

// FileLockInfo is a site's file lock for the UI.
type FileLockInfo struct {
	FileLock
	Preset    string              `json:"preset"`
	Presets   map[string][]string `json:"presets"`
	LockedAt  time.Time           `json:"locked_at,omitzero"`
	Missing   []string            `json:"missing"`
	Busy      bool                `json:"busy"`
	ScanAt    time.Time           `json:"scan_at,omitzero"`
	Changes   []FileChange        `json:"changes"`
	ChangeSum int                 `json:"change_count"`
	Partial   bool                `json:"partial"`
}

// FileLockStatus returns a site's file lock settings and state.
func (m *Manager) FileLockStatus(ctx context.Context, id int64) (*FileLockInfo, error) {
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return nil, err
	}
	p, err := m.store.SiteProtect(ctx, id)
	if err != nil {
		return nil, err
	}
	info := &FileLockInfo{FileLock: FileLock{On: p.LockOn, Writable: p.LockWritable, Alert: p.LockAlert},
		Preset: lockPreset(site), Presets: LockPresets, LockedAt: p.LockedAt, Busy: m.lockIsBusy(id),
		ScanAt: p.ScanAt, Missing: []string{}, Changes: []FileChange{}}
	if info.Writable == nil {
		info.Writable = []string{}
	}
	if !p.LockOn && len(p.LockWritable) == 0 && info.Preset != "" {
		info.Writable = slices.Clone(LockPresets[info.Preset])
	}
	if uid, gid, err := lookupIDs(site.SysUser); err == nil {
		_, info.Missing = writableDirs(m.paths.siteRoot(site.Domain), info.Writable, uid, gid, false)
		if info.Missing == nil {
			info.Missing = []string{}
		}
	}
	if r := m.readScan(id); r != nil {
		info.Changes, info.ChangeSum, info.Partial = r.Changes, r.Total, r.Partial
	}
	return info, nil
}

// SetFileLock locks or unlocks a site's files, or changes its writable
// folders (the site is then locked again with them) or the change alert.
func (m *Manager) SetFileLock(ctx context.Context, id int64, fl FileLock) (*FileLockInfo, error) {
	if !m.Supported() {
		return nil, ErrUnsupported
	}
	writable, err := normalizeWritable(fl.Writable)
	if err != nil {
		return nil, err
	}
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(site.App, "docker:") {
		// A Docker app's files belong to the container's users.
		return nil, usererr.New("lock_docker_app")
	}
	mu := m.lockSiteMutex(id)
	if !mu.TryLock() {
		return nil, usererr.New("lock_busy", site.Domain)
	}
	defer mu.Unlock()
	p, err := m.store.SiteProtect(ctx, id)
	if err != nil {
		return nil, err
	}
	was := *p
	p.LockWritable, p.LockAlert = writable, fl.Alert && fl.On
	switch {
	case fl.On:
		if _, err := m.lockTree(ctx, site, p); err != nil {
			return nil, err
		}
		p.LockOn, p.LockedAt, p.ScanAt, p.ScanChanges = true, m.now(), time.Time{}, 0
	case was.LockOn || fileExists(m.lockManifest(id)):
		if err := m.unlockTree(ctx, site); err != nil {
			return nil, err
		}
		p.LockOn, p.LockedAt, p.ScanAt, p.ScanChanges = false, time.Time{}, time.Time{}, 0
		removeIfExists(m.lockBaseline(id))
		removeIfExists(m.lockScanFile(id))
	}
	if err := m.store.SetSiteProtect(ctx, p); err != nil {
		return nil, err
	}
	return m.FileLockStatus(ctx, id)
}

// ResetFileBaseline takes a new snapshot of the PHP files of a locked site
// (the changes found were the administrator's own).
func (m *Manager) ResetFileBaseline(ctx context.Context, id int64) (*FileLockInfo, error) {
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return nil, err
	}
	p, err := m.store.SiteProtect(ctx, id)
	if err != nil {
		return nil, err
	}
	if !p.LockOn || !p.LockAlert {
		return nil, usererr.New("lock_not_on", site.Domain)
	}
	if err := m.writeBaseline(ctx, site); err != nil {
		return nil, err
	}
	removeIfExists(m.lockScanFile(id))
	p.ScanAt, p.ScanChanges = time.Time{}, 0
	if err := m.store.SetSiteProtect(ctx, p); err != nil {
		return nil, err
	}
	return m.FileLockStatus(ctx, id)
}

// FilesLocked reports whether a site's files are locked.
func (m *Manager) FilesLocked(ctx context.Context, id int64) bool {
	p, err := m.store.SiteProtect(ctx, id)
	return err == nil && p.LockOn
}

// RefuseLocked returns the error for an operation that is refused while a
// site's files are locked (an app install, an import into the site).
func (m *Manager) RefuseLocked(ctx context.Context, site *store.Site) error {
	if m.FilesLocked(ctx, site.ID) {
		return usererr.New("site_files_locked", site.Domain)
	}
	return nil
}

// WithUnlocked runs fn with a site's files unlocked if they are locked, and
// locks them again afterwards, also when fn fails. lg (may be nil) gets a
// line before and after.
func (m *Manager) WithUnlocked(ctx context.Context, siteID int64, lg Logger, fn func() error) error {
	p, err := m.store.SiteProtect(ctx, siteID)
	if err != nil {
		return err
	}
	if !p.LockOn {
		return fn()
	}
	site, err := m.store.SiteByID(ctx, siteID)
	if err != nil {
		return err
	}
	mu := m.lockSiteMutex(siteID)
	mu.Lock()
	defer mu.Unlock()
	m.setLockBusy(siteID, 1)
	defer m.setLockBusy(siteID, -1)
	if lg != nil {
		lg.Logf("Unlocking the files of %s for this change", site.Domain)
	}
	if err := m.unlockTree(ctx, site); err != nil {
		// Halfway: lock everything again rather than leave it half open.
		if _, lerr := m.lockTree(context.WithoutCancel(ctx), site, p); lerr != nil {
			m.log.Warn("file lock: lock again", "domain", site.Domain, "err", lerr)
		}
		return fmt.Errorf("unlock the files of %s: %w", site.Domain, err)
	}
	ferr := fn()
	bg := context.WithoutCancel(ctx)
	if lg != nil {
		lg.Logf("Locking the files of %s again", site.Domain)
	}
	if _, err := m.lockTree(bg, site, p); err != nil {
		m.log.Warn("file lock: lock again", "domain", site.Domain, "err", err)
		if ferr == nil {
			ferr = fmt.Errorf("the files of %s could not be locked again (the panel retries every hour): %w", site.Domain, err)
		}
		return ferr
	}
	p.LockedAt, p.ScanAt, p.ScanChanges = m.now(), time.Time{}, 0
	if err := m.store.SetSiteProtect(bg, p); err != nil {
		m.log.Warn("file lock", "domain", site.Domain, "err", err)
	}
	return ferr
}

// UnlockedFor runs fn with the files of the site whose folder holds path
// unlocked (see WithUnlocked); a path in no site's folder runs fn as is.
func (m *Manager) UnlockedFor(ctx context.Context, abs string, lg Logger, fn func() error) error {
	sites, err := m.store.ListSites(ctx)
	if err != nil {
		return err
	}
	abs = filepath.Clean(abs)
	run := fn
	for _, s := range sites {
		root := m.paths.siteRoot(s.Domain)
		if abs == root || strings.HasPrefix(abs, root+"/") || strings.HasPrefix(root, strings.TrimSuffix(abs, "/")+"/") {
			id, inner := s.ID, run
			run = func() error { return m.WithUnlocked(ctx, id, lg, inner) }
		}
	}
	return run()
}

// CheckWritable is the file manager's check: in a site whose files are
// locked, only its writable folders may change. rel is relative to the
// site's folder; self allows the writable folder itself (to change its
// mode), not only what is in it.
func (m *Manager) CheckWritable(ctx context.Context, site *store.Site, rel string, self bool) error {
	p, err := m.store.SiteProtect(ctx, site.ID)
	if err != nil || !p.LockOn {
		return err
	}
	root := m.paths.siteRoot(site.Domain)
	rel = realRel(root, strings.Trim(path.Clean("/"+filepath.ToSlash(rel)), "/"))
	dirs, _ := writableDirs(root, p.LockWritable, -1, -1, false)
	for d := range dirs {
		if strings.HasPrefix(rel, d+"/") || (self && rel == d) {
			return nil
		}
	}
	return usererr.New("path_locked", filepath.Join(m.paths.siteRoot(site.Domain), rel))
}

// realRel resolves the links on the way to rel (relative to root): what
// the file manager would really change. A path that leads out of root
// resolves to "" (the site's folder itself: locked).
func realRel(root, rel string) string {
	// Component by component, following links (also dangling ones: a
	// write through one creates its target), like the file manager's
	// os.Root does inside the site's folder.
	todo := strings.Split(rel, "/")
	var done []string
	for hops := 0; len(todo) > 0; {
		part := todo[0]
		todo = todo[1:]
		switch part {
		case "", ".":
			continue
		case "..":
			if len(done) == 0 {
				return ""
			}
			done = done[:len(done)-1]
			continue
		}
		cur := filepath.Join(root, filepath.FromSlash(strings.Join(append(done, part), "/")))
		fi, err := os.Lstat(cur)
		if err != nil || fi.Mode()&os.ModeSymlink == 0 {
			done = append(done, part)
			continue
		}
		if hops++; hops > 40 {
			return ""
		}
		target, err := os.Readlink(cur)
		if err != nil {
			return ""
		}
		target = filepath.ToSlash(target)
		if strings.HasPrefix(target, "/") {
			// An absolute link counts only if it stays in the site's folder.
			r, err := filepath.Rel(root, filepath.FromSlash(target))
			if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
				return ""
			}
			done = nil
			target = filepath.ToSlash(r)
		}
		todo = append(strings.Split(target, "/"), todo...)
	}
	return strings.Join(done, "/")
}

// HealFileLocks locks again the sites whose lock an interrupted operation
// (or a crash) left open. It runs from the hourly maintenance loop.
func (m *Manager) HealFileLocks(ctx context.Context) {
	if !m.Supported() {
		return
	}
	prot, err := m.store.SiteProtects(ctx)
	if err != nil {
		return
	}
	for _, p := range prot {
		if !p.LockOn || m.lockIsBusy(p.SiteID) {
			continue
		}
		if mf, err := readManifest(m.lockManifest(p.SiteID)); err == nil && mf.done {
			continue
		}
		site, err := m.store.SiteByID(ctx, p.SiteID)
		if err != nil {
			continue
		}
		mu := m.lockSiteMutex(p.SiteID)
		if !mu.TryLock() {
			continue
		}
		if _, err := m.lockTree(ctx, site, p); err != nil {
			m.log.Warn("file lock: lock again", "domain", site.Domain, "err", err)
		} else {
			m.log.Info("locked the files again", "domain", site.Domain)
		}
		mu.Unlock()
	}
}

// ---- the PHP files snapshot and the daily check ----

// isPHPFile reports whether PHP would run the file.
func isPHPFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".php", ".phtml", ".phar", ".php3", ".php4", ".php5", ".php7", ".php8", ".phps":
		return true
	}
	return false
}

// phpFiles hashes the PHP files of a site (writable folders included, tmp
// and logs not), within the scan limits. partial is set when it stopped at
// a limit.
func (m *Manager) phpFiles(ctx context.Context, site *store.Site) (map[string]string, bool, error) {
	root := m.paths.siteRoot(site.Domain)
	out := map[string]string{}
	var bytes int64
	partial := false
	err := filepath.WalkDir(root, func(full string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) {
				return nil
			}
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, full)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if slices.Contains(lockAlways, rel) {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !isPHPFile(d.Name()) {
			return nil
		}
		if len(out) >= scanMaxFiles || bytes >= scanMaxBytes {
			partial = true
			return fs.SkipAll
		}
		f, err := os.Open(full)
		if err != nil {
			return nil
		}
		h := sha256.New()
		n, err := io.Copy(h, io.LimitReader(f, scanMaxBytes-bytes))
		f.Close()
		if err != nil {
			return nil
		}
		bytes += n
		out[rel] = hex.EncodeToString(h.Sum(nil))
		return nil
	})
	return out, partial, err
}

// writeBaseline takes the snapshot of a site's PHP files.
func (m *Manager) writeBaseline(ctx context.Context, site *store.Site) error {
	files, _, err := m.phpFiles(ctx, site)
	if err != nil {
		return err
	}
	if err := fsutil.MkdirAll(m.paths.FileLock, 0o700); err != nil {
		return err
	}
	tmp := m.lockBaseline(site.ID) + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	gz := gzip.NewWriter(f)
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	w := bufio.NewWriter(gz)
	for _, n := range names {
		fmt.Fprintf(w, "%s %q\n", files[n], n)
	}
	err = w.Flush()
	if err == nil {
		err = gz.Close()
	}
	if err == nil {
		err = f.Sync()
	}
	f.Close()
	if err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, m.lockBaseline(site.ID))
}

func (m *Manager) readBaseline(id int64) (map[string]string, error) {
	f, err := os.Open(m.lockBaseline(id))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	sc := bufio.NewScanner(gz)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		sum, q, ok := strings.Cut(sc.Text(), " ")
		if !ok {
			continue
		}
		if name, err := strconv.Unquote(q); err == nil {
			out[name] = sum
		}
	}
	return out, sc.Err()
}

// scanResult is the last check of a site's PHP files.
type scanResult struct {
	At      time.Time    `json:"at"`
	Total   int          `json:"total"`
	Changes []FileChange `json:"changes"` // the first 100
	Partial bool         `json:"partial"`
	Digest  string       `json:"digest"` // of all changes: an alert only when they change
}

func (m *Manager) readScan(id int64) *scanResult {
	b, err := os.ReadFile(m.lockScanFile(id))
	if err != nil {
		return nil
	}
	var r scanResult
	if json.Unmarshal(b, &r) != nil {
		return nil
	}
	if r.Changes == nil {
		r.Changes = []FileChange{}
	}
	return &r
}

// CheckLockedFiles compares the PHP files of every locked site that asked
// for alerts with its snapshot, at most once a day per site, and calls
// alert with a message when the differences are new. It runs from the
// hourly maintenance loop.
func (m *Manager) CheckLockedFiles(ctx context.Context, alert func(domain, text string)) {
	if !m.Supported() {
		return
	}
	prot, err := m.store.SiteProtects(ctx)
	if err != nil {
		return
	}
	for _, p := range prot {
		if !p.LockOn || !p.LockAlert || m.now().Sub(p.ScanAt) < scanEvery || m.lockIsBusy(p.SiteID) {
			continue
		}
		if ctx.Err() != nil {
			return
		}
		site, err := m.store.SiteByID(ctx, p.SiteID)
		if err != nil {
			continue
		}
		if text, err := m.checkSiteFiles(ctx, site, p); err != nil {
			m.log.Warn("file lock: check PHP files", "domain", site.Domain, "err", err)
		} else if text != "" {
			alert(site.Domain, text)
		}
	}
}

// checkSiteFiles runs one check and returns the alert text, if any.
func (m *Manager) checkSiteFiles(ctx context.Context, site *store.Site, p *store.SiteProtect) (string, error) {
	base, err := m.readBaseline(site.ID)
	if errors.Is(err, fs.ErrNotExist) {
		// Alerts turned on without a snapshot (or it was lost): take one.
		return "", m.writeBaseline(ctx, site)
	}
	if err != nil {
		return "", err
	}
	now, partial, err := m.phpFiles(ctx, site)
	if err != nil {
		return "", err
	}
	var changes []FileChange
	for name, sum := range now {
		if old, ok := base[name]; !ok {
			changes = append(changes, FileChange{name, "added"})
		} else if old != sum {
			changes = append(changes, FileChange{name, "changed"})
		}
	}
	if !partial {
		for name := range base {
			if _, ok := now[name]; !ok {
				changes = append(changes, FileChange{name, "removed"})
			}
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	h := sha256.New()
	for _, c := range changes {
		fmt.Fprintf(h, "%s %s\n", c.Kind, c.Path)
	}
	digest := hex.EncodeToString(h.Sum(nil))
	prev := m.readScan(site.ID)
	res := scanResult{At: m.now(), Total: len(changes), Changes: changes[:min(len(changes), 100)], Partial: partial, Digest: digest}
	if res.Changes == nil {
		res.Changes = []FileChange{}
	}
	b, _ := json.Marshal(res)
	if err := fsutil.WriteFileAtomic(m.lockScanFile(site.ID), b, 0o600); err != nil {
		return "", err
	}
	p.ScanAt, p.ScanChanges = res.At, len(changes)
	if err := m.store.SetSiteProtect(ctx, p); err != nil {
		return "", err
	}
	if len(changes) == 0 || (prev != nil && prev.Digest == digest) {
		return "", nil
	}
	counts := map[string]int{}
	var sample []string
	for _, c := range changes {
		counts[c.Kind]++
		if len(sample) < 5 {
			sample = append(sample, c.Path)
		}
	}
	var parts []string
	for _, k := range []string{"changed", "added", "removed"} {
		if counts[k] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[k], k))
		}
	}
	more := ""
	if len(changes) > len(sample) {
		more = ", …"
	}
	return fmt.Sprintf("PHP files of %s differ from the snapshot taken when its files were locked: %s (%s%s). "+
		"If you made these changes, take a new snapshot in Websites → %s → File lock.",
		site.Domain, strings.Join(parts, ", "), strings.Join(sample, ", "), more, site.Domain), nil
}

// forgetFileLock removes what the file lock keeps for a deleted site.
func (m *Manager) forgetFileLock(ctx context.Context, site *store.Site, uid int) {
	for _, f := range []string{m.lockManifest(site.ID), m.lockBaseline(site.ID), m.lockScanFile(site.ID)} {
		removeIfExists(f)
	}
	if uid > 0 {
		m.removeLockGroup(ctx, uid)
	}
}

// UnlockAllFiles gives every locked site's files back to its user, as they
// were before the lock (the panel is being removed: "nefo unlock-files").
// It also unlocks sites that have a lock record but are not marked locked
// (an interrupted change), removes the lock groups, and marks every site
// unlocked, so a panel that still runs does not lock them again. report is
// called for each site; the error counts the sites that failed.
func (m *Manager) UnlockAllFiles(ctx context.Context, report func(domain string, err error)) (int, error) {
	sites, err := m.store.ListSites(ctx)
	if err != nil {
		return 0, err
	}
	done, failed := 0, 0
	for _, site := range sites {
		p, err := m.store.SiteProtect(ctx, site.ID)
		if err != nil {
			return done, err
		}
		if !p.LockOn && !fileExists(m.lockManifest(site.ID)) {
			// Unlocked before: its lock group may still be there (it is kept
			// between locks), and nothing of the panel needs it any more.
			if uid, _, lerr := lookupIDs(site.SysUser); lerr == nil {
				m.removeLockGroup(ctx, uid)
			}
			continue
		}
		mu := m.lockSiteMutex(site.ID)
		mu.Lock()
		err = m.unlockTree(ctx, site)
		mu.Unlock()
		if err == nil {
			p.LockOn, p.LockAlert, p.LockedAt, p.ScanAt, p.ScanChanges = false, false, time.Time{}, time.Time{}, 0
			err = m.store.SetSiteProtect(ctx, p)
			removeIfExists(m.lockBaseline(site.ID))
			removeIfExists(m.lockScanFile(site.ID))
			if uid, _, lerr := lookupIDs(site.SysUser); lerr == nil {
				m.removeLockGroup(ctx, uid)
			}
		}
		report(site.Domain, err)
		if err != nil {
			failed++
			continue
		}
		done++
	}
	if failed > 0 {
		return done, fmt.Errorf("the files of %d websites could not be unlocked", failed)
	}
	return done, nil
}
