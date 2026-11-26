package siteimport

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
	"time"
)

// budget is what an import may still write, shared by its files and dumps.
type budget struct{ left int64 }

func (b *budget) take(n int64) error {
	b.left -= n
	if b.left < 0 {
		return fmt.Errorf("the import writes more than the limit: %w", ErrTooLarge)
	}
	return nil
}

// skipLog counts the entries an import left out, by reason, with the
// first few names of each.
type skipLog struct {
	counts   map[string]int
	examples map[string][]string
}

// Reasons an entry is left out.
const (
	skipLink    = "link"
	skipHard    = "hardlink"
	skipSpecial = "special"
	skipUnsafe  = "unsafe_name"
	skipLarge   = "too_large"
	skipClash   = "conflict"
)

var skipText = map[string][2]string{
	skipLink:    {"symbolic link", "symbolic links"},
	skipHard:    {"hard link", "hard links"},
	skipSpecial: {"device, FIFO or other special file", "devices, FIFOs and other special files"},
	skipUnsafe:  {"entry with an absolute name or ..", "entries with an absolute name or .."},
	skipLarge:   {"file over the size limit", "files over the size limit"},
	skipClash:   {"entry that clashes with a folder or a link", "entries that clash with a folder or a link"},
}

// count writes "1 file" or "3 files" from a singular and a plural.
func count(n int64, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

func newSkipLog() *skipLog {
	return &skipLog{counts: map[string]int{}, examples: map[string][]string{}}
}

func (s *skipLog) add(reason, name string) {
	s.counts[reason]++
	if len(s.examples[reason]) < 5 {
		if len(name) > 120 {
			name = name[:120] + "…"
		}
		s.examples[reason] = append(s.examples[reason], name)
	}
}

func (s *skipLog) total() int {
	n := 0
	for _, c := range s.counts {
		n += c
	}
	return n
}

// lines describes what was left out, for the job log.
func (s *skipLog) lines() []string {
	var reasons []string
	for r := range s.counts {
		reasons = append(reasons, r)
	}
	sort.Strings(reasons)
	var out []string
	for _, r := range reasons {
		out = append(out, fmt.Sprintf("Left out %s, such as %s", count(int64(s.counts[r]), skipText[r][0], skipText[r][1]), strings.Join(s.examples[r], ", ")))
	}
	return out
}

// extractor writes archive entries into one website's folder, through an
// os.Root: nothing it writes can be outside it, whatever the names say or
// the folder holds.
type extractor struct {
	root     *os.Root
	uid, gid int // -1 keeps root's
	lim      Limits
	budget   *budget
	skips    *skipLog
	files    int
	bytes    int64
	lg       Logger
	next     time.Time
	label    string
}

// fileMode keeps the owner's bits and the read and execute bits of the
// others, at least read and write for the owner: no setuid, setgid or
// sticky bit, no write for the group or the world.
func fileMode(m fs.FileMode) fs.FileMode { return m.Perm()&0o755 | 0o600 }

// dirMode is the same for folders, which the owner must be able to enter.
func dirMode(m fs.FileMode) fs.FileMode { return m.Perm()&0o755 | 0o700 }

// put writes one entry at rel, a clean path relative to the root. name is
// the entry's name in the archive, for messages.
func (x *extractor) put(rel, name string, e *entry) error {
	switch e.Kind {
	case kindDir:
		if rel == "" {
			return nil
		}
		if err := x.mkdirAll(rel, dirMode(e.Mode), name, true); err != nil && !errors.Is(err, errSkip) {
			return err
		}
	case kindFile:
		return x.file(rel, name, e)
	case kindSymlink:
		x.skips.add(skipLink, name)
	case kindHardlink:
		x.skips.add(skipHard, name)
	default:
		x.skips.add(skipSpecial, name)
	}
	return nil
}

// errSkip means the entry was left out (and logged).
var errSkip = errors.New("skipped")

// mkdirAll makes rel and its missing parents, owned by the site's user. An
// existing part that is not a folder (a file, or a link the site made)
// stops it: the entry is left out. An existing folder keeps its mode,
// unless explicit (the archive's own entry for it).
func (x *extractor) mkdirAll(rel string, mode fs.FileMode, name string, explicit bool) error {
	parts := strings.Split(rel, "/")
	for i := range parts {
		p := strings.Join(parts[:i+1], "/")
		m := fs.FileMode(0o755)
		if i == len(parts)-1 {
			m = mode
		}
		fi, err := x.root.Lstat(p)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			if err := x.root.Mkdir(p, 0o700); err != nil {
				return err
			}
		case err != nil:
			return err
		case fi.IsDir():
			if i < len(parts)-1 || !explicit {
				continue
			}
		default:
			x.skips.add(skipClash, name)
			return errSkip
		}
		if err := x.root.Chmod(p, m); err != nil {
			return err
		}
		if x.uid >= 0 {
			if err := x.root.Lchown(p, x.uid, x.gid); err != nil {
				return err
			}
		}
	}
	return nil
}

func (x *extractor) file(rel, name string, e *entry) error {
	if e.Size > x.lim.MaxFile {
		x.skips.add(skipLarge, name)
		return nil
	}
	if dir := path.Dir(rel); dir != "." {
		if err := x.mkdirAll(dir, 0o755, name, false); err != nil {
			if errors.Is(err, errSkip) {
				return nil
			}
			return err
		}
	}
	if fi, err := x.root.Lstat(rel); err == nil {
		if fi.IsDir() {
			x.skips.add(skipClash, name)
			return nil
		}
		// A file of the same name earlier in the archive: the later wins.
		// Removing it first means a link there is never written through.
		if err := x.root.Remove(rel); err != nil {
			return err
		}
	}
	f, err := x.root.OpenFile(rel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	limit := min(x.lim.MaxFile, max(x.budget.left, 0))
	n, err := io.Copy(f, io.LimitReader(e.R, limit+1))
	if err == nil && n > limit {
		f.Close()
		x.root.Remove(rel)
		if n > x.lim.MaxFile {
			x.skips.add(skipLarge, name)
			return nil
		}
		return x.budget.take(n)
	}
	if err == nil {
		// Through the open file, never through the name.
		err = f.Chmod(fileMode(e.Mode))
	}
	if err == nil && x.uid >= 0 {
		err = f.Chown(x.uid, x.gid)
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		x.root.Remove(rel)
		return err
	}
	if err := x.budget.take(n); err != nil {
		return err
	}
	if !e.ModTime.IsZero() {
		x.root.Chtimes(rel, e.ModTime, e.ModTime)
	}
	x.files++
	x.bytes += n
	if x.lg != nil && time.Now().After(x.next) {
		x.next = time.Now().Add(5 * time.Second)
		x.lg.Logf("%s: %s (%s) so far", x.label, count(int64(x.files), "file", "files"), humanBytes(x.bytes))
	}
	return nil
}
