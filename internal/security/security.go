// Package security manages the server's firewall (ufw), fail2ban and SSH
// settings, with guards against locking the administrator out.
package security

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/usererr"
)

// Manager changes firewall, fail2ban and SSH settings.
type Manager struct {
	run       run.Runner
	panelPort int
	mu        sync.Mutex

	sshDropIn string   // /etc/ssh/sshd_config.d/00-panel.conf
	f2bJail   string   // /etc/fail2ban/jail.d/panel.conf
	f2bFilter string   // /etc/fail2ban/filter.d/panel.conf
	rootKeys  string   // /root/.ssh/authorized_keys
	passwd    string   // /etc/passwd
	shadow    string   // /etc/shadow
	group     string   // /etc/group
	procNet   []string // replaceable in tests

	// webPorts lists the ports the installed web server needs ("80/tcp",
	// "443/tcp"…), or nothing when there is none. See SetWebPorts.
	webPorts func() []string

	// chown gives a file or folder inside root to an account, without
	// following a symbolic link; replaceable in tests.
	chown func(root *os.Root, name string, uid, gid int) error
}

// New returns a Manager. panelPort is the panel's own port, which is
// protected like SSH's.
func New(r run.Runner, panelPort int) *Manager {
	return &Manager{run: r, panelPort: panelPort,
		sshDropIn: "/etc/ssh/sshd_config.d/00-panel.conf",
		f2bJail:   "/etc/fail2ban/jail.d/panel.conf", f2bFilter: "/etc/fail2ban/filter.d/panel.conf",
		rootKeys: "/root/.ssh/authorized_keys",
		passwd:   "/etc/passwd", shadow: "/etc/shadow", group: "/etc/group",
		chown: (*os.Root).Lchown}
}

// SetWebPorts tells the manager which ports the websites need, so that
// turning the firewall on keeps them reachable.
func (m *Manager) SetWebPorts(fn func() []string) { m.webPorts = fn }

// Logger receives progress messages for a job log.
type Logger interface {
	io.Writer
	Logf(format string, args ...any)
}

// ---- firewall ----

// Rule is one ufw rule.
type Rule struct {
	Number    int    `json:"number"`
	To        string `json:"to"`
	Action    string `json:"action"`
	From      string `json:"from"`
	Comment   string `json:"comment,omitempty"`
	V6        bool   `json:"v6"`
	Protected bool   `json:"protected"`
}

// Firewall is the state of ufw.
type Firewall struct {
	Installed bool   `json:"installed"`
	Active    bool   `json:"active"`
	Rules     []Rule `json:"rules"`
	// Added are the rules ufw keeps while it is off, as "ufw show added"
	// prints them ("allow 22/tcp comment 'SSH'"); Rules is then empty, as
	// ufw only numbers rules while it is on.
	Added     []string `json:"added"`
	SSHPort   int      `json:"ssh_port"`
	PanelPort int      `json:"panel_port"`
	// WebPorts are the ports turning the firewall on also opens, for the
	// websites ("80/tcp", "443/tcp"…); empty without a web server.
	WebPorts []string `json:"web_ports"`
}

// "[ 1] 22/tcp                     ALLOW IN    Anywhere                   # comment"
var ruleRe = regexp.MustCompile(`^\[\s*(\d+)\]\s+(.+?)\s+(ALLOW|DENY|REJECT|LIMIT)(?: IN| OUT| FWD)?\s+(.+?)\s*(?:#\s*(.*))?$`)

// Firewall reads ufw's state and rules. Every rule that lets SSH or the
// panel in is marked as protected; FirewallFor is more precise.
func (m *Manager) Firewall(ctx context.Context) Firewall {
	return m.firewall(ctx, netip.Addr{})
}

// FirewallFor reads ufw's state and rules for the administrator at address
// client: a rule for SSH or the panel is protected only when no other rule
// lets client in on that port (see protect).
func (m *Manager) FirewallFor(ctx context.Context, client string) Firewall {
	return m.firewall(ctx, clientAddr(client))
}

func clientAddr(s string) netip.Addr {
	a, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}
	}
	return a.Unmap()
}

func (m *Manager) firewall(ctx context.Context, client netip.Addr) Firewall {
	fw := Firewall{Rules: []Rule{}, Added: []string{}, SSHPort: m.sshPort(ctx), PanelPort: m.panelPort, WebPorts: m.neededWebPorts()}
	out, err := m.run.Output(ctx, "ufw", "status", "numbered")
	if err != nil {
		return fw
	}
	fw.Installed = true
	fw.Active = strings.Contains(out, "Status: active")
	if !fw.Active {
		fw.Added = m.addedRules(ctx)
	}
	for _, line := range strings.Split(out, "\n") {
		mm := ruleRe.FindStringSubmatch(strings.TrimSpace(line))
		if mm == nil {
			continue
		}
		n, _ := strconv.Atoi(mm[1])
		r := Rule{Number: n, To: strings.TrimSpace(mm[2]), Action: mm[3], From: strings.TrimSpace(mm[4]), Comment: mm[5]}
		// ufw writes "(v6)" after "Anywhere", but not after an IPv6 address.
		r.V6 = strings.Contains(r.To, "(v6)") || strings.Contains(r.From, "(v6)") || strings.Contains(r.From, ":")
		fw.Rules = append(fw.Rules, r)
	}
	protect(fw.Rules, fw.SSHPort, m.panelPort, client)
	return fw
}

// addedRules reads the rules ufw keeps while it is off: "ufw show added"
// prints a heading, then one ufw command per rule ("ufw allow 22/tcp"),
// with each IPv4 and IPv6 pair shown once.
func (m *Manager) addedRules(ctx context.Context) []string {
	rules := []string{}
	out, err := m.run.Output(ctx, "ufw", "show", "added")
	if err != nil {
		return rules
	}
	for _, line := range strings.Split(out, "\n") {
		if cmd, ok := strings.CutPrefix(strings.TrimSpace(line), "ufw "); ok {
			rules = append(rules, cmd)
		}
	}
	return rules
}

// protect marks the rules that keep SSH and the panel reachable. An ALLOW
// or LIMIT rule for either port is protected unless another rule lets
// client in on that port too, so the administrator can replace "from
// anywhere" with their own address, but never delete their last way in.
// Without a client address, every such rule is protected.
func protect(rules []Rule, sshPort, panelPort int, client netip.Addr) {
	for i := range rules {
		r := &rules[i]
		p := portOf(r.To)
		r.Protected = letsIn(*r) && (p == sshPort || p == panelPort)
		if !r.Protected || !client.IsValid() {
			continue
		}
		for j, o := range rules {
			if j != i && letsIn(o) && portIn(o.To, p) && covers(o, client) {
				r.Protected = false
				break
			}
		}
	}
}

// letsIn reports whether a rule lets TCP connections in (ALLOW, or LIMIT,
// which allows them at a slower rate).
func letsIn(r Rule) bool {
	if r.Action != "ALLOW" && r.Action != "LIMIT" {
		return false
	}
	_, proto, _ := strings.Cut(trimV6(r.To), "/")
	return proto == "" || proto == "tcp"
}

// portIn reports whether a rule's destination includes TCP port p: "22",
// "22/tcp", a list or range such as "80,443/tcp" or "6000:6010/tcp", or
// "Anywhere" (every port).
func portIn(to string, p int) bool {
	to = trimV6(to)
	if strings.EqualFold(to, "Anywhere") {
		return true
	}
	ports, _, _ := strings.Cut(to, "/")
	for _, part := range strings.Split(ports, ",") {
		lo, hi, isRange := strings.Cut(part, ":")
		a, err1 := strconv.Atoi(lo)
		b, err2 := a, error(nil)
		if isRange {
			b, err2 = strconv.Atoi(hi)
		}
		if err1 == nil && err2 == nil && a <= p && p <= b {
			return true
		}
	}
	return false
}

// covers reports whether a rule's source includes addr.
func covers(r Rule, addr netip.Addr) bool {
	from := trimV6(r.From)
	if strings.EqualFold(from, "Anywhere") {
		return r.V6 == addr.Is6()
	}
	if p, err := netip.ParsePrefix(from); err == nil {
		return p.Contains(addr)
	}
	if a, err := netip.ParseAddr(from); err == nil {
		return a.Unmap() == addr
	}
	return false
}

func trimV6(s string) string {
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "(v6)"))
}

func portOf(to string) int {
	p, _, _ := strings.Cut(trimV6(to), "/")
	n, err := strconv.Atoi(p)
	if err != nil {
		return 0
	}
	return n
}

// neededWebPorts lists the ports the websites need, if a web server is
// installed.
func (m *Manager) neededWebPorts() []string {
	if m.webPorts == nil {
		return []string{}
	}
	if p := m.webPorts(); p != nil {
		return p
	}
	return []string{}
}

var portRe = regexp.MustCompile(`^[0-9]{1,5}(:[0-9]{1,5})?$`)

// NewRule describes a rule to add.
type NewRule struct {
	Action  string `json:"action"` // allow or deny
	Port    string `json:"port"`   // 8080 or 6000:6010
	Proto   string `json:"proto"`  // tcp, udp or "" for both
	From    string `json:"from"`   // IP, CIDR or "" for anywhere
	Comment string `json:"comment"`
}

// AddRule adds a ufw rule. Values are checked and passed as separate
// arguments.
func (m *Manager) AddRule(ctx context.Context, r NewRule) error {
	if r.Action != "allow" && r.Action != "deny" {
		return usererr.New("invalid_rule")
	}
	if !portRe.MatchString(r.Port) {
		return usererr.New("invalid_port", r.Port)
	}
	for _, p := range strings.Split(r.Port, ":") {
		if n, _ := strconv.Atoi(p); n < 1 || n > 65535 {
			return usererr.New("invalid_port", r.Port)
		}
	}
	if strings.Contains(r.Port, ":") && r.Proto == "" {
		r.Proto = "tcp" // ufw needs a protocol for port ranges
	}
	if r.Proto != "" && r.Proto != "tcp" && r.Proto != "udp" {
		return usererr.New("invalid_rule")
	}
	if r.Action == "deny" && (portOf(r.Port) == m.sshPort(ctx) || portOf(r.Port) == m.panelPort) && r.From == "" {
		return usererr.New("rule_locks_out", r.Port)
	}
	from := "any"
	if r.From != "" {
		if _, err := netip.ParsePrefix(r.From); err != nil {
			if _, err := netip.ParseAddr(r.From); err != nil {
				return usererr.New("invalid_ip", r.From)
			}
		}
		from = r.From
	}
	if len(r.Comment) > 60 || strings.ContainsAny(r.Comment, "'\"\n\r\\") {
		return usererr.New("invalid_comment")
	}
	args := []string{r.Action, "from", from, "to", "any", "port", r.Port}
	// ufw uses the first rule that matches. Blocking one address must come
	// before the rules that let everyone in (such as "allow 80/tcp"), or it
	// would do nothing; "prepend" puts it at the top of the IPv4 or IPv6
	// rules. Other rules are added at the end.
	if r.Action == "deny" && r.From != "" {
		args = append([]string{"prepend"}, args...)
	}
	if r.Proto != "" {
		args = append(args, "proto", r.Proto)
	}
	if r.Comment != "" {
		args = append(args, "comment", r.Comment)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	_, err := m.run.Output(ctx, "ufw", args...)
	return err
}

// DeleteAllow deletes the rule "allow from <from> to any port <port> proto
// <proto>" by what it says rather than by number, which also works while
// ufw is off (ufw numbers its rules only while it is on). It never deletes
// a rule for SSH's or the panel's port.
func (m *Manager) DeleteAllow(ctx context.Context, port int, proto, from string) error {
	if port == m.sshPort(ctx) || port == m.panelPort {
		return usererr.New("rule_protected", strconv.Itoa(port))
	}
	if port < 1 || port > 65535 || (proto != "tcp" && proto != "udp") {
		return usererr.New("invalid_rule")
	}
	if _, err := netip.ParsePrefix(from); err != nil {
		if _, err := netip.ParseAddr(from); err != nil {
			return usererr.New("invalid_ip", from)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	_, err := m.run.Output(ctx, "ufw", "delete", "allow", "from", from, "to", "any", "port", strconv.Itoa(port), "proto", proto)
	return err
}

// DeleteRule deletes rule number n after checking it is still the rule the
// user saw (numbers shift when rules change) and that it is not protected.
// Every rule that lets SSH or the panel in is protected; DeleteRuleFor is
// more precise.
func (m *Manager) DeleteRule(ctx context.Context, n int, to, from string) error {
	return m.deleteRule(ctx, n, to, from, netip.Addr{})
}

// DeleteRuleFor is DeleteRule for the administrator at address client: a
// rule for SSH or the panel can be deleted when another rule still lets
// client in on that port.
func (m *Manager) DeleteRuleFor(ctx context.Context, n int, to, from, client string) error {
	return m.deleteRule(ctx, n, to, from, clientAddr(client))
}

func (m *Manager) deleteRule(ctx context.Context, n int, to, from string, client netip.Addr) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.firewall(ctx, client).Rules {
		if r.Number != n {
			continue
		}
		if r.To != to || r.From != from {
			return usererr.New("rule_changed")
		}
		if r.Protected {
			return usererr.New("rule_protected", r.To)
		}
		_, err := m.run.Output(ctx, "ufw", "--force", "delete", strconv.Itoa(n))
		return err
	}
	return usererr.New("rule_changed")
}

// EnableFirewall makes sure SSH, the panel and the websites stay reachable,
// then turns ufw on.
func (m *Manager) EnableFirewall(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.run.Output(ctx, "ufw", "allow", strconv.Itoa(m.sshPort(ctx))+"/tcp", "comment", "SSH"); err != nil {
		return err
	}
	// The panel port, or a website port, may already be limited to some
	// addresses: only open a port when no rule mentions it. ufw numbers
	// its rules only while it is on, so from off (Rules is empty, the
	// rules are in Added) every port is opened: a rule the administrator
	// added for one of them can't lock them out.
	rules := m.Firewall(ctx).Rules
	mentioned := func(port string) bool {
		n, proto, _ := strings.Cut(port, "/")
		for _, r := range rules {
			p, rp, _ := strings.Cut(trimV6(r.To), "/")
			if p == n && (rp == "" || rp == proto) {
				return true
			}
		}
		return false
	}
	type open struct{ port, comment string }
	ports := []open{{strconv.Itoa(m.panelPort) + "/tcp", "Nefo"}}
	// Websites: the web server opens 80 and 443 when it is installed, but
	// only if the firewall was already on then.
	for _, p := range m.neededWebPorts() {
		ports = append(ports, open{p, "Websites"})
	}
	for _, p := range ports {
		if mentioned(p.port) {
			continue
		}
		if _, err := m.run.Output(ctx, "ufw", "allow", p.port, "comment", p.comment); err != nil {
			return err
		}
	}
	_, err := m.run.Output(ctx, "ufw", "--force", "enable")
	return err
}

// ---- SSH ----

// SSH holds the SSH server settings the panel manages.
type SSH struct {
	Port            int    `json:"port"`
	PasswordAuth    bool   `json:"password_auth"`
	PermitRootLogin string `json:"permit_root_login"` // yes, prohibit-password or no (or another sshd value, read only)
	RootHasKeys     bool   `json:"root_has_keys"`
	SocketActivated bool   `json:"socket_activated"`
	// Logins are the accounts that can sign in over SSH with these
	// settings (read only).
	Logins []Login `json:"logins"`
}

// Login is an account that can sign in over SSH, and how.
type Login struct {
	User     string `json:"user"`
	Key      bool   `json:"key"`
	Password bool   `json:"password"`
}

// sshdValues reads sshd's effective settings. Some keywords (AllowUsers…)
// appear on several lines, so each keeps all its values in order.
func (m *Manager) sshdValues(ctx context.Context) map[string][]string {
	out, err := m.run.Output(ctx, "sshd", "-T")
	cfg := map[string][]string{}
	if err != nil {
		return cfg
	}
	for _, line := range strings.Split(out, "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), " "); ok {
			cfg[k] = append(cfg[k], v)
		}
	}
	return cfg
}

func (m *Manager) sshdConfig(ctx context.Context) map[string]string {
	cfg := map[string]string{}
	for k, v := range m.sshdValues(ctx) {
		cfg[k] = v[0]
	}
	return cfg
}

func (m *Manager) sshPort(ctx context.Context) int {
	if p, err := strconv.Atoi(m.sshdConfig(ctx)["port"]); err == nil {
		return p
	}
	return 22
}

// SSHSettings reads the current SSH settings.
func (m *Manager) SSHSettings(ctx context.Context) SSH {
	return m.sshSettings(ctx, m.sshdValues(ctx))
}

// sshSettings is SSHSettings from sshd's settings vals (sshd -T).
func (m *Manager) sshSettings(ctx context.Context, vals map[string][]string) SSH {
	first := func(k string) string {
		if v := vals[k]; len(v) > 0 {
			return v[0]
		}
		return ""
	}
	s := SSH{Port: 22, PasswordAuth: first("passwordauthentication") != "no", PermitRootLogin: rootLoginValue(first("permitrootlogin"))}
	if p, err := strconv.Atoi(first("port")); err == nil {
		s.Port = p
	}
	s.RootHasKeys = m.rootHasKeys()
	s.Logins = m.sshLogins(vals, s)
	out, _ := m.run.Output(ctx, "systemctl", "is-active", "ssh.socket")
	s.SocketActivated = strings.HasPrefix(out, "active")
	return s
}

// account is a user of the server, from /etc/passwd.
type account struct {
	name, home, shell string
	uid, gid          int
}

// sshLogins lists who could sign in over SSH with the settings s, and
// how: root if PermitRootLogin lets it in with a key or a password it has,
// and the people's accounts (UID 1000 and up) with a login shell and a key,
// or a password while password sign-in is on. AllowUsers, DenyUsers,
// AllowGroups and DenyGroups are taken into account; Match blocks are not.
// vals are sshd's other settings (sshd -T), which the panel does not change.
func (m *Manager) sshLogins(vals map[string][]string, s SSH) []Login {
	return m.loginsWith(vals, s, nil)
}

// loginsWith is sshLogins with the authorized_keys files in override (by
// path) holding the given text instead of what they hold now: who could
// sign in after a change to them.
func (m *Manager) loginsWith(vals map[string][]string, s SSH, override map[string]string) []Login {
	logins := []Login{}
	accounts := m.accounts()
	if accounts == nil {
		return logins
	}
	pw := m.passwords()
	groups := m.groupsOf(accounts)
	keyCmd := keyCommand(vals)
	keyFiles := authorizedKeysFiles(vals)
	for _, a := range accounts {
		root := a.uid == 0
		if !keyAccount(a) {
			continue
		}
		if !sshAllowed(vals, a.name, groups[a.name]) {
			continue
		}
		key := keyCmd || m.hasKeys(a, keyFiles, override)
		password := s.PasswordAuth && pw(a.name)
		if root {
			switch s.PermitRootLogin {
			case "yes":
			case "prohibit-password":
				password = false
			default:
				continue
			}
		}
		if key || password {
			logins = append(logins, Login{User: a.name, Key: key, Password: password})
		}
	}
	return logins
}

func (m *Manager) accounts() []account {
	b, err := os.ReadFile(m.passwd)
	if err != nil {
		return nil
	}
	accounts := []account{}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Split(strings.TrimSpace(line), ":")
		if len(f) < 7 || f[0] == "" || strings.HasPrefix(f[0], "#") {
			continue
		}
		uid, err1 := strconv.Atoi(f[2])
		gid, err2 := strconv.Atoi(f[3])
		if err1 != nil || err2 != nil {
			continue
		}
		// The shell is the last field (the home folder has no ":" on Linux,
		// but a Windows path in the tests does).
		n := len(f) - 1
		accounts = append(accounts, account{name: f[0], uid: uid, gid: gid, home: strings.Join(f[5:n], ":"), shell: f[n]})
	}
	return accounts
}

// passwords returns a function that reports whether an account has a
// password it can sign in with. When /etc/shadow can't be read, it answers
// yes, so the guard never refuses on a guess.
func (m *Manager) passwords() func(string) bool {
	b, err := os.ReadFile(m.shadow)
	if err != nil {
		return func(string) bool { return true }
	}
	has := map[string]bool{}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Split(strings.TrimSpace(line), ":")
		if len(f) < 2 {
			continue
		}
		// "" is no password (sshd refuses it), "!…" a locked one and "*"
		// or "!!" none at all.
		has[f[0]] = f[1] != "" && !strings.HasPrefix(f[1], "!") && !strings.HasPrefix(f[1], "*")
	}
	return func(name string) bool { return has[name] }
}

// groupsOf maps account names to their group names (primary and others).
func (m *Manager) groupsOf(accounts []account) map[string][]string {
	out := map[string][]string{}
	b, err := os.ReadFile(m.group)
	if err != nil {
		return out
	}
	byGID := map[int]string{}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Split(strings.TrimSpace(line), ":")
		if len(f) < 4 {
			continue
		}
		if gid, err := strconv.Atoi(f[2]); err == nil {
			byGID[gid] = f[0]
		}
		for _, u := range strings.Split(f[3], ",") {
			if u = strings.TrimSpace(u); u != "" {
				out[u] = append(out[u], f[0])
			}
		}
	}
	for _, a := range accounts {
		if g, ok := byGID[a.gid]; ok {
			out[a.name] = append(out[a.name], g)
		}
	}
	return out
}

// loginShell reports whether a shell gives a command line: not nologin,
// false and the like (the panel's SFTP users have nologin).
func loginShell(shell string) bool {
	base := filepath.Base(strings.TrimSpace(shell))
	if shell == "" {
		return true // the default, /bin/sh
	}
	return !slices.Contains([]string{"nologin", "false", "true", "sync", "halt", "shutdown", "git-shell"}, base)
}

// sshAllowed applies sshd's DenyUsers, AllowUsers, DenyGroups and
// AllowGroups to an account, in that order. Patterns may use * and ?; a
// "user@host" pattern is counted on its user part only.
func sshAllowed(vals map[string][]string, name string, groups []string) bool {
	list := func(k string) []string { return strings.Fields(strings.Join(vals[k], " ")) }
	match := func(patterns []string, names ...string) bool {
		for _, p := range patterns {
			p, _, _ = strings.Cut(p, "@")
			for _, n := range names {
				if ok, _ := filepath.Match(p, n); ok {
					return true
				}
			}
		}
		return false
	}
	if match(list("denyusers"), name) {
		return false
	}
	if a := list("allowusers"); len(a) > 0 && !match(a, name) {
		return false
	}
	if match(list("denygroups"), groups...) {
		return false
	}
	if a := list("allowgroups"); len(a) > 0 && !match(a, groups...) {
		return false
	}
	return true
}

// hasKeys reports whether an account has an SSH public key in one of its
// AuthorizedKeysFile files (see keyPaths). A file in override is counted
// with the text given there instead of what it holds, and more strictly:
// only keys that parse and are not weak (see usableKeys).
func (m *Manager) hasKeys(a account, files []string, override map[string]string) bool {
	for _, p := range m.keyPaths(a, files) {
		if text, ok := override[p]; ok {
			if usableKeys(text) {
				return true
			}
		} else if keysIn(p) {
			return true
		}
	}
	return false
}

// keyPaths lists an account's authorized_keys files, from sshd's
// AuthorizedKeysFile: %h is the home folder, %u the user name, and a
// relative path is in the home folder. root's keys file always comes first.
func (m *Manager) keyPaths(a account, files []string) []string {
	var paths []string
	if a.uid == 0 {
		paths = append(paths, filepath.Clean(m.rootKeys))
	}
	for _, f := range files {
		f = strings.NewReplacer("%h", a.home, "%u", a.name, "%%", "%").Replace(f)
		if f == "none" {
			continue
		}
		if !filepath.IsAbs(f) {
			f = filepath.Join(a.home, f)
		}
		if f = filepath.Clean(f); !slices.Contains(paths, f) {
			paths = append(paths, f)
		}
	}
	return paths
}

// authorizedKeysFiles is sshd's AuthorizedKeysFile, or its default.
func authorizedKeysFiles(vals map[string][]string) []string {
	if v := vals["authorizedkeysfile"]; len(v) > 0 {
		return strings.Fields(strings.Join(v, " "))
	}
	return []string{".ssh/authorized_keys", ".ssh/authorized_keys2"}
}

// keyCommand reports whether sshd also asks a program for keys
// (AuthorizedKeysCommand), which the panel can't see.
func keyCommand(vals map[string][]string) bool {
	return len(vals["authorizedkeyscommand"]) > 0 && vals["authorizedkeyscommand"][0] != "none"
}

// keyAccount reports whether the guard counts an account: root, and the
// people's accounts (UID 1000 and up) with a login shell.
func keyAccount(a account) bool {
	return a.uid == 0 || (a.uid >= 1000 && a.uid != 65534 && loginShell(a.shell))
}

// keysIn reports whether an authorized_keys file holds a public key.
func keysIn(path string) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return keysInText(string(b))
}

// keysInText reports whether authorized_keys text holds a public key.
func keysInText(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		// OpenSSH refuses DSA keys (ssh-dss) since version 7.0.
		if line == "" || strings.HasPrefix(line, "#") || slices.Contains(strings.Fields(line), "ssh-dss") {
			continue
		}
		if strings.Contains(line, "ssh-") || strings.Contains(line, "ecdsa-sha2-") || strings.Contains(line, "sk-") {
			return true
		}
	}
	return false
}

// rootLoginValue gives sshd's PermitRootLogin in the panel's words.
// "without-password" is the older name of "prohibit-password", and some
// sshd versions (Ubuntu 24.04 among them) still print it with sshd -T.
func rootLoginValue(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "without-password" {
		return "prohibit-password"
	}
	return v
}

func (m *Manager) rootHasKeys() bool { return keysIn(m.rootKeys) }

// SetSSH writes the panel's SSH settings and restarts SSH. It refuses
// changes that would lock the administrator out: settings with which
// nobody could sign in any more (no key while passwords are off, root not
// allowed and no other account…), or a port another program uses.
func (m *Manager) SetSSH(ctx context.Context, s SSH) error {
	s.PermitRootLogin = rootLoginValue(s.PermitRootLogin)
	if !slices.Contains([]string{"yes", "prohibit-password", "no"}, s.PermitRootLogin) {
		return usererr.New("invalid_ssh_settings")
	}
	if s.Port < 1 || s.Port > 65535 || s.Port == m.panelPort || s.Port == 80 || s.Port == 443 {
		return usererr.New("invalid_port", strconv.Itoa(s.Port))
	}
	// Locked from the check on, so that removing an SSH key meanwhile
	// (RemoveSSHKey) can't combine with these settings into a lock-out.
	m.mu.Lock()
	defer m.mu.Unlock()
	cur := m.SSHSettings(ctx)
	// Refuse only what makes things worse: if nobody can sign in now (a
	// server used from its provider's console), other changes stay possible.
	if len(m.sshLogins(m.sshdValues(ctx), s)) == 0 && len(cur.Logins) > 0 {
		switch {
		case s.PermitRootLogin != "no" && !s.PasswordAuth:
			return usererr.New("ssh_no_keys")
		case s.PermitRootLogin == "prohibit-password":
			return usererr.New("ssh_root_needs_key")
		default:
			return usererr.New("ssh_no_login")
		}
	}
	if s.Port != cur.Port && cur.SocketActivated {
		return usererr.New("ssh_socket_activated")
	}
	// Never move SSH onto a port another program listens on (the panel,
	// MariaDB…): SSH would fail to start, and the firewall would open that
	// program's port to everyone.
	if s.Port != cur.Port && m.listening(s.Port) {
		return usererr.New("port_in_use", strconv.Itoa(s.Port))
	}
	if s.Port != cur.Port {
		// Open the new port before SSH moves to it.
		if out, err := m.run.Output(ctx, "ufw", "status"); err == nil && strings.Contains(out, "Status: active") {
			if _, err := m.run.Output(ctx, "ufw", "allow", strconv.Itoa(s.Port)+"/tcp", "comment", "SSH"); err != nil {
				return err
			}
		}
	}
	yesNo := map[bool]string{true: "yes", false: "no"}
	conf := fmt.Sprintf(`# Managed by the panel. Do not edit: the panel rewrites this file.
# Files in sshd_config.d are read in name order and the first value wins,
# so this file overrides the others.
Port %d
PasswordAuthentication %s
KbdInteractiveAuthentication no
PermitRootLogin %s
`, s.Port, yesNo[s.PasswordAuth], s.PermitRootLogin)
	old, oldErr := os.ReadFile(m.sshDropIn)
	if err := fsutil.WriteFileAtomic(m.sshDropIn, []byte(conf), 0o644); err != nil {
		return err
	}
	restore := func() {
		if oldErr == nil {
			fsutil.WriteFileAtomic(m.sshDropIn, old, 0o644)
		} else {
			os.Remove(m.sshDropIn)
		}
	}
	if out, err := m.run.Output(ctx, "sshd", "-t"); err != nil {
		restore()
		return usererr.New("config_test_failed", run.Tail(out, 5))
	}
	if _, err := m.run.Output(ctx, "systemctl", "restart", "ssh"); err != nil {
		restore()
		m.run.Output(ctx, "systemctl", "restart", "ssh")
		return err
	}
	if s.Port != cur.Port {
		// fail2ban must watch the new port. SSH has moved already, so a
		// failure here is not reported as one of SSH: the next start of the
		// panel tries again (RefreshFail2ban).
		m.writeFail2ban(ctx, s.Port, false)
	}
	return nil
}

// ---- fail2ban ----

// Jail is a fail2ban jail with its banned addresses.
type Jail struct {
	Name   string   `json:"name"`
	Failed int      `json:"failed"`
	Banned []string `json:"banned"`
	Total  int      `json:"total_banned"`
}

// Fail2ban is the state of fail2ban.
type Fail2ban struct {
	Installed bool   `json:"installed"`
	Running   bool   `json:"running"`
	Jails     []Jail `json:"jails"`
}

// Fail2banStatus reads the jails and banned addresses.
func (m *Manager) Fail2banStatus(ctx context.Context) Fail2ban {
	st := Fail2ban{Jails: []Jail{}}
	if _, err := os.Stat("/usr/bin/fail2ban-client"); err != nil {
		return st
	}
	st.Installed = true
	out, err := m.run.Output(ctx, "fail2ban-client", "status")
	if err != nil {
		return st
	}
	st.Running = true
	var names []string
	for _, line := range strings.Split(out, "\n") {
		if _, list, ok := strings.Cut(line, "Jail list:"); ok {
			for _, n := range strings.Split(list, ",") {
				if n = strings.TrimSpace(n); jailRe.MatchString(n) {
					names = append(names, n)
				}
			}
		}
	}
	for _, n := range names {
		j := Jail{Name: n, Banned: []string{}}
		out, err := m.run.Output(ctx, "fail2ban-client", "status", n)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(out, "\n") {
			k, v, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			v = strings.TrimSpace(v)
			switch {
			case strings.Contains(k, "Currently failed"):
				j.Failed, _ = strconv.Atoi(v)
			case strings.Contains(k, "Total banned"):
				j.Total, _ = strconv.Atoi(v)
			case strings.Contains(k, "Banned IP list"):
				for _, ip := range strings.Fields(v) {
					if _, err := netip.ParseAddr(ip); err == nil {
						j.Banned = append(j.Banned, ip)
					}
				}
			}
		}
		st.Jails = append(st.Jails, j)
	}
	return st
}

var jailRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,40}$`)

// InstallFail2ban installs fail2ban with a jail for SSH and one for the
// panel's sign-in page.
func (m *Manager) InstallFail2ban(ctx context.Context, lg Logger) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	lg.Logf("Installing fail2ban")
	if err := run.Apt(ctx, m.run, lg, "update"); err != nil {
		return err
	}
	if err := run.Apt(ctx, m.run, lg, "install", "--no-install-recommends", "fail2ban", "python3-systemd"); err != nil {
		return err
	}
	lg.Logf("Configuring the jails for SSH and the panel")
	if _, err := m.writeFail2ban(ctx, m.sshPort(ctx), true); err != nil {
		return err
	}
	if _, err := m.run.Output(ctx, "systemctl", "enable", "fail2ban"); err != nil {
		return err
	}
	if _, err := m.run.Output(ctx, "systemctl", "restart", "fail2ban"); err != nil {
		return err
	}
	lg.Logf("fail2ban is running")
	return nil
}

// f2bFilterConf is the filter of the panel jail. The panel logs failed
// sign-ins as "msg=audit … action=login_failed ip=X", and attempts refused
// while an address is locked out as action=login_blocked, with the address
// last, so the $ anchor ignores look-alike text in the (quoted) user name.
//
// The panel locks an address out itself after 5 failures in 15 minutes;
// the jail bans it in the firewall for an hour when it keeps trying (10
// attempts in 10 minutes, counting the refused ones).
const f2bFilterConf = `# Managed by the panel. Do not edit: the panel rewrites this file.
[Definition]
failregex = ^.*msg=audit .* action=(?:login_failed|login_blocked) ip=<HOST>$
ignoreregex =
journalmatch = _SYSTEMD_UNIT=nefo.service
`

func (m *Manager) jailConf(sshPort int) string {
	return fmt.Sprintf(`# Managed by the panel. Do not edit: the panel rewrites this file.
[DEFAULT]
bantime = 1h
findtime = 10m
maxretry = 5
backend = systemd

[sshd]
enabled = true
port = %d

[panel]
enabled = true
port = %d
filter = panel
maxretry = 10
`, sshPort, m.panelPort)
}

// writeFail2ban writes the panel's fail2ban filter and jails for SSH on
// sshPort. Unless install is true, it does nothing when the panel never
// set fail2ban up (no jail file), and it reloads a running fail2ban when a
// file changed. It reports whether one did.
func (m *Manager) writeFail2ban(ctx context.Context, sshPort int, install bool) (bool, error) {
	if !install {
		if _, err := os.Stat(m.f2bJail); err != nil {
			return false, nil
		}
	}
	changed := false
	for path, content := range map[string]string{m.f2bFilter: f2bFilterConf, m.f2bJail: m.jailConf(sshPort)} {
		if old, err := os.ReadFile(path); err == nil && string(old) == content {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return changed, err
		}
		if err := fsutil.WriteFileAtomic(path, []byte(content), 0o644); err != nil {
			return changed, err
		}
		changed = true
	}
	if changed && !install {
		// Only if it is running: a fail2ban the administrator stopped stays
		// stopped.
		if _, err := m.run.Output(ctx, "systemctl", "try-reload-or-restart", "fail2ban"); err != nil {
			return changed, err
		}
	}
	return changed, nil
}

// RefreshFail2ban brings the panel's fail2ban files up to date, after the
// SSH or panel port changed or an update changed them. It does nothing when
// fail2ban was not set up by the panel, and reports whether a file changed.
func (m *Manager) RefreshFail2ban(ctx context.Context) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writeFail2ban(ctx, m.sshPort(ctx), false)
}

// Unban removes an address from a jail.
func (m *Manager) Unban(ctx context.Context, jail, ip string) error {
	if !jailRe.MatchString(jail) {
		return usererr.New("invalid_rule")
	}
	if _, err := netip.ParseAddr(ip); err != nil {
		return usererr.New("invalid_ip", ip)
	}
	_, err := m.run.Output(ctx, "fail2ban-client", "set", jail, "unbanip", ip)
	if err != nil {
		return errors.New("fail2ban could not unban " + ip)
	}
	return nil
}

// listening reports whether a TCP socket listens on port, from
// /proc/net/tcp and tcp6 (state 0A is LISTEN).
func (m *Manager) listening(port int) bool {
	files := m.procNet
	if files == nil {
		files = []string{"/proc/net/tcp", "/proc/net/tcp6"}
	}
	want := fmt.Sprintf(":%04X", port)
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n")[1:] {
			fields := strings.Fields(line)
			if len(fields) > 3 && fields[3] == "0A" && strings.HasSuffix(fields[1], want) {
				return true
			}
		}
	}
	return false
}
