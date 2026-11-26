package store

import (
	"context"
	"strconv"
	"strings"
)

// Remote access to a database is kept in the settings table, one row per
// database: the key "db_remote_hosts.<id>" holds the client addresses that
// may connect from other machines, separated by spaces. No migration is
// needed, and a row without its database is ignored.

const remoteHostsPrefix = "db_remote_hosts."

func remoteHostsKey(dbID int64) string { return remoteHostsPrefix + strconv.FormatInt(dbID, 10) }

// DBRemoteHosts returns the client addresses allowed to reach a database
// from other machines, or an empty list.
func (s *Store) DBRemoteHosts(ctx context.Context, dbID int64) ([]string, error) {
	v, err := s.Setting(ctx, remoteHostsKey(dbID))
	if err != nil {
		return nil, err
	}
	return splitHosts(v), nil
}

// SetDBRemoteHosts saves the client addresses of a database; an empty list
// removes the row.
func (s *Store) SetDBRemoteHosts(ctx context.Context, dbID int64, hosts []string) error {
	return s.SetSetting(ctx, remoteHostsKey(dbID), strings.Join(hosts, " "))
}

// AllDBRemoteHosts returns the client addresses of every database that
// allows remote access, by database ID.
func (s *Store) AllDBRemoteHosts(ctx context.Context) (map[int64][]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT d.id, s.value FROM settings s
		JOIN databases d ON s.key = 'db_remote_hosts.' || d.id`)
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

func splitHosts(v string) []string {
	f := strings.Fields(v)
	if f == nil {
		return []string{}
	}
	return f
}
