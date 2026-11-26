package backup

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// Folder backups keep any folder of the server, such as /etc/nginx,
// /etc/apache2, /usr/local/lsws or /opt/app, in the same archive format as
// websites: the folder's content is under files/, with owners, modes and
// symbolic links. The folder itself is the entry files/.

// systemFolders are never backed up or restored, nor any folder inside or
// containing them.
var systemFolders = []string{"/proc", "/sys", "/dev", "/run"}

// defaultDataDir is the panel's data folder unless configured otherwise
// (see ProtectFolders).
const defaultDataDir = "/var/lib/nefo"

// panelFolders are the panel's other folders. /var/lib/panel-sftp holds bind
// mounts of website folders, which a restore must never delete through.
var panelFolders = []string{"/var/lib/panel-sftp", "/var/lib/panel-acme", "/var/lib/panel-quarantine"}

// ProtectFolders adds folders, such as the panel's data folder, that folder
// backups must not include or restore over.
func (m *Manager) ProtectFolders(dirs ...string) {
	for _, d := range dirs {
		if d == "" {
			continue
		}
		if abs, err := filepath.Abs(d); err == nil {
			m.protected = append(m.protected, abs)
		}
	}
}

// BackupFolder backs up a folder of the server.
func (m *Manager) BackupFolder(ctx context.Context, dir string, scheduleID int64, remote bool, lg Logger) (*store.Backup, error) {
	if err := m.CheckTarget(ctx, KindFolder, dir); err != nil {
		return nil, err
	}
	man := manifest{Format: 1, Kind: KindFolder, Path: dir}
	return m.make(ctx, KindFolder, dir, man, scheduleID, remote, lg, func(tw *tar.Writer) error {
		root, err := os.OpenRoot(dir)
		if err != nil {
			return err
		}
		defer root.Close()
		lg.Logf("Packing %s", dir)
		n, err := addFolderTree(ctx, tw, root, "files")
		if err != nil {
			return err
		}
		lg.Logf("%d files packed", n)
		return nil
	})
}

// validFolderPath checks the form of a folder path: absolute and clean.
func validFolderPath(p string) error {
	if p == "" || len(p) > 1024 || !filepath.IsAbs(p) || filepath.Clean(p) != p ||
		strings.ContainsFunc(p, func(r rune) bool { return r < ' ' || r == 0x7f }) {
		return usererr.New("invalid_folder", p)
	}
	return nil
}

// checkFolder refuses the root of the file system, the system's virtual
// folders and the panel's own folders, and any folder inside or containing
// one of them, also when reached through a symbolic link.
func (m *Manager) checkFolder(p string) error {
	if err := validFolderPath(p); err != nil {
		return err
	}
	paths := []string{p}
	if real, err := filepath.EvalSymlinks(p); err == nil && real != p {
		paths = append(paths, real)
	}
	for _, q := range paths {
		if filepath.Dir(q) == q {
			return usererr.New("folder_protected", p)
		}
		for _, d := range m.protected {
			if within(q, d) || within(d, q) {
				return usererr.New("folder_protected", p)
			}
		}
	}
	return nil
}

// within reports whether p is dir or inside it.
func within(p, dir string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && !filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// storageDir names the folder that holds a target's backups, on this
// server and in remote storage. A folder path becomes a readable slug plus
// a short hash, so that /a/b_c and /a_b/c do not share a folder.
func storageDir(kind, target string) string {
	if kind != KindFolder {
		return target
	}
	sum := sha256.Sum256([]byte(target))
	return folderSlug(target) + "-" + hex.EncodeToString(sum[:4])
}

// fileBase starts the name of a backup file.
func fileBase(kind, target string) string {
	if kind == KindFolder {
		return folderSlug(target)
	}
	return target
}

// folderSlug turns a path into a file name: its last 60 characters, with
// anything but letters, digits, dots and dashes replaced by underscores.
func folderSlug(p string) string {
	var b strings.Builder
	for _, r := range strings.Trim(filepath.ToSlash(p), "/") {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	s := b.String()
	if len(s) > 60 {
		s = s[len(s)-60:]
	}
	if s = strings.Trim(s, "._"); s != "" {
		return s
	}
	return "folder"
}

// Names of the panel's temporary folders while it restores a folder. Left
// over after a crash, they are not backed up.
const (
	restoreStagePrefix = ".panel-restore-"
	restoreOldPrefix   = ".panel-old-"
)

func isRestoreLeftover(name string) bool {
	return strings.HasPrefix(name, restoreStagePrefix) || strings.HasPrefix(name, restoreOldPrefix)
}

// addFolderTree adds root and its content under prefix/, with owners,
// modes and symbolic links. Sockets, pipes and devices are left out.
func addFolderTree(ctx context.Context, tw *tar.Writer, root *os.Root, prefix string) (int, error) {
	return addFolderTreeFiltered(ctx, tw, root, prefix, nil)
}

// addFolderTreeFiltered is addFolderTree leaving out the paths (relative,
// with slashes) for which skip returns true.
func addFolderTreeFiltered(ctx context.Context, tw *tar.Writer, root *os.Root, prefix string, skip func(string) bool) (int, error) {
	count := 0
	err := fs.WalkDir(root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if p != "." && ((!strings.Contains(p, "/") && isRestoreLeftover(p)) || (skip != nil && skip(p))) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		link := ""
		switch {
		case fi.Mode()&fs.ModeSymlink != 0:
			if link, err = root.Readlink(p); err != nil {
				return err
			}
		case !fi.IsDir() && !fi.Mode().IsRegular():
			return nil
		}
		hdr, err := tar.FileInfoHeader(fi, link)
		if err != nil {
			return err
		}
		hdr.Name = prefix + "/"
		if p != "." {
			hdr.Name += p
			if fi.IsDir() {
				hdr.Name += "/"
			}
		}
		hdr.AccessTime, hdr.ChangeTime = time.Time{}, time.Time{}
		if !fi.Mode().IsRegular() {
			return tw.WriteHeader(hdr)
		}
		src, err := root.Open(p)
		if err != nil {
			return err
		}
		defer src.Close()
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		// A file that grows while it is read must not overflow its header.
		n, err := io.Copy(tw, io.LimitReader(src, fi.Size()))
		if err == nil && n < fi.Size() {
			_, err = io.CopyN(tw, zeros{}, fi.Size()-n) // it shrank: pad
		}
		count++
		return err
	})
	return count, err
}

// restoreFolder puts a folder back where it was. The backup is first
// written to a temporary folder inside it; only when that worked is the
// current content replaced, so a broken backup leaves the folder as it was.
func (m *Manager) restoreFolder(ctx context.Context, b *store.Backup, man manifest, tr *tar.Reader, lg Logger) error {
	dir := b.Target
	if man.Kind != KindFolder || man.Path != dir {
		return fmt.Errorf("the archive is not a backup of %s", dir)
	}
	if err := m.checkFolder(dir); err != nil {
		return err
	}
	return restoreTree(ctx, dir, tr, lg, nil)
}

// restoreTree puts the files/ entries of an archive into dir (created if
// missing), replacing its content only once everything was unpacked. Other
// entries go to other (nil: ignored).
func restoreTree(ctx context.Context, dir string, tr *tar.Reader, lg Logger, other func(name string, r io.Reader) error) error {
	fi, err := os.Stat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		lg.Logf("Creating %s", dir)
		if err := mkdirParents(filepath.Dir(dir)); err != nil {
			return err
		}
		if err := os.Mkdir(dir, 0o700); err != nil {
			return err
		}
	case err != nil:
		return err
	case !fi.IsDir():
		return fmt.Errorf("%s is not a folder", dir)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()

	stage := restoreStagePrefix + randomHex(4)
	if err := root.Mkdir(stage, 0o700); err != nil {
		return err
	}
	keepStage := false
	defer func() {
		if !keepStage {
			root.RemoveAll(stage)
		}
	}()

	lg.Logf("Unpacking the backup next to the current files")
	var top *tar.Header
	var dirs []*tar.Header
	count := 0
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read backup: %w", err)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		name := path.Clean(hdr.Name)
		if name == "files" {
			if hdr.Typeflag == tar.TypeDir {
				top = hdr
			}
			continue
		}
		rel, ok := strings.CutPrefix(name, "files/")
		if !ok {
			if other != nil && hdr.Typeflag == tar.TypeReg {
				if err := other(name, tr); err != nil {
					return err
				}
			}
			continue
		}
		if !fs.ValidPath(rel) {
			return fmt.Errorf("the backup contains an invalid name %q", hdr.Name)
		}
		p := stage + "/" + rel
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(p, 0o700); err != nil {
				return err
			}
			h := *hdr
			h.Name = p
			dirs = append(dirs, &h)
			continue // modes and times once the folder is filled
		case tar.TypeReg:
			if err := root.MkdirAll(path.Dir(p), 0o700); err != nil {
				return err
			}
			f, err := root.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
			if err != nil {
				return err
			}
			_, err = io.Copy(f, tr)
			if cerr := f.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				return err
			}
			if count++; count%1000 == 0 {
				lg.Logf("%d files restored", count)
			}
		case tar.TypeSymlink:
			if err := root.MkdirAll(path.Dir(p), 0o700); err != nil {
				return err
			}
			if err := root.Symlink(hdr.Linkname, p); err != nil {
				return err
			}
		default:
			continue
		}
		if err := setAttrs(root, p, hdr); err != nil {
			return err
		}
	}
	for i := len(dirs) - 1; i >= 0; i-- { // deepest first, so times stick
		if err := setAttrs(root, dirs[i].Name, dirs[i]); err != nil {
			return err
		}
	}

	lg.Logf("Replacing the content of %s", dir)
	old := restoreOldPrefix + randomHex(4)
	if err := root.Mkdir(old, 0o700); err != nil {
		return err
	}
	current, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return err
	}
	var moved []string
	for _, e := range current {
		if e.Name() == stage || e.Name() == old {
			continue
		}
		if err := root.Rename(e.Name(), old+"/"+e.Name()); err != nil {
			for _, n := range moved { // put back what was moved
				root.Rename(old+"/"+n, n)
			}
			root.Remove(old)
			return fmt.Errorf("move the current files aside: %w", err)
		}
		moved = append(moved, e.Name())
	}
	restored, err := fs.ReadDir(root.FS(), stage)
	if err != nil {
		return err
	}
	keepStage = true
	for _, e := range restored {
		if err := root.Rename(stage+"/"+e.Name(), e.Name()); err != nil {
			return fmt.Errorf("move the restored files in place: %w (the previous files are in %s, the rest of the backup in %s)",
				err, filepath.Join(dir, old), filepath.Join(dir, stage))
		}
	}
	root.Remove(stage)
	if err := root.RemoveAll(old); err != nil {
		lg.Logf("Warning: could not remove the previous files in %s: %v", filepath.Join(dir, old), err)
	}
	if top != nil {
		if err := setAttrs(root, ".", top); err != nil {
			return err
		}
	}
	lg.Logf("%d files restored", count)
	return nil
}

// setAttrs gives a restored entry its owner, mode and modification time.
func setAttrs(root *os.Root, p string, hdr *tar.Header) error {
	if runtime.GOOS != "windows" {
		if err := root.Lchown(p, hdr.Uid, hdr.Gid); err != nil {
			return err
		}
	}
	if hdr.Typeflag == tar.TypeSymlink {
		return nil
	}
	// After chown, which clears setuid and setgid.
	mode := hdr.FileInfo().Mode() & (fs.ModePerm | fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky)
	if err := root.Chmod(p, mode); err != nil {
		return err
	}
	return root.Chtimes(p, hdr.ModTime, hdr.ModTime)
}

// mkdirParents creates dir and its missing parents with mode 0755 (the
// panel's umask would make them private).
func mkdirParents(dir string) error {
	if fi, err := os.Stat(dir); err == nil {
		if !fi.IsDir() {
			return fmt.Errorf("%s is not a folder", dir)
		}
		return nil
	}
	if parent := filepath.Dir(dir); parent != dir {
		if err := mkdirParents(parent); err != nil {
			return err
		}
	}
	if err := os.Mkdir(dir, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	return os.Chmod(dir, 0o755)
}
