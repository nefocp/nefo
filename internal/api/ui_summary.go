package api

import (
	"net"
	"net/http"
	"time"
)

// Small read-only answers for the user interface's frame (top bar and
// sidebar). They run no programs.

// jobSummary is a job without its log. The top bar asks for it every few
// seconds to show the running job; a full log can hold thousands of lines.
type jobSummary struct {
	ID       string     `json:"id"`
	Kind     string     `json:"kind"`
	Target   string     `json:"target"`
	Status   string     `json:"status"`
	Error    string     `json:"error,omitempty"`
	Last     string     `json:"last,omitempty"` // the newest log line
	Lines    int        `json:"lines"`
	Started  time.Time  `json:"started"`
	Finished *time.Time `json:"finished,omitempty"`
}

// handleJobSummary answers GET /api/jobs/summary with the remembered jobs,
// newest first, without their logs.
func (s *Server) handleJobSummary(w http.ResponseWriter, r *http.Request, a *authed) {
	recent := s.jobs.Recent()
	out := make([]jobSummary, 0, len(recent))
	for _, j := range recent {
		js := jobSummary{ID: j.ID, Kind: j.Kind, Target: j.Target, Status: j.Status, Error: j.Error,
			Lines: len(j.Log), Started: j.Started, Finished: j.Finished}
		if n := len(j.Log); n > 0 {
			js.Last = j.Log[n-1]
		}
		out = append(out, js)
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": out})
}

// handleAddresses answers GET /api/system/addresses with this server's IP
// addresses, read from its network interfaces. primary is the IPv4 address
// the default route leaves from ("" without one); nothing is sent to find
// it.
func (s *Server) handleAddresses(w http.ResponseWriter, r *http.Request, a *authed) {
	list := []string{}
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, ad := range addrs {
			n, ok := ad.(*net.IPNet)
			if !ok || n.IP.IsLoopback() || n.IP.IsLinkLocalUnicast() || n.IP.IsMulticast() {
				continue
			}
			list = append(list, n.IP.String())
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"primary":   routeSource("udp4", "192.0.2.1:9"),
		"primary6":  routeSource("udp6", "[2001:db8::1]:9"),
		"addresses": list,
	})
}

// routeSource returns the local address the system would use to reach addr
// (a documentation address), or "". Connecting a UDP socket only looks up
// the route: no packet leaves the server.
func routeSource(network, addr string) string {
	d := net.Dialer{Timeout: time.Second}
	c, err := d.Dial(network, addr)
	if err != nil {
		return ""
	}
	defer c.Close()
	if u, ok := c.LocalAddr().(*net.UDPAddr); ok && !u.IP.IsLoopback() && !u.IP.IsUnspecified() {
		return u.IP.String()
	}
	return ""
}
