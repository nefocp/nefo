package api

import "net/http"

func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request, _ *authed) {
	writeJSON(w, http.StatusOK, s.sys.Info())
}

func (s *Server) handleSystemStats(w http.ResponseWriter, r *http.Request, _ *authed) {
	st, err := s.sys.Stats()
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request, _ *authed) {
	entries, err := s.store.RecentAudit(r.Context(), 100)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}
