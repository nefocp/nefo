package files

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nefocp/nefo/internal/usererr"
)

// Search limits keep one search from tying up the server.
const (
	maxSearchWalk  = 20_000  // entries looked at
	maxSearchHits  = 200     // results returned
	maxSearchFile  = 2 << 20 // larger files are not searched for content
	maxSearchBytes = 256 << 20
	searchBudget   = 20 * time.Second
	maxQueryLen    = 200
	excerptWidth   = 120
)

type searchLimits struct {
	walk, hits int
	fileSize   int64
	bytes      int64
	budget     time.Duration
}

var defaultSearchLimits = searchLimits{walk: maxSearchWalk, hits: maxSearchHits, fileSize: maxSearchFile,
	bytes: maxSearchBytes, budget: searchBudget}

// SearchHit is one file or folder found.
type SearchHit struct {
	Path     string    `json:"path"`
	Type     string    `json:"type"` // dir, file, link or other
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	Line     int       `json:"line,omitempty"`    // content searches: first matching line
	Excerpt  string    `json:"excerpt,omitempty"` // and some text around the match
}

// SearchResult lists what a search found.
type SearchResult struct {
	Path      string      `json:"path"`
	Hits      []SearchHit `json:"hits"`
	Walked    int         `json:"walked"`
	Skipped   int         `json:"skipped"`   // files too large or binary for a content search
	Truncated bool        `json:"truncated"` // a limit was reached: there may be more
}

// Search looks for files and folders under dir whose name contains name
// (ignoring case) and, when content is set, files that also contain content
// (ignoring case). Symbolic links are listed but never followed, and the
// search stays inside the scope.
func (m *Manager) Search(ctx context.Context, dir, name, content string) (*SearchResult, error) {
	name = strings.TrimSpace(name)
	if strings.TrimSpace(content) == "" {
		content = ""
	}
	if name == "" && content == "" {
		return nil, usererr.New("search_empty")
	}
	if len(name) > maxQueryLen || len(content) > maxQueryLen || strings.ContainsAny(name+content, "\x00\n") {
		return nil, usererr.New("search_invalid")
	}
	t, err := m.open(ctx, dir)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	fi, err := t.root.Stat(t.rel)
	if err != nil {
		return nil, mapErr(err, t.abs)
	}
	if !fi.IsDir() {
		return nil, usererr.New("not_a_folder", t.abs)
	}
	res, err := search(ctx, t.root, t.rel, t.scope.Root, name, content, defaultSearchLimits)
	if res != nil {
		res.Path = t.abs
	}
	return res, err
}

// search walks folder rel of root. Hit paths are joined to base, the
// scope's folder.
func search(ctx context.Context, root *os.Root, rel, base, name, content string, lim searchLimits) (*SearchResult, error) {
	res := &SearchResult{Hits: []SearchHit{}}
	lname := strings.ToLower(name)
	match := contentMatcher(content)
	deadline := time.Now().Add(lim.budget)
	var read int64
	err := walkRoot(root, rel, func(p string, d fs.DirEntry) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if base == "/" && d.IsDir() && pseudoFS[p] {
			return fs.SkipDir
		}
		if res.Walked >= lim.walk || read > lim.bytes || time.Now().After(deadline) {
			res.Truncated = true
			return fs.SkipAll
		}
		res.Walked++
		if lname != "" && !strings.Contains(strings.ToLower(d.Name()), lname) {
			return nil
		}
		hit := SearchHit{Path: path.Join(base, p), Type: entryType(d.Type())}
		if match != nil {
			if !d.Type().IsRegular() {
				return nil
			}
			info, err := root.Lstat(p)
			if err != nil || !info.Mode().IsRegular() {
				return nil
			}
			if info.Size() == 0 {
				return nil
			}
			if info.Size() > lim.fileSize {
				res.Skipped++
				return nil
			}
			b, err := readLimited(root, p, lim.fileSize)
			read += int64(len(b))
			if err != nil {
				return nil
			}
			if isBinary(b) {
				res.Skipped++
				return nil
			}
			start, end := match(b)
			if start < 0 {
				return nil
			}
			hit.Line, hit.Excerpt = lineAndExcerpt(b, start, end)
		}
		if info, err := root.Lstat(p); err == nil {
			hit.Size, hit.Modified = info.Size(), info.ModTime()
		}
		if hit.Type != "file" {
			hit.Size = 0
		}
		if len(res.Hits) >= lim.hits {
			res.Truncated = true
			return fs.SkipAll
		}
		res.Hits = append(res.Hits, hit)
		return nil
	})
	if err != nil {
		return nil, mapErr(err, path.Join(base, rel))
	}
	return res, nil
}

func readLimited(root *os.Root, p string, limit int64) ([]byte, error) {
	f, err := openRegular(root, p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, limit))
}

// openRegular opens a regular file of root for reading. It never waits on a
// FIFO put in the file's place, and refuses anything but a regular file.
func openRegular(root *os.Root, name string) (*os.File, error) {
	f, err := root.OpenFile(name, os.O_RDONLY|openNonblock, 0)
	if err != nil {
		return nil, err
	}
	if fi, err := f.Stat(); err != nil || !fi.Mode().IsRegular() {
		f.Close()
		return nil, usererr.New("not_a_file", name)
	}
	return f, nil
}

// pseudoFS are the server's virtual file systems, left out of searches from
// "/": they are huge, and some files there block when read (/proc/kmsg).
var pseudoFS = map[string]bool{"proc": true, "sys": true, "dev": true}

// contentMatcher returns a function that finds q in a file ignoring case,
// or nil when q is empty. An ASCII query is matched byte by byte, which is
// much faster than a regular expression.
func contentMatcher(q string) func([]byte) (int, int) {
	if q == "" {
		return nil
	}
	if isASCII(q) {
		lq := []byte(strings.ToLower(q))
		return func(b []byte) (int, int) {
			i := bytes.Index(asciiLower(b), lq)
			if i < 0 {
				return -1, -1
			}
			return i, i + len(lq)
		}
	}
	re := regexp.MustCompile("(?i)" + regexp.QuoteMeta(q))
	return func(b []byte) (int, int) {
		loc := re.FindIndex(b)
		if loc == nil {
			return -1, -1
		}
		return loc[0], loc[1]
	}
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// asciiLower lowercases A-Z only, so byte offsets stay the same.
func asciiLower(b []byte) []byte {
	out := make([]byte, len(b))
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		out[i] = c
	}
	return out
}

// lineAndExcerpt returns the line number of the match b[start:end] and
// about excerptWidth bytes of its line around it.
func lineAndExcerpt(b []byte, start, end int) (int, string) {
	line := bytes.Count(b[:start], []byte{'\n'}) + 1
	ls := bytes.LastIndexByte(b[:start], '\n') + 1
	le := len(b)
	if i := bytes.IndexByte(b[start:], '\n'); i >= 0 {
		le = start + i
	}
	return line, excerpt(b[ls:le], start-ls, end-ls, excerptWidth)
}

func excerpt(line []byte, ms, me, width int) string {
	from, to := 0, len(line)
	if len(line) > width {
		from = ms - (width-(me-ms))/2
		if from < 0 {
			from = 0
		}
		to = from + width
		if to > len(line) {
			to = len(line)
			from = max(0, to-width)
		}
		for from > 0 && !utf8.RuneStart(line[from]) {
			from++
		}
		for to < len(line) && !utf8.RuneStart(line[to]) {
			to--
		}
	}
	s := strings.ToValidUTF8(string(line[from:to]), "�")
	s = strings.TrimSpace(strings.ReplaceAll(s, "\t", " "))
	if from > 0 {
		s = "…" + s
	}
	if to < len(line) {
		s += "…"
	}
	return s
}
