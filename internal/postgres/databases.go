package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/nefocp/nefo/internal/secure"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
)

// reserved are names PostgreSQL itself, pg_hba.conf or SQL give a meaning
// to, on top of validate.DBName's list. Names starting with pg_ are
// reserved for PostgreSQL's own roles.
var reserved = map[string]bool{
	"postgres": true, "template0": true, "template1": true, "public": true, "none": true,
	"all": true, "sameuser": true, "samerole": true, "samegroup": true, "replication": true,
	"current_user": true, "session_user": true, "current_role": true, "user": true,
	"pg_signal_backend": true,
}

// checkName checks a database or role name made by the panel.
func checkName(name string) error {
	if validate.DBName(name) != nil || reserved[name] || strings.HasPrefix(name, "pg_") {
		return usererr.New("invalid_db_name", name)
	}
	return nil
}

// ident returns a checked name as a quoted SQL identifier. The allow-list
// (a-z, 0-9 and _) means nothing needs escaping; quoting keeps SQL keywords
// usable as names.
func ident(name string) (string, error) {
	if err := checkName(name); err != nil {
		return "", err
	}
	return `"` + name + `"`, nil
}

// literal returns s as an SQL string literal: quotes doubled, and the E”
// form with doubled backslashes when there are any, so it means the same
// whatever standard_conforming_strings says.
func literal(s string) (string, error) {
	if strings.ContainsRune(s, 0) {
		return "", errors.New("a NUL byte in an SQL string")
	}
	q := "'" + strings.ReplaceAll(s, "'", "''") + "'"
	if strings.Contains(s, `\`) {
		q = "E" + strings.ReplaceAll(q, `\`, `\\`)
	}
	return q, nil
}

// NewPassword returns a random database password.
func NewPassword() string { return secure.String(24, secure.PasswordChars) }

// Info is a database with its size and owner, for the list in the UI.
type Info struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Username   string `json:"username"`
	Owner      string `json:"owner,omitempty"` // the role that owns it, as the server says
	SiteID     int64  `json:"site_id,omitempty"`
	SiteDomain string `json:"site_domain,omitempty"`
	Size       int64  `json:"size"`
	// Missing means the database was removed outside the panel.
	Missing   bool   `json:"missing"`
	CreatedAt string `json:"created_at"`
	// RemoteHosts are the client addresses that may connect from other
	// machines (see SetRemote).
	RemoteHosts []string `json:"remote_hosts"`
}

// List returns the panel's PostgreSQL databases with their sizes and owners.
func (m *Manager) List(ctx context.Context) ([]Info, error) {
	dbs, err := m.store.ListPGDatabases(ctx)
	if err != nil {
		return nil, err
	}
	type row struct {
		size  int64
		owner string
	}
	rows := map[string]row{}
	known := false
	if c, err := m.ready(ctx); err == nil {
		out, err := m.sql(ctx, c, "postgres", `SELECT d.datname, pg_database_size(d.oid), r.rolname
FROM pg_database d JOIN pg_roles r ON r.oid = d.datdba WHERE NOT d.datistemplate;`)
		if err == nil {
			known = true
			for _, line := range strings.Split(out, "\n") {
				if f := strings.Split(line, "\t"); len(f) == 3 {
					size, _ := strconv.ParseInt(f[1], 10, 64)
					rows[f[0]] = row{size, f[2]}
				}
			}
		}
	}
	sites, err := m.store.ListSites(ctx)
	if err != nil {
		return nil, err
	}
	domains := map[int64]string{}
	for _, s := range sites {
		domains[s.ID] = s.Domain
	}
	remote, err := m.store.AllPGRemoteHosts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Info, 0, len(dbs))
	for _, d := range dbs {
		r, ok := rows[d.Name]
		hosts := remote[d.ID]
		if hosts == nil {
			hosts = []string{}
		}
		out = append(out, Info{
			ID: d.ID, Name: d.Name, Username: d.Username, Owner: r.owner, SiteID: d.SiteID, SiteDomain: domains[d.SiteID],
			Size: r.size, Missing: known && !ok, CreatedAt: d.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
			RemoteHosts: hosts,
		})
	}
	return out, nil
}

// checkPassword returns password, or a random one when it is empty.
func checkPassword(password string) (string, error) {
	if password == "" {
		return NewPassword(), nil
	}
	if err := validate.DBPassword(password); err != nil {
		return "", usererr.New("invalid_db_password")
	}
	return password, nil
}

// roleSQL creates the login role of a database: no superuser, no other
// databases, no other roles, no replication.
func roleSQL(name, password string) (string, error) {
	id, err := ident(name)
	if err != nil {
		return "", err
	}
	pw, err := literal(scramVerifier(password))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("CREATE ROLE %s LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD %s;\n", id, pw), nil
}

// passwordSQL sets a role's password.
func passwordSQL(name, password string) (string, error) {
	id, err := ident(name)
	if err != nil {
		return "", err
	}
	pw, err := literal(scramVerifier(password))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("ALTER ROLE %s PASSWORD %s;\n", id, pw), nil
}

// databaseSQL creates a database owned by its role, in UTF-8 whatever the
// cluster's default, that no other role may connect to (by default every
// role may connect to every database).
func databaseSQL(name, owner string) (string, error) {
	db, err := ident(name)
	if err != nil {
		return "", err
	}
	role, err := ident(owner)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("CREATE DATABASE %[1]s OWNER %[2]s TEMPLATE template0 ENCODING 'UTF8' LC_COLLATE 'C.UTF-8' LC_CTYPE 'C.UTF-8';\n"+
		"REVOKE ALL ON DATABASE %[1]s FROM PUBLIC;\nGRANT ALL ON DATABASE %[1]s TO %[2]s;\n", db, role), nil
}

// dropSQL drops a database (closing its connections) and its role.
func dropSQL(name, owner string) (string, error) {
	db, err := ident(name)
	if err != nil {
		return "", err
	}
	role, err := ident(owner)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE);\nDROP ROLE IF EXISTS %s;\n", db, role), nil
}

// ownSchema gives the owner the public schema of a new database. Since
// PostgreSQL 15 the database owner owns it already; 14 gives it to postgres.
func (m *Manager) ownSchema(ctx context.Context, c cluster, name, owner string) error {
	if c.major() >= 15 {
		return nil
	}
	role, err := ident(owner)
	if err != nil {
		return err
	}
	_, err = m.sql(ctx, c, name, "ALTER SCHEMA public OWNER TO "+role+";\n")
	return err
}

// Create makes a database and a role of the same name that owns it and may
// only use it. An empty password is replaced by a random one.
func (m *Manager) Create(ctx context.Context, name, password string, siteID int64) (*store.Database, error) {
	c, err := m.ready(ctx)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	name = strings.ToLower(strings.TrimSpace(name))
	if err := checkName(name); err != nil {
		return nil, err
	}
	if password, err = checkPassword(password); err != nil {
		return nil, err
	}
	if siteID != 0 {
		if _, err := m.store.SiteByID(ctx, siteID); err != nil {
			return nil, err
		}
	}
	if taken, err := m.store.DBNameTaken(ctx, name); err != nil {
		return nil, err
	} else if taken {
		return nil, usererr.New("db_exists", name)
	}
	lit, _ := literal(name) // checked
	out, err := m.sql(ctx, c, "postgres", fmt.Sprintf(
		"SELECT (SELECT count(*) FROM pg_database WHERE datname = %[1]s) + (SELECT count(*) FROM pg_roles WHERE rolname = %[1]s);\n", lit))
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(out) != "0" {
		return nil, usererr.New("db_exists", name)
	}
	role, err := roleSQL(name, password)
	if err != nil {
		return nil, err
	}
	db, err := databaseSQL(name, name)
	if err != nil {
		return nil, err
	}
	// CREATE DATABASE cannot run in a transaction: on an error, drop what
	// was made.
	undo := func() { m.drop(context.WithoutCancel(ctx), c, name, name) }
	if _, err := m.sql(ctx, c, "postgres", role+db); err != nil {
		undo()
		return nil, err
	}
	if err := m.ownSchema(ctx, c, name, name); err != nil {
		undo()
		return nil, err
	}
	d := &store.Database{Name: name, Username: name, Password: password, SiteID: siteID, Engine: store.EnginePostgres}
	if err := m.store.CreateDatabase(ctx, d); err != nil {
		undo()
		if errors.Is(err, store.ErrConflict) {
			return nil, usererr.New("db_exists", name)
		}
		return nil, err
	}
	return d, nil
}

func (m *Manager) drop(ctx context.Context, c cluster, name, owner string) error {
	q, err := dropSQL(name, owner)
	if err != nil {
		return err
	}
	_, err = m.sql(ctx, c, "postgres", q)
	return err
}

// SetPassword changes a database role's password. An empty password is
// replaced by a random one.
func (m *Manager) SetPassword(ctx context.Context, id int64, password string) (*store.Database, error) {
	c, err := m.ready(ctx)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	d, err := m.store.PGDatabaseByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if password, err = checkPassword(password); err != nil {
		return nil, err
	}
	q, err := passwordSQL(d.Username, password)
	if err != nil {
		return nil, err
	}
	if _, err := m.sql(ctx, c, "postgres", q); err != nil {
		return nil, err
	}
	d.Password = password
	if err := m.store.UpdateDatabase(ctx, d); err != nil {
		return nil, err
	}
	return d, nil
}

// Delete drops a database and its role. confirm must equal the name. While
// PostgreSQL is installed but not running, it is refused: the panel would
// forget a database that still exists. Its remote access goes with it: its
// pg_hba.conf lines and ufw rules, and the listen drop-in when it was the
// last database with remote access (the server then listens on all
// interfaces until it restarts: see RestartPending).
func (m *Manager) Delete(ctx context.Context, id int64, confirm string) error {
	d, err := m.store.PGDatabaseByID(ctx, id)
	if err != nil {
		return err
	}
	if strings.TrimSpace(confirm) != d.Name {
		return usererr.New("confirm_mismatch")
	}
	c, err := m.ready(ctx)
	var ue *usererr.Error
	if err != nil && !(errors.As(err, &ue) && ue.Code == "pg_not_installed") {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err == nil {
		if err := m.drop(ctx, c, d.Name, d.Username); err != nil {
			return err
		}
	}
	if hosts, herr := m.store.PGRemoteHosts(ctx, d.ID); herr != nil {
		return herr
	} else if len(hosts) > 0 {
		if rerr := m.removeRemote(ctx, d, c, err == nil); rerr != nil {
			return rerr
		}
	}
	return m.store.DeleteDatabase(ctx, d.ID)
}

// Reset empties a database before a backup is restored into it: it is
// dropped and created again, owned by the same role.
func (m *Manager) Reset(ctx context.Context, name string) error {
	c, err := m.ready(ctx)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	d, err := m.store.PGDatabaseByName(ctx, name)
	if err != nil {
		return err
	}
	id, err := ident(d.Name)
	if err != nil {
		return err
	}
	create, err := databaseSQL(d.Name, d.Username)
	if err != nil {
		return err
	}
	if _, err := m.sql(ctx, c, "postgres", "DROP DATABASE IF EXISTS "+id+" WITH (FORCE);\n"+create); err != nil {
		return err
	}
	return m.ownSchema(ctx, c, d.Name, d.Username)
}
