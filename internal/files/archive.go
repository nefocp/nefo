package files

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"

	"github.com/nefocp/nefo/internal/usererr"
)

// Logger receives progress messages for a job log.
type Logger interface {
	Logf(format string, args ...any)
}

const (
	maxExtractBytes   = 20 << 30 // refuse archive bombs
	maxExtractEntries = 500_000
)

// ArchiveFormat returns "zip", "tar.gz" or "tar" from a file name, or "".
func ArchiveFormat(name string) string {
	n := strings.ToLower(name)
	switch {
	case strings.HasSuffix(n, ".zip"):
		return "zip"
	case strings.HasSuffix(n, ".tar.gz"), strings.HasSuffix(n, ".tgz"):
		return "tar.gz"
	case strings.HasSuffix(n, ".tar"):
		return "tar"
	}
	return ""
}

// Compress packs files and folders into a new .zip or .tar.gz archive. All
// paths must be in the same scope. Links and special files are skipped.
func (m *Manager) Compress(ctx context.Context, paths []string, dest string, lg Logger) error {
	if len(paths) == 0 {
		return usererr.New("nothing_selected")
	}
	format := ArchiveFormat(dest)
	if format != "zip" && format != "tar.gz" {
		return usererr.New("invalid_archive_name", dest)
	}
	out, err := m.open(ctx, dest)
	if err != nil {
		return err
	}
	defer out.Close()
	var rels []string
	for _, p := range paths {
		t, err := m.open(ctx, p)
		if err != nil {
			return err
		}
		t.Close()
		if t.scope.Root != out.scope.Root {
			return usererr.New("cross_scope")
		}
		rels = append(rels, t.rel)
	}
	f, err := out.root.OpenFile(out.rel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return mapErr(err, out.abs)
	}
	err = WriteArchive(ctx, out.root, rels, f, format, func(rel string) bool { return rel == out.rel }, lg)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		out.root.Remove(out.rel)
		return err
	}
	return out.own(out.rel, 0o644)
}

// WriteArchive writes the items rels of root, recursively, to w as a zip or
// tar.gz archive. Each item is stored under its base name. skip leaves out
// paths (relative to root). It is also used by backups.
func WriteArchive(ctx context.Context, root *os.Root, rels []string, w io.Writer, format string, skip func(string) bool, lg Logger) error {
	var zw *zip.Writer
	var tw *tar.Writer
	var gz *gzip.Writer
	if format == "zip" {
		zw = zip.NewWriter(w)
	} else {
		gz = gzip.NewWriter(w)
		tw = tar.NewWriter(gz)
	}
	fsys := root.FS()
	count := 0
	for _, rel := range rels {
		base := path.Dir(rel)
		err := fs.WalkDir(fsys, rel, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if skip != nil && skip(p) {
				if d.IsDir() {
					return fs.SkipDir
				}
				return nil
			}
			name := p
			if base != "." {
				name = strings.TrimPrefix(p, base+"/")
			}
			if name == "." || name == "" {
				return nil // the folder itself when rel is "."
			}
			fi, err := d.Info()
			if err != nil {
				return err
			}
			switch {
			case fi.IsDir():
				if zw != nil {
					_, err = zw.CreateHeader(&zip.FileHeader{Name: name + "/", Modified: fi.ModTime()})
					return err
				}
				return tw.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: name + "/", Mode: int64(fi.Mode().Perm()), ModTime: fi.ModTime()})
			case fi.Mode().IsRegular():
				count++
				if lg != nil && count%1000 == 0 {
					lg.Logf("%d files packed", count)
				}
				return addFile(fsys, p, name, fi, zw, tw)
			}
			return nil // links, sockets, devices
		})
		if err != nil {
			return mapErr(err, rel)
		}
	}
	if zw != nil {
		if err := zw.Close(); err != nil {
			return err
		}
	} else {
		if err := tw.Close(); err != nil {
			return err
		}
		if err := gz.Close(); err != nil {
			return err
		}
	}
	if lg != nil {
		lg.Logf("%d files packed", count)
	}
	return nil
}

func addFile(fsys fs.FS, p, name string, fi fs.FileInfo, zw *zip.Writer, tw *tar.Writer) error {
	src, err := fsys.Open(p)
	if err != nil {
		return err
	}
	defer src.Close()
	var dst io.Writer
	if zw != nil {
		h, err := zip.FileInfoHeader(fi)
		if err != nil {
			return err
		}
		h.Name, h.Method = name, zip.Deflate
		if dst, err = zw.CreateHeader(h); err != nil {
			return err
		}
	} else {
		if err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: name, Size: fi.Size(),
			Mode: int64(fi.Mode().Perm()), ModTime: fi.ModTime()}); err != nil {
			return err
		}
		dst = tw
	}
	// A file that grows while it is read must not overflow the tar header.
	_, err = io.Copy(dst, io.LimitReader(src, fi.Size()))
	return err
}

// Extract unpacks a .zip, .tar.gz or .tar archive into folder dest, in the
// same scope. Existing files are overwritten. Links and special files in
// the archive are skipped, and files get normal permissions (0644, or 0755
// when executable) and the scope's owner.
func (m *Manager) Extract(ctx context.Context, archive, dest string, lg Logger) error {
	format := ArchiveFormat(archive)
	if format == "" {
		return usererr.New("not_an_archive", archive)
	}
	src, err := m.open(ctx, archive)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := m.open(ctx, dest)
	if err != nil {
		return err
	}
	defer dst.Close()
	if src.scope.Root != dst.scope.Root {
		return usererr.New("cross_scope")
	}
	f, err := src.root.Open(src.rel)
	if err != nil {
		return mapErr(err, src.abs)
	}
	defer f.Close()
	n, err := ExtractTo(ctx, f, format, dst.root, dst.rel, dst.scope.UID, dst.scope.GID, lg)
	if lg != nil {
		lg.Logf("%d files extracted", n)
	}
	return err
}

// ExtractTo unpacks an archive read from f into dir (relative to root). It
// is also used to restore backups.
func ExtractTo(ctx context.Context, f *os.File, format string, root *os.Root, dir string, uid, gid int, lg Logger) (int, error) {
	x, err := NewExtractor(ctx, root, dir, uid, gid, false, lg)
	if err != nil {
		return 0, err
	}
	switch format {
	case "zip":
		fi, err := f.Stat()
		if err != nil {
			return 0, err
		}
		zr, err := zip.NewReader(f, fi.Size())
		if err != nil {
			return 0, fmt.Errorf("read zip: %w", err)
		}
		for _, zf := range zr.File {
			mode := zf.Mode()
			switch {
			case mode.IsDir():
				err = x.Folder(zf.Name)
			case mode.IsRegular():
				var rc io.ReadCloser
				if rc, err = zf.Open(); err == nil {
					err = x.File(zf.Name, rc, mode)
					rc.Close()
				}
			}
			if err != nil {
				return x.count, err
			}
		}
	case "tar", "tar.gz":
		var r io.Reader = f
		if format == "tar.gz" {
			gz, err := gzip.NewReader(f)
			if err != nil {
				return 0, fmt.Errorf("read gzip: %w", err)
			}
			defer gz.Close()
			r = gz
		}
		tr := tar.NewReader(r)
		for {
			h, err := tr.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return x.count, fmt.Errorf("read tar: %w", err)
			}
			switch h.Typeflag {
			case tar.TypeDir:
				err = x.Folder(h.Name)
			case tar.TypeReg:
				err = x.File(h.Name, tr, fs.FileMode(h.Mode))
			}
			if err != nil {
				return x.count, err
			}
		}
	default:
		return 0, usererr.New("not_an_archive")
	}
	return x.count, nil
}

// NewExtractor prepares to write archive entries into folder dir of root,
// owned by uid:gid. With keepModes, files keep their permission bits (never
// setuid or setgid); otherwise they get 0644, or 0755 when executable.
func NewExtractor(ctx context.Context, root *os.Root, dir string, uid, gid int, keepModes bool, lg Logger) (*Extractor, error) {
	x := &Extractor{ctx: ctx, root: root, dir: dir, uid: uid, gid: gid, keepModes: keepModes, lg: lg}
	if err := x.mkdirAll(dir); err != nil {
		return nil, err
	}
	return x, nil
}

// Extractor writes archive entries safely inside an os.Root.
type Extractor struct {
	keepModes bool
	ctx       context.Context
	root      *os.Root
	dir       string
	uid, gid  int
	lg        Logger
	count     int
	bytes     int64
}

// Count returns the number of files written so far.
func (x *Extractor) Count() int { return x.count }

// name turns an archive entry name into a path under x.dir, or "" to skip it.
func (x *Extractor) name(n string) string {
	n = strings.TrimPrefix(path.Clean("/"+strings.ReplaceAll(n, "\\", "/")), "/")
	if n == "" || n == "." || !fs.ValidPath(n) {
		return ""
	}
	return path.Join(x.dir, n)
}

func (x *Extractor) Folder(n string) error {
	p := x.name(n)
	if p == "" {
		return nil
	}
	return x.mkdirAll(p)
}

// mkdirAll creates p and missing parents one by one, each with mode 0755 and
// the scope's owner. (MkdirAll alone would leave parents with the panel's
// umask 077 and owned by root.)
func (x *Extractor) mkdirAll(p string) error {
	if p == "." || p == "" {
		return nil
	}
	if fi, err := x.root.Stat(p); err == nil {
		if !fi.IsDir() {
			return usererr.New("path_exists", p)
		}
		return nil
	}
	if err := x.mkdirAll(path.Dir(p)); err != nil {
		return err
	}
	if err := x.root.Mkdir(p, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
		return mapErr(err, p)
	}
	if err := x.root.Chmod(p, 0o755); err != nil {
		return err
	}
	return x.root.Lchown(p, x.uid, x.gid)
}

func (x *Extractor) File(n string, r io.Reader, mode fs.FileMode) error {
	if err := x.ctx.Err(); err != nil {
		return err
	}
	p := x.name(n)
	if p == "" {
		return nil
	}
	x.count++
	if x.count > maxExtractEntries {
		return usererr.New("archive_too_large")
	}
	if err := x.mkdirAll(path.Dir(p)); err != nil {
		return err
	}
	perm := fs.FileMode(0o644)
	if mode&0o111 != 0 {
		perm = 0o755
	}
	if x.keepModes {
		perm = mode.Perm() | 0o600 // the owner must keep access
	}
	f, err := x.root.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return mapErr(err, p)
	}
	n64, err := io.Copy(f, io.LimitReader(r, maxExtractBytes-x.bytes+1))
	x.bytes += n64
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if x.bytes > maxExtractBytes {
		return usererr.New("archive_too_large")
	}
	if x.lg != nil && x.count%1000 == 0 {
		x.lg.Logf("%d files extracted", x.count)
	}
	if err := x.root.Chmod(p, perm); err != nil {
		return err
	}
	return x.root.Lchown(p, x.uid, x.gid)
}
