package hosting

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/nefocp/nefo/internal/usererr"
)

func testKey(t *testing.T) (line string, fp string) {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	k, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(k))), ssh.FingerprintSHA256(k)
}

func TestParseKeys(t *testing.T) {
	a, fpA := testKey(t)
	b, _ := testKey(t)
	text := "# my keys\n\n" + `command="/bin/sh",no-pty ` + a + " laptop\x07 key\n  " + b + "  \n"
	lines, keys, err := parseKeys(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 || len(keys) != 2 {
		t.Fatalf("got %d lines, %d keys", len(lines), len(keys))
	}
	// Options are dropped and control characters removed from the comment.
	if lines[0] != a+" laptop key" {
		t.Fatalf("line 0 = %q", lines[0])
	}
	if lines[1] != b {
		t.Fatalf("line 1 = %q", lines[1])
	}
	if keys[0].Type != "ssh-ed25519" || keys[0].Fingerprint != fpA || keys[0].Comment != "laptop key" {
		t.Fatalf("key 0 = %+v", keys[0])
	}

	for _, bad := range []string{"ssh-ed25519 AAAAnotbase64", "hello world", a + "\nnot-a-key"} {
		if _, _, err := parseKeys(bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
	if lines, keys, err := parseKeys(""); err != nil || len(lines) != 0 || len(keys) != 0 {
		t.Fatalf("empty text: %v %v %v", lines, keys, err)
	}
	if _, _, err := parseKeys(strings.Repeat(a+"\n", maxSFTPKeys+1)); err == nil {
		t.Fatal("accepted too many keys")
	}
}

func TestSFTPPassword(t *testing.T) {
	for pw, ok := range map[string]bool{
		"short":                      false,
		"twelve chars":               true,
		"has:colon and more":         true,
		"new\nline in password":      false,
		"tab\tin the password":       false,
		strings.Repeat("x", 129):     false,
		"Mật khẩu có dấu tiếng Việt": true,
	} {
		if validSFTPPassword(pw) != ok {
			t.Errorf("validSFTPPassword(%q) = %v", pw, !ok)
		}
	}
}

func TestSSHDropIn(t *testing.T) {
	s := string(sshdDropInBytes())
	mustContain(t, s,
		"DenyGroups panel-site\n",
		"Match Group panel-sftp\n",
		"ChrootDirectory /var/lib/panel-sftp/%u\n",
		"ForceCommand internal-sftp -u 0027\n",
		"AuthorizedKeysFile /etc/ssh/panel-sftp-keys/%u\n",
		"DisableForwarding yes\n",
		"PermitTTY no\n",
	)
	// DenyGroups is a global setting: it must come before the Match block.
	if strings.Index(s, "DenyGroups") > strings.Index(s, "Match Group") {
		t.Fatal("DenyGroups is inside the Match block")
	}
}

func TestVhostDisablesForeignSymlinks(t *testing.T) {
	d := baseVhost()
	out, err := renderVhost(d)
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, string(out), "disable_symlinks if_not_owner from=$document_root;")
}

func TestSetPasswordPolicy(t *testing.T) {
	r := &fakeRunner{outputs: map[string]fakeResult{
		"chpasswd": {out: "BAD PASSWORD: The password fails the dictionary check - it is too simplistic/systematic\nchpasswd: (line 1, user u) password not changed\n", err: errors.New("exit status 1")},
	}}
	m := &Manager{run: r}
	err := m.setPassword(context.Background(), "u", "aaaaaaaaaaaa")
	if !errors.Is(err, errWeakPassword) {
		t.Fatalf("err = %v, want errWeakPassword", err)
	}
	var ue *usererr.Error
	if !errors.As(err, &ue) || ue.Code != "weak_sftp_password" || !strings.Contains(ue.Detail, "too simplistic") {
		t.Fatalf("user error = %+v", ue)
	}
	if got := r.stdin["chpasswd"]; got != "u:aaaaaaaaaaaa\n" {
		t.Fatalf("stdin = %q", got)
	}
	r.outputs["chpasswd"] = fakeResult{out: "chpasswd: something else\n", err: errors.New("exit status 1")}
	if err := m.setPassword(context.Background(), "u", "x"); err == nil || errors.Is(err, errWeakPassword) {
		t.Fatalf("other failure: %v", err)
	}
}

type fakeResult struct {
	out string
	err error
}

// fakeRunner answers commands by name and records what was sent on stdin.
type fakeRunner struct {
	outputs map[string]fakeResult
	stdin   map[string]string
}

func (f *fakeRunner) Output(_ context.Context, name string, _ ...string) (string, error) {
	r := f.outputs[name]
	return r.out, r.err
}

func (f *fakeRunner) Input(ctx context.Context, stdin []byte, name string, args ...string) (string, error) {
	if f.stdin == nil {
		f.stdin = map[string]string{}
	}
	f.stdin[name] = string(stdin)
	return f.Output(ctx, name, args...)
}

func (f *fakeRunner) Stream(ctx context.Context, w io.Writer, name string, args ...string) error {
	_, err := f.Output(ctx, name, args...)
	return err
}

func (f *fakeRunner) Pipe(ctx context.Context, _ io.Reader, _ io.Writer, name string, args ...string) error {
	_, err := f.Output(ctx, name, args...)
	return err
}
