package siteimport

import (
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/validate"
)

// Logger receives progress messages for a job log.
type Logger interface {
	Logf(format string, args ...any)
}

// Kinds of import.
const (
	KindCPanel  = "cpanel"
	KindArchive = "archive"
)

// Plan is what an analysis found and what an import would do with it,
// for the administrator to choose from. Codes (Blocked, Problem, Note)
// are for the UI to translate.
type Plan struct {
	Kind      string      `json:"kind"`
	Archive   *Archive    `json:"archive"`
	User      string      `json:"user,omitempty"` // the cPanel account
	Domains   []*Domain   `json:"domains"`
	Databases []*Database `json:"databases"`
	Cron      []*CronLine `json:"cron"`
	Skipped   []Skipped   `json:"skipped"`
	Notes     []Note      `json:"notes"`
	// Site is the website of a site archive.
	Site *ArchiveSite `json:"site,omitempty"`

	DBReady bool     `json:"db_ready"` // MariaDB runs
	PHP     []string `json:"php"`      // installed PHP versions
	// Free bytes for the websites' files and for MariaDB (-1: unknown),
	// and whether both are on the same disk.
	Free     int64 `json:"free"`
	FreeDB   int64 `json:"free_db"`
	SameDisk bool  `json:"same_disk"`
}

// Note is something the administrator should know.
type Note struct {
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}

// Domain is a website of a cPanel account: it becomes one Nefo website.
type Domain struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`    // main, addon or sub
	DocRoot string   `json:"docroot"` // on the old server
	Aliases []string `json:"aliases"`
	// Taken are aliases another website here already has: left out.
	Taken  []string `json:"taken_aliases,omitempty"`
	PHPOld string   `json:"php_old"` // the old server's PHP version, "" when unknown
	PHP    string   `json:"php"`     // the version here, "" for a static site
	Size   int64    `json:"size"`
	Files  int64    `json:"files"`
	App    string   `json:"app"`          // wordpress, php or static
	DB     string   `json:"db,omitempty"` // the database its wp-config.php names
	Cert   *Cert    `json:"certificate,omitempty"`
	// Nested are the other domains whose folders are inside this one's;
	// their files go to their own websites when they are imported.
	Nested []string `json:"nested,omitempty"`
	// Blocked says why it cannot be imported: invalid_domain, wildcard,
	// domain_exists, site_dir_exists, docroot_outside, docroot_missing or
	// php_missing.
	Blocked string `json:"blocked,omitempty"`

	rel string // the document root, relative to the home folder
}

// Cert is the certificate found for a domain.
type Cert struct {
	Names    []string  `json:"names"`
	Issuer   string    `json:"issuer"`
	NotAfter time.Time `json:"not_after"`
	Trusted  bool      `json:"trusted"`
	Usable   bool      `json:"usable"`
	// Problem is no_key, expired, not_yet_valid or names (it does not cover
	// every name of the website: Missing).
	Problem string   `json:"problem,omitempty"`
	Missing []string `json:"missing,omitempty"`
}

// Database is a database to bring over.
type Database struct {
	Name    string   `json:"name"`     // on the old server
	NewName string   `json:"new_name"` // here
	Size    int64    `json:"size"`     // of the dump, unpacked (about, for a .sql.gz)
	Users   []string `json:"users"`    // that could use it on the old server
	User    string   `json:"user"`     // its user here
	// Password is "kept" (the old password still works: its hash was kept),
	// "known" (kept, and the panel knows it) or "new".
	Password string   `json:"password"`
	Site     string   `json:"site,omitempty"` // the website it is linked to
	UsedBy   []string `json:"used_by"`
	// ConfigChange: the websites' configuration must name the new name,
	// user or password. Nefo rewrites WordPress's wp-config.php.
	ConfigChange bool   `json:"config_change"`
	Source       string `json:"source,omitempty"` // a separate dump file
	Blocked      string `json:"blocked,omitempty"`

	plaintext string // the password, when known
	hash      string // the mysql_native_password hash to keep
	format    string // of a separate dump
}

// Skipped is something in the backup that is not imported.
type Skipped struct {
	What  string `json:"what"` // mail, dns, ftp, lists, postgres, other_files, logs
	Size  int64  `json:"size,omitempty"`
	Count int    `json:"count,omitempty"`
}

// ArchiveSite is the website in a site archive.
type ArchiveSite struct {
	// Root is the folder of the archive that becomes the website's public
	// folder ("" for the whole archive).
	Root    string `json:"root"`
	Files   int64  `json:"files"`
	Size    int64  `json:"size"`
	App     string `json:"app"`
	PHP     string `json:"php"`              // suggested
	Domain  string `json:"domain,omitempty"` // suggested by the archive's name
	OldRoot string `json:"old_root,omitempty"`
	DB      string `json:"db,omitempty"` // the database its wp-config.php names
}

// Selection is what the administrator chose from a plan.
type Selection struct {
	Domains      []string `json:"domains"`
	Databases    []string `json:"databases"` // names on the old server (sources for an archive)
	Certificates []string `json:"certificates"`
	Cron         []int    `json:"cron"`
	// For a site archive: a new website (Domain, with www. as an alias if
	// WWW) or an existing empty one (SiteID), and its PHP version.
	Domain string `json:"domain"`
	WWW    bool   `json:"www"`
	SiteID int64  `json:"site_id"`
	PHP    string `json:"php"`
}

// Env is what a plan needs to know about this server.
type Env struct {
	PHP         []string // installed PHP versions, oldest first
	DefaultPHP  string
	DomainTaken func(name string) bool     // another website has the name
	DirExists   func(domain string) bool   // /var/www/<domain> exists
	Root        func(domain string) string // a website's folder here (nil: /var/www/<domain>)
	DBNameFree  func(name string) bool     // no database or user has the name
	DBReady     bool
	// NoHashes: the database server cannot take an old password hash (MySQL
	// 8.4 refuses mysql_native_password by default): users get new passwords
	// unless the website's configuration holds the old one.
	NoHashes bool
	Now      time.Time
	Free     int64
	FreeDB   int64
	SameDisk bool
}

func (e Env) base(kind string, a *Archive) *Plan {
	return &Plan{Kind: kind, Archive: a, Domains: []*Domain{}, Databases: []*Database{}, Cron: []*CronLine{},
		Skipped: []Skipped{}, Notes: []Note{}, DBReady: e.DBReady, PHP: append([]string{}, e.PHP...),
		Free: e.Free, FreeDB: e.FreeDB, SameDisk: e.SameDisk}
}

var phpNameRe = regexp.MustCompile(`(?:^|[^0-9])php-?([5-9])\.?([0-9]{1,2})$`)

// oldPHP turns cPanel's ea-php81 or CloudLinux's alt-php81 into 8.1.
func oldPHP(s string) string {
	m := phpNameRe.FindStringSubmatch(strings.ToLower(strings.TrimSpace(s)))
	if m == nil {
		return ""
	}
	return m[1] + "." + m[2]
}

// choosePHP picks the PHP version here for a site that ran old (maybe ""):
// the same if it is installed, else the distribution's (if installed),
// else the newest installed. "" means none is installed.
func (e Env) choosePHP(old string) string {
	if old != "" && slices.Contains(e.PHP, old) {
		return old
	}
	if e.DefaultPHP != "" && slices.Contains(e.PHP, e.DefaultPHP) {
		return e.DefaultPHP
	}
	if len(e.PHP) > 0 {
		return e.PHP[len(e.PHP)-1]
	}
	return ""
}

// freeDBName returns name when a database can have it here, else the
// first free one of name_2, name_3…, or name made valid first.
func (e Env) freeDBName(name string, taken map[string]bool) string {
	base := dbSafe(name)
	for i := 1; i <= 50; i++ {
		n := base
		if i > 1 {
			suffix := "_" + itoa(i)
			n = strings.TrimRight(base[:min(len(base), 32-len(suffix))], "_") + suffix
		}
		if !taken[n] && validate.DBName(n) == nil && (e.DBNameFree == nil || e.DBNameFree(n)) {
			return n
		}
	}
	return ""
}

// dbSafe turns a name into one validate.DBName accepts.
func dbSafe(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		case r == '-' || r == '.' || r == ' ':
			b.WriteByte('_')
		}
	}
	n := strings.Trim(b.String(), "_")
	if n == "" || n[0] < 'a' || n[0] > 'z' {
		n = "db_" + n
	}
	if len(n) > 32 {
		n = strings.TrimRight(n[:32], "_")
	}
	if validate.DBName(n) != nil {
		n = "db_" + n
		n = strings.TrimRight(n[:min(len(n), 32)], "_")
	}
	return n
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

// cleanDomain lower-cases a name and drops a final dot.
func cleanDomain(s string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), ".")
}

func (e Env) siteRoot(domain string) string {
	if e.Root != nil {
		return strings.TrimRight(filepathToSlash(e.Root(domain)), "/")
	}
	return "/var/www/" + domain
}
