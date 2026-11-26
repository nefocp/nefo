package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// The access an API key gives.
const (
	APIKeyRead = "read" // GET and HEAD requests only
	APIKeyFull = "full" // every request a key may make
)

// ErrTooManyKeys is returned when there are already as many API keys as allowed.
var ErrTooManyKeys = errors.New("too many API keys")

// APIKey lets a script use the panel's API as a user, without a browser
// session. Only a SHA-256 hash of the key is stored; Prefix is its first
// characters, so the user can tell keys apart. Addresses are the client
// addresses (IPs or CIDR networks) it may be used from; none means any.
// ExpiresAt and LastUsedAt are zero for never.
type APIKey struct {
	ID         int64
	UserID     int64
	Name       string
	Prefix     string
	Hash       string
	Access     string
	Addresses  []string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	LastUsedAt time.Time
	LastUsedIP string
}

// ReadOnly reports whether the key may only read.
func (k *APIKey) ReadOnly() bool { return k.Access != APIKeyFull }

const apiKeyCols = `id, user_id, name, prefix, hash, access, addresses, created_at, expires_at, last_used_at, last_used_ip`

func scanAPIKey(row scanner) (*APIKey, error) {
	var k APIKey
	var addrs string
	var created, expires, used int64
	err := row.Scan(&k.ID, &k.UserID, &k.Name, &k.Prefix, &k.Hash, &k.Access, &addrs, &created, &expires, &used, &k.LastUsedIP)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	k.Addresses = strings.Fields(addrs)
	if k.Addresses == nil {
		k.Addresses = []string{}
	}
	k.CreatedAt, k.ExpiresAt, k.LastUsedAt = time.Unix(created, 0), unixOrZero(expires), unixOrZero(used)
	return &k, nil
}

// CreateAPIKey stores a new key and sets its ID. It returns ErrConflict if
// the name is taken, and ErrTooManyKeys if there are already max keys.
func (s *Store) CreateAPIKey(ctx context.Context, k *APIKey, max int) error {
	if k.CreatedAt.IsZero() {
		k.CreatedAt = time.Now()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_keys`).Scan(&n); err != nil {
		return err
	}
	if n >= max {
		return ErrTooManyKeys
	}
	res, err := tx.ExecContext(ctx,
		`INSERT INTO api_keys (user_id, name, prefix, hash, access, addresses, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		k.UserID, k.Name, k.Prefix, k.Hash, k.Access, strings.Join(k.Addresses, " "),
		k.CreatedAt.Unix(), zeroOrUnix(k.ExpiresAt))
	if isUniqueViolation(err) {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	if k.ID, err = res.LastInsertId(); err != nil {
		return err
	}
	return tx.Commit()
}

// APIKeys returns every key, oldest first.
func (s *Store) APIKeys(ctx context.Context) ([]*APIKey, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+apiKeyCols+` FROM api_keys ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*APIKey{}
	for rows.Next() {
		k, err := scanAPIKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// APIKeyByID returns one key.
func (s *Store) APIKeyByID(ctx context.Context, id int64) (*APIKey, error) {
	return scanAPIKey(s.db.QueryRowContext(ctx, `SELECT `+apiKeyCols+` FROM api_keys WHERE id = ?`, id))
}

// DeleteAPIKey revokes a key.
func (s *Store) DeleteAPIKey(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM api_keys WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// TouchAPIKey records that a key was used at a time from an address, unless
// it was already recorded less than every ago, so a busy script does not
// write to the database on each request. It reports whether it wrote.
func (s *Store) TouchAPIKey(ctx context.Context, id int64, at time.Time, ip string, every time.Duration) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE api_keys SET last_used_at = ?, last_used_ip = ? WHERE id = ? AND last_used_at <= ?`,
		at.Unix(), ip, id, at.Add(-every).Unix())
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
