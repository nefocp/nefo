package backup

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// testSFTPServer is an SSH server with the SFTP subsystem, serving a
// temporary folder, on 127.0.0.1.
type testSFTPServer struct {
	t        *testing.T
	ln       net.Listener
	dir      string
	user     string
	password string

	mu       sync.Mutex
	hostKey  ssh.Signer
	userKeys []ssh.PublicKey
}

func newTestSFTPServer(t *testing.T) *testSFTPServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &testSFTPServer{t: t, ln: ln, dir: t.TempDir(), user: "backup", password: "correct horse", hostKey: newEd25519Signer(t)}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.handle(conn)
		}
	}()
	return s
}

func (s *testSFTPServer) port() int { return s.ln.Addr().(*net.TCPAddr).Port }

func (s *testSFTPServer) setHostKey(k ssh.Signer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hostKey = k
}

func (s *testSFTPServer) allowKey(k ssh.PublicKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.userKeys = append(s.userKeys, k)
}

func (s *testSFTPServer) handle(conn net.Conn) {
	defer conn.Close()
	s.mu.Lock()
	cfg := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pw []byte) (*ssh.Permissions, error) {
			if c.User() == s.user && string(pw) == s.password {
				return nil, nil
			}
			return nil, errors.New("wrong password")
		},
		PublicKeyCallback: func(c ssh.ConnMetadata, k ssh.PublicKey) (*ssh.Permissions, error) {
			s.mu.Lock()
			defer s.mu.Unlock()
			for _, ok := range s.userKeys {
				if c.User() == s.user && bytes.Equal(k.Marshal(), ok.Marshal()) {
					return nil, nil
				}
			}
			return nil, errors.New("unknown key")
		},
	}
	cfg.AddHostKey(s.hostKey)
	s.mu.Unlock()
	_, chans, reqs, err := ssh.NewServerConn(conn, cfg)
	if err != nil {
		return
	}
	go ssh.DiscardRequests(reqs)
	for nc := range chans {
		if nc.ChannelType() != "session" {
			nc.Reject(ssh.UnknownChannelType, "")
			continue
		}
		ch, chReqs, err := nc.Accept()
		if err != nil {
			return
		}
		go func() {
			for req := range chReqs {
				ok := req.Type == "subsystem" && len(req.Payload) > 4 && string(req.Payload[4:]) == "sftp"
				req.Reply(ok, nil)
				if !ok {
					continue
				}
				srv, err := sftp.NewServer(ch, sftp.WithServerWorkingDirectory(s.dir))
				if err != nil {
					ch.Close()
					return
				}
				go func() {
					srv.Serve()
					srv.Close()
				}()
			}
		}()
	}
}

func newEd25519Signer(t *testing.T) ssh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	s, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// testKeys returns private keys in the formats users paste, with the
// public key each one should give.
func testKeys(t *testing.T) map[string]struct {
	pem, passphrase string
	pub             ssh.PublicKey
} {
	t.Helper()
	out := map[string]struct {
		pem, passphrase string
		pub             ssh.PublicKey
	}{}
	pubOf := func(k any) ssh.PublicKey {
		s, err := ssh.NewSignerFromKey(k)
		if err != nil {
			t.Fatal(err)
		}
		return s.PublicKey()
	}
	_, ed, _ := ed25519.GenerateKey(rand.Reader)
	b, err := ssh.MarshalPrivateKey(ed, "panel")
	if err != nil {
		t.Fatal(err)
	}
	out["openssh ed25519"] = struct {
		pem, passphrase string
		pub             ssh.PublicKey
	}{string(pem.EncodeToMemory(b)), "", pubOf(ed)}

	b, err = ssh.MarshalPrivateKeyWithPassphrase(ed, "panel", []byte("s3cret phrase"))
	if err != nil {
		t.Fatal(err)
	}
	out["openssh encrypted"] = struct {
		pem, passphrase string
		pub             ssh.PublicKey
	}{string(pem.EncodeToMemory(b)), "s3cret phrase", pubOf(ed)}

	rk, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	// Pasted from Windows: CRLF line ends.
	rsaPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rk)})
	out["pem rsa (crlf)"] = struct {
		pem, passphrase string
		pub             ssh.PublicKey
	}{strings.ReplaceAll(string(rsaPEM), "\n", "\r\n"), "", pubOf(rk)}

	ek, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, err := x509.MarshalPKCS8PrivateKey(ek)
	if err != nil {
		t.Fatal(err)
	}
	out["pkcs8 ecdsa"] = struct {
		pem, passphrase string
		pub             ssh.PublicKey
	}{string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), "", pubOf(ek)}
	return out
}

func (s *testSFTPServer) config() SFTPConfig {
	return SFTPConfig{Host: "127.0.0.1", Port: s.port(), Username: s.user, Auth: SFTPAuthPassword,
		Password: s.password, Folder: "backups/panel"}
}

type testLog struct{ t *testing.T }

func (l testLog) Logf(format string, args ...any) { l.t.Logf(format, args...) }

func newTestManager(t *testing.T) (*Manager, *store.Store) {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	m := New(st, nil, nil, t.TempDir(), slog.New(slog.DiscardHandler))
	return m, st
}

func userCode(err error) string {
	var ue *usererr.Error
	if errors.As(err, &ue) {
		return ue.Code
	}
	return ""
}

func TestSFTPConfigValidate(t *testing.T) {
	good := SFTPConfig{Host: " backup.example.com ", Username: "u123", Auth: SFTPAuthPassword, Password: "pw",
		PrivateKey: "left over", Folder: "~/backups/panel/"}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
	if good.Host != "backup.example.com" || good.Port != 22 || good.Folder != "backups/panel" || good.PrivateKey != "" {
		t.Fatalf("not normalised: %+v", good)
	}
	v6 := SFTPConfig{Host: "[2001:db8::1]", Port: 2222, Username: "u", Auth: SFTPAuthPassword, Password: "pw", Folder: "/srv/backups"}
	if err := v6.Validate(); err != nil || v6.Host != "2001:db8::1" || v6.Folder != "/srv/backups" {
		t.Fatalf("%+v %v", v6, err)
	}
	bad := map[string]SFTPConfig{
		"backup_sftp_host":     {Host: "bad host", Username: "u", Auth: SFTPAuthPassword, Password: "pw"},
		"backup_sftp_port":     {Host: "h", Port: 70000, Username: "u", Auth: SFTPAuthPassword, Password: "pw"},
		"backup_sftp_username": {Host: "h", Username: "a b", Auth: SFTPAuthPassword, Password: "pw"},
		"backup_sftp_password": {Host: "h", Username: "u", Auth: SFTPAuthPassword},
		"backup_sftp_auth":     {Host: "h", Username: "u", Auth: "magic", Password: "pw"},
		"backup_sftp_folder":   {Host: "h", Username: "u", Auth: SFTPAuthPassword, Password: "pw", Folder: "a/../../etc"},
		"backup_sftp_key":      {Host: "h", Username: "u", Auth: SFTPAuthKey, PrivateKey: "PuTTY-User-Key-File-3: ssh-ed25519"},
	}
	for code, c := range bad {
		if err := c.Validate(); userCode(err) != code {
			t.Errorf("%s: got %v", code, err)
		}
	}

	for name, k := range testKeys(t) {
		c := SFTPConfig{Host: "h", Username: "u", Auth: SFTPAuthKey, PrivateKey: k.pem, Passphrase: k.passphrase, Password: "x"}
		if err := c.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		pub, _ := parsePublicKey(c.PublicKey)
		if pub == nil || !bytes.Equal(pub.Marshal(), k.pub.Marshal()) || c.Password != "" {
			t.Errorf("%s: public key %q, password %q", name, c.PublicKey, c.Password)
		}
		if k.passphrase == "" {
			c := SFTPConfig{Host: "h", Username: "u", Auth: SFTPAuthKey, PrivateKey: k.pem, Passphrase: "not needed"}
			if err := c.Validate(); err != nil || c.Passphrase != "" {
				t.Errorf("%s: a needless passphrase is kept or refused: %v", name, err)
			}
			continue
		}
		c = SFTPConfig{Host: "h", Username: "u", Auth: SFTPAuthKey, PrivateKey: k.pem}
		if err := c.Validate(); userCode(err) != "backup_sftp_passphrase" {
			t.Errorf("%s without passphrase: %v", name, err)
		}
		c = SFTPConfig{Host: "h", Username: "u", Auth: SFTPAuthKey, PrivateKey: k.pem, Passphrase: "wrong"}
		if err := c.Validate(); userCode(err) != "backup_sftp_wrong_passphrase" {
			t.Errorf("%s with a wrong passphrase: %v", name, err)
		}
	}
}

func TestSFTPClientOperations(t *testing.T) {
	srv := newTestSFTPServer(t)
	cfg := srv.config()
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	c, err := dialSFTP(ctx, cfg, true)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if !bytes.Equal(c.hostKey.Marshal(), srv.hostKey.PublicKey().Marshal()) {
		t.Fatal("presented host key not recorded")
	}

	key := "site/example.com/example.com_20261125-030000.tar.gz"
	body := strings.Repeat("backup data ", 100_000) // several SFTP packets, written concurrently
	if err := c.Put(ctx, key, strings.NewReader(body), int64(len(body)), ""); err != nil {
		t.Fatal(err)
	}
	onDisk := filepath.Join(srv.dir, "backups", "panel", "site", "example.com", "example.com_20261125-030000.tar.gz")
	if b, err := os.ReadFile(onDisk); err != nil || string(b) != body {
		t.Fatalf("file on the server: %d bytes, %v", len(b), err)
	}
	entries, _ := os.ReadDir(filepath.Dir(onDisk))
	if len(entries) != 1 {
		t.Fatalf("temporary upload left behind: %v", entries)
	}

	// Uploading again replaces the file (posix-rename).
	if err := c.Put(ctx, key, strings.NewReader("v2"), 2, ""); err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	if err := c.Get(ctx, key, &got); err != nil || got.String() != "v2" {
		t.Fatalf("Get = %q, %v", got.String(), err)
	}

	// A short reader must not leave a file under the final name.
	if err := c.Put(ctx, "site/example.com/short.tar.gz", strings.NewReader("abc"), 10, ""); err == nil {
		t.Fatal("short upload accepted")
	}
	files, err := c.List(ctx, "site/example.com")
	if err != nil || len(files) != 1 || files[0].Name != filepath.Base(key) || files[0].Size != 2 {
		t.Fatalf("List = %+v, %v", files, err)
	}
	if files, err := c.List(ctx, "site/missing.example"); err != nil || len(files) != 0 {
		t.Fatalf("List of a missing folder = %+v, %v", files, err)
	}

	if err := c.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(onDisk); !os.IsNotExist(err) {
		t.Fatalf("file still on the server: %v", err)
	}
	if err := c.Delete(ctx, key); err != nil {
		t.Fatalf("deleting a missing file: %v", err)
	}
	if err := c.Get(ctx, key, io.Discard); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("Get after delete: %v", err)
	}
	if err := testStorage(ctx, c); err != nil {
		t.Fatalf("storage test: %v", err)
	}
}

func TestSFTPKeyAuth(t *testing.T) {
	srv := newTestSFTPServer(t)
	for name, k := range testKeys(t) {
		srv.allowKey(k.pub)
		cfg := SFTPConfig{Host: "127.0.0.1", Port: srv.port(), Username: srv.user, Auth: SFTPAuthKey,
			PrivateKey: k.pem, Passphrase: k.passphrase}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		c, err := dialSFTP(context.Background(), cfg, true)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if err := testStorage(context.Background(), c); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		c.Close()
	}

	// A key the server does not know.
	other := testKeys(t)["openssh ed25519"]
	cfg := SFTPConfig{Host: "127.0.0.1", Port: srv.port(), Username: srv.user, Auth: SFTPAuthKey, PrivateKey: other.pem}
	cfg.Validate()
	if _, err := dialSFTP(context.Background(), cfg, true); userCode(sftpUserError(err)) != "backup_sftp_login" {
		t.Fatalf("unknown key: %v", err)
	}
}

func TestSFTPSaveAndHostKeyPinning(t *testing.T) {
	srv := newTestSFTPServer(t)
	m, st := newTestManager(t)
	ctx := context.Background()

	wrong := srv.config()
	wrong.Password = "wrong"
	if _, err := m.SaveSFTP(ctx, wrong); userCode(err) != "backup_sftp_login" {
		t.Fatalf("wrong password: %v", err)
	}
	if v, _ := m.SFTPStorage(ctx); v != nil {
		t.Fatal("settings saved although the test failed")
	}

	res, err := m.SaveSFTP(ctx, srv.config())
	if err != nil {
		t.Fatal(err)
	}
	fp := ssh.FingerprintSHA256(srv.hostKey.PublicKey())
	if res.SFTP.HostKeyFingerprint != fp || res.SFTP.HostKeyType != ssh.KeyAlgoED25519 || res.PreviousFingerprint != "" {
		t.Fatalf("saved = %+v", res)
	}
	if !strings.HasPrefix(fp, "SHA256:") {
		t.Fatalf("fingerprint %q", fp)
	}

	// The UI never gets the password back, and an empty one keeps it.
	view, _ := m.SFTPStorage(ctx)
	vb, _ := json.Marshal(view)
	if strings.Contains(string(vb), srv.password) || !view.HasPassword {
		t.Fatalf("view = %s", vb)
	}
	again := srv.config()
	again.Password = ""
	if _, err := m.SaveSFTP(ctx, again); err != nil {
		t.Fatalf("saving without retyping the password: %v", err)
	}
	stored, _ := st.Setting(ctx, sftpSettingKey)
	if !strings.Contains(stored, srv.password) {
		t.Fatal("stored password lost")
	}

	// Later connections must present the trusted key.
	r, err := m.openRemote(ctx, RemoteSFTP)
	if err != nil {
		t.Fatal(err)
	}
	r.Close()

	newKey := newEd25519Signer(t)
	srv.setHostKey(newKey)
	_, err = m.openRemote(ctx, RemoteSFTP)
	var hk *HostKeyError
	if !errors.As(err, &hk) || hk.Got == nil {
		t.Fatalf("changed host key accepted: %v", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, ssh.FingerprintSHA256(newKey.PublicKey())) || !strings.Contains(msg, fp) || !strings.Contains(msg, "Test and save") {
		t.Fatalf("unclear message: %s", msg)
	}

	// The server only has a key of another type now: refused as well.
	rk, _ := rsa.GenerateKey(rand.Reader, 2048)
	rsaSigner, _ := ssh.NewSignerFromKey(rk)
	srv.setHostKey(rsaSigner)
	if _, err = m.openRemote(ctx, RemoteSFTP); !errors.As(err, &hk) || hk.Got != nil || !strings.Contains(err.Error(), "no longer offers") {
		t.Fatalf("other key type: %v", err)
	}

	// Test and save again trusts the new key and tells which one it replaced.
	srv.setHostKey(newKey)
	res, err = m.SaveSFTP(ctx, again)
	if err != nil {
		t.Fatal(err)
	}
	if res.PreviousFingerprint != fp || res.SFTP.HostKeyFingerprint != ssh.FingerprintSHA256(newKey.PublicKey()) {
		t.Fatalf("after re-trust: %+v", res)
	}
	r, err = m.openRemote(ctx, RemoteSFTP)
	if err != nil {
		t.Fatalf("new key not trusted: %v", err)
	}
	r.Close()

	// Switching to a key keeps no password.
	k := testKeys(t)["openssh encrypted"]
	srv.allowKey(k.pub)
	withKey := srv.config()
	withKey.Auth, withKey.Password, withKey.PrivateKey, withKey.Passphrase = SFTPAuthKey, "", k.pem, k.passphrase
	if _, err := m.SaveSFTP(ctx, withKey); err != nil {
		t.Fatal(err)
	}
	view, _ = m.SFTPStorage(ctx)
	if view.HasPassword || !view.HasKey || !view.HasPassphrase || view.KeyFingerprint != ssh.FingerprintSHA256(k.pub) {
		t.Fatalf("view = %+v", view)
	}
	stored, _ = st.Setting(ctx, sftpSettingKey)
	if strings.Contains(stored, srv.password) {
		t.Fatal("old password still stored")
	}
	// Keeping the stored key and passphrase.
	withKey.PrivateKey, withKey.Passphrase = "", ""
	if _, err := m.SaveSFTP(ctx, withKey); err != nil {
		t.Fatalf("saving without repasting the key: %v", err)
	}

	if err := m.RemoveSFTP(ctx); err != nil {
		t.Fatal(err)
	}
	if names, _ := m.ConfiguredRemotes(ctx); len(names) != 0 {
		t.Fatalf("remotes after removal: %v", names)
	}
}

func TestSFTPHandshakeTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() { // accepts, then says nothing
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			defer c.Close()
		}
	}()
	old := sftpHandshakeTimeout
	sftpHandshakeTimeout = 300 * time.Millisecond
	defer func() { sftpHandshakeTimeout = old }()
	cfg := SFTPConfig{Host: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port, Username: "u", Auth: SFTPAuthPassword, Password: "p"}
	start := time.Now()
	_, err = dialSFTP(context.Background(), cfg, true)
	if userCode(sftpUserError(err)) != "backup_sftp_timeout" || time.Since(start) > 10*time.Second {
		t.Fatalf("silent server: %v after %s", err, time.Since(start))
	}

	// Nothing listens: a quick, clear error.
	ln2, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln2.Addr().(*net.TCPAddr).Port
	ln2.Close()
	cfg.Port = port
	if _, err := dialSFTP(context.Background(), cfg, true); userCode(sftpUserError(err)) != "backup_sftp_connect" ||
		!strings.Contains(err.Error(), strconv.Itoa(port)) {
		t.Fatalf("closed port: %v", err)
	}
}

func TestLocations(t *testing.T) {
	locs := parseLocations("site/a.com/a.tar.gz sftp:site/a.com/a.tar.gz")
	if len(locs) != 2 || locs[0] != (location{RemoteS3, "site/a.com/a.tar.gz"}) || locs[1] != (location{RemoteSFTP, "site/a.com/a.tar.gz"}) {
		t.Fatalf("%+v", locs)
	}
	if s := formatLocations(locs); s != "site/a.com/a.tar.gz sftp:site/a.com/a.tar.gz" {
		t.Fatal(s)
	}
	if got := Remotes(&store.Backup{RemoteKey: "database/db/db.tar.gz"}); len(got) != 1 || got[0] != RemoteS3 {
		t.Fatalf("legacy key: %v", got)
	}
	if got := Remotes(&store.Backup{}); len(got) != 0 {
		t.Fatalf("no copy: %v", got)
	}
}
