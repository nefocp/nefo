package siteimport

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

const aaConfig = `<?php
define('DB_NAME', 'shopdb');
define('DB_USER', 'shopdb');
define('DB_PASSWORD', 'aaPanel-Pass-123');
define('DB_HOST', 'localhost');
`

const aaDump = "-- MySQL dump 10.13  Distrib 8.0.24, for Linux (x86_64)\n--\n-- Host: localhost    Database: shopdb\n" +
	"SET @@GLOBAL.GTID_PURGED=/*!80000 '+'*/ 'x:1-2';\n" +
	"CREATE TABLE `wp_options` (`v` text) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;\n" +
	"INSERT INTO `wp_options` VALUES ('/www/wwwroot/shop.example.com/wp-content');\n"

// aaPanel's backup: web_DOMAIN_DATE_TIME_RANDOM.tar.gz with the site's
// folder at its top, and a dump db_NAME_DATE_TIME_mysql_data.sql.gz.
func aaPanel(t *testing.T, dir string) (string, string) {
	t.Helper()
	top := "shop.example.com/"
	site := makeTarGz(t, dir, "web_shop.example.com_20261120_101010_ab12cd.tar.gz", []te{
		{name: top, typ: tar.TypeDir, mode: 0o755},
		file(top+"index.php", "<?php require 'wp-blog-header.php';"),
		file(top+"wp-config.php", aaConfig),
		file(top+".user.ini", "open_basedir=/www/wwwroot/shop.example.com/:/tmp/\n"),
		file(top+"wp-content/uploads/pic.jpg", "JPEG"),
		modeFile(top+"wp-content/world-writable.php", 0o777),
		link(top+"wp-content/escape", "/etc"),
		file(top+"../../outside.txt", "evil"),
		{name: top + "pipe", typ: tar.TypeFifo},
	})
	dump := writeGz(t, dir, "db_shopdb_20261120_101010_mysql_data.sql.gz", aaDump)
	return site, dump
}

func analyseSite(t *testing.T, path string, dumps ...string) (*os.File, *SiteArchive, []*Dump, map[string]*os.File) {
	t.Helper()
	f, a := openArchive(t, path, false)
	s, err := AnalyseSite(context.Background(), f, a, DefaultLimits, &testLog{})
	if err != nil {
		t.Fatal(err)
	}
	var ds []*Dump
	files := map[string]*os.File{}
	for _, p := range dumps {
		df, da := openArchive(t, p, true)
		d, err := AnalyseDump(df, da)
		if err != nil {
			t.Fatal(err)
		}
		ds = append(ds, d)
		files[da.Path] = df
	}
	return f, s, ds, files
}

func TestSiteArchivePlan(t *testing.T) {
	fh := newFakeHost(t)
	site, dump := aaPanel(t, t.TempDir())
	_, s, ds, _ := analyseSite(t, site, dump)
	p := s.Plan(testEnv(fh), ds)
	ps := p.Site
	if ps.Root != "shop.example.com" || ps.Domain != "shop.example.com" || ps.App != "wordpress" || ps.DB != "shopdb" ||
		ps.OldRoot != "/www/wwwroot/shop.example.com" || ps.PHP != "8.3" {
		t.Fatalf("site: %+v", ps)
	}
	if len(p.Databases) != 1 || p.Databases[0].Name != "shopdb" || p.Databases[0].NewName != "shopdb" ||
		p.Databases[0].Source != dump || !slices.Equal(p.Databases[0].UsedBy, []string{"wordpress"}) {
		t.Fatalf("databases: %+v", p.Databases[0])
	}
}

func TestSiteArchiveImport(t *testing.T) {
	fh := newFakeHost(t)
	site, dump := aaPanel(t, t.TempDir())
	f, s, ds, files := analyseSite(t, site, dump)
	p := s.Plan(testEnv(fh), ds)
	sel := Selection{Domain: "Shop.Example.com.", WWW: true, PHP: "8.3", Databases: []string{dump}}
	lg := &testLog{}
	res, err := RunSite(context.Background(), f, s, p, files, sel, nil, fh.host(), DefaultLimits, lg)
	if err != nil {
		t.Fatalf("%v\n%s", err, lg)
	}
	pub := filepath.Join(fh.root("shop.example.com"), "public")
	if readFile(t, filepath.Join(pub, "wp-content", "uploads", "pic.jpg")) != "JPEG" || exists(filepath.Join(pub, "shop.example.com")) {
		t.Fatal("the top folder was not stripped")
	}
	for _, p := range []string{filepath.Join(pub, "wp-content", "escape"), filepath.Join(pub, "pipe"), filepath.Join(fh.base, "outside.txt"),
		filepath.Join(fh.root("shop.example.com"), "outside.txt"), filepath.Join(pub, "index.html")} {
		if exists(p) {
			t.Errorf("%s exists", p)
		}
	}
	if runtime.GOOS != "windows" {
		if fi, _ := os.Stat(filepath.Join(pub, "wp-content", "world-writable.php")); fi.Mode().Perm() != 0o755 {
			t.Errorf("mode %v", fi.Mode())
		}
	}
	cfg := readFile(t, filepath.Join(pub, "wp-config.php"))
	if !strings.Contains(cfg, "define( 'DB_PASSWORD', 'Generated-Pass-2026' );") || !strings.Contains(cfg, "define( 'DB_NAME', 'shopdb' );") {
		t.Errorf("wp-config.php:\n%s", cfg)
	}
	newPub := filepathSlash(pub)
	if ini := readFile(t, filepath.Join(pub, ".user.ini")); !strings.Contains(ini, newPub+"/:/tmp/") {
		t.Errorf(".user.ini: %s", ini)
	}
	if d := fh.dumps["shopdb"]; strings.Contains(d, "GTID") || !strings.Contains(d, "utf8mb4_unicode_520_ci") || !strings.Contains(d, "/www/wwwroot/shop.example.com/wp-content") {
		t.Errorf("dump: %s", d)
	}
	for _, want := range []string{"create site shop.example.com php=8.3 aliases=www.shop.example.com", "wordpress shop.example.com",
		"replace shop.example.com /www/wwwroot/shop.example.com -> " + newPub, "suspend shop.example.com false"} {
		if !fh.has(want) {
			t.Errorf("missing step %q in %q", want, fh.steps)
		}
	}
	if len(res.Databases) != 1 || res.Databases[0].Manual {
		t.Errorf("result: %+v", res.Databases)
	}
}

// A zip of a home folder: public_html is the website.
func TestZipWithPublicHTML(t *testing.T) {
	fh := newFakeHost(t)
	z := makeZip(t, t.TempDir(), "site.zip", []te{
		file("public_html/index.html", "HELLO"),
		file("public_html/css/a.css", "body{}"),
		file("mail/new/1", "mail"),
		file("..\\..\\evil.txt", "evil"),
		link("public_html/link", "/etc/passwd"),
	})
	f, s, _, _ := analyseSite(t, z)
	p := s.Plan(testEnv(fh), nil)
	if p.Site.Root != "public_html" || p.Site.App != "static" || p.Site.Files != 2 {
		t.Fatalf("site: %+v", p.Site)
	}
	if _, err := RunSite(context.Background(), f, s, p, nil, Selection{Domain: "static.example.com"}, nil, fh.host(), DefaultLimits, &testLog{}); err != nil {
		t.Fatal(err)
	}
	pub := filepath.Join(fh.root("static.example.com"), "public")
	if readFile(t, filepath.Join(pub, "index.html")) != "HELLO" || exists(filepath.Join(pub, "new")) || exists(filepath.Join(pub, "link")) {
		t.Fatal("wrong files")
	}
	if exists(filepath.Join(fh.base, "evil.txt")) || exists(filepath.Join(fh.root("static.example.com"), "evil.txt")) {
		t.Fatal("escaped")
	}
}

// An existing empty website: a failure empties it again and puts its
// placeholder back.
func TestExistingSiteRollsBack(t *testing.T) {
	fh := newFakeHost(t)
	site, dump := aaPanel(t, t.TempDir())
	f, s, ds, files := analyseSite(t, site, dump)
	p := s.Plan(testEnv(fh), ds)
	target, err := fh.host().CreateSite(context.Background(), "empty.example.com", nil, "8.3")
	if err != nil {
		t.Fatal(err)
	}
	target.Disabled, target.Note = false, "my note"
	fh.failDB = "shopdb"
	_, err = RunSite(context.Background(), f, s, p, files, Selection{SiteID: target.ID, Databases: []string{dump}}, target, fh.host(), DefaultLimits, &testLog{})
	if err == nil {
		t.Fatal("no error")
	}
	pub := filepath.Join(fh.root("empty.example.com"), "public")
	entries, _ := os.ReadDir(pub)
	if len(entries) != 1 || entries[0].Name() != "index.html" || readFile(t, filepath.Join(pub, "index.html")) != "placeholder" {
		t.Fatalf("public: %v", entries)
	}
	if fh.sites["empty.example.com"] == nil || len(fh.dbs) != 0 || fh.notes["empty.example.com"] != "my note" {
		t.Fatalf("site %v, databases %v, note %q", fh.sites, fh.dbs, fh.notes["empty.example.com"])
	}
	if !fh.has("suspend empty.example.com true") || !fh.has("suspend empty.example.com false") {
		t.Fatalf("steps %q", fh.steps)
	}
}

// ---- limits ----

func extractAll(t *testing.T, path string, lim Limits) (*extractor, error) {
	t.Helper()
	f, a := openArchive(t, path, false)
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	x := &extractor{root: root, uid: -1, gid: -1, lim: lim, budget: &budget{left: lim.MaxTotal}, skips: newSkipLog()}
	w := &walker{lim: lim}
	err = w.walk(context.Background(), f, a, func(e *entry) error {
		name, ok := cleanName(e.Name)
		if !ok {
			x.skips.add(skipUnsafe, e.Name)
			return nil
		}
		return x.put(name, e.Name, e)
	})
	return x, err
}

func TestGzipBombRefused(t *testing.T) {
	// 64 MB of zeros compress to about 64 KB: far more than 20 times.
	zeros := strings.Repeat("\x00", 64<<20)
	p := makeTarGz(t, t.TempDir(), "bomb.tar.gz", []te{file("zeros.bin", zeros)})
	lim := Limits{MaxFile: 1 << 40, MaxTotal: 1 << 40, MaxEntries: 1000, MaxRatio: 20, MinInflate: 1 << 20}
	_, err := extractAll(t, p, lim)
	if !errors.Is(err, ErrTooLarge) || !strings.Contains(err.Error(), "archive bomb") {
		t.Fatalf("err = %v", err)
	}
	// The analysis stops too.
	f, a := openArchive(t, p, false)
	if _, err := AnalyseSite(context.Background(), f, a, lim, &testLog{}); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("analysis: %v", err)
	}
}

func TestZipBombCounted(t *testing.T) {
	// Each entry is small compressed and 1 MB unpacked: the bytes really
	// written count, not what the archive says.
	var entries []te
	for i := 0; i < 8; i++ {
		entries = append(entries, file("f"+itoa(i)+".bin", strings.Repeat("A", 1<<20)))
	}
	z := makeZip(t, t.TempDir(), "bomb.zip", entries)
	lim := Limits{MaxFile: 2 << 20, MaxTotal: 3 << 20, MaxEntries: 1000, MaxRatio: 1000, MinInflate: 1 << 20}
	x, err := extractAll(t, z, lim)
	if !errors.Is(err, ErrTooLarge) || x.files > 3 {
		t.Fatalf("err = %v after %d files", err, x.files)
	}
}

func TestPerFileLimitAndEntryCount(t *testing.T) {
	dir := t.TempDir()
	p := makeTarGz(t, dir, "big.tar.gz", []te{file("big.bin", strings.Repeat("B", 4096)), file("small.txt", "ok")})
	lim := Limits{MaxFile: 1024, MaxTotal: 1 << 30, MaxEntries: 10, MaxRatio: 1000, MinInflate: 1 << 30}
	x, err := extractAll(t, p, lim)
	if err != nil || x.files != 1 || x.skips.counts[skipLarge] != 1 {
		t.Fatalf("err %v, files %d, skips %v", err, x.files, x.skips.counts)
	}
	var many []te
	for i := 0; i < 11; i++ {
		many = append(many, file("f"+itoa(i), "x"))
	}
	p = makeTarGz(t, dir, "many.tar.gz", many)
	if _, err := extractAll(t, p, lim); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("11 entries over a limit of 10: %v", err)
	}
}

// A link the website's user put in its folder is never written through.
func TestLinksInTheDestination(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need privileges on Windows")
	}
	outside := t.TempDir()
	dir := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "up")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "victim"), filepath.Join(dir, "victim")); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(outside, "victim"), []byte("safe"), 0o644)
	root, _ := os.OpenRoot(dir)
	defer root.Close()
	x := &extractor{root: root, uid: -1, gid: -1, lim: DefaultLimits, budget: &budget{left: 1 << 30}, skips: newSkipLog()}
	if err := x.put("up/x.txt", "up/x.txt", &entry{Kind: kindFile, Mode: 0o644, Size: 4, R: strings.NewReader("evil")}); err != nil {
		t.Fatal(err)
	}
	if err := x.put("victim", "victim", &entry{Kind: kindFile, Mode: 0o644, Size: 4, R: strings.NewReader("evil")}); err != nil {
		t.Fatal(err)
	}
	if exists(filepath.Join(outside, "x.txt")) || readFile(t, filepath.Join(outside, "victim")) != "safe" {
		t.Fatal("wrote through a link")
	}
	if fi, err := os.Lstat(filepath.Join(dir, "victim")); err != nil || !fi.Mode().IsRegular() {
		t.Fatalf("victim: %v %v", fi, err)
	}
	if x.skips.counts[skipClash] != 1 {
		t.Fatalf("skips %v", x.skips.counts)
	}
}

func TestModes(t *testing.T) {
	for in, want := range map[fs.FileMode]fs.FileMode{0o777: 0o755, 0o4755 | fs.ModeSetuid: 0o755, 0o666: 0o644, 0o400: 0o600, 0o2775 | fs.ModeSetgid: 0o755} {
		if got := fileMode(in); got != want {
			t.Errorf("fileMode(%v) = %v, want %v", in, got, want)
		}
	}
	if dirMode(0o777) != 0o755 || dirMode(0o500) != 0o700 || dirMode(0o1777|fs.ModeSticky) != 0o755 {
		t.Error("dirMode")
	}
}

func TestCleanName(t *testing.T) {
	for in, want := range map[string]string{"./a/b": "a/b", "a//b/": "a/b", "a/./b": "a/b", ".": "", "a\\b": "a/b"} {
		if got, ok := cleanName(in); !ok || got != want {
			t.Errorf("cleanName(%q) = %q %v", in, got, ok)
		}
	}
	for _, bad := range []string{"/etc/passwd", "../x", "a/../../x", "a/..", "\\etc\\x", "a\x00b"} {
		if _, ok := cleanName(bad); ok {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestOpenRefuses(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := Open("relative/path.tar.gz", false); err == nil {
		t.Error("relative path")
	}
	if _, _, err := Open(dir, false); err == nil {
		t.Error("a folder")
	}
	empty := filepath.Join(dir, "empty.tar.gz")
	os.WriteFile(empty, nil, 0o644)
	if _, _, err := Open(empty, false); err == nil {
		t.Error("an empty file")
	}
	text := filepath.Join(dir, "notes.txt")
	os.WriteFile(text, []byte("just text"), 0o644)
	if _, _, err := Open(text, false); err == nil {
		t.Error("a text file as an archive")
	}
	if f, a, err := Open(text, true); err != nil || a.Format != FormatSQL {
		t.Errorf("a text file as a dump: %v", err)
	} else {
		f.Close()
	}
	denied := makeTarGz(t, dir, "x.tar.gz", []te{file("a", "b")})
	if _, _, err := Open(denied, false, dir); err == nil {
		t.Error("a denied folder")
	}
	if runtime.GOOS == "linux" {
		for _, p := range []string{"/proc/self/environ", "/dev/zero", "/etc/passwd", "/sys/kernel/notes"} {
			if _, _, err := Open(p, false); err == nil {
				t.Errorf("%s opened", p)
			}
		}
		// A link to a denied place is refused too.
		l := filepath.Join(dir, "link.tar.gz")
		os.Symlink("/etc/shadow", l)
		if _, _, err := Open(l, false); err == nil {
			t.Error("a link to /etc")
		}
		fifo := filepath.Join(dir, "fifo.tar.gz")
		if err := mkfifo(fifo); err == nil {
			if _, _, err := Open(fifo, false); err == nil {
				t.Error("a FIFO")
			}
		}
	}
	var b bytes.Buffer
	writeTar(t, &b, []te{file("a", "b")})
	plain := filepath.Join(dir, "plain.tar")
	os.WriteFile(plain, b.Bytes(), 0o644)
	if f, a, err := Open(plain, false); err != nil || a.Format != FormatTar {
		t.Errorf("a tar: %v", err)
	} else {
		f.Close()
	}
}
