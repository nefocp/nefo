package hosting

// The reverse proxy part of the virtual hosts (see proxysite.go). The
// site's access options come before it in each template: a suspended or
// redirected site never reaches the upstream, and the password, the
// blocked addresses and the firewall apply as for other sites.

// vhostUpstream is Nginx's. The request's path goes after the upstream's
// (proxy_pass with $request_uri), and a host name is looked up when
// requests come in, with the resolver: a name that does not resolve at
// boot never keeps Nginx from starting.
const vhostUpstream = `
    access_log {{.LogDir}}/{{.Domain}}.access.log;
    error_log {{.LogDir}}/{{.Domain}}.error.log;
    client_max_body_size {{.UploadMB}}m;

    location / {
{{- with .Upstream}}
{{- if .Resolver}}
        resolver {{.Resolver}} valid=60s{{if not $.IPv6}} ipv6=off{{end}};
        resolver_timeout 5s;
{{- end}}
        proxy_pass {{.URL}}$request_uri;
        proxy_http_version 1.1;
        proxy_set_header Host {{if .HostHeader}}{{.HostHeader}}{{else}}$host{{end}};
{{- if .Forwarded}}
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-Host $host;
{{- else}}
        proxy_set_header X-Real-IP "";
        proxy_set_header X-Forwarded-For "";
        proxy_set_header X-Forwarded-Proto "";
        proxy_set_header X-Forwarded-Host "";
{{- end}}
{{- if .WebSocket}}
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $panel_connection_upgrade;
{{- else}}
        proxy_set_header Connection "";
{{- end}}
        proxy_connect_timeout {{.ConnectTimeout}}s;
        proxy_send_timeout {{.Timeout}}s;
        proxy_read_timeout {{.Timeout}}s;
{{- range .RedirectFrom}}
        proxy_redirect {{.}} /;
{{- end}}
{{- if .HTTPS}}
        proxy_ssl_server_name on;
        proxy_ssl_name {{if .HostHeader}}{{.Name}}{{else}}$host{{end}};
{{- if .Insecure}}
        # The upstream's certificate is not checked (Websites → Reverse proxy).
        proxy_ssl_verify off;
{{- else}}
        proxy_ssl_verify on;
        proxy_ssl_verify_depth 4;
        proxy_ssl_trusted_certificate ` + caFile + `;
{{- end}}
{{- end}}
{{- end}}
{{- template "ratelimit" .}}
    }
{{- template "custom" .}}`

// apacheUpstream is Apache's.
const apacheUpstream = `
{{- with .Upstream}}

    ProxyPreserveHost {{if .HostHeader}}Off{{else}}On{{end}}
{{- if .Forwarded}}
    RequestHeader set X-Real-IP expr=%{REMOTE_ADDR}
    RequestHeader set X-Forwarded-Proto expr=%{REQUEST_SCHEME}
{{- else}}
    ProxyAddHeaders Off
    RequestHeader unset X-Real-IP
    RequestHeader unset X-Forwarded-For
    RequestHeader unset X-Forwarded-Proto
    RequestHeader unset X-Forwarded-Host
{{- end}}
{{- if .HTTPS}}
    SSLProxyEngine On
{{- if .Insecure}}
    # The upstream's certificate is not checked (Websites → Reverse proxy).
    SSLProxyVerify none
    SSLProxyCheckPeerCN Off
    SSLProxyCheckPeerName Off
    SSLProxyCheckPeerExpire Off
{{- else}}
    SSLProxyVerify require
    SSLProxyVerifyDepth 4
    SSLProxyCACertificateFile ` + caFile + `
{{- end}}
{{- end}}
    ProxyPass /.well-known/acme-challenge/ !
    ProxyPass / {{.URL}}/{{if .WebSocket}} upgrade=websocket{{end}} timeout={{.Timeout}} connectiontimeout={{.ConnectTimeout}}
    ProxyPassReverse / {{.URL}}/
{{- end}}
{{- template "custom" .}}`

// olsUpstream is OpenLiteSpeed's proxy handler and, for an http://
// upstream, its WebSocket proxy (OpenLiteSpeed's WebSocket proxy cannot use
// TLS, and it passes the request's path without the upstream's).
// olsUpstreamContext sends the site's requests to the handler with a [P]
// rewrite in the context "/", whose realm and headers apply to them: the
// upstream's path goes in front of the request's, and Proxy-Host is the
// upstream's Host header. A [P] rule of the virtual host itself would skip
// the context's password. OpenLiteSpeed never checks an upstream's
// certificate, adds no X-Forwarded-For with the visitor's address and has
// no time limit for a proxied answer (initTimeout is for new connections).
const olsUpstream = `
{{- with .Upstream}}

  extprocessor proxy_{{$.User}} {
    type                  proxy
    address               {{.OLSAddress}}
    maxConns              100
    pcKeepAliveTimeout    60
    initTimeout           {{.Timeout}}
    retryTimeout          0
    respBuffer            0
  }
{{- if and .WebSocket (not .HTTPS)}}
  websocket / {
    address               {{.OLSAddress}}
  }
{{- end}}
{{- end}}`

const olsUpstreamContext = `

  context / {
    location              {{.EmptyRoot}}/
    allowBrowse           1
{{- template "auth" .}}
    # OpenLiteSpeed cannot set X-Real-IP to the client's address: a forged
    # one is dropped.
    extraHeaders          <<<END_extraHeaders
RequestHeader unset X-Real-IP
{{- if not (and .Upstream.WebSocket (not .Upstream.HTTPS))}}
RequestHeader unset Upgrade
{{- end}}
{{- if not .Upstream.Forwarded}}
RequestHeader unset X-Forwarded-For
{{- end}}
{{- if .HSTS}}
Strict-Transport-Security: {{.HSTS}}
{{- end}}
END_extraHeaders
    rewrite {
      enable              1
      rules               <<<END_rules
RewriteRule ^/?(.*)$ http://proxy_{{.User}}{{olsrwesc .Upstream.Path}}/$1 [P,L{{with .Upstream.OLSProxyHost}},E=Proxy-Host:{{.}}{{end}}]
END_rules
    }
  }`
