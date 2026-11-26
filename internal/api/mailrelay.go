package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/jobs"
	"github.com/nefocp/nefo/internal/mailrelay"
)

func (s *Server) handleMailRelayStatus(w http.ResponseWriter, r *http.Request, a *authed) {
	writeJSON(w, http.StatusOK, s.mail.Status(r.Context()))
}

func (s *Server) handleMailRelaySave(w http.ResponseWriter, r *http.Request, a *authed) {
	var req mailrelay.Settings
	if !decodeJSON(w, r, &req) {
		return
	}
	// Check what can be checked before the job (the saved password may be
	// kept: an empty one is fine here).
	probe := req
	if err := probe.Check("unchanged"); err != nil {
		s.opError(w, r, err)
		return
	}
	user, ip := a.user.Username, clientIP(r)
	job, err := s.jobs.Start("mail_relay", req.Host, 15*time.Minute, func(ctx context.Context, j *jobs.Job) error {
		err := s.mail.Configure(ctx, req, j)
		if err == nil {
			s.audit(ctx, user, "mail_relay_configured", ip, req.Host)
		}
		return err
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

func (s *Server) handleMailRelayTest(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		To string `json:"to"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	res, err := s.mail.SendTest(r.Context(), req.To)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "mail_relay_tested", clientIP(r), req.To)
	// res is "<Postfix status>: <the provider's answer>", such as "sent: 250
	// OK". The UI translates the status and shows the answer as it is.
	status, detail, _ := strings.Cut(res, ": ")
	writeJSON(w, http.StatusOK, map[string]string{"result": res, "status": status, "detail": detail})
}

func (s *Server) handleMailRelayRemove(w http.ResponseWriter, r *http.Request, a *authed) {
	user, ip := a.user.Username, clientIP(r)
	job, err := s.jobs.Start("mail_relay_remove", "postfix", 15*time.Minute, func(ctx context.Context, j *jobs.Job) error {
		err := s.mail.Remove(ctx, j)
		if err == nil {
			s.audit(ctx, user, "mail_relay_removed", ip, "")
		}
		return err
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}
