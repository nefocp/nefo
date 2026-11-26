package hosting

import (
	"bytes"
	"fmt"
	htmltemplate "html/template"
	"regexp"
	"slices"
	"strings"
	"text/template"

	"github.com/nefocp/nefo/internal/validate"
	"github.com/nefocp/nefo/internal/version"
)

// Config files are rendered from templates. Every value that goes into a
// template has passed the allow-lists in internal/validate, and vhostData /
// poolData check them again before rendering, so a bug elsewhere can never
// inject directives into Nginx or PHP configuration.

var funcs = template.FuncMap{"app": func() string { return version.AppName }}

func mustTemplate(name, text string) *template.Template {
	return template.Must(template.New(name).Funcs(funcs).Option("missingkey=error").Parse(text))
}

// nginxFeatures depend on the installed Nginx version.
type nginxFeatures struct {
	// HTTP2Directive: "http2 on;" exists from 1.25.1; older versions use "listen … http2".
	HTTP2Directive bool
	// RejectHandshake: ssl_reject_handshake exists from 1.19.4.
	RejectHandshake bool
	IPv6            bool
}

func featuresFor(nginxVersion string, ipv6 bool) nginxFeatures {
	return nginxFeatures{
		HTTP2Directive:  versionAtLeast(nginxVersion, "1.25.1"),
		RejectHandshake: versionAtLeast(nginxVersion, "1.19.4"),
		IPv6:            ipv6,
	}
}

// ---- shared Nginx snippets ----

var acmeSnippetTmpl = mustTemplate("acme", `# Managed by {{app}}. Do not edit: the panel rewrites this file.
# Answers Let's Encrypt HTTP-01 challenges for every site.
location ^~ /.well-known/acme-challenge/ {
    auth_basic off;
    root {{.Webroot}};
    default_type text/plain;
    try_files $uri =404;
}
`)

var sslSnippetTmpl = mustTemplate("ssl", `# Managed by {{app}}. Do not edit: the panel rewrites this file.
ssl_protocols TLSv1.2 TLSv1.3;
ssl_prefer_server_ciphers off;
ssl_session_cache shared:PanelTLS:10m;
ssl_session_timeout 1d;
ssl_session_tickets off;
`)

// hookTmpl goes into conf.d, which Debian and Ubuntu include inside the http
// block. Its default servers answer unknown host names with nothing, so the
// server's IP address shows no page and no certificate.
var hookTmpl = mustTemplate("hook", `# Managed by {{app}}. Do not edit: the panel rewrites this file.
server {
    listen 80 default_server;
{{- if .IPv6}}
    listen [::]:80 default_server;
{{- end}}
    server_name _;
    # Let's Encrypt can validate any name that points here (for the panel's
    # own certificate); everything else gets no answer.
    include {{.SnippetDir}}/acme.conf;
    location / {
        return 444;
    }
}
{{- if .RejectHandshake}}

server {
    listen 443 ssl{{if not .HTTP2Directive}} http2{{end}} default_server;
{{- if .IPv6}}
    listen [::]:443 ssl{{if not .HTTP2Directive}} http2{{end}} default_server;
{{- end}}
    server_name _;
    ssl_reject_handshake on;
}
{{- end}}

# WebSocket support for app sites.
map $http_upgrade $panel_connection_upgrade {
    default upgrade;
    ''      close;
}
{{- if .Gzip}}

# Compression for CSS, JavaScript, JSON, SVG and fonts too: Debian and
# Ubuntu compress HTML only. What nginx.conf sets itself is left out here,
# since a directive set twice in the http block is an error.
{{- range .Gzip}}
{{.}}
{{- end}}
{{- end}}

{{- if .WAF}}

# Web application firewall: ModSecurity with the OWASP Core Rule Set.
include {{.SnippetDir}}/waf/http.conf;
{{- end}}

include {{.SitesDir}}/*.conf;
`)

type hookData struct {
	nginxFeatures
	SitesDir   string
	SnippetDir string
	WAF        bool
	Gzip       []string // compression directives, see gzipDirectives
}

// gzipSettings are the compression directives the panel wants in the http
// block. text/html is always compressed once gzip is on, and nginx warns
// when gzip_types lists it again.
var gzipSettings = []struct{ name, line string }{
	{"gzip", "gzip on;"},
	{"gzip_vary", "gzip_vary on;"},
	{"gzip_proxied", "gzip_proxied any;"},
	{"gzip_comp_level", "gzip_comp_level 5;"},
	{"gzip_min_length", "gzip_min_length 256;"},
	{"gzip_types", "gzip_types text/css text/plain text/xml text/javascript application/javascript application/json " +
		"application/ld+json application/manifest+json application/xml application/rss+xml application/atom+xml " +
		"application/wasm image/svg+xml image/x-icon font/ttf font/otf application/vnd.ms-fontobject;"},
}

// gzipDirectives returns the compression directives that the other files
// of the http block (nginx.conf and the rest of conf.d, given as their
// text) don't set already. Commented-out lines don't count.
func gzipDirectives(httpConf string) []string {
	set := map[string]bool{}
	for _, line := range strings.Split(httpConf, "\n") {
		line, _, _ = strings.Cut(line, "#")
		if f := strings.Fields(strings.TrimSpace(line)); len(f) > 0 {
			set[f[0]] = true
		}
	}
	var out []string
	for _, g := range gzipSettings {
		if !set[g.name] {
			out = append(out, g.line)
		}
	}
	return out
}

// ---- site virtual host ----

var vhostTmpl = func() *template.Template {
	t := mustTemplate("vhost", vhostText)
	template.Must(t.New("body").Parse(vhostBody))
	template.Must(t.New("custom").Parse(vhostCustom))
	template.Must(t.New("ratelimit").Parse(vhostRateLimit))
	template.Must(t.New("githook").Parse(vhostGitHook))
	template.Must(t.New("pagecache").Parse(vhostPageCache))
	return t
}()

const vhostText = `# Managed by {{app}}. Do not edit: the panel rewrites this file.
# Site: {{.Domain}}
{{- if .RateLimit}}
limit_req_zone $binary_remote_addr zone={{.RateZone}}:1m rate={{.RateLimit}}r/s;
{{- end}}
{{- if .CacheZone}}
# Page cache of this site's PHP pages: purging empties the folder.
fastcgi_cache_path {{.CacheDir}} levels=1:2 keys_zone={{.CacheZone}}:4m max_size=256m inactive=60m use_temp_path=off;
{{- end}}
server {
    listen 80;
{{- if .IPv6}}
    listen [::]:80;
{{- end}}
    server_name {{.ServerNames}};

    include {{.SnippetDir}}/acme.conf;
{{- if and .SSL .ForceHTTPS}}
{{- template "githook" .}}

    location / {
        return 301 https://$host$request_uri;
    }
{{- else}}
{{template "body" .}}
{{- end}}
}
{{- if .SSL}}

server {
    listen 443 ssl{{if not .HTTP2Directive}} http2{{end}};
{{- if .IPv6}}
    listen [::]:443 ssl{{if not .HTTP2Directive}} http2{{end}};
{{- end}}
{{- if .HTTP2Directive}}
    http2 on;
{{- end}}
    server_name {{.ServerNames}};

    ssl_certificate     {{.CertFile}};
    ssl_certificate_key {{.KeyFile}};
    include {{.SnippetDir}}/ssl.conf;
{{- if .HSTS}}
    # Browsers use only HTTPS for this site from now on. The site's
    # locations have no add_header of their own, so they all inherit it.
    add_header Strict-Transport-Security "{{.HSTS}}" always;
{{- end}}
{{template "body" .}}
}
{{- end}}
`

// vhostGitHook forwards a repository's push webhook (Websites, Git) to the
// panel's local listener, which checks its signature. Every site has it, so
// saving or removing Git settings needs no vhost rewrite: the listener answers
// 404 for a site without settings. An exact match wins over the dot-file rule.
const vhostGitHook = `

    # Deploy from Git on push. The panel checks the webhook's signature.
    location = /.panel/deploy/{{.ID}} {
        auth_basic off;
{{- if .WAF}}
        modsecurity off;
{{- end}}
        client_max_body_size 1m;
        proxy_pass http://127.0.0.1:7809/git/{{.ID}};
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_read_timeout 30s;
    }`

const vhostBody = `
{{- range .BlockedIPs}}
    deny {{.}};
{{- end}}
{{- if .WAF}}
    modsecurity on;
{{- if eq .WAF "detect"}}
    modsecurity_rules 'SecRuleEngine DetectionOnly';
{{- end}}
{{- end}}
{{- if .AuthFile}}
    auth_basic "Restricted";
    auth_basic_user_file {{.AuthFile}};
{{- end}}
{{- template "githook" .}}
{{- if .Disabled}}
    access_log {{.LogDir}}/{{.Domain}}.access.log;
    error_log {{.LogDir}}/{{.Domain}}.error.log;

    location / {
        default_type text/plain;
        return 503 "This website is temporarily unavailable.";
    }
{{- else if .RedirectURL}}
    access_log {{.LogDir}}/{{.Domain}}.access.log;
    error_log {{.LogDir}}/{{.Domain}}.error.log;

    location / {
        return 301 {{.RedirectURL}}$request_uri;
    }
{{- else if .ProxyPort}}
    access_log {{.LogDir}}/{{.Domain}}.access.log;
    error_log {{.LogDir}}/{{.Domain}}.error.log;
    client_max_body_size {{.UploadMB}}m;

    location / {
        proxy_pass http://127.0.0.1:{{.ProxyPort}};
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $panel_connection_upgrade;
        proxy_read_timeout 300s;
{{- template "ratelimit" .}}
    }
{{- template "custom" .}}
{{- else}}
    root {{.Public}};
    # Follow a symlink only if it has the same owner as its target: a site
    # (over SFTP, say) cannot link to another site's files.
    disable_symlinks if_not_owner from=$document_root;
    index {{if .PHPSocket}}index.php {{end}}index.html index.htm;
    access_log {{.LogDir}}/{{.Domain}}.access.log;
    error_log {{.LogDir}}/{{.Domain}}.error.log;
    client_max_body_size {{.UploadMB}}m;
{{- template "pagecache" .}}

    # Hide .git, .env and other dot files.
    location ~ /\.(?!well-known/) {
        deny all;
    }
{{- template "custom" .}}
{{- if .Custom404}}

    error_page 404 /404.html;
{{- end}}

{{.RewriteRules}}
{{- if .Hotlink}}

    # Hotlink protection: media only for this site's pages, the sites listed
    # in the panel and search engines.
    location ~* \.(?:jpe?g|png|gif|webp|avif|svg|ico|bmp|mp4|webm|mp3|ogg|pdf)$ {
        valid_referers none blocked server_names{{range .HotlinkAllow}} {{.}}{{end}} ~\.google\. ~\.bing\. ~\.duckduckgo\. ~\.yahoo\.;
        if ($invalid_referer) {
            return 403;
        }
        try_files $uri {{if .PHPSocket}}/index.php?$args{{else}}=404{{end}};
    }
{{- end}}
{{- if .PHPSocket}}

    # PATH_INFO URLs (/index.php/page) work; only existing .php files run.
    location ~ [^/]\.php(/|$) {
        fastcgi_split_path_info ^(.+?\.php)(/.*)$;
        set $path_info $fastcgi_path_info;
        try_files $fastcgi_script_name =404;
        include fastcgi_params;
        fastcgi_param PATH_INFO $path_info;
        fastcgi_param SCRIPT_FILENAME $realpath_root$fastcgi_script_name;
        fastcgi_param DOCUMENT_ROOT $realpath_root;
        fastcgi_pass unix:{{.PHPSocket}};
        fastcgi_read_timeout {{.TimeoutSec}}s;
{{- if .CacheZone}}
        fastcgi_cache {{.CacheZone}};
        fastcgi_cache_key $scheme$request_method$host$request_uri;
        fastcgi_cache_valid 200 301 10m;
        fastcgi_cache_valid 404 1m;
        fastcgi_cache_use_stale error timeout updating invalid_header http_500 http_503;
        fastcgi_cache_background_update on;
        fastcgi_cache_lock on;
        fastcgi_cache_bypass $panel_no_cache;
        fastcgi_no_cache $panel_no_cache;
{{- end}}
{{- template "ratelimit" .}}
    }
{{- end}}
{{- end}}`

// vhostCustom includes the administrator's own directives. They come before
// the preset's locations, so their regex locations take precedence.
// vhostRateLimit limits PHP and app requests per IP; static files are
// cheap and pages load many of them at once, so they are not counted.
const vhostRateLimit = `
{{- if .RateLimit}}
        limit_req zone={{.RateZone}} burst={{.RateBurst}} nodelay;
        limit_req_status 429;
{{- end}}`

// vhostPageCache decides which PHP requests the page cache may answer (see
// the PHP location). It sets X-Cache-Status for the server: no location of
// the vhost has an add_header of its own, which would drop it.
const vhostPageCache = `
{{- if .CacheZone}}

    # Page cache: PHP pages are kept for 10 minutes (404 pages for 1), but
    # not for requests other than GET and HEAD, URLs with a query string,
    # visitors who are signed in or have a cart, and the admin, login, API
    # and shop pages.
    set $panel_no_cache 0;
    if ($request_method !~ "^(?:GET|HEAD)$") {
        set $panel_no_cache 1;
    }
    if ($query_string != "") {
        set $panel_no_cache 1;
    }
    if ($http_cookie ~* "wordpress_logged_in_|wp-postpass_|comment_author_|woocommerce_items_in_cart|wp_woocommerce_session_|PHPSESSID") {
        set $panel_no_cache 1;
    }
    if ($request_uri ~* "/(?:wp-admin/|wp-login\.php|xmlrpc\.php|wp-json/|wp-cron\.php|cart/|checkout/|my-account/)") {
        set $panel_no_cache 1;
    }
    # HIT, MISS, BYPASS…; static files get none.
    add_header X-Cache-Status $upstream_cache_status always;
{{- end}}`

const vhostCustom = `
{{- if .CustomFile}}

    # Your own directives (Websites → Nginx configuration).
    include {{.CustomFile}};
{{- end}}`

type vhostData struct {
	nginxFeatures
	ID         int64 // the webhook location /.panel/deploy/<ID>
	Domain     string
	Aliases    []string
	Public     string
	LogDir     string
	SnippetDir string
	// EmptyRoot is Apache's DocumentRoot when the site serves no files
	// (suspended, redirected or an app): a folder without site files.
	EmptyRoot   string
	PHPSocket   string // empty for a static site
	SSL         bool
	ForceHTTPS  bool
	CertFile    string
	KeyFile     string
	ServerNames string
	UploadMB    int
	TimeoutSec  int
	ProxyPort   int
	Disabled    bool
	RedirectURL string
	AuthFile    string
	// RewriteRules are the location rules from rewriteRules; CustomFile is
	// the administrator's own directives, if any.
	RewriteRules string
	CustomFile   string
	// Options (see siteopts.go).
	Custom404    bool
	Hotlink      bool
	HotlinkAllow []string
	// Firewall (see waf.go).
	WAF        string
	RateLimit  int
	RateBurst  int
	RateZone   string
	BlockedIPs []string
	// User is the site's Linux user and PHPWorkers the size of its PHP-FPM
	// pool: OpenLiteSpeed's virtual host and FastCGI connection limit.
	User       string
	PHPWorkers int
	// HSTS is the Strict-Transport-Security value of the HTTPS server, or
	// "" (see siteHSTS).
	HSTS string
	// CacheZone and CacheDir turn on Nginx's page cache (FastCGI cache) for
	// a PHP site (see sitehttps.go).
	CacheZone string
	CacheDir  string
}

func (d *vhostData) check() error {
	if err := validate.Domain(d.Domain); err != nil {
		return fmt.Errorf("refusing to render %q: %w", d.Domain, err)
	}
	for _, n := range d.Aliases {
		if err := validate.Alias(n); err != nil {
			return fmt.Errorf("refusing to render %q: %w", n, err)
		}
	}
	if d.RedirectURL != "" && !redirectRe.MatchString(d.RedirectURL) {
		return fmt.Errorf("refusing to render redirect %q", d.RedirectURL)
	}
	if err := checkFirewall(d.WAF, d.RateLimit, d.RateZone, d.BlockedIPs); err != nil {
		return err
	}
	for _, n := range d.HotlinkAllow {
		if err := validate.Alias(n); err != nil {
			return fmt.Errorf("refusing to render hotlink name %q: %w", n, err)
		}
	}
	if !knownRules(d.RewriteRules) {
		return fmt.Errorf("refusing to render unknown rewrite rules")
	}
	if d.User != "" && validate.SystemUsername(d.User) != nil {
		return fmt.Errorf("refusing to render user %q", d.User)
	}
	if d.PHPWorkers < 0 || d.PHPWorkers > 200 {
		return fmt.Errorf("refusing to render %d PHP workers", d.PHPWorkers)
	}
	if d.HSTS != "" && ((d.HSTS != hstsValue && d.HSTS != hstsValueSubdomains) || !d.SSL || !d.ForceHTTPS) {
		return fmt.Errorf("refusing to render HSTS %q", d.HSTS)
	}
	if (d.CacheZone != "" || d.CacheDir != "") && (!cacheZoneRe.MatchString(d.CacheZone) || d.CacheDir == "" || d.PHPSocket == "") {
		return fmt.Errorf("refusing to render the page cache %q in %q", d.CacheZone, d.CacheDir)
	}
	for _, p := range []string{d.Public, d.LogDir, d.SnippetDir, d.EmptyRoot, d.PHPSocket, d.CertFile, d.KeyFile, d.AuthFile, d.CustomFile, d.CacheDir} {
		if !safePath(p) {
			return fmt.Errorf("refusing to render unsafe path %q", p)
		}
	}
	return nil
}

func renderVhost(d vhostData) ([]byte, error) {
	if d.RewriteRules == "" {
		d.RewriteRules = rewriteRules("", d.PHPSocket != "")
	}
	if err := d.check(); err != nil {
		return nil, err
	}
	d.ServerNames = strings.Join(append([]string{d.Domain}, d.Aliases...), " ")
	if d.UploadMB <= 0 {
		d.UploadMB = 64
	}
	if d.TimeoutSec <= 0 {
		d.TimeoutSec = 300
	}
	return render(vhostTmpl, d)
}

// ---- PHP-FPM pool ----

// poolTmpl runs each site's PHP as the site's own Linux user, confined to the
// site directory with open_basedir, with its own temp and session folders.
// pm = ondemand starts PHP workers only when requests come in, which keeps
// small servers with many quiet sites within their memory.
var poolTmpl = mustTemplate("pool", `; Managed by {{app}}. Do not edit: the panel rewrites this file.
; Site: {{.Domain}}
[{{.User}}]
user = {{.User}}
group = {{.User}}
listen = {{.Socket}}
listen.owner = www-data
listen.group = www-data
listen.mode = 0660

{{- if eq .PM "dynamic"}}
pm = dynamic
pm.max_children = {{.Workers}}
pm.start_servers = {{.Start}}
pm.min_spare_servers = 1
pm.max_spare_servers = {{.Spare}}
{{- else}}
pm = ondemand
pm.max_children = {{.Workers}}
pm.process_idle_timeout = 30s
{{- end}}
pm.max_requests = 500

chdir = /
security.limit_extensions = .php
env[TMPDIR] = {{.Root}}/tmp

php_admin_value[open_basedir] = {{.Root}}/
php_admin_value[sys_temp_dir] = {{.Root}}/tmp
php_admin_value[upload_tmp_dir] = {{.Root}}/tmp
php_admin_value[session.save_path] = {{.Root}}/tmp
php_admin_value[error_log] = {{.Root}}/logs/php-error.log
php_admin_flag[log_errors] = on
{{- if .Disabled}}
php_admin_value[disable_functions] = {{.Disabled}}
{{- end}}
php_value[upload_max_filesize] = {{.UploadMB}}M
php_value[post_max_size] = {{.UploadMB}}M
php_value[memory_limit] = {{.MemoryMB}}M
php_value[max_execution_time] = {{.TimeSec}}
{{- if .MaxInputVars}}
php_value[max_input_vars] = {{.MaxInputVars}}
{{- end}}
`)

type poolData struct {
	Domain   string
	User     string
	Root     string
	Socket   string
	MemoryMB int
	UploadMB int
	TimeSec  int
	// PM is "ondemand" (workers start with requests) or "dynamic" (some
	// stay ready); Workers is pm.max_children.
	PM      string
	Workers int
	Start   int
	Spare   int
	// Allow lists the HardenedFunctions this site may use; all others stay
	// disabled (an empty list disables them all).
	Allow []string
	// Disabled is computed by renderPool.
	Disabled string
	// MaxInputVars, when set, raises max_input_vars (Moodle needs 5000).
	MaxInputVars int
}

// HardenedFunctions are disabled for every site unless allowed back: they
// let a hacked site run programs on the server.
var HardenedFunctions = []string{"exec", "passthru", "shell_exec", "system", "proc_open", "popen", "pcntl_exec"}

// disabledFor returns the disable_functions value for a site that may use
// the functions in allow.
func disabledFor(allow []string) string {
	var out []string
	for _, f := range HardenedFunctions {
		if !slices.Contains(allow, f) {
			out = append(out, f)
		}
	}
	return strings.Join(out, ",")
}

var disabledRe = regexp.MustCompile(`^[a-z_,]*$`)

func renderPool(d poolData) ([]byte, error) {
	if err := validate.Domain(d.Domain); err != nil {
		return nil, fmt.Errorf("refusing to render %q: %w", d.Domain, err)
	}
	if err := validate.SystemUsername(d.User); err != nil {
		return nil, fmt.Errorf("refusing to render user %q: %w", d.User, err)
	}
	if !safePath(d.Root) || !safePath(d.Socket) || d.Root == "" || d.Socket == "" {
		return nil, fmt.Errorf("refusing to render unsafe path %q or %q", d.Root, d.Socket)
	}
	if d.MemoryMB <= 0 {
		d.MemoryMB, d.UploadMB, d.TimeSec = 256, 64, 300
	}
	if d.PM == "" {
		d.PM = "ondemand"
	}
	if d.Workers <= 0 {
		d.Workers = 5
	}
	for _, f := range d.Allow {
		if !slices.Contains(HardenedFunctions, f) {
			return nil, fmt.Errorf("refusing to allow %q", f)
		}
	}
	d.Disabled = disabledFor(d.Allow)
	if (d.PM != "ondemand" && d.PM != "dynamic") || d.Workers > 200 || !disabledRe.MatchString(d.Disabled) {
		return nil, fmt.Errorf("refusing to render pool settings %q %d %q", d.PM, d.Workers, d.Disabled)
	}
	d.Start, d.Spare = min(2, d.Workers), max(1, d.Workers/2)
	return render(poolTmpl, d)
}

// ---- placeholder page ----

var welcomeTmpl = htmltemplate.Must(htmltemplate.New("welcome").Parse(`<!doctype html>
<html lang="en">
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Domain}}</title>
<body style="font-family:system-ui,sans-serif;max-width:40rem;margin:4rem auto;padding:0 1rem;line-height:1.5">
<h1>{{.Domain}} is ready</h1>
<p>This placeholder page was created by the server's control panel. Upload your website to replace it.</p>
</body>
</html>
`))

func renderWelcome(domain string) ([]byte, error) {
	var b bytes.Buffer
	err := welcomeTmpl.Execute(&b, struct{ Domain string }{domain})
	return b.Bytes(), err
}

// ---- helpers ----

func render(t *template.Template, data any) ([]byte, error) {
	var b bytes.Buffer
	if err := t.Execute(&b, data); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// safePath allows the characters of absolute paths built from validated
// domains and user names, and nothing that Nginx or PHP-FPM would parse
// (spaces, quotes, semicolons, braces, $, newlines).
func safePath(p string) bool {
	for _, r := range p {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '/', r == '.', r == '-', r == '_':
		default:
			return false
		}
	}
	return !strings.Contains(p, "..")
}

// versionAtLeast compares dotted version numbers such as "1.24.0".
func versionAtLeast(have, want string) bool {
	h, w := strings.Split(have, "."), strings.Split(want, ".")
	for i := 0; i < len(w); i++ {
		var hv, wv int
		if i < len(h) {
			fmt.Sscanf(h[i], "%d", &hv)
		}
		fmt.Sscanf(w[i], "%d", &wv)
		if hv != wv {
			return hv > wv
		}
	}
	return true
}
