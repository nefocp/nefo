package api

import "net/http"

// ---- SSH public keys of root and the people's accounts ----

func (s *Server) handleSSHKeys(w http.ResponseWriter, r *http.Request, a *authed) {
	writeJSON(w, http.StatusOK, s.security.SSHKeys(r.Context()))
}

func (s *Server) handleAddSSHKey(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		User string `json:"user"`
		Key  string `json:"key"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	k, err := s.security.AddSSHKey(r.Context(), req.User, req.Key)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "ssh_key_added", clientIP(r), keyAuditDetail(req.User, k.Fingerprint, k.Comment))
	writeJSON(w, http.StatusOK, k)
}

func (s *Server) handleRemoveSSHKey(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		User        string `json:"user"`
		Fingerprint string `json:"fingerprint"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	k, err := s.security.RemoveSSHKey(r.Context(), req.User, req.Fingerprint)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "ssh_key_removed", clientIP(r), keyAuditDetail(req.User, k.Fingerprint, k.Comment))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// keyAuditDetail is "deploy SHA256:… (me@laptop)".
func keyAuditDetail(user, fingerprint, comment string) string {
	d := user + " " + fingerprint
	if comment != "" {
		d += " (" + truncate(comment, 60) + ")"
	}
	return d
}
