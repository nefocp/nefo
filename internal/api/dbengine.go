package api

import (
	"context"
	"errors"

	"github.com/nefocp/nefo/internal/database"
	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/software"
	"github.com/nefocp/nefo/internal/usererr"
)

// The database server is MariaDB or MySQL 8.4, chosen when it is
// installed: the Software page's items "mariadb" and "mysql" (also the
// setup's choices), POST /api/software/{id}/install, or POST
// /api/db/install with {"engine": "mysql"}. A server has one of them; the
// other's install is refused while one is installed.

// reason is a usererr as JSON, for why something is not offered.
type reason struct {
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}

func reasonOf(err error) *reason {
	if err == nil {
		return nil
	}
	var ue *usererr.Error
	if errors.As(err, &ue) {
		return &reason{ue.Code, ue.Detail}
	}
	return &reason{Code: "operation_failed", Detail: err.Error()}
}

// dbEngineOf maps a Software page ID to its engine.
func dbEngineOf(id string) (database.Engine, bool) {
	switch id {
	case "mariadb":
		return database.MariaDB, true
	case "mysql":
		return database.MySQL, true
	}
	return "", false
}

// checkDBInstall returns why the database server id ("mariadb" or
// "mysql") cannot be installed now, before a job starts; nil for other IDs.
func (s *Server) checkDBInstall(ctx context.Context, id string) error {
	e, ok := dbEngineOf(id)
	if !ok {
		return nil
	}
	return s.db.CheckInstall(ctx, e)
}

// installDB installs the database server e.
func (s *Server) installDB(ctx context.Context, e database.Engine, lg software.Logger) error {
	if e == database.MySQL {
		return s.db.InstallMySQL(ctx, lg)
	}
	return s.db.Install(ctx, lg)
}

// dbToolItems are the Software page's MySQL and phpMyAdmin items. Where
// Oracle's repository has no MySQL packages for the server, MySQL's item
// says why (NotOffered), so the setup leaves it out too.
func (s *Server) dbToolItems(ctx context.Context, st database.Status) []software.Item {
	mysql := software.Item{ID: "mysql", Kind: "mysql", Group: "data", Installed: st.Installed && st.Engine == database.MySQL,
		RAMMB: database.MemoryMBFor(database.MySQL), DataDir: "/var/lib/mysql", HasData: softwarePathExists("/var/lib/mysql")}
	if mysql.Installed {
		mysql.Version = st.Version
		if dbs, err := s.db.UserDatabases(ctx); err == nil {
			mysql.Usage = map[string]int{"databases": len(dbs)}
		}
		mysql.Block(s.db.CheckRemoveEngine(ctx, database.MySQL, false, ""))
		mysql.Stopped = !st.Running
	} else {
		mysql.Version = "8.4 LTS"
		if r := reasonOf(s.db.MySQLAvailable()); r != nil {
			mysql.NotOffered, mysql.NotOfferedDetail = r.Code, r.Detail
		}
	}
	pma := software.Item{ID: "phpmyadmin", Kind: "phpmyadmin", Group: "data", Version: hosting.PhpMyAdminVersion,
		Installed: s.hosting.PhpMyAdminInstalled()}
	if pma.Installed {
		pma.Block(nil)
	}
	return []software.Item{mysql, pma}
}

// mysqlUnavailable says why MySQL is not offered on this server, nil when
// it is (or is installed).
func (s *Server) mysqlUnavailable(st database.Status) *reason {
	if st.Engine == database.MySQL {
		return nil
	}
	return reasonOf(s.db.MySQLAvailable())
}

// appDBServer is what a PHP app's install needs to know of the database
// server: MariaDB's version, by which Moodle picks its release (on MySQL
// 8.4 the oldest release of the catalog, which needs the least), and the
// engine, for the apps with a driver per server.
func appDBServer(st database.Status) (mariadb, engine string) {
	if st.Engine == database.MySQL {
		return "", string(database.MySQL)
	}
	return st.Version, string(database.MariaDB)
}
