package security

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

// keysTest is newTest with root's keys in its home folder, as on a real
// server, and with chown recorded instead of done.
func keysTest(t *testing.T) (*Manager, *fake, *[]string) {
	m, f := newTest(t)
	dir := filepath.Dir(m.passwd)
	m.rootKeys = filepath.Join(dir, "root", ".ssh", "authorized_keys")
	os.MkdirAll(filepath.Join(dir, "root"), 0o700)
	chowns := &[]string{}
	m.chown = func(r *os.Root, name string, uid, gid int) error {
		*chowns = append(*chowns, fmt.Sprintf("%s %d:%d", filepath.ToSlash(name), uid, gid))
		return nil
	}
	return m, f, chowns
}

func homeOf(m *Manager, name string) string {
	return filepath.Join(filepath.Dir(m.passwd), "home", name)
}

// edKey returns a new Ed25519 public key line and its fingerprint.
func edKey(t *testing.T, comment string) (string, string) {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	k, _ := ssh.NewPublicKey(pub)
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(k))) + " " + comment, ssh.FingerprintSHA256(k)
}

func rsaKey(t *testing.T, bits int, comment string) (string, string) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		t.Fatal(err)
	}
	k, _ := ssh.NewPublicKey(&priv.PublicKey)
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(k))) + " " + comment, ssh.FingerprintSHA256(k)
}

// dsaKey returns an ssh-dss line: sizes right (1024 and 160 bits), numbers
// random, which is enough to parse.
func dsaKey() string {
	n := func(bits int) *big.Int {
		b := make([]byte, bits/8)
		rand.Read(b)
		b[0] |= 0x80
		return new(big.Int).SetBytes(b)
	}
	blob := ssh.Marshal(struct {
		Name       string
		P, Q, G, Y *big.Int
	}{"ssh-dss", n(1024), n(160), n(1016), n(1016)})
	return "ssh-dss " + base64.StdEncoding.EncodeToString(blob) + " old"
}

func writeKeys(t *testing.T, path, text string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o700)
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readText(path string) string {
	b, _ := os.ReadFile(path)
	return string(b)
}

func TestSSHKeysList(t *testing.T) {
	m, f, _ := keysTest(t)
	rootLine, rootFP := edKey(t, "admin@laptop")
	writeKeys(t, m.rootKeys, "# keys"+nl+`from="203.0.113.0/24",no-pty `+rootLine+nl+"not a key"+nl)
	addUser(t, m, "deploy", 1000, "/bin/bash", "!", "", false)
	deployLine, deployFP := rsaKey(t, 2048, "deploy@ci")
	writeKeys(t, filepath.Join(homeOf(m, "deploy"), ".ssh", "authorized_keys2"), deployLine+nl)
	addUser(t, m, "site1", 1001, "/usr/sbin/nologin", "!", "", true) // an SFTP account
	addUser(t, m, "git", 998, "/bin/bash", "!", "", true)            // a service account
	f.sshdT = "port 22\npasswordauthentication no\npermitrootlogin prohibit-password\nallowusers root\n"

	got := m.SSHKeys(context.Background())
	if len(got.Accounts) != 2 || got.Accounts[0].User != "root" || got.Accounts[1].User != "deploy" || got.KeyCommand {
		t.Fatalf("accounts = %+v", got)
	}
	root := got.Accounts[0]
	if len(root.Keys) != 1 || root.Invalid != 1 || !root.Allowed || root.File != m.rootKeys {
		t.Fatalf("root = %+v", root)
	}
	k := root.Keys[0]
	if k.Type != "ed25519" || k.Bits != 256 || k.Fingerprint != rootFP || k.Comment != "admin@laptop" || k.Weak ||
		strings.Join(k.Options, " ") != `from="203.0.113.0/24" no-pty` || k.File != m.rootKeys {
		t.Fatalf("root key = %+v", k)
	}
	deploy := got.Accounts[1]
	wantFile := filepath.Join(homeOf(m, "deploy"), ".ssh", "authorized_keys")
	if deploy.Allowed || deploy.File != wantFile || len(deploy.Keys) != 1 || len(deploy.Skipped) != 0 {
		t.Fatalf("deploy = %+v", deploy)
	}
	if k := deploy.Keys[0]; k.Type != "rsa" || k.Bits != 2048 || k.Fingerprint != deployFP || k.Comment != "deploy@ci" || len(k.Options) != 0 {
		t.Fatalf("deploy key = %+v", k)
	}
}

func TestAddSSHKey(t *testing.T) {
	m, _, chowns := keysTest(t)
	ctx := context.Background()
	addUser(t, m, "deploy", 1000, "/bin/bash", "!", "", false)
	os.MkdirAll(homeOf(m, "deploy"), 0o755)
	file := filepath.Join(homeOf(m, "deploy"), ".ssh", "authorized_keys")

	line, fp := edKey(t, "me@laptop")
	k, err := m.AddSSHKey(ctx, "deploy", "  "+line+"\n")
	if err != nil {
		t.Fatal(err)
	}
	if k.Fingerprint != fp || k.File != file || k.Comment != "me@laptop" {
		t.Fatalf("added %+v", k)
	}
	if got := readText(file); got != line+nl {
		t.Fatalf("authorized_keys = %q", got)
	}
	// ~/.ssh and the new file belong to the account.
	if c := *chowns; len(c) != 2 || c[0] != ".ssh 1000:1000" || !strings.HasPrefix(c[1], ".ssh/.nefo-keys-") || !strings.HasSuffix(c[1], " 1000:1000") {
		t.Fatalf("chown: %q", c)
	}
	if runtime.GOOS != "windows" {
		di, _ := os.Stat(filepath.Dir(file))
		fi, _ := os.Stat(file)
		if di.Mode().Perm() != 0o700 || fi.Mode().Perm() != 0o600 {
			t.Fatalf("modes %v %v", di.Mode(), fi.Mode())
		}
	}
	if entries, _ := os.ReadDir(filepath.Dir(file)); len(entries) != 1 {
		t.Fatalf("left in ~/.ssh: %v", entries)
	}
	if got := fmt.Sprint(m.SSHSettings(ctx).Logins); got != "[{root false true} {deploy true false}]" {
		t.Fatalf("logins = %s", got)
	}

	// Appended to an existing file, after a last line without a newline,
	// with the key's options.
	old, _ := edKey(t, "old")
	writeKeys(t, m.rootKeys, "# mine"+nl+old)
	line2, _ := edKey(t, "ci")
	line2 = `from="203.0.113.7",no-pty ` + line2
	k2, err := m.AddSSHKey(ctx, "root", line2)
	if err != nil {
		t.Fatal(err)
	}
	if got := readText(m.rootKeys); got != "# mine"+nl+old+nl+line2+nl {
		t.Fatalf("root's authorized_keys = %q", got)
	}
	if strings.Join(k2.Options, " ") != `from="203.0.113.7" no-pty` {
		t.Fatalf("options = %q", k2.Options)
	}

	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	blk, _ := ssh.MarshalPrivateKey(priv, "")
	short, _ := rsaKey(t, 1024, "old")
	other, _ := edKey(t, "other")
	addUser(t, m, "site1", 1001, "/usr/sbin/nologin", "!", "", false)
	addUser(t, m, "git", 998, "/bin/bash", "!", "", false)
	bad := []struct{ user, key, want string }{
		{"deploy", line, "ssh_key_exists"},
		{"deploy", strings.Replace(line, "me@laptop", "same key, other comment", 1), "ssh_key_exists"},
		{"deploy", string(pem.EncodeToMemory(blk)), "ssh_key_private"},
		{"deploy", "-----BEGIN RSA PRIVATE KEY-----", "ssh_key_private"},
		{"deploy", short, "ssh_key_rsa_short"},
		{"deploy", dsaKey(), "ssh_key_dsa"},
		{"deploy", line + nl + other, "ssh_key_one_line"},
		{"deploy", "", "ssh_key_one_line"},
		{"deploy", "ssh-ed25519 AAAA nope", "invalid_ssh_key"},
		{"deploy", "hello", "invalid_ssh_key"},
		{"deploy", "bogus-option " + other, "invalid_ssh_key"},
		{"deploy", other + " \x1b[2J", "invalid_ssh_key"},
		{"site1", other, "ssh_key_bad_user"},
		{"git", other, "ssh_key_bad_user"},
		{"nobody-here", other, "ssh_key_bad_user"},
	}
	for _, c := range bad {
		if _, err := m.AddSSHKey(ctx, c.user, c.key); code(err) != c.want {
			t.Errorf("%s %.50q: %v, want %s", c.user, c.key, err, c.want)
		}
	}
	if got := readText(file); got != line+nl {
		t.Fatalf("refused keys changed the file: %q", got)
	}

	// No home folder to keep the key in.
	addUser(t, m, "ghost", 1002, "/bin/bash", "!", "", false)
	if _, err := m.AddSSHKey(ctx, "ghost", other); code(err) != "ssh_keys_no_folder" {
		t.Fatalf("no home: %v", err)
	}
}

func TestRemoveSSHKey(t *testing.T) {
	m, f, _ := keysTest(t)
	ctx := context.Background()
	a, fpA := edKey(t, "a")
	b, fpB := edKey(t, "b")
	dsa := dsaKey() // OpenSSH refuses it: not a way in
	writeKeys(t, m.rootKeys, "# keep me"+nl+a+nl+dsa+nl+b+nl)
	f.sshdT = "port 22\npasswordauthentication no\npermitrootlogin prohibit-password\n"

	k, err := m.RemoveSSHKey(ctx, "root", fpA)
	if err != nil {
		t.Fatal(err)
	}
	if k.Comment != "a" || k.File != m.rootKeys {
		t.Fatalf("removed %+v", k)
	}
	if got := readText(m.rootKeys); got != "# keep me"+nl+dsa+nl+b+nl {
		t.Fatalf("authorized_keys = %q", got)
	}
	if _, err := m.RemoveSSHKey(ctx, "root", fpA); code(err) != "ssh_key_not_found" {
		t.Fatalf("again: %v", err)
	}
	// b is the last way in: passwords are off, and the DSA key doesn't count.
	if _, err := m.RemoveSSHKey(ctx, "root", fpB); code(err) != "ssh_key_last_way_in" {
		t.Fatalf("last key: %v", err)
	}
	if got := readText(m.rootKeys); got != "# keep me"+nl+dsa+nl+b+nl {
		t.Fatalf("refused, but authorized_keys = %q", got)
	}

	// Another account with a key: root's last key may go.
	addUser(t, m, "deploy", 1000, "/bin/bash", "!", "", false)
	c, fpC := edKey(t, "c")
	writeKeys(t, filepath.Join(homeOf(m, "deploy"), ".ssh", "authorized_keys"), c+nl)
	if _, err := m.RemoveSSHKey(ctx, "root", fpB); err != nil {
		t.Fatal(err)
	}
	if got := readText(m.rootKeys); got != "# keep me"+nl+dsa+nl {
		t.Fatalf("authorized_keys = %q", got)
	}
	if _, err := m.RemoveSSHKey(ctx, "deploy", fpC); code(err) != "ssh_key_last_way_in" {
		t.Fatalf("deploy's last key: %v", err)
	}
	// root can sign in with its password: deploy's key may go.
	f.sshdT = "port 22\npasswordauthentication yes\npermitrootlogin yes\n"
	if _, err := m.RemoveSSHKey(ctx, "deploy", fpC); err != nil {
		t.Fatal(err)
	}
	if _, err := m.RemoveSSHKey(ctx, "site1", fpC); code(err) != "ssh_key_bad_user" {
		t.Fatalf("unknown account: %v", err)
	}
}

// The panel runs as root: a link in a user's home must not make it read or
// replace another file.
func TestSSHKeysDoNotFollowLinks(t *testing.T) {
	m, _, _ := keysTest(t)
	ctx := context.Background()
	dir := filepath.Dir(m.passwd)
	addUser(t, m, "deploy", 1000, "/bin/bash", "!", "", false)
	home := homeOf(m, "deploy")
	os.MkdirAll(filepath.Join(home, ".ssh"), 0o700)
	keys := filepath.Join(home, ".ssh", "authorized_keys")
	victimLine, _ := edKey(t, "victim")
	victim := filepath.Join(dir, "victim")
	os.WriteFile(victim, []byte(victimLine+nl), 0o600)
	if err := os.Symlink(victim, keys); err != nil {
		t.Skip("cannot make symbolic links here:", err)
	}
	line, _ := edKey(t, "me")
	if _, err := m.AddSSHKey(ctx, "deploy", line); code(err) != "ssh_keys_unsafe" {
		t.Fatalf("authorized_keys is a link: %v", err)
	}
	if got := readText(victim); got != victimLine+nl {
		t.Fatalf("victim = %q", got)
	}
	d := m.SSHKeys(ctx).Accounts[1]
	if d.User != "deploy" || len(d.Keys) != 0 || !slices.Contains(d.Skipped, keys) {
		t.Fatalf("listed %+v", d)
	}

	// ~/.ssh itself is a link to a folder elsewhere.
	os.Remove(keys)
	os.Remove(filepath.Join(home, ".ssh"))
	elsewhere := filepath.Join(dir, "elsewhere")
	os.MkdirAll(elsewhere, 0o700)
	if err := os.Symlink(elsewhere, filepath.Join(home, ".ssh")); err != nil {
		t.Fatal(err)
	}
	if _, err := m.AddSSHKey(ctx, "deploy", line); code(err) != "ssh_keys_unsafe" {
		t.Fatalf("~/.ssh is a link: %v", err)
	}
	if entries, _ := os.ReadDir(elsewhere); len(entries) != 0 {
		t.Fatalf("written elsewhere: %v", entries)
	}
}

// What is not a regular file is neither read nor replaced.
func TestSSHKeysNotRegular(t *testing.T) {
	m, _, _ := keysTest(t)
	ctx := context.Background()
	addUser(t, m, "deploy", 1000, "/bin/bash", "!", "", false)
	home := homeOf(m, "deploy")
	keys := filepath.Join(home, ".ssh", "authorized_keys")
	os.MkdirAll(keys, 0o700) // a folder
	line, _ := edKey(t, "me")
	if _, err := m.AddSSHKey(ctx, "deploy", line); code(err) != "ssh_keys_unsafe" {
		t.Fatalf("authorized_keys is a folder: %v", err)
	}
	if d := m.SSHKeys(ctx).Accounts[1]; !slices.Contains(d.Skipped, keys) {
		t.Fatalf("listed %+v", d)
	}
	os.RemoveAll(filepath.Join(home, ".ssh"))
	os.WriteFile(filepath.Join(home, ".ssh"), []byte("x"), 0o600) // a file
	if _, err := m.AddSSHKey(ctx, "deploy", line); code(err) != "ssh_keys_unsafe" {
		t.Fatalf("~/.ssh is a file: %v", err)
	}
}
