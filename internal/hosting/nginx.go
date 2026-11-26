package hosting

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/version"
)

// nginxServer is Nginx, the default web server. Its files are exactly those
// the panel wrote before Apache was added (testdata/golden pins them).
type nginxServer struct{ m *Manager }

func (n nginxServer) kind() string  { return ServerNginx }
func (n nginxServer) title() string { return "Nginx" }
func (n nginxServer) unit() string  { return "nginx" }

func (n nginxServer) snippetDir() string { return filepath.Join(n.m.paths.Nginx, "panel") }
func (n nginxServer) sitesDir() string   { return filepath.Join(n.m.paths.Nginx, "panel", "sites") }
func (n nginxServer) hookFile() string   { return filepath.Join(n.m.paths.Nginx, "conf.d", "panel.conf") }
func (n nginxServer) logDir() string     { return n.m.paths.NginxLogs }

func (n nginxServer) packages() []string { return []string{"nginx"} }
func (n nginxServer) removable() []string {
	return []string{"nginx", "nginx-common", "nginx-core", "nginx-full", "nginx-light", "nginx-extras",
		"libnginx-mod-http-modsecurity", "modsecurity-crs"}
}

func (n nginxServer) info(ctx context.Context) ServerInfo {
	var info ServerInfo
	out, err := n.m.run.Output(ctx, "nginx", "-v")
	if err != nil {
		return info
	}
	info.Installed = true
	if mm := nginxVersionRe.FindStringSubmatch(out); mm != nil {
		info.Version = mm[1]
	}
	info.Running = n.m.unitActive(ctx, "nginx")
	return info
}

func (n nginxServer) features(ctx context.Context) nginxFeatures {
	return featuresFor(n.info(ctx).Version, ipv6Enabled())
}

func (n nginxServer) prepare(ctx context.Context) error { return nil }

// writeBase writes the panel's shared Nginx files and turns off the
// distribution's default site, whose default_server would clash with ours.
func (n nginxServer) writeBase(ctx context.Context) error {
	p := n.m.paths
	for _, d := range []string{n.snippetDir(), n.sitesDir(), p.WWW, p.ACMEWebroot, filepath.Dir(p.challengeDir()), p.challengeDir()} {
		if err := mkdir(d, 0o755); err != nil {
			return err
		}
	}
	files, err := n.baseFiles(ctx)
	if err != nil {
		return err
	}
	for path, data := range files {
		if err := writeFile(path, data, 0o644); err != nil {
			return err
		}
	}
	def := filepath.Join(p.Nginx, "sites-enabled", "default")
	if fi, err := os.Lstat(def); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		if err := os.Remove(def); err != nil {
			return fmt.Errorf("turn off the default Nginx site: %w", err)
		}
	}
	return nil
}

func (n nginxServer) removeBase() error {
	if err := removeIfExists(n.hookFile()); err != nil {
		return err
	}
	if err := os.RemoveAll(n.snippetDir()); err != nil {
		return err
	}
	removeEmptyDirs(n.m.paths.Nginx)
	return nil
}

// baseFiles renders the panel's shared Nginx files.
func (n nginxServer) baseFiles(ctx context.Context) (map[string][]byte, error) {
	m := n.m
	p := m.paths
	acme, err := render(acmeSnippetTmpl, struct{ Webroot string }{p.ACMEWebroot})
	if err != nil {
		return nil, err
	}
	ssl, err := render(sslSnippetTmpl, nil)
	if err != nil {
		return nil, err
	}
	waf := m.wafEnabled()
	hook, err := render(hookTmpl, hookData{nginxFeatures: n.features(ctx), SitesDir: n.sitesDir(), SnippetDir: n.snippetDir(), WAF: waf})
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{
		filepath.Join(n.snippetDir(), "acme.conf"): acme,
		filepath.Join(n.snippetDir(), "ssl.conf"):  ssl,
		n.hookFile(): hook,
	}
	if waf {
		sites, err := m.store.ListSites(ctx)
		if err != nil {
			return nil, err
		}
		for path, data := range n.wafFiles(sites) {
			files[path] = data
		}
	}
	return files, nil
}

func (n nginxServer) vhost(ctx context.Context, d vhostData, rewrite string) ([]byte, error) {
	d.nginxFeatures = n.features(ctx)
	d.RewriteRules = rewriteRules(rewrite, d.PHPSocket != "")
	return renderVhost(d)
}

func (n nginxServer) customBytes(site *store.Site) []byte { return customBytes(site) }

func (n nginxServer) test(ctx context.Context) (string, error) {
	return n.m.run.Output(ctx, "nginx", "-t")
}

func (n nginxServer) testError(out string) string { return nginxTestError(out) }

// reload checks the configuration, reloads Nginx and waits until the new
// configuration answers requests.
func (n nginxServer) reload(ctx context.Context) error {
	if out, err := n.test(ctx); err != nil {
		return fmt.Errorf("nginx configuration test failed:\n%s", run.Tail(out, 10))
	}
	before := activeNginxWorkers()
	if err := n.m.systemctl(ctx, "reload", "nginx"); err != nil {
		return err
	}
	if !waitNginxReload(before, 5*time.Second) {
		n.m.log.Warn("nginx reload: old workers still active after 5 seconds")
	}
	return nil
}

func (n nginxServer) restart(ctx context.Context) error {
	if out, err := n.test(ctx); err != nil {
		return fmt.Errorf("nginx configuration test failed:\n%s", run.Tail(out, 10))
	}
	return n.m.systemctl(ctx, "restart", "nginx")
}

// adminerConf is the internal Nginx server for Adminer. The token is a
// secret: the file is readable by root only.
func (n nginxServer) adminerConf(port, token, php string) []byte {
	return []byte(fmt.Sprintf(`# Managed by %[1]s. Adminer, reachable only through the panel.
server {
    listen 127.0.0.1:%[2]s;
    server_name _;
    access_log off;
    server_tokens off;
    client_max_body_size 256m;

    if ($http_x_panel_adminer != "%[3]s") {
        return 404;
    }

    location / {
        include fastcgi_params;
        fastcgi_param SCRIPT_FILENAME %[4]s/index.php;
        fastcgi_param HTTPS $http_x_panel_https if_not_empty;
        fastcgi_param HTTP_X_PANEL_ADMINER "" if_not_empty;
        fastcgi_param HTTP_X_PANEL_HTTPS "" if_not_empty;
        fastcgi_param PHP_VALUE "upload_max_filesize=256M
post_max_size=256M
max_execution_time=600
memory_limit=256M";
        fastcgi_pass unix:/run/php/php%[5]s-fpm.sock;
        fastcgi_read_timeout 600s;
    }
}
`, version.AppName, port, token, adminerDir, php))
}

func (n nginxServer) wafAvailable() bool {
	return fileExists(wafModuleConf) && fileExists(filepath.Join(wafCRSRules, "REQUEST-901-INITIALIZATION.conf")) && fileExists(wafCRSSetup)
}

func (n nginxServer) wafPackages() []string {
	return []string{"libnginx-mod-http-modsecurity", "modsecurity-crs"}
}

func (n nginxServer) wafFiles(sites []*store.Site) map[string][]byte { return n.m.nginxWAFFiles(sites) }

func (n nginxServer) wafExtraFiles() []string { return nil }
