package api

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"unicode/utf8"

	"github.com/nefocp/nefo/internal/hosting"
)

// The DNS page (Cloudflare zones, records and cache) and "point this
// website's names here" in the site drawer. Every change is in the audit
// log; nothing here is sent anywhere but to Cloudflare.

func (s *Server) handleDNSZones(w http.ResponseWriter, r *http.Request, a *authed) {
	zones, err := s.hosting.DNSZones(r.Context())
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"zones": zones, "editable_types": hosting.EditableTypes})
}

func (s *Server) handleDNSRecords(w http.ResponseWriter, r *http.Request, a *authed) {
	list, err := s.hosting.DNSRecords(r.Context(), r.PathValue("zone"))
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateDNSRecord(w http.ResponseWriter, r *http.Request, a *authed) {
	var req hosting.RecordInput
	if !decodeJSON(w, r, &req) {
		return
	}
	rec, err := s.hosting.CreateDNSRecord(r.Context(), r.PathValue("zone"), req)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "dns_record_created", clientIP(r), recordText(rec))
	writeJSON(w, http.StatusCreated, rec)
}

func (s *Server) handleUpdateDNSRecord(w http.ResponseWriter, r *http.Request, a *authed) {
	var req hosting.RecordInput
	if !decodeJSON(w, r, &req) {
		return
	}
	old, rec, err := s.hosting.UpdateDNSRecord(r.Context(), r.PathValue("zone"), r.PathValue("id"), req)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "dns_record_updated", clientIP(r), recordText(old)+" → "+recordValue(rec))
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) handleDeleteDNSRecord(w http.ResponseWriter, r *http.Request, a *authed) {
	old, err := s.hosting.DeleteDNSRecord(r.Context(), r.PathValue("zone"), r.PathValue("id"))
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "dns_record_deleted", clientIP(r), recordText(old))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePurgeDNSCache(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Everything bool     `json:"everything"`
		URLs       []string `json:"urls"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.URLs) > 1000 {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	zone, n, err := s.hosting.PurgeCache(r.Context(), r.PathValue("zone"), req.Everything, req.URLs)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	detail := zone + ": everything"
	if !req.Everything {
		detail = fmt.Sprintf("%s: %d URL(s)", zone, n)
	}
	s.audit(r.Context(), a.user.Username, "dns_cache_purged", clientIP(r), detail)
	writeJSON(w, http.StatusOK, map[string]any{"zone": zone, "urls": n})
}

// handlePointDNSPlan shows what pointing the site's names to this server
// would change at Cloudflare. It changes nothing.
func (s *Server) handlePointDNSPlan(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := siteID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "site_not_found")
		return
	}
	var req hosting.PointOptions
	if !decodeJSON(w, r, &req) {
		return
	}
	plan, err := s.hosting.PlanPointDNS(r.Context(), id, req, serverAddrs())
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

// handlePointDNSApply makes the changes of the plan the admin saw: hash is
// the plan's, and nothing changes when the plan is different now.
func (s *Server) handlePointDNSApply(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := siteID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "site_not_found")
		return
	}
	var req struct {
		hosting.PointOptions
		Hash string `json:"hash"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	user, ip := a.user.Username, clientIP(r)
	n, err := s.hosting.ApplyPointDNS(r.Context(), id, req.PointOptions, serverAddrs(), req.Hash, func(ch hosting.PointChange) {
		action, detail := "dns_record_updated", fmt.Sprintf("%s %s %s → %s", ch.Type, ch.Name, ch.Old, ch.New)
		switch ch.Action {
		case "create":
			action, detail = "dns_record_created", fmt.Sprintf("%s %s %s", ch.Type, ch.Name, ch.New)
		case "delete":
			action, detail = "dns_record_deleted", fmt.Sprintf("%s %s %s", ch.Type, ch.Name, ch.Old)
		}
		if ch.Proxied && ch.Action != "delete" {
			detail += " (proxied)"
		}
		s.audit(r.Context(), user, action, ip, detail)
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"changed": n})
}

// serverAddrs reads this server's addresses from its network interfaces
// (see handleAddresses); nothing is sent to find them.
func serverAddrs() hosting.ServerAddrs {
	out := hosting.ServerAddrs{Route4: routeSource("udp4", "192.0.2.1:9"), Route6: routeSource("udp6", "[2001:db8::1]:9")}
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, ad := range addrs {
			n, ok := ad.(*net.IPNet)
			if !ok || n.IP.IsLoopback() || n.IP.IsLinkLocalUnicast() || n.IP.IsMulticast() {
				continue
			}
			if a, ok := netip.AddrFromSlice(n.IP); ok {
				out.Local = append(out.Local, a.Unmap())
			}
		}
	}
	return out
}

// recordText describes a record for the audit log, such as
// "A www.example.com 203.0.113.10 (proxied)".
func recordText(r *hosting.DNSRecord) string {
	return r.Type + " " + r.Name + " " + recordValue(r)
}

func recordValue(r *hosting.DNSRecord) string {
	v := r.Content
	if r.Priority != nil {
		v = fmt.Sprintf("%d %s", *r.Priority, v)
	}
	v = strings.Join(strings.Fields(v), " ")
	if len(v) > 200 {
		cut := 200
		for cut > 0 && !utf8.RuneStart(v[cut]) {
			cut--
		}
		v = v[:cut] + "…"
	}
	if r.Proxied {
		v += " (proxied)"
	}
	return v
}
