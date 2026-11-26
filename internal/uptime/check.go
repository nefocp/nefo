// Package uptime checks that the websites answer, as a visitor would see
// them, and sends an alert when one goes down, comes back, or serves a
// certificate that ends soon.
package uptime

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// UserAgent is sent with every check, so the checks are easy to tell apart
// (and to leave out) in the access logs.
const UserAgent = "Nefo-Monitor/1.0"

const (
	checkTimeout = 10 * time.Second
	maxBody      = 256 << 10 // read for the keyword, at most
	maxRedirects = 5
)

// Intervals are the check intervals to choose from, in minutes.
var Intervals = []int{1, 2, 5, 10}

// Settings are the monitoring settings of a website.
type Settings struct {
	Enabled   bool   `json:"enabled"`
	Interval  int    `json:"interval"` // minutes
	Path      string `json:"path"`
	Keyword   string `json:"keyword"`
	Strict4xx bool   `json:"strict_4xx"` // a 4xx answer counts as down
}

// Validate checks the settings and fills in the defaults.
func (s *Settings) Validate() error {
	if s.Interval == 0 {
		s.Interval = 5
	}
	if !slices.Contains(Intervals, s.Interval) {
		return usererr.New("invalid_monitor_interval")
	}
	s.Path = strings.TrimSpace(s.Path)
	if s.Path == "" {
		s.Path = "/"
	}
	if len(s.Path) > 300 || s.Path[0] != '/' || strings.ContainsAny(s.Path, "#\\") || strings.HasPrefix(s.Path, "//") {
		return usererr.New("invalid_monitor_path")
	}
	for _, c := range s.Path {
		if c <= ' ' || c > '~' {
			return usererr.New("invalid_monitor_path")
		}
	}
	if _, err := url.ParseRequestURI(s.Path); err != nil {
		return usererr.New("invalid_monitor_path")
	}
	s.Keyword = strings.TrimSpace(s.Keyword)
	if utf8.RuneCountInString(s.Keyword) > 200 || !utf8.ValidString(s.Keyword) ||
		strings.ContainsFunc(s.Keyword, unicode.IsControl) {
		return usererr.New("invalid_monitor_keyword")
	}
	return nil
}

// Target is what one check asks for.
type Target struct {
	URL       string
	Names     []string // the website's own names: redirects to them are followed
	Keyword   string
	Strict4xx bool
}

// targetFor is the website's public address: https:// when it has a
// certificate, through DNS like any visitor.
func targetFor(site *store.Site, m store.SiteMonitor) Target {
	scheme := "http"
	if site.SSL {
		scheme = "https"
	}
	path := m.Path
	if path == "" {
		path = "/"
	}
	names := []string{}
	for _, n := range site.Names() {
		names = append(names, strings.ToLower(n))
	}
	return Target{URL: scheme + "://" + site.Domain + path, Names: names, Keyword: m.Keyword, Strict4xx: m.Strict4xx}
}

// Result is the outcome of a check, with the expiry of the certificate the
// website served (zero over plain HTTP or when the connection failed).
type Result struct {
	store.UptimeCheck
	CertNotAfter time.Time
}

// Checker runs checks. It keeps no connections open between them.
type Checker struct {
	client  *http.Client
	timeout time.Duration
}

// NewChecker returns a Checker that connects like a visitor: the system's
// DNS resolver and certificate authorities, no proxy.
func NewChecker() *Checker { return newChecker(nil, nil, checkTimeout) }

// newChecker lets tests send every connection to a local server and trust
// their own certificates.
func newChecker(dial func(ctx context.Context, network, addr string) (net.Conn, error), roots *x509.CertPool, timeout time.Duration) *Checker {
	if dial == nil {
		dial = (&net.Dialer{Timeout: timeout}).DialContext
	}
	tr := &http.Transport{
		Proxy:                  nil,
		DialContext:            dial,
		TLSClientConfig:        &tls.Config{RootCAs: roots},
		TLSHandshakeTimeout:    timeout,
		ResponseHeaderTimeout:  timeout,
		DisableKeepAlives:      true,
		ForceAttemptHTTP2:      true,
		MaxResponseHeaderBytes: 64 << 10,
	}
	return &Checker{timeout: timeout, client: &http.Client{Transport: tr, CheckRedirect: checkRedirect}}
}

type namesKey struct{}

var errRedirectLoop = errors.New("too many redirects")

// checkRedirect follows redirects to the website's own names (such as
// http:// to https://, or to www.) and stops at the first one elsewhere,
// whose answer then counts.
func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return errRedirectLoop
	}
	names, _ := req.Context().Value(namesKey{}).([]string)
	if !slices.Contains(names, strings.ToLower(req.URL.Hostname())) {
		return http.ErrUseLastResponse
	}
	return nil
}

// Check asks for the target once. A check fails when no answer comes
// (DNS, connection, certificate, timeout), on an HTTP status of 500 or more
// (or 4xx with Strict4xx), or when the keyword is not on the page. The
// response time is the time until the answer's headers arrived, connection
// included.
func (c *Checker) Check(ctx context.Context, t Target) Result {
	start := time.Now()
	r := Result{UptimeCheck: store.UptimeCheck{Time: start}}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(context.WithValue(ctx, namesKey{}, t.Names), http.MethodGet, t.URL, nil)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,*/*;q=0.8")
	req.Header.Set("Cache-Control", "no-cache")
	resp, err := c.client.Do(req)
	r.MS = int(time.Since(start) / time.Millisecond)
	if err != nil {
		r.Error = describe(err, c.timeout)
		return r
	}
	defer resp.Body.Close()
	r.Code = resp.StatusCode
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		r.CertNotAfter = resp.TLS.PeerCertificates[0].NotAfter
	}
	switch {
	case r.Code >= 500, r.Code >= 400 && t.Strict4xx:
		r.Error = fmt.Sprintf("HTTP %d", r.Code)
	case t.Keyword != "":
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
		if err != nil {
			r.Error = "the page did not load completely: " + describe(err, c.timeout)
		} else if !bytes.Contains(bytes.ToLower(body), bytes.ToLower([]byte(t.Keyword))) {
			r.Error = fmt.Sprintf("%q is not on the page", t.Keyword)
		}
	}
	r.OK = r.Error == ""
	return r
}

// describe turns a connection error into a short English reason, the
// one alerts show.
func describe(err error, timeout time.Duration) string {
	var (
		dnsErr   *net.DNSError
		invalid  x509.CertificateInvalidError
		hostErr  x509.HostnameError
		authErr  x509.UnknownAuthorityError
		netErr   net.Error
		recordEr tls.RecordHeaderError
	)
	switch {
	case errors.Is(err, errRedirectLoop):
		return "too many redirects"
	case errors.As(err, &dnsErr):
		if dnsErr.IsNotFound {
			return "DNS: the domain does not resolve"
		}
		if dnsErr.IsTimeout {
			return "DNS: no answer"
		}
		return "DNS lookup failed"
	case errors.As(err, &invalid):
		if invalid.Reason == x509.Expired {
			return "the certificate has expired or is not valid yet"
		}
		return "the certificate is not valid"
	case errors.As(err, &hostErr):
		return "the certificate is not for this domain"
	case errors.As(err, &authErr):
		return "the certificate is not trusted (self-signed or unknown issuer)"
	case errors.As(err, &recordEr):
		return "the server does not speak HTTPS on port 443"
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		return fmt.Sprintf("no answer within %d seconds", int(timeout/time.Second))
	case errors.Is(err, syscall.ECONNREFUSED), strings.Contains(err.Error(), "connection refused"),
		strings.Contains(err.Error(), "actively refused"):
		return "connection refused"
	case errors.Is(err, syscall.ECONNRESET), errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return "the connection was closed"
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	s := err.Error()
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}
