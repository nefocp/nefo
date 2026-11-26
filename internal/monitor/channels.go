package monitor

// Alert channels besides Telegram and e-mail: Discord, Slack and a generic
// webhook. Every alert goes through deliver, which sends to all configured
// channels at once.

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf16"

	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/version"
)

const (
	hookTimeout   = 10 * time.Second
	maxHookURL    = 2048
	discordLimit  = 2000     // characters in a Discord message
	slackLimit    = 4000     // characters Slack advises for a message
	maxHookAnswer = 64 << 10 // bytes of an answer read before closing it
)

var (
	discordRe = regexp.MustCompile(`^https://(?:(?:ptb|canary)\.)?discord(?:app)?\.com/api/(?:v[0-9]{1,2}/)?webhooks/[0-9]{1,25}/[A-Za-z0-9_-]{1,100}(?:\?thread_id=[0-9]{1,25})?$`)
	slackRe   = regexp.MustCompile(`^https://hooks\.slack\.com/services/[A-Za-z0-9]{1,32}/[A-Za-z0-9]{1,32}/[A-Za-z0-9]{1,64}$`)
	// hookHostRe is a host name: labels of letters, digits and inner dashes.
	hookHostRe = regexp.MustCompile(`^(?:[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?\.)*[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?\.?$`)
)

// HasChannel reports whether at least one channel is configured.
func (s *Settings) HasChannel() bool {
	return s.HasTelegram() || s.HasEmail() || s.DiscordURL != "" || s.SlackURL != "" || s.WebhookURL != ""
}

// validateHooks checks the Discord, Slack and webhook settings.
func (s *Settings) validateHooks() error {
	if s.DiscordURL != "" && !discordRe.MatchString(s.DiscordURL) {
		return usererr.New("invalid_discord")
	}
	if s.SlackURL != "" && !slackRe.MatchString(s.SlackURL) {
		return usererr.New("invalid_slack")
	}
	if s.WebhookURL != "" {
		if err := checkWebhookURL(s.WebhookURL, s.WebhookHTTP); err != nil {
			return err
		}
	}
	if s.WebhookSecret != "" && (len(s.WebhookSecret) < 16 || len(s.WebhookSecret) > 200 || !visibleASCII(s.WebhookSecret)) {
		return usererr.New("invalid_webhook_secret")
	}
	return nil
}

// checkWebhookURL accepts an https:// address (http:// when allowed) with a
// host, without user name, password or fragment, and refuses addresses of
// this server and of cloud metadata services. Names are checked again after
// DNS resolution, when sending.
func checkWebhookURL(raw string, allowHTTP bool) error {
	if len(raw) > maxHookURL || !visibleASCII(raw) || strings.Contains(raw, "#") {
		return usererr.New("invalid_webhook")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Opaque != "" || u.User != nil || u.Host == "" {
		return usererr.New("invalid_webhook")
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !allowHTTP {
			return usererr.New("webhook_needs_https")
		}
	default:
		return usererr.New("invalid_webhook")
	}
	if p := u.Port(); p != "" {
		if n, err := strconv.Atoi(p); err != nil || n < 1 || n > 65535 {
			return usererr.New("invalid_webhook")
		}
	}
	host := u.Hostname()
	if a, err := netip.ParseAddr(host); err == nil {
		if blockedAddr(a) {
			return usererr.New("webhook_blocked_address", host)
		}
		return nil
	}
	if len(host) > 253 || !hookHostRe.MatchString(host) {
		return usererr.New("invalid_webhook")
	}
	if h := strings.ToLower(strings.TrimSuffix(host, ".")); h == "localhost" || strings.HasSuffix(h, ".localhost") {
		return usererr.New("webhook_blocked_address", host)
	}
	return nil
}

// visibleASCII reports whether s has only printable ASCII without spaces.
func visibleASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] <= ' ' || s[i] >= 0x7f {
			return false
		}
	}
	return true
}

// maskHooks hides the webhook addresses, which hold their token, and the
// secret.
func maskHooks(s *Settings) {
	s.DiscordURL, s.SlackURL, s.WebhookURL, s.WebhookSecret = mask(s.DiscordURL), mask(s.SlackURL), mask(s.WebhookURL), mask(s.WebhookSecret)
}

// keepHooks trims the webhook settings and keeps the saved value of a field
// the form sends back masked. An empty address removes the channel.
func keepHooks(s *Settings, old Settings) {
	for _, f := range []struct {
		cur *string
		old string
	}{{&s.DiscordURL, old.DiscordURL}, {&s.SlackURL, old.SlackURL}, {&s.WebhookURL, old.WebhookURL}, {&s.WebhookSecret, old.WebhookSecret}} {
		if *f.cur = strings.TrimSpace(*f.cur); *f.cur == mask("x") {
			*f.cur = f.old
		}
	}
	if s.WebhookURL == "" {
		s.WebhookSecret, s.WebhookHTTP = "", false
	}
}

// ---- sending to every channel ----

// message is an alert or a test, before each channel formats it.
type message struct {
	kind    string // "alert" or "test", for the webhook
	subject string // of the e-mail
	text    string // without the server name, which the channels add
}

// deliver sends msg to every configured channel at once and returns how it
// went on each. A slow or failing channel does not hold up the others.
func (m *Monitor) deliver(ctx context.Context, s Settings, msg message) map[string]TestResult {
	server := hostname()
	line := fmt.Sprintf("[%s] %s", server, msg.text)
	type channel struct {
		name, code string // code: for errors without one of their own
		send       func() error
	}
	var chs []channel
	if s.HasTelegram() {
		chs = append(chs, channel{"telegram", "telegram_failed", func() error { return m.telegram(ctx, s, line) }})
	}
	if s.HasEmail() {
		chs = append(chs, channel{"email", "smtp_failed", func() error { return sendMail(s, msg.subject, line) }})
	}
	if s.DiscordURL != "" {
		chs = append(chs, channel{"discord", "webhook_unreachable", func() error { return m.discord(ctx, s.DiscordURL, line) }})
	}
	if s.SlackURL != "" {
		chs = append(chs, channel{"slack", "webhook_unreachable", func() error { return m.slack(ctx, s.SlackURL, line) }})
	}
	if s.WebhookURL != "" {
		chs = append(chs, channel{"webhook", "webhook_unreachable", func() error { return m.webhook(ctx, s, server, msg) }})
	}
	res := make(map[string]TestResult, len(chs))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, ch := range chs {
		wg.Go(func() {
			r := testResult(ch.send(), ch.code)
			mu.Lock()
			res[ch.name] = r
			mu.Unlock()
		})
	}
	wg.Wait()
	return res
}

func (m *Monitor) discord(ctx context.Context, hook, line string) error {
	// No @everyone or role pings from a message's text.
	body, _ := json.Marshal(map[string]any{"content": truncateUTF16(line, discordLimit),
		"allowed_mentions": map[string][]string{"parse": {}}})
	return m.postHook(ctx, hook, body, nil, func(b []byte) string {
		var e struct {
			Message string `json:"message"`
		}
		json.Unmarshal(b, &e)
		return e.Message
	})
}

func (m *Monitor) slack(ctx context.Context, hook, line string) error {
	// Slack reads <…> as links and mentions, such as <!channel>.
	esc := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(truncateUTF16(line, slackLimit))
	body, _ := json.Marshal(map[string]string{"text": esc})
	return m.postHook(ctx, hook, body, nil, func(b []byte) string { return string(b) })
}

// hookPayload is what the generic webhook receives.
type hookPayload struct {
	Server string `json:"server"`
	Text   string `json:"text"`
	Time   string `json:"time"`
	Kind   string `json:"kind"`
}

// webhook posts the alert as JSON. With a secret, X-Nefo-Signature holds
// "sha256=" and the hex HMAC-SHA256 of the body.
func (m *Monitor) webhook(ctx context.Context, s Settings, server string, msg message) error {
	body, _ := json.Marshal(hookPayload{Server: server, Text: msg.text, Time: time.Now().UTC().Format(time.RFC3339), Kind: msg.kind})
	var h http.Header
	if s.WebhookSecret != "" {
		h = http.Header{"X-Nefo-Signature": {signature(s.WebhookSecret, body)}}
	}
	// The receiver's answer is not shown: the address may be on a private
	// network.
	return m.postHook(ctx, s.WebhookURL, body, h, nil)
}

func signature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// postHook posts a JSON body. Redirects are not followed, so a 3xx answer
// is a failure, and at most maxHookAnswer bytes of the answer are read.
// answer, when set, extracts a short reason from a refusal. Errors never
// contain the address, which holds the webhook's token.
func (m *Monitor) postHook(ctx context.Context, hook string, body []byte, header http.Header, answer func([]byte) string) error {
	ctx, cancel := context.WithTimeout(ctx, hookTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hook, bytes.NewReader(body))
	if err != nil {
		return usererr.New("invalid_webhook")
	}
	for k, v := range header {
		req.Header[k] = v
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", version.AppName+"/"+version.Version)
	resp, err := m.hooks.Do(req)
	if err != nil {
		var be *blockedError
		if errors.As(err, &be) {
			return usererr.New("webhook_blocked_address", be.addr.String())
		}
		var ne net.Error
		if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout()) {
			return usererr.New("webhook_timeout")
		}
		return usererr.New("webhook_unreachable", hookCause(err, hook))
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, maxHookAnswer))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		detail := "HTTP " + strconv.Itoa(resp.StatusCode)
		if answer != nil {
			if a := oneLine(answer(b), 120); a != "" {
				detail += ", " + a
			}
		}
		return usererr.New("webhook_refused", detail)
	}
	return nil
}

// hookCause describes a connection failure without the webhook's address.
func hookCause(err error, hook string) string {
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	s := strings.ReplaceAll(err.Error(), hook, "…")
	if u, perr := url.Parse(hook); perr == nil && len(u.EscapedPath()) > 1 {
		s = strings.ReplaceAll(s, u.EscapedPath(), "/…")
	}
	return oneLine(s, 200)
}

// oneLine keeps the first line of s, printable characters only, cut to at
// most n runes.
func oneLine(s string, n int) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	s = strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, s)
	if r := []rune(s); len(r) > n {
		s = string(r[:n-1]) + "…"
	}
	return s
}

// truncateUTF16 cuts s to at most n UTF-16 code units, as Discord counts,
// between two characters, ending with "…" when cut.
func truncateUTF16(s string, n int) string {
	units := 0
	for _, r := range s {
		units += utf16.RuneLen(r)
	}
	if units <= n {
		return s
	}
	units = 1 // the "…"
	for i, r := range s {
		if units+utf16.RuneLen(r) > n {
			return s[:i] + "…"
		}
		units += utf16.RuneLen(r)
	}
	return s
}

// ---- reaching webhooks safely ----

// metadataAddrs are cloud metadata and platform services outside the
// link-local range.
var metadataAddrs = []netip.Addr{
	netip.MustParseAddr("100.100.100.200"), // Alibaba Cloud
	netip.MustParseAddr("168.63.129.16"),   // Azure platform
	netip.MustParseAddr("fd00:ec2::254"),   // AWS over IPv6
}

var (
	thisNetwork = netip.MustParsePrefix("0.0.0.0/8") // reaches this server on Linux
	v4Compat    = netip.MustParsePrefix("::/96")
	nat64Prefix = netip.MustParsePrefix("64:ff9b::/96")
)

// blockedAddr reports whether webhooks must not connect to a: loopback,
// link-local (with 169.254.169.254), cloud metadata, unspecified and
// multicast addresses. Private networks are allowed, for receivers at home
// or in the same network.
func blockedAddr(a netip.Addr) bool {
	a = a.Unmap().WithZone("")
	if !a.IsValid() || a.IsLoopback() || a.IsLinkLocalUnicast() || a.IsUnspecified() || a.IsMulticast() ||
		a.IsLinkLocalMulticast() || a.IsInterfaceLocalMulticast() || thisNetwork.Contains(a) || v4Compat.Contains(a) {
		return true
	}
	if nat64Prefix.Contains(a) {
		b := a.As16()
		return blockedAddr(netip.AddrFrom4([4]byte(b[12:])))
	}
	for _, m := range metadataAddrs {
		if a == m {
			return true
		}
	}
	return false
}

// blockedError is a webhook address the panel refuses to connect to.
type blockedError struct{ addr netip.Addr }

func (e *blockedError) Error() string {
	return e.addr.String() + " is a loopback, link-local or cloud metadata address"
}

// hookNet is how webhook requests reach the network. Tests replace the
// resolver and the dialer; the address check always runs between them.
type hookNet struct {
	resolve func(ctx context.Context, host string) ([]netip.Addr, error)
	dial    func(ctx context.Context, network, addr string) (net.Conn, error)
	tls     *tls.Config
}

func defaultHookNet() hookNet {
	d := &net.Dialer{Timeout: hookTimeout, KeepAlive: 30 * time.Second}
	return hookNet{
		resolve: func(ctx context.Context, host string) ([]netip.Addr, error) {
			return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		},
		dial: d.DialContext,
	}
}

// dialContext resolves the host itself and connects only to an address
// blockedAddr allows, so a name that resolves to 127.0.0.1 or to the
// metadata service is refused even if it changes its answer.
func (n hookNet) dialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	var addrs []netip.Addr
	if a, err := netip.ParseAddr(host); err == nil {
		addrs = []netip.Addr{a}
	} else if addrs, err = n.resolve(ctx, host); err != nil {
		return nil, err
	}
	var blocked, dialErr error
	for _, a := range addrs {
		if blockedAddr(a) {
			blocked = &blockedError{a.Unmap().WithZone("")}
			continue
		}
		c, err := n.dial(ctx, network, net.JoinHostPort(a.Unmap().String(), port))
		if err == nil {
			return c, nil
		}
		dialErr = err
	}
	switch {
	case dialErr != nil:
		return nil, dialErr
	case blocked != nil:
		return nil, blocked
	}
	return nil, fmt.Errorf("no address found for %s", host)
}

// newHookClient returns the client for webhooks: 10 seconds in all, no
// proxy (it would bypass the address check) and no redirects.
func newHookClient(n hookNet) *http.Client {
	return &http.Client{
		Timeout: hookTimeout,
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           n.dialContext,
			TLSClientConfig:       n.tls,
			ForceAttemptHTTP2:     true,
			TLSHandshakeTimeout:   hookTimeout,
			ResponseHeaderTimeout: hookTimeout,
			MaxIdleConns:          4,
			IdleConnTimeout:       30 * time.Second,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}
