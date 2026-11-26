// Package siteclone copies a PHP or static website to a new domain: a new
// website of the same type with the same settings, a copy of its folder
// owned by the new website's user and, if asked, a copy of each of its
// databases. WordPress sites are copied by package wptoolkit instead, which
// also points WordPress at the new database and address.
package siteclone

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
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
	// version, or static); DeleteSite removes it with its files.
	CreateSite func(ctx context.Context, from *store.Site, domain string) (*store.Site, error)
	DeleteSite func(ctx context.Context, site *store.Site) error
	// CopySettings gives the new site the app, PHP settings and rewrite
	// preset of from, and the directives given (from's own, with its folder
	// replaced by the new one). Its errors are warnings: the copy works
	// without them.
	CopySettings func(ctx context.Context, from, to *store.Site, directives string, lg Logger) error
	// CreateDB creates an empty database linked to site, for a copy of from.
	CreateDB func(ctx context.Context, site *store.Site, from *store.Database) (*store.Database, error)
	DeleteDB func(ctx context.Context, db *store.Database) error
	// CopyDB copies the tables and data of one database into another.
	CopyDB func(ctx context.Context, from, to string) error
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
}

// LeftOut are the folders of a website that a clone does not copy: logs,
// temporary files and sessions, the Git deploy key (Git deploy settings are
// not copied) and the panel's build folder.
var LeftOut = []string{"logs", "tmp", ".ssh", ".panel-build"}

// Check refuses a website the panel cannot clone yet: app websites run a
// service or containers, whose copy needs more than files.
func Check(site *store.Site) error {
	if site == nil || validate.Domain(site.Domain) != nil || validate.SystemUsername(site.SysUser) != nil {
		return errors.New("invalid site")
	}
	if site.ProxyPort != 0 || strings.HasPrefix(site.App, "docker:") {
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

	kind := "a static website"
	if from.PHPVersion != "" {
		kind = "a PHP " + from.PHPVersion + " website"
	}
	lg.Logf("Creating %s for %s", kind, domain)
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
	if err := h.CopySettings(ctx, from, site, ReplaceRoot(from.NginxCustom, oldRoot, newRoot), lg); err != nil {
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
	stats, err := fsutil.CopyTree(ctx, oldRoot, newRoot, fsutil.CopyOptions{
		UID: uid, GID: gid, SrcUID: srcUID, Skip: LeftOut, Log: lg,
		// PHP settings of the site, such as a file included before each script.
		Transform: map[string]func([]byte) ([]byte, error){"public/.user.ini": paths},
	})
	if err != nil {
		return nil, fmt.Errorf("copy the files: %w", err)
	}
	lg.Logf("Copied %d files (%d MB)", stats.Files, stats.Bytes>>20)
	if n := len(stats.Skipped); n > 0 {
		lg.Logf("Left out %d items (logs, temporary files, links and files the site cannot read): %s",
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
		if err := h.CopyDB(ctx, d.Name, db.Name); err != nil {
			return nil, err
		}
		res.Databases = append(res.Databases, Copied{From: d.Name, To: db})
	}

	lg.Logf("The copy is ready at http://%s/ once its domain points at this server", site.Domain)
	for _, c := range res.Databases {
		lg.Logf("The copy of the database %s is %s, with the user %s. The site's configuration files still name %s: change them to the new database, user and password (Databases → Connection details).",
			c.From, c.To.Name, c.To.Username, c.From)
	}
	lg.Logf("Files that name the folder %s (an app's settings or their cache) still do: change them to %s.", oldRoot, newRoot)
	return res, nil
}
