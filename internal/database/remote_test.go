package database

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/security"
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
	if r.Action != "allow" || r.Port != "3306" || r.Proto != "tcp" {
		return fmt.Errorf("unexpected rule %+v", r)
	}
	if !f.active {
		f.added = append(f.added, fmt.Sprintf("allow from %s to any port 3306 proto tcp comment '%s'", r.From, r.Comment))
		return nil
	}
	f.rules = append(f.rules, security.Rule{To: r.Port + "/" + r.Proto, Action: "ALLOW", From: r.From, Comment: r.Comment})
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

func (f *fakeFirewall) froms(comment string) []string {
	var out []string
	for _, r := range f.rules {
		if r.Comment == comment {
			out = append(out, r.From)
		}
	}
	return out
}

func TestParseRemoteHosts(t *testing.T) {
	got, err := ParseRemoteHosts([]string{" 203.0.113.5 ", "", "10.1.2.3/16", "2001:DB8::1", "::ffff:203.0.113.9",
		"203.0.113.5/32", "203.0.113.5", "192.168.7.200/24", "2001:db8::2/128"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"203.0.113.5", "10.1.0.0/16", "2001:db8::1", "203.0.113.9", "192.168.7.0/24", "2001:db8::2"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}

	cases := map[string]string{
		"0.0.0.0/0":         "remote_host_too_wide",
		"::/0":              "remote_host_too_wide",
		"10.0.0.0/8":        "remote_host_too_wide",
		"172.16.0.0/15":     "remote_host_too_wide",
		"2001:db8::/64":     "remote_ipv6_range",
		"2001:db8::/16":     "remote_ipv6_range",
		"127.0.0.1":         "invalid_remote_host",
		"127.0.0.0/16":      "invalid_remote_host",
		"0.0.0.0":           "invalid_remote_host",
		"::":                "invalid_remote_host",
		"::1":               "invalid_remote_host",
		"224.0.0.1":         "invalid_remote_host",
		"fe80::1%eth0":      "invalid_remote_host",
		"localhost":         "invalid_remote_host",
		"%":                 "invalid_remote_host",
		"10.0.%":            "invalid_remote_host",
		"1.2.3.4'; DROP --": "invalid_remote_host",
		"1.2.3.4/33":        "invalid_remote_host",
		"1.2.3.4 5.6.7.8":   "invalid_remote_host",
	}
	for in, code := range cases {
		if _, err := ParseRemoteHosts([]string{in}); !isCode(err, code) {
			t.Errorf("%q: err = %v, want %s", in, err, code)
		}
	}

	var many []string
	for i := range MaxRemoteHosts + 1 {
		many = append(many, fmt.Sprintf("203.0.113.%d", i+1))
	}
	if _, err := ParseRemoteHosts(many); !isCode(err, "too_many_remote_hosts") {
		t.Errorf("11 hosts: err = %v", err)
	}
	if got, err := ParseRemoteHosts(many[:MaxRemoteHosts]); err != nil || len(got) != MaxRemoteHosts {
		t.Errorf("10 hosts: %v %v", got, err)
	}
}

func TestMariaHost(t *testing.T) {
	cases := map[string]string{
		"203.0.113.5":    "203.0.113.5",
		"2001:db8::1":    "2001:db8::1",
		"10.1.0.0/16":    "10.1.0.0/255.255.0.0",
		"192.168.7.0/24": "192.168.7.0/255.255.255.0",
		"203.0.113.0/27": "203.0.113.0/255.255.255.224",
		"10.128.0.0/17":  "10.128.0.0/255.255.128.0",
		"10.1.2.4/31":    "10.1.2.4/255.255.255.254",
	}
	for in, want := range cases {
		mh := mariaHost(in)
		if mh != want {
			t.Errorf("mariaHost(%q) = %q, want %q", in, mh, want)
		}
		if back, ok := fromMariaHost(mh); !ok || back != in {
			t.Errorf("fromMariaHost(%q) = %q, %v", mh, back, ok)
		}
	}
	for _, s := range []string{"%", "localhost", "", "10.1.0.0/255.0.255.0", "10.0.0.0/255.0.0.0",
		"10.1.0.5/255.255.0.0", "10.1.0.0/16", "10.1.%", "::1", "a.example.com", "10.1.0.0/255.255.0.0/1"} {
		if h, ok := fromMariaHost(s); ok {
			t.Errorf("fromMariaHost(%q) accepted as %q", s, h)
		}
	}
}

func TestRemoteUserSQL(t *testing.T) {
	q, err := remoteUserSQL("shop", "shop", "Secret-password1", []string{"203.0.113.5", "10.1.0.0/255.255.0.0"}, []string{"2001:db8::1"})
	if err != nil {
		t.Fatal(err)
	}
	want := "DROP USER IF EXISTS 'shop'@'2001:db8::1';\n" +
		"CREATE OR REPLACE USER 'shop'@'203.0.113.5' IDENTIFIED BY 'Secret-password1';\n" +
		"GRANT ALL PRIVILEGES ON `shop`.* TO 'shop'@'203.0.113.5';\n" +
		"CREATE OR REPLACE USER 'shop'@'10.1.0.0/255.255.0.0' IDENTIFIED BY 'Secret-password1';\n" +
		"GRANT ALL PRIVILEGES ON `shop`.* TO 'shop'@'10.1.0.0/255.255.0.0';\n"
	if q != want {
		t.Fatalf("got:\n%s\nwant:\n%s", q, want)
	}
	for _, bad := range []struct{ name, pw, host string }{
		{"shop", "Secret-password1", "1.2.3.4'--"},
		{"shop", "Secret-password1", "%"},
		{"sh`op", "Secret-password1", "1.2.3.4"},
		{"shop", "short", "1.2.3.4"},
		{"shop", "it's-a-password", "1.2.3.4"},
	} {
		if _, err := remoteUserSQL(bad.name, bad.name, bad.pw, []string{bad.host}, nil); err == nil {
			t.Errorf("accepted %+v", bad)
		}
	}
	if _, err := remoteUserSQL("shop", "shop", "", nil, []string{"1.2.3.4' OR 1=1"}); err == nil {
		t.Error("accepted a bad host to drop")
	}
}

func newRemoteTest(t *testing.T) (*Manager, *fakeRunner, *fakeFirewall) {
	t.Helper()
	m, f := newTest(t)
	fw := &fakeFirewall{installed: true, active: true,
		rules: []security.Rule{{To: "22/tcp", Action: "ALLOW", From: "Anywhere"}}}
	m.SetFirewall(fw)
	f.answer["@@global.bind_address"] = "127.0.0.1\n"
	return m, f, fw
}

// lastSQL returns the last batch that changed something, skipping queries.
func lastSQL(f *fakeRunner) string {
	for i := len(f.sql) - 1; i >= 0; i-- {
		if !strings.HasPrefix(f.sql[i], "SELECT ") {
			return f.sql[i]
		}
	}
	return ""
}

func TestRemoteAccessLifecycle(t *testing.T) {
	m, f, fw := newRemoteTest(t)
	ctx := context.Background()
	d, err := m.Create(ctx, "shop", "", 0)
	if err != nil {
		t.Fatal(err)
	}

	restart, err := m.SetRemote(ctx, d.ID, []string{"198.51.100.7", " 10.1.2.3/16 ", "198.51.100.7"})
	if err != nil {
		t.Fatal(err)
	}
	if !restart {
		t.Error("no restart although MariaDB listens on 127.0.0.1")
	}
	q := lastSQL(f)
	for _, want := range []string{
		"CREATE OR REPLACE USER 'shop'@'198.51.100.7' IDENTIFIED BY '" + d.Password + "';",
		"GRANT ALL PRIVILEGES ON `shop`.* TO 'shop'@'198.51.100.7';",
		"CREATE OR REPLACE USER 'shop'@'10.1.0.0/255.255.0.0' IDENTIFIED BY '" + d.Password + "';",
		"GRANT ALL PRIVILEGES ON `shop`.* TO 'shop'@'10.1.0.0/255.255.0.0';",
	} {
		if !strings.Contains(q, want) {
			t.Errorf("missing %q in:\n%s", want, q)
		}
	}
	if got := fw.froms("MariaDB shop"); !slices.Equal(got, []string{"198.51.100.7", "10.1.0.0/16"}) {
		t.Errorf("firewall: %q", got)
	}
	if b, err := os.ReadFile(m.remoteConf); err != nil || !strings.Contains(string(b), "[mysqld]\nbind-address = 0.0.0.0\n") {
		t.Errorf("listen file: %q %v", b, err)
	}
	if list, _ := m.List(ctx); len(list) != 1 || len(list[0].RemoteHosts) != 2 {
		t.Errorf("list: %+v", list)
	}

	// MariaDB restarted and now listens everywhere; one address goes away.
	if err := m.Restart(ctx, discard{}); err != nil {
		t.Fatal(err)
	}
	f.answer["@@global.bind_address"] = "0.0.0.0\n"
	f.answer["SELECT Host FROM mysql.user"] = "198.51.100.7\n10.1.0.0/255.255.0.0\n%\n"
	restart, err = m.SetRemote(ctx, d.ID, []string{"198.51.100.7"})
	if err != nil {
		t.Fatal(err)
	}
	if restart {
		t.Error("restart although the listen setting did not change")
	}
	q = lastSQL(f)
	if !strings.Contains(q, "DROP USER IF EXISTS 'shop'@'10.1.0.0/255.255.0.0';") || strings.Contains(q, "'%'") {
		t.Errorf("drop SQL:\n%s", q)
	}
	if got := fw.froms("MariaDB shop"); !slices.Equal(got, []string{"198.51.100.7"}) {
		t.Errorf("firewall: %q", got)
	}
	ra, err := m.Remote(ctx, d.ID)
	if err != nil || !slices.Equal(ra.Hosts, []string{"198.51.100.7"}) || !ra.Listening || ra.RestartPending || !ra.FirewallActive {
		t.Errorf("remote state: %+v %v", ra, err)
	}

	// A new password reaches the remote twin too.
	d2, err := m.SetPassword(ctx, d.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	q = lastSQL(f)
	for _, want := range []string{
		"ALTER USER 'shop'@'localhost' IDENTIFIED BY '" + d2.Password + "';",
		"CREATE OR REPLACE USER 'shop'@'198.51.100.7' IDENTIFIED BY '" + d2.Password + "';",
	} {
		if !strings.Contains(q, want) {
			t.Errorf("missing %q in:\n%s", want, q)
		}
	}

	// Off: twins, rules and the listen file go.
	f.answer["SELECT Host FROM mysql.user"] = "198.51.100.7\n"
	restart, err = m.SetRemote(ctx, d.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !restart {
		t.Error("no restart after the last remote access was turned off")
	}
	if q := lastSQL(f); !strings.Contains(q, "DROP USER IF EXISTS 'shop'@'198.51.100.7';") {
		t.Errorf("drop SQL:\n%s", q)
	}
	if got := fw.froms("MariaDB shop"); len(got) != 0 {
		t.Errorf("rules left: %q", got)
	}
	if len(fw.rules) != 1 || fw.rules[0].To != "22/tcp" {
		t.Errorf("other rules touched: %+v", fw.rules)
	}
	if _, err := os.Stat(m.remoteConf); !os.IsNotExist(err) {
		t.Errorf("listen file still there: %v", err)
	}
	if err := m.Restart(ctx, discard{}); err != nil {
		t.Fatal(err)
	}
	if m.RestartPending(ctx) {
		t.Error("restart still pending after the restart")
	}
}

func TestDeleteRemovesRemoteAccess(t *testing.T) {
	m, f, fw := newRemoteTest(t)
	ctx := context.Background()
	shop, _ := m.Create(ctx, "shop", "", 0)
	blog, _ := m.Create(ctx, "blog", "", 0)
	if _, err := m.SetRemote(ctx, shop.ID, []string{"203.0.113.5"}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.SetRemote(ctx, blog.ID, []string{"2001:db8::1"}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(m.remoteConf); !strings.Contains(string(b), "bind-address = ::\n") {
		t.Errorf("IPv6 client but listen file %q", b)
	}
	// ufw shows IPv6 rules with (v6).
	for i := range fw.rules {
		if fw.rules[i].From == "2001:db8::1" {
			fw.rules[i].To, fw.rules[i].From = "3306/tcp (v6)", "2001:db8::1 (v6)"
		}
	}
	if _, err := m.SetRemote(ctx, blog.ID, []string{"2001:db8::1"}); err != nil {
		t.Fatal(err)
	}
	if got := fw.froms("MariaDB blog"); len(got) != 1 {
		t.Errorf("IPv6 rule not recognised: %q", got)
	}

	f.answer["SELECT Host FROM mysql.user"] = "203.0.113.5\n"
	if err := m.Delete(ctx, shop.ID, "shop"); err != nil {
		t.Fatal(err)
	}
	q := lastSQL(f)
	for _, want := range []string{"DROP USER IF EXISTS 'shop'@'203.0.113.5';", "DROP DATABASE IF EXISTS `shop`;", "DROP USER IF EXISTS 'shop'@'localhost';"} {
		if !strings.Contains(q, want) {
			t.Errorf("missing %q in:\n%s", want, q)
		}
	}
	if got := fw.froms("MariaDB shop"); len(got) != 0 {
		t.Errorf("rules left: %q", got)
	}
	if b, _ := os.ReadFile(m.remoteConf); !strings.Contains(string(b), "bind-address") {
		t.Error("listen file removed although blog still allows remote access")
	}
	if hosts, _ := m.store.DBRemoteHosts(ctx, shop.ID); len(hosts) != 0 {
		t.Errorf("stored hosts left: %q", hosts)
	}

	f.answer["SELECT Host FROM mysql.user"] = "2001:db8::1\n"
	f.answer["@@global.bind_address"] = "::\n"
	if err := m.Delete(ctx, blog.ID, "blog"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(m.remoteConf); !os.IsNotExist(err) {
		t.Error("listen file kept after the last database was deleted")
	}
	if !m.RestartPending(ctx) {
		t.Error("no restart pending after the listen file was removed")
	}
	if len(fw.rules) != 1 {
		t.Errorf("rules left: %+v", fw.rules)
	}
}

func TestRemoteRefusals(t *testing.T) {
	m, f, fw := newRemoteTest(t)
	ctx := context.Background()
	d, _ := m.Create(ctx, "shop", "", 0)
	n := len(f.sql)
	if _, err := m.SetRemote(ctx, d.ID, []string{"0.0.0.0/0"}); !isCode(err, "remote_host_too_wide") {
		t.Errorf("0.0.0.0/0: %v", err)
	}
	fw.installed = false
	if _, err := m.SetRemote(ctx, d.ID, []string{"203.0.113.5"}); !isCode(err, "firewall_not_installed") {
		t.Errorf("no ufw: %v", err)
	}
	if len(f.sql) != n {
		t.Errorf("SQL ran for a refused change: %q", f.sql[n:])
	}
	if hosts, _ := m.store.DBRemoteHosts(ctx, d.ID); len(hosts) != 0 {
		t.Errorf("refused hosts stored: %q", hosts)
	}
	// Turning access off works without ufw.
	if _, err := m.SetRemote(ctx, d.ID, nil); err != nil {
		t.Errorf("off without ufw: %v", err)
	}
	m.SetFirewall(nil)
	if _, err := m.SetRemote(ctx, d.ID, []string{"203.0.113.5"}); err == nil {
		t.Error("remote access without a firewall manager")
	}
}

type discard struct{}

func (discard) Write(p []byte) (int, error)     { return len(p), nil }
func (discard) Logf(format string, args ...any) {}

// While ufw is off its rules have no numbers: a removed host's rule is
// deleted by what it says, so turning the firewall on can't make it live.
func TestRemoteFirewallWhileOff(t *testing.T) {
	m := &Manager{}
	fw := &fakeFirewall{installed: true}
	m.SetFirewall(fw)
	ctx := context.Background()
	if err := m.syncRemoteFirewall(ctx, "shop", []string{"203.0.113.5", "198.51.100.0/24"}); err != nil {
		t.Fatal(err)
	}
	if len(fw.added) != 2 {
		t.Fatalf("added: %q", fw.added)
	}
	// Again: nothing is added twice.
	if err := m.syncRemoteFirewall(ctx, "shop", []string{"203.0.113.5", "198.51.100.0/24"}); err != nil || len(fw.added) != 2 {
		t.Fatalf("second sync: %v, %q", err, fw.added)
	}
	fw.added = append(fw.added, "allow 22/tcp comment 'SSH'", "allow from 192.0.2.9 to any port 3306 proto tcp comment 'MariaDB other'")
	if err := m.syncRemoteFirewall(ctx, "shop", []string{"198.51.100.0/24"}); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"allow from 198.51.100.0/24 to any port 3306 proto tcp comment 'MariaDB shop'",
		"allow 22/tcp comment 'SSH'",
		"allow from 192.0.2.9 to any port 3306 proto tcp comment 'MariaDB other'",
	}
	if !slices.Equal(fw.added, want) {
		t.Fatalf("added: %q", fw.added)
	}
}
