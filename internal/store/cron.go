package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// CronJob is a command run on a schedule by the system's cron.
type CronJob struct {
	ID        int64
	Name      string
	Schedule  string // five cron fields, or @daily etc.
	Command   string
	RunAs     string // "root" or a site's Linux user
	Enabled   bool
	CreatedAt time.Time
}

const cronCols = `id, name, schedule, command, run_as, enabled, created_at`

func scanCron(row scanner) (*CronJob, error) {
	var c CronJob
	var created int64
	err := row.Scan(&c.ID, &c.Name, &c.Schedule, &c.Command, &c.RunAs, &c.Enabled, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	c.CreatedAt = time.Unix(created, 0)
	return &c, nil
}

// ListCronJobs returns all cron jobs.
func (s *Store) ListCronJobs(ctx context.Context) ([]*CronJob, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+cronCols+` FROM cron_jobs ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*CronJob{}
	for rows.Next() {
		c, err := scanCron(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CronJobByID returns one cron job.
func (s *Store) CronJobByID(ctx context.Context, id int64) (*CronJob, error) {
	return scanCron(s.db.QueryRowContext(ctx, `SELECT `+cronCols+` FROM cron_jobs WHERE id = ?`, id))
}

// SaveCronJob inserts (ID 0) or updates a cron job.
func (s *Store) SaveCronJob(ctx context.Context, c *CronJob) error {
	if c.ID == 0 {
		c.CreatedAt = time.Now()
		res, err := s.db.ExecContext(ctx,
			`INSERT INTO cron_jobs (name, schedule, command, run_as, enabled, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			c.Name, c.Schedule, c.Command, c.RunAs, boolInt(c.Enabled), c.CreatedAt.Unix())
		if err != nil {
			return err
		}
		c.ID, err = res.LastInsertId()
		return err
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE cron_jobs SET name = ?, schedule = ?, command = ?, run_as = ?, enabled = ? WHERE id = ?`,
		c.Name, c.Schedule, c.Command, c.RunAs, boolInt(c.Enabled), c.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteCronJob removes a cron job.
func (s *Store) DeleteCronJob(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM cron_jobs WHERE id = ?`, id)
	return err
}
