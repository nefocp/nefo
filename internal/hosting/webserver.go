package hosting

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/nefocp/nefo/internal/software"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// A server runs one web server, chosen when the web stack is installed:
// Nginx (the default, and the only choice before Apache was added), Apache,
// whose point is .htaccess support, or OpenLiteSpeed (.htaccess rewrite rules
// and LiteSpeed Cache). All serve the same sites from the same stored
// settings; webServer hides what differs between them.

const (
	ServerNginx  = "nginx"
	ServerApache = "apache"
	ServerOLS    = "openlitespeed"

	// webServerKey is the setting that names the managed web server. A panel
	// from before it existed has none: its Nginx hook file says "nginx".
	webServerKey = "web_server"
)

// webServer is one web server the panel can manage.
type webServer interface {
	kind() string  // ServerNginx, ServerApache or ServerOLS
	title() string // for messages: "Nginx", "Apache" or "OpenLiteSpeed"
	unit() string  // the systemd unit

	snippetDir() string // the panel's own folder: snippets, auth, custom, waf
	sitesDir() string   // one file per site
	hookFile() string   // the panel's main include; it exists once set up
	logDir() string     // <domain>.access.log and <domain>.error.log

	info(ctx context.Context) ServerInfo
	packages() []string // what InstallServer installs, besides PHP
	// removable are the packages RemoveServer purges, if installed.
	removable() []string

	// prepare creates what the base files rely on (folders, the default
	// certificate); writeBase also writes them and adjusts the
	// distribution's defaults.
	prepare(ctx context.Context) error
	writeBase(ctx context.Context) error
	// removeBase deletes the panel's files, after the packages are purged.
	removeBase() error
	baseFiles(ctx context.Context) (map[string][]byte, error)
	// vhost renders a site from the server-neutral fields of d.
	vhost(ctx context.Context, d vhostData, rewrite string) ([]byte, error)
	adminerConf(port, token, php string) []byte
	// phpMyAdminConf is phpMyAdmin's internal server (see phpmyadmin.go).
	phpMyAdminConf(port, token, php string) []byte
	// customBytes is the file with the administrator's own directives.
	customBytes(site *store.Site) []byte

	// test checks the configuration; the error carries the output.
	test(ctx context.Context) (string, error)
	// testError keeps the lines of a failed test that explain it.
	testError(out string) string
	// reload tests the configuration, reloads it gracefully and waits until
	// the new configuration answers requests.
	reload(ctx context.Context) error
	restart(ctx context.Context) error

	// The web application firewall (see waf.go).
	wafAvailable() bool
	wafPackages() []string
	wafFiles(sites []*store.Site) map[string][]byte
	// wafExtraFiles are files outside wafDir that load the rules.
	wafExtraFiles() []string
}

// ServerInfo describes an installed web server.
type ServerInfo struct {
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
	Running   bool   `json:"running"`
}

// NginxInfo is the older name of ServerInfo.
type NginxInfo = ServerInfo

// serverKind returns the web server this panel manages: ServerNginx,
// ServerApache, ServerOLS or "" before one is installed.
func (m *Manager) serverKind() string {
	m.kindMu.Lock()
	defer m.kindMu.Unlock()
	if m.kindLoaded {
		return m.kind
	}
	k := ""
	if m.store != nil {
		k, _ = m.store.Setting(context.Background(), webServerKey)
	}
	if !slices.Contains(serverKinds, k) {
		// No setting: a panel from before the choice existed, or a
		// failed write. The hook file tells.
		k = ""
		for _, kind := range serverKinds {
			if fileExists(m.server(kind).hookFile()) {
				k = kind
				break
			}
		}
	}
	m.kind, m.kindLoaded = k, true
	return k
}

// setServerKind records the managed web server ("" for none).
func (m *Manager) setServerKind(ctx context.Context, kind string) error {
	m.kindMu.Lock()
	defer m.kindMu.Unlock()
	if err := m.store.SetSetting(ctx, webServerKey, kind); err != nil {
		return err
	}
	m.kind, m.kindLoaded = kind, true
	return nil
}

// serverKinds are the web servers the panel can manage, in the order
// conflicts are checked and hook files detected.
var serverKinds = []string{ServerNginx, ServerApache, ServerOLS}

// server returns the implementation of a web server kind.
func (m *Manager) server(kind string) webServer {
	switch kind {
	case ServerApache:
		if m.apache == nil {
			return apacheServer{m}
		}
		return m.apache
	case ServerOLS:
		if m.ols == nil {
			return olsServer{m}
		}
		return m.ols
	}
	if m.nginx == nil {
		return nginxServer{m}
	}
	return m.nginx
}

// web returns the managed web server, or Nginx before one is installed (its
// hook file is missing, so the panel reports that nothing is ready).
func (m *Manager) web() webServer { return m.server(m.serverKind()) }

// WebServer returns the managed web server: ServerNginx, ServerApache,
// ServerOLS or "".
func (m *Manager) WebServer() string { return m.serverKind() }

// FirewallPorts lists the ports the managed web server needs open in the
// firewall: 80/tcp and 443/tcp, and 443/udp when it answers HTTP/3. It is
// empty when no web server is installed.
func (m *Manager) FirewallPorts() []string {
	kind := m.serverKind()
	if kind == "" || !fileExists(m.server(kind).hookFile()) {
		return nil
	}
	ports := []string{"80/tcp", "443/tcp"}
	if u, ok := m.server(kind).(udpServer); ok && u.http3() {
		ports = append(ports, "443/udp")
	}
	return ports
}

// LogDir is the folder of the sites' access and error logs.
func (m *Manager) LogDir() string { return m.web().logDir() }

func (m *Manager) vhostFile(domain string) string {
	return filepath.Join(m.web().sitesDir(), domain+".conf")
}

// reloadWeb reloads the managed web server (see webServer.reload).
func (m *Manager) reloadWeb(ctx context.Context) error { return m.web().reload(ctx) }

// ---- install and remove ----

// ConflictError is a refusal that the API answers with 409 and Code. Its
// message is an English sentence, for job logs.
type ConflictError struct {
	Code   string
	Detail string
	msg    string
}

func (e *ConflictError) Error() string { return e.msg }

// validKind checks a web server name from a request; "" means Nginx.
func validKind(kind string) (string, error) {
	switch kind {
	case "", ServerNginx:
		return ServerNginx, nil
	case ServerApache, ServerOLS:
		return kind, nil
	}
	return "", usererr.New("invalid_web_server", kind)
}

// CheckInstall reports whether the web server kind ("" for Nginx) may be
// installed: no other one may be managed or installed, since they all need
// ports 80 and 443.
func (m *Manager) CheckInstall(ctx context.Context, kind string) (string, error) {
	if !m.Supported() {
		return "", ErrUnsupported
	}
	kind, err := validKind(kind)
	if err != nil {
		return "", err
	}
	for _, k := range serverKinds {
		if k == kind {
			continue
		}
		other := m.server(k)
		if m.serverKind() == other.kind() || other.info(ctx).Installed || fileExists(other.hookFile()) {
			return "", &ConflictError{Code: "web_server_conflict", Detail: other.kind(),
				msg: fmt.Sprintf("%s is installed on this server. Both cannot use ports 80 and 443: remove %s first (its websites must be deleted before).",
					other.title(), other.title())}
		}
	}
	return kind, nil
}

// Install installs Nginx; see InstallServer.
func (m *Manager) Install(ctx context.Context, lg Logger) error {
	return m.InstallServer(ctx, ServerNginx, lg)
}

// InstallServer installs a web server (Nginx, Apache or OpenLiteSpeed) and
// the distribution's PHP-FPM, configures it for the panel and opens ports 80
// and 443 in ufw. It is safe to run again.
func (m *Manager) InstallServer(ctx context.Context, kind string, lg Logger) error {
	kind, err := m.CheckInstall(ctx, kind)
	if err != nil {
		return err
	}
	ws := m.server(kind)
	m.ops.Lock()
	defer m.ops.Unlock()

	// OpenLiteSpeed comes from its vendor's repository.
	if r, ok := ws.(repoServer); ok {
		if err := r.addRepo(ctx, lg); err != nil {
			return err
		}
	}
	lg.Logf("Updating the package lists")
	if err := m.apt(ctx, lg, "update"); err != nil {
		return err
	}
	php := m.distroPHP(ctx)
	if php == "" {
		return errors.New("could not find out which PHP version this distribution offers (apt-cache depends php-fpm)")
	}
	pkgs := append(ws.packages(), phpPackages(php)...)
	lg.Logf("Installing %s", strings.Join(pkgs, " "))
	if err := m.apt(ctx, lg, append([]string{"install", "--no-install-recommends"}, pkgs...)...); err != nil {
		return err
	}
	if err := m.setServerKind(ctx, kind); err != nil {
		return err
	}

	lg.Logf("Configuring %s", ws.title())
	if err := ws.writeBase(ctx); err != nil {
		return err
	}
	if err := software.TunePHPPool(php); err != nil {
		lg.Logf("Warning: %v", err)
	}
	for _, unit := range []string{ws.unit(), "php" + php + "-fpm"} {
		if _, err := m.run.Output(ctx, "systemctl", "enable", "--now", unit); err != nil {
			return err
		}
	}
	if kind == ServerApache || kind == ServerOLS {
		// Apache: modules changed (and maybe the MPM). OpenLiteSpeed: the
		// listeners changed. A reload is not enough.
		if err := ws.restart(ctx); err != nil {
			return err
		}
	}
	if err := ws.reload(ctx); err != nil {
		return err
	}

	if m.ufwActive(ctx) {
		ports := []string{"80/tcp", "443/tcp"}
		if u, ok := ws.(udpServer); ok && u.http3() {
			lg.Logf("Opening ports 80 and 443 in the firewall (ufw), and 443/udp for HTTP/3")
			ports = append(ports, "443/udp")
		} else {
			lg.Logf("Opening ports 80 and 443 in the firewall (ufw)")
		}
		for _, port := range ports {
			if _, err := m.run.Output(ctx, "ufw", "allow", port, "comment", "Websites"); err != nil {
				return err
			}
		}
	}
	lg.Logf("The web server is ready: %s and PHP %s", ws.title(), php)
	return nil
}

// sitesOnDisk returns the site files in a web server's sites folder. A
// throwaway panel on the same server keeps its sites there too, which this
// panel's database does not know about.
func sitesOnDisk(ws webServer) []string {
	matches, _ := filepath.Glob(filepath.Join(ws.sitesDir(), "*.conf"))
	var out []string
	for _, p := range matches {
		name := strings.TrimSuffix(filepath.Base(p), ".conf")
		if !strings.HasPrefix(name, "_") { // _adminer.conf is not a site
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// CheckRemove reports whether the web server confirm names may be removed:
// no website may exist, in this panel or on disk.
func (m *Manager) CheckRemove(ctx context.Context, confirm string) (string, error) {
	if !m.Supported() {
		return "", ErrUnsupported
	}
	if !slices.Contains(serverKinds, confirm) {
		return "", usererr.New("invalid_web_server", confirm)
	}
	ws := m.server(confirm)
	if m.serverKind() != confirm && !ws.info(ctx).Installed && !fileExists(ws.hookFile()) {
		return "", usererr.New("web_server_not_installed", confirm)
	}
	sites, err := m.store.ListSites(ctx)
	if err != nil {
		return "", err
	}
	names := []string{}
	for _, s := range sites {
		names = append(names, s.Domain)
	}
	for _, d := range sitesOnDisk(ws) {
		if !slices.Contains(names, d) {
			names = append(names, d)
		}
	}
	if len(names) > 0 {
		detail := strings.Join(names, ", ")
		if len(names) > 5 {
			detail = strings.Join(names[:5], ", ") + " … (" + strconv.Itoa(len(names)) + ")"
		}
		return "", &ConflictError{Code: "web_server_in_use", Detail: detail,
			msg: fmt.Sprintf("%s still serves websites (%s). Delete them first: switching a server that has websites to the other web server is not supported.",
				ws.title(), detail)}
	}
	return confirm, nil
}

// RemoveServer uninstalls a web server: it stops it, purges its packages
// (and its firewall module), deletes the panel's files for it, Adminer's
// and phpMyAdmin's internal servers included, and closes the ports ufw
// opened for websites.
// Site folders in /var/www, certificates and the ACME account stay. PHP
// stays too: the Software page manages it.
func (m *Manager) RemoveServer(ctx context.Context, confirm string, lg Logger) error {
	m.ops.Lock()
	defer m.ops.Unlock()
	kind, err := m.CheckRemove(ctx, confirm)
	if err != nil {
		return err
	}
	ws := m.server(kind)

	lg.Logf("Stopping %s", ws.title())
	m.run.Output(ctx, "systemctl", "disable", "--now", ws.unit()) // fine if it is not running

	if pkgs := m.installedPackages(ctx, ws.removable()...); len(pkgs) > 0 {
		lg.Logf("Removing %s", strings.Join(pkgs, " "))
		purge := func() error { return m.apt(ctx, lg, append([]string{"purge"}, pkgs...)...) }
		if g, ok := ws.(phpGuard); ok && g.guardsPHPModules() {
			// See ols_phpguard.go: the purge would turn off PHP's modules.
			st := snapshotPHPModules(m.paths.PHP, phpModuleRegistry)
			err := purge()
			if n := st.restore(m.paths.PHP, phpModuleRegistry); n > 0 {
				lg.Logf("Restored %d PHP module settings that the removal of LiteSpeed's lsphp packages changed", n)
			}
			if err != nil {
				return err
			}
		} else if err := purge(); err != nil {
			return err
		}
	}
	// Purging apache2 deletes /var/www when no site is left in it.
	if err := mkdir(m.paths.WWW, 0o755); err != nil {
		return err
	}

	lg.Logf("Removing the panel's %s configuration", ws.title())
	if m.AdminerInstalled() || fileExists(filepath.Join(adminerDir, "index.php")) {
		lg.Logf("Removing Adminer, which needs the web server")
	}
	if err := m.removeAdminer(ctx); err != nil {
		return err
	}
	if fileExists(filepath.Join(phpMyAdminDir, "index.php")) {
		lg.Logf("Removing phpMyAdmin, which needs the web server")
	}
	if err := m.removePhpMyAdmin(ctx); err != nil {
		return err
	}
	for _, f := range ws.wafExtraFiles() {
		if err := removeIfExists(f); err != nil {
			return err
		}
	}
	if err := ws.removeBase(); err != nil {
		return err
	}
	if !wafAvailableAny(m) {
		removeIfExists(wafLogrotate)
	}

	if m.ufwActive(ctx) {
		lg.Logf("Closing ports 80 and 443 in the firewall (ufw rules for websites)")
		if err := m.deleteUFWWebsites(ctx); err != nil {
			return err
		}
	}
	if m.serverKind() == kind {
		if err := m.setServerKind(ctx, ""); err != nil {
			return err
		}
	}
	lg.Logf("%s was removed. Website folders in %s, certificates and the Let's Encrypt account are kept.", ws.title(), m.paths.WWW)
	return nil
}

// wafAvailableAny reports whether any web server's firewall module is
// still installed.
func wafAvailableAny(m *Manager) bool {
	for _, k := range serverKinds {
		if m.server(k).wafAvailable() {
			return true
		}
	}
	return false
}

// repoServer is a web server whose packages come from a repository the
// panel adds before installing (OpenLiteSpeed).
type repoServer interface {
	addRepo(ctx context.Context, lg Logger) error
}

// udpServer is a web server that can also answer HTTP/3 (QUIC) on UDP 443.
type udpServer interface {
	http3() bool
}

// installedPackages returns those of names that dpkg knows as installed or
// as removed with their configuration kept.
func (m *Manager) installedPackages(ctx context.Context, names ...string) []string {
	if len(names) == 0 {
		return nil
	}
	// Unknown names make dpkg-query fail but it still prints the others.
	out, _ := m.run.Output(ctx, "dpkg-query", append([]string{"-W", "-f=${Package} ${Status}\n"}, names...)...)
	var pkgs []string
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 4 || !slices.Contains(names, f[0]) {
			continue
		}
		if st := strings.Join(f[1:], " "); st == "install ok installed" || strings.HasSuffix(st, "config-files") || strings.HasSuffix(st, "half-configured") || strings.HasSuffix(st, "unpacked") {
			pkgs = append(pkgs, f[0])
		}
	}
	return pkgs
}

// deleteUFWWebsites deletes the ufw rules InstallServer added: those whose
// comment is "Websites". Rules the administrator added stay.
func (m *Manager) deleteUFWWebsites(ctx context.Context) error {
	out, err := m.run.Output(ctx, "ufw", "status", "numbered")
	if err != nil {
		return err
	}
	nums := ufwWebsiteRules(out)
	// Highest first: deleting a rule renumbers the ones after it.
	for i := len(nums) - 1; i >= 0; i-- {
		if _, err := m.run.Output(ctx, "ufw", "--force", "delete", strconv.Itoa(nums[i])); err != nil {
			return err
		}
	}
	return nil
}

// ufwWebsiteRules returns the numbers of the "Websites" rules in the output
// of "ufw status numbered", in ascending order.
func ufwWebsiteRules(out string) []int {
	var nums []int
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "# Websites") {
			continue
		}
		end := strings.IndexByte(line, ']')
		if end < 0 {
			continue
		}
		if n, err := strconv.Atoi(strings.TrimSpace(line[1:end])); err == nil && n > 0 {
			nums = append(nums, n)
		}
	}
	sort.Ints(nums)
	return nums
}

// removeEmptyDirs deletes the empty folders under dir, and dir itself if it
// ends up empty. Folders with files stay.
func removeEmptyDirs(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() && e.Type()&os.ModeSymlink == 0 {
			removeEmptyDirs(filepath.Join(dir, e.Name()))
		}
	}
	os.Remove(dir) // fails, harmlessly, unless empty
}
