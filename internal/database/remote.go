package database

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/security"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
)

// Remote access lets chosen client addresses connect to one database from
// other machines. For each address the database user gets a twin
// 'user'@'address' with the same password and privileges, ufw opens port
// 3306 to that address only, and the server listens on every interface while
// at least one database allows remote access. MySQL works the same way; the
// differences are noted where they are.

const (
	// remoteConfName makes the server listen on every interface. In the
	// engine's drop-in folder it sorts after 50-server.cnf (MariaDB),
	// mysqld.cnf (MySQL) and 90-panel.cnf, and the last value read wins.
	remoteConfName = "99-panel-remote.cnf"
	// MaxRemoteHosts is the number of client addresses one database may allow.
	MaxRemoteHosts = 10
	// minRemotePrefix is the widest IPv4 range allowed (/16: 65,536 addresses).
	minRemotePrefix = 16
	mariadbPort     = 3306
)

// Firewall is the part of the firewall manager that remote access needs;
// *security.Manager implements it.
type Firewall interface {
	Firewall(ctx context.Context) security.Firewall
	AddRule(ctx context.Context, r security.NewRule) error
	DeleteRule(ctx context.Context, n int, to, from string) error
	DeleteAllow(ctx context.Context, port int, proto, from string) error
}

// SetFirewall gives the manager the firewall used for remote access. It must
// be called before remote access is turned on.
func (m *Manager) SetFirewall(fw Firewall) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fw = fw
}

// ParseRemoteHosts checks and normalises client addresses: IPv4 or IPv6
// addresses, or IPv4 ranges in CIDR form no wider than /16. Empty entries
// and duplicates are dropped. The result is what the panel stores and what
// ufw gets ("203.0.113.5", "10.1.0.0/16", "2001:db8::1").
func ParseRemoteHosts(in []string) ([]string, error) {
	out := []string{}
	for _, raw := range in {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		h, err := parseRemoteHost(s)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(out, h) {
			out = append(out, h)
		}
	}
	if len(out) > MaxRemoteHosts {
		return nil, usererr.New("too_many_remote_hosts", strconv.Itoa(MaxRemoteHosts))
	}
	return out, nil
}

func parseRemoteHost(s string) (string, error) {
	if len(s) > 64 {
		return "", usererr.New("invalid_remote_host", s[:64])
	}
	if !strings.Contains(s, "/") {
		a, err := netip.ParseAddr(s)
		if err != nil {
			return "", usererr.New("invalid_remote_host", s)
		}
		return checkRemoteAddr(a, s)
	}
	p, err := netip.ParsePrefix(s)
	if err != nil {
		return "", usererr.New("invalid_remote_host", s)
	}
	if p.Bits() < minRemotePrefix {
		return "", usererr.New("remote_host_too_wide", s) // includes 0.0.0.0/0 and ::/0
	}
	if p.Addr().Is6() {
		if p.Bits() == 128 {
			return checkRemoteAddr(p.Addr(), s)
		}
		// MariaDB's host patterns only describe IPv4 ranges.
		return "", usererr.New("remote_ipv6_range", s)
	}
	if p.Bits() == 32 {
		return checkRemoteAddr(p.Addr(), s)
	}
	p = p.Masked()
	if p.Addr().IsUnspecified() || p.Addr().IsLoopback() || p.Addr().IsMulticast() {
		return "", usererr.New("invalid_remote_host", s)
	}
	return p.String(), nil
}

func checkRemoteAddr(a netip.Addr, s string) (string, error) {
	if a.Zone() != "" {
		return "", usererr.New("invalid_remote_host", s)
	}
	a = a.Unmap()
	if a.IsUnspecified() || a.IsLoopback() || a.IsMulticast() {
		// localhost already has its own user; 0.0.0.0 and :: match nobody.
		return "", usererr.New("invalid_remote_host", s)
	}
	return a.String(), nil
}

// mariaHost turns a normalised client address into MariaDB's host form: an
// address stays as it is, and an IPv4 range becomes 'network/netmask', such
// as 10.1.0.0/255.255.0.0.
func mariaHost(h string) string {
	p, err := netip.ParsePrefix(h)
	if err != nil || !p.Addr().Is4() {
		return h
	}
	return p.Addr().String() + "/" + net.IP(net.CIDRMask(p.Bits(), 32)).String()
}

// fromMariaHost is the reverse of mariaHost. It only accepts host forms the
// panel itself creates, so a host read back from mysql.user is safe to put
// in SQL, and hosts the administrator added by hand ('%', names) are left
// alone.
func fromMariaHost(s string) (string, bool) {
	h := s
	if addr, mask, ok := strings.Cut(s, "/"); ok {
		m, err := netip.ParseAddr(mask)
		if err != nil || !m.Is4() {
			return "", false
		}
		b := m.As4()
		ones, bits := net.IPMask(b[:]).Size()
		if bits == 0 { // not a contiguous mask
			return "", false
		}
		h = addr + "/" + strconv.Itoa(ones)
	}
	n, err := parseRemoteHost(h)
	if err != nil || mariaHost(n) != s {
		return "", false
	}
	return n, true
}

// safeHostRe is a last line of defence: a host that reaches SQL only holds
// characters of IP addresses and netmasks.
var safeHostRe = regexp.MustCompile(`^[0-9a-f.:/]{2,64}$`)

// remoteUserSQL returns the statements that drop the user's twins for the
// hosts in drop and create (or refresh) those for add, with the database's
// password and the same privileges as the local user. Hosts are in MariaDB
// form, which MySQL reads the same way.
func remoteUserSQL(name, user, password string, add, drop []string) (string, error) {
	return remoteUserSQLFor(MariaDB, name, user, password, add, drop)
}

// remoteUserSQLFor is remoteUserSQL for an engine. MySQL has no CREATE OR
// REPLACE USER, so a twin is dropped and created again there.
func remoteUserSQLFor(e Engine, name, user, password string, add, drop []string) (string, error) {
	if validate.DBName(name) != nil || validate.DBName(user) != nil {
		return "", fmt.Errorf("invalid database name %q", name)
	}
	if len(add) > 0 && validate.DBPassword(password) != nil {
		return "", errors.New("the stored database password is not valid")
	}
	var b strings.Builder
	for _, h := range drop {
		if !safeHostRe.MatchString(h) {
			return "", fmt.Errorf("invalid host %q", h)
		}
		fmt.Fprintf(&b, "DROP USER IF EXISTS '%s'@'%s';\n", user, h)
	}
	for _, h := range add {
		if !safeHostRe.MatchString(h) {
			return "", fmt.Errorf("invalid host %q", h)
		}
		if e == MySQL {
			fmt.Fprintf(&b, "DROP USER IF EXISTS '%s'@'%s';\nCREATE USER '%s'@'%s' IDENTIFIED BY '%s';\n", user, h, user, h, password)
		} else {
			fmt.Fprintf(&b, "CREATE OR REPLACE USER '%s'@'%s' IDENTIFIED BY '%s';\n", user, h, password)
		}
		fmt.Fprintf(&b, "GRANT ALL PRIVILEGES ON `%s`.* TO '%s'@'%s';\n", name, user, h)
	}
	return b.String(), nil
}

// remoteConf is the content of the listen drop-in. "::" also accepts IPv4
// connections; it is only used when an IPv6 client is allowed.
func remoteConf(ipv6 bool) string {
	bind := "0.0.0.0"
	if ipv6 {
		bind = "::"
	}
	return `# Managed by the panel. Do not edit: the panel removes this file when no
# database allows remote access. ufw only lets the allowed addresses in.
[mysqld]
bind-address = ` + bind + "\n"
}

func listensAll(bind string) bool {
	for _, b := range strings.Split(bind, ",") {
		switch strings.TrimSpace(b) {
		case "", "*", "0.0.0.0", "::":
			return true
		}
	}
	return false
}

// RemoteAccess is the remote access state of one database.
type RemoteAccess struct {
	Hosts []string `json:"hosts"`
	// BindAddress is where the server listens now; Listening means it
	// accepts connections from other machines.
	BindAddress string `json:"bind_address"`
	Listening   bool   `json:"listening"`
	// RestartPending means the server must restart before the listen
	// setting takes effect.
	RestartPending  bool `json:"restart_pending"`
	FirewallActive  bool `json:"firewall_active"`
	FirewallPresent bool `json:"firewall_installed"`
	Port            int  `json:"port"`
}

// Remote returns the remote access state of a database.
func (m *Manager) Remote(ctx context.Context, id int64) (*RemoteAccess, error) {
	d, err := m.store.DatabaseByID(ctx, id)
	if err != nil {
		return nil, err
	}
	hosts, err := m.store.DBRemoteHosts(ctx, d.ID)
	if err != nil {
		return nil, err
	}
	ra := &RemoteAccess{Hosts: hosts, Port: mariadbPort}
	if m.Status(ctx).Running {
		if bind, err := m.bindAddress(ctx); err == nil {
			ra.BindAddress, ra.Listening = bind, listensAll(bind)
		}
	}
	ra.RestartPending = m.RestartPending(ctx)
	m.mu.Lock()
	fw := m.fw
	m.mu.Unlock()
	if fw != nil {
		f := fw.Firewall(ctx)
		ra.FirewallPresent, ra.FirewallActive = f.Installed, f.Active
	}
	return ra, nil
}

// SetRemote replaces the client addresses that may reach a database from
// other machines; an empty list turns remote access off. It reports whether
// the server must now restart to change where it listens (see Restart).
func (m *Manager) SetRemote(ctx context.Context, id int64, hosts []string) (restart bool, err error) {
	hosts, err = ParseRemoteHosts(hosts)
	if err != nil {
		return false, err
	}
	if err := m.ready(ctx); err != nil {
		return false, err
	}
	if !m.Status(ctx).Running {
		return false, usererr.New("db_not_running")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fw == nil {
		return false, errors.New("the panel has no firewall manager for remote access")
	}
	if len(hosts) > 0 && !m.fw.Firewall(ctx).Installed {
		return false, usererr.New("firewall_not_installed")
	}
	d, err := m.store.DatabaseByID(ctx, id)
	if err != nil {
		return false, err
	}
	prev, err := m.store.DBRemoteHosts(ctx, d.ID)
	if err != nil {
		return false, err
	}
	// Save first: whatever fails below, the stored list says what the
	// administrator wants, and the next change or delete cleans up after it.
	if err := m.store.SetDBRemoteHosts(ctx, d.ID, hosts); err != nil {
		return false, err
	}
	if err := m.syncRemoteUsers(ctx, d, hosts, prev); err != nil {
		return false, err
	}
	if err := m.syncRemoteFirewall(ctx, d.Name, hosts); err != nil {
		return false, err
	}
	if err := m.writeListenConf(ctx); err != nil {
		return false, err
	}
	return m.restartPendingLocked(ctx), nil
}

// remoteHostsInDB returns the panel-made remote hosts of a user that exist
// in the server, normalised.
func (m *Manager) remoteHostsInDB(ctx context.Context, user string) ([]string, error) {
	if validate.DBName(user) != nil {
		return nil, fmt.Errorf("invalid user name %q", user)
	}
	out, err := m.sql(ctx, fmt.Sprintf("SELECT Host FROM mysql.user WHERE User = '%s' AND Host <> 'localhost';", user))
	if err != nil {
		return nil, err
	}
	var hosts []string
	for _, line := range strings.Split(out, "\n") {
		if h, ok := fromMariaHost(strings.TrimSpace(line)); ok {
			hosts = append(hosts, h)
		}
	}
	return hosts, nil
}

// syncRemoteUsers makes the user's twins match hosts: twins for addresses
// that are no longer allowed are dropped, and the others are created or
// refreshed with the current password.
func (m *Manager) syncRemoteUsers(ctx context.Context, d *store.Database, hosts, prev []string) error {
	existing, err := m.remoteHostsInDB(ctx, d.Username)
	if err != nil {
		return err
	}
	var add, drop []string
	for _, h := range hosts {
		add = append(add, mariaHost(h))
	}
	for _, h := range append(existing, prev...) {
		if mh := mariaHost(h); !slices.Contains(add, mh) && !slices.Contains(drop, mh) {
			if _, ok := fromMariaHost(mh); ok {
				drop = append(drop, mh)
			}
		}
	}
	if len(add) == 0 && len(drop) == 0 {
		return nil
	}
	q, err := remoteUserSQLFor(m.engine(), d.Name, d.Username, d.Password, add, drop)
	if err != nil {
		return err
	}
	_, err = m.sql(ctx, q)
	return err
}

// firewallComment marks the ufw rules of a database: "MariaDB shop" or
// "MySQL shop".
func (m *Manager) firewallComment(name string) string { return m.engine().Name() + " " + name }

// ownRule reports whether a ufw rule's comment is the panel's for the
// database name, with either engine's name: a rule from before the server
// was switched still belongs to the database.
func ownRule(comment, name string) bool {
	return comment == MariaDB.Name()+" "+name || comment == MySQL.Name()+" "+name
}

func rulePort(to string) int {
	to = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(to), "(v6)"))
	p, _, _ := strings.Cut(to, "/")
	n, _ := strconv.Atoi(p)
	return n
}

// ruleFrom normalises the source of a ufw rule as shown by "ufw status".
func ruleFrom(from string) string {
	from = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(from), "(v6)"))
	if p, err := netip.ParsePrefix(from); err == nil {
		if p.IsSingleIP() {
			return p.Addr().Unmap().String()
		}
		return p.Masked().String()
	}
	if a, err := netip.ParseAddr(from); err == nil {
		return a.Unmap().String()
	}
	return from
}

// syncRemoteFirewall makes the database's ufw rules (recognised by their
// comment) allow port 3306 from exactly the given addresses.
func (m *Manager) syncRemoteFirewall(ctx context.Context, name string, hosts []string) error {
	if m.fw == nil {
		if len(hosts) > 0 {
			return errors.New("the panel has no firewall manager for remote access")
		}
		return nil
	}
	comment := m.firewallComment(name)
	if fw := m.fw.Firewall(ctx); fw.Installed && !fw.Active {
		return m.syncRemoteFirewallOff(ctx, fw, name, comment, hosts)
	}
	// Rule numbers change after every delete, so read the rules again each time.
	for range 100 {
		fw := m.fw.Firewall(ctx)
		if !fw.Installed {
			if len(hosts) > 0 {
				return usererr.New("firewall_not_installed")
			}
			return nil
		}
		have := map[string]bool{}
		var stale *security.Rule
		for i, r := range fw.Rules {
			if !ownRule(r.Comment, name) || rulePort(r.To) != mariadbPort {
				continue
			}
			from := ruleFrom(r.From)
			if slices.Contains(hosts, from) && !have[from] {
				have[from] = true
				continue
			}
			stale = &fw.Rules[i]
			break
		}
		if stale != nil {
			if err := m.fw.DeleteRule(ctx, stale.Number, stale.To, stale.From); err != nil {
				return fmt.Errorf("remove firewall rule %d: %w", stale.Number, err)
			}
			continue
		}
		for _, h := range hosts {
			if have[h] {
				continue
			}
			err := m.fw.AddRule(ctx, security.NewRule{
				Action: "allow", Port: strconv.Itoa(mariadbPort), Proto: "tcp", From: h, Comment: comment,
			})
			if err != nil {
				return fmt.Errorf("open port %d to %s: %w", mariadbPort, h, err)
			}
		}
		return nil
	}
	return errors.New("the firewall rules kept changing")
}

// addedRuleRe matches a remote access rule as "ufw show added" lists it
// while ufw is off.
var addedRuleRe = regexp.MustCompile(`^allow from (\S+) to any port 3306 proto tcp comment '([^']*)'$`)

// syncRemoteFirewallOff is syncRemoteFirewall while ufw is off: its rules
// have no numbers then, so a stale one is deleted by what it says, before
// turning the firewall on could make it live.
func (m *Manager) syncRemoteFirewallOff(ctx context.Context, fw security.Firewall, name, comment string, hosts []string) error {
	have := map[string]bool{}
	for _, line := range fw.Added {
		mm := addedRuleRe.FindStringSubmatch(strings.TrimSpace(line))
		if mm == nil || !ownRule(mm[2], name) {
			continue
		}
		if slices.Contains(hosts, mm[1]) && !have[mm[1]] {
			have[mm[1]] = true
			continue
		}
		if err := m.fw.DeleteAllow(ctx, mariadbPort, "tcp", mm[1]); err != nil {
			return fmt.Errorf("remove the firewall rule for %s: %w", mm[1], err)
		}
	}
	for _, h := range hosts {
		if have[h] {
			continue
		}
		err := m.fw.AddRule(ctx, security.NewRule{
			Action: "allow", Port: strconv.Itoa(mariadbPort), Proto: "tcp", From: h, Comment: comment,
		})
		if err != nil {
			return fmt.Errorf("open port %d to %s: %w", mariadbPort, h, err)
		}
	}
	return nil
}

// writeListenConf writes the listen drop-in while any database allows
// remote access and removes it when none does. A change marks the server
// for a restart.
func (m *Manager) writeListenConf(ctx context.Context) error {
	all, err := m.store.AllDBRemoteHosts(ctx)
	if err != nil {
		return err
	}
	anyRemote, ipv6 := false, false
	for _, hosts := range all {
		for _, h := range hosts {
			anyRemote = true
			ipv6 = ipv6 || strings.Contains(h, ":")
		}
	}
	conf := m.remoteConfPath()
	old, readErr := os.ReadFile(conf)
	switch {
	case !anyRemote && readErr == nil:
		if err := os.Remove(conf); err != nil {
			return err
		}
		m.listenDirty = true
	case anyRemote && (readErr != nil || string(old) != remoteConf(ipv6)):
		if err := fsutil.WriteFileAtomic(conf, []byte(remoteConf(ipv6)), 0o644); err != nil {
			return err
		}
		m.listenDirty = true
	}
	return nil
}

// bindAddress returns where the server listens. An empty value (MariaDB)
// or "*" (MySQL's default) means every interface.
func (m *Manager) bindAddress(ctx context.Context) (string, error) {
	out, err := m.sql(ctx, "SELECT @@global.bind_address;")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// RestartPending reports whether the server must restart before it listens
// where the panel's configuration says.
func (m *Manager) RestartPending(ctx context.Context) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.restartPendingLocked(ctx)
}

func (m *Manager) restartPendingLocked(ctx context.Context) bool {
	if !m.Status(ctx).Running {
		return false // the next start reads the files anyway
	}
	bind, err := m.bindAddress(ctx)
	if err != nil {
		return m.listenDirty // the server is busy, maybe restarting
	}
	if fsutil.Exists(m.remoteConfPath()) {
		return !listensAll(bind)
	}
	// The panel removed its file. The server may also listen everywhere
	// because of the administrator's own settings, so only the restart
	// right after the change counts.
	return m.listenDirty && listensAll(bind)
}

// Restart restarts the server so a new listen setting takes effect. If it
// does not come back with the panel's remote access file, the file is
// removed and the server started again without it, so the websites keep
// working.
func (m *Manager) Restart(ctx context.Context, lg Logger) error {
	if err := m.ready(ctx); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	e := m.engine()
	conf := m.remoteConfPath()
	lg.Logf("Restarting %s", e.Name())
	if _, err := m.run.Output(ctx, "systemctl", "restart", e.service()); err != nil {
		if !fsutil.Exists(conf) {
			return err
		}
		lg.Logf("%s did not start: %v", e.Name(), err)
		lg.Logf("Removing %s and starting %s again", conf, e.Name())
		if rmErr := os.Remove(conf); rmErr != nil {
			return rmErr
		}
		if _, err2 := m.run.Output(ctx, "systemctl", "restart", e.service()); err2 != nil {
			return err2
		}
		m.listenDirty = false
		return fmt.Errorf("%s could not start with remote access, so it was started without it", e.Name())
	}
	m.listenDirty = false
	switch bind, err := m.bindAddress(ctx); {
	case err != nil:
		lg.Logf("%s restarted, but does not answer yet: %v", e.Name(), err)
	case listensAll(bind):
		lg.Logf("%s listens on all interfaces (%s), port %d", e.Name(), bind, mariadbPort)
	default:
		lg.Logf("%s listens on %s only", e.Name(), bind)
	}
	return nil
}

// remoteTwins returns the hosts (in MariaDB form) of a user's remote twins:
// the stored ones and, while the server runs, any panel-made twin it still
// has.
func (m *Manager) remoteTwins(ctx context.Context, d *store.Database, running bool) ([]string, error) {
	hosts, err := m.store.DBRemoteHosts(ctx, d.ID)
	if err != nil {
		return nil, err
	}
	if running {
		existing, err := m.remoteHostsInDB(ctx, d.Username)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, existing...)
	}
	var out []string
	for _, h := range hosts {
		if mh := mariaHost(h); !slices.Contains(out, mh) {
			out = append(out, mh)
		}
	}
	return out, nil
}
