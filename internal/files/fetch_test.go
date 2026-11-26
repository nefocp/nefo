package files

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestSanitizeName(t *testing.T) {
	long := strings.Repeat("é", 150) + ".tar.gz"
	for in, want := range map[string]string{
		"report.pdf":              "report.pdf",
		"../../etc/passwd":        "passwd",
		`..\..\windows\evil.exe`:  "evil.exe",
		"/abs/path/file.txt":      "file.txt",
		".htaccess":               "htaccess",
		"...user.ini":             "user.ini",
		"..":                      "",
		".":                       "",
		"":                        "",
		"dir/":                    "dir",
		"a\x00b\x01c.txt":         "abc.txt",
		"line\nbreak.txt":         "line break.txt",
		"photo‮gpj.exe":           "photogpj.exe", // right-to-left override hides the real extension
		"  spaced name.zip  ":     "spaced name.zip",
		"trailing dots...":        "trailing dots",
		"Tài liệu tiếng Việt.doc": "Tài liệu tiếng Việt.doc",
		"bad\xffutf8.txt":         "bad_utf8.txt",
	} {
		if got := sanitizeName(in); got != want {
			t.Errorf("sanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
	got := sanitizeName(long)
	if len(got) > maxNameBytes || !strings.HasSuffix(got, ".tar.gz") || !ValidName(got) {
		t.Errorf("long name = %q (%d bytes)", got, len(got))
	}
}

func TestDownloadName(t *testing.T) {
	u := func(s string) *url.URL { v, _ := url.Parse(s); return v }
	cases := []struct {
		cd   string
		urls []*url.URL
		want string
	}{
		{`attachment; filename="../../evil.php"`, []*url.URL{u("https://x.test/a")}, "evil.php"},
		{`attachment; filename*=UTF-8''na%C3%AFve%20file.txt`, nil, "naïve file.txt"},
		{`attachment; filename=".user.ini"`, nil, "user.ini"},
		{`not a valid header;;`, []*url.URL{u("https://x.test/dl/latest.zip?x=1")}, "latest.zip"},
		{"", []*url.URL{u("https://x.test/"), u("https://cdn.test/files/app.tar.gz")}, "app.tar.gz"},
		{"", []*url.URL{u("https://x.test/files/caf%C3%A9.txt")}, "café.txt"},
		{"", []*url.URL{u("https://x.test/")}, "download"},
	}
	for _, c := range cases {
		if got := downloadName(c.cd, c.urls...); got != c.want {
			t.Errorf("downloadName(%q) = %q, want %q", c.cd, got, c.want)
		}
	}
	for name, want := range map[string]string{"a.txt": "a (2).txt", "b.tar.gz": "b (2).tar.gz", "README": "README (2)"} {
		if got := numberedName(name, 2); got != want {
			t.Errorf("numberedName(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestInternalAddr(t *testing.T) {
	for s, want := range map[string]bool{
		"127.0.0.1": true, "127.8.9.10": true, "10.0.0.1": true, "172.16.0.1": true, "172.31.255.255": true,
		"192.168.1.1": true, "169.254.169.254": true, "100.64.0.1": true, "100.127.255.255": true,
		"0.0.0.0": true, "0.1.2.3": true, "255.255.255.255": true, "224.0.0.1": true, "198.18.0.1": true,
		"::1": true, "::": true, "fe80::1": true, "fc00::1": true, "fd12:3456::1": true,
		"::ffff:127.0.0.1": true, "::ffff:10.0.0.1": true, "::127.0.0.1": true, "64:ff9b::7f00:1": true, "64:ff9b::a9fe:a9fe": true,
		"8.8.8.8": false, "1.1.1.1": false, "172.32.0.1": false, "100.128.0.1": false, "93.184.215.14": false,
		"::ffff:8.8.8.8": false, "64:ff9b::808:808": false, "2606:4700:4700::1111": false,
	} {
		if got := internalAddr(netip.MustParseAddr(s)); got != want {
			t.Errorf("internalAddr(%s) = %v, want %v", s, got, want)
		}
	}
}

func TestCheckFetchURL(t *testing.T) {
	ctx := context.Background()
	for _, raw := range []string{"", "ftp://example.com/x", "file:///etc/passwd", "javascript:alert(1)", "http://", "http:///x", "example.com/x", "https://" + strings.Repeat("a", 3000)} {
		if _, err := parseFetchURL(raw); code(err) != "invalid_url" {
			t.Errorf("parseFetchURL(%q): %v", raw, err)
		}
	}
	for _, raw := range []string{"http://127.0.0.1/", "http://169.254.169.254/latest/meta-data/", "http://[::1]:8080/", "http://10.1.2.3/",
		"http://[::ffff:127.0.0.1]/", "http://100.64.1.1/", "http://0.0.0.0:22/", "http://localhost/"} {
		u, err := parseFetchURL(raw)
		if err != nil {
			t.Fatalf("parseFetchURL(%q): %v", raw, err)
		}
		if err := checkFetchHost(ctx, u, publicAddr); code(err) != "url_not_allowed" {
			t.Errorf("checkFetchHost(%q): %v", raw, err)
		}
	}
	u, _ := parseFetchURL("https://93.184.215.14:8443/file.zip")
	if err := checkFetchHost(ctx, u, publicAddr); err != nil {
		t.Errorf("public address refused: %v", err)
	}
	u, _ = parseFetchURL("http://93.184.215.14:99999/")
	if err := checkFetchHost(ctx, u, publicAddr); code(err) != "invalid_url" {
		t.Errorf("bad port: %v", err)
	}
}

// fetchServer serves test downloads; the returned check allows the server
// itself and applies the real rules to every other address.
func fetchServer(t *testing.T) (*httptest.Server, func(netip.AddrPort) error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/file.txt", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "hello world") })
	mux.HandleFunc("/named", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="../../.htaccess"`)
		fmt.Fprint(w, "deny from all")
	})
	mux.HandleFunc("/gz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		w.Write([]byte{0x1f, 0x8b, 1, 2, 3})
	})
	mux.HandleFunc("/big", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("length") == "1" {
			w.Header().Set("Content-Length", "5000")
		}
		w.(http.Flusher).Flush()
		w.Write([]byte(strings.Repeat("x", 5000)))
	})
	mux.HandleFunc("/short", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		w.Write([]byte("only ten b"))
	})
	mux.HandleFunc("/missing", http.NotFound)
	mux.HandleFunc("/metadata", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
	})
	mux.HandleFunc("/hop/", func(w http.ResponseWriter, r *http.Request) {
		n, _ := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/hop/"))
		last, _ := strconv.Atoi(r.URL.Query().Get("last"))
		if n < last {
			http.Redirect(w, r, fmt.Sprintf("/hop/%d?last=%d", n+1, last), http.StatusFound)
			return
		}
		fmt.Fprint(w, "arrived")
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	self := netip.MustParseAddrPort(srv.Listener.Addr().String())
	return srv, func(ap netip.AddrPort) error {
		if ap == self {
			return nil
		}
		return publicAddr(ap)
	}
}

type testLog struct{ lines []string }

func (l *testLog) Logf(format string, args ...any) {
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

func fetchHere(t *testing.T, check func(netip.AddrPort) error, raw string, limit int64) (*os.Root, string, string, error) {
	t.Helper()
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("already here"), 0o644)
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	client, tr := newFetchClient(check)
	defer tr.CloseIdleConnections()
	rel, _, err := fetchTo(context.Background(), client, u, root, ".", -1, -1, limit, &testLog{})
	return root, dir, rel, err
}

// noPartials fails if a temporary download file was left behind.
func noPartials(t *testing.T, dir string) {
	t.Helper()
	des, _ := os.ReadDir(dir)
	for _, de := range des {
		if strings.HasPrefix(de.Name(), ".panel-download-") {
			t.Errorf("partial file %s left behind", de.Name())
		}
	}
}

func TestFetch(t *testing.T) {
	srv, check := fetchServer(t)

	// An existing file is never replaced.
	_, dir, rel, err := fetchHere(t, check, srv.URL+"/file.txt", 1<<20)
	if err != nil || rel != "file (1).txt" {
		t.Fatalf("fetch = %q, %v", rel, err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "file (1).txt")); string(b) != "hello world" {
		t.Fatalf("content = %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "file.txt")); string(b) != "already here" {
		t.Fatalf("existing file changed to %q", b)
	}
	if runtime.GOOS != "windows" {
		if fi, _ := os.Stat(filepath.Join(dir, "file (1).txt")); fi.Mode().Perm() != 0o640 {
			t.Errorf("mode = %v", fi.Mode().Perm())
		}
	}
	noPartials(t, dir)

	// The server's name is cleaned up.
	if _, _, rel, err := fetchHere(t, check, srv.URL+"/named", 1<<20); err != nil || rel != "htaccess" {
		t.Errorf("named = %q, %v", rel, err)
	}
	// Bytes are saved as served, not decompressed.
	if _, dir, rel, err := fetchHere(t, check, srv.URL+"/gz", 1<<20); err != nil {
		t.Errorf("gz: %v", err)
	} else if b, _ := os.ReadFile(filepath.Join(dir, rel)); len(b) != 5 {
		t.Errorf("gz content changed: %v", b)
	}
	// Five redirects are followed, six are not.
	// The name comes from the URL asked for, like curl -O.
	if _, _, rel, err := fetchHere(t, check, srv.URL+"/hop/0?last=5", 1<<20); err != nil || rel != "0" {
		t.Errorf("5 redirects: %q, %v", rel, err)
	}
	if _, dir, _, err := fetchHere(t, check, srv.URL+"/hop/0?last=6", 1<<20); err == nil || !strings.Contains(err.Error(), "redirects") {
		t.Errorf("6 redirects: %v", err)
	} else {
		noPartials(t, dir)
	}
}

func TestFetchFailuresLeaveNothing(t *testing.T) {
	srv, check := fetchServer(t)
	for _, c := range []struct{ path, want string }{
		{"/big", "limit"},
		{"/big?length=1", "limit"},
		{"/short", "interrupted"},
		{"/missing", "404"},
		{"/metadata", "internal address"}, // a redirect to an internal address is refused
	} {
		_, dir, _, err := fetchHere(t, check, srv.URL+c.path, 1000)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: %v, want an error with %q", c.path, err, c.want)
		}
		noPartials(t, dir)
		des, _ := os.ReadDir(dir)
		if len(des) != 1 {
			t.Errorf("%s: folder holds %d items", c.path, len(des))
		}
	}
}

func TestFetchRefusesLoopback(t *testing.T) {
	srv, _ := fetchServer(t)
	// With the real rules, the test server on 127.0.0.1 cannot be reached.
	_, dir, _, err := fetchHere(t, publicAddr, srv.URL+"/file.txt", 1000)
	if err == nil || !strings.Contains(err.Error(), "internal address") {
		t.Fatalf("loopback download: %v", err)
	}
	noPartials(t, dir)
	// A name that resolves to loopback is refused when dialling too.
	_, _, _, err = fetchHere(t, publicAddr, strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)+"/file.txt", 1000)
	if err == nil {
		t.Fatal("localhost download succeeded")
	}
}

func TestManagerFetch(t *testing.T) {
	m, site := newTest(t)
	srv, check := fetchServer(t)
	m.checkAddr = check
	os.Mkdir(site+"/public", 0o750)
	uid, gid := os.Getuid(), os.Getgid()
	if uid == 0 {
		uid, gid = 4242, 4343
		os.Chown(site+"/public", uid, gid)
	}
	ctx := context.Background()
	if _, err := m.CheckFetch(ctx, site+"/public", "http://127.0.0.1:1/x"); code(err) != "url_not_allowed" {
		t.Fatalf("CheckFetch of another loopback port: %v", err)
	}
	u, err := m.CheckFetch(ctx, site+"/public", srv.URL+"/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	p, err := m.Fetch(ctx, site+"/public", u, &testLog{})
	if err != nil || p != site+"/public/file.txt" {
		t.Fatalf("Fetch = %q, %v", p, err)
	}
	fi, err := os.Stat(p)
	if err != nil || fi.Mode().Perm() != 0o640 {
		t.Fatalf("mode: %v %v", fi, err)
	}
	if fu, fg := ownerOf(t, p); fu != uid || fg != gid {
		t.Errorf("owner = %d:%d, want %d:%d", fu, fg, uid, gid)
	}
	if _, err := m.CheckFetch(ctx, site+"/public/file.txt", "https://example.com/x"); code(err) != "not_a_folder" {
		t.Errorf("into a file: %v", err)
	}
}
