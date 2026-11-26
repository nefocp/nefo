package hosting

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"text/template"
)

// OpenLiteSpeed configuration. Each site is one self-contained virtualHost
// block in its own file: it names its domains and listeners, so the
// server's file only includes the folder.
//
//   - The panel's rules (suspension, redirects, HTTPS, body limits, hotlink
//     protection, the presets' protective rules, dot files) are virtual-host
//     rewrite rules, which run before any context's rules.
//   - The site's .htaccess rewrite rules (filtered by the panel, see
//     ols_htaccess.go) are the rules of the context of their folder, like
//     OpenLiteSpeed's own .htaccess support; for PHP sites the front
//     controller follows them.
//   - A symlink is followed only if it has the owner of its target, and
//     only files of the site's user, root or www-data are served.

func mustOLSTemplate(name, text string) *template.Template {
	return template.Must(template.New(name).Funcs(funcs).Funcs(apacheFuncs).Funcs(olsFuncs).Option("missingkey=error").Parse(text))
}

var olsFuncs = template.FuncMap{
	// olsrwesc escapes text for a rewrite substitution: %N and $N are
	// back-references, and OpenLiteSpeed URL-decodes the literal parts once,
	// so a literal % is written as an escaped %25.
	"olsrwesc": func(s string) string { return strings.NewReplacer("%", `\%25`, "$", `\$`).Replace(s) },
}

// olsMainTmpl is conf/httpd_config.conf, the file OpenLiteSpeed reads. The
// package's upgrade script reads the run-as account from it: the lines
// below must stay the only ones with those words.
var olsMainTmpl = mustOLSTemplate("ols-main", `# Managed by {{app}}. Do not edit: the panel rewrites this file.
# OpenLiteSpeed runs as www-data, like Nginx and Apache; the rest of its
# settings are in the included file.
user                      www-data
group                     www-data
include                   {{.Hook}}
`)

type olsHookData struct {
	Home, SnippetDir, SitesDir, EmptyRoot, DataDir, Hook string
	IPv6, WAF                                            bool
}

var olsHookTmpl = mustOLSTemplate("ols-hook", `# Managed by {{app}}. Do not edit: the panel rewrites this file.
# OpenLiteSpeed's settings, included from conf/httpd_config.conf. The
# WebAdmin console is off: it would listen on port 7080 and change these
# files behind the panel's back.
disableWebAdmin           1
priority                  0
autoRestart               1
enableChroot              0
inMemBufSize              60M
swappingDir               /tmp/lshttpd/swap
autoFix503                1
gracefulRestartTimeout    300
mime                      conf/mime.properties
showVersionNumber         0
indexFiles                index.html

errorlog {{.Home}}/logs/error.log {
  logLevel                WARN
  debugLevel              0
  rollingSize             10M
  keepDays                7
  enableStderrLog         1
}

accesslog {{.Home}}/logs/access.log {
  rollingSize             10M
  keepDays                7
  compressArchive         1
}

tuning {
  maxConnections          10000
  maxSSLConnections       10000
  connTimeout             300
  maxKeepAliveReq         10000
  keepAliveTimeout        5
  maxReqURLLen            32768
  maxReqHeaderSize        65536
  maxReqBodySize          2047M
  maxDynRespHeaderSize    32768
  maxDynRespSize          2047M
  enableGzipCompress      1
  enableDynGzipCompress   1
  gzipCompressLevel       6
  compressibleTypes       default
  quicEnable              1
  quicShmDir              /dev/shm/ols
}

# Follow a symlink only if it has the same owner as its target, and serve
# only files of the site's own account (or root): a site (over SFTP, say)
# cannot reach another site's files.
fileAccessControl {
  followSymbolLink        2
  checkSymbolLink         1
  forceStrictOwnership    1
  requiredPermissionMask  000
  restrictedPermissionMask 000
}

accessDenyDir {
  dir                     /
  dir                     /etc/*
  dir                     /dev/*
  dir                     /root/*
  dir                     {{.DataDir}}/*
  dir                     {{.Home}}/conf/*
  dir                     {{.Home}}/admin/conf/*
  dir                     {{.SnippetDir}}/*
}

accessControl {
  allow                   ALL
}

# The sites' Git push webhooks go to the panel's listener.
extprocessor panel_githook {
  type                    proxy
  address                 {{.Hook}}
  maxConns                10
  initTimeout             30
  retryTimeout            0
  respBuffer              0
}

listener http {
  address                 {{if .IPv6}}[ANY]{{else}}*{{end}}:80
  secure                  0
  map                     _default *
}

listener https {
  address                 {{if .IPv6}}[ANY]{{else}}*{{end}}:443
  secure                  1
  keyFile                 {{.SnippetDir}}/default.key
  certFile                {{.SnippetDir}}/default.crt
  sslProtocol             24
  enableQuic              1
  map                     _default *
}

# Unknown host names get no website (404). Let's Encrypt can still validate
# any name that points here, for the panel's own certificate.
virtualHost _default {
  vhRoot                  {{.EmptyRoot}}/
  user                    www-data
  group                   www-data
  allowSymbolLink         0
  enableScript            0
  restrained              1
  docRoot                 {{.EmptyRoot}}/
  context /.well-known/acme-challenge/ {
    location              {{.EmptyRoot}}/.well-known/acme-challenge/
    allowBrowse           1
    forceType             text/plain
  }
  rewrite {
    enable                1
    autoLoadHtaccess      0
    rules                 <<<END_rules
RewriteRule ^/\.well-known/acme-challenge/ - [L]
RewriteRule ^ - [R=404,L]
END_rules
  }
}

include                   {{.SitesDir}}/*.conf

# LiteSpeed Cache: on for the server, off for every site until its
# application asks for it (the LiteSpeed Cache plugin, or a .htaccess rule
# with E=cache-control:max-age=...).
module cache {
  ls_enabled              1
  checkPrivateCache       1
  checkPublicCache        1
  maxCacheObjSize         10000000
  maxStaleAge             200
  qsCache                 1
  reqCookieCache          1
  respCookieCache         1
  ignoreReqCacheCtrl      1
  ignoreRespCacheCtrl     0
  enableCache             0
  expireInSeconds         3600
  enablePrivateCache      0
  privateExpireInSeconds  3600
}
{{- if .WAF}}

# Web application firewall: ModSecurity with the OWASP Core Rule Set,
# loaded once; each site turns it on.
module mod_security {
  modsecurity             on
  modsecurity_rules_file  {{.SnippetDir}}/waf/main.conf
  ls_enabled              0
}
{{- end}}
`)

var olsVhostTmpl = func() *template.Template {
	t := mustOLSTemplate("ols-vhost", olsVhostText)
	template.Must(t.New("auth").Parse(olsAuth))
	return t
}()

const olsAuth = `
{{- if .AuthFile}}
    realm                 site
    authName              Restricted
    required              valid-user
{{- end}}`

const olsVhostText = `# Managed by {{app}}. Do not edit: the panel rewrites this file.
# Site: {{.Domain}}
virtualHost {{.Domain}} {
  vhRoot                  {{.Root}}/
  vhDomain                {{.Domain}}
{{- if .Aliases}}
  vhAliases               {{join .Aliases ", "}}
{{- end}}
  listeners               http{{if .SSL}}, https{{end}}
  user                    {{.User}}
  group                   {{.User}}
  allowSymbolLink         2
  forceStrictOwnership    1
  enableScript            1
  restrained              1
  setUIDMode              0
  docRoot                 {{if .Files}}{{.Public}}{{else}}{{.EmptyRoot}}{{end}}/
  enableGzip              1

  errorlog {{.LogDir}}/{{.Domain}}.error.log {
    useServer             0
    logLevel              ERROR
    rollingSize           0
  }
  # Nginx's "combined" format, which the traffic statistics read.
  accesslog {{.LogDir}}/{{.Domain}}.access.log {
    useServer             0
    logFormat             %h %l %u %t "%r" %>s %b "%{Referer}i" "%{User-Agent}i"
    rollingSize           0
  }
{{- if .SSL}}

  vhssl {
    keyFile               {{.KeyFile}}
    certFile              {{.CertFile}}
    certChain             1
    sslProtocol           24
    enableQuic            1
  }
{{- end}}
{{- if .WAF}}

  module mod_security {
    ls_enabled            1
    modsecurity_rules     SecRuleEngine {{if eq .WAF "detect"}}DetectionOnly{{else}}On{{end}}
  }
{{- end}}
{{- if .BlockedIPs}}

  accessControl {
    allow                 ALL
    deny                  {{join .BlockedIPs ", "}}
  }
{{- end}}
{{- if .AuthFile}}

  realm site {
    userDB {
      location            {{.AuthFile}}
    }
  }
{{- end}}
{{- if and .Files .PHPSocket}}

  # PHP runs in the site's own PHP-FPM pool. Only existing .php files run:
  # /uploads/a.jpg/x.php is a.jpg with a PATH_INFO, which is refused.
  extprocessor php_{{.User}} {
    type                  fcgi
    address               uds:/{{.PHPSocket}}
    maxConns              {{.Workers}}
    initTimeout           {{.TimeoutSec}}
    retryTimeout          0
    persistConn           0
    respBuffer            0
    autoStart             0
  }
  scripthandler {
    add                   fcgi:php_{{.User}} php
  }
{{- end}}
{{- if .Files}}

  index {
    useServer             0
    indexFiles            {{if .PHPSocket}}index.php, {{end}}index.html, index.htm
    autoIndex             0
  }
{{- if .Custom404}}

  errorpage 404 {
    url                   /404.html
  }
{{- end}}
{{- end}}
{{- if .Disabled}}

  errorpage 503 {
    url                   /.panel-error/503.html
  }
  context /.panel-error/ {
    location              {{.ErrorDir}}/
    allowBrowse           1
  }
{{- end}}
{{- if .ProxyPort}}

  extprocessor app_{{.User}} {
    type                  proxy
    address               127.0.0.1:{{.ProxyPort}}
    maxConns              100
    pcKeepAliveTimeout    60
    initTimeout           300
    retryTimeout          0
    respBuffer            0
  }
  websocket / {
    address               127.0.0.1:{{.ProxyPort}}
  }
{{- end}}

  # Let's Encrypt HTTP-01 challenges, for every site.
  context /.well-known/acme-challenge/ {
    location              {{.EmptyRoot}}/.well-known/acme-challenge/
    allowBrowse           1
    forceType             text/plain
  }
{{- if .ProxyPort}}

  context / {
    type                  proxy
    handler               app_{{.User}}
    addDefaultCharset     off
{{- template "auth" .}}
    # OpenLiteSpeed cannot set X-Real-IP to the client's address: a forged
    # one is dropped. The app finds the client in X-Forwarded-For.
    extraHeaders          <<<END_extraHeaders
RequestHeader unset X-Real-IP
END_extraHeaders
  }
{{- else if .Files}}

  context / {
    location              {{.Public}}/
    allowBrowse           1
{{- template "auth" .}}
{{- with .RootRules}}
    rewrite {
      enable              1
      rules               <<<END_rules
{{.}}
END_rules
    }
{{- end}}
  }
{{- range .Contexts}}

  # The rewrite rules of {{.Location}}.htaccess
  context {{.URI}} {
    location              {{.Location}}
    allowBrowse           1
    rewrite {
      enable              1
      rules               <<<END_rules
{{.Rules}}
END_rules
    }
  }
{{- end}}
{{- end}}

  # The panel's rules: they run before the site's .htaccess rules, which
  # cannot undo them. The Git webhook and Let's Encrypt pass first.
  rewrite {
    enable                1
    autoLoadHtaccess      0
    rules                 <<<END_rules
RewriteRule ^/\.well-known/acme-challenge/ - [L]
RewriteCond %{HTTP:Content-Length} -gt1048576
RewriteRule ^/\.panel/deploy/{{.ID}}$ - [R=413,L]
RewriteRule ^/\.panel/deploy/{{.ID}}$ http://panel_githook/git/{{.ID}} [P,L]
{{- if and .SSL .ForceHTTPS}}
RewriteCond %{HTTPS} !on
RewriteCond %{THE_REQUEST} ^\S+\s+(/[^\s?]*)
RewriteRule ^ https://%{SERVER_NAME}%1 [R=301,L,NE]
RewriteCond %{HTTPS} !on
RewriteRule ^/?(.*)$ https://%{SERVER_NAME}/$1 [R=301,L]
{{- end}}
{{- if and .BodyLimit (or .Files .ProxyPort)}}
RewriteCond %{HTTP:Content-Length} -gt{{.BodyLimit}}
RewriteRule ^ - [R=413,L]
{{- end}}
{{- if .Disabled}}
RewriteRule ^/\.panel-error/ - [L]
RewriteRule ^ - [R=503,L]
{{- else if .RedirectURL}}
RewriteCond %{THE_REQUEST} ^\S+\s+(/[^\s?]*)
RewriteRule ^ {{olsrwesc .RedirectURL}}%1 [R=301,L,NE]
RewriteRule ^/?(.*)$ {{olsrwesc .RedirectURL}}/$1 [R=301,L]
{{- else if .Files}}
RewriteRule /\.(?!well-known/) - [F]
{{- with .ServerRules}}
{{.}}
{{- end}}
{{- if .Hotlink}}
RewriteCond %{HTTP_REFERER} ^https?:// [NC]
RewriteCond %{HTTP_REFERER} !^https?://(?:{{.HotlinkHosts}})(?::[0-9]+)?(?:[/?#]|$) [NC]
RewriteCond %{HTTP_REFERER} !\.(?:google|bing|duckduckgo|yahoo)\. [NC]
RewriteRule \.(?:jpe?g|png|gif|webp|avif|svg|ico|bmp|mp4|webm|mp3|ogg|pdf)$ - [F,NC]
{{- end}}
{{- end}}
END_rules
  }
{{- if .CustomFile}}

  # Your own directives (Websites → OpenLiteSpeed configuration).
  include                 {{.CustomFile}}
{{- end}}
}
`

// olsHtContext is a folder below the site's public folder whose .htaccess
// file has rewrite rules.
type olsHtContext struct {
	URI      string // "/blog/"
	Location string // "<public>/blog/"
	Rules    string
}

// olsHtaccess are a site's .htaccess rewrite rules, filtered.
type olsHtaccess struct {
	Root     string // of <public>/.htaccess
	Contexts []olsHtContext
}

// olsVhost adds what the OpenLiteSpeed template computes to vhostData.
type olsVhost struct {
	vhostData
	ErrorDir string
	Htaccess olsHtaccess

	Root         string // the site's folder, above public
	Files        bool   // the site serves its public folder (static or PHP)
	Workers      int    // FastCGI connections: the PHP-FPM pool's size
	BodyLimit    int64
	RootRules    string
	Contexts     []olsHtContext
	ServerRules  string
	HotlinkHosts string
}

var olsContextURIRe = regexp.MustCompile(`^/(?:[A-Za-z0-9_~@+-][A-Za-z0-9._~@+-]{0,127}/)*$`)

func renderOLSVhost(v olsVhost, rewrite string) ([]byte, error) {
	if rewrite != "" {
		if _, ok := rewritePreset(rewrite); !ok {
			return nil, fmt.Errorf("refusing to render unknown rewrite preset %q", rewrite)
		}
	}
	if err := v.check(); err != nil {
		return nil, err
	}
	if v.User == "" {
		return nil, fmt.Errorf("refusing to render %q without its user", v.Domain)
	}
	if v.EmptyRoot == "" || v.Public == "" || v.LogDir == "" || !safePath(v.ErrorDir) || v.ErrorDir == "" {
		return nil, fmt.Errorf("refusing to render %q without its folders", v.Domain)
	}
	if v.UploadMB <= 0 {
		v.UploadMB = 64
	}
	if v.TimeoutSec <= 0 {
		v.TimeoutSec = 300
	}
	v.Workers = v.PHPWorkers
	if v.Workers <= 0 {
		v.Workers = 5
	}
	v.Root = path.Dir(v.Public)
	v.Files = !v.Disabled && v.RedirectURL == "" && v.ProxyPort == 0
	v.BodyLimit = int64(v.UploadMB) << 20
	rules := olsRewriteRules(rewrite, v.PHPSocket != "")
	v.ServerRules = rules.Server
	if v.Files {
		v.RootRules = joinRules(v.Htaccess.Root, rules.Dir)
		for _, c := range v.Htaccess.Contexts {
			if !olsContextURIRe.MatchString(c.URI) || c.URI == "/" || c.Location != v.Public+c.URI ||
				!safePath(c.Location) || !safeRules(c.Rules) {
				return nil, fmt.Errorf("refusing to render the .htaccess context %q", c.URI)
			}
			if rules.Nested {
				c.Rules = joinRules(c.Rules, rules.Dir)
			}
			v.Contexts = append(v.Contexts, c)
		}
		if !safeRules(v.RootRules) {
			return nil, fmt.Errorf("refusing to render the .htaccess rules of %q", v.Domain)
		}
	}
	if v.Hotlink {
		v.HotlinkHosts = hostRegex(slices.Concat([]string{v.Domain}, v.Aliases, v.HotlinkAllow))
	}
	return render(olsVhostTmpl, v)
}

func joinRules(parts ...string) string {
	var keep []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			keep = append(keep, p)
		}
	}
	return strings.Join(keep, "\n")
}

// safeRules reports whether rules can go into a "rules <<<END_rules" block:
// no line may end it early.
func safeRules(rules string) bool {
	for _, l := range strings.Split(rules, "\n") {
		if strings.EqualFold(strings.TrimSpace(l), "END_rules") || strings.Contains(l, "\r") || strings.ContainsRune(l, 0) {
			return false
		}
	}
	return len(rules) <= olsMaxRules
}

// olsMaxRules keeps a rules block within OpenLiteSpeed's limit for
// multi-line values (64 KB).
const olsMaxRules = 48 << 10

// ---- rewrite presets ----

// olsRules are a preset's rules for OpenLiteSpeed. Dir follows the site's
// own .htaccess rules in its root folder's context (the front controller,
// which the application's .htaccess usually provides too); with Nested it
// also follows the rules of the folders that have their own .htaccess.
// Server holds the virtual-host rules that protect files.
type olsRules struct {
	Dir    string
	Nested bool
	Server string
}

const (
	olsFallbackPHP = `RewriteCond %{REQUEST_FILENAME} !-f
RewriteCond %{REQUEST_FILENAME} !-d
RewriteRule ^ /index.php [L]`
	olsFallbackHTML = `RewriteCond %{REQUEST_FILENAME} !-f
RewriteCond %{REQUEST_FILENAME} !-d
RewriteRule ^ /index.html [L]`
)

var olsPresetDir = map[string]olsRules{
	"wordpress":   {Dir: olsFallbackPHP, Nested: true},
	"laravel":     {Dir: olsFallbackPHP, Nested: true},
	"symfony":     {Dir: olsFallbackPHP, Nested: true},
	"codeigniter": {Dir: olsFallbackPHP, Nested: true},
	"drupal":      {Dir: olsFallbackPHP, Nested: true},
	"joomla":      {Dir: olsFallbackPHP, Nested: true},
	"opencart": {Dir: `RewriteCond %{REQUEST_FILENAME} !-f
RewriteCond %{REQUEST_FILENAME} !-d
RewriteRule ^(.+)$ /index.php?_route_=$1 [L,QSA]`},
	"nextcloud": {Dir: olsFallbackPHP, Nested: true},
	"moodle": {Dir: `RewriteCond %{REQUEST_FILENAME} !-f
RewriteCond %{REQUEST_FILENAME} !-d
RewriteRule ^ /r.php [L]`, Nested: true},
	"spa": {Dir: olsFallbackHTML, Nested: true},
	// The whole path, not a back-reference: in a folder with its own
	// .htaccess, the pattern only sees the part below that folder.
	"static-clean": {Dir: `RewriteCond %{REQUEST_FILENAME} !-f
RewriteCond %{REQUEST_FILENAME}.html -f
RewriteRule ^ %{REQUEST_URI}.html [L]`, Nested: true},
}

// olsRewriteRules returns the OpenLiteSpeed rules for a site, like
// rewriteRules does for Nginx: PHP sites send unknown paths to index.php by
// default; a preset that needs PHP falls back to the default without it.
// The protective rules are Apache's, which OpenLiteSpeed reads as they are.
func olsRewriteRules(id string, php bool) olsRules {
	if p, ok := rewritePreset(id); ok && (php || !p.PHP) {
		r := olsPresetDir[id]
		r.Server = plainRules(apachePresetRules[id].Server)
		return r
	}
	if php {
		return olsRules{Dir: olsFallbackPHP, Nested: true}
	}
	return olsRules{}
}

// plainRules drops the indentation, comments and empty lines of rules.
func plainRules(s string) string {
	var keep []string
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "#") {
			keep = append(keep, l)
		}
	}
	return strings.Join(keep, "\n")
}

// olsPresetList is RewritePresets with each preset's OpenLiteSpeed rules.
func olsPresetList() []RewritePreset {
	out := make([]RewritePreset, len(RewritePresets))
	for i, p := range RewritePresets {
		r := olsRewriteRules(p.ID, p.PHP)
		var b strings.Builder
		if r.Dir != "" {
			b.WriteString("# After the site's own .htaccess rules in its root folder:\n")
			b.WriteString(r.Dir)
		}
		if r.Server != "" {
			if b.Len() > 0 {
				b.WriteString("\n\n")
			}
			b.WriteString("# For the whole site, before any .htaccess rule:\n")
			b.WriteString(r.Server)
		}
		p.Rules = b.String()
		out[i] = p
	}
	return out
}

// ---- Adminer ----

var olsAdminerTmpl = mustOLSTemplate("ols-adminer", `# Managed by {{app}}. Adminer, reachable only through the panel.
listener panel_adminer {
  address                 127.0.0.1:{{.Port}}
  secure                  0
  map                     _adminer *
}

virtualHost _adminer {
  vhRoot                  {{.Dir}}/
  user                    www-data
  group                   www-data
  allowSymbolLink         0
  enableScript            1
  restrained              1
  docRoot                 {{.Dir}}/
  extprocessor adminer_php {
    type                  fcgi
    address               uds://run/php/php{{.PHP}}-fpm.sock
    maxConns              5
    initTimeout           600
    retryTimeout          0
    persistConn           0
    respBuffer            0
    autoStart             0
  }
  scripthandler {
    add                   fcgi:adminer_php php
  }
  index {
    useServer             0
    indexFiles            index.php
    autoIndex             0
  }
  # Only requests with the panel's token get in.
  rewrite {
    enable                1
    autoLoadHtaccess      0
    rules                 <<<END_rules
RewriteCond %{HTTP:X-Panel-Adminer} !={{.Token}}
RewriteRule ^ - [R=404,L]
RewriteCond %{HTTP:Content-Length} -gt268435456
RewriteRule ^ - [R=413,L]
RewriteCond %{HTTP:X-Panel-Https} =on
RewriteRule ^ /index.php [E=HTTPS:on,L]
RewriteRule ^ /index.php [L]
END_rules
  }
}
`)
