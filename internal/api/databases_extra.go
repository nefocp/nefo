package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/database"
	"github.com/nefocp/nefo/internal/jobs"
)

// ---- remote access ----

// handleDBRemote returns a database's remote access state.
func (s *Server) handleDBRemote(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	ra, err := s.db.Remote(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, ra)
}

// handleSetDBRemote replaces the client addresses that may connect to a
// database from other machines; an empty list turns remote access off.
// When MariaDB must restart to change where it listens, the restart runs as
// a job, which the answer includes.
func (s *Server) handleSetDBRemote(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	var req struct {
		Hosts []string `json:"hosts"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	d, err := s.store.DatabaseByID(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	restart, err := s.db.SetRemote(r.Context(), id, req.Hosts)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	user, ip := a.user.Username, clientIP(r)
	hosts, _ := database.ParseRemoteHosts(req.Hosts) // already checked by SetRemote
	detail := d.Name + ": off"
	if len(hosts) > 0 {
		detail = d.Name + ": " + strings.Join(hosts, " ")
	}
	s.audit(r.Context(), user, "db_remote_access", ip, detail)

	// Read the state before the restart starts: the restart holds the
	// database manager until MariaDB is back.
	ra, err := s.db.Remote(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	res := map[string]any{"remote": ra}
	if restart {
		job, err := s.startDBRestart(user, ip)
		switch {
		case err == nil:
			res["job"] = job.Snapshot()
		case errors.Is(err, jobs.ErrBusy):
			// Another job runs: the state shows the restart as pending, and
			// the administrator can start it later (POST /api/db/restart).
		default:
			s.opError(w, r, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, res)
}

// handleDBRestart restarts MariaDB as a job, for a listen setting that is
// still pending because another job was running.
func (s *Server) handleDBRestart(w http.ResponseWriter, r *http.Request, a *authed) {
	job, err := s.startDBRestart(a.user.Username, clientIP(r))
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

func (s *Server) startDBRestart(user, ip string) (*jobs.Job, error) {
	return s.jobs.Start("db_restart", database.InstalledEngine().Name(), 5*time.Minute, func(ctx context.Context, j *jobs.Job) error {
		err := s.db.Restart(ctx, j)
		if err == nil {
			s.audit(ctx, user, "db_restarted", ip, "listen setting")
		}
		return err
	})
}

// restartDBIfNeeded restarts MariaDB in a job when the listen setting
// changed, for example after the last database with remote access was
// deleted. Call it after s.db.Delete. It returns the job, or nil when no
// restart is needed or another job is running (MariaDB then keeps
// listening on all interfaces until its next restart; the firewall and the
// dropped users still keep everyone out).
func (s *Server) restartDBIfNeeded(ctx context.Context, user, ip string) *jobs.Job {
	if !s.db.RestartPending(ctx) {
		return nil
	}
	job, err := s.startDBRestart(user, ip)
	if err != nil {
		s.log.Warn("MariaDB must restart to apply its listen setting", "err", err)
		return nil
	}
	return job
}

// ---- slow query log ----

func (s *Server) slowLogState(ctx context.Context) (map[string]any, error) {
	st, err := s.db.SlowLogStatus(ctx)
	if err != nil {
		return nil, err
	}
	entries, err := s.db.SlowQueries(ctx, database.SlowQueryLimit)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"settings": st, "entries": entries, "limit": database.SlowQueryLimit,
		"min_threshold": database.MinSlowThreshold, "max_threshold": database.MaxSlowThreshold,
	}, nil
}

// handleDBSlowLog returns the slow query log settings and its newest entries.
func (s *Server) handleDBSlowLog(w http.ResponseWriter, r *http.Request, a *authed) {
	res, err := s.slowLogState(r.Context())
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// handleSetDBSlowLog turns the slow query log on or off.
func (s *Server) handleSetDBSlowLog(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Enabled   bool    `json:"enabled"`
		Threshold float64 `json:"threshold"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.db.SetSlowLog(r.Context(), req.Enabled, req.Threshold); err != nil {
		s.opError(w, r, err)
		return
	}
	detail := "off"
	if req.Enabled {
		detail = "on, " + strconv.FormatFloat(req.Threshold, 'f', -1, 64) + " s"
	}
	s.audit(r.Context(), a.user.Username, "db_slowlog_changed", clientIP(r), detail)
	res, err := s.slowLogState(r.Context())
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// handleClearDBSlowLog empties the slow query log.
func (s *Server) handleClearDBSlowLog(w http.ResponseWriter, r *http.Request, a *authed) {
	if err := s.db.ClearSlowLog(r.Context()); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "db_slowlog_cleared", clientIP(r), "")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
