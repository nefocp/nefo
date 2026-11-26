package api

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/jobs"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/security"
	"github.com/nefocp/nefo/internal/update"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/version"
)

// ---- firewall, fail2ban, SSH ----

func (s *Server) handleSecurityStatus(w http.ResponseWriter, r *http.Request, a *authed) {
	ctx := r.Context()
	writeJSON(w, http.StatusOK, map[string]any{
		"firewall": s.security.FirewallFor(ctx, clientIP(r)),
		"fail2ban": s.security.Fail2banStatus(ctx),
		"ssh":      s.security.SSHSettings(ctx),
		"your_ip":  clientIP(r),
	})
}

func (s *Server) handleAddRule(w http.ResponseWriter, r *http.Request, a *authed) {
	var req security.NewRule
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.security.AddRule(r.Context(), req); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "firewall_rule_added", clientIP(r), req.Action+" "+req.Port+" from "+req.From)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteRule(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Number int    `json:"number"`
		To     string `json:"to"`
		From   string `json:"from"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.security.DeleteRuleFor(r.Context(), req.Number, req.To, req.From, clientIP(r)); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "firewall_rule_deleted", clientIP(r), req.To+" from "+req.From)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleEnableFirewall(w http.ResponseWriter, r *http.Request, a *authed) {
	if err := s.security.EnableFirewall(r.Context()); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "firewall_enabled", clientIP(r), "")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSetSSH(w http.ResponseWriter, r *http.Request, a *authed) {
	var req security.SSH
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.security.SetSSH(r.Context(), req); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "ssh_settings_changed", clientIP(r),
		"port "+strconv.Itoa(req.Port)+", passwords "+strconv.FormatBool(req.PasswordAuth)+", root "+req.PermitRootLogin)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleInstallFail2ban(w http.ResponseWriter, r *http.Request, a *authed) {
	user, ip := a.user.Username, clientIP(r)
	job, err := s.jobs.Start("install_fail2ban", "", 20*time.Minute, func(ctx context.Context, j *jobs.Job) error {
		err := s.security.InstallFail2ban(ctx, j)
		if err == nil {
			s.audit(ctx, user, "fail2ban_installed", ip, "")
		}
		return err
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

func (s *Server) handleUnban(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Jail string `json:"jail"`
		IP   string `json:"ip"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.security.Unban(r.Context(), req.Jail, req.IP); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "ip_unbanned", clientIP(r), req.Jail+" "+req.IP)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ---- panel certificate and updates ----

const panelDomainKey = "panel_domain"

func (s *Server) handlePanelSettings(w http.ResponseWriter, r *http.Request, a *authed) {
	ctx := r.Context()
	domain, _ := s.store.Setting(ctx, panelDomainKey)
	cert, err := s.hosting.PanelCert()
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"domain":     domain,
		"cert":       cert,
		"port":       s.cfg.Port(),
		"version":    version.Version,
		"web_ready":  s.hosting.Status(ctx).Ready,
		"update_url": s.cfg.UpdateURL,
	})
}

func (s *Server) handlePanelCertificate(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Domain string `json:"domain"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	user, ip, domain := a.user.Username, clientIP(r), req.Domain
	job, err := s.jobs.Start("panel_certificate", domain, 10*time.Minute, func(ctx context.Context, j *jobs.Job) error {
		_, err := s.hosting.IssuePanelCertificate(ctx, domain, j)
		if err == nil {
			s.store.SetSetting(ctx, panelDomainKey, domain)
			s.audit(ctx, user, "panel_cert_issued", ip, domain)
		}
		return err
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

func (s *Server) handleRemovePanelCertificate(w http.ResponseWriter, r *http.Request, a *authed) {
	if err := s.hosting.RemovePanelCertificate(); err != nil {
		s.opError(w, r, err)
		return
	}
	s.store.SetSetting(r.Context(), panelDomainKey, "")
	s.audit(r.Context(), a.user.Username, "panel_cert_removed", clientIP(r), "")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// panelRenewFailedKey holds when the last automatic renewal of the panel
// certificate failed (Unix seconds), so it is retried after
// hosting.RenewRetry, like the websites' certificates.
const panelRenewFailedKey = "panel_cert_renew_failed_at"

// renewPanelCertificate runs from the hourly maintenance loop.
func (s *Server) renewPanelCertificate(ctx context.Context) {
	domain, _ := s.store.Setting(ctx, panelDomainKey)
	info, err := s.hosting.PanelCert()
	now := time.Now()
	if domain == "" || err != nil || !hosting.PanelCertDue(info, now) {
		return
	}
	if v, _ := s.store.Setting(ctx, panelRenewFailedKey); v != "" {
		if at, err := strconv.ParseInt(v, 10, 64); err == nil && now.Sub(time.Unix(at, 0)) < hosting.RenewRetry {
			return
		}
	}
	_, err = s.hosting.IssuePanelCertificate(ctx, domain, slogJob{s})
	action, failedAt := "panel_cert_renewed", ""
	if err != nil {
		action, failedAt = "panel_cert_renew_failed", strconv.FormatInt(time.Now().Unix(), 10)
		s.log.Warn("renew panel certificate", "err", err)
	}
	if err := s.store.SetSetting(context.WithoutCancel(ctx), panelRenewFailedKey, failedAt); err != nil {
		s.log.Warn("save panel certificate renewal", "err", err)
	}
	s.audit(ctx, "system", action, "", domain)
}

type slogJob struct{ s *Server }

func (l slogJob) Write(p []byte) (int, error) { return len(p), nil }
func (l slogJob) Logf(format string, args ...any) {
	l.s.log.Info("panel certificate: " + sprintf(format, args...))
}

func (s *Server) updater() (*update.Updater, error) {
	bin, err := os.Executable()
	if err != nil {
		return nil, err
	}
	u, err := update.New(s.cfg.UpdateURL, bin, run.System{})
	if err != nil {
		return nil, usererr.New("invalid_update_url")
	}
	return u, nil
}

func (s *Server) handleCheckUpdate(w http.ResponseWriter, r *http.Request, a *authed) {
	u, err := s.updater()
	if err != nil {
		s.opError(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	latest, err := u.Latest(ctx)
	if err != nil {
		s.opError(w, r, usererr.New("update_check_failed", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"current": version.Version, "latest": latest,
		"available": latest != version.Version && "v"+version.Version != latest})
}

func (s *Server) handleApplyUpdate(w http.ResponseWriter, r *http.Request, a *authed) {
	u, err := s.updater()
	if err != nil {
		s.opError(w, r, err)
		return
	}
	user, ip := a.user.Username, clientIP(r)
	job, err := s.jobs.Start("update", "", 15*time.Minute, func(ctx context.Context, j *jobs.Job) error {
		v, err := u.Apply(ctx, j)
		if err == nil {
			s.audit(ctx, user, "panel_updated", ip, version.Version+" → "+v)
		}
		return err
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}
