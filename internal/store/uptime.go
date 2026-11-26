package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// SiteMonitor is how a website is monitored: whether it is, how often (in
// minutes), the path to ask for, a text the page must contain, and whether a
// 4xx answer counts as down. CertNotAfter is the expiry of the certificate
// the website served at the last check over HTTPS; CertWarnedAt is when an
// alert last said it ends soon.
type SiteMonitor struct {
	SiteID       int64
	Enabled      bool
	Interval     int
	Path         string
	Keyword      string
	Strict4xx    bool
	CertNotAfter time.Time
	CertWarnedAt time.Time
}

// UptimeCheck is the result of one check of a website. Code is the HTTP
// status (0 when no answer came), MS the response time in milliseconds and
// Error why the check failed.
type UptimeCheck struct {
	SiteID int64     `json:"-"`
	Time   time.Time `json:"t"`
	OK     bool      `json:"ok"`
	Code   int       `json:"code,omitempty"`
	MS     int       `json:"ms"`
	Error  string    `json:"error,omitempty"`
}

// UptimeStats sums up the checks of a website over a period: how many ran,
// how many found it up, and the average response time of those.
type UptimeStats struct {
	Checks int     `json:"checks"`
	Up     int     `json:"up"`
	AvgMS  float64 `json:"avg_ms"`
}

// SiteMonitor returns the monitoring settings of a website, with the
// defaults (off, every 5 minutes, "/") when they were never saved.
func (s *Store) SiteMonitor(ctx context.Context, siteID int64) (*SiteMonitor, error) {
	m, err := scanMonitor(s.db.QueryRowContext(ctx, `SELECT `+monitorCols+` FROM site_monitors WHERE site_id = ?`, siteID))
	if errors.Is(err, ErrNotFound) {
		return &SiteMonitor{SiteID: siteID, Interval: 5, Path: "/"}, nil
	}
	return m, err
}

// SiteMonitors returns the settings of every website that has saved some.
func (s *Store) SiteMonitors(ctx context.Context) ([]*SiteMonitor, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+monitorCols+` FROM site_monitors ORDER BY site_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*SiteMonitor{}
	for rows.Next() {
		m, err := scanMonitor(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

const monitorCols = `site_id, enabled, interval_min, path, keyword, strict_4xx, cert_not_after, cert_warned_at`

func scanMonitor(row scanner) (*SiteMonitor, error) {
	var m SiteMonitor
	var notAfter, warned int64
	err := row.Scan(&m.SiteID, &m.Enabled, &m.Interval, &m.Path, &m.Keyword, &m.Strict4xx, &notAfter, &warned)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	m.CertNotAfter, m.CertWarnedAt = unixOrZero(notAfter), unixOrZero(warned)
	return &m, nil
}

// SaveSiteMonitor stores the settings of a website's monitoring. The
// certificate fields are kept: SetMonitorCert changes them.
func (s *Store) SaveSiteMonitor(ctx context.Context, m *SiteMonitor) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO site_monitors (site_id, enabled, interval_min, path, keyword, strict_4xx) VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT (site_id) DO UPDATE SET enabled = excluded.enabled, interval_min = excluded.interval_min,
			path = excluded.path, keyword = excluded.keyword, strict_4xx = excluded.strict_4xx`,
		m.SiteID, boolInt(m.Enabled), m.Interval, m.Path, m.Keyword, boolInt(m.Strict4xx))
	if err != nil && strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
		return ErrNotFound // the website is gone
	}
	return err
}

// SetMonitorCert records the expiry of the certificate a website served,
// and when an alert last said that it ends soon.
func (s *Store) SetMonitorCert(ctx context.Context, siteID int64, notAfter, warnedAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE site_monitors SET cert_not_after = ?, cert_warned_at = ? WHERE site_id = ?`,
		zeroOrUnix(notAfter), zeroOrUnix(warnedAt), siteID)
	return err
}

// AddUptimeCheck stores the result of a check. A second check in the same
// second replaces the first.
func (s *Store) AddUptimeCheck(ctx context.Context, c UptimeCheck) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO uptime_checks (site_id, ts, ok, code, ms, error) VALUES (?, ?, ?, ?, ?, ?)`,
		c.SiteID, c.Time.Unix(), boolInt(c.OK), c.Code, c.MS, c.Error)
	return err
}

// UptimeChecks returns the newest checks of a website since t, newest
// first, at most limit of them. failedOnly keeps the failed checks.
func (s *Store) UptimeChecks(ctx context.Context, siteID int64, since time.Time, limit int, failedOnly bool) ([]UptimeCheck, error) {
	q := `SELECT ts, ok, code, ms, error FROM uptime_checks WHERE site_id = ? AND ts >= ?`
	if failedOnly {
		q += ` AND ok = 0`
	}
	rows, err := s.db.QueryContext(ctx, q+` ORDER BY ts DESC LIMIT ?`, siteID, since.Unix(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UptimeCheck{}
	for rows.Next() {
		c := UptimeCheck{SiteID: siteID}
		var ts int64
		if err := rows.Scan(&ts, &c.OK, &c.Code, &c.MS, &c.Error); err != nil {
			return nil, err
		}
		c.Time = time.Unix(ts, 0)
		out = append(out, c)
	}
	return out, rows.Err()
}

// UptimeStreak returns how many of a website's newest checks failed in a
// row, and when the first of them ran.
func (s *Store) UptimeStreak(ctx context.Context, siteID int64) (int, time.Time, error) {
	var n int
	var first sql.NullInt64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), MIN(ts) FROM uptime_checks WHERE site_id = ? AND ts >
			COALESCE((SELECT MAX(ts) FROM uptime_checks WHERE site_id = ? AND ok = 1), -1)`,
		siteID, siteID).Scan(&n, &first)
	if err != nil || !first.Valid {
		return 0, time.Time{}, err
	}
	return n, time.Unix(first.Int64, 0), nil
}

// UptimeStats sums up the checks of a website since t.
func (s *Store) UptimeStats(ctx context.Context, siteID int64, since time.Time) (UptimeStats, error) {
	var st UptimeStats
	var up sql.NullInt64
	var avg sql.NullFloat64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), SUM(ok), AVG(CASE WHEN ok = 1 THEN ms END) FROM uptime_checks WHERE site_id = ? AND ts >= ?`,
		siteID, since.Unix()).Scan(&st.Checks, &up, &avg)
	st.Up, st.AvgMS = int(up.Int64), avg.Float64
	return st, err
}

// UptimePoint is the average response time of the successful checks in a
// time bucket.
type UptimePoint struct {
	Time time.Time `json:"t"`
	MS   float64   `json:"v"`
}

// UptimeSeries returns the response times of a website since t, averaged
// into buckets of step (at least a minute), oldest first.
func (s *Store) UptimeSeries(ctx context.Context, siteID int64, since time.Time, step time.Duration) ([]UptimePoint, error) {
	sec := max(int64(step/time.Second), 60)
	rows, err := s.db.QueryContext(ctx,
		`SELECT (ts / ?) * ?, AVG(ms) FROM uptime_checks WHERE site_id = ? AND ts >= ? AND ok = 1
		 GROUP BY ts / ? ORDER BY 1`, sec, sec, siteID, since.Unix(), sec)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UptimePoint{}
	for rows.Next() {
		var p UptimePoint
		var ts int64
		if err := rows.Scan(&ts, &p.MS); err != nil {
			return nil, err
		}
		p.Time = time.Unix(ts, 0)
		out = append(out, p)
	}
	return out, rows.Err()
}

// PurgeUptimeBefore removes the checks older than t.
func (s *Store) PurgeUptimeBefore(ctx context.Context, t time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM uptime_checks WHERE ts < ?`, t.Unix())
	return err
}
