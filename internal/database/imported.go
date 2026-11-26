package database

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
)

// nativeHashRe is a mysql_native_password hash: "*" and 40 hex digits. It
// needs no quoting in SQL.
var nativeHashRe = regexp.MustCompile(`^\*[0-9A-F]{40}$`)

// NewDB describes a database brought over from another server (see
// package siteimport), whose website's configuration names its database,
// user and password.
type NewDB struct {
	Name string
	// User is the database's user, "" for one named like the database. It
	// may differ from Name, as on cPanel.
	User string
	// Password is the user's password when it is known (it must pass
	// validate.DBPassword). Otherwise Hash, a mysql_native_password hash
	// from the other server, keeps the old password working without the
	// panel knowing it: only on MariaDB (see AcceptsNativeHash; MySQL
	// refuses it with the usererr native_hash_unsupported). With neither, a
	// random password is made.
	Password string
	Hash     string
	SiteID   int64
}

// CreateWithUser makes a database and its user, like Create, but with the
// user name and password (or password hash) of the server it comes from.
// When only the hash is kept, the panel does not know the password: the
// record's password is empty until it is changed.
func (m *Manager) CreateWithUser(ctx context.Context, n NewDB) (*store.Database, error) {
	if err := m.ready(ctx); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	name := strings.ToLower(strings.TrimSpace(n.Name))
	user := strings.ToLower(strings.TrimSpace(n.User))
	if user == "" {
		user = name
	}
	if validate.DBName(name) != nil {
		return nil, usererr.New("invalid_db_name", name)
	}
	if validate.DBName(user) != nil {
		return nil, usererr.New("invalid_db_name", user)
	}
	password, hash := n.Password, strings.ToUpper(n.Hash)
	switch {
	case password != "":
		if validate.DBPassword(password) != nil {
			return nil, usererr.New("invalid_db_password")
		}
		hash = ""
	case hash != "":
		if !nativeHashRe.MatchString(hash) {
			return nil, fmt.Errorf("not a mysql_native_password hash")
		}
		if !m.AcceptsNativeHash(ctx) {
			return nil, usererr.New("native_hash_unsupported", user)
		}
	default:
		password = NewPassword()
	}
	if n.SiteID != 0 {
		if _, err := m.store.SiteByID(ctx, n.SiteID); err != nil {
			return nil, err
		}
	}
	for _, x := range []string{name, user} {
		if taken, err := m.store.DBNameTaken(ctx, x); err != nil {
			return nil, err
		} else if taken {
			return nil, usererr.New("db_exists", x)
		}
	}
	out, err := m.sql(ctx, fmt.Sprintf(`SELECT
 (SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = '%[1]s') +
 (SELECT COUNT(*) FROM mysql.user WHERE User = '%[1]s' OR User = '%[2]s');`, name, user))
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) != "0" {
		return nil, usererr.New("db_exists", name)
	}

	auth := "IDENTIFIED BY '" + password + "'"
	if hash != "" {
		auth = "IDENTIFIED BY PASSWORD '" + hash + "'"
	}
	if _, err := m.sql(ctx, fmt.Sprintf("CREATE DATABASE `%[1]s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;\n"+
		"CREATE USER '%[2]s'@'localhost' %[3]s;\n"+
		"GRANT ALL PRIVILEGES ON `%[4]s`.* TO '%[2]s'@'localhost';", name, user, auth, grantPattern(name))); err != nil {
		m.drop(context.WithoutCancel(ctx), name, user)
		return nil, err
	}
	d := &store.Database{Name: name, Username: user, Password: password, SiteID: n.SiteID}
	if err := m.store.CreateDatabase(ctx, d); err != nil {
		m.drop(context.WithoutCancel(ctx), name, user)
		return nil, err
	}
	return d, nil
}

// TakenNames returns the names a new database or database user cannot
// have: those of the panel's records and, while the server runs, its
// databases and users.
func (m *Manager) TakenNames(ctx context.Context) (map[string]bool, error) {
	taken := map[string]bool{}
	dbs, err := m.store.ListDatabases(ctx)
	if err != nil {
		return nil, err
	}
	for _, d := range dbs {
		taken[d.Name], taken[d.Username] = true, true
	}
	if !m.Status(ctx).Running {
		return taken, nil
	}
	out, err := m.sql(ctx, "SELECT SCHEMA_NAME FROM information_schema.SCHEMATA UNION SELECT User FROM mysql.user;")
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(out, "\n") {
		if n := strings.TrimSpace(line); n != "" {
			taken[strings.ToLower(n)] = true
		}
	}
	return taken, nil
}
