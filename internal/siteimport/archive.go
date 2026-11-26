package siteimport

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/usererr"
)

// Limits bound what one import may unpack.
type Limits struct {
	MaxFile    int64 // one file
	MaxTotal   int64 // everything an archive or dump unpacks to
	MaxEntries int   // entries in an archive
	// MaxRatio is how much larger than its compressed size a gzip stream
	// may grow (but at least to MinInflate): websites and dumps compress
	// far less.
	MaxRatio   int64
	MinInflate int64
}

// DefaultLimits are the limits of an import.
var DefaultLimits = Limits{MaxFile: 8 << 30, MaxTotal: 200 << 30, MaxEntries: 2_000_000, MaxRatio: 200, MinInflate: 1 << 30}

// Formats of archives and dumps.
const (
	FormatTarGz = "tar.gz"
	FormatTar   = "tar"
	FormatZip   = "zip"
	FormatSQL   = "sql"
	FormatSQLGz = "sql.gz"
)

// Archive is an archive or SQL dump opened for an import.
type Archive struct {
	Path   string    `json:"path"`
	Name   string    `json:"name"`
	Size   int64     `json:"size"`
	Format string    `json:"format"`
	MTime  time.Time `json:"-"`
	id     fileID
}

// Same tells whether f is still the file that was analysed.
func (a *Archive) Same(b *Archive) bool {
	return a != nil && b != nil && a.id == b.id && a.Size == b.Size && a.MTime.Equal(b.MTime)
}

// DeniedPaths are never read as archives: kernel and device files, the
// system's configuration and runtime folders.
var DeniedPaths = []string{"/proc", "/sys", "/dev", "/run", "/var/run", "/etc", "/boot"}

func under(p, dir string) bool {
	p, dir = filepath.ToSlash(p), filepath.ToSlash(dir)
	return p == dir || strings.HasPrefix(p, strings.TrimSuffix(dir, "/")+"/")
}

// Open opens an archive or SQL dump on this server for reading. The path
// must be absolute and lead (also through links) to a regular file outside
// DeniedPaths and the extra folders deny, such as the panel's own data.
// sql tells whether an SQL dump is expected.
func Open(p string, sql bool, deny ...string) (*os.File, *Archive, error) {
	if p == "" || !filepath.IsAbs(p) || strings.ContainsRune(p, 0) || len(p) > 4096 {
		return nil, nil, usererr.New("import_path_invalid", p)
	}
	p = filepath.Clean(p)
	real, err := filepath.EvalSymlinks(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, usererr.New("path_not_found", p)
	}
	if err != nil {
		return nil, nil, usererr.New("import_path_invalid", p)
	}
	denied := append(append([]string{}, DeniedPaths...), deny...)
	isDenied := func(q string) bool {
		for _, d := range denied {
			if d != "" && under(q, d) {
				return true
			}
		}
		return false
	}
	if isDenied(p) || isDenied(real) {
		return nil, nil, usererr.New("import_path_denied", p)
	}
	// No following a link swapped in meanwhile, no waiting on a FIFO.
	f, err := os.OpenFile(real, os.O_RDONLY|openFlags, 0)
	if err != nil {
		return nil, nil, usererr.New("import_not_file", p)
	}
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		f.Close()
		return nil, nil, usererr.New("import_not_file", p)
	}
	// Where the open file really is, in case a folder on the way changed.
	if now, ok := openedPath(f); ok && isDenied(now) {
		f.Close()
		return nil, nil, usererr.New("import_path_denied", p)
	}
	if fi.Size() == 0 {
		f.Close()
		return nil, nil, usererr.New("import_empty", p)
	}
	a := &Archive{Path: p, Name: filepath.Base(p), Size: fi.Size(), MTime: fi.ModTime(), id: idOf(fi)}
	a.Format = sniff(f, sql)
	if a.Format == "" {
		f.Close()
		return nil, nil, usererr.New("import_unknown_format", a.Name)
	}
	return f, a, nil
}

// sniff tells the format from the first bytes, not the name.
func sniff(f *os.File, sql bool) string {
	head := make([]byte, 512)
	n, _ := f.ReadAt(head, 0)
	head = head[:n]
	gz := len(head) >= 2 && head[0] == 0x1f && head[1] == 0x8b
	switch {
	case sql && gz:
		return FormatSQLGz
	case sql && !bytes.HasPrefix(head, []byte("PK\x03\x04")) && !gz && !(len(head) > 262 && string(head[257:262]) == "ustar"):
		return FormatSQL
	case sql:
		return ""
	case gz:
		return FormatTarGz
	case bytes.HasPrefix(head, []byte("PK\x03\x04")) || bytes.HasPrefix(head, []byte("PK\x05\x06")):
		return FormatZip
	case len(head) > 262 && string(head[257:262]) == "ustar":
		return FormatTar
	}
	return ""
}

// ---- reading archives ----

type entryKind int

const (
	kindDir entryKind = iota
	kindFile
	kindSymlink
	kindHardlink
	kindOther // devices, FIFOs, sparse files and anything else
)

// entry is one item of an archive. R, for a regular file, is valid during
// the callback only.
type entry struct {
	Name    string
	Kind    entryKind
	Mode    fs.FileMode
	Size    int64
	ModTime time.Time
	R       io.Reader
}

// ErrTooLarge stops an archive that unpacks to more than the limits.
var ErrTooLarge = errors.New("too large")

// walker reads archives within limits.
type walker struct {
	lim     Limits
	entries int
}

func (w *walker) count() error {
	if w.entries++; w.entries > w.lim.MaxEntries {
		return fmt.Errorf("the archive has more than %d entries: %w", w.lim.MaxEntries, ErrTooLarge)
	}
	return nil
}

// inflateLimit is how many bytes a gzip stream of size compressed bytes
// may grow to.
func (l Limits) inflateLimit(compressed int64) int64 {
	n := max(l.MinInflate, compressed*l.MaxRatio)
	if compressed > 0 && n/compressed < l.MaxRatio { // overflow
		n = l.MaxTotal
	}
	return min(n, l.MaxTotal)
}

// capReader fails once more than max bytes came through.
type capReader struct {
	r   io.Reader
	n   int64
	max int64
}

func (c *capReader) Read(p []byte) (int, error) {
	if c.n > c.max {
		return 0, c.err()
	}
	n, err := c.r.Read(p)
	c.n += int64(n)
	if c.n > c.max {
		return n, c.err()
	}
	return n, err
}

func (c *capReader) err() error {
	return fmt.Errorf("it unpacks to more than %s, far more than its own size: refused as a possible archive bomb (%w)", humanBytes(c.max), ErrTooLarge)
}

// walk calls fn for each entry of an archive.
func (w *walker) walk(ctx context.Context, f *os.File, a *Archive, fn func(*entry) error) error {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	switch a.Format {
	case FormatTarGz:
		gz, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("read %s: %w", a.Name, err)
		}
		defer gz.Close()
		return w.walkTar(ctx, &capReader{r: gz, max: w.lim.inflateLimit(a.Size)}, fn)
	case FormatTar:
		return w.walkTar(ctx, f, fn)
	case FormatZip:
		return w.walkZip(ctx, f, a.Size, fn)
	}
	return usererr.New("import_unknown_format", a.Name)
}

func (w *walker) walkTar(ctx context.Context, r io.Reader, fn func(*entry) error) error {
	tr := tar.NewReader(r)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		// With GODEBUG=tarinsecurepath=0, a name with ".." or an absolute
		// one comes with this error: such names are left out below.
		if errors.Is(err, tar.ErrInsecurePath) && h != nil {
			err = nil
		}
		if err != nil {
			if errors.Is(err, ErrTooLarge) {
				return err
			}
			return fmt.Errorf("the archive is damaged or incomplete: %w", err)
		}
		if err := w.count(); err != nil {
			return err
		}
		e := &entry{Name: h.Name, Mode: fs.FileMode(h.Mode).Perm(), Size: h.Size, ModTime: h.ModTime}
		switch h.Typeflag {
		case tar.TypeDir:
			e.Kind = kindDir
		case tar.TypeReg, '\x00':
			if strings.HasSuffix(h.Name, "/") {
				e.Kind = kindDir
			} else {
				e.Kind, e.R = kindFile, tr
			}
		case tar.TypeSymlink:
			e.Kind = kindSymlink
		case tar.TypeLink:
			e.Kind = kindHardlink
		default:
			e.Kind = kindOther
		}
		if err := fn(e); err != nil {
			return err
		}
	}
}

func (w *walker) walkZip(ctx context.Context, f *os.File, size int64, fn func(*entry) error) error {
	zr, err := zip.NewReader(f, size)
	if errors.Is(err, zip.ErrInsecurePath) && zr != nil {
		err = nil // such names are left out
	}
	if err != nil {
		return fmt.Errorf("the zip archive is damaged: %w", err)
	}
	for _, zf := range zr.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := w.count(); err != nil {
			return err
		}
		mode := zf.Mode()
		e := &entry{Name: zf.Name, Mode: mode.Perm(), Size: int64(min(zf.UncompressedSize64, 1<<62)), ModTime: zf.Modified}
		switch {
		case mode.IsDir() || strings.HasSuffix(zf.Name, "/"):
			e.Kind = kindDir
		case mode&fs.ModeSymlink != 0:
			e.Kind = kindSymlink
		case mode.IsRegular():
			e.Kind = kindFile
		default:
			e.Kind = kindOther
		}
		if e.Kind != kindFile {
			if err := fn(e); err != nil {
				return err
			}
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return fmt.Errorf("the zip archive is damaged (%s): %w", zf.Name, err)
		}
		e.R = rc
		err = fn(e)
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// cleanName turns an entry name into a path relative to the archive's
// root, or reports it unsafe: absolute names and names with ".." are
// never followed. "" with ok is the root itself.
func cleanName(n string) (string, bool) {
	n = strings.ReplaceAll(n, "\\", "/")
	if strings.HasPrefix(n, "/") || strings.ContainsRune(n, 0) {
		return "", false
	}
	var parts []string
	for _, p := range strings.Split(n, "/") {
		switch p {
		case "", ".":
		case "..":
			return "", false
		default:
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return "", true
	}
	clean := strings.Join(parts, "/")
	if !fs.ValidPath(clean) || len(clean) > 4000 {
		return "", false
	}
	return clean, true
}

// readSmall reads a small file of an archive, at most limit bytes; a
// larger one gives nil.
func readSmall(r io.Reader, limit int64) []byte {
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil || int64(len(b)) > limit {
		return nil
	}
	return b
}

// depth is the number of parts of a relative path.
func depth(p string) int {
	if p == "" {
		return 0
	}
	return strings.Count(p, "/") + 1
}

// ancestors returns "a", "a/b", "a/b/c" for "a/b/c/file", at most max of them.
func ancestors(p string, max int) []string {
	dir := path.Dir(p)
	if dir == "." {
		return nil
	}
	parts := strings.Split(dir, "/")
	var out []string
	for i := 1; i <= len(parts) && i <= max; i++ {
		out = append(out, strings.Join(parts[:i], "/"))
	}
	return out
}

// humanBytes formats a size like the UI does: 1.5 MB.
func humanBytes(n int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	v := float64(n)
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if i == 0 || v >= 100 {
		return fmt.Sprintf("%.0f %s", v, units[i])
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}

func filepathToSlash(p string) string { return filepath.ToSlash(p) }
