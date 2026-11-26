package store

import (
	"context"
	"database/sql"
	"time"
)

// Metric is one sample of the server's load, in percent.
type Metric struct {
	Time  time.Time `json:"t"`
	CPU   float64   `json:"cpu"`
	Mem   float64   `json:"mem"`
	Disk  float64   `json:"disk"`
	Load1 float64   `json:"load1"`
	// NetRx and NetTx are the bytes per second received and sent by the
	// server's internet-facing interfaces, averaged over the sample's minute.
	// They are nil when unknown (older samples, or no /proc/net/dev).
	NetRx *float64 `json:"rx,omitempty"`
	NetTx *float64 `json:"tx,omitempty"`
}

// AddMetric stores a sample.
func (s *Store) AddMetric(ctx context.Context, m Metric) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO metrics (ts, cpu, mem, disk, load1, net_rx, net_tx) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		m.Time.Unix(), m.CPU, m.Mem, m.Disk, m.Load1, m.NetRx, m.NetTx)
	return err
}

// Metrics returns the samples since t, averaged into buckets of step so a
// chart gets a manageable number of points.
func (s *Store) Metrics(ctx context.Context, since time.Time, step time.Duration) ([]Metric, error) {
	sec := int64(step / time.Second)
	if sec < 60 {
		sec = 60
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT (ts / ?) * ?, AVG(cpu), AVG(mem), AVG(disk), AVG(load1), AVG(net_rx), AVG(net_tx)
		 FROM metrics WHERE ts >= ? GROUP BY ts / ? ORDER BY 1`, sec, sec, since.Unix(), sec)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Metric{}
	for rows.Next() {
		var m Metric
		var ts int64
		var rx, tx sql.NullFloat64
		if err := rows.Scan(&ts, &m.CPU, &m.Mem, &m.Disk, &m.Load1, &rx, &tx); err != nil {
			return nil, err
		}
		if rx.Valid && tx.Valid {
			m.NetRx, m.NetTx = &rx.Float64, &tx.Float64
		}
		m.Time = time.Unix(ts, 0)
		out = append(out, m)
	}
	return out, rows.Err()
}

// NetTotals returns the bytes received and sent since t. Each sample is the
// average rate over about one minute (the sampling interval), so its bytes
// are the rate times 60.
func (s *Store) NetTotals(ctx context.Context, since time.Time) (rx, tx float64, err error) {
	var r, t sql.NullFloat64
	err = s.db.QueryRowContext(ctx,
		`SELECT SUM(net_rx) * 60, SUM(net_tx) * 60 FROM metrics WHERE ts >= ?`, since.Unix()).Scan(&r, &t)
	return r.Float64, t.Float64, err
}

// PurgeMetricsBefore removes samples older than t.
func (s *Store) PurgeMetricsBefore(ctx context.Context, t time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM metrics WHERE ts < ?`, t.Unix())
	return err
}
