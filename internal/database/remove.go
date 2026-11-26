package database

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/nefocp/nefo/internal/software/apt"
	"github.com/nefocp/nefo/internal/usererr"
)

// Uninstalling MariaDB or MySQL. It is refused while the server holds any
// database other than its own, whoever made it: the server itself is
// asked, not the panel's list, which does not know databases made by hand
// or by another panel on the same server. The data folder stays unless the
// administrator asks for it to be deleted and types "mariadb" (or "mysql").

// systemDatabases are the server's own (the same on MariaDB and MySQL).
var systemDatabases = []string{"information_schema", "mysql", "performance_schema", "sys"}

const (
	defaultDataDir = "/var/lib/mysql"
	defaultEtcDir  = "/etc/mysql"
)

func (m *Manager) dataDirPath() string {
	if m.dataDir != "" {
		return m.dataDir
	}
	return defaultDataDir
}

func (m *Manager) etcDirPath() string {
	if m.etcDir != "" {
		return m.etcDir
	}
	return defaultEtcDir
}

// Installed reports whether a database server, MariaDB or MySQL, is
// installed, without running anything (see InstalledEngine).
func Installed() bool { return InstalledEngine() != "" }

// userDatabase reports whether a name from SHOW DATABASES (or a folder in
// the data directory) is a database someone made.
func userDatabase(name string) bool {
	return name != "" && !slices.Contains(systemDatabases, name) &&
		!strings.HasPrefix(name, "#") && name != "lost+found"
}

// UserDatabases lists the databases on the server other than its own. When
// the server is not running, the folders in its data directory are listed
// instead; if the data directory is not the default one, that is not
// possible and the answer is the usererr mariadb_not_running (or
// mysql_not_running).
func (m *Manager) UserDatabases(ctx context.Context) ([]string, error) {
	notRunning := string(m.engine()) + "_not_running"
	names := []string{}
	if m.Status(ctx).Running {
		out, err := m.sql(ctx, "SHOW DATABASES;")
		if err != nil {
			return nil, err
		}
		for _, l := range strings.Split(out, "\n") {
			if l = strings.TrimSpace(l); userDatabase(l) {
				names = append(names, l)
			}
		}
		sort.Strings(names)
		return names, nil
	}
	if m.customDataDir() {
		return nil, usererr.New(notRunning)
	}
	entries, err := os.ReadDir(m.dataDirPath())
	if errors.Is(err, fs.ErrNotExist) {
		return names, nil
	}
	if err != nil {
		return nil, usererr.New(notRunning)
	}
	for _, e := range entries {
		if e.IsDir() && userDatabase(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

var datadirRe = regexp.MustCompile(`(?m)^[ \t]*datadir[ \t]*=[ \t]*(\S+)`)

// customDataDir reports whether a MariaDB or MySQL configuration file
// moves the data directory away from the default.
func (m *Manager) customDataDir() bool {
	etc := m.etcDirPath()
	files := []string{filepath.Join(etc, "my.cnf"), filepath.Join(etc, "mariadb.cnf"), filepath.Join(etc, "mysql.cnf")}
	for _, d := range []string{"conf.d", "mariadb.conf.d", "mysql.conf.d"} {
		more, _ := filepath.Glob(filepath.Join(etc, d, "*.cnf"))
		files = append(files, more...)
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, mm := range datadirRe.FindAllStringSubmatch(string(b), -1) {
			if strings.TrimRight(strings.Trim(mm[1], `"'`), "/") != defaultDataDir {
				return true
			}
		}
	}
	return false
}

// CheckRemove returns why MariaDB cannot be uninstalled now (a usererr),
// or nil. deleteData asks for the data directory to be deleted too, which
// confirm must allow by being "mariadb".
func (m *Manager) CheckRemove(ctx context.Context, deleteData bool, confirm string) error {
	return m.CheckRemoveEngine(ctx, MariaDB, deleteData, confirm)
}

// CheckRemoveEngine is CheckRemove for MariaDB or MySQL: the engine must be
// the installed one, and confirm its ID ("mariadb" or "mysql").
func (m *Manager) CheckRemoveEngine(ctx context.Context, e Engine, deleteData bool, confirm string) error {
	if !m.supported() {
		return usererr.New("unsupported")
	}
	if !m.Status(ctx).Installed || m.engine() != e {
		return usererr.New("software_not_installed", e.Name())
	}
	if deleteData && strings.TrimSpace(confirm) != string(e) {
		return usererr.New("software_confirm_mismatch", string(e))
	}
	dbs, err := m.UserDatabases(ctx)
	if err != nil {
		return err
	}
	if len(dbs) > 0 {
		list := dbs
		if len(list) > 8 {
			list = append(list[:8:8], "…")
		}
		return usererr.New(string(e)+"_has_databases", strings.Join(list, ", "))
	}
	return nil
}

var (
	serverPkgRe      = regexp.MustCompile(`^mariadb-server(-[0-9.]+)?$`)
	mysqlServerPkgRe = regexp.MustCompile(`^mysql-community-server$`)
)

// Remove purges the MariaDB server and client packages and the panel's
// configuration files for them. The data directory is kept, so installing
// MariaDB again finds it, unless deleteData is set.
func (m *Manager) Remove(ctx context.Context, deleteData bool, confirm string, lg Logger) error {
	return m.RemoveEngine(ctx, MariaDB, deleteData, confirm, lg)
}

// RemoveEngine is Remove for MariaDB or MySQL. For MySQL it also removes
// Oracle's repository and its key, which only the panel added.
func (m *Manager) RemoveEngine(ctx context.Context, e Engine, deleteData bool, confirm string, lg Logger) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.CheckRemoveEngine(ctx, e, deleteData, confirm); err != nil {
		return err
	}
	patterns := []string{"mariadb-server*", "mariadb-client*", "mariadb-plugin-*",
		"mariadb-backup*", "default-mysql-server*", "default-mysql-client*"}
	if e == MySQL {
		// mysql-common and the libraries go with the leftovers, when nothing
		// else needs them.
		patterns = []string{"mysql-community-server*", "mysql-community-client*", "mysql-server", "mysql-client"}
	}
	pkgs, err := apt.Installed(ctx, m.run, true, patterns...)
	if err != nil {
		return err
	}
	// The purge script asks whether to delete the databases. apt runs
	// without questions and the answer defaults to no; say so explicitly,
	// so an earlier answer cannot delete them.
	var answers strings.Builder
	for _, p := range pkgs {
		switch {
		case e == MariaDB && serverPkgRe.MatchString(p):
			answers.WriteString(p + " " + p + "/postrm_remove_databases boolean false\n")
		case e == MySQL && mysqlServerPkgRe.MatchString(p):
			answers.WriteString(p + " " + p + "/remove-data-dir boolean false\n")
		}
	}
	if answers.Len() > 0 {
		if out, err := m.run.Input(ctx, []byte(answers.String()), "debconf-set-selections"); err != nil {
			lg.Logf("Warning: debconf-set-selections: %s", strings.TrimSpace(out))
		}
	}
	if err := apt.Check(ctx, m.run, true, pkgs...); err != nil {
		return err
	}
	// The package scripts stop it too, unless a policy forbids them.
	name := e.Name()
	lg.Logf("Stopping %s", name)
	if out, err := m.run.Output(ctx, "systemctl", "stop", e.service()); err != nil {
		return fmt.Errorf("stopping %s failed: %s", name, strings.TrimSpace(out))
	}
	lg.Logf("Removing %s", name)
	if err := apt.Remove(ctx, m.run, lg, true, pkgs...); err != nil {
		return err
	}
	confDir := m.confDir(e)
	confs := []string{filepath.Join(confDir, panelConfName), m.remoteConfPath(), m.slowConfPath()}
	more, _ := filepath.Glob(filepath.Join(confDir, "99-panel-*.cnf"))
	if e == MySQL {
		// Without the server the repository has nothing left to update.
		confs = append(confs, m.repo.list, m.repo.keyring)
	}
	for _, f := range append(confs, more...) {
		if err := os.Remove(f); err == nil {
			lg.Logf("Removed %s", f)
		}
	}
	m.listenDirty = false
	dir := m.dataDirPath()
	if !deleteData {
		if _, err := os.Stat(dir); err == nil {
			lg.Logf("%s is removed. Its data directory %s was kept; installing %s again uses it.", name, dir, name)
		} else {
			lg.Logf("%s is removed.", name)
		}
		return nil
	}
	if out, err := m.run.Output(ctx, "systemctl", "is-active", e.service()); err == nil && strings.HasPrefix(out, "active") {
		return fmt.Errorf("%s is still running, so its data directory was not deleted", name)
	}
	dirs := []string{dir}
	if e == MySQL && m.dataDir == "" {
		// Where MySQL's packages keep import and export files and keyrings.
		dirs = append(dirs, "/var/lib/mysql-files", "/var/lib/mysql-keyring")
	}
	for _, d := range dirs {
		lg.Logf("Deleting %s", d)
		if err := os.RemoveAll(d); err != nil {
			return err
		}
	}
	lg.Logf("%s is removed, with its data directory.", name)
	return nil
}

// bufferPoolMB is the InnoDB buffer pool the panel gives MariaDB and MySQL
// on a server with memTotal bytes of RAM (see tuning and mysqlTuning).
func bufferPoolMB(memTotal int64) int {
	switch {
	case memTotal >= 7<<30:
		return 1024
	case memTotal >= 3<<30:
		return 512
	case memTotal >= 1500<<20:
		return 256
	}
	return 128
}

// MemoryMB is roughly how much memory MariaDB uses with the panel's
// settings on this server: its buffer pool and about 100 MB more.
func MemoryMB() int { return MemoryMBFor(MariaDB) }

// MemoryMBFor is MemoryMB for an engine. MySQL 8.4 needs more besides its
// buffer pool than MariaDB: about 150 MB with the performance schema off,
// as the panel sets it below about 4 GB of RAM, and about 300 MB with it
// on. These are estimates of an idle server with a few small databases;
// the Software page shows them before installing.
func MemoryMBFor(e Engine) int {
	t := memTotal()
	pool := bufferPoolMB(t)
	if e != MySQL {
		return pool + 100
	}
	if performanceSchema(t) {
		return pool + 300
	}
	return pool + 150
}

// dataDirEngine tells whose data the data directory holds: MySQL 8 keeps
// its data dictionary in mysql.ibd, which MariaDB never writes, and
// MariaDB always has aria_log_control. It is "" for a missing or empty
// directory and "?" for anything else.
func (m *Manager) dataDirEngine() Engine {
	dir := m.dataDirPath()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	empty := true
	for _, e := range entries {
		if e.Name() != "lost+found" {
			empty = false
			break
		}
	}
	switch {
	case empty:
		return ""
	case fileExists(filepath.Join(dir, "mysql.ibd")):
		return MySQL
	case fileExists(filepath.Join(dir, "aria_log_control")):
		return MariaDB
	}
	return "?"
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// otherServerPackages are the packages of the other engine's server that
// rule out installing e.
func otherServerPackages(e Engine) []string {
	if e == MySQL {
		return []string{"mariadb-server*"}
	}
	return []string{"mysql-community-server*", "mysql-server-core-*"}
}

// checkInstall returns why engine e cannot be installed (a usererr), or
// nil. A server has one database server: apt would remove the other one to
// install e, and the two share /var/lib/mysql, whose files neither can
// read from the other (MySQL would even try to upgrade MariaDB's).
func (m *Manager) checkInstall(ctx context.Context, e Engine) error {
	other := MariaDB
	if e == MariaDB {
		other = MySQL
	}
	detect := m.detect
	if detect == nil {
		detect = InstalledEngine
	}
	if detect() == other {
		return usererr.New("db_engine_conflict", other.Name())
	}
	if pkgs, err := apt.Installed(ctx, m.run, false, otherServerPackages(e)...); err == nil && len(pkgs) > 0 {
		return usererr.New("db_engine_conflict", other.Name())
	}
	switch de := m.dataDirEngine(); {
	case e == MySQL && (de == MariaDB || de == "?"):
		return usererr.New("db_datadir_other", m.dataDirPath())
	case e == MariaDB && de == MySQL:
		return usererr.New("db_datadir_other", m.dataDirPath())
	}
	return nil
}
