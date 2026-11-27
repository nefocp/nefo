package hosting

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/nefocp/nefo/internal/usererr"
)

// The e-mail helper of the DNS page: the MX, SPF and DMARC records (and a
// DKIM record the provider generated) that a mail provider needs, worked
// out as a plan the administrator checks before anything changes, like
// "point this website here". There is always exactly one SPF record: an
// existing one is merged, never doubled. MX records of another provider are
// replaced only when the administrator confirms it (ReplaceMX).

// Limits on what the helper accepts.
const (
	maxMailMX    = 5
	maxMailIPs   = 10
	maxMailDKIM  = 2
	maxSPFLookup = 10 // RFC 7208: more DNS lookups make the SPF check fail
)

// MailMX is an MX record a provider needs.
type MailMX struct {
	Host     string `json:"host"`
	Priority int    `json:"priority"`
}

// MailDKIM is a DKIM record the mail provider generated: a TXT record with
// the public key, or a CNAME to a record the provider keeps.
type MailDKIM struct {
	Name  string `json:"name"` // "google._domainkey", or the full name
	Type  string `json:"type"` // TXT or CNAME
	Value string `json:"value"`
}

// MailOptions say which provider receives a domain's e-mail.
type MailOptions struct {
	// Domain is the mail domain: "" or "@" for the zone itself, or a name in
	// the zone ("mail" or "mail.example.com").
	Domain   string `json:"domain"`
	Provider string `json:"provider"` // google, microsoft, zoho, cloudflare or custom
	Region   string `json:"region"`   // zoho: com, eu, in or com.au
	// Custom provider: its MX hosts, and what SPF should allow.
	MX         []MailMX `json:"mx"`
	SPFInclude string   `json:"spf_include"` // a domain, for include:
	SPFIPs     []string `json:"spf_ips"`     // addresses or networks, for ip4: and ip6:
	// DMARCReport is the address DMARC reports go to (optional).
	DMARCReport string     `json:"dmarc_report"`
	DKIM        []MailDKIM `json:"dkim"`
	// ReplaceMX deletes the MX records of the name that are not the
	// provider's. Without it, a plan with such records cannot be applied.
	ReplaceMX bool `json:"replace_mx"`
}

// MailChange is one record the e-mail setup creates, changes, deletes or
// keeps. Action "conflict" is an MX record of another provider that is
// deleted only with ReplaceMX.
type MailChange struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Role   string `json:"role"`   // mx, spf, dmarc or dkim
	Action string `json:"action"` // create, update, delete, keep or conflict
	Old    string `json:"old,omitempty"`
	New    string `json:"new,omitempty"`

	recordID string
	priority int // MX
}

// MailNote is a remark about the plan, as a code the UI translates
// (dnsmail.note_<code>) with an optional detail.
type MailNote struct {
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}

// MailPlan is what setting up a provider's e-mail records would change.
type MailPlan struct {
	Zone     DNSZone      `json:"zone"`
	Domain   string       `json:"domain"`
	Provider string       `json:"provider"`
	Changes  []MailChange `json:"changes"`
	// Conflicts counts the MX records of another provider; NeedConfirm is
	// true while they are there and ReplaceMX is not set.
	Conflicts   int        `json:"conflicts"`
	NeedConfirm bool       `json:"need_confirm"`
	Notes       []MailNote `json:"notes"`
	// Relay is the SMTP relay's host whose SPF include the plan adds (""
	// when there is no relay, or its provider is not known).
	Relay        string `json:"relay,omitempty"`
	RelayInclude string `json:"relay_include,omitempty"`
	SPF          string `json:"spf"`  // the SPF record after the change
	DKIM         string `json:"dkim"` // the provider whose DKIM step the UI shows
	Hash         string `json:"hash"`
}

// mailProvider is what a provider's e-mail needs.
type mailProvider struct {
	mx      []MailMX
	include string // the SPF include, or ""
	// family accepts any existing MX host of the provider (accept) instead
	// of exactly mx: Google's older five records, the host Microsoft 365
	// gives a tenant. Then nothing is added while one of them is there.
	family bool
	accept func(host string) bool
	// anyPriority keeps an existing record's priority (Cloudflare picks
	// them per zone).
	anyPriority bool
}

// zohoRegions are Zoho Mail's data centres the helper knows: the MX and
// SPF domains differ per region. Others go through "custom".
var zohoRegions = map[string]string{"com": "com", "eu": "eu", "in": "in", "com.au": "com.au"}

// googleMX are Google Workspace's MX hosts: smtp.google.com since 2023, and
// the five older records, which still work.
var googleMX = []string{"smtp.google.com", "aspmx.l.google.com", "alt1.aspmx.l.google.com", "alt2.aspmx.l.google.com",
	"alt3.aspmx.l.google.com", "alt4.aspmx.l.google.com", "aspmx2.googlemail.com", "aspmx3.googlemail.com",
	"aspmx4.googlemail.com", "aspmx5.googlemail.com"}

var spfIPRe = regexp.MustCompile(`^[0-9a-fA-F.:/]{2,50}$`)

func (o *MailOptions) provider(domain string) (*mailProvider, error) {
	switch o.Provider {
	case "google":
		return &mailProvider{mx: []MailMX{{"smtp.google.com", 1}}, include: "_spf.google.com", family: true,
			accept: func(h string) bool { return slices.Contains(googleMX, h) }}, nil
	case "microsoft":
		// The host Microsoft 365 gives: the domain with dashes for dots;
		// tenants with DNSSEC get one under mx.microsoft instead.
		host := strings.ReplaceAll(domain, ".", "-") + ".mail.protection.outlook.com"
		return &mailProvider{mx: []MailMX{{host, 0}}, include: "spf.protection.outlook.com", family: true,
			accept: func(h string) bool {
				return strings.HasSuffix(h, ".mail.protection.outlook.com") || strings.HasSuffix(h, ".mx.microsoft")
			}}, nil
	case "zoho":
		r, ok := zohoRegions[o.Region]
		if o.Region == "" {
			r, ok = "com", true
		}
		if !ok {
			return nil, usererr.New("dns_mail_region", truncateText(o.Region, 20))
		}
		return &mailProvider{mx: []MailMX{{"mx.zoho." + r, 10}, {"mx2.zoho." + r, 20}, {"mx3.zoho." + r, 50}}, include: "zohomail." + r}, nil
	case "cloudflare":
		return &mailProvider{mx: []MailMX{{"route1.mx.cloudflare.net", 10}, {"route2.mx.cloudflare.net", 20}, {"route3.mx.cloudflare.net", 30}},
			include: "_spf.mx.cloudflare.net", anyPriority: true}, nil
	case "custom":
		if len(o.MX) == 0 {
			return nil, usererr.New("dns_mail_no_mx")
		}
		if len(o.MX) > maxMailMX {
			return nil, usererr.New("dns_mail_too_many_mx", strconv.Itoa(maxMailMX))
		}
		p := &mailProvider{}
		for _, mx := range o.MX {
			h, ok := hostName(mx.Host)
			if !ok {
				return nil, usererr.New("dns_invalid_mail_server", truncateText(mx.Host, 100))
			}
			if mx.Priority < 0 || mx.Priority > 65535 {
				return nil, usererr.New("dns_invalid_priority")
			}
			if !slices.ContainsFunc(p.mx, func(x MailMX) bool { return x.Host == h }) {
				p.mx = append(p.mx, MailMX{h, mx.Priority})
			}
		}
		if inc := strings.TrimSpace(o.SPFInclude); inc != "" {
			h, ok := hostName(inc)
			if !ok {
				return nil, usererr.New("dns_mail_invalid_include", truncateText(inc, 100))
			}
			p.include = h
		}
		return p, nil
	}
	return nil, usererr.New("dns_mail_provider", truncateText(o.Provider, 20))
}

// spfIPs turns the custom provider's addresses into ip4: and ip6: terms.
func (o *MailOptions) spfIPs() ([]string, error) {
	var out []string
	if len(o.SPFIPs) > maxMailIPs {
		return nil, usererr.New("dns_mail_too_many_ips", strconv.Itoa(maxMailIPs))
	}
	for _, s := range o.SPFIPs {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		var a netip.Addr
		if !spfIPRe.MatchString(s) {
			return nil, usererr.New("dns_mail_invalid_ip", truncateText(s, 60))
		}
		if p, err := netip.ParsePrefix(s); err == nil {
			a, s = p.Addr(), p.Masked().String()
		} else if x, err := netip.ParseAddr(s); err == nil && x.Zone() == "" {
			a, s = x.Unmap(), x.Unmap().String()
		} else {
			return nil, usererr.New("dns_mail_invalid_ip", truncateText(s, 60))
		}
		term := "ip6:" + s
		if a.Unmap().Is4() {
			term = "ip4:" + strings.TrimPrefix(s, "::ffff:")
		}
		if !slices.Contains(out, term) {
			out = append(out, term)
		}
	}
	return out, nil
}

var mailAddrRe = regexp.MustCompile(`^[A-Za-z0-9._%+-]{1,64}@[A-Za-z0-9.-]{1,253}\.[A-Za-z]{2,63}$`)

// txtText is a TXT record's text: the "quoted" strings joined, as a
// resolver reads them, or the content itself when it has no quotes.
func txtText(content string) string {
	s := strings.TrimSpace(content)
	if !strings.HasPrefix(s, `"`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '"' {
			continue
		}
		for i++; i < len(s) && s[i] != '"'; i++ {
			if s[i] == '\\' && i+1 < len(s) {
				i++
			}
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// hasTag reports whether a TXT text starts with tag ("v=spf1"), followed by
// a space, a ";" or nothing.
func hasTag(text, tag string) bool {
	if len(text) < len(tag) || !strings.EqualFold(text[:len(tag)], tag) {
		return false
	}
	rest := text[len(tag):]
	return rest == "" || rest[0] == ' ' || rest[0] == ';'
}

// mergeSPF merges SPF records into one with the extra terms: their own
// terms first (once each), then the extra ones they lack, then the first
// "all" (or "~all" when none had one and there is no redirect=).
func mergeSPF(records []string, extra []string) string {
	var terms []string
	all, redirect := "", ""
	seen := map[string]bool{}
	for _, r := range records {
		for _, t := range strings.Fields(r)[1:] {
			lt := strings.ToLower(t)
			switch {
			case strings.TrimLeft(lt, "+-~?") == "all":
				if all == "" {
					all = t
				}
			case strings.HasPrefix(lt, "redirect="):
				if redirect == "" {
					redirect = t
				}
			case !seen[strings.TrimPrefix(lt, "+")]:
				seen[strings.TrimPrefix(lt, "+")] = true
				terms = append(terms, t)
			}
		}
	}
	for _, t := range extra {
		if !seen[strings.ToLower(t)] {
			seen[strings.ToLower(t)] = true
			terms = append(terms, t)
		}
	}
	out := append([]string{"v=spf1"}, terms...)
	switch {
	case all != "":
		out = append(out, all)
	case redirect != "":
		out = append(out, redirect)
	default:
		out = append(out, "~all")
	}
	return strings.Join(out, " ")
}

// spfLookups counts the terms of an SPF record that make the receiver look
// up DNS names.
func spfLookups(spf string) int {
	n := 0
	for _, t := range strings.Fields(spf)[1:] {
		t = strings.ToLower(strings.TrimLeft(t, "+-~?"))
		name, _, _ := strings.Cut(t, ":")
		name, _, _ = strings.Cut(name, "/")
		name, _, _ = strings.Cut(name, "=")
		switch name {
		case "include", "a", "mx", "ptr", "exists", "redirect":
			n++
		}
	}
	return n
}

// mailName turns a name typed for the mail domain ("", "@", "mail" or the
// full name) into the full name in the zone.
func mailName(zone, s string) (string, error) {
	n, err := recordName(zone, s)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(n, "*") {
		return "", usererr.New("dns_invalid_name", truncateText(s, 100))
	}
	return n, nil
}

// dkimName turns a DKIM record's name ("google._domainkey" under the mail
// domain, or the full name in the zone) into the full name.
func dkimName(zone, domain, s string) (string, error) {
	n := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), ".")
	if n != zone && !strings.HasSuffix(n, "."+zone) {
		n += "." + domain
	}
	full, err := recordName(zone, n)
	if err != nil {
		return "", err
	}
	sel, _, ok := strings.Cut(full, "._domainkey.")
	if !ok || sel == "" || strings.Contains(sel, "*") {
		return "", usererr.New("dns_mail_dkim_name", truncateText(s, 100))
	}
	return full, nil
}

func mxText(prio int, host string) string { return fmt.Sprintf("%d %s", prio, host) }

func recPriority(r DNSRecord) int {
	if r.Priority != nil {
		return *r.Priority
	}
	return 0
}

// PlanMailDNS works out the records a mail provider needs in a zone. It
// changes nothing. relayHost and relayInclude are the panel's SMTP relay and
// its SPF include, which the SPF record also allows ("" without a relay).
func (m *Manager) PlanMailDNS(ctx context.Context, zoneID string, o MailOptions, relayHost, relayInclude string) (*MailPlan, error) {
	plan, _, err := m.planMailDNS(ctx, zoneID, o, relayHost, relayInclude)
	return plan, err
}

func (m *Manager) planMailDNS(ctx context.Context, zoneID string, o MailOptions, relayHost, relayInclude string) (*MailPlan, *cloudflare, error) {
	c, zone, err := m.dnsZone(ctx, zoneID)
	if err != nil {
		return nil, nil, err
	}
	domain, err := mailName(zone.Name, o.Domain)
	if err != nil {
		return nil, nil, err
	}
	prov, err := o.provider(domain)
	if err != nil {
		return nil, nil, err
	}
	ips, err := o.spfIPs()
	if err != nil {
		return nil, nil, err
	}
	report := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(o.DMARCReport), "mailto:"))
	if report != "" && !mailAddrRe.MatchString(report) {
		return nil, nil, usererr.New("dns_mail_invalid_report", truncateText(report, 100))
	}
	if len(o.DKIM) > maxMailDKIM {
		return nil, nil, usererr.New("dns_mail_too_many_dkim", strconv.Itoa(maxMailDKIM))
	}
	type dkimRec struct{ name, typ, value string }
	var dkims []dkimRec
	for _, d := range o.DKIM {
		if strings.TrimSpace(d.Name) == "" && strings.TrimSpace(d.Value) == "" {
			continue
		}
		name, err := dkimName(zone.Name, domain, d.Name)
		if err != nil {
			return nil, nil, err
		}
		typ := strings.ToUpper(strings.TrimSpace(d.Type))
		v := strings.TrimSpace(d.Value)
		switch typ {
		case "TXT":
			if err := checkTXT(v); err != nil {
				return nil, nil, err
			}
		case "CNAME":
			h, ok := hostName(v)
			if !ok {
				return nil, nil, usererr.New("dns_invalid_target", truncateText(v, 100))
			}
			v = h
		default:
			return nil, nil, usererr.New("dns_mail_dkim_type", truncateText(d.Type, 20))
		}
		if slices.ContainsFunc(dkims, func(x dkimRec) bool { return x.name == name }) {
			return nil, nil, usererr.New("dns_mail_dkim_twice", name)
		}
		dkims = append(dkims, dkimRec{name, typ, v})
	}

	plan := &MailPlan{Zone: zoneJSON(*zone), Domain: domain, Provider: o.Provider, Changes: []MailChange{}, Notes: []MailNote{}, DKIM: o.Provider}
	byName := map[string][]DNSRecord{}
	fetch := func(name string) ([]DNSRecord, error) {
		if recs, ok := byName[name]; ok {
			return recs, nil
		}
		recs, _, err := c.records(ctx, zone.ID, "&name="+url.QueryEscape(name))
		if err != nil {
			return nil, cfUserError(err, permDNSRead)
		}
		var mine []DNSRecord
		for _, r := range recs {
			if strings.EqualFold(r.Name, name) {
				mine = append(mine, r)
			}
		}
		byName[name] = mine
		return mine, nil
	}
	recs, err := fetch(domain)
	if err != nil {
		return nil, nil, err
	}

	// MX: the provider's records; others are conflicts.
	var existing []DNSRecord
	for _, r := range recs {
		if r.Type == "MX" {
			existing = append(existing, r)
		}
	}
	host := func(r DNSRecord) string { return strings.TrimSuffix(strings.ToLower(r.Content), ".") }
	matched := map[string]bool{}
	accepted := 0
	for _, r := range existing {
		h := host(r)
		ch := MailChange{Name: domain, Type: "MX", Role: "mx", Old: mxText(recPriority(r), h), recordID: r.ID, priority: recPriority(r)}
		want := slices.IndexFunc(prov.mx, func(x MailMX) bool { return x.Host == h })
		switch {
		case !matched[h] && want >= 0:
			matched[h] = true
			accepted++
			ch.Action, ch.New = "keep", ch.Old
			if p := prov.mx[want].Priority; !prov.anyPriority && p != recPriority(r) {
				ch.Action, ch.New, ch.priority = "update", mxText(p, h), p
			}
		case !matched[h] && prov.family && prov.accept(h):
			matched[h] = true
			accepted++
			ch.Action, ch.New = "keep", ch.Old
		default:
			plan.Conflicts++
			ch.Action = "conflict"
			if o.ReplaceMX {
				ch.Action = "delete"
			}
		}
		plan.Changes = append(plan.Changes, ch)
	}
	if !prov.family || accepted == 0 {
		for _, mx := range prov.mx {
			if !matched[mx.Host] {
				plan.Changes = append(plan.Changes, MailChange{Name: domain, Type: "MX", Role: "mx", Action: "create", New: mxText(mx.Priority, mx.Host), priority: mx.Priority})
			}
		}
	}
	plan.NeedConfirm = plan.Conflicts > 0 && !o.ReplaceMX

	// SPF: one record, merged.
	var spfs []DNSRecord
	var texts []string
	for _, r := range recs {
		if r.Type == "TXT" && hasTag(txtText(r.Content), "v=spf1") {
			spfs = append(spfs, r)
			texts = append(texts, txtText(r.Content))
		}
	}
	var extra []string
	if prov.include != "" {
		extra = append(extra, "include:"+prov.include)
	}
	extra = append(extra, ips...)
	if relayInclude != "" {
		plan.Relay, plan.RelayInclude = relayHost, relayInclude
		extra = append(extra, "include:"+relayInclude)
		plan.Notes = append(plan.Notes, MailNote{Code: "relay", Detail: relayInclude})
	}
	if len(extra) == 0 && len(spfs) == 0 {
		// A custom provider without include or addresses: SPF allows the
		// domain's MX hosts.
		extra = append(extra, "mx")
	}
	plan.SPF = mergeSPF(texts, extra)
	if err := checkTXT(plan.SPF); err != nil {
		return nil, nil, err
	}
	if n := spfLookups(plan.SPF); n > maxSPFLookup {
		plan.Notes = append(plan.Notes, MailNote{Code: "spf_lookups", Detail: strconv.Itoa(n)})
	}
	if len(spfs) == 0 {
		plan.Changes = append(plan.Changes, MailChange{Name: domain, Type: "TXT", Role: "spf", Action: "create", New: plan.SPF})
	} else {
		ch := MailChange{Name: domain, Type: "TXT", Role: "spf", Action: "keep", Old: texts[0], New: plan.SPF, recordID: spfs[0].ID}
		if texts[0] != plan.SPF {
			ch.Action = "update"
		}
		plan.Changes = append(plan.Changes, ch)
		for i, r := range spfs[1:] {
			plan.Changes = append(plan.Changes, MailChange{Name: domain, Type: "TXT", Role: "spf", Action: "delete", Old: texts[i+1], recordID: r.ID})
		}
		if len(spfs) > 1 {
			plan.Notes = append(plan.Notes, MailNote{Code: "spf_merged", Detail: strconv.Itoa(len(spfs))})
		}
	}

	// DMARC: kept when there is one.
	dmarcName := "_dmarc." + domain
	drecs, err := fetch(dmarcName)
	if err != nil {
		return nil, nil, err
	}
	kept := false
	for _, r := range drecs {
		if r.Type == "TXT" && hasTag(txtText(r.Content), "v=DMARC1") {
			kept = true
			plan.Changes = append(plan.Changes, MailChange{Name: dmarcName, Type: "TXT", Role: "dmarc", Action: "keep", Old: txtText(r.Content), New: txtText(r.Content), recordID: r.ID})
		}
	}
	if kept {
		plan.Notes = append(plan.Notes, MailNote{Code: "dmarc_kept"})
	} else {
		v := "v=DMARC1; p=none"
		if report != "" {
			v += "; rua=mailto:" + report
		}
		plan.Changes = append(plan.Changes, MailChange{Name: dmarcName, Type: "TXT", Role: "dmarc", Action: "create", New: v})
	}

	// DKIM: the record the provider gave, replacing an older one of the name.
	for _, d := range dkims {
		krecs, err := fetch(d.name)
		if err != nil {
			return nil, nil, err
		}
		done := false
		for _, r := range krecs {
			if r.Type != "TXT" && r.Type != "CNAME" {
				continue
			}
			old := r.Content
			if r.Type == "TXT" {
				old = txtText(old)
			}
			ch := MailChange{Name: d.name, Type: r.Type, Role: "dkim", Old: old, recordID: r.ID}
			switch {
			case !done && r.Type == d.typ && (old == d.value || (d.typ == "TXT" && old == txtText(d.value)) || (d.typ == "CNAME" && strings.EqualFold(strings.TrimSuffix(old, "."), d.value))):
				ch.Action, ch.New = "keep", old
				done = true
			case !done && r.Type == d.typ:
				ch.Action, ch.New = "update", d.value
				done = true
			default:
				ch.Action = "delete"
			}
			plan.Changes = append(plan.Changes, ch)
		}
		if !done {
			plan.Changes = append(plan.Changes, MailChange{Name: d.name, Type: d.typ, Role: "dkim", Action: "create", New: d.value})
		}
	}

	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%t\n", plan.Domain, plan.Provider, o.ReplaceMX)
	for _, ch := range plan.Changes {
		fmt.Fprintf(h, "%s|%s|%s|%s|%s|%s|%d\n", ch.Name, ch.Type, ch.Action, ch.recordID, ch.Old, ch.New, ch.priority)
	}
	plan.Hash = hex.EncodeToString(h.Sum(nil)[:16])
	return plan, c, nil
}

// ApplyMailDNS makes the changes of the plan the administrator saw (hash),
// after working it out again. done is called after each change. On an error
// the changes made so far stay; the count says how many.
func (m *Manager) ApplyMailDNS(ctx context.Context, zoneID string, o MailOptions, relayHost, relayInclude, hash string, done func(MailChange)) (int, error) {
	plan, c, err := m.planMailDNS(ctx, zoneID, o, relayHost, relayInclude)
	if err != nil {
		return 0, err
	}
	if hash == "" || plan.Hash != hash {
		return 0, usererr.New("dns_plan_changed")
	}
	if plan.NeedConfirm {
		return 0, usererr.New("dns_mail_mx_conflict", strconv.Itoa(plan.Conflicts))
	}
	base := "/zones/" + url.PathEscape(plan.Zone.ID) + "/dns_records"
	n := 0
	for _, action := range []string{"delete", "update", "create"} {
		for _, ch := range plan.Changes {
			if ch.Action != action {
				continue
			}
			var err error
			switch action {
			case "delete":
				err = c.do(ctx, http.MethodDelete, base+"/"+url.PathEscape(ch.recordID), nil, nil)
			case "update":
				body := map[string]any{"content": ch.New}
				if ch.Type == "MX" {
					_, h, _ := strings.Cut(ch.New, " ")
					body = map[string]any{"content": h, "priority": ch.priority}
				}
				err = c.do(ctx, http.MethodPatch, base+"/"+url.PathEscape(ch.recordID), body, nil)
			case "create":
				body := map[string]any{"type": ch.Type, "name": ch.Name, "content": ch.New, "ttl": 1}
				if ch.Type == "MX" {
					_, h, _ := strings.Cut(ch.New, " ")
					body["content"], body["priority"] = h, ch.priority
				}
				if ch.Type == "CNAME" {
					body["proxied"] = false
				}
				err = c.do(ctx, http.MethodPost, base, body, nil)
			}
			if err != nil {
				return n, cfUserError(err, permDNSEdit)
			}
			n++
			done(ch)
		}
	}
	return n, nil
}
