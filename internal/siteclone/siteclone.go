// Package siteclone copies a website to a new domain: a new website of the
// same type with the same settings, a copy of its folder owned by the new
// website's user and, if asked, a copy of each of its databases (MariaDB,
// MySQL or PostgreSQL). It copies PHP and static websites, reverse proxy
// websites and app websites (Node.js, Python or any other runtime the
// panel runs apps with): an app's copy gets a port of its own and the same
// runtime and start command, and its background programs are copied too.
//
// WordPress sites are copied by package wptoolkit instead, which also
// points WordPress at the new database and address, and Docker apps by
// hosting.CloneDockerApp, which copies the app's data.
package siteclone

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
)

// Logger receives progress messages for a job log.
type Logger interface {
	Logf(format string, args ...any)
}

// Host is what a clone needs from the rest of the panel; the API fills it
// with the hosting and database managers.
type Host struct {
	// Root returns a site's folder, as hosting.Root does.
	Root func(domain string) string
	// LookupIDs returns the uid and gid of a Linux account.
	LookupIDs func(name string) (uid, gid int, err error)
	// CreateSite creates a website of the same type as from (the same PHP
	// version, static, the same reverse proxy address, or an app website
	// on a free port of its own, without its service yet); DeleteSite
	// removes it with its files and background programs.
	CreateSite func(ctx context.Context, from *store.Site, domain string) (*store.Site, error)
	DeleteSite func(ctx context.Context, site *store.Site) error
	// CopySettings gives the new site the app, PHP settings and rewrite
	// preset of from, and the directives given (from's own, with its folder
	// replaced by the new one). Its errors are warnings: the copy works
	// without them.
	CopySettings func(ctx context.Context, from, to *store.Site, directives string, lg Logger) error
	// StartApp gives an app website's copy the runtime of from and its
	// start command (command, with from's folder replaced by the new one),
	// which starts the copy's service. Nil for sites without apps.
	StartApp func(ctx context.Context, from, to *store.Site, command string, lg Logger) error
	// CopyDaemons copies the background programs of from to the copy,
	// stopped, with paths to from's folder replaced. It returns how many.
	// Nil copies none.
	CopyDaemons func(ctx context.Context, from, to *store.Site, replace func(string) string) (int, error)
	// CreateDB creates an empty database of the same engine as from,
	// linked to site, for a copy of from.
	CreateDB func(ctx context.Context, site *store.Site, from *store.Database) (*store.Database, error)
	DeleteDB func(ctx context.Context, db *store.Database) error
	// CopyDB copies the tables and data of one database into another of
	// the same engine.
	CopyDB func(ctx context.Context, from, to *store.Database) error
}

// Copied is a database and its copy.
type Copied struct {
	From string
	To   *store.Database
}

// Result is the new website and the copies of its databases.
type Result struct {
	Site      *store.Site
	Databases []Copied
	Daemons   int
}

// LeftOut are the folders of a website that a clone does not copy: logs,
// temporary files and sessions, the Git deploy key (Git deploy settings are
// not copied) and the panel's build folder.
var LeftOut = []string{"logs", "tmp", ".ssh", ".panel-build"}

// Relocated are folders of an app whose files name the app's own folder
// (absolute paths written when they were made): a Python virtual
// environment has it in its scripts' first line ("#!/var/www/…/python"),
// pyvenv.cfg and its .pth files. In the copy, their text files name the
// copy's folder. A runtime whose dependencies record their folder like
// that adds it here.
var Relocated = []string{"app/.venv"}

// IsApp reports whether a site runs a program of its own (an app website,
// not a reverse proxy to an address or a Docker app).
func IsApp(site *store.Site) bool {
	return site.ProxyPort != 0 && site.ProxyURL == "" && !IsDocker(site)
}

// IsDocker reports whether a site is a Docker app from the app store.
func IsDocker(site *store.Site) bool { return strings.HasPrefix(site.App, "docker:") }

// Check refuses a website that this package cannot clone: a Docker app
// (hosting.CloneDockerApp copies those) or an invalid site.
func Check(site *store.Site) error {
	if site == nil || validate.Domain(site.Domain) != nil || validate.SystemUsername(site.SysUser) != nil {
		return errors.New("invalid site")
	}
	if IsDocker(site) {
		return usererr.New("clone_app_site", site.Domain)
	}
	if site.PHPVersion != "" && validate.PHPVersion(site.PHPVersion) != nil {
		return errors.New("invalid site")
	}
	return nil
}

// ReplaceRoot replaces a website's folder with another one in s, where it
// is a whole path: /var/www/example.com/x, but not /var/www/example.com.au.
func ReplaceRoot(s, oldRoot, newRoot string) string {
	var b strings.Builder
	for {
		i := strings.Index(s, oldRoot)
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		end := i + len(oldRoot)
		b.WriteString(s[:i])
		if end < len(s) && isNameByte(s[end]) {
			b.WriteString(oldRoot)
		} else {
			b.WriteString(newRoot)
		}
		s = s[end:]
	}
}

func isNameByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '-' || c == '_'
}

// RuntimeName names an app's runtime for the log: "" is the server's
// Node.js, "node:22" Node.js 22, "python" Python; others as they are.
func RuntimeName(rt string) string {
	switch {
	case rt == "":
		return "Node.js"
	case rt == "python":
		return "Python"
	case strings.HasPrefix(rt, "node:"):
		return "Node.js " + strings.TrimPrefix(rt, "node:")
	}
	name, ver, _ := strings.Cut(rt, ":")
	if name == "" {
		return rt
	}
	return strings.TrimSpace(strings.ToUpper(name[:1]) + name[1:] + " " + ver)
}

func kindOf(site *store.Site) string {
	switch {
	case site.ProxyURL != "":
		return "a reverse proxy website"
	case IsApp(site):
		return "a " + RuntimeName(site.AppRuntime) + " app website"
	case site.PHPVersion != "":
		return "a PHP " + site.PHPVersion + " website"
	}
	return "a static website"
}

// Clone copies a website to a new one at domain, with copies of the
// databases dbs. If anything fails, what was created is removed again.
func Clone(ctx context.Context, from *store.Site, domain string, dbs []*store.Database, h Host, lg Logger) (res *Result, err error) {
	if err := Check(from); err != nil {
		return nil, err
	}
	for _, d := range dbs {
		if validate.DBName(d.Name) != nil {
			return nil, usererr.New("invalid_db_name", d.Name)
		}
	}
	srcUID, _, err := h.LookupIDs(from.SysUser)
	if err != nil {
		return nil, err
	}

	bg := context.WithoutCancel(ctx)
	var undo []func()
	defer func() {
		if err == nil {
			return
		}
		lg.Logf("Removing what was created for the copy")
		for i := len(undo) - 1; i >= 0; i-- {
			undo[i]()
		}
		res = nil
	}()

	lg.Logf("Creating %s for %s", kindOf(from), domain)
	site, err := h.CreateSite(ctx, from, domain)
	if err != nil {
		return nil, err
	}
	undo = append(undo, func() {
		if err := h.DeleteSite(bg, site); err != nil {
			lg.Logf("Warning: could not remove the website %s: %v", site.Domain, err)
		}
	})
	if err = Check(site); err != nil {
		return nil, err
	}
	oldRoot, newRoot := h.Root(from.Domain), h.Root(site.Domain)
	replace := func(s string) string { return ReplaceRoot(s, oldRoot, newRoot) }
	if err := h.CopySettings(ctx, from, site, replace(from.NginxCustom), lg); err != nil {
		lg.Logf("Warning: %v", err)
	}

	lg.Logf("Copying the files of %s", from.Domain)
	uid, gid, err := h.LookupIDs(site.SysUser)
	if err != nil {
		return nil, err
	}
	// The welcome page of the new site would stay next to the copied files.
	if root, err := os.OpenRoot(filepath.Join(newRoot, "public")); err == nil {
		root.Remove("index.html")
		root.Close()
	}
	paths := func(b []byte) ([]byte, error) {
		return bytes.ReplaceAll(b, []byte(oldRoot+"/"), []byte(newRoot+"/")), nil
	}
	// PHP settings of the site, such as a file included before each script.
	transform := map[string]func([]byte) ([]byte, error){"public/.user.ini": paths}
	for _, p := range relocatedFiles(oldRoot) {
		transform[p] = func(b []byte) ([]byte, error) {
			if bytes.IndexByte(b, 0) >= 0 { // a program, not a script
				return b, nil
			}
			return paths(b)
		}
	}
	stats, err := fsutil.CopyTree(ctx, oldRoot, newRoot, fsutil.CopyOptions{
		UID: uid, GID: gid, SrcUID: srcUID, Skip: LeftOut, Log: lg, Transform: transform,
		// node_modules/.bin, a virtual environment's python: links stay
		// links, and those into the site's folder lead into the copy's.
		Links: func(target string) string {
			if path.IsAbs(target) {
				return ReplaceRoot(target, oldRoot, newRoot)
			}
			return target
		},
	})
	if err != nil {
		return nil, fmt.Errorf("copy the files: %w", err)
	}
	lg.Logf("Copied %d files (%d MB)", stats.Files, stats.Bytes>>20)
	if n := len(stats.Skipped); n > 0 {
		lg.Logf("Left out %d items (logs, temporary files and files the site cannot read): %s",
			n, strings.Join(stats.Skipped[:min(10, n)], ", "))
	}

	res = &Result{Site: site}
	for _, d := range dbs {
		lg.Logf("Creating a database for the copy of %s", d.Name)
		db, err := h.CreateDB(ctx, site, d)
		if err != nil {
			return nil, err
		}
		undo = append(undo, func() {
			if err := h.DeleteDB(bg, db); err != nil {
				lg.Logf("Warning: could not remove the database %s: %v", db.Name, err)
			}
		})
		lg.Logf("Copying the database %s into %s", d.Name, db.Name)
		if err := h.CopyDB(ctx, d, db); err != nil {
			return nil, err
		}
		res.Databases = append(res.Databases, Copied{From: d.Name, To: db})
	}

	if IsApp(from) && h.StartApp != nil {
		if from.AppCommand != "" {
			lg.Logf("Starting the app of %s on port %d", site.Domain, site.ProxyPort)
		}
		if err := h.StartApp(ctx, from, site, replace(from.AppCommand), lg); err != nil {
			return nil, err
		}
	}
	if h.CopyDaemons != nil {
		n, err := h.CopyDaemons(ctx, from, site, replace)
		if err != nil {
			return nil, fmt.Errorf("copy the background programs: %w", err)
		}
		res.Daemons = n
	}

	lg.Logf("The copy is ready at http://%s/ once its domain points at this server", site.Domain)
	for _, c := range res.Databases {
		lg.Logf("The copy of the database %s is %s, with the user %s. The site's configuration files still name %s: change them to the new database, user and password (Databases → Connection details).",
			c.From, c.To.Name, c.To.Username, c.From)
	}
	if IsApp(from) {
		lg.Logf("The copy's app gets its port in the PORT variable (%d instead of %d). An app that sets its own port must be changed to use %d.",
			site.ProxyPort, from.ProxyPort, site.ProxyPort)
	}
	if res.Daemons > 0 {
		lg.Logf("%d background programs were copied, stopped: they would use the same databases and queues as %s until the copy's settings are changed. Start them under Background programs.",
			res.Daemons, from.Domain)
	}
	lg.Logf("Files that name the folder %s (an app's settings or their cache) still do: change them to %s.", oldRoot, newRoot)
	return res, nil
}

// relocatedFiles lists the files of the Relocated folders of a site
// (relative, with slashes) that may name the site's folder: what is in a
// bin folder (scripts; programs are told apart when they are copied),
// pyvenv.cfg and .pth files, of at most 1 MB.
func relocatedFiles(root string) []string {
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil
	}
	defer r.Close()
	var out []string
	for _, dir := range Relocated {
		fs.WalkDir(r.FS(), dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil || !d.Type().IsRegular() {
				return nil
			}
			name := path.Base(p)
			if name != "pyvenv.cfg" && !strings.HasSuffix(name, ".pth") && path.Base(path.Dir(p)) != "bin" {
				return nil
			}
			if fi, err := d.Info(); err == nil && fi.Size() <= 1<<20 {
				out = append(out, p)
			}
			return nil
		})
	}
	return out
}
