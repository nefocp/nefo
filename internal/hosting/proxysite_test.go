package hosting

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

func TestParseUpstream(t *testing.T) {
	for in, want := range map[string]string{
		"http://10.0.0.5:8080":            "http://10.0.0.5:8080",
		" HTTPS://Backend.Example.com/ ":  "https://backend.example.com:443",
		"http://backend":                  "http://backend:80",
		"https://[2001:db8::1]:8443/app/": "https://[2001:db8::1]:8443/app",
		"http://127.0.0.1:3000/a/b%20c":   "http://127.0.0.1:3000/a/b%20c",
		"http://localhost:9000":           "http://localhost:9000",
	} {
		u, err := parseUpstream(in)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if u.String() != want {
			t.Errorf("%q: %q, want %q", in, u.String(), want)
		}
	}
	for _, bad := range []string{
		"", "ftp://x.com", "//x.com", "http://", "http://x.com:0", "http://x.com:65536", "http://x.com:08080",
		"http://user:pw@x.com", "http://x.com/?a=1", "http://x.com/#f", "http://x.com/a b", "http://x.com/$host",
		"http://x.com/a;b", "http://x.com/a/../b", "http://x.com/./a", "http://x.com//a", "http://0.0.0.0:80",
		"http://224.0.0.1", "http://x_y.com", "http://x.com:", "http://[fe80::1%25eth0]:80", "http://x.com/a\"b",
		"javascript:alert(1)", "http://x.com/{a}", "http://" + strings.Repeat("a", 300) + ".com",
	} {
		if _, err := parseUpstream(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func proxyTestManager(t *testing.T) *Manager {
	m := newTestManager(t)
	m.SetReservedPorts(46949, 7809)
	m.localAddrs = func() []net.IP { return []net.IP{net.ParseIP("203.0.113.10"), net.ParseIP("127.0.0.1")} }
	m.lookupHost = func(_ context.Context, host string) ([]net.IP, error) {
		switch host {
		case "backend.example.com":
			return []net.IP{net.ParseIP("198.51.100.7")}, nil
		case "panel.example.com":
			return []net.IP{net.ParseIP("203.0.113.10")}, nil
		}
		return nil, errors.New("no such host")
	}
	m.resolvConf = func() string { return "# a comment\nnameserver 127.0.0.53\nnameserver 2001:db8::53\noptions edns0\n" }
	return m
}

func TestCheckProxyRefusals(t *testing.T) {
	m := proxyTestManager(t)
	ctx := context.Background()
	for in, code := range map[string]string{
		"http://127.0.0.1:46949":          "proxy_upstream_panel",
		"https://localhost:46949":         "proxy_upstream_panel",
		"http://127.0.0.1:7809/git/1":     "proxy_upstream_panel",
		"http://203.0.113.10:46949":       "proxy_upstream_panel",
		"https://panel.example.com:46949": "proxy_upstream_panel",
		"http://[::1]:7809":               "proxy_upstream_panel",
		"http://127.0.0.1":                "proxy_upstream_loop",
		"https://panel.example.com":       "proxy_upstream_loop",
		"http://nowhere.example.com:80":   "proxy_unknown_host",
		"http://x.com/a b":                "invalid_proxy_url",
	} {
		o := DefaultProxyOptions()
		_, err := m.checkProxy(ctx, in, &o)
		var ue *usererr.Error
		if !errors.As(err, &ue) || ue.Code != code {
			t.Errorf("%s: %v, want %s", in, err, code)
		}
	}
	// Another server on the panel's port, and local services on others.
	for _, ok := range []string{"http://198.51.100.7:46949", "https://backend.example.com:46949", "http://127.0.0.1:3000", "http://localhost:8081/app"} {
		o := DefaultProxyOptions()
		if _, err := m.checkProxy(ctx, ok, &o); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
	o := store.ProxyOptions{Timeout: 4}
	if _, err := m.checkProxy(ctx, "http://127.0.0.1:3000", &o); err == nil {
		t.Error("a timeout of 4 s accepted")
	}
	o = store.ProxyOptions{Insecure: true}
	if _, err := m.checkProxy(ctx, "http://127.0.0.1:3000", &o); err != nil || o.Insecure || o.Timeout != 60 {
		t.Errorf("plain http: %+v %v", o, err)
	}
}

func TestProxyVhostFor(t *testing.T) {
	m := proxyTestManager(t)
	site := &store.Site{Domain: "proxy.example.com", ProxyURL: "https://backend.example.com:443/app",
		ProxyOpts: store.ProxyOptions{UpstreamHost: true, WebSocket: true, Forwarded: true, Timeout: 120}}
	p, err := m.proxyVhostFor(site)
	if err != nil {
		t.Fatal(err)
	}
	if p.URL != "https://backend.example.com:443/app" || p.HostHeader != "backend.example.com" || p.Name != "backend.example.com" ||
		p.Resolver != "127.0.0.53 [2001:db8::53]" || p.ConnectTimeout != 30 || p.OLSAddress != "https://backend.example.com:443" ||
		strings.Join(p.RedirectFrom, " ") != "https://backend.example.com:443/app/ https://backend.example.com/app/" {
		t.Fatalf("%+v", p)
	}
	site.ProxyURL, site.ProxyOpts = "http://localhost:8081", store.ProxyOptions{Timeout: 10}
	if p, err = m.proxyVhostFor(site); err != nil || p.URL != "http://127.0.0.1:8081" || p.Resolver != "" || p.HostHeader != "" || p.ConnectTimeout != 10 {
		t.Fatalf("%+v %v", p, err)
	}
	site.ProxyURL = "http://[2001:db8::7]:8080"
	site.ProxyOpts.UpstreamHost = true
	if p, err = m.proxyVhostFor(site); err != nil || p.HostHeader != "[2001:db8::7]:8080" || p.OLSAddress != "[2001:db8::7]:8080" || p.Resolver != "" {
		t.Fatalf("%+v %v", p, err)
	}
	m.resolvConf = func() string { return "" }
	site.ProxyURL = "http://backend.example.com:8080"
	if _, err := m.proxyVhostFor(site); err == nil {
		t.Fatal("a host name without a DNS server rendered")
	}
}

// The templates refuse an upstream that did not come from proxyVhostFor.
func TestProxyRenderRefusesInjection(t *testing.T) {
	base := baseVhost()
	for _, p := range []*proxyVhost{
		{URL: "http://x.com:80; return 200", OLSAddress: "x.com:80", Name: "x.com", Timeout: 60, ConnectTimeout: 30},
		{URL: "http://x.com:80", OLSAddress: "x.com:80", Name: "x.com", HostHeader: "x.com\nproxy_pass", Timeout: 60, ConnectTimeout: 30},
		{URL: "http://x.com:80", OLSAddress: "x.com:80", Name: "x.com", Resolver: "8.8.8.8; include /etc", Timeout: 60, ConnectTimeout: 30},
		{URL: "http://x.com:80/", OLSAddress: "x.com:80", Name: "x.com", Timeout: 60, ConnectTimeout: 30},
		{URL: "http://x.com:80", OLSAddress: "x.com:80", Name: "x.com", Timeout: 0, ConnectTimeout: 30},
		{URL: "http://x.com:80", OLSAddress: "x.com:80\n}", Name: "x.com", Timeout: 60, ConnectTimeout: 30},
		{URL: "http://x.com:80", OLSAddress: "x.com:80", Name: "x.com", RedirectFrom: []string{"http://x.com/ /evil"}, Timeout: 60, ConnectTimeout: 30},
	} {
		d := base
		d.Upstream = p
		if _, err := renderVhost(d); err == nil {
			t.Errorf("%+v rendered", p)
		}
	}
	d := base
	d.ProxyPort, d.Upstream = 3000, &proxyVhost{URL: "http://x.com:80", OLSAddress: "x.com:80", Name: "x.com", Timeout: 60, ConnectTimeout: 30}
	if _, err := renderVhost(d); err == nil {
		t.Error("an app and a reverse proxy rendered")
	}
}

func TestCloneRefusal(t *testing.T) {
	for id, want := range map[string]string{"n8n": "clone_app_workflows", "odoo": "clone_app_jobs", "uptime-kuma": "", "ghost": "", "wikijs": ""} {
		if got := CloneRefusal(id); got != want {
			t.Errorf("%s: %q, want %q", id, got, want)
		}
	}
}
