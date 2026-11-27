package backup

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/nefocp/nefo/internal/usererr"
)

const sftpSettingKey = "backup_sftp"

// Variables so that tests can shorten them.
var (
	sftpDialTimeout      = 15 * time.Second
	sftpHandshakeTimeout = 30 * time.Second // SSH handshake, login and SFTP start
	sftpKeepAlive        = 30 * time.Second // a server silent for twice this long is dropped
)

// Ways to sign in to an SFTP server.
const (
	SFTPAuthPassword = "password"
	SFTPAuthKey      = "key"
)

// SFTPConfig describes a folder on another server, reached over SFTP.
type SFTPConfig struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Auth       string `json:"auth"` // SFTPAuthPassword or SFTPAuthKey
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"private_key,omitempty"` // OpenSSH or PEM
	Passphrase string `json:"passphrase,omitempty"`  // of an encrypted private key
	// Folder holds the backups. It is relative to the account's home
	// folder unless it starts with a slash; empty means the home folder.
	Folder string `json:"folder"`
	// PublicKey is the public half of PrivateKey, in authorized_keys format.
	PublicKey string `json:"public_key,omitempty"`
	// HostKey is the server key trusted by the last "Test and save", in
	// authorized_keys format. Every later connection must present it.
	HostKey string `json:"host_key,omitempty"`
	Encryption
}

// SFTPView is what the UI sees of the SFTP settings: no secrets.
type SFTPView struct {
	Host               string `json:"host"`
	Port               int    `json:"port"`
	Username           string `json:"username"`
	Auth               string `json:"auth"`
	Folder             string `json:"folder"`
	HostKeyType        string `json:"host_key_type"`
	HostKeyFingerprint string `json:"host_key_fingerprint"` // SHA256:…
	HasPassword        bool   `json:"has_password"`
	HasKey             bool   `json:"has_key"`
	HasPassphrase      bool   `json:"has_passphrase"`
	KeyType            string `json:"key_type,omitempty"`
	KeyFingerprint     string `json:"key_fingerprint,omitempty"` // of the public key, as a hint
	PublicKey          string `json:"public_key,omitempty"`      // to add to authorized_keys
	Encrypt            bool   `json:"encrypt"`                   // copies are encrypted before upload
}

func (c *SFTPConfig) view() *SFTPView {
	v := &SFTPView{Host: c.Host, Port: c.Port, Username: c.Username, Auth: c.Auth, Folder: c.Folder,
		HasPassword: c.Password != "", HasKey: c.PrivateKey != "", HasPassphrase: c.Passphrase != "", Encrypt: c.Encrypt}
	if k, err := parsePublicKey(c.HostKey); err == nil {
		v.HostKeyType, v.HostKeyFingerprint = k.Type(), ssh.FingerprintSHA256(k)
	}
	if k, err := parsePublicKey(c.PublicKey); err == nil {
		v.KeyType, v.KeyFingerprint, v.PublicKey = k.Type(), ssh.FingerprintSHA256(k), c.PublicKey
	}
	return v
}

// Validate checks the settings and normalises them. Secrets that the
// chosen way of signing in does not use are dropped.
func (c *SFTPConfig) Validate() error {
	c.Host = strings.TrimSpace(c.Host)
	if h := strings.TrimSuffix(strings.TrimPrefix(c.Host, "["), "]"); net.ParseIP(h) != nil {
		c.Host = h
	} else if !validHostname(c.Host) {
		return usererr.New("backup_sftp_host", c.Host)
	}
	if c.Port == 0 {
		c.Port = 22
	}
	if c.Port < 1 || c.Port > 65535 {
		return usererr.New("backup_sftp_port")
	}
	c.Username = strings.TrimSpace(c.Username)
	if c.Username == "" || len(c.Username) > 128 || strings.ContainsFunc(c.Username, func(r rune) bool { return r <= ' ' || r == 0x7f }) {
		return usererr.New("backup_sftp_username")
	}
	switch c.Auth {
	case SFTPAuthPassword:
		if c.Password == "" || len(c.Password) > 1024 || strings.ContainsAny(c.Password, "\x00\r\n") {
			return usererr.New("backup_sftp_password")
		}
		c.PrivateKey, c.Passphrase, c.PublicKey = "", "", ""
	case SFTPAuthKey:
		c.PrivateKey = strings.TrimSpace(strings.ReplaceAll(c.PrivateKey, "\r\n", "\n")) + "\n"
		s, encrypted, err := parseSigner(c.PrivateKey, c.Passphrase)
		if err != nil {
			return err
		}
		if !encrypted {
			c.Passphrase = ""
		}
		c.Password = ""
		c.PublicKey = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(s.PublicKey())))
	default:
		return usererr.New("backup_sftp_auth")
	}
	f := strings.TrimSpace(c.Folder)
	if f == "~" {
		f = ""
	}
	f = strings.TrimPrefix(f, "~/") // SFTP paths are relative to the home folder already
	if len(f) > 512 || strings.ContainsFunc(f, func(r rune) bool { return r < ' ' || r == 0x7f || r == '\\' }) {
		return usererr.New("backup_sftp_folder")
	}
	for _, e := range strings.Split(f, "/") {
		if e == ".." {
			return usererr.New("backup_sftp_folder")
		}
	}
	if f != "" {
		if f = path.Clean(f); f == "." {
			f = ""
		}
	}
	c.Folder = f
	return nil
}

func validHostname(h string) bool {
	if h == "" || len(h) > 253 {
		return false
	}
	for _, label := range strings.Split(strings.TrimSuffix(h, "."), ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
				return false
			}
		}
	}
	return true
}

// parseSigner reads a private key in OpenSSH or PEM (PKCS#1, PKCS#8, SEC 1)
// format. encrypted tells whether the passphrase was needed.
func parseSigner(key, passphrase string) (s ssh.Signer, encrypted bool, err error) {
	b := []byte(key)
	s, err = ssh.ParsePrivateKey(b)
	var missing *ssh.PassphraseMissingError
	if errors.As(err, &missing) {
		if passphrase == "" {
			return nil, true, usererr.New("backup_sftp_passphrase")
		}
		s, err = ssh.ParsePrivateKeyWithPassphrase(b, []byte(passphrase))
		if errors.Is(err, x509.IncorrectPasswordError) {
			return nil, true, usererr.New("backup_sftp_wrong_passphrase")
		}
		encrypted = true
	}
	if err != nil {
		return nil, encrypted, usererr.New("backup_sftp_key")
	}
	return s, encrypted, nil
}

func parsePublicKey(s string) (ssh.PublicKey, error) {
	if s == "" {
		return nil, errors.New("no key")
	}
	k, _, _, _, err := ssh.ParseAuthorizedKey([]byte(s))
	return k, err
}

// trustAlgorithms are the host key algorithms offered when a new key is
// trusted: plain keys only (a host certificate would change when it is
// renewed), strongest first.
var trustAlgorithms = []string{
	ssh.KeyAlgoED25519, ssh.KeyAlgoECDSA256, ssh.KeyAlgoECDSA384, ssh.KeyAlgoECDSA521,
	ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256,
}

// pinnedAlgorithms makes the server present the trusted key, and not
// another of its keys, which it may prefer for any reason.
func pinnedAlgorithms(k ssh.PublicKey) []string {
	if k.Type() == ssh.KeyAlgoRSA {
		return []string{ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256}
	}
	return []string{k.Type()}
}

// HostKeyError means the SFTP server did not present the trusted host key.
type HostKeyError struct {
	Got, Want ssh.PublicKey // Got is nil when the server no longer has a key of that type
}

func (e *HostKeyError) Error() string {
	const how = "If the server was reinstalled or its keys were changed on purpose, " +
		"click Test and save under Remote storage on the Backups page to trust the new key."
	if e.Got == nil {
		return fmt.Sprintf("the SFTP server no longer offers the trusted host key %s %s. %s",
			e.Want.Type(), ssh.FingerprintSHA256(e.Want), how)
	}
	return fmt.Sprintf("the SFTP server's host key has changed: it presented %s %s, but %s %s is trusted. "+
		"This happens when the server was reinstalled, but also when someone intercepts the connection. %s",
		e.Got.Type(), ssh.FingerprintSHA256(e.Got), e.Want.Type(), ssh.FingerprintSHA256(e.Want), how)
}

// connError is a connection problem. Its message reads well in a job log;
// code and detail let "Test and save" answer with a translated message.
type connError struct {
	code, detail, msg string
}

func (e *connError) Error() string { return e.msg }

// sftpUserError turns a failed test into an error the UI translates.
func sftpUserError(err error) error {
	var ce *connError
	var ue *usererr.Error
	switch {
	case errors.As(err, &ce):
		return usererr.New(ce.code, ce.detail)
	case errors.As(err, &ue):
		return err
	}
	return usererr.New("storage_test_failed", err.Error())
}

// sftpClient is an open SFTP session.
type sftpClient struct {
	cfg     SFTPConfig
	conn    net.Conn
	ssh     *ssh.Client
	sftp    *sftp.Client
	hostKey ssh.PublicKey // the key the server presented
	stop    func() bool
	done    chan struct{}
	once    sync.Once
}

// dialSFTP connects and signs in. With trustNew, any host key is accepted
// (and kept in hostKey); otherwise it must be exactly cfg.HostKey.
func dialSFTP(ctx context.Context, cfg SFTPConfig, trustNew bool) (*sftpClient, error) {
	var auth []ssh.AuthMethod
	if cfg.Auth == SFTPAuthKey {
		s, _, err := parseSigner(cfg.PrivateKey, cfg.Passphrase)
		if err != nil {
			return nil, err
		}
		auth = append(auth, ssh.PublicKeys(s))
	} else {
		pw := cfg.Password
		auth = append(auth, ssh.Password(pw),
			// Servers that only allow keyboard-interactive (PAM) ask for the password that way.
			ssh.KeyboardInteractive(func(_, _ string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range questions {
					if !echos[i] {
						answers[i] = pw
					}
				}
				return answers, nil
			}))
	}
	var pinned ssh.PublicKey
	algos := trustAlgorithms
	if !trustNew {
		k, err := parsePublicKey(cfg.HostKey)
		if err != nil {
			return nil, errors.New("the SFTP server's host key is not saved: click Test and save under Remote storage on the Backups page")
		}
		pinned, algos = k, pinnedAlgorithms(k)
	}
	var presented ssh.PublicKey
	config := &ssh.ClientConfig{
		User:              cfg.Username,
		Auth:              auth,
		HostKeyAlgorithms: algos,
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			presented = key
			if pinned != nil && !bytes.Equal(key.Marshal(), pinned.Marshal()) {
				return &HostKeyError{Got: key, Want: pinned}
			}
			return nil
		},
		Timeout: sftpDialTimeout,
	}

	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	d := net.Dialer{Timeout: sftpDialTimeout, KeepAlive: 30 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, &connError{"backup_sftp_connect", addr + ": " + err.Error(),
			fmt.Sprintf("could not connect to the SFTP server %s: %v", addr, err)}
	}
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	fail := func(err error) (*sftpClient, error) {
		stop()
		conn.Close()
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}

	conn.SetDeadline(time.Now().Add(sftpHandshakeTimeout))
	sc, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		var hk *HostKeyError
		var ne net.Error
		switch {
		case errors.As(err, &hk):
			return fail(hk)
		case pinned != nil && strings.Contains(err.Error(), "no common algorithm for host key"):
			return fail(&HostKeyError{Want: pinned})
		case strings.Contains(err.Error(), "unable to authenticate"):
			return fail(&connError{"backup_sftp_login", cfg.Username,
				fmt.Sprintf("the SFTP server %s refused to sign in %s: check the password or key", addr, cfg.Username)})
		case errors.As(err, &ne) && ne.Timeout():
			return fail(&connError{"backup_sftp_timeout", addr,
				fmt.Sprintf("the SFTP server %s did not finish the SSH handshake within %s", addr, sftpHandshakeTimeout)})
		}
		return fail(&connError{"backup_sftp_connect", addr + ": " + err.Error(),
			fmt.Sprintf("SSH connection to %s failed: %v", addr, err)})
	}
	client := ssh.NewClient(sc, chans, reqs)
	sc2, err := sftp.NewClient(client, sftp.UseConcurrentWrites(true))
	if err != nil {
		client.Close()
		return fail(&connError{"backup_sftp_unavailable", err.Error(),
			fmt.Sprintf("%s accepted the login, but SFTP is not available: %v", addr, err)})
	}
	conn.SetDeadline(time.Time{})
	c := &sftpClient{cfg: cfg, conn: conn, ssh: client, sftp: sc2, hostKey: presented, stop: stop, done: make(chan struct{})}
	go c.keepAlive()
	return c, nil
}

// keepAlive drops the connection when the server stops answering, so a
// backup job does not hang for hours on a dead connection.
func (c *sftpClient) keepAlive() {
	t := time.NewTicker(sftpKeepAlive)
	defer t.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-t.C:
		}
		res := make(chan error, 1)
		go func() {
			_, _, err := c.ssh.SendRequest("keepalive@openssh.com", true, nil)
			res <- err
		}()
		select {
		case err := <-res:
			if err != nil {
				return
			}
		case <-time.After(2 * sftpKeepAlive):
			c.conn.Close()
			return
		case <-c.done:
			return
		}
	}
}

func (c *sftpClient) Close() error {
	c.once.Do(func() { close(c.done) })
	c.stop()
	c.sftp.Close()
	return c.ssh.Close()
}

// path is where key is stored on the server.
func (c *sftpClient) path(key string) string {
	if p := path.Join(c.cfg.Folder, key); p != "" {
		return p
	}
	return "."
}

func (c *sftpClient) Describe(key string) string {
	return fmt.Sprintf("SFTP %s@%s:%s", c.cfg.Username, net.JoinHostPort(c.cfg.Host, strconv.Itoa(c.cfg.Port)), c.path(key))
}

// mkdirAll creates dir and its missing parents, private to the account.
func (c *sftpClient) mkdirAll(dir string) error {
	if dir == "" || dir == "." || dir == "/" {
		return nil
	}
	fi, err := c.sftp.Stat(dir)
	if err == nil {
		if !fi.IsDir() {
			return fmt.Errorf("%s on the SFTP server is not a folder", dir)
		}
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("check %s on the SFTP server: %w", dir, err)
	}
	if err := c.mkdirAll(path.Dir(dir)); err != nil {
		return err
	}
	if err := c.sftp.Mkdir(dir); err != nil {
		if fi, serr := c.sftp.Stat(dir); serr == nil && fi.IsDir() {
			return nil
		}
		return fmt.Errorf("create the folder %s on the SFTP server: %w", dir, err)
	}
	c.sftp.Chmod(dir, 0o700) // best effort: some SFTP services refuse to change modes
	return nil
}

// Put streams r to a temporary file next to its final name, checks its
// size and renames it, so a broken upload never looks like a backup.
func (c *sftpClient) Put(ctx context.Context, key string, r io.Reader, size int64, _ string) error {
	p := c.path(key)
	dir := path.Dir(p)
	if err := c.mkdirAll(dir); err != nil {
		return err
	}
	tmp := path.Join(dir, "."+path.Base(p)+"."+randomHex(6)+".part")
	f, err := c.sftp.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return fmt.Errorf("create %s on the SFTP server: %w", tmp, err)
	}
	ok := false
	defer func() {
		if !ok {
			c.sftp.Remove(tmp)
		}
	}()
	f.Chmod(0o600) // best effort, before any data is written
	// ReadFrom learns the size from r and keeps several writes in flight,
	// which matters on a distant server.
	n, err := f.ReadFrom(r)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("upload to the SFTP server: %w", err)
	}
	if n != size {
		return fmt.Errorf("read %d bytes instead of %d while uploading", n, size)
	}
	fi, err := c.sftp.Stat(tmp)
	if err != nil {
		return fmt.Errorf("check the uploaded file: %w", err)
	}
	if fi.Size() != size {
		return fmt.Errorf("the uploaded file has %d bytes on the SFTP server instead of %d", fi.Size(), size)
	}
	if err := c.rename(tmp, p); err != nil {
		return fmt.Errorf("rename the uploaded file to %s: %w", p, err)
	}
	ok = true
	return nil
}

// rename replaces to with from. The basic SFTP rename refuses to replace a
// file, so the OpenSSH extension is used when the server has it.
func (c *sftpClient) rename(from, to string) error {
	if _, ok := c.sftp.HasExtension("posix-rename@openssh.com"); ok {
		return c.sftp.PosixRename(from, to)
	}
	if err := c.sftp.Remove(to); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return c.sftp.Rename(from, to)
}

func (c *sftpClient) Get(ctx context.Context, key string, w io.Writer) error {
	f, err := c.sftp.Open(c.path(key))
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s is missing on the SFTP server", c.path(key))
	}
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(w, f); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("download from the SFTP server: %w", err)
	}
	return nil
}

func (c *sftpClient) Delete(ctx context.Context, key string) error {
	err := c.sftp.Remove(c.path(key))
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("delete %s on the SFTP server: %w", c.path(key), err)
}

// List returns the files (not folders) under prefix, by name.
func (c *sftpClient) List(ctx context.Context, prefix string) ([]RemoteFile, error) {
	entries, err := c.sftp.ReadDirContext(ctx, c.path(prefix))
	if errors.Is(err, fs.ErrNotExist) {
		return []RemoteFile{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list %s on the SFTP server: %w", c.path(prefix), err)
	}
	out := []RemoteFile{}
	for _, e := range entries {
		if e.Mode().IsRegular() {
			out = append(out, RemoteFile{Name: e.Name(), Size: e.Size(), ModTime: e.ModTime()})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ListTree lists the files under prefix, at most two folders deep (the
// layout is kind/target/file). Names are relative to the storage's folder.
func (c *sftpClient) ListTree(ctx context.Context, prefix string) ([]RemoteFile, error) {
	base := c.path(prefix)
	out := []RemoteFile{}
	w := c.sftp.Walk(base)
	for w.Step() {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if err := w.Err(); err != nil {
			if w.Path() == base && errors.Is(err, fs.ErrNotExist) {
				return out, nil
			}
			return nil, fmt.Errorf("list %s on the SFTP server: %w", w.Path(), err)
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(w.Path(), base), "/")
		fi := w.Stat()
		if fi.IsDir() {
			if rel != "" && strings.Count(rel, "/") >= 1 {
				w.SkipDir()
			}
			continue
		}
		if fi.Mode().IsRegular() && len(out) < 100000 {
			out = append(out, RemoteFile{Name: strings.Trim(prefix, "/") + "/" + rel, Size: fi.Size(), ModTime: fi.ModTime()})
		}
	}
	return out, nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ---- settings ----

func (m *Manager) sftpWithSecret(ctx context.Context) (*SFTPConfig, error) {
	v, err := m.store.Setting(ctx, sftpSettingKey)
	if err != nil || v == "" {
		return nil, err
	}
	var c SFTPConfig
	if err := json.Unmarshal([]byte(v), &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// SFTPStorage returns the SFTP settings without their secrets, or nil when
// SFTP storage is not set up.
func (m *Manager) SFTPStorage(ctx context.Context) (*SFTPView, error) {
	c, err := m.sftpWithSecret(ctx)
	if c == nil || err != nil {
		return nil, err
	}
	return c.view(), nil
}

// SFTPSaved is the result of a successful "Test and save".
type SFTPSaved struct {
	SFTP *SFTPView `json:"sftp"`
	// PreviousFingerprint is set when the server presented another host key
	// than the one trusted before.
	PreviousFingerprint string `json:"previous_fingerprint,omitempty"`
}

// SaveSFTP validates the settings, connects, trusts the host key that the
// server presents now, tests writing, listing, reading and deleting a file
// and stores the settings. An empty password or private key keeps the
// stored one when the way of signing in is unchanged, an empty encryption
// passphrase the stored one.
func (m *Manager) SaveSFTP(ctx context.Context, c SFTPConfig) (*SFTPSaved, error) {
	c.HostKey, c.PublicKey = "", ""
	old, err := m.sftpWithSecret(ctx)
	if err != nil {
		return nil, err
	}
	if old != nil && old.Auth == c.Auth {
		switch c.Auth {
		case SFTPAuthPassword:
			if c.Password == "" {
				c.Password = old.Password
			}
		case SFTPAuthKey:
			if strings.TrimSpace(c.PrivateKey) == "" {
				c.PrivateKey = old.PrivateKey
				if c.Passphrase == "" {
					c.Passphrase = old.Passphrase
				}
			}
		}
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	var oldEnc Encryption
	if old != nil {
		oldEnc = old.Encryption
	}
	if err := mergeEncryption(&c.Encryption, oldEnc); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	cl, err := dialSFTP(ctx, c, true)
	if err != nil {
		return nil, sftpUserError(err)
	}
	defer cl.Close()
	if err := testStorage(ctx, cl, c.Encryption); err != nil {
		return nil, sftpUserError(err)
	}
	c.HostKey = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(cl.hostKey)))
	res := &SFTPSaved{SFTP: c.view()}
	if old != nil && old.HostKey != "" && old.HostKey != c.HostKey {
		if k, err := parsePublicKey(old.HostKey); err == nil {
			res.PreviousFingerprint = ssh.FingerprintSHA256(k)
		}
	}
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	if err := m.store.SetSetting(ctx, sftpSettingKey, string(b)); err != nil {
		return nil, err
	}
	return res, nil
}

// RemoveSFTP forgets the SFTP settings. Backups already uploaded stay on
// the SFTP server.
func (m *Manager) RemoveSFTP(ctx context.Context) error {
	return m.store.SetSetting(ctx, sftpSettingKey, "")
}
