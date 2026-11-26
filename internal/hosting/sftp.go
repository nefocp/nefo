package hosting

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/crypto/ssh"

	"github.com/nefocp/nefo/internal/secure"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
	"github.com/nefocp/nefo/internal/version"
)

// SFTP lets a site's own Linux account sign in with SFTP, so uploaded files
// get the right owner. The account is locked in a chroot that contains only
// the site's folder, bind-mounted at its real path (/var/www/<domain>), so
// paths are the same as in the panel and in PHP.
//
// Every website account is in siteGroup, which sshd denies outright (a
// hacked site could otherwise add an SSH key to its home folder and use SSH
// port forwarding). Turning SFTP on moves the account to sftpGroup instead,
// where sshd forces internal-sftp inside the chroot. During a change the
// account may be in both groups for a moment: it is then denied.
const (
	siteGroup     = "panel-site"
	sftpGroup     = "panel-sftp"
	sftpRoot      = "/var/lib/panel-sftp"
	sftpKeysDir   = "/etc/ssh/panel-sftp-keys"
	sshdBinary    = "/usr/sbin/sshd"
	sshdDropInDir = "/etc/ssh/sshd_config.d"
	sftpCommand   = "internal-sftp -u 0027"
	maxSFTPKeys   = 50
)

func sshdDropIn() string                 { return filepath.Join(sshdDropInDir, "99-panel-sftp.conf") }
func sftpChroot(sysUser string) string   { return filepath.Join(sftpRoot, sysUser) }
func sftpKeysFile(sysUser string) string { return filepath.Join(sftpKeysDir, sysUser) }

// sftpMountPoint is where the site folder appears inside the chroot.
func (m *Manager) sftpMountPoint(site *store.Site) string {
	return filepath.Join(sftpChroot(site.SysUser), m.paths.siteRoot(site.Domain))
}

// sshdAvailable reports whether OpenSSH's server is installed with a
// drop-in folder the panel can use.
func sshdAvailable() bool {
	st, err := os.Stat(sshdDropInDir)
	return fileExists(sshdBinary) && err == nil && st.IsDir()
}

func sshdDropInBytes() []byte {
	return []byte(fmt.Sprintf(`# Managed by %[1]s. Do not edit: the panel rewrites this file.
# Website accounts may not sign in over SSH. Those with SFTP turned on in
# the panel are in %[2]s instead and are locked in their site's folder.
DenyGroups %[3]s

Match Group %[2]s
    ChrootDirectory %[4]s/%%u
    ForceCommand %[5]s
    AuthorizedKeysFile %[6]s/%%u
    PasswordAuthentication yes
    PubkeyAuthentication yes
    KbdInteractiveAuthentication no
    DisableForwarding yes
    PermitTunnel no
    PermitTTY no
    PermitUserRC no
`, version.AppName, sftpGroup, siteGroup, sftpRoot, sftpCommand, sftpKeysDir))
}

// sshdEffective returns sshd's effective settings, for a connection by
// user if one is given ("sshd -T -C").
func (m *Manager) sshdEffective(ctx context.Context, sysUser string) (map[string]string, error) {
	args := []string{"-T"}
	if sysUser != "" {
		args = append(args, "-C", "user="+sysUser+",host=localhost,addr=127.0.0.1")
	}
	out, err := m.run.Output(ctx, "sshd", args...)
	if err != nil {
		return nil, err
	}
	cfg := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), " "); ok {
			if _, seen := cfg[k]; !seen {
				cfg[k] = v
			}
		}
	}
	return cfg, nil
}

// ensureSSHD creates the two groups, puts every website account in the
// right one and installs the sshd drop-in. Without OpenSSH it does nothing.
func (m *Manager) ensureSSHD(ctx context.Context) error {
	if !sshdAvailable() {
		return nil
	}
	for _, g := range []string{siteGroup, sftpGroup} {
		if _, err := user.LookupGroup(g); err != nil {
			if _, err := m.run.Output(ctx, "groupadd", "--system", g); err != nil {
				return err
			}
		}
	}
	sites, err := m.store.ListSites(ctx)
	if err != nil {
		return err
	}
	for _, s := range sites {
		if validate.SystemUsername(s.SysUser) != nil {
			continue
		}
		if s.SFTP {
			continue // SetSFTP manages these
		}
		if err := m.addToGroup(ctx, s.SysUser, siteGroup); err != nil {
			return err
		}
	}

	conf := sshdDropInBytes()
	old, oldErr := os.ReadFile(sshdDropIn())
	if oldErr == nil && string(old) == string(conf) {
		return nil
	}
	if err := writeFile(sshdDropIn(), conf, 0o644); err != nil {
		return err
	}
	restore := func() {
		if oldErr == nil {
			writeFile(sshdDropIn(), old, 0o644)
		} else {
			os.Remove(sshdDropIn())
		}
	}
	if out, err := m.run.Output(ctx, "sshd", "-t"); err != nil {
		restore()
		return fmt.Errorf("sshd rejected the SFTP settings: %s", strings.TrimSpace(out))
	}
	if _, err := m.run.Output(ctx, "systemctl", "try-reload-or-restart", "ssh"); err != nil {
		restore()
		m.run.Output(context.WithoutCancel(ctx), "systemctl", "try-reload-or-restart", "ssh")
		return err
	}
	if cfg, err := m.sshdEffective(ctx, ""); err != nil || !strings.Contains(" "+cfg["denygroups"]+" ", " "+siteGroup+" ") {
		return errors.New("sshd does not read " + sshdDropInDir + ": SFTP accounts are not available")
	}
	return nil
}

// inGroup reports whether a Linux account is a member of a group.
func inGroup(name, group string) bool {
	u, err := user.Lookup(name)
	if err != nil {
		return false
	}
	g, err := user.LookupGroup(group)
	if err != nil {
		return false
	}
	ids, err := u.GroupIds()
	return err == nil && slices.Contains(ids, g.Gid)
}

func (m *Manager) addToGroup(ctx context.Context, name, group string) error {
	if inGroup(name, group) {
		return nil
	}
	_, err := m.run.Output(ctx, "usermod", "--append", "--groups", group, name)
	return err
}

func (m *Manager) removeFromGroup(ctx context.Context, name, group string) error {
	if !inGroup(name, group) {
		return nil
	}
	_, err := m.run.Output(ctx, "gpasswd", "--delete", name, group)
	return err
}

// passwordSet reports whether a Linux account has a usable password.
func passwordSet(name string) bool {
	b, err := os.ReadFile("/etc/shadow")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.SplitN(line, ":", 3)
		if len(f) >= 2 && f[0] == name {
			h := f[1]
			return h != "" && !strings.HasPrefix(h, "!") && !strings.HasPrefix(h, "*")
		}
	}
	return false
}

var errWeakPassword = errors.New("rejected by the password policy")

// weakPasswordError is the user error shown for a password the policy
// rejected; it also matches errWeakPassword.
type weakPasswordError struct{ ue error }

func (e weakPasswordError) Error() string   { return e.ue.Error() }
func (e weakPasswordError) Unwrap() []error { return []error{e.ue, errWeakPassword} }

// setPassword sets a Linux account's password with chpasswd (through PAM,
// so the server's password policy applies). The password goes on stdin,
// never on a command line.
func (m *Manager) setPassword(ctx context.Context, name, password string) error {
	out, err := m.run.Input(ctx, []byte(name+":"+password+"\n"), "chpasswd")
	if err == nil {
		return nil
	}
	for _, line := range strings.Split(out, "\n") {
		if reason, ok := strings.CutPrefix(strings.TrimSpace(line), "BAD PASSWORD:"); ok {
			return weakPasswordError{usererr.New("weak_sftp_password", strings.TrimSpace(reason))}
		}
	}
	return errors.New("could not set the SFTP password")
}

// SSHKey is a public key that may sign in to an SFTP account.
type SSHKey struct {
	Type        string `json:"type"`
	Comment     string `json:"comment"`
	Fingerprint string `json:"fingerprint"`
}

// parseKeys checks authorized_keys text and returns it normalised: one
// key per line, options (command=, from=…) removed.
func parseKeys(text string) ([]string, []SSHKey, error) {
	lines, keys := []string{}, []SSHKey{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		short := line
		if len(short) > 40 {
			short = short[:40] + "…"
		}
		if len(line) > 16<<10 {
			return nil, nil, usererr.New("invalid_ssh_key", short)
		}
		pub, comment, _, rest, err := ssh.ParseAuthorizedKey([]byte(line))
		if err != nil || strings.TrimSpace(string(rest)) != "" {
			return nil, nil, usererr.New("invalid_ssh_key", short)
		}
		comment = strings.Map(func(r rune) rune {
			if unicode.IsPrint(r) {
				return r
			}
			return -1
		}, comment)
		if len(comment) > 100 {
			comment = comment[:100]
		}
		l := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub)))
		if comment != "" {
			l += " " + comment
		}
		lines = append(lines, l)
		keys = append(keys, SSHKey{Type: pub.Type(), Comment: comment, Fingerprint: ssh.FingerprintSHA256(pub)})
	}
	if len(lines) > maxSFTPKeys {
		return nil, nil, usererr.New("too_many_ssh_keys")
	}
	return lines, keys, nil
}

func validSFTPPassword(pw string) bool {
	if len(pw) < 12 || len(pw) > 128 {
		return false
	}
	for _, r := range pw {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// SFTPInfo describes a site's SFTP account.
type SFTPInfo struct {
	Available   bool     `json:"available"` // OpenSSH is installed
	Enabled     bool     `json:"enabled"`
	Username    string   `json:"username"`
	Path        string   `json:"path"`
	HasPassword bool     `json:"has_password"`
	Keys        []SSHKey `json:"keys"`
	KeysText    string   `json:"keys_text"`
	// Password is set only in the answer to a change that generated it.
	Password string `json:"password,omitempty"`
}

func (m *Manager) sftpInfo(site *store.Site) *SFTPInfo {
	info := &SFTPInfo{
		Available: sshdAvailable(),
		Enabled:   site.SFTP,
		Username:  site.SysUser,
		Path:      m.paths.siteRoot(site.Domain),
		Keys:      []SSHKey{},
	}
	if site.SFTP {
		info.HasPassword = passwordSet(site.SysUser)
		if b, err := os.ReadFile(sftpKeysFile(site.SysUser)); err == nil {
			lines, keys, err := parseKeys(string(b))
			if err == nil {
				info.Keys, info.KeysText = keys, strings.Join(lines, "\n")
			}
		}
	}
	return info
}

// SFTP returns a site's SFTP account.
func (m *Manager) SFTP(ctx context.Context, id int64) (*SFTPInfo, error) {
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return m.sftpInfo(site), nil
}

// SFTPSettings is a change to a site's SFTP account.
type SFTPSettings struct {
	Enabled          bool
	Password         string  // a new password; "" keeps the current one
	GeneratePassword bool    // set a new random password, returned once
	RemovePassword   bool    // sign in with keys only
	Keys             *string // authorized_keys text; nil keeps the current keys
}

// SetSFTP turns a site's SFTP account on or off and sets how it signs in.
func (m *Manager) SetSFTP(ctx context.Context, id int64, req SFTPSettings) (*SFTPInfo, error) {
	if !m.Supported() {
		return nil, ErrUnsupported
	}
	if !sshdAvailable() {
		return nil, usererr.New("sftp_no_ssh")
	}
	if err := m.tryLock(); err != nil {
		return nil, err
	}
	defer m.ops.Unlock()
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if validate.Domain(site.Domain) != nil || validate.SystemUsername(site.SysUser) != nil {
		return nil, fmt.Errorf("invalid site %q", site.Domain)
	}

	if !req.Enabled {
		if err := m.disableSFTP(ctx, site); err != nil {
			return nil, err
		}
		site.SFTP = false
		if err := m.store.UpdateSite(ctx, site); err != nil {
			return nil, err
		}
		return m.sftpInfo(site), nil
	}

	// Check the whole request before changing anything.
	var keyLines []string
	if req.Keys != nil {
		if keyLines, _, err = parseKeys(*req.Keys); err != nil {
			return nil, err
		}
	} else if site.SFTP {
		if b, err := os.ReadFile(sftpKeysFile(site.SysUser)); err == nil {
			keyLines, _, _ = parseKeys(string(b))
		}
	}
	if req.Password != "" && !validSFTPPassword(req.Password) {
		return nil, usererr.New("invalid_sftp_password")
	}
	newPassword := req.Password != "" || req.GeneratePassword
	hasPassword := newPassword || (site.SFTP && !req.RemovePassword && passwordSet(site.SysUser))
	if !hasPassword && len(keyLines) == 0 {
		return nil, usererr.New("sftp_needs_login")
	}

	wasEnabled := site.SFTP
	bg := context.WithoutCancel(ctx)
	fail := func(err error) (*SFTPInfo, error) {
		if !wasEnabled {
			if derr := m.disableSFTP(bg, site); derr != nil {
				m.log.Warn("undo SFTP", "domain", site.Domain, "err", derr)
			}
		}
		return nil, err
	}
	if err := m.ensureSSHD(ctx); err != nil {
		return nil, err
	}
	if !wasEnabled {
		if err := m.enableSFTP(ctx, site); err != nil {
			return fail(err)
		}
	}
	if req.Keys != nil {
		if len(keyLines) == 0 {
			if err := removeIfExists(sftpKeysFile(site.SysUser)); err != nil {
				return fail(err)
			}
		} else {
			if err := mkdir(sftpKeysDir, 0o755); err != nil {
				return fail(err)
			}
			if err := writeFile(sftpKeysFile(site.SysUser), []byte(strings.Join(keyLines, "\n")+"\n"), 0o644); err != nil {
				return fail(err)
			}
		}
	}
	generated := ""
	switch {
	case req.GeneratePassword:
		// The server's password policy (pam_pwquality) may reject even a
		// random password, as "too systematic": then try another one.
		for i := 0; ; i++ {
			generated = secure.String(20, "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789")
			err := m.setPassword(ctx, site.SysUser, generated)
			if err == nil {
				break
			}
			if !errors.Is(err, errWeakPassword) || i == 9 {
				return fail(err)
			}
		}
	case req.Password != "":
		if err := m.setPassword(ctx, site.SysUser, req.Password); err != nil {
			return fail(err)
		}
	case req.RemovePassword || !wasEnabled:
		if _, err := m.run.Output(ctx, "usermod", "--password", "*", site.SysUser); err != nil {
			return fail(err)
		}
	}
	site.SFTP = true
	if err := m.store.UpdateSite(ctx, site); err != nil {
		return fail(err)
	}
	info := m.sftpInfo(site)
	info.Password = generated
	return info, nil
}

// enableSFTP builds the chroot, mounts the site folder in it and moves the
// account to sftpGroup. The account has no password or key yet.
func (m *Manager) enableSFTP(ctx context.Context, site *store.Site) error {
	// The chroot and every folder above the mount point must belong to root
	// and be writable by root only, or sshd refuses the login.
	dir := sftpRoot
	if err := mkdir(dir, 0o755); err != nil {
		return err
	}
	where := m.sftpMountPoint(site)
	rel, err := filepath.Rel(sftpRoot, where)
	if err != nil {
		return err
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		dir = filepath.Join(dir, part)
		if st, err := os.Lstat(dir); err == nil && !st.IsDir() {
			return fmt.Errorf("%s is not a folder", dir)
		}
		if err := mkdir(dir, 0o755); err != nil {
			return err
		}
		if err := os.Chown(dir, 0, 0); err != nil {
			return err
		}
	}

	unit, err := m.mountUnit(ctx, where)
	if err != nil {
		return err
	}
	body := fmt.Sprintf(`# Managed by %s: the site folder inside its SFTP chroot.
[Unit]
Description=SFTP folder for %s

[Mount]
What=%s
Where=%s
Type=none
Options=bind,nosuid,nodev

[Install]
WantedBy=multi-user.target
`, version.AppName, site.Domain, m.paths.siteRoot(site.Domain), where)
	if err := writeFile(filepath.Join("/etc/systemd/system", unit), []byte(body), 0o644); err != nil {
		return err
	}
	if _, err := m.run.Output(ctx, "systemctl", "daemon-reload"); err != nil {
		return err
	}
	if _, err := m.run.Output(ctx, "systemctl", "enable", "--now", unit); err != nil {
		return err
	}
	if !m.mounted(ctx, where) {
		return errors.New("the site folder could not be mounted for SFTP")
	}

	if err := m.addToGroup(ctx, site.SysUser, sftpGroup); err != nil {
		return err
	}
	if err := m.removeFromGroup(ctx, site.SysUser, siteGroup); err != nil {
		return err
	}
	// Ask sshd itself: the account must get the chroot and nothing but SFTP.
	cfg, err := m.sshdEffective(ctx, site.SysUser)
	if err != nil {
		return err
	}
	// "sshd -T" may print the chroot with or without %u expanded.
	chroot := cfg["chrootdirectory"]
	if (chroot != sftpChroot(site.SysUser) && chroot != sftpRoot+"/%u") || cfg["forcecommand"] != sftpCommand {
		return errors.New("sshd does not apply the SFTP settings to this account")
	}
	return nil
}

// disableSFTP removes the account's SFTP access and the chroot. It is safe
// to call when SFTP is already off.
func (m *Manager) disableSFTP(ctx context.Context, site *store.Site) error {
	name := site.SysUser
	if _, err := user.Lookup(name); err == nil {
		if _, err := user.LookupGroup(siteGroup); err == nil {
			if err := m.addToGroup(ctx, name, siteGroup); err != nil {
				return err
			}
		}
		if err := m.removeFromGroup(ctx, name, sftpGroup); err != nil {
			return err
		}
		if _, err := m.run.Output(ctx, "usermod", "--password", "*", name); err != nil {
			return err
		}
		// End open sessions, which would also keep the folder mounted.
		m.run.Output(ctx, "pkill", "--signal", "KILL", "--euid", name, "--exact", "sshd|sshd-session")
	}
	if err := removeIfExists(sftpKeysFile(name)); err != nil {
		return err
	}

	where := m.sftpMountPoint(site)
	if unit, err := m.mountUnit(ctx, where); err == nil {
		file := filepath.Join("/etc/systemd/system", unit)
		if fileExists(file) {
			m.run.Output(ctx, "systemctl", "disable", "--now", unit)
			if err := removeIfExists(file); err != nil {
				return err
			}
			m.run.Output(ctx, "systemctl", "daemon-reload")
		}
	}
	if m.mounted(ctx, where) {
		m.run.Output(ctx, "umount", "--lazy", where)
	}
	if m.mounted(ctx, where) {
		return fmt.Errorf("could not unmount %s", where)
	}
	// Remove the empty chroot folders, deepest first. Never RemoveAll here:
	// a folder that is still mounted would take the site's files with it.
	for dir := where; strings.HasPrefix(dir, sftpRoot+"/"); dir = filepath.Dir(dir) {
		if err := os.Remove(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// mountUnit returns the systemd unit name for a mount point.
func (m *Manager) mountUnit(ctx context.Context, where string) (string, error) {
	out, err := m.run.Output(ctx, "systemd-escape", "--path", "--suffix=mount", where)
	name := strings.TrimSpace(out)
	if err != nil || !strings.HasSuffix(name, ".mount") || strings.ContainsAny(name, "/\n") {
		return "", fmt.Errorf("systemd-escape %s: %v", where, err)
	}
	return name, nil
}

func (m *Manager) mounted(ctx context.Context, where string) bool {
	_, err := m.run.Output(ctx, "mountpoint", "--quiet", where)
	return err == nil
}
