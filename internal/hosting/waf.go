package hosting

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/version"
)

// The web application firewall is ModSecurity with the OWASP Core Rule Set,
// both from the distribution: the Nginx module (ModSecurity 3) or Apache's
// mod_security2. The rules are loaded once, with the engine off; each site
// turns it on (and may stay in detection-only mode) in its own server block.
// Rate limits (Nginx only) and blocked addresses are plain web server
// directives and work without ModSecurity.

const (
	wafModuleConf = "/etc/nginx/modules-enabled/50-mod-http-modsecurity.conf"
	wafCRSRules   = "/usr/share/modsecurity-crs/rules"
	wafCRSSetup   = "/etc/modsecurity/crs/crs-setup.conf"
	wafUnicodeMap = "/etc/nginx/unicode.mapping"
	wafLogDir     = "/var/log/panel-waf"
	wafLogrotate  = "/etc/logrotate.d/panel-waf"

	// WAF modes of a site.
	WAFOff    = ""
	WAFDetect = "detect"
	WAFBlock  = "block"

	maxRateLimit  = 1000
	maxBlockedIPs = 200
)

func wafLog() string { return filepath.Join(wafLogDir, "audit.log") }

func wafDirOf(ws webServer) string { return filepath.Join(ws.snippetDir(), "waf") }

func (m *Manager) wafDir() string { return wafDirOf(m.web()) }

// wafEnabled reports whether the panel manages the firewall configuration:
// the packages are installed and InstallWAF created its folder.
func (m *Manager) wafEnabled() bool { return wafEnabledOf(m.web()) }

func wafEnabledOf(ws webServer) bool {
	st, err := os.Stat(wafDirOf(ws))
	return ws.wafAvailable() && err == nil && st.IsDir()
}

// aptLists finds apt's package lists (a variable for the tests).
var aptLists = "/var/lib/apt/lists/*_Packages*"

// WAFPackaged reports whether apt can install the web firewall for the web
// server kind (ServerNginx, ServerApache, ServerOLS): Ubuntu 22.04 has no
// ModSecurity module for its Nginx, for one. When apt can't tell (its lists
// were never downloaded) it says yes, and an install reports the real error.
// A definite answer is kept: the distribution's packages don't come and go.
func (m *Manager) WAFPackaged(ctx context.Context, kind string) bool {
	if v, ok := m.wafPackaged.Load(kind); ok {
		return v.(bool)
	}
	if lists, _ := filepath.Glob(aptLists); len(lists) == 0 {
		return true
	}
	for _, pkg := range m.server(kind).wafPackages() {
		out, err := m.run.Output(ctx, "apt-cache", "policy", pkg)
		if err != nil {
			return true
		}
		// A package apt doesn't know prints nothing; one it can't install
		// says "Candidate: (none)".
		if !strings.Contains(out, "Candidate:") || strings.Contains(out, "Candidate: (none)") {
			m.wafPackaged.Store(kind, false)
			return false
		}
	}
	m.wafPackaged.Store(kind, true)
	return true
}

// ServerTitle is the web server's name as people write it ("Nginx").
func (m *Manager) ServerTitle(kind string) string { return m.server(kind).title() }

// WAFInstalled reports whether the firewall is installed and configured.
func (m *Manager) WAFInstalled() bool {
	return m.wafEnabled() && fileExists(filepath.Join(m.wafDir(), "main.conf"))
}

// wafFiles renders the managed web server's shared ModSecurity files.
func (m *Manager) wafFiles(sites []*store.Site) map[string][]byte { return m.web().wafFiles(sites) }

// wafExclusions are the rules that turn on the rule set's exclusions for
// sites running WordPress or Drupal, selected by host name before the rules
// run.
func wafExclusions(sites []*store.Site) string {
	var wp, drupal []string
	for _, s := range sites {
		names := s.Names()
		if s.App == AppWordPress || s.Rewrite == "wordpress" {
			wp = append(wp, names...)
		}
		if s.Rewrite == "drupal" {
			drupal = append(drupal, names...)
		}
	}
	var excl strings.Builder
	for _, e := range []struct {
		id    int
		names []string
		tx    string
	}{{9011001, wp, "crs_exclusions_wordpress"}, {9011002, drupal, "crs_exclusions_drupal"}} {
		if len(e.names) > 0 {
			fmt.Fprintf(&excl, "SecRule REQUEST_HEADERS:Host \"@rx ^(?:%s)(?::[0-9]+)?$\" \\\n    \"id:%d,phase:1,pass,nolog,t:none,t:lowercase,setvar:tx.%s=1\"\n",
				hostRegex(e.names), e.id, e.tx)
		}
	}
	return excl.String()
}

// nginxWAFFiles renders Nginx's shared ModSecurity configuration.
func (m *Manager) nginxWAFFiles(sites []*store.Site) map[string][]byte {
	excl := wafExclusions(sites)
	wafDir := wafDirOf(m.server(ServerNginx))

	main := fmt.Sprintf(`# Managed by %[1]s. Do not edit: the panel rewrites this file.
# ModSecurity settings, then the OWASP Core Rule Set.
SecRuleEngine On
SecRequestBodyAccess On
SecRule REQUEST_HEADERS:Content-Type "^(?:application(?:/soap\+|/)|text/)xml" \
    "id:200000,phase:1,t:none,t:lowercase,pass,nolog,ctl:requestBodyProcessor=XML"
SecRule REQUEST_HEADERS:Content-Type "^application/json" \
    "id:200001,phase:1,t:none,t:lowercase,pass,nolog,ctl:requestBodyProcessor=JSON"
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
SecUnicodeMapFile %[2]s 20127
SecStatusEngine Off

# One JSON line per request the rules flagged, shown in the panel.
SecAuditEngine RelevantOnly
SecAuditLogRelevantStatus "^(?:5|4(?!04))"
SecAuditLogParts ABHZ
SecAuditLogType Serial
SecAuditLogFormat JSON
SecAuditLog %[3]s

%[4]s
Include %[5]s
%[7]sInclude %[6]s/*.conf
%[8]s`, version.AppName, wafUnicodeMap, wafLog(), excl, wafCRSSetup, wafCRSRules,
		optionalInclude("/etc/modsecurity/crs/REQUEST-900-EXCLUSION-RULES-BEFORE-CRS.conf"),
		optionalInclude("/etc/modsecurity/crs/RESPONSE-999-EXCLUSION-RULES-AFTER-CRS.conf"))

	http := fmt.Sprintf(`# Managed by %s. Do not edit: the panel rewrites this file.
# The rules are loaded once here; each site turns the engine on.
modsecurity off;
modsecurity_rules_file %s;
`, version.AppName, filepath.Join(wafDir, "main.conf"))

	return map[string][]byte{
		filepath.Join(wafDir, "main.conf"): []byte(main),
		filepath.Join(wafDir, "http.conf"): []byte(http),
	}
}

// optionalInclude includes a rules file if it exists: ModSecurity 3 has no
// IncludeOptional.
func optionalInclude(path string) string {
	if !fileExists(path) {
		return ""
	}
	return "Include " + path + "\n"
}

// hostRegex matches any of names; "*.example.com" matches one label.
func hostRegex(names []string) string {
	parts := make([]string, 0, len(names))
	for _, n := range names {
		if rest, ok := strings.CutPrefix(n, "*."); ok {
			parts = append(parts, `[a-z0-9-]+\.`+regexp.QuoteMeta(rest))
		} else {
			parts = append(parts, regexp.QuoteMeta(n))
		}
	}
	return strings.Join(parts, "|")
}

const wafLogrotateConf = `# Managed by the panel.
/var/log/panel-waf/audit.log {
    daily
    rotate 7
    maxsize 50M
    missingok
    notifempty
    compress
    delaycompress
    copytruncate
}
`

// InstallWAF installs ModSecurity and the OWASP Core Rule Set and loads
// them into the web server. Sites keep their firewall off until it is
// turned on.
func (m *Manager) InstallWAF(ctx context.Context, lg Logger) error {
	if !m.Supported() {
		return ErrUnsupported
	}
	ws := m.web()
	if !fileExists(ws.hookFile()) {
		return ErrNotReady
	}
	lg.Logf("Installing ModSecurity and the OWASP Core Rule Set")
	if err := run.Apt(ctx, m.run, lg, append([]string{"install", "-y", "--no-install-recommends"}, ws.wafPackages()...)...); err != nil {
		return err
	}
	if !ws.wafAvailable() {
		return errors.New("the ModSecurity packages did not install the expected files")
	}
	if err := mkdir(wafLogDir, 0o750); err != nil {
		return err
	}
	if err := writeFile(wafLogrotate, []byte(wafLogrotateConf), 0o644); err != nil {
		return err
	}
	if err := mkdir(m.wafDir(), 0o755); err != nil {
		return err
	}
	lg.Logf("Loading the rules into %s", ws.title())
	if _, err := m.Refresh(ctx); err != nil {
		return err
	}
	if !m.WAFInstalled() {
		return errors.New("the firewall configuration could not be written")
	}
	lg.Logf("The firewall is ready. Turn it on for a website in Websites → Settings → Web firewall (WAF).")
	return nil
}

// RemoveWAF unloads the firewall from the web server and removes its
// packages. Sites keep their chosen mode, which applies again if it is
// reinstalled; rate limits and blocked addresses keep working.
func (m *Manager) RemoveWAF(ctx context.Context, lg Logger) error {
	if !m.Supported() {
		return ErrUnsupported
	}
	ws := m.web()
	lg.Logf("Unloading the firewall rules from %s", ws.title())
	if err := os.RemoveAll(m.wafDir()); err != nil {
		return err
	}
	for _, f := range ws.wafExtraFiles() {
		if err := removeIfExists(f); err != nil {
			return err
		}
	}
	if _, err := m.Refresh(ctx); err != nil {
		return err
	}
	lg.Logf("Removing ModSecurity and the OWASP Core Rule Set")
	if err := run.Apt(ctx, m.run, lg, append([]string{"purge", "-y"}, ws.wafPackages()...)...); err != nil {
		return err
	}
	removeIfExists(wafLogrotate)
	// A reload keeps loaded modules (and their memory): restart the web
	// server to let the module go, after checking the configuration
	// without it.
	if out, err := ws.test(ctx); err != nil {
		return fmt.Errorf("%s configuration test failed:\n%s", strings.ToLower(ws.title()), run.Tail(out, 10))
	}
	lg.Logf("Restarting %s to free the module's memory", ws.title())
	if err := ws.restart(ctx); err != nil {
		return err
	}
	lg.Logf("The firewall was removed. Its log stays in %s.", wafLogDir)
	return nil
}

// Firewall are a site's firewall settings.
type Firewall struct {
	WAF        string   `json:"waf"`        // "", detect or block
	RateLimit  int      `json:"rate_limit"` // PHP or app requests per second per IP; 0 is off
	BlockedIPs []string `json:"blocked_ips"`
}

// normalizeIPs checks addresses and ranges and returns them in canonical
// form, without duplicates.
func normalizeIPs(in []string) ([]string, error) {
	out := []string{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		var v string
		if strings.Contains(s, "/") {
			_, n, err := net.ParseCIDR(s)
			if err != nil {
				return nil, usererr.New("invalid_ip", s)
			}
			v = n.String()
		} else {
			ip := net.ParseIP(s)
			if ip == nil {
				return nil, usererr.New("invalid_ip", s)
			}
			v = ip.String()
		}
		if !slices.Contains(out, v) {
			out = append(out, v)
		}
	}
	if len(out) > maxBlockedIPs {
		return nil, usererr.New("too_many_ips")
	}
	return out, nil
}

// SetFirewall changes a site's firewall settings.
func (m *Manager) SetFirewall(ctx context.Context, id int64, f Firewall) (*store.Site, error) {
	if !m.Supported() {
		return nil, ErrUnsupported
	}
	if !slices.Contains([]string{WAFOff, WAFDetect, WAFBlock}, f.WAF) {
		return nil, usererr.New("invalid_waf_mode")
	}
	if f.WAF != WAFOff && !m.WAFInstalled() {
		return nil, usererr.New("waf_not_installed")
	}
	if f.RateLimit < 0 || f.RateLimit > maxRateLimit {
		return nil, usererr.New("invalid_rate_limit")
	}
	// Apache has no light equivalent of Nginx's limit_req, and
	// OpenLiteSpeed's per-client throttling delays requests instead of
	// refusing them (see ols.go).
	if k := m.serverKind(); f.RateLimit > 0 && (k == ServerApache || k == ServerOLS) {
		return nil, usererr.New("rate_limit_unsupported", m.web().title())
	}
	ips, err := normalizeIPs(f.BlockedIPs)
	if err != nil {
		return nil, err
	}
	if err := m.tryLock(); err != nil {
		return nil, err
	}
	defer m.ops.Unlock()
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return nil, err
	}
	old := *site
	site.WAF, site.RateLimit, site.BlockedIPs = f.WAF, f.RateLimit, strings.Join(ips, " ")
	if err := m.updateSite(ctx, site, &old); err != nil {
		return nil, err
	}
	return site, nil
}

// writeWAFConf refreshes the shared ModSecurity file (its app exclusions
// depend on the sites), with changed in place of its stored copy. The caller
// reloads the web server.
func (m *Manager) writeWAFConf(ctx context.Context, changed *store.Site) error {
	if !m.WAFInstalled() {
		return nil
	}
	sites, err := m.store.ListSites(ctx)
	if err != nil {
		return err
	}
	for i, s := range sites {
		if s.ID == changed.ID {
			sites[i] = changed
		}
	}
	for path, data := range m.wafFiles(sites) {
		if err := writeFile(path, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// WAFEvent is a request the firewall flagged.
type WAFEvent struct {
	Time   string    `json:"time"`
	IP     string    `json:"ip"`
	Method string    `json:"method"`
	URI    string    `json:"uri"`
	Host   string    `json:"host"`
	Status int       `json:"status"`
	Rules  []WAFRule `json:"rules"`
}

// WAFRule is one rule that matched.
type WAFRule struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

// auditEntry is the part of a ModSecurity JSON audit record the panel uses.
// ModSecurity 3 (Nginx) nests everything in "transaction"; ModSecurity 2
// (Apache) has "transaction", "request", "response" and "audit_data" side by
// side, with the matched rules as text.
type auditEntry struct {
	Transaction struct {
		ClientIP      string `json:"client_ip"`
		TimeStamp     string `json:"time_stamp"`
		RemoteAddress string `json:"remote_address"` // ModSecurity 2
		Time          string `json:"time"`           // ModSecurity 2
		Request       struct {
			Method  string            `json:"method"`
			URI     string            `json:"uri"`
			Headers map[string]string `json:"headers"`
		} `json:"request"`
		Response struct {
			HTTPCode int `json:"http_code"`
		} `json:"response"`
		Messages []struct {
			Message string `json:"message"`
			Details struct {
				RuleID string `json:"ruleId"`
			} `json:"details"`
		} `json:"messages"`
	} `json:"transaction"`
	// ModSecurity 2
	Request struct {
		RequestLine string            `json:"request_line"`
		Headers     map[string]string `json:"headers"`
	} `json:"request"`
	Response struct {
		Status int `json:"status"`
	} `json:"response"`
	AuditData struct {
		Messages []string `json:"messages"`
	} `json:"audit_data"`
}

var (
	auditV2IDRe  = regexp.MustCompile(`\[id "([0-9]+)"\]`)
	auditV2MsgRe = regexp.MustCompile(`\[msg "((?:[^"\\]|\\.)*)"\]`)
)

// v3 turns a ModSecurity 2 record into the ModSecurity 3 fields.
func (e *auditEntry) v3() {
	t := &e.Transaction
	if t.Request.Method != "" || e.Request.RequestLine == "" {
		return
	}
	t.ClientIP = t.RemoteAddress
	t.TimeStamp = t.Time
	// "24/Nov/2026:23:30:32.272338 +0000": drop the microseconds.
	if i := strings.IndexByte(t.TimeStamp, '.'); i > 0 {
		if j := strings.IndexByte(t.TimeStamp[i:], ' '); j > 0 {
			t.TimeStamp = t.TimeStamp[:i] + t.TimeStamp[i+j:]
		}
	}
	if f := strings.Fields(e.Request.RequestLine); len(f) >= 2 {
		t.Request.Method, t.Request.URI = f[0], f[1]
	}
	t.Request.Headers = e.Request.Headers
	t.Response.HTTPCode = e.Response.Status
	for _, text := range e.AuditData.Messages {
		var m struct {
			Message string `json:"message"`
			Details struct {
				RuleID string `json:"ruleId"`
			} `json:"details"`
		}
		if id := auditV2IDRe.FindStringSubmatch(text); id != nil {
			m.Details.RuleID = id[1]
		}
		if msg := auditV2MsgRe.FindStringSubmatch(text); msg != nil {
			m.Message = strings.ReplaceAll(msg[1], `\"`, `"`)
		}
		t.Messages = append(t.Messages, m)
	}
}

// hostMatches reports whether a Host header belongs to one of names.
func hostMatches(host string, names []string) bool {
	host = strings.ToLower(host)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	for _, n := range names {
		if rest, ok := strings.CutPrefix(n, "*."); ok {
			if sub, ok := strings.CutSuffix(host, "."+rest); ok && sub != "" && !strings.Contains(sub, ".") {
				return true
			}
		} else if host == n {
			return true
		}
	}
	return false
}

// parseAudit reads ModSecurity JSON audit lines and returns the newest
// events for names, at most limit.
func parseAudit(r io.Reader, names []string, limit int) []WAFEvent {
	var events []WAFEvent
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 4<<20)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var e auditEntry
		if json.Unmarshal(line, &e) != nil {
			continue
		}
		e.v3()
		t := e.Transaction
		host := ""
		for k, v := range t.Request.Headers {
			if strings.EqualFold(k, "host") {
				host = v
			}
		}
		if !hostMatches(host, names) {
			continue
		}
		ev := WAFEvent{Time: t.TimeStamp, IP: t.ClientIP, Method: t.Request.Method, URI: t.Request.URI,
			Host: host, Status: t.Response.HTTPCode, Rules: []WAFRule{}}
		if len(ev.URI) > 500 {
			ev.URI = ev.URI[:500] + "…"
		}
		for _, msg := range t.Messages {
			// The anomaly score totals repeat what the rules above found.
			if msg.Details.RuleID == "949110" || msg.Details.RuleID == "980130" || msg.Message == "" {
				continue
			}
			ev.Rules = append(ev.Rules, WAFRule{ID: msg.Details.RuleID, Message: msg.Message})
		}
		events = append(events, ev)
	}
	slices.Reverse(events)
	if len(events) > limit {
		events = events[:limit]
	}
	return events
}

// WAFEvents returns the newest firewall events of a site.
func (m *Manager) WAFEvents(ctx context.Context, id int64, limit int) ([]WAFEvent, error) {
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(wafLog())
	if errors.Is(err, os.ErrNotExist) {
		return []WAFEvent{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// Only the end of the log: it can grow large between rotations.
	const tail = 8 << 20
	if st, err := f.Stat(); err == nil && st.Size() > tail {
		f.Seek(st.Size()-tail, io.SeekStart)
	}
	return parseAudit(f, site.Names(), limit), nil
}

// rateZone is the name of a site's limit_req zone.
func rateZone(sysUser string) string { return "panel_rl_" + sysUser }

var rateZoneRe = regexp.MustCompile(`^panel_rl_[a-z0-9_-]{1,64}$`)

// checkFirewall validates firewall values before they reach a template.
func checkFirewall(waf string, rate int, zone string, ips []string) error {
	if !slices.Contains([]string{WAFOff, WAFDetect, WAFBlock}, waf) {
		return fmt.Errorf("refusing to render WAF mode %q", waf)
	}
	if rate < 0 || rate > maxRateLimit || (rate > 0 && !rateZoneRe.MatchString(zone)) {
		return fmt.Errorf("refusing to render rate limit %d in %q", rate, zone)
	}
	clean, err := normalizeIPs(ips)
	if err != nil || !slices.Equal(clean, ips) {
		return fmt.Errorf("refusing to render blocked addresses %q", ips)
	}
	return nil
}
