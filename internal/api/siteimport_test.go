package api

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/siteimport"
	"github.com/nefocp/nefo/internal/usererr"
)

// writeCPMove writes a small cPanel backup with one website.
func writeCPMove(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "cpmove-bob.tar.gz")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for name, body := range map[string]string{
		"cpmove-bob/userdata/main":                  "main_domain: example.com\n",
		"cpmove-bob/userdata/example.com":           "documentroot: /home/bob/public_html\nservername: example.com\nhomedir: /home/bob\nuser: bob\n",
		"cpmove-bob/homedir/public_html/index.html": "hello",
	} {
		tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg, ModTime: time.Now()})
		tw.Write([]byte(body))
	}
	tw.Close()
	gz.Close()
	f.Close()
	return p
}

// readySession adds an analysed import to the server, as the analysis job
// would.
func readySession(t *testing.T, e *testEnv) *importSession {
	t.Helper()
	p := writeCPMove(t, t.TempDir())
	f, a, err := siteimport.Open(p, false)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	c, err := siteimport.AnalyseCPanel(context.Background(), f, a, siteimport.DefaultLimits, time.Now(), discardLog{})
	if err != nil {
		t.Fatal(err)
	}
	sess := &importSession{ID: "testsession", Kind: siteimport.KindCPanel, Created: time.Now(), status: "ready", arch: a, cpanel: c}
	e.srv.imports.put(sess)
	return sess
}

type discardLog struct{}

func (discardLog) Logf(string, ...any) {}

func TestImportSessions(t *testing.T) {
	e := newEnv(t)
	e.login(t)
	if res, out := e.do(t, "GET", "/api/import/nope", nil, nil); res.StatusCode != 404 || out["error"] != "import_not_found" {
		t.Fatalf("unknown import: %d %v", res.StatusCode, out)
	}
	if res, out := e.do(t, "POST", "/api/import", map[string]any{"kind": "other"}, nil); res.StatusCode == 202 {
		t.Fatalf("bad kind started: %v", out)
	}
	readySession(t, e)
	res, out := e.do(t, "GET", "/api/import/testsession", nil, nil)
	if res.StatusCode != 200 || out["status"] != "ready" {
		t.Fatalf("get: %d %v", res.StatusCode, out)
	}
	plan, _ := out["plan"].(map[string]any)
	doms, _ := plan["domains"].([]any)
	if len(doms) != 1 || doms[0].(map[string]any)["name"] != "example.com" {
		t.Fatalf("plan: %v", plan)
	}
	for sel, code := range map[string]string{`{}`: "import_nothing", `{"domains":["other.com"]}`: "import_unknown_item"} {
		var body map[string]any
		_ = json.Unmarshal([]byte(sel), &body)
		if res, out := e.do(t, "POST", "/api/import/testsession/run", body, nil); res.StatusCode != 400 || out["error"] != code {
			t.Errorf("%s: %d %v", sel, res.StatusCode, out)
		}
	}
	if res, out := e.do(t, "GET", "/api/import", nil, nil); res.StatusCode != 200 || len(out["imports"].([]any)) != 1 {
		t.Fatalf("list: %v", out)
	}
	if res, _ := e.do(t, "DELETE", "/api/import/testsession", nil, nil); res.StatusCode != 200 {
		t.Fatalf("delete: %d", res.StatusCode)
	}
	if res, _ := e.do(t, "GET", "/api/import/testsession", nil, nil); res.StatusCode != 404 {
		t.Fatalf("still there: %d", res.StatusCode)
	}
}

func TestImportChangedArchiveRefused(t *testing.T) {
	e := newEnv(t)
	e.login(t)
	sess := readySession(t, e)
	if err := os.WriteFile(sess.arch.Path, []byte("\x1f\x8bchanged"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, out := e.do(t, "POST", "/api/import/testsession/run", map[string]any{"domains": []string{"example.com"}}, nil)
	if res.StatusCode != 400 || out["error"] != "import_changed" {
		t.Fatalf("changed archive: %d %v", res.StatusCode, out)
	}
}

func TestImportDeniesPanelData(t *testing.T) {
	e := newEnv(t)
	p := filepath.Join(e.srv.cfg.DataDir, "secret.tar.gz")
	os.WriteFile(p, []byte("\x1f\x8b\x08"), 0o600)
	if _, _, err := siteimport.Open(p, false, e.srv.importDeny()...); !isUserCode(err, "import_path_denied") {
		t.Fatalf("panel data: %v", err)
	}
}

func TestEmptyPublic(t *testing.T) {
	dir := t.TempDir()
	pub := filepath.Join(dir, "public")
	os.MkdirAll(pub, 0o755)
	check := func() bool { return emptyPublic(pub) }
	if !check() {
		t.Fatal("an empty folder is not empty")
	}
	os.WriteFile(filepath.Join(pub, "index.html"), []byte("<p>"+placeholderText+"</p>"), 0o644)
	if !check() {
		t.Fatal("the placeholder page counts")
	}
	os.WriteFile(filepath.Join(pub, "index.html"), []byte("my own page"), 0o644)
	if check() {
		t.Fatal("an own index.html is empty")
	}
	os.Remove(filepath.Join(pub, "index.html"))
	os.WriteFile(filepath.Join(pub, "app.php"), []byte("x"), 0o644)
	if check() {
		t.Fatal("a file is empty")
	}
}

func TestImportErrSentences(t *testing.T) {
	err := importErr(usererr.New("import_not_cpanel", "site.tar.gz"))
	if !strings.Contains(err.Error(), "site.tar.gz: this is not a cPanel full backup") {
		t.Fatalf("%v", err)
	}
	other := errors.New("boom")
	if importErr(other) != other {
		t.Fatal("other errors change")
	}
}

func isUserCode(err error, code string) bool {
	var ue *usererr.Error
	return errors.As(err, &ue) && ue.Code == code
}
