package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/nefocp/nefo/internal/logstats"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
)

// statsDays are the periods the statistics offer, in days.
var statsDays = map[int]bool{1: true, 7: true, 14: true, 30: true}

// statsCache keeps each report for a minute: reading 30 days of logs can take
// a few seconds on a busy site.
var statsCache = logstats.NewCache(60 * time.Second)

// handleSiteStats answers GET /api/sites/{id}/stats?days=7 with the site's
// traffic statistics, computed from its access logs. Nginx, Apache and
// OpenLiteSpeed write them as <domain>.access.log in the same format (see
// the hosting package's templates), in the web server's log folder.
func (s *Server) handleSiteStats(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	days := 7
	if v := r.URL.Query().Get("days"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || !statsDays[n] {
			s.opError(w, r, usererr.New("invalid_days", v))
			return
		}
		days = n
	}
	site, err := s.store.SiteByID(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	// The domain becomes a file name.
	if validate.Domain(site.Domain) != nil {
		s.opError(w, r, errors.New("invalid site domain"))
		return
	}
	// The key includes the Server (tests run several) and the domain (an ID
	// can be reused after a site is deleted).
	key := fmt.Sprintf("%p/%d/%s/%d", s, site.ID, site.Domain, days)
	report, err := statsCache.Get(r.Context(), key, func() (*logstats.Report, error) {
		names := append([]string{site.Domain}, site.Aliases...)
		return logstats.Compute(s.hosting.LogDir(), site.Domain, names, days, s.now())
	})
	if err != nil {
		if r.Context().Err() != nil {
			return // the browser went away while waiting
		}
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}
