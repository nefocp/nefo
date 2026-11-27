package hosting

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/validate"
)

const maxAliases = 20

// CreateRequest describes a new site.
type CreateRequest struct {
	Domain  string
	Aliases []string
	// PHPVersion is an installed PHP-FPM version, or "" for a static site.
	PHPVersion string
	// ProxyPort > 0 makes an app site (see app.go); AppCommand is optional.
	ProxyPort  int
	AppCommand string
	// Suspended creates the site closed (503) until it is opened: Docker
	// apps open their site only after their set-up closed default logins.
	Suspended bool
	// ProxyURL makes a reverse proxy site that forwards to this address
	// (see proxysite.go), with ProxyOpts (DefaultProxyOptions if zero).
	ProxyURL  string
	ProxyOpts store.ProxyOptions
}

// Root returns the folder of a site; its public files are in Root/public.
func (m *Manager) Root(domain string) string { return m.paths.siteRoot(domain) }

// ListSites returns every site.
func (m *Manager) ListSites(ctx context.Context) ([]*store.Site, error) {
	return m.store.ListSites(ctx)
}

func normalizeName(s string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), ".")
}

// checkNames validates and normalises a domain and its aliases, and makes
// sure no other site uses them.
// exceptID is the site being changed, whose own names do not count as taken.
func (m *Manager) checkNames(ctx context.Context, domain string, aliases []string, exceptID int64) (string, []string, error) {
	domain = normalizeName(domain)
	if err := validate.Domain(domain); err != nil {
		return "", nil, inputErr("invalid_domain", domain)
	}
	seen := map[string]bool{domain: true}
	clean := []string{}
	for _, a := range aliases {
		a = normalizeName(a)
		if a == "" || seen[a] {
			continue
		}
		if err := validate.Alias(a); err != nil {
			return "", nil, inputErr("invalid_alias", a)
		}
		seen[a] = true
		clean = append(clean, a)
	}
	if len(clean) > maxAliases {
		return "", nil, inputErr("too_many_aliases")
	}
	sites, err := m.store.ListSites(ctx)
	if err != nil {
		return "", nil, err
	}
	for _, s := range sites {
		if s.ID == exceptID {
			continue
		}
		for _, n := range s.Names() {
			if seen[n] {
				return "", nil, inputErr("domain_exists", n)
			}
		}
	}
	return domain, clean, nil
}

// CreateSite creates the Linux user, folders, PHP-FPM pool and web server
// configuration of a new site. If any step fails, the steps already done are
// undone, so a failed attempt leaves nothing behind.
func (m *Manager) CreateSite(ctx context.Context, req CreateRequest) (*store.Site, error) {
	if !m.Supported() {
		return nil, ErrUnsupported
	}
	if err := m.tryLock(); err != nil {
		return nil, err
	}
	defer m.ops.Unlock()
	if !fileExists(m.web().hookFile()) {
		return nil, ErrNotReady
	}

	domain, aliases, err := m.checkNames(ctx, req.Domain, req.Aliases, 0)
	if err != nil {
		return nil, err
	}
	if req.PHPVersion != "" {
		if validate.PHPVersion(req.PHPVersion) != nil || !m.phpInstalled(req.PHPVersion) {
			return nil, inputErr("php_not_installed", req.PHPVersion)
		}
	}
	req.AppCommand = strings.TrimSpace(req.AppCommand)
	if req.ProxyPort != 0 {
		if req.PHPVersion != "" {
			return nil, inputErr("invalid_app")
		}
		if err := m.checkApp(ctx, req.ProxyPort, req.AppCommand, 0); err != nil {
			return nil, err
		}
	} else if req.AppCommand != "" {
		return nil, inputErr("invalid_app")
	}
	if req.ProxyURL != "" {
		if req.PHPVersion != "" || req.ProxyPort != 0 {
			return nil, inputErr("invalid_app")
		}
		if req.ProxyOpts == (store.ProxyOptions{}) {
			req.ProxyOpts = DefaultProxyOptions()
		}
		if req.ProxyURL, err = m.checkProxy(ctx, req.ProxyURL, &req.ProxyOpts); err != nil {
			return nil, err
		}
	}
	root := m.paths.siteRoot(domain)
	if _, err := os.Lstat(root); err == nil {
		return nil, inputErr("site_dir_exists", root)
	}
	sysUser, err := m.pickUser(ctx, domain)
	if err != nil {
		return nil, err
	}

	site := &store.Site{Domain: domain, Aliases: aliases, SysUser: sysUser, PHPVersion: req.PHPVersion,
		ProxyPort: req.ProxyPort, AppCommand: req.AppCommand, Disabled: req.Suspended,
		ProxyURL: req.ProxyURL, ProxyOpts: req.ProxyOpts}
	if err := validProxySite(site); err != nil {
		return nil, err
	}
	if err := m.store.CreateSite(ctx, site); err != nil {
		if errors.Is(err, store.ErrConflict) {
			return nil, inputErr("domain_exists", domain)
		}
		return nil, err
	}

	// Undo steps run in reverse order if anything below fails. They use a
	// context that is not cancelled with the request.
	bg := context.WithoutCancel(ctx)
	var undo []func()
	ok := false
	defer func() {
		if ok {
			return
		}
		for i := len(undo) - 1; i >= 0; i-- {
			undo[i]()
		}
		if err := m.store.DeleteSite(bg, site.ID); err != nil {
			m.log.Warn("undo site", "domain", domain, "err", err)
		}
	}()

	// 1. The Linux account that owns the files and runs PHP. No password and
	//    no shell: it cannot log in. Its uid and gid come from a range of
	//    their own (see siteIDs), never shared with the uids containers run
	//    as (999, 1000…).
	id, err := freeSiteID()
	if err != nil {
		return nil, err
	}
	if _, err := m.run.Output(ctx, "groupadd", "--system", "--gid", strconv.Itoa(id), sysUser); err != nil {
		return nil, err
	}
	if _, err := m.run.Output(ctx, "useradd", "--system", "--uid", strconv.Itoa(id), "--gid", strconv.Itoa(id),
		"--home-dir", root, "--no-create-home", "--shell", "/usr/sbin/nologin", sysUser); err != nil {
		m.run.Output(context.WithoutCancel(ctx), "groupdel", sysUser)
		return nil, err
	}
	undo = append(undo, func() {
		if err := m.deleteUser(bg, sysUser); err != nil {
			m.log.Warn("undo site user", "user", sysUser, "err", err)
		}
	})
	uid, gid, err := lookupIDs(sysUser)
	if err != nil {
		return nil, err
	}

	// 2. Folders: only the site user (and the web server, through the group) can read them.
	undo = append(undo, func() { os.RemoveAll(root) })
	dirs := []struct {
		path string
		mode os.FileMode
	}{
		{root, 0o750},
		{filepath.Join(root, "public"), 0o750},
		{filepath.Join(root, "logs"), 0o750},
		{filepath.Join(root, "tmp"), 0o700},
	}
	for _, d := range dirs {
		if err := mkdir(d.path, d.mode); err != nil {
			return nil, err
		}
		if err := os.Chown(d.path, uid, gid); err != nil {
			return nil, err
		}
	}
	page, err := renderWelcome(domain)
	if err != nil {
		return nil, err
	}
	index := filepath.Join(root, "public", "index.html")
	if err := writeFile(index, page, 0o640); err != nil {
		return nil, err
	}
	if err := os.Chown(index, uid, gid); err != nil {
		return nil, err
	}

	// 3. Let the web server (www-data) read the site's files through the site's group.
	if _, err := m.run.Output(ctx, "usermod", "--append", "--groups", sysUser, "www-data"); err != nil {
		return nil, err
	}
	// Website accounts may not sign in over SSH (see sftp.go).
	if err := m.ensureSSHD(ctx); err != nil {
		m.log.Warn("ssh settings for websites", "err", err)
	}

	// 4. The PHP-FPM pool, running as the site user.
	if site.PHPVersion != "" {
		pool := m.paths.poolFile(site.PHPVersion, sysUser)
		undo = append(undo, func() {
			removeIfExists(pool)
			if err := m.reloadPHP(bg, site.PHPVersion); err != nil {
				m.log.Warn("undo PHP pool", "err", err)
			}
		})
		if err := m.writePool(site); err != nil {
			return nil, err
		}
		if err := m.reloadPHP(ctx, site.PHPVersion); err != nil {
			return nil, err
		}
		// The reload is asynchronous too: wait for the pool's socket, so the
		// site does not answer 502 to the first visitor.
		if !waitForFile(m.paths.phpSocket(sysUser), 5*time.Second) {
			return nil, fmt.Errorf("PHP-FPM did not create %s", m.paths.phpSocket(sysUser))
		}
	}

	// 5. The virtual host.
	vhost := m.vhostFile(domain)
	undo = append(undo, func() {
		removeIfExists(vhost)
		if err := m.reloadWeb(bg); err != nil {
			m.log.Warn("undo vhost", "err", err)
		}
	})
	if err := m.writeVhost(ctx, site); err != nil {
		return nil, err
	}
	if err := m.reloadWeb(ctx); err != nil {
		return nil, err
	}

	// 6. The app's folder and service.
	if site.ProxyPort != 0 {
		if err := m.appDir(site, uid, gid); err != nil {
			return nil, err
		}
		undo = append(undo, func() { m.removeApp(bg, site) })
		if err := m.writeApp(ctx, site); err != nil {
			return nil, err
		}
	}

	ok = true
	return site, nil
}

// DeleteSite removes a site completely: configuration, certificate, Linux
// user and all of its files. confirm must equal the domain. Cleanup steps that
// fail are returned as warnings; the site is removed from the panel anyway.
func (m *Manager) DeleteSite(ctx context.Context, id int64, confirm string) ([]string, error) {
	if !m.Supported() {
		return nil, ErrUnsupported
	}
	if err := m.tryLock(); err != nil {
		return nil, err
	}
	defer m.ops.Unlock()

	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if normalizeName(confirm) != site.Domain {
		return nil, inputErr("confirm_mismatch")
	}
	if validate.Domain(site.Domain) != nil || validate.SystemUsername(site.SysUser) != nil {
		return nil, fmt.Errorf("refusing to delete a site with an invalid name: %q", site.Domain)
	}

	warnings := []string{}
	warn := func(err error) {
		if err != nil {
			m.log.Warn("delete site", "domain", site.Domain, "err", err)
			warnings = append(warnings, err.Error())
		}
	}

	prot, _ := m.store.SiteProtect(ctx, site.ID)
	siteUID, _, _ := lookupIDs(site.SysUser)
	warn(removeIfExists(m.vhostFile(site.Domain)))
	if m.web().info(ctx).Running {
		warn(m.reloadWeb(ctx))
	}
	if site.PHPVersion != "" {
		warn(removeIfExists(m.paths.poolFile(site.PHPVersion, site.SysUser)))
		if m.phpInstalled(site.PHPVersion) && m.unitActive(ctx, "php"+site.PHPVersion+"-fpm") {
			warn(m.reloadPHP(ctx, site.PHPVersion))
		}
	}
	warn(m.removeApp(ctx, site))
	warn(m.removeSiteApp(ctx, site))
	warn(removeIfExists(m.authFile(site.Domain)))
	warn(removeIfExists(m.customFile(site.Domain)))
	warn(m.deleteSiteMalware(ctx, site.ID))
	if site.SFTP || fileExists(sftpChroot(site.SysUser)) {
		warn(m.disableSFTP(ctx, site))
	}
	m.forgetFileLock(ctx, site, siteUID)
	warn(m.deleteUser(ctx, site.SysUser))
	warn(os.RemoveAll(m.paths.siteRoot(site.Domain)))
	warn(os.RemoveAll(m.paths.certDir(site.Domain)))
	if dir := m.pageCacheDir(site.Domain); dir != "" {
		warn(os.RemoveAll(dir))
	}
	logs, _ := filepath.Glob(filepath.Join(m.web().logDir(), site.Domain+".*.log*"))
	for _, l := range logs {
		warn(os.Remove(l))
	}
	if err := m.store.DeleteSite(ctx, site.ID); err != nil {
		return warnings, err
	}
	if prot != nil && prot.GeoOn() && m.web().info(ctx).Running {
		// The countries only this site listed need not stay loaded.
		warn(m.applyGeo(ctx, nil))
	}
	return warnings, nil
}

// SetForceHTTPS turns the redirect from HTTP to HTTPS on or off.
func (m *Manager) SetForceHTTPS(ctx context.Context, id int64, on bool) (*store.Site, error) {
	if !m.Supported() {
		return nil, ErrUnsupported
	}
	if err := m.tryLock(); err != nil {
		return nil, err
	}
	defer m.ops.Unlock()

	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if on && !site.SSL {
		return nil, inputErr("ssl_required")
	}
	if site.ForceHTTPS == on {
		return site, nil
	}
	old := *site
	site.ForceHTTPS = on
	if !on { // HSTS needs the redirect
		site.HSTS, site.HSTSSubdomains = false, false
	}
	if err := m.updateSite(ctx, site, &old); err != nil {
		return nil, err
	}
	return site, nil
}

// updateSite applies a changed site to the web server and saves it. If the
// web server rejects the new configuration, the old one is put back.
func (m *Manager) updateSite(ctx context.Context, site, old *store.Site) error {
	restore := func() {
		bg := context.WithoutCancel(ctx)
		err := m.writeVhost(bg, old)
		if err == nil {
			err = m.reloadWeb(bg)
		}
		if err != nil {
			m.log.Warn("restore vhost", "domain", old.Domain, "err", err)
		}
	}
	if err := m.writeVhost(ctx, site); err != nil {
		return err
	}
	// The firewall's app exclusions follow the site's app and preset.
	if err := m.writeWAFConf(ctx, site); err != nil {
		restore()
		return err
	}
	if err := m.reloadWeb(ctx); err != nil {
		restore()
		return err
	}
	if err := m.store.UpdateSite(ctx, site); err != nil {
		restore()
		return err
	}
	return nil
}

func (m *Manager) writeVhost(ctx context.Context, site *store.Site) error {
	data, err := m.vhostBytes(ctx, site)
	if err != nil {
		return err
	}
	return writeFile(m.vhostFile(site.Domain), data, 0o644)
}

func (m *Manager) vhostBytes(ctx context.Context, site *store.Site) ([]byte, error) {
	ws := m.web()
	d := vhostData{
		ID:          site.ID,
		Domain:      site.Domain,
		Aliases:     site.Aliases,
		Public:      filepath.Join(m.paths.siteRoot(site.Domain), "public"),
		LogDir:      ws.logDir(),
		SnippetDir:  ws.snippetDir(),
		EmptyRoot:   m.paths.ACMEWebroot,
		SSL:         site.SSL,
		ForceHTTPS:  site.ForceHTTPS,
		UploadMB:    site.PHPUpload,
		TimeoutSec:  site.PHPTime + 30,
		ProxyPort:   site.ProxyPort,
		Disabled:    site.Disabled,
		RedirectURL: site.RedirectURL,
		User:        site.SysUser,
		PHPWorkers:  site.PHPWorkers,
	}
	if site.AuthUser != "" {
		d.AuthFile = m.authFile(site.Domain)
	}
	if site.WAF != WAFOff && m.WAFInstalled() {
		d.WAF = site.WAF
	}
	if site.RateLimit > 0 {
		d.RateLimit, d.RateZone = site.RateLimit, rateZone(site.SysUser)
		d.RateBurst = min(site.RateLimit*5, 1000)
	}
	d.BlockedIPs = strings.Fields(site.BlockedIPs)
	d.Custom404, d.Hotlink, d.HotlinkAllow = site.Custom404, site.Hotlink, strings.Fields(site.HotlinkAllow)
	d.HSTS = siteHSTS(site)
	if site.PageCache && site.PHPVersion != "" && ws.kind() == ServerNginx {
		d.CacheZone, d.CacheDir = cacheZone(site.SysUser), m.pageCacheDir(site.Domain)
	}
	if site.NginxCustom != "" {
		d.CustomFile = m.customFile(site.Domain)
	}
	m.geoVhost(ctx, site, &d)
	if site.SSL {
		d.CertFile = filepath.Join(m.paths.certDir(site.Domain), "fullchain.pem")
		d.KeyFile = filepath.Join(m.paths.certDir(site.Domain), "privkey.pem")
	}
	if site.PHPVersion != "" {
		d.PHPSocket = m.paths.phpSocket(site.SysUser)
	}
	if site.ProxyURL != "" {
		up, err := m.proxyVhostFor(site)
		if err != nil {
			return nil, err
		}
		d.Upstream = up
	}
	return ws.vhost(ctx, d, site.Rewrite)
}

func (m *Manager) writePool(site *store.Site) error {
	data, err := m.poolBytes(site)
	if err != nil {
		return err
	}
	return writeFile(m.paths.poolFile(site.PHPVersion, site.SysUser), data, 0o644)
}

func (m *Manager) poolBytes(site *store.Site) ([]byte, error) {
	return renderPool(poolData{
		Domain:   site.Domain,
		User:     site.SysUser,
		Root:     m.paths.siteRoot(site.Domain),
		Socket:   m.paths.phpSocket(site.SysUser),
		MemoryMB: site.PHPMemory,
		UploadMB: site.PHPUpload,
		TimeSec:  site.PHPTime,
		PM:       site.PHPPM,
		Workers:  site.PHPWorkers,
		Allow:    strings.Fields(site.PHPAllow),

		MaxInputVars: appMaxInputVars(site.App),
		PHP:          site.PHPVersion,
	})
}

// deleteUser removes a site's Linux account and group.
func (m *Manager) deleteUser(ctx context.Context, name string) error {
	if validate.SystemUsername(name) != nil {
		return fmt.Errorf("invalid user name %q", name)
	}
	m.run.Output(ctx, "gpasswd", "--delete", "www-data", name) // fine if not a member
	var err error
	for i := 0; i < 5; i++ {
		if _, lookupErr := user.Lookup(name); lookupErr != nil {
			err = nil
			break
		}
		// userdel refuses while PHP workers of the user are still exiting.
		if _, err = m.run.Output(ctx, "userdel", name); err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if _, lookupErr := user.LookupGroup(name); lookupErr == nil {
		m.run.Output(ctx, "groupdel", name)
	}
	return err
}

// New site users get a uid and an equal gid from siteIDMin to siteIDMax,
// skipping ids that exist already. Before panel 0.17 they were system users
// counting down from 999, which is also the uid of several images
// (mongo, postgres on Debian); those accounts stay as they are.
const (
	siteIDMin = 20000
	siteIDMax = 29999
)

// idTaken is replaceable in tests.
var idTaken = func(id int) bool {
	_, uerr := user.LookupId(strconv.Itoa(id))
	_, gerr := user.LookupGroupId(strconv.Itoa(id))
	return uerr == nil || gerr == nil
}

// freeSiteID returns the lowest id of the range that is neither a uid nor
// a gid.
func freeSiteID() (int, error) {
	for id := siteIDMin; id <= siteIDMax; id++ {
		if !idTaken(id) {
			return id, nil
		}
	}
	return 0, fmt.Errorf("no free user id between %d and %d", siteIDMin, siteIDMax)
}

func lookupIDs(name string) (int, int, error) {
	u, err := user.Lookup(name)
	if err != nil {
		return 0, 0, err
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return 0, 0, err
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return 0, 0, err
	}
	return uid, gid, nil
}

// LookupIDs returns the numeric user and group ID of a Linux account.
func LookupIDs(name string) (int, int, error) { return lookupIDs(name) }
