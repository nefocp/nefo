package hosting

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/appstore"
	"github.com/nefocp/nefo/internal/store"
)

func TestSetEnv(t *testing.T) {
	in := "APP_NAME=Laravel\nAPP_ENV=local\nDB_CONNECTION=sqlite\n# DB_HOST=127.0.0.1\n# DB_DATABASE=laravel\n\nAPP_URL=http://localhost"
	out := setEnv(in, map[string]string{"APP_ENV": "production", "DB_CONNECTION": "mariadb", "DB_HOST": "localhost",
		"DB_DATABASE": "lv_x", "DB_PASSWORD": "p@ss!word#1", "APP_URL": "https://x.com"})
	for _, want := range []string{"APP_NAME=Laravel", "APP_ENV='production'", "DB_CONNECTION='mariadb'", "DB_HOST='localhost'",
		"DB_DATABASE='lv_x'", "APP_URL='https://x.com'", "DB_PASSWORD='p@ss!word#1'"} {
		if !strings.Contains(out, want+"\n") && !strings.HasSuffix(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "# DB_HOST") || strings.Count(out, "DB_HOST=") != 1 {
		t.Errorf("commented key not replaced once:\n%s", out)
	}
}

func TestExtractTar(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	add := func(name string, typ byte, body string) {
		tw.WriteHeader(&tar.Header{Name: name, Typeflag: typ, Mode: 0o777, Size: int64(len(body)), Linkname: "/etc/passwd"})
		tw.Write([]byte(body))
	}
	add("nextcloud/", tar.TypeDir, "")
	add("nextcloud/index.php", tar.TypeReg, "<?php")
	add("nextcloud/lib/base.php", tar.TypeReg, "base")
	add("nextcloud/../../escape.php", tar.TypeReg, "x")
	add("nextcloud/link", tar.TypeSymlink, "")
	add("other/file", tar.TypeReg, "y")
	tw.Close()

	base := t.TempDir()
	dir := filepath.Join(base, "public")
	os.Mkdir(dir, 0o755)
	n, err := extractTar(tar.NewReader(&buf), dir, "nextcloud/", -1, -1)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("extracted %d files", n)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "lib", "base.php")); string(b) != "base" {
		t.Fatalf("index.php = %q", b)
	}
	for _, p := range []string{filepath.Join(base, "escape.php"), filepath.Join(dir, "link"), filepath.Join(dir, "file"), filepath.Join(dir, "other")} {
		if _, err := os.Lstat(p); err == nil {
			t.Errorf("%s should not exist", p)
		}
	}
}

func TestDockerAppCatalog(t *testing.T) {
	a, ok := siteDockerApp(&store.Site{App: "docker:ghost"})
	if !ok || a.Docker.Legacy == nil {
		t.Fatal("docker:ghost not recognised")
	}
	if _, ok := siteDockerApp(&store.Site{App: "wordpress"}); ok {
		t.Fatal("wordpress taken for a Docker app")
	}
	// Every app installed before the catalog can be converted.
	for id, l := range legacyApps {
		c, ok := appstore.Lookup(id)
		if !ok || c.Docker.Legacy == nil || !strings.HasPrefix(l.image, c.Docker.Legacy.Images[0]+":") {
			t.Errorf("%s: no legacy conversion in the catalog", id)
		}
		env := strings.Join(l.env("https://x.example.com", "x.example.com", true), " ")
		if strings.ContainsAny(env, "\n\x00") {
			t.Errorf("%s: bad env", id)
		}
	}
}

func TestAppCatalog(t *testing.T) {
	list := PHPApps()
	if len(list) < 8 {
		t.Fatalf("%d PHP apps", len(list))
	}
	for _, p := range list {
		a, _ := phpApp(p.ID)
		if _, ok := rewritePreset(a.PHP.Rewrite); !ok {
			t.Errorf("%s: unknown rewrite preset %q", a.ID, a.PHP.Rewrite)
		}
	}
	if AppDBPrefix("nextcloud") != "nc" || AppDBPrefix("moodle") != "md" || AppDBPrefix("nope") != "app" {
		t.Fatal("AppDBPrefix")
	}
	if !isWordPressApp("woocommerce") || isWordPressApp("joomla") {
		t.Fatal("isWordPressApp")
	}
	if appMaxInputVars("moodle") != 5000 || appMaxInputVars("wordpress") != 0 {
		t.Fatal("max_input_vars")
	}
	pool, err := renderPool(poolData{Domain: "lms.example.com", User: "lms", Root: "/var/www/lms.example.com", Socket: "/run/php/x.sock",
		MemoryMB: 256, UploadMB: 64, TimeSec: 300, PM: "ondemand", Workers: 5, MaxInputVars: 5000})
	if err != nil || !strings.Contains(string(pool), "php_value[max_input_vars] = 5000\n") {
		t.Fatalf("pool: %v\n%s", err, pool)
	}
}

func TestCheckInputs(t *testing.T) {
	specs := map[string]*appstore.Input{"country": {Type: "country", Default: "US"}, "currency": {Type: "currency", Default: "USD"},
		"port": {Type: "port", Optional: true}}
	got, err := checkInputs(specs, map[string]string{"country": "vn"})
	if err != nil || got["country"] != "VN" || got["currency"] != "USD" {
		t.Fatalf("%v %v", got, err)
	}
	for _, bad := range []map[string]string{{"country": "VNM"}, {"currency": "D"}, {"port": "80"}, {"other": "x"}} {
		if _, err := checkInputs(specs, bad); err == nil {
			t.Errorf("%v accepted", bad)
		}
	}
}

func TestBreakingUpdate(t *testing.T) {
	for _, c := range []struct {
		breaking, from, to string
		want               bool
	}{
		{"major", "18.1", "18.7", false}, {"major", "17.12", "18.7", true}, {"minor", "8.0.38", "8.0.40", false},
		{"minor", "8.0.38", "8.2.1", true}, {"", "1.27.3", "28.4.0", false},
	} {
		if got := breakingUpdate(c.breaking, c.from, c.to); got != c.want {
			t.Errorf("%+v: %v", c, got)
		}
	}
}

func TestParseMemMB(t *testing.T) {
	for in, want := range map[string]int{"40.71MiB": 41, "1.2GiB": 1229, "512KiB": 1, "0B": 0, "x": 0} {
		if got := parseMemMB(in); got != want {
			t.Errorf("%s: %d", in, got)
		}
	}
}

func TestFreeSiteID(t *testing.T) {
	old := idTaken
	defer func() { idTaken = old }()
	idTaken = func(id int) bool { return id < 20003 }
	if id, err := freeSiteID(); err != nil || id != 20003 {
		t.Fatalf("%d %v", id, err)
	}
	idTaken = func(int) bool { return true }
	if _, err := freeSiteID(); err == nil {
		t.Fatal("full range accepted")
	}
}

func TestPHPAppCron(t *testing.T) {
	m := &Manager{paths: Paths{WWW: "/var/www"}}
	s, c, ok := m.PHPAppCron(&store.Site{Domain: "lms.example.com", PHPVersion: "8.3"}, "moodle")
	if !ok || s != "* * * * *" || c != "php8.3 -d max_input_vars=5000 /var/www/lms.example.com/admin/cli/cron.php" {
		t.Fatalf("%q %q", s, c)
	}
	if _, c, _ := m.PHPAppCron(&store.Site{Domain: "shop.example.com", PHPVersion: "8.3"}, "woocommerce"); c != "php8.3 /var/www/shop.example.com/public/wp-cron.php" {
		t.Fatalf("%q", c)
	}
	if _, _, ok := m.PHPAppCron(&store.Site{Domain: "x.example.com", PHPVersion: "8.3"}, "joomla"); ok {
		t.Fatal("joomla has no cron")
	}
}

func TestOpenCartStorage(t *testing.T) {
	in := []byte("define('DIR_SYSTEM', DIR_OPENCART . 'system/');\ndefine('DIR_STORAGE', DIR_SYSTEM . 'storage/');\n")
	out := ocStorageRe.ReplaceAllLiteral(in, []byte("define('DIR_STORAGE', '/var/www/x/storage/');"))
	if !bytes.Contains(out, []byte("define('DIR_STORAGE', '/var/www/x/storage/');")) {
		t.Fatalf("%s", out)
	}
}

func TestExtractZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name, body string, mode os.FileMode) {
		h := &zip.FileHeader{Name: name}
		h.SetMode(mode)
		w, _ := zw.CreateHeader(h)
		w.Write([]byte(body))
	}
	add("upload/", "", os.ModeDir|0o755)
	add("upload/index.php", "<?php", 0o644)
	add("upload/admin/config-dist.php", "cfg", 0o644)
	add("upload/../../escape.php", "x", 0o644)
	add("upload/link", "/etc/passwd", os.ModeSymlink|0o777)
	add("README.md", "y", 0o644)
	zw.Close()
	zr, _ := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	base := t.TempDir()
	dir := filepath.Join(base, "public")
	os.Mkdir(dir, 0o755)
	n, err := extractZip(zr, dir, "upload/", -1, -1)
	if err != nil || n != 2 {
		t.Fatalf("%d %v", n, err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "admin", "config-dist.php")); string(b) != "cfg" {
		t.Fatal("content")
	}
	for _, p := range []string{filepath.Join(base, "escape.php"), filepath.Join(dir, "link"), filepath.Join(dir, "README.md")} {
		if _, err := os.Lstat(p); err == nil {
			t.Errorf("%s should not exist", p)
		}
	}
}

func TestAdminEmailAndPinning(t *testing.T) {
	for _, bad := range []string{"", "a@b", "x@example.test", "x@host.local", "a b@example.com", "x@example.com\nY: z"} {
		if checkAdminEmail(bad) == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if checkAdminEmail("owner@example.com") != nil {
		t.Error("a normal address refused")
	}
	vw, _ := appstore.Lookup("vaultwarden")
	if !NeedsAdminEmail(vw) {
		t.Error("Vaultwarden invites the administrator's address")
	}
	if pg, _ := appstore.Lookup("postgresql"); NeedsAdminEmail(pg) {
		t.Error("PostgreSQL asks for no e-mail")
	}
	// A re-render that is not an update keeps the instance's images.
	m := &Manager{}
	inst := &store.App{Images: "vaultwarden/server:1.36.0-alpine@sha256:" + strings.Repeat("a", 64)}
	got := m.pinnedApp(vw, inst)
	if s := got.Docker.Services["app"]; s.Image != "vaultwarden/server:1.36.0-alpine" || s.Digest != "sha256:"+strings.Repeat("a", 64) {
		t.Errorf("pinned %s@%s", s.Image, s.Digest)
	}
	if vw.Docker.Services["app"].Image != "vaultwarden/server:1.37.5-alpine" {
		t.Error("the catalog entry was changed")
	}
	inst.Images = imageList(vw)
	if m.pinnedApp(vw, inst) != vw {
		t.Error("same images: the recipe itself")
	}
}

func TestClearOtherBits(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permissions")
	}
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "cache"), 0o700)
	os.Chmod(filepath.Join(dir, "cache"), 0o777|os.ModeSetgid)
	os.WriteFile(filepath.Join(dir, ".htaccess"), []byte("deny from all"), 0o600)
	os.Chmod(filepath.Join(dir, ".htaccess"), 0o666)
	outside := filepath.Join(t.TempDir(), "outside")
	os.WriteFile(outside, nil, 0o644)
	os.Chmod(outside, 0o666)
	os.Symlink(outside, filepath.Join(dir, "link"))
	if err := clearOtherBits(dir); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]os.FileMode{"cache": 0o770 | os.ModeSetgid | os.ModeDir, ".htaccess": 0o660} {
		fi, err := os.Stat(filepath.Join(dir, name))
		if err != nil || fi.Mode() != want {
			t.Errorf("%s: %v, want %v (%v)", name, fi.Mode(), want, err)
		}
	}
	if fi, err := os.Stat(outside); err != nil || fi.Mode().Perm() != 0o666 {
		t.Error("followed the link out of the folder")
	}
}
