// Package mailrelay lets websites send e-mail. It installs Postfix as a
// send-only relay: it listens on localhost only and passes every message to
// an SMTP provider (Gmail, SendGrid, Amazon SES…) with the account the
// administrator enters. PHP's mail() uses /usr/sbin/sendmail, which Postfix
// provides, so applications work without plugins. The provider's password
// is kept in a file only root can read; websites never see it.
package mailrelay

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/usererr"
)

// Security of the connection to the provider.
const (
	StartTLS = "starttls" // port 587: plain connection upgraded to TLS
	TLS      = "tls"      // port 465: TLS from the start
	None     = "none"     // only for a relay on this server
)

const (
	saslFile    = "/etc/postfix/sasl_passwd"
	sendmailBin = "/usr/sbin/sendmail"
)

// Settings is the relay configuration.
type Settings struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Security string `json:"security"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"` // never sent back to the UI
	// From, if set, replaces the sender of every message (some providers
	// only accept their verified addresses).
	From string `json:"from"`
}

// Status describes the relay.
type Status struct {
	Installed  bool      `json:"installed"`
	Configured bool      `json:"configured"`
	Running    bool      `json:"running"`
	Settings   *Settings `json:"settings,omitempty"` // without the password
	Queued     int       `json:"queued"`
	LastError  string    `json:"last_error,omitempty"`
}

// Store keeps the settings (the panel's settings table).
type Store interface {
	Setting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
}

const settingKey = "mail_relay"

// Manager configures Postfix.
type Manager struct {
	run   run.Runner
	store Store
}

// New returns a Manager.
func New(r run.Runner, st Store) *Manager { return &Manager{run: r, store: st} }

var (
	hostRe  = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]{0,62}\.)*[A-Za-z0-9-]{1,63}$`)
	emailRe = regexp.MustCompile(`^[A-Za-z0-9._%+-]{1,64}@[A-Za-z0-9.-]{1,253}\.[A-Za-z]{2,63}$`)
)

// Check validates settings. When the password is empty, keep is the one
// saved before.
func (s *Settings) Check(keep string) error {
	s.Host = strings.TrimSpace(strings.ToLower(s.Host))
	s.Username = strings.TrimSpace(s.Username)
	s.From = strings.TrimSpace(s.From)
	if !hostRe.MatchString(s.Host) && net.ParseIP(s.Host) == nil {
		return usererr.New("invalid_smtp_host", s.Host)
	}
	if s.Port < 1 || s.Port > 65535 {
		return usererr.New("invalid_port", strconv.Itoa(s.Port))
	}
	switch s.Security {
	case StartTLS, TLS:
	case None:
		// Credentials must never cross the internet unencrypted.
		if s.Host != "localhost" && s.Host != "127.0.0.1" && s.Host != "::1" {
			return usererr.New("smtp_tls_required")
		}
	default:
		return usererr.New("invalid_smtp_security")
	}
	if s.Password == "" {
		s.Password = keep
	}
	for _, v := range []string{s.Username, s.Password} {
		if len(v) > 300 || strings.ContainsAny(v, " \t\r\n\x00") {
			return usererr.New("invalid_smtp_credentials")
		}
	}
	if (s.Username == "") != (s.Password == "") {
		return usererr.New("invalid_smtp_credentials")
	}
	if s.From != "" && !emailRe.MatchString(s.From) {
		return usererr.New("invalid_email")
	}
	return nil
}

func (m *Manager) load(ctx context.Context) (*Settings, error) {
	v, err := m.store.Setting(ctx, settingKey)
	if err != nil || v == "" {
		return nil, err
	}
	var s Settings
	if err := json.Unmarshal([]byte(v), &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Installed reports whether Postfix is installed.
func (m *Manager) Installed() bool {
	_, err := os.Stat("/usr/sbin/postfix")
	return err == nil
}

// Status returns the relay's state.
func (m *Manager) Status(ctx context.Context) Status {
	st := Status{Installed: m.Installed()}
	if s, err := m.load(ctx); err == nil && s != nil {
		s.Password = ""
		st.Settings, st.Configured = s, true
	}
	if st.Installed {
		out, _ := m.run.Output(ctx, "systemctl", "is-active", "postfix")
		st.Running = strings.HasPrefix(out, "active")
		st.Queued, st.LastError = m.queue(ctx)
	}
	return st
}

// queue counts the queued messages and returns the latest delivery error.
func (m *Manager) queue(ctx context.Context) (int, string) {
	out, err := m.run.Output(ctx, "postqueue", "-j")
	if err != nil {
		return 0, ""
	}
	n, last := 0, ""
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		var e struct {
			Recipients []struct {
				Reason string `json:"delay_reason"`
			} `json:"recipients"`
		}
		if json.Unmarshal(sc.Bytes(), &e) != nil {
			continue
		}
		n++
		for _, r := range e.Recipients {
			if r.Reason != "" {
				last = r.Reason
			}
		}
	}
	return n, last
}

// Logger receives progress messages (a job).
type Logger interface {
	Write(p []byte) (int, error)
	Logf(format string, args ...any)
}

// Configure installs Postfix if needed and points it at the provider.
func (m *Manager) Configure(ctx context.Context, s Settings, lg Logger) error {
	old, err := m.load(ctx)
	if err != nil {
		return err
	}
	keep := ""
	if old != nil {
		keep = old.Password
	}
	if err := s.Check(keep); err != nil {
		return err
	}
	if !m.Installed() {
		lg.Logf("Installing Postfix (send-only)")
		// "Local only" creates complete configuration files (listening on
		// localhost); the panel then turns it into a relay with postconf.
		host, _ := os.Hostname()
		if !hostRe.MatchString(host) {
			host = "localhost"
		}
		sel := "postfix postfix/main_mailer_type select Local only\npostfix postfix/mailname string " + host + "\n"
		if _, err := m.run.Input(ctx, []byte(sel), "debconf-set-selections"); err != nil {
			return err
		}
		if err := run.Apt(ctx, m.run, lg, "install", "-y", "--no-install-recommends",
			"postfix", "libsasl2-modules", "ca-certificates"); err != nil {
			return err
		}
	}
	hostname, _ := os.Hostname()
	if hostname == "" || !hostRe.MatchString(hostname) {
		hostname = "localhost"
	}
	lg.Logf("Configuring Postfix to relay through %s:%d", s.Host, s.Port)
	if err := fsutil.WriteFileAtomic(saslFile, []byte(fmt.Sprintf("[%s]:%d %s:%s\n", s.Host, s.Port, s.Username, s.Password)), 0o600); err != nil {
		return err
	}
	conf := map[string]string{
		"myhostname":                     hostname,
		"inet_interfaces":                "loopback-only",
		"inet_protocols":                 "all",
		"mydestination":                  "$myhostname, localhost.$mydomain, localhost",
		"relayhost":                      fmt.Sprintf("[%s]:%d", s.Host, s.Port),
		"smtp_tls_CAfile":                "/etc/ssl/certs/ca-certificates.crt",
		"smtp_tls_loglevel":              "1",
		"smtp_sasl_auth_enable":          "no",
		"smtp_sasl_password_maps":        "texthash:" + saslFile,
		"smtp_sasl_security_options":     "noanonymous",
		"smtp_sasl_tls_security_options": "noanonymous",
		"smtp_tls_wrappermode":           "no",
		"smtp_tls_security_level":        "encrypt",
		"sender_canonical_maps":          "",
		"maximal_queue_lifetime":         "2d",
		// Debian's "Local only" setup bounces all outgoing mail.
		"default_transport": "smtp",
		"relay_transport":   "smtp",
	}
	if s.Username != "" {
		conf["smtp_sasl_auth_enable"] = "yes"
	}
	switch s.Security {
	case TLS:
		conf["smtp_tls_wrappermode"] = "yes"
	case None:
		conf["smtp_tls_security_level"] = "none"
	}
	if s.From != "" {
		// Envelope and From header of every message.
		conf["sender_canonical_maps"] = "static:" + s.From
	}
	args := []string{"-e"}
	for k, v := range conf {
		args = append(args, k+"="+v)
	}
	if _, err := m.run.Output(ctx, "postconf", args...); err != nil {
		return err
	}
	if out, err := m.run.Output(ctx, "postfix", "check"); err != nil {
		return fmt.Errorf("postfix check: %s", run.Tail(out, 5))
	}
	if _, err := m.run.Output(ctx, "systemctl", "enable", "postfix"); err != nil {
		return err
	}
	if _, err := m.run.Output(ctx, "systemctl", "restart", "postfix"); err != nil {
		return err
	}
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if err := m.store.SetSetting(ctx, settingKey, string(b)); err != nil {
		return err
	}
	lg.Logf("Websites can send e-mail now. Send a test message to check the account.")
	return nil
}

// SendTest sends a test message through the relay and waits up to a minute
// for the provider's answer. It returns how the delivery went.
func (m *Manager) SendTest(ctx context.Context, to string) (string, error) {
	to = strings.TrimSpace(to)
	if !emailRe.MatchString(to) {
		return "", usererr.New("invalid_email")
	}
	if !m.Installed() {
		return "", usererr.New("mail_relay_not_configured")
	}
	id := fmt.Sprintf("panel-test-%d", time.Now().UnixNano())
	msg := fmt.Sprintf("To: %s\nSubject: Test message from your server\nMessage-ID: <%s@localhost>\nContent-Type: text/plain; charset=utf-8\n\n"+
		"This message was sent from the control panel to check that websites on this server can send e-mail.\n", to, id)
	if _, err := m.run.Input(ctx, []byte(msg), sendmailBin, "-t", "-i"); err != nil {
		return "", err
	}
	// Follow the message in the mail log until the provider answers.
	deadline := time.Now().Add(60 * time.Second)
	var qid string
	for time.Now().Before(deadline) {
		out, _ := m.run.Output(ctx, "journalctl", "--facility=mail", "--since", "-5min", "--no-pager", "-o", "cat")
		if qid == "" {
			qid = findQueueID(out, id)
		}
		if qid != "" {
			if res := deliveryResult(out, qid); res != "" {
				return res, nil
			}
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return "", usererr.New("mail_test_timeout")
}

var (
	queueIDRe = regexp.MustCompile(`([0-9A-F]{6,})(?::)? message-id=<([^>]+)>`)
	statusRe  = regexp.MustCompile(`status=(\w+) \((.*)\)`)
)

// findQueueID finds the Postfix queue ID of the message with Message-ID id.
func findQueueID(log, id string) string {
	for _, l := range strings.Split(log, "\n") {
		if m := queueIDRe.FindStringSubmatch(l); m != nil && strings.HasPrefix(m[2], id) {
			return m[1]
		}
	}
	return ""
}

// deliveryResult returns "sent: …", "deferred: …" or "bounced: …" for a
// queue ID, or "" while there is no answer yet.
func deliveryResult(log, qid string) string {
	for _, l := range strings.Split(log, "\n") {
		if !strings.Contains(l, qid+":") || !strings.Contains(l, " to=<") {
			continue
		}
		if m := statusRe.FindStringSubmatch(l); m != nil {
			return m[1] + ": " + m[2]
		}
	}
	return ""
}

// Remove uninstalls Postfix and forgets the settings.
func (m *Manager) Remove(ctx context.Context, lg Logger) error {
	lg.Logf("Removing Postfix")
	if err := run.Apt(ctx, m.run, lg, "purge", "-y", "postfix"); err != nil {
		return err
	}
	os.Remove(saslFile)
	return m.store.SetSetting(ctx, settingKey, "")
}
