package security

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/usererr"
)

const ufwOut = `Status: active

     To                         Action      From
     --                         ------      ----
[ 1] 22/tcp                     ALLOW IN    Anywhere                   # SSH
[ 2] 46949/tcp                  ALLOW IN    203.0.113.35              # Panel
[ 3] 80/tcp                     ALLOW IN    Anywhere                   # Websites
[ 4] 3306                       DENY IN     10.0.0.0/8
[ 5] 22/tcp (v6)                ALLOW IN    Anywhere (v6)              # SSH
`

type fake struct {
	calls []string
	files map[string]string
	sshdT string // what sshd -T prints; empty for the default below
	ufw   string // what ufw status prints; empty for ufwOut
	added string // what ufw show added prints
}

func (f *fake) Output(ctx context.Context, name string, args ...string) (string, error) {
	call := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, call)
	switch {
	case (call == "ufw status numbered" || call == "ufw status") && f.ufw != "":
		return f.ufw, nil
	case call == "ufw status numbered" || call == "ufw status":
		return ufwOut, nil
	case call == "ufw show added":
		return f.added, nil
	case call == "sshd -T" && f.sshdT != "":
		return f.sshdT, nil
	case call == "sshd -T":
		return "port 22\npasswordauthentication yes\npermitrootlogin yes\n", nil
	case call == "sshd -t":
		return "", nil
	}
	return "", nil
}
func (f *fake) Stream(context.Context, io.Writer, string, ...string) error { return nil }
func (f *fake) Input(context.Context, []byte, string, ...string) (string, error) {
	return "", nil
}
func (f *fake) Pipe(context.Context, io.Reader, io.Writer, string, ...string) error { return nil }

func newTest(t *testing.T) (*Manager, *fake) {
	f := &fake{}
	m := New(f, 46949)
	dir := t.TempDir()
	m.sshDropIn = filepath.Join(dir, "00-panel.conf")
	m.rootKeys = filepath.Join(dir, "authorized_keys")
	m.f2bJail = filepath.Join(dir, "jail.d", "panel.conf")
	m.f2bFilter = filepath.Join(dir, "filter.d", "panel.conf")
	// root has a password and no key.
	m.passwd = filepath.Join(dir, "passwd")
	m.shadow = filepath.Join(dir, "shadow")
	m.group = filepath.Join(dir, "group")
	os.WriteFile(m.passwd, []byte("root:x:0:0:root:"+filepath.Join(dir, "root")+":/bin/bash"+nl+
		"daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin"+nl), 0o644)
	os.WriteFile(m.shadow, []byte("root:$6$salt$hash:19000:0:99999:7:::"+nl+"daemon:*:19000:0:99999:7:::"+nl), 0o600)
	os.WriteFile(m.group, []byte("root:x:0:"+nl+"daemon:x:1:"+nl), 0o644)
	return m, f
}

// addUser adds an account to the test's passwd, shadow and group files,
// with a key in its home folder if key is true.
func addUser(t *testing.T, m *Manager, name string, uid int, shell, hash, group string, key bool) {
	t.Helper()
	home := filepath.Join(filepath.Dir(m.passwd), "home", name)
	appendTo := func(path, line string) {
		b, _ := os.ReadFile(path)
		os.WriteFile(path, append(b, []byte(line+nl)...), 0o644)
	}
	appendTo(m.passwd, fmt.Sprintf("%s:x:%d:%d::%s:%s", name, uid, uid, home, shell))
	appendTo(m.shadow, name+":"+hash+":19000:0:99999:7:::")
	appendTo(m.group, fmt.Sprintf("%s:x:%d:", name, uid))
	if group != "" {
		appendTo(m.group, group+":x:27:"+name)
	}
	if key {
		os.MkdirAll(filepath.Join(home, ".ssh"), 0o700)
		os.WriteFile(filepath.Join(home, ".ssh", "authorized_keys"), []byte("ssh-ed25519 AAAA "+name+nl), 0o600)
	}
}

func code(err error) string {
	var ue *usererr.Error
	if errors.As(err, &ue) {
		return ue.Code
	}
	return ""
}

func TestFirewallParsing(t *testing.T) {
	m, _ := newTest(t)
	fw := m.Firewall(context.Background())
	if !fw.Active || len(fw.Rules) != 5 || fw.SSHPort != 22 {
		t.Fatalf("firewall = %+v", fw)
	}
	r := fw.Rules[1]
	if r.To != "46949/tcp" || r.From != "203.0.113.35" || r.Comment != "Panel" || !r.Protected {
		t.Fatalf("rule 2 = %+v", r)
	}
	if fw.Rules[2].Protected || !fw.Rules[0].Protected || !fw.Rules[4].V6 {
		t.Fatalf("protection flags wrong: %+v", fw.Rules)
	}
	if fw.Rules[3].Action != "DENY" || fw.Rules[3].From != "10.0.0.0/8" {
		t.Fatalf("rule 4 = %+v", fw.Rules[3])
	}
}

func TestFirewallInactive(t *testing.T) {
	m, f := newTest(t)
	ctx := context.Background()
	f.ufw = "Status: inactive\n"
	f.added = `Added user rules (see 'ufw status' for running firewall):
ufw allow 22/tcp comment 'SSH'
ufw deny from 198.51.100.7 to any port 80 proto tcp
ufw allow from 203.0.113.5 to any port 46949 proto tcp comment 'my office'
`
	fw := m.Firewall(ctx)
	want := []string{"allow 22/tcp comment 'SSH'", "deny from 198.51.100.7 to any port 80 proto tcp",
		"allow from 203.0.113.5 to any port 46949 proto tcp comment 'my office'"}
	if !fw.Installed || fw.Active || len(fw.Rules) != 0 || strings.Join(fw.Added, nl) != strings.Join(want, nl) {
		t.Fatalf("firewall = %+v", fw)
	}
	f.added = "Added user rules (see 'ufw status' for running firewall):\n(None)\n"
	if fw := m.Firewall(ctx); fw.Added == nil || len(fw.Added) != 0 {
		t.Fatalf("no rules: %q", fw.Added)
	}
	f.ufw = ufwOut
	if fw := m.Firewall(ctx); len(fw.Added) != 0 {
		t.Fatalf("active firewall: added = %q", fw.Added)
	}
}

func TestEnableFirewallFromOffOpensThePanel(t *testing.T) {
	m, f := newTest(t)
	f.ufw = "Status: inactive\n"
	// Limited to an address that may not be the administrator's: turning
	// the firewall on still opens the panel's port to everyone.
	f.added = "Added user rules (see 'ufw status' for running firewall):\nufw allow from 203.0.113.5 to any port 46949 proto tcp\n"
	if err := m.EnableFirewall(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(f.calls, "ufw allow 46949/tcp comment Nefo") {
		t.Fatalf("ran: %q", f.calls)
	}
}

func TestAddRule(t *testing.T) {
	m, f := newTest(t)
	ctx := context.Background()
	if err := m.AddRule(ctx, NewRule{Action: "allow", Port: "8080", Proto: "tcp", From: "203.0.113.0/24", Comment: "API"}); err != nil {
		t.Fatal(err)
	}
	if last := f.calls[len(f.calls)-1]; last != "ufw allow from 203.0.113.0/24 to any port 8080 proto tcp comment API" {
		t.Fatalf("ran %q", last)
	}
	bad := map[string]NewRule{
		"invalid_port":    {Action: "allow", Port: "80;reboot"},
		"invalid_ip":      {Action: "allow", Port: "80", From: "evil host"},
		"invalid_rule":    {Action: "delete", Port: "80"},
		"rule_locks_out":  {Action: "deny", Port: "22"},
		"invalid_comment": {Action: "allow", Port: "80", Comment: "a'b"},
	}
	for want, r := range bad {
		if got := code(m.AddRule(ctx, r)); got != want {
			t.Errorf("%+v: %q, want %q", r, got, want)
		}
	}
}

func TestDeleteRuleGuards(t *testing.T) {
	m, f := newTest(t)
	ctx := context.Background()
	if got := code(m.DeleteRule(ctx, 1, "22/tcp", "Anywhere")); got != "rule_protected" {
		t.Fatalf("SSH rule: %q", got)
	}
	if got := code(m.DeleteRule(ctx, 3, "443/tcp", "Anywhere")); got != "rule_changed" {
		t.Fatalf("stale rule: %q", got)
	}
	if err := m.DeleteRule(ctx, 3, "80/tcp", "Anywhere"); err != nil {
		t.Fatal(err)
	}
	if last := f.calls[len(f.calls)-1]; last != "ufw --force delete 3" {
		t.Fatalf("ran %q", last)
	}
}

func TestSetSSH(t *testing.T) {
	m, _ := newTest(t)
	ctx := context.Background()
	if got := code(m.SetSSH(ctx, SSH{Port: 22, PasswordAuth: false, PermitRootLogin: "yes"})); got != "ssh_no_keys" {
		t.Fatalf("no keys: %q", got)
	}
	os.WriteFile(m.rootKeys, []byte("# comment\nssh-ed25519 AAAA admin\n"), 0o600)
	if err := m.SetSSH(ctx, SSH{Port: 2222, PasswordAuth: false, PermitRootLogin: "prohibit-password"}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(m.sshDropIn)
	for _, want := range []string{"Port 2222", "PasswordAuthentication no", "PermitRootLogin prohibit-password"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("missing %q in:\n%s", want, b)
		}
	}
	if got := code(m.SetSSH(ctx, SSH{Port: 46949, PasswordAuth: true, PermitRootLogin: "yes"})); got != "invalid_port" {
		t.Fatalf("panel port: %q", got)
	}
}

func TestSSHRefusesBusyPort(t *testing.T) {
	m, _ := newTest(t)
	tcp := filepath.Join(t.TempDir(), "tcp")
	// 0.0.0.0:3306 and 127.0.0.1:8080 listening (0A), :443 established (01).
	os.WriteFile(tcp, []byte(`  sl  local_address rem_address   st
   0: 00000000:0CEA 00000000:0000 0A
   1: 0100007F:1F90 00000000:0000 0A
   2: 0A000001:01BB 0A000002:C350 01
`), 0o644)
	m.procNet = []string{tcp}
	os.WriteFile(m.rootKeys, []byte("ssh-ed25519 AAAA admin\n"), 0o600)
	for _, p := range []int{3306, 8080} {
		if got := code(m.SetSSH(context.Background(), SSH{Port: p, PasswordAuth: true, PermitRootLogin: "yes"})); got != "port_in_use" {
			t.Errorf("port %d: %q", p, got)
		}
	}
	if !m.listening(3306) || m.listening(443) {
		t.Fatal("listening() wrong")
	}
}

// sshd -T prints the older name "without-password" on some systems: the
// panel shows and writes it as "prohibit-password".
func TestSSHWithoutPassword(t *testing.T) {
	m, f := newTest(t)
	f.sshdT = "port 22" + nl + "passwordauthentication yes" + nl + "permitrootlogin without-password" + nl
	ctx := context.Background()
	if got := m.SSHSettings(ctx).PermitRootLogin; got != "prohibit-password" {
		t.Fatalf("PermitRootLogin = %q, want prohibit-password", got)
	}
	if err := m.SetSSH(ctx, SSH{Port: 22, PasswordAuth: true, PermitRootLogin: "without-password"}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(m.sshDropIn)
	if !strings.Contains(string(b), "PermitRootLogin prohibit-password") {
		t.Fatalf("drop-in:%s%s", nl, b)
	}
	if got := code(m.SetSSH(ctx, SSH{Port: 22, PasswordAuth: true, PermitRootLogin: "forced-commands-only"})); got != "invalid_ssh_settings" {
		t.Fatalf("other values: %q, want invalid_ssh_settings", got)
	}
}

const nl = "\n"

// Nobody may be left without a way to sign in over SSH.
func TestSSHLockOutGuard(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name  string
		setup func(t *testing.T, m *Manager, f *fake)
		set   SSH
		want  string // error code, "" for accepted
	}{
		{"passwords off, no key anywhere", nil, SSH{Port: 22, PermitRootLogin: "yes"}, "ssh_no_keys"},
		{"root not allowed, no other account", nil, SSH{Port: 22, PasswordAuth: true, PermitRootLogin: "no"}, "ssh_no_login"},
		{"root key only, root has no key", nil, SSH{Port: 22, PasswordAuth: true, PermitRootLogin: "prohibit-password"}, "ssh_root_needs_key"},
		{"root not allowed, a user with a key", func(t *testing.T, m *Manager, f *fake) {
			addUser(t, m, "deploy", 1000, "/bin/bash", "!", "", true)
		}, SSH{Port: 22, PermitRootLogin: "no"}, ""},
		{"root not allowed, a user with a password", func(t *testing.T, m *Manager, f *fake) {
			addUser(t, m, "deploy", 1000, "/bin/bash", "$6$x$y", "", false)
		}, SSH{Port: 22, PasswordAuth: true, PermitRootLogin: "no"}, ""},
		{"passwords off, the user only has a password", func(t *testing.T, m *Manager, f *fake) {
			addUser(t, m, "deploy", 1000, "/bin/bash", "$6$x$y", "", false)
		}, SSH{Port: 22, PermitRootLogin: "no"}, "ssh_no_login"},
		{"an SFTP user with nologin does not count", func(t *testing.T, m *Manager, f *fake) {
			addUser(t, m, "site1", 1001, "/usr/sbin/nologin", "$6$x$y", "", true)
		}, SSH{Port: 22, PasswordAuth: true, PermitRootLogin: "no"}, "ssh_no_login"},
		{"a service account below UID 1000 does not count", func(t *testing.T, m *Manager, f *fake) {
			addUser(t, m, "git", 998, "/bin/bash", "!", "", true)
		}, SSH{Port: 22, PermitRootLogin: "no"}, "ssh_no_login"},
		{"AllowUsers leaves the user out", func(t *testing.T, m *Manager, f *fake) {
			addUser(t, m, "deploy", 1000, "/bin/bash", "!", "", true)
			f.sshdT = "port 22\npasswordauthentication yes\npermitrootlogin yes\nallowusers root\n"
		}, SSH{Port: 22, PermitRootLogin: "no"}, "ssh_no_login"},
		{"AllowGroups lets the user's group in", func(t *testing.T, m *Manager, f *fake) {
			addUser(t, m, "deploy", 1000, "/bin/bash", "!", "sudo", true)
			f.sshdT = "port 22\npasswordauthentication yes\npermitrootlogin yes\nallowgroups sudo root\n"
		}, SSH{Port: 22, PermitRootLogin: "no"}, ""},
		{"DenyUsers with a pattern", func(t *testing.T, m *Manager, f *fake) {
			addUser(t, m, "deploy", 1000, "/bin/bash", "!", "", true)
			f.sshdT = "port 22\npasswordauthentication yes\npermitrootlogin yes\ndenyusers dep*\n"
		}, SSH{Port: 22, PermitRootLogin: "no"}, "ssh_no_login"},
		{"keys in a custom AuthorizedKeysFile", func(t *testing.T, m *Manager, f *fake) {
			dir := filepath.Dir(m.passwd)
			os.MkdirAll(filepath.Join(dir, "keys"), 0o755)
			os.WriteFile(filepath.Join(dir, "keys", "root"), []byte("ssh-ed25519 AAAA x"+nl), 0o600)
			f.sshdT = "port 22\npasswordauthentication yes\npermitrootlogin yes\nauthorizedkeysfile " + filepath.Join(dir, "keys", "%u") + "\n"
		}, SSH{Port: 22, PermitRootLogin: "prohibit-password"}, ""},
		{"nobody can sign in now: other changes stay possible", func(t *testing.T, m *Manager, f *fake) {
			os.WriteFile(m.shadow, []byte("root:!:19000:0:99999:7:::"+nl), 0o600)
		}, SSH{Port: 22, PasswordAuth: true, PermitRootLogin: "yes"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, f := newTest(t)
			if c.setup != nil {
				c.setup(t, m, f)
			}
			if got := code(m.SetSSH(ctx, c.set)); got != c.want {
				t.Fatalf("SetSSH(%+v) = %q, want %q (logins now: %v)", c.set, got, c.want, m.SSHSettings(ctx).Logins)
			}
		})
	}
}

func TestSSHLogins(t *testing.T) {
	m, f := newTest(t)
	os.WriteFile(m.rootKeys, []byte("ssh-ed25519 AAAA admin"+nl), 0o600)
	addUser(t, m, "deploy", 1000, "/bin/bash", "$6$x$y", "", false)
	f.sshdT = "port 22\npasswordauthentication yes\npermitrootlogin prohibit-password\n"
	got := fmt.Sprint(m.SSHSettings(context.Background()).Logins)
	if got != "[{root true false} {deploy false true}]" {
		t.Fatalf("logins = %q", got)
	}
}

// A rule for SSH or the panel can be deleted when another rule still lets
// the administrator in.
func TestFirewallForClient(t *testing.T) {
	m, f := newTest(t)
	f.ufw = `Status: active

     To                         Action      From
     --                         ------      ----
[ 1] 22/tcp                     LIMIT IN    Anywhere                   # SSH
[ 2] 46949/tcp                  ALLOW IN    Anywhere                   # Nefo
[ 3] 46949/tcp                  ALLOW IN    203.0.113.0/24            # Office
[ 4] 46949/udp                  ALLOW IN    198.51.100.7
[ 5] 22/tcp (v6)                ALLOW IN    Anywhere (v6)              # SSH
[ 6] 46949/tcp (v6)             ALLOW IN    Anywhere (v6)              # Nefo
[ 7] 443/tcp                    DENY IN     2001:db8::7
`
	ctx := context.Background()
	prot := func(fw Firewall) string {
		var b strings.Builder
		for _, r := range fw.Rules {
			if r.Protected {
				fmt.Fprint(&b, r.Number)
			}
		}
		return b.String()
	}
	if got := prot(m.Firewall(ctx)); got != "12356" {
		t.Fatalf("without a client: protected %q", got)
	}
	if fw := m.Firewall(ctx); !fw.Rules[6].V6 || fw.Rules[2].V6 {
		t.Fatalf("IPv6 flags: %+v", fw.Rules)
	}
	// From the office: either IPv4 panel rule may go, and the IPv6 ones.
	if got := prot(m.FirewallFor(ctx, "203.0.113.35")); got != "1" {
		t.Fatalf("office client: protected %q", got)
	}
	// From elsewhere: the office rule may go, not "anywhere".
	if got := prot(m.FirewallFor(ctx, "198.51.100.7")); got != "12" {
		t.Fatalf("other client: protected %q", got)
	}
	// From IPv6: the IPv4 rules may go, not the IPv6 ones.
	if got := prot(m.FirewallFor(ctx, "2001:db8::1")); got != "56" {
		t.Fatalf("IPv6 client: protected %q", got)
	}
	if got := code(m.DeleteRuleFor(ctx, 2, "46949/tcp", "Anywhere", "198.51.100.7")); got != "rule_protected" {
		t.Fatalf("delete the only rule for the client: %q", got)
	}
	if err := m.DeleteRuleFor(ctx, 2, "46949/tcp", "Anywhere", "203.0.113.35"); err != nil {
		t.Fatal(err)
	}
	if last := f.calls[len(f.calls)-1]; last != "ufw --force delete 2" {
		t.Fatalf("ran %q", last)
	}
}

func TestPortIn(t *testing.T) {
	for to, want := range map[string]bool{
		"22": true, "22/tcp": true, "22/tcp (v6)": true, "80,22/tcp": true, "20:30/tcp": true,
		"Anywhere": true, "Anywhere (v6)": true, "23/tcp": false, "2222/tcp": false, "x": false,
	} {
		if got := portIn(to, 22); got != want {
			t.Errorf("portIn(%q, 22) = %v", to, got)
		}
	}
}

func TestDenyFromAddressComesFirst(t *testing.T) {
	m, f := newTest(t)
	ctx := context.Background()
	if err := m.AddRule(ctx, NewRule{Action: "deny", Port: "80", Proto: "tcp", From: "198.51.100.7"}); err != nil {
		t.Fatal(err)
	}
	if last := f.calls[len(f.calls)-1]; last != "ufw prepend deny from 198.51.100.7 to any port 80 proto tcp" {
		t.Fatalf("ran %q", last)
	}
	if err := m.AddRule(ctx, NewRule{Action: "deny", Port: "3306"}); err != nil {
		t.Fatal(err)
	}
	if last := f.calls[len(f.calls)-1]; last != "ufw deny from any to any port 3306" {
		t.Fatalf("ran %q", last)
	}
}

func TestEnableFirewallOpensWebPorts(t *testing.T) {
	m, f := newTest(t)
	m.SetWebPorts(func() []string { return []string{"80/tcp", "443/tcp", "443/udp"} })
	if err := m.EnableFirewall(context.Background()); err != nil {
		t.Fatal(err)
	}
	// ufwOut already has 80/tcp and a panel rule for one address.
	want := []string{
		"ufw allow 22/tcp comment SSH",
		"ufw allow 443/tcp comment Websites",
		"ufw allow 443/udp comment Websites",
		"ufw --force enable",
	}
	var got []string
	for _, c := range f.calls {
		if strings.HasPrefix(c, "ufw allow") || strings.HasPrefix(c, "ufw --force") {
			got = append(got, c)
		}
	}
	if strings.Join(got, nl) != strings.Join(want, nl) {
		t.Fatalf("ran:%s%s", nl, strings.Join(got, nl))
	}
	if fw := m.Firewall(context.Background()); len(fw.WebPorts) != 3 {
		t.Fatalf("web ports = %v", fw.WebPorts)
	}
}

func TestFail2banFollowsSSHPort(t *testing.T) {
	m, f := newTest(t)
	ctx := context.Background()
	// Not set up by the panel: nothing is written.
	if changed, err := m.RefreshFail2ban(ctx); err != nil || changed {
		t.Fatalf("no jail: changed %v, %v", changed, err)
	}
	if _, err := os.Stat(m.f2bJail); err == nil {
		t.Fatal("jail written without fail2ban")
	}
	os.MkdirAll(filepath.Dir(m.f2bJail), 0o755)
	os.WriteFile(m.f2bJail, []byte("# old"+nl), 0o644)
	if err := m.SetSSH(ctx, SSH{Port: 2222, PasswordAuth: true, PermitRootLogin: "yes"}); err != nil {
		t.Fatal(err)
	}
	jail, _ := os.ReadFile(m.f2bJail)
	if !strings.Contains(string(jail), "[sshd]"+nl+"enabled = true"+nl+"port = 2222") || !strings.Contains(string(jail), "port = 46949") {
		t.Fatalf("jail:%s%s", nl, jail)
	}
	filter, _ := os.ReadFile(m.f2bFilter)
	if !strings.Contains(string(filter), "action=(?:login_failed|login_blocked) ip=<HOST>$") {
		t.Fatalf("filter:%s%s", nl, filter)
	}
	if last := f.calls[len(f.calls)-1]; last != "systemctl try-reload-or-restart fail2ban" {
		t.Fatalf("ran %q", last)
	}
	// Up to date: no reload.
	f.sshdT = "port 2222\npasswordauthentication yes\npermitrootlogin yes\n"
	n := len(f.calls)
	if changed, err := m.RefreshFail2ban(ctx); err != nil || changed {
		t.Fatalf("up to date: changed %v, %v", changed, err)
	}
	for _, c := range f.calls[n:] {
		if strings.Contains(c, "fail2ban") {
			t.Fatalf("ran %q", c)
		}
	}
}
