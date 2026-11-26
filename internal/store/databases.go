package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Database is a MariaDB database with its own user, managed by the panel.
type Database struct {
	ID       int64
	Name     string
	Username string
	// Password is kept so the administrator can look it up again, as in
	// other panels. The panel database is readable by root only.
	Password  string
	SiteID    int64 // 0 when not linked to a site
	CreatedAt time.Time
	// Engine is EngineMySQL (MariaDB or MySQL) or EnginePostgres. The
	// functions of this file only see EngineMySQL databases, so the MariaDB
	// code never touches a PostgreSQL one; pgdatabases.go has the others.
	Engine string
}

const dbCols = `id, name, username, password, COALESCE(site_id, 0), created_at, engine`

func scanDatabase(row scanner) (*Database, error) {
	var d Database
	var created int64
	err := row.Scan(&d.ID, &d.Name, &d.Username, &d.Password, &d.SiteID, &created, &d.Engine)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	d.CreatedAt = time.Unix(created, 0)
	return &d, nil
}

func nullID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

// CreateDatabase stores a database record and sets its ID.
func (s *Store) CreateDatabase(ctx context.Context, d *Database) error {
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now()
	}
	if d.Engine == "" {
		d.Engine = EngineMySQL
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO databases (name, username, password, site_id, created_at, engine) VALUES (?, ?, ?, ?, ?, ?)`,
		d.Name, d.Username, d.Password, nullID(d.SiteID), d.CreatedAt.Unix(), d.Engine)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	d.ID, err = res.LastInsertId()
	return err
}

// ListDatabases returns every MariaDB database record, sorted by name.
func (s *Store) ListDatabases(ctx context.Context) ([]*Database, error) {
	return s.queryDatabases(ctx, `SELECT `+dbCols+` FROM databases WHERE engine = 'mysql' ORDER BY name`)
}

// DatabasesForSite returns the MariaDB databases linked to a site.
func (s *Store) DatabasesForSite(ctx context.Context, siteID int64) ([]*Database, error) {
	return s.queryDatabases(ctx, `SELECT `+dbCols+` FROM databases WHERE site_id = ? AND engine = 'mysql' ORDER BY name`, siteID)
}

func (s *Store) queryDatabases(ctx context.Context, q string, args ...any) ([]*Database, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Database{}
	for rows.Next() {
		d, err := scanDatabase(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DatabaseByID returns one MariaDB database record.
func (s *Store) DatabaseByID(ctx context.Context, id int64) (*Database, error) {
	return scanDatabase(s.db.QueryRowContext(ctx, `SELECT `+dbCols+` FROM databases WHERE id = ? AND engine = 'mysql'`, id))
}

// UpdateDatabase saves the password and site link of a database.
func (s *Store) UpdateDatabase(ctx context.Context, d *Database) error {
	res, err := s.db.ExecContext(ctx, `UPDATE databases SET password = ?, site_id = ? WHERE id = ?`,
		d.Password, nullID(d.SiteID), d.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteDatabase removes a database record.
func (s *Store) DeleteDatabase(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM databases WHERE id = ?`, id)
	return err
}

// DBNameTaken reports whether a database or database user of any engine
// already uses name.
func (s *Store) DBNameTaken(ctx context.Context, name string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM databases WHERE name = ? OR username = ?`, name, name).Scan(&n)
	return n > 0, err
}

// DatabaseByName returns the MariaDB database record with the given name.
func (s *Store) DatabaseByName(ctx context.Context, name string) (*Database, error) {
	return scanDatabase(s.db.QueryRowContext(ctx, `SELECT `+dbCols+` FROM databases WHERE name = ? AND engine = 'mysql'`, name))
}
