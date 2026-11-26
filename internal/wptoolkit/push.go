package wptoolkit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// PushOptions chooses what a push copies from the staging copy to the live
// site.
type PushOptions struct {
	Files    bool
	Database bool
}

// PushHost is what a push needs from the rest of the panel; the API fills
// it with the backup and database managers.
type PushHost struct {
	// Backup backs up the live site's files and database (dbName; "" when
	// the database is not pushed) and returns a description of the backup
	// for the log. If it fails, nothing is pushed.
	Backup func(ctx context.Context, live *store.Site, dbName string, lg Logger) (string, error)
	// ResetDB empties a database: it is dropped and created again, and its
	// user keeps access.
	ResetDB func(ctx context.Context, name string) error
	// CopyDB copies the tables and data of one database into another.
	CopyDB func(ctx context.Context, from, to string) error
}

// PushPlan is what PreparePush read from the two sites' wp-config.php.
type PushPlan struct {
	LiveDB, StagingDB string // "" when the database is not pushed
}

// pushKept are the live site's own files: a push neither replaces nor
// removes them. wp-config.php points at the live database; the cache
// drop-ins are the ones the staging copy left out; .maintenance is the
// maintenance mode the push turned on.
var pushKept = append([]string{"wp-config.php", ".maintenance"}, leftOut...)

var tablePrefixRe = regexp.MustCompile(`(?m)^\s*\$table_prefix\s*=\s*['"]([A-Za-z0-9_]+)['"]\s*;`)

func tablePrefix(cfg string) string {
	if m := tablePrefixRe.FindStringSubmatch(cfg); m != nil {
		return m[1]
	}
	return ""
}

// PreparePush checks that a staging copy can be pushed to its live site
// and, when the database goes too, finds the two databases.
func (t *Toolkit) PreparePush(staging, live *store.Site, o PushOptions) (*PushPlan, error) {
	if err := checkSite(staging); err != nil {
		return nil, err
	}
	if err := checkSite(live); err != nil {
		return nil, err
	}
	if staging.ID == live.ID || staging.Domain == live.Domain || staging.SysUser == live.SysUser {
		return nil, errors.New("the staging copy and the live site are the same site")
	}
	if !o.Files && !o.Database {
		return nil, usererr.New("wp_push_nothing")
	}
	for _, s := range []*store.Site{staging, live} {
		if fi, err := os.Stat(t.public(s)); err != nil || !fi.IsDir() {
			return nil, usererr.New("wp_config_missing", s.Domain)
		}
	}
	plan := &PushPlan{}
	if !o.Database {
		return plan, nil
	}
	liveCfg, err := t.readConfig(live)
	if err != nil {
		return nil, err
	}
	liveSrc, err := parseCloneSource(liveCfg)
	if err != nil {
		return nil, err
	}
	b, err := readSiteFile(t.public(staging), "wp-config.php", 1<<20)
	if errors.Is(err, os.ErrNotExist) {
		return nil, usererr.New("wp_config_missing", staging.Domain)
	}
	if err != nil {
		return nil, err
	}
	stagingSrc, err := parseCloneSource(string(b))
	if err != nil {
		return nil, err
	}
	if liveSrc.DBName == stagingSrc.DBName {
		return nil, usererr.New("wp_push_same_db", liveSrc.DBName)
	}
	if p, q := tablePrefix(liveCfg), tablePrefix(string(b)); p != q {
		return nil, usererr.New("wp_push_prefix", q+" / "+p)
	}
	plan.LiveDB, plan.StagingDB = liveSrc.DBName, stagingSrc.DBName
	return plan, nil
}

// liveAddresses are the live site's settings that the staging database
// would otherwise bring along: its addresses, and whether search engines
// may index it (staging copies ask them not to).
type liveAddresses struct {
	home, siteURL, blogPublic string
}

func (t *Toolkit) readLiveAddresses(ctx context.Context, live *store.Site) (*liveAddresses, error) {
	a := &liveAddresses{}
	for _, opt := range []struct {
		name string
		dst  *string
	}{{"home", &a.home}, {"siteurl", &a.siteURL}} {
		out, err := t.wp(ctx, live, nil, "option", "get", opt.name, "--format=json")
		if err != nil {
			return nil, err
		}
		if *opt.dst = parseURL(out); *opt.dst == "" {
			return nil, fmt.Errorf("the %s address of %s is not an http or https address", opt.name, live.Domain)
		}
	}
	out, err := t.wp(ctx, live, nil, "option", "get", "blog_public")
	if err != nil {
		return nil, err
	}
	if a.blogPublic = strings.TrimSpace(out); a.blogPublic != "0" {
		a.blogPublic = "1"
	}
	return a, nil
}

// stagingOrigins returns the addresses (scheme://host) the staging copy
// answers on: those of its home and siteurl options, and its domain with
// either scheme (it may have got a certificate since it was made).
func (t *Toolkit) stagingOrigins(ctx context.Context, staging *store.Site) ([]string, error) {
	origins, err := t.siteOrigins(ctx, staging)
	if err != nil {
		return nil, err
	}
	for _, o := range []string{"https://" + staging.Domain, "http://" + staging.Domain} {
		if !slices.Contains(origins, o) {
			origins = append(origins, o)
		}
	}
	return origins, nil
}

// Push copies a staging copy back over its live site: the files of
// public/ (as rsync --delete would: what the staging copy does not have is
// removed), the database, or both. The live site is backed up first and
// nothing changes if the backup fails. While the push runs, the live site
// is in maintenance mode, which is turned off again however the push ends
// (unless the site was in maintenance mode before).
//
// The live site keeps its wp-config.php and its cache drop-ins. The
// database push reverses what the staging copy did: the staging address is
// replaced with the live one, and the live site's addresses and search
// engine setting are put back.
func (t *Toolkit) Push(ctx context.Context, staging, live *store.Site, o PushOptions, host PushHost, lg Logger) (err error) {
	plan, err := t.PreparePush(staging, live, o)
	if err != nil {
		return err
	}
	if err := t.Ensure(ctx); err != nil {
		return err
	}
	stagingUID, _, err := t.lookupIDs(staging.SysUser)
	if err != nil {
		return err
	}
	uid, gid, err := t.lookupIDs(live.SysUser)
	if err != nil {
		return err
	}
	// Read before anything changes: the staging database replaces these.
	var addr *liveAddresses
	var origins []string
	if o.Database {
		if addr, err = t.readLiveAddresses(ctx, live); err != nil {
			// The push may be what repairs a live database WordPress cannot
			// use: the panel knows the site's address.
			base := "http://" + live.Domain
			if live.SSL {
				base = "https://" + live.Domain
			}
			lg.Logf("Warning: could not read the live site's address (%v); using %s", err, base)
			addr = &liveAddresses{home: base, siteURL: base, blogPublic: "1"}
		}
		if origins, err = t.stagingOrigins(ctx, staging); err != nil {
			return err
		}
	}

	lg.Logf("Backing up %s before the push", live.Domain)
	backup, err := host.Backup(ctx, live, plan.LiveDB, lg)
	if err != nil {
		return fmt.Errorf("the backup of %s failed, so nothing was pushed: %w", live.Domain, err)
	}

	bg := context.WithoutCancel(ctx)
	if t.maintenanceOn(live) {
		lg.Logf("%s is in maintenance mode already; it stays in it", live.Domain)
	} else {
		lg.Logf("Turning on maintenance mode on %s", live.Domain)
		if err := t.SetMaintenance(ctx, live, true); err != nil {
			// A live site that is broken (its database, say) is a reason to
			// push: WordPress reads the file without wp-cli's help.
			lg.Logf("wp-cli could not turn on maintenance mode (%v); writing .maintenance directly", err)
			if werr := writeMaintenance(t.public(live), uid, gid); werr != nil {
				return fmt.Errorf("could not turn on maintenance mode, so nothing was pushed: %w", err)
			}
		}
		defer func() {
			lg.Logf("Turning off maintenance mode on %s", live.Domain)
			if merr := t.SetMaintenance(bg, live, false); merr != nil {
				// wp-cli needs a working database; the file can go without one.
				if rerr := t.removeMaintenance(live); rerr != nil {
					lg.Logf("Warning: could not turn off maintenance mode: %v", rerr)
					if err == nil {
						err = fmt.Errorf("the push worked, but maintenance mode is still on: %w", rerr)
					}
				}
			}
		}()
	}
	failed := func(err error) error {
		return fmt.Errorf("%w. The live site was backed up first (%s): restore that backup on the Backups page if the site does not work", err, backup)
	}

	if o.Files {
		lg.Logf("Copying the files of %s over those of %s", staging.Domain, live.Domain)
		stagingRoot, liveRoot := t.root(staging.Domain), t.root(live.Domain)
		pathsBack := func(b []byte) ([]byte, error) {
			return bytes.ReplaceAll(b, []byte(stagingRoot+"/"), []byte(liveRoot+"/")), nil
		}
		stats, err := fsutil.CopyTree(ctx, t.public(staging), t.public(live), fsutil.CopyOptions{
			UID: uid, GID: gid, SrcUID: stagingUID, Skip: pushKept, Mirror: true, Log: lg,
			Transform: map[string]func([]byte) ([]byte, error){".user.ini": pathsBack, "wordfence-waf.php": pathsBack},
		})
		if err != nil {
			return failed(fmt.Errorf("copy the files: %w", err))
		}
		lg.Logf("Copied %d files (%d MB), %d unchanged, %d removed", stats.Files, stats.Bytes>>20, stats.Unchanged, stats.Removed)
		lg.Logf("Kept the live site's own %s", strings.Join(pushKept, ", "))
		logLeftOut(lg, slices.DeleteFunc(stats.Skipped, func(p string) bool { return slices.Contains(pushKept, p) }))
	}

	if o.Database {
		lg.Logf("Emptying the database %s", plan.LiveDB)
		if err := host.ResetDB(ctx, plan.LiveDB); err != nil {
			return failed(fmt.Errorf("empty the database %s: %w", plan.LiveDB, err))
		}
		lg.Logf("Copying the database %s into %s", plan.StagingDB, plan.LiveDB)
		if err := host.CopyDB(ctx, plan.StagingDB, plan.LiveDB); err != nil {
			return failed(err)
		}
		if err := t.pointAtLive(ctx, staging, live, addr, origins, lg); err != nil {
			return failed(err)
		}
	}
	lg.Logf("%s now runs the staging copy of %s", live.Domain, staging.Domain)
	return nil
}

// writeMaintenance writes the panel's .maintenance file into public,
// owned by the site's user (uid -1 leaves the owner alone).
func writeMaintenance(public string, uid, gid int) error {
	root, err := os.OpenRoot(public)
	if err != nil {
		return err
	}
	defer root.Close()
	if fi, err := root.Lstat(".maintenance"); err == nil && !fi.Mode().IsRegular() {
		return errors.New(".maintenance is not a regular file")
	}
	if err := root.WriteFile(".maintenance", []byte(maintenanceFile), 0o640); err != nil {
		return err
	}
	if uid < 0 {
		return nil
	}
	return root.Lchown(".maintenance", uid, gid)
}

// pointAtLive reverses, in the live database, what the staging copy did:
// the staging addresses (also as JSON, as page builders save them) and
// folder become the live ones, then the live site's own addresses and
// search engine setting are written back and the caches emptied.
func (t *Toolkit) pointAtLive(ctx context.Context, staging, live *store.Site, addr *liveAddresses, origins []string, lg Logger) error {
	liveOrigin := origin(addr.home)
	pairs := [][2]string{}
	for _, o := range origins {
		pairs = append(pairs, [2]string{o, liveOrigin},
			[2]string{strings.ReplaceAll(o, "/", `\/`), strings.ReplaceAll(liveOrigin, "/", `\/`)})
	}
	pairs = append(pairs, [2]string{t.root(staging.Domain) + "/", t.root(live.Domain) + "/"})
	for _, p := range pairs {
		lg.Logf("Replacing %s with %s in the database", p[0], p[1])
		if err := t.stream(ctx, live, lg, "027", "search-replace", p[0], p[1],
			"--all-tables", "--precise", "--report-changed-only"); err != nil {
			return err
		}
	}
	lg.Logf("Setting the addresses back to %s", addr.home)
	for _, kv := range [][2]string{{"home", addr.home}, {"siteurl", addr.siteURL}, {"blog_public", addr.blogPublic}} {
		if _, err := t.wp(ctx, live, nil, "option", "update", kv[0], kv[1]); err != nil {
			return err
		}
	}
	lg.Logf("Emptying the caches")
	if _, err := t.wp(ctx, live, nil, "cache", "flush"); err != nil {
		lg.Logf("Warning: %v", err)
	}
	// WordPress builds the rewrite rules again, with every plugin loaded, on
	// the next visit. "wp rewrite flush" would build them without plugins.
	if _, err := t.wp(ctx, live, nil, "option", "delete", "rewrite_rules"); err != nil && !hasMessage(err, "Could not delete") {
		lg.Logf("Warning: %v", err)
	}
	return nil
}
