// Package monitor records the server's load every minute and sends alerts
// by Telegram, e-mail, Discord, Slack or a webhook when something needs
// attention.
package monitor

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/system"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/version"
)

// Settings configure where alerts go and when they are sent.
type Settings struct {
	TelegramToken string `json:"telegram_token,omitempty"`
	TelegramChat  string `json:"telegram_chat"`
	SMTPHost      string `json:"smtp_host"`
	SMTPPort      int    `json:"smtp_port"`
	SMTPUser      string `json:"smtp_user"`
	SMTPPassword  string `json:"smtp_password,omitempty"`
	EmailFrom     string `json:"email_from"`
	EmailTo       string `json:"email_to"`
	CPUPercent    int    `json:"cpu_percent"`  // 0 = off; average over 5 minutes
	MemPercent    int    `json:"mem_percent"`  // 0 = off
	DiskPercent   int    `json:"disk_percent"` // 0 = off
	Services      bool   `json:"services"`     // the web server, PHP-FPM, MariaDB… stopped
	Events        bool   `json:"events"`       // failed backups and certificate renewals

	// Webhook channels (channels.go). The addresses hold their token, so
	// they are secrets like the Telegram token.
	DiscordURL    string `json:"discord_url"`
	SlackURL      string `json:"slack_url"`
	WebhookURL    string `json:"webhook_url"`
	WebhookSecret string `json:"webhook_secret"` // signs the webhook's body; optional
	WebhookHTTP   bool   `json:"webhook_http"`   // allow an http:// webhook

	// Sign-in alerts (logins.go): "", "all" or "new_ip".
	SSHLogins   string `json:"ssh_logins"`
	SSHFailed   int    `json:"ssh_failed"` // failed SSH attempts in 10 minutes that send an alert; 0 = off
	PanelLogins string `json:"panel_logins"`
}

// HasTelegram and HasEmail report which channels are configured.
func (s *Settings) HasTelegram() bool { return s.TelegramToken != "" && s.TelegramChat != "" }
func (s *Settings) HasEmail() bool    { return s.SMTPHost != "" && s.EmailTo != "" && s.EmailFrom != "" }

var (
	tokenRe = regexp.MustCompile(`^[0-9]{5,15}:[A-Za-z0-9_-]{20,60}$`)
	chatRe  = regexp.MustCompile(`^-?[0-9]{1,20}$|^@[A-Za-z0-9_]{4,40}$`)
	hostRe  = regexp.MustCompile(`^[A-Za-z0-9.-]{1,253}$`)
	mailRe  = regexp.MustCompile(`^[^@\s<>,;"]{1,64}@[A-Za-z0-9.-]{1,253}$`)
)

// Validate checks the settings.
func (s *Settings) Validate() error {
	if s.TelegramToken != "" && !tokenRe.MatchString(s.TelegramToken) {
		return usererr.New("invalid_telegram")
	}
	if s.TelegramChat != "" && !chatRe.MatchString(s.TelegramChat) {
		return usererr.New("invalid_telegram")
	}
	if s.SMTPHost != "" {
		if !hostRe.MatchString(s.SMTPHost) || s.SMTPPort < 1 || s.SMTPPort > 65535 ||
			!mailRe.MatchString(s.EmailFrom) || !mailRe.MatchString(s.EmailTo) || strings.ContainsAny(s.SMTPUser, "\r\n") {
			return usererr.New("invalid_smtp")
		}
	}
	if err := s.validateHooks(); err != nil {
		return err
	}
	if err := s.validateLogins(); err != nil {
		return err
	}
	for _, p := range []int{s.CPUPercent, s.MemPercent, s.DiskPercent} {
		if p != 0 && (p < 10 || p > 100) {
			return usererr.New("invalid_threshold")
		}
	}
	return nil
}

const settingsKey = "alerts"

// Monitor collects metrics and sends alerts.
type Monitor struct {
	store    *store.Store
	sys      *system.Collector
	log      *slog.Logger
	services func(ctx context.Context) map[string]string // unit -> ActiveState
	http     *http.Client
	hooks    *http.Client // Discord, Slack and the webhook; see newHookClient

	mu     sync.Mutex
	active map[string]time.Time // alert key -> last sent

	logins *loginWatch // sign-in alerts (logins.go)
}

// New returns a Monitor. services reports the state of the units to watch.
func New(st *store.Store, sys *system.Collector, log *slog.Logger, services func(context.Context) map[string]string) *Monitor {
	return &Monitor{store: st, sys: sys, log: log, services: services,
		http: &http.Client{Timeout: 20 * time.Second}, hooks: newHookClient(defaultHookNet()), active: map[string]time.Time{},
		logins: &loginWatch{now: time.Now}}
}

// Settings returns the alert settings without secrets.
func (m *Monitor) Settings(ctx context.Context) (Settings, error) {
	s, err := m.settings(ctx)
	s.TelegramToken, s.SMTPPassword = mask(s.TelegramToken), mask(s.SMTPPassword)
	maskHooks(&s)
	return s, err
}

func mask(s string) string {
	if s == "" {
		return ""
	}
	return "••••••"
}

func (m *Monitor) settings(ctx context.Context) (Settings, error) {
	s := Settings{SMTPPort: 587, CPUPercent: 90, MemPercent: 90, DiskPercent: 90, Services: true, Events: true}
	v, err := m.store.Setting(ctx, settingsKey)
	if err != nil || v == "" {
		return s, err
	}
	err = json.Unmarshal([]byte(v), &s)
	return s, err
}

// Save stores new settings. Masked or empty secrets keep their old value.
func (m *Monitor) Save(ctx context.Context, s Settings) error {
	old, err := m.settings(ctx)
	if err != nil {
		return err
	}
	if s.TelegramToken == "" || s.TelegramToken == mask("x") {
		s.TelegramToken = old.TelegramToken
	}
	if s.SMTPPassword == "" || s.SMTPPassword == mask("x") {
		s.SMTPPassword = old.SMTPPassword
	}
	if s.TelegramChat == "" {
		s.TelegramToken = ""
	}
	keepHooks(&s, old)
	if err := s.Validate(); err != nil {
		return err
	}
	b, _ := json.Marshal(s)
	return m.store.SetSetting(ctx, settingsKey, string(b))
}

// ---- collecting ----

// Run samples the server every minute, checks the alert rules and keeps
// seven days of history, until ctx is done.
func (m *Monitor) Run(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	purged := time.Time{}
	net, netAt := system.NetCounters(), time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		m.watchSSH(ctx) // sign-in alerts, from the journal (logins.go)
		st, err := m.sys.Stats()
		if err != nil || !st.Supported {
			continue
		}
		now := time.Now()
		mt := store.Metric{Time: now, CPU: st.CPUPercent, Mem: pct(st.MemUsed, st.MemTotal), Disk: pct(st.DiskUsed, st.DiskTotal), Load1: st.Load1}
		if cur := system.NetCounters(); cur != nil {
			if net != nil {
				rx, tx := system.NetRate(net, cur, now.Sub(netAt))
				mt.NetRx, mt.NetTx = &rx, &tx
			}
			net, netAt = cur, now
		}
		if err := m.store.AddMetric(ctx, mt); err != nil {
			m.log.Warn("store metric", "err", err)
		}
		if now.Sub(purged) > time.Hour {
			m.store.PurgeMetricsBefore(ctx, now.Add(-7*24*time.Hour))
			purged = now
		}
		m.check(ctx, mt)
	}
}

func pct(used, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(used) * 100 / float64(total)
}

// check evaluates the rules. Each problem is reported once (and again after
// six hours if it lasts), and its end is reported too.
func (m *Monitor) check(ctx context.Context, mt store.Metric) {
	s, err := m.settings(ctx)
	if err != nil || !s.HasChannel() {
		return
	}
	problems := map[string]string{}
	if s.CPUPercent > 0 {
		hist, _ := m.store.Metrics(ctx, mt.Time.Add(-5*time.Minute), time.Minute)
		if len(hist) >= 5 {
			sum := 0.0
			for _, h := range hist {
				sum += h.CPU
			}
			if avg := sum / float64(len(hist)); avg >= float64(s.CPUPercent) {
				problems["cpu"] = fmt.Sprintf("CPU usage has been %.0f%% on average for 5 minutes.", avg)
			}
		}
	}
	if s.MemPercent > 0 && mt.Mem >= float64(s.MemPercent) {
		problems["mem"] = fmt.Sprintf("Memory usage is %.0f%%.", mt.Mem)
	}
	if s.DiskPercent > 0 && mt.Disk >= float64(s.DiskPercent) {
		problems["disk"] = fmt.Sprintf("The disk is %.0f%% full.", mt.Disk)
	}
	if s.Services && m.services != nil {
		for unit, state := range m.services(ctx) {
			if state != "active" && state != "activating" && state != "reloading" {
				problems["service:"+unit] = fmt.Sprintf("The service %s is %s.", unit, state)
			}
		}
	}

	m.mu.Lock()
	var send []string
	now := time.Now()
	for key, text := range problems {
		if last, ok := m.active[key]; !ok || now.Sub(last) > 6*time.Hour {
			m.active[key] = now
			send = append(send, "⚠️ "+text)
		}
	}
	for key := range m.active {
		if _, still := problems[key]; !still && !strings.HasPrefix(key, "event:") {
			delete(m.active, key)
			send = append(send, "✅ Resolved: "+describe(key))
		}
	}
	m.mu.Unlock()
	for _, text := range send {
		m.send(ctx, s, text)
	}
}

func describe(key string) string {
	switch key {
	case "cpu":
		return "CPU usage is back to normal."
	case "mem":
		return "memory usage is back to normal."
	case "disk":
		return "disk usage is back below the limit."
	}
	return strings.TrimPrefix(key, "service:") + " is running again."
}

// Event reports a one-off problem, such as a failed backup.
func (m *Monitor) Event(ctx context.Context, text string) {
	s, err := m.settings(ctx)
	if err != nil || !s.Events || !s.HasChannel() {
		return
	}
	go m.send(context.WithoutCancel(ctx), s, "⚠️ "+text)
}

// Notify sends text through every configured channel and returns when it is
// sent. Its caller decided that it is worth an alert (a monitored website is
// down, for example), so the Events switch does not apply.
func (m *Monitor) Notify(ctx context.Context, text string) {
	if s, err := m.settings(ctx); err == nil {
		m.send(ctx, s, text)
	}
}

// ---- sending ----

func hostname() string {
	h, _ := os.Hostname()
	return h
}

// send delivers an alert to every configured channel at once (see
// deliver). A failing channel is logged by name and does not stop the others.
func (m *Monitor) send(ctx context.Context, s Settings, text string) {
	for ch, r := range m.deliver(ctx, s, message{kind: "alert", subject: "Server alert: " + hostname(), text: text}) {
		if !r.OK {
			m.log.Warn("alert not sent", "channel", ch, "err", r.Error, "detail", r.Detail)
		}
	}
}

// TestResult is how a test message went on one channel. Error is a code the
// UI translates (errors.<code>), with Detail as its {detail}.
type TestResult struct {
	OK     bool   `json:"ok"`
	Error  string `json:"error,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// Test sends a test message through every configured channel and returns
// one result per channel.
func (m *Monitor) Test(ctx context.Context) map[string]TestResult {
	s, _ := m.settings(ctx)
	return m.deliver(ctx, s, message{kind: "test", subject: "Test from " + version.AppName,
		text: "Test message from " + version.AppName + ". Alerts work."})
}

// testResult turns a send error into a result. Errors without a code of
// their own, such as the SMTP server's answer, get code with the text as
// detail.
func testResult(err error, code string) TestResult {
	if err == nil {
		return TestResult{OK: true}
	}
	var ue *usererr.Error
	if errors.As(err, &ue) {
		return TestResult{Error: ue.Code, Detail: ue.Detail}
	}
	return TestResult{Error: code, Detail: err.Error()}
}

func (m *Monitor) telegram(ctx context.Context, s Settings, text string) error {
	body, _ := json.Marshal(map[string]string{"chat_id": s.TelegramChat, "text": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.telegram.org/bot"+s.TelegramToken+"/sendMessage", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.http.Do(req)
	if err != nil {
		// The URL contains the token: never log the raw error.
		return usererr.New("telegram_unreachable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Description string `json:"description"`
		}
		json.NewDecoder(resp.Body).Decode(&e)
		return usererr.New("telegram_refused", fmt.Sprintf("HTTP %d: %s", resp.StatusCode, e.Description))
	}
	return nil
}

// sendMail sends a plain-text message. Port 465 uses TLS from the start;
// other ports use STARTTLS, which is required before signing in.
func sendMail(s Settings, subject, text string) error {
	addr := net.JoinHostPort(s.SMTPHost, strconv.Itoa(s.SMTPPort))
	msg := []byte("From: " + s.EmailFrom + "\r\nTo: " + s.EmailTo + "\r\nSubject: " + subject +
		"\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n" +
		strings.ReplaceAll(text, "\n", "\r\n") + "\r\n")
	tlsCfg := &tls.Config{ServerName: s.SMTPHost, MinVersion: tls.VersionTLS12}
	var c *smtp.Client
	if s.SMTPPort == 465 {
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 15 * time.Second}, "tcp", addr, tlsCfg)
		if err != nil {
			return err
		}
		if c, err = smtp.NewClient(conn, s.SMTPHost); err != nil {
			return err
		}
	} else {
		conn, err := net.DialTimeout("tcp", addr, 15*time.Second)
		if err != nil {
			return err
		}
		if c, err = smtp.NewClient(conn, s.SMTPHost); err != nil {
			return err
		}
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(tlsCfg); err != nil {
				return err
			}
		} else if s.SMTPUser != "" {
			return usererr.New("smtp_no_starttls")
		}
	}
	defer c.Close()
	if s.SMTPUser != "" {
		if err := c.Auth(smtp.PlainAuth("", s.SMTPUser, s.SMTPPassword, s.SMTPHost)); err != nil {
			return err
		}
	}
	if err := c.Mail(s.EmailFrom); err != nil {
		return err
	}
	if err := c.Rcpt(s.EmailTo); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}
