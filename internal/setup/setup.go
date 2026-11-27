// Package setup is the first setup of a server: the web server, PHP
// versions, a database server and a few extras, chosen on one page of the
// panel (or with "nefo setup") and installed one after another as one job.
//
// It only decides what to offer, plans and runs the steps in order. The
// installs themselves are the Software page's own install functions,
// reached through Installer.
package setup

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/nefocp/nefo/internal/usererr"
)

// The wizard's state is the setting StateKey: "" until the setup has run
// to the end once (StateDone) or was skipped (StateSkipped).
const (
	StateKey     = "setup_wizard"
	StateDone    = "done"
	StateSkipped = "skipped"
)

// The web servers, by the names /api/web/install takes.
const (
	Nginx         = "nginx"
	Apache        = "apache"
	OpenLiteSpeed = "openlitespeed"
)

// WebServers are the web servers in the order they are offered.
var WebServers = []string{Nginx, Apache, OpenLiteSpeed}

// Databases are the database servers a server can have one of, by their
// Software page IDs. MariaDB is the recommended one: the distribution's
// ("mariadb"), or a series from MariaDB's repository ("mariadb-11.4"),
// offered when the Software page lists it (see IsDatabase).
var Databases = []string{"mariadb", "mysql"}

// IsDatabase reports whether id is a database server: one of Databases or
// a MariaDB series such as "mariadb-11.4".
func IsDatabase(id string) bool {
	return slices.Contains(Databases, id) || strings.HasPrefix(id, "mariadb-")
}

// databaseIDs are the database servers offered, in the order the setup
// shows them: MariaDB, its series from the catalog, MySQL.
func databaseIDs(catalog []string) []string {
	ids := []string{"mariadb"}
	for _, id := range catalog {
		if strings.HasPrefix(id, "mariadb-") && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return append(ids, Databases[1:]...)
}

// baseID is the part whose figures a MariaDB series shares ("mariadb").
func baseID(id string) string {
	if strings.HasPrefix(id, "mariadb-") {
		return "mariadb"
	}
	return id
}

// Extras are the other parts, by their Software page IDs, in the order a
// plan installs them: Adminer and the web firewall after the web server
// and PHP, ClamAV last (its databases download in the background).
// Parts the Software page does not list (Facts.Catalog) are not offered.
var Extras = []string{"postgresql", "redis", "memcached", "nodejs", "python", "docker", "adminer", "waf", "clamav"}

// ClamAVMinMB is the RAM from which ClamAV is offered: it needs about
// 1.4 GB of free memory while it scans, so it suits servers with 2 GB or
// more (a "2 GB" server reports a little less).
const ClamAVMinMB = 1800

// Redis comes with the recommended choice from this much RAM.
const redisDefaultMB = 1500

var names = map[string]string{
	Nginx: "Nginx", Apache: "Apache", OpenLiteSpeed: "OpenLiteSpeed",
	"mariadb": "MariaDB", "mysql": "MySQL", "postgresql": "PostgreSQL", "redis": "Redis", "memcached": "Memcached",
	"nodejs": "Node.js", "python": "Python", "docker": "Docker",
	"adminer": "Adminer", "waf": "Web firewall (ModSecurity)", "clamav": "ClamAV",
}

// Name is the English name of a part, for logs: "Nginx", "PHP 8.4".
func Name(id string) string {
	if v, ok := strings.CutPrefix(id, "php"); ok {
		return "PHP " + v
	}
	if n, ok := names[id]; ok {
		return n
	}
	if v, ok := strings.CutPrefix(id, "mariadb-"); ok {
		return "MariaDB " + v
	}
	return id
}

// Rough costs, in MB. Memory is what a part uses while idle: the Software
// page's figures (web/src/webservers.js for the web servers), which Facts.RAM
// overrides where the Software page has one for this server (MariaDB,
// Redis…). The web firewall's is the measured growth of Nginx when it
// loads the rules.
//
// Disk is what a part added to the root file system when it was installed
// in a fresh sandbox of Ubuntu 24.04 (amd64) on 2026-11-27, rounded up: the
// installed packages, the copies of the downloaded packages that apt keeps
// in /var/cache/apt/archives (until "apt-get clean" or the System page's
// cleanup), apt's grown caches, the database servers' first data files, the
// unpacked Node.js release and ClamAV's virus databases. diskByRelease has
// the releases whose figures differ by more than about 10%. The
// measurements are in the runtimes agent's NOTES (batch 5).
var (
	ramMB = map[string]int{
		Nginx: 15, Apache: 40, OpenLiteSpeed: 26, "php": 25,
		"mariadb": 228, "mysql": 360, "postgresql": 40, "redis": 64, "memcached": 64,
		"docker": 90, "waf": 115, "clamav": 15,
	}
	// diskMB is what each part adds to the disk (apt's download and the
	// installed files), measured in sandboxes of the five releases on
	// 2026-11-27; a minimal server image needs about 130 MB for PHP.
	diskMB = map[string]int{
		Nginx: 5, Apache: 75, OpenLiteSpeed: 150, "php": 130,
		"mariadb": 400, "mysql": 470, "postgresql": 280, "redis": 12, "memcached": 3,
		"nodejs": 230, "python": 35, "docker": 420, "adminer": 1, "waf": 15, "clamav": 180,
	}
	// diskByRelease are the releases whose figure differs from diskMB by
	// more than about 10% (Ubuntu 24.04 images usually have PHP's libraries
	// already).
	diskByRelease = map[string]map[string]int{
		"jammy":    {"mariadb": 300},
		"noble":    {"php": 60},
		"resolute": {"mariadb": 360, "postgresql": 100},
		"bookworm": {"php": 150, "docker": 320},
		"trixie":   {"php": 145, "postgresql": 340, "docker": 290},
	}
	// aptListsMB is what the first "apt-get update" adds on a server whose
	// package lists were never downloaded (or were deleted), by release,
	// then by distribution: the Packages and Translation-en lists of the
	// release's default sources (sizes from their Release files, 2026-11-27)
	// and apt's caches built from them (measured: 62-78% of the Packages
	// lists).
	aptListsMB = map[string]int{
		"jammy": 450, "noble": 300, "resolute": 250, "bookworm": 155, "trixie": 175,
		"ubuntu": 300, "debian": 175,
	}
	groups = map[string]string{
		"mariadb": "db", "mysql": "db",
		"postgresql": "data", "redis": "data", "memcached": "data", "adminer": "data",
		"nodejs": "runtimes", "python": "runtimes", "docker": "runtimes",
		"waf": "security", "clamav": "security",
	}
)

// Server describes the machine.
type Server struct {
	CPUs        int    `json:"cpus"`
	MemTotalMB  int    `json:"mem_total_mb"` // 0 when unknown
	MemUsedMB   int    `json:"mem_used_mb"`
	DiskTotalMB int    `json:"disk_total_mb"` // 0 when unknown
	DiskFreeMB  int    `json:"disk_free_mb"`
	OS          string `json:"os"`
	Arch        string `json:"arch"` // GOARCH: amd64, arm64…
}

// RAMBudgetMB is how much memory the setup may add before it warns: up to
// 85% of the RAM in use, the rest left for PHP workers, the websites and
// the system's cache. ok is false when the RAM is unknown.
func (s Server) RAMBudgetMB() (mb int, ok bool) {
	if s.MemTotalMB <= 0 {
		return 0, false
	}
	return s.MemTotalMB*85/100 - s.MemUsedMB, true
}

// DiskBudgetMB is how much disk the setup may take before it warns,
// keeping 1 GB free. ok is false when the disk is unknown.
func (s Server) DiskBudgetMB() (mb int, ok bool) {
	if s.DiskTotalMB <= 0 {
		return 0, false
	}
	return s.DiskFreeMB - 1024, true
}

// Reason is why a part can't be chosen: an error code the UI translates
// (such as web_server_conflict) and its detail.
type Reason struct {
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}

// Facts are what the panel knows about the server; NewEnv turns them into
// what the setup offers.
type Facts struct {
	Supported bool
	Server    Server
	// WebServer is the web server the panel manages, "" for none.
	WebServer string
	// DistroPHP is the distribution's PHP version, which every web server
	// installs with it ("" if apt could not tell).
	DistroPHP string
	// PHPRepo is where the other PHP versions come from, for the UI:
	// "ppa:ondrej/php" or "packages.sury.org".
	PHPRepo string
	// PHPVersions are the versions the Software page offers.
	PHPVersions []string
	// Catalog are the IDs of the Software page's items: databases and
	// extras it does not list are not offered. nil offers all of them.
	Catalog []string
	// OLS: LiteSpeed's repository has OpenLiteSpeed for this release.
	OLS bool
	// Installed, Blocked and EOL are by part ID ("nginx", "php8.3",
	// "redis"). EOL marks PHP versions past their end of life: they are
	// not offered, and an installed one is never recommended.
	Installed map[string]bool
	Blocked   map[string]Reason
	EOL       map[string]bool
	// RAM holds the Software page's memory figures where it has them, and
	// RAMMax which of them are configured upper limits (Redis).
	RAM    map[string]int
	RAMMax map[string]bool
	// OSID and Codename are from /etc/os-release ("ubuntu", "noble"), for
	// the disk figures of this release. AptListsMissing: apt's package
	// lists were never downloaded, so the first install downloads them.
	OSID, Codename  string
	AptListsMissing bool
	// Versions are the Software page's versions of the database servers
	// ("10.11", "11.4"), and Until the end of MariaDB's support for a
	// series of its repository.
	Versions map[string]string
	Until    map[string]string
}

// Part is one thing the setup can install.
type Part struct {
	ID        string `json:"id"`    // "nginx", "php8.4", "mariadb"
	Group     string `json:"group"` // web, php, db, data, runtimes, security
	Version   string `json:"version,omitempty"`
	Installed bool   `json:"installed"`
	// RAMMB and DiskMB are roughly what it adds. A web server's include the
	// distribution's PHP (PHP) when that is not installed yet.
	RAMMB  int    `json:"ram_mb"`
	RAMMax bool   `json:"ram_max,omitempty"`
	DiskMB int    `json:"disk_mb"`
	PHP    string `json:"php,omitempty"`
	// ThirdParty: a PHP version from Env.PHPRepo, which the install adds.
	ThirdParty bool `json:"third_party,omitempty"`
	// EOL: a PHP version past its end of life.
	EOL bool `json:"eol,omitempty"`
	// NeedsWeb: it can only be installed with a web server.
	NeedsWeb bool    `json:"needs_web,omitempty"`
	Blocked  *Reason `json:"blocked,omitempty"`
	// Until is the end of support of a MariaDB series from MariaDB's
	// repository (its Version is the series).
	Until string `json:"until,omitempty"`
}

// Env is what the setup offers on this server, and what a plan depends on.
type Env struct {
	Supported bool   `json:"supported"`
	Server    Server `json:"server"`
	WebServer string `json:"web_server"`
	DistroPHP string `json:"distro_php"`
	PHPRepo   string `json:"php_repo"`
	Parts     []Part `json:"parts"`
	// AptListsMB is the disk the first install adds for apt's package
	// lists, when they were never downloaded (0 otherwise).
	AptListsMB int `json:"apt_lists_mb,omitempty"`
}

// NewEnv works out what the setup offers: the Software page's parts that
// this server supports. OpenLiteSpeed needs LiteSpeed's packages for this
// release and an amd64 or arm64 CPU, Node.js an amd64 or arm64 CPU, and
// ClamAV about 2 GB of RAM. A server has one database server: once MariaDB
// or MySQL is installed, the other can't be chosen.
func NewEnv(f Facts) Env {
	e := Env{Supported: f.Supported, Server: f.Server, WebServer: f.WebServer, DistroPHP: f.DistroPHP, PHPRepo: f.PHPRepo, Parts: []Part{}}
	if !f.Supported {
		return e
	}
	if f.AptListsMissing {
		e.AptListsMB = aptListsMB[f.OSID]
		if mb, ok := aptListsMB[f.Codename]; ok {
			e.AptListsMB = mb
		}
	}
	disk := func(id string) int {
		if mb, ok := diskByRelease[f.Codename][id]; ok {
			return mb
		}
		return diskMB[id]
	}
	bigCPU := f.Server.Arch == "amd64" || f.Server.Arch == "arm64"
	listed := func(id string) bool { return f.Catalog == nil || slices.Contains(f.Catalog, id) }
	ram := func(id string) int {
		if mb, ok := f.RAM[id]; ok && mb > 0 {
			return mb
		}
		key := baseID(id)
		if strings.HasPrefix(id, "php") {
			key = "php"
		}
		return ramMB[key]
	}
	part := func(id, group string) Part {
		p := Part{ID: id, Group: group, Installed: f.Installed[id], RAMMB: ram(id), RAMMax: f.RAMMax[id], DiskMB: disk(baseID(id))}
		if r, ok := f.Blocked[id]; ok && r.Code != "" {
			p.Blocked = &r
		}
		return p
	}
	distroPHPInstalled := f.DistroPHP != "" && f.Installed["php"+f.DistroPHP]
	for _, id := range WebServers {
		if id == OpenLiteSpeed && !(f.OLS && bigCPU) && !f.Installed[id] {
			continue
		}
		p := part(id, "web")
		if f.DistroPHP != "" {
			p.PHP = f.DistroPHP
			if !distroPHPInstalled && !p.Installed {
				p.RAMMB += ram("php" + f.DistroPHP)
				p.DiskMB += disk("php")
			}
		}
		e.Parts = append(e.Parts, p)
	}
	versions := slices.Clone(f.PHPVersions)
	slices.SortFunc(versions, compareVersions)
	for _, v := range slices.Compact(versions) {
		// Old versions past their end of life are for old websites: the
		// Software page installs them, after a warning.
		if f.EOL["php"+v] && !f.Installed["php"+v] {
			continue
		}
		p := part("php"+v, "php")
		p.Version, p.DiskMB = v, disk("php")
		p.ThirdParty = f.DistroPHP != "" && v != f.DistroPHP
		p.EOL = f.EOL[p.ID]
		e.Parts = append(e.Parts, p)
	}
	installedDB := ""
	dbs := databaseIDs(f.Catalog)
	for _, id := range dbs {
		if f.Installed[id] {
			installedDB = id
		}
	}
	for _, id := range dbs {
		if !listed(id) && !f.Installed[id] {
			continue
		}
		p := part(id, "db")
		p.Version, p.Until = f.Versions[id], f.Until[id]
		if installedDB != "" && installedDB != id && p.Blocked == nil {
			p.Blocked = &Reason{Code: "setup_db_conflict", Detail: Name(installedDB)}
		}
		e.Parts = append(e.Parts, p)
	}
	for _, id := range Extras {
		switch {
		case f.Installed[id]:
		case !listed(id):
			continue
		case id == "nodejs" && !bigCPU:
			continue
		case id == "clamav" && f.Server.MemTotalMB < ClamAVMinMB:
			continue
		}
		p := part(id, groups[id])
		p.NeedsWeb = id == "adminer" || id == "waf"
		e.Parts = append(e.Parts, p)
	}
	return e
}

// Part returns the part with this ID, if the server offers it.
func (e Env) Part(id string) (Part, bool) {
	for _, p := range e.Parts {
		if p.ID == id {
			return p, true
		}
	}
	return Part{}, false
}

// compareVersions orders "7.4" < "8.0" < "8.10".
func compareVersions(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			return x - y
		}
	}
	return strings.Compare(a, b)
}

// Choice is what the administrator picked.
type Choice struct {
	// Web is "nginx", "apache", "openlitespeed", or "" (or "none") for no
	// web server.
	Web string `json:"web"`
	// PHP are versions such as "8.4". The web server brings the
	// distribution's version with it whether or not it is listed.
	PHP []string `json:"php"`
	// DB is "mariadb", "mysql", or "" (or "none").
	DB string `json:"db"`
	// Extras are IDs from Extras. A database server listed here counts as DB.
	Extras []string `json:"extras"`
}

func none(s string) bool { return s == "" || s == "none" }

// Empty reports whether nothing was chosen.
func (c Choice) Empty() bool {
	return none(c.Web) && none(c.DB) && len(c.PHP) == 0 && len(c.Extras) == 0
}

// Step is one install of a plan.
type Step struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Installed is whether it was installed when the plan was made; Run
	// looks again before each step.
	Installed  bool `json:"installed"`
	RAMMB      int  `json:"ram_mb"`
	RAMMax     bool `json:"ram_max,omitempty"`
	DiskMB     int  `json:"disk_mb"`
	ThirdParty bool `json:"third_party,omitempty"`
}

// Plan is the installs of a choice, in order.
type Plan struct {
	Steps []Step `json:"steps"`
	// RAMMB and DiskMB add up the steps that are not installed yet.
	RAMMB  int `json:"ram_mb"`
	DiskMB int `json:"disk_mb"`
	// AptListsMB is the part of DiskMB for apt's package lists (Env).
	AptListsMB int `json:"apt_lists_mb,omitempty"`
	// Warnings are "ram" and "disk": the plan may not fit. They never stop
	// an install.
	Warnings []string `json:"warnings"`
}

// Target names the plan's parts, for the job's title.
func (p Plan) Target() string {
	parts := make([]string, len(p.Steps))
	for i, s := range p.Steps {
		parts[i] = s.Name
	}
	return strings.Join(parts, ", ")
}

// Plan checks a choice and orders its installs: the web server (with the
// distribution's PHP), the other PHP versions, the database server, then
// Extras in their order. Parts already installed stay in the plan, marked,
// and are skipped when it runs. Errors are *usererr.Error.
func (e Env) Plan(c Choice) (Plan, error) {
	p := Plan{Steps: []Step{}, Warnings: []string{}}
	if !e.Supported {
		return p, usererr.New("unsupported")
	}
	if c.Empty() {
		return p, usererr.New("setup_empty")
	}
	web, db := c.Web, c.DB
	if none(web) {
		web = ""
	}
	if none(db) {
		db = ""
	}
	var extras []string
	for _, id := range c.Extras {
		switch {
		case IsDatabase(id):
			if db != "" && db != id {
				return p, usererr.New("setup_db_conflict", Name(db))
			}
			db = id
		case slices.Contains(Extras, id):
			extras = append(extras, id)
		default:
			return p, usererr.New("unknown_software", id)
		}
	}
	add := func(pt Part, name string) {
		p.Steps = append(p.Steps, Step{ID: pt.ID, Name: name, Installed: pt.Installed, RAMMB: pt.RAMMB, RAMMax: pt.RAMMax,
			DiskMB: pt.DiskMB, ThirdParty: pt.ThirdParty})
	}
	// offered returns the part, or why it can't be installed.
	offered := func(id string) (Part, error) {
		pt, ok := e.Part(id)
		if !ok {
			return pt, usererr.New("setup_not_offered", Name(id))
		}
		if pt.Blocked != nil && !pt.Installed {
			return pt, usererr.New(pt.Blocked.Code, pt.Blocked.Detail)
		}
		return pt, nil
	}
	webComing := false // a web server will be installed, with the distribution's PHP
	if web != "" {
		if !slices.Contains(WebServers, web) {
			return p, usererr.New("invalid_web_server", web)
		}
		pt, err := offered(web)
		if err != nil {
			return p, err
		}
		if e.WebServer != "" && e.WebServer != web {
			return p, usererr.New("web_server_conflict", e.WebServer)
		}
		name := Name(web)
		if !pt.Installed && pt.PHP != "" {
			if dp, ok := e.Part("php" + pt.PHP); !ok || !dp.Installed {
				name += " and PHP " + pt.PHP
			}
		}
		add(pt, name)
		webComing = !pt.Installed
	}
	var versions []string
	for _, v := range c.PHP {
		v = strings.TrimSpace(v)
		if v == "" || slices.Contains(versions, v) {
			continue
		}
		if _, ok := e.Part("php" + v); !ok {
			return p, usererr.New("setup_not_offered", Name("php"+v))
		}
		versions = append(versions, v)
	}
	slices.SortFunc(versions, compareVersions)
	for _, v := range versions {
		if webComing && v == e.DistroPHP {
			continue // the web server installs it
		}
		pt, _ := e.Part("php" + v)
		add(pt, Name(pt.ID))
	}
	if db != "" {
		if !IsDatabase(db) {
			return p, usererr.New("unknown_software", db)
		}
		pt, err := offered(db)
		if err != nil {
			return p, err
		}
		add(pt, Name(db))
	}
	hasWeb := web != "" || e.WebServer != ""
	for _, id := range Extras {
		if !slices.Contains(extras, id) {
			continue
		}
		pt, err := offered(id)
		if err != nil {
			return p, err
		}
		if pt.NeedsWeb && !hasWeb && !pt.Installed {
			return p, usererr.New("setup_needs_web", Name(id))
		}
		add(pt, Name(id))
	}
	for _, s := range p.Steps {
		if !s.Installed {
			p.RAMMB += s.RAMMB
			p.DiskMB += s.DiskMB
		}
	}
	// The first install downloads apt's package lists on a server that
	// never had them.
	if p.DiskMB > 0 && e.AptListsMB > 0 {
		p.AptListsMB = e.AptListsMB
		p.DiskMB += e.AptListsMB
	}
	if b, ok := e.Server.RAMBudgetMB(); ok && p.RAMMB > b {
		p.Warnings = append(p.Warnings, "ram")
	}
	if b, ok := e.Server.DiskBudgetMB(); ok && p.DiskMB > b {
		p.Warnings = append(p.Warnings, "disk")
	}
	return p, nil
}

// Defaults is the choice the wizard starts with: the managed web server or
// Nginx, the distribution's PHP version (else the newest one that is not
// past its end of life), MariaDB
// (or the database server that is installed), and Redis from about 1.5 GB
// of RAM. Docker, Node.js, Python, Adminer, the web firewall and ClamAV
// are left to the administrator. When that would not fit in the memory,
// Redis, then the extra PHP version, then the database are left out.
func (e Env) Defaults() Choice {
	c := Choice{PHP: []string{}, Extras: []string{}}
	if !e.Supported {
		return c
	}
	if e.WebServer != "" {
		c.Web = e.WebServer
	} else {
		for _, id := range WebServers {
			if pt, ok := e.Part(id); ok && pt.Blocked == nil {
				c.Web = id
				break
			}
		}
	}
	// The distribution's PHP: it comes with the web server, gets its
	// security fixes from the distribution and needs no other repository.
	if pt, ok := e.Part("php" + e.DistroPHP); ok && e.DistroPHP != "" && pt.Group == "php" {
		c.PHP = []string{pt.Version}
	}
	for i := len(e.Parts) - 1; i >= 0 && len(c.PHP) == 0; i-- {
		if pt := e.Parts[i]; pt.Group == "php" && !pt.EOL {
			c.PHP = []string{pt.Version}
		}
	}
	for _, pt := range e.Parts {
		if pt.Group == "db" && pt.Installed {
			c.DB = pt.ID
		}
	}
	if _, ok := e.Part("mariadb"); ok && c.DB == "" {
		c.DB = "mariadb"
	}
	if _, ok := e.Part("redis"); ok && e.Server.MemTotalMB >= redisDefaultMB {
		c.Extras = append(c.Extras, "redis")
	}
	fits := func() bool {
		p, err := e.Plan(c)
		return err != nil || !slices.Contains(p.Warnings, "ram")
	}
	for _, drop := range []string{"redis", "php", "db"} {
		if fits() {
			break
		}
		switch drop {
		case "php":
			if c.Web != "" { // the web server still brings the distribution's PHP
				c.PHP = []string{}
			}
		case "db":
			c.DB = ""
		default:
			c.Extras = slices.DeleteFunc(c.Extras, func(id string) bool { return id == drop })
		}
	}
	return c
}

// Offered reports whether the panel should open the wizard after signing
// in: on a supported server with no web server and no websites, until the
// setup has finished once or was skipped.
func Offered(supported bool, webServer, state string, sites int) bool {
	return supported && webServer == "" && sites == 0 && state == ""
}

// View is what GET /api/setup answers: the offer, the recommended choice
// and the budgets the UI warns against.
type View struct {
	Env
	State        string `json:"state"`
	Offered      bool   `json:"offered"`
	Sites        int    `json:"sites"`
	Defaults     Choice `json:"defaults"`
	RAMBudgetMB  *int   `json:"ram_budget_mb,omitempty"`
	DiskBudgetMB *int   `json:"disk_budget_mb,omitempty"`
}

// NewView returns the View of e.
func NewView(e Env, state string, sites int) View {
	v := View{Env: e, State: state, Sites: sites, Offered: Offered(e.Supported, e.WebServer, state, sites), Defaults: e.Defaults()}
	if mb, ok := e.Server.RAMBudgetMB(); ok {
		v.RAMBudgetMB = &mb
	}
	if mb, ok := e.Server.DiskBudgetMB(); ok {
		v.DiskBudgetMB = &mb
	}
	return v
}

// Logger receives the setup's log.
type Logger interface {
	io.Writer
	Logf(format string, args ...any)
}

// Installer installs the parts. The panel implements it with the Software
// page's install functions.
type Installer interface {
	// Installed reports whether a part is installed now.
	Installed(ctx context.Context, id string) bool
	Install(ctx context.Context, id string, lg Logger) error
}

// StepError is a setup that stopped at a step.
type StepError struct {
	Step Step
	// Installed are the steps that are installed: before this run, or by it.
	Installed []string
	Err       error
}

func (e *StepError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s could not be installed: %v\n", e.Step.Name, e.Err)
	if len(e.Installed) > 0 {
		fmt.Fprintf(&b, "Installed before it stopped: %s.\n", strings.Join(e.Installed, ", "))
	} else {
		b.WriteString("Nothing was installed before it stopped.\n")
	}
	b.WriteString("Fix the problem, then run the setup again: the parts that are installed are skipped.")
	return b.String()
}

func (e *StepError) Unwrap() error { return e.Err }

// Run installs the plan's steps one after another. A part that is
// installed already is skipped, so a setup that stopped can simply run
// again. It stops at the first failure with a *StepError.
func Run(ctx context.Context, in Installer, p Plan, lg Logger) error {
	var installed []string
	n := len(p.Steps)
	for i, s := range p.Steps {
		if err := ctx.Err(); err != nil {
			return &StepError{Step: s, Installed: installed, Err: err}
		}
		if in.Installed(ctx, s.ID) {
			lg.Logf("==> [%d/%d] %s is installed already: skipped", i+1, n, s.Name)
			installed = append(installed, s.Name)
			continue
		}
		lg.Logf("==> [%d/%d] Installing %s", i+1, n, s.Name)
		if err := in.Install(ctx, s.ID, lg); err != nil {
			lg.Logf("==> %s could not be installed. The setup stopped here.", s.Name)
			return &StepError{Step: s, Installed: installed, Err: err}
		}
		installed = append(installed, s.Name)
	}
	lg.Logf("==> The setup has finished: %s.", strings.Join(installed, ", "))
	return nil
}
