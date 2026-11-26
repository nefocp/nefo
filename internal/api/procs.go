package api

import (
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"sync"

	"github.com/nefocp/nefo/internal/daemons"
	"github.com/nefocp/nefo/internal/procs"
	"github.com/nefocp/nefo/internal/run"
)

// The process and background program managers are made on first use, so
// these handlers need no fields in Server. (They could equally be Server
// fields set in New.)
var (
	extraMu         sync.Mutex
	procsManagers   = map[*Server]*procs.Manager{}
	daemonsManagers = map[*Server]*daemons.Manager{}
)

func (s *Server) procsManager() *procs.Manager {
	extraMu.Lock()
	defer extraMu.Unlock()
	m, ok := procsManagers[s]
	if !ok {
		m = procs.New(run.System{})
		procsManagers[s] = m
	}
	return m
}

func (s *Server) daemonsManager() *daemons.Manager {
	extraMu.Lock()
	defer extraMu.Unlock()
	m, ok := daemonsManagers[s]
	if !ok {
		m = daemons.New(s.store, run.System{})
		daemonsManagers[s] = m
	}
	return m
}

func (s *Server) handleProcesses(w http.ResponseWriter, r *http.Request, a *authed) {
	list, total, err := s.procsManager().List(r.Context(), r.URL.Query().Get("sort"))
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"processes": list, "total": total, "cpus": runtime.NumCPU()})
}

func (s *Server) handleKillProcess(w http.ResponseWriter, r *http.Request, a *authed) {
	pid, err := strconv.Atoi(r.PathValue("pid"))
	if err != nil || pid <= 0 {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	var req struct {
		Signal string `json:"signal"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Signal == "" {
		req.Signal = "TERM"
	}
	p, err := s.procsManager().Kill(r.Context(), pid, req.Signal)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "process_killed", clientIP(r),
		truncate(fmt.Sprintf("%s %d (%s): %s", req.Signal, p.PID, p.User, p.Command), 500))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handlePorts(w http.ResponseWriter, r *http.Request, a *authed) {
	ports, err := s.procsManager().Ports(r.Context())
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ports": ports})
}

func (s *Server) handleSSHLogins(w http.ResponseWriter, r *http.Request, a *authed) {
	logins, err := s.procsManager().SSHLogins(r.Context())
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"logins": logins})
}
