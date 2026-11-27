package postgres

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/security"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/tlsutil"
	"github.com/nefocp/nefo/internal/usererr"
)

// Remote access lets chosen client addresses connect to one database from
// other machines, as for MariaDB, with what PostgreSQL has for it:
//
//   - pg_hba.conf gets one line per address, for that database and its own
//     role only, with scram-sha-256: hostssl when the server has TLS (it
//     then refuses unencrypted connections from those addresses), host
//     otherwise. PostgreSQL 16 and newer read them from nefo-remote.conf
//     next to pg_hba.conf, through one include_if_exists line the panel adds
//     at the end; older ones get a marked block at the end of pg_hba.conf.
//     Nothing else in the file is touched. A change is checked with
//     pg_hba_file_rules before the server reloads it.
//   - conf.d/95-nefo-remote.conf sets listen_addresses = '*' while at least
//     one database allows remote access, and goes when the last one stops;
//     a new listen setting takes a restart. TLS: the cluster's own
//     certificate when it has one (Debian and Ubuntu make clusters with the
//     ssl-cert package's "snakeoil" certificate), otherwise the drop-in
//     turns TLS on with a self-signed certificate the panel makes; that
//     only takes a reload.
//   - ufw opens the cluster's port to each allowed address only (comment
//     "PostgreSQL <database>"), as for MariaDB, also while ufw is off.

const (
	remoteConfName = "95-nefo-remote.conf"
	hbaIncludeName = "nefo-remote.conf"
	// MaxRemoteHosts is the number of client addresses one database may allow.
	MaxRemoteHosts = 10
	// The widest ranges allowed: /16 for IPv4 (65,536 addresses) and /48
	// for IPv6 (a site's allocation).
	minRemotePrefix4 = 16
	minRemotePrefix6 = 48
	panelCertName    = "nefo-ssl.crt"
	panelKeyName     = "nefo-ssl.key"
	snakeoilCert     = "/etc/ssl/certs/ssl-cert-snakeoil.pem"
	// hbaFirstInclude is the first major version whose pg_hba.conf can
	// include other files.
	hbaFirstInclude = 16
)

// The marks of the panel's lines in pg_hba.conf.
const (
	hbaBegin          = "# BEGIN Nefo remote access: managed by the panel, do not edit between these lines."
	hbaEnd            = "# END Nefo remote access"
	hbaIncludeComment = "# Nefo remote access (managed by the panel): the allowed addresses are in " + hbaIncludeName + "."
	hbaIncludeLine    = "include_if_exists " + hbaIncludeName
)

// Firewall is the part of the firewall manager that remote access needs;
// *security.Manager implements it.
type Firewall interface {
	Firewall(ctx context.Context) security.Firewall
	AddRule(ctx context.Context, r security.NewRule) error
	DeleteRule(ctx context.Context, n int, to, from string) error
	DeleteAllow(ctx context.Context, port int, proto, from string) error
}

// SetFirewall gives the manager the firewall used for remote access.
func (m *Manager) SetFirewall(fw Firewall) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fw = fw
}

// ParseRemoteHosts checks and normalises client addresses: IPv4 or IPv6
// addresses, or ranges in CIDR form no wider than /16 (IPv4) or /48
// (IPv6). Empty entries and duplicates are dropped. The result is what the
// panel stores and what ufw gets ("203.0.113.5", "10.1.0.0/16",
// "2001:db8::1", "2001:db8:5::/48").
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
	if err != nil || p.Addr().Zone() != "" {
		return "", usererr.New("invalid_remote_host", s)
	}
	if p.Addr().Is4In6() {
		return "", usererr.New("invalid_remote_host", s)
	}
	if p.IsSingleIP() {
		return checkRemoteAddr(p.Addr(), s)
	}
	min := minRemotePrefix4
	if p.Addr().Is6() {
		min = minRemotePrefix6
	}
	if p.Bits() < min {
		return "", usererr.New("pg_remote_host_too_wide", s) // includes 0.0.0.0/0 and ::/0
	}
	p = p.Masked()
	if a := p.Addr(); a.IsUnspecified() || a.IsLoopback() || a.IsMulticast() || a.IsLinkLocalUnicast() {
		return "", usererr.New("invalid_remote_host", s)
	}
	return p.String(), nil
}

func checkRemoteAddr(a netip.Addr, s string) (string, error) {
	if a.Zone() != "" {
		return "", usererr.New("invalid_remote_host", s)
	}
	a = a.Unmap()
	if a.IsUnspecified() || a.IsLoopback() || a.IsMulticast() || a.IsLinkLocalUnicast() {
		// localhost has its own lines; 0.0.0.0 and :: match nobody.
		return "", usererr.New("invalid_remote_host", s)
	}
	return a.String(), nil
}

// hbaAddress turns a normalised client address into pg_hba.conf's CIDR form.
func hbaAddress(h string) (string, error) {
	if p, err := netip.ParsePrefix(h); err == nil {
		return p.Masked().String(), nil
	}
	a, err := netip.ParseAddr(h)
	if err != nil {
		return "", fmt.Errorf("invalid address %q", h)
	}
	return netip.PrefixFrom(a, a.BitLen()).String(), nil
}

// remoteEntry is one database's remote access.
type remoteEntry struct {
	db, user string
	hosts    []string
}

// hbaLines returns the panel's pg_hba.conf lines for the entries.
func hbaLines(entries []remoteEntry, ssl bool) ([]string, error) {
	typ := "host"
	if ssl {
		typ = "hostssl"
	}
	var lines []string
	for _, e := range entries {
		db, err := ident(e.db)
		if err != nil {
			return nil, err
		}
		user, err := ident(e.user)
		if err != nil {
			return nil, err
		}
		for _, h := range e.hosts {
			addr, err := hbaAddress(h)
			if err != nil {
				return nil, err
			}
			lines = append(lines, fmt.Sprintf("%-7s %s %s %s scram-sha-256", typ, db, user, addr))
		}
	}
	return lines, nil
}

// rewriteHBA returns pg_hba.conf with the panel's remote access lines
// replaced: an include line (include) or a block with them, where the
// panel's lines were before (rules are read in order, so a line of the
// administrator's that came after them stays after them), or at the end
// when there were none. Without lines, the panel's are taken out with the
// empty line it put before them. Every other line stays as it was.
func rewriteHBA(old string, lines []string, include bool) (string, error) {
	if old == "" {
		old = "\n"
	}
	src := strings.Split(strings.TrimSuffix(old, "\n"), "\n")
	out := make([]string, 0, len(src)+len(lines)+3)
	pos, inBlock := -1, false
	for _, l := range src {
		t := strings.TrimSpace(l)
		switch {
		case inBlock:
			if t == hbaEnd {
				inBlock = false
			}
			continue
		case t == hbaBegin:
			if pos < 0 {
				pos = len(out)
			}
			inBlock = true
			continue
		case t == hbaEnd:
			return "", errors.New("pg_hba.conf has the panel's END line without its BEGIN line: fix the file by hand")
		case t == hbaIncludeComment || t == hbaIncludeLine:
			if pos < 0 {
				pos = len(out)
			}
			continue
		}
		out = append(out, l)
	}
	if inBlock {
		return "", errors.New("pg_hba.conf has the panel's BEGIN line without its END line: fix the file by hand")
	}
	var section []string
	if len(lines) > 0 {
		if include {
			section = []string{hbaIncludeComment, hbaIncludeLine}
		} else {
			section = append(append([]string{hbaBegin}, lines...), hbaEnd)
		}
	}
	switch {
	case pos < 0 && section == nil:
		return old, nil
	case pos < 0:
		for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
			out = out[:len(out)-1]
		}
		out = append(append(out, ""), section...)
	case section == nil:
		// The empty line before the panel's lines goes with them, when
		// nothing or another empty line follows.
		if pos > 0 && strings.TrimSpace(out[pos-1]) == "" && (pos == len(out) || strings.TrimSpace(out[pos]) == "") {
			out = slices.Delete(out, pos-1, pos)
		}
	default:
		out = slices.Insert(out, pos, section...)
	}
	return strings.Join(out, "\n") + "\n", nil
}

// hbaInclude is the content of nefo-remote.conf.
func hbaInclude(lines []string) string {
	return "# Managed by Nefo. Do not edit: the panel rewrites this file from the\n" +
		"# remote access settings of each database on the Databases page.\n" +
		"# TYPE  DATABASE  USER  ADDRESS  METHOD\n" + strings.Join(lines, "\n") + "\n"
}

// remoteConf is the content of the listen drop-in; certDir is where the
// panel's own certificate is, when the drop-in also turns TLS on.
func remoteConf(certDir string) string {
	s := `# Managed by Nefo. Do not edit: the panel removes this file when no
# database allows remote access. pg_hba.conf and ufw let only the allowed
# addresses in. A change to listen_addresses takes a restart.
listen_addresses = '*'
`
	if certDir != "" {
		s += "# The cluster had no TLS certificate: this one is self-signed, made by the panel.\n" +
			"ssl = on\n" +
			"ssl_cert_file = '" + filepath.Join(certDir, panelCertName) + "'\n" +
			"ssl_key_file = '" + filepath.Join(certDir, panelKeyName) + "'\n"
	}
	return s
}

// remoteConfPath is the listen drop-in of a cluster.
func (m *Manager) remoteConfPath(c cluster) string {
	return filepath.Join(m.mainDir(c.version), "conf.d", remoteConfName)
}

// show returns the value of a setting of the running server.
func (m *Manager) show(ctx context.Context, c cluster, name string) (string, error) {
	if !settingRe.MatchString(name) {
		return "", fmt.Errorf("invalid setting %q", name)
	}
	out, err := m.sql(ctx, c, "postgres", "SHOW "+name+";\n")
	return strings.TrimSpace(out), err
}

var settingRe = regexp.MustCompile(`^[a-z_]{2,40}$`)

// fileSetting returns the value the configuration files give a setting now
// (the last one wins, postgresql.auto.conf included), as the server would
// read them at its next start, or "" when no file sets it.
func (m *Manager) fileSetting(ctx context.Context, c cluster, name string) (string, error) {
	lit, err := literal(name)
	if err != nil || !settingRe.MatchString(name) {
		return "", fmt.Errorf("invalid setting %q", name)
	}
	out, err := m.sql(ctx, c, "postgres", "SELECT setting FROM pg_file_settings WHERE name = "+lit+" ORDER BY seqno DESC LIMIT 1;\n")
	return strings.TrimSpace(out), err
}

// owned writes a file owned like the cluster's postgresql.conf (by
// postgres), so the server can read it.
func (m *Manager) owned(c cluster, path string, data []byte, mode os.FileMode) error {
	if err := fsutil.WriteFileAtomic(path, data, mode); err != nil {
		return err
	}
	if fi, err := os.Stat(filepath.Join(m.mainDir(c.version), "postgresql.conf")); err == nil {
		if uid, gid := fsutil.Owner(fi); uid >= 0 {
			return os.Chown(path, uid, gid)
		}
	}
	return nil
}

// remoteEntries returns the stored remote access of every PostgreSQL
// database, sorted by name.
func (m *Manager) remoteEntries(ctx context.Context) ([]remoteEntry, error) {
	all, err := m.store.AllPGRemoteHosts(ctx)
	if err != nil {
		return nil, err
	}
	var out []remoteEntry
	for id, hosts := range all {
		d, err := m.store.PGDatabaseByID(ctx, id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				continue
			}
			return nil, err
		}
		out = append(out, remoteEntry{d.Name, d.Username, hosts})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].db < out[j].db })
	return out, nil
}

// applyRemote makes the cluster's files match the stored remote access of
// every database: the listen drop-in (and TLS), then pg_hba.conf, checked
// and reloaded. The server must be running; m.mu is held.
func (m *Manager) applyRemote(ctx context.Context, c cluster) error {
	entries, err := m.remoteEntries(ctx)
	if err != nil {
		return err
	}
	conf := m.remoteConfPath(c)
	old, readErr := os.ReadFile(conf)

	ssl := false
	if len(entries) > 0 {
		certDir := ""
		switch {
		case readErr == nil && strings.Contains(string(old), "\nssl = on\n"):
			certDir = m.mainDir(c.version) // the panel turned TLS on before: keep it
		default:
			if v, err := m.show(ctx, c, "ssl"); err == nil && v != "on" {
				certDir = m.mainDir(c.version)
			}
		}
		if certDir != "" {
			if err := m.panelCert(c); err != nil {
				return fmt.Errorf("make a TLS certificate for PostgreSQL: %w", err)
			}
		}
		want := remoteConf(certDir)
		if readErr != nil || string(old) != want {
			if err := fsutil.MkdirAll(filepath.Dir(conf), 0o755); err != nil {
				return err
			}
			if err := m.owned(c, conf, []byte(want), 0o644); err != nil {
				return err
			}
		}
		// TLS only takes a reload: read it again with the drop-in in place.
		if _, err := m.sql(ctx, c, "postgres", "SELECT pg_reload_conf();\n"); err != nil {
			return err
		}
		ssl = m.waitSSL(ctx, c)
	} else if readErr == nil {
		if err := os.Remove(conf); err != nil {
			return err
		}
	}
	if err := m.writeHBA(ctx, c, entries, ssl); err != nil {
		return err
	}
	_, err = m.sql(ctx, c, "postgres", "SELECT pg_reload_conf();\n")
	return err
}

// waitSSL reports whether the server has TLS on, giving a reload a moment.
func (m *Manager) waitSSL(ctx context.Context, c cluster) bool {
	for i := 0; i < 10; i++ {
		if v, err := m.show(ctx, c, "ssl"); err == nil && v == "on" {
			return true
		}
		if m.wait == 0 {
			break
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(m.wait / 4):
		}
	}
	return false
}

// panelCert makes the panel's self-signed certificate for the cluster, or
// renews it near its expiry, owned by postgres (the key 0600).
func (m *Manager) panelCert(c cluster) error {
	dir := m.mainDir(c.version)
	crt, key := filepath.Join(dir, panelCertName), filepath.Join(dir, panelKeyName)
	if _, err := tlsutil.EnsureSelfSigned(crt, key); err != nil {
		return err
	}
	if fi, err := os.Stat(filepath.Join(dir, "postgresql.conf")); err == nil {
		if uid, gid := fsutil.Owner(fi); uid >= 0 {
			for _, p := range []string{crt, key} {
				if err := os.Chown(p, uid, gid); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// hbaFile returns the cluster's pg_hba.conf, as the server says.
func (m *Manager) hbaFile(ctx context.Context, c cluster) string {
	if v, err := m.show(ctx, c, "hba_file"); err == nil && filepath.IsAbs(v) {
		return v
	}
	return filepath.Join(m.mainDir(c.version), "pg_hba.conf")
}

// writeHBA writes the panel's lines into pg_hba.conf (and nefo-remote.conf
// from PostgreSQL 16), checks the result with pg_hba_file_rules and puts
// the old files back if PostgreSQL finds an error.
func (m *Manager) writeHBA(ctx context.Context, c cluster, entries []remoteEntry, ssl bool) error {
	lines, err := hbaLines(entries, ssl)
	if err != nil {
		return err
	}
	path := m.hbaFile(ctx, c)
	incPath := filepath.Join(filepath.Dir(path), hbaIncludeName)
	old, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	oldInc, incErr := os.ReadFile(incPath)
	include := c.major() >= hbaFirstInclude
	main, err := rewriteHBA(string(old), lines, include)
	if err != nil {
		return err
	}
	wantInc := include && len(lines) > 0
	incSame := (wantInc && incErr == nil && string(oldInc) == hbaInclude(lines)) || (!wantInc && incErr != nil)
	if main == string(old) && incSame {
		return nil
	}
	restore := func() {
		m.owned(c, path, old, 0o640)
		if incErr == nil {
			m.owned(c, incPath, oldInc, 0o640)
		} else {
			os.Remove(incPath)
		}
	}
	if wantInc {
		if err := m.owned(c, incPath, []byte(hbaInclude(lines)), 0o640); err != nil {
			return err
		}
	} else if incErr == nil {
		if err := os.Remove(incPath); err != nil {
			restore()
			return err
		}
	}
	if main != string(old) {
		if err := m.owned(c, path, []byte(main), 0o640); err != nil {
			restore()
			return err
		}
	}
	out, err := m.sql(ctx, c, "postgres", "SELECT line_number || ': ' || error FROM pg_hba_file_rules WHERE error IS NOT NULL LIMIT 1;\n")
	if err == nil && strings.TrimSpace(out) != "" {
		restore()
		return fmt.Errorf("PostgreSQL refuses the new pg_hba.conf (line %s), so it was put back as it was", strings.TrimSpace(out))
	}
	if err != nil {
		restore()
		return err
	}
	return nil
}

// RemoteAccess is the remote access state of one database.
type RemoteAccess struct {
	Hosts []string `json:"hosts"`
	// ListenAddresses is where the server listens now; Listening means it
	// accepts connections from other machines.
	ListenAddresses string `json:"listen_addresses"`
	Listening       bool   `json:"listening"`
	// RestartPending means the server must restart before the listen
	// setting takes effect.
	RestartPending  bool `json:"restart_pending"`
	FirewallActive  bool `json:"firewall_active"`
	FirewallPresent bool `json:"firewall_installed"`
	Port            int  `json:"port"`
	// SSL is whether the server has TLS on; then the allowed addresses
	// must use it (hostssl lines), which Encrypted says for this database.
	SSL       bool `json:"ssl"`
	Encrypted bool `json:"encrypted"`
	// Certificate is "snakeoil" (the distribution's self-signed one),
	// "panel" (made by the panel) or "custom", with its file.
	Certificate string `json:"certificate,omitempty"`
	CertFile    string `json:"cert_file,omitempty"`
	// HBAFile is the pg_hba.conf the panel's lines go into, HBAInclude
	// whether they are in nefo-remote.conf next to it (PostgreSQL 16+)
	// rather than in a marked block.
	HBAFile    string `json:"hba_file,omitempty"`
	HBAInclude bool   `json:"hba_include"`
}

// Remote returns the remote access state of a database.
func (m *Manager) Remote(ctx context.Context, id int64) (*RemoteAccess, error) {
	d, err := m.store.PGDatabaseByID(ctx, id)
	if err != nil {
		return nil, err
	}
	hosts, err := m.store.PGRemoteHosts(ctx, d.ID)
	if err != nil {
		return nil, err
	}
	ra := &RemoteAccess{Hosts: hosts, Port: defaultPort}
	if c, err := m.ready(ctx); err == nil {
		ra.Port = c.port
		ra.HBAInclude = c.major() >= hbaFirstInclude
		ra.HBAFile = m.hbaFile(ctx, c)
		if v, err := m.show(ctx, c, "listen_addresses"); err == nil {
			ra.ListenAddresses, ra.Listening = v, listensAll(v)
		}
		if v, err := m.show(ctx, c, "ssl"); err == nil && v == "on" {
			ra.SSL = true
			if f, err := m.show(ctx, c, "ssl_cert_file"); err == nil {
				ra.CertFile = f
				switch {
				case f == snakeoilCert:
					ra.Certificate = "snakeoil"
				case filepath.Base(f) == panelCertName:
					ra.Certificate = "panel"
				default:
					ra.Certificate = "custom"
				}
			}
		}
		ra.Encrypted = ra.SSL && len(hosts) > 0 && m.hbaEncrypted(ctx, c, d.Name)
		ra.RestartPending = m.restartPending(ctx, c)
	}
	m.mu.Lock()
	fw := m.fw
	m.mu.Unlock()
	if fw != nil {
		f := fw.Firewall(ctx)
		ra.FirewallPresent, ra.FirewallActive = f.Installed, f.Active
	}
	return ra, nil
}

// hbaEncrypted reports whether the panel's pg_hba.conf lines for a
// database all require TLS (hostssl).
func (m *Manager) hbaEncrypted(ctx context.Context, c cluster, name string) bool {
	path := m.hbaFile(ctx, c)
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	if c.major() >= hbaFirstInclude {
		if b, err = os.ReadFile(filepath.Join(filepath.Dir(path), hbaIncludeName)); err != nil {
			return false
		}
	}
	db := `"` + name + `"`
	found := false
	for _, l := range strings.Split(string(b), "\n") {
		f := strings.Fields(l)
		if len(f) < 5 || f[1] != db || (f[0] != "host" && f[0] != "hostssl") {
			continue
		}
		if f[0] != "hostssl" {
			return false
		}
		found = true
	}
	return found
}

func listensAll(v string) bool {
	for _, a := range strings.Split(v, ",") {
		switch strings.TrimSpace(a) {
		case "*", "0.0.0.0", "::":
			return true
		}
	}
	return false
}

// restartPending reports whether the server listens elsewhere than its
// configuration files say, so it must restart.
func (m *Manager) restartPending(ctx context.Context, c cluster) bool {
	now, err := m.show(ctx, c, "listen_addresses")
	if err != nil {
		return false
	}
	want, err := m.fileSetting(ctx, c, "listen_addresses")
	if err != nil || want == "" {
		return false
	}
	return normList(now) != normList(want)
}

func normList(s string) string {
	f := strings.Split(s, ",")
	for i := range f {
		f[i] = strings.TrimSpace(f[i])
	}
	return strings.Join(f, ",")
}

// RestartPending reports whether PostgreSQL must restart before it listens
// where the panel's configuration says.
func (m *Manager) RestartPending(ctx context.Context) bool {
	c, err := m.ready(ctx)
	if err != nil {
		return false
	}
	return m.restartPending(ctx, c)
}

// SetRemote replaces the client addresses that may reach a database from
// other machines; an empty list turns remote access off. It reports whether
// the server must now restart to change where it listens (see Restart).
func (m *Manager) SetRemote(ctx context.Context, id int64, hosts []string) (restart bool, err error) {
	hosts, err = ParseRemoteHosts(hosts)
	if err != nil {
		return false, err
	}
	c, err := m.ready(ctx)
	if err != nil {
		return false, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fw == nil {
		return false, errors.New("the panel has no firewall manager for remote access")
	}
	if len(hosts) > 0 && !m.fw.Firewall(ctx).Installed {
		return false, usererr.New("firewall_not_installed")
	}
	d, err := m.store.PGDatabaseByID(ctx, id)
	if err != nil {
		return false, err
	}
	if err := checkName(d.Name); err != nil {
		return false, err
	}
	prev, err := m.store.PGRemoteHosts(ctx, d.ID)
	if err != nil {
		return false, err
	}
	// Save first: whatever fails below, the stored list says what the
	// administrator wants, and the next change or delete cleans up after it.
	if err := m.store.SetPGRemoteHosts(ctx, d.ID, hosts); err != nil {
		return false, err
	}
	if err := m.applyRemote(ctx, c); err != nil {
		// Back to what was there: the stored list, then the files from it.
		ctx := context.WithoutCancel(ctx)
		if m.store.SetPGRemoteHosts(ctx, d.ID, prev) == nil {
			m.applyRemote(ctx, c)
		}
		return false, err
	}
	if err := m.syncRemoteFirewall(ctx, d.Name, c.port, hosts); err != nil {
		return false, err
	}
	return m.restartPending(ctx, c), nil
}

// removeRemote ends the remote access of a database that is being deleted:
// its ufw rules go, and while the server runs its pg_hba lines too. m.mu
// is held.
func (m *Manager) removeRemote(ctx context.Context, d *store.Database, c cluster, running bool) error {
	if err := m.store.SetPGRemoteHosts(ctx, d.ID, nil); err != nil {
		return err
	}
	if running {
		if err := m.applyRemote(ctx, c); err != nil {
			return err
		}
	}
	return m.syncRemoteFirewall(ctx, d.Name, c.port, nil)
}

// firewallComment marks the ufw rules of a database.
func firewallComment(name string) string { return "PostgreSQL " + name }

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
// comment) allow the cluster's port from exactly the given addresses. A
// rule for another port (the cluster moved) is removed. m.mu is held.
func (m *Manager) syncRemoteFirewall(ctx context.Context, name string, port int, hosts []string) error {
	if m.fw == nil {
		if len(hosts) > 0 {
			return errors.New("the panel has no firewall manager for remote access")
		}
		return nil
	}
	if port == 0 {
		port = defaultPort
	}
	comment := firewallComment(name)
	if fw := m.fw.Firewall(ctx); fw.Installed && !fw.Active {
		return m.syncRemoteFirewallOff(ctx, fw, comment, port, hosts)
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
			if r.Comment != comment {
				continue
			}
			from := ruleFrom(r.From)
			if rulePort(r.To) == port && slices.Contains(hosts, from) && !have[from] {
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
				Action: "allow", Port: strconv.Itoa(port), Proto: "tcp", From: h, Comment: comment,
			})
			if err != nil {
				return fmt.Errorf("open port %d to %s: %w", port, h, err)
			}
		}
		return nil
	}
	return errors.New("the firewall rules kept changing")
}

// addedRuleRe matches a remote access rule as "ufw show added" lists it
// while ufw is off.
var addedRuleRe = regexp.MustCompile(`^allow from (\S+) to any port ([0-9]{1,5}) proto tcp comment '([^']*)'$`)

// syncRemoteFirewallOff is syncRemoteFirewall while ufw is off: its rules
// have no numbers then, so a stale one is deleted by what it says, before
// turning the firewall on could make it live.
func (m *Manager) syncRemoteFirewallOff(ctx context.Context, fw security.Firewall, comment string, port int, hosts []string) error {
	have := map[string]bool{}
	for _, line := range fw.Added {
		mm := addedRuleRe.FindStringSubmatch(strings.TrimSpace(line))
		if mm == nil || mm[3] != comment {
			continue
		}
		p, _ := strconv.Atoi(mm[2])
		if p == port && slices.Contains(hosts, mm[1]) && !have[mm[1]] {
			have[mm[1]] = true
			continue
		}
		if err := m.fw.DeleteAllow(ctx, p, "tcp", mm[1]); err != nil {
			return fmt.Errorf("remove the firewall rule for %s: %w", mm[1], err)
		}
	}
	for _, h := range hosts {
		if have[h] {
			continue
		}
		err := m.fw.AddRule(ctx, security.NewRule{
			Action: "allow", Port: strconv.Itoa(port), Proto: "tcp", From: h, Comment: comment,
		})
		if err != nil {
			return fmt.Errorf("open port %d to %s: %w", port, h, err)
		}
	}
	return nil
}
