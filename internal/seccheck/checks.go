package seccheck

import (
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/security"
)

// checks is the list, in the order they run and the UI shows them.
var checks = []check{
	{"ssh_root", checkSSHRoot},
	{"ssh_password", checkSSHPassword},
	{"ssh_port", checkSSHPort},
	{"firewall", checkFirewall},
	{"fail2ban", checkFail2ban},
	{"panel_https", checkPanelHTTPS},
	{"panel_2fa", checkPanel2FA},
	{"os_updates", checkOSUpdates},
	{"os_auto_updates", checkAutoUpdates},
	{"os_reboot", checkReboot},
	{"accounts_uid0", checkUID0},
	{"accounts_empty_password", checkEmptyPasswords},
	{"public_services", checkPublicServices},
	{"redis_password", checkRedisPassword},
	{"docker_ports", checkDockerPorts},
	{"site_https", checkSiteHTTPS},
	{"site_certs", checkSiteCerts},
	{"site_files", checkSiteFiles},
	{"site_permissions", checkSitePermissions},
	{"php_display_errors", checkPHPDisplayErrors},
	{"php_expose", checkPHPExpose},
	{"wordpress", checkWordPress},
}

// ---- SSH ----

func checkSSHRoot(c *runner) ([]Finding, bool) {
	s := c.f.SSH
	if s == nil {
		return nil, false
	}
	for _, l := range s.Logins {
		if l.User == "root" && l.Password {
			return []Finding{finding("ssh_root_password", "ssh_root_password", Critical, page("firewall", "tab", "ssh"))}, true
		}
	}
	return nil, true
}

func checkSSHPassword(c *runner) ([]Finding, bool) {
	s := c.f.SSH
	if s == nil {
		return nil, false
	}
	// root is the check above.
	var users []string
	for _, l := range s.Logins {
		if l.Password && l.User != "root" {
			users = append(users, l.User)
		}
	}
	if !s.PasswordAuth || len(users) == 0 {
		return nil, true
	}
	return []Finding{finding("ssh_password", "ssh_password", Warning, page("firewall", "tab", "ssh"), "users", list(users, 5))}, true
}

func checkSSHPort(c *runner) ([]Finding, bool) {
	s := c.f.SSH
	if s == nil {
		return nil, false
	}
	if s.Port != 22 {
		return nil, true
	}
	return []Finding{finding("ssh_port_22", "ssh_port_22", Info, page("firewall", "tab", "ssh"))}, true
}

// ---- firewall and fail2ban ----

func checkFirewall(c *runner) ([]Finding, bool) {
	fw := c.f.Firewall
	switch {
	case !fw.Installed:
		return []Finding{finding("firewall_missing", "firewall_missing", Warning, page("firewall"))}, true
	case !fw.Active:
		return []Finding{finding("firewall_off", "firewall_off", Warning, page("firewall"))}, true
	}
	return nil, true
}

func checkFail2ban(c *runner) ([]Finding, bool) {
	f := c.f.Fail2ban
	fix := page("firewall", "tab", "f2b")
	switch {
	case !f.Installed:
		return []Finding{finding("fail2ban_missing", "fail2ban_missing", Warning, fix)}, true
	case !f.Running:
		return []Finding{finding("fail2ban_stopped", "fail2ban_stopped", Warning, page("services"))}, true
	case c.f.SSH != nil && !slices.ContainsFunc(f.Jails, func(j security.Jail) bool { return j.Name == "sshd" }):
		return []Finding{finding("fail2ban_no_sshd", "fail2ban_no_sshd", Warning, fix)}, true
	}
	return nil, true
}

// ---- the panel ----

func checkPanelHTTPS(c *runner) ([]Finding, bool) {
	p := c.f.Panel
	switch {
	case !p.TLS && !p.Loopback:
		return []Finding{finding("panel_http", "panel_http", Critical, page("settings"))}, true
	case p.TLS && p.SelfSigned:
		return []Finding{finding("panel_self_signed", "panel_self_signed", Info, page("settings"))}, true
	}
	return nil, true
}

func checkPanel2FA(c *runner) ([]Finding, bool) {
	var users []string
	for _, u := range c.f.Panel.Users {
		if !u.TOTP {
			users = append(users, u.Name)
		}
	}
	if len(users) == 0 {
		return nil, len(c.f.Panel.Users) > 0
	}
	return []Finding{finding("panel_2fa", "panel_2fa", Critical, page("security"), "users", list(users, 5))}, true
}

// ---- updates ----

// staleUpdates is how old the last update check may be before the check
// says it does not know: the panel checks every few hours by itself.
const staleUpdates = 7 * 24 * time.Hour

func checkOSUpdates(c *runner) ([]Finding, bool) {
	u := c.f.Updates
	if u == nil {
		return nil, false
	}
	fix := page("system")
	if u.CheckedAt == nil || c.env.Now.Sub(*u.CheckedAt) > staleUpdates {
		return []Finding{finding("os_updates_unknown", "os_updates_unknown", Info, fix)}, true
	}
	if u.Security > 0 {
		return []Finding{finding("os_security_updates", "os_security_updates", Warning, fix, "n", itoa(u.Security))}, true
	}
	return nil, true
}

func checkAutoUpdates(c *runner) ([]Finding, bool) {
	u := c.f.Updates
	if u == nil {
		return nil, false
	}
	if u.Auto.Enabled {
		return nil, true
	}
	return []Finding{finding("os_auto_updates_off", "os_auto_updates_off", Warning, page("system"))}, true
}

func checkReboot(c *runner) ([]Finding, bool) {
	u := c.f.Updates
	if u == nil {
		return nil, false
	}
	if !u.Reboot.Required {
		return nil, true
	}
	return []Finding{finding("os_reboot", "os_reboot", Info, page("settings"))}, true
}

// ---- system accounts ----

func checkUID0(c *runner) ([]Finding, bool) {
	b, err := os.ReadFile(c.env.Passwd)
	if err != nil {
		return nil, false
	}
	var names []string
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Split(strings.TrimSpace(line), ":")
		if len(f) >= 4 && f[0] != "root" && f[0] != "" && !strings.HasPrefix(f[0], "#") && f[2] == "0" {
			names = append(names, f[0])
		}
	}
	if len(names) == 0 {
		return nil, true
	}
	return []Finding{finding("uid0", "uid0", Critical, page("terminal"), "users", list(names, 5))}, true
}

func checkEmptyPasswords(c *runner) ([]Finding, bool) {
	b, err := os.ReadFile(c.env.Shadow)
	if err != nil {
		return nil, false
	}
	var names []string
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Split(strings.TrimSpace(line), ":")
		// "" is an empty password: anyone may sign in as this account where
		// a program allows it (su, a console, sshd with PermitEmptyPasswords).
		if len(f) >= 2 && f[0] != "" && !strings.HasPrefix(f[0], "#") && f[1] == "" {
			names = append(names, f[0])
		}
	}
	if len(names) == 0 {
		return nil, true
	}
	return []Finding{finding("empty_password", "empty_password", Critical, page("terminal"), "users", list(names, 5))}, true
}

// ---- network services ----

// services are the programs that must not be reachable from the internet,
// by the process name ss shows, with their name and the panel page that
// manages them.
var services = map[string]struct{ name, page, tab string }{
	"mariadbd":      {"MariaDB", "databases", ""},
	"mysqld":        {"MySQL", "databases", ""},
	"postgres":      {"PostgreSQL", "databases", "postgres"},
	"redis-server":  {"Redis", "databases", "redis"},
	"valkey-server": {"Valkey", "databases", "redis"},
	"memcached":     {"Memcached", "databases", "memcached"},
	"mongod":        {"MongoDB", "docker", ""},
}

var privateNets = []netip.Prefix{
	netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("172.16.0.0/12"), netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("fc00::/7"), netip.MustParsePrefix("fe80::/10"),
}

// reach says how far an address reaches: "local" (loopback), "private" (a
// private network) or "public" (every address, or a public one).
func reach(addr string) string {
	host, _, _ := strings.Cut(addr, "%")
	if host == "" || host == "*" || host == "0.0.0.0" || host == "::" {
		return "public"
	}
	a, err := netip.ParseAddr(host)
	if err != nil {
		return "public"
	}
	a = a.Unmap()
	if a.IsLoopback() {
		return "local"
	}
	for _, p := range privateNets {
		if p.Contains(a) {
			return "private"
		}
	}
	return "public"
}

func checkPublicServices(c *runner) ([]Finding, bool) {
	if c.f.Ports == nil {
		return nil, false
	}
	type key struct {
		proc string
		port int
		udp  bool
	}
	seen := map[key][]string{}
	var order []key
	for _, p := range c.f.Ports {
		if _, ok := services[p.Process]; !ok || reach(p.Address) == "local" {
			continue
		}
		k := key{p.Process, p.Port, p.Proto == "udp"}
		if _, ok := seen[k]; !ok {
			order = append(order, k)
		}
		if a := p.Address; !slices.Contains(seen[k], a) {
			seen[k] = append(seen[k], a)
		}
	}
	var out []Finding
	for _, k := range order {
		svc := services[k.proc]
		public := false
		for _, a := range seen[k] {
			public = public || reach(a) == "public"
		}
		exposure := "private"
		if public {
			exposure = c.f.Firewall.Exposure(k.port)
		}
		sev := Info
		if exposure == security.ExposureOpen {
			sev = Critical
		}
		proto := "tcp"
		if k.udp {
			proto = "udp"
		}
		fix := page(svc.page)
		if svc.tab != "" {
			fix = page(svc.page, "tab", svc.tab)
		}
		id := "service_public:" + k.proc + ":" + strconv.Itoa(k.port) + "/" + proto
		out = append(out, finding(id, "service_public", sev, fix, "service", svc.name, "port", strconv.Itoa(k.port)+"/"+proto,
			"address", strings.Join(seen[k], ", "), "exposure", exposure))
	}
	return out, true
}

func checkRedisPassword(c *runner) ([]Finding, bool) {
	listening := []string{}
	for _, p := range c.f.Ports {
		if p.Process == "redis-server" || p.Process == "valkey-server" {
			listening = append(listening, p.Address)
		}
	}
	b, err := os.ReadFile(c.env.RedisConf)
	if err != nil && len(listening) == 0 {
		return nil, false // no Redis
	}
	if redisPassword(string(b)) {
		return nil, true
	}
	where, sev := "local", Info
	port := 6379
	for _, p := range c.f.Ports {
		if (p.Process == "redis-server" || p.Process == "valkey-server") && reach(p.Address) == "public" {
			where, sev, port = "public", Warning, p.Port
			if c.f.Firewall.Exposure(port) == security.ExposureOpen {
				sev = Critical
			}
			break
		}
	}
	return []Finding{finding("redis_no_password", "redis_no_password", sev, page("databases", "tab", "redis"), "where", where)}, true
}

// redisPassword reports whether a redis.conf sets a password: requirepass,
// or a user line for the default user with a password (">…" or "#…").
func redisPassword(conf string) bool {
	pw := false
	for _, line := range strings.Split(conf, "\n") {
		f := strings.Fields(strings.TrimSpace(line))
		if len(f) < 2 || strings.HasPrefix(f[0], "#") {
			continue
		}
		switch strings.ToLower(f[0]) {
		case "requirepass":
			v := strings.Trim(f[1], `"'`)
			pw = v != ""
		case "user":
			if f[1] == "default" {
				for _, r := range f[2:] {
					if strings.HasPrefix(r, ">") || strings.HasPrefix(r, "#") {
						pw = true
					}
					if r == "nopass" {
						pw = false
					}
				}
			}
		}
	}
	return pw
}

func checkDockerPorts(c *runner) ([]Finding, bool) {
	if c.f.Docker == nil {
		return nil, false
	}
	var out []Finding
	for _, ct := range c.f.Docker {
		if ct.State != "" && ct.State != "running" {
			continue
		}
		ports := publishedPorts(ct.Ports)
		if len(ports) == 0 {
			continue
		}
		out = append(out, finding("docker_public:"+ct.Name, "docker_public", Warning, page("docker"),
			"container", ct.Name, "ports", strings.Join(ports, ", ")))
	}
	return out, true
}

// publishedPorts reads the host ports Docker publishes on every address
// from "docker ps" Ports: "0.0.0.0:8080->80/tcp, [::]:8080->80/tcp,
// 127.0.0.1:5432->5432/tcp, 9000/tcp". Ports on one address (127.0.0.1)
// and ports only exposed to other containers are left out.
func publishedPorts(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		host, _, ok := strings.Cut(strings.TrimSpace(part), "->")
		if !ok {
			continue
		}
		i := strings.LastIndexByte(host, ':')
		if i < 0 {
			continue
		}
		addr, port := strings.Trim(host[:i], "[]"), host[i+1:]
		if addr != "0.0.0.0" && addr != "::" && addr != "" {
			continue
		}
		if !slices.Contains(out, port) {
			out = append(out, port)
		}
	}
	sort.Strings(out)
	return out
}

// ---- websites ----

func siteFix(s Site, tab string) *Fix {
	return page("websites", "site", strconv.FormatInt(s.ID, 10), "tab", tab)
}

func checkSiteHTTPS(c *runner) ([]Finding, bool) {
	if len(c.f.Sites) == 0 {
		return nil, false
	}
	var out []Finding
	for _, s := range c.f.Sites {
		if s.Disabled || s.Redirect {
			continue
		}
		switch {
		case !s.SSL:
			out = append(out, finding("site_no_https:"+s.Domain, "site_no_https", Warning, siteFix(s, "ssl"), "domain", s.Domain))
		case !s.ForceHTTPS:
			out = append(out, finding("site_no_redirect:"+s.Domain, "site_no_redirect", Info, siteFix(s, "ssl"), "domain", s.Domain))
		}
	}
	return out, true
}

// expiringDays is when a certificate's coming expiry becomes a warning:
// the panel renews Let's Encrypt certificates 30 days before they expire,
// so one this close has failed to renew for two weeks.
const expiringDays = 14

func checkSiteCerts(c *runner) ([]Finding, bool) {
	has := false
	var out []Finding
	for _, s := range c.f.Sites {
		if !s.SSL || s.CertExpiry.IsZero() {
			continue
		}
		has = true
		left := s.CertExpiry.Sub(c.env.Now)
		days := int(left.Hours() / 24)
		custom := strconv.FormatBool(s.CustomCert)
		switch {
		case left < 0:
			out = append(out, finding("site_cert_expired:"+s.Domain, "site_cert_expired", Critical, siteFix(s, "ssl"), "domain", s.Domain, "custom", custom))
		case days <= expiringDays:
			f := finding("site_cert_expiring:"+s.Domain, "site_cert_expiring", Warning, siteFix(s, "ssl"), "domain", s.Domain, "custom", custom)
			f.Extra = map[string]string{"days": itoa(days)}
			out = append(out, f)
		}
	}
	return out, has
}

// fileKind says why a file in a web root must not be served: "secret"
// (credentials, source code, database dumps) or "backup" (archives and
// copies that may hold them), or "" for a file that may be served.
func fileKind(name, domain string, dir bool) string {
	lower := strings.ToLower(name)
	if dir {
		if lower == ".git" || lower == ".svn" || lower == ".hg" {
			return "secret"
		}
		return ""
	}
	switch {
	case lower == ".env" || strings.HasPrefix(lower, ".env."), lower == ".htpasswd", lower == ".git-credentials",
		lower == "id_rsa", lower == "id_ed25519", lower == ".my.cnf", lower == ".pgpass":
		return "secret"
	case strings.HasPrefix(lower, "wp-config") && lower != "wp-config.php" && lower != "wp-config-sample.php":
		return "secret" // wp-config.php.bak, wp-config.old…: PHP source served as text
	case lower == "phpinfo.php" || lower == "info.php":
		return "backup"
	}
	for _, ext := range []string{".sql", ".sql.gz", ".sql.bz2", ".sql.xz", ".sql.zip", ".dump", ".sqlite", ".sqlite3", ".db"} {
		if strings.HasSuffix(lower, ext) {
			return "secret"
		}
	}
	for _, ext := range []string{".bak", ".old", ".orig", ".save", ".swp", ".php~", ".php.txt"} {
		if strings.HasSuffix(lower, ext) {
			return "backup"
		}
	}
	if strings.HasSuffix(lower, "~") {
		return "backup"
	}
	// Archives only when their name looks like a copy of the site: a zip
	// file to download is normal.
	for _, ext := range []string{".zip", ".tar", ".tar.gz", ".tgz", ".tar.bz2", ".tar.xz", ".tar.zst", ".rar", ".7z", ".gz"} {
		if strings.HasSuffix(lower, ext) {
			label, _, _ := strings.Cut(strings.ToLower(domain), ".")
			for _, w := range []string{"backup", "bak", "dump", "database", "public_html", "htdocs", "www", "site", "wordpress", label} {
				if w != "" && strings.Contains(lower, w) {
					return "backup"
				}
			}
			return ""
		}
	}
	return ""
}

func checkSiteFiles(c *runner) ([]Finding, bool) {
	if len(c.f.Sites) == 0 || c.env.Probe == nil {
		return nil, false
	}
	probes := 0
	var out []Finding
	for _, s := range c.f.Sites {
		if s.Disabled || s.Redirect || s.Proxy || s.Password {
			continue
		}
		public := filepath.Join(s.Dir, "public")
		entries, err := readDirN(public, c.env.ListLimit)
		if err != nil {
			continue
		}
		var found []string
		secret := false
		for _, e := range entries {
			kind := fileKind(e.Name(), s.Domain, e.IsDir())
			if kind == "" || e.Type()&os.ModeSymlink != 0 {
				continue
			}
			path := "/" + e.Name()
			if e.IsDir() {
				// A repository: its HEAD (or entries) file answers for it.
				path = ""
				for _, f := range []string{"HEAD", "config", "entries", "wc.db"} {
					if fi, err := os.Lstat(filepath.Join(public, e.Name(), f)); err == nil && fi.Mode().IsRegular() {
						path = "/" + e.Name() + "/" + f
						break
					}
				}
				if path == "" {
					continue
				}
			}
			if probes >= c.env.ProbeLimit || c.ctx.Err() != nil {
				c.partial = append(c.partial, "site_files")
				break
			}
			probes++
			code, err := c.env.Probe(c.ctx, s.Domain, path)
			if err != nil || (code != 200 && code != 206) {
				continue
			}
			found = append(found, path)
			secret = secret || kind == "secret"
		}
		if len(found) == 0 {
			continue
		}
		sev := Warning
		if secret {
			sev = Critical
		}
		f := finding("site_exposed:"+s.Domain, "site_exposed", sev, page("files", "path", public), "domain", s.Domain, "files", list(found, 5))
		out = append(out, f)
	}
	return out, true
}

// readDirN reads at most n entries of a folder, sorted by name.
func readDirN(dir string, n int) ([]os.DirEntry, error) {
	d, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	entries, err := d.ReadDir(n)
	if err != nil && len(entries) == 0 {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, nil
}

// unixPerms is false on Windows, where file modes are not Unix
// permissions (the tests run there too).
var unixPerms = runtime.GOOS != "windows"

func checkSitePermissions(c *runner) ([]Finding, bool) {
	if len(c.f.Sites) == 0 || !unixPerms {
		return nil, false
	}
	total := 0
	var out []Finding
	for _, s := range c.f.Sites {
		if total >= c.env.WalkTotal || c.ctx.Err() != nil {
			c.partial = append(c.partial, "site_permissions")
			break
		}
		limit := min(c.env.WalkLimit, c.env.WalkTotal-total)
		count, examples, walked, done := worldWritable(c, s.Dir, limit)
		total += walked
		if !done {
			c.partial = append(c.partial, "site_permissions")
		}
		if count == 0 {
			continue
		}
		f := finding("site_world_writable:"+s.Domain, "site_world_writable", Warning, page("files", "path", s.Dir), "domain", s.Domain, "count", itoa(count))
		f.Extra = map[string]string{"examples": list(examples, 3)}
		out = append(out, f)
	}
	return out, true
}

// worldWritable walks dir (symbolic links are not followed) for files and
// folders anyone on the server may change. A folder with the sticky bit,
// such as a shared tmp, is fine. It stops after limit entries.
func worldWritable(c *runner, dir string, limit int) (count int, examples []string, walked int, done bool) {
	done = true
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		walked++
		if walked > limit || (walked%512 == 0 && c.ctx.Err() != nil) {
			done = false
			return filepath.SkipAll
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		m := fi.Mode()
		if m.Perm()&0o002 != 0 && !(m.IsDir() && m&os.ModeSticky != 0) && (m.IsRegular() || m.IsDir()) {
			count++
			if len(examples) < 3 {
				rel, _ := filepath.Rel(dir, path)
				examples = append(examples, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	return count, examples, walked, done
}

// ---- PHP ----

func checkPHPDisplayErrors(c *runner) ([]Finding, bool) {
	vers := phpVersions(c.env.PHPDir)
	if len(vers) == 0 {
		return nil, false
	}
	var out []Finding
	for _, v := range vers {
		ini := phpIni(c.env.PHPDir, v)
		pools := poolsWith(c.env.PHPDir, v, "display_errors")
		if !iniOn(ini["display_errors"], true) && len(pools) == 0 {
			continue
		}
		scope := "all"
		if !iniOn(ini["display_errors"], true) {
			scope = list(pools, 5)
		}
		out = append(out, finding("php_display_errors:"+v, "php_display_errors", Warning, page("software", "php", v), "version", v, "scope", scope))
	}
	return out, true
}

func checkPHPExpose(c *runner) ([]Finding, bool) {
	vers := phpVersions(c.env.PHPDir)
	if len(vers) == 0 {
		return nil, false
	}
	var out []Finding
	for _, v := range vers {
		// PHP's own default for expose_php is On.
		val, set := phpIni(c.env.PHPDir, v)["expose_php"]
		if set && !iniOn(val, false) {
			continue
		}
		out = append(out, finding("php_expose:"+v, "php_expose", Info, page("software", "php", v), "version", v))
	}
	return out, true
}

// phpVersions lists the PHP versions with PHP-FPM under dir (/etc/php).
func phpVersions(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		v := e.Name()
		if a, b, ok := strings.Cut(v, "."); ok && isNum(a) && isNum(b) {
			if _, err := os.Stat(filepath.Join(dir, v, "fpm", "php.ini")); err == nil {
				out = append(out, v)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return versionLess(out[i], out[j]) })
	return out
}

func isNum(s string) bool {
	_, err := strconv.Atoi(s)
	return s != "" && err == nil
}

func versionLess(a, b string) bool {
	a1, a2, _ := strings.Cut(a, ".")
	b1, b2, _ := strings.Cut(b, ".")
	x, _ := strconv.Atoi(a1)
	y, _ := strconv.Atoi(b1)
	if x != y {
		return x < y
	}
	x, _ = strconv.Atoi(a2)
	y, _ = strconv.Atoi(b2)
	return x < y
}

// phpIni reads the values PHP-FPM of version v uses: php.ini, then the
// files of conf.d in name order, the last value winning (as PHP reads
// them). Only display_errors and expose_php are kept.
func phpIni(dir, v string) map[string]string {
	out := map[string]string{}
	files := []string{filepath.Join(dir, v, "fpm", "php.ini")}
	if more, err := filepath.Glob(filepath.Join(dir, v, "fpm", "conf.d", "*.ini")); err == nil {
		sort.Strings(more)
		files = append(files, more...)
	}
	for _, f := range files {
		b, err := readSmall(f, 1<<20)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n") {
			k, val, ok := strings.Cut(line, "=")
			k = strings.TrimSpace(k)
			if !ok || k == "" || strings.HasPrefix(k, ";") || strings.HasPrefix(k, "[") {
				continue
			}
			if k == "display_errors" || k == "expose_php" {
				val, _, _ = strings.Cut(val, ";")
				out[k] = strings.Trim(strings.TrimSpace(val), `"'`)
			}
		}
	}
	return out
}

// poolsWith lists the PHP-FPM pools of version v that turn an ini flag on
// (php_flag[name], php_admin_flag[name], php_value… = on): the panel's
// pools are named panel-<user>.conf, and the user is listed.
func poolsWith(dir, v, name string) []string {
	files, _ := filepath.Glob(filepath.Join(dir, v, "fpm", "pool.d", "*.conf"))
	sort.Strings(files)
	var out []string
	for _, f := range files {
		b, err := readSmall(f, 1<<20)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n") {
			k, val, ok := strings.Cut(line, "=")
			k = strings.ReplaceAll(strings.TrimSpace(k), " ", "")
			if !ok || strings.HasPrefix(k, ";") {
				continue
			}
			for _, prefix := range []string{"php_flag", "php_admin_flag", "php_value", "php_admin_value"} {
				if k == prefix+"["+name+"]" && iniOn(strings.Trim(strings.TrimSpace(val), `"'`), true) {
					pool := strings.TrimSuffix(filepath.Base(f), ".conf")
					out = append(out, strings.TrimPrefix(pool, "panel-"))
				}
			}
		}
	}
	return slices.Compact(out)
}

// iniOn reads a php.ini boolean. For display_errors, "stdout" also shows
// errors to visitors, while "stderr" sends them to PHP-FPM's log.
func iniOn(v string, display bool) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "on", "true", "yes":
		return true
	case "stdout":
		return display
	}
	return false
}

func readSmall(path string, max int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b := make([]byte, 0, 4096)
	buf := make([]byte, 32<<10)
	for int64(len(b)) < max {
		n, err := f.Read(buf)
		b = append(b, buf[:n]...)
		if err != nil {
			break
		}
	}
	return b, nil
}

// ---- WordPress ----

func checkWordPress(c *runner) ([]Finding, bool) {
	if len(c.f.WordPress) == 0 {
		return nil, false
	}
	ids := map[string]Site{}
	for _, s := range c.f.Sites {
		ids[s.Domain] = s
	}
	domains := make([]string, 0, len(c.f.WordPress))
	for d := range c.f.WordPress {
		domains = append(domains, d)
	}
	sort.Strings(domains)
	var out []Finding
	for _, d := range domains {
		w := c.f.WordPress[d]
		s, ok := ids[d]
		if !ok {
			continue // a site deleted since its check
		}
		fix := siteFix(s, "wordpress")
		checked := map[string]string{"checked": w.CheckedAt.UTC().Format(time.RFC3339)}
		if w.CoreUpdate != "" || w.PluginUpdates > 0 || w.ThemeUpdates > 0 {
			f := finding("wp_updates:"+d, "wp_updates", Warning, fix, "domain", d, "core", w.CoreUpdate,
				"plugins", itoa(w.PluginUpdates), "themes", itoa(w.ThemeUpdates))
			f.Extra = checked
			out = append(out, f)
		}
		if w.Debug {
			f := finding("wp_debug:"+d, "wp_debug", Warning, fix, "domain", d)
			f.Extra = checked
			out = append(out, f)
		}
	}
	return out, true
}
