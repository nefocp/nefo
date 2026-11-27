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

// dbEngineOf maps a Software page ID to its engine: "mariadb" (and its
// series, "mariadb-11.4") or "mysql".
func dbEngineOf(id string) (database.Engine, bool) {
	if _, ok := database.ParseMariaDBID(id); ok {
		return database.MariaDB, true
	}
	if id == "mysql" {
		return database.MySQL, true
	}
	return "", false
}

// checkDBInstall returns why the database server id ("mariadb",
// "mariadb-11.4" or "mysql") cannot be installed now, before a job starts;
// nil for other IDs.
func (s *Server) checkDBInstall(ctx context.Context, id string) error {
	if series, ok := database.ParseMariaDBID(id); ok {
		return s.db.CheckInstallMariaDB(ctx, series)
	}
	e, ok := dbEngineOf(id)
	if !ok {
		return nil
	}
	return s.db.CheckInstall(ctx, e)
}

// mariadbItems are the Software page's MariaDB item, and one item per
// series of MariaDB's repository (mariadb-11.4…), which the database
// server card and the setup offer as versions of MariaDB. An installed
// series' item is installed too; the others then say what blocks them.
func (s *Server) mariadbItems(ctx context.Context, st database.Status) (software.Item, []software.Item) {
	installed := st.Installed && st.Engine == database.MariaDB
	maria := software.Item{ID: "mariadb", Kind: "mariadb", Group: "data", Installed: installed, Source: "apt",
		RAMMB: database.MemoryMBFor(database.MariaDB), DataDir: "/var/lib/mysql", HasData: softwarePathExists("/var/lib/mysql")}
	choices := s.db.MariaDBChoices()
	if installed {
		maria.Version = st.Version
		if st.Source == "mariadb" {
			maria.Source = "official"
		}
		if dbs, err := s.db.UserDatabases(ctx); err == nil {
			maria.Usage = map[string]int{"databases": len(dbs)}
		}
		maria.Block(s.db.CheckRemove(ctx, false, ""))
		maria.Stopped = !st.Running
	} else if len(choices) > 0 {
		maria.Version = choices[0].Series
		if r := reasonOf(choices[0].Unavailable); r != nil {
			maria.NotOffered, maria.NotOfferedDetail = r.Code, r.Detail
		}
	}
	var series []software.Item
	for _, c := range choices[1:] {
		it := software.Item{ID: c.ID, Kind: "mariadb", Group: "data", Version: c.Series, ThirdParty: true, Source: "official",
			SecurityUntil: c.Until, RAMMB: maria.RAMMB, DataDir: maria.DataDir, HasData: maria.HasData}
		if installed && st.Source == "mariadb" && st.Series == c.Series {
			it.Installed, it.Usage, it.Stopped = true, maria.Usage, maria.Stopped
			it.Block(s.db.CheckRemove(ctx, false, ""))
		} else if r := reasonOf(c.Unavailable); r != nil {
			it.NotOffered, it.NotOfferedDetail = r.Code, r.Detail
		}
		series = append(series, it)
	}
	return maria, series
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
