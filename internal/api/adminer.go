package api

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/nefocp/nefo/internal/hosting"
)

// isAdminer reports whether a request (path after the entrance) is for the
// Adminer database UI, which has its own CSP and CSRF tokens.
func isAdminer(r *http.Request) bool { return strings.HasPrefix(r.URL.Path, "/adminer/") }

// isPhpMyAdmin is isAdminer for phpMyAdmin.
func isPhpMyAdmin(r *http.Request) bool { return strings.HasPrefix(r.URL.Path, "/phpmyadmin/") }

// isDBTool reports whether a request is for one of the database web UIs
// behind the sign-in, which send their own CSP and check their own CSRF
// tokens.
func isDBTool(r *http.Request) bool { return isAdminer(r) || isPhpMyAdmin(r) }

// handleAdminer forwards signed-in requests to Adminer on its internal port.
func (s *Server) handleAdminer(w http.ResponseWriter, r *http.Request, a *authed) {
	addr, token := s.hosting.AdminerTarget(r.Context())
	s.proxyDBTool(w, r, "Adminer", addr, hosting.AdminerTokenHeader, token)
}

// handlePhpMyAdmin forwards signed-in requests to phpMyAdmin on its
// internal port. Like Adminer's, the route is for browser sessions only
// (s.sessionOnly): an API key cannot reach it.
func (s *Server) handlePhpMyAdmin(w http.ResponseWriter, r *http.Request, a *authed) {
	addr, token := s.hosting.PhpMyAdminTarget(r.Context())
	s.proxyDBTool(w, r, "phpMyAdmin", addr, hosting.PhpMyAdminTokenHeader, token)
}

// proxyDBTool forwards a request to a database web UI's internal server at
// addr with its secret token in tokenHeader. The panel's own cookies stay
// behind.
func (s *Server) proxyDBTool(w http.ResponseWriter, r *http.Request, name, addr, tokenHeader, token string) {
	if addr == "" {
		http.NotFound(w, r)
		return
	}
	entrance := s.cfg.Entrance
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			// The tools build their links and cookie paths from the full path.
			pr.Out.URL = &url.URL{Scheme: "http", Host: addr, Path: "/" + entrance + pr.In.URL.Path, RawQuery: pr.In.URL.RawQuery}
			pr.Out.Host = pr.In.Host // they compare the Origin with the Host
			// A browser cannot pass either tool's token along.
			pr.Out.Header.Del(hosting.AdminerTokenHeader)
			pr.Out.Header.Del(hosting.PhpMyAdminTokenHeader)
			pr.Out.Header.Set(tokenHeader, token)
			pr.Out.Header.Del(hosting.AdminerHTTPSHeader)
			if pr.In.TLS != nil {
				pr.Out.Header.Set(hosting.AdminerHTTPSHeader, "on")
			}
			pr.Out.Header.Del("Cookie")
			// Pass on the tool's own cookies only, never the panel's session.
			for _, c := range pr.In.Cookies() {
				if !strings.HasPrefix(c.Name, "panel_") {
					pr.Out.AddCookie(c)
				}
			}
		},
		ModifyResponse: func(resp *http.Response) error {
			// The panel already set these (securityHeaders); drop the tool's
			// copies so each is sent once, and do not reveal the server version.
			for _, h := range []string{"Referrer-Policy", "X-Frame-Options", "X-Content-Type-Options", "Server"} {
				resp.Header.Del(h)
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			s.log.Warn("database tool proxy", "tool", name, "err", err)
			http.Error(w, name+" is not available", http.StatusBadGateway)
		},
	}
	proxy.ServeHTTP(w, r)
}
