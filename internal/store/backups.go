package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Backup is a backup archive on this server, optionally copied to remote storage.
type Backup struct {
	ID         int64
	Kind       string // "site" or "database"
	Target     string // domain or database name
	File       string // absolute path
	Size       int64
	SHA256     string
	RemoteKey  string // object key in remote storage, or ""
	ScheduleID int64  // 0 for a manual backup
	// AppID is the app store app of an app backup, so the list can show
	// its name and logo after the app is uninstalled ("" for other kinds).
	AppID     string
	CreatedAt time.Time
}

// Schedule runs backups automatically.
type Schedule struct {
	ID        int64
	Kind      string
	Target    string
	Frequency string // "daily" or "weekly"
	Hour      int    // 0-23, server time
	Weekday   int    // 0 = Sunday, for weekly schedules
	Keep      int    // number of backups to keep
	Remote    bool   // also upload to remote storage
	LastRunAt time.Time
	LastError string
	CreatedAt time.Time
}

const backupCols = `id, kind, target, file, size, sha256, remote_key, schedule_id, app_id, created_at`

func scanBackup(row scanner) (*Backup, error) {
	var b Backup
	var created int64
	err := row.Scan(&b.ID, &b.Kind, &b.Target, &b.File, &b.Size, &b.SHA256, &b.RemoteKey, &b.ScheduleID, &b.AppID, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	b.CreatedAt = time.Unix(created, 0)
	return &b, nil
}

// CreateBackup records a backup and sets its ID.
func (s *Store) CreateBackup(ctx context.Context, b *Backup) error {
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now()
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO backups (kind, target, file, size, sha256, remote_key, schedule_id, app_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		b.Kind, b.Target, b.File, b.Size, b.SHA256, b.RemoteKey, b.ScheduleID, b.AppID, b.CreatedAt.Unix())
	if err != nil {
		return err
	}
	b.ID, err = res.LastInsertId()
	return err
}

// SetBackupChecksum records the SHA-256 of a backup's archive, for a
// backup found on a storage, whose checksum was not known.
func (s *Store) SetBackupChecksum(ctx context.Context, id int64, sum string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE backups SET sha256 = ? WHERE id = ?`, sum, id)
	return err
}

// SetBackupRemote records where a backup was uploaded.
func (s *Store) SetBackupRemote(ctx context.Context, id int64, key string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE backups SET remote_key = ? WHERE id = ?`, key, id)
	return err
}

// ListBackups returns all backups, newest first.
func (s *Store) ListBackups(ctx context.Context) ([]*Backup, error) {
	return s.queryBackups(ctx, `SELECT `+backupCols+` FROM backups ORDER BY created_at DESC, id DESC`)
}

// BackupsFor returns the backups of one target made by a schedule, newest first.
func (s *Store) BackupsFor(ctx context.Context, kind, target string, scheduleID int64) ([]*Backup, error) {
	return s.queryBackups(ctx, `SELECT `+backupCols+` FROM backups WHERE kind = ? AND target = ? AND schedule_id = ?
		ORDER BY created_at DESC, id DESC`, kind, target, scheduleID)
}

func (s *Store) queryBackups(ctx context.Context, q string, args ...any) ([]*Backup, error) {
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Backup{}
	for rows.Next() {
		b, err := scanBackup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// BackupByID returns one backup.
func (s *Store) BackupByID(ctx context.Context, id int64) (*Backup, error) {
	return scanBackup(s.db.QueryRowContext(ctx, `SELECT `+backupCols+` FROM backups WHERE id = ?`, id))
}

// DeleteBackup removes a backup record.
func (s *Store) DeleteBackup(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM backups WHERE id = ?`, id)
	return err
}

const scheduleCols = `id, kind, target, frequency, hour, weekday, keep, remote, last_run_at, last_error, created_at`

func scanSchedule(row scanner) (*Schedule, error) {
	var sc Schedule
	var last, created int64
	err := row.Scan(&sc.ID, &sc.Kind, &sc.Target, &sc.Frequency, &sc.Hour, &sc.Weekday, &sc.Keep, &sc.Remote,
		&last, &sc.LastError, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	sc.LastRunAt, sc.CreatedAt = unixOrZero(last), time.Unix(created, 0)
	return &sc, nil
}

// CreateSchedule stores a backup schedule.
func (s *Store) CreateSchedule(ctx context.Context, sc *Schedule) error {
	if sc.CreatedAt.IsZero() {
		sc.CreatedAt = time.Now()
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO backup_schedules (kind, target, frequency, hour, weekday, keep, remote, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sc.Kind, sc.Target, sc.Frequency, sc.Hour, sc.Weekday, sc.Keep, boolInt(sc.Remote), sc.CreatedAt.Unix())
	if err != nil {
		return err
	}
	sc.ID, err = res.LastInsertId()
	return err
}

// ListSchedules returns all schedules.
func (s *Store) ListSchedules(ctx context.Context) ([]*Schedule, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+scheduleCols+` FROM backup_schedules ORDER BY target, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Schedule{}
	for rows.Next() {
		sc, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

// ScheduleRan records the result of a scheduled run.
func (s *Store) ScheduleRan(ctx context.Context, id int64, at time.Time, errText string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE backup_schedules SET last_run_at = ?, last_error = ? WHERE id = ?`,
		at.Unix(), errText, id)
	return err
}

// DeleteSchedule removes a schedule. Its backups stay.
func (s *Store) DeleteSchedule(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM backup_schedules WHERE id = ?`, id)
	return err
}

// DeleteSchedulesFor removes the schedules of a deleted site, database or app.
func (s *Store) DeleteSchedulesFor(ctx context.Context, kind, target string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM backup_schedules WHERE kind = ? AND target = ?`, kind, target)
	return err
}

// Setting returns a stored setting, or "" when it is not set.
func (s *Store) Setting(ctx context.Context, key string) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

// SetSetting stores a setting; an empty value removes it.
func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	if value == "" {
		_, err := s.db.ExecContext(ctx, `DELETE FROM settings WHERE key = ?`, key)
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}
