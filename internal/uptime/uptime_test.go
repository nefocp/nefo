package uptime

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"log"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/store"
)

// localChecker sends every connection to srv, whatever the name, and
// trusts roots: the tests never use DNS or the internet.
func localChecker(srv *httptest.Server, roots *x509.CertPool, timeout time.Duration) *Checker {
	addr := srv.Listener.Addr().String()
	dial := func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}
	return newChecker(dial, roots, timeout)
}

var target = Target{URL: "http://example.com/", Names: []string{"example.com", "www.example.com"}}

func TestCheck(t *testing.T) {
	var code atomic.Int32
	code.Store(200)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.UserAgent() != UserAgent {
			t.Errorf("User-Agent %q", r.UserAgent())
		}
		w.WriteHeader(int(code.Load()))
		io.WriteString(w, "<h1>Welcome to Example</h1>")
	}))
	defer srv.Close()
	c := localChecker(srv, nil, 2*time.Second)
	ctx := context.Background()

	if r := c.Check(ctx, target); !r.OK || r.Code != 200 || r.Error != "" || !r.CertNotAfter.IsZero() {
		t.Fatalf("up: %+v", r)
	}
	code.Store(502)
	if r := c.Check(ctx, target); r.OK || r.Code != 502 || r.Error != "HTTP 502" {
		t.Fatalf("502: %+v", r)
	}
	// 4xx: up but noted, unless it counts as down.
	code.Store(404)
	if r := c.Check(ctx, target); !r.OK || r.Code != 404 {
		t.Fatalf("404: %+v", r)
	}
	strict := target
	strict.Strict4xx = true
	if r := c.Check(ctx, strict); r.OK || r.Error != "HTTP 404" {
		t.Fatalf("404, strict: %+v", r)
	}

	// The keyword, in any case.
	code.Store(200)
	kw := target
	kw.Keyword = "welcome TO"
	if r := c.Check(ctx, kw); !r.OK {
		t.Fatalf("keyword found: %+v", r)
	}
	kw.Keyword = "Dashboard"
	if r := c.Check(ctx, kw); r.OK || r.Code != 200 || r.Error != `"Dashboard" is not on the page` {
		t.Fatalf("keyword missing: %+v", r)
	}
}

func TestCheckTimeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer srv.Close()
	defer close(release)
	start := time.Now()
	r := localChecker(srv, nil, 300*time.Millisecond).Check(context.Background(), target)
	if r.OK || !strings.HasPrefix(r.Error, "no answer within") {
		t.Fatalf("timeout: %+v", r)
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("took %v", d)
	}
}

func TestCheckRedirects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Host == "www.example.com":
			io.WriteString(w, "home")
		case r.URL.Path == "/":
			http.Redirect(w, r, "http://www.example.com/", http.StatusMovedPermanently)
		case r.URL.Path == "/away":
			http.Redirect(w, r, "https://elsewhere.example/", http.StatusFound)
		default:
			http.Redirect(w, r, "/loop", http.StatusFound)
		}
	}))
	defer srv.Close()
	c := localChecker(srv, nil, 2*time.Second)
	ctx := context.Background()

	// To one of the website's own names: followed.
	if r := c.Check(ctx, target); !r.OK || r.Code != 200 {
		t.Fatalf("own name: %+v", r)
	}
	// Elsewhere: not followed, the redirect is the answer.
	away := target
	away.URL = "http://example.com/away"
	if r := c.Check(ctx, away); !r.OK || r.Code != 302 {
		t.Fatalf("elsewhere: %+v", r)
	}
	loop := target
	loop.URL = "http://example.com/loop"
	if r := c.Check(ctx, loop); r.OK || r.Error != "too many redirects" {
		t.Fatalf("loop: %+v", r)
	}
}

// testCert makes a self-signed certificate for example.com that ends at
// notAfter, and a pool that trusts it.
func testCert(t *testing.T, notAfter time.Time) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: "example.com"},
		DNSNames:              []string{"example.com", "www.example.com"},
		NotBefore:             notAfter.Add(-90 * 24 * time.Hour),
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, pool
}

func tlsServer(t *testing.T, cert tls.Certificate) *httptest.Server {
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	srv.Config.ErrorLog = log.New(io.Discard, "", 0) // the refused handshakes are expected
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv
}

func TestCheckCertificate(t *testing.T) {
	ctx := context.Background()
	https := target
	https.URL = "https://example.com/"

	ends := time.Now().Add(5 * 24 * time.Hour).Truncate(time.Second)
	cert, pool := testCert(t, ends)
	srv := tlsServer(t, cert)
	r := localChecker(srv, pool, 2*time.Second).Check(ctx, https)
	if !r.OK || !r.CertNotAfter.Equal(ends) {
		t.Fatalf("valid: %+v", r)
	}
	// Not trusted.
	if r := localChecker(srv, x509.NewCertPool(), 2*time.Second).Check(ctx, https); r.OK || !strings.Contains(r.Error, "not trusted") {
		t.Fatalf("untrusted: %+v", r)
	}
	// For another name.
	other := https
	other.URL = "https://example.net/"
	if r := localChecker(srv, pool, 2*time.Second).Check(ctx, other); r.OK || r.Error != "the certificate is not for this domain" {
		t.Fatalf("other name: %+v", r)
	}

	expired, pool2 := testCert(t, time.Now().Add(-time.Hour))
	srv2 := tlsServer(t, expired)
	if r := localChecker(srv2, pool2, 2*time.Second).Check(ctx, https); r.OK || !strings.Contains(r.Error, "expired") {
		t.Fatalf("expired: %+v", r)
	}
}

func TestDescribe(t *testing.T) {
	wrap := func(err error) error { return &url.Error{Op: "Get", URL: "https://example.com/", Err: err} }
	cases := map[string]error{
		"DNS: the domain does not resolve": wrap(&net.OpError{Op: "dial", Err: &net.DNSError{Err: "no such host", Name: "example.com", IsNotFound: true}}),
		"DNS lookup failed":                wrap(&net.DNSError{Err: "server misbehaving", Name: "example.com"}),
		"connection refused":               wrap(errors.New("dial tcp 1.2.3.4:443: connect: connection refused")),
		"no answer within 10 seconds":      wrap(context.DeadlineExceeded),
		"something else":                   wrap(errors.New("something else")),
	}
	for want, err := range cases {
		if got := describe(err, 10*time.Second); got != want {
			t.Errorf("describe(%v) = %q, want %q", err, got, want)
		}
	}
}

func TestSettingsValidate(t *testing.T) {
	s := Settings{Enabled: true}
	if err := s.Validate(); err != nil || s.Interval != 5 || s.Path != "/" {
		t.Fatalf("defaults: %+v, %v", s, err)
	}
	for _, ok := range []Settings{{Interval: 1, Path: "/health?full=1"}, {Interval: 10, Path: "/a/b-c_d.php", Keyword: "Xin chào"}} {
		if err := ok.Validate(); err != nil {
			t.Errorf("%+v: %v", ok, err)
		}
	}
	for _, bad := range []Settings{{Interval: 3}, {Path: "health"}, {Path: "/a b"}, {Path: "//evil.example/"}, {Path: "/#x"},
		{Keyword: "a\nb"}, {Keyword: strings.Repeat("x", 201)}, {Path: "/" + strings.Repeat("x", 300)}} {
		if err := bad.Validate(); err == nil {
			t.Errorf("%+v: accepted", bad)
		}
	}
}

func TestHuman(t *testing.T) {
	for d, want := range map[time.Duration]string{
		20 * time.Second:                "less than a minute",
		time.Minute:                     "1 minute",
		12*time.Minute + 10*time.Second: "12 minutes",
		time.Hour:                       "1 hour",
		3*time.Hour + 5*time.Minute:     "3 hours 5 minutes",
		50 * time.Hour:                  "2 days 2 hours",
	} {
		if got := human(d); got != want {
			t.Errorf("human(%v) = %q, want %q", d, got, want)
		}
	}
}

// ---- the service ----

type harness struct {
	st    *store.Store
	site  *store.Site
	code  atomic.Int32
	clock time.Time
	mu    sync.Mutex
	msgs  []string
}

func newHarness(t *testing.T, ssl bool) (*harness, *httptest.Server) {
	// Whole seconds, as the database keeps them.
	h := &harness{clock: time.Now().Truncate(time.Second)}
	h.code.Store(200)
	var err error
	if h.st, err = store.Open(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.st.Close() })
	h.site = &store.Site{Domain: "example.com", Aliases: []string{"www.example.com"}, SysUser: "example_com"}
	ctx := context.Background()
	if err := h.st.CreateSite(ctx, h.site); err != nil {
		t.Fatal(err)
	}
	h.site.SSL = ssl
	if err := h.st.UpdateSite(ctx, h.site); err != nil {
		t.Fatal(err)
	}
	if err := h.st.SaveSiteMonitor(ctx, &store.SiteMonitor{SiteID: h.site.ID, Enabled: true, Interval: 5, Path: "/"}); err != nil {
		t.Fatal(err)
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(int(h.code.Load())) })
	if ssl {
		return h, nil
	}
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return h, srv
}

func (h *harness) service(c *Checker) *Service {
	s := New(h.st, func(_ context.Context, text string) {
		h.mu.Lock()
		h.msgs = append(h.msgs, text)
		h.mu.Unlock()
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.check = c
	s.now = func() time.Time {
		h.mu.Lock()
		defer h.mu.Unlock()
		return h.clock
	}
	return s
}

func (h *harness) advance(d time.Duration) {
	h.mu.Lock()
	h.clock = h.clock.Add(d)
	h.mu.Unlock()
}

// sent returns the alerts sent since the last call.
func (h *harness) sent() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	m := h.msgs
	h.msgs = nil
	return m
}

func TestDownAndRecovery(t *testing.T) {
	ctx := context.Background()
	h, srv := newHarness(t, false)
	s := h.service(localChecker(srv, nil, 2*time.Second))
	check := func(code int) Status {
		t.Helper()
		h.code.Store(int32(code))
		s.CheckNow(ctx, h.site.ID)
		st, ok := s.Status(h.site.ID)
		if !ok {
			t.Fatal("not monitored")
		}
		return st
	}

	if st, _ := (func() (Status, bool) { s.Reload(ctx); return s.Status(h.site.ID) })(); st.State != "pending" {
		t.Fatalf("before the first check: %+v", st)
	}
	if st := check(200); st.State != "up" || st.Last == nil || st.Last.Code != 200 {
		t.Fatalf("up: %+v", st)
	}
	// One failure is not enough.
	h.advance(5 * time.Minute)
	first := h.clock
	if st := check(502); st.State != "up" || st.Fails != 1 || len(h.sent()) != 0 {
		t.Fatalf("one failure: %+v", st)
	}
	h.advance(5 * time.Minute)
	if st := check(502); st.State != "down" || st.Since == nil || !st.Since.Equal(first) {
		t.Fatalf("two failures: %+v", st)
	}
	if m := h.sent(); len(m) != 1 || m[0] != "🔴 example.com is down (HTTP 502, 2 checks in a row)." {
		t.Fatalf("down alert: %q", m)
	}
	// Not again for 6 hours.
	h.advance(time.Hour)
	check(503)
	if m := h.sent(); len(m) != 0 {
		t.Fatalf("repeated too soon: %q", m)
	}
	h.advance(6 * time.Hour)
	check(503)
	if m := h.sent(); len(m) != 1 || m[0] != "🔴 example.com is still down (HTTP 503), for 7 hours 5 minutes now." {
		t.Fatalf("repeat: %q", m)
	}

	// A restart of the panel remembers that it is down, and does not repeat the alert.
	s = h.service(localChecker(srv, nil, 2*time.Second))
	h.advance(7 * time.Minute)
	if st := check(500); st.State != "down" || st.Fails != 5 || !st.Since.Equal(first) || len(h.sent()) != 0 {
		t.Fatalf("after a restart: %+v", st)
	}
	h.advance(time.Minute)
	if st := check(200); st.State != "up" || st.Since != nil || st.Fails != 0 {
		t.Fatalf("recovered: %+v", st)
	}
	if m := h.sent(); len(m) != 1 || m[0] != "✅ example.com is up again after 7 hours 13 minutes." {
		t.Fatalf("recovery alert: %q", m)
	}

	// Everything was recorded.
	if n, _, _ := h.st.UptimeStreak(ctx, h.site.ID); n != 0 {
		t.Fatalf("streak %d", n)
	}
	day, _ := h.st.UptimeStats(ctx, h.site.ID, time.Time{})
	if day.Checks != 7 || day.Up != 2 {
		t.Fatalf("stats: %+v", day)
	}

	// Monitoring off, or the website suspended: not watched.
	h.site.Disabled = true
	h.st.UpdateSite(ctx, h.site)
	s.Reload(ctx)
	if _, ok := s.Status(h.site.ID); ok || len(s.Summary()) != 0 {
		t.Fatal("a suspended website is watched")
	}
}

// Failures from before a long pause (the monitoring was off) are not
// picked up again: the website starts afresh.
func TestStaleHistory(t *testing.T) {
	ctx := context.Background()
	h, srv := newHarness(t, false)
	for _, ago := range []time.Duration{3 * time.Hour, 2 * time.Hour} {
		if err := h.st.AddUptimeCheck(ctx, store.UptimeCheck{SiteID: h.site.ID, Time: h.clock.Add(-ago), Code: 502, Error: "HTTP 502"}); err != nil {
			t.Fatal(err)
		}
	}
	s := h.service(localChecker(srv, nil, 2*time.Second))
	s.Reload(ctx)
	if st, _ := s.Status(h.site.ID); st.State != "pending" || st.Fails != 0 {
		t.Fatalf("status: %+v", st)
	}
	s.CheckNow(ctx, h.site.ID)
	if st, _ := s.Status(h.site.ID); st.State != "up" || len(h.sent()) != 0 {
		t.Fatalf("after a check: %+v", st)
	}
}

// A check that ends after the monitoring was turned off sends nothing.
func TestNoAlertWhenTurnedOff(t *testing.T) {
	ctx := context.Background()
	h, srv := newHarness(t, false)
	s := h.service(localChecker(srv, nil, 2*time.Second))
	h.code.Store(502)
	s.CheckNow(ctx, h.site.ID)
	h.advance(5 * time.Minute)
	s.mu.Lock()
	w := s.watches[h.site.ID]
	w.running = true
	s.mu.Unlock()
	if err := h.st.SaveSiteMonitor(ctx, &store.SiteMonitor{SiteID: h.site.ID, Interval: 5, Path: "/"}); err != nil {
		t.Fatal(err)
	}
	s.run(ctx, w)
	if m := h.sent(); len(m) != 0 {
		t.Fatalf("sent: %q", m)
	}
}

func TestCertificateAlert(t *testing.T) {
	ctx := context.Background()
	h, _ := newHarness(t, true)
	ends := time.Now().Add(5*24*time.Hour + time.Hour).Truncate(time.Second)
	cert, pool := testCert(t, ends)
	srv := tlsServer(t, cert)
	s := h.service(localChecker(srv, pool, 2*time.Second))

	s.CheckNow(ctx, h.site.ID)
	m := h.sent()
	if len(m) != 1 || !strings.HasPrefix(m[0], "⚠️ The certificate that example.com serves ends in 5 days (") {
		t.Fatalf("alert: %q", m)
	}
	st, _ := s.Status(h.site.ID)
	if st.State != "up" || st.CertExpires == nil || !st.CertExpires.Equal(ends) {
		t.Fatalf("status: %+v", st)
	}
	saved, _ := h.st.SiteMonitor(ctx, h.site.ID)
	if !saved.CertNotAfter.Equal(ends) || !saved.CertWarnedAt.Equal(h.clock.Truncate(time.Second)) {
		t.Fatalf("saved: %+v", saved)
	}
	// Once a day at most, also after a restart.
	h.advance(5 * time.Minute)
	s.CheckNow(ctx, h.site.ID)
	s = h.service(localChecker(srv, pool, 2*time.Second))
	h.advance(5 * time.Minute)
	s.CheckNow(ctx, h.site.ID)
	if m := h.sent(); len(m) != 0 {
		t.Fatalf("repeated: %q", m)
	}
	h.advance(24 * time.Hour)
	s.CheckNow(ctx, h.site.ID)
	if m := h.sent(); len(m) != 1 || !strings.Contains(m[0], "ends in 4 days") {
		t.Fatalf("next day: %q", m)
	}
}

func TestRunLoop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h, srv := newHarness(t, false)
	// A check from 8 days ago, pruned by the loop.
	if err := h.st.AddUptimeCheck(ctx, store.UptimeCheck{SiteID: h.site.ID, Time: h.clock.Add(-8 * 24 * time.Hour), OK: true, Code: 200}); err != nil {
		t.Fatal(err)
	}
	s := h.service(localChecker(srv, nil, 2*time.Second))
	s.tick = 10 * time.Millisecond
	s.Reload(ctx)
	s.mu.Lock()
	s.watches[h.site.ID].next = time.Time{} // due now, not at its turn
	s.mu.Unlock()
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		all, _ := h.st.UptimeChecks(ctx, h.site.ID, time.Time{}, 10, false)
		if len(all) == 1 && all[0].Time.After(h.clock.Add(-time.Minute)) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("checks: %+v", all)
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	<-done
	if st, _ := s.Status(h.site.ID); st.State != "up" {
		t.Fatalf("status: %+v", st)
	}
}

func TestSpread(t *testing.T) {
	now := time.Now()
	seen := map[time.Time]bool{}
	for id := int64(1); id <= 6; id++ {
		at := spread(now, id, 5)
		if at.Before(now) || !at.Before(now.Add(5*time.Minute)) {
			t.Fatalf("site %d at %v", id, at.Sub(now))
		}
		seen[at] = true
	}
	if len(seen) != 6 {
		t.Fatalf("websites share a time: %d", len(seen))
	}
}
