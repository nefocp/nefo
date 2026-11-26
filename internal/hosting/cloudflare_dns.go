package hosting

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// The DNS page: a zone's records at Cloudflare, read and changed with the
// token of Settings → Cloudflare, and "point this website here", which
// writes the A and AAAA records of a site's names. Only the record types in
// EditableTypes can be changed; the others are listed read-only.

// The permissions a request needs, as Cloudflare's token editor names them.
// The UI shows the missing one when Cloudflare refuses the token.
const (
	permZoneRead = "Zone → Zone → Read"
	permDNSRead  = "Zone → DNS → Read"
	permDNSEdit  = "Zone → DNS → Edit"
	permPurge    = "Zone → Cache Purge → Purge"
)

// Limits on what the panel reads from and sends to Cloudflare.
const (
	maxZones     = 1000
	maxRecords   = 5000 // records listed per zone; the page says when there are more
	maxPurgeURLs = 30   // Cloudflare's limit per request on most plans
	maxTXT       = 2048 // Cloudflare's limit on a TXT record's content
)

// EditableTypes are the record types the DNS page can create and change.
var EditableTypes = []string{"A", "AAAA", "CNAME", "TXT", "MX", "CAA"}

func editableType(t string) bool  { return slices.Contains(EditableTypes, t) }
func proxiableType(t string) bool { return t == "A" || t == "AAAA" || t == "CNAME" }

// cfIDRe matches Cloudflare's object identifiers.
var cfIDRe = regexp.MustCompile(`^[0-9a-f]{32}$`)

// hostLabelRe is one label of a DNS name. Underscores appear in names such
// as _dmarc and selector._domainkey.
var hostLabelRe = regexp.MustCompile(`^[a-z0-9_]([a-z0-9_-]{0,61}[a-z0-9_])?$`)

// DNSZone is a Cloudflare zone the token can see.
type DNSZone struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"` // active, pending, moved…
	Plan   string `json:"plan"`
}

// CAAData is the content of a CAA record.
type CAAData struct {
	Flags int    `json:"flags"`
	Tag   string `json:"tag"`
	Value string `json:"value"`
}

// DNSRecord is one record of a zone.
type DNSRecord struct {
	ID        string   `json:"id"`
	Type      string   `json:"type"`
	Name      string   `json:"name"`
	Content   string   `json:"content"`
	Proxied   bool     `json:"proxied"`
	Proxiable bool     `json:"proxiable"`
	TTL       int      `json:"ttl"` // 1 is "automatic"
	Priority  *int     `json:"priority,omitempty"`
	Data      *CAAData `json:"data,omitempty"` // CAA only
	Comment   string   `json:"comment,omitempty"`
}

// cfRecord is a record as Cloudflare sends it. "data" has a different shape
// for each type, so it is only decoded for CAA.
type cfRecord struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Name      string          `json:"name"`
	Content   string          `json:"content"`
	Proxied   bool            `json:"proxied"`
	Proxiable bool            `json:"proxiable"`
	TTL       int             `json:"ttl"`
	Priority  *int            `json:"priority"`
	Data      json.RawMessage `json:"data"`
	Comment   string          `json:"comment"`
}

func (r cfRecord) record() DNSRecord {
	d := DNSRecord{ID: r.ID, Type: r.Type, Name: r.Name, Content: r.Content, Proxied: r.Proxied,
		Proxiable: r.Proxiable, TTL: r.TTL, Comment: r.Comment}
	if r.Type == "MX" {
		d.Priority = r.Priority
	}
	if r.Type == "CAA" && len(r.Data) > 0 {
		var c CAAData
		if json.Unmarshal(r.Data, &c) == nil {
			d.Data = &c
		}
	}
	return d
}

// listZones returns every zone the token can see, by name.
func (c *cloudflare) listZones(ctx context.Context) ([]cfZone, error) {
	all := []cfZone{}
	for page := 1; page <= maxZones/50; page++ {
		var zs []cfZone
		info, err := c.call(ctx, http.MethodGet, fmt.Sprintf("/zones?per_page=50&page=%d", page), nil, &zs)
		if err != nil {
			return nil, err
		}
		all = append(all, zs...)
		if info == nil || len(zs) == 0 || page >= info.TotalPages {
			break
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })
	return all, nil
}

// records returns a zone's records (query adds filters such as
// "&name=…"), at most maxRecords; more reports that the zone has more.
func (c *cloudflare) records(ctx context.Context, zoneID, query string) (recs []DNSRecord, more bool, err error) {
	recs = []DNSRecord{}
	for page := 1; page <= maxRecords/100; page++ {
		var rs []cfRecord
		path := fmt.Sprintf("/zones/%s/dns_records?per_page=100&page=%d%s", url.PathEscape(zoneID), page, query)
		info, err := c.call(ctx, http.MethodGet, path, nil, &rs)
		if err != nil {
			return nil, false, err
		}
		for _, r := range rs {
			recs = append(recs, r.record())
		}
		if info == nil || len(rs) == 0 || page >= info.TotalPages {
			return recs, false, nil
		}
	}
	return recs, true, nil
}

func (c *cloudflare) record(ctx context.Context, zoneID, id string) (*DNSRecord, error) {
	var r cfRecord
	if err := c.do(ctx, http.MethodGet, "/zones/"+url.PathEscape(zoneID)+"/dns_records/"+url.PathEscape(id), nil, &r); err != nil {
		return nil, err
	}
	d := r.record()
	return &d, nil
}

// cfClient returns a client with the saved token.
func (m *Manager) cfClient(ctx context.Context) (*cloudflare, error) {
	token, _ := m.store.Setting(ctx, CloudflareTokenKey)
	if token == "" {
		return nil, usererr.New("cloudflare_not_configured")
	}
	return newCloudflare(token), nil
}

// cfUserError turns a refusal from Cloudflare into an error the UI
// explains: a missing permission names the one perm the request needs.
// Network errors stay as they are.
func cfUserError(err error, perm string) error {
	var ce *cfError
	if !errors.As(err, &ce) {
		return err
	}
	switch {
	case ce.denied():
		return usererr.New("cloudflare_permission", perm)
	case ce.badToken():
		return usererr.New("cloudflare_token_invalid", ce.Text)
	default:
		return usererr.New("cloudflare_error", ce.Text)
	}
}

func zoneJSON(z cfZone) DNSZone {
	return DNSZone{ID: z.ID, Name: z.Name, Status: z.Status, Plan: z.Plan.Name}
}

// DNSZones lists the zones the saved token can see.
func (m *Manager) DNSZones(ctx context.Context) ([]DNSZone, error) {
	c, err := m.cfClient(ctx)
	if err != nil {
		return nil, err
	}
	zones, err := c.listZones(ctx)
	if err != nil {
		return nil, cfUserError(err, permZoneRead)
	}
	out := make([]DNSZone, 0, len(zones))
	for _, z := range zones {
		out = append(out, zoneJSON(z))
	}
	return out, nil
}

// dnsZone returns the client and the zone with this ID, which must be one
// the token can see. The zone comes from the list, which works with the
// "Edit zone DNS" template's permissions alone.
func (m *Manager) dnsZone(ctx context.Context, zoneID string) (*cloudflare, *cfZone, error) {
	if !cfIDRe.MatchString(zoneID) {
		return nil, nil, store.ErrNotFound
	}
	c, err := m.cfClient(ctx)
	if err != nil {
		return nil, nil, err
	}
	zones, err := c.listZones(ctx)
	if err != nil {
		return nil, nil, cfUserError(err, permZoneRead)
	}
	for _, z := range zones {
		if z.ID == zoneID {
			return c, &z, nil
		}
	}
	return nil, nil, store.ErrNotFound
}

// DNSRecordList is a zone with its records.
type DNSRecordList struct {
	Zone    DNSZone     `json:"zone"`
	Records []DNSRecord `json:"records"`
	More    bool        `json:"more"` // the zone has more than the records listed
	Limit   int         `json:"limit"`
}

// DNSRecords lists a zone's records, by name and type.
func (m *Manager) DNSRecords(ctx context.Context, zoneID string) (*DNSRecordList, error) {
	c, zone, err := m.dnsZone(ctx, zoneID)
	if err != nil {
		return nil, err
	}
	recs, more, err := c.records(ctx, zone.ID, "")
	if err != nil {
		return nil, cfUserError(err, permDNSRead)
	}
	sort.SliceStable(recs, func(i, j int) bool {
		a, b := recs[i], recs[j]
		if a.Name != b.Name {
			// The zone's apex first, then names in order.
			if a.Name == zone.Name || b.Name == zone.Name {
				return a.Name == zone.Name
			}
			return a.Name < b.Name
		}
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		return a.Content < b.Content
	})
	return &DNSRecordList{Zone: zoneJSON(*zone), Records: recs, More: more, Limit: maxRecords}, nil
}

// RecordInput is a record to create, or the new state of one.
type RecordInput struct {
	Type     string   `json:"type"`
	Name     string   `json:"name"` // "@", "www", or the full name
	Content  string   `json:"content"`
	Proxied  bool     `json:"proxied"`
	TTL      int      `json:"ttl"` // 1 (or 0) is automatic
	Priority int      `json:"priority"`
	Data     *CAAData `json:"data"`
}

// CreateDNSRecord adds a record to a zone.
func (m *Manager) CreateDNSRecord(ctx context.Context, zoneID string, in RecordInput) (*DNSRecord, error) {
	c, zone, err := m.dnsZone(ctx, zoneID)
	if err != nil {
		return nil, err
	}
	body, err := recordBody(zone.Name, in)
	if err != nil {
		return nil, err
	}
	var r cfRecord
	if err := c.do(ctx, http.MethodPost, "/zones/"+url.PathEscape(zone.ID)+"/dns_records", body, &r); err != nil {
		return nil, cfUserError(err, permDNSEdit)
	}
	rec := r.record()
	return &rec, nil
}

// UpdateDNSRecord changes a record; its type stays. It returns the record
// before and after.
func (m *Manager) UpdateDNSRecord(ctx context.Context, zoneID, id string, in RecordInput) (old, rec *DNSRecord, err error) {
	c, zone, old, err := m.editableRecord(ctx, zoneID, id)
	if err != nil {
		return nil, nil, err
	}
	if !strings.EqualFold(strings.TrimSpace(in.Type), old.Type) {
		return nil, nil, usererr.New("dns_type_change", old.Type)
	}
	body, err := recordBody(zone.Name, in)
	if err != nil {
		return nil, nil, err
	}
	// PATCH keeps what the panel does not edit, such as the record's comment.
	var r cfRecord
	if err := c.do(ctx, http.MethodPatch, "/zones/"+url.PathEscape(zone.ID)+"/dns_records/"+url.PathEscape(id), body, &r); err != nil {
		return nil, nil, cfUserError(err, permDNSEdit)
	}
	d := r.record()
	return old, &d, nil
}

// DeleteDNSRecord deletes a record and returns it.
func (m *Manager) DeleteDNSRecord(ctx context.Context, zoneID, id string) (*DNSRecord, error) {
	c, zone, old, err := m.editableRecord(ctx, zoneID, id)
	if err != nil {
		return nil, err
	}
	if err := c.do(ctx, http.MethodDelete, "/zones/"+url.PathEscape(zone.ID)+"/dns_records/"+url.PathEscape(id), nil, nil); err != nil {
		return nil, cfUserError(err, permDNSEdit)
	}
	return old, nil
}

// editableRecord reads a record of a type the panel may change.
func (m *Manager) editableRecord(ctx context.Context, zoneID, id string) (*cloudflare, *cfZone, *DNSRecord, error) {
	if !cfIDRe.MatchString(id) {
		return nil, nil, nil, store.ErrNotFound
	}
	c, zone, err := m.dnsZone(ctx, zoneID)
	if err != nil {
		return nil, nil, nil, err
	}
	old, err := c.record(ctx, zone.ID, id)
	if err != nil {
		return nil, nil, nil, cfUserError(err, permDNSEdit)
	}
	if !editableType(old.Type) {
		return nil, nil, nil, usererr.New("dns_type_unsupported", old.Type)
	}
	return c, zone, old, nil
}

// PurgeCache empties Cloudflare's cache for the zone: everything, or the
// given URLs, which must belong to the zone. It returns the zone's name.
func (m *Manager) PurgeCache(ctx context.Context, zoneID string, everything bool, urls []string) (string, int, error) {
	c, zone, err := m.dnsZone(ctx, zoneID)
	if err != nil {
		return "", 0, err
	}
	body := map[string]any{"purge_everything": true}
	n := 0
	if !everything {
		files, err := purgeURLs(zone.Name, urls)
		if err != nil {
			return "", 0, err
		}
		body = map[string]any{"files": files}
		n = len(files)
	}
	if err := c.do(ctx, http.MethodPost, "/zones/"+url.PathEscape(zone.ID)+"/purge_cache", body, nil); err != nil {
		return "", 0, cfUserError(err, permPurge)
	}
	return zone.Name, n, nil
}

// purgeURLs checks the URLs to purge: http(s), in the zone, at most
// maxPurgeURLs. Empty lines are skipped.
func purgeURLs(zone string, in []string) ([]string, error) {
	out := []string{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || slices.Contains(out, s) {
			continue
		}
		u, err := url.Parse(s)
		if err != nil || len(s) > 2048 || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
			return nil, usererr.New("dns_invalid_url", truncateText(s, 200))
		}
		if h := strings.ToLower(u.Hostname()); h != zone && !strings.HasSuffix(h, "."+zone) {
			return nil, usererr.New("dns_url_not_in_zone", truncateText(s, 200))
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil, usererr.New("dns_no_urls")
	}
	if len(out) > maxPurgeURLs {
		return nil, usererr.New("dns_too_many_urls", fmt.Sprint(maxPurgeURLs))
	}
	return out, nil
}

// recordName turns "@", "www" or "www.example.com" into the full name in
// the zone.
func recordName(zone, s string) (string, error) {
	n := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), ".")
	switch {
	case n == "" || n == "@":
		return zone, nil
	case n == zone || strings.HasSuffix(n, "."+zone):
	default:
		n += "." + zone
	}
	if len(n) > 253 {
		return "", usererr.New("dns_invalid_name", truncateText(s, 100))
	}
	rel := strings.TrimSuffix(strings.TrimSuffix(n, zone), ".")
	if rel == "" {
		return n, nil
	}
	for i, l := range strings.Split(rel, ".") {
		if l == "*" && i == 0 {
			continue // a wildcard record
		}
		if !hostLabelRe.MatchString(l) {
			return "", usererr.New("dns_invalid_name", truncateText(s, 100))
		}
	}
	return n, nil
}

// hostName checks a host name that a record points to (CNAME, MX).
func hostName(s string) (string, bool) {
	h := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), ".")
	if h == "" || len(h) > 253 || !strings.Contains(h, ".") {
		return "", false
	}
	for _, l := range strings.Split(h, ".") {
		if !hostLabelRe.MatchString(l) {
			return "", false
		}
	}
	return h, true
}

// recordBody checks a record for the zone and returns the request body for
// Cloudflare.
func recordBody(zone string, in RecordInput) (map[string]any, error) {
	typ := strings.ToUpper(strings.TrimSpace(in.Type))
	if !editableType(typ) {
		return nil, usererr.New("dns_type_unsupported", truncateText(in.Type, 20))
	}
	name, err := recordName(zone, in.Name)
	if err != nil {
		return nil, err
	}
	if in.Proxied && !proxiableType(typ) {
		return nil, usererr.New("dns_proxied_not_allowed", typ)
	}
	ttl := in.TTL
	if ttl == 0 || in.Proxied {
		ttl = 1 // proxied records always have Cloudflare's automatic TTL
	}
	if ttl != 1 && (ttl < 60 || ttl > 86400) {
		return nil, usererr.New("dns_invalid_ttl")
	}
	body := map[string]any{"type": typ, "name": name, "ttl": ttl}
	if proxiableType(typ) {
		body["proxied"] = in.Proxied
	}
	content := strings.TrimSpace(in.Content)
	short := truncateText(content, 100)
	switch typ {
	case "A":
		a, err := netip.ParseAddr(content)
		if err != nil || !a.Is4() {
			return nil, usererr.New("dns_invalid_ipv4", short)
		}
		content = a.String()
	case "AAAA":
		a, err := netip.ParseAddr(content)
		if err != nil || !a.Is6() || a.Is4In6() || a.Zone() != "" {
			return nil, usererr.New("dns_invalid_ipv6", short)
		}
		content = a.String()
	case "CNAME":
		h, ok := hostName(content)
		if !ok {
			return nil, usererr.New("dns_invalid_target", short)
		}
		if h == name {
			return nil, usererr.New("dns_cname_self", h)
		}
		content = h
	case "MX":
		h, ok := hostName(content)
		if !ok {
			return nil, usererr.New("dns_invalid_mail_server", short)
		}
		if in.Priority < 0 || in.Priority > 65535 {
			return nil, usererr.New("dns_invalid_priority")
		}
		body["priority"] = in.Priority
		content = h
	case "TXT":
		if err := checkTXT(content); err != nil {
			return nil, err
		}
	case "CAA":
		if in.Data == nil {
			return nil, usererr.New("dns_invalid_caa_tag", "")
		}
		d, err := checkCAA(*in.Data)
		if err != nil {
			return nil, err
		}
		body["data"] = d
		return body, nil // Cloudflare writes the content from data
	}
	body["content"] = content
	return body, nil
}

// checkTXT checks a TXT record's content: one line of at most maxTXT
// bytes, either plain text (Cloudflare splits it into strings of 255
// bytes) or one or more "quoted strings" of at most 255 bytes each.
func checkTXT(s string) error {
	if s == "" {
		return usererr.New("dns_txt_empty")
	}
	if len(s) > maxTXT {
		return usererr.New("dns_txt_too_long", fmt.Sprint(maxTXT))
	}
	if !utf8.ValidString(s) {
		return usererr.New("dns_txt_invalid")
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return usererr.New("dns_txt_invalid")
		}
	}
	if !strings.Contains(s, `"`) {
		return nil
	}
	for i := 0; i < len(s); {
		if s[i] == ' ' {
			i++
			continue
		}
		if s[i] != '"' {
			return usererr.New("dns_txt_quotes")
		}
		i++
		n, closed := 0, false
		for i < len(s) {
			switch {
			case s[i] == '\\' && i+1 < len(s):
				i += 2
			case s[i] == '"':
				closed = true
				i++
			default:
				i++
			}
			if closed {
				break
			}
			n++
		}
		if !closed {
			return usererr.New("dns_txt_quotes")
		}
		if n > 255 {
			return usererr.New("dns_txt_string_long")
		}
		if i < len(s) && s[i] != ' ' {
			return usererr.New("dns_txt_quotes")
		}
	}
	return nil
}

// checkCAA checks a CAA record: flags 0 or 128 (critical), the tag issue,
// issuewild or iodef, and a value that suits the tag.
func checkCAA(d CAAData) (CAAData, error) {
	tag := strings.ToLower(strings.TrimSpace(d.Tag))
	v := strings.TrimSpace(d.Value)
	if d.Flags != 0 && d.Flags != 128 {
		return d, usererr.New("dns_invalid_caa_flags")
	}
	bad := v == "" || len(v) > 255 || strings.Contains(v, `"`)
	for _, r := range v {
		if r < 0x20 || r >= 0x7f {
			bad = true
		}
	}
	switch tag {
	case "issue", "issuewild":
		// "letsencrypt.org", "letsencrypt.org; validationmethods=dns-01",
		// or ";" for "no CA may issue".
		ca, _, _ := strings.Cut(v, ";")
		if ca = strings.TrimSpace(ca); ca != "" {
			if _, ok := hostName(ca); !ok {
				bad = true
			}
		}
	case "iodef":
		if rest, ok := strings.CutPrefix(v, "mailto:"); ok {
			bad = bad || !strings.Contains(rest, "@")
		} else if u, err := url.Parse(v); err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			bad = true
		}
	default:
		return d, usererr.New("dns_invalid_caa_tag", truncateText(d.Tag, 20))
	}
	if bad {
		return d, usererr.New("dns_invalid_caa_value", truncateText(v, 100))
	}
	return CAAData{Flags: d.Flags, Tag: tag, Value: v}, nil
}

// ---- point a website's names to this server ----

// ServerAddrs are this server's addresses, read from its network
// interfaces: nothing is sent anywhere to find them.
type ServerAddrs struct {
	Local  []netip.Addr // every address of the interfaces
	Route4 string       // the IPv4 address the default route leaves from
	Route6 string
}

var cgnat = netip.MustParsePrefix("100.64.0.0/10")

func publicIPv4(a netip.Addr) bool {
	return a.Is4() && a.IsGlobalUnicast() && !a.IsPrivate() && !cgnat.Contains(a)
}

func publicIPv6(a netip.Addr) bool {
	return a.Is6() && !a.Is4In6() && a.IsGlobalUnicast() && !a.IsPrivate()
}

// detect returns the public IPv4 and IPv6 address to point names to (""
// when there is none), preferring the default route's, and the private
// IPv4 address the server has when it has no public one.
func (s ServerAddrs) detect() (v4, v6, private4 string) {
	cands := []netip.Addr{}
	for _, r := range []string{s.Route4, s.Route6} {
		if a, err := netip.ParseAddr(r); err == nil {
			cands = append(cands, a.Unmap())
		}
	}
	for _, a := range s.Local {
		cands = append(cands, a.Unmap())
	}
	for _, a := range cands {
		switch {
		case v4 == "" && publicIPv4(a):
			v4 = a.String()
		case v6 == "" && publicIPv6(a):
			v6 = a.String()
		case private4 == "" && a.Is4() && !a.IsLoopback():
			private4 = a.String()
		}
	}
	if v4 != "" {
		private4 = ""
	}
	return v4, v6, private4
}

// PointOptions say where to point a website's names.
type PointOptions struct {
	IPv4    string `json:"ipv4"`    // "" uses the detected public address
	IPv6    string `json:"ipv6"`    // "" uses the detected one, if any
	NoIPv6  bool   `json:"no_ipv6"` // publish no AAAA record
	Proxied bool   `json:"proxied"`
}

// PointChange is one record that pointing the names changes, or keeps.
type PointChange struct {
	Name       string `json:"name"`
	Zone       string `json:"zone"`
	Type       string `json:"type"`
	Action     string `json:"action"` // create, update, delete or keep
	Old        string `json:"old,omitempty"`
	OldProxied bool   `json:"old_proxied"`
	New        string `json:"new,omitempty"`
	Proxied    bool   `json:"proxied"`

	zoneID, recordID string
	ttl              int
}

// PointSkip is a name of the site that is left alone.
type PointSkip struct {
	Name   string `json:"name"`
	Reason string `json:"reason"` // no_zone: no zone the token can see holds it
}

// PointPlan is what pointing a website's names to this server would change.
type PointPlan struct {
	IPv4      string        `json:"ipv4"`
	IPv6      string        `json:"ipv6"`
	Detected4 string        `json:"detected_ipv4"`
	Detected6 string        `json:"detected_ipv6"`
	Proxied   bool          `json:"proxied"`
	Changes   []PointChange `json:"changes"`
	Skipped   []PointSkip   `json:"skipped"`
	Zones     []DNSZone     `json:"zones"`
	SSLDNS    bool          `json:"ssl_dns"` // the site's certificate is validated through DNS
	Hash      string        `json:"hash"`    // sent back to apply exactly this plan
}

// PlanPointDNS works out the records that point the website's names (its
// domain and aliases) to this server. Only records of those names change:
// A records (and AAAA with an IPv6 address) are created or updated, other
// A and AAAA records of the names are deleted, and a CNAME is replaced,
// unless it points to another name of the site.
func (m *Manager) PlanPointDNS(ctx context.Context, siteID int64, opts PointOptions, addrs ServerAddrs) (*PointPlan, error) {
	plan, _, err := m.planPointDNS(ctx, siteID, opts, addrs)
	return plan, err
}

func (m *Manager) planPointDNS(ctx context.Context, siteID int64, opts PointOptions, addrs ServerAddrs) (*PointPlan, *cloudflare, error) {
	site, err := m.store.SiteByID(ctx, siteID)
	if err != nil {
		return nil, nil, err
	}
	c, err := m.cfClient(ctx)
	if err != nil {
		return nil, nil, err
	}
	v4, v6, private4 := addrs.detect()
	plan := &PointPlan{Detected4: v4, Detected6: v6, Proxied: opts.Proxied, SSLDNS: site.SSLDNS,
		Changes: []PointChange{}, Skipped: []PointSkip{}, Zones: []DNSZone{}}

	ip4 := strings.TrimSpace(opts.IPv4)
	if ip4 == "" {
		if v4 == "" {
			return nil, nil, usererr.New("dns_need_ipv4", private4)
		}
		ip4 = v4
	}
	a4, err := netip.ParseAddr(ip4)
	if err != nil || !a4.Is4() {
		return nil, nil, usererr.New("dns_invalid_ipv4", truncateText(ip4, 50))
	}
	if !publicIPv4(a4) {
		return nil, nil, usererr.New("dns_ip_not_public", a4.String())
	}
	plan.IPv4 = a4.String()
	if !opts.NoIPv6 {
		ip6 := strings.TrimSpace(opts.IPv6)
		if ip6 == "" {
			ip6 = v6
		}
		if ip6 != "" {
			a6, err := netip.ParseAddr(ip6)
			if err != nil || !a6.Is6() || a6.Is4In6() || a6.Zone() != "" {
				return nil, nil, usererr.New("dns_invalid_ipv6", truncateText(ip6, 50))
			}
			if !publicIPv6(a6) {
				return nil, nil, usererr.New("dns_ip_not_public", a6.String())
			}
			plan.IPv6 = a6.String()
		}
	}

	zones, err := c.listZones(ctx)
	if err != nil {
		return nil, nil, cfUserError(err, permZoneRead)
	}
	type target struct {
		name string
		zone cfZone
	}
	var targets []target
	planned := map[string]bool{}
	for _, n := range append([]string{site.Domain}, site.Aliases...) {
		z := zoneOf(zones, n)
		if z == nil {
			plan.Skipped = append(plan.Skipped, PointSkip{Name: n, Reason: "no_zone"})
			continue
		}
		targets = append(targets, target{n, *z})
		planned[n] = true
		if !slices.ContainsFunc(plan.Zones, func(d DNSZone) bool { return d.ID == z.ID }) {
			plan.Zones = append(plan.Zones, zoneJSON(*z))
		}
	}
	if len(targets) == 0 {
		return nil, nil, usererr.New("dns_no_zone", strings.Join(append([]string{site.Domain}, site.Aliases...), ", "))
	}
	local := map[netip.Addr]bool{}
	for _, a := range addrs.Local {
		local[a.Unmap()] = true
	}
	for _, t := range targets {
		recs, _, err := c.records(ctx, t.zone.ID, "&name="+url.QueryEscape(t.name))
		if err != nil {
			return nil, nil, cfUserError(err, permDNSRead)
		}
		var mine []DNSRecord
		for _, r := range recs {
			if strings.EqualFold(r.Name, t.name) && (r.Type == "A" || r.Type == "AAAA" || r.Type == "CNAME") {
				mine = append(mine, r)
			}
		}
		base := PointChange{Name: t.name, Zone: t.zone.Name, zoneID: t.zone.ID}
		plan.Changes = append(plan.Changes, pointName(base, mine, plan.IPv4, plan.IPv6, opts.Proxied, planned, local)...)
	}
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%t\n", plan.IPv4, plan.IPv6, plan.Proxied)
	for _, ch := range plan.Changes {
		fmt.Fprintf(h, "%s|%s|%s|%s|%s|%t|%s|%t\n", ch.Name, ch.Type, ch.Action, ch.recordID, ch.Old, ch.OldProxied, ch.New, ch.Proxied)
	}
	plan.Hash = hex.EncodeToString(h.Sum(nil)[:16])
	return plan, c, nil
}

// zoneOf returns the zone that holds name: the longest zone name that is
// name or a parent of it, an active zone before others of the same name.
func zoneOf(zones []cfZone, name string) *cfZone {
	var best *cfZone
	for i := range zones {
		z := &zones[i]
		if name != z.Name && !strings.HasSuffix(name, "."+z.Name) {
			continue
		}
		if best == nil || len(z.Name) > len(best.Name) || (len(z.Name) == len(best.Name) && z.Status == "active" && best.Status != "active") {
			best = z
		}
	}
	return best
}

// pointName plans the records of one name.
func pointName(base PointChange, recs []DNSRecord, ip4, ip6 string, proxied bool, planned map[string]bool, local map[netip.Addr]bool) []PointChange {
	var as, aaaas, cnames []DNSRecord
	for _, r := range recs {
		switch r.Type {
		case "A":
			as = append(as, r)
		case "AAAA":
			aaaas = append(aaaas, r)
		case "CNAME":
			cnames = append(cnames, r)
		}
	}
	var out []PointChange
	for _, r := range cnames {
		// "www CNAME example.com" already reaches this server once
		// example.com does: keep it.
		if target := strings.TrimSuffix(strings.ToLower(r.Content), "."); planned[target] && target != base.Name {
			ch := change(base, "CNAME", r, r.Content, proxied)
			return []PointChange{ch}
		}
	}
	for _, r := range cnames {
		out = append(out, deletion(base, r))
	}
	out = append(out, pointType(base, "A", as, ip4, proxied)...)
	if ip6 != "" {
		return append(out, pointType(base, "AAAA", aaaas, ip6, proxied)...)
	}
	// Without IPv6, AAAA records that point elsewhere would send IPv6
	// visitors (and Let's Encrypt) to another server.
	for _, r := range aaaas {
		if a, err := netip.ParseAddr(r.Content); err == nil && local[a] {
			out = append(out, change(base, "AAAA", r, r.Content, proxied))
		} else {
			out = append(out, deletion(base, r))
		}
	}
	return out
}

// pointType plans the A or AAAA records of a name: one record with want,
// and no other.
func pointType(base PointChange, typ string, recs []DNSRecord, want string, proxied bool) []PointChange {
	keep := -1
	for i, r := range recs {
		if sameAddr(r.Content, want) {
			keep = i
			break
		}
	}
	if keep < 0 && len(recs) > 0 {
		keep = 0
	}
	var out []PointChange
	if keep < 0 {
		ch := base
		ch.Type, ch.Action, ch.New, ch.Proxied, ch.ttl = typ, "create", want, proxied, 1
		out = append(out, ch)
	} else {
		out = append(out, change(base, typ, recs[keep], want, proxied))
	}
	for i, r := range recs {
		if i != keep {
			out = append(out, deletion(base, r))
		}
	}
	return out
}

// change keeps record r, with the content want and the proxy setting.
func change(base PointChange, typ string, r DNSRecord, want string, proxied bool) PointChange {
	ch := base
	ch.Type, ch.recordID, ch.ttl = typ, r.ID, r.TTL
	ch.Old, ch.OldProxied, ch.New, ch.Proxied = r.Content, r.Proxied, want, proxied
	ch.Action = "update"
	if (r.Content == want || sameAddr(r.Content, want)) && r.Proxied == proxied {
		ch.Action = "keep"
	}
	return ch
}

func deletion(base PointChange, r DNSRecord) PointChange {
	ch := base
	ch.Type, ch.Action, ch.recordID = r.Type, "delete", r.ID
	ch.Old, ch.OldProxied = r.Content, r.Proxied
	return ch
}

func sameAddr(a, b string) bool {
	x, err1 := netip.ParseAddr(a)
	y, err2 := netip.ParseAddr(b)
	return err1 == nil && err2 == nil && x == y
}

// ApplyPointDNS makes the changes of the plan the admin saw (hash), after
// working it out again. done is called after each change. On an error the
// changes made so far stay; the count says how many.
func (m *Manager) ApplyPointDNS(ctx context.Context, siteID int64, opts PointOptions, addrs ServerAddrs, hash string, done func(PointChange)) (int, error) {
	plan, c, err := m.planPointDNS(ctx, siteID, opts, addrs)
	if err != nil {
		return 0, err
	}
	if hash == "" || plan.Hash != hash {
		return 0, usererr.New("dns_plan_changed")
	}
	// Deletions first: a CNAME has to go before an A record of its name
	// can be added.
	var order []PointChange
	for _, action := range []string{"delete", "update", "create"} {
		for _, ch := range plan.Changes {
			if ch.Action == action {
				order = append(order, ch)
			}
		}
	}
	n := 0
	for _, ch := range order {
		base := "/zones/" + url.PathEscape(ch.zoneID) + "/dns_records"
		var err error
		switch ch.Action {
		case "delete":
			err = c.do(ctx, http.MethodDelete, base+"/"+url.PathEscape(ch.recordID), nil, nil)
		case "update":
			ttl := ch.ttl
			if ch.Proxied || ttl == 0 {
				ttl = 1
			}
			err = c.do(ctx, http.MethodPatch, base+"/"+url.PathEscape(ch.recordID),
				map[string]any{"content": ch.New, "proxied": ch.Proxied, "ttl": ttl}, nil)
		case "create":
			err = c.do(ctx, http.MethodPost, base,
				map[string]any{"type": ch.Type, "name": ch.Name, "content": ch.New, "proxied": ch.Proxied, "ttl": 1}, nil)
		}
		if err != nil {
			return n, cfUserError(err, permDNSEdit)
		}
		n++
		done(ch)
	}
	return n, nil
}
