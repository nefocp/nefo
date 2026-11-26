package hosting

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pmaArchive builds a phpMyAdmin-like release archive.
func pmaArchive(t *testing.T, top string, entries map[string]string, extra ...*tar.Header) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: top + "/", Typeflag: tar.TypeDir, Mode: 0o755})
	for name, body := range entries {
		if strings.HasSuffix(name, "/") {
			tw.WriteHeader(&tar.Header{Name: top + "/" + name, Typeflag: tar.TypeDir, Mode: 0o755})
			continue
		}
		tw.WriteHeader(&tar.Header{Name: top + "/" + name, Typeflag: tar.TypeReg, Mode: 0o664, Size: int64(len(body))})
		tw.Write([]byte(body))
	}
	for _, h := range extra {
		tw.WriteHeader(h)
	}
	tw.Close()
	gz.Close()
	sum := sha256.Sum256(buf.Bytes())
	return buf.Bytes(), hex.EncodeToString(sum[:])
}

const pmaTop = "phpMyAdmin-5.2.3-all-languages"

func TestUnpackPhpMyAdmin(t *testing.T) {
	archive, sum := pmaArchive(t, pmaTop, map[string]string{
		"index.php": "<?php // pma", "url.php": "<?php", "js/": "", "js/messages.php": "<?php",
		"js/dist/app.js": "x", "themes/pmahomme/css/theme.css": "body{}",
		"setup/index.php": "<?php // setup", "setup/config.php": "<?php",
		"examples/signon.php": "<?php", "doc/html/index.html": "<html>",
		"config.sample.inc.php": "<?php", "show_config_errors.php": "<?php", "package.json": "{}",
	})
	dir := filepath.Join(t.TempDir(), "pma")

	// A download that is not the pinned release is refused before anything
	// is written.
	if err := unpackPhpMyAdmin(archive, phpMyAdminSHA256, dir); err == nil || !strings.Contains(err.Error(), "pinned checksum") {
		t.Fatalf("checksum mismatch: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("folder created for a refused download")
	}
	if err := unpackPhpMyAdmin(append(append([]byte{}, archive...), 0), sum, dir); err == nil {
		t.Fatal("changed archive accepted")
	}

	if err := unpackPhpMyAdmin(archive, sum, dir); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"index.php", "url.php", "js/messages.php", "js/dist/app.js", "themes/pmahomme/css/theme.css"} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("%s missing", want)
		}
	}
	// No setup script, examples, local docs or sample configuration.
	for _, gone := range []string{"setup", "examples", "doc", "config.sample.inc.php", "show_config_errors.php", "package.json"} {
		if _, err := os.Stat(filepath.Join(dir, gone)); err == nil {
			t.Errorf("%s unpacked", gone)
		}
	}
	if fi, _ := os.Stat(filepath.Join(dir, "index.php")); os.PathSeparator == '/' && fi.Mode().Perm() != 0o644 {
		t.Errorf("mode %v", fi.Mode())
	}
	// The folder must be new.
	if err := unpackPhpMyAdmin(archive, sum, dir); err == nil {
		t.Error("unpacked over an existing folder")
	}
}

func TestUnpackPhpMyAdminRefusesOddEntries(t *testing.T) {
	for name, extra := range map[string]*tar.Header{
		"traversal": {Name: pmaTop + "/../evil.php", Typeflag: tar.TypeReg, Size: 0},
		"absolute":  {Name: "/etc/passwd", Typeflag: tar.TypeReg, Size: 0},
		"other top": {Name: "other/index.php", Typeflag: tar.TypeReg, Size: 0},
		"symlink":   {Name: pmaTop + "/link", Typeflag: tar.TypeSymlink, Linkname: "/etc"},
		"hard link": {Name: pmaTop + "/hard", Typeflag: tar.TypeLink, Linkname: pmaTop + "/index.php"},
		"device":    {Name: pmaTop + "/dev", Typeflag: tar.TypeChar},
	} {
		archive, sum := pmaArchive(t, pmaTop, map[string]string{"index.php": "<?php"}, extra)
		if err := unpackPhpMyAdmin(archive, sum, filepath.Join(t.TempDir(), "pma")); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// An archive without phpMyAdmin's entry point is not phpMyAdmin.
	archive, sum := pmaArchive(t, pmaTop, map[string]string{"README": "x"})
	if err := unpackPhpMyAdmin(archive, sum, filepath.Join(t.TempDir(), "pma")); err == nil {
		t.Error("archive without index.php accepted")
	}
}

func TestPhpMyAdminConfig(t *testing.T) {
	secret := strings.Repeat("aB3", 10) + "xy"
	b, err := phpMyAdminConfig(secret, "/var/lib/nefo-phpmyadmin/tmp")
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, string(b),
		"<?php\n",
		"$cfg['blowfish_secret'] = '"+secret+"';",
		"$cfg['Servers'][$i]['auth_type'] = 'cookie';",
		"$cfg['Servers'][$i]['host'] = 'localhost';",
		"$cfg['Servers'][$i]['AllowNoPassword'] = false;",
		"$cfg['Servers'][$i]['AllowRoot'] = false;",
		"$cfg['AllowArbitraryServer'] = false;",
		"$cfg['TempDir'] = '/var/lib/nefo-phpmyadmin/tmp';",
		"$cfg['VersionCheck'] = false;",
		"$cfg['SendErrorReports'] = 'never';",
		"$cfg['ShowChgPassword'] = false;",
		"preg_match('~^/[a-z0-9]{3,32}/phpmyadmin/~'",
		"$cfg['PmaAbsoluteUri'] = $nefoScheme . '://' . $nefoHost . $nefoBase;",
	)
	for _, bad := range []string{"", "short", strings.Repeat("a", 31) + "'", strings.Repeat("a", 33)} {
		if _, err := phpMyAdminConfig(bad, "/tmp/x"); err == nil {
			t.Errorf("secret %q accepted", bad)
		}
	}
	if _, err := phpMyAdminConfig(secret, "/tmp/x';phpinfo();'"); err == nil {
		t.Error("temporary folder with a quote accepted")
	}
}

func TestPhpMyAdminServerConfs(t *testing.T) {
	m := newTestManager(t)
	const token = "0123456789abcdef0123456789abcdef"
	n := nginxServer{m}.phpMyAdminConf("46001", token, "8.3")
	mustContain(t, string(n), "listen 127.0.0.1:46001;", `if ($http_x_panel_phpmyadmin != "`+token+`")`,
		"root /usr/share/nefo-phpmyadmin;", "fastcgi_param SCRIPT_FILENAME /usr/share/nefo-phpmyadmin/$pma_script;",
		`fastcgi_param HTTP_X_PANEL_PHPMYADMIN "" if_not_empty;`, "fastcgi_pass unix:/run/php/php8.3-fpm.sock;",
		"(?<pma_script>index\\.php|url\\.php|js/messages\\.php)$", "location / {\n        return 404;")
	a := apacheServer{m}.phpMyAdminConf("46001", token, "8.3")
	mustContain(t, string(a), "Listen 127.0.0.1:46001", "<VirtualHost 127.0.0.1:46001>",
		"RewriteCond %{HTTP:X-Panel-Phpmyadmin} !="+token, "RequestHeader unset X-Panel-Phpmyadmin",
		"RewriteRule \\.\\. - [R=404,L]", `SetHandler "proxy:unix:/run/php/php8.3-fpm.sock|fcgi://localhost"`)
	o := olsServer{m}.phpMyAdminConf("46001", token, "8.3")
	mustContain(t, string(o), "address                 127.0.0.1:46001", "map                     _phpmyadmin *",
		"RewriteCond %{HTTP:X-Panel-Phpmyadmin} !="+token, "uds://run/php/php8.3-fpm.sock",
		"docRoot                 /usr/share/nefo-phpmyadmin/")
	for name, conf := range map[string][]byte{"nginx": n, "apache": a, "ols": o} {
		s := string(conf)
		// Adminer's files are never touched, and no setup script is reachable.
		if strings.Contains(s, "panel-adminer") || strings.Contains(s, "setup") {
			t.Errorf("%s:\n%s", name, s)
		}
		// The last rule refuses everything else.
		if !strings.Contains(s, "404") {
			t.Errorf("%s: no 404", name)
		}
	}
	// OpenLiteSpeed's listener check finds phpMyAdmin's address.
	if mm := olsAdminerAddrRe.FindSubmatch(o); mm == nil || string(mm[1]) != "127.0.0.1:46001" {
		t.Errorf("listener address: %q", mm)
	}
	for _, bad := range [][3]string{{"46001", "x\nListen 80", "8.3"}, {"80 443", "abcdefabcdefabcd", "8.3"}, {"46001", "abcdefabcdefabcd", "8.3;"}} {
		ng, ap, ol := nginxServer{m}, apacheServer{m}, olsServer{m}
		if ng.phpMyAdminConf(bad[0], bad[1], bad[2]) != nil || ap.phpMyAdminConf(bad[0], bad[1], bad[2]) != nil ||
			ol.phpMyAdminConf(bad[0], bad[1], bad[2]) != nil {
			t.Errorf("rendered %q", bad)
		}
	}
}

func TestPhpMyAdminNotInstalled(t *testing.T) {
	m := newTestManager(t)
	if addr, token := m.PhpMyAdminTarget(t.Context()); addr != "" || token != "" {
		t.Fatalf("target %q %q", addr, token)
	}
}
