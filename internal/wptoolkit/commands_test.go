package wptoolkit

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/usererr"
)

// The wp-config.php the panel writes: no "That's all, stop editing!" line.
const panelConfig = `<?php
// Written by panel when it installed WordPress.

define( 'DB_NAME', 'wp_example_com' );
define( 'DB_USER', 'wp_example_com' );
define( 'DB_PASSWORD', 'Abc123-secret.pw' );
define( 'DB_HOST', 'localhost' );
define( 'DB_CHARSET', 'utf8mb4' );
define( 'DB_COLLATE', '' );

$table_prefix = 'wp_';

define( 'WP_DEBUG', false );
// PHP runs as the site's own user, so WordPress can update itself directly.
define( 'FS_METHOD', 'direct' );

if ( ! defined( 'ABSPATH' ) ) {
	define( 'ABSPATH', __DIR__ . '/' );
}

require_once ABSPATH . 'wp-settings.php';
`

func TestCommandLine(t *testing.T) {
	k, _ := newKit(t, &fakeRunner{})
	site := newSite(t, k, "example.com", "example_com")
	root := k.root("example.com")
	tmp := filepath.Join(root, "tmp")
	got, err := k.command(site, "027", "core", "version")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-u", "example_com", "--", "sh", "-c", `umask 027 && exec "$@"`, "sh",
		"env", "HOME=" + tmp, "TMPDIR=" + tmp, "PATH=/usr/local/bin:/usr/bin:/bin",
		"WP_CLI_CACHE_DIR=" + filepath.Join(tmp, ".wp-cli", "cache"), "WP_CLI_DISABLE_AUTO_CHECK_UPDATE=1",
		"php8.3", "-d", "memory_limit=512M", "-d", "sendmail_path=/bin/true",
		k.phar, "--path=" + filepath.Join(root, "public"), "--skip-plugins", "--skip-themes", "--skip-packages",
		"core", "version"}
	if !slices.Equal(got, want) {
		t.Fatalf("command:\n%q\nwant:\n%q", got, want)
	}
	for _, bad := range []struct{ user, php, domain string }{
		{"root;id", "8.3", "example.com"}, {"u", "8.3 -r x", "example.com"}, {"u", "8.3", "../etc"},
	} {
		site.SysUser, site.PHPVersion, site.Domain = bad.user, bad.php, bad.domain
		if _, err := k.command(site, "027"); err == nil {
			t.Errorf("accepted %+v", bad)
		}
	}
}

// statusAnswers is a healthy site with updates.
func statusAnswers(c call) (string, error) {
	switch w := c.wpLine(); {
	case w == "core version":
		return "6.8.3\n", nil
	case strings.HasPrefix(w, "config list WP_DEBUG WP_AUTO_UPDATE_CORE --strict"):
		return `[{"name":"WP_DEBUG","value":true,"type":"constant"},{"name":"WP_AUTO_UPDATE_CORE","value":true,"type":"constant"}]`, nil
	case w == "option get home --format=json":
		return `"https:\/\/example.com"`, nil
	case strings.HasPrefix(w, "core check-update"):
		return coreUpdateJSON, nil
	case strings.HasPrefix(w, "plugin list"):
		return pluginListJSON, nil
	case strings.HasPrefix(w, "theme list"):
		return themeListJSON, nil
	}
	return "", cliFail("unexpected command " + c.wpLine())
}

func TestStatus(t *testing.T) {
	r := &fakeRunner{answer: statusAnswers}
	k, _ := newKit(t, r)
	site := newSite(t, k, "example.com", "example_com")
	writeFile(t, filepath.Join(k.public(site), ".maintenance"), maintenanceFile)

	st, err := k.Status(context.Background(), site)
	if err != nil {
		t.Fatal(err)
	}
	if st.Error != "" || st.Version != "6.8.3" || st.CoreUpdate != "6.9" || st.URL != "https://example.com" ||
		!st.Maintenance || !st.Debug || !st.AutoUpdates {
		t.Fatalf("status = %+v", st)
	}
	// Must-use plugins and drop-ins are not counted; "unavailable" is no update.
	if st.Plugins != 3 || st.PluginUpdates != 1 || st.Themes != 2 || st.ThemeUpdates != 1 {
		t.Fatalf("counts = %+v", st)
	}
	want := []Pending{
		{"core", "wordpress", "WordPress", "6.8.3", "6.9"},
		{"plugin", "akismet", "Akismet Anti-spam: Spam Protection", "5.3.3", "5.4"},
		{"theme", "twentytwentyfive", "Twenty Twenty-Five", "1.2", "1.3"},
	}
	if !slices.Equal(st.Updates, want) {
		t.Fatalf("updates = %+v", st.Updates)
	}
	wantLines(t, r.wpLines(), []string{
		"core version",
		"config list WP_DEBUG WP_AUTO_UPDATE_CORE --strict --fields=name,value,type --format=json",
		"option get home --format=json",
		"core check-update --force-check --format=json",
		"plugin list " + extFields + " --format=json",
		"theme list " + extFields + " --format=json",
	})
	for _, c := range r.calls {
		if c.name != "runuser" || c.user() != "example_com" || c.method != "Pipe" {
			t.Fatalf("ran %s %s as %q", c.method, c.name, c.user())
		}
	}
}

func TestStatusProblems(t *testing.T) {
	// The database is down: what could be read is shown, with wp-cli's words.
	r := &fakeRunner{answer: func(c call) (string, error) {
		if c.wpLine() == "option get home --format=json" {
			return "", cliFail("Error establishing a database connection.")
		}
		if strings.HasPrefix(c.wpLine(), "config list") {
			return "", cliFail("No matching entries found in 'wp-config.php'.")
		}
		return statusAnswers(c)
	}}
	k, _ := newKit(t, r)
	site := newSite(t, k, "example.com", "example_com")
	st, err := k.Status(context.Background(), site)
	if err != nil {
		t.Fatal(err)
	}
	if st.Version != "6.8.3" || st.Error != "Error establishing a database connection." || st.Debug || st.Maintenance {
		t.Fatalf("status = %+v", st)
	}
	if n := len(r.wpLines()); n != 3 {
		t.Fatalf("ran %d commands after the failure", n)
	}

	// Endless output is cut off.
	r.answer = func(call) (string, error) { return strings.Repeat("x", maxOutput+1), nil }
	if st, err := k.Status(context.Background(), site); err != nil || !strings.Contains(st.Error, "more output than expected") {
		t.Fatalf("huge output: %+v, %v", st, err)
	}
}

func TestEnsure(t *testing.T) {
	k, base := newKit(t, &fakeRunner{})
	k.phar = filepath.Join(base, "lib", "panel", "wp-cli.phar")
	data := []byte("the real wp-cli")
	sum := sha512.Sum512(data)
	k.sha512 = hex.EncodeToString(sum[:])
	downloads := 0
	k.download = func(context.Context) ([]byte, error) { downloads++; return data, nil }

	if k.Installed() {
		t.Fatal("installed before Ensure")
	}
	for range 2 {
		if err := k.Ensure(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if downloads != 1 || readFile(t, k.phar) != string(data) {
		t.Fatalf("downloads = %d", downloads)
	}

	// A changed file is replaced; a download with the wrong checksum is refused.
	k.installed = false
	writeFile(t, k.phar, "tampered")
	k.download = func(context.Context) ([]byte, error) { return []byte("evil"), nil }
	if err := k.Ensure(context.Background()); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("err = %v", err)
	}
	if readFile(t, k.phar) != "tampered" {
		t.Fatal("a bad download was installed")
	}
}

// fakeMaintenance makes the runner create and remove .maintenance as
// wp-cli does.
func fakeMaintenance(public string) func(c call) (string, error) {
	file := filepath.Join(public, ".maintenance")
	return func(c call) (string, error) {
		switch c.wpLine() {
		case "maintenance-mode activate --force":
			return "", os.WriteFile(file, []byte("<?php $upgrading = 1795602600; ?>"), 0o640)
		case "maintenance-mode deactivate":
			if _, err := os.Stat(file); err != nil {
				return "", cliFail("Maintenance mode already deactivated.")
			}
			return "", os.Remove(file)
		}
		return "", nil
	}
}

func TestMaintenance(t *testing.T) {
	r := &fakeRunner{}
	k, _ := newKit(t, r)
	site := newSite(t, k, "example.com", "example_com")
	r.answer = fakeMaintenance(k.public(site))
	ctx := context.Background()

	if err := k.SetMaintenance(ctx, site, true); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(k.public(site), ".maintenance")); got != maintenanceFile {
		t.Fatalf(".maintenance = %q", got)
	}
	// Still on long after WordPress would have ended wp-cli's 10 minutes.
	k.now = func() time.Time { return time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC) }
	if !k.maintenanceOn(site) {
		t.Fatal("maintenance mode ended")
	}
	if err := k.SetMaintenance(ctx, site, false); err != nil {
		t.Fatal(err)
	}
	if k.maintenanceOn(site) {
		t.Fatal("still on")
	}
	// Off when wp-cli says it is already off: an expired file left behind
	// is removed.
	file := filepath.Join(k.public(site), ".maintenance")
	writeFile(t, file, "<?php $upgrading = 1600000000; ?>")
	r.answer = func(call) (string, error) { return "", cliFail("Maintenance mode already deactivated.") }
	if err := k.SetMaintenance(ctx, site, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("stale file: %v", err)
	}
	wantLines(t, r.wpLines(), []string{"maintenance-mode activate --force", "maintenance-mode deactivate", "maintenance-mode deactivate"})
}

func TestDebugAndAutoUpdates(t *testing.T) {
	r := &fakeRunner{answer: func(c call) (string, error) {
		if strings.Contains(c.wpLine(), "auto-updates") {
			return "", cliFail("No themes provided to enable auto-updates for.")
		}
		return "", nil
	}}
	k, _ := newKit(t, r)
	site := newSite(t, k, "example.com", "example_com")
	writeFile(t, filepath.Join(k.public(site), "wp-config.php"), panelConfig)
	ctx := context.Background()

	if err := k.SetDebug(ctx, site, true); err != nil {
		t.Fatal(err)
	}
	if err := k.SetDebug(ctx, site, false); err != nil {
		t.Fatal(err)
	}
	if err := k.SetAutoUpdates(ctx, site, true); err == nil || !strings.Contains(err.Error(), "No themes provided") {
		// The plugin command fails with a message about themes: a real error.
		t.Fatalf("err = %v", err)
	}
	anchor := "--anchor=require_once ABSPATH . 'wp-settings.php';"
	log := filepath.Join(k.root("example.com"), "logs", "wp-debug.log")
	wantLines(t, r.wpLines(), []string{
		"config set WP_DEBUG true --type=constant --raw " + anchor,
		"config set WP_DEBUG_LOG " + log + " --type=constant " + anchor,
		"config set WP_DEBUG_DISPLAY false --type=constant --raw " + anchor,
		"config set WP_DEBUG false --type=constant --raw " + anchor,
		"config set WP_DEBUG_LOG false --type=constant --raw " + anchor,
		"config set WP_AUTO_UPDATE_CORE true --type=constant --raw " + anchor,
		"plugin auto-updates enable --all --disabled-only",
	})

	// "Nothing to change" is fine.
	r.calls = nil
	r.answer = func(c call) (string, error) {
		if w := c.wpLine(); strings.HasPrefix(w, "plugin auto-updates") {
			return "", cliFail("No plugins provided to disable auto-updates for.")
		} else if strings.HasPrefix(w, "theme auto-updates") {
			return "", cliFail("No themes provided to disable auto-updates for.")
		}
		return "", nil
	}
	if err := k.SetAutoUpdates(ctx, site, false); err != nil {
		t.Fatal(err)
	}
	wantLines(t, r.wpLines(), []string{
		"config set WP_AUTO_UPDATE_CORE minor --type=constant " + anchor,
		"plugin auto-updates disable --all --enabled-only",
		"theme auto-updates disable --all --enabled-only",
	})
}

func TestPickAnchor(t *testing.T) {
	sample := "<?php\n$table_prefix = 'wp_';\n/* That's all, stop editing! Happy publishing. */\nrequire_once ABSPATH . 'wp-settings.php';\n"
	for cfg, want := range map[string]string{
		sample:      "/* That's all, stop editing!",
		panelConfig: "require_once ABSPATH . 'wp-settings.php';",
		"<?php\n$table_prefix = 'x_';\nrequire_once( ABSPATH . 'wp-settings.php' );": "$table_prefix",
	} {
		if got, ok := pickAnchor(cfg); !ok || got != want {
			t.Errorf("pickAnchor = %q, %v; want %q", got, ok, want)
		}
	}
	if _, ok := pickAnchor("<?php\n$table_prefix = 'a';\n// $table_prefix\n"); ok {
		t.Error("an anchor that appears twice must not be used")
	}
}

var b64Re = regexp.MustCompile(`base64_decode\('([A-Za-z0-9+/=]+)'\)`)

func TestResetPassword(t *testing.T) {
	const pw = "Correct horse 'battery' $staple"
	r := &fakeRunner{answer: func(c call) (string, error) { return "panel-wp-ok admin\n", nil }}
	k, _ := newKit(t, r)
	site := newSite(t, k, "example.com", "example_com")

	login, err := k.ResetPassword(context.Background(), site, 1, pw)
	if err != nil || login != "admin" {
		t.Fatalf("login = %q, %v", login, err)
	}
	c := r.calls[0]
	if c.wpLine() != "eval-file -" || c.user() != "example_com" {
		t.Fatalf("ran %q as %q", c.wpLine(), c.user())
	}
	for _, a := range c.args {
		if strings.Contains(a, "horse") {
			t.Fatalf("the password is on the command line: %q", a)
		}
	}
	m := b64Re.FindStringSubmatch(c.stdin)
	if !strings.HasPrefix(c.stdin, "<?php") || m == nil || strings.Contains(c.stdin, "horse") {
		t.Fatalf("stdin = %q", c.stdin)
	}
	raw, _ := base64.StdEncoding.DecodeString(m[1])
	var in struct {
		ID       int64  `json:"id"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(raw, &in); err != nil || in.ID != 1 || in.Password != pw {
		t.Fatalf("input = %+v, %v", in, err)
	}

	r.answer = func(call) (string, error) { return "", cliFail("This user is not an administrator.") }
	var ue *usererr.Error
	if _, err := k.ResetPassword(context.Background(), site, 2, pw); !errors.As(err, &ue) || ue.Code != "wp_not_admin" {
		t.Fatalf("not an admin: %v", err)
	}
	for _, bad := range []string{"short", strings.Repeat("x", 129), "twelve chars\nnewline"} {
		if _, err := k.ResetPassword(context.Background(), site, 1, bad); !errors.As(err, &ue) || ue.Code != "wp_invalid_password" {
			t.Errorf("%q: %v", bad, err)
		}
	}
	if len(NewPassword()) != 20 || CheckPassword(NewPassword()) != nil {
		t.Fatal("NewPassword")
	}
}

func TestUpdate(t *testing.T) {
	r := &fakeRunner{}
	k, _ := newKit(t, r)
	site := newSite(t, k, "example.com", "example_com")
	tmp := k.tmp(site)
	public := k.public(site)
	maint := fakeMaintenance(public)
	var exportUmask string
	r.answer = func(c call) (string, error) {
		switch w := c.wp(); {
		case len(w) == 3 && w[0] == "db" && w[1] == "export":
			exportUmask = c.args[5]
			return "Success: Exported to '" + w[2] + "'.\n", os.WriteFile(w[2], []byte("-- dump"), 0o644)
		case c.wpLine() == "core update":
			// WordPress ends maintenance mode after an update.
			os.Remove(filepath.Join(public, ".maintenance"))
			return "Success: WordPress updated successfully.\n", nil
		case c.wpLine() == "plugin update --all":
			return "Error: Only updated 1 of 2 plugins.\n", cliFail("Only updated 1 of 2 plugins.")
		}
		return maint(c)
	}
	for _, old := range []string{"20260101-000000", "20260102-000000", "20260103-000000", "20260104-000000"} {
		writeFile(t, filepath.Join(tmp, "panel-wp-backup-"+old+".sql"), "old")
	}
	writeFile(t, filepath.Join(tmp, "keep-me.sql"), "")
	writeFile(t, filepath.Join(public, ".maintenance"), maintenanceFile)

	lg := &testLog{}
	err := k.Update(context.Background(), site, lg)
	if err == nil || !strings.Contains(err.Error(), "some plugins could not be updated") {
		t.Fatalf("err = %v", err)
	}
	backup := filepath.Join(tmp, "panel-wp-backup-20261125-103000.sql")
	wantLines(t, r.wpLines(), []string{
		"db export " + backup, "core update", "core update-db", "plugin update --all", "theme update --all",
		"maintenance-mode activate --force",
	})
	if exportUmask != "umask 077 && exec \"$@\"" {
		t.Fatalf("db export ran with %q", exportUmask)
	}
	entries, _ := os.ReadDir(tmp)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	want := []string{"keep-me.sql", "panel-wp-backup-20260103-000000.sql", "panel-wp-backup-20260104-000000.sql", "panel-wp-backup-20261125-103000.sql"}
	if !slices.Equal(names, want) {
		t.Fatalf("tmp = %v", names)
	}
	if !k.maintenanceOn(site) {
		t.Fatal("maintenance mode was not turned back on")
	}

	// No backup, no update; the failed export's file does not stay.
	r.calls = nil
	r.answer = func(c call) (string, error) {
		os.WriteFile(c.wp()[2], nil, 0o600)
		return "", cliFail("Failed to get current SQL modes.")
	}
	k.now = func() time.Time { return time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC) }
	if err := k.Update(context.Background(), site, lg); err == nil || !strings.Contains(err.Error(), "nothing was updated") {
		t.Fatalf("err = %v", err)
	}
	if n := len(r.wpLines()); n != 1 {
		t.Fatalf("%d commands ran after a failed backup", n)
	}
	if _, err := os.Stat(filepath.Join(tmp, "panel-wp-backup-20261126-000000.sql")); !os.IsNotExist(err) {
		t.Fatalf("failed backup left: %v", err)
	}
}
