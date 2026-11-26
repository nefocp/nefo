package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

// App is an installed Docker or service app from the app store (PHP apps
// are recorded in Site.App).
type App struct {
	ID int64
	// Name is the instance: the website's Linux user, or the service app's
	// ID. Its files are in /var/lib/panel-apps/<Name>.
	Name   string
	AppID  string // catalog ID
	Kind   string // "docker" or "service"
	SiteID int64  // 0 for service apps
	// Version is the catalog version installed; Images the image
	// references in use (space-separated).
	Version string
	Images  string
	// Port is the host port on 127.0.0.1 of the website's service or of the
	// service app; ExtraPorts maps other container ports to host ports.
	Port       int
	ExtraPorts map[int]int
	Inputs     map[string]string
	AdminEmail string
	// State is "installing", "ready" or "failed".
	State     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

const appCols = `id, name, app_id, kind, site_id, version, images, port, extra_ports, inputs, admin_email, state, created_at, updated_at`

func scanApp(row scanner) (*App, error) {
	var a App
	var site sql.NullInt64
	var extra, inputs string
	var created, updated int64
	err := row.Scan(&a.ID, &a.Name, &a.AppID, &a.Kind, &site, &a.Version, &a.Images, &a.Port, &extra, &inputs,
		&a.AdminEmail, &a.State, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	a.SiteID = site.Int64
	a.ExtraPorts = parsePorts(extra)
	a.Inputs = map[string]string{}
	json.Unmarshal([]byte(inputs), &a.Inputs)
	a.CreatedAt, a.UpdatedAt = time.Unix(created, 0), time.Unix(updated, 0)
	return &a, nil
}

// "9000:20002 9001:20003"
func formatPorts(m map[int]int) string {
	var parts []string
	for k, v := range m {
		parts = append(parts, strconv.Itoa(k)+":"+strconv.Itoa(v))
	}
	return strings.Join(parts, " ")
}

func parsePorts(s string) map[int]int {
	m := map[int]int{}
	for _, f := range strings.Fields(s) {
		a, b, ok := strings.Cut(f, ":")
		x, err1 := strconv.Atoi(a)
		y, err2 := strconv.Atoi(b)
		if ok && err1 == nil && err2 == nil {
			m[x] = y
		}
	}
	return m
}

func nullSite(id int64) any {
	if id == 0 {
		return nil
	}
	return id
}

// CreateApp records a new instance; its Name must be unused.
func (s *Store) CreateApp(ctx context.Context, a *App) error {
	now := time.Now()
	inputs, _ := json.Marshal(a.Inputs)
	if a.State == "" {
		a.State = "installing"
	}
	res, err := s.db.ExecContext(ctx, `INSERT INTO apps (name, app_id, kind, site_id, version, images, port, extra_ports,
		inputs, admin_email, state, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.Name, a.AppID, a.Kind, nullSite(a.SiteID), a.Version, a.Images, a.Port, formatPorts(a.ExtraPorts), string(inputs),
		a.AdminEmail, a.State, now.Unix(), now.Unix())
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return ErrConflict
		}
		return err
	}
	a.ID, _ = res.LastInsertId()
	a.CreatedAt, a.UpdatedAt = now, now
	return nil
}

// UpdateApp saves an instance's version, images, ports and state.
func (s *Store) UpdateApp(ctx context.Context, a *App) error {
	inputs, _ := json.Marshal(a.Inputs)
	a.UpdatedAt = time.Now()
	_, err := s.db.ExecContext(ctx, `UPDATE apps SET version = ?, images = ?, port = ?, extra_ports = ?, inputs = ?,
		admin_email = ?, state = ?, updated_at = ? WHERE id = ?`,
		a.Version, a.Images, a.Port, formatPorts(a.ExtraPorts), string(inputs), a.AdminEmail, a.State, a.UpdatedAt.Unix(), a.ID)
	return err
}

// AppByName returns an instance.
func (s *Store) AppByName(ctx context.Context, name string) (*App, error) {
	return scanApp(s.db.QueryRowContext(ctx, `SELECT `+appCols+` FROM apps WHERE name = ?`, name))
}

// AppForSite returns the Docker app of a website, or ErrNotFound.
func (s *Store) AppForSite(ctx context.Context, siteID int64) (*App, error) {
	return scanApp(s.db.QueryRowContext(ctx, `SELECT `+appCols+` FROM apps WHERE site_id = ?`, siteID))
}

// ListApps returns every instance.
func (s *Store) ListApps(ctx context.Context) ([]*App, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+appCols+` FROM apps ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*App{}
	for rows.Next() {
		a, err := scanApp(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// DeleteApp forgets an instance and its secrets.
func (s *Store) DeleteApp(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM apps WHERE id = ?`, id)
	return err
}

// AppSecrets returns an instance's secrets.
func (s *Store) AppSecrets(ctx context.Context, appID int64) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, value FROM app_secrets WHERE app_id = ?`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// SetAppSecrets stores (adds or replaces) an instance's secrets.
func (s *Store) SetAppSecrets(ctx context.Context, appID int64, secrets map[string]string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for k, v := range secrets {
		if _, err := tx.ExecContext(ctx, `INSERT INTO app_secrets (app_id, name, value) VALUES (?, ?, ?)
			ON CONFLICT(app_id, name) DO UPDATE SET value = excluded.value`, appID, k, v); err != nil {
			return err
		}
	}
	return tx.Commit()
}
