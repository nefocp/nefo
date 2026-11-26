package hosting

import (
	"fmt"
	"slices"
	"strings"
	"text/template"
)

// Apache configuration. A site's .htaccess files work (that is Apache's
// point here), but they cannot weaken what the panel sets:
//
//   - The panel's access rules (password, blocked addresses, firewall,
//     the PHP pool guard) live in <Location> blocks, which Apache applies
//     after the .htaccess files, with AuthMerging And and Satisfy All.
//   - Suspension, redirects, HTTPS, hotlink protection and the presets'
//     protective rules are mod_rewrite rules of the virtual host, which run
//     before any .htaccess file is read.
//   - .htaccess may not change Options, so symlinks are only followed when
//     the link and its target have the same owner (like Nginx's
//     disable_symlinks if_not_owner). Forbidden and unknown directives
//     (php_value without mod_php, Options) are ignored with a warning in
//     the site's error log instead of breaking the site (Nonfatal=All).
//   - PHP requests may only reach the site's own PHP-FPM pool: a .htaccess
//     file can name any handler (SetHandler "proxy:unix:…"), so the
//     <Location> block blanks SCRIPT_FILENAME for any other socket, and it
//     drops PHP_VALUE and PHP_ADMIN_VALUE, which would override the pool's
//     php_admin_value limits.

var apacheFuncs = template.FuncMap{
	"join": strings.Join,
	// rwesc escapes text for a mod_rewrite substitution, where %N and $N
	// are back-references.
	"rwesc": func(s string) string { return strings.NewReplacer("%", `\%`, "$", `\$`).Replace(s) },
}

func mustApacheTemplate(name, text string) *template.Template {
	return template.Must(template.New(name).Funcs(funcs).Funcs(apacheFuncs).Option("missingkey=error").Parse(text))
}

var apacheACMETmpl = mustApacheTemplate("apache-acme", `# Managed by {{app}}. Do not edit: the panel rewrites this file.
# Answers Let's Encrypt HTTP-01 challenges for every site.
Alias /.well-known/acme-challenge/ {{.Webroot}}/.well-known/acme-challenge/
<Directory {{.Webroot}}/.well-known/acme-challenge/>
    Options None
    AllowOverride None
    ForceType text/plain
    Require all granted
</Directory>
`)

// apacheHookTmpl is loaded from conf-enabled, so before sites-enabled: its
// default virtual hosts come first and answer unknown host names.
var apacheHookTmpl = mustApacheTemplate("apache-hook", `# Managed by {{app}}. Do not edit: the panel rewrites this file.
# Loaded from conf-enabled, before the sites: the defaults of every site and
# the virtual hosts for unknown host names.
ServerName localhost

# Nginx's "combined" format, which the traffic statistics read.
LogFormat "%a %l %u %t \"%r\" %>s %b \"%{Referer}i\" \"%{User-Agent}i\"" panel_combined

# Nothing under {{.WWW}} is served unless a site's own block allows it.
<Directory {{.WWW}}/>
    Options None
    AllowOverride None
    Require all denied
</Directory>

<IfModule ssl_module>
    SSLProtocol -all +TLSv1.2 +TLSv1.3
    SSLHonorCipherOrder off
    SSLSessionTickets off
</IfModule>
<IfModule http2_module>
    Protocols h2 http/1.1
</IfModule>

# Unknown host names get no website (404). Let's Encrypt can still validate
# any name that points here, for the panel's own certificate.
<VirtualHost *:80>
    ServerName default.invalid
    DocumentRoot {{.EmptyRoot}}
    Include {{.SnippetDir}}/acme.conf
    RewriteEngine On
    RewriteRule ^/\.well-known/acme-challenge/ - [L]
    RewriteRule ^ - [R=404,L]
</VirtualHost>

<IfModule ssl_module>
    <VirtualHost *:443>
        ServerName default.invalid
        DocumentRoot {{.EmptyRoot}}
        SSLEngine on
        SSLCertificateFile {{.SnippetDir}}/default.crt
        SSLCertificateKeyFile {{.SnippetDir}}/default.key
        RewriteEngine On
        RewriteRule ^ - [R=404,L]
    </VirtualHost>
</IfModule>

IncludeOptional {{.SitesDir}}/*.conf
`)

type apacheHookData struct {
	WWW, EmptyRoot, SnippetDir, SitesDir string
}

var apacheVhostTmpl = func() *template.Template {
	t := mustApacheTemplate("apache-vhost", apacheVhostText)
	template.Must(t.New("names").Parse(apacheNames))
	template.Must(t.New("body").Parse(apacheBody))
	template.Must(t.New("githook").Parse(apacheGitHook))
	template.Must(t.New("open").Parse(apacheOpen))
	template.Must(t.New("custom").Parse(apacheCustom))
	template.Must(t.New("exempt").Parse(apacheExempt))
	return t
}()

const apacheVhostText = `# Managed by {{app}}. Do not edit: the panel rewrites this file.
# Site: {{.Domain}}
<VirtualHost *:80>
{{- template "names" .}}
{{- if and .SSL .ForceHTTPS}}
    DocumentRoot {{.EmptyRoot}}
    Include {{.SnippetDir}}/acme.conf
{{- template "githook" .}}

{{- template "exempt" .}}

    # Everything else moves to HTTPS, with its path and query.
    RewriteCond %{THE_REQUEST} ^\S+\s+(/[^\s?]*)
    RewriteRule ^ https://%{SERVER_NAME}%1 [R=301,L,NE]
    RewriteRule ^/?(.*)$ https://%{SERVER_NAME}/$1 [R=301,L]
{{- else}}
{{template "body" .}}
{{- end}}
</VirtualHost>
{{- if .SSL}}

<VirtualHost *:443>
{{- template "names" .}}
    SSLEngine on
    SSLCertificateFile {{.CertFile}}
    SSLCertificateKeyFile {{.KeyFile}}
{{template "body" .}}
</VirtualHost>
{{- end}}
`

const apacheNames = `
    ServerName {{.Domain}}
{{- range .Aliases}}
    ServerAlias {{.}}
{{- end}}`

// apacheOpen grants access to a location (the webhook, Let's Encrypt),
// still refusing the blocked addresses.
const apacheOpen = `
{{- if .BlockedIPs}}
        <RequireAll>
            Require all granted
            Require not ip {{join .BlockedIPs " "}}
        </RequireAll>
{{- else}}
        Require all granted
{{- end}}`

// apacheGitHook forwards a repository's push webhook to the panel's local
// listener, without password, firewall or the site's .htaccess files.
const apacheGitHook = `

    # Deploy from Git on push. The panel checks the webhook's signature.
    ProxyPass /.panel/deploy/{{.ID}} http://127.0.0.1:7809/git/{{.ID}} timeout=30
    <Location /.panel/deploy/{{.ID}}>
        ProxyPreserveHost On
        RequestHeader set X-Real-IP expr=%{REMOTE_ADDR}
{{- template "open" .}}
{{- if .WAF}}
        <IfModule security2_module>
            SecRuleEngine Off
        </IfModule>
{{- end}}
    </Location>`

// apacheExempt starts the virtual host's rewrite rules: Let's Encrypt and
// the Git webhook pass untouched, the webhook with a body of at most 1 MB.
// The body limits use Content-Length: Apache's LimitRequestBody does not
// apply to requests it proxies (PHP-FPM, apps, the webhook).
const apacheExempt = `

    RewriteEngine On
    RewriteRule ^/\.well-known/acme-challenge/ - [L]
    RewriteCond expr "%{HTTP:Content-Length} -gt 1048576"
    RewriteRule ^/\.panel/deploy/{{.ID}}$ - [R=413,L]
    RewriteRule ^/\.panel/deploy/{{.ID}}$ - [L]`

const apacheCustom = `
{{- if .CustomFile}}

    # Your own directives (Websites → Apache configuration).
    Include {{.CustomFile}}
{{- end}}`

const apacheBody = `
{{- if .Files}}
    DocumentRoot {{.Public}}
{{- else}}
    DocumentRoot {{.EmptyRoot}}
{{- end}}
    CustomLog {{.LogDir}}/{{.Domain}}.access.log panel_combined
    ErrorLog {{.LogDir}}/{{.Domain}}.error.log
    Include {{.SnippetDir}}/acme.conf
{{- if .WAF}}
    <IfModule security2_module>
        SecRuleEngine {{if eq .WAF "detect"}}DetectionOnly{{else}}On{{end}}
    </IfModule>
{{- end}}

    # The panel's rules: Apache applies them after the site's .htaccess
    # files, which cannot undo them.
    <Location />
        # PHP requests reach this site's own PHP-FPM pool only, and without
        # PHP settings from .htaccess (SetEnv PHP_VALUE…).
        ProxyFCGISetEnvIf "{{if .PHPSocket}}note('uds_path') != '{{.PHPSocket}}'{{else}}true{{end}}" SCRIPT_FILENAME /nonexistent
        ProxyFCGISetEnvIf "true" !PHP_VALUE
        ProxyFCGISetEnvIf "true" !PHP_ADMIN_VALUE
{{- if or .AuthFile .BlockedIPs}}
{{- if .AuthFile}}
        AuthType Basic
        AuthName "Restricted"
        AuthBasicProvider file
        AuthUserFile {{.AuthFile}}
{{- end}}
        AuthMerging And
        Satisfy All
        <RequireAll>
            Require {{if .AuthFile}}valid-user{{else}}all granted{{end}}
{{- if .BlockedIPs}}
            Require not ip {{join .BlockedIPs " "}}
{{- end}}
        </RequireAll>
{{- end}}
    </Location>
{{- if .Files}}

    # Hide .git, .env and other dot files.
    <LocationMatch "/\.(?!well-known/)">
        Require all denied
    </LocationMatch>
{{- end}}
{{- if .AuthFile}}

    # Let's Encrypt validates without the password.
    <Location /.well-known/acme-challenge/>
{{- template "open" .}}
    </Location>
{{- end}}
{{- template "githook" .}}
{{- template "exempt" .}}
{{- if and .BodyLimit (or .Files .ProxyPort)}}
    RewriteCond expr "%{HTTP:Content-Length} -gt {{.BodyLimit}}"
    RewriteRule ^ - [R=413,L]
{{- end}}
{{- if .Disabled}}
    ErrorDocument 503 "This website is temporarily unavailable."
    RewriteRule ^ - [R=503,L]
{{- else if .RedirectURL}}
    RewriteCond %{THE_REQUEST} ^\S+\s+(/[^\s?]*)
    RewriteRule ^ {{rwesc .RedirectURL}}%1 [R=301,L,NE]
    RewriteRule ^/?(.*)$ {{rwesc .RedirectURL}}/$1 [R=301,L]
{{- else if .ProxyPort}}

    ProxyPreserveHost On
    RequestHeader set X-Real-IP expr=%{REMOTE_ADDR}
    RequestHeader set X-Forwarded-Proto expr=%{REQUEST_SCHEME}
    ProxyPass /.well-known/acme-challenge/ !
    ProxyPass / http://127.0.0.1:{{.ProxyPort}}/ upgrade=websocket timeout=300
    ProxyPassReverse / http://127.0.0.1:{{.ProxyPort}}/
{{- template "custom" .}}
{{- else}}
{{- with .ServerRules}}
{{.}}
{{- end}}
{{- if .Hotlink}}

    # Hotlink protection: media only for this site's pages, the sites listed
    # in the panel and search engines.
    RewriteCond %{HTTP_REFERER} ^https?:// [NC]
    RewriteCond %{HTTP_REFERER} !^https?://(?:{{.HotlinkHosts}})(?::[0-9]+)?(?:[/?#]|$) [NC]
    RewriteCond %{HTTP_REFERER} !\.(?:google|bing|duckduckgo|yahoo)\. [NC]
    RewriteRule \.(?:jpe?g|png|gif|webp|avif|svg|ico|bmp|mp4|webm|mp3|ogg|pdf)$ - [F,NC]
{{- end}}

    <Directory {{.Public}}>
        # Follow a symlink only if it has the same owner as its target: a
        # site (over SFTP, say) cannot link to another site's files. The
        # site's .htaccess files work, but cannot change Options; lines they
        # may not use, and unknown ones such as php_value, are ignored
        # (logged in the error log) instead of breaking the site.
        Options SymLinksIfOwnerMatch
        AllowOverride AuthConfig FileInfo Indexes Limit Nonfatal=All
        Require all granted
        DirectoryIndex {{if .PHPSocket}}index.php {{end}}index.html index.htm
{{- if .PHPSocket}}
        # Only .php files run: /uploads/a.jpg/x.php is a.jpg, never PHP.
        <FilesMatch "\.php$">
            SetHandler "proxy:unix:{{.PHPSocket}}|fcgi://localhost"
        </FilesMatch>
{{- end}}
{{- with .DirRules}}
{{.}}
{{- end}}
    </Directory>
{{- if .PHPSocket}}
    ProxyTimeout {{.TimeoutSec}}
{{- end}}
{{- if .Custom404}}
    ErrorDocument 404 /404.html
{{- end}}
{{- template "custom" .}}
{{- end}}`

// apacheVhost adds what the Apache template computes to vhostData.
type apacheVhost struct {
	vhostData
	Files        bool  // the site serves its public folder (static or PHP)
	BodyLimit    int64 // largest request body in bytes, 0 for no limit
	DirRules     string
	ServerRules  string
	HotlinkHosts string // regular expression of the allowed referrer hosts
}

func renderApacheVhost(d vhostData, rewrite string) ([]byte, error) {
	if rewrite != "" {
		if _, ok := rewritePreset(rewrite); !ok {
			return nil, fmt.Errorf("refusing to render unknown rewrite preset %q", rewrite)
		}
	}
	if err := d.check(); err != nil {
		return nil, err
	}
	if d.UploadMB <= 0 {
		d.UploadMB = 64
	}
	if d.TimeoutSec <= 0 {
		d.TimeoutSec = 300
	}
	if d.EmptyRoot == "" {
		return nil, fmt.Errorf("refusing to render %q without an empty document root", d.Domain)
	}
	a := apacheVhost{vhostData: d}
	a.Files = !d.Disabled && d.RedirectURL == "" && d.ProxyPort == 0
	a.BodyLimit = int64(d.UploadMB) << 20
	rules := apacheRewriteRules(rewrite, d.PHPSocket != "")
	a.DirRules, a.ServerRules = rules.Dir, rules.Server
	if d.Hotlink {
		a.HotlinkHosts = hostRegex(slices.Concat([]string{d.Domain}, d.Aliases, d.HotlinkAllow))
	}
	return render(apacheVhostTmpl, a)
}

// ---- rewrite presets ----

// apacheRules are a preset's rules for Apache. Dir goes into the site's
// <Directory> block: the front controller, which the application's own
// .htaccess file usually provides too and may replace. Server holds the
// rules that protect files; they run before any .htaccess file.
type apacheRules struct{ Dir, Server string }

const apacheFallbackPHP = `        FallbackResource /index.php`

var apachePresetRules = map[string]apacheRules{
	"wordpress": {Dir: apacheFallbackPHP, Server: `
    # Files in uploads never run as PHP.
    RewriteRule ^/wp-content/uploads/.*\.php$ - [F,NC]`},
	"laravel":     {Dir: apacheFallbackPHP},
	"symfony":     {Dir: apacheFallbackPHP},
	"codeigniter": {Dir: apacheFallbackPHP},
	"drupal": {Dir: apacheFallbackPHP, Server: `
    # Private files, and scripts in files and vendor, are never served.
    RewriteRule ^/sites/.*/private/ - [F]
    RewriteRule ^/sites/[^/]+/files/.*\.php$ - [F]
    RewriteRule /vendor/.*\.php$ - [F]`},
	"joomla": {Dir: apacheFallbackPHP, Server: `
    # Scripts in these folders never run.
    RewriteRule ^/(?:images|cache|media|logs|tmp)/.*\.(?:php|pl|py|jsp|asp|sh|cgi)$ - [F,NC]`},
	"opencart": {Dir: `        RewriteEngine On
        RewriteBase /
        RewriteCond %{REQUEST_FILENAME} !-f
        RewriteCond %{REQUEST_FILENAME} !-d
        RewriteRule ^(.+)$ index.php?_route_=$1 [L,QSA]`, Server: `
    # Templates, settings and logs are never served.
    RewriteRule \.(?:twig|tpl|ini|log)$ - [F,NC]`},
	"nextcloud": {Dir: `        FallbackResource /index.php
        AddType text/javascript .mjs`, Server: `
    # Service discovery for calendars and contacts.
    RewriteRule ^/\.well-known/carddav$ /remote.php/dav/ [R=301,L]
    RewriteRule ^/\.well-known/caldav$ /remote.php/dav/ [R=301,L]
    RewriteRule ^/\.well-known/ /index.php%{REQUEST_URI} [R=301,L]
    # Nextcloud's own folders are never served.
    RewriteRule ^/(?:build|tests|config|lib|3rdparty|templates|data)(?:$|/) - [R=404,L]
    RewriteRule ^/(?:\.|autotest|occ|issue|indie|db_|console) - [R=404,L]`},
	"moodle": {Dir: `        FallbackResource /r.php`},
	"spa":    {Dir: `        FallbackResource /index.html`},
	"static-clean": {Dir: `        RewriteEngine On
        RewriteCond %{REQUEST_FILENAME} !-f
        RewriteCond %{REQUEST_FILENAME}.html -f
        RewriteRule ^ %{REQUEST_URI}.html [L]`},
}

// apacheRewriteRules returns the Apache rules for a site, like
// rewriteRules does for Nginx: PHP sites send unknown paths to index.php by
// default; a preset that needs PHP falls back to the default without it.
func apacheRewriteRules(id string, php bool) apacheRules {
	if p, ok := rewritePreset(id); ok && (php || !p.PHP) {
		return apachePresetRules[id]
	}
	if php {
		return apacheRules{Dir: apacheFallbackPHP}
	}
	return apacheRules{}
}

// apachePresetList is RewritePresets with each preset's Apache rules.
func apachePresetList() []RewritePreset {
	out := make([]RewritePreset, len(RewritePresets))
	for i, p := range RewritePresets {
		r := apachePresetRules[p.ID]
		var b strings.Builder
		if r.Dir != "" {
			b.WriteString("# In the site's <Directory> block; the site's .htaccess may replace these:\n")
			b.WriteString(dedent(r.Dir))
		}
		if r.Server != "" {
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString("# For the whole site, before any .htaccess file:\n")
			b.WriteString(dedent(strings.TrimLeft(r.Server, "\n")))
		}
		p.Rules = b.String()
		out[i] = p
	}
	return out
}

func dedent(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimLeft(l, " ")
	}
	return strings.Join(lines, "\n")
}

// ---- Adminer ----

var apacheAdminerTmpl = mustApacheTemplate("apache-adminer", `# Managed by {{app}}. Adminer, reachable only through the panel.
Listen 127.0.0.1:{{.Port}}
<VirtualHost 127.0.0.1:{{.Port}}>
    ServerName adminer.invalid
    DocumentRoot {{.Dir}}
    ProxyTimeout 600

    # Only requests with the panel's token get in; Adminer never sees it.
    RewriteEngine On
    RewriteCond %{HTTP:X-Panel-Adminer} !={{.Token}}
    RewriteRule ^ - [R=404,L]
    RewriteCond expr "%{HTTP:Content-Length} -gt 268435456"
    RewriteRule ^ - [R=413,L]
    RewriteRule ^ {{.Dir}}/index.php [L]
    SetEnvIf X-Panel-Https "^on$" HTTPS=on
    RequestHeader unset X-Panel-Adminer
    RequestHeader unset X-Panel-Https

    <Directory {{.Dir}}>
        Options None
        AllowOverride None
        Require all granted
        <FilesMatch "\.php$">
            SetHandler "proxy:unix:/run/php/php{{.PHP}}-fpm.sock|fcgi://localhost"
        </FilesMatch>
    </Directory>
</VirtualHost>
`)
