package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// SiteProtect holds a website's country blocking and file lock settings.
//
// GeoMode is "" (off), "block" (the countries in GeoCountries are refused)
// or "allow" (only they get in); GeoCountries are ISO 3166 codes,
// space-separated.
//
// LockOn makes the site's code read-only for its PHP and app (see
// hosting/filelock.go), except the folders in LockWritable (one per line,
// relative to the site's folder). LockAlert asks for a daily check of the
// PHP files against the copy taken when the files were locked (LockedAt);
// ScanAt is the last check and ScanChanges how many PHP files differed.
type SiteProtect struct {
	SiteID       int64
	GeoMode      string
	GeoCountries []string
	LockOn       bool
	LockWritable []string
	LockAlert    bool
	LockedAt     time.Time
	ScanAt       time.Time
	ScanChanges  int
}

// GeoOn reports whether the site blocks or allows countries.
func (p *SiteProtect) GeoOn() bool { return p.GeoMode != "" && len(p.GeoCountries) > 0 }

const protectCols = `site_id, geo_mode, geo_countries, lock_on, lock_writable, lock_alert, locked_at, scan_at, scan_changes`

func scanProtect(row scanner) (*SiteProtect, error) {
	var p SiteProtect
	var countries, writable string
	var locked, scanned int64
	if err := row.Scan(&p.SiteID, &p.GeoMode, &countries, &p.LockOn, &writable, &p.LockAlert, &locked, &scanned, &p.ScanChanges); err != nil {
		return nil, err
	}
	p.GeoCountries = strings.Fields(countries)
	for _, w := range strings.Split(writable, "\n") {
		if w = strings.TrimSpace(w); w != "" {
			p.LockWritable = append(p.LockWritable, w)
		}
	}
	p.LockedAt, p.ScanAt = unixOrZero(locked), unixOrZero(scanned)
	return &p, nil
}

// SiteProtect returns a site's settings, all off when none were saved.
func (s *Store) SiteProtect(ctx context.Context, siteID int64) (*SiteProtect, error) {
	p, err := scanProtect(s.db.QueryRowContext(ctx, `SELECT `+protectCols+` FROM site_protect WHERE site_id = ?`, siteID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &SiteProtect{SiteID: siteID}, nil
		}
		return nil, err
	}
	return p, nil
}

// SiteProtects returns the settings of every site that has saved some.
func (s *Store) SiteProtects(ctx context.Context) ([]*SiteProtect, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+protectCols+` FROM site_protect ORDER BY site_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*SiteProtect{}
	for rows.Next() {
		p, err := scanProtect(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SetSiteProtect saves a site's settings.
func (s *Store) SetSiteProtect(ctx context.Context, p *SiteProtect) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO site_protect (`+protectCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (site_id) DO UPDATE SET geo_mode = excluded.geo_mode, geo_countries = excluded.geo_countries,
			lock_on = excluded.lock_on, lock_writable = excluded.lock_writable, lock_alert = excluded.lock_alert,
			locked_at = excluded.locked_at, scan_at = excluded.scan_at, scan_changes = excluded.scan_changes`,
		p.SiteID, p.GeoMode, strings.Join(p.GeoCountries, " "), boolInt(p.LockOn), strings.Join(p.LockWritable, "\n"),
		boolInt(p.LockAlert), zeroOrUnix(p.LockedAt), zeroOrUnix(p.ScanAt), p.ScanChanges)
	return err
}
