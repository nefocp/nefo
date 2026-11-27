package api

import (
	"fmt"
	"net/http"

	"github.com/nefocp/nefo/internal/hosting"
)

// The e-mail helper of the DNS page: the MX, SPF, DMARC (and DKIM) records
// of a mail provider, as a preview and then applied. The SPF record also
// allows the panel's SMTP relay (Settings → Mail), worked out here from the
// saved relay settings, never from the request.

// mailRelaySPF returns the relay's host and its SPF include, or "" and ""
// without a relay.
func (s *Server) mailRelaySPF(r *http.Request) (string, string) {
	return s.mail.RelaySPF(r.Context())
}

// handleMailDNSPlan shows what setting up the provider's e-mail records
// would change in the zone. It changes nothing.
func (s *Server) handleMailDNSPlan(w http.ResponseWriter, r *http.Request, a *authed) {
	var req hosting.MailOptions
	if !decodeJSON(w, r, &req) {
		return
	}
	host, inc := s.mailRelaySPF(r)
	plan, err := s.hosting.PlanMailDNS(r.Context(), r.PathValue("zone"), req, host, inc)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

// handleMailDNSApply makes the changes of the plan the admin saw: hash is
// the plan's, and nothing changes when the plan is different now.
func (s *Server) handleMailDNSApply(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		hosting.MailOptions
		Hash string `json:"hash"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	host, inc := s.mailRelaySPF(r)
	user, ip := a.user.Username, clientIP(r)
	n, err := s.hosting.ApplyMailDNS(r.Context(), r.PathValue("zone"), req.MailOptions, host, inc, req.Hash, func(ch hosting.MailChange) {
		action, detail := "dns_record_updated", fmt.Sprintf("%s %s %s → %s", ch.Type, ch.Name, ch.Old, ch.New)
		switch ch.Action {
		case "create":
			action, detail = "dns_record_created", fmt.Sprintf("%s %s %s", ch.Type, ch.Name, ch.New)
		case "delete":
			action, detail = "dns_record_deleted", fmt.Sprintf("%s %s %s", ch.Type, ch.Name, ch.Old)
		}
		s.audit(r.Context(), user, action, ip, truncate(detail, 500))
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"changed": n})
}
