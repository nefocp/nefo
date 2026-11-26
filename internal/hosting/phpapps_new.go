package hosting

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/bzip2"
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
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/appstore"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// checkPHPAppSite checks a PHP website against an app's needs: PHP version,
// extensions, the database server (Moodle) and the server's resources.
func (m *Manager) checkPHPAppSite(ctx context.Context, app *appstore.App, site *store.Site, req AppRequest) error {
	p := app.PHP
	if site.PHPVersion == "" || site.ProxyPort != 0 {
		return usererr.New("wp_needs_php")
	}
	if !versionAtLeast(site.PHPVersion, p.MinPHP) {
		return usererr.New("app_needs_newer_php", p.MinPHP)
	}
	if p.MaxPHP != "" && !versionAtLeast(p.MaxPHP, site.PHPVersion) {
		return usererr.New("app_needs_older_php", p.MaxPHP)
	}
	if len(p.Variants) > 0 {
		if _, _, ok := p.PickDownload(req.MariaDB); !ok {
			return usererr.New("app_needs_newer_mariadb", p.Variants[len(p.Variants)-1].MinMariaDB)
		}
	}
	if missing := m.missingExtensions(ctx, site.PHPVersion, p.Extensions); len(missing) > 0 {
		return usererr.New("php_extensions_missing", strings.Join(missing, ", "))
	}
	if _, err := checkInputs(p.Inputs, req.Inputs); err != nil {
		return err
	}
	if res := appstore.Check(app, appstore.ReadSystem(m.paths.WWW)); !res.OK() {
		return usererr.New(res.Refusals[0].Code, res.Refusals[0].Detail)
	}
	return nil
}

// missingExtensions returns the PHP extensions a PHP version lacks.
func (m *Manager) missingExtensions(ctx context.Context, version string, want []string) []string {
	if len(want) == 0 {
		return nil
	}
	out, err := m.run.Output(ctx, "php"+version, "-m")
	if err != nil {
		return nil // PHP itself is checked elsewhere
	}
	have := map[string]bool{}
	for _, l := range strings.Split(strings.ToLower(out), "\n") {
		have[strings.TrimSpace(l)] = true
	}
	var missing []string
	for _, e := range want {
		if !have[e] {
			missing = append(missing, e)
		}
	}
	return missing
}

// fetchApp downloads an app's archive (pinned SHA-256 from the catalog)
// and unpacks it into dir.
func (m *Manager) fetchApp(ctx context.Context, id, mariadb, dir string, uid, gid int, lg Logger) error {
	app, ok := phpApp(id)
	if !ok {
		return fmt.Errorf("unknown app %s", id)
	}
	d, ver, ok := app.PHP.PickDownload(mariadb)
	if !ok {
		return usererr.New("app_needs_newer_mariadb")
	}
	return m.fetchArchive(ctx, app.Name+" "+ver, d, dir, uid, gid, lg)
}

func (m *Manager) fetchArchive(ctx context.Context, what string, d *appstore.Download, dir string, uid, gid int, lg Logger) error {
	lg.Logf("Downloading %s", what)
	f, size, err := download(ctx, d.URL, d.SHA256)
	if err != nil {
		return err
	}
	defer func() { f.Close(); os.Remove(f.Name()) }()
	lg.Logf("Checksum verified (%d MB). Extracting the files", size>>20)
	var n int
	switch d.Format {
	case "tgz":
		gz, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		n, err = extractTar(tar.NewReader(gz), dir, d.Strip, uid, gid)
		if err != nil {
			return err
		}
	case "tbz2":
		if n, err = extractTar(tar.NewReader(bzip2.NewReader(f)), dir, d.Strip, uid, gid); err != nil {
			return err
		}
	case "zip":
		zr, err := zip.NewReader(f, size)
		if err != nil {
			return err
		}
		if n, err = extractZip(zr, dir, d.Strip, uid, gid); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown archive format %q", d.Format)
	}
	lg.Logf("Extracted %d files", n)
	return nil
}

// extractZip is extractTar for zip archives: inside an os.Root, links and
// special files skipped, sizes limited.
func extractZip(zr *zip.Reader, dir, prefix string, uid, gid int) (int, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return 0, err
	}
	defer root.Close()
	count := 0
	for _, zf := range zr.File {
		name, ok := strings.CutPrefix(path.Clean(zf.Name), prefix)
		if !ok || name == "" || name == "." || !fs.ValidPath(name) {
			continue
		}
		mode := zf.Mode()
		switch {
		case mode.IsDir():
			if err := root.MkdirAll(name, 0o755); err != nil {
				return count, err
			}
			if err := chmodChown(root, name, 0o755, uid, gid); err != nil {
				return count, err
			}
		case mode.IsRegular():
			if zf.UncompressedSize64 > maxEntry {
				return count, fmt.Errorf("%s is larger than expected", zf.Name)
			}
			if d := path.Dir(name); d != "." {
				if err := root.MkdirAll(d, 0o755); err != nil {
					return count, err
				}
				if err := chmodChown(root, d, 0o755, uid, gid); err != nil {
					return count, err
				}
			}
			src, err := zf.Open()
			if err != nil {
				return count, err
			}
			dst, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
			if err != nil {
				src.Close()
				return count, err
			}
			_, err = io.Copy(dst, io.LimitReader(src, maxEntry))
			src.Close()
			if cerr := dst.Close(); err == nil {
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

// ---- WooCommerce ----

const wooActivateScript = wpBootstrap + `
require $in['public'] . '/wp-load.php';
require_once ABSPATH . 'wp-admin/includes/plugin.php';
$r = activate_plugin('woocommerce/woocommerce.php');
if (is_wp_error($r)) { fwrite(STDERR, $r->get_error_message() . "\n"); exit(1); }
update_option('woocommerce_default_country', $in['country']);
update_option('woocommerce_currency', $in['currency']);
echo "panel-wp-ok\n";
`

// installWooCommerce adds the WooCommerce plugin (pinned SHA-256) to a
// fresh WordPress, activates it (WooCommerce creates its tables and its
// shop, cart, checkout and account pages) and sets the store's country
// and currency. WordPress's own cron then runs from the system's cron.
func (m *Manager) installWooCommerce(ctx context.Context, siteID int64, app *appstore.App, req AppRequest, lg Logger) error {
	site, err := m.store.SiteByID(ctx, siteID)
	if err != nil {
		return err
	}
	uid, gid, err := lookupIDs(site.SysUser)
	if err != nil {
		return err
	}
	inputs, err := checkInputs(app.PHP.Inputs, req.Inputs)
	if err != nil {
		return err
	}
	public := filepath.Join(m.paths.siteRoot(site.Domain), "public")
	plugins := filepath.Join(public, "wp-content", "plugins")
	dir := filepath.Join(plugins, "woocommerce")
	ok := false
	defer func() {
		if !ok {
			lg.Logf("Removing the partly installed files")
			m.emptyPublic(site, uid, gid)
			site.App = ""
			m.store.UpdateSite(context.WithoutCancel(ctx), site)
		}
	}()
	if err := mkdir(dir, 0o755); err != nil {
		return err
	}
	if err := os.Chown(dir, uid, gid); err != nil {
		return err
	}
	if err := m.fetchArchive(ctx, "WooCommerce "+app.PHP.Plugin.Version, app.PHP.Plugin, dir, uid, gid, lg); err != nil {
		return err
	}
	if !fileExists(filepath.Join(dir, "woocommerce.php")) {
		return errors.New("the WooCommerce archive has no woocommerce.php")
	}
	// The store runs WordPress's scheduled tasks from the system's cron
	// (see the cron job the panel adds) and needs more memory.
	cfg := filepath.Join(public, "wp-config.php")
	b, err := os.ReadFile(cfg)
	if err != nil {
		return err
	}
	add := "define( 'WP_MEMORY_LIMIT', '256M' );\ndefine( 'DISABLE_WP_CRON', true );\n"
	b = bytes.Replace(b, []byte("define( 'FS_METHOD', 'direct' );\n"), []byte("define( 'FS_METHOD', 'direct' );\n"+add), 1)
	if !bytes.Contains(b, []byte("DISABLE_WP_CRON")) {
		return errors.New("could not add the WooCommerce settings to wp-config.php")
	}
	if err := writeOwned(public, "wp-config.php", b, 0o600, uid, gid); err != nil {
		return err
	}
	lg.Logf("Activating WooCommerce (it creates its tables and pages)")
	url := "http://" + site.Domain
	if site.SSL {
		url = "https://" + site.Domain
	}
	out, err := m.wpRun(ctx, site, wooActivateScript, map[string]any{"public": public, "host": site.Domain, "https": site.SSL,
		"url": url, "country": inputs["country"], "currency": inputs["currency"]})
	if err != nil {
		return err
	}
	if !strings.Contains(out, "panel-wp-ok") {
		return fmt.Errorf("WooCommerce was not activated:\n%s", run.Tail(out, 10))
	}
	ok = true
	lg.Logf("WooCommerce %s is active. Set up the store in wp-admin.", app.PHP.Plugin.Version)
	return nil
}

// ---- OpenCart ----

var ocStorageRe = regexp.MustCompile(`define\('DIR_STORAGE',\s*DIR_SYSTEM \. 'storage/'\);`)

func (m *Manager) installOpenCart(ctx context.Context, site *store.Site, app *appstore.App, req AppRequest, uid, gid int, lg Logger) error {
	root := m.paths.siteRoot(site.Domain)
	public := filepath.Join(root, "public")
	removeIfExists(filepath.Join(public, "index.html"))
	if err := m.fetchApp(ctx, app.ID, "", public, uid, gid, lg); err != nil {
		return err
	}
	for _, dir := range []string{public, filepath.Join(public, "admin")} {
		dist, err := os.ReadFile(filepath.Join(dir, "config-dist.php"))
		if err != nil {
			return err
		}
		if err := writeOwned(dir, "config.php", dist, 0o640, uid, gid); err != nil {
			return err
		}
	}
	scheme := "http"
	if site.SSL {
		scheme = "https"
	}
	lg.Logf("Running the OpenCart installer")
	inst := filepath.Join(public, "install")
	out, err := m.runArgv(ctx, site, inst, filepath.Join(inst, "cli_install.php"), []string{
		"cli_install.php", "install",
		"--username", req.AdminUser, "--password", req.AdminPassword, "--email", req.AdminEmail,
		"--http_server", scheme + "://" + site.Domain + "/",
		"--db_driver", "mysqli", "--db_hostname", "localhost", "--db_username", req.DB.Username,
		"--db_password", req.DB.Password, "--db_database", req.DB.Name, "--db_port", "3306", "--db_prefix", "oc_",
	})
	if err != nil || !strings.Contains(out, "SUCCESS") {
		return fmt.Errorf("the OpenCart installer failed: %v\n%s", err, run.Tail(out, 15))
	}
	// OpenCart's own security notices: no installer, storage outside the
	// web folder.
	if pr, err := os.OpenRoot(public); err == nil {
		pr.RemoveAll("install")
		pr.Remove("php.ini")
		pr.Close()
	}
	storage := filepath.Join(root, "storage")
	lg.Logf("Moving OpenCart's storage folder out of the web folder")
	if err := os.Rename(filepath.Join(public, "system", "storage"), storage); err != nil {
		return err
	}
	for _, dir := range []string{public, filepath.Join(public, "admin")} {
		b, err := os.ReadFile(filepath.Join(dir, "config.php"))
		if err != nil {
			return err
		}
		if !ocStorageRe.Match(b) {
			return fmt.Errorf("%s: DIR_STORAGE not found", filepath.Join(dir, "config.php"))
		}
		b = ocStorageRe.ReplaceAllLiteral(b, []byte("define('DIR_STORAGE', '"+storage+"/');"))
		if err := writeOwned(dir, "config.php", b, 0o640, uid, gid); err != nil {
			return err
		}
	}
	lg.Logf("OpenCart is installed. Sign in at /admin/")
	return nil
}

// ---- Moodle ----

// moodleUserRe: Moodle user names are lowercase.
var moodleUserRe = regexp.MustCompile(`^[a-z0-9_.@-]{1,60}$`)

func (m *Manager) installMoodle(ctx context.Context, site *store.Site, app *appstore.App, req AppRequest, uid, gid int, lg Logger) error {
	root := m.paths.siteRoot(site.Domain)
	user := strings.ToLower(req.AdminUser)
	if !moodleUserRe.MatchString(user) {
		return usererr.New("invalid_wp_user")
	}
	// Moodle 5.1 and later have their own public/ folder: the whole release
	// goes into the site folder and its public/ is the site's web folder.
	removeIfExists(filepath.Join(root, "public", "index.html"))
	if err := m.fetchApp(ctx, app.ID, req.MariaDB, root, uid, gid, lg); err != nil {
		return err
	}
	if !fileExists(filepath.Join(root, "public", "index.php")) || !fileExists(filepath.Join(root, "admin", "cli", "install.php")) {
		return errors.New("the Moodle archive does not have the expected layout (public/ folder)")
	}
	data := filepath.Join(root, "moodledata")
	if err := mkdir(data, 0o750); err != nil {
		return err
	}
	if err := os.Chown(data, uid, gid); err != nil {
		return err
	}
	scheme := "http"
	if site.SSL {
		scheme = "https"
	}
	short := strings.SplitN(site.Domain, ".", 2)[0]
	if len(short) > 20 {
		short = short[:20]
	}
	lg.Logf("Running the Moodle installer (several minutes)")
	dbtype := "mariadb"
	if req.mysqlEngine() {
		dbtype = "mysqli"
	}
	in, err := json.Marshal(map[string]any{"dir": root, "script": filepath.Join(root, "admin", "cli", "install.php"), "argv": []string{
		"install.php", "--non-interactive", "--agree-license", "--lang=en", "--chmod=2770",
		"--wwwroot=" + scheme + "://" + site.Domain, "--dataroot=" + data,
		"--dbtype=" + dbtype, "--dbhost=localhost", "--dbname=" + req.DB.Name, "--dbuser=" + req.DB.Username,
		"--dbpass=" + req.DB.Password, "--prefix=mdl_",
		"--fullname=" + strings.TrimSpace(req.Title), "--shortname=" + short,
		"--adminuser=" + user, "--adminpass=" + req.AdminPassword, "--adminemail=" + req.AdminEmail,
		"--supportemail=" + req.AdminEmail,
	}})
	if err != nil {
		return err
	}
	full, err := m.phpArgsINI(site, "512M", moodleINI(app), "-r", argvScript)
	if err != nil {
		return err
	}
	ctx2, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	out, err := m.run.Input(ctx2, in, "runuser", full...)
	if err != nil || !strings.Contains(out, "Installation completed successfully") || !fileExists(filepath.Join(root, "config.php")) {
		return fmt.Errorf("the Moodle installer failed: %v\n%s", err, run.Tail(out, 15))
	}
	// The installer works with umask 0 and leaves files such as
	// moodledata/.htaccess world-writable; --chmod=2770 only covers the
	// folders it makes.
	if err := clearOtherBits(data); err != nil {
		return err
	}
	lg.Logf("Moodle is installed. Sign in at /login/index.php")
	return nil
}

// clearOtherBits takes every permission for other users away below dir,
// without following links out of it.
func clearOtherBits(dir string) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	return fs.WalkDir(root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		mode := fi.Mode() & (fs.ModePerm | fs.ModeSetuid | fs.ModeSetgid | fs.ModeSticky)
		if mode&0o007 == 0 {
			return nil
		}
		return root.Chmod(p, mode&^0o007)
	})
}

// moodleINI returns the app's php.ini settings as -d values.
func moodleINI(app *appstore.App) []string {
	var out []string
	for _, k := range sortedNames(app.PHP.INI) {
		out = append(out, k+"="+app.PHP.INI[k])
	}
	return out
}

// PHPAppCron is the cron job a PHP app needs: its schedule and the shell
// command (run as the site's user).
func (m *Manager) PHPAppCron(site *store.Site, appID string) (schedule, command string, ok bool) {
	app, found := phpApp(appID)
	if !found || app.PHP.Cron == nil || site.PHPVersion == "" {
		return "", "", false
	}
	c := app.PHP.Cron
	args := []string{"php" + site.PHPVersion}
	args = append(args, c.PHPArg...)
	args = append(args, filepath.ToSlash(filepath.Join(m.paths.siteRoot(site.Domain), c.Script)))
	if slices.ContainsFunc(args, func(a string) bool { return strings.ContainsAny(a, " '\"$;&|`\\") }) {
		return "", "", false
	}
	return c.Schedule, strings.Join(args, " "), true
}

// appMaxInputVars returns the max_input_vars a site's PHP app needs (0:
// PHP's default).
func appMaxInputVars(appID string) int {
	app, ok := phpApp(appID)
	if !ok {
		return 0
	}
	n, err := strconv.Atoi(app.PHP.INI["max_input_vars"])
	if err != nil || n < 1000 || n > 100000 {
		return 0
	}
	return n
}
