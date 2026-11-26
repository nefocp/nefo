package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/monitor"
)

// alertEvents are audit actions that also send an alert, when enabled.
var alertEvents = map[string]string{
	"backup_failed":           "A backup failed",
	"restore_failed":          "A restore failed",
	"ssl_renew_failed":        "A website certificate could not be renewed",
	"panel_cert_renew_failed": "The panel certificate could not be renewed",
}

// watchedServices are the services whose failure sends an alert.
func (s *Server) watchedServices(ctx context.Context) map[string]string {
	out := map[string]string{}
	for _, svc := range s.services.List(ctx) {
		if svc.Name == "nginx" || svc.Name == "apache2" || svc.Name == "lshttpd" || svc.Name == "mariadb" || svc.Name == "mysql" || svc.Name == "redis-server" || svc.Name == "fail2ban" ||
			svc.Name == "memcached" || strings.HasPrefix(svc.Name, "postgresql@") ||
			strings.HasSuffix(svc.Name, "-fpm") {
			// PostgreSQL's clusters start through postgresql.service, so their
			// own units are not enabled.
			if svc.Enabled || strings.HasPrefix(svc.Name, "postgresql@") {
				out[svc.Name] = svc.Active
			}
		}
	}
	return out
}

// RunMonitor records metrics and checks alerts until ctx is done.
func (s *Server) RunMonitor(ctx context.Context) { s.monitor.Run(ctx) }

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request, a *authed) {
	hours, _ := strconv.Atoi(r.URL.Query().Get("hours"))
	if hours < 1 || hours > 168 {
		hours = 24
	}
	span := time.Duration(hours) * time.Hour
	since := time.Now().Add(-span)
	points, err := s.store.Metrics(r.Context(), since, span/240)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	rx, tx, err := s.store.NetTotals(r.Context(), since)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	// net: bytes received and sent in the range, from the one-minute samples.
	writeJSON(w, http.StatusOK, map[string]any{"points": points, "net": map[string]float64{"rx": rx, "tx": tx}})
}

func (s *Server) handleGetAlerts(w http.ResponseWriter, r *http.Request, a *authed) {
	st, err := s.monitor.Settings(r.Context())
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleSaveAlerts(w http.ResponseWriter, r *http.Request, a *authed) {
	var req monitor.Settings
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.monitor.Save(r.Context(), req); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "alerts_saved", clientIP(r), "")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleTestAlerts(w http.ResponseWriter, r *http.Request, a *authed) {
	ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
	defer cancel()
	writeJSON(w, http.StatusOK, map[string]any{"results": s.monitor.Test(ctx)})
}
