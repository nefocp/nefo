package database

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/nefocp/nefo/internal/usererr"
)

// Engine is the MySQL-compatible database server: MariaDB (the default,
// lighter on small servers) or MySQL 8.4 LTS from Oracle's repository. A
// server has one of them, never both. Everything else in this package
// (databases, users, import and export, remote access, the slow query log,
// removal) works the same on both; the differences are the programs and
// service names below, a few statements, and where the drop-ins live.
type Engine string

const (
	MariaDB Engine = "mariadb"
	MySQL   Engine = "mysql"
)

// The server programs. MariaDB 10.5 and later install mariadbd, and some
// versions add /usr/sbin/mysqld as a link to it, so MariaDB is looked for
// first.
const (
	mariadbBinary = "/usr/sbin/mariadbd"
	mysqlBinary   = "/usr/sbin/mysqld"
)

// Name is the engine's name for people: "MariaDB" or "MySQL".
func (e Engine) Name() string {
	if e == MySQL {
		return "MySQL"
	}
	return "MariaDB"
}

// service is the systemd unit: Oracle's packages call it mysql.
func (e Engine) service() string {
	if e == MySQL {
		return "mysql"
	}
	return "mariadb"
}

// client is the command-line client. MariaDB 11 warns when called as mysql.
func (e Engine) client() string {
	if e == MySQL {
		return "mysql"
	}
	return "mariadb"
}

func (e Engine) dumper() string {
	if e == MySQL {
		return "mysqldump"
	}
	return "mariadb-dump"
}

func (e Engine) daemon() string {
	if e == MySQL {
		return "mysqld"
	}
	return "mariadbd"
}

// confSubdir is the folder of /etc/mysql that the engine's main
// configuration includes last, where the panel's drop-ins go. Both read
// the files in it in name order.
func (e Engine) confSubdir() string {
	if e == MySQL {
		return "mysql.conf.d"
	}
	return "mariadb.conf.d"
}

// InstalledEngine reports which database server is installed, without
// running anything: MariaDB, MySQL or "" for none.
func InstalledEngine() Engine {
	if _, err := os.Stat(mariadbBinary); err == nil {
		return MariaDB
	}
	if _, err := os.Stat(mysqlBinary); err == nil {
		return MySQL
	}
	return ""
}

// hostEngine finds the installed server for a Manager without a detect
// hook. Tests replace it (see TestMain): a test machine may have a real
// MySQL, such as GitHub's runners.
var hostEngine = InstalledEngine

// engine is the server the manager talks to: the installed one, or MariaDB
// while none is (its Status then says it is not installed).
func (m *Manager) engine() Engine {
	detect := m.detect
	if detect == nil {
		detect = hostEngine
	}
	if e := detect(); e != "" {
		return e
	}
	return MariaDB
}

// Engine returns the installed database server, or "" when none is.
func (m *Manager) Engine(ctx context.Context) Engine {
	if !m.Status(ctx).Installed {
		return ""
	}
	return m.engine()
}

// AcceptsNativeHash reports whether a user can be created from a
// mysql_native_password hash (see CreateWithUser), as cPanel and older
// MySQL servers store them. MariaDB can. MySQL 8.4 ships the plugin
// disabled (and MySQL 9 removed it): the panel leaves it off, as it hashes
// with unsalted SHA-1, so on MySQL an imported user needs its password, or
// a new one.
func (m *Manager) AcceptsNativeHash(ctx context.Context) bool {
	return m.engine() == MariaDB
}

func (m *Manager) confDir(e Engine) string {
	return filepath.Join(m.etcDirPath(), e.confSubdir())
}

// remoteConfPath is the listen drop-in (see remoteConfFile).
func (m *Manager) remoteConfPath() string {
	if m.remoteConf != "" {
		return m.remoteConf
	}
	return filepath.Join(m.confDir(m.engine()), remoteConfName)
}

func (m *Manager) slowConfPath() string {
	if m.slowConf != "" {
		return m.slowConf
	}
	return filepath.Join(m.confDir(m.engine()), slowConfName)
}

// slowLogFile is where the engine writes the slow query log, in the
// directory both packages create for their logs.
func (e Engine) slowLogFile() string {
	if e == MySQL {
		return "/var/log/mysql/mysql-slow.log"
	}
	return slowLogFile
}

// slowLogPath is the slow query log the viewer reads.
func (m *Manager) slowLogPath() string {
	if m.slowLog != "" {
		return m.slowLog
	}
	return m.engine().slowLogFile()
}

// mysqlUserMax is the longest user name MySQL accepts; MariaDB takes 80.
const mysqlUserMax = 32

// importUser is the name of the temporary account an import into the
// database name runs as (see Import). On MySQL, where names stop at 32
// characters, a long database name is cut and a hash of it added, so each
// database still gets its own account.
func (m *Manager) importUser(name string) string {
	u := importUserPrefix + name
	if m.engine() != MySQL || len(u) <= mysqlUserMax {
		return u
	}
	sum := sha256.Sum256([]byte(name))
	h := hex.EncodeToString(sum[:4])
	keep := mysqlUserMax - len(importUserPrefix) - 1 - len(h)
	return importUserPrefix + name[:keep] + "-" + h
}

// importDefinerGrants are the statements that let the import account
// create triggers, views and routines for another DEFINER, as a dump made
// with --routines --triggers needs, in the order they are tried. MariaDB
// 10.5.2 and later have SET USER and older versions SUPER. MySQL 8.2 split
// what SUPER allowed here into SET_ANY_DEFINER and ALLOW_NONEXISTENT_DEFINER
// (8.4 has no SET_USER_ID any more); there, an account without SYSTEM_USER
// still cannot make an object run as root.
func (m *Manager) importDefinerGrants(user string) []string {
	if m.engine() == MySQL {
		return []string{"GRANT SET_ANY_DEFINER, ALLOW_NONEXISTENT_DEFINER ON *.* TO '" + user + "'@'localhost';"}
	}
	return []string{
		"GRANT SET USER ON *.* TO '" + user + "'@'localhost';",
		"GRANT SUPER ON *.* TO '" + user + "'@'localhost';",
	}
}

// CheckInstall returns why engine e cannot be installed now (a usererr), or
// nil: MySQL where Oracle's repository has no packages for the server (see
// MySQLAvailable), either one while the other is installed, or over the
// other's data (see checkInstall). Install and InstallMySQL check the same.
func (m *Manager) CheckInstall(ctx context.Context, e Engine) error {
	if !m.supported() {
		return usererr.New("unsupported")
	}
	if e == MySQL {
		if err := m.MySQLAvailable(); err != nil {
			return err
		}
	}
	return m.checkInstall(ctx, e)
}
