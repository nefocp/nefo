package monitor

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

const (
	testDiscord = "https://discord.com/api/webhooks/1234567890/abc_DEF-xyz"
	testSlack   = "https://hooks.slack.com/services/T0001/B0002/XXXXyyyyZZZZ"
	testHook    = "https://hooks.example.net/nefo?team=ops"
)

// fakeNet answers DNS from names and connects the resolved address to a
// local test server, so requests to discord.com never leave this machine.
// The address check of hookNet still runs on the resolved address.
type fakeNet struct {
	names   map[string][]netip.Addr
	servers map[netip.Addr]*httptest.Server
	mu      sync.Mutex
	dialed  []string
}

func (f *fakeNet) hookNet(t *testing.T) hookNet {
	// Every httptest TLS server uses the same certificate, for example.com.
	pool := httptest.NewTLSServer(http.NotFoundHandler())
	roots := pool.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs
	pool.Close()
	return hookNet{
		resolve: func(_ context.Context, host string) ([]netip.Addr, error) {
			if a, ok := f.names[host]; ok {
				return a, nil
			}
			return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
		},
		dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
			f.mu.Lock()
			f.dialed = append(f.dialed, addr)
			f.mu.Unlock()
			ap := netip.MustParseAddrPort(addr)
			srv, ok := f.servers[ap.Addr()]
			if !ok {
				return nil, errors.New("connection refused")
			}
			var d net.Dialer
			return d.DialContext(ctx, network, srv.Listener.Addr().String())
		},
		tls: &tls.Config{RootCAs: roots, ServerName: "example.com"},
	}
}

// received is one request a test server got.
type received struct {
	header http.Header
	body   []byte
}

// recorder is a test server that answers with status and body and keeps
// what it received.
func recorder(t *testing.T, status int, answer string) (*httptest.Server, func() []received) {
	var mu sync.Mutex
	var got []received
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		got = append(got, received{r.Header.Clone(), b})
		mu.Unlock()
		w.WriteHeader(status)
		io.WriteString(w, answer)
	}))
	t.Cleanup(srv.Close)
	return srv, func() []received {
		mu.Lock()
		defer mu.Unlock()
		return append([]received(nil), got...)
	}
}

func addr(s string) netip.Addr { return netip.MustParseAddr(s) }

func testHookMonitor(t *testing.T, f *fakeNet) *Monitor {
	return &Monitor{log: slog.New(slog.DiscardHandler), hooks: newHookClient(f.hookNet(t)), active: map[string]time.Time{}}
}

func TestHookValidate(t *testing.T) {
	good := []Settings{
		{DiscordURL: testDiscord},
		{DiscordURL: "https://discordapp.com/api/webhooks/1/x"},
		{DiscordURL: "https://ptb.discord.com/api/webhooks/1/x"},
		{DiscordURL: "https://canary.discord.com/api/v10/webhooks/1/x?thread_id=42"},
		{SlackURL: testSlack},
		{WebhookURL: testHook},
		{WebhookURL: "https://10.0.0.5:8443/hook"},
		{WebhookURL: "https://[2001:db8::1]/hook"},
		{WebhookURL: "http://192.168.1.10:5678/webhook/nefo", WebhookHTTP: true},
		{WebhookURL: testHook, WebhookSecret: "0123456789abcdef"},
	}
	for _, s := range good {
		if err := s.Validate(); err != nil {
			t.Errorf("refused %+v: %v", s, err)
		}
	}
	bad := []struct {
		want string
		s    Settings
	}{
		{"invalid_discord", Settings{DiscordURL: "https://evil.com/api/webhooks/1/x"}},
		{"invalid_discord", Settings{DiscordURL: "https://discord.com.evil.com/api/webhooks/1/x"}},
		{"invalid_slack", Settings{SlackURL: "https://hooks.slack.com/workflows/T/B/X"}},
		{"invalid_webhook", Settings{WebhookURL: "https://user:pass@hooks.example.net/"}},
		{"invalid_webhook", Settings{WebhookURL: "ftp://hooks.example.net/"}},
		{"invalid_webhook", Settings{WebhookURL: "https://hooks.example.net/a b"}},
		{"invalid_webhook", Settings{WebhookURL: "https://hooks.example.net/#frag"}},
		{"invalid_webhook", Settings{WebhookURL: "https:///path"}},
		{"invalid_webhook", Settings{WebhookURL: "https://bad_host.example/"}},
		{"invalid_webhook", Settings{WebhookURL: "https://hooks.example.net:99999/"}},
		{"invalid_webhook", Settings{WebhookURL: "https://" + strings.Repeat("a", 2048) + ".net/"}},
		{"invalid_webhook", Settings{WebhookURL: "https://hôte.example/"}},
		{"webhook_needs_https", Settings{WebhookURL: "http://192.168.1.10/hook"}},
		{"webhook_blocked_address", Settings{WebhookURL: "https://127.0.0.1/hook"}},
		{"invalid_webhook_secret", Settings{WebhookURL: testHook, WebhookSecret: "short"}},
		{"invalid_webhook_secret", Settings{WebhookURL: testHook, WebhookSecret: "has a space in the secret"}},
	}
	for _, c := range bad {
		if got := code(c.s.Validate()); got != c.want {
			t.Errorf("%.80v: got %q, want %q", c.s, got, c.want)
		}
	}
	for raw, want := range map[string]string{
		"https://169.254.169.254/latest/meta-data/": "webhook_blocked_address",
		"https://[::1]:8080/":                       "webhook_blocked_address",
		"https://[fe80::1%25eth0]/":                 "webhook_blocked_address",
		"https://localhost/":                        "webhook_blocked_address",
		"https://api.localhost./":                   "webhook_blocked_address",
		"https://0.0.0.0/":                          "webhook_blocked_address",
		"https://user@hooks.example.net/":           "invalid_webhook",
		"javascript:alert(1)":                       "invalid_webhook",
		"https://hooks.example.net/\x00":            "invalid_webhook",
	} {
		s := Settings{WebhookURL: raw, WebhookHTTP: true}
		if got := code(s.Validate()); got != want {
			t.Errorf("%q: got %q, want %q", raw, got, want)
		}
	}
	for _, raw := range []string{"https://discord.com/api/webhooks/1/x/../../evil", "https://discord.com:8443/api/webhooks/1/x",
		"http://discord.com/api/webhooks/1/x", "https://discord.com/api/webhooks/1/x#", "https://u@discord.com/api/webhooks/1/x"} {
		if s := (Settings{DiscordURL: raw}); s.Validate() == nil {
			t.Errorf("accepted Discord %q", raw)
		}
	}
}

func code(err error) string {
	var ue *usererr.Error
	if errors.As(err, &ue) {
		return ue.Code
	}
	if err != nil {
		return err.Error()
	}
	return ""
}

func TestBlockedAddr(t *testing.T) {
	for a, want := range map[string]bool{
		"127.0.0.1": true, "127.8.9.10": true, "::1": true, "169.254.169.254": true, "169.254.1.1": true,
		"fe80::1": true, "fd00:ec2::254": true, "100.100.100.200": true, "168.63.129.16": true, "0.0.0.0": true,
		"0.1.2.3": true, "::": true, "::ffff:127.0.0.1": true, "::ffff:169.254.169.254": true, "64:ff9b::7f00:1": true,
		"224.0.0.1": true, "ff02::1": true,
		"10.0.0.5": false, "192.168.1.10": false, "172.17.0.2": false, "fd12::1": false, "203.0.113.7": false,
		"8.8.8.8": false, "2001:db8::1": false, "64:ff9b::808:808": false,
	} {
		if got := blockedAddr(addr(a)); got != want {
			t.Errorf("blockedAddr(%s) = %v, want %v", a, got, want)
		}
	}
}

func TestDeliverPayloads(t *testing.T) {
	disc, discGot := recorder(t, http.StatusNoContent, "")
	slack, slackGot := recorder(t, http.StatusOK, "ok")
	hook, hookGot := recorder(t, http.StatusOK, "thanks")
	f := &fakeNet{
		names: map[string][]netip.Addr{"discord.com": {addr("203.0.113.1")}, "hooks.slack.com": {addr("203.0.113.2")},
			"hooks.example.net": {addr("203.0.113.3")}},
		servers: map[netip.Addr]*httptest.Server{addr("203.0.113.1"): disc, addr("203.0.113.2"): slack, addr("203.0.113.3"): hook},
	}
	m := testHookMonitor(t, f)
	secret := "a-shared-secret-of-some-length"
	s := Settings{DiscordURL: testDiscord, SlackURL: testSlack, WebhookURL: testHook, WebhookSecret: secret}
	text := "⚠️ The service <!channel> nginx is failed & @everyone"
	res := m.deliver(context.Background(), s, message{kind: "alert", subject: "x", text: text})
	for _, ch := range []string{"discord", "slack", "webhook"} {
		if !res[ch].OK {
			t.Errorf("%s: %+v", ch, res[ch])
		}
	}
	if len(res) != 3 {
		t.Errorf("results %v", res)
	}
	line := "[" + hostname() + "] " + text

	d := discGot()
	if len(d) != 1 {
		t.Fatalf("discord got %d requests", len(d))
	}
	var dp struct {
		Content         string              `json:"content"`
		AllowedMentions map[string][]string `json:"allowed_mentions"`
	}
	if err := json.Unmarshal(d[0].body, &dp); err != nil || dp.Content != line {
		t.Errorf("discord payload %s (%v)", d[0].body, err)
	}
	if p, ok := dp.AllowedMentions["parse"]; !ok || len(p) != 0 {
		t.Errorf("discord mentions not disabled: %s", d[0].body)
	}
	if d[0].header.Get("Content-Type") != "application/json" {
		t.Errorf("discord content type %q", d[0].header.Get("Content-Type"))
	}

	sl := slackGot()
	var sp map[string]string
	if len(sl) != 1 || json.Unmarshal(sl[0].body, &sp) != nil || len(sp) != 1 ||
		sp["text"] != "["+hostname()+"] ⚠️ The service &lt;!channel&gt; nginx is failed &amp; @everyone" {
		t.Errorf("slack payload %v", sl)
	}

	h := hookGot()
	if len(h) != 1 {
		t.Fatalf("webhook got %d requests", len(h))
	}
	var hp map[string]string
	if err := json.Unmarshal(h[0].body, &hp); err != nil {
		t.Fatal(err)
	}
	if len(hp) != 4 || hp["server"] != hostname() || hp["text"] != text || hp["kind"] != "alert" {
		t.Errorf("webhook payload %s", h[0].body)
	}
	if at, err := time.Parse(time.RFC3339, hp["time"]); err != nil || time.Since(at) > time.Minute {
		t.Errorf("webhook time %q", hp["time"])
	}
	if h[0].header.Get("Content-Type") != "application/json" {
		t.Errorf("webhook content type %q", h[0].header.Get("Content-Type"))
	}
	// The receiver's check, as the documentation shows it.
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(h[0].body)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if got := h[0].header.Get("X-Nefo-Signature"); !hmac.Equal([]byte(got), []byte(want)) {
		t.Errorf("signature %q, want %q", got, want)
	}

	// Without a secret, there is no signature.
	s.WebhookSecret = ""
	m.deliver(context.Background(), Settings{WebhookURL: testHook}, message{kind: "test", text: "t"})
	if h := hookGot(); len(h) != 2 || h[1].header.Get("X-Nefo-Signature") != "" || !strings.Contains(string(h[1].body), `"kind":"test"`) {
		t.Errorf("unsigned webhook %v", h)
	}
}

func TestTruncateUTF16(t *testing.T) {
	long := strings.Repeat("a🔥é", 1000) // 4 UTF-16 units each
	got := truncateUTF16(long, discordLimit)
	if n := len(utf16.Encode([]rune(got))); n > discordLimit || n < discordLimit-3 {
		t.Errorf("%d units", n)
	}
	if !utf8.ValidString(got) || !strings.HasSuffix(got, "…") || !strings.HasPrefix(long, strings.TrimSuffix(got, "…")) {
		t.Errorf("bad cut: %q", got[len(got)-20:])
	}
	if s := "short ⚠️"; truncateUTF16(s, discordLimit) != s {
		t.Error("cut a short message")
	}
	if got := truncateUTF16("abcdef", 4); got != "abc…" {
		t.Errorf("got %q", got)
	}

	// Through Discord: the content is cut, the request still goes.
	disc, discGot := recorder(t, http.StatusNoContent, "")
	f := &fakeNet{names: map[string][]netip.Addr{"discord.com": {addr("203.0.113.1")}},
		servers: map[netip.Addr]*httptest.Server{addr("203.0.113.1"): disc}}
	res := testHookMonitor(t, f).deliver(context.Background(), Settings{DiscordURL: testDiscord}, message{text: long})
	var dp struct {
		Content string `json:"content"`
	}
	if d := discGot(); !res["discord"].OK || len(d) != 1 || json.Unmarshal(d[0].body, &dp) != nil ||
		len(utf16.Encode([]rune(dp.Content))) > discordLimit {
		t.Errorf("discord long message: %+v", res)
	}
}

func TestWebhookRefusesInternalAddresses(t *testing.T) {
	hook, hookGot := recorder(t, http.StatusOK, "")
	f := &fakeNet{
		names: map[string][]netip.Addr{
			"hooks.example.net": {addr("127.0.0.1")},
			"meta.example.net":  {addr("169.254.169.254")},
			"v6.example.net":    {addr("::1"), addr("fe80::1")},
			"mixed.example.net": {addr("127.0.0.1"), addr("203.0.113.3")},
		},
		// Were the check skipped, these would reach the server.
		servers: map[netip.Addr]*httptest.Server{addr("127.0.0.1"): hook, addr("169.254.169.254"): hook,
			addr("::1"): hook, addr("203.0.113.3"): hook},
	}
	m := testHookMonitor(t, f)
	for _, raw := range []string{testHook, "https://meta.example.net/latest/meta-data/", "https://v6.example.net/"} {
		r := m.deliver(context.Background(), Settings{WebhookURL: raw}, message{kind: "test", text: "t"})["webhook"]
		if r.OK || r.Error != "webhook_blocked_address" {
			t.Errorf("%s: %+v", raw, r)
		}
	}
	if len(f.dialed) != 0 || len(hookGot()) != 0 {
		t.Fatalf("connected to a blocked address: %v", f.dialed)
	}
	// A name with a public address too uses that one only.
	r := m.deliver(context.Background(), Settings{WebhookURL: "https://mixed.example.net/"}, message{text: "t"})["webhook"]
	if !r.OK || len(f.dialed) != 1 || f.dialed[0] != "203.0.113.3:443" {
		t.Errorf("mixed: %+v, dialed %v", r, f.dialed)
	}
}

func TestOneChannelFails(t *testing.T) {
	disc, _ := recorder(t, http.StatusNotFound, `{"message": "Unknown Webhook", "code": 10015}`)
	slack, slackGot := recorder(t, http.StatusOK, "ok")
	hook, _ := recorder(t, http.StatusInternalServerError, "secret internal page\nline 2")
	other, otherGot := recorder(t, http.StatusOK, "")
	redirect := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://other.example.net/", http.StatusFound)
	}))
	defer redirect.Close()
	f := &fakeNet{
		names: map[string][]netip.Addr{"discord.com": {addr("203.0.113.1")}, "hooks.slack.com": {addr("203.0.113.2")},
			"hooks.example.net": {addr("203.0.113.3")}, "redirect.example.net": {addr("203.0.113.4")},
			"other.example.net": {addr("203.0.113.5")}},
		servers: map[netip.Addr]*httptest.Server{addr("203.0.113.1"): disc, addr("203.0.113.2"): slack,
			addr("203.0.113.3"): hook, addr("203.0.113.4"): redirect, addr("203.0.113.5"): other},
	}
	m := testHookMonitor(t, f)
	res := m.deliver(context.Background(), Settings{DiscordURL: testDiscord, SlackURL: testSlack, WebhookURL: testHook},
		message{kind: "alert", text: "down"})
	if r := res["discord"]; r.OK || r.Error != "webhook_refused" || r.Detail != "HTTP 404, Unknown Webhook" {
		t.Errorf("discord %+v", r)
	}
	if r := res["slack"]; !r.OK || len(slackGot()) != 1 {
		t.Errorf("slack %+v", r)
	}
	// The generic webhook's answer is not repeated: only its status.
	if r := res["webhook"]; r.OK || r.Error != "webhook_refused" || r.Detail != "HTTP 500" {
		t.Errorf("webhook %+v", r)
	}

	// Redirects are not followed.
	r := m.deliver(context.Background(), Settings{WebhookURL: "https://redirect.example.net/x"}, message{text: "t"})["webhook"]
	if r.OK || r.Detail != "HTTP 302" || len(otherGot()) != 0 {
		t.Errorf("redirect %+v", r)
	}

	// An unknown name and a refused connection fail without the address.
	for _, raw := range []string{"https://nowhere.example.net/token123", "https://hooks.example.org/token123"} {
		f.names["hooks.example.org"] = []netip.Addr{addr("203.0.113.99")}
		r := m.deliver(context.Background(), Settings{WebhookURL: raw}, message{text: "t"})["webhook"]
		if r.OK || r.Error != "webhook_unreachable" || strings.Contains(r.Detail, "token123") || r.Detail == "" {
			t.Errorf("%s: %+v", raw, r)
		}
		t.Logf("%s: %s", raw, r.Detail)
	}

	// A receiver that does not answer in time.
	release := make(chan struct{})
	slow := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release }))
	defer slow.Close()
	defer close(release)
	f.names["slow.example.net"] = []netip.Addr{addr("203.0.113.6")}
	f.servers[addr("203.0.113.6")] = slow
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	res = m.deliver(ctx, Settings{SlackURL: testSlack, WebhookURL: "https://slow.example.net/"}, message{text: "t"})
	if r := res["webhook"]; r.OK || r.Error != "webhook_timeout" {
		t.Errorf("slow: %+v", r)
	}
	if !res["slack"].OK {
		t.Errorf("slack held up by the slow webhook: %+v", res["slack"])
	}
}

func TestSaveMasksAndKeepsHooks(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	m := New(st, nil, slog.New(slog.DiscardHandler), nil)
	ctx := context.Background()
	secret := "0123456789abcdef-secret"
	if err := m.Save(ctx, Settings{DiscordURL: " " + testDiscord + "\n", SlackURL: testSlack, WebhookURL: testHook,
		WebhookSecret: secret, CPUPercent: 90}); err != nil {
		t.Fatal(err)
	}
	shown, err := m.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(shown)
	for _, leak := range []string{"abc_DEF", "XXXXyyyyZZZZ", "hooks.example.net", secret} {
		if strings.Contains(string(b), leak) {
			t.Errorf("Settings shows %q: %s", leak, b)
		}
	}
	if shown.DiscordURL != mask("x") || shown.SlackURL != mask("x") || shown.WebhookURL != mask("x") || shown.WebhookSecret != mask("x") {
		t.Errorf("not masked: %+v", shown)
	}
	if !shown.HasChannel() {
		t.Error("no channel")
	}

	// Saving the form as shown keeps every address and the secret.
	if err := m.Save(ctx, shown); err != nil {
		t.Fatal(err)
	}
	kept, _ := m.settings(ctx)
	if kept.DiscordURL != testDiscord || kept.SlackURL != testSlack || kept.WebhookURL != testHook || kept.WebhookSecret != secret {
		t.Errorf("not kept: %+v", kept)
	}

	// Clearing an address removes the channel; clearing the webhook's
	// address removes its secret and the HTTP permission too.
	shown.SlackURL, shown.WebhookURL, shown.WebhookHTTP = "", "", true
	if err := m.Save(ctx, shown); err != nil {
		t.Fatal(err)
	}
	kept, _ = m.settings(ctx)
	if kept.DiscordURL != testDiscord || kept.SlackURL != "" || kept.WebhookURL != "" || kept.WebhookSecret != "" || kept.WebhookHTTP {
		t.Errorf("after clearing: %+v", kept)
	}

	// A bad address is refused and nothing changes.
	if err := m.Save(ctx, Settings{DiscordURL: "https://example.com/api/webhooks/1/x"}); code(err) != "invalid_discord" {
		t.Errorf("saved a bad address: %v", err)
	}
	if kept, _ = m.settings(ctx); kept.DiscordURL != testDiscord {
		t.Errorf("changed after a refusal: %+v", kept)
	}
}
