package store

import (
	"context"
	"strconv"
	"strings"
)

// Remote access to a PostgreSQL database is kept like MariaDB's (see
// dbaccess.go), in the settings table, under its own prefix
// "pg_remote_hosts.<id>": MariaDB's code counts every db_remote_hosts row
// when it decides whether MariaDB listens on all interfaces, and a
// PostgreSQL database must not make it do so.

const pgRemoteHostsPrefix = "pg_remote_hosts."

func pgRemoteHostsKey(dbID int64) string { return pgRemoteHostsPrefix + strconv.FormatInt(dbID, 10) }

// PGRemoteHosts returns the client addresses allowed to reach a PostgreSQL
// database from other machines, or an empty list.
func (s *Store) PGRemoteHosts(ctx context.Context, dbID int64) ([]string, error) {
	v, err := s.Setting(ctx, pgRemoteHostsKey(dbID))
	if err != nil {
		return nil, err
	}
	return splitHosts(v), nil
}

// SetPGRemoteHosts saves the client addresses of a PostgreSQL database; an
// empty list removes the row.
func (s *Store) SetPGRemoteHosts(ctx context.Context, dbID int64, hosts []string) error {
	return s.SetSetting(ctx, pgRemoteHostsKey(dbID), strings.Join(hosts, " "))
}

// AllPGRemoteHosts returns the client addresses of every PostgreSQL
// database that allows remote access, by database ID. Rows whose database
// is gone are left out.
func (s *Store) AllPGRemoteHosts(ctx context.Context) (map[int64][]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT d.id, s.value FROM settings s
		JOIN databases d ON s.key = 'pg_remote_hosts.' || d.id WHERE d.engine = 'postgresql'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]string{}
	for rows.Next() {
		var id int64
		var v string
		if err := rows.Scan(&id, &v); err != nil {
			return nil, err
		}
		if h := splitHosts(v); len(h) > 0 {
			out[id] = h
		}
	}
	return out, rows.Err()
}
