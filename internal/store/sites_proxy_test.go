package store

import (
	"context"
	"testing"
)

func TestSiteProxyFields(t *testing.T) {
	ctx := context.Background()
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	opts := ProxyOptions{UpstreamHost: true, WebSocket: true, Forwarded: true, Timeout: 90, Insecure: true}
	site := &Site{Domain: "proxy.example.com", SysUser: "proxy_example_com", ProxyURL: "https://10.0.0.5:8443/app", ProxyOpts: opts}
	if err := st.CreateSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	plain := &Site{Domain: "plain.example.com", SysUser: "plain_example_com"}
	if err := st.CreateSite(ctx, plain); err != nil {
		t.Fatal(err)
	}
	got, err := st.SiteByID(ctx, site.ID)
	if err != nil || got.ProxyURL != site.ProxyURL || got.ProxyOpts != opts {
		t.Fatalf("got %+v, %v", got, err)
	}
	got.ProxyOpts.Timeout, got.ProxyOpts.Insecure = 30, false
	if err := st.UpdateSite(ctx, got); err != nil {
		t.Fatal(err)
	}
	again, _ := st.SiteByID(ctx, site.ID)
	if again.ProxyOpts.Timeout != 30 || again.ProxyOpts.Insecure {
		t.Fatalf("after the update: %+v", again.ProxyOpts)
	}
	var raw string
	st.db.QueryRow(`SELECT proxy_opts FROM sites WHERE id = ?`, plain.ID).Scan(&raw)
	if p, _ := st.SiteByID(ctx, plain.ID); p.ProxyURL != "" || p.ProxyOpts != (ProxyOptions{}) || raw != "" {
		t.Fatalf("plain site: %+v %q", p, raw)
	}
}
