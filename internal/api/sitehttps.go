package api

import "net/http"

// Turning HTTPS off, HSTS and the page cache of a website (see
// internal/hosting/sitehttps.go).

// handleRemoveSSL turns HTTPS off: the site is served over HTTP only and
// its certificate files are deleted.
func (s *Server) handleRemoveSSL(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := siteID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "site_not_found")
		return
	}
	site, err := s.hosting.RemoveCertificate(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "ssl_removed", clientIP(r), site.Domain)
	writeJSON(w, http.StatusOK, s.siteJSON(site))
}

func (s *Server) handleSetHSTS(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := siteID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "site_not_found")
		return
	}
	var req struct {
		Enabled           bool `json:"enabled"`
		IncludeSubdomains bool `json:"include_subdomains"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	site, err := s.hosting.SetHSTS(r.Context(), id, req.Enabled, req.IncludeSubdomains)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	action := "hsts_off"
	if req.Enabled {
		action = "hsts_on"
	}
	s.audit(r.Context(), a.user.Username, action, clientIP(r), site.Domain)
	writeJSON(w, http.StatusOK, s.siteJSON(site))
}

func (s *Server) handleSetPageCache(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := siteID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "site_not_found")
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	site, err := s.hosting.SetPageCache(r.Context(), id, req.Enabled)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	action := "page_cache_off"
	if req.Enabled {
		action = "page_cache_on"
	}
	s.audit(r.Context(), a.user.Username, action, clientIP(r), site.Domain)
	writeJSON(w, http.StatusOK, s.siteJSON(site))
}

func (s *Server) handlePurgePageCache(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := siteID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "site_not_found")
		return
	}
	site, err := s.hosting.PurgePageCache(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "page_cache_purged", clientIP(r), site.Domain)
	writeJSON(w, http.StatusOK, s.siteJSON(site))
}
