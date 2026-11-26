package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// ErrConflict is returned when a unique value, such as a domain, is already taken.
var ErrConflict = errors.New("already exists")

// Site is a website served by Nginx.
type Site struct {
	ID      int64
	Domain  string
	Aliases []string
	// SysUser is the Linux account that owns the files and runs the site's PHP.
	SysUser string
	// PHPVersion is "" for a static site.
	PHPVersion    string
	SSL           bool
	ForceHTTPS    bool
	CertNotBefore time.Time
	CertNotAfter  time.Time
	// SSLError is the last certificate error, shown until a renewal succeeds.
	SSLError     string
	SSLAttemptAt time.Time
	// App is the application the panel installed, such as "wordpress", or "".
	App string
	// PHP limits: memory_limit and upload size in MB, max_execution_time in seconds.
	PHPMemory int
	PHPUpload int
	PHPTime   int
	// ProxyPort > 0 makes an app site: Nginx forwards requests to
	// 127.0.0.1:ProxyPort. AppCommand, if set, is run as a service.
	ProxyPort  int
	AppCommand string
	// AppRuntime is "" (the server's node), "node:<major>" or "python".
	AppRuntime string
	// Access: a suspended site answers 503; a redirect sends every request
	// to RedirectURL; AuthUser/AuthHash (bcrypt) ask for a password.
	Disabled    bool
	RedirectURL string
	AuthUser    string
	AuthHash    string
	// SFTP lets the site's Linux user sign in with SFTP, locked in the site's folder.
	SFTP bool
	// Rewrite is the rewrite preset ("" for the default rules) and
	// NginxCustom the administrator's own directives for the server block.
	Rewrite     string
	NginxCustom string
	// SSLSource is "" for a certificate from Let's Encrypt (renewed by the
	// panel) or "custom" for one the administrator uploaded. SSLDNS asks for
	// DNS validation (Cloudflare) instead of HTTP.
	SSLSource string
	SSLDNS    bool
	// Firewall: WAF is "", "detect" or "block"; RateLimit is PHP or app
	// requests per second per IP (0 is off); BlockedIPs is space-separated.
	WAF        string
	RateLimit  int
	BlockedIPs string
	// PHP-FPM: workers (pm.max_children), pool mode (ondemand or dynamic)
	// and the normally disabled functions this site may use (space-separated).
	PHPWorkers int
	PHPPM      string
	PHPAllow   string
	// Options: a custom 404 page (public/404.html), hotlink protection for
	// media (with other allowed sites), and an expiry date (zero: never).
	Custom404    bool
	Hotlink      bool
	HotlinkAllow string
	ExpiresAt    time.Time
	// HSTS sends Strict-Transport-Security (with includeSubDomains if
	// HSTSSubdomains) while HTTPS and the redirect to it are on. PageCache
	// turns on Nginx's FastCGI cache for a PHP site.
	HSTS           bool
	HSTSSubdomains bool
	PageCache      bool
	// Note is the administrator's own one-line note, shown under the domain.
	// It changes only through SetSiteNote, never through UpdateSite.
	Note      string
	CreatedAt time.Time
}

// Names returns the domain followed by its aliases.
func (s *Site) Names() []string {
	return append([]string{s.Domain}, s.Aliases...)
}

const siteCols = `id, domain, aliases, sys_user, php_version, ssl, force_https,
	cert_not_before, cert_not_after, ssl_error, ssl_attempt_at, app, php_memory, php_upload, php_time, proxy_port, app_command, disabled, redirect_url, auth_user, auth_hash, sftp, rewrite, nginx_custom, ssl_source, ssl_dns, waf, rate_limit, blocked_ips, php_workers, php_pm, php_allow, custom_404, hotlink, hotlink_allow, expires_at, app_runtime, note, hsts, hsts_subdomains, page_cache, created_at`

type scanner interface{ Scan(dest ...any) error }

func scanSite(row scanner) (*Site, error) {
	var s Site
	var aliases string
	var notBefore, notAfter, attempt, created, expires int64
	err := row.Scan(&s.ID, &s.Domain, &aliases, &s.SysUser, &s.PHPVersion, &s.SSL, &s.ForceHTTPS,
		&notBefore, &notAfter, &s.SSLError, &attempt, &s.App, &s.PHPMemory, &s.PHPUpload, &s.PHPTime, &s.ProxyPort, &s.AppCommand, &s.Disabled, &s.RedirectURL, &s.AuthUser, &s.AuthHash, &s.SFTP, &s.Rewrite, &s.NginxCustom, &s.SSLSource, &s.SSLDNS, &s.WAF, &s.RateLimit, &s.BlockedIPs, &s.PHPWorkers, &s.PHPPM, &s.PHPAllow, &s.Custom404, &s.Hotlink, &s.HotlinkAllow, &expires, &s.AppRuntime, &s.Note, &s.HSTS, &s.HSTSSubdomains, &s.PageCache, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	s.Aliases = strings.Fields(aliases)
	if s.Aliases == nil {
		s.Aliases = []string{}
	}
	s.CertNotBefore, s.CertNotAfter = unixOrZero(notBefore), unixOrZero(notAfter)
	s.SSLAttemptAt, s.CreatedAt = unixOrZero(attempt), time.Unix(created, 0)
	s.ExpiresAt = unixOrZero(expires)
	return &s, nil
}

func unixOrZero(v int64) time.Time {
	if v == 0 {
		return time.Time{}
	}
	return time.Unix(v, 0)
}

func zeroOrUnix(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// CreateSite stores a new site and sets its ID. It returns ErrConflict if the
// domain or the system user is already taken.
func (s *Store) CreateSite(ctx context.Context, site *Site) error {
	if site.CreatedAt.IsZero() {
		site.CreatedAt = time.Now()
	}
	if site.PHPMemory == 0 {
		site.PHPMemory, site.PHPUpload, site.PHPTime = 256, 64, 300
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO sites (domain, aliases, sys_user, php_version, php_memory, php_upload, php_time, proxy_port, app_command, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		site.Domain, strings.Join(site.Aliases, " "), site.SysUser, site.PHPVersion,
		site.PHPMemory, site.PHPUpload, site.PHPTime, site.ProxyPort, site.AppCommand, site.CreatedAt.Unix())
	if isUniqueViolation(err) {
		return ErrConflict
	}
	if err != nil {
		return err
	}
	site.ID, err = res.LastInsertId()
	return err
}

// ListSites returns every site, sorted by domain.
func (s *Store) ListSites(ctx context.Context) ([]*Site, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+siteCols+` FROM sites ORDER BY domain`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sites := []*Site{}
	for rows.Next() {
		site, err := scanSite(rows)
		if err != nil {
			return nil, err
		}
		sites = append(sites, site)
	}
	return sites, rows.Err()
}

// SiteByID returns one site.
func (s *Store) SiteByID(ctx context.Context, id int64) (*Site, error) {
	return scanSite(s.db.QueryRowContext(ctx, `SELECT `+siteCols+` FROM sites WHERE id = ?`, id))
}

// UpdateSite saves the settings and certificate state of a site. The domain
// and system user never change.
func (s *Store) UpdateSite(ctx context.Context, site *Site) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE sites SET aliases = ?, php_version = ?, ssl = ?, force_https = ?,
			cert_not_before = ?, cert_not_after = ?, ssl_error = ?, ssl_attempt_at = ?, app = ?,
			php_memory = ?, php_upload = ?, php_time = ?, proxy_port = ?, app_command = ?,
			disabled = ?, redirect_url = ?, auth_user = ?, auth_hash = ?, sftp = ?,
			rewrite = ?, nginx_custom = ?, ssl_source = ?, ssl_dns = ?,
			waf = ?, rate_limit = ?, blocked_ips = ?, php_workers = ?, php_pm = ?, php_allow = ?,
			custom_404 = ?, hotlink = ?, hotlink_allow = ?, expires_at = ?, app_runtime = ?,
			hsts = ?, hsts_subdomains = ?, page_cache = ?
		 WHERE id = ?`,
		strings.Join(site.Aliases, " "), site.PHPVersion, boolInt(site.SSL), boolInt(site.ForceHTTPS),
		zeroOrUnix(site.CertNotBefore), zeroOrUnix(site.CertNotAfter), site.SSLError, zeroOrUnix(site.SSLAttemptAt), site.App,
		site.PHPMemory, site.PHPUpload, site.PHPTime, site.ProxyPort, site.AppCommand,
		boolInt(site.Disabled), site.RedirectURL, site.AuthUser, site.AuthHash, boolInt(site.SFTP),
		site.Rewrite, site.NginxCustom, site.SSLSource, boolInt(site.SSLDNS),
		site.WAF, site.RateLimit, site.BlockedIPs, site.PHPWorkers, site.PHPPM, site.PHPAllow,
		boolInt(site.Custom404), boolInt(site.Hotlink), site.HotlinkAllow, zeroOrUnix(site.ExpiresAt), site.AppRuntime,
		boolInt(site.HSTS), boolInt(site.HSTSSubdomains), boolInt(site.PageCache), site.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetSiteNote saves a site's note. It is separate from UpdateSite so that a
// settings change made at the same time never overwrites the note (or the
// other way round).
func (s *Store) SetSiteNote(ctx context.Context, id int64, note string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE sites SET note = ? WHERE id = ?`, note, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteSite removes a site.
func (s *Store) DeleteSite(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sites WHERE id = ?`, id)
	return err
}

// SysUserTaken reports whether a site already uses the Linux account name.
func (s *Store) SysUserTaken(ctx context.Context, name string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sites WHERE sys_user = ?`, name).Scan(&n)
	return n > 0, err
}
