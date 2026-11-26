package siteimport

import (
	"archive/tar"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)

const wpConfig = `<?php
define( 'DB_NAME', 'bob_wp' );
define( 'DB_USER', 'bob_wpuser' );
define( 'DB_PASSWORD', 'Secret-Pass-2026' );
define( 'DB_HOST', 'localhost' );
$table_prefix = 'wp_';
require_once ABSPATH . 'wp-settings.php';
`

const cpHtaccess = `# BEGIN WordPress
RewriteEngine On
# END WordPress

# php -- BEGIN cPanel-generated handler, do not edit
# Set the "ea-php81" package as the default "PHP" programming language.
<IfModule mime_module>
  AddHandler application/x-httpd-ea-php81 .php .php8 .phtml
</IfModule>
# php -- END cPanel-generated handler, do not edit
`

// cpmove builds a cPanel full backup: example.com (WordPress, with
// parked.net), addon.com in its own folder, blog.example.com inside
// public_html, two databases, mail, DNS, a crontab, a certificate, and
// hostile entries.
func cpmove(t *testing.T, out string, extra ...te) string {
	t.Helper()
	cert, key := selfSigned(t, []string{"example.com", "www.example.com", "parked.net", "www.parked.net"}, testNow.AddDate(0, -1, 0))
	expired, expiredKey := selfSigned(t, []string{"addon.com", "www.addon.com"}, testNow.AddDate(-2, 0, 0))
	p := "cpmove-bob/"
	hash := NativeHash("Secret-Pass-2026")
	entries := []te{
		dir(p[:len(p)-1]),
		file(p+"version", "pkgacct version: 10\narchive version: 4\n"),
		dir(p + "homedir"),
		dir(p + "homedir/public_html"),
		file(p+"homedir/public_html/index.php", "<?php echo 'MAIN SITE';\n"),
		file(p+"homedir/public_html/wp-config.php", wpConfig),
		file(p+"homedir/public_html/.htaccess", cpHtaccess),
		file(p+"homedir/public_html/.user.ini", "auto_prepend_file = '/home/bob/public_html/wordfence-waf.php'\n"),
		file(p+"homedir/public_html/wp-content/uploads/a.txt", "upload"),
		file(p+"homedir/public_html/blog/index.html", "BLOG"),
		file(p+"homedir/addon.com/index.html", "ADDON"),
		file(p+"homedir/mail/example.com/info/cur/1", "From: x\n\nmail"),
		file(p+"homedir/etc/example.com/passwd", "info:x:1:1::/home/bob/mail/example.com/info:/bin/false"),
		file(p+"homedir/.bashrc", "# bash"),
		file(p+"homedir/ssl/keys/abc.key", key),
		file(p+"homedir/ssl/certs/www_example_com.crt", cert),
		// Hostile entries.
		file(p+"homedir/public_html/../../../../../../tmp/escaped.txt", "evil"),
		file("/tmp/absolute.txt", "evil"),
		link(p+"homedir/public_html/passwd", "/etc/passwd"),
		link(p+"homedir/public_html/up", "../../../.."),
		file(p+"homedir/public_html/up/escaped-through-link.txt", "evil"),
		{name: p + "homedir/public_html/hard", typ: tar.TypeLink, link: "/etc/shadow"},
		{name: p + "homedir/public_html/dev", typ: tar.TypeChar},
		{name: p + "homedir/public_html/fifo", typ: tar.TypeFifo},
		modeFile(p+"homedir/public_html/setuid.php", 0o6777),
		// Metadata.
		file(p+"userdata/main", mainYAML),
		file(p+"userdata/example.com", vhostYAML),
		file(p+"userdata/example.com_SSL", vhostYAML),
		file(p+"userdata/cache", "junk"),
		file(p+"userdata/addon.example.com", "documentroot: /home/bob/addon.com\nservername: addon.example.com\nserveralias: addon.com www.addon.com www.addon.example.com\nphpversion: ea-php74\nuser: bob\n"),
		file(p+"userdata/blog.example.com", "documentroot: /home/bob/public_html/blog\nservername: blog.example.com\nserveralias: www.blog.example.com\nuser: bob\n"),
		file(p+"mysql/bob_wp.create", "CREATE DATABASE `bob_wp`;\n"),
		file(p+"mysql/bob_wp.sql", "CREATE TABLE `wp_options` (`a` text) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;\n"+
			"INSERT INTO `wp_options` VALUES ('/home/bob/public_html');\n"),
		file(p+"mysql/bob_shop.sql", "CREATE TABLE `orders` (`id` int);\n"),
		file(p+"mysql.sql", "GRANT USAGE ON *.* TO 'bob'@'localhost' IDENTIFIED BY PASSWORD '*AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA';\n"+
			"GRANT ALL PRIVILEGES ON `bob\\_%`.* TO 'bob'@'localhost';\n"+
			"GRANT USAGE ON *.* TO 'bob_wpuser'@'localhost' IDENTIFIED BY PASSWORD '"+hash+"';\n"+
			"GRANT ALL PRIVILEGES ON `bob\\_wp`.* TO 'bob_wpuser'@'localhost';\n"+
			"GRANT USAGE ON *.* TO 'bob_shopuser'@'localhost' IDENTIFIED BY PASSWORD '*BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB';\n"+
			"GRANT ALL PRIVILEGES ON `bob\\_shop`.* TO 'bob_shopuser'@'localhost';\n"+
			// A MySQL 8 user (its hash can't be kept) sorted before
			// bob_shopuser: the user whose password carries over still wins.
			"CREATE USER 'bob_m8'@'localhost' IDENTIFIED WITH 'caching_sha2_password' AS '$A$005$abcdef';\n"+
			"GRANT SELECT ON `bob\\_shop`.* TO `bob_m8`@`localhost`;\n"),
		file(p+"cron/bob", "MAILTO=\"\"\n*/5 * * * * /usr/local/bin/php /home/bob/public_html/wp-cron.php\n0 3 * * * /home/bob/scripts/backup.sh\n"),
		file(p+"apache_tls/addon.com", expiredKey+expired),
		file(p+"dnszones/example.com.db", "$TTL 14400\n"),
		file(p+"dnszones/addon.com.db", "$TTL 14400\n"),
		file(p+"proftpdpasswd", "bob:x:1000:1000::/home/bob:/bin/ftpsh\n"),
		file(p+"cp/bob", "DNS=example.com\n"),
	}
	return makeTarGz(t, out, "cpmove-bob.tar.gz", append(entries, extra...))
}

func testEnv(f *fakeHost) Env {
	return Env{PHP: []string{"8.1", "8.3"}, DefaultPHP: "8.3", DBReady: true, Now: testNow,
		DomainTaken: func(n string) bool { return n == "www.blog.example.com" },
		DirExists:   func(string) bool { return false },
		DBNameFree:  func(n string) bool { return n != "bob_shop" },
		Root:        f.root, Free: 1 << 40, FreeDB: 1 << 40, SameDisk: true}
}

func analyse(t *testing.T, path string) (*os.File, *CPanel) {
	t.Helper()
	f, a := openArchive(t, path, false)
	c, err := AnalyseCPanel(context.Background(), f, a, DefaultLimits, testNow, &testLog{})
	if err != nil {
		t.Fatal(err)
	}
	return f, c
}

func TestCPanelPlan(t *testing.T) {
	fh := newFakeHost(t)
	_, c := analyse(t, cpmove(t, t.TempDir()))
	p := c.Plan(testEnv(fh))
	if p.User != "bob" || len(p.Domains) != 3 {
		t.Fatalf("plan: user %q, %d domains", p.User, len(p.Domains))
	}
	main, addon, blog := p.Domains[0], p.Domains[1], p.Domains[2]
	if main.Name != "example.com" || main.Type != "main" || main.App != "wordpress" || main.DB != "bob_wp" ||
		main.PHPOld != "8.1" || main.PHP != "8.1" || main.Blocked != "" {
		t.Errorf("main: %+v", main)
	}
	if !slices.Equal(main.Aliases, []string{"www.example.com", "parked.net", "www.parked.net"}) {
		t.Errorf("main aliases: %q", main.Aliases)
	}
	if !slices.Equal(main.Nested, []string{"blog.example.com"}) {
		t.Errorf("nested: %q", main.Nested)
	}
	if main.Cert == nil || !main.Cert.Usable || main.Cert.Trusted {
		t.Errorf("main certificate: %+v", main.Cert)
	}
	// addon.com: cPanel's internal addon.example.com is not an alias, PHP
	// 7.4 is not installed, and its certificate expired.
	if addon.Name != "addon.com" || addon.Type != "addon" || !slices.Equal(addon.Aliases, []string{"www.addon.com"}) ||
		addon.PHPOld != "7.4" || addon.PHP != "8.3" || addon.App != "static" || addon.Size != int64(len("ADDON")) {
		t.Errorf("addon: %+v", addon)
	}
	if addon.Cert == nil || addon.Cert.Usable || addon.Cert.Problem != "expired" {
		t.Errorf("addon certificate: %+v", addon.Cert)
	}
	if blog.Name != "blog.example.com" || blog.Type != "sub" || !slices.Equal(blog.Taken, []string{"www.blog.example.com"}) {
		t.Errorf("blog: %+v", blog)
	}
	if len(p.Databases) != 2 {
		t.Fatalf("databases: %+v", p.Databases)
	}
	shop, wp := p.Databases[0], p.Databases[1]
	if shop.Name != "bob_shop" || shop.NewName != "bob_shop_2" || shop.User != "bob_shopuser" || shop.Password != "kept" || !shop.ConfigChange {
		t.Errorf("bob_shop: %+v", shop)
	}
	if wp.Name != "bob_wp" || wp.NewName != "bob_wp" || wp.User != "bob_wpuser" || wp.Password != "known" || wp.ConfigChange ||
		!slices.Equal(wp.UsedBy, []string{"example.com"}) || !slices.Equal(wp.Users, []string{"bob", "bob_wpuser"}) {
		t.Errorf("bob_wp: %+v", wp)
	}
	if len(p.Cron) != 2 || p.Cron[0].NewCommand != "/usr/bin/php8.1 "+filepathSlash(fh.root("example.com"))+"/public/wp-cron.php" {
		t.Errorf("cron: %+v", p.Cron[0])
	}
	whats := map[string]Skipped{}
	for _, s := range p.Skipped {
		whats[s.What] = s
	}
	if whats["mail"].Count != 2 || whats["dns"].Count != 2 || whats["other_files"].Size == 0 {
		t.Errorf("skipped: %+v", p.Skipped)
	}
	if _, ok := whats["ftp"]; !ok {
		t.Errorf("FTP accounts not reported: %+v", p.Skipped)
	}
}

func filepathSlash(p string) string { return filepath.ToSlash(p) }

func allOf(p *Plan) Selection {
	sel := Selection{Certificates: []string{"example.com", "addon.com"}, Cron: []int{0, 1}}
	for _, d := range p.Domains {
		sel.Domains = append(sel.Domains, d.Name)
	}
	for _, d := range p.Databases {
		sel.Databases = append(sel.Databases, d.Name)
	}
	return sel
}

func TestCPanelImport(t *testing.T) {
	fh := newFakeHost(t)
	f, c := analyse(t, cpmove(t, t.TempDir()))
	plan := c.Plan(testEnv(fh))
	lg := &testLog{}
	res, err := RunCPanel(context.Background(), f, c, plan, allOf(plan), fh.host(), DefaultLimits, lg)
	if err != nil {
		t.Fatalf("%v\n%s", err, lg)
	}
	if testing.Verbose() {
		t.Log(lg)
	}
	pub := func(d, p string) string { return filepath.Join(fh.root(d), "public", filepath.FromSlash(p)) }
	if readFile(t, pub("example.com", "index.php")) != "<?php echo 'MAIN SITE';\n" || readFile(t, pub("addon.com", "index.html")) != "ADDON" ||
		readFile(t, pub("blog.example.com", "index.html")) != "BLOG" || readFile(t, pub("example.com", "wp-content/uploads/a.txt")) != "upload" {
		t.Fatal("files missing")
	}
	// blog.example.com's folder went to its own website only.
	if exists(pub("example.com", "blog")) {
		t.Error("the subdomain's folder was copied into the main website too")
	}
	// The placeholder page is gone; mail and other home files stayed behind.
	for _, p := range []string{pub("example.com", "index.html"), filepath.Join(fh.root("example.com"), "mail"), pub("example.com", "../.bashrc")} {
		if exists(p) {
			t.Errorf("%s exists", p)
		}
	}
	// Nothing hostile got through.
	for _, n := range []string{"passwd", "hard", "dev", "fifo"} {
		if exists(pub("example.com", n)) {
			t.Errorf("%s was unpacked", n)
		}
	}
	for _, p := range []string{filepath.Join(fh.base, "tmp"), "/tmp/escaped.txt", "/tmp/absolute.txt", filepath.Join(fh.base, "escaped-through-link.txt")} {
		if b, err := os.ReadFile(p); err == nil && string(b) == "evil" {
			t.Errorf("%s was written", p)
		}
	}
	// The link "up" was left out, so the file behind it went into an
	// ordinary folder of the website.
	if fi, err := os.Lstat(pub("example.com", "up")); err != nil || !fi.IsDir() {
		t.Errorf("up: %v %v", fi, err)
	}
	if runtime.GOOS != "windows" {
		fi, err := os.Stat(pub("example.com", "setuid.php"))
		if err != nil || fi.Mode()&(os.ModeSetuid|os.ModeSetgid) != 0 || fi.Mode().Perm() != 0o755 {
			t.Errorf("setuid.php: %v %v", fi.Mode(), err)
		}
	}
	log := lg.String()
	for _, want := range []string{"Left out 2 symbolic links", "Left out 1 hard link,", "Left out 2 devices, FIFOs", "entries with an absolute name or .."} {
		if !strings.Contains(log, want) {
			t.Errorf("log lacks %q:\n%s", want, log)
		}
	}
	// .htaccess lost cPanel's handler; .user.ini points at the new folder.
	ht := readFile(t, pub("example.com", ".htaccess"))
	if strings.Contains(ht, "AddHandler") || !strings.Contains(ht, "# BEGIN WordPress") {
		t.Errorf(".htaccess:\n%s", ht)
	}
	if ini := readFile(t, pub("example.com", ".user.ini")); !strings.Contains(ini, filepathSlash(fh.root("example.com"))+"/public/wordfence-waf.php") {
		t.Errorf(".user.ini: %s", ini)
	}
	// wp-config.php stays: the database kept its name, user and password.
	if readFile(t, pub("example.com", "wp-config.php")) != wpConfig {
		t.Error("wp-config.php changed")
	}
	// Databases: bob_wp with its known password, bob_shop renamed with the
	// old hash; the dump was adapted for MariaDB.
	if len(fh.newDBs) != 2 {
		t.Fatalf("databases: %+v", fh.newDBs)
	}
	for _, n := range fh.newDBs {
		switch n.Name {
		case "bob_wp":
			if n.User != "bob_wpuser" || n.Password != "Secret-Pass-2026" || n.Hash != "" || n.SiteID != fh.sites["example.com"].ID {
				t.Errorf("bob_wp: %+v", n)
			}
		case "bob_shop_2":
			if n.User != "bob_shopuser" || n.Password != "" || n.Hash != "*BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB" {
				t.Errorf("bob_shop_2: %+v", n)
			}
		default:
			t.Errorf("unexpected database %+v", n)
		}
	}
	if d := fh.dumps["bob_wp"]; !strings.Contains(d, "COLLATE=utf8mb4_unicode_520_ci") || strings.Contains(d, "0900") {
		t.Errorf("bob_wp dump: %s", d)
	}
	if !strings.Contains(log, "1 MySQL 8 collation name replaced") {
		t.Errorf("no adaptation in the log:\n%s", log)
	}
	newPub := filepathSlash(fh.root("example.com")) + "/public"
	for _, want := range []string{"create site example.com php=8.1 aliases=www.example.com,parked.net,www.parked.net",
		"create site addon.com php=8.3", "create site blog.example.com php=",
		"wordpress example.com", "replace example.com /home/bob/public_html -> " + newPub,
		"cert example.com", "cron example_com */5 * * * * /usr/bin/php8.1 " + newPub + "/wp-cron.php",
		"suspend example.com false", "suspend addon.com false"} {
		if !fh.has(want) {
			t.Errorf("missing step %q in %q", want, fh.steps)
		}
	}
	// The expired certificate was not installed, though ticked.
	if fh.has("cert addon.com") {
		t.Error("the expired certificate was installed")
	}
	if fh.notes["example.com"] != "Imported from cpmove-bob.tar.gz on 2026-11-26 10:00" {
		t.Errorf("note: %q", fh.notes["example.com"])
	}
	if len(res.Sites) != 3 || res.Sites[0].App != "wordpress" {
		t.Errorf("result sites: %+v", res.Sites)
	}
	for _, d := range res.Databases {
		if d.From == "bob_shop" && (!d.Manual || d.Name != "bob_shop_2") {
			t.Errorf("bob_shop needs a hand: %+v", d)
		}
		if d.From == "bob_wp" && d.Manual {
			t.Errorf("bob_wp: %+v", d)
		}
	}
}

func TestCPanelImportRewritesWPConfigForANewName(t *testing.T) {
	fh := newFakeHost(t)
	f, c := analyse(t, cpmove(t, t.TempDir()))
	env := testEnv(fh)
	env.DBNameFree = func(n string) bool { return n != "bob_wp" && n != "bob_wpuser" }
	plan := c.Plan(env)
	wp := plan.database("bob_wp", false)
	if wp.NewName != "bob_wp_2" || wp.User != "bob_wp_2" || wp.Password != "new" || !wp.ConfigChange {
		t.Fatalf("plan: %+v", wp)
	}
	sel := Selection{Domains: []string{"example.com"}, Databases: []string{"bob_wp"}}
	if _, err := RunCPanel(context.Background(), f, c, plan, sel, fh.host(), DefaultLimits, &testLog{}); err != nil {
		t.Fatal(err)
	}
	cfg := readFile(t, filepath.Join(fh.root("example.com"), "public", "wp-config.php"))
	for _, want := range []string{"define( 'DB_NAME', 'bob_wp_2' );", "define( 'DB_USER', 'bob_wp_2' );", "define( 'DB_PASSWORD', 'Generated-Pass-2026' );"} {
		if !strings.Contains(cfg, want) {
			t.Errorf("missing %q in:\n%s", want, cfg)
		}
	}
	if fh.sites["addon.com"] != nil {
		t.Error("a website that was not chosen was created")
	}
}

func TestCPanelImportRollsBack(t *testing.T) {
	fh := newFakeHost(t)
	f, c := analyse(t, cpmove(t, t.TempDir()))
	plan := c.Plan(testEnv(fh))
	fh.failDB = "bob_wp"
	lg := &testLog{}
	res, err := RunCPanel(context.Background(), f, c, plan, allOf(plan), fh.host(), DefaultLimits, lg)
	if err == nil || res != nil {
		t.Fatalf("no error: %+v", res)
	}
	if len(fh.sites) != 0 || len(fh.dbs) != 0 {
		t.Fatalf("left behind: sites %v, databases %v", fh.sites, fh.dbs)
	}
	for _, d := range []string{"example.com", "addon.com", "blog.example.com"} {
		if exists(fh.root(d)) {
			t.Errorf("%s's folder is still there", d)
		}
	}
	if !strings.Contains(lg.String(), "Removing what the import created") {
		t.Errorf("log:\n%s", lg)
	}
}

func TestCPanelImportStopped(t *testing.T) {
	fh := newFakeHost(t)
	f, c := analyse(t, cpmove(t, t.TempDir()))
	plan := c.Plan(testEnv(fh))
	fh.blockDB = "bob_shop_2"
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for !fh.has("create db bob_wp") {
			time.Sleep(5 * time.Millisecond)
		}
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	_, err := RunCPanel(ctx, f, c, plan, allOf(plan), fh.host(), DefaultLimits, &testLog{})
	if err == nil || err.Error() != "the import was stopped" {
		t.Fatalf("err = %v", err)
	}
	if len(fh.sites) != 0 || len(fh.dbs) != 0 {
		t.Fatalf("left behind: sites %v, databases %v", fh.sites, fh.dbs)
	}
}

func TestCPanelSelectionChecks(t *testing.T) {
	fh := newFakeHost(t)
	_, c := analyse(t, cpmove(t, t.TempDir()))
	env := testEnv(fh)
	env.DomainTaken = func(n string) bool { return n == "addon.com" }
	p := c.Plan(env)
	if p.domain("addon.com").Blocked != "domain_exists" {
		t.Fatalf("addon.com: %+v", p.domain("addon.com"))
	}
	for _, sel := range []Selection{{}, {Domains: []string{"addon.com"}}, {Domains: []string{"nope.com"}}, {Databases: []string{"x"}}} {
		if err := p.Check(sel); err == nil {
			t.Errorf("%+v accepted", sel)
		}
	}
	p.DBReady = false
	if err := p.Check(Selection{Databases: []string{"bob_wp"}}); err == nil {
		t.Error("databases without MariaDB accepted")
	}
}

func TestNotACPanelBackup(t *testing.T) {
	dir := t.TempDir()
	p := makeTarGz(t, dir, "site.tar.gz", []te{file("public_html/index.html", "x")})
	f, a := openArchive(t, p, false)
	if _, err := AnalyseCPanel(context.Background(), f, a, DefaultLimits, testNow, &testLog{}); err == nil {
		t.Fatal("a plain archive passed as a cPanel backup")
	}
}

// An old backup keeps the home folder in a homedir.tar of its own.
func TestCPanelHomedirTar(t *testing.T) {
	var home bytes.Buffer
	writeTar(t, &home, []te{file("public_html/index.php", "<?php echo 'OLD';"), file("../escape.txt", "evil")})
	dir := t.TempDir()
	p := makeTarGz(t, dir, "backup-1.2.2014_10-00-00_bob.tar.gz", []te{
		file("backup-1.2.2014_10-00-00_bob/homedir.tar", home.String()),
		file("backup-1.2.2014_10-00-00_bob/userdata/main", "main_domain: example.com\n"),
		file("backup-1.2.2014_10-00-00_bob/userdata/example.com", "documentroot: /home/bob/public_html\nservername: example.com\nhomedir: /home/bob\nuser: bob\n"),
	})
	fh := newFakeHost(t)
	f, c := analyse(t, p)
	plan := c.Plan(testEnv(fh))
	if len(plan.Domains) != 1 || plan.Domains[0].App != "php" || plan.Notes[0].Code != "old_format" {
		t.Fatalf("plan: %+v %+v", plan.Domains, plan.Notes)
	}
	if _, err := RunCPanel(context.Background(), f, c, plan, Selection{Domains: []string{"example.com"}}, fh.host(), DefaultLimits, &testLog{}); err != nil {
		t.Fatal(err)
	}
	if readFile(t, filepath.Join(fh.root("example.com"), "public", "index.php")) != "<?php echo 'OLD';" {
		t.Error("file from homedir.tar missing")
	}
	if exists(filepath.Join(fh.base, "www", "example.com", "escape.txt")) {
		t.Error("escaped from homedir.tar")
	}
}

func TestCPanelChangedArchive(t *testing.T) {
	dir := t.TempDir()
	p := cpmove(t, dir)
	f, a := openArchive(t, p, false)
	f.Close()
	makeTarGz(t, dir, "cpmove-bob.tar.gz", []te{file("cpmove-bob/userdata/main", "main_domain: other.com\n")})
	os.Chtimes(p, time.Now().Add(time.Hour), time.Now().Add(time.Hour))
	g, b := openArchive(t, p, false)
	defer g.Close()
	if a.Same(b) || !a.Same(a) {
		t.Fatal("a changed archive looks the same")
	}
}

// The panel gives no clock (Host.Now nil): the run uses time.Now, also
// when it installs a certificate (a nil clock once panicked there).
func TestCPanelImportDefaultClock(t *testing.T) {
	fh := newFakeHost(t)
	f, c := analyse(t, cpmove(t, t.TempDir()))
	plan := c.Plan(testEnv(fh))
	sel := allOf(plan)
	if len(sel.Certificates) == 0 {
		t.Fatal("the fixture offers no certificate")
	}
	h := fh.host()
	h.Now = nil
	lg := &testLog{}
	if _, err := RunCPanel(context.Background(), f, c, plan, sel, h, DefaultLimits, lg); err != nil {
		t.Fatalf("%v\n%s", err, lg)
	}
	if !slices.ContainsFunc(fh.steps, func(s string) bool { return strings.HasPrefix(s, "cert ") }) {
		t.Errorf("no certificate installed: %v", fh.steps)
	}
}
