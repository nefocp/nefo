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

// handleAdminer forwards signed-in requests to Adminer on its internal port.
func (s *Server) handleAdminer(w http.ResponseWriter, r *http.Request, a *authed) {
	addr, token := s.hosting.AdminerTarget(r.Context())
	if addr == "" {
		http.NotFound(w, r)
		return
	}
	entrance := s.cfg.Entrance
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			// Adminer builds its links and cookie paths from the full path.
			pr.Out.URL = &url.URL{Scheme: "http", Host: addr, Path: "/" + entrance + pr.In.URL.Path, RawQuery: pr.In.URL.RawQuery}
			pr.Out.Host = pr.In.Host // Adminer compares the Origin with the Host
			pr.Out.Header.Set(hosting.AdminerTokenHeader, token)
			pr.Out.Header.Del(hosting.AdminerHTTPSHeader)
			if pr.In.TLS != nil {
				pr.Out.Header.Set(hosting.AdminerHTTPSHeader, "on")
			}
			pr.Out.Header.Del("Cookie")
			// Pass on Adminer's own cookies only, never the panel's session.
			for _, c := range pr.In.Cookies() {
				if !strings.HasPrefix(c.Name, "panel_") {
					pr.Out.AddCookie(c)
				}
			}
		},
		ModifyResponse: func(resp *http.Response) error {
			// The panel already set these (securityHeaders); drop Adminer's
			// copies so each is sent once, and do not reveal the Nginx version.
			for _, h := range []string{"Referrer-Policy", "X-Frame-Options", "X-Content-Type-Options", "Server"} {
				resp.Header.Del(h)
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			s.log.Warn("adminer proxy", "err", err)
			http.Error(w, "Adminer is not available", http.StatusBadGateway)
		},
	}
	proxy.ServeHTTP(w, r)
}
