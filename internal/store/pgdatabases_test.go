package store

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
)

// migrationIndex returns the index of the migration whose SQL contains s.
func migrationIndex(t *testing.T, s string) int {
	t.Helper()
	for i, m := range migrations {
		if strings.Contains(m, s) {
			return i
		}
	}
	t.Fatalf("no migration contains %q", s)
	return 0
}

// The MariaDB functions see MariaDB databases only, the PG functions
// PostgreSQL ones, and names stay unique across both.
func TestDatabaseEngines(t *testing.T) {
	ctx := context.Background()
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	site := &Site{Domain: "example.com", SysUser: "example_com"}
	if err := st.CreateSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	maria := &Database{Name: "shop", Username: "shop", Password: "p1", SiteID: site.ID}
	pg := &Database{Name: "app", Username: "app", Password: "p2", SiteID: site.ID, Engine: EnginePostgres}
	for _, d := range []*Database{maria, pg} {
		if err := st.CreateDatabase(ctx, d); err != nil {
			t.Fatal(err)
		}
	}
	if maria.Engine != EngineMySQL {
		t.Fatalf("default engine %q", maria.Engine)
	}

	if l, err := st.ListDatabases(ctx); err != nil || len(l) != 1 || l[0].Name != "shop" {
		t.Fatalf("ListDatabases: %+v, %v", l, err)
	}
	if l, err := st.ListPGDatabases(ctx); err != nil || len(l) != 1 || l[0].Name != "app" || l[0].Engine != EnginePostgres {
		t.Fatalf("ListPGDatabases: %+v, %v", l, err)
	}
	if l, _ := st.DatabasesForSite(ctx, site.ID); len(l) != 1 || l[0].Name != "shop" {
		t.Fatalf("DatabasesForSite: %+v", l)
	}
	if l, _ := st.PGDatabasesForSite(ctx, site.ID); len(l) != 1 || l[0].Name != "app" {
		t.Fatalf("PGDatabasesForSite: %+v", l)
	}
	// The MariaDB code can never reach a PostgreSQL database, by ID or name.
	if _, err := st.DatabaseByID(ctx, pg.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DatabaseByID of a PG database: %v", err)
	}
	if _, err := st.DatabaseByName(ctx, "app"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DatabaseByName of a PG database: %v", err)
	}
	if _, err := st.PGDatabaseByID(ctx, maria.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("PGDatabaseByID of a MariaDB database: %v", err)
	}
	if d, err := st.PGDatabaseByName(ctx, "app"); err != nil || d.ID != pg.ID || d.Password != "p2" {
		t.Fatalf("PGDatabaseByName: %+v, %v", d, err)
	}
	// One name space for both engines.
	if taken, _ := st.DBNameTaken(ctx, "app"); !taken {
		t.Fatal("a PostgreSQL name is free for MariaDB")
	}
	if err := st.CreateDatabase(ctx, &Database{Name: "shop", Username: "shop2", Password: "x", Engine: EnginePostgres}); !errors.Is(err, ErrConflict) {
		t.Fatalf("same name on another engine: %v", err)
	}
	// Deleting the site unlinks both.
	if err := st.DeleteSite(ctx, site.ID); err != nil {
		t.Fatal(err)
	}
	if d, _ := st.PGDatabaseByID(ctx, pg.ID); d == nil || d.SiteID != 0 {
		t.Fatalf("after the site went: %+v", d)
	}
}

// A database from before migration 24: its databases are MariaDB ones.
func TestDatabaseEngineMigration(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	i := migrationIndex(t, "ADD COLUMN engine")
	// The later migrations run again below: app_memory and proxy_url are
	// undone here with it, and the site_protect table is created IF NOT
	// EXISTS.
	if i != len(migrations)-4 || migrationIndex(t, "ADD COLUMN app_memory") != i+1 ||
		!strings.Contains(migrations[i+2], "IF NOT EXISTS site_protect") ||
		migrationIndex(t, "ADD COLUMN proxy_url") != i+3 {
		t.Fatalf("migration %d is not the last but three: undo the later ones in this test", i+1)
	}
	if _, err := st.db.Exec(`ALTER TABLE databases DROP COLUMN engine; ALTER TABLE sites DROP COLUMN app_memory;
		ALTER TABLE sites DROP COLUMN proxy_url; ALTER TABLE sites DROP COLUMN proxy_opts;
		INSERT INTO databases (name, username, password, created_at) VALUES ('wp_shop', 'wp_shop', 'pw', 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`PRAGMA user_version = ` + strconv.Itoa(i)); err != nil {
		t.Fatal(err)
	}
	st.Close()
	if st, err = Open(dir); err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	var v int
	if err := st.db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil || v != len(migrations) {
		t.Fatalf("user_version %d, %v", v, err)
	}
	d, err := st.DatabaseByName(ctx, "wp_shop")
	if err != nil || d.Engine != EngineMySQL {
		t.Fatalf("old database: %+v, %v", d, err)
	}
	if l, err := st.ListPGDatabases(ctx); err != nil || len(l) != 0 {
		t.Fatalf("PG databases after the migration: %+v, %v", l, err)
	}
}
