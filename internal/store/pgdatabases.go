package store

import "context"

// PostgreSQL databases live in the databases table too, with the engine
// column (migration 24) set to EnginePostgres. Sharing the table keeps the
// names of all databases and their users unique across engines, so a
// backup or a backup schedule of the kind "database" names one database
// without saying which engine, and a site's link to its databases works the
// same way. The functions in databases.go see EngineMySQL rows only.

// The engines of a database record.
const (
	EngineMySQL    = "mysql" // MariaDB or MySQL
	EnginePostgres = "postgresql"
)

// ListPGDatabases returns every PostgreSQL database record, sorted by name.
func (s *Store) ListPGDatabases(ctx context.Context) ([]*Database, error) {
	return s.queryDatabases(ctx, `SELECT `+dbCols+` FROM databases WHERE engine = 'postgresql' ORDER BY name`)
}

// PGDatabasesForSite returns the PostgreSQL databases linked to a site.
func (s *Store) PGDatabasesForSite(ctx context.Context, siteID int64) ([]*Database, error) {
	return s.queryDatabases(ctx, `SELECT `+dbCols+` FROM databases WHERE site_id = ? AND engine = 'postgresql' ORDER BY name`, siteID)
}

// PGDatabaseByID returns one PostgreSQL database record.
func (s *Store) PGDatabaseByID(ctx context.Context, id int64) (*Database, error) {
	return scanDatabase(s.db.QueryRowContext(ctx, `SELECT `+dbCols+` FROM databases WHERE id = ? AND engine = 'postgresql'`, id))
}

// PGDatabaseByName returns the PostgreSQL database record with the given name.
func (s *Store) PGDatabaseByName(ctx context.Context, name string) (*Database, error) {
	return scanDatabase(s.db.QueryRowContext(ctx, `SELECT `+dbCols+` FROM databases WHERE name = ? AND engine = 'postgresql'`, name))
}
