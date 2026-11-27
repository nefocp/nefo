package store

import (
	"context"
	"time"
)

// UserSummary is an account without its secrets, for the security check.
type UserSummary struct {
	Username    string
	TOTPEnabled bool
	CreatedAt   time.Time
}

// ListUsers returns every account, by name, without password hashes or
// TOTP secrets.
func (s *Store) ListUsers(ctx context.Context) ([]UserSummary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT username, totp_enabled, created_at FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UserSummary{}
	for rows.Next() {
		var u UserSummary
		var created int64
		if err := rows.Scan(&u.Username, &u.TOTPEnabled, &created); err != nil {
			return nil, err
		}
		u.CreatedAt = time.Unix(created, 0)
		out = append(out, u)
	}
	return out, rows.Err()
}
