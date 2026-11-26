package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/nefocp/nefo/internal/daemons"
	"github.com/nefocp/nefo/internal/store"
)

func (s *Server) handleListDaemons(w http.ResponseWriter, r *http.Request, a *authed) {
	m := s.daemonsManager()
	list, err := m.List(r.Context())
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"daemons": list, "supported": m.Supported()})
}

// handleSaveDaemon creates a program (POST) or changes one (PUT).
func (s *Server) handleSaveDaemon(w http.ResponseWriter, r *http.Request, a *authed) {
	var req daemons.Request
	if !decodeJSON(w, r, &req) {
		return
	}
	var d *store.Daemon
	var err error
	action := "daemon_created"
	if r.PathValue("id") == "" {
		d, err = s.daemonsManager().Create(r.Context(), req)
	} else {
		id, ok := pathID(r)
		if !ok {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}
		action = "daemon_updated"
		d, err = s.daemonsManager().Update(r.Context(), id, req)
	}
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, action, clientIP(r), truncate(d.Name+": "+d.Command, 500))
	writeJSON(w, http.StatusOK, map[string]int64{"id": d.ID})
}

func (s *Server) handleDeleteDaemon(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	d, err := s.store.DaemonByID(r.Context(), id)
	if err == nil {
		err = s.daemonsManager().Delete(r.Context(), id)
	}
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "daemon_deleted", clientIP(r), d.Name)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDaemonAction(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	action := r.PathValue("action")
	if err := s.daemonsManager().Action(r.Context(), id, action); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "daemon_"+action, clientIP(r), r.PathValue("id"))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDaemonLogs(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	text, err := s.daemonsManager().Logs(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"log": text})
}

// removeSiteDaemons stops and removes a website's background programs. Call
// it in handleDeleteSite before s.hosting.DeleteSite: the programs run as the
// site's Linux user, and userdel refuses to delete a user with running
// programs. It does nothing when confirm does not match the domain, because
// DeleteSite then refuses too.
func (s *Server) removeSiteDaemons(ctx context.Context, site *store.Site, confirm string) {
	if strings.TrimSuffix(strings.ToLower(strings.TrimSpace(confirm)), ".") != site.Domain {
		return
	}
	if err := s.daemonsManager().RemoveForSite(ctx, site.ID); err != nil {
		s.log.Warn("remove the site's background programs", "domain", site.Domain, "err", err)
	}
}
