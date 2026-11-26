package api

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/jobs"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/usererr"
)

// Restarting the server or the panel. Both refuse while a background job
// runs (an install or a backup would be cut off), are written to the audit
// log, and act a moment after the answer, so the browser gets it.

// powerRun runs systemctl; tests replace it.
var powerRun run.Runner = run.System{}

// powerDelay is how long the panel waits after answering.
var powerDelay = 2 * time.Second

// panelUnit returns the systemd unit the panel runs in ("nefo.service"),
// read from /proc/self/cgroup, or "" when it does not run as a service.
var panelUnit = func() string {
	b, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		for _, part := range strings.Split(line, "/") {
			if strings.HasSuffix(part, ".service") && validUnitName(part) {
				return part
			}
		}
	}
	return ""
}

func validUnitName(s string) bool {
	if len(s) > 120 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("-_.@", c)) {
			return false
		}
	}
	return true
}

func (s *Server) jobRunning() bool {
	for _, j := range s.jobs.Recent() {
		if j.Status == jobs.StatusRunning {
			return true
		}
	}
	return false
}

// powerLater runs systemctl with args after powerDelay, in the background.
func (s *Server) powerLater(args ...string) {
	go func() {
		time.Sleep(powerDelay)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if out, err := powerRun.Output(ctx, "systemctl", args...); err != nil {
			s.log.Error("systemctl failed", "args", strings.Join(args, " "), "err", err, "out", run.Tail(out, 5))
		}
	}()
}

// handleReboot answers POST /api/system/reboot {"confirm": "<hostname>"}:
// the administrator types the server's name, then the server reboots.
func (s *Server) handleReboot(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Confirm string `json:"confirm"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	host, _ := os.Hostname()
	if host == "" || !strings.EqualFold(strings.TrimSpace(req.Confirm), host) {
		s.opError(w, r, usererr.New("reboot_confirm_mismatch"))
		return
	}
	if s.jobRunning() {
		writeError(w, http.StatusConflict, "busy")
		return
	}
	s.audit(r.Context(), a.user.Username, "server_reboot", clientIP(r), host)
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "delay_seconds": int(powerDelay.Seconds())})
	s.powerLater("--no-block", "reboot")
}

// handlePanelRestart answers POST /api/panel/restart: systemd restarts the
// panel's own service; websites and databases keep running.
func (s *Server) handlePanelRestart(w http.ResponseWriter, r *http.Request, a *authed) {
	unit := panelUnit()
	if unit == "" {
		s.opError(w, r, usererr.New("panel_not_service"))
		return
	}
	if s.jobRunning() {
		writeError(w, http.StatusConflict, "busy")
		return
	}
	s.audit(r.Context(), a.user.Username, "panel_restart", clientIP(r), unit)
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "delay_seconds": int(powerDelay.Seconds())})
	s.powerLater("--no-block", "restart", unit)
}
