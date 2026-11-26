package backup

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/validate"
)

// PostgreSQL databases go into backups next to the MariaDB ones, as
// postgres/<name>.sql (plain pg_dump output, made as the database's own
// role) and listed in the manifest's pg_databases. A panel without
// PostgreSQL support skips those entries when it restores.

// pgDir is the folder of PostgreSQL dumps in a backup archive.
const pgDir = "postgres/"

// Postgres is what backups need from the PostgreSQL manager.
type Postgres interface {
	Dump(ctx context.Context, name string, w io.Writer) error
	Create(ctx context.Context, name, password string, siteID int64) (*store.Database, error)
	Reset(ctx context.Context, name string) error
	Import(ctx context.Context, name string, r io.Reader) error
}

// SetPostgres lets backups include PostgreSQL databases.
func (m *Manager) SetPostgres(p Postgres) { m.pg = p }

// pgDatabasesForSite returns the PostgreSQL databases linked to a site.
func (m *Manager) pgDatabasesForSite(ctx context.Context, siteID int64) ([]*store.Database, error) {
	dbs, err := m.store.PGDatabasesForSite(ctx, siteID)
	if err != nil || len(dbs) == 0 || m.pg != nil {
		return dbs, err
	}
	return nil, errors.New("this panel cannot back up PostgreSQL databases")
}

// addPGDump dumps a PostgreSQL database to a temporary file (tar needs the
// size first) and adds it as postgres/<name>.sql.
func (m *Manager) addPGDump(ctx context.Context, tw *tar.Writer, name string, lg Logger) error {
	if m.pg == nil {
		return errors.New("this panel cannot back up PostgreSQL databases")
	}
	lg.Logf("Dumping PostgreSQL database %s", name)
	tmp, err := os.CreateTemp(m.dir, ".dump-*.sql")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if err := m.pg.Dump(ctx, name, tmp); err != nil {
		return err
	}
	fi, err := tmp.Stat()
	if err != nil {
		return err
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: pgDir + name + ".sql",
		Size: fi.Size(), Mode: 0o600, ModTime: m.now()}); err != nil {
		return err
	}
	_, err = io.Copy(tw, tmp)
	return err
}

// restorePG empties a PostgreSQL database and imports a dump into it. A
// database the panel no longer knows is created again (linked to the site),
// with a new password.
func (m *Manager) restorePG(ctx context.Context, name string, site *store.Site, r io.Reader, lg Logger) error {
	if err := validate.DBName(name); err != nil {
		return fmt.Errorf("backup contains an invalid database name %q", name)
	}
	if m.pg == nil {
		return errors.New("this panel cannot restore PostgreSQL databases")
	}
	if _, err := m.store.PGDatabaseByName(ctx, name); errors.Is(err, store.ErrNotFound) {
		var siteID int64
		if site != nil {
			siteID = site.ID
		}
		lg.Logf("Creating PostgreSQL database %s again", name)
		if _, err := m.pg.Create(ctx, name, "", siteID); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	lg.Logf("Restoring PostgreSQL database %s", name)
	if err := m.pg.Reset(ctx, name); err != nil {
		return err
	}
	return m.pg.Import(ctx, name, r)
}
