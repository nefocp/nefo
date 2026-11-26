package siteimport

import (
	"context"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
)

// CPanel is what the analysis of a cPanel full backup found. Only
// metadata is kept: names, sizes, the userdata files, mysql.sql, the
// crontab and the wp-config.php files' database settings.
type CPanel struct {
	arch    *Archive
	prefix  string // the top folder, such as "cpmove-bob/"
	user    string
	homeDir string // the account's home folder on the old server
	main    string
	addons  map[string]string // addon domain -> its cPanel subdomain
	parked  []string
	subs    []string
	vhosts  map[string]*vhost // by userdata file name and by server name
	dirs    map[string]*dirAgg
	home    dirAgg
	wp      map[string]*wpInfo // by folder, relative to the home folder
	dumps   map[string]int64   // database -> size of mysql/DB.sql
	grants  *grantSet
	cron    string
	certs   map[string]*Cert // by domain
	zones   int
	ftp     bool
	lists   int
	psql    int
	logs    dirAgg
	homeTar bool
}

type vhost struct {
	server, docroot, homeDir, user, php string
	aliases                             []string
}

type dirAgg struct{ bytes, files, php int64 }

func (d *dirAgg) add(size int64, php bool) {
	d.bytes += size
	d.files++
	if php {
		d.php++
	}
}

// dirDepth is how deep under the home folder sizes are counted: document
// roots are public_html, public_html/addon.com or domains/x/public_html.
const dirDepth = 5

// topNames are the entries a cPanel backup has at its top.
var topNames = map[string]bool{"homedir": true, "homedir.tar": true, "userdata": true, "mysql": true, "mysql.sql": true,
	"cp": true, "cron": true, "version": true, "apache_tls": true, "sslkeys": true, "sslcerts": true, "dnszones": true}

// AnalyseCPanel reads a cPanel full backup once, as a stream, and returns
// what it holds. Nothing is unpacked.
func AnalyseCPanel(ctx context.Context, f *os.File, a *Archive, lim Limits, now time.Time, lg Logger) (*CPanel, error) {
	if a.Format != FormatTarGz && a.Format != FormatTar {
		return nil, usererr.New("import_not_cpanel", a.Name)
	}
	c := &CPanel{arch: a, prefix: "\x00", addons: map[string]string{}, vhosts: map[string]*vhost{},
		dirs: map[string]*dirAgg{}, wp: map[string]*wpInfo{}, dumps: map[string]int64{}, grants: parseGrants(nil)}
	pool := &certPool{}
	w := &walker{lim: lim}
	var mainYAML []byte
	userdata := map[string][]byte{}
	var homedirPaths string
	next := time.Now().Add(5 * time.Second)
	var read int64

	var visit func(e *entry, inHome bool) error
	visit = func(e *entry, inHome bool) error {
		name, ok := cleanName(e.Name)
		if !ok || name == "" {
			return nil
		}
		if e.Kind == kindFile {
			read += e.Size
			if time.Now().After(next) {
				next = time.Now().Add(5 * time.Second)
				lg.Logf("Read %s of the archive", humanBytes(read))
			}
		}
		var p string
		if inHome {
			p = "homedir/" + name
		} else {
			if c.prefix == "\x00" {
				c.prefix = topPrefix(name)
			}
			rest, ok := strings.CutPrefix(name+"/", c.prefix)
			if !ok {
				return nil
			}
			p = strings.TrimSuffix(rest, "/")
		}
		if p == "" {
			return nil
		}
		if e.Kind == kindFile && certFile(p) && e.Size <= maxPEMFile {
			pool.add(certGroup(p), readSmall(e.R, maxPEMFile))
			return nil
		}
		if rel, ok := strings.CutPrefix(p, "homedir/"); ok {
			if e.Kind == kindFile {
				c.homeFile(rel, e)
			}
			return nil
		}
		if e.Kind != kindFile {
			return nil
		}
		dir, base := path.Split(p)
		switch {
		case p == "homedir.tar":
			c.homeTar = true
			return w.walkTar(ctx, e.R, func(e *entry) error { return visit(e, true) })
		case p == "userdata/main":
			mainYAML = readSmall(e.R, 1<<20)
		case dir == "userdata/":
			if !strings.HasSuffix(base, ".cache") && !strings.HasSuffix(base, "_SSL") && !strings.Contains(base, ".json") &&
				!strings.HasSuffix(base, ".yaml") && base != "cache" && len(userdata) < 5000 {
				if b := readSmall(e.R, 1<<20); b != nil {
					userdata[base] = b
				}
			}
		case dir == "mysql/" && strings.HasSuffix(base, ".sql"):
			c.dumps[strings.TrimSuffix(base, ".sql")] = e.Size
		case p == "mysql.sql":
			c.grants = parseGrants(readSmall(e.R, 8<<20))
		case dir == "cron/" && c.cron == "":
			c.cron = string(readSmall(e.R, 256<<10))
		case dir == "dnszones/":
			c.zones++
		case p == "proftpdpasswd":
			c.ftp = true
		case strings.HasPrefix(p, "mm/") || strings.HasPrefix(p, "mma/"):
			c.lists++
		case strings.HasPrefix(p, "psql/") && strings.HasSuffix(p, ".tar"):
			c.psql++
		case strings.HasPrefix(p, "logs/"):
			c.logs.add(e.Size, false)
		case p == "homedir_paths":
			homedirPaths = strings.TrimSpace(strings.SplitN(string(readSmall(e.R, 4096)), "\n", 2)[0])
		case dir == "cp/" && c.user == "":
			c.user = base
		}
		return nil
	}
	if err := w.walk(ctx, f, a, func(e *entry) error { return visit(e, false) }); err != nil {
		return nil, err
	}
	if mainYAML == nil {
		return nil, usererr.New("import_not_cpanel", a.Name)
	}
	if err := c.readUserdata(mainYAML, userdata); err != nil {
		return nil, err
	}
	if c.homeDir == "" {
		c.homeDir = homedirPaths
	}
	if c.homeDir == "" && c.user != "" {
		c.homeDir = "/home/" + c.user
	}
	c.homeDir = strings.TrimRight(path.Clean("/"+c.homeDir), "/")
	c.certs = map[string]*Cert{}
	for _, d := range c.domainNames() {
		if p := pool.best(d.name, d.names, now); p != nil {
			c.certs[d.name] = p.summary
		}
	}
	lg.Logf("Read the whole archive: %s, %d websites, %d database dumps", humanBytes(read), len(c.domainNames()), len(c.dumps))
	return c, nil
}

// topPrefix finds the top folder from the first entry's name.
func topPrefix(name string) string {
	first, _, _ := strings.Cut(name, "/")
	if topNames[first] {
		return ""
	}
	return first + "/"
}

// homeFile counts a file of the home folder and keeps what the plan needs.
func (c *CPanel) homeFile(rel string, e *entry) {
	php := strings.HasSuffix(strings.ToLower(rel), ".php")
	c.home.add(e.Size, php)
	for _, d := range ancestors(rel, dirDepth) {
		a := c.dirs[d]
		if a == nil {
			a = &dirAgg{}
			c.dirs[d] = a
		}
		a.add(e.Size, php)
	}
	if path.Base(rel) == "wp-config.php" && depth(rel) <= dirDepth+1 && e.Size <= maxConfig && len(c.wp) < 500 {
		if b := readSmall(e.R, maxConfig); b != nil {
			dir := path.Dir(rel)
			if dir == "." {
				dir = ""
			}
			c.wp[dir] = parseWPConfig(b)
		}
	}
}

func (c *CPanel) readUserdata(mainYAML []byte, files map[string][]byte) error {
	v, err := parseYAML(mainYAML)
	if err != nil {
		return usererr.New("import_not_cpanel", "userdata/main")
	}
	m := yMap(v)
	c.main = cleanDomain(yString(m, "main_domain"))
	if c.main == "" {
		return usererr.New("import_not_cpanel", "userdata/main")
	}
	if ad := yMap(m["addon_domains"]); ad != nil {
		for k, v := range ad {
			s, _ := v.(string)
			c.addons[cleanDomain(k)] = cleanDomain(s)
		}
	}
	for _, p := range yStrings(m, "parked_domains") {
		c.parked = append(c.parked, cleanDomain(p))
	}
	for _, s := range yStrings(m, "sub_domains") {
		c.subs = append(c.subs, cleanDomain(s))
	}
	sort.Strings(c.parked)
	sort.Strings(c.subs)
	for name, b := range files {
		v, err := parseYAML(b)
		if err != nil {
			continue
		}
		m := yMap(v)
		vh := &vhost{server: cleanDomain(yString(m, "servername")), docroot: yString(m, "documentroot"),
			homeDir: yString(m, "homedir"), user: yString(m, "user"), php: yString(m, "phpversion")}
		for _, a := range yStrings(m, "serveralias") {
			vh.aliases = append(vh.aliases, cleanDomain(a))
		}
		if vh.docroot == "" {
			continue
		}
		c.vhosts[cleanDomain(name)] = vh
		if vh.server != "" {
			if _, ok := c.vhosts[vh.server]; !ok {
				c.vhosts[vh.server] = vh
			}
		}
	}
	if vh := c.vhosts[c.main]; vh != nil {
		if vh.user != "" {
			c.user = vh.user
		}
		if vh.homeDir != "" {
			c.homeDir = vh.homeDir
		}
	}
	return nil
}

// cpDomain is a website of the account before planning.
type cpDomain struct {
	name, kind string
	vh         *vhost
	names      []string // the name and the aliases it had
}

// domainNames lists the account's websites: the main domain (with the
// parked domains as aliases), the addon domains (without cPanel's internal
// subdomain for them) and the other subdomains.
func (c *CPanel) domainNames() []cpDomain {
	var out []cpDomain
	internal := map[string]bool{}
	for _, s := range c.addons {
		internal[s] = true
	}
	add := func(name, kind string, vh *vhost, extra []string) {
		if vh == nil {
			vh = &vhost{}
		}
		// www. and the parked domains (with their www. when the old server
		// had it); not cPanel's own names (mail., cpanel., webmail.…).
		names := []string{name}
		if contains(vh.aliases, "www."+name) {
			names = append(names, "www."+name)
		}
		for _, p := range extra {
			if !contains(names, p) {
				names = append(names, p)
			}
			if contains(vh.aliases, "www."+p) && !contains(names, "www."+p) {
				names = append(names, "www."+p)
			}
		}
		out = append(out, cpDomain{name: name, kind: kind, vh: vh, names: names})
	}
	add(c.main, "main", c.vhosts[c.main], c.parked)
	var addons []string
	for a := range c.addons {
		addons = append(addons, a)
	}
	sort.Strings(addons)
	for _, a := range addons {
		vh := c.vhosts[c.addons[a]]
		if vh == nil {
			vh = c.vhosts[a]
		}
		add(a, "addon", vh, nil)
	}
	for _, s := range c.subs {
		if internal[s] || s == c.main {
			continue
		}
		add(s, "sub", c.vhosts[s], nil)
	}
	return out
}

// relDocroot returns a document root relative to the home folder.
func (c *CPanel) relDocroot(docroot string) (string, bool) {
	d := path.Clean("/" + docroot)
	rel, ok := strings.CutPrefix(d, c.homeDir+"/")
	if !ok || rel == "" {
		return "", false
	}
	if clean, ok := cleanName(rel); ok && clean != "" {
		return clean, true
	}
	return "", false
}

// Plan says what an import of the backup would do on this server.
func (c *CPanel) Plan(env Env) *Plan {
	p := env.base(KindCPanel, c.arch)
	p.User = c.user
	// Domains.
	for _, d := range c.domainNames() {
		dom := &Domain{Name: d.name, Type: d.kind, DocRoot: d.vh.docroot, PHPOld: oldPHP(d.vh.php), Aliases: []string{}}
		for _, a := range d.names[1:] {
			switch {
			case validate.Alias(a) != nil || validate.IsWildcard(a):
			case env.DomainTaken != nil && env.DomainTaken(a):
				dom.Taken = append(dom.Taken, a)
			case len(dom.Aliases) < 20:
				dom.Aliases = append(dom.Aliases, a)
			}
		}
		rel, ok := c.relDocroot(d.vh.docroot)
		dom.rel = rel
		agg := c.dirs[rel]
		switch {
		case validate.IsWildcard(d.name):
			dom.Blocked = "wildcard"
		case validate.Domain(d.name) != nil:
			dom.Blocked = "invalid_domain"
		case env.DomainTaken != nil && env.DomainTaken(d.name):
			dom.Blocked = "domain_exists"
		case env.DirExists != nil && env.DirExists(d.name):
			dom.Blocked = "site_dir_exists"
		case d.vh.docroot == "" || !ok:
			dom.Blocked = "docroot_outside"
		case agg == nil:
			dom.Blocked = "docroot_missing"
		}
		if agg != nil {
			dom.Size, dom.Files = agg.bytes, agg.files
		}
		if w := c.wp[rel]; w != nil && ok {
			dom.App, dom.DB = "wordpress", w.DBName
		} else if agg != nil && agg.php > 0 {
			dom.App = "php"
		} else {
			dom.App = "static"
		}
		if dom.App != "static" || dom.PHPOld != "" {
			dom.PHP = env.choosePHP(dom.PHPOld)
			if dom.PHP == "" && dom.App != "static" && dom.Blocked == "" {
				dom.Blocked = "php_missing"
			}
		}
		if ct := c.certs[d.name]; ct != nil {
			cp := *ct
			cp.Missing = nil
			for _, n := range append([]string{dom.Name}, dom.Aliases...) {
				if !certCovers(ct, n) {
					cp.Missing = append(cp.Missing, n)
				}
			}
			if cp.Problem == "names" || cp.Problem == "" {
				cp.Problem = ""
				if len(cp.Missing) > 0 {
					cp.Problem = "names"
				}
			}
			cp.Usable = cp.Problem == ""
			dom.Cert = &cp
		}
		p.Domains = append(p.Domains, dom)
	}
	// Nested folders: a domain's own size leaves out the domains inside it.
	for _, d := range p.Domains {
		for _, o := range p.Domains {
			if o != d && d.rel != "" && strings.HasPrefix(o.rel, d.rel+"/") && o.Blocked == "" {
				d.Nested = append(d.Nested, o.Name)
				d.Size -= o.Size
				d.Files -= o.Files
			}
		}
		d.Size, d.Files = max(d.Size, 0), max(d.Files, 0)
	}
	c.planDatabases(p, env)
	// Cron.
	roots := map[string][2]string{}
	php := map[string]string{}
	for _, d := range p.Domains {
		if d.DocRoot != "" && d.Blocked == "" {
			roots[strings.TrimRight(d.DocRoot, "/")] = [2]string{d.Name, env.siteRoot(d.Name) + "/public"}
		}
		php[d.Name] = d.PHP
	}
	if c.cron != "" {
		p.Cron = cronLines(c.cron, c.homeDir, c.main, roots, php)
	}
	// What stays behind.
	var inSites int64
	for _, d := range p.Domains {
		if d.Blocked == "" {
			inSites += d.Size
		}
	}
	mail := sumDirs(c.dirs, "mail", "etc")
	if mail.files > 0 {
		p.Skipped = append(p.Skipped, Skipped{What: "mail", Size: mail.bytes, Count: int(mail.files)})
	}
	if c.zones > 0 {
		p.Skipped = append(p.Skipped, Skipped{What: "dns", Count: c.zones})
	}
	if ftp := sumDirs(c.dirs, "public_ftp"); c.ftp || ftp.files > 0 {
		p.Skipped = append(p.Skipped, Skipped{What: "ftp", Size: ftp.bytes})
	}
	if c.lists > 0 {
		p.Skipped = append(p.Skipped, Skipped{What: "lists", Count: c.lists})
	}
	if c.psql > 0 {
		p.Skipped = append(p.Skipped, Skipped{What: "postgres", Count: c.psql})
	}
	if c.logs.files > 0 {
		p.Skipped = append(p.Skipped, Skipped{What: "logs", Size: c.logs.bytes, Count: int(c.logs.files)})
	}
	if other := c.home.bytes - inSites - mail.bytes; other > 0 {
		p.Skipped = append(p.Skipped, Skipped{What: "other_files", Size: other})
	}
	if c.homeTar {
		p.Notes = append(p.Notes, Note{Code: "old_format"})
	}
	return p
}

func sumDirs(dirs map[string]*dirAgg, names ...string) dirAgg {
	var s dirAgg
	for _, n := range names {
		if d := dirs[n]; d != nil {
			s.bytes += d.bytes
			s.files += d.files
		}
	}
	return s
}

func certCovers(c *Cert, name string) bool {
	for _, n := range c.Names {
		if n == name || strings.HasPrefix(n, "*.") && strings.Count(name, ".") == strings.Count(n, ".") && strings.HasSuffix(name, n[1:]) {
			return true
		}
	}
	return false
}

// planDatabases decides, for each dump, its name and user here. Names are
// kept when they are free, so the websites' configuration keeps working;
// so is the password, through the old server's mysql_native_password hash.
func (c *CPanel) planDatabases(p *Plan, env Env) {
	var names []string
	for n := range c.dumps {
		names = append(names, n)
	}
	sort.Strings(names)
	// Database and user names share one name space here (see
	// store.DBNameTaken).
	used := map[string]bool{}
	for _, name := range names {
		db := &Database{Name: name, Size: c.dumps[name], Users: c.grants.usersOf(name), UsedBy: []string{}}
		if db.Users == nil {
			db.Users = []string{}
		}
		// The websites whose wp-config.php names it, and the user and
		// password they use.
		var cfg *wpInfo
		for _, d := range p.Domains {
			if w := c.wp[d.rel]; w != nil && d.rel != "" && w.DBName == name && localHost(w.DBHost) {
				db.UsedBy = append(db.UsedBy, d.Name)
				if cfg == nil {
					cfg = w
				}
			}
		}
		switch {
		case len(db.UsedBy) > 0:
			db.Site = db.UsedBy[0]
		case len(p.Domains) > 0:
			db.Site = p.Domains[0].Name
		}
		db.NewName = name
		if validate.DBName(name) != nil || used[name] || (env.DBNameFree != nil && !env.DBNameFree(name)) {
			db.NewName = env.freeDBName(name, used)
			if db.NewName == "" {
				db.Blocked = "no_name"
			}
		}
		used[db.NewName] = true
		// The user: the one the website's configuration names (no other
		// would work with its password); without one, a user granted this
		// database by name, else any with access, the account's own last.
		var candidates []string
		if cfg != nil && cfg.DBUser != "" {
			candidates = append(candidates, cfg.DBUser)
		} else {
			users := append([]string{}, db.Users...)
			sort.SliceStable(users, func(i, j int) bool {
				return c.grants.explicit(users[i], name) && !c.grants.explicit(users[j], name)
			})
			for _, u := range users {
				if u != c.user {
					candidates = append(candidates, u)
				}
			}
			if contains(db.Users, c.user) {
				candidates = append(candidates, c.user)
			}
		}
		db.Password = "new"
		eligible := func(u string) bool {
			return validate.DBName(u) == nil && (!used[u] || u == db.NewName) &&
				(env.DBNameFree == nil || u == db.NewName || env.DBNameFree(u))
		}
		// keep says how u's password carries over: "known" (the website's
		// own), "kept" (its hash), or "" (a new one is needed).
		keep := func(u string) (how, hash string) {
			hash = c.grants.nativeHash(u)
			if env.NoHashes {
				hash = ""
			}
			known := cfg != nil && cfg.DBUser == u && cfg.DBPassword != ""
			switch {
			case known && validate.DBPassword(cfg.DBPassword) == nil:
				// The website's own password: it works whatever the hash.
				return "known", ""
			case known && hash != "" && NativeHash(cfg.DBPassword) == hash:
				// A password the panel cannot store (too short, or with
				// quotes): the hash keeps it.
				return "kept", hash
			case !known && hash != "":
				return "kept", hash
			}
			return "", ""
		}
		// The first user whose password carries over, else the first one
		// (with a new password): a user with a MySQL 8 hash would otherwise
		// shut out a later one whose password still works.
		chosen := ""
		for _, u := range candidates {
			if how, _ := keep(u); eligible(u) && how != "" {
				chosen = u
				break
			}
		}
		for _, u := range candidates {
			if chosen == "" && eligible(u) {
				chosen = u
			}
		}
		if chosen != "" {
			db.User = chosen
			switch how, hash := keep(chosen); how {
			case "known":
				db.Password, db.plaintext = "known", cfg.DBPassword
			case "kept":
				db.Password, db.hash = "kept", hash
			}
		}
		if db.Password == "new" && (len(candidates) == 0 || db.User != candidates[0] || cfg == nil) {
			// With a new password, an old user name only helps a
			// configuration that names it.
			db.User = db.NewName
		}
		if db.User == "" {
			db.User = db.NewName
		}
		used[db.User] = true
		db.ConfigChange = db.NewName != name || db.Password == "new" || (cfg != nil && cfg.DBUser != db.User)
		p.Databases = append(p.Databases, db)
	}
}

// dumpPath is where a database's dump is in the backup.
func dumpPath(name string) string { return "mysql/" + name + ".sql" }
