package api

import (
	"context"
	"net/http"
	"time"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/uptime"
)

// RunUptime checks the monitored websites until ctx is done. Where the
// panel can't host websites (a development machine), nothing is checked.
func (s *Server) RunUptime(ctx context.Context) {
	if !s.hosting.Supported() {
		return
	}
	s.uptime.Run(ctx)
}

// siteMonitorResponse is a website's monitoring: its settings, where it
// stands, the last 24 hours and 7 days, its response times over the last 24
// hours and its last failures.
type siteMonitorResponse struct {
	Settings uptime.Settings     `json:"settings"`
	Paused   bool                `json:"paused"` // on, but the website is suspended
	Status   *uptime.Status      `json:"status,omitempty"`
	Day      store.UptimeStats   `json:"day"`
	Week     store.UptimeStats   `json:"week"`
	Points   []store.UptimePoint `json:"points"`
	Failures []store.UptimeCheck `json:"failures"`
}

func (s *Server) siteMonitorJSON(ctx context.Context, site *store.Site) (*siteMonitorResponse, error) {
	m, err := s.store.SiteMonitor(ctx, site.ID)
	if err != nil {
		return nil, err
	}
	res := &siteMonitorResponse{
		Settings: uptime.Settings{Enabled: m.Enabled, Interval: m.Interval, Path: m.Path, Keyword: m.Keyword, Strict4xx: m.Strict4xx},
		Paused:   m.Enabled && site.Disabled,
	}
	if st, ok := s.uptime.Status(site.ID); ok {
		res.Status = &st
	}
	now := time.Now()
	if res.Day, err = s.store.UptimeStats(ctx, site.ID, now.Add(-24*time.Hour)); err != nil {
		return nil, err
	}
	if res.Week, err = s.store.UptimeStats(ctx, site.ID, now.Add(-7*24*time.Hour)); err != nil {
		return nil, err
	}
	// About 288 points: one per 5 minutes, or per check when they are rarer.
	step := max(time.Duration(m.Interval)*time.Minute, 5*time.Minute)
	if res.Points, err = s.store.UptimeSeries(ctx, site.ID, now.Add(-24*time.Hour), step); err != nil {
		return nil, err
	}
	res.Failures, err = s.store.UptimeChecks(ctx, site.ID, now.Add(-7*24*time.Hour), 10, true)
	return res, err
}

func (s *Server) handleGetSiteMonitor(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := siteID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "site_not_found")
		return
	}
	site, err := s.store.SiteByID(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	res, err := s.siteMonitorJSON(r.Context(), site)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// handleSaveSiteMonitor saves a website's monitoring settings. When they
// turn it on, the website is checked at once (10 seconds at most), so the
// answer already has its status.
func (s *Server) handleSaveSiteMonitor(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := siteID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "site_not_found")
		return
	}
	var req uptime.Settings
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := req.Validate(); err != nil {
		s.opError(w, r, err)
		return
	}
	site, err := s.store.SiteByID(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	m := &store.SiteMonitor{SiteID: id, Enabled: req.Enabled, Interval: req.Interval, Path: req.Path, Keyword: req.Keyword, Strict4xx: req.Strict4xx}
	if err := s.store.SaveSiteMonitor(r.Context(), m); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "site_monitor_saved", clientIP(r), site.Domain)
	if s.hosting.Supported() {
		s.uptime.CheckNow(r.Context(), id) // reloads the settings, also when turned off
	}
	res, err := s.siteMonitorJSON(r.Context(), site)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// handleUptimeSummary lists where each monitored website stands, for the
// website list and the bell.
func (s *Server) handleUptimeSummary(w http.ResponseWriter, r *http.Request, a *authed) {
	writeJSON(w, http.StatusOK, map[string]any{"sites": s.uptime.Summary()})
}
