package hosting

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/secure"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
	"github.com/nefocp/nefo/internal/version"
)

const (
	wordpressMaxSize = 128 << 20
	AppWordPress     = "wordpress"
)

var (
	wpUserRe   = regexp.MustCompile(`^[A-Za-z0-9_.@-]{1,60}$`)
	emailRe    = regexp.MustCompile(`^[^@\s]{1,64}@[A-Za-z0-9.-]{1,253}\.[A-Za-z]{2,63}$`)
	wpDefineRe = regexp.MustCompile(`define\(\s*'DB_PASSWORD',\s*'[^'\\]*'\s*\);`)
)

// WordPressRequest describes a WordPress installation.
type WordPressRequest struct {
	Title         string
	AdminUser     string
	AdminEmail    string
	AdminPassword string
	DB            *store.Database
	// MariaDB is the database server's version (Moodle picks its release
	// by it), "" on MySQL; Inputs are app-specific form values
	// (WooCommerce's country).
	MariaDB string
	Inputs  map[string]string
	// DBEngine is the database server: "mariadb" (or "") or "mysql". Apps
	// with a driver per server (Laravel, Moodle) are set up for it.
	DBEngine string
}

// mysqlEngine reports whether the request's database server is MySQL.
func (r WordPressRequest) mysqlEngine() bool { return r.DBEngine == "mysql" }

// CheckWordPress validates a request before any work starts, so the UI can
// show mistakes at once.
func (m *Manager) CheckWordPress(ctx context.Context, siteID int64, req WordPressRequest) error {
	site, err := m.store.SiteByID(ctx, siteID)
	if err != nil {
		return err
	}
	if site.PHPVersion == "" {
		return usererr.New("wp_needs_php")
	}
	if site.App != "" {
		return usererr.New("app_exists", site.App)
	}
	if t := strings.TrimSpace(req.Title); t == "" || len(t) > 200 {
		return usererr.New("invalid_wp_title")
	}
	if !wpUserRe.MatchString(req.AdminUser) {
		return usererr.New("invalid_wp_user")
	}
	if !emailRe.MatchString(req.AdminEmail) {
		return usererr.New("invalid_email")
	}
	if err := checkAdminPassword(req.AdminPassword); err != nil {
		return err
	}
	if err := m.publicIsEmpty(site); err != nil {
		return err
	}
	return nil
}

// minAdminPassword is the shortest administrator password an app install
// accepts (WordPress and the other PHP apps alike); the UI checks the same.
const minAdminPassword = 12

func checkAdminPassword(p string) error {
	if n := len(p); n < minAdminPassword || n > 128 {
		return usererr.New("invalid_app_password")
	}
	return nil
}

// publicIsEmpty refuses to install over existing files. The panel's own
// placeholder page does not count.
func (m *Manager) publicIsEmpty(site *store.Site) error {
	public := filepath.Join(m.paths.siteRoot(site.Domain), "public")
	entries, err := os.ReadDir(public)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Name() == "index.html" && m.isPlaceholder(filepath.Join(public, "index.html")) {
			continue
		}
		return usererr.New("site_not_empty", public)
	}
	return nil
}

func (m *Manager) isPlaceholder(p string) bool {
	b, err := os.ReadFile(p)
	return err == nil && len(b) < 4096 && bytes.Contains(b, []byte("placeholder page was created by the server's control panel"))
}

// InstallWordPress downloads WordPress into an empty site, writes
// wp-config.php for the given database and runs the WordPress installer as
// the site's user. On failure the site's public folder is emptied again.
func (m *Manager) InstallWordPress(ctx context.Context, siteID int64, req WordPressRequest, lg Logger) error {
	if !m.Supported() {
		return ErrUnsupported
	}
	if err := m.CheckWordPress(ctx, siteID, req); err != nil {
		return err
	}
	if req.DB == nil || validate.DBName(req.DB.Name) != nil || validate.DBName(req.DB.Username) != nil ||
		validate.DBPassword(req.DB.Password) != nil {
		return errors.New("invalid database settings")
	}
	m.ops.Lock()
	defer m.ops.Unlock()

	site, err := m.store.SiteByID(ctx, siteID)
	if err != nil {
		return err
	}
	public := filepath.Join(m.paths.siteRoot(site.Domain), "public")
	uid, gid, err := lookupIDs(site.SysUser)
	if err != nil {
		return err
	}

	lg.Logf("Downloading WordPress")
	archive, size, err := downloadWordPress(ctx)
	if err != nil {
		return err
	}
	defer func() {
		archive.Close()
		os.Remove(archive.Name())
	}()
	lg.Logf("Checksum verified (%d MB)", size>>20)

	ok := false
	defer func() {
		if !ok {
			lg.Logf("Removing the partly installed files")
			m.emptyPublic(site, uid, gid)
		}
	}()

	removeIfExists(filepath.Join(public, "index.html"))
	lg.Logf("Extracting the files")
	n, err := extractWordPress(archive, public, uid, gid)
	if err != nil {
		return err
	}
	lg.Logf("Extracted %d files", n)

	lg.Logf("Writing wp-config.php")
	cfg, err := wpConfig(req.DB)
	if err != nil {
		return err
	}
	if err := writeOwned(public, "wp-config.php", cfg, 0o600, uid, gid); err != nil {
		return err
	}

	lg.Logf("Running the WordPress installer")
	url := "http://" + site.Domain
	if site.SSL {
		url = "https://" + site.Domain
	}
	out, err := m.wpRun(ctx, site, wpInstallScript, map[string]any{
		"public": public, "host": site.Domain, "https": site.SSL, "url": url,
		"title": strings.TrimSpace(req.Title), "user": req.AdminUser, "email": req.AdminEmail,
		"password": req.AdminPassword,
	})
	if err != nil {
		return err
	}
	if !strings.Contains(out, "panel-wp-ok") {
		return fmt.Errorf("the WordPress installer did not finish:\n%s", out)
	}

	site.App = AppWordPress
	if err := m.store.UpdateSite(ctx, site); err != nil {
		return err
	}
	ok = true
	lg.Logf("WordPress is installed. Sign in at %s/wp-admin/", url)
	return nil
}

// UpdateDBPassword writes a changed database password into a WordPress
// site's wp-config.php. It does nothing for other sites or databases.
func (m *Manager) UpdateDBPassword(ctx context.Context, siteID int64, dbName, password string) error {
	m.ops.Lock()
	defer m.ops.Unlock()
	site, err := m.store.SiteByID(ctx, siteID)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil || site.App != AppWordPress {
		return err
	}
	if validate.DBPassword(password) != nil {
		return errors.New("invalid database password")
	}
	public := filepath.Join(m.paths.siteRoot(site.Domain), "public")
	root, err := os.OpenRoot(public)
	if err != nil {
		return err
	}
	defer root.Close()
	b, err := root.ReadFile("wp-config.php")
	if err != nil {
		return err
	}
	if !bytes.Contains(b, []byte("'DB_NAME', '"+dbName+"'")) {
		return nil // the site uses another database
	}
	if !wpDefineRe.Match(b) {
		return errors.New("wp-config.php: DB_PASSWORD not found; update it by hand")
	}
	b = wpDefineRe.ReplaceAllLiteral(b, []byte("define( 'DB_PASSWORD', '"+password+"' );"))
	if err := root.WriteFile("wp-config.php", b, 0o600); err != nil {
		return err
	}
	// OPcache keeps the old wp-config.php for up to two seconds; a reload
	// clears it, so the site does not fail with the old password meanwhile.
	return m.reloadPHP(ctx, site.PHPVersion)
}

// wpSetURL points WordPress at https:// after a certificate was issued.
func (m *Manager) wpSetURL(ctx context.Context, site *store.Site, url string) error {
	public := filepath.Join(m.paths.siteRoot(site.Domain), "public")
	out, err := m.wpRun(ctx, site, wpURLScript, map[string]any{"public": public, "host": site.Domain, "url": url})
	if err != nil {
		return err
	}
	if !strings.Contains(out, "panel-wp-ok") {
		return fmt.Errorf("could not update the WordPress address:\n%s", out)
	}
	return nil
}

// wpRun runs a constant PHP script as the site's user with the PHP CLI. The
// input, which may hold passwords, goes in as JSON on stdin.
func (m *Manager) wpRun(ctx context.Context, site *store.Site, script string, input any) (string, error) {
	if validate.SystemUsername(site.SysUser) != nil || validate.PHPVersion(site.PHPVersion) != nil {
		return "", errors.New("invalid site")
	}
	data, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	return m.run.Input(ctx, data, "runuser", "-u", site.SysUser, "--",
		"php"+site.PHPVersion, "-d", "sendmail_path=/bin/true", "-d", "memory_limit=256M", "-r", script)
}

const wpBootstrap = `
$in = json_decode(stream_get_contents(STDIN), true);
if (!is_array($in)) { fwrite(STDERR, "bad input\n"); exit(1); }
$_SERVER['HTTP_HOST'] = $in['host'];
$_SERVER['SERVER_NAME'] = $in['host'];
$_SERVER['REQUEST_URI'] = '/';
if (!empty($in['https'])) { $_SERVER['HTTPS'] = 'on'; }
`

const wpInstallScript = wpBootstrap + `
define('WP_INSTALLING', true);
require $in['public'] . '/wp-load.php';
require_once ABSPATH . 'wp-admin/includes/upgrade.php';
if (is_blog_installed()) { fwrite(STDERR, "WordPress is already installed in this database\n"); exit(1); }
$r = wp_install($in['title'], $in['user'], $in['email'], true, '', wp_slash($in['password']));
if (is_wp_error($r)) { fwrite(STDERR, $r->get_error_message() . "\n"); exit(1); }
update_option('siteurl', $in['url']);
update_option('home', $in['url']);
echo "panel-wp-ok\n";
`

const wpURLScript = wpBootstrap + `
require $in['public'] . '/wp-load.php';
foreach (['siteurl', 'home'] as $o) {
	$old = get_option($o);
	if ($old === 'http://' . $in['host'] || $old === 'http://' . $in['host'] . '/') { update_option($o, $in['url']); }
}
echo "panel-wp-ok\n";
`

// downloadWordPress saves the WordPress release of the catalog to a
// temporary file, checking its pinned SHA-256, and returns the open file.
// The caller closes and removes it.
func downloadWordPress(ctx context.Context) (*os.File, int64, error) {
	app, ok := phpApp(AppWordPress)
	if !ok || app.PHP.Download == nil {
		return nil, 0, errors.New("WordPress is missing from the app catalog")
	}
	return download(ctx, app.PHP.Download.URL, app.PHP.Download.SHA256)
}

// extractWordPress unpacks the wordpress/ folder of the archive into dir.
// It works inside an os.Root, so no entry (and no symlink planted in the
// folder) can write outside dir. Links in the archive are skipped.
func extractWordPress(archive io.Reader, dir string, uid, gid int) (int, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return 0, err
	}
	defer root.Close()
	gz, err := gzip.NewReader(archive)
	if err != nil {
		return 0, err
	}
	tr := tar.NewReader(gz)
	count := 0
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, err
		}
		name, ok := strings.CutPrefix(path.Clean(h.Name), "wordpress/")
		if !ok || name == "" || !fs.ValidPath(name) {
			continue
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(name, 0o755); err != nil {
				return count, err
			}
			if err := chmodChown(root, name, 0o755, uid, gid); err != nil {
				return count, err
			}
		case tar.TypeReg:
			if d := path.Dir(name); d != "." {
				if err := root.MkdirAll(d, 0o755); err != nil {
					return count, err
				}
			}
			f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
			if err != nil {
				return count, err
			}
			_, err = io.Copy(f, io.LimitReader(tr, wordpressMaxSize))
			if cerr := f.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				return count, err
			}
			if err := chmodChown(root, name, 0o644, uid, gid); err != nil {
				return count, err
			}
			count++
		}
	}
	return count, nil
}

// chmodChown sets a mode and owner; uid -1 leaves the owner alone.
func chmodChown(root *os.Root, name string, mode os.FileMode, uid, gid int) error {
	if err := root.Chmod(name, mode); err != nil {
		return err
	}
	if uid < 0 {
		return nil
	}
	return root.Lchown(name, uid, gid)
}

func writeOwned(dir, name string, data []byte, mode os.FileMode, uid, gid int) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.WriteFile(name, data, mode); err != nil {
		return err
	}
	return chmodChown(root, name, mode, uid, gid)
}

// emptyPublic removes everything in a site's public folder and puts the
// placeholder page back.
func (m *Manager) emptyPublic(site *store.Site, uid, gid int) {
	public := filepath.Join(m.paths.siteRoot(site.Domain), "public")
	root, err := os.OpenRoot(public)
	if err != nil {
		return
	}
	defer root.Close()
	entries, _ := os.ReadDir(public)
	for _, e := range entries {
		root.RemoveAll(e.Name())
	}
	if page, err := renderWelcome(site.Domain); err == nil {
		writeOwned(public, "index.html", page, 0o640, uid, gid)
	}
}

// wpConfig renders wp-config.php. Database values passed validate.DBName and
// validate.DBPassword, so they cannot contain quotes or backslashes.
func wpConfig(db *store.Database) ([]byte, error) {
	var keys strings.Builder
	for _, k := range []string{"AUTH_KEY", "SECURE_AUTH_KEY", "LOGGED_IN_KEY", "NONCE_KEY",
		"AUTH_SALT", "SECURE_AUTH_SALT", "LOGGED_IN_SALT", "NONCE_SALT"} {
		fmt.Fprintf(&keys, "define( '%s', '%s' );\n", k, secure.String(64, saltChars))
	}
	return []byte(fmt.Sprintf(`<?php
// Written by %s when it installed WordPress.

define( 'DB_NAME', '%s' );
define( 'DB_USER', '%s' );
define( 'DB_PASSWORD', '%s' );
define( 'DB_HOST', 'localhost' );
define( 'DB_CHARSET', 'utf8mb4' );
define( 'DB_COLLATE', '' );

%s
$table_prefix = 'wp_';

define( 'WP_DEBUG', false );
// PHP runs as the site's own user, so WordPress can update itself directly.
define( 'FS_METHOD', 'direct' );

if ( ! defined( 'ABSPATH' ) ) {
	define( 'ABSPATH', __DIR__ . '/' );
}

require_once ABSPATH . 'wp-settings.php';
`, version.AppName, db.Name, db.Username, db.Password, keys.String())), nil
}

// saltChars leaves out quotes and backslashes.
const saltChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!#$%&()*+,-./:;<=>?@[]^_{|}~"
