package hosting

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/store"
)

// fakeWeb is a web server for Manager tests: it records the vhost data it
// is asked to render instead of rendering it (the real templates are pinned
// by the golden files), and reloads nothing.
type fakeWeb struct {
	nginxServer
	name   string
	vhosts []vhostData
}

func (f *fakeWeb) kind() string  { return f.name }
func (f *fakeWeb) title() string { return f.name }
func (f *fakeWeb) vhost(_ context.Context, d vhostData, _ string) ([]byte, error) {
	f.vhosts = append(f.vhosts, d)
	return []byte("# " + d.Domain + "\n"), nil
}
func (f *fakeWeb) reload(context.Context) error { return nil }
func (f *fakeWeb) wafAvailable() bool           { return false }

func (f *fakeWeb) last(t *testing.T) vhostData {
	t.Helper()
	if len(f.vhosts) == 0 {
		t.Fatal("no vhost was written")
	}
	return f.vhosts[len(f.vhosts)-1]
}

// newSiteTestManager returns a Manager whose every path is in a temporary
// folder, managing the web server kind through a fakeWeb.
func newSiteTestManager(t *testing.T, kind string) (*Manager, *fakeWeb) {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	root := t.TempDir()
	in := func(name string) string { return filepath.Join(root, name) }
	p := Paths{WWW: in("www"), Nginx: in("nginx"), Apache: in("apache"), OLS: in("lsws"), PHP: in("php"),
		NginxLogs: in("log-nginx"), ApacheLogs: in("log-apache"), OLSLogs: in("log-ols"), PHPRun: in("run-php"),
		ACMEWebroot: in("acme-webroot"), Certs: in("data/certs"), ACME: in("data/acme"), Quarantine: in("quarantine"),
		NginxCache: in("cache/nginx/nefo")}
	m := New(p, &fakeRunner{}, st, slog.New(slog.NewTextHandler(io.Discard, nil)), "")
	fw := &fakeWeb{nginxServer: nginxServer{m}, name: kind}
	switch kind {
	case ServerApache:
		m.apache = fw
	case ServerOLS:
		m.ols = fw
	default:
		m.nginx = fw
	}
	if err := m.setServerKind(context.Background(), kind); err != nil {
		t.Fatal(err)
	}
	if err := mkdir(fw.sitesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	return m, fw
}

// addSite stores a site with all its settings.
func addSite(t *testing.T, m *Manager, s *store.Site) *store.Site {
	t.Helper()
	ctx := context.Background()
	if err := m.store.CreateSite(ctx, s); err != nil {
		t.Fatal(err)
	}
	if err := m.store.UpdateSite(ctx, s); err != nil {
		t.Fatal(err)
	}
	got, err := m.store.SiteByID(ctx, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestSiteHSTSValue(t *testing.T) {
	for _, c := range []struct {
		site store.Site
		want string
	}{
		{store.Site{HSTS: true, SSL: true, ForceHTTPS: true}, "max-age=15552000"},
		{store.Site{HSTS: true, HSTSSubdomains: true, SSL: true, ForceHTTPS: true}, "max-age=15552000; includeSubDomains"},
		{store.Site{HSTS: true, SSL: true}, ""},        // no redirect
		{store.Site{HSTS: true, ForceHTTPS: true}, ""}, // no HTTPS
		{store.Site{HSTSSubdomains: true, SSL: true, ForceHTTPS: true}, ""},
	} {
		if got := siteHSTS(&c.site); got != c.want {
			t.Errorf("%+v: %q, want %q", c.site, got, c.want)
		}
		if strings.Contains(siteHSTS(&c.site), "preload") {
			t.Error("preload is never sent")
		}
	}
}

func TestVhostRefusesBadHSTSAndCache(t *testing.T) {
	force := func() vhostData {
		d := baseVhost()
		d.PHPSocket = "/run/php/panel-example_com.sock"
		d.SSL, d.ForceHTTPS, d.CertFile, d.KeyFile = true, true, "/c/fullchain.pem", "/c/privkey.pem"
		d.LogDir, d.EmptyRoot, d.User = "/var/log/x", "/var/lib/panel-acme", "example_com"
		return d
	}
	bad := map[string]func(d *vhostData){
		"hsts without redirect": func(d *vhostData) { d.HSTS, d.ForceHTTPS = hstsValue, false },
		"hsts without https":    func(d *vhostData) { d.HSTS, d.SSL, d.ForceHTTPS = hstsValue, false, false },
		"hsts preload":          func(d *vhostData) { d.HSTS = hstsValue + "; preload" },
		"hsts injection":        func(d *vhostData) { d.HSTS = `max-age=1" always; return 200 "x` },
		"cache without php":     func(d *vhostData) { d.CacheZone, d.CacheDir, d.PHPSocket = "panel_fc_example_com", "/var/cache/x", "" },
		"cache without dir":     func(d *vhostData) { d.CacheZone = "panel_fc_example_com" },
		"cache bad zone":        func(d *vhostData) { d.CacheZone, d.CacheDir = "panel_fc_a b", "/var/cache/x" },
		"cache unsafe dir":      func(d *vhostData) { d.CacheZone, d.CacheDir = "panel_fc_example_com", "/var/cache/x;" },
	}
	for name, f := range bad {
		d := force()
		f(&d)
		if _, err := renderVhost(d); err == nil {
			t.Errorf("nginx: %s rendered", name)
		}
		if _, err := renderApacheVhost(d, ""); err == nil {
			t.Errorf("apache: %s rendered", name)
		}
		if _, err := renderOLSVhost(olsVhost{vhostData: d, ErrorDir: olsErrorDir}, ""); err == nil {
			t.Errorf("openlitespeed: %s rendered", name)
		}
	}
	d := force()
	d.HSTS, d.CacheZone, d.CacheDir = hstsValueSubdomains, "panel_fc_example_com", "/var/cache/nginx/nefo/example.com"
	out, err := renderVhost(d)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	// The zone is declared at the top, in the http block, and both headers
	// are set for the server (indented once), never in a location: a
	// location's own add_header would drop the server's.
	if !strings.HasPrefix(s[strings.Index(s, "fastcgi_cache_path"):], "fastcgi_cache_path /var/cache/nginx/nefo/example.com ") ||
		strings.Index(s, "fastcgi_cache_path") > strings.Index(s, "server {") {
		t.Errorf("cache zone not at http level:\n%s", s)
	}
	https := s[strings.LastIndex(s, "server {"):]
	mustContain(t, https, "\n    add_header Strict-Transport-Security \"max-age=15552000; includeSubDomains\" always;\n",
		"\n    add_header X-Cache-Status $upstream_cache_status always;\n")
	// Once each: with the redirect on, the HTTP server serves nothing.
	if n := strings.Count(s, "\n    add_header "); n != 2 || strings.Contains(s, "\n        add_header") {
		t.Errorf("add_header %d times at server level, want 2, and none in a location:\n%s", n, s)
	}
	mustNotContain(t, s[:strings.LastIndex(s, "server {")], "Strict-Transport-Security")
	mustContain(t, s, "fastcgi_cache_bypass $panel_no_cache;", "fastcgi_no_cache $panel_no_cache;",
		"wordpress_logged_in_", "PHPSESSID", "wp-admin/", "my-account/", `$query_string != ""`)
	// Apache and OpenLiteSpeed: the header only, no Nginx cache.
	a, err := renderApacheVhost(d, "")
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, string(a), `Header always set Strict-Transport-Security "max-age=15552000; includeSubDomains"`)
	mustNotContain(t, string(a), "fastcgi_cache", "X-Cache-Status")
	o, err := renderOLSVhost(olsVhost{vhostData: d, ErrorDir: olsErrorDir}, "")
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, string(o), "Strict-Transport-Security: max-age=15552000; includeSubDomains")
	mustNotContain(t, string(o), "fastcgi_cache", "X-Cache-Status")
}

// HTTPS turned off renders exactly the vhost of a site that never had it.
func TestHTTPSOffGoldenIsPlainHTTP(t *testing.T) {
	for _, dir := range []string{"nginx", "apache", "ols"} {
		off, err := os.ReadFile(filepath.Join("testdata", "golden", dir, "vhost-php-https-off.conf"))
		if err != nil {
			t.Fatal(err)
		}
		plain, err := os.ReadFile(filepath.Join("testdata", "golden", dir, "vhost-php.conf"))
		if err != nil {
			t.Fatal(err)
		}
		if string(off) != string(plain) {
			t.Errorf("%s: the vhost with HTTPS turned off differs from a plain HTTP one", dir)
		}
		mustNotContain(t, string(off), "443", "Strict-Transport-Security", "https")
	}
}

func TestRemoveCertificate(t *testing.T) {
	for _, kind := range []string{ServerNginx, ServerApache, ServerOLS} {
		t.Run(kind, func(t *testing.T) {
			m, fw := newSiteTestManager(t, kind)
			ctx := context.Background()
			now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
			site := addSite(t, m, &store.Site{Domain: "example.com", SysUser: "example_com", PHPVersion: "8.3",
				SSL: true, ForceHTTPS: true, HSTS: true, HSTSSubdomains: true, SSLSource: SSLCustom, SSLDNS: true,
				CertNotBefore: now, CertNotAfter: now.AddDate(0, 3, 0), SSLError: "old error", SSLAttemptAt: now})
			certs := m.paths.certDir("example.com")
			other := m.paths.certDir("other.example.com")
			account := filepath.Join(m.paths.ACME, "account.key")
			for _, f := range []string{filepath.Join(certs, "fullchain.pem"), filepath.Join(certs, "privkey.pem"),
				filepath.Join(other, "fullchain.pem"), account} {
				if err := os.MkdirAll(filepath.Dir(f), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(f, []byte("pem"), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			got, err := m.removeCertificate(ctx, site.ID)
			if err != nil {
				t.Fatal(err)
			}
			stored, _ := m.store.SiteByID(ctx, site.ID)
			for _, s := range []*store.Site{got, stored} {
				if s.SSL || s.ForceHTTPS || s.HSTS || s.HSTSSubdomains || s.SSLSource != "" || s.SSLError != "" ||
					!s.CertNotAfter.IsZero() || !s.CertNotBefore.IsZero() {
					t.Errorf("HTTPS settings left: %+v", s)
				}
				if !s.SSLDNS {
					t.Error("the DNS validation choice was lost")
				}
			}
			if d := fw.last(t); d.SSL || d.ForceHTTPS || d.HSTS != "" || d.CertFile != "" || d.KeyFile != "" {
				t.Errorf("vhost still has HTTPS: %+v", d)
			}
			if _, err := os.Stat(certs); !os.IsNotExist(err) {
				t.Errorf("the certificate folder is still there: %v", err)
			}
			for _, keep := range []string{account, filepath.Join(other, "fullchain.pem")} {
				if _, err := os.Stat(keep); err != nil {
					t.Errorf("%s was deleted: %v", keep, err)
				}
			}
			// Nothing renews it: RenewDue only renews sites with HTTPS on.
			if stored.SSL {
				t.Error("renewal would continue")
			}
			// Again, on a site without HTTPS: nothing to do, no error.
			if _, err := m.removeCertificate(ctx, site.ID); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSetHSTS(t *testing.T) {
	m, fw := newSiteTestManager(t, ServerApache)
	ctx := context.Background()
	plain := addSite(t, m, &store.Site{Domain: "plain.example.com", SysUser: "plain_example_com"})
	noRedirect := addSite(t, m, &store.Site{Domain: "ssl.example.com", SysUser: "ssl_example_com", SSL: true})
	site := addSite(t, m, &store.Site{Domain: "example.com", SysUser: "example_com", SSL: true, ForceHTTPS: true})

	for _, s := range []*store.Site{plain, noRedirect} {
		_, err := m.setHSTS(ctx, s.ID, true, false)
		wantCode(t, err, "hsts_needs_redirect")
	}
	if len(fw.vhosts) != 0 {
		t.Fatal("a refused change wrote a vhost")
	}
	// Off on a site without HTTPS is fine (nothing to do).
	if _, err := m.setHSTS(ctx, plain.ID, false, true); err != nil {
		t.Fatal(err)
	}

	got, err := m.setHSTS(ctx, site.ID, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if !got.HSTS || !got.HSTSSubdomains || fw.last(t).HSTS != hstsValueSubdomains {
		t.Fatalf("on with subdomains: %+v, vhost %q", got, fw.last(t).HSTS)
	}
	if got, _ = m.setHSTS(ctx, site.ID, true, false); got.HSTSSubdomains || fw.last(t).HSTS != hstsValue {
		t.Fatalf("on without subdomains: %+v, vhost %q", got, fw.last(t).HSTS)
	}
	// Off clears includeSubDomains too.
	if got, _ = m.setHSTS(ctx, site.ID, false, true); got.HSTS || got.HSTSSubdomains || fw.last(t).HSTS != "" {
		t.Fatalf("off: %+v, vhost %q", got, fw.last(t).HSTS)
	}
	stored, _ := m.store.SiteByID(ctx, site.ID)
	if stored.HSTS || stored.HSTSSubdomains {
		t.Fatalf("stored: %+v", stored)
	}
}

func TestPageCache(t *testing.T) {
	m, fw := newSiteTestManager(t, ServerNginx)
	ctx := context.Background()
	static := addSite(t, m, &store.Site{Domain: "static.example.com", SysUser: "static_example_com"})
	app := addSite(t, m, &store.Site{Domain: "app.example.com", SysUser: "app_example_com", ProxyPort: 3000})
	site := addSite(t, m, &store.Site{Domain: "example.com", SysUser: "example_com", PHPVersion: "8.3"})

	for _, s := range []*store.Site{static, app} {
		_, err := m.setPageCache(ctx, s.ID, true)
		wantCode(t, err, "page_cache_needs_php")
	}
	_, err := m.purgePageCache(ctx, site.ID)
	wantCode(t, err, "page_cache_off")
	if len(fw.vhosts) != 0 {
		t.Fatal("a refused change wrote a vhost")
	}

	dir := filepath.Join(m.paths.NginxCache, "example.com")
	got, err := m.setPageCache(ctx, site.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	d := fw.last(t)
	if !got.PageCache || d.CacheZone != "panel_fc_example_com" || d.CacheDir != dir {
		t.Fatalf("on: %+v, vhost zone %q dir %q", got, d.CacheZone, d.CacheDir)
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Fatalf("cache folder: %v", err)
	}

	// Purge empties the folder and keeps it; other sites' caches stay.
	other := filepath.Join(m.paths.NginxCache, "other.example.com", "a")
	for _, f := range []string{filepath.Join(dir, "c", "4f", "0123abc"), filepath.Join(dir, "top"), other} {
		os.MkdirAll(filepath.Dir(f), 0o700)
		if err := os.WriteFile(f, []byte("cached"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := m.purgePageCache(ctx, site.ID); err != nil {
		t.Fatal(err)
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatalf("after purge: %v %v", entries, err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("another site's cache was purged: %v", err)
	}

	// Off: no zone, and the folder is gone.
	os.WriteFile(filepath.Join(dir, "page"), []byte("cached"), 0o600)
	if got, err = m.setPageCache(ctx, site.ID, false); err != nil || got.PageCache {
		t.Fatalf("off: %+v %v", got, err)
	}
	if d := fw.last(t); d.CacheZone != "" || d.CacheDir != "" {
		t.Fatalf("off: vhost still has the cache %q", d.CacheZone)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("the cache folder is still there: %v", err)
	}
}

func TestPageCacheNginxOnly(t *testing.T) {
	for _, kind := range []string{ServerApache, ServerOLS} {
		m, fw := newSiteTestManager(t, kind)
		ctx := context.Background()
		site := addSite(t, m, &store.Site{Domain: "example.com", SysUser: "example_com", PHPVersion: "8.3"})
		_, err := m.setPageCache(ctx, site.ID, true)
		wantCode(t, err, "page_cache_nginx_only")
		if _, err := os.Stat(m.paths.NginxCache); !os.IsNotExist(err) {
			t.Errorf("%s: a cache folder was created", kind)
		}
		// A stored flag (from a former Nginx setup) renders nothing here.
		site.PageCache = true
		if err := m.store.UpdateSite(ctx, site); err != nil {
			t.Fatal(err)
		}
		if _, err := m.vhostBytes(ctx, site); err != nil {
			t.Fatal(err)
		}
		if d := fw.last(t); d.CacheZone != "" || d.CacheDir != "" {
			t.Errorf("%s: vhost has the Nginx cache", kind)
		}
	}
}
