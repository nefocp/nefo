package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/store"
)

// Country blocking and the file lock of a website (see hosting/geo.go and
// hosting/filelock.go): /api/sites/{id}/country and /api/sites/{id}/filelock.

func (s *Server) handleGetCountry(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	info, err := s.hosting.GeoStatus(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleSetCountry(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	var req hosting.CountryBlock
	if !decodeJSON(w, r, &req) {
		return
	}
	// The first use downloads the database (a few MB): it goes on if the
	// browser gives up waiting.
	ctx := context.WithoutCancel(r.Context())
	info, err := s.hosting.SetCountryBlock(ctx, id, req)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	detail := ""
	if site, err := s.store.SiteByID(ctx, id); err == nil {
		detail = site.Domain
	}
	if info.Mode != "" {
		detail += " " + info.Mode + " " + strings.Join(info.Countries, ",")
	}
	s.audit(ctx, a.user.Username, "site_country_changed", clientIP(r), truncate(detail, 500))
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleGetFileLock(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	info, err := s.hosting.FileLockStatus(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleSetFileLock(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	var req hosting.FileLock
	if !decodeJSON(w, r, &req) {
		return
	}
	// Locking a large site takes a while: it finishes even if the browser
	// stops waiting, so it never stops halfway.
	ctx := context.WithoutCancel(r.Context())
	before := s.hosting.FilesLocked(ctx, id)
	info, err := s.hosting.SetFileLock(ctx, id, req)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	action := "site_files_lock_changed"
	switch {
	case info.On && !before:
		action = "site_files_locked"
	case !info.On && before:
		action = "site_files_unlocked"
	}
	if site, err := s.store.SiteByID(ctx, id); err == nil {
		s.audit(ctx, a.user.Username, action, clientIP(r), site.Domain)
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleFileLockSnapshot(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	info, err := s.hosting.ResetFileBaseline(context.WithoutCancel(r.Context()), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	if site, err := s.store.SiteByID(r.Context(), id); err == nil {
		s.audit(r.Context(), a.user.Username, "site_files_snapshot", clientIP(r), site.Domain)
	}
	writeJSON(w, http.StatusOK, info)
}

// siteByDomain finds a site by its main domain.
func (s *Server) siteByDomain(ctx context.Context, domain string) (*store.Site, error) {
	sites, err := s.store.ListSites(ctx)
	if err != nil {
		return nil, err
	}
	for _, site := range sites {
		if site.Domain == domain {
			return site, nil
		}
	}
	return nil, store.ErrNotFound
}

// checkLockedFiles and refreshGeo run from the hourly maintenance loop:
// the daily check of locked sites' PHP files (alerts go out as events),
// locks an interrupted operation left open, and the monthly country
// database.
func (s *Server) checkLockedFiles(ctx context.Context) {
	s.hosting.HealFileLocks(ctx)
	s.hosting.CheckLockedFiles(ctx, func(domain, text string) {
		s.audit(ctx, "system", "site_files_changed", "", domain)
		s.monitor.Notify(ctx, "⚠️ "+text)
	})
}

func (s *Server) refreshGeo(ctx context.Context) { s.hosting.RefreshGeoDB(ctx) }
