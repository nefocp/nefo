package wptoolkit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/nefocp/nefo/internal/secure"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// ---- maintenance mode ----

// SetMaintenance turns WordPress's maintenance page (503) on or off.
func (t *Toolkit) SetMaintenance(ctx context.Context, site *store.Site, on bool) error {
	if err := t.Ensure(ctx); err != nil {
		return err
	}
	if !on {
		_, err := t.wp(ctx, site, nil, "maintenance-mode", "deactivate")
		if hasMessage(err, "already deactivated") {
			// An expired .maintenance file can be left behind.
			return t.removeMaintenance(site)
		}
		return err
	}
	if _, err := t.wp(ctx, site, nil, "maintenance-mode", "activate", "--force"); err != nil {
		return err
	}
	return t.pinMaintenance(site)
}

// pinMaintenance makes the .maintenance file wp-cli wrote last until it is
// removed (see maintenanceFile). wp-cli created the file as the site's
// user, so it keeps that owner.
func (t *Toolkit) pinMaintenance(site *store.Site) error {
	root, err := os.OpenRoot(t.public(site))
	if err != nil {
		return err
	}
	defer root.Close()
	fi, err := root.Lstat(".maintenance")
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		return errors.New(".maintenance is not a regular file")
	}
	return root.WriteFile(".maintenance", []byte(maintenanceFile), 0o640)
}

func (t *Toolkit) removeMaintenance(site *store.Site) error {
	root, err := os.OpenRoot(t.public(site))
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.Remove(".maintenance"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// ---- wp-config.php constants ----

// anchors are places in wp-config.php before which "wp config set" may add
// a constant, in order of preference. wp-cli only knows the first, which the
// wp-config.php the panel writes does not have. Each must appear once.
var anchors = []string{
	"/* That's all, stop editing!",
	"require_once ABSPATH . 'wp-settings.php';",
	"$table_prefix",
}

func pickAnchor(cfg string) (string, bool) {
	for _, a := range anchors {
		if strings.Count(cfg, a) == 1 {
			return a, true
		}
	}
	return "", false
}

// readConfig reads wp-config.php, which WordPress also finds one folder
// above public/.
func (t *Toolkit) readConfig(site *store.Site) (string, error) {
	b, err := readSiteFile(t.public(site), "wp-config.php", 1<<20)
	if errors.Is(err, os.ErrNotExist) {
		b, err = readSiteFile(t.root(site.Domain), "wp-config.php", 1<<20)
	}
	if errors.Is(err, os.ErrNotExist) {
		return "", usererr.New("wp_config_missing", site.Domain)
	}
	return string(b), err
}

// setConstants runs "wp config set" for each constant. Values are not
// secrets. raw values are PHP code (true, false); the others are strings.
func (t *Toolkit) setConstants(ctx context.Context, site *store.Site, values []constant) error {
	// Without a readable wp-config.php, wp-cli's own default anchor is tried.
	anchor, ok := "", false
	if cfg, err := t.readConfig(site); err == nil {
		anchor, ok = pickAnchor(cfg)
	}
	for _, c := range values {
		args := []string{"config", "set", c.name, c.value, "--type=constant"}
		if c.raw {
			args = append(args, "--raw")
		}
		if ok {
			args = append(args, "--anchor="+anchor)
		}
		if _, err := t.wp(ctx, site, nil, args...); err != nil {
			return err
		}
	}
	return nil
}

type constant struct {
	name, value string
	raw         bool
}

// DebugLog is where WordPress writes PHP errors from wp-cli and cron runs
// while debug mode is on: in the site's logs folder, which is not served.
// Errors of web requests go to logs/php-error.log, which the PHP-FPM pool
// sets and WordPress cannot change.
func (t *Toolkit) DebugLog(site *store.Site) string {
	return filepath.Join(t.root(site.Domain), "logs", "wp-debug.log")
}

// SetDebug turns WP_DEBUG and WP_DEBUG_LOG on or off. Errors are logged,
// never shown to visitors.
func (t *Toolkit) SetDebug(ctx context.Context, site *store.Site, on bool) error {
	if err := t.Ensure(ctx); err != nil {
		return err
	}
	if on {
		return t.setConstants(ctx, site, []constant{
			{"WP_DEBUG", "true", true},
			{"WP_DEBUG_LOG", t.DebugLog(site), false},
			{"WP_DEBUG_DISPLAY", "false", true},
		})
	}
	return t.setConstants(ctx, site, []constant{
		{"WP_DEBUG", "false", true},
		{"WP_DEBUG_LOG", "false", true},
	})
}

// SetAutoUpdates turns WordPress's automatic updates on for everything
// (core including major versions, every plugin and theme), or back to
// WordPress's default: security and minor releases of core only.
func (t *Toolkit) SetAutoUpdates(ctx context.Context, site *store.Site, on bool) error {
	if err := t.Ensure(ctx); err != nil {
		return err
	}
	core := constant{"WP_AUTO_UPDATE_CORE", "minor", false}
	verb, only := "disable", "--enabled-only"
	if on {
		core = constant{"WP_AUTO_UPDATE_CORE", "true", true}
		verb, only = "enable", "--disabled-only"
	}
	if err := t.setConstants(ctx, site, []constant{core}); err != nil {
		return err
	}
	for _, kind := range []string{"plugin", "theme"} {
		_, err := t.wp(ctx, site, nil, kind, "auto-updates", verb, "--all", only)
		// Nothing to change, or no plugins at all.
		if err != nil && !hasMessage(err, "No "+kind+"s provided") {
			return err
		}
	}
	return nil
}

// ---- administrators ----

// Admins lists the users with the administrator role.
func (t *Toolkit) Admins(ctx context.Context, site *store.Site) ([]User, error) {
	if err := t.Ensure(ctx); err != nil {
		return nil, err
	}
	out, err := t.wp(ctx, site, nil, "user", "list", "--role=administrator",
		"--fields=ID,user_login,user_email,display_name", "--format=json")
	if err != nil {
		return nil, err
	}
	return parseUsers(out)
}

// NewPassword returns a strong random password.
func NewPassword() string { return secure.String(20, secure.PasswordChars) }

// CheckPassword validates a password typed for a WordPress account.
func CheckPassword(pw string) error {
	if n := utf8.RuneCountInString(pw); n < 12 || n > 128 || !utf8.ValidString(pw) ||
		strings.IndexFunc(pw, unicode.IsControl) >= 0 {
		return usererr.New("wp_invalid_password")
	}
	return nil
}

// resetScript sets an administrator's password and signs the account out
// everywhere. It reaches PHP on stdin, with its input as base64 (only
// letters, digits, + / and =, so it cannot end the PHP string).
const resetScript = `<?php
$in = json_decode(base64_decode('%s'), true);
$u = get_user_by('id', (int) $in['id']);
if (!$u || !in_array('administrator', (array) $u->roles, true)) {
	WP_CLI::error('This user is not an administrator.');
}
wp_set_password($in['password'], $u->ID);
if (class_exists('WP_Session_Tokens')) {
	WP_Session_Tokens::get_instance($u->ID)->destroy_all();
}
echo 'panel-wp-ok ' . $u->user_login . "\n";
`

var okLineRe = regexp.MustCompile(`(?m)^panel-wp-ok (.+)$`)

// ResetPassword sets the password of an administrator and returns the
// account's login name. The password goes to PHP on stdin.
func (t *Toolkit) ResetPassword(ctx context.Context, site *store.Site, userID int64, password string) (string, error) {
	if err := CheckPassword(password); err != nil {
		return "", err
	}
	if userID <= 0 {
		return "", usererr.New("wp_not_admin")
	}
	if err := t.Ensure(ctx); err != nil {
		return "", err
	}
	in, err := json.Marshal(map[string]any{"id": userID, "password": password})
	if err != nil {
		return "", err
	}
	code := fmt.Sprintf(resetScript, base64.StdEncoding.EncodeToString(in))
	out, err := t.wp(ctx, site, []byte(code), "eval-file", "-")
	if hasMessage(err, "not an administrator") {
		return "", usererr.New("wp_not_admin")
	}
	if err != nil {
		return "", err
	}
	m := okLineRe.FindStringSubmatch(out)
	if m == nil {
		return "", errors.New("wp eval-file: the password was not changed")
	}
	return clean(m[1]), nil
}

// ---- updates and backups ----

var backupRe = regexp.MustCompile(`^panel-wp-backup-[0-9]{8}-[0-9]{6}\.sql$`)

// Backup saves the site's database with "wp db export" into the site's tmp
// folder (not served), readable only by the site's user, and keeps the
// newest three of these files.
func (t *Toolkit) Backup(ctx context.Context, site *store.Site, lg Logger) (string, error) {
	if err := checkSite(site); err != nil {
		return "", err
	}
	name := "panel-wp-backup-" + t.now().UTC().Format("20060102-150405") + ".sql"
	file := filepath.Join(t.tmp(site), name)
	root, err := os.OpenRoot(t.tmp(site))
	if err != nil {
		return "", err
	}
	defer root.Close()
	// umask 077: the dump holds password hashes and private data.
	if err := t.stream(ctx, site, lg, "077", "db", "export", file); err != nil {
		root.Remove(name) // a failed export leaves an empty or partial file
		return "", err
	}
	if err := root.Chmod(name, 0o600); err != nil {
		return "", err
	}
	t.pruneBackups(root, lg)
	return file, nil
}

func (t *Toolkit) pruneBackups(root *os.Root, lg Logger) {
	f, err := root.Open(".")
	if err != nil {
		return
	}
	entries, err := f.ReadDir(-1)
	f.Close()
	if err != nil {
		return
	}
	var names []string
	for _, e := range entries {
		if e.Type().IsRegular() && backupRe.MatchString(e.Name()) {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names) // the names sort by time
	for len(names) > keepBackups {
		if err := root.Remove(names[0]); err != nil {
			lg.Logf("Warning: could not remove the old backup %s: %v", names[0], err)
		}
		names = names[1:]
	}
}

// Update backs up the database, then updates WordPress, its database
// schema, every plugin and every theme. Nothing is updated if the backup
// fails. A site that was in maintenance mode stays in it (WordPress ends
// maintenance mode after each update).
func (t *Toolkit) Update(ctx context.Context, site *store.Site, lg Logger) error {
	if err := checkSite(site); err != nil {
		return err
	}
	if err := t.Ensure(ctx); err != nil {
		return err
	}
	wasMaintenance := t.maintenanceOn(site)

	lg.Logf("Backing up the database")
	file, err := t.Backup(ctx, site, lg)
	if err != nil {
		return fmt.Errorf("the database backup failed, so nothing was updated: %w", err)
	}
	lg.Logf("Database saved to %s", file)

	defer func() {
		if wasMaintenance && !t.maintenanceOn(site) {
			if err := t.SetMaintenance(context.WithoutCancel(ctx), site, true); err != nil {
				lg.Logf("Warning: could not turn maintenance mode back on: %v", err)
			}
		}
	}()

	var problems []string
	if min := coreMinPHP(); compareVersions(site.PHPVersion, min) < 0 {
		// An old PHP version (a website moved from elsewhere): the newest
		// WordPress would not run on it. Plugins and themes are fine, wp-cli
		// skips their updates that need a newer PHP.
		lg.Logf("Not updating WordPress itself: it needs PHP %s or newer, and this website runs PHP %s. Choose a newer PHP version in the website's settings first.", min, site.PHPVersion)
		problems = append(problems, fmt.Sprintf("WordPress itself was not updated: it needs PHP %s or newer, and this website runs PHP %s", min, site.PHPVersion))
	} else {
		lg.Logf("Updating WordPress")
		if err := t.stream(ctx, site, lg, "027", "core", "update"); err != nil {
			return err
		}
		if err := t.stream(ctx, site, lg, "027", "core", "update-db"); err != nil {
			return err
		}
	}
	var failed []string
	for _, kind := range []string{"plugin", "theme"} {
		lg.Logf("Updating %ss", kind)
		if err := t.stream(ctx, site, lg, "027", kind, "update", "--all"); err != nil {
			lg.Logf("Warning: %v", err)
			failed = append(failed, kind+"s")
		}
	}
	if len(failed) > 0 {
		problems = append(problems, fmt.Sprintf("some %s could not be updated; the log above says why", strings.Join(failed, " and ")))
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s. The database backup is %s", strings.Join(problems, "; "), file)
	}
	lg.Logf("WordPress, plugins and themes are up to date")
	return nil
}
