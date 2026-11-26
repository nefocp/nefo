package hosting

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/validate"
)

// Per-site HTTPS settings beyond the certificate: turning HTTPS off again,
// HSTS, and Nginx's page cache (FastCGI cache) for PHP sites.

// ---- turning HTTPS off ----

// RemoveCertificate turns HTTPS off for a site: the web server serves it
// over plain HTTP only, the redirect to HTTPS and HSTS are off, and the
// certificate files (from Let's Encrypt or installed) are deleted, so
// nothing renews them. The ACME account stays. A WordPress site goes back
// to its http:// address and a Docker app is restarted with it.
func (m *Manager) RemoveCertificate(ctx context.Context, id int64) (*store.Site, error) {
	if !m.Supported() {
		return nil, ErrUnsupported
	}
	if err := m.tryLock(); err != nil {
		return nil, err
	}
	defer m.ops.Unlock()
	return m.removeCertificate(ctx, id)
}

func (m *Manager) removeCertificate(ctx context.Context, id int64) (*store.Site, error) {
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if validate.Domain(site.Domain) != nil || m.paths.Certs == "" {
		return nil, fmt.Errorf("refusing to change a site with an invalid name: %q", site.Domain)
	}
	old := *site
	site.SSL, site.ForceHTTPS, site.HSTS, site.HSTSSubdomains = false, false, false, false
	site.SSLSource, site.SSLError = "", ""
	site.CertNotBefore, site.CertNotAfter, site.SSLAttemptAt = time.Time{}, time.Time{}, time.Time{}
	if old.SSL {
		if err := m.updateSite(ctx, site, &old); err != nil {
			return nil, err
		}
	} else if err := m.store.UpdateSite(ctx, site); err != nil {
		return nil, err
	}
	// Only now: the web server no longer reads the files.
	if err := os.RemoveAll(m.paths.certDir(site.Domain)); err != nil {
		m.log.Warn("delete certificate", "domain", site.Domain, "err", err)
	}
	if old.SSL {
		m.refreshDockerApp(ctx, site, slogLogger{m})
		if site.App == AppWordPress {
			if err := m.wpSetURL(ctx, site, "http://"+site.Domain); err != nil {
				m.log.Warn("switch WordPress to http", "domain", site.Domain, "err", err)
			}
		}
	}
	return site, nil
}

// ---- HSTS ----

// HSTS tells browsers to use only HTTPS for the site during max-age (180
// days, about 6 months). The panel never adds "preload": that list is
// hard to leave.
const (
	hstsValue           = "max-age=15552000"
	hstsValueSubdomains = "max-age=15552000; includeSubDomains"
)

// siteHSTS is the Strict-Transport-Security value a site sends, or "".
// It needs HTTPS and the redirect to it.
func siteHSTS(site *store.Site) string {
	switch {
	case !site.HSTS || !site.SSL || !site.ForceHTTPS:
		return ""
	case site.HSTSSubdomains:
		return hstsValueSubdomains
	}
	return hstsValue
}

// SetHSTS turns the Strict-Transport-Security header on or off, with
// includeSubDomains if subdomains is set. It needs HTTPS and the redirect
// from HTTP to HTTPS; turning either off turns HSTS off too.
func (m *Manager) SetHSTS(ctx context.Context, id int64, on, subdomains bool) (*store.Site, error) {
	if !m.Supported() {
		return nil, ErrUnsupported
	}
	if err := m.tryLock(); err != nil {
		return nil, err
	}
	defer m.ops.Unlock()
	return m.setHSTS(ctx, id, on, subdomains)
}

func (m *Manager) setHSTS(ctx context.Context, id int64, on, subdomains bool) (*store.Site, error) {
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if on && (!site.SSL || !site.ForceHTTPS) {
		return nil, inputErr("hsts_needs_redirect")
	}
	subdomains = on && subdomains
	if site.HSTS == on && site.HSTSSubdomains == subdomains {
		return site, nil
	}
	old := *site
	site.HSTS, site.HSTSSubdomains = on, subdomains
	if err := m.updateSite(ctx, site, &old); err != nil {
		return nil, err
	}
	return site, nil
}

// ---- page cache (Nginx FastCGI cache) ----

// Each PHP site with the page cache on has its own cache zone and folder,
// declared at the top of its vhost file (which Nginx reads in the http
// block, like the rate limit's zone). Purging empties the folder; turning
// the cache off or deleting the site removes it.

func cacheZone(sysUser string) string { return "panel_fc_" + sysUser }

var cacheZoneRe = regexp.MustCompile(`^panel_fc_[a-z0-9_-]{1,64}$`)

// pageCacheDir is a site's page cache folder, or "" if there can be none.
func (m *Manager) pageCacheDir(domain string) string {
	if !filepath.IsAbs(m.paths.NginxCache) || validate.Domain(domain) != nil {
		return ""
	}
	return filepath.Join(m.paths.NginxCache, domain)
}

// makePageCacheDir creates a site's page cache folder for Nginx's workers
// (www-data). The folders above it stay root's, so only root can replace
// them. The panel runs with umask 077: every level gets its mode here.
func (m *Manager) makePageCacheDir(dir string) error {
	root := m.paths.NginxCache
	for _, d := range []string{filepath.Dir(root), root} {
		if err := mkdir(d, 0o755); err != nil {
			return err
		}
	}
	if err := mkdir(dir, 0o700); err != nil {
		return err
	}
	// Nginx also gives the folder to its workers when it loads the
	// configuration; this is in case it does not.
	if uid, gid, err := lookupIDs("www-data"); err == nil {
		if err := os.Chown(dir, uid, gid); err != nil {
			m.log.Warn("page cache folder owner", "dir", dir, "err", err)
		}
	}
	return nil
}

// emptyDir deletes everything inside dir and keeps dir itself. Symbolic
// links are deleted, never followed.
func emptyDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// SetPageCache turns Nginx's page cache on or off for a PHP site. Turning
// it off deletes the cached pages.
func (m *Manager) SetPageCache(ctx context.Context, id int64, on bool) (*store.Site, error) {
	if !m.Supported() {
		return nil, ErrUnsupported
	}
	if err := m.tryLock(); err != nil {
		return nil, err
	}
	defer m.ops.Unlock()
	return m.setPageCache(ctx, id, on)
}

func (m *Manager) setPageCache(ctx context.Context, id int64, on bool) (*store.Site, error) {
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if on {
		if ws := m.web(); ws.kind() != ServerNginx {
			return nil, inputErr("page_cache_nginx_only", ws.title())
		}
		if site.PHPVersion == "" {
			return nil, inputErr("page_cache_needs_php")
		}
	}
	if site.PageCache == on {
		return site, nil
	}
	dir := m.pageCacheDir(site.Domain)
	if dir == "" {
		return nil, fmt.Errorf("refusing to use a page cache folder for %q", site.Domain)
	}
	old := *site
	site.PageCache = on
	if on {
		if err := m.makePageCacheDir(dir); err != nil {
			return nil, err
		}
	}
	if err := m.updateSite(ctx, site, &old); err != nil {
		if on {
			os.RemoveAll(dir)
		}
		return nil, err
	}
	if !on {
		if err := os.RemoveAll(dir); err != nil {
			m.log.Warn("delete page cache", "domain", site.Domain, "err", err)
		}
	}
	return site, nil
}

// PurgePageCache deletes a site's cached pages: Nginx asks PHP again for
// each page on its next visit.
func (m *Manager) PurgePageCache(ctx context.Context, id int64) (*store.Site, error) {
	if !m.Supported() {
		return nil, ErrUnsupported
	}
	return m.purgePageCache(ctx, id)
}

func (m *Manager) purgePageCache(ctx context.Context, id int64) (*store.Site, error) {
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !site.PageCache {
		return nil, inputErr("page_cache_off")
	}
	dir := m.pageCacheDir(site.Domain)
	if dir == "" {
		return nil, fmt.Errorf("refusing to purge a page cache folder for %q", site.Domain)
	}
	if err := emptyDir(dir); err != nil {
		return nil, err
	}
	return site, nil
}
