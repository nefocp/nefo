package database

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/software/apt"
	"github.com/nefocp/nefo/internal/usererr"
)

// Switching the database server from MariaDB to MySQL or back, with the
// databases. A server has one of them (see checkInstall), and neither can
// read the other's data directory, so the data goes over as SQL:
//
//  1. the new server's packages are downloaded, and the old ones are kept
//     for going back;
//  2. the old server is made read-only (websites still read), every user
//     database is exported (dumps.go) and its rows counted, and its
//     accounts and privileges are read (accounts.go);
//  3. the old server stops, its data directory is moved aside
//     (/var/lib/mysql.<engine>-<time>) and /etc/mysql copied, and its
//     packages are purged;
//  4. the new server is installed as the Software page installs it, kept
//     read-only, with the panel's remote access and slow log settings;
//  5. the accounts are created, the databases imported (adapted on the way
//     by the DumpFixer: MySQL 8's collations for MariaDB, MariaDB 11's for
//     MySQL) and the privileges granted;
//  6. the new server must hold the same tables with the same rows, views,
//     routines, triggers and events, and the panel's database users must
//     sign in. Then it accepts writes, the old data directory is deleted
//     and the dumps stay until the administrator deletes them.
//
// When a step from 3 on fails, the new server is removed and the old one
// comes back with its data directory and configuration as they were.
// Without user databases the same job simply replaces one server with the
// other.

// DumpFixer adapts a dump made by one engine for the other while it
// streams (the website import's SQL fixer, see package siteimport).
type DumpFixer func(r io.Reader, to Engine) io.Reader

// SwitchDB is a database a switch carries over.
type SwitchDB struct {
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	Panel bool   `json:"panel"` // the panel's (on the Databases page)
}

// Problem is why a switch can't run now: a usererr code and its detail.
type Problem struct {
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}

// SwitchPlan describes a switch from the installed server to the other.
type SwitchPlan struct {
	From        Engine        `json:"from"`
	FromVersion string        `json:"from_version"`
	To          Engine        `json:"to"`
	ToVersion   string        `json:"to_version"` // "8.4" for MySQL, the distribution's series for MariaDB
	Databases   []SwitchDB    `json:"databases"`
	Accounts    []AccountPlan `json:"accounts"`
	Problems    []Problem     `json:"problems"`
	DataMB      int64         `json:"data_mb"`
	NeedMB      int64         `json:"need_mb"`
	FreeMB      int64         `json:"free_mb"`
	DumpsDir    string        `json:"dumps_dir"`
	SlowLog     bool          `json:"slow_log"` // the slow query log is on and stays on
	Remote      int           `json:"remote"`   // databases with remote access, which keeps working

	accounts  []account
	threshold string // the slow log's, when SlowLog
}

// switchExtraMB is roughly the disk the packages of both servers and the
// new server's empty data directory take.
const switchExtraMB = 700

// mysqlEngines are the storage engines and table types MySQL 8.4 has.
var mysqlEngines = map[string]bool{"InnoDB": true, "MyISAM": true, "MEMORY": true, "CSV": true, "ARCHIVE": true, "BLACKHOLE": true, "MRG_MYISAM": true}

var collationNameRe = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)

// CheckSwitch returns the plan of a switch to engine to, or why it can't
// be planned at all (a usererr). A plan with Problems can't run yet.
func (m *Manager) CheckSwitch(ctx context.Context, to Engine) (*SwitchPlan, error) {
	if !m.supported() {
		return nil, usererr.New("unsupported")
	}
	if to != MariaDB && to != MySQL {
		return nil, usererr.New("unknown_software", string(to))
	}
	st := m.Status(ctx)
	if !st.Installed {
		return nil, usererr.New("software_not_installed", "MariaDB")
	}
	if st.Engine == to {
		return nil, usererr.New("db_switch_same", to.Name())
	}
	if to == MySQL {
		if err := m.MySQLAvailable(); err != nil {
			return nil, err
		}
	}
	if !st.Running {
		return nil, usererr.New("db_not_running")
	}
	if m.customDataDir() {
		return nil, usererr.New("db_datadir_custom")
	}
	from := st.Engine
	p := &SwitchPlan{From: from, FromVersion: st.Version, To: to, Databases: []SwitchDB{}, Problems: []Problem{}, DumpsDir: m.dumpsPath()}
	p.ToVersion = "8.4"
	if to == MariaDB {
		p.ToVersion = m.DistroMariaDBSeries()
	}
	dbs, err := m.UserDatabases(ctx)
	if err != nil {
		return nil, err
	}
	records, err := m.store.ListDatabases(ctx)
	if err != nil {
		return nil, err
	}
	panel := map[string]bool{}
	for _, d := range records {
		panel[d.Name] = true
	}
	sizes := map[string]int64{}
	if len(dbs) > 0 {
		rows, err := m.hexRows(ctx, from, "SELECT HEX(TABLE_SCHEMA), HEX(CONCAT(COALESCE(SUM(DATA_LENGTH + INDEX_LENGTH), 0))) FROM information_schema.TABLES "+
			"WHERE TABLE_SCHEMA IN ("+sqlList(dbs)+") GROUP BY TABLE_SCHEMA;")
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			if len(r) == 2 {
				var n int64
				fmt.Sscan(r[1], &n)
				sizes[r[0]] = n
			}
		}
	}
	for _, db := range dbs {
		p.Databases = append(p.Databases, SwitchDB{Name: db, Size: sizes[db], Panel: panel[db]})
		if strings.HasPrefix(db, "-") {
			p.Problems = append(p.Problems, Problem{"switch_db_name", db})
		}
	}
	if p.accounts, err = m.readAccounts(ctx, from); err != nil {
		return nil, err
	}
	p.Accounts = accountPlans(p.accounts)
	var unknown []string
	for _, a := range p.accounts {
		if a.Panel && a.How == HowNew {
			unknown = append(unknown, a.User)
		}
	}
	if len(unknown) > 0 {
		p.Problems = append(p.Problems, Problem{"switch_password_unknown", shortList(unknown)})
	}
	if to == MySQL && len(dbs) > 0 {
		probs, err := m.mysqlProblems(ctx, dbs)
		if err != nil {
			return nil, err
		}
		p.Problems = append(p.Problems, probs...)
	}
	remote, err := m.store.AllDBRemoteHosts(ctx)
	if err != nil {
		return nil, err
	}
	for _, hosts := range remote {
		if len(hosts) > 0 {
			p.Remote++
		}
	}
	if b, err := os.ReadFile(m.slowConfPath()); err == nil {
		on, t := parseSlowConf(string(b))
		if on {
			p.SlowLog = true
			p.threshold, _ = formatThreshold(t)
			if p.threshold == "" {
				p.threshold = "10"
			}
		}
	}
	p.DataMB = dirSize(m.dataDirPath()) >> 20
	var total int64
	for _, s := range sizes {
		total += s
	}
	// The dumps (compressed, at most about the data's size) and the
	// imported data next to the old data directory, which stays until the
	// end.
	p.NeedMB = total>>20 + total>>20 + switchExtraMB
	p.FreeMB = freeDiskMB(m.dataDirPath())
	if err := diskCheck(m.dataDirPath(), p.NeedMB); err != nil {
		var ue *usererr.Error
		if errors.As(err, &ue) {
			p.Problems = append(p.Problems, Problem{ue.Code, ue.Detail})
		}
	}
	return p, nil
}

// mysqlProblems lists what MySQL 8.4 does not have in the databases:
// MariaDB's Aria (and other) storage engines, sequences, system-versioned
// tables, the INET4, INET6 and UUID column types, and definers with names
// longer than MySQL's 32 characters.
func (m *Manager) mysqlProblems(ctx context.Context, dbs []string) ([]Problem, error) {
	list := sqlList(dbs)
	var probs []Problem
	rows, err := m.hexRows(ctx, MariaDB, "SELECT HEX(TABLE_SCHEMA), HEX(TABLE_NAME), HEX(IFNULL(ENGINE, '')), HEX(TABLE_TYPE) FROM information_schema.TABLES "+
		"WHERE TABLE_SCHEMA IN ("+list+") AND TABLE_TYPE <> 'VIEW';")
	if err != nil {
		return nil, err
	}
	var bad []string
	for _, r := range rows {
		if len(r) != 4 {
			continue
		}
		switch {
		case r[3] == "SEQUENCE":
			bad = append(bad, r[0]+"."+r[1]+" (sequence)")
		case r[3] == "SYSTEM VERSIONED":
			bad = append(bad, r[0]+"."+r[1]+" (system-versioned)")
		case r[2] != "" && !mysqlEngines[r[2]]:
			bad = append(bad, r[0]+"."+r[1]+" ("+r[2]+")")
		}
	}
	if len(bad) > 0 {
		probs = append(probs, Problem{"switch_mysql_tables", shortList(bad)})
	}
	rows, err = m.hexRows(ctx, MariaDB, "SELECT HEX(TABLE_SCHEMA), HEX(TABLE_NAME), HEX(COLUMN_NAME), HEX(DATA_TYPE) FROM information_schema.COLUMNS "+
		"WHERE TABLE_SCHEMA IN ("+list+") AND DATA_TYPE IN ('inet4', 'inet6', 'uuid');")
	if err != nil {
		return nil, err
	}
	bad = nil
	for _, r := range rows {
		if len(r) == 4 {
			bad = append(bad, r[0]+"."+r[1]+"."+r[2]+" ("+strings.ToUpper(r[3])+")")
		}
	}
	if len(bad) > 0 {
		probs = append(probs, Problem{"switch_mysql_columns", shortList(bad)})
	}
	bad = nil
	for _, q := range []string{
		"SELECT HEX(TRIGGER_SCHEMA), HEX(DEFINER) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA IN (" + list + ")",
		"SELECT HEX(TABLE_SCHEMA), HEX(DEFINER) FROM information_schema.VIEWS WHERE TABLE_SCHEMA IN (" + list + ")",
		"SELECT HEX(ROUTINE_SCHEMA), HEX(DEFINER) FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA IN (" + list + ")",
		"SELECT HEX(EVENT_SCHEMA), HEX(DEFINER) FROM information_schema.EVENTS WHERE EVENT_SCHEMA IN (" + list + ")",
	} {
		rows, err := m.hexRows(ctx, MariaDB, q+";")
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			if len(r) != 2 {
				continue
			}
			user, _, _ := strings.Cut(r[1], "@")
			if len(user) > mysqlUserMax && !containsString(bad, r[0]+": "+user) {
				bad = append(bad, r[0]+": "+user)
			}
		}
	}
	if len(bad) > 0 {
		probs = append(probs, Problem{"switch_mysql_definers", shortList(bad)})
	}
	return probs, nil
}

func shortList(list []string) string {
	sort.Strings(list)
	if len(list) > 8 {
		list = append(list[:8:8], "…")
	}
	return strings.Join(list, ", ")
}

// rollbackPackages are the packages of engine e that going back installs
// again in their old versions.
func rollbackPackages(e Engine) []string {
	if e == MySQL {
		return []string{"mysql-community-*", "mysql-common", "mysql-client", "mysql-server"}
	}
	return upgradePackages
}

// Switch replaces the installed database server with the other one, with
// its databases, accounts and privileges (see the comment at the top of
// this file). fix adapts the dumps; nil imports them unchanged.
func (m *Manager) Switch(ctx context.Context, to Engine, fix DumpFixer, lg Logger) error {
	plan, err := m.CheckSwitch(ctx, to)
	if err != nil {
		return err
	}
	if len(plan.Problems) > 0 {
		return usererr.New(plan.Problems[0].Code, plan.Problems[0].Detail)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.busy.Store(true)
	defer m.busy.Store(false)
	from := plan.From
	fromName, toName := from.Name()+" "+plan.FromVersion, strings.TrimSpace(to.Name()+" "+plan.ToVersion)
	dir, err := m.newDumpDir(fmt.Sprintf("%s-%s-to-%s", from, SeriesOf(plan.FromVersion), to))
	if err != nil {
		return err
	}
	mf := &manifest{Kind: "switch", From: fromName, To: toName, Created: time.Now().UTC()}
	finish := func(result string, err error) error {
		mf.Result = result
		if err != nil {
			mf.Error = err.Error()
		}
		if werr := writeManifest(dir, mf); werr != nil {
			lg.Logf("Warning: %v", werr)
		}
		return err
	}
	if err := writeManifest(dir, mf); err != nil {
		return err
	}
	dbs := make([]string, len(plan.Databases))
	for i, d := range plan.Databases {
		dbs[i] = d.Name
	}
	lg.Logf("Switching from %s to %s, with %d databases. While they are exported, websites can read their databases but not write to them; "+
		"then the databases are offline until %s has them.", fromName, toName, len(dbs), to.Name())

	// 1. The packages, before anything changes for the websites.
	old := m.installedPackages(ctx, rollbackPackages(from)...)
	unused, unusedErr := apt.Orphans(ctx, m.run) // to clean up after the old server, at the end
	repo := m.saveMariaDBRepo()
	undoRepos := func() {
		if to == MySQL {
			os.Remove(m.repo.list)
			os.Remove(m.repo.keyring)
		}
		repo.restore(m)
	}
	if err := m.fetchSwitchPackages(ctx, from, to, old, lg); err != nil {
		undoRepos()
		return finish("failed", err)
	}

	// 2. Read-only, counted, exported.
	if _, err := m.sqlWith(ctx, from, "SET GLOBAL read_only = ON;"); err != nil {
		undoRepos()
		return finish("failed", err)
	}
	writable := func() { m.sqlWith(context.WithoutCancel(ctx), from, "SET GLOBAL read_only = OFF;") }
	accounts, err := m.readAccounts(ctx, from)
	if err != nil {
		writable()
		undoRepos()
		return finish("failed", err)
	}
	for _, a := range accounts {
		mf.Accounts = append(mf.Accounts, a.User+"@"+a.Host)
	}
	lg.Logf("Counting the rows of every table")
	counts, err := m.countObjects(ctx, from, dbs)
	if err != nil {
		writable()
		undoRepos()
		return finish("failed", err)
	}
	mf.Counts = counts
	lg.Logf("Exporting the databases into %s", dir)
	if mf.Databases, err = m.dumpDatabases(ctx, from, dir, dbs, lg); err != nil {
		writable()
		undoRepos()
		return finish("failed", err)
	}
	if err := writeManifest(dir, mf); err != nil {
		writable()
		undoRepos()
		return finish("failed", err)
	}

	// 3. The old server goes.
	lg.Logf("Stopping %s", from.Name())
	if out, err := m.run.Output(ctx, "systemctl", "stop", from.service()); err != nil {
		writable()
		undoRepos()
		return finish("failed", fmt.Errorf("stopping %s failed: %s", from.Name(), strings.TrimSpace(out)))
	}
	saved, err := m.moveServerFiles(ctx, dir, string(from), lg)
	if err != nil {
		m.run.Output(context.WithoutCancel(ctx), "systemctl", "start", from.service())
		undoRepos()
		return finish("failed", err)
	}

	report, err := m.switchSteps(ctx, plan, accounts, mf.Databases, counts, dir, fix, lg)
	if err == nil {
		m.finishSwitch(ctx, from, saved, lg)
		if unusedErr == nil {
			// The databases are back; what only the old server needed goes now.
			if err := apt.CleanUp(ctx, m.run, lg, true, unused); err != nil {
				lg.Logf("Warning: %v", err)
			}
		}
		for _, line := range report {
			lg.Logf("%s", line)
		}
		lg.Logf("%s runs now, with the databases of %s. The dumps from before stay in %s until you delete them.", m.Status(ctx).Name(), fromName, dir)
		return finish("done", nil)
	}
	lg.Logf("The switch failed: %v", err)
	lg.Logf("Going back to %s with its data", fromName)
	undoRepos()
	if rerr := m.restoreEngine(ctx, from, SeriesOf(plan.FromVersion), old, repo, saved, lg); rerr != nil {
		return finish("failed", fmt.Errorf("%w. Going back to %s failed as well: %v. Its data directory is %s, its configuration %s, "+
			"and the dumps are in %s", err, fromName, rerr, saved.data, saved.etc, dir))
	}
	return finish("rolled_back", fmt.Errorf("the switch to %s failed, and %s is back as it was: %w", to.Name(), fromName, err))
}

// Name is the status' engine name with its version, "MySQL 8.4.7".
func (st Status) Name() string { return strings.TrimSpace(st.Engine.Name() + " " + st.Version) }

// fetchSwitchPackages adds the new server's repository (for MySQL),
// updates the lists, keeps the old server's packages for going back and
// downloads the new server's.
func (m *Manager) fetchSwitchPackages(ctx context.Context, from, to Engine, old []pkgVersion, lg Logger) error {
	pkgs := []string{"mariadb-server", "mariadb-client"}
	if to == MySQL {
		lg.Logf("Adding Oracle's MySQL repository (%s, signing key %s)", mysqlTrack, mysqlKeyFingerprint)
		if err := m.addMySQLRepo(ctx); err != nil {
			return err
		}
		pkgs = []string{"mysql-community-server", "mysql-community-client"}
	}
	lg.Logf("Updating the package lists")
	if err := run.Apt(ctx, m.run, lg, "update"); err != nil {
		return err
	}
	m.keepOldPackages(ctx, old, lg)
	// A MariaDB from MariaDB's repository goes: its repository must not
	// give the new server its libraries (going back restores it, and apt
	// still has its lists).
	m.removeMariaDBRepo(nil)
	// The new server conflicts with the old one, so apt plans the download
	// with the old one's packages removed ("pkg-"); it removes nothing in
	// download-only mode.
	args := append([]string{"install", "--download-only", "--no-install-recommends"}, pkgs...)
	for _, p := range m.installedPackages(ctx, enginePackages(from)...) {
		args = append(args, p.Name+"-")
	}
	lg.Logf("Downloading %s", to.Name())
	if err := run.Apt(ctx, m.run, lg, args...); err != nil {
		// Not fatal: the packages are then downloaded when the old server
		// is gone (and going back needs no network either way). But apt
		// must know the new server.
		if m.candidate(ctx, pkgs[0]) == "" {
			return fmt.Errorf("apt has no %s package: %w", pkgs[0], err)
		}
		lg.Logf("The packages could not be downloaded in advance (%v); they are downloaded when %s is installed", err, to.Name())
	}
	return nil
}

// moveServerFiles copies the stopped server's configuration into dir and
// moves its data directory aside (/var/lib/mysql.<tag>-<time>).
func (m *Manager) moveServerFiles(ctx context.Context, dir, tag string, lg Logger) (savedFiles, error) {
	s := savedFiles{data: m.dataDirPath() + "." + tag + "-" + time.Now().Format("20060102-150405"), etc: filepath.Join(dir, "etc-mysql"), moved: true}
	lg.Logf("Copying the configuration %s to %s", m.etcDirPath(), s.etc)
	if err := m.copyTree(ctx, m.etcDirPath(), s.etc); err != nil {
		return s, err
	}
	lg.Logf("Moving the data directory %s to %s", m.dataDirPath(), s.data)
	if err := os.Rename(m.dataDirPath(), s.data); err != nil {
		return s, err
	}
	return s, nil
}

// switchSteps installs the new server and brings the databases into it
// (steps 3 to 6). It returns what the administrator should know of the
// accounts.
func (m *Manager) switchSteps(ctx context.Context, plan *SwitchPlan, accounts []account, dumped []dumpedDB,
	counts map[string]objectCounts, dir string, fix DumpFixer, lg Logger) ([]string, error) {
	from, to := plan.From, plan.To
	if err := m.removeEnginePackages(ctx, from, false, lg); err != nil {
		return nil, err
	}
	// Its shared files and client library, from its repository (Oracle's
	// or MariaDB's) or the distribution, so that the new server brings its
	// own. Best effort: something else may still need them.
	common := []string{"mariadb-common", "mysql-common", "libmariadb3"}
	if from == MySQL {
		common = []string{"mysql-common"}
	}
	if pkgs, _ := apt.Installed(ctx, m.run, true, common...); len(pkgs) > 0 {
		if err := apt.RemoveOnly(ctx, m.run, lg, true, pkgs...); err != nil {
			lg.Logf("Keeping %s: %v", strings.Join(pkgs, " "), err)
		}
	}
	// The panel's settings for the old server (they are in the copy of
	// /etc/mysql).
	oldConf := m.confDir(from)
	more, _ := filepath.Glob(filepath.Join(oldConf, "99-panel-*.cnf"))
	for _, f := range append(more, filepath.Join(oldConf, panelConfName)) {
		os.Remove(f)
	}
	var err error
	if to == MySQL {
		err = m.installMySQL(ctx, lg)
	} else {
		err = m.installMariaDB(ctx, "", lg)
	}
	if err != nil {
		return nil, err
	}
	if m.engine() != to {
		return nil, fmt.Errorf("%s was installed, but the server is not %s", to.Name(), to.Name())
	}
	// Read-only until the data is checked; the panel's other settings.
	roConf := filepath.Join(m.confDir(to), readOnlyConfName)
	if err := fsutil.WriteFileAtomic(roConf, []byte(readOnlyConf), 0o644); err != nil {
		return nil, err
	}
	if err := m.writeListenConf(ctx); err != nil {
		return nil, err
	}
	if plan.SlowLog {
		file := to.slowLogFile()
		lg.Logf("Turning the slow query log on again (%s)", file)
		if _, err := m.run.Output(ctx, "install", "-d", "-o", "mysql", "-g", "adm", "-m", "2750", filepath.Dir(file)); err != nil {
			return nil, err
		}
		if err := fsutil.WriteFileAtomic(m.slowConfPath(), []byte(slowConfFor(file, true, plan.threshold)), 0o644); err != nil {
			return nil, err
		}
	}
	if out, err := m.run.Output(ctx, "systemctl", "restart", to.service()); err != nil {
		return nil, fmt.Errorf("%s does not start: %s", to.Name(), strings.TrimSpace(out))
	}
	m.listenDirty = false
	if err := m.waitReady(ctx, to); err != nil {
		return nil, err
	}

	// 5. Accounts, databases, privileges.
	var report []string
	lg.Logf("Creating %d accounts", len(accounts))
	for _, a := range accounts {
		if a.How == HowSkip {
			report = append(report, fmt.Sprintf("The account %s@%s was not carried over: %s does not allow user names longer than %d characters.", a.User, a.Host, to.Name(), mysqlUserMax))
			continue
		}
		q, err := createAccountSQL(a, to)
		if err != nil {
			return nil, err
		}
		if _, err := m.sqlWith(ctx, to, q); err != nil {
			if a.Panel {
				return nil, fmt.Errorf("create the account %s@%s: %w", a.User, a.Host, err)
			}
			report = append(report, fmt.Sprintf("The account %s@%s could not be created: %v", a.User, a.Host, err))
			continue
		}
		if a.How == HowNew {
			report = append(report, fmt.Sprintf("The account %s@%s signed in with %s, which %s can't use: it has a new random password and is locked. "+
				"Set a password with ALTER USER … IDENTIFIED BY … ACCOUNT UNLOCK.", a.User, a.Host, a.Why, to.Name()))
		}
	}
	for _, d := range dumped {
		lg.Logf("Importing %s", d.Name)
		if err := m.importSwitchDump(ctx, to, d, filepath.Join(dir, d.File), fix); err != nil {
			return nil, fmt.Errorf("import %s: %w", d.Name, err)
		}
	}
	for _, a := range accounts {
		if a.How == HowSkip {
			continue
		}
		keep, skipped := carriedGrants(a.grants)
		for _, g := range keep {
			if _, err := m.sqlWith(ctx, to, g); err != nil {
				if a.Panel {
					return nil, fmt.Errorf("grant %s@%s its privileges: %w", a.User, a.Host, err)
				}
				report = append(report, fmt.Sprintf("%s refused this privilege of %s@%s, which was left out: %s", to.Name(), a.User, a.Host, g))
			}
		}
		for _, g := range skipped {
			report = append(report, fmt.Sprintf("Not carried over: %s", g))
		}
	}

	// 6. The check.
	if err := m.checkData(ctx, to, counts, lg); err != nil {
		return nil, err
	}
	if err := os.Remove(roConf); err != nil {
		return nil, err
	}
	if _, err := m.sqlWith(ctx, to, "SET GLOBAL read_only = OFF;"); err != nil {
		return nil, err
	}
	return report, nil
}

// importSwitchDump creates a database on engine to and imports its dump
// into it as root, with the old default character set and collation.
func (m *Manager) importSwitchDump(ctx context.Context, to Engine, d dumpedDB, path string, fix DumpFixer) error {
	if _, err := m.sqlWith(ctx, to, "CREATE DATABASE "+sqlIdent(d.Name)+";"); err != nil {
		return err
	}
	r, closeDump, err := gunzipFile(path)
	if err != nil {
		return err
	}
	defer closeDump()
	if to == MySQL {
		r = newNoAutoCreateUser(skipSandboxLine(r))
	}
	head := ""
	if collationNameRe.MatchString(d.Charset) && collationNameRe.MatchString(d.Collation) {
		// Without a name ALTER DATABASE changes the current one; the fixer
		// adapts the collation's name like those in the dump.
		head = "ALTER DATABASE CHARACTER SET " + d.Charset + " COLLATE " + d.Collation + ";\n"
	}
	var src io.Reader = io.MultiReader(strings.NewReader(head), r)
	if fix != nil {
		src = fix(src, to)
	}
	return m.run.Pipe(ctx, src, io.Discard, to.client(), "--default-character-set=utf8mb4", "--batch", d.Name)
}

// finishSwitch removes what only the old server needed: its repository
// and its moved data directory.
func (m *Manager) finishSwitch(ctx context.Context, from Engine, saved savedFiles, lg Logger) {
	if from == MySQL {
		for _, f := range []string{m.repo.list, m.repo.keyring} {
			if err := os.Remove(f); err == nil {
				lg.Logf("Removed %s", f)
			}
		}
	}
	lg.Logf("Deleting the old data directory %s", saved.data)
	if err := os.RemoveAll(saved.data); err != nil {
		lg.Logf("Warning: %v", err)
	}
	os.RemoveAll(saved.etc)
}
