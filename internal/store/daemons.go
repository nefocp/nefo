package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Daemon is a background program the panel keeps running as a website's
// Linux user, such as a queue worker ("php artisan queue:work").
type Daemon struct {
	ID      int64
	Name    string
	SiteID  int64
	Command string
	// Dir is the working folder, relative to the site's folder; "" is the
	// site's folder itself.
	Dir string
	// Env holds KEY=value lines.
	Env       []string
	Enabled   bool
	CreatedAt time.Time
}

const daemonCols = `id, name, site_id, command, dir, env, enabled, created_at`

func scanDaemon(row scanner) (*Daemon, error) {
	var d Daemon
	var env string
	var created int64
	err := row.Scan(&d.ID, &d.Name, &d.SiteID, &d.Command, &d.Dir, &env, &d.Enabled, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	d.Env = splitLines(env)
	d.CreatedAt = time.Unix(created, 0)
	return &d, nil
}

func splitLines(s string) []string {
	out := []string{}
	for _, l := range strings.Split(s, "\n") {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

func (s *Store) queryDaemons(ctx context.Context, where string, args ...any) ([]*Daemon, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+daemonCols+` FROM daemons `+where+` ORDER BY id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Daemon{}
	for rows.Next() {
		d, err := scanDaemon(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ListDaemons returns all background programs.
func (s *Store) ListDaemons(ctx context.Context) ([]*Daemon, error) {
	return s.queryDaemons(ctx, "")
}

// DaemonsForSite returns the background programs of one website.
func (s *Store) DaemonsForSite(ctx context.Context, siteID int64) ([]*Daemon, error) {
	return s.queryDaemons(ctx, "WHERE site_id = ?", siteID)
}

// DaemonByID returns one background program.
func (s *Store) DaemonByID(ctx context.Context, id int64) (*Daemon, error) {
	return scanDaemon(s.db.QueryRowContext(ctx, `SELECT `+daemonCols+` FROM daemons WHERE id = ?`, id))
}

// SaveDaemon inserts (ID 0) or updates a background program.
func (s *Store) SaveDaemon(ctx context.Context, d *Daemon) error {
	env := strings.Join(d.Env, "\n")
	if d.ID == 0 {
		d.CreatedAt = time.Now()
		res, err := s.db.ExecContext(ctx,
			`INSERT INTO daemons (name, site_id, command, dir, env, enabled, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			d.Name, d.SiteID, d.Command, d.Dir, env, boolInt(d.Enabled), d.CreatedAt.Unix())
		if err != nil {
			return err
		}
		d.ID, err = res.LastInsertId()
		return err
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE daemons SET name = ?, site_id = ?, command = ?, dir = ?, env = ?, enabled = ? WHERE id = ?`,
		d.Name, d.SiteID, d.Command, d.Dir, env, boolInt(d.Enabled), d.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteDaemon removes a background program.
func (s *Store) DeleteDaemon(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM daemons WHERE id = ?`, id)
	return err
}
