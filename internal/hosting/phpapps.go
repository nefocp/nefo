package hosting

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/appstore"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/software"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
	"github.com/nefocp/nefo/internal/version"
)

// PHP apps from the app store install into an empty PHP website, with a
// database of their own, and run as the site's user like everything else
// on the site. Passwords never go on a command line: PHP gets them as JSON
// on stdin, so other users cannot read them in the process list.

// PHPApp is a PHP app of the catalog, as the API lists it.
type PHPApp struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	MinPHP   string `json:"min_php"`
	Admin    bool   `json:"admin"`    // asks for an administrator account
	Composer bool   `json:"composer"` // built with Composer: needs the whole site folder
}

// PHPApps lists the catalog's PHP apps.
func PHPApps() []PHPApp {
	out := []PHPApp{}
	for _, a := range appstore.Current().Apps {
		if a.PHP != nil {
			out = append(out, PHPApp{ID: a.ID, Name: a.Name, MinPHP: a.PHP.MinPHP, Admin: a.PHP.Admin, Composer: a.PHP.Composer != nil})
		}
	}
	return out
}

const (
	maxArchive = 1 << 30
	maxEntry   = 256 << 20
)

// AppRequest describes an application installation.
type AppRequest = WordPressRequest

// phpApp returns a PHP app of the catalog.
func phpApp(id string) (*appstore.App, bool) {
	a, ok := appstore.Lookup(id)
	if !ok || a.PHP == nil {
		return nil, false
	}
	return a, true
}

// AppDBPrefix returns the database name prefix of an app ("wp", "nc"…).
func AppDBPrefix(id string) string {
	if a, ok := phpApp(id); ok {
		return a.PHP.DBPrefix
	}
	return "app"
}

// isWordPressApp: WooCommerce is WordPress with a plugin, and its site is
// a WordPress site for everything else (toolkit, SSL, firewall).
func isWordPressApp(id string) bool { return id == AppWordPress || id == "woocommerce" }

var siteDefaults = []string{"public", "logs", "tmp"}

// NeedsSiteFolder reports whether a PHP app takes the whole site folder,
// not just public/: apps built with Composer, and Moodle, whose release has
// its own public/ folder. Their site folder must hold nothing but the
// empty public/, logs/ and tmp/.
func NeedsSiteFolder(app *appstore.App) bool {
	return app.PHP != nil && (app.PHP.Composer != nil || app.ID == "moodle")
}

// CheckPHPApp validates a request before a job starts.
func (m *Manager) CheckPHPApp(ctx context.Context, siteID int64, id string, req AppRequest) error {
	app, ok := phpApp(id)
	if !ok {
		return usererr.New("unknown_app", id)
	}
	site, err := m.store.SiteByID(ctx, siteID)
	if err != nil {
		return err
	}
	if err := m.checkPHPAppSite(ctx, app, site, req); err != nil {
		return err
	}
	if isWordPressApp(id) {
		return m.CheckWordPress(ctx, siteID, req)
	}
	if site.App != "" {
		return usererr.New("app_exists", site.App)
	}
	if app.PHP.Admin {
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
	}
	if err := m.publicIsEmpty(site); err != nil {
		return err
	}
	if NeedsSiteFolder(app) {
		entries, err := os.ReadDir(m.paths.siteRoot(site.Domain))
		if err != nil {
			return err
		}
		for _, e := range entries {
			if !slices.Contains(siteDefaults, e.Name()) {
				return usererr.New("site_not_empty", filepath.Join(m.paths.siteRoot(site.Domain), e.Name()))
			}
		}
	}
	return nil
}

// InstallPHPApp installs an app into an empty PHP site. On failure the
// site's folder is emptied again.
func (m *Manager) InstallPHPApp(ctx context.Context, siteID int64, id string, req AppRequest, lg Logger) error {
	if !m.Supported() {
		return ErrUnsupported
	}
	app, ok := phpApp(id)
	if !ok {
		return usererr.New("unknown_app", id)
	}
	if isWordPressApp(id) {
		if err := m.CheckPHPApp(ctx, siteID, id, req); err != nil {
			return err
		}
		if err := m.InstallWordPress(ctx, siteID, req, lg); err != nil {
			return err
		}
		if id == "woocommerce" {
			if err := m.installWooCommerce(ctx, siteID, app, req, lg); err != nil {
				return err
			}
		}
		return m.finishApp(ctx, siteID, app, lg)
	}
	if err := m.CheckPHPApp(ctx, siteID, id, req); err != nil {
		return err
	}
	if req.DB == nil || validate.DBName(req.DB.Name) != nil || validate.DBName(req.DB.Username) != nil ||
		validate.DBPassword(req.DB.Password) != nil {
		return errors.New("invalid database settings")
	}
	if app.PHP.Composer != nil {
		if err := m.ensureComposer(ctx, lg); err != nil {
			return err
		}
	}
	m.ops.Lock()
	site, err := m.store.SiteByID(ctx, siteID)
	if err != nil {
		m.ops.Unlock()
		return err
	}
	uid, gid, err := lookupIDs(site.SysUser)
	if err != nil {
		m.ops.Unlock()
		return err
	}
	ok = false
	defer func() {
		if !ok {
			lg.Logf("Removing the partly installed files")
			m.emptySite(site, uid, gid)
		}
	}()
	switch id {
	case "joomla":
		err = m.installJoomla(ctx, site, req, uid, gid, lg)
	case "nextcloud":
		err = m.installNextcloud(ctx, site, req, uid, gid, lg)
	case "laravel":
		err = m.installLaravel(ctx, site, req, uid, gid, lg)
	case "drupal":
		err = m.installDrupal(ctx, site, req, uid, gid, lg)
	case "opencart":
		err = m.installOpenCart(ctx, site, app, req, uid, gid, lg)
	case "moodle":
		err = m.installMoodle(ctx, site, app, req, uid, gid, lg)
	default:
		err = fmt.Errorf("no installer for %s", id)
	}
	m.ops.Unlock()
	if err != nil {
		return err
	}
	if err := m.finishApp(ctx, siteID, app, lg); err != nil {
		return err
	}
	ok = true
	return nil
}

// finishApp records the app and gives the site its rewrite preset and the
// PHP limits the app needs.
func (m *Manager) finishApp(ctx context.Context, siteID int64, app *appstore.App, lg Logger) error {
	m.ops.Lock()
	defer m.ops.Unlock()
	site, err := m.store.SiteByID(ctx, siteID)
	if err != nil {
		return err
	}
	old := *site
	site.App, site.Rewrite = app.ID, app.PHP.Rewrite
	if isWordPressApp(app.ID) {
		site.App = AppWordPress
	}
	phpChanged := len(app.PHP.INI) > 0 // pool settings such as max_input_vars
	if app.PHP.MemoryMB > site.PHPMemory {
		site.PHPMemory, phpChanged = app.PHP.MemoryMB, true
	}
	if app.PHP.UploadMB > site.PHPUpload {
		site.PHPUpload, phpChanged = app.PHP.UploadMB, true
	}
	if phpChanged {
		lg.Logf("Raising the site's PHP limits to %d MB of memory and %d MB uploads", site.PHPMemory, site.PHPUpload)
		if err := m.writePool(site); err != nil {
			return err
		}
		if err := m.reloadPHP(ctx, site.PHPVersion); err != nil {
			return err
		}
	}
	lg.Logf("Applying the %s rewrite rules", app.Name)
	return m.updateSite(ctx, site, &old)
}

// emptySite puts a site back as it was created: the placeholder page in an
// empty public folder, and nothing else but the logs and tmp folders.
func (m *Manager) emptySite(site *store.Site, uid, gid int) {
	dir := m.paths.siteRoot(site.Domain)
	if root, err := os.OpenRoot(dir); err == nil {
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if !slices.Contains(siteDefaults, e.Name()) {
				root.RemoveAll(e.Name())
			}
		}
		root.Close()
	}
	m.emptyPublic(site, uid, gid)
}

// ---- downloads and archives ----

// download saves url to a temporary file and checks its SHA-256 against
// want (hex). The caller closes and removes the file.
func download(ctx context.Context, url, want string) (*os.File, int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", version.AppName+"/"+version.Version)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	f, err := os.CreateTemp("", "panel-app-*")
	if err != nil {
		return nil, 0, err
	}
	fail := func(err error) (*os.File, int64, error) {
		f.Close()
		os.Remove(f.Name())
		return nil, 0, err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, maxArchive+1))
	if err != nil {
		return fail(fmt.Errorf("download %s: %w", url, err))
	}
	if n > maxArchive {
		return fail(fmt.Errorf("download %s: larger than expected", url))
	}
	if hex.EncodeToString(h.Sum(nil)) != strings.ToLower(want) {
		return fail(fmt.Errorf("the download of %s does not match its published checksum", url))
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return fail(err)
	}
	return f, n, nil
}

// extractTar unpacks the files under prefix ("" for all) of a tar stream
// into dir, owned by uid:gid. It works inside an os.Root, so no entry (and
// no symlink planted in the folder) can write outside dir; links in the
// archive are skipped.
func extractTar(tr *tar.Reader, dir, prefix string, uid, gid int) (int, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return 0, err
	}
	defer root.Close()
	count := 0
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return count, nil
		}
		if err != nil {
			return count, err
		}
		name, ok := strings.CutPrefix(path.Clean(h.Name), prefix)
		if !ok || name == "" || name == "." || !fs.ValidPath(name) {
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
				if err := chmodChown(root, d, 0o755, uid, gid); err != nil {
					return count, err
				}
			}
			f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
			if err != nil {
				return count, err
			}
			_, err = io.Copy(f, io.LimitReader(tr, maxEntry))
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
}

// ---- running PHP as the site user ----

// siteEnv is the environment for commands run as a site's user.
func (m *Manager) siteEnv(site *store.Site) []string {
	tmp := filepath.Join(m.paths.siteRoot(site.Domain), "tmp")
	return []string{"env", "HOME=" + tmp, "COMPOSER_HOME=" + filepath.Join(tmp, ".composer"),
		"COMPOSER_NO_INTERACTION=1", "TMPDIR=" + tmp, "PATH=/usr/local/bin:/usr/bin:/bin"}
}

// phpArgs builds "runuser -u <user> -- env … php<version> -d … <args>".
func (m *Manager) phpArgs(site *store.Site, memory string, args ...string) ([]string, error) {
	return m.phpArgsINI(site, memory, nil, args...)
}

// phpArgsINI is phpArgs with extra -d settings ("max_input_vars=5000").
func (m *Manager) phpArgsINI(site *store.Site, memory string, ini []string, args ...string) ([]string, error) {
	if validate.SystemUsername(site.SysUser) != nil || validate.PHPVersion(site.PHPVersion) != nil {
		return nil, errors.New("invalid site")
	}
	// umask 027 like the rest of the site: Nginx reads files through the
	// site's group (the panel's own umask is 077). The shell only sets the
	// umask; the command comes as separate arguments.
	full := append([]string{"-u", site.SysUser, "--", "sh", "-c", "umask 027 && exec \"$@\"", "sh"}, m.siteEnv(site)...)
	full = append(full, "php"+site.PHPVersion, "-d", "memory_limit="+memory, "-d", "sendmail_path=/bin/true")
	for _, s := range ini {
		full = append(full, "-d", s)
	}
	return append(full, args...), nil
}

// sitePHP runs PHP as the site's user with stdin, and returns its output.
func (m *Manager) sitePHP(ctx context.Context, site *store.Site, stdin []byte, args ...string) (string, error) {
	full, err := m.phpArgs(site, "512M", args...)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	out, err := m.run.Input(ctx, stdin, "runuser", full...)
	if err != nil {
		return out, fmt.Errorf("%w\n%s", err, run.Tail(out, 15))
	}
	return out, nil
}

// composer runs Composer as the site's user, streaming its output: the one
// "composer" in a shell runs too (software.ComposerPath), the official
// release in /usr/local/bin after an update on the Software page, else the
// distribution's.
func (m *Manager) composer(ctx context.Context, site *store.Site, lg Logger, args ...string) error {
	bin := software.ComposerPath()
	if bin == "" {
		return errors.New("Composer is not installed")
	}
	full, err := m.phpArgs(site, "-1", append([]string{bin, "--no-interaction", "--no-progress"}, args...)...)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Minute)
	defer cancel()
	return m.run.Stream(ctx, lg, "runuser", full...)
}

func (m *Manager) ensureComposer(ctx context.Context, lg Logger) error {
	if software.ComposerPath() != "" {
		return nil
	}
	lg.Logf("Installing Composer")
	return run.Apt(ctx, m.run, lg, "install", "-y", "--no-install-recommends", "composer")
}

// argvScript runs a PHP command-line program with arguments read as JSON
// from stdin, so they never appear in the process list.
const argvScript = `
$in = json_decode(stream_get_contents(STDIN), true);
if (!is_array($in)) { fwrite(STDERR, "bad input\n"); exit(1); }
chdir($in['dir']);
$_SERVER['argv'] = $GLOBALS['argv'] = $in['argv'];
$_SERVER['argc'] = $GLOBALS['argc'] = count($in['argv']);
require $in['script'];
`

func (m *Manager) runArgv(ctx context.Context, site *store.Site, dir, script string, argv []string) (string, error) {
	in, err := json.Marshal(map[string]any{"dir": dir, "script": script, "argv": argv})
	if err != nil {
		return "", err
	}
	return m.sitePHP(ctx, site, in, "-r", argvScript)
}

// ---- Joomla ----

func (m *Manager) installJoomla(ctx context.Context, site *store.Site, req AppRequest, uid, gid int, lg Logger) error {
	public := filepath.Join(m.paths.siteRoot(site.Domain), "public")
	removeIfExists(filepath.Join(public, "index.html"))
	if err := m.fetchApp(ctx, "joomla", "", public, uid, gid, lg); err != nil {
		return err
	}

	lg.Logf("Running the Joomla installer")
	inst := filepath.Join(public, "installation")
	out, err := m.runArgv(ctx, site, inst, filepath.Join(inst, "joomla.php"), []string{
		"joomla.php", "install", "--no-interaction",
		"--site-name=" + strings.TrimSpace(req.Title), "--admin-user=" + req.AdminUser,
		"--admin-username=" + req.AdminUser, "--admin-password=" + req.AdminPassword, "--admin-email=" + req.AdminEmail,
		"--db-type=mysqli", "--db-host=localhost", "--db-user=" + req.DB.Username, "--db-pass=" + req.DB.Password,
		"--db-name=" + req.DB.Name, "--db-prefix=jos_", "--db-encryption=0",
	})
	if err != nil {
		return fmt.Errorf("the Joomla installer failed: %w", err)
	}
	if !fileExists(filepath.Join(public, "configuration.php")) {
		return fmt.Errorf("the Joomla installer did not finish:\n%s", run.Tail(out, 15))
	}
	// The installer must not stay reachable.
	if root, err := os.OpenRoot(public); err == nil {
		root.RemoveAll("installation")
		root.Close()
	}
	lg.Logf("Joomla is installed. Sign in at /administrator/")
	return nil
}

// ---- Nextcloud ----

func (m *Manager) installNextcloud(ctx context.Context, site *store.Site, req AppRequest, uid, gid int, lg Logger) error {
	dir := m.paths.siteRoot(site.Domain)
	public := filepath.Join(dir, "public")
	removeIfExists(filepath.Join(public, "index.html"))
	if err := m.fetchApp(ctx, "nextcloud", "", public, uid, gid, lg); err != nil {
		return err
	}

	// User files live outside the web folder.
	data := filepath.Join(dir, "data")
	if err := mkdir(data, 0o750); err != nil {
		return err
	}
	if err := os.Chown(data, uid, gid); err != nil {
		return err
	}
	lg.Logf("Running the Nextcloud installer")
	out, err := m.runArgv(ctx, site, public, filepath.Join(public, "console.php"), []string{
		"occ", "maintenance:install", "--no-interaction",
		"--database=mysql", "--database-host=localhost", "--database-name=" + req.DB.Name,
		"--database-user=" + req.DB.Username, "--database-pass=" + req.DB.Password,
		"--admin-user=" + req.AdminUser, "--admin-pass=" + req.AdminPassword, "--admin-email=" + req.AdminEmail,
		"--data-dir=" + data,
	})
	if err != nil || !strings.Contains(out, "installed") {
		return fmt.Errorf("the Nextcloud installer failed: %v\n%s", err, run.Tail(out, 15))
	}
	// Before reading --data-dir, Nextcloud logs to its default data folder
	// inside the web folder; the real one is outside it.
	if root, err := os.OpenRoot(public); err == nil {
		root.RemoveAll("data")
		root.Close()
	}
	scheme := "http"
	if site.SSL {
		scheme = "https"
	}
	occ := func(args ...string) error {
		_, err := m.runArgv(ctx, site, public, filepath.Join(public, "console.php"), append([]string{"occ"}, args...))
		return err
	}
	for i, name := range site.Names() {
		if err := occ("config:system:set", "trusted_domains", fmt.Sprint(i+1), "--value="+name); err != nil {
			return err
		}
	}
	if err := occ("config:system:set", "overwrite.cli.url", "--value="+scheme+"://"+site.Domain); err != nil {
		return err
	}
	lg.Logf("Nextcloud is installed")
	return nil
}

// ---- Composer projects (Laravel, Drupal) ----

// moveIntoSite moves a project built in dir into the site folder: its
// public folder's content into the site's public folder, the rest next to it.
func (m *Manager) moveIntoSite(site *store.Site, dir string) error {
	root := m.paths.siteRoot(site.Domain)
	public := filepath.Join(root, "public")
	removeIfExists(filepath.Join(public, "index.html"))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Name() == "public" {
			inner, err := os.ReadDir(filepath.Join(dir, "public"))
			if err != nil {
				return err
			}
			for _, p := range inner {
				if err := os.Rename(filepath.Join(dir, "public", p.Name()), filepath.Join(public, p.Name())); err != nil {
					return err
				}
			}
			continue
		}
		dst := filepath.Join(root, e.Name())
		if _, err := os.Lstat(dst); err == nil {
			return fmt.Errorf("%s already exists", dst)
		}
		if err := os.Rename(filepath.Join(dir, e.Name()), dst); err != nil {
			return err
		}
	}
	return nil
}

// buildDir makes an empty folder in the site, owned by its user, for
// Composer to build a project in.
func (m *Manager) buildDir(site *store.Site, uid, gid int) (string, error) {
	dir := filepath.Join(m.paths.siteRoot(site.Domain), ".panel-build")
	if err := os.RemoveAll(dir); err != nil {
		return "", err
	}
	if err := mkdir(dir, 0o750); err != nil {
		return "", err
	}
	return dir, os.Chown(dir, uid, gid)
}

var envLineRe = regexp.MustCompile(`^#?\s*([A-Z_][A-Z0-9_]*)=`)

// setEnv sets keys in a .env file's text: existing lines (also commented
// out ones) are replaced, missing keys are added. Values are single-quoted;
// callers pass values without quotes.
func setEnv(text string, values map[string]string) string {
	done := map[string]bool{}
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		mm := envLineRe.FindStringSubmatch(l)
		if mm == nil {
			continue
		}
		if v, ok := values[mm[1]]; ok && !done[mm[1]] {
			lines[i] = mm[1] + "='" + v + "'"
			done[mm[1]] = true
		}
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		if !done[k] {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	for _, k := range keys {
		lines = append(lines, k+"='"+values[k]+"'")
	}
	return strings.Join(lines, "\n")
}

func (m *Manager) installLaravel(ctx context.Context, site *store.Site, req AppRequest, uid, gid int, lg Logger) error {
	root := m.paths.siteRoot(site.Domain)
	build, err := m.buildDir(site, uid, gid)
	if err != nil {
		return err
	}
	defer os.RemoveAll(build)
	lg.Logf("Creating a Laravel project with Composer (a few minutes)")
	if err := m.composer(ctx, site, lg, "create-project", "--prefer-dist", "laravel/laravel", build); err != nil {
		return err
	}
	if err := m.moveIntoSite(site, build); err != nil {
		return err
	}
	scheme := "http"
	if site.SSL {
		scheme = "https"
	}
	envPath := filepath.Join(root, ".env")
	b, err := os.ReadFile(envPath)
	if err != nil {
		return err
	}
	driver := "mariadb"
	if req.mysqlEngine() {
		driver = "mysql"
	}
	env := setEnv(string(b), map[string]string{
		"APP_ENV": "production", "APP_DEBUG": "false", "APP_URL": scheme + "://" + site.Domain,
		"DB_CONNECTION": driver, "DB_HOST": "localhost", "DB_PORT": "3306",
		"DB_DATABASE": req.DB.Name, "DB_USERNAME": req.DB.Username, "DB_PASSWORD": req.DB.Password,
	})
	if err := writeOwned(root, ".env", []byte(env), 0o600, uid, gid); err != nil {
		return err
	}
	lg.Logf("Creating the database tables")
	if out, err := m.sitePHP(ctx, site, nil, filepath.Join(root, "artisan"), "migrate", "--force", "--no-interaction"); err != nil {
		return fmt.Errorf("artisan migrate failed: %w\n%s", err, run.Tail(out, 10))
	}
	if _, err := m.sitePHP(ctx, site, nil, filepath.Join(root, "artisan"), "storage:link", "--no-interaction"); err != nil {
		lg.Logf("Warning: %v", err)
	}
	lg.Logf("Laravel is installed in %s; the web folder is %s/public", root, root)
	return nil
}

func (m *Manager) installDrupal(ctx context.Context, site *store.Site, req AppRequest, uid, gid int, lg Logger) error {
	root := m.paths.siteRoot(site.Domain)
	build, err := m.buildDir(site, uid, gid)
	if err != nil {
		return err
	}
	defer os.RemoveAll(build)
	lg.Logf("Creating a Drupal project with Composer (several minutes)")
	if err := m.composer(ctx, site, lg, "create-project", "--no-install", "drupal/recommended-project:^11", build); err != nil {
		return err
	}
	// The site's web folder is public/, not web/.
	cj := filepath.Join(build, "composer.json")
	b, err := os.ReadFile(cj)
	if err != nil {
		return err
	}
	if err := writeOwned(build, "composer.json", []byte(strings.ReplaceAll(string(b), `"web/`, `"public/`)), 0o644, uid, gid); err != nil {
		return err
	}
	if err := m.composer(ctx, site, lg, "install", "--working-dir="+build); err != nil {
		return err
	}
	if err := m.composer(ctx, site, lg, "require", "--working-dir="+build, "drush/drush"); err != nil {
		return err
	}
	if err := m.moveIntoSite(site, build); err != nil {
		return err
	}

	// settings.php with the database, so drush needs no password argument.
	public := filepath.Join(root, "public")
	def, err := os.ReadFile(filepath.Join(public, "sites", "default", "default.settings.php"))
	if err != nil {
		return err
	}
	var hosts []string
	for _, n := range site.Names() {
		hosts = append(hosts, "'^"+strings.ReplaceAll(regexp.QuoteMeta(n), `\*`, `[^.]+`)+"$'")
	}
	settings := string(def) + fmt.Sprintf(`
// Added by %s.
$databases['default']['default'] = [
  'database' => '%s',
  'username' => '%s',
  'password' => '%s',
  'host' => 'localhost',
  'port' => '3306',
  'driver' => 'mysql',
  'prefix' => '',
  'collation' => 'utf8mb4_general_ci',
];
$settings['config_sync_directory'] = '../config/sync';
$settings['trusted_host_patterns'] = [%s];
`, version.AppName, req.DB.Name, req.DB.Username, req.DB.Password, strings.Join(hosts, ", "))
	if err := writeOwned(filepath.Join(public, "sites", "default"), "settings.php", []byte(settings), 0o640, uid, gid); err != nil {
		return err
	}
	// vendor/bin/drush is a shell wrapper; PHP runs drush.php itself.
	drush := filepath.Join(root, "vendor", "drush", "drush", "drush.php")
	lg.Logf("Running the Drupal installer")
	if out, err := m.sitePHP(ctx, site, nil, drush, "--root="+public, "site:install", "standard", "--yes",
		"--site-name="+strings.TrimSpace(req.Title), "--account-name="+req.AdminUser,
		"--account-mail="+req.AdminEmail, "--site-mail="+req.AdminEmail); err != nil {
		return fmt.Errorf("the Drupal installer failed: %w\n%s", err, run.Tail(out, 10))
	}
	// The administrator's password, from stdin.
	in, _ := json.Marshal(map[string]string{"user": req.AdminUser, "password": req.AdminPassword})
	out, err := m.sitePHP(ctx, site, in, drush, "--root="+public, "php:eval",
		`$in = json_decode(stream_get_contents(STDIN), true); $u = user_load_by_name($in['user']); $u->setPassword($in['password']); $u->save(); echo "panel-ok";`)
	if err != nil || !strings.Contains(out, "panel-ok") {
		return fmt.Errorf("could not set the Drupal password: %v\n%s", err, run.Tail(out, 10))
	}
	os.Chmod(filepath.Join(public, "sites", "default", "settings.php"), 0o440)
	lg.Logf("Drupal is installed. Sign in at /user/login")
	return nil
}
