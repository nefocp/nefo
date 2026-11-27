package hosting

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/validate"
)

// Reverse proxy websites forward every request to an address the
// administrator gives: another server, or a local service the panel does
// not run (http(s)://host:port[/path]). The site's domain, HTTPS, access
// options and firewall work as for other sites; the upstream gets the
// request with the path prefix added, the Host header chosen (the site's
// domain or the upstream's), the X-Forwarded-* headers and WebSocket
// upgrades if asked.

// DefaultProxyOptions are the options of a new reverse proxy site.
func DefaultProxyOptions() store.ProxyOptions {
	return store.ProxyOptions{WebSocket: true, Forwarded: true, Timeout: 60}
}

const (
	minProxyTimeout = 5
	maxProxyTimeout = 3600
	// caFile holds the certificate authorities Nginx and Apache check an
	// https:// upstream's certificate against (package ca-certificates).
	caFile = "/etc/ssl/certs/ca-certificates.crt"
)

// upstream is a checked upstream address.
type upstream struct {
	Scheme string
	Host   string // a lower-case name, or an IP address (without brackets)
	Port   int
	Path   string // "" or "/a/b", without a trailing slash
	IP     net.IP // nil for a name
}

var (
	upstreamNameRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$`)
	upstreamPathRe = regexp.MustCompile(`^(/[A-Za-z0-9._~%-]+)*/?$`)
)

// parseUpstream checks an upstream address: http or https, a host name or
// an IP address, an optional port and path, nothing else (no user, query
// or fragment).
func parseUpstream(raw string) (*upstream, error) {
	raw = strings.TrimSpace(raw)
	bad := inputErr("invalid_proxy_url", raw)
	if raw == "" || len(raw) > 500 || strings.ContainsAny(raw, " \t\r\n\\\"'`$;{}") {
		return nil, bad
	}
	u, err := url.Parse(raw)
	if err != nil || u.Opaque != "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(raw, "#") {
		return nil, bad
	}
	up := &upstream{Scheme: strings.ToLower(u.Scheme), Host: strings.ToLower(u.Hostname())}
	switch up.Scheme {
	case "http":
		up.Port = 80
	case "https":
		up.Port = 443
	default:
		return nil, bad
	}
	if p := u.Port(); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 || strings.HasPrefix(p, "0") {
			return nil, bad
		}
		up.Port = n
	} else if strings.HasSuffix(u.Host, ":") {
		return nil, bad
	}
	if ip := net.ParseIP(up.Host); ip != nil {
		if ip.IsUnspecified() || ip.IsMulticast() || strings.Contains(up.Host, "%") {
			return nil, bad
		}
		up.IP = ip
		if v4 := ip.To4(); v4 != nil {
			up.IP, up.Host = v4, v4.String()
		} else {
			up.Host = ip.String()
		}
	} else if len(up.Host) > 253 || !upstreamNameRe.MatchString(up.Host) || strings.Contains(u.Host, "[") {
		return nil, bad
	}
	p := u.EscapedPath()
	if !upstreamPathRe.MatchString(p) {
		return nil, bad
	}
	if p = strings.TrimSuffix(p, "/"); p != "" && path.Clean(p) != p {
		return nil, bad // "/a/../b", "/./a"
	}
	up.Path = p
	return up, nil
}

// hostPort is host[:port] as a Host header or an address: IPv6 addresses
// in brackets, the port left out when it is the scheme's.
func (u *upstream) hostPort(always bool) string {
	h := u.Host
	if u.IP != nil && u.IP.To4() == nil {
		h = "[" + h + "]"
	}
	if !always && ((u.Scheme == "http" && u.Port == 80) || (u.Scheme == "https" && u.Port == 443)) {
		return h
	}
	return h + ":" + strconv.Itoa(u.Port)
}

// String is the upstream as stored: always with its port.
func (u *upstream) String() string { return u.Scheme + "://" + u.hostPort(true) + u.Path }

// loopback reports whether the upstream is this server itself.
func (m *Manager) upstreamIsLocal(ctx context.Context, u *upstream) (bool, error) {
	ips := []net.IP{u.IP}
	if u.IP == nil {
		if u.Host == "localhost" || strings.HasSuffix(u.Host, ".localhost") {
			return true, nil
		}
		lookup := m.lookupHost
		if lookup == nil {
			lookup = func(ctx context.Context, host string) ([]net.IP, error) {
				ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()
				addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
				out := make([]net.IP, 0, len(addrs))
				for _, a := range addrs {
					out = append(out, a.IP)
				}
				return out, err
			}
		}
		var err error
		if ips, err = lookup(ctx, u.Host); err != nil || len(ips) == 0 {
			return false, inputErr("proxy_unknown_host", u.Host)
		}
	}
	local := m.localAddrs
	if local == nil {
		local = func() []net.IP {
			addrs, _ := net.InterfaceAddrs()
			var out []net.IP
			for _, a := range addrs {
				if n, ok := a.(*net.IPNet); ok {
					out = append(out, n.IP)
				}
			}
			return out
		}
	}
	mine := local()
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsUnspecified() || slices.ContainsFunc(mine, ip.Equal) {
			return true, nil
		}
	}
	return false, nil
}

// checkProxy checks an upstream and its options for a site and returns
// the address as stored. The panel's own ports (its web interface, the
// Git webhook listener) and this server's web server itself (ports 80 and
// 443 here: the site would forward to itself) are refused.
func (m *Manager) checkProxy(ctx context.Context, raw string, o *store.ProxyOptions) (string, error) {
	u, err := parseUpstream(raw)
	if err != nil {
		return "", err
	}
	if o.Timeout == 0 {
		o.Timeout = DefaultProxyOptions().Timeout
	}
	if o.Timeout < minProxyTimeout || o.Timeout > maxProxyTimeout {
		return "", inputErr("invalid_proxy_timeout", strconv.Itoa(o.Timeout))
	}
	if u.Scheme != "https" {
		o.Insecure = false
	}
	local, err := m.upstreamIsLocal(ctx, u)
	if err != nil {
		return "", err
	}
	if local && (slices.Contains(m.reserved, u.Port) || u.Port == 7809) {
		return "", inputErr("proxy_upstream_panel", u.hostPort(true))
	}
	if local && (u.Port == 80 || u.Port == 443) {
		return "", inputErr("proxy_upstream_loop", u.hostPort(true))
	}
	return u.String(), nil
}

// proxyVhost is what the templates need of a reverse proxy site.
type proxyVhost struct {
	// URL is scheme://host:port[/path], the base proxy_pass and ProxyPass
	// add the request's path to.
	URL   string
	HTTPS bool
	Path  string
	// Name is the upstream's host (an IPv6 address in brackets), for TLS.
	Name string
	// HostHeader is the upstream's host[:port] when it is sent as the Host
	// header; "" sends the site's domain.
	HostHeader string
	// RedirectFrom are the upstream's own addresses in Location headers,
	// which Nginx changes to the site's (with the upstream's Host header).
	RedirectFrom []string
	WebSocket    bool
	Forwarded    bool
	Insecure     bool
	Timeout      int
	// ConnectTimeout is at most 30 seconds.
	ConnectTimeout int
	// Resolver is the DNS servers Nginx looks a host name up with, "" for
	// an IP address.
	Resolver string
	// OLSAddress is OpenLiteSpeed's proxy address (host:port, with
	// https:// for TLS). OLSProxyHost is the Host header OpenLiteSpeed sends
	// instead of the site's (its rewrite flag Proxy-Host, which cannot hold
	// an IPv6 address in brackets).
	OLSAddress   string
	OLSProxyHost string
}

var (
	proxyURLRe    = regexp.MustCompile(`^https?://(\[[0-9a-f:.]+\]|[a-z0-9.-]+):[0-9]{1,5}(/[A-Za-z0-9._~%/-]*)?$`)
	proxyHostRe   = regexp.MustCompile(`^(\[[0-9a-f:.]+\]|[a-z0-9.-]+)(:[0-9]{1,5})?$`)
	proxyDNSRe    = regexp.MustCompile(`^[0-9a-f:.\[\] ]+$`)
	proxyOLSRe    = regexp.MustCompile(`^(https://)?(\[[0-9a-f:.]+\]|[a-z0-9.-]+):[0-9]{1,5}$`)
	proxyRedirRe  = regexp.MustCompile(`^https?://(\[[0-9a-f:.]+\]|[a-z0-9.-]+)(:[0-9]{1,5})?(/[A-Za-z0-9._~%-]+)*/$`)
	errBadUpstrem = errors.New("refusing to render the reverse proxy")
)

func (p *proxyVhost) check() error {
	if !proxyURLRe.MatchString(p.URL) || strings.HasSuffix(p.URL, "/") || !proxyOLSRe.MatchString(p.OLSAddress) ||
		(p.HostHeader != "" && !proxyHostRe.MatchString(p.HostHeader)) || !proxyHostRe.MatchString(p.Name) ||
		(p.OLSProxyHost != "" && (p.OLSProxyHost != p.HostHeader || strings.ContainsAny(p.OLSProxyHost, "[]"))) ||
		(p.Resolver != "" && !proxyDNSRe.MatchString(p.Resolver)) || (p.Path != "" && !upstreamPathRe.MatchString(p.Path)) ||
		p.Timeout < 1 || p.Timeout > maxProxyTimeout || p.ConnectTimeout < 1 || p.ConnectTimeout > 60 {
		return fmt.Errorf("%w %q", errBadUpstrem, p.URL)
	}
	for _, r := range p.RedirectFrom {
		if !proxyRedirRe.MatchString(r) {
			return fmt.Errorf("%w redirect %q", errBadUpstrem, r)
		}
	}
	return nil
}

// proxyVhostFor builds the template data of a reverse proxy site.
func (m *Manager) proxyVhostFor(site *store.Site) (*proxyVhost, error) {
	u, err := parseUpstream(site.ProxyURL)
	if err != nil {
		return nil, fmt.Errorf("the reverse proxy address of %s: %w", site.Domain, err)
	}
	o := site.ProxyOpts
	if o.Timeout <= 0 {
		o.Timeout = DefaultProxyOptions().Timeout
	}
	p := &proxyVhost{URL: u.String(), HTTPS: u.Scheme == "https", Path: u.Path, Name: u.Host,
		WebSocket: o.WebSocket, Forwarded: o.Forwarded, Insecure: o.Insecure && u.Scheme == "https",
		Timeout: o.Timeout, ConnectTimeout: min(o.Timeout, 30)}
	if u.IP != nil && u.IP.To4() == nil {
		p.Name = "[" + u.Host + "]"
	}
	p.OLSAddress = u.hostPort(true)
	if p.HTTPS {
		p.OLSAddress = "https://" + p.OLSAddress
	}
	if o.UpstreamHost {
		p.HostHeader = u.hostPort(false)
		if !strings.Contains(p.HostHeader, "[") {
			p.OLSProxyHost = p.HostHeader
		}
		p.RedirectFrom = []string{u.String() + "/"}
		if short := u.Scheme + "://" + u.hostPort(false) + u.Path + "/"; short != p.RedirectFrom[0] {
			p.RedirectFrom = append(p.RedirectFrom, short)
		}
	}
	if u.IP == nil && u.Host != "localhost" {
		read := m.resolvConf
		if read == nil {
			read = func() string { b, _ := os.ReadFile("/etc/resolv.conf"); return string(b) }
		}
		if p.Resolver = nameservers(read()); p.Resolver == "" {
			return nil, fmt.Errorf("%s forwards to %s, but /etc/resolv.conf names no DNS server to look it up with: use its IP address", site.Domain, u.Host)
		}
	} else if u.Host == "localhost" {
		p.URL = strings.Replace(p.URL, "://localhost:", "://127.0.0.1:", 1)
		p.OLSAddress = strings.Replace(p.OLSAddress, "localhost:", "127.0.0.1:", 1)
	}
	return p, p.check()
}

// nameservers returns the DNS servers of a resolv.conf for Nginx's
// resolver directive (IPv6 addresses in brackets).
func nameservers(conf string) string {
	var out []string
	for _, l := range strings.Split(conf, "\n") {
		f := strings.Fields(l)
		if len(f) < 2 || f[0] != "nameserver" {
			continue
		}
		ip := net.ParseIP(f[1])
		if ip == nil || strings.Contains(f[1], "%") {
			continue
		}
		s := ip.String()
		if ip.To4() == nil {
			s = "[" + s + "]"
		}
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return strings.Join(out, " ")
}

// SetProxy changes the address and options of a reverse proxy site. If the
// web server rejects the result, the old configuration is put back.
func (m *Manager) SetProxy(ctx context.Context, id int64, raw string, o store.ProxyOptions) (*store.Site, error) {
	if !m.Supported() {
		return nil, ErrUnsupported
	}
	if err := m.tryLock(); err != nil {
		return nil, err
	}
	defer m.ops.Unlock()
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if site.ProxyURL == "" {
		return nil, inputErr("not_proxy_site")
	}
	addr, err := m.checkProxy(ctx, raw, &o)
	if err != nil {
		return nil, err
	}
	old := *site
	site.ProxyURL, site.ProxyOpts = addr, o
	if err := m.updateSite(ctx, site, &old); err != nil {
		return nil, err
	}
	return site, nil
}

// IsProxySite reports whether a site forwards to an address of its own.
func IsProxySite(site *store.Site) bool { return site.ProxyURL != "" }

// validProxySite is a last check before a reverse proxy site is stored.
func validProxySite(site *store.Site) error {
	if site.ProxyURL == "" {
		return nil
	}
	if site.ProxyPort != 0 || site.PHPVersion != "" || site.AppCommand != "" || validate.Domain(site.Domain) != nil {
		return inputErr("invalid_app")
	}
	return nil
}
