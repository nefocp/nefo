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

// Uninstalling MariaDB. It is refused while the server holds any database
// other than MariaDB's own, whoever made it: MariaDB itself is asked, not
// the panel's list, which does not know databases made by hand or by
// another panel on the same server. The data folder stays unless the
// administrator asks for it to be deleted and types "mariadb".

// systemDatabases are MariaDB's own.
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

// Installed reports whether the MariaDB server is installed, without
// running anything.
func Installed() bool {
	_, err := os.Stat("/usr/sbin/mariadbd")
	return err == nil
}

// userDatabase reports whether a name from SHOW DATABASES (or a folder in
// the data directory) is a database someone made.
func userDatabase(name string) bool {
	return name != "" && !slices.Contains(systemDatabases, name) &&
		!strings.HasPrefix(name, "#") && name != "lost+found"
}

// UserDatabases lists the databases on the server other than MariaDB's
// own. When MariaDB is not running, the folders in its data directory are
// listed instead; if the data directory is not the default one, that is not
// possible and the answer is the usererr mariadb_not_running.
func (m *Manager) UserDatabases(ctx context.Context) ([]string, error) {
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
		return nil, usererr.New("mariadb_not_running")
	}
	entries, err := os.ReadDir(m.dataDirPath())
	if errors.Is(err, fs.ErrNotExist) {
		return names, nil
	}
	if err != nil {
		return nil, usererr.New("mariadb_not_running")
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

// customDataDir reports whether a MariaDB configuration file moves the data
// directory away from the default.
func (m *Manager) customDataDir() bool {
	etc := m.etcDirPath()
	files := []string{filepath.Join(etc, "my.cnf"), filepath.Join(etc, "mariadb.cnf")}
	for _, d := range []string{"conf.d", "mariadb.conf.d"} {
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
	if !m.supported() {
		return usererr.New("unsupported")
	}
	if !m.Status(ctx).Installed {
		return usererr.New("software_not_installed", "MariaDB")
	}
	if deleteData && strings.TrimSpace(confirm) != "mariadb" {
		return usererr.New("software_confirm_mismatch", "mariadb")
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
		return usererr.New("mariadb_has_databases", strings.Join(list, ", "))
	}
	return nil
}

var serverPkgRe = regexp.MustCompile(`^mariadb-server(-[0-9.]+)?$`)

// Remove purges the MariaDB server and client packages and the panel's
// configuration files for them. The data directory is kept, so installing
// MariaDB again finds it, unless deleteData is set.
func (m *Manager) Remove(ctx context.Context, deleteData bool, confirm string, lg Logger) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.CheckRemove(ctx, deleteData, confirm); err != nil {
		return err
	}
	pkgs, err := apt.Installed(ctx, m.run, true, "mariadb-server*", "mariadb-client*", "mariadb-plugin-*",
		"mariadb-backup*", "default-mysql-server*", "default-mysql-client*")
	if err != nil {
		return err
	}
	// The purge script asks whether to delete the databases. apt runs
	// without questions and the answer defaults to no; say so explicitly,
	// so an earlier answer cannot delete them.
	var answers strings.Builder
	for _, p := range pkgs {
		if serverPkgRe.MatchString(p) {
			answers.WriteString(p + " " + p + "/postrm_remove_databases boolean false\n")
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
	lg.Logf("Stopping MariaDB")
	if out, err := m.run.Output(ctx, "systemctl", "stop", "mariadb"); err != nil {
		return fmt.Errorf("stopping MariaDB failed: %s", strings.TrimSpace(out))
	}
	lg.Logf("Removing MariaDB")
	if err := apt.Remove(ctx, m.run, lg, true, pkgs...); err != nil {
		return err
	}
	confs := []string{filepath.Join(m.etcDirPath(), "mariadb.conf.d", "90-panel.cnf"), m.remoteConf, m.slowConf}
	more, _ := filepath.Glob(filepath.Join(m.etcDirPath(), "mariadb.conf.d", "99-panel-*.cnf"))
	for _, f := range append(confs, more...) {
		if err := os.Remove(f); err == nil {
			lg.Logf("Removed %s", f)
		}
	}
	m.listenDirty = false
	dir := m.dataDirPath()
	if !deleteData {
		if _, err := os.Stat(dir); err == nil {
			lg.Logf("MariaDB is removed. Its data directory %s was kept; installing MariaDB again uses it.", dir)
		} else {
			lg.Logf("MariaDB is removed.")
		}
		return nil
	}
	if out, err := m.run.Output(ctx, "systemctl", "is-active", "mariadb"); err == nil && strings.HasPrefix(out, "active") {
		return errors.New("MariaDB is still running, so its data directory was not deleted")
	}
	lg.Logf("Deleting %s", dir)
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	lg.Logf("MariaDB is removed, with its data directory.")
	return nil
}

// MemoryMB is roughly how much memory MariaDB uses with the panel's
// settings on this server: its buffer pool and about 100 MB more.
func MemoryMB() int {
	switch t := memTotal(); {
	case t >= 7<<30:
		return 1024 + 100
	case t >= 3<<30:
		return 512 + 100
	case t >= 1500<<20:
		return 256 + 100
	}
	return 128 + 100
}
