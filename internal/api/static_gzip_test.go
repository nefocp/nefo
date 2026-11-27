package api

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestGzipAssets(t *testing.T) {
	big := strings.Repeat("console.log('nefo');\n", 400)
	g := newGzipAssets(fstest.MapFS{
		"assets/index-abc.js": {Data: []byte(big)},
		"assets/tiny.css":     {Data: []byte("a{}")},
		"assets/logo.png":     {Data: []byte(big)},
	})
	get := func(name, enc string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/"+name, nil)
		if enc != "" {
			r.Header.Set("Accept-Encoding", enc)
		}
		w := httptest.NewRecorder()
		if !g.serve(w, r, name) {
			w.Code = 0 // the caller serves it as it is
		}
		return w
	}

	w := get("assets/index-abc.js", "gzip, deflate, br")
	if w.Code != http.StatusOK || w.Header().Get("Content-Encoding") != "gzip" || w.Body.Len() >= len(big)/3 {
		t.Fatalf("js: %d %v %d bytes", w.Code, w.Header(), w.Body.Len())
	}
	zr, err := gzip.NewReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := io.ReadAll(zr); string(b) != big {
		t.Fatal("the compressed copy does not give the file back")
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "javascript") || w.Header().Get("Vary") != "Accept-Encoding" {
		t.Fatalf("headers %v", w.Header())
	}
	// Without gzip, small files, images and files that don't exist are
	// served as they are, and nothing unknown is cached.
	for _, c := range []struct{ name, enc string }{
		{"assets/index-abc.js", ""}, {"assets/tiny.css", "gzip"}, {"assets/logo.png", "gzip"},
		{"assets/nope.js", "gzip"}, {"assets/../x.js", "gzip"},
	} {
		if w := get(c.name, c.enc); w.Code != 0 {
			t.Errorf("%s (%q): answered %d", c.name, c.enc, w.Code)
		}
	}
	if _, ok := g.cache["assets/nope.js"]; ok || len(g.cache) != 2 {
		t.Fatalf("cache %v", g.cache)
	}
	// HEAD has the headers and no body.
	r := httptest.NewRequest(http.MethodHead, "/assets/index-abc.js", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	hw := httptest.NewRecorder()
	if !g.serve(hw, r, "assets/index-abc.js") || hw.Body.Len() != 0 || hw.Header().Get("Content-Length") == "" {
		t.Fatalf("HEAD: %v %d", hw.Header(), hw.Body.Len())
	}
}
