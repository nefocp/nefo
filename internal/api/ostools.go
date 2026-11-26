package api

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/jobs"
	"github.com/nefocp/nefo/internal/systools"
)

// The System page: operating system updates, swap, time and host name,
// TCP BBR and disk cleanup (package systools). Long changes are jobs; all
// changes are written to the audit log.

// systemSupported answers 409 "unsupported" off Debian and Ubuntu.
func (s *Server) systemSupported(w http.ResponseWriter, r *http.Request) bool {
	if !s.hosting.Supported() {
		s.opError(w, r, hosting.ErrUnsupported)
		return false
	}
	return true
}

// startSystemJob starts a job of the System page and answers 202.
func (s *Server) startSystemJob(w http.ResponseWriter, r *http.Request, kind, target string, timeout time.Duration, fn jobs.Func) {
	job, err := s.jobs.Start(kind, target, timeout, fn)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

// osCheckDelay is how long after a start the background update check waits.
const osCheckDelay = 30 * time.Minute

// checkOSUpdates refreshes the list of operating system updates from the
// hourly maintenance when the last check is older than systools.CheckEvery.
// It waits for another time while a job runs, as that may use apt.
func (s *Server) checkOSUpdates(ctx context.Context) {
	if !s.hosting.Supported() || s.jobRunning() {
		return
	}
	// Not right after a start (a reboot, an update): that is when the
	// administrator is most likely to run apt in a shell.
	if s.now().Sub(s.started) < osCheckDelay {
		return
	}
	if checked, err := s.systools.CheckIfStale(ctx, systools.CheckEvery); err != nil && ctx.Err() == nil {
		s.log.Warn("check for system updates", "err", err)
	} else if checked {
		s.log.Info("checked for system updates")
	}
}

// sysLog writes the progress of a quick change, which has no job, to the
// panel's log.
type sysLog struct{ s *Server }

func (l sysLog) Write(p []byte) (int, error) { return len(p), nil }
func (l sysLog) Logf(format string, args ...any) {
	l.s.log.Info("system tools: " + sprintf(format, args...))
}

// ---- updates ----

// handleOSUpdates answers GET /api/system/updates with the last check: it
// never runs apt-get update (see checkOSUpdates and handleOSUpdatesCheck).
func (s *Server) handleOSUpdates(w http.ResponseWriter, r *http.Request, a *authed) {
	writeJSON(w, http.StatusOK, struct {
		systools.Updates
		Supported bool `json:"supported"`
	}{s.systools.Updates(r.Context()), s.hosting.Supported()})
}

func (s *Server) handleOSUpdatesCheck(w http.ResponseWriter, r *http.Request, a *authed) {
	if !s.systemSupported(w, r) {
		return
	}
	s.startSystemJob(w, r, "os_updates_check", "", 20*time.Minute, func(ctx context.Context, j *jobs.Job) error {
		return s.systools.CheckUpdates(ctx, j)
	})
}

// handleOSUpgrade answers POST /api/system/updates/install {"security": true}:
// all pending updates, or only the security updates.
func (s *Server) handleOSUpgrade(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Security bool `json:"security"`
	}
	if !decodeJSON(w, r, &req) || !s.systemSupported(w, r) {
		return
	}
	user, ip := a.user.Username, clientIP(r)
	kind, which := "os_upgrade", "all updates"
	if req.Security {
		kind, which = "os_upgrade_security", "security updates"
	}
	s.startSystemJob(w, r, kind, "", time.Hour, func(ctx context.Context, j *jobs.Job) error {
		n, err := s.systools.Upgrade(ctx, req.Security, j)
		if err == nil && n > 0 {
			s.audit(ctx, user, "os_updated", ip, which+": "+strconv.Itoa(n)+" packages")
		}
		return err
	})
}

// handleOSAutoUpdates answers PUT /api/system/updates/auto {"enabled": true}.
// Turning it on installs unattended-upgrades first when it is missing, as a
// job (202); otherwise the answer is 200.
func (s *Server) handleOSAutoUpdates(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if !decodeJSON(w, r, &req) || !s.systemSupported(w, r) {
		return
	}
	user, ip := a.user.Username, clientIP(r)
	action := "auto_updates_off"
	if req.Enabled {
		action = "auto_updates_on"
	}
	if req.Enabled && s.systools.NeedsInstall() {
		s.startSystemJob(w, r, "auto_updates", "", 20*time.Minute, func(ctx context.Context, j *jobs.Job) error {
			err := s.systools.SetAutoUpdates(ctx, true, j)
			if err == nil {
				s.audit(ctx, user, action, ip, "")
			}
			return err
		})
		return
	}
	if s.jobRunning() {
		writeError(w, http.StatusConflict, "busy")
		return
	}
	if err := s.systools.SetAutoUpdates(r.Context(), req.Enabled, sysLog{s}); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), user, action, ip, "")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ---- swap ----

func (s *Server) handleSwap(w http.ResponseWriter, r *http.Request, a *authed) {
	if !s.systemSupported(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, s.systools.Swap())
}

// handleCreateSwap answers POST /api/system/swap {"size_mb": 2048}.
func (s *Server) handleCreateSwap(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		SizeMB int `json:"size_mb"`
	}
	if !decodeJSON(w, r, &req) || !s.systemSupported(w, r) {
		return
	}
	if err := s.systools.CheckCreateSwap(req.SizeMB); err != nil {
		s.opError(w, r, err)
		return
	}
	user, ip, size := a.user.Username, clientIP(r), strconv.Itoa(req.SizeMB)+" MB"
	s.startSystemJob(w, r, "swap_create", size, 30*time.Minute, func(ctx context.Context, j *jobs.Job) error {
		err := s.systools.CreateSwap(ctx, req.SizeMB, j)
		if err == nil {
			s.audit(ctx, user, "swap_created", ip, size)
		}
		return err
	})
}

func (s *Server) handleRemoveSwap(w http.ResponseWriter, r *http.Request, a *authed) {
	if !s.systemSupported(w, r) {
		return
	}
	if err := s.systools.CheckRemoveSwap(); err != nil {
		s.opError(w, r, err)
		return
	}
	user, ip := a.user.Username, clientIP(r)
	s.startSystemJob(w, r, "swap_remove", "", 30*time.Minute, func(ctx context.Context, j *jobs.Job) error {
		err := s.systools.RemoveSwap(ctx, j)
		if err == nil {
			s.audit(ctx, user, "swap_removed", ip, "")
		}
		return err
	})
}

// handleSwappiness answers PUT /api/system/swap/swappiness {"value": 10}.
func (s *Server) handleSwappiness(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Value int `json:"value"`
	}
	if !decodeJSON(w, r, &req) || !s.systemSupported(w, r) {
		return
	}
	if err := s.systools.SetSwappiness(req.Value); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "swappiness_set", clientIP(r), strconv.Itoa(req.Value))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ---- time and host name ----

func (s *Server) handleSystemTime(w http.ResponseWriter, r *http.Request, a *authed) {
	if !s.systemSupported(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, s.systools.Time(r.Context()))
}

func (s *Server) handleTimezones(w http.ResponseWriter, r *http.Request, a *authed) {
	if !s.systemSupported(w, r) {
		return
	}
	zones, err := s.systools.Timezones(r.Context())
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"timezones": zones})
}

// handleSetTimezone answers PUT /api/system/timezone {"timezone": "Asia/Ho_Chi_Minh"}.
func (s *Server) handleSetTimezone(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Timezone string `json:"timezone"`
	}
	if !decodeJSON(w, r, &req) || !s.systemSupported(w, r) {
		return
	}
	if err := s.systools.SetTimezone(r.Context(), req.Timezone); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "timezone_set", clientIP(r), req.Timezone)
	writeJSON(w, http.StatusOK, s.systools.Time(r.Context()))
}

// handleSetNTP answers PUT /api/system/ntp {"enabled": true}.
func (s *Server) handleSetNTP(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if !decodeJSON(w, r, &req) || !s.systemSupported(w, r) {
		return
	}
	if err := s.systools.SetNTP(r.Context(), req.Enabled); err != nil {
		s.opError(w, r, err)
		return
	}
	action := "ntp_off"
	if req.Enabled {
		action = "ntp_on"
	}
	s.audit(r.Context(), a.user.Username, action, clientIP(r), "")
	writeJSON(w, http.StatusOK, s.systools.Time(r.Context()))
}

// handleSetHostname answers PUT /api/system/hostname {"hostname": "web1"}.
func (s *Server) handleSetHostname(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Hostname string `json:"hostname"`
	}
	if !decodeJSON(w, r, &req) || !s.systemSupported(w, r) {
		return
	}
	old, _ := os.Hostname()
	if err := s.systools.SetHostname(r.Context(), req.Hostname); err != nil {
		s.opError(w, r, err)
		return
	}
	name, _ := systools.ValidHostname(req.Hostname)
	s.audit(r.Context(), a.user.Username, "hostname_set", clientIP(r), old+" → "+name)
	writeJSON(w, http.StatusOK, s.systools.Time(r.Context()))
}

// ---- network ----

func (s *Server) handleNetwork(w http.ResponseWriter, r *http.Request, a *authed) {
	if !s.systemSupported(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, s.systools.Network())
}

// handleSetBBR answers PUT /api/system/network/bbr {"enabled": true}.
func (s *Server) handleSetBBR(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if !decodeJSON(w, r, &req) || !s.systemSupported(w, r) {
		return
	}
	var err error
	action := "bbr_off"
	if req.Enabled {
		action = "bbr_on"
		err = s.systools.EnableBBR(r.Context())
	} else {
		err = s.systools.DisableBBR(r.Context())
	}
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, action, clientIP(r), "")
	writeJSON(w, http.StatusOK, s.systools.Network())
}

// ---- cleanup ----

func (s *Server) handleCleanup(w http.ResponseWriter, r *http.Request, a *authed) {
	if !s.systemSupported(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.systools.Cleanup(r.Context())})
}

// handleClean answers POST /api/system/cleanup/{item} {"packages": [...]}:
// packages is the autoremove list the administrator confirmed.
func (s *Server) handleClean(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Packages []string `json:"packages"`
	}
	if !decodeJSON(w, r, &req) || !s.systemSupported(w, r) {
		return
	}
	id := r.PathValue("item")
	if err := s.systools.CheckClean(id); err != nil {
		s.opError(w, r, err)
		return
	}
	user, ip := a.user.Username, clientIP(r)
	s.startSystemJob(w, r, "system_cleanup", id, time.Hour, func(ctx context.Context, j *jobs.Job) error {
		freed, err := s.systools.Clean(ctx, id, req.Packages, j)
		if err != nil {
			return err
		}
		j.Logf("Freed %s", systools.MB(freed))
		s.audit(ctx, user, "cleanup_done", ip, id+": "+systools.MB(freed))
		return nil
	})
}
