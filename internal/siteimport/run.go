package siteimport

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/database"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
	"github.com/nefocp/nefo/internal/wptoolkit"
)

// Host is what an import needs from the rest of the panel; the API fills
// it with the hosting, database, certificate and cron functions.
type Host struct {
	// Root returns a website's folder; its files are in Root/public.
	Root      func(domain string) string
	LookupIDs func(name string) (uid, gid int, err error)
	// CreateSite creates a website, suspended (it answers 503) until the
	// import is done; DeleteSite removes it with its files and cron jobs.
	CreateSite func(ctx context.Context, domain string, aliases []string, php string) (*store.Site, error)
	DeleteSite func(ctx context.Context, site *store.Site) error
	// Suspend closes (on) or opens a website.
	Suspend       func(ctx context.Context, site *store.Site, on bool) error
	SetNote       func(ctx context.Context, site *store.Site, note string) error
	MarkWordPress func(ctx context.Context, site *store.Site, lg Logger) error
	CreateDB      func(ctx context.Context, d database.NewDB) (*store.Database, error)
	DeleteDB      func(ctx context.Context, db *store.Database) error
	// ImportDB runs a dump as the database's limited import account.
	ImportDB    func(ctx context.Context, name string, r io.Reader) error
	ReplaceInDB func(ctx context.Context, site *store.Site, from, to string, lg Logger) error
	InstallCert func(ctx context.Context, site *store.Site, chain, key []byte) error
	AddCron     func(ctx context.Context, site *store.Site, name, schedule, command string) error
	// CheckSpace refuses an import whose files and databases do not fit.
	CheckSpace func(files, databases int64) error
	Now        func() time.Time
	// MySQL says the database server is MySQL 8, not MariaDB: dumps then
	// keep their MySQL 8 collations and comments (see sqlfix.go).
	MySQL bool
}

// Result is what an import created.
type Result struct {
	Sites     []ResultSite `json:"sites"`
	Databases []ResultDB   `json:"databases"`
	Warnings  []string     `json:"warnings"`
}

// ResultSite is a website the import created (or filled).
type ResultSite struct {
	ID     int64  `json:"id"`
	Domain string `json:"domain"`
	App    string `json:"app"`
}

// ResultDB is a database the import created.
type ResultDB struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	User     string `json:"user"`
	From     string `json:"from"`
	Password string `json:"password"` // kept, known or new
	Site     string `json:"site,omitempty"`
	// Manual: the website's configuration must be changed by hand to use
	// the new name, user or password (Nefo only rewrites WordPress's).
	Manual bool `json:"manual"`
}

// Check validates a selection against the plan, before a job starts.
func (p *Plan) Check(sel Selection) error {
	if p.Kind == KindArchive {
		if sel.SiteID == 0 {
			d := cleanDomain(sel.Domain)
			if validate.Domain(d) != nil {
				return usererr.New("invalid_domain", d)
			}
		}
		for _, s := range sel.Databases {
			if p.database(s, true) == nil {
				return usererr.New("import_unknown_item", s)
			}
		}
		if len(sel.Databases) > 0 && !p.DBReady {
			return usererr.New("db_not_installed")
		}
		return nil
	}
	n := 0
	for _, name := range sel.Domains {
		d := p.domain(name)
		if d == nil {
			return usererr.New("import_unknown_item", name)
		}
		if d.Blocked != "" {
			return usererr.New("import_blocked", name)
		}
		n++
	}
	for _, name := range sel.Databases {
		db := p.database(name, false)
		if db == nil {
			return usererr.New("import_unknown_item", name)
		}
		if db.Blocked != "" {
			return usererr.New("import_blocked", name)
		}
		n++
	}
	if n == 0 {
		return usererr.New("import_nothing")
	}
	if len(sel.Databases) > 0 && !p.DBReady {
		return usererr.New("db_not_installed")
	}
	return nil
}

func (p *Plan) domain(name string) *Domain {
	for _, d := range p.Domains {
		if d.Name == name {
			return d
		}
	}
	return nil
}

func (p *Plan) database(name string, bySource bool) *Database {
	for _, d := range p.Databases {
		if (bySource && d.Source == name) || (!bySource && d.Name == name) {
			return d
		}
	}
	return nil
}

// run holds what an import created, to remove it again if it fails.
type run struct {
	h        Host
	lg       Logger
	undo     []func(context.Context)
	res      *Result
	budget   *budget
	lim      Limits
	skips    *skipLog
	stamp    string
	archName string
}

func newRun(h Host, lg Logger, lim Limits, archName string) *run {
	if h.Now == nil {
		h.Now = time.Now
	}
	return &run{h: h, lg: lg, lim: lim, budget: &budget{left: lim.MaxTotal}, skips: newSkipLog(),
		res:   &Result{Sites: []ResultSite{}, Databases: []ResultDB{}, Warnings: []string{}},
		stamp: h.Now().Format("2006-01-02 15:04"), archName: archName}
}

func (r *run) warn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	r.lg.Logf("Warning: %s", msg)
	r.res.Warnings = append(r.res.Warnings, msg)
}

// rollback removes what the import created, newest first, also when the
// job was stopped.
func (r *run) rollback(ctx context.Context, err error) error {
	bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Minute)
	defer cancel()
	if errors.Is(err, context.Canceled) {
		err = errors.New("the import was stopped")
	} else if errors.Is(err, context.DeadlineExceeded) {
		err = errors.New("the import took too long and was stopped")
	}
	r.lg.Logf("The import failed: %v", err)
	if len(r.undo) > 0 {
		r.lg.Logf("Removing what the import created")
	}
	for i := len(r.undo) - 1; i >= 0; i-- {
		r.undo[i](bg)
	}
	return err
}

// createSite creates a website for the import, suspended and marked with
// a note until the import is done.
func (r *run) createSite(ctx context.Context, domain string, aliases []string, php string) (*store.Site, error) {
	kind := "a static website"
	if php != "" {
		kind = "a PHP " + php + " website"
	}
	if len(aliases) > 0 {
		r.lg.Logf("Creating %s for %s (also %s)", kind, domain, strings.Join(aliases, ", "))
	} else {
		r.lg.Logf("Creating %s for %s", kind, domain)
	}
	site, err := r.h.CreateSite(ctx, domain, aliases, php)
	if err != nil {
		return nil, fmt.Errorf("create the website %s: %w", domain, err)
	}
	r.undo = append(r.undo, func(bg context.Context) {
		if err := r.h.DeleteSite(bg, site); err != nil {
			r.lg.Logf("Warning: could not remove the website %s: %v. It stays suspended, with a note saying the import did not finish.", site.Domain, err)
		}
	})
	if validate.Domain(site.Domain) != nil || validate.SystemUsername(site.SysUser) != nil {
		return nil, errors.New("invalid site")
	}
	if err := r.h.SetNote(ctx, site, fmt.Sprintf("Import from %s not finished (started %s)", truncName(r.archName), r.stamp)); err != nil {
		r.lg.Logf("Warning: %v", err)
	}
	return site, nil
}

func truncName(s string) string {
	if len(s) > 80 {
		return s[:80] + "…"
	}
	return s
}

// extractorFor opens a website's public folder for writing.
func (r *run) extractorFor(site *store.Site) (*extractor, error) {
	uid, gid, err := r.h.LookupIDs(site.SysUser)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(filepath.Join(r.h.Root(site.Domain), "public"))
	if err != nil {
		return nil, err
	}
	return &extractor{root: root, uid: uid, gid: gid, lim: r.lim, budget: r.budget, skips: r.skips, lg: r.lg,
		next: time.Now().Add(5 * time.Second), label: site.Domain}, nil
}

// budgetReader counts what passes through against the import's budget.
type budgetReader struct {
	r io.Reader
	b *budget
}

func (br *budgetReader) Read(p []byte) (int, error) {
	n, err := br.r.Read(p)
	if e := br.b.take(int64(n)); e != nil {
		return n, e
	}
	return n, err
}

// importDump runs a dump into a database, adapted for MariaDB on the way.
func (r *run) importDump(ctx context.Context, db, from string, src io.Reader) error {
	fx := newSQLFixer(&budgetReader{r: src, b: r.budget})
	fx.mysql = r.h.MySQL
	start := time.Now()
	if err := r.h.ImportDB(ctx, db, fx); err != nil {
		return fmt.Errorf("import the database %s: %w", from, err)
	}
	if s := fx.Stats.String(); s != "" {
		r.lg.Logf("Adapted the dump of %s for this server: %s", from, s)
	}
	r.lg.Logf("Imported %s into %s (%s)", from, db, time.Since(start).Round(time.Second))
	return nil
}

func (r *run) createDB(ctx context.Context, n database.NewDB, from string) (*store.Database, error) {
	r.lg.Logf("Creating the database %s with the user %s", n.Name, cmpOr(n.User, n.Name))
	d, err := r.h.CreateDB(ctx, n)
	if err != nil {
		return nil, fmt.Errorf("create the database %s: %w", n.Name, err)
	}
	r.undo = append(r.undo, func(bg context.Context) {
		if err := r.h.DeleteDB(bg, d); err != nil {
			r.lg.Logf("Warning: could not remove the database %s: %v", d.Name, err)
		}
	})
	return d, nil
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// removePlaceholder removes the welcome page of a new website and returns
// it, so the copied files do not sit next to it.
func removePlaceholder(public string) []byte {
	root, err := os.OpenRoot(public)
	if err != nil {
		return nil
	}
	defer root.Close()
	b, err := root.ReadFile("index.html")
	if err != nil || len(b) > 64<<10 {
		return nil
	}
	root.Remove("index.html")
	return b
}

// finish opens the websites and records the import in their notes.
func (r *run) finish(ctx context.Context, sites []*store.Site, note string) {
	for _, s := range sites {
		if err := r.h.Suspend(ctx, s, false); err != nil {
			r.warn("%s stays suspended (%v): open it on the website's Expiry and suspension tab", s.Domain, err)
		}
		if err := r.h.SetNote(ctx, s, note); err != nil {
			r.lg.Logf("Warning: %v", err)
		}
	}
}

// ---- cPanel ----

// RunCPanel imports what sel chose from a cPanel backup: one website per
// domain, the databases with their users, then, in one more pass over the
// archive, each website's files and each database's dump; then the
// fix-ups (paths, wp-config.php, WordPress's database), certificates and
// cron jobs. If anything fails, everything it created is removed.
func RunCPanel(ctx context.Context, f *os.File, c *CPanel, plan *Plan, sel Selection, h Host, lim Limits, lg Logger) (res *Result, err error) {
	if err := plan.Check(sel); err != nil {
		return nil, err
	}
	r := newRun(h, lg, lim, c.arch.Name)
	defer func() {
		// A bug (a panic) undoes the import too, then goes on up so the
		// job reports it with its stack.
		p := recover()
		if p != nil {
			err = fmt.Errorf("internal error: %v", p)
		}
		if err != nil {
			err = r.rollback(ctx, err)
			res = nil
		}
		if p != nil {
			panic(p)
		}
	}()
	var doms []*Domain
	for _, d := range plan.Domains {
		if slices.Contains(sel.Domains, d.Name) {
			doms = append(doms, d)
		}
	}
	var dbs []*Database
	for _, d := range plan.Databases {
		if slices.Contains(sel.Databases, d.Name) {
			dbs = append(dbs, d)
		}
	}
	var files, dbBytes int64
	for _, d := range doms {
		files += d.Size
	}
	for _, d := range dbs {
		dbBytes += d.Size * 2 // tables and their indexes take more than the dump
	}
	if h.CheckSpace != nil {
		if err := h.CheckSpace(files, dbBytes); err != nil {
			return nil, err
		}
	}

	// 1. The websites, closed until the import is done.
	sites := map[string]*store.Site{}
	var order []*store.Site
	for _, d := range doms {
		site, err := r.createSite(ctx, d.Name, d.Aliases, d.PHP)
		if err != nil {
			return nil, err
		}
		removePlaceholder(filepath.Join(h.Root(site.Domain), "public"))
		sites[d.Name], order = site, append(order, site)
	}
	// 2. The databases and their users.
	made := map[string]*store.Database{} // by old name
	for _, d := range dbs {
		var siteID int64
		if s := sites[d.Site]; s != nil {
			siteID = s.ID
		} else if len(order) > 0 {
			siteID = order[0].ID
		}
		db, err := r.createDB(ctx, database.NewDB{Name: d.NewName, User: d.User, Password: d.plaintext, Hash: d.hash, SiteID: siteID}, d.Name)
		if err != nil {
			return nil, err
		}
		switch d.Password {
		case "kept":
			lg.Logf("%s keeps its password from the old server", db.Username)
		case "known":
			lg.Logf("%s keeps its password from the old server (the website's configuration has it)", db.Username)
		}
		made[d.Name] = db
	}

	// 3. One pass over the archive: files and dumps.
	routes := map[string][]*extractor{}
	var xs []*extractor
	defer func() {
		for _, x := range xs {
			x.root.Close()
		}
	}()
	for _, d := range doms {
		x, err := r.extractorFor(sites[d.Name])
		if err != nil {
			return nil, err
		}
		xs = append(xs, x)
		routes[d.rel] = append(routes[d.rel], x)
	}
	route := func(q string) ([]*extractor, string) {
		for p := q; ; {
			if xs := routes[p]; xs != nil {
				if p == q {
					return xs, ""
				}
				return xs, q[len(p)+1:]
			}
			i := strings.LastIndexByte(p, '/')
			if i < 0 {
				return nil, ""
			}
			p = p[:i]
		}
	}
	wantCerts := map[string]bool{}
	for _, n := range sel.Certificates {
		if d := plan.domain(n); d != nil && slices.Contains(sel.Domains, n) && d.Cert != nil && d.Cert.Usable {
			wantCerts[n] = true
		}
	}
	pool := &certPool{}
	dumps := map[string]*Database{}
	for _, d := range dbs {
		dumps[dumpPath(d.Name)] = d
	}
	done := map[string]bool{}
	w := &walker{lim: lim}
	if len(doms) > 0 {
		lg.Logf("Unpacking the files")
	}
	var visit func(e *entry, inHome bool) error
	visit = func(e *entry, inHome bool) error {
		name, ok := cleanName(e.Name)
		if !ok {
			r.skips.add(skipUnsafe, e.Name)
			return nil
		}
		var p string
		if inHome {
			p = "homedir/" + name
		} else {
			rest, ok := strings.CutPrefix(name+"/", c.prefix)
			if !ok {
				return nil
			}
			p = strings.TrimSuffix(rest, "/")
		}
		if p == "" {
			return nil
		}
		if len(wantCerts) > 0 && e.Kind == kindFile && certFile(p) && e.Size <= maxPEMFile {
			pool.add(certGroup(p), readSmall(e.R, maxPEMFile))
			return nil
		}
		if q, ok := strings.CutPrefix(p, "homedir/"); ok {
			targets, rel := route(q)
			if len(targets) == 0 {
				return nil
			}
			if err := targets[0].put(rel, e.Name, e); err != nil {
				return fmt.Errorf("unpack %s: %w", e.Name, err)
			}
			// Two websites with the same folder: the others get a copy.
			for _, x := range targets[1:] {
				e2 := *e
				if e.Kind == kindFile {
					src, err := targets[0].root.Open(rel)
					if err != nil {
						continue // it was left out
					}
					e2.R = src
					err = x.put(rel, e.Name, &e2)
					src.Close()
					if err != nil {
						return fmt.Errorf("unpack %s: %w", e.Name, err)
					}
					continue
				}
				if err := x.put(rel, e.Name, &e2); err != nil {
					return fmt.Errorf("unpack %s: %w", e.Name, err)
				}
			}
			return nil
		}
		if p == "homedir.tar" && e.Kind == kindFile {
			return w.walkTar(ctx, e.R, func(e *entry) error { return visit(e, true) })
		}
		if d := dumps[p]; d != nil && e.Kind == kindFile && !done[p] {
			lg.Logf("Importing the database %s (%s)", d.Name, humanBytes(e.Size))
			if err := r.importDump(ctx, made[d.Name].Name, d.Name, e.R); err != nil {
				return err
			}
			done[p] = true
		}
		return nil
	}
	if err := w.walk(ctx, f, c.arch, func(e *entry) error { return visit(e, false) }); err != nil {
		return nil, err
	}
	for p, d := range dumps {
		if !done[p] {
			return nil, fmt.Errorf("the dump of the database %s (%s) is not in the archive any more", d.Name, p)
		}
	}
	for _, x := range xs {
		lg.Logf("%s: %s (%s)", x.label, count(int64(x.files), "file", "files"), humanBytes(x.bytes))
	}
	for _, l := range r.skips.lines() {
		lg.Logf("%s", l)
	}

	// 4. Paths, wp-config.php and WordPress's database.
	for i, d := range doms {
		site := sites[d.Name]
		newPublic := filepath.ToSlash(filepath.Join(h.Root(site.Domain), "public"))
		oldRoot := strings.TrimRight(path.Clean("/"+d.DocRoot), "/")
		var wpDB *Database
		rewrite := func(cfg string) (string, error) {
			name, ok := wptoolkit.ConfigDefine(cfg, "DB_NAME")
			if !ok {
				return cfg, errors.New("wp-config.php does not name its database as plain text: check DB_NAME, DB_USER and DB_PASSWORD by hand")
			}
			pd := plan.database(name, false)
			if pd == nil || made[name] == nil {
				return cfg, fmt.Errorf("wp-config.php uses the database %s, which was not imported", name)
			}
			wpDB = pd
			if !pd.ConfigChange {
				return cfg, nil
			}
			db := made[name]
			lg.Logf("Pointing %s's wp-config.php at the database %s", d.Name, db.Name)
			return wptoolkit.SetConfigDB(cfg, db.Name, db.Username, db.Password)
		}
		wp, warns := fixFiles(xs[i], oldRoot, newPublic, true, rewrite, lg)
		for _, w := range warns {
			r.warn("%s: %s", d.Name, w)
		}
		app := d.App
		if wp && site.PHPVersion != "" {
			app = "wordpress"
			if err := h.MarkWordPress(ctx, site, lg); err != nil {
				r.warn("%s: %v", d.Name, err)
			}
			if wpDB != nil && oldRoot != newPublic {
				if err := h.ReplaceInDB(ctx, site, oldRoot, newPublic, lg); err != nil {
					r.warn("%s: replacing the old folder in the database failed: %v", d.Name, err)
				}
			}
		}
		r.res.Sites = append(r.res.Sites, ResultSite{ID: site.ID, Domain: site.Domain, App: app})
	}
	for _, d := range dbs {
		db := made[d.Name]
		rd := ResultDB{ID: db.ID, Name: db.Name, User: db.Username, From: d.Name, Password: d.Password, Site: d.Site}
		// WordPress websites were pointed at it; others need a hand.
		if d.ConfigChange {
			rd.Manual = true
			for _, n := range d.UsedBy {
				if slices.Contains(sel.Domains, n) {
					rd.Manual = false
				}
			}
		}
		if rd.Manual {
			pw := "its old password"
			if d.Password == "new" {
				pw = "a new password (Show login on the Databases page)"
			}
			lg.Logf("The database %s is now %s, with the user %s and %s: change the website's configuration to use them", d.Name, db.Name, db.Username, pw)
		}
		r.res.Databases = append(r.res.Databases, rd)
	}

	// 5. Certificates and cron jobs: a failure is only a warning.
	for _, d := range doms {
		if !wantCerts[d.Name] {
			continue
		}
		pk := pool.best(d.Name, append([]string{d.Name}, d.Aliases...), r.h.Now())
		if pk == nil || !pk.summary.Usable || pk.key == nil {
			r.warn("%s: the certificate is not usable any more; issue one from Let's Encrypt on the website's SSL / HTTPS tab", d.Name)
			continue
		}
		lg.Logf("Installing the certificate of %s (valid until %s)", d.Name, pk.summary.NotAfter.Format("2006-01-02"))
		if err := h.InstallCert(ctx, sites[d.Name], pk.chain, pk.key); err != nil {
			r.warn("%s: the certificate was not installed (%v); issue one from Let's Encrypt on the website's SSL / HTTPS tab", d.Name, err)
		}
	}
	for _, i := range sel.Cron {
		if i < 0 || i >= len(plan.Cron) {
			continue
		}
		cl := plan.Cron[i]
		site := sites[cl.Domain]
		if !cl.OK || site == nil {
			continue
		}
		name := "Imported: " + cl.NewCommand
		if len(name) > 90 {
			name = name[:90] + "…"
		}
		lg.Logf("Adding the cron job %q as the user of %s", cl.Schedule+" "+cl.NewCommand, cl.Domain)
		if err := h.AddCron(ctx, site, strings.ToValidUTF8(name, ""), cl.Schedule, cl.NewCommand); err != nil {
			r.warn("the cron job %q was not added: %v", cl.Raw, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// 6. Open the websites.
	r.finish(ctx, order, fmt.Sprintf("Imported from %s on %s", truncName(c.arch.Name), r.stamp))
	lg.Logf("The import is done: %d websites, %d databases", len(order), len(made))
	return r.res, nil
}

// ---- site archive ----

// RunSite imports a site archive and its dumps into a new website or an
// existing empty one (target), as sel says. dumps are the open dump files
// by path. If anything fails, what it created is removed and an existing
// website is emptied again.
func RunSite(ctx context.Context, f *os.File, s *SiteArchive, plan *Plan, dumps map[string]*os.File, sel Selection,
	target *store.Site, h Host, lim Limits, lg Logger) (res *Result, err error) {
	if err := plan.Check(sel); err != nil {
		return nil, err
	}
	r := newRun(h, lg, lim, s.arch.Name)
	defer func() {
		// A bug (a panic) undoes the import too, then goes on up so the
		// job reports it with its stack.
		p := recover()
		if p != nil {
			err = fmt.Errorf("internal error: %v", p)
		}
		if err != nil {
			err = r.rollback(ctx, err)
			res = nil
		}
		if p != nil {
			panic(p)
		}
	}()
	var dbs []*Database
	for _, d := range plan.Databases {
		if slices.Contains(sel.Databases, d.Source) {
			dbs = append(dbs, d)
		}
	}
	var dbBytes int64
	for _, d := range dbs {
		dbBytes += d.Size * 2
	}
	if h.CheckSpace != nil {
		if err := h.CheckSpace(plan.Site.Size, dbBytes); err != nil {
			return nil, err
		}
	}
	// 1. The website.
	site := target
	note := fmt.Sprintf("Imported from %s on %s", truncName(s.arch.Name), r.stamp)
	if site == nil {
		domain := cleanDomain(sel.Domain)
		var aliases []string
		if sel.WWW && !strings.HasPrefix(domain, "www.") {
			aliases = []string{"www." + domain}
		}
		if site, err = r.createSite(ctx, domain, aliases, sel.PHP); err != nil {
			return nil, err
		}
		removePlaceholder(filepath.Join(h.Root(site.Domain), "public"))
	} else {
		lg.Logf("Importing into the website %s", site.Domain)
		oldNote, wasDisabled := site.Note, site.Disabled
		note = oldNote
		uid, gid, err := h.LookupIDs(site.SysUser)
		if err != nil {
			return nil, err
		}
		public := filepath.Join(h.Root(site.Domain), "public")
		placeholder := removePlaceholder(public)
		r.undo = append(r.undo, func(bg context.Context) {
			lg.Logf("Emptying %s again", public)
			emptyFolder(public, placeholder, uid, gid)
			h.SetNote(bg, site, oldNote)
			if !wasDisabled {
				h.Suspend(bg, site, false)
			}
		})
		if !wasDisabled {
			if err := h.Suspend(ctx, site, true); err != nil {
				return nil, err
			}
		}
		if err := h.SetNote(ctx, site, fmt.Sprintf("Import from %s not finished (started %s)", truncName(s.arch.Name), r.stamp)); err != nil {
			lg.Logf("Warning: %v", err)
		}
	}
	// 2. The databases.
	made := map[string]*store.Database{}
	for _, d := range dbs {
		db, err := r.createDB(ctx, database.NewDB{Name: d.NewName, SiteID: site.ID}, d.Source)
		if err != nil {
			return nil, err
		}
		made[d.Source] = db
		src := dumps[d.Source]
		if src == nil {
			return nil, fmt.Errorf("the dump %s is not open", d.Source)
		}
		if _, err := src.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		var in io.Reader = src
		if d.format == FormatSQLGz {
			gz, err := gzip.NewReader(src)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", d.Source, err)
			}
			fi, _ := src.Stat()
			in = &capReader{r: gz, max: lim.inflateLimit(fi.Size())}
			defer gz.Close()
		}
		lg.Logf("Importing %s into the database %s", filepath.Base(d.Source), db.Name)
		if err := r.importDump(ctx, db.Name, filepath.Base(d.Source), in); err != nil {
			return nil, err
		}
	}
	// 3. The files.
	x, err := r.extractorFor(site)
	if err != nil {
		return nil, err
	}
	defer x.root.Close()
	lg.Logf("Unpacking the files")
	w := &walker{lim: lim}
	if err := w.walk(ctx, f, s.arch, func(e *entry) error {
		name, ok := cleanName(e.Name)
		if !ok {
			r.skips.add(skipUnsafe, e.Name)
			return nil
		}
		rel := name
		if s.root != "" {
			if name == s.root {
				return nil
			}
			var ok bool
			if rel, ok = strings.CutPrefix(name, s.root+"/"); !ok {
				return nil
			}
		}
		if err := x.put(rel, e.Name, e); err != nil {
			return fmt.Errorf("unpack %s: %w", e.Name, err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	lg.Logf("%s: %s (%s)", x.label, count(int64(x.files), "file", "files"), humanBytes(x.bytes))
	for _, l := range r.skips.lines() {
		lg.Logf("%s", l)
	}
	// 4. wp-config.php and the old folder.
	newPublic := filepath.ToSlash(filepath.Join(h.Root(site.Domain), "public"))
	oldRoot := plan.Site.OldRoot
	var wpDB *store.Database
	rewrite := func(cfg string) (string, error) {
		if len(made) == 0 {
			return cfg, errors.New("no database was imported: wp-config.php still names the old one")
		}
		name, _ := wptoolkit.ConfigDefine(cfg, "DB_NAME")
		for _, d := range dbs {
			if d.Name == name || len(dbs) == 1 {
				wpDB = made[d.Source]
			}
		}
		if wpDB == nil {
			return cfg, fmt.Errorf("wp-config.php uses the database %s, which none of the dumps is: set DB_NAME, DB_USER and DB_PASSWORD by hand", name)
		}
		lg.Logf("Pointing wp-config.php at the database %s", wpDB.Name)
		return wptoolkit.SetConfigDB(cfg, wpDB.Name, wpDB.Username, wpDB.Password)
	}
	wp, warns := fixFiles(x, oldRoot, newPublic, false, rewrite, lg)
	for _, w := range warns {
		r.warn("%s", w)
	}
	app := plan.Site.App
	if wp && site.PHPVersion != "" {
		app = "wordpress"
		if err := h.MarkWordPress(ctx, site, lg); err != nil {
			r.warn("%v", err)
		}
		if wpDB != nil && oldRoot != "" && oldRoot != newPublic {
			if err := h.ReplaceInDB(ctx, site, oldRoot, newPublic, lg); err != nil {
				r.warn("replacing the old folder in the database failed: %v", err)
			}
		}
	}
	r.res.Sites = append(r.res.Sites, ResultSite{ID: site.ID, Domain: site.Domain, App: app})
	for _, d := range dbs {
		db := made[d.Source]
		r.res.Databases = append(r.res.Databases, ResultDB{ID: db.ID, Name: db.Name, User: db.Username, From: d.Name,
			Password: "new", Site: site.Domain, Manual: wpDB == nil || wpDB.ID != db.ID})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if target != nil && target.Disabled {
		// It was suspended before: it stays so.
		h.SetNote(ctx, site, note)
	} else {
		r.finish(ctx, []*store.Site{site}, note)
	}
	lg.Logf("The import is done")
	return r.res, nil
}

// emptyFolder removes what an import put into an existing website's
// folder, and puts its placeholder page back.
func emptyFolder(public string, placeholder []byte, uid, gid int) {
	root, err := os.OpenRoot(public)
	if err != nil {
		return
	}
	defer root.Close()
	d, err := root.Open(".")
	if err != nil {
		return
	}
	names, _ := d.Readdirnames(-1)
	d.Close()
	for _, n := range names {
		root.RemoveAll(n)
	}
	if placeholder != nil && root.WriteFile("index.html", placeholder, 0o640) == nil && uid >= 0 {
		root.Lchown("index.html", uid, gid)
	}
}
