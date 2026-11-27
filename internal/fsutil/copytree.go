package fsutil

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path"
	"slices"
	"strconv"
	"time"
)

// Logger receives progress messages for a job log.
type Logger interface {
	Logf(format string, args ...any)
}

// CopyOptions tells CopyTree how to copy.
type CopyOptions struct {
	// UID and GID own the copies; a UID of -1 leaves the owner to the
	// process that copies.
	UID, GID int
	// SrcUID is the owner of the source folder, or -1 when unknown. Files of
	// other users that SrcUID could not read are left out: the copy belongs
	// to another user, who must not get them through it. SrcUID reads a
	// file of another owner that is world-readable, or group-readable by
	// one of its groups (a website whose files are locked: root owns them,
	// see hosting/filelock.go).
	SrcUID int
	// Skip are paths (relative, with slashes) that are not copied. With
	// Mirror, they are not removed from the destination either.
	Skip []string
	// Transform rewrites files on the way, by path. They are at most 1 MB.
	Transform map[string]func([]byte) ([]byte, error)
	// Mirror makes the destination like the source, as rsync --delete
	// does: what the source does not have is removed from the destination
	// (except Skip paths and symbolic links, which the source copy never
	// had), and files with the same size and modification time are left as
	// they are.
	Mirror bool
	// Links, if set, copies symbolic links as links (their targets are
	// never followed): it gets a link's target and returns the target of
	// the copy, or "" to leave the link out. A path that already exists in
	// the destination stays as it is. Nil leaves every link out.
	Links func(target string) string
	Log   Logger // nil for no progress messages
}

// CopyStats counts what CopyTree did.
type CopyStats struct {
	Files     int   // files written
	Bytes     int64 // bytes written
	Unchanged int   // files left as they were (Mirror)
	Removed   int   // files and folders removed from the destination (Mirror)
	Skipped   []string
}

// CopyTree copies the content of srcDir into dstDir, as root, inside two
// os.Roots: no symbolic link can lead the copy out of either folder.
// Symbolic links (unless Links is set), devices and FIFOs are left out, and
// so are files of other users that SrcUID could not read (a hard link to a
// file of root, say).
// A path that is a symbolic link in the destination is left as it is.
//
// Files are written next to their final name and renamed into place, so a
// reader never sees half a file and a hard link in the destination never
// carries the new content elsewhere. Modification times are kept, except
// for transformed files.
func CopyTree(ctx context.Context, srcDir, dstDir string, o CopyOptions) (*CopyStats, error) {
	src, err := os.OpenRoot(srcDir)
	if err != nil {
		return nil, err
	}
	defer src.Close()
	dst, err := os.OpenRoot(dstDir)
	if err != nil {
		return nil, err
	}
	defer dst.Close()
	c := &copier{src: src, dst: dst, o: o, st: &CopyStats{}, groups: userGroups(o.SrcUID)}
	err = fs.WalkDir(src.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if p != "." && slices.Contains(o.Skip, p) {
			c.skip(p)
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		switch {
		case d.IsDir():
			if p != "." {
				if ok, err := c.dir(p, d); err != nil || !ok {
					if err == nil {
						err = fs.SkipDir
					}
					return err
				}
			}
			if o.Mirror {
				return c.prune(p)
			}
			return nil
		case d.Type().IsRegular():
			return c.file(p)
		case d.Type()&fs.ModeSymlink != 0 && o.Links != nil:
			return c.link(p)
		default:
			c.skip(p)
			return nil
		}
	})
	return c.st, err
}

type copier struct {
	src, dst *os.Root
	o        CopyOptions
	st       *CopyStats
	tmpN     int
	groups   map[int]bool // SrcUID's groups
}

// userGroups returns the groups of the account uid (none if unknown).
func userGroups(uid int) map[int]bool {
	out := map[int]bool{}
	if uid < 0 {
		return out
	}
	u, err := user.LookupId(strconv.Itoa(uid))
	if err != nil {
		return out
	}
	ids, _ := u.GroupIds()
	for _, s := range append(ids, u.Gid) {
		if g, err := strconv.Atoi(s); err == nil {
			out[g] = true
		}
	}
	return out
}

// readable reports whether SrcUID could read the file.
func (c *copier) readable(fi fs.FileInfo) bool {
	owner, group := Owner(fi)
	perm := fi.Mode().Perm()
	return c.o.SrcUID < 0 || owner == c.o.SrcUID || perm&0o004 != 0 || (perm&0o040 != 0 && c.groups[group])
}

func (c *copier) skip(p string) { c.st.Skipped = append(c.st.Skipped, p) }

// dir makes the folder p in the destination. It reports false when p is
// a symbolic link (or a device) there, which stays as it is.
func (c *copier) dir(p string, d fs.DirEntry) (bool, error) {
	info, err := d.Info()
	if err != nil {
		return false, err
	}
	fi, err := c.dst.Lstat(p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if err := c.dst.Mkdir(p, 0o700); err != nil {
			return false, err
		}
	case err != nil:
		return false, err
	case fi.IsDir():
	case fi.Mode().IsRegular():
		if err := c.dst.Remove(p); err != nil {
			return false, err
		}
		c.st.Removed++
		if err := c.dst.Mkdir(p, 0o700); err != nil {
			return false, err
		}
	default:
		c.skip(p)
		return false, nil
	}
	if err := c.dst.Chmod(p, info.Mode().Perm()); err != nil {
		return false, err
	}
	if c.o.UID < 0 {
		return true, nil
	}
	return true, c.dst.Lchown(p, c.o.UID, c.o.GID)
}

// prune removes from the destination folder p what the source folder does
// not have.
func (c *copier) prune(p string) error {
	f, err := c.dst.Open(p)
	if err != nil {
		return err
	}
	entries, err := f.ReadDir(-1)
	f.Close()
	if err != nil {
		return err
	}
	for _, e := range entries {
		q := path.Join(p, e.Name())
		if slices.Contains(c.o.Skip, q) || e.Type()&fs.ModeSymlink != 0 {
			continue
		}
		if _, err := c.src.Lstat(q); err == nil || !errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err := c.dst.RemoveAll(q); err != nil {
			return err
		}
		c.st.Removed++
	}
	return nil
}

func (c *copier) file(p string) error {
	in, err := c.src.OpenFile(p, os.O_RDONLY|openNonBlock, 0)
	if err != nil {
		return err
	}
	defer in.Close()
	fi, err := in.Stat()
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		c.skip(p)
		return nil
	}
	if !c.readable(fi) {
		c.skip(p)
		return nil
	}
	transform, transformed := c.o.Transform[p]

	old, err := c.dst.Lstat(p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return err
	case old.IsDir():
		if err := c.dst.RemoveAll(p); err != nil {
			return err
		}
		c.st.Removed++
	case !old.Mode().IsRegular():
		c.skip(p) // a symbolic link stays
		return nil
	case c.o.Mirror && !transformed && old.Size() == fi.Size() && old.ModTime().Equal(fi.ModTime()):
		c.st.Unchanged++
		c.progress()
		if err := c.dst.Chmod(p, fi.Mode().Perm()); err != nil {
			return err
		}
		if c.o.UID < 0 {
			return nil
		}
		return c.dst.Lchown(p, c.o.UID, c.o.GID)
	}

	var r io.Reader = in
	if transformed {
		b, err := io.ReadAll(io.LimitReader(in, 1<<20+1))
		if err != nil {
			return err
		}
		if len(b) > 1<<20 {
			return fmt.Errorf("%s is larger than expected", p)
		}
		if b, err = transform(b); err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	tmp, out, err := c.temp(p)
	if err != nil {
		return err
	}
	done := false
	defer func() {
		if !done {
			out.Close()
			c.dst.Remove(tmp)
		}
	}()
	n, err := io.Copy(out, r)
	if err != nil {
		return err
	}
	// Through the open file: the folder may belong to a user who could put
	// a link in place of the name.
	if err := out.Chmod(fi.Mode().Perm()); err != nil {
		return err
	}
	if c.o.UID >= 0 {
		if err := out.Chown(c.o.UID, c.o.GID); err != nil {
			return err
		}
	}
	if err := out.Close(); err != nil {
		return err
	}
	done = true
	if !transformed {
		if err := c.dst.Chtimes(tmp, time.Time{}, fi.ModTime()); err != nil {
			c.dst.Remove(tmp)
			return err
		}
	}
	if err := c.dst.Rename(tmp, p); err != nil {
		c.dst.Remove(tmp)
		return err
	}
	c.st.Files++
	c.st.Bytes += n
	c.progress()
	return nil
}

// link makes the symbolic link p in the destination, with the target
// Links gives, owned like the copied files.
func (c *copier) link(p string) error {
	target, err := c.src.Readlink(p)
	if err != nil {
		return err
	}
	if target = c.o.Links(target); target == "" {
		c.skip(p)
		return nil
	}
	if _, err := c.dst.Lstat(p); err == nil {
		c.skip(p)
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := c.dst.Symlink(target, p); err != nil {
		return err
	}
	if c.o.UID >= 0 {
		return c.dst.Lchown(p, c.o.UID, c.o.GID)
	}
	return nil
}

// temp creates a new file next to p.
func (c *copier) temp(p string) (string, *os.File, error) {
	for range 100 {
		c.tmpN++
		name := path.Join(path.Dir(p), ".nefo-copy-"+strconv.Itoa(os.Getpid())+"-"+strconv.Itoa(c.tmpN))
		f, err := c.dst.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		return name, f, err
	}
	return "", nil, fmt.Errorf("no free temporary name next to %s", p)
}

func (c *copier) progress() {
	if n := c.st.Files + c.st.Unchanged; c.o.Log != nil && n%5000 == 0 {
		c.o.Log.Logf("Copied %d files", n)
	}
}
