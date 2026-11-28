package seccheck

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Env is what the checks read and do themselves; tests replace it.
type Env struct {
	Now       time.Time
	Passwd    string // /etc/passwd
	Shadow    string // /etc/shadow
	PHPDir    string // /etc/php: <version>/fpm/php.ini, conf.d and pool.d
	RedisConf string // /etc/redis/redis.conf
	// Probe asks this server's own web server for a path of a website and
	// returns the status code of the answer (see LocalProbe).
	Probe func(ctx context.Context, domain, path string) (int, error)
	// Limits that keep a run short on a server with many websites.
	WalkLimit  int // entries walked in one website's folder
	WalkTotal  int // entries walked in all the websites
	ProbeLimit int // requests to the web server in all
	ListLimit  int // entries read from one website's web root
	// Workers is how many requests, or websites' walks, run at a time.
	Workers int
}

// DefaultEnv is the real server.
func DefaultEnv() Env {
	return Env{Passwd: "/etc/passwd", Shadow: "/etc/shadow", PHPDir: "/etc/php", RedisConf: "/etc/redis/redis.conf", Probe: LocalProbe}
}

func (e Env) withDefaults() Env {
	if e.Now.IsZero() {
		e.Now = time.Now()
	}
	if e.WalkLimit == 0 {
		e.WalkLimit = 20000
	}
	if e.WalkTotal == 0 {
		e.WalkTotal = 100000
	}
	if e.ProbeLimit == 0 {
		e.ProbeLimit = 200
	}
	if e.ListLimit == 0 {
		e.ListLimit = 5000
	}
	if e.Workers == 0 {
		e.Workers = 4
	}
	return e
}

// LocalProbe requests a website's path from the web server on this
// server (127.0.0.1, ports 80 and 443) with the website's name, whatever
// its DNS says, and returns the answer's status. A redirect to HTTPS is
// followed once, to port 443 with the certificate not checked: the
// question is only whether the file is served. It asks for one byte, and
// gives up on a request after probeTimeout: a file the web server serves
// answers at once, while a slow answer comes from an application (PHP)
// that the check does not need to wait for.
func LocalProbe(ctx context.Context, domain, path string) (int, error) {
	code, loc, err := probeOnce(ctx, "http", domain, path)
	if err != nil {
		return 0, err
	}
	if code >= 300 && code < 400 {
		if u, err := url.Parse(loc); err == nil && u.Scheme == "https" && (u.Hostname() == domain || u.Hostname() == "www."+domain || "www."+u.Hostname() == domain) {
			code, _, err = probeOnce(ctx, "https", u.Hostname(), path)
		}
	}
	return code, err
}

// probeTimeout bounds one request of LocalProbe.
const probeTimeout = 3 * time.Second

func probeOnce(ctx context.Context, scheme, domain, path string) (int, string, error) {
	port := "80"
	if scheme == "https" {
		port = "443"
	}
	d := &net.Dialer{Timeout: 2 * time.Second}
	tr := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return d.DialContext(ctx, "tcp", net.JoinHostPort("127.0.0.1", port))
		},
		TLSClientConfig:       &tls.Config{ServerName: domain, InsecureSkipVerify: true}, // only "is it served?", never data
		TLSHandshakeTimeout:   2 * time.Second,
		ResponseHeaderTimeout: probeTimeout,
		DisableKeepAlives:     true,
	}
	defer tr.CloseIdleConnections()
	c := &http.Client{Transport: tr, Timeout: probeTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	u := &url.URL{Scheme: scheme, Host: domain, Path: path}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Range", "bytes=0-0")
	req.Header.Set("User-Agent", "Nefo-security-check")
	resp, err := c.Do(req)
	if err != nil {
		return 0, "", err
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	resp.Body.Close()
	return resp.StatusCode, resp.Header.Get("Location"), nil
}

func itoa(n int) string { return strconv.Itoa(n) }
