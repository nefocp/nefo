package security

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"unicode"

	"golang.org/x/crypto/ssh"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/usererr"
)

// SSHKey is a public key in an account's authorized_keys file.
type SSHKey struct {
	// Type is ed25519, rsa, ecdsa, ed25519-sk, ecdsa-sk or dsa (sshd's own
	// name for another type), and Bits its size.
	Type        string `json:"type"`
	Bits        int    `json:"bits"`
	Fingerprint string `json:"fingerprint"` // SHA256:…
	Comment     string `json:"comment"`
	// Options are the key's options, such as from="203.0.113.7" or no-pty.
	Options []string `json:"options"`
	// Weak marks a DSA key, or an RSA key under 2048 bits.
	Weak bool   `json:"weak"`
	File string `json:"file"`
}

// KeyAccount is an account that can have SSH keys, with its keys.
type KeyAccount struct {
	User string   `json:"user"`
	Keys []SSHKey `json:"keys"`
	// File is where an added key goes: the first AuthorizedKeysFile.
	File string `json:"file"`
	// Allowed is false when AllowUsers, DenyUsers, AllowGroups or
	// DenyGroups keep the account out of SSH, keys or not.
	Allowed bool `json:"allowed"`
	// Skipped are key files the panel neither reads nor changes: symbolic
	// links, and what is not a regular file or is larger than 1 MB.
	Skipped []string `json:"skipped"`
	// Invalid counts the lines of its files that are not keys the panel
	// can read (comments aside).
	Invalid int `json:"invalid"`
}

// SSHKeys lists the SSH public keys of root and of the people's accounts.
type SSHKeys struct {
	Accounts []KeyAccount `json:"accounts"`
	// KeyCommand is set when sshd also asks a program for keys
	// (AuthorizedKeysCommand); those keys are not listed.
	KeyCommand bool `json:"key_command"`
}

const maxKeysFile = 1 << 20

// keyOptions are the options OpenSSH accepts before a key.
var keyOptions = []string{"agent-forwarding", "cert-authority", "command", "environment", "expiry-time", "from",
	"no-agent-forwarding", "no-port-forwarding", "no-pty", "no-touch-required", "no-user-rc", "no-x11-forwarding",
	"permitlisten", "permitopen", "port-forwarding", "principals", "pty", "restrict", "tunnel", "user-rc",
	"verify-required", "x11-forwarding"}

// SSHKeys lists the keys of the accounts the SSH guard counts (root, and
// the people's accounts with a login shell), from their AuthorizedKeysFile
// files.
func (m *Manager) SSHKeys(ctx context.Context) SSHKeys {
	vals := m.sshdValues(ctx)
	files := authorizedKeysFiles(vals)
	out := SSHKeys{Accounts: []KeyAccount{}, KeyCommand: keyCommand(vals)}
	accounts := m.accounts()
	groups := m.groupsOf(accounts)
	for _, a := range accounts {
		if !keyAccount(a) {
			continue
		}
		ka := KeyAccount{User: a.name, Keys: []SSHKey{}, Skipped: []string{}, Allowed: sshAllowed(vals, a.name, groups[a.name])}
		paths := m.keyPaths(a, files)
		if len(paths) > 0 {
			ka.File = paths[0]
		}
		for _, p := range paths {
			text, err := m.readKeyFile(a, p)
			if err != nil {
				if !errors.Is(err, fs.ErrNotExist) {
					ka.Skipped = append(ka.Skipped, p)
				}
				continue
			}
			keys, invalid := parseKeyFile(text)
			for _, k := range keys {
				k.File = p
				ka.Keys = append(ka.Keys, k)
			}
			ka.Invalid += invalid
		}
		out.Accounts = append(out.Accounts, ka)
	}
	return out
}

// AddSSHKey adds a public key, one authorized_keys line, to an account's
// first AuthorizedKeysFile (~/.ssh/authorized_keys by default), creating
// the file and its folder when missing. It refuses private keys, DSA keys,
// RSA keys under 2048 bits and a key the account has already.
func (m *Manager) AddSSHKey(ctx context.Context, user, text string) (SSHKey, error) {
	k, line, err := newKeyLine(text)
	if err != nil {
		return SSHKey{}, err
	}
	a, err := m.keyAccountNamed(user)
	if err != nil {
		return SSHKey{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	paths := m.keyPaths(a, authorizedKeysFiles(m.sshdValues(ctx)))
	if len(paths) == 0 {
		return SSHKey{}, usererr.New("ssh_key_no_file", a.name) // AuthorizedKeysFile none
	}
	var current string
	for i, p := range paths {
		text, err := m.readKeyFile(a, p)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			if i == 0 {
				return SSHKey{}, err // the file the key goes to
			}
			continue
		}
		if i == 0 {
			current = text
		}
		keys, _ := parseKeyFile(text)
		for _, e := range keys {
			if e.Fingerprint == k.Fingerprint {
				return SSHKey{}, usererr.New("ssh_key_exists", a.name)
			}
		}
	}
	if current != "" && !strings.HasSuffix(current, "\n") {
		current += "\n"
	}
	if err := m.writeKeyFile(a, paths[0], current+line+"\n"); err != nil {
		return SSHKey{}, err
	}
	k.File = paths[0]
	return k, nil
}

// RemoveSSHKey removes a key, by its SHA256 fingerprint, from an account's
// authorized_keys files. Like SetSSH, it refuses when nobody could sign in
// over SSH any more with the current settings, while someone can now.
func (m *Manager) RemoveSSHKey(ctx context.Context, user, fingerprint string) (SSHKey, error) {
	a, err := m.keyAccountNamed(user)
	if err != nil {
		return SSHKey{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	vals := m.sshdValues(ctx)
	changed := map[string]string{}
	var removed *SSHKey
	for _, p := range m.keyPaths(a, authorizedKeysFiles(vals)) {
		text, err := m.readKeyFile(a, p)
		if err != nil {
			continue
		}
		var keep strings.Builder
		n := 0
		for _, line := range strings.SplitAfter(text, "\n") {
			if k, _, ok := parseKeyLine(strings.TrimSpace(line)); ok && k.Fingerprint == fingerprint {
				if removed == nil {
					k.File = p
					removed = &k
				}
				n++
				continue
			}
			keep.WriteString(line)
		}
		if n > 0 {
			changed[p] = keep.String()
		}
	}
	if removed == nil {
		return SSHKey{}, usererr.New("ssh_key_not_found")
	}
	cur := m.sshSettings(ctx, vals)
	if len(cur.Logins) > 0 && len(m.loginsWith(vals, cur, changed)) == 0 {
		return SSHKey{}, usererr.New("ssh_key_last_way_in")
	}
	for p, text := range changed {
		if err := m.writeKeyFile(a, p, text); err != nil {
			return SSHKey{}, err
		}
	}
	return *removed, nil
}

func (m *Manager) keyAccountNamed(user string) (account, error) {
	for _, a := range m.accounts() {
		if a.name == user && keyAccount(a) {
			return a, nil
		}
	}
	if len(user) > 64 {
		user = user[:64]
	}
	return account{}, usererr.New("ssh_key_bad_user", user)
}

// newKeyLine checks a public key pasted to be added, and returns it with
// its authorized_keys line.
func newKeyLine(text string) (SSHKey, string, error) {
	text = strings.TrimSpace(strings.TrimPrefix(text, string(rune(0xfeff)))) // a byte order mark
	if strings.Contains(text, "PRIVATE KEY-----") || strings.HasPrefix(text, "PuTTY-User-Key-File") {
		return SSHKey{}, "", usererr.New("ssh_key_private")
	}
	if text == "" || strings.ContainsAny(text, "\r\n") {
		return SSHKey{}, "", usererr.New("ssh_key_one_line")
	}
	short := text
	if len(short) > 40 {
		short = short[:40] + "…"
	}
	if len(text) > 16<<10 || strings.ContainsFunc(text, func(r rune) bool { return r != '\t' && unicode.IsControl(r) }) {
		return SSHKey{}, "", usererr.New("invalid_ssh_key", short)
	}
	k, pub, ok := parseKeyLine(text)
	if !ok {
		if f := strings.Fields(text); slices.Contains(f, "ssh-dss") {
			return SSHKey{}, "", usererr.New("ssh_key_dsa")
		}
		return SSHKey{}, "", usererr.New("invalid_ssh_key", short)
	}
	if _, cert := pub.(*ssh.Certificate); cert {
		return SSHKey{}, "", usererr.New("invalid_ssh_key", short)
	}
	for _, o := range k.Options {
		name, _, _ := strings.Cut(o, "=")
		if !slices.Contains(keyOptions, strings.ToLower(name)) {
			return SSHKey{}, "", usererr.New("invalid_ssh_key", o)
		}
	}
	switch {
	case k.Type == "dsa":
		return SSHKey{}, "", usererr.New("ssh_key_dsa")
	case k.Type == "rsa" && k.Bits < 2048:
		return SSHKey{}, "", usererr.New("ssh_key_rsa_short", strconv.Itoa(k.Bits))
	}
	line := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub)))
	if len(k.Options) > 0 {
		line = strings.Join(k.Options, ",") + " " + line
	}
	if k.Comment != "" {
		line += " " + k.Comment
	}
	return k, line, nil
}

// parseKeyFile reads the keys in authorized_keys text, and counts the
// other lines that are not comments.
func parseKeyFile(text string) ([]SSHKey, int) {
	keys, invalid := []SSHKey{}, 0
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, _, ok := parseKeyLine(line); ok {
			keys = append(keys, k)
		} else {
			invalid++
		}
	}
	return keys, invalid
}

// usableKeys reports whether authorized_keys text holds a key that sshd
// accepts: one that parses and is not weak (OpenSSH refuses DSA keys).
func usableKeys(text string) bool {
	keys, _ := parseKeyFile(text)
	return slices.ContainsFunc(keys, func(k SSHKey) bool { return !k.Weak })
}

// parseKeyLine reads one authorized_keys line.
func parseKeyLine(line string) (SSHKey, ssh.PublicKey, bool) {
	if line == "" || strings.HasPrefix(line, "#") {
		return SSHKey{}, nil, false
	}
	pub, comment, options, rest, err := ssh.ParseAuthorizedKey([]byte(line))
	if err != nil || strings.TrimSpace(string(rest)) != "" {
		return SSHKey{}, nil, false
	}
	t, bits := keyKind(pub)
	if options == nil {
		options = []string{}
	}
	comment = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, comment))
	return SSHKey{Type: t, Bits: bits, Fingerprint: ssh.FingerprintSHA256(pub), Comment: comment, Options: options,
		Weak: t == "dsa" || (t == "rsa" && bits < 2048)}, pub, true
}

// keyKind names a key's type the short way, with its size in bits.
func keyKind(pub ssh.PublicKey) (string, int) {
	switch pub.Type() {
	case ssh.KeyAlgoED25519:
		return "ed25519", 256
	case ssh.KeyAlgoSKED25519:
		return "ed25519-sk", 256
	case ssh.KeyAlgoSKECDSA256:
		return "ecdsa-sk", 256
	case ssh.KeyAlgoDSA:
		return "dsa", 1024
	}
	if ck, ok := pub.(ssh.CryptoPublicKey); ok {
		switch k := ck.CryptoPublicKey().(type) {
		case *rsa.PublicKey:
			return "rsa", k.N.BitLen()
		case *ecdsa.PublicKey:
			return "ecdsa", k.Curve.Params().BitSize
		}
	}
	return pub.Type(), 0
}

// keyFile is an authorized_keys file, reached without following symbolic
// links. In the account's home folder, neither the file nor a folder on the
// way to it may be a link: the account could point it at any file, which
// the panel (as root) would then read or replace. A file outside the home
// (such as AuthorizedKeysFile /etc/ssh/keys/%u) is in a folder that root
// controls, and only the file itself is checked.
type keyFile struct {
	path      string // the full path
	base, rel string // the folder opened as an os.Root, and the file in it
}

func keyFileOf(a account, path string) keyFile {
	path = filepath.Clean(path)
	if home := filepath.Clean(a.home); a.home != "" && filepath.Dir(home) != home {
		if rel, err := filepath.Rel(home, path); err == nil && filepath.IsLocal(rel) {
			return keyFile{path: path, base: home, rel: rel}
		}
	}
	return keyFile{path: path, base: filepath.Dir(path), rel: filepath.Base(path)}
}

func unsafeKeys(path string) error { return usererr.New("ssh_keys_unsafe", path) }

// openKeyRoot opens the folder k is in, as an os.Root that can't be left,
// after checking that no folder on the way from k.base is a symbolic link.
// With create, missing folders are made, 0700 and owned by uid and gid.
func (m *Manager) openKeyRoot(k keyFile, create bool, uid, gid int) (*os.Root, error) {
	r, err := os.OpenRoot(k.base)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(k.rel)
	if dir == "." {
		return r, nil
	}
	parts := strings.Split(filepath.ToSlash(dir), "/")
	for i := range parts {
		p := filepath.Join(parts[:i+1]...)
		fi, err := r.Lstat(p)
		switch {
		case errors.Is(err, fs.ErrNotExist) && create:
			if err = r.Mkdir(p, 0o700); err == nil {
				err = m.chown(r, p, uid, gid)
			}
			if err != nil {
				r.Close()
				return nil, err
			}
		case err != nil:
			r.Close()
			return nil, err
		case !fi.IsDir(): // a symbolic link, or a file
			r.Close()
			return nil, unsafeKeys(filepath.Join(k.base, p))
		}
	}
	return r, nil
}

// readKeyFile reads one of an account's authorized_keys files (see
// keyFile). A missing file or folder gives an error that is
// fs.ErrNotExist.
func (m *Manager) readKeyFile(a account, path string) (string, error) {
	k := keyFileOf(a, path)
	r, err := m.openKeyRoot(k, false, 0, 0)
	if err != nil {
		return "", err
	}
	defer r.Close()
	fi, err := r.Lstat(k.rel)
	if err != nil {
		return "", err
	}
	if !fi.Mode().IsRegular() || fi.Size() > maxKeysFile {
		return "", unsafeKeys(k.path)
	}
	flag := os.O_RDONLY
	if runtime.GOOS != "windows" {
		flag |= syscall.O_NONBLOCK // a FIFO put there meanwhile must not hang the panel
	}
	f, err := r.OpenFile(k.rel, flag, 0)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if st, err := f.Stat(); err != nil || !st.Mode().IsRegular() {
		return "", unsafeKeys(k.path)
	}
	b, err := io.ReadAll(io.LimitReader(f, maxKeysFile+1))
	if err != nil {
		return "", err
	}
	if len(b) > maxKeysFile {
		return "", unsafeKeys(k.path)
	}
	return string(b), nil
}

// writeKeyFile replaces one of an account's authorized_keys files with
// text, atomically: it writes a new file next to it and renames it over
// the old one, which replaces a link rather than following it. A new file
// and missing folders belong to the account (0600 and 0700); an existing
// file keeps its owner, and its mode without write access for others.
func (m *Manager) writeKeyFile(a account, path, text string) error {
	k := keyFileOf(a, path)
	r, err := m.openKeyRoot(k, true, a.uid, a.gid)
	if errors.Is(err, fs.ErrNotExist) {
		return usererr.New("ssh_keys_no_folder", k.base)
	}
	if err != nil {
		return err
	}
	defer r.Close()
	uid, gid, perm := a.uid, a.gid, fs.FileMode(0o600)
	switch fi, err := r.Lstat(k.rel); {
	case err == nil:
		if !fi.Mode().IsRegular() {
			return unsafeKeys(k.path)
		}
		perm = fi.Mode().Perm() &^ 0o022
		if u, g := fsutil.Owner(fi); u >= 0 {
			uid, gid = u, g
		}
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}
	tmp := filepath.Join(filepath.Dir(k.rel), ".nefo-keys-"+rand.Text())
	f, err := r.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer r.Remove(tmp) // nothing left to remove once renamed
	_, err = f.WriteString(text)
	if err == nil {
		err = f.Chmod(perm)
	}
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = m.chown(r, tmp, uid, gid)
	}
	if err == nil {
		err = r.Rename(tmp, k.rel)
	}
	return err
}
