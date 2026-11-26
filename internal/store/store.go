// Package store keeps the panel's state in a local SQLite database.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver: no CGO, easy cross-compiling to ARM
)

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = errors.New("not found")

// Store is the panel database.
type Store struct{ db *sql.DB }

// User is a panel administrator account.
type User struct {
	ID           int64
	Username     string
	PasswordHash string
	TOTPSecret   string
	TOTPEnabled  bool
	CreatedAt    time.Time
}

// Session is a signed-in browser. Only a hash of the token is stored, so a
// leaked database does not give away live sessions.
type Session struct {
	TokenHash string
	UserID    int64
	CreatedAt time.Time
	ExpiresAt time.Time
	IP        string
	UserAgent string
}

// AuditEntry records a security-relevant action.
type AuditEntry struct {
	ID       int64     `json:"id"`
	Time     time.Time `json:"time"`
	Username string    `json:"username"`
	Action   string    `json:"action"`
	IP       string    `json:"ip"`
	Detail   string    `json:"detail"`
}

// migrations run in order; the database remembers how many have run in
// PRAGMA user_version. Never edit a migration once it has been released:
// add a new one instead.
var migrations = []string{
	// 1: users, sessions and the audit log
	`CREATE TABLE users (
		id            INTEGER PRIMARY KEY,
		username      TEXT    NOT NULL UNIQUE,
		password_hash TEXT    NOT NULL,
		totp_secret   TEXT    NOT NULL DEFAULT '',
		totp_enabled  INTEGER NOT NULL DEFAULT 0,
		created_at    INTEGER NOT NULL
	);
	CREATE TABLE sessions (
		token_hash TEXT    PRIMARY KEY,
		user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		created_at INTEGER NOT NULL,
		expires_at INTEGER NOT NULL,
		ip         TEXT    NOT NULL,
		user_agent TEXT    NOT NULL
	);
	CREATE INDEX sessions_user_id ON sessions(user_id);
	CREATE TABLE audit_log (
		id       INTEGER PRIMARY KEY,
		ts       INTEGER NOT NULL,
		username TEXT    NOT NULL,
		action   TEXT    NOT NULL,
		ip       TEXT    NOT NULL,
		detail   TEXT    NOT NULL DEFAULT ''
	);
	CREATE INDEX audit_log_ts ON audit_log(ts);`,

	// 2: websites
	`CREATE TABLE sites (
		id              INTEGER PRIMARY KEY,
		domain          TEXT    NOT NULL UNIQUE,
		aliases         TEXT    NOT NULL DEFAULT '',
		sys_user        TEXT    NOT NULL UNIQUE,
		php_version     TEXT    NOT NULL DEFAULT '',
		ssl             INTEGER NOT NULL DEFAULT 0,
		force_https     INTEGER NOT NULL DEFAULT 0,
		cert_not_before INTEGER NOT NULL DEFAULT 0,
		cert_not_after  INTEGER NOT NULL DEFAULT 0,
		ssl_error       TEXT    NOT NULL DEFAULT '',
		ssl_attempt_at  INTEGER NOT NULL DEFAULT 0,
		created_at      INTEGER NOT NULL
	);`,

	// 3: MariaDB databases, and the application installed on a site
	`CREATE TABLE databases (
		id         INTEGER PRIMARY KEY,
		name       TEXT    NOT NULL UNIQUE,
		username   TEXT    NOT NULL UNIQUE,
		password   TEXT    NOT NULL,
		site_id    INTEGER REFERENCES sites(id) ON DELETE SET NULL,
		created_at INTEGER NOT NULL
	);
	ALTER TABLE sites ADD COLUMN app TEXT NOT NULL DEFAULT '';`,

	// 4: backups, backup schedules and settings
	`CREATE TABLE backups (
		id         INTEGER PRIMARY KEY,
		kind       TEXT    NOT NULL,
		target     TEXT    NOT NULL,
		file       TEXT    NOT NULL,
		size       INTEGER NOT NULL,
		sha256     TEXT    NOT NULL,
		remote_key TEXT    NOT NULL DEFAULT '',
		schedule_id INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL
	);
	CREATE INDEX backups_target ON backups(kind, target);
	CREATE TABLE backup_schedules (
		id          INTEGER PRIMARY KEY,
		kind        TEXT    NOT NULL,
		target      TEXT    NOT NULL,
		frequency   TEXT    NOT NULL,
		hour        INTEGER NOT NULL,
		weekday     INTEGER NOT NULL DEFAULT 0,
		keep        INTEGER NOT NULL,
		remote      INTEGER NOT NULL DEFAULT 0,
		last_run_at INTEGER NOT NULL DEFAULT 0,
		last_error  TEXT    NOT NULL DEFAULT '',
		created_at  INTEGER NOT NULL
	);
	CREATE TABLE settings (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);`,

	// 5: per-site PHP limits
	`ALTER TABLE sites ADD COLUMN php_memory INTEGER NOT NULL DEFAULT 256;
	ALTER TABLE sites ADD COLUMN php_upload INTEGER NOT NULL DEFAULT 64;
	ALTER TABLE sites ADD COLUMN php_time   INTEGER NOT NULL DEFAULT 300;`,

	// 6: cron jobs
	`CREATE TABLE cron_jobs (
		id         INTEGER PRIMARY KEY,
		name       TEXT    NOT NULL,
		schedule   TEXT    NOT NULL,
		command    TEXT    NOT NULL,
		run_as     TEXT    NOT NULL,
		enabled    INTEGER NOT NULL DEFAULT 1,
		created_at INTEGER NOT NULL
	);`,

	// 7: monitoring history, and app sites behind a reverse proxy
	`CREATE TABLE metrics (
		ts     INTEGER PRIMARY KEY,
		cpu    REAL    NOT NULL,
		mem    REAL    NOT NULL,
		disk   REAL    NOT NULL,
		load1  REAL    NOT NULL
	);
	ALTER TABLE sites ADD COLUMN proxy_port  INTEGER NOT NULL DEFAULT 0;
	ALTER TABLE sites ADD COLUMN app_command TEXT    NOT NULL DEFAULT '';`,

	// 8: site access: suspend, redirect, password protection
	`ALTER TABLE sites ADD COLUMN disabled     INTEGER NOT NULL DEFAULT 0;
	ALTER TABLE sites ADD COLUMN redirect_url TEXT    NOT NULL DEFAULT '';
	ALTER TABLE sites ADD COLUMN auth_user    TEXT    NOT NULL DEFAULT '';
	ALTER TABLE sites ADD COLUMN auth_hash    TEXT    NOT NULL DEFAULT '';`,

	// 9: SFTP account for the site's Linux user
	`ALTER TABLE sites ADD COLUMN sftp INTEGER NOT NULL DEFAULT 0;`,

	// 10: rewrite preset, own Nginx directives, own certificate, DNS validation
	`ALTER TABLE sites ADD COLUMN rewrite      TEXT    NOT NULL DEFAULT '';
	ALTER TABLE sites ADD COLUMN nginx_custom TEXT    NOT NULL DEFAULT '';
	ALTER TABLE sites ADD COLUMN ssl_source   TEXT    NOT NULL DEFAULT '';
	ALTER TABLE sites ADD COLUMN ssl_dns      INTEGER NOT NULL DEFAULT 0;`,

	// 11: website firewall
	`ALTER TABLE sites ADD COLUMN waf         TEXT    NOT NULL DEFAULT '';
	ALTER TABLE sites ADD COLUMN rate_limit  INTEGER NOT NULL DEFAULT 0;
	ALTER TABLE sites ADD COLUMN blocked_ips TEXT    NOT NULL DEFAULT '';`,

	// 12: malware scans and quarantine
	`CREATE TABLE malware_scans (
		site_id     INTEGER PRIMARY KEY,
		finished_at INTEGER NOT NULL,
		files       INTEGER NOT NULL,
		skipped     INTEGER NOT NULL,
		findings    TEXT    NOT NULL,
		clamav      TEXT    NOT NULL DEFAULT ''
	);
	CREATE TABLE quarantine (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		site_id    INTEGER NOT NULL,
		path       TEXT    NOT NULL,
		rule       TEXT    NOT NULL,
		size       INTEGER NOT NULL,
		mode       INTEGER NOT NULL,
		uid        INTEGER NOT NULL,
		gid        INTEGER NOT NULL,
		created_at INTEGER NOT NULL
	);
	CREATE INDEX quarantine_site ON quarantine(site_id);`,

	// 13: PHP workers, pool mode and functions allowed back per site
	`ALTER TABLE sites ADD COLUMN php_workers INTEGER NOT NULL DEFAULT 5;
	ALTER TABLE sites ADD COLUMN php_pm      TEXT    NOT NULL DEFAULT 'ondemand';
	ALTER TABLE sites ADD COLUMN php_allow   TEXT    NOT NULL DEFAULT '';`,

	// 14: custom 404 page, hotlink protection, expiry date
	`ALTER TABLE sites ADD COLUMN custom_404    INTEGER NOT NULL DEFAULT 0;
	ALTER TABLE sites ADD COLUMN hotlink       INTEGER NOT NULL DEFAULT 0;
	ALTER TABLE sites ADD COLUMN hotlink_allow TEXT    NOT NULL DEFAULT '';
	ALTER TABLE sites ADD COLUMN expires_at    INTEGER NOT NULL DEFAULT 0;`,

	// 15: runtime of app sites (default node, node:<major>, python)
	`ALTER TABLE sites ADD COLUMN app_runtime TEXT NOT NULL DEFAULT '';`,
	// 16: background programs, kept running by systemd as a site's user
	`CREATE TABLE daemons (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		name       TEXT    NOT NULL,
		site_id    INTEGER NOT NULL,
		command    TEXT    NOT NULL,
		dir        TEXT    NOT NULL DEFAULT '',
		env        TEXT    NOT NULL DEFAULT '',
		enabled    INTEGER NOT NULL DEFAULT 1,
		created_at INTEGER NOT NULL
	);
	CREATE INDEX daemons_site ON daemons(site_id);`,
	// 17: deploy from Git
	`CREATE TABLE git_deploys (
		site_id        INTEGER PRIMARY KEY REFERENCES sites(id) ON DELETE CASCADE,
		repo           TEXT    NOT NULL,
		branch         TEXT    NOT NULL,
		target         TEXT    NOT NULL,
		script         TEXT    NOT NULL DEFAULT '',
		hook_secret    TEXT    NOT NULL,
		last_commit    TEXT    NOT NULL DEFAULT '',
		last_deploy_at INTEGER NOT NULL DEFAULT 0,
		last_status    TEXT    NOT NULL DEFAULT ''
	);`,
	// 18: app store instances (Docker and service apps) and their secrets
	`CREATE TABLE apps (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		name        TEXT    NOT NULL UNIQUE,
		app_id      TEXT    NOT NULL,
		kind        TEXT    NOT NULL,
		site_id     INTEGER REFERENCES sites(id) ON DELETE CASCADE,
		version     TEXT    NOT NULL,
		images      TEXT    NOT NULL DEFAULT '',
		port        INTEGER NOT NULL DEFAULT 0,
		extra_ports TEXT    NOT NULL DEFAULT '',
		inputs      TEXT    NOT NULL DEFAULT '{}',
		admin_email TEXT    NOT NULL DEFAULT '',
		state       TEXT    NOT NULL DEFAULT 'installing',
		created_at  INTEGER NOT NULL,
		updated_at  INTEGER NOT NULL
	);
	CREATE TABLE app_secrets (
		app_id INTEGER NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
		name   TEXT    NOT NULL,
		value  TEXT    NOT NULL,
		PRIMARY KEY (app_id, name)
	);`,
	// 19: a note per website, and network traffic (bytes per second) in the
	// monitoring history; NULL for samples taken before it was recorded
	`ALTER TABLE sites   ADD COLUMN note   TEXT NOT NULL DEFAULT '';
	ALTER TABLE metrics ADD COLUMN net_rx REAL;
	ALTER TABLE metrics ADD COLUMN net_tx REAL;`,
	// 20: the app store app of an app backup, so its name and logo stay
	// known after the app is uninstalled; filled for the apps still installed
	`ALTER TABLE backups ADD COLUMN app_id TEXT NOT NULL DEFAULT '';
	UPDATE backups SET app_id = COALESCE((SELECT app_id FROM apps WHERE apps.name = backups.target), '')
		WHERE kind = 'app';`,
}

// Open opens (and creates or upgrades) the database in dataDir.
func Open(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dataDir, "nefo.db")
	dsn := "file:" + filepath.ToSlash(path) +
		"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite has a single writer. One connection keeps the panel free of
	// "database is locked" errors; the CLI waits via busy_timeout.
	db.SetMaxOpenConns(1)

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	// The file holds password hashes and 2FA secrets.
	_ = os.Chmod(path, 0o600)
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	var v int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if v > len(migrations) {
		return fmt.Errorf("database schema version %d is newer than this panel supports (%d): upgrade the panel", v, len(migrations))
	}
	for i := v; i < len(migrations); i++ {
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, i+1)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ---- users ----

const userCols = `id, username, password_hash, totp_secret, totp_enabled, created_at`

func scanUser(row *sql.Row) (*User, error) {
	var u User
	var created int64
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.TOTPSecret, &u.TOTPEnabled, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	u.CreatedAt = time.Unix(created, 0)
	return &u, nil
}

// CountUsers returns the number of accounts.
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// CreateUser adds an account and returns its ID.
func (s *Store) CreateUser(ctx context.Context, username, passwordHash string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users (username, password_hash, created_at) VALUES (?, ?, ?)`,
		username, passwordHash, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UserByName looks up an account by username.
func (s *Store) UserByName(ctx context.Context, username string) (*User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE username = ?`, username))
}

// UserByID looks up an account by ID.
func (s *Store) UserByID(ctx context.Context, id int64) (*User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = ?`, id))
}

// SetPassword replaces a user's password hash and signs out all of their sessions.
func (s *Store) SetPassword(ctx context.Context, userID int64, passwordHash string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

// SetTOTP stores a user's 2FA secret and whether 2FA is switched on.
func (s *Store) SetTOTP(ctx context.Context, userID int64, secret string, enabled bool) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET totp_secret = ?, totp_enabled = ? WHERE id = ?`,
		secret, boolInt(enabled), userID)
	return err
}

// ---- sessions ----

// CreateSession stores a new session.
func (s *Store) CreateSession(ctx context.Context, sess *Session) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, user_id, created_at, expires_at, ip, user_agent) VALUES (?, ?, ?, ?, ?, ?)`,
		sess.TokenHash, sess.UserID, sess.CreatedAt.Unix(), sess.ExpiresAt.Unix(), sess.IP, sess.UserAgent)
	return err
}

// ValidSession returns the session for tokenHash if it exists and has not expired at now.
func (s *Store) ValidSession(ctx context.Context, tokenHash string, now time.Time) (*Session, error) {
	var sess Session
	var created, expires int64
	err := s.db.QueryRowContext(ctx,
		`SELECT token_hash, user_id, created_at, expires_at, ip, user_agent
		 FROM sessions WHERE token_hash = ? AND expires_at > ?`,
		tokenHash, now.Unix(),
	).Scan(&sess.TokenHash, &sess.UserID, &created, &expires, &sess.IP, &sess.UserAgent)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	sess.CreatedAt, sess.ExpiresAt = time.Unix(created, 0), time.Unix(expires, 0)
	return &sess, nil
}

// DeleteSession signs out one session.
func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

// PurgeExpiredSessions removes sessions that expired before now.
func (s *Store) PurgeExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, now.Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ---- audit log ----

// Audit appends an entry to the audit log.
func (s *Store) Audit(ctx context.Context, username, action, ip, detail string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO audit_log (ts, username, action, ip, detail) VALUES (?, ?, ?, ?, ?)`,
		time.Now().Unix(), username, action, ip, detail)
	return err
}

// RecentAudit returns the newest audit entries first.
func (s *Store) RecentAudit(ctx context.Context, limit int) ([]AuditEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, ts, username, action, ip, detail FROM audit_log ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []AuditEntry{}
	for rows.Next() {
		var e AuditEntry
		var ts int64
		if err := rows.Scan(&e.ID, &ts, &e.Username, &e.Action, &e.IP, &e.Detail); err != nil {
			return nil, err
		}
		e.Time = time.Unix(ts, 0)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// PurgeAuditBefore removes audit entries older than t.
func (s *Store) PurgeAuditBefore(ctx context.Context, t time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM audit_log WHERE ts < ?`, t.Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
