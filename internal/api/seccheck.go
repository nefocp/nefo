package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/seccheck"
	"github.com/nefocp/nefo/internal/wptoolkit"
)

// The security check (package seccheck): the facts the panel already
// knows, a run on demand and once a day from the maintenance, accepted
// findings, and the WordPress toolkit's last check of each site.

// secChecks holds each Server's check service, so this file needs no field
// in Server (like wpKits).
var secChecks sync.Map // *Server → *seccheck.Service

func (s *Server) seccheck() *seccheck.Service {
	if v, ok := secChecks.Load(s); ok {
		return v.(*seccheck.Service)
	}
	v, _ := secChecks.LoadOrStore(s, seccheck.NewService(s.store))
	return v.(*seccheck.Service)
}

// secEnv is the environment of a run; tests replace it (secEnvHook).
func (s *Server) secEnv() seccheck.Env {
	if h, ok := secEnvHooks.Load(s); ok {
		return h.(func() seccheck.Env)()
	}
	return seccheck.DefaultEnv()
}

var secEnvHooks sync.Map // *Server → func() seccheck.Env, in tests

// securityFacts gathers what the check needs from the panel's managers.
func (s *Server) securityFacts(ctx context.Context) *seccheck.Facts {
	if h, ok := secFactsHooks.Load(s); ok {
		return h.(func(context.Context) *seccheck.Facts)(ctx)
	}
	f := &seccheck.Facts{}
	if fileExistsAPI("/usr/sbin/sshd") {
		ssh := s.security.SSHSettings(ctx)
		f.SSH = &ssh
	}
	f.Firewall = s.security.Firewall(ctx)
	f.Fail2ban = s.security.Fail2banStatus(ctx)
	if fileExistsAPI("/usr/bin/apt-get") {
		u := s.systools.Updates(ctx)
		f.Updates = &u
	}
	if ports, err := s.procsManager().Ports(ctx); err == nil {
		f.Ports = ports
	}
	if s.docker.Installed() {
		if cs, err := s.docker.Containers(ctx); err == nil {
			f.Docker = cs
		}
	}
	f.Panel = seccheck.Panel{TLS: s.cfg.TLS.Enabled, Loopback: loopbackListen(s.cfg.Listen)}
	if s.cfg.TLS.Enabled && s.cfg.TLS.Cert == "" && !fileExistsAPI(filepath.Join(s.cfg.DataDir, "certs", "_panel", "fullchain.pem")) {
		f.Panel.SelfSigned = true
	}
	if users, err := s.store.ListUsers(ctx); err == nil {
		for _, u := range users {
			f.Panel.Users = append(f.Panel.Users, seccheck.User{Name: u.Username, TOTP: u.TOTPEnabled})
		}
	}
	if sites, err := s.store.ListSites(ctx); err == nil {
		for _, st := range sites {
			f.Sites = append(f.Sites, seccheck.Site{ID: st.ID, Domain: st.Domain, Dir: s.hosting.Root(st.Domain), SSL: st.SSL,
				ForceHTTPS: st.ForceHTTPS, CertExpiry: st.CertNotAfter, CustomCert: st.SSLSource == "custom", Disabled: st.Disabled,
				Password: st.AuthUser != "", Redirect: st.RedirectURL != "", Proxy: st.ProxyPort != 0})
		}
	}
	f.WordPress = s.wpLastChecks(ctx)
	return f
}

var secFactsHooks sync.Map // *Server → func(ctx) *seccheck.Facts, in tests

func fileExistsAPI(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// loopbackListen reports whether the panel listens on a loopback address
// only.
func loopbackListen(listen string) bool {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// secCheckResponse is the page's data: the last report (nil before the
// first run), its counts and the settings.
type secCheckResponse struct {
	Report   *seccheck.Report  `json:"report"`
	Counts   seccheck.Counts   `json:"counts"`
	Settings seccheck.Settings `json:"settings"`
	Running  bool              `json:"running"`
	Checks   []string          `json:"checks"`
}

func (s *Server) secCheckJSON(ctx context.Context, rep *seccheck.Report) (secCheckResponse, error) {
	set, err := s.seccheck().Settings(ctx)
	out := secCheckResponse{Report: rep, Settings: set, Running: s.seccheck().Running(), Checks: seccheck.Keys()}
	if rep != nil {
		out.Counts = rep.Counts()
	}
	return out, err
}

func (s *Server) handleSecurityCheck(w http.ResponseWriter, r *http.Request, a *authed) {
	rep, err := s.seccheck().Last(r.Context())
	if err != nil {
		s.opError(w, r, err)
		return
	}
	out, err := s.secCheckJSON(r.Context(), rep)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// handleRunSecurityCheck runs the check now. It takes a few seconds: every
// check is bounded, and the whole run stops after 45 seconds (the checks not
// done by then are listed as partial).
func (s *Server) handleRunSecurityCheck(w http.ResponseWriter, r *http.Request, a *authed) {
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()
	rep, _, err := s.seccheck().Run(ctx, s.securityFacts, s.secEnv())
	if err != nil {
		s.opError(w, r, err)
		return
	}
	// Last marks the accepted findings the same way Run did.
	out, err := s.secCheckJSON(r.Context(), rep)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleSecurityCheckSettings(w http.ResponseWriter, r *http.Request, a *authed) {
	var req seccheck.Settings
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.seccheck().SaveSettings(r.Context(), req); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "security_check_settings", clientIP(r), fmt.Sprintf("daily %t, alert %t", req.Daily, req.Notify))
	rep, _ := s.seccheck().Last(r.Context())
	out, _ := s.secCheckJSON(r.Context(), rep)
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleAcceptFinding(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		ID       string `json:"id"`
		Accepted bool   `json:"accepted"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.ID == "" || len(req.ID) > 300 {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	rep, err := s.seccheck().Accept(r.Context(), req.ID, req.Accepted)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	action := "security_finding_accepted"
	if !req.Accepted {
		action = "security_finding_restored"
	}
	s.audit(r.Context(), a.user.Username, action, clientIP(r), truncate(req.ID, 300))
	out, err := s.secCheckJSON(r.Context(), rep)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// dailySecurityCheck runs the check when a daily run is due, from the
// hourly maintenance, in the background. Not in the first minutes after
// the panel starts, when it has other work. When alerts are on for it, the
// critical problems and warnings the previous run did not have are sent.
func (s *Server) dailySecurityCheck(ctx context.Context) {
	if time.Since(s.started) < 10*time.Minute || !s.seccheck().Due(ctx, time.Now()) {
		return
	}
	go func() {
		rctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		_, fresh, err := s.seccheck().Run(rctx, s.securityFacts, s.secEnv())
		if err != nil {
			if ctx.Err() == nil {
				s.log.Warn("daily security check", "err", err)
			}
			return
		}
		if set, err := s.seccheck().Settings(ctx); err == nil && set.Notify && len(fresh) > 0 {
			s.monitor.Notify(ctx, secCheckAlert(fresh))
		}
	}()
}

// secCheckAlert is the alert text for new findings.
func secCheckAlert(fresh []seccheck.Finding) string {
	var b strings.Builder
	fmt.Fprintf(&b, "🛡 The daily security check found %d new problem(s):", len(fresh))
	for i, f := range fresh {
		if i == 10 {
			fmt.Fprintf(&b, "\n… and %d more", len(fresh)-10)
			break
		}
		sev := "warning"
		if f.Severity == seccheck.Critical {
			sev = "critical"
		}
		fmt.Fprintf(&b, "\n- [%s] %s", sev, seccheck.Title(f))
	}
	b.WriteString("\nSee Security check in the panel.")
	return b.String()
}

// ---- the WordPress toolkit's last check of each site ----

// wpChecksKey keeps, by domain, the last successful status check of the
// WordPress toolkit, for the security check (which never runs wp-cli).
const wpChecksKey = "wp_last_checks"

type wpLastCheck struct {
	CheckedAt     time.Time `json:"checked_at"`
	CoreUpdate    string    `json:"core_update"`
	PluginUpdates int       `json:"plugin_updates"`
	ThemeUpdates  int       `json:"theme_updates"`
	Debug         bool      `json:"debug"`
}

var wpChecksMu sync.Mutex

func (s *Server) loadWPChecks(ctx context.Context) map[string]wpLastCheck {
	m := map[string]wpLastCheck{}
	if v, err := s.store.Setting(ctx, wpChecksKey); err == nil && v != "" {
		json.Unmarshal([]byte(v), &m)
	}
	return m
}

// rememberWPCheck records a status check (st nil forgets the site, after
// an update or when it is deleted).
func (s *Server) rememberWPCheck(ctx context.Context, domain string, st *wptoolkit.Status) {
	wpChecksMu.Lock()
	defer wpChecksMu.Unlock()
	m := s.loadWPChecks(ctx)
	if st == nil {
		if _, ok := m[domain]; !ok {
			return
		}
		delete(m, domain)
	} else {
		if st.Error != "" {
			return
		}
		m[domain] = wpLastCheck{CheckedAt: time.Now().UTC(), CoreUpdate: st.CoreUpdate, PluginUpdates: st.PluginUpdates,
			ThemeUpdates: st.ThemeUpdates, Debug: st.Debug}
	}
	v := ""
	if len(m) > 0 {
		b, _ := json.Marshal(m)
		v = string(b)
	}
	if err := s.store.SetSetting(ctx, wpChecksKey, v); err != nil {
		s.log.Warn("remember the WordPress check", "err", err)
	}
}

// wpLastChecks returns the toolkit's last checks of the panel's WordPress
// sites, or nil when there are none.
func (s *Server) wpLastChecks(ctx context.Context) map[string]seccheck.WordPress {
	wpChecksMu.Lock()
	m := s.loadWPChecks(ctx)
	wpChecksMu.Unlock()
	if len(m) == 0 {
		return nil
	}
	sites, err := s.store.ListSites(ctx)
	if err != nil {
		return nil
	}
	wp := map[string]bool{}
	for _, st := range sites {
		wp[st.Domain] = st.App == hosting.AppWordPress
	}
	out := map[string]seccheck.WordPress{}
	for d, c := range m {
		if wp[d] {
			out[d] = seccheck.WordPress{CheckedAt: c.CheckedAt, CoreUpdate: c.CoreUpdate, PluginUpdates: c.PluginUpdates,
				ThemeUpdates: c.ThemeUpdates, Debug: c.Debug}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
