package hosting

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/secure"
	"github.com/nefocp/nefo/internal/validate"
	"github.com/nefocp/nefo/internal/version"
)

// phpMyAdmin is the second database web UI, next to Adminer and built the
// same way: the pinned official release, run by the distribution's PHP
// behind an internal server of the web server that only listens on
// 127.0.0.1 and answers only requests carrying a secret token, which the
// panel adds when it forwards /<entrance>/phpmyadmin/ after its own
// sign-in. It works with MariaDB and MySQL alike (cookie login, as the
// database's own user, on this server only). Adminer's files are separate.
const (
	PhpMyAdminVersion = "5.2.3"
	// phpMyAdminURL is the release of 2025-10-08 listed on
	// https://www.phpmyadmin.net/downloads/ ("all languages": the panel
	// has Vietnamese users). phpMyAdminSHA256 is the checksum published
	// next to it (phpMyAdmin-5.2.3-all-languages.tar.gz.sha256); the
	// archive's signature (.asc) is by the release manager Isaac Bennetch,
	// key 3D06A59ECE730EB71B511C17CE752F178259BD92.
	phpMyAdminURL    = "https://files.phpmyadmin.net/phpMyAdmin/5.2.3/phpMyAdmin-5.2.3-all-languages.tar.gz"
	phpMyAdminSHA256 = "12ba1c425fa4071abbd4e7668c9ebdeac0b0755a467a6d6d5026122bb47c102b"
	phpMyAdminSize   = 32 << 20 // the archive is about 14 MB
	phpMyAdminDir    = "/usr/share/nefo-phpmyadmin"
	// phpMyAdminVar holds its temporary files (TempDir: the template
	// cache, uploads being imported), writable by PHP only.
	phpMyAdminVar      = "/var/lib/nefo-phpmyadmin"
	phpMyAdminPortKey  = "phpmyadmin_port"
	phpMyAdminTokenKey = "phpmyadmin_token"
	// phpMyAdminSecretKey is the setting with the cookie encryption key
	// (blowfish_secret), so config.inc.php can be written again.
	phpMyAdminSecretKey = "phpmyadmin_secret"
	// PhpMyAdminTokenHeader is the request header the internal server checks.
	PhpMyAdminTokenHeader = "X-Panel-Phpmyadmin"
)

func (m *Manager) phpMyAdminConf() string {
	return filepath.Join(m.web().sitesDir(), "_phpmyadmin.conf")
}

// PhpMyAdminInstalled reports whether phpMyAdmin is installed.
func (m *Manager) PhpMyAdminInstalled() bool {
	return fileExists(filepath.Join(phpMyAdminDir, "index.php")) && fileExists(m.phpMyAdminConf())
}

// PhpMyAdminTarget returns the internal address phpMyAdmin listens on and
// the token to send with each request, or empty strings if it is not
// installed.
func (m *Manager) PhpMyAdminTarget(ctx context.Context) (addr, token string) {
	if !m.PhpMyAdminInstalled() {
		return "", ""
	}
	p, _ := m.store.Setting(ctx, phpMyAdminPortKey)
	t, _ := m.store.Setting(ctx, phpMyAdminTokenKey)
	if p == "" || t == "" {
		return "", ""
	}
	return "127.0.0.1:" + p, t
}

// phpMyAdminConfBytes renders phpMyAdmin's internal server, or nil before
// it is set up. It runs on the PHP version Adminer would use.
func (m *Manager) phpMyAdminConfBytes(ctx context.Context) ([]byte, error) {
	port, _ := m.store.Setting(ctx, phpMyAdminPortKey)
	token, _ := m.store.Setting(ctx, phpMyAdminTokenKey)
	if port == "" || token == "" {
		return nil, nil
	}
	php := m.adminerPHP(ctx)
	if php == "" {
		return nil, errors.New("PHP is not installed")
	}
	conf := m.web().phpMyAdminConf(port, token, php)
	if conf == nil {
		return nil, fmt.Errorf("refusing to render phpMyAdmin's server for port %q", port)
	}
	return conf, nil
}

// phpMyAdminSecretChars make up the cookie key (blowfish_secret): 32 of
// them, as phpMyAdmin 5.2 wants 32 bytes.
var phpMyAdminSecretChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

// phpMyAdminConfig is config.inc.php. Logins are the databases' own users
// (cookie auth) on this server's socket only; root, other servers, the
// setup script, version checks and error reports are off, and phpMyAdmin
// keeps no tables of its own in the users' databases. Its links, redirects
// and cookie paths follow the path the panel forwards from
// (/<entrance>/phpmyadmin/), and the scheme the panel saw.
func phpMyAdminConfig(secret, tmpDir string) ([]byte, error) {
	if len(secret) != 32 || strings.Trim(secret, phpMyAdminSecretChars) != "" {
		return nil, errors.New("invalid phpMyAdmin secret")
	}
	if !path.IsAbs(tmpDir) || strings.ContainsAny(tmpDir, `'\`+"\n") {
		return nil, errors.New("invalid phpMyAdmin temporary folder")
	}
	return []byte(`<?php
// Managed by ` + version.AppName + `. Do not edit: the panel rewrites this file.
// phpMyAdmin is reachable only through the panel, after its sign-in.
declare(strict_types=1);

$cfg['blowfish_secret'] = '` + secret + `';

$i = 1;
$cfg['Servers'][$i]['auth_type'] = 'cookie';
$cfg['Servers'][$i]['host'] = 'localhost';
$cfg['Servers'][$i]['compress'] = false;
$cfg['Servers'][$i]['AllowNoPassword'] = false;
$cfg['Servers'][$i]['AllowRoot'] = false;

$cfg['AllowArbitraryServer'] = false;
$cfg['TempDir'] = '` + tmpDir + `';
$cfg['UploadDir'] = '';
$cfg['SaveDir'] = '';
$cfg['VersionCheck'] = false;
$cfg['SendErrorReports'] = 'never';
$cfg['ZeroConf'] = false;
$cfg['ShowPhpInfo'] = false;
// The panel keeps each database's password (websites use it): change it on
// the panel's Databases page.
$cfg['ShowChgPassword'] = false;
$cfg['ShowCreateDb'] = false;
$cfg['ExecTimeLimit'] = 600;

// The panel forwards /<entrance>/phpmyadmin/ to this server.
$nefoBase = '/';
if (preg_match('~^/[a-z0-9]{3,32}/phpmyadmin/~', (string) ($_SERVER['REQUEST_URI'] ?? ''), $nefoMatch) === 1) {
    $nefoBase = $nefoMatch[0];
}
$nefoHost = (string) ($_SERVER['HTTP_HOST'] ?? '');
if (preg_match('~^([A-Za-z0-9.-]{1,253}|\[[0-9A-Fa-f:.]{2,45}\])(:[0-9]{1,5})?$~', $nefoHost) !== 1) {
    $nefoHost = 'localhost';
}
$nefoScheme = (($_SERVER['HTTP_X_PANEL_HTTPS'] ?? '') === 'on') ? 'https' : 'http';
$cfg['PmaAbsoluteUri'] = $nefoScheme . '://' . $nefoHost . $nefoBase;
unset($nefoBase, $nefoHost, $nefoScheme, $nefoMatch);
`), nil
}

// phpMyAdminSkip are the parts of the release the panel leaves out: the
// setup script (never reachable), the examples, the built documentation
// (links go to docs.phpmyadmin.net instead), the sample configuration and
// the developers' files.
var phpMyAdminSkip = []string{
	"setup/", "examples/", "doc/", "test/", "tests/",
	"config.sample.inc.php", "show_config_errors.php",
	"package.json", "yarn.lock", "composer.json", "composer.lock", "babel.config.json", ".rtlcssrc.json", "CONTRIBUTING.md",
}

func skipPhpMyAdminFile(name string) bool {
	for _, s := range phpMyAdminSkip {
		if name == s || (strings.HasSuffix(s, "/") && strings.HasPrefix(name, s)) {
			return true
		}
	}
	return false
}

// unpackPhpMyAdmin checks the archive against its pinned SHA-256 and
// unpacks it into dir, which must not exist. Only plain files and folders
// are taken, inside the release's one top folder, at most limits.
func unpackPhpMyAdmin(archive []byte, wantSHA256, dir string) error {
	sum := sha256.Sum256(archive)
	if hex.EncodeToString(sum[:]) != wantSHA256 {
		return errors.New("the phpMyAdmin download does not match its pinned checksum: not installed")
	}
	// Explicit modes: the panel's umask (077) would leave the files to root,
	// and PHP-FPM reads them as another user.
	if err := os.Mkdir(dir, 0o755); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		return err
	}
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	const maxFiles, maxBytes = 20000, 256 << 20
	files, total := 0, int64(0)
	top := ""
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read the phpMyAdmin archive: %w", err)
		}
		name := strings.TrimPrefix(h.Name, "./")
		first, rest, _ := strings.Cut(name, "/")
		if top == "" {
			top = first
		}
		if first != top || !strings.HasPrefix(top, "phpMyAdmin-") {
			return fmt.Errorf("unexpected path %q in the phpMyAdmin archive", h.Name)
		}
		if rest == "" {
			continue // the top folder itself
		}
		clean := path.Clean(rest)
		if clean != strings.TrimSuffix(rest, "/") || strings.HasPrefix(clean, "../") || clean == ".." || path.IsAbs(clean) {
			return fmt.Errorf("unexpected path %q in the phpMyAdmin archive", h.Name)
		}
		if skipPhpMyAdminFile(clean) || skipPhpMyAdminFile(clean+"/") {
			continue
		}
		target := filepath.Join(dir, filepath.FromSlash(clean))
		switch h.Typeflag {
		case tar.TypeDir:
			if err := fsutil.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if files++; files > maxFiles {
				return errors.New("the phpMyAdmin archive has too many files")
			}
			if total += h.Size; h.Size < 0 || total > maxBytes {
				return errors.New("the phpMyAdmin archive is too large")
			}
			if err := fsutil.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
			if err != nil {
				return err
			}
			if err := f.Chmod(0o644); err != nil {
				f.Close()
				return err
			}
			_, err = io.Copy(f, io.LimitReader(tr, h.Size))
			if cerr := f.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				return err
			}
		default:
			// Links and devices: a release has none, and none is followed.
			return fmt.Errorf("unexpected entry %q (type %c) in the phpMyAdmin archive", h.Name, h.Typeflag)
		}
	}
	if !fileExists(filepath.Join(dir, "index.php")) {
		return errors.New("the phpMyAdmin archive has no index.php")
	}
	return nil
}

// InstallPhpMyAdmin downloads the pinned phpMyAdmin release (checked
// against its SHA-256), writes its configuration and sets up the web
// server's internal server for it.
func (m *Manager) InstallPhpMyAdmin(ctx context.Context, lg Logger) error {
	if !m.Supported() {
		return ErrUnsupported
	}
	if !fileExists(m.web().hookFile()) {
		return ErrNotReady
	}
	if m.adminerPHP(ctx) == "" {
		return errors.New("PHP is not installed")
	}
	m.ops.Lock()
	defer m.ops.Unlock()

	lg.Logf("Downloading phpMyAdmin %s", PhpMyAdminVersion)
	b, err := fetch(ctx, phpMyAdminURL, phpMyAdminSize)
	if err != nil {
		return err
	}
	secret, _ := m.store.Setting(ctx, phpMyAdminSecretKey)
	if secret == "" {
		secret = secure.String(32, phpMyAdminSecretChars)
		if err := m.store.SetSetting(ctx, phpMyAdminSecretKey, secret); err != nil {
			return err
		}
	}
	cfg, err := phpMyAdminConfig(secret, filepath.Join(phpMyAdminVar, "tmp"))
	if err != nil {
		return err
	}
	g, err := user.LookupGroup("www-data")
	if err != nil {
		return fmt.Errorf("the www-data group PHP runs as: %w", err)
	}
	gid, _ := strconv.Atoi(g.Gid)
	u, err := user.Lookup("www-data")
	if err != nil {
		return fmt.Errorf("the www-data user PHP runs as: %w", err)
	}
	uid, _ := strconv.Atoi(u.Uid)

	// Unpacked next to the old copy, then swapped in.
	lg.Logf("Unpacking phpMyAdmin %s into %s", PhpMyAdminVersion, phpMyAdminDir)
	stage, old := phpMyAdminDir+".new", phpMyAdminDir+".old"
	os.RemoveAll(stage)
	if err := unpackPhpMyAdmin(b, phpMyAdminSHA256, stage); err != nil {
		os.RemoveAll(stage)
		return err
	}
	conf := filepath.Join(stage, "config.inc.php")
	if err := writeFile(conf, cfg, 0o640); err != nil {
		os.RemoveAll(stage)
		return err
	}
	if err := os.Chown(conf, 0, gid); err != nil {
		os.RemoveAll(stage)
		return err
	}
	if err := writeFile(filepath.Join(stage, ".user.ini"), []byte(adminerUserIni), 0o644); err != nil {
		os.RemoveAll(stage)
		return err
	}
	os.RemoveAll(old)
	if fileExists(phpMyAdminDir) {
		if err := os.Rename(phpMyAdminDir, old); err != nil {
			os.RemoveAll(stage)
			return err
		}
	}
	if err := os.Rename(stage, phpMyAdminDir); err != nil {
		os.Rename(old, phpMyAdminDir)
		return err
	}
	os.RemoveAll(old)
	if err := mkdir(phpMyAdminVar, 0o755); err != nil {
		return err
	}
	tmp := filepath.Join(phpMyAdminVar, "tmp")
	if err := mkdir(tmp, 0o700); err != nil {
		return err
	}
	if err := os.Chown(tmp, uid, gid); err != nil {
		return err
	}

	if p, _ := m.store.Setting(ctx, phpMyAdminPortKey); p == "" {
		if err := m.store.SetSetting(ctx, phpMyAdminPortKey, strconv.Itoa(freePort())); err != nil {
			return err
		}
	}
	if t, _ := m.store.Setting(ctx, phpMyAdminTokenKey); t == "" {
		if err := m.store.SetSetting(ctx, phpMyAdminTokenKey, hex.EncodeToString(secure.Bytes(24))); err != nil {
			return err
		}
	}
	sconf, err := m.phpMyAdminConfBytes(ctx)
	if err != nil {
		return err
	}
	if err := writeFile(m.phpMyAdminConf(), sconf, 0o600); err != nil {
		return err
	}
	if err := m.reloadWeb(ctx); err != nil {
		removeIfExists(m.phpMyAdminConf())
		m.reloadWeb(context.WithoutCancel(ctx))
		return err
	}
	lg.Logf("phpMyAdmin %s is ready. Open it from the Databases page and sign in with a database's user and password.", PhpMyAdminVersion)
	return nil
}

// RemovePhpMyAdmin uninstalls phpMyAdmin: its internal server, its files
// and its settings. Databases are not touched.
func (m *Manager) RemovePhpMyAdmin(ctx context.Context, lg Logger) error {
	if !m.Supported() {
		return ErrUnsupported
	}
	m.ops.Lock()
	defer m.ops.Unlock()
	lg.Logf("Removing phpMyAdmin")
	hadConf := fileExists(m.phpMyAdminConf())
	if err := m.removePhpMyAdmin(ctx); err != nil {
		return err
	}
	if hadConf && fileExists(m.web().hookFile()) && m.web().info(ctx).Running {
		if err := m.reloadWeb(ctx); err != nil {
			return err
		}
	}
	lg.Logf("phpMyAdmin was removed.")
	return nil
}

// removePhpMyAdmin deletes phpMyAdmin's internal server configuration (of
// every web server), its files and its settings. The caller holds the lock
// and reloads the web server.
func (m *Manager) removePhpMyAdmin(ctx context.Context) error {
	for _, kind := range serverKinds {
		if err := removeIfExists(filepath.Join(m.server(kind).sitesDir(), "_phpmyadmin.conf")); err != nil {
			return err
		}
	}
	for _, d := range []string{phpMyAdminDir, phpMyAdminDir + ".new", phpMyAdminDir + ".old", phpMyAdminVar} {
		if err := os.RemoveAll(d); err != nil {
			return err
		}
	}
	for _, k := range []string{phpMyAdminPortKey, phpMyAdminTokenKey, phpMyAdminSecretKey} {
		if err := m.store.SetSetting(ctx, k, ""); err != nil {
			return err
		}
	}
	return nil
}

// The internal servers get the path the panel forwards, /<entrance>/
// phpmyadmin/<file> (the entrance is one segment of a-z and 0-9), so
// phpMyAdmin sees the address the browser uses. Only its three entry
// points run as PHP, and only its scripts, styles and images are served
// as files; everything else is 404. The token is a secret: the files are
// readable by root only.

// phpMyAdminConf is the internal Nginx server for phpMyAdmin.
func (n nginxServer) phpMyAdminConf(port, token, php string) []byte {
	if !adminerTokenRe.MatchString(token) || !adminerPortRe.MatchString(port) || validate.PHPVersion(php) != nil {
		return nil
	}
	return []byte(fmt.Sprintf(`# Managed by %[1]s. phpMyAdmin, reachable only through the panel.
server {
    listen 127.0.0.1:%[2]s;
    server_name _;
    access_log off;
    server_tokens off;
    client_max_body_size 256m;
    root %[4]s;

    if ($http_x_panel_phpmyadmin != "%[3]s") {
        return 404;
    }
    rewrite ^(/[a-z0-9]+/phpmyadmin/)$ $1index.php last;

    location ~ ^/[a-z0-9]+/phpmyadmin/(?<pma_script>index\.php|url\.php|js/messages\.php)$ {
        include fastcgi_params;
        fastcgi_param SCRIPT_FILENAME %[4]s/$pma_script;
        fastcgi_param HTTP_X_PANEL_PHPMYADMIN "" if_not_empty;
        fastcgi_param PHP_VALUE "upload_max_filesize=256M
post_max_size=256M
max_execution_time=600
memory_limit=256M";
        fastcgi_pass unix:/run/php/php%[5]s-fpm.sock;
        fastcgi_read_timeout 600s;
    }
    location ~ ^/[a-z0-9]+/phpmyadmin/(?<pma_file>(?:js|themes)/[A-Za-z0-9._/-]+\.(?:js|css|map|png|svg|gif|jpg|ico)|favicon\.ico)$ {
        alias %[4]s/$pma_file;
        expires 1d;
    }
    location / {
        return 404;
    }
}
`, version.AppName, port, token, phpMyAdminDir, php))
}

var apachePhpMyAdminTmpl = mustApacheTemplate("apache-phpmyadmin", `# Managed by {{app}}. phpMyAdmin, reachable only through the panel.
Listen 127.0.0.1:{{.Port}}
<VirtualHost 127.0.0.1:{{.Port}}>
    ServerName phpmyadmin.invalid
    DocumentRoot {{.Dir}}
    ProxyTimeout 600

    # Only requests with the panel's token get in; phpMyAdmin never sees it.
    RewriteEngine On
    RewriteCond %{HTTP:X-Panel-Phpmyadmin} !={{.Token}}
    RewriteRule ^ - [R=404,L]
    RewriteCond expr "%{HTTP:Content-Length} -gt 268435456"
    RewriteRule ^ - [R=413,L]
    RewriteRule \.\. - [R=404,L]
    RewriteRule ^/[a-z0-9]+/phpmyadmin/$ {{.Dir}}/index.php [L]
    RewriteRule ^/[a-z0-9]+/phpmyadmin/(index\.php|url\.php|js/messages\.php)$ {{.Dir}}/$1 [L]
    RewriteRule ^/[a-z0-9]+/phpmyadmin/((js|themes)/[A-Za-z0-9._/-]+\.(js|css|map|png|svg|gif|jpg|ico)|favicon\.ico)$ {{.Dir}}/$1 [L]
    RewriteRule ^ - [R=404,L]
    RequestHeader unset X-Panel-Phpmyadmin

    <Directory {{.Dir}}>
        Options None
        AllowOverride None
        Require all granted
        <FilesMatch "\.php$">
            SetHandler "proxy:unix:/run/php/php{{.PHP}}-fpm.sock|fcgi://localhost"
        </FilesMatch>
    </Directory>
</VirtualHost>
`)

func (a apacheServer) phpMyAdminConf(port, token, php string) []byte {
	if !adminerTokenRe.MatchString(token) || !adminerPortRe.MatchString(port) || validate.PHPVersion(php) != nil {
		return nil
	}
	out, err := render(apachePhpMyAdminTmpl, struct{ Port, Token, Dir, PHP string }{port, token, phpMyAdminDir, php})
	if err != nil {
		return nil
	}
	return out
}

var olsPhpMyAdminTmpl = mustOLSTemplate("ols-phpmyadmin", `# Managed by {{app}}. phpMyAdmin, reachable only through the panel.
listener panel_phpmyadmin {
  address                 127.0.0.1:{{.Port}}
  secure                  0
  map                     _phpmyadmin *
}

virtualHost _phpmyadmin {
  vhRoot                  {{.Dir}}/
  user                    www-data
  group                   www-data
  allowSymbolLink         0
  enableScript            1
  restrained              1
  docRoot                 {{.Dir}}/
  extprocessor phpmyadmin_php {
    type                  fcgi
    address               uds://run/php/php{{.PHP}}-fpm.sock
    maxConns              5
    initTimeout           600
    retryTimeout          0
    persistConn           0
    respBuffer            0
    autoStart             0
  }
  scripthandler {
    add                   fcgi:phpmyadmin_php php
  }
  index {
    useServer             0
    indexFiles            index.php
    autoIndex             0
  }
  # Only requests with the panel's token get in.
  rewrite {
    enable                1
    autoLoadHtaccess      0
    rules                 <<<END_rules
RewriteCond %{HTTP:X-Panel-Phpmyadmin} !={{.Token}}
RewriteRule ^ - [R=404,L]
RewriteCond %{HTTP:Content-Length} -gt268435456
RewriteRule ^ - [R=413,L]
RewriteRule \.\. - [R=404,L]
RewriteRule ^/?[a-z0-9]+/phpmyadmin/$ /index.php [L]
RewriteRule ^/?[a-z0-9]+/phpmyadmin/(index\.php|url\.php|js/messages\.php)$ /$1 [L]
RewriteRule ^/?[a-z0-9]+/phpmyadmin/((js|themes)/[A-Za-z0-9._/-]+\.(js|css|map|png|svg|gif|jpg|ico)|favicon\.ico)$ /$1 [L]
RewriteRule ^ - [R=404,L]
END_rules
  }
}
`)

func (o olsServer) phpMyAdminConf(port, token, php string) []byte {
	if !adminerTokenRe.MatchString(token) || !adminerPortRe.MatchString(port) || validate.PHPVersion(php) != nil {
		return nil
	}
	out, err := render(olsPhpMyAdminTmpl, struct{ Port, Token, Dir, PHP string }{port, token, phpMyAdminDir, php})
	if err != nil {
		return nil
	}
	return out
}
