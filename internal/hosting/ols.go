package hosting

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/validate"
	"github.com/nefocp/nefo/internal/version"
)

// olsServer is OpenLiteSpeed from LiteSpeed's own apt repository (Debian
// and Ubuntu do not package it). Its workers run as www-data, like Nginx's
// and Apache's, and PHP runs in each site's own PHP-FPM pool through
// FastCGI, never in LiteSpeed's lsphp.
//
// The WebAdmin console is switched off (disableWebAdmin): it would listen on
// port 7080 for the whole internet and edit the files the panel writes.
// The panel's own files live in <OLS>/panel, outside conf/, which the
// package's upgrade script hands over to its lsadm user.
//
// OpenLiteSpeed never reads the sites' .htaccess files itself: it would only
// notice changes after a restart, and a .htaccess file could pass PHP_VALUE
// or PHP_ADMIN_VALUE to PHP-FPM through a rewrite rule's environment flag
// (extension=, sendmail_path= or open_basedir= for the pool's user). The
// panel copies their rewrite rules, without such flags, into the site's
// configuration, and rewrites it when a file changes (ols_htaccess.go).
type olsServer struct{ m *Manager }

const (
	olsUnit = "lshttpd"
	// olsPIDFile is where lswsctrl and the server keep the main process.
	olsPIDFile = "/tmp/lshttpd/lshttpd.pid"
	// olsDropIn keeps systemd from starting OpenLiteSpeed before the
	// panel's configuration exists: the package starts it at once with its
	// demo site on 8088 and the WebAdmin console on 7080.
	olsDropIn    = "/etc/systemd/system/lshttpd.service.d/panel.conf"
	olsLogrotate = "/etc/logrotate.d/panel-openlitespeed"
)

func (o olsServer) kind() string  { return ServerOLS }
func (o olsServer) title() string { return "OpenLiteSpeed" }
func (o olsServer) unit() string  { return olsUnit }

func (o olsServer) home() string       { return o.m.paths.OLS }
func (o olsServer) bin() string        { return filepath.Join(o.home(), "bin", "openlitespeed") }
func (o olsServer) snippetDir() string { return filepath.Join(o.home(), "panel") }
func (o olsServer) sitesDir() string   { return filepath.Join(o.home(), "panel", "sites") }
func (o olsServer) hookFile() string   { return filepath.Join(o.home(), "panel", "server.conf") }
func (o olsServer) logDir() string     { return o.m.paths.OLSLogs }

// mainConf is the file OpenLiteSpeed reads; it includes hookFile.
func (o olsServer) mainConf() string { return filepath.Join(o.home(), "conf", "httpd_config.conf") }

// errorDir holds the page suspended sites show. It is outside the panel's
// folder, which is in the server's list of folders never served.
func (o olsServer) errorDir() string { return filepath.Join(o.home(), "panel-errors") }

func (o olsServer) packages() []string { return []string{"openlitespeed"} }

// removable are the packages RemoveServer purges: OpenLiteSpeed, its
// modules and the lsphp build the package depends on (lsphp83 today).
func (o olsServer) removable() []string {
	pkgs := []string{"openlitespeed", "ols-modsecurity", "ols-pagespeed", "modsecurity-crs"}
	out, _ := o.m.run.Output(context.Background(), "dpkg-query", "-W", "-f=${Package}\n", "lsphp*")
	for _, p := range strings.Fields(out) {
		if lsphpPkgRe.MatchString(p) && !slices.Contains(pkgs, p) {
			pkgs = append(pkgs, p)
		}
	}
	return pkgs
}

var lsphpPkgRe = regexp.MustCompile(`^lsphp[0-9]{2}(-[a-z0-9]+)*$`)

func (o olsServer) info(ctx context.Context) ServerInfo {
	var info ServerInfo
	// Look for the program first: on Nginx and Apache servers this costs
	// no process.
	if !fileExists(o.bin()) {
		return info
	}
	out, err := o.m.run.Output(ctx, o.bin(), "-v")
	if err != nil {
		return info
	}
	info.Installed = true
	if mm := olsVersionRe.FindStringSubmatch(out); mm != nil {
		info.Version = mm[1]
	}
	info.Running = o.m.unitActive(ctx, olsUnit)
	return info
}

// http3 reports that the panel's listeners answer HTTP/3 on UDP 443, which
// costs OpenLiteSpeed no extra process: ufw opens 443/udp too.
func (o olsServer) http3() bool { return true }

// prepare creates the panel's folders and the self-signed certificate of
// the default HTTPS listener, which answers unknown names.
func (o olsServer) prepare(ctx context.Context) error {
	p := o.m.paths
	for _, d := range []string{o.snippetDir(), o.sitesDir(), filepath.Join(o.snippetDir(), "auth"),
		filepath.Join(o.snippetDir(), "custom"), o.errorDir(), p.WWW, p.ACMEWebroot,
		filepath.Dir(p.challengeDir()), p.challengeDir()} {
		if err := mkdir(d, 0o755); err != nil {
			return err
		}
	}
	// The site logs: OpenLiteSpeed's main process (root) creates them, its
	// workers (www-data) write them. Site users may not read them.
	if err := mkdir(o.logDir(), 0o750); err != nil {
		return err
	}
	if gid, ok := wwwDataGID(); ok {
		os.Chown(o.logDir(), 0, gid)
	}
	crt, key := filepath.Join(o.snippetDir(), "default.crt"), filepath.Join(o.snippetDir(), "default.key")
	if fileExists(crt) && fileExists(key) {
		return nil
	}
	c, k, err := selfSigned("default.invalid", o.m.now())
	if err != nil {
		return err
	}
	if err := writeFile(key, k, 0o600); err != nil {
		return err
	}
	return writeFile(crt, c, 0o600)
}

// writeBase sets OpenLiteSpeed up for the panel: its configuration, the
// systemd drop-in that keeps it stopped without that configuration, and
// log rotation.
func (o olsServer) writeBase(ctx context.Context) error {
	if err := o.prepare(ctx); err != nil {
		return err
	}
	files, err := o.baseFiles(ctx)
	if err != nil {
		return err
	}
	for path, data := range files {
		if err := writeFile(path, data, 0o644); err != nil {
			return err
		}
	}
	return o.writeDropIn(ctx)
}

func (o olsServer) writeDropIn(ctx context.Context) error {
	want := []byte(fmt.Sprintf(`# Managed by %s. OpenLiteSpeed starts only once the panel has written
# its configuration: the package's own (a demo site on 8088 and the WebAdmin
# console on 7080) is never served.
[Unit]
ConditionPathExists=%s
`, version.AppName, o.hookFile()))
	if old, err := os.ReadFile(olsDropIn); err == nil && bytes.Equal(old, want) {
		return nil
	}
	if err := mkdir(filepath.Dir(olsDropIn), 0o755); err != nil {
		return err
	}
	if err := writeFile(olsDropIn, want, 0o644); err != nil {
		return err
	}
	_, err := o.m.run.Output(ctx, "systemctl", "daemon-reload")
	return err
}

// removeBase deletes what is left after the packages are purged: the
// panel's files, OpenLiteSpeed's home with its logs and cache, its runtime
// files, the repository and its key.
func (o olsServer) removeBase() error {
	stopOLSProcesses()
	for _, f := range []string{olsDropIn, olsLogrotate, olsRepoList, olsKeyring} {
		if err := removeIfExists(f); err != nil {
			return err
		}
	}
	os.Remove(filepath.Dir(olsDropIn))
	o.m.run.Output(context.Background(), "systemctl", "daemon-reload")
	for _, d := range []string{o.home(), "/tmp/lshttpd", "/dev/shm/ols"} {
		// OpenLiteSpeed's own folders only (the home is /usr/local/lsws).
		if !filepath.IsAbs(d) || (filepath.Base(d) != "lsws" && filepath.Base(d) != "lshttpd" && filepath.Base(d) != "ols") {
			continue
		}
		if err := os.RemoveAll(d); err != nil {
			return err
		}
	}
	removeEmptyDirs(o.logDir())
	return nil
}

// stopOLSProcesses ends OpenLiteSpeed processes that outlived the service:
// its unit uses KillMode=none, so a stop can leave lscgid or an older
// generation of workers behind.
func stopOLSProcesses() {
	pids := olsProcesses(nil)
	for _, pid := range pids {
		if p, err := os.FindProcess(pid); err == nil {
			p.Signal(syscall.SIGTERM)
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && len(olsProcesses(nil)) > 0 {
		time.Sleep(50 * time.Millisecond)
	}
	for _, pid := range olsProcesses(nil) {
		if p, err := os.FindProcess(pid); err == nil {
			p.Kill()
		}
	}
}

// olsProcesses returns the PIDs of OpenLiteSpeed's processes ("openlitespeed
// (lshttpd - main)", "... - #01", "... (lscgid)"); with match, only those
// whose command line it accepts.
func olsProcesses(match func(cmdline string, pid int) bool) []int {
	var pids []int
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		cmd, err := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline"))
		if err != nil || !bytes.HasPrefix(cmd, []byte("openlitespeed (")) {
			continue
		}
		if match == nil || match(string(cmd), pid) {
			pids = append(pids, pid)
		}
	}
	return pids
}

// baseFiles renders the panel's shared OpenLiteSpeed files.
func (o olsServer) baseFiles(ctx context.Context) (map[string][]byte, error) {
	m := o.m
	p := m.paths
	for _, path := range []string{p.WWW, p.ACMEWebroot, o.home(), o.snippetDir(), o.sitesDir(), o.errorDir(), p.Certs} {
		if !safePath(path) || path == "" {
			return nil, fmt.Errorf("refusing to render unsafe path %q", path)
		}
	}
	waf := wafEnabledOf(o)
	main, err := render(olsMainTmpl, struct{ Hook string }{o.hookFile()})
	if err != nil {
		return nil, err
	}
	hook, err := render(olsHookTmpl, olsHookData{
		Home: o.home(), SnippetDir: o.snippetDir(), SitesDir: o.sitesDir(), EmptyRoot: p.ACMEWebroot,
		DataDir: filepath.Dir(p.Certs), IPv6: ipv6Enabled(), WAF: waf, Hook: hookPort,
	})
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{
		o.mainConf():                            main,
		o.hookFile():                            hook,
		filepath.Join(o.errorDir(), "503.html"): []byte("This website is temporarily unavailable.\n"),
		olsLogrotate:                            []byte(olsLogrotateConf(o.logDir())),
	}
	if waf {
		sites, err := m.store.ListSites(ctx)
		if err != nil {
			return nil, err
		}
		for path, data := range o.wafFiles(sites) {
			files[path] = data
		}
	}
	return files, nil
}

// hookPort is the panel's Git webhook listener (gitdeploy.HookAddr).
const hookPort = "127.0.0.1:7809"

func olsLogrotateConf(dir string) string {
	return fmt.Sprintf(`# Managed by %s.
# OpenLiteSpeed keeps its log files open: copytruncate rotates them in place.
%s/*.log {
    daily
    rotate 14
    missingok
    notifempty
    compress
    delaycompress
    copytruncate
}
`, version.AppName, dir)
}

func (o olsServer) vhost(ctx context.Context, d vhostData, rewrite string) ([]byte, error) {
	// OpenLiteSpeed's per-client throttling delays requests instead of
	// refusing them: SetFirewall refuses a rate limit, and one set under
	// Nginx is not applied.
	d.RateLimit, d.RateBurst, d.RateZone = 0, 0, ""
	v := olsVhost{vhostData: d, ErrorDir: o.errorDir()}
	// The sites' own rewrite rules, for sites that serve their files.
	if !d.Disabled && d.RedirectURL == "" && d.ProxyPort == 0 && d.Public != "" {
		v.Htaccess = o.m.scanHtaccess(d.ID, d.Public)
	}
	return renderOLSVhost(v, rewrite)
}

func (o olsServer) customBytes(site *store.Site) []byte {
	return []byte("# Managed by " + version.AppName + ": edit in Websites → OpenLiteSpeed configuration.\n" +
		strings.TrimRight(site.NginxCustom, "\n") + "\n")
}

// test runs OpenLiteSpeed's configuration test. It exits with 1 when it only
// warns: the problems that count are olsTestProblems.
func (o olsServer) test(ctx context.Context) (string, error) {
	out, err := o.m.run.Output(ctx, o.bin(), "-t")
	if problems := olsTestProblems(out, o.m.paths.WWW, filepath.Join(o.snippetDir(), "custom")); len(problems) > 0 {
		return out, errors.New(strings.Join(problems, "\n"))
	}
	var exit *exec.ExitError
	if err != nil && !(errors.As(err, &exit) && exit.ExitCode() == 1) {
		return out, err
	}
	return out, nil
}

func (o olsServer) testError(out string) string {
	if p := olsTestProblems(out, o.m.paths.WWW, filepath.Join(o.snippetDir(), "custom")); len(p) > 0 {
		if len(p) > 10 {
			p = p[:10]
		}
		return strings.Join(p, "\n")
	}
	return run.Tail(out, 5)
}

var olsLogPrefixRe = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9:.]+ (\[[A-Z]+\]) (?:\[[0-9]+\] )?`)

// olsTestProblems returns the lines of "openlitespeed -t" that make a
// configuration unusable: errors, and the parser's warnings about
// unbalanced braces. Unknown directives only count in the administrator's
// own files (customDir): OpenLiteSpeed ignores them, and a key an update
// drops must not stop every change. A folder of a website that vanished
// (under www) is not the configuration's fault either.
func olsTestProblems(out, www, customDir string) []string {
	var keep []string
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimSpace(l)
		mm := olsLogPrefixRe.FindStringSubmatch(l)
		if mm == nil {
			continue
		}
		level, msg := mm[1], strings.TrimSpace(l[len(mm[0]):])
		switch {
		case level == "[ERROR]":
			if i := strings.Index(msg, "path is not accessible: "); i >= 0 &&
				strings.HasPrefix(msg[i+len("path is not accessible: "):], www+"/") {
				continue
			}
			// Rewrite rules: the sites' .htaccess rules may be wrong;
			// OpenLiteSpeed skips them while running.
			if strings.Contains(msg, "[REWRITE]") {
				continue
			}
		case level == "[WARN]":
			switch {
			case strings.Contains(msg, "do not match"), strings.Contains(msg, "found more '}'"):
			case strings.Contains(msg, "Not support [") && strings.Contains(msg, customDir+"/"):
			default:
				continue
			}
		default:
			continue
		}
		keep = append(keep, level+" "+msg)
	}
	return keep
}

// reload checks the configuration and restarts OpenLiteSpeed gracefully:
// its main process starts a new one with the new configuration, and the
// old workers finish their requests. lswsctrl (systemctl reload) waits at
// least three seconds; the panel signals the main process itself and waits
// until the new workers accept connections.
func (o olsServer) reload(ctx context.Context) error {
	if out, err := o.test(ctx); err != nil {
		return fmt.Errorf("openlitespeed configuration test failed:\n%s", o.testError(out))
	}
	pid := olsMainPID()
	if pid == 0 || !o.m.unitActive(ctx, olsUnit) {
		return o.m.systemctl(ctx, "reload", olsUnit)
	}
	// A graceful restart hands every listening socket to the new process,
	// also those the configuration no longer has (Adminer's, after its
	// removal): then only a restart closes them.
	if o.listenersStale(pid) {
		return o.restartTested(ctx)
	}
	before := olsWorkers(pid)
	if _, err := o.m.run.Output(ctx, "kill", "-USR1", strconv.Itoa(pid)); err != nil {
		return o.m.systemctl(ctx, "reload", olsUnit)
	}
	if !waitOLSRestart(pid, before, 10*time.Second) {
		o.m.log.Warn("openlitespeed reload: the new workers did not take over within 10 seconds")
	}
	o.cleanRCS()
	return nil
}

// cleanRCS deletes the copies OpenLiteSpeed makes of each configuration
// file it reads (<file>0 and its RCS history <file>0,v, for the WebAdmin
// console's history; the setting that turns the console off applies only
// after the files are read). They would keep old site configurations
// around, Adminer's token among them.
func (o olsServer) cleanRCS() {
	for _, dir := range []string{o.snippetDir(), o.sitesDir(), filepath.Join(o.snippetDir(), "custom")} {
		for _, pat := range []string{"*.conf0", "*.conf0,v"} {
			matches, _ := filepath.Glob(filepath.Join(dir, pat))
			for _, p := range matches {
				os.Remove(p)
			}
		}
	}
}

func (o olsServer) restart(ctx context.Context) error {
	if out, err := o.test(ctx); err != nil {
		return fmt.Errorf("openlitespeed configuration test failed:\n%s", o.testError(out))
	}
	return o.restartTested(ctx)
}

// restartTested restarts OpenLiteSpeed, whose configuration was checked.
// "systemctl restart" is not enough: its stop is graceful (lswsctrl
// delay-stop) and the new process takes the old one's sockets over, so a
// removed listener would stay open. The old processes are given 10 seconds
// to finish their requests.
func (o olsServer) restartTested(ctx context.Context) error {
	o.m.run.Output(ctx, "systemctl", "stop", olsUnit) // fine if it is not running
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && len(olsProcesses(func(cmd string, _ int) bool {
		return strings.HasPrefix(cmd, "openlitespeed (lshttpd")
	})) > 0 {
		time.Sleep(50 * time.Millisecond)
	}
	stopOLSProcesses()
	defer o.cleanRCS()
	if err := o.m.systemctl(ctx, "start", olsUnit); err != nil {
		return err
	}
	if !waitOLSRestart(0, nil, 10*time.Second) {
		o.m.log.Warn("openlitespeed restart: no worker accepts connections after 10 seconds")
	}
	return nil
}

var olsAdminerAddrRe = regexp.MustCompile(`(?m)^\s*address\s+(127\.0\.0\.1:[0-9]{2,5})\s*$`)

// listenersStale reports whether the main process listens on other TCP
// addresses than the configuration has: ports 80 and 443 on every address
// (IPv6 too when the server has it) and Adminer's, if installed.
func (o olsServer) listenersStale(main int) bool {
	any := "0.0.0.0"
	if ipv6Enabled() {
		any = "::"
	}
	want := map[string]bool{any + ":80": true, any + ":443": true}
	if b, err := os.ReadFile(filepath.Join(o.sitesDir(), "_adminer.conf")); err == nil {
		if mm := olsAdminerAddrRe.FindSubmatch(b); mm != nil {
			want[string(mm[1])] = true
		}
	}
	have := listenAddrs(main)
	if len(have) == 0 {
		return false // cannot tell
	}
	if len(have) != len(want) {
		return true
	}
	for a := range want {
		if !have[a] {
			return true
		}
	}
	return false
}

// listenAddrs returns the listening TCP addresses ("0.0.0.0:80", "::443",
// "127.0.0.1:46000") a process holds.
func listenAddrs(pid int) map[string]bool {
	inodes := map[string]string{}
	for _, name := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		b, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n") {
			f := strings.Fields(line)
			if len(f) > 9 && f[3] == "0A" {
				if a := procNetAddr(f[1]); a != "" {
					inodes[f[9]] = a
				}
			}
		}
	}
	out := map[string]bool{}
	dir := filepath.Join("/proc", strconv.Itoa(pid), "fd")
	fds, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	for _, fd := range fds {
		target, err := os.Readlink(filepath.Join(dir, fd.Name()))
		if err != nil {
			continue
		}
		if ino, ok := strings.CutPrefix(target, "socket:["); ok {
			if a, ok := inodes[strings.TrimSuffix(ino, "]")]; ok {
				out[a] = true
			}
		}
	}
	return out
}

// procNetAddr turns /proc/net/tcp's "0100007F:1F90" into "127.0.0.1:8080"
// (and an IPv6 address into its short form).
func procNetAddr(s string) string {
	host, port, ok := strings.Cut(s, ":")
	if !ok {
		return ""
	}
	p, err := strconv.ParseUint(port, 16, 16)
	if err != nil {
		return ""
	}
	raw, err := hex.DecodeString(host)
	if err != nil || (len(raw) != 4 && len(raw) != 16) {
		return ""
	}
	// The kernel prints each 32-bit word in host order (little endian).
	for i := 0; i+4 <= len(raw); i += 4 {
		raw[i], raw[i+1], raw[i+2], raw[i+3] = raw[i+3], raw[i+2], raw[i+1], raw[i]
	}
	return net.IP(raw).String() + ":" + strconv.FormatUint(p, 10)
}

// olsMainPID reads the main process from the PID file, if it is really
// OpenLiteSpeed's.
func olsMainPID() int {
	b, err := os.ReadFile(olsPIDFile)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 1 {
		return 0
	}
	cmd, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
	if err != nil || !bytes.HasPrefix(cmd, []byte("openlitespeed (lshttpd - main)")) {
		return 0
	}
	return pid
}

// olsWorkers returns the workers of a main process that hold a listening
// socket (all workers when main is 0).
func olsWorkers(main int) map[int]bool {
	out := map[int]bool{}
	listen := listeningInodes()
	for _, pid := range olsProcesses(func(cmd string, pid int) bool {
		return strings.HasPrefix(cmd, "openlitespeed (lshttpd - #") && (main == 0 || procParent(pid) == main)
	}) {
		if holdsSocket(pid, listen) {
			out[pid] = true
		}
	}
	return out
}

// waitOLSRestart waits until a new main process (other than old) has
// workers that accept connections and none of the workers in before still
// do, for at most timeout.
func waitOLSRestart(old int, before map[int]bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if pid := olsMainPID(); pid != 0 && pid != old {
			now := olsWorkers(pid)
			stale := false
			all := olsWorkers(0)
			for p := range before {
				if all[p] {
					stale = true
				}
			}
			if len(now) > 0 && !stale {
				return true
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	return false
}

// adminerConf is Adminer's internal listener and virtual host. The token is
// a secret: the file is readable by root only.
func (o olsServer) adminerConf(port, token, php string) []byte {
	if !adminerTokenRe.MatchString(token) || !adminerPortRe.MatchString(port) || validate.PHPVersion(php) != nil {
		return nil
	}
	out, err := render(olsAdminerTmpl, struct{ Port, Token, Dir, PHP string }{port, token, adminerDir, php})
	if err != nil {
		return nil
	}
	return out
}

// ---- web application firewall (ModSecurity 3 in ols-modsecurity) ----

func (o olsServer) modSecurity() string {
	return filepath.Join(o.home(), "modules", "mod_security.so")
}

func (o olsServer) wafAvailable() bool {
	return fileExists(o.modSecurity()) && fileExists(filepath.Join(wafCRSRules, "REQUEST-901-INITIALIZATION.conf")) &&
		fileExists(wafCRSSetup)
}

func (o olsServer) wafPackages() []string { return []string{"ols-modsecurity", "modsecurity-crs"} }

func (o olsServer) wafExtraFiles() []string { return nil }

// wafFiles renders the ModSecurity settings OpenLiteSpeed loads once. They
// leave the engine unset: each site sets it in its virtual host (a value
// set here would win over the site's).
func (o olsServer) wafFiles(sites []*store.Site) map[string][]byte {
	dir := wafDirOf(o)
	main := fmt.Sprintf(`# Managed by %[1]s. Do not edit: the panel rewrites this file.
# ModSecurity settings, then the OWASP Core Rule Set. Each site turns the
# engine on (or to detection only) in its virtual host.
SecRequestBodyAccess On
SecRule REQUEST_HEADERS:Content-Type "^(?:application(?:/soap\+|/)|text/)xml" \
    "id:200000,phase:1,t:none,t:lowercase,pass,nolog,ctl:requestBodyProcessor=XML"
SecRule REQUEST_HEADERS:Content-Type "^application/json" \
    "id:200001,phase:1,t:none,t:lowercase,pass,nolog,ctl:requestBodyProcessor=JSON"
# The Git webhook is signed and checked by the panel: never inspected.
SecRule REQUEST_URI "@rx ^/\.panel/deploy/[0-9]+$" \
    "id:200010,phase:1,t:none,pass,nolog,ctl:ruleEngine=Off"
# Only the first 13 MB of a request are inspected, so large uploads still
# work and memory use stays bounded.
SecRequestBodyLimit 13107200
SecRequestBodyNoFilesLimit 4194304
SecRequestBodyLimitAction ProcessPartial
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
SecDataDir /tmp/
SecArgumentSeparator &
SecCookieFormat 0
SecStatusEngine Off

# One JSON line per request the rules flagged, shown in the panel.
SecAuditEngine RelevantOnly
SecAuditLogRelevantStatus "^(?:5|4(?!04))"
SecAuditLogParts ABHZ
SecAuditLogType Serial
SecAuditLogFormat JSON
SecAuditLog %[2]s

%[3]s
Include %[4]s
%[6]sInclude %[5]s/*.conf
%[7]s`, version.AppName, wafLog(), wafExclusions(sites), wafCRSSetup, wafCRSRules,
		optionalInclude("/etc/modsecurity/crs/REQUEST-900-EXCLUSION-RULES-BEFORE-CRS.conf"),
		optionalInclude("/etc/modsecurity/crs/RESPONSE-999-EXCLUSION-RULES-AFTER-CRS.conf"))
	return map[string][]byte{filepath.Join(dir, "main.conf"): []byte(main)}
}

// ---- helpers ----

// wwwDataGID is the group of the web servers' workers.
func wwwDataGID() (int, bool) {
	g, err := user.LookupGroup("www-data")
	if err != nil {
		return 0, false
	}
	gid, err := strconv.Atoi(g.Gid)
	return gid, err == nil
}

// olsRelPath turns a path below the site's public folder into the URI of
// its context ("/a/b/"), or "" if a part has characters the configuration
// cannot carry.
func olsRelPath(rel string) string {
	if rel == "." || rel == "" {
		return "/"
	}
	rel = filepath.ToSlash(rel)
	for _, part := range strings.Split(rel, "/") {
		if part == "" || strings.HasPrefix(part, ".") || !olsPathPartRe.MatchString(part) {
			return ""
		}
	}
	return "/" + path.Clean(rel) + "/"
}

var olsPathPartRe = regexp.MustCompile(`^[A-Za-z0-9_~@+-][A-Za-z0-9._~@+-]{0,127}$`)
