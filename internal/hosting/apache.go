package hosting

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/validate"
	"github.com/nefocp/nefo/internal/version"
)

// apacheServer is Apache 2.4 from the distribution (package apache2) with
// the event MPM. PHP runs in each site's own PHP-FPM pool, as with Nginx,
// never in mod_php.
type apacheServer struct{ m *Manager }

const (
	apacheSecurityLoad = "mods-available/security2.load"
	apacheWAFInclude   = "/etc/modsecurity/panel.conf"
	apacheUnicodeMap   = "/etc/modsecurity/unicode.mapping"
)

// apacheModules are the modules the panel's configuration uses. a2enmod
// adds their dependencies (ssl needs socache_shmcb, for one).
var apacheModules = []string{
	"mpm_event", "proxy", "proxy_fcgi", "proxy_http", "proxy_wstunnel", "rewrite", "headers", "ssl", "http2",
	"setenvif", "alias", "auth_basic", "authn_core", "authn_file", "authz_core", "authz_host", "authz_user",
	"dir", "mime", "env", "expires",
}

// apacheModulesOff are turned off: directory listings and their /icons/,
// the status page, CGI, home folders (~user), and the MPMs that conflict
// with event.
var apacheModulesOff = []string{"autoindex", "status", "userdir", "cgi", "cgid", "mpm_prefork", "mpm_worker"}

func (a apacheServer) kind() string  { return ServerApache }
func (a apacheServer) title() string { return "Apache" }
func (a apacheServer) unit() string  { return "apache2" }

func (a apacheServer) snippetDir() string { return filepath.Join(a.m.paths.Apache, "panel") }
func (a apacheServer) sitesDir() string   { return filepath.Join(a.m.paths.Apache, "panel", "sites") }
func (a apacheServer) hookFile() string {
	return filepath.Join(a.m.paths.Apache, "conf-available", "panel.conf")
}
func (a apacheServer) hookLink() string {
	return filepath.Join(a.m.paths.Apache, "conf-enabled", "panel.conf")
}
func (a apacheServer) logDir() string { return a.m.paths.ApacheLogs }

func (a apacheServer) packages() []string { return []string{"apache2"} }
func (a apacheServer) removable() []string {
	return []string{"apache2", "apache2-bin", "apache2-data", "apache2-utils", "libapache2-mod-security2", "modsecurity-crs"}
}

func (a apacheServer) info(ctx context.Context) ServerInfo {
	var info ServerInfo
	out, err := a.m.run.Output(ctx, "apache2", "-v")
	if err != nil {
		return info
	}
	info.Installed = true
	if mm := apacheVersionRe.FindStringSubmatch(out); mm != nil {
		info.Version = mm[1]
	}
	info.Running = a.m.unitActive(ctx, "apache2")
	return info
}

// prepare creates the panel's folders and the self-signed certificate of
// the default HTTPS host, which answers unknown names.
func (a apacheServer) prepare(ctx context.Context) error {
	p := a.m.paths
	for _, d := range []string{a.snippetDir(), a.sitesDir(), p.WWW, p.ACMEWebroot, filepath.Dir(p.challengeDir()), p.challengeDir()} {
		if err := mkdir(d, 0o755); err != nil {
			return err
		}
	}
	crt, key := filepath.Join(a.snippetDir(), "default.crt"), filepath.Join(a.snippetDir(), "default.key")
	if fileExists(crt) && fileExists(key) {
		return nil
	}
	c, k, err := selfSigned("default.invalid", a.m.now())
	if err != nil {
		return err
	}
	if err := writeFile(key, k, 0o600); err != nil {
		return err
	}
	return writeFile(crt, c, 0o600)
}

// selfSigned makes a certificate for name, valid for ten years.
func selfSigned(name string, now time.Time) (certPEM, keyPEM []byte, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	if err != nil {
		return nil, nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: name},
		DNSNames:     []string{name},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.AddDate(10, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	kd, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: kd}), nil
}

// writeBase sets Apache up for the panel: the modules it needs, the
// distribution's defaults that would serve files outside the sites off,
// and the panel's shared files.
func (a apacheServer) writeBase(ctx context.Context) error {
	m := a.m
	if err := a.prepare(ctx); err != nil {
		return err
	}
	// Disable first: mpm_event conflicts with an enabled prefork.
	for _, mod := range a.enabledPHPModules() {
		m.run.Output(ctx, "a2dismod", "-q", "-f", mod)
	}
	for _, mod := range apacheModulesOff {
		if fileExists(filepath.Join(m.paths.Apache, "mods-enabled", mod+".load")) {
			if out, err := m.run.Output(ctx, "a2dismod", "-q", "-f", mod); err != nil {
				return fmt.Errorf("turn off the Apache module %s: %s", mod, run.Tail(out, 3))
			}
		}
	}
	if out, err := m.run.Output(ctx, "a2enmod", append([]string{"-q"}, apacheModules...)...); err != nil {
		return fmt.Errorf("turn on the Apache modules: %s", run.Tail(out, 5))
	}
	// alias.conf maps /icons/ on every site, hiding the sites' own folder.
	removeIfExists(filepath.Join(m.paths.Apache, "mods-enabled", "alias.conf"))
	// The default site serves /var/www/html; cgi-bin, the shared access log
	// and the distribution's PHP-FPM handler (the www-data pool for every
	// .php file) are not wanted either.
	for _, site := range []string{"000-default", "default-ssl"} {
		removeIfExists(filepath.Join(m.paths.Apache, "sites-enabled", site+".conf"))
	}
	confs, _ := filepath.Glob(filepath.Join(m.paths.Apache, "conf-enabled", "php*-fpm.conf"))
	for _, c := range append(confs,
		filepath.Join(m.paths.Apache, "conf-enabled", "serve-cgi-bin.conf"),
		filepath.Join(m.paths.Apache, "conf-enabled", "other-vhosts-access-log.conf")) {
		removeIfExists(c)
	}

	files, err := a.baseFiles(ctx)
	if err != nil {
		return err
	}
	for path, data := range files {
		if err := writeFile(path, data, 0o644); err != nil {
			return err
		}
	}
	if err := mkdir(filepath.Dir(a.hookLink()), 0o755); err != nil {
		return err
	}
	if target, err := os.Readlink(a.hookLink()); err != nil || target != "../conf-available/panel.conf" {
		removeIfExists(a.hookLink())
		if err := os.Symlink("../conf-available/panel.conf", a.hookLink()); err != nil {
			return fmt.Errorf("turn on the panel's Apache configuration: %w", err)
		}
	}
	return nil
}

// enabledPHPModules lists enabled mod_php modules (php8.3 and so on): the
// panel runs PHP in PHP-FPM only.
func (a apacheServer) enabledPHPModules() []string {
	matches, _ := filepath.Glob(filepath.Join(a.m.paths.Apache, "mods-enabled", "php*.load"))
	var out []string
	for _, p := range matches {
		out = append(out, strings.TrimSuffix(filepath.Base(p), ".load"))
	}
	return out
}

func (a apacheServer) removeBase() error {
	for _, f := range []string{a.hookLink(), a.hookFile()} {
		if err := removeIfExists(f); err != nil {
			return err
		}
	}
	if err := os.RemoveAll(a.snippetDir()); err != nil {
		return err
	}
	removeEmptyDirs(a.m.paths.Apache)
	return nil
}

func (a apacheServer) baseFiles(ctx context.Context) (map[string][]byte, error) {
	m := a.m
	p := m.paths
	acme, err := render(apacheACMETmpl, struct{ Webroot string }{p.ACMEWebroot})
	if err != nil {
		return nil, err
	}
	for _, path := range []string{p.WWW, p.ACMEWebroot, a.snippetDir(), a.sitesDir()} {
		if !safePath(path) || path == "" {
			return nil, fmt.Errorf("refusing to render unsafe path %q", path)
		}
	}
	hook, err := render(apacheHookTmpl, apacheHookData{WWW: p.WWW, EmptyRoot: p.ACMEWebroot, SnippetDir: a.snippetDir(), SitesDir: a.sitesDir()})
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{
		filepath.Join(a.snippetDir(), "acme.conf"): acme,
		a.hookFile(): hook,
	}
	if wafEnabledOf(a) {
		sites, err := m.store.ListSites(ctx)
		if err != nil {
			return nil, err
		}
		for path, data := range a.wafFiles(sites) {
			files[path] = data
		}
	}
	return files, nil
}

func (a apacheServer) vhost(ctx context.Context, d vhostData, rewrite string) ([]byte, error) {
	// Apache has no light per-IP rate limit; SetFirewall refuses one, and
	// one set under Nginx is not applied.
	d.RateLimit, d.RateBurst, d.RateZone = 0, 0, ""
	return renderApacheVhost(d, rewrite)
}

func (a apacheServer) customBytes(site *store.Site) []byte {
	return []byte("# Managed by " + version.AppName + ": edit in Websites → Apache configuration.\n" +
		strings.TrimRight(site.NginxCustom, "\n") + "\n")
}

func (a apacheServer) test(ctx context.Context) (string, error) {
	return a.m.run.Output(ctx, "apache2ctl", "-t")
}

// testError keeps the lines of "apache2ctl -t" output that explain the
// problem.
func (a apacheServer) testError(out string) string { return apacheTestError(out) }

func apacheTestError(out string) string {
	var keep []string
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.Contains(l, "AH00558") || strings.HasPrefix(l, "Action '") ||
			strings.HasPrefix(l, "The Apache error log may have more information") || l == "Syntax OK" {
			continue
		}
		keep = append(keep, l)
	}
	if len(keep) == 0 {
		return run.Tail(out, 5)
	}
	if len(keep) > 10 {
		keep = keep[:10]
	}
	return strings.Join(keep, "\n")
}

func (a apacheServer) reload(ctx context.Context) error {
	if out, err := a.test(ctx); err != nil {
		return fmt.Errorf("apache configuration test failed:\n%s", apacheTestError(out))
	}
	before := activeApacheWorkers()
	if err := a.m.systemctl(ctx, "reload", "apache2"); err != nil {
		return err
	}
	if !waitWorkers(before, 5*time.Second, activeApacheWorkers) {
		a.m.log.Warn("apache reload: old workers still accept connections after 5 seconds")
	}
	return nil
}

func (a apacheServer) restart(ctx context.Context) error {
	if out, err := a.test(ctx); err != nil {
		return fmt.Errorf("apache configuration test failed:\n%s", apacheTestError(out))
	}
	if err := a.m.systemctl(ctx, "restart", "apache2"); err != nil {
		return err
	}
	if !waitWorkers(nil, 10*time.Second, activeApacheWorkers) {
		a.m.log.Warn("apache restart: no worker accepts connections after 10 seconds")
	}
	return nil
}

var (
	adminerTokenRe = regexp.MustCompile(`^[0-9a-f]{16,128}$`)
	adminerPortRe  = regexp.MustCompile(`^[0-9]{2,5}$`)
)

func (a apacheServer) adminerConf(port, token, php string) []byte {
	if !adminerTokenRe.MatchString(token) || !adminerPortRe.MatchString(port) || validate.PHPVersion(php) != nil {
		return nil
	}
	out, err := render(apacheAdminerTmpl, struct{ Port, Token, Dir, PHP string }{port, token, adminerDir, php})
	if err != nil {
		return nil
	}
	return out
}

// ---- web application firewall (mod_security2) ----

func (a apacheServer) wafAvailable() bool {
	return fileExists(filepath.Join(a.m.paths.Apache, apacheSecurityLoad)) &&
		fileExists(filepath.Join(wafCRSRules, "REQUEST-901-INITIALIZATION.conf")) && fileExists(wafCRSSetup)
}

func (a apacheServer) wafPackages() []string {
	return []string{"libapache2-mod-security2", "modsecurity-crs"}
}

func (a apacheServer) wafExtraFiles() []string { return []string{apacheWAFInclude} }

// wafFiles renders Apache's ModSecurity settings. The distribution's
// security2.conf loads /etc/modsecurity/*.conf and then the rule set, so
// the panel's file there only includes its settings: the engine is off
// unless a site turns it on in its <Location /> block.
func (a apacheServer) wafFiles(sites []*store.Site) map[string][]byte {
	dir := wafDirOf(a)
	main := fmt.Sprintf(`# Managed by %[1]s. Do not edit: the panel rewrites this file.
# ModSecurity settings, loaded before the OWASP Core Rule Set (which the
# distribution's security2.conf includes). Sites turn the engine on.
SecRuleEngine Off
SecRequestBodyAccess On
SecRule REQUEST_HEADERS:Content-Type "^(?:application(?:/soap\+|/)|text/)xml" \
    "id:200000,phase:1,t:none,t:lowercase,pass,nolog,ctl:requestBodyProcessor=XML"
SecRule REQUEST_HEADERS:Content-Type "^application/json" \
    "id:200001,phase:1,t:none,t:lowercase,pass,nolog,ctl:requestBodyProcessor=JSON"
# ModSecurity 2 cannot pass the rest of a body it inspected in part
# (ProcessPartial) to PHP-FPM or an app: the request fails or hangs. So
# bodies are inspected whole, buffered on disk past 128 KB: uploads up to
# 1 GB (ModSecurity 2's maximum), other bodies (forms, JSON) up to 16 MB.
# Larger ones get 413 while the firewall is on.
SecRequestBodyLimit 1073741824
SecRequestBodyNoFilesLimit 16777216
SecRequestBodyInMemoryLimit 131072
SecRequestBodyLimitAction Reject
SecRequestBodyJsonDepthLimit 512
SecArgumentsLimit 1000
SecRule &ARGS "@ge 1000" \
    "id:200007,phase:2,t:none,log,deny,status:400,msg:'Failed to fully parse request body due to large argument count',severity:2"
# A body cut at the limit above cannot parse completely (a large upload
# misses its final boundary): only refuse bodies that were read in full.
SecRule REQBODY_ERROR "!@eq 0" \
    "id:200002,phase:2,t:none,log,deny,status:400,msg:'Failed to parse request body.',logdata:'%%{reqbody_error_msg}',severity:2,chain"
    SecRule INBOUND_DATA_ERROR "@eq 0" "t:none"
SecPcreMatchLimit 100000
SecPcreMatchLimitRecursion 100000
SecResponseBodyAccess Off
SecTmpDir /tmp/
SecArgumentSeparator &
SecCookieFormat 0
SecUnicodeMapFile %[2]s 20127
SecStatusEngine Off

# One JSON line per request the rules flagged, shown in the panel.
SecAuditEngine RelevantOnly
SecAuditLogRelevantStatus "^(?:5|4(?!04))"
SecAuditLogParts ABFHZ
SecAuditLogType Serial
SecAuditLogFormat JSON
SecAuditLog %[3]s

%[4]s`, version.AppName, apacheUnicodeMap, wafLog(), wafExclusions(sites))
	include := fmt.Sprintf(`# Managed by %s. Do not edit: the panel rewrites this file.
# The panel's ModSecurity settings (Websites → Settings → Web firewall).
IncludeOptional %s
`, version.AppName, filepath.Join(dir, "main.conf"))
	return map[string][]byte{
		filepath.Join(dir, "main.conf"): []byte(main),
		apacheWAFInclude:                []byte(include),
	}
}

// ---- waiting for a graceful reload ----

// activeApacheWorkers returns the PIDs of Apache's child processes that
// still hold a listening socket. After a graceful reload the old children
// close their listeners at once and only finish the requests they have, so
// a child that listens answers new connections.
func activeApacheWorkers() map[int]bool {
	pids := map[int]bool{}
	listen := listeningInodes()
	if len(listen) == 0 {
		return pids
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return pids
	}
	parent := map[int]int{} // apache2 processes: pid → parent pid
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		comm, err := os.ReadFile(filepath.Join("/proc", e.Name(), "comm"))
		if err != nil || strings.TrimSpace(string(comm)) != "apache2" {
			continue
		}
		parent[pid] = procParent(pid)
	}
	for pid, ppid := range parent {
		if _, ok := parent[ppid]; !ok {
			continue // the main process
		}
		if holdsSocket(pid, listen) {
			pids[pid] = true
		}
	}
	return pids
}

// procParent reads the parent PID from /proc/<pid>/stat.
func procParent(pid int) int {
	b, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return 0
	}
	// "pid (comm) state ppid …": comm may contain spaces, so start after ')'.
	i := bytes.LastIndexByte(b, ')')
	if i < 0 {
		return 0
	}
	f := strings.Fields(string(b[i+1:]))
	if len(f) < 2 {
		return 0
	}
	ppid, _ := strconv.Atoi(f[1])
	return ppid
}

// listeningInodes returns the inodes of the listening TCP sockets.
func listeningInodes() map[string]bool {
	out := map[string]bool{}
	for _, name := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		f, err := os.Open(name)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			fields := strings.Fields(sc.Text())
			// sl local rem st tx:rx tr:when retrnsmt uid timeout inode
			if len(fields) > 9 && fields[3] == "0A" {
				out[fields[9]] = true
			}
		}
		f.Close()
	}
	return out
}

func holdsSocket(pid int, inodes map[string]bool) bool {
	dir := filepath.Join("/proc", strconv.Itoa(pid), "fd")
	fds, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, fd := range fds {
		target, err := os.Readlink(filepath.Join(dir, fd.Name()))
		if err != nil {
			continue
		}
		if ino, ok := strings.CutPrefix(target, "socket:["); ok && inodes[strings.TrimSuffix(ino, "]")] {
			return true
		}
	}
	return false
}

// waitWorkers waits until the workers in before no longer accept
// connections and at least one new worker does, for at most timeout.
func waitWorkers(before map[int]bool, timeout time.Duration, active func() map[int]bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		now := active()
		fresh, stale := false, false
		for pid := range now {
			if before[pid] {
				stale = true
			} else {
				fresh = true
			}
		}
		if fresh && !stale {
			return true
		}
		time.Sleep(25 * time.Millisecond)
	}
	return false
}
