package hosting

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/version"
)

// RewritePreset is a set of Nginx location rules an application needs. A
// preset replaces the site's default "location /" block.
type RewritePreset struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	PHP  bool   `json:"php"` // only for PHP sites
	// Rules are the location blocks, indented for the server block.
	Rules string `json:"rules"`
}

const (
	defaultPHPRules = `    location / {
        try_files $uri $uri/ /index.php?$args;
    }`
	defaultStaticRules = `    location / {
        try_files $uri $uri/ =404;
    }`
)

// RewritePresets lists the presets; "" is the default.
var RewritePresets = []RewritePreset{
	{ID: "wordpress", Name: "WordPress", PHP: true, Rules: `    # Files in uploads never run as PHP.
    location ~* ^/wp-content/uploads/.*\.php$ {
        deny all;
    }

    location / {
        try_files $uri $uri/ /index.php?$args;
    }`},
	{ID: "laravel", Name: "Laravel", PHP: true, Rules: `    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }`},
	{ID: "symfony", Name: "Symfony", PHP: true, Rules: `    location / {
        try_files $uri /index.php$is_args$args;
    }`},
	{ID: "codeigniter", Name: "CodeIgniter 4", PHP: true, Rules: `    location / {
        try_files $uri $uri/ /index.php$is_args$args;
    }`},
	{ID: "drupal", Name: "Drupal", PHP: true, Rules: `    location / {
        try_files $uri /index.php?$query_string;
    }

    location @rewrite {
        rewrite ^ /index.php;
    }

    # Image styles are generated on first use.
    location ~ ^/sites/.*/files/styles/ {
        try_files $uri @rewrite;
    }

    location ~ ^(/[a-z\-]+)?/system/files/ {
        try_files $uri /index.php?$query_string;
    }

    location ~ ^/sites/.*/private/ {
        return 403;
    }

    location ~ ^/sites/[^/]+/files/.*\.php$ {
        deny all;
    }

    location ~ /vendor/.*\.php$ {
        deny all;
    }`},
	{ID: "joomla", Name: "Joomla", PHP: true, Rules: `    location / {
        try_files $uri $uri/ /index.php?$args;
    }

    # Scripts in these folders never run.
    location ~* ^/(images|cache|media|logs|tmp)/.*\.(php|pl|py|jsp|asp|sh|cgi)$ {
        return 403;
    }`},
	{ID: "opencart", Name: "OpenCart", PHP: true, Rules: `    location / {
        try_files $uri $uri/ @opencart;
    }

    location @opencart {
        rewrite ^/(.+)$ /index.php?_route_=$1 last;
    }

    # Templates, settings and logs are never served.
    location ~* \.(twig|tpl|ini|log)$ {
        deny all;
    }`},
	{ID: "nextcloud", Name: "Nextcloud", PHP: true, Rules: `    # Service discovery for calendars and contacts.
    location ^~ /.well-known {
        location = /.well-known/carddav {
            return 301 /remote.php/dav/;
        }
        location = /.well-known/caldav {
            return 301 /remote.php/dav/;
        }
        return 301 /index.php$request_uri;
    }

    # Nextcloud's own folders are never served.
    location ~ ^/(?:build|tests|config|lib|3rdparty|templates|data)(?:$|/) {
        return 404;
    }

    location ~ ^/(?:\.|autotest|occ|issue|indie|db_|console) {
        return 404;
    }

    location ~ \.mjs$ {
        default_type text/javascript;
        try_files $uri /index.php$request_uri;
    }

    location / {
        try_files $uri $uri/ /index.php$request_uri;
    }`},
	{ID: "moodle", Name: "Moodle", PHP: true, Rules: `    # Moodle 5 routes paths that are not files through r.php.
    location / {
        try_files $uri $uri/ /r.php?$args;
    }`},
	{ID: "spa", Name: "Single-page app", Rules: `    location / {
        try_files $uri $uri/ /index.html;
    }`},
	// Clean URLs, as static site generators such as VitePress make them:
	// /docs/install serves docs/install.html.
	{ID: "static-clean", Name: "Static site (clean URLs)", Rules: `    location / {
        try_files $uri $uri.html $uri/ =404;
    }`},
}

// knownRules reports whether rules are the default or a preset's rules, so
// nothing else can reach the template.
func knownRules(rules string) bool {
	if rules == "" || rules == defaultPHPRules || rules == defaultStaticRules {
		return true
	}
	return slices.ContainsFunc(RewritePresets, func(p RewritePreset) bool { return p.Rules == rules })
}

func rewritePreset(id string) (RewritePreset, bool) {
	i := slices.IndexFunc(RewritePresets, func(p RewritePreset) bool { return p.ID == id })
	if i < 0 {
		return RewritePreset{}, false
	}
	return RewritePresets[i], true
}

// rewriteRules returns the location rules for a site. A preset that needs
// PHP falls back to the default rules when the site has no PHP.
func rewriteRules(id string, php bool) string {
	if p, ok := rewritePreset(id); ok && (php || !p.PHP) {
		return p.Rules
	}
	if php {
		return defaultPHPRules
	}
	return defaultStaticRules
}

// ---- the administrator's own directives ----

const maxNginxCustom = 32 << 10

func (m *Manager) customFile(domain string) string {
	return filepath.Join(m.web().snippetDir(), "custom", domain+".conf")
}

func customBytes(site *store.Site) []byte {
	return []byte("# Managed by " + version.AppName + ": edit in Websites → Nginx configuration.\n" +
		strings.TrimRight(site.NginxCustom, "\n") + "\n")
}

// NginxConfig is a site's rewrite preset and own directives, with the
// configuration file the panel generates from them. On Apache and
// OpenLiteSpeed the presets' rules and the directives are theirs (see
// apachePresetRules and olsPresetDir).
type NginxConfig struct {
	Server    string          `json:"server"` // "nginx", "apache" or "openlitespeed"
	Rewrite   string          `json:"rewrite"`
	Custom    string          `json:"custom"`
	Presets   []RewritePreset `json:"presets"`
	Generated string          `json:"generated"`
}

// NginxConfig returns a site's web server settings.
func (m *Manager) NginxConfig(ctx context.Context, id int64) (*NginxConfig, error) {
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return nil, err
	}
	gen, err := m.vhostBytes(ctx, site)
	if err != nil {
		return nil, err
	}
	kind := m.web().kind()
	presets := RewritePresets
	switch kind {
	case ServerApache:
		presets = apachePresetList()
	case ServerOLS:
		presets = olsPresetList()
	}
	return &NginxConfig{Server: kind, Rewrite: site.Rewrite, Custom: site.NginxCustom, Presets: presets, Generated: string(gen)}, nil
}

// SetNginxConfig changes a site's rewrite preset and own directives (in the
// managed web server's syntax). The web server checks the result first; if
// it rejects it, nothing changes and the error says why.
func (m *Manager) SetNginxConfig(ctx context.Context, id int64, rewrite, custom string) (*NginxConfig, error) {
	if !m.Supported() {
		return nil, ErrUnsupported
	}
	custom = strings.ReplaceAll(custom, "\r\n", "\n")
	if len(custom) > maxNginxCustom || strings.ContainsRune(custom, 0) {
		return nil, usererr.New("nginx_custom_too_long")
	}
	if strings.TrimSpace(custom) == "" {
		custom = ""
	}
	if err := m.tryLock(); err != nil {
		return nil, err
	}
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		m.ops.Unlock()
		return nil, err
	}
	if rewrite != "" {
		p, ok := rewritePreset(rewrite)
		if !ok || (p.PHP && site.PHPVersion == "") {
			m.ops.Unlock()
			return nil, usererr.New("invalid_rewrite", rewrite)
		}
	}
	old := *site
	site.Rewrite, site.NginxCustom = rewrite, custom

	ws := m.web()
	file := m.customFile(site.Domain)
	oldCustom, oldErr := os.ReadFile(file)
	restoreCustom := func() {
		if oldErr == nil {
			writeFile(file, oldCustom, 0o644)
		} else {
			os.Remove(file)
		}
	}
	err = func() error {
		if custom == "" {
			if err := removeIfExists(file); err != nil {
				return err
			}
		} else {
			if err := mkdir(filepath.Dir(file), 0o755); err != nil {
				return err
			}
			if err := writeFile(file, ws.customBytes(site), 0o644); err != nil {
				return err
			}
		}
		if err := m.writeVhost(ctx, site); err != nil {
			return err
		}
		// Test before reloading, so a mistake is reported in the web
		// server's words.
		if out, err := ws.test(ctx); err != nil {
			return usererr.New(ws.kind()+"_config_invalid", ws.testError(out))
		}
		return m.updateSite(ctx, site, &old)
	}()
	if err != nil {
		restoreCustom()
		if werr := m.writeVhost(context.WithoutCancel(ctx), &old); werr != nil {
			m.log.Warn("restore vhost", "domain", site.Domain, "err", werr)
		}
		m.ops.Unlock()
		return nil, err
	}
	m.ops.Unlock()
	return m.NginxConfig(ctx, id)
}

// nginxTestError keeps the lines of "nginx -t" output that explain the
// problem.
func nginxTestError(out string) string {
	var keep []string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "[emerg]") || strings.Contains(l, "[error]") {
			keep = append(keep, strings.TrimSpace(strings.TrimPrefix(l, "nginx: ")))
		}
	}
	if len(keep) == 0 {
		return run.Tail(out, 5)
	}
	return strings.Join(keep, "\n")
}
