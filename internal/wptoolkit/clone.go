package wptoolkit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
)

// CloneHost is what a staging copy needs from the rest of the panel; the
// API fills it with the hosting and database managers.
type CloneHost struct {
	// CreateSite creates a PHP website; DeleteSite removes it with its files.
	CreateSite func(ctx context.Context, domain, phpVersion string) (*store.Site, error)
	DeleteSite func(ctx context.Context, site *store.Site) error
	// CreateDB creates an empty database linked to the site.
	CreateDB func(ctx context.Context, site *store.Site) (*store.Database, error)
	DeleteDB func(ctx context.Context, db *store.Database) error
	// CopyDB copies the tables and data of one database into another.
	CopyDB func(ctx context.Context, from, to string) error
	// MarkWordPress records the new site as a WordPress site, with the same
	// rewrite rules as the live one.
	MarkWordPress func(ctx context.Context, from, to *store.Site, lg Logger) error
}

// CloneSource is what PrepareClone read from the live site's wp-config.php.
type CloneSource struct {
	DBName string
}

// leftOut are files of the live site the copy does without: cache drop-ins
// would share the live site's object cache (and read its cached options) or
// load page-cache code configured for the live site's paths.
var leftOut = []string{"wp-content/object-cache.php", "wp-content/advanced-cache.php"}

// PrepareClone checks that a site can be copied: its wp-config.php is in
// public/ (only public/ is copied), names a local database the panel can
// copy, and is not a multisite network.
func (t *Toolkit) PrepareClone(site *store.Site) (*CloneSource, error) {
	if err := checkSite(site); err != nil {
		return nil, err
	}
	b, err := readSiteFile(t.public(site), "wp-config.php", 1<<20)
	if errors.Is(err, os.ErrNotExist) {
		return nil, usererr.New("wp_config_missing", site.Domain)
	}
	if err != nil {
		return nil, err
	}
	return parseCloneSource(string(b))
}

var multisiteRe = regexp.MustCompile(`(?i:define)\s*\(\s*['"]MULTISITE['"]\s*,\s*true\s*\)`)

func parseCloneSource(cfg string) (*CloneSource, error) {
	if multisiteRe.MatchString(cfg) {
		return nil, usererr.New("wp_multisite")
	}
	name, ok := defineValue(cfg, "DB_NAME")
	if !ok {
		return nil, usererr.New("wp_config_unreadable", "DB_NAME")
	}
	if validate.DBName(name) != nil {
		return nil, usererr.New("wp_db_unsupported", name)
	}
	if host, ok := defineValue(cfg, "DB_HOST"); ok && !localDBHost(host) {
		return nil, usererr.New("wp_remote_db", host)
	}
	return &CloneSource{DBName: name}, nil
}

// localDBHost tells whether DB_HOST is this server's MariaDB.
func localDBHost(h string) bool {
	host, rest, _ := strings.Cut(strings.ToLower(strings.TrimSpace(h)), ":")
	if host != "localhost" && host != "127.0.0.1" {
		return false
	}
	return rest == "" || rest == "3306" || strings.HasPrefix(rest, "/")
}

// ---- wp-config.php ----

// defineRe matches define( 'NAME', 'value' ); with either kind of quotes.
func defineRe(name string) *regexp.Regexp {
	n := regexp.QuoteMeta(name)
	return regexp.MustCompile(`(?i:define)\s*\(\s*(?:'` + n + `'|"` + n + `")\s*,\s*(?:'((?:[^'\\]|\\.)*)'|"((?:[^"\\]|\\.)*)")\s*\)\s*;`)
}

// defineRawRe matches define( 'NAME', <anything but a parenthesis> );
func defineRawRe(name string) *regexp.Regexp {
	n := regexp.QuoteMeta(name)
	return regexp.MustCompile(`(?i:define)\s*\(\s*(?:'` + n + `'|"` + n + `")\s*,\s*[^();]*\)\s*;`)
}

// commented tells whether the match at i is on a line that is commented
// out. Block comments over several lines are not recognised.
func commented(src string, i int) bool {
	line := src[strings.LastIndexByte(src[:i], '\n')+1 : i]
	l := strings.TrimSpace(line)
	return strings.Contains(line, "//") || strings.HasPrefix(l, "#") || strings.HasPrefix(l, "*") || strings.HasPrefix(l, "/*")
}

// defineValue returns the value of the first active define of a constant
// with a plain string value.
func defineValue(cfg, name string) (string, bool) {
	for _, m := range defineRe(name).FindAllStringSubmatchIndex(cfg, -1) {
		if commented(cfg, m[0]) {
			continue
		}
		if m[2] >= 0 {
			return strings.NewReplacer(`\'`, `'`, `\\`, `\`).Replace(cfg[m[2]:m[3]]), true
		}
		return cfg[m[4]:m[5]], true
	}
	return "", false
}

// setDefine replaces the active defines of a constant with a string value.
// value must not contain quotes or backslashes.
func setDefine(cfg, name, value string, required bool) (string, error) {
	return replaceDefines(cfg, defineRe(name), name, "'"+value+"'", required)
}

func replaceDefines(cfg string, re *regexp.Regexp, name, php string, required bool) (string, error) {
	var b strings.Builder
	last, n := 0, 0
	for _, m := range re.FindAllStringIndex(cfg, -1) {
		if commented(cfg, m[0]) {
			continue
		}
		b.WriteString(cfg[last:m[0]])
		b.WriteString("define( '" + name + "', " + php + " );")
		last = m[1]
		n++
	}
	if n == 0 {
		if required {
			return "", usererr.New("wp_config_unreadable", name)
		}
		return cfg, nil
	}
	b.WriteString(cfg[last:])
	return b.String(), nil
}

// rewriteConfig points a copied wp-config.php at the staging site: the
// replacements (old, new pairs: the live site's folder and addresses) are
// made first, then the database settings are written, so a replacement can
// never touch the new password. HTTPS for the dashboard is not forced: the
// staging site starts without a certificate.
func rewriteConfig(cfg string, db *store.Database, replacements []string) (string, error) {
	if db == nil || validate.DBName(db.Name) != nil || validate.DBName(db.Username) != nil || validate.DBPassword(db.Password) != nil {
		return "", errors.New("invalid database settings")
	}
	cfg = strings.NewReplacer(replacements...).Replace(cfg)
	var err error
	for _, d := range []struct {
		name, value string
		required    bool
	}{
		{"DB_NAME", db.Name, true}, {"DB_USER", db.Username, true}, {"DB_PASSWORD", db.Password, true},
		{"DB_HOST", "localhost", false},
	} {
		if cfg, err = setDefine(cfg, d.name, d.value, d.required); err != nil {
			return "", err
		}
	}
	return replaceDefines(cfg, defineRawRe("FORCE_SSL_ADMIN"), "FORCE_SSL_ADMIN", "false", false)
}

// ---- the clone ----

// origin returns scheme://host of an address.
func origin(addr string) string {
	u, err := url.Parse(addr)
	if err != nil {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// siteOrigins returns the addresses (scheme://host) of the site's home and
// siteurl options, which the copy replaces.
func (t *Toolkit) siteOrigins(ctx context.Context, site *store.Site) ([]string, error) {
	var origins []string
	for _, opt := range []string{"home", "siteurl"} {
		out, err := t.wp(ctx, site, nil, "option", "get", opt, "--format=json")
		if err != nil {
			return nil, err
		}
		u := parseURL(out)
		if u == "" {
			return nil, fmt.Errorf("the site's %s address is not an http or https address", opt)
		}
		if o := origin(u); !slices.Contains(origins, o) {
			origins = append(origins, o)
		}
	}
	return origins, nil
}

// Clone copies a WordPress site to a new website, normally
// staging.<domain>: a new PHP site with the same PHP version, a copy of
// public/ owned by the new site's user, a copy of the database, wp-config.php
// pointed at it and the site's address replaced everywhere. Search engines
// are asked not to index the copy. If anything fails, what was created is
// removed again.
func (t *Toolkit) Clone(ctx context.Context, from *store.Site, domain string, host CloneHost, lg Logger) (to *store.Site, err error) {
	if err := checkSite(from); err != nil {
		return nil, err
	}
	if err := t.Ensure(ctx); err != nil {
		return nil, err
	}
	src, err := t.PrepareClone(from)
	if err != nil {
		return nil, err
	}
	origins, err := t.siteOrigins(ctx, from)
	if err != nil {
		return nil, err
	}
	srcUID, _, err := t.lookupIDs(from.SysUser)
	if err != nil {
		return nil, err
	}

	bg := context.WithoutCancel(ctx)
	var undo []func()
	defer func() {
		if err == nil {
			return
		}
		lg.Logf("Removing what was created for the staging site")
		for i := len(undo) - 1; i >= 0; i-- {
			undo[i]()
		}
		to = nil
	}()

	lg.Logf("Creating the website %s with PHP %s", domain, from.PHPVersion)
	site, err := host.CreateSite(ctx, domain, from.PHPVersion)
	if err != nil {
		return nil, err
	}
	undo = append(undo, func() {
		if err := host.DeleteSite(bg, site); err != nil {
			lg.Logf("Warning: could not remove the website %s: %v", site.Domain, err)
		}
	})
	if err = checkSite(site); err != nil {
		return nil, err
	}

	// The database first: the copied wp-config.php must never point at the
	// live database, even for a moment.
	lg.Logf("Creating a database")
	db, err := host.CreateDB(ctx, site)
	if err != nil {
		return nil, err
	}
	undo = append(undo, func() {
		if err := host.DeleteDB(bg, db); err != nil {
			lg.Logf("Warning: could not remove the database %s: %v", db.Name, err)
		}
	})
	lg.Logf("Copying the database %s into %s", src.DBName, db.Name)
	if err = host.CopyDB(ctx, src.DBName, db.Name); err != nil {
		return nil, err
	}

	newOrigin := "http://" + site.Domain
	oldRoot, newRoot := t.root(from.Domain), t.root(site.Domain)
	replacements := []string{oldRoot + "/", newRoot + "/"}
	for _, o := range origins {
		replacements = append(replacements, o, newOrigin)
	}
	pathsOnly := func(b []byte) ([]byte, error) {
		return bytes.ReplaceAll(b, []byte(oldRoot+"/"), []byte(newRoot+"/")), nil
	}
	lg.Logf("Copying the files")
	uid, gid, err := t.lookupIDs(site.SysUser)
	if err != nil {
		return nil, err
	}
	removePlaceholder(t.public(site))
	stats, err := fsutil.CopyTree(ctx, t.public(from), t.public(site), fsutil.CopyOptions{
		UID: uid, GID: gid, SrcUID: srcUID, Skip: leftOut, Log: lg,
		Transform: map[string]func([]byte) ([]byte, error){
			"wp-config.php": func(b []byte) ([]byte, error) {
				s, err := rewriteConfig(string(b), db, replacements)
				return []byte(s), err
			},
			// Paths of the live site, such as Wordfence's firewall.
			".user.ini": pathsOnly, "wordfence-waf.php": pathsOnly,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("copy the files: %w", err)
	}
	lg.Logf("Copied %d files (%d MB)", stats.Files, stats.Bytes>>20)
	logLeftOut(lg, stats.Skipped)

	for _, o := range origins {
		for _, pair := range [][2]string{
			{o, newOrigin},
			// The same address in JSON, as page builders save it.
			{strings.ReplaceAll(o, "/", `\/`), strings.ReplaceAll(newOrigin, "/", `\/`)},
		} {
			lg.Logf("Replacing %s with %s in the database", pair[0], pair[1])
			if err = t.stream(ctx, site, lg, "027", "search-replace", pair[0], pair[1],
				"--all-tables", "--precise", "--report-changed-only"); err != nil {
				return nil, err
			}
		}
	}
	lg.Logf("Asking search engines not to index the staging site")
	if _, err = t.wp(ctx, site, nil, "option", "update", "blog_public", "0"); err != nil {
		return nil, err
	}
	if err = host.MarkWordPress(ctx, from, site, lg); err != nil {
		return nil, err
	}
	lg.Logf("The staging site is ready at %s/ once its domain points at this server", newOrigin)
	return site, nil
}

// logLeftOut lists the first of the items a copy left out.
func logLeftOut(lg Logger, skipped []string) {
	if len(skipped) > 0 {
		lg.Logf("Left out %d items (cache drop-ins, links and files the site cannot read): %s",
			len(skipped), strings.Join(skipped[:min(10, len(skipped))], ", "))
	}
}

// removePlaceholder removes the welcome page of a new site, so it does not
// stay next to the copied files.
func removePlaceholder(public string) {
	if root, err := os.OpenRoot(public); err == nil {
		root.Remove("index.html")
		root.Close()
	}
}
