package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/security"
	"github.com/nefocp/nefo/internal/store"
)

// fakeFirewall keeps ufw rules in memory, numbered from 1 like ufw.
type fakeFirewall struct {
	installed, active bool
	rules             []security.Rule
	added             []string // while off: "ufw show added" lines
}

func (f *fakeFirewall) Firewall(ctx context.Context) security.Firewall {
	fw := security.Firewall{Installed: f.installed, Active: f.active, Rules: []security.Rule{}, Added: slices.Clone(f.added)}
	for i, r := range f.rules {
		r.Number = i + 1
		fw.Rules = append(fw.Rules, r)
	}
	return fw
}

func (f *fakeFirewall) AddRule(ctx context.Context, r security.NewRule) error {
	if r.Action != "allow" || r.Proto != "tcp" || r.From == "" {
		return fmt.Errorf("unexpected rule %+v", r)
	}
	if !f.active {
		f.added = append(f.added, fmt.Sprintf("allow from %s to any port %s proto tcp comment '%s'", r.From, r.Port, r.Comment))
		return nil
	}
	to := r.Port + "/" + r.Proto
	if strings.Contains(r.From, ":") {
		to += " (v6)"
	}
	f.rules = append(f.rules, security.Rule{To: to, Action: "ALLOW", From: r.From, Comment: r.Comment})
	return nil
}

func (f *fakeFirewall) DeleteAllow(ctx context.Context, port int, proto, from string) error {
	prefix := fmt.Sprintf("allow from %s to any port %d proto %s ", from, port, proto)
	n := len(f.added)
	f.added = slices.DeleteFunc(f.added, func(l string) bool { return strings.HasPrefix(l, prefix) })
	if len(f.added) == n {
		return fmt.Errorf("no rule %q", prefix)
	}
	return nil
}

func (f *fakeFirewall) DeleteRule(ctx context.Context, n int, to, from string) error {
	if n < 1 || n > len(f.rules) || f.rules[n-1].To != to || f.rules[n-1].From != from {
		return fmt.Errorf("rule %d changed", n)
	}
	f.rules = slices.Delete(f.rules, n-1, n)
	return nil
}

// rules returns "port from" for the rules with a comment.
func (f *fakeFirewall) list(comment string) []string {
	var out []string
	for _, r := range f.rules {
		if r.Comment == comment {
			out = append(out, fmt.Sprint(rulePort(r.To)), r.From)
		}
	}
	for _, l := range f.added {
		if mm := addedRuleRe.FindStringSubmatch(l); mm != nil && mm[3] == comment {
			out = append(out, mm[2], mm[1])
		}
	}
	return out
}

// fakePG answers the SQL that remote access sends, from the cluster's
// files, like a running server: TLS follows a reload, listen_addresses a
// restart.
type fakePG struct {
	m       *Manager
	version string
	listen  string // what the running server listens on
	ssl     string // "on" or "off"
	hbaErr  string // what pg_hba_file_rules reports
	noSSL   bool   // a reload never turns TLS on
	reloads int
}

func (p *fakePG) dropIn() string {
	b, _ := os.ReadFile(filepath.Join(p.m.mainDir(p.version), "conf.d", remoteConfName))
	return string(b)
}

func (p *fakePG) fileListen() string {
	if strings.Contains(p.dropIn(), "listen_addresses = '*'") {
		return "*"
	}
	return "localhost"
}

func (p *fakePG) sql(stdin string) (string, error) {
	switch {
	case stdin == "SHOW ssl;\n":
		return p.ssl + "\n", nil
	case stdin == "SHOW ssl_cert_file;\n":
		if strings.Contains(p.dropIn(), panelCertName) {
			return filepath.Join(p.m.mainDir(p.version), panelCertName) + "\n", nil
		}
		return snakeoilCert + "\n", nil
	case stdin == "SHOW hba_file;\n":
		return filepath.Join(p.m.mainDir(p.version), "pg_hba.conf") + "\n", nil
	case stdin == "SHOW listen_addresses;\n":
		return p.listen + "\n", nil
	case strings.HasPrefix(stdin, "SELECT setting FROM pg_file_settings WHERE name = 'listen_addresses'"):
		return p.fileListen() + "\n", nil
	case strings.Contains(stdin, "pg_hba_file_rules"):
		return p.hbaErr, nil
	case stdin == "SELECT pg_reload_conf();\n":
		p.reloads++
		if strings.Contains(p.dropIn(), "ssl = on") && !p.noSSL {
			p.ssl = "on"
		}
		return "t\n", nil
	}
	return "", nil
}

// newRemoteManager returns a test manager for PostgreSQL version with the
// panel's pg_hba.conf, a fake server and an active firewall.
func newRemoteManager(t *testing.T, version string) (*Manager, *fakeRunner, *store.Store, *fakePG, *fakeFirewall) {
	t.Helper()
	m, f, st := newTestManager(t)
	if version != "16" {
		os.RemoveAll(m.libDir)
		os.RemoveAll(m.etcDir)
		for _, p := range []string{filepath.Join(m.libDir, version, "bin", "postgres"), filepath.Join(m.mainDir(version), "postgresql.conf")} {
			os.MkdirAll(filepath.Dir(p), 0o755)
			os.WriteFile(p, []byte("port = 5432\n"), 0o644)
		}
	}
	if err := os.WriteFile(filepath.Join(m.mainDir(version), "pg_hba.conf"), []byte(hbaConf), 0o640); err != nil {
		t.Fatal(err)
	}
	pg := &fakePG{m: m, version: version, listen: "localhost", ssl: "on"}
	f.sql = pg.sql
	f.output = func(name string, args []string) (string, error) {
		switch {
		case strings.HasSuffix(name, "/postgres") && len(args) == 1 && args[0] == "--version":
			return "postgres (PostgreSQL) " + version + ".1 (Ubuntu " + version + ".1-1)\n", nil
		case name == "systemctl" && len(args) == 2 && args[0] == "is-active":
			return "active\n", nil
		case name == "systemctl" && len(args) == 2 && args[0] == "restart":
			pg.listen = pg.fileListen()
		}
		return "", nil
	}
	fw := &fakeFirewall{installed: true, active: true}
	m.SetFirewall(fw)
	return m, f, st, pg, fw
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestParseRemoteHosts(t *testing.T) {
	got, err := ParseRemoteHosts([]string{" 203.0.113.5 ", "", "10.1.2.3/16", "2001:DB8::1", "::ffff:203.0.113.9",
		"203.0.113.5/32", "203.0.113.5", "192.168.7.200/24", "2001:db8::2/128", "2001:db8:5:6::/48", "2001:db8:7::/64"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"203.0.113.5", "10.1.0.0/16", "2001:db8::1", "203.0.113.9", "192.168.7.0/24", "2001:db8::2", "2001:db8:5::/48", "2001:db8:7::/64"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for bad, code := range map[string]string{
		"0.0.0.0/0": "pg_remote_host_too_wide", "::/0": "pg_remote_host_too_wide", "10.0.0.0/8": "pg_remote_host_too_wide",
		"2001:db8::/32": "pg_remote_host_too_wide", "127.0.0.1": "invalid_remote_host", "::1": "invalid_remote_host",
		"fe80::1": "invalid_remote_host", "fe80::/64": "invalid_remote_host", "%": "invalid_remote_host", "shop.example": "invalid_remote_host",
		"1.2.3.4' OR 1=1": "invalid_remote_host", "fe80::1%eth0": "invalid_remote_host", "224.0.0.1": "invalid_remote_host",
		"203.0.113.5 scram": "invalid_remote_host",
	} {
		if _, err := ParseRemoteHosts([]string{bad}); userCode(err) != code {
			t.Errorf("%q: %v, want %s", bad, err, code)
		}
	}
	var many []string
	for i := 1; i <= 11; i++ {
		many = append(many, fmt.Sprintf("203.0.113.%d", i))
	}
	if _, err := ParseRemoteHosts(many); userCode(err) != "too_many_remote_hosts" {
		t.Fatalf("11 addresses: %v", err)
	}
}

func TestHBALines(t *testing.T) {
	lines, err := hbaLines([]remoteEntry{{"shop", "shop", []string{"203.0.113.5", "10.1.0.0/16", "2001:db8::1", "2001:db8:5::/48"}}}, true)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`hostssl "shop" "shop" 203.0.113.5/32 scram-sha-256`,
		`hostssl "shop" "shop" 10.1.0.0/16 scram-sha-256`,
		`hostssl "shop" "shop" 2001:db8::1/128 scram-sha-256`,
		`hostssl "shop" "shop" 2001:db8:5::/48 scram-sha-256`,
	}
	if !slices.Equal(lines, want) {
		t.Fatalf("%q", lines)
	}
	if l, _ := hbaLines([]remoteEntry{{"shop", "shop", []string{"203.0.113.5"}}}, false); l[0] != `host    "shop" "shop" 203.0.113.5/32 scram-sha-256` {
		t.Fatalf("%q", l)
	}
	for _, e := range []remoteEntry{{"all", "shop", []string{"203.0.113.5"}}, {"shop", "postgres", []string{"203.0.113.5"}}, {"shop", "shop", []string{"203.0.113.5 trust"}}} {
		if l, err := hbaLines([]remoteEntry{e}, true); err == nil {
			t.Errorf("%+v accepted: %q", e, l)
		}
	}
}

func TestRewriteHBA(t *testing.T) {
	lines := []string{`hostssl "shop" "shop" 203.0.113.5/32 scram-sha-256`}
	// A marked block at the end, replaced in place, and gone again.
	got, err := rewriteHBA(hbaConf, lines, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := hbaConf + "\n" + hbaBegin + "\n" + lines[0] + "\n" + hbaEnd + "\n"; got != want {
		t.Fatalf("block:\n%s", got)
	}
	admin := got + "host all all 198.51.100.0/24 reject\n"
	two := append(lines, `hostssl "blog" "blog" 192.0.2.1/32 scram-sha-256`)
	got2, err := rewriteHBA(admin, two, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := hbaConf + "\n" + hbaBegin + "\n" + two[0] + "\n" + two[1] + "\n" + hbaEnd + "\nhost all all 198.51.100.0/24 reject\n"; got2 != want {
		t.Fatalf("an administrator's later line must stay after the block:\n%s", got2)
	}
	if back, _ := rewriteHBA(got, nil, false); back != hbaConf {
		t.Fatalf("not back to the original:\n%q", back)
	}
	if same, _ := rewriteHBA(hbaConf, nil, true); same != hbaConf {
		t.Fatalf("changed without lines:\n%q", same)
	}
	// From PostgreSQL 16, one include line; a block from before is replaced by it.
	inc, err := rewriteHBA(got, lines, true)
	if err != nil {
		t.Fatal(err)
	}
	if want := hbaConf + "\n" + hbaIncludeComment + "\n" + hbaIncludeLine + "\n"; inc != want {
		t.Fatalf("include:\n%s", inc)
	}
	if again, _ := rewriteHBA(inc, two, true); again != inc {
		t.Fatalf("include line written twice:\n%s", again)
	}
	if back, _ := rewriteHBA(inc, nil, true); back != hbaConf {
		t.Fatalf("include not removed:\n%q", back)
	}
	for _, bad := range []string{hbaConf + hbaBegin + "\nhost x\n", hbaConf + hbaEnd + "\n"} {
		if _, err := rewriteHBA(bad, lines, false); err == nil {
			t.Errorf("damaged block accepted:\n%s", bad)
		}
	}
}

// PostgreSQL 16: lines in nefo-remote.conf, TLS from the cluster's own
// certificate, the firewall opened and closed, the listen drop-in written
// and removed, each needing a restart.
func TestRemoteAccess16(t *testing.T) {
	ctx := context.Background()
	m, _, st, pg, fw := newRemoteManager(t, "16")
	d := addPGDatabase(t, st, "shop", "Shop-Password-1")
	other := addPGDatabase(t, st, "blog", "Blog-Password-1")
	dir := m.mainDir("16")

	restart, err := m.SetRemote(ctx, d.ID, []string{"203.0.113.5", "2001:db8:5::/48"})
	if err != nil {
		t.Fatal(err)
	}
	if !restart {
		t.Fatal("no restart for a new listen setting")
	}
	if got := readFile(t, filepath.Join(dir, "conf.d", remoteConfName)); !strings.Contains(got, "listen_addresses = '*'") || strings.Contains(got, "ssl = on") {
		t.Fatalf("drop-in:\n%s", got)
	}
	if got := readFile(t, filepath.Join(dir, "pg_hba.conf")); got != hbaConf+"\n"+hbaIncludeComment+"\n"+hbaIncludeLine+"\n" {
		t.Fatalf("pg_hba.conf:\n%s", got)
	}
	inc := readFile(t, filepath.Join(dir, hbaIncludeName))
	for _, want := range []string{`hostssl "shop" "shop" 203.0.113.5/32 scram-sha-256`, `hostssl "shop" "shop" 2001:db8:5::/48 scram-sha-256`} {
		if !strings.Contains(inc, want+"\n") {
			t.Errorf("%s lacks %q:\n%s", hbaIncludeName, want, inc)
		}
	}
	if got := fw.list("PostgreSQL shop"); !slices.Equal(got, []string{"5432", "203.0.113.5", "5432", "2001:db8:5::/48"}) {
		t.Fatalf("firewall: %v", got)
	}
	ra, err := m.Remote(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ra.RestartPending || ra.Listening || !ra.SSL || !ra.Encrypted || ra.Certificate != "snakeoil" || !ra.HBAInclude || ra.Port != 5432 {
		t.Fatalf("state: %+v", ra)
	}
	if err := m.Restart(ctx, testLog{t}); err != nil {
		t.Fatal(err)
	}
	if ra, _ := m.Remote(ctx, d.ID); ra.RestartPending || !ra.Listening || ra.ListenAddresses != "*" {
		t.Fatalf("after the restart: %+v", ra)
	}
	// Another database: its own lines and rules, no restart.
	if restart, err := m.SetRemote(ctx, other.ID, []string{"192.0.2.7"}); err != nil || restart {
		t.Fatalf("second database: %v %v", restart, err)
	}
	if inc := readFile(t, filepath.Join(dir, hbaIncludeName)); !strings.Contains(inc, `hostssl "blog" "blog" 192.0.2.7/32`) || !strings.Contains(inc, `"shop"`) {
		t.Fatalf("both databases:\n%s", inc)
	}
	// Fewer addresses for shop: the stale rule goes.
	if _, err := m.SetRemote(ctx, d.ID, []string{"2001:db8:5::/48"}); err != nil {
		t.Fatal(err)
	}
	if got := fw.list("PostgreSQL shop"); !slices.Equal(got, []string{"5432", "2001:db8:5::/48"}) {
		t.Fatalf("firewall after removing one: %v", got)
	}
	// Deleting blog removes its lines and rules; then shop is turned off.
	if err := m.Delete(ctx, other.ID, "blog"); err != nil {
		t.Fatal(err)
	}
	if inc := readFile(t, filepath.Join(dir, hbaIncludeName)); strings.Contains(inc, "blog") {
		t.Fatalf("blog's lines stayed:\n%s", inc)
	}
	if got := fw.list("PostgreSQL blog"); len(got) != 0 {
		t.Fatalf("blog's rules stayed: %v", got)
	}
	if v, _ := st.Setting(ctx, "pg_remote_hosts."+fmt.Sprint(other.ID)); v != "" {
		t.Fatalf("setting kept: %q", v)
	}
	restart, err = m.SetRemote(ctx, d.ID, nil)
	if err != nil || !restart {
		t.Fatalf("off: %v %v", restart, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "conf.d", remoteConfName)); !os.IsNotExist(err) {
		t.Fatal("drop-in kept")
	}
	if _, err := os.Stat(filepath.Join(dir, hbaIncludeName)); !os.IsNotExist(err) {
		t.Fatal("include file kept")
	}
	if got := readFile(t, filepath.Join(dir, "pg_hba.conf")); got != hbaConf {
		t.Fatalf("pg_hba.conf not as before:\n%s", got)
	}
	if len(fw.rules) != 0 {
		t.Fatalf("rules left: %v", fw.rules)
	}
	if err := m.Restart(ctx, testLog{t}); err != nil || pg.listen != "localhost" {
		t.Fatalf("listen after the restart: %s %v", pg.listen, err)
	}
	// MariaDB's remote access never sees PostgreSQL's.
	if all, _ := st.AllDBRemoteHosts(ctx); len(all) != 0 {
		t.Fatalf("MariaDB's list: %v", all)
	}
}

// PostgreSQL 15: a marked block in pg_hba.conf; a cluster without TLS gets
// the panel's own certificate in the drop-in.
func TestRemoteAccess15(t *testing.T) {
	ctx := context.Background()
	m, _, st, pg, _ := newRemoteManager(t, "15")
	pg.ssl = "off"
	d := addPGDatabase(t, st, "shop", "Shop-Password-1")
	dir := m.mainDir("15")
	if _, err := m.SetRemote(ctx, d.ID, []string{"203.0.113.5"}); err != nil {
		t.Fatal(err)
	}
	conf := readFile(t, filepath.Join(dir, "conf.d", remoteConfName))
	for _, want := range []string{"ssl = on\n", "ssl_cert_file = '" + filepath.Join(dir, panelCertName) + "'\n", "ssl_key_file = '" + filepath.Join(dir, panelKeyName) + "'\n"} {
		if !strings.Contains(conf, want) {
			t.Errorf("drop-in lacks %q:\n%s", want, conf)
		}
	}
	if fi, err := os.Stat(filepath.Join(dir, panelKeyName)); err != nil || (runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600) {
		t.Fatalf("key: %v %v", fi, err)
	}
	want := hbaConf + "\n" + hbaBegin + "\n" + `hostssl "shop" "shop" 203.0.113.5/32 scram-sha-256` + "\n" + hbaEnd + "\n"
	if got := readFile(t, filepath.Join(dir, "pg_hba.conf")); got != want {
		t.Fatalf("pg_hba.conf:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(dir, hbaIncludeName)); !os.IsNotExist(err) {
		t.Fatal("an include file for PostgreSQL 15")
	}
	if ra, _ := m.Remote(ctx, d.ID); !ra.Encrypted || ra.Certificate != "panel" || ra.HBAInclude {
		t.Fatalf("state: %+v", ra)
	}
	// Saved again: the drop-in keeps the certificate it turned on.
	if _, err := m.SetRemote(ctx, d.ID, []string{"203.0.113.5", "198.51.100.9"}); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dir, "conf.d", remoteConfName)); got != conf {
		t.Fatalf("drop-in changed:\n%s", got)
	}

	// Without TLS at all, the lines say host, and the state says so.
	pg.ssl, pg.noSSL = "off", true
	os.Remove(filepath.Join(dir, "conf.d", remoteConfName))
	if _, err := m.SetRemote(ctx, d.ID, []string{"203.0.113.5"}); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dir, "pg_hba.conf")); !strings.Contains(got, "\nhost    \"shop\" \"shop\" 203.0.113.5/32 scram-sha-256\n") {
		t.Fatalf("pg_hba.conf:\n%s", got)
	}
	if ra, _ := m.Remote(ctx, d.ID); ra.Encrypted || ra.SSL {
		t.Fatalf("state: %+v", ra)
	}
}

// A pg_hba.conf PostgreSQL refuses is put back; the saved list is too.
func TestRemoteAccessHBAError(t *testing.T) {
	ctx := context.Background()
	m, _, st, pg, fw := newRemoteManager(t, "16")
	d := addPGDatabase(t, st, "shop", "Shop-Password-1")
	pg.hbaErr = "100: invalid connection type\n"
	if _, err := m.SetRemote(ctx, d.ID, []string{"203.0.113.5"}); err == nil || !strings.Contains(err.Error(), "put back") {
		t.Fatalf("%v", err)
	}
	if got := readFile(t, filepath.Join(m.mainDir("16"), "pg_hba.conf")); got != hbaConf {
		t.Fatalf("not put back:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(m.mainDir("16"), hbaIncludeName)); !os.IsNotExist(err) {
		t.Fatal("include file kept")
	}
	if h, _ := st.PGRemoteHosts(ctx, d.ID); len(h) != 0 {
		t.Fatalf("saved anyway: %v", h)
	}
	if fileThere(m.remoteConfPath(cluster{version: "16"})) {
		t.Fatal("the listen drop-in stayed")
	}
	if len(fw.rules) != 0 {
		t.Fatalf("port opened: %v", fw.rules)
	}
}

// While ufw is off, its rules are kept by what they say; a cluster on
// another port gets rules for that port.
func TestRemoteFirewallOffAndPort(t *testing.T) {
	ctx := context.Background()
	m, _, st, _, fw := newRemoteManager(t, "16")
	os.WriteFile(filepath.Join(m.mainDir("16"), "postgresql.conf"), []byte("port = 5433\n"), 0o644)
	fw.active = false
	d := addPGDatabase(t, st, "shop", "Shop-Password-1")
	if _, err := m.SetRemote(ctx, d.ID, []string{"203.0.113.5", "192.0.2.0/24"}); err != nil {
		t.Fatal(err)
	}
	if got := fw.list("PostgreSQL shop"); !slices.Equal(got, []string{"5433", "203.0.113.5", "5433", "192.0.2.0/24"}) {
		t.Fatalf("rules while off: %v", got)
	}
	if _, err := m.SetRemote(ctx, d.ID, []string{"192.0.2.0/24"}); err != nil {
		t.Fatal(err)
	}
	if got := fw.list("PostgreSQL shop"); !slices.Equal(got, []string{"5433", "192.0.2.0/24"}) {
		t.Fatalf("after removing one: %v", got)
	}
	// A rule left for the old port goes too.
	fw.added = append(fw.added, "allow from 192.0.2.0/24 to any port 5432 proto tcp comment 'PostgreSQL shop'")
	if _, err := m.SetRemote(ctx, d.ID, []string{"192.0.2.0/24"}); err != nil {
		t.Fatal(err)
	}
	if got := fw.list("PostgreSQL shop"); !slices.Equal(got, []string{"5433", "192.0.2.0/24"}) {
		t.Fatalf("old port kept: %v", got)
	}
	if ra, _ := m.Remote(ctx, d.ID); ra.Port != 5433 || ra.FirewallActive || !ra.FirewallPresent {
		t.Fatalf("state: %+v", ra)
	}
	// No firewall at all: refused.
	fw.installed = false
	if _, err := m.SetRemote(ctx, d.ID, []string{"203.0.113.5"}); userCode(err) != "firewall_not_installed" {
		t.Fatalf("%v", err)
	}
}

// A restart that fails with the drop-in removes it and starts again.
func TestRestartFallsBack(t *testing.T) {
	ctx := context.Background()
	m, f, st, pg, _ := newRemoteManager(t, "16")
	d := addPGDatabase(t, st, "shop", "Shop-Password-1")
	if _, err := m.SetRemote(ctx, d.ID, []string{"203.0.113.5"}); err != nil {
		t.Fatal(err)
	}
	prev := f.output
	tries := 0
	f.output = func(name string, args []string) (string, error) {
		if name == "systemctl" && args[0] == "restart" {
			tries++
			if tries == 1 {
				return "Job failed", errors.New("exit status 1")
			}
		}
		return prev(name, args)
	}
	err := m.Restart(ctx, testLog{t})
	if err == nil || !strings.Contains(err.Error(), "without it") || tries != 2 {
		t.Fatalf("%v, %d tries", err, tries)
	}
	if _, err := os.Stat(m.remoteConfPath(cluster{version: "16"})); !os.IsNotExist(err) {
		t.Fatal("drop-in kept")
	}
	if pg.listen != "localhost" {
		t.Fatalf("listens on %s", pg.listen)
	}
}
