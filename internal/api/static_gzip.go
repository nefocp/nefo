package api

import (
	"bytes"
	"compress/gzip"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
)

// gzipAssets keeps gzip-compressed copies of the interface's text assets,
// made the first time each is asked for: the JavaScript and the language
// files shrink to about a third, which a first visit over a slow link
// notices. Only files that exist in the embedded UI are kept, so the cache
// can't grow past the UI's own size (under 1 MB compressed).
type gzipAssets struct {
	files fs.FS
	mu    sync.Mutex
	cache map[string][]byte // nil value: exists but not worth compressing
}

func newGzipAssets(files fs.FS) *gzipAssets {
	return &gzipAssets{files: files, cache: map[string][]byte{}}
}

// compressible reports whether a file is text that gzip shrinks.
func compressible(name string) bool {
	switch path.Ext(name) {
	case ".js", ".css", ".svg", ".json", ".html", ".txt":
		return true
	}
	return false
}

func (g *gzipAssets) get(name string) ([]byte, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if b, ok := g.cache[name]; ok {
		return b, b != nil
	}
	if !fs.ValidPath(name) {
		return nil, false
	}
	data, err := fs.ReadFile(g.files, name)
	if err != nil {
		return nil, false // not in the UI: never cached
	}
	if len(data) < 1024 {
		g.cache[name] = nil
		return nil, false
	}
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	zw.Write(data)
	zw.Close()
	b := buf.Bytes()
	g.cache[name] = b
	return b, true
}

// serve answers with the compressed copy when the browser takes gzip; it
// reports false when the caller should serve the file as it is.
func (g *gzipAssets) serve(w http.ResponseWriter, r *http.Request, name string) bool {
	if !compressible(name) {
		return false
	}
	w.Header().Add("Vary", "Accept-Encoding")
	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		return false
	}
	b, ok := g.get(name)
	if !ok {
		return false
	}
	if ct := mime.TypeByExtension(path.Ext(name)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		w.Write(b)
	}
	return true
}
