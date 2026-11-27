package hosting

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
)

// mx lists the zone's MX records of a name as "priority host".
func (f *fakeDNS) mx(name string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, r := range f.records {
		if r["type"] == "MX" && r["name"] == name {
			p, _ := r["priority"].(float64)
			if n, ok := r["priority"].(int); ok {
				p = float64(n)
			}
			out = append(out, fmt.Sprintf("%d %s", int(p), r["content"]))
		}
	}
	sort.Strings(out)
	return out
}

func (f *fakeDNS) addMX(name, host string, prio int) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.addLocked(zoneCom, map[string]any{"type": "MX", "name": name, "content": host, "priority": prio, "ttl": 1, "proxied": false})
}

// changes describes a plan's changes as "role type name action new".
func changes(p *MailPlan) []string {
	var out []string
	for _, c := range p.Changes {
		s := fmt.Sprintf("%s %s %s %s", c.Role, c.Type, c.Name, c.Action)
		if c.New != "" {
			s += " " + c.New
		} else {
			s += " -" + c.Old
		}
		out = append(out, s)
	}
	return out
}

func hasChange(t *testing.T, p *MailPlan, want ...string) {
	t.Helper()
	got := changes(p)
	for _, w := range want {
		found := false
		for _, g := range got {
			if g == w {
				found = true
			}
		}
		if !found {
			t.Errorf("missing change %q in:\n  %s", w, strings.Join(got, "\n  "))
		}
	}
}

func pending(p *MailPlan) int {
	n := 0
	for _, c := range p.Changes {
		if c.Action != "keep" {
			n++
		}
	}
	return n
}

func applyMail(t *testing.T, m *Manager, o MailOptions, relayHost, relayInc string) int {
	t.Helper()
	ctx := context.Background()
	p, err := m.PlanMailDNS(ctx, zoneCom, o, relayHost, relayInc)
	if err != nil {
		t.Fatal(err)
	}
	n, err := m.ApplyMailDNS(ctx, zoneCom, o, relayHost, relayInc, p.Hash, func(MailChange) {})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	return n
}

func TestMailPlanProviders(t *testing.T) {
	ctx := context.Background()
	m, _ := dnsTest(t, "good-token")
	for _, tc := range []struct {
		o    MailOptions
		want []string
	}{
		{MailOptions{Provider: "google"}, []string{"mx MX example.com create 1 smtp.google.com",
			"spf TXT example.com create v=spf1 include:_spf.google.com ~all", "dmarc TXT _dmarc.example.com create v=DMARC1; p=none"}},
		{MailOptions{Provider: "microsoft"}, []string{"mx MX example.com create 0 example-com.mail.protection.outlook.com",
			"spf TXT example.com create v=spf1 include:spf.protection.outlook.com ~all"}},
		{MailOptions{Provider: "zoho", Region: "eu"}, []string{"mx MX example.com create 10 mx.zoho.eu", "mx MX example.com create 20 mx2.zoho.eu",
			"mx MX example.com create 50 mx3.zoho.eu", "spf TXT example.com create v=spf1 include:zohomail.eu ~all"}},
		{MailOptions{Provider: "zoho"}, []string{"mx MX example.com create 10 mx.zoho.com", "spf TXT example.com create v=spf1 include:zohomail.com ~all"}},
		{MailOptions{Provider: "cloudflare"}, []string{"mx MX example.com create 10 route1.mx.cloudflare.net",
			"mx MX example.com create 30 route3.mx.cloudflare.net", "spf TXT example.com create v=spf1 include:_spf.mx.cloudflare.net ~all"}},
		{MailOptions{Provider: "custom", MX: []MailMX{{"Mail.Example.net.", 5}, {"backup.example.net", 20}}, SPFInclude: "spf.example.net",
			SPFIPs: []string{"203.0.113.10", "2001:db8::/32", "198.51.100.0/24"}, DMARCReport: "mailto:dmarc@example.com"},
			[]string{"mx MX example.com create 5 mail.example.net", "mx MX example.com create 20 backup.example.net",
				"spf TXT example.com create v=spf1 include:spf.example.net ip4:203.0.113.10 ip6:2001:db8::/32 ip4:198.51.100.0/24 ~all",
				"dmarc TXT _dmarc.example.com create v=DMARC1; p=none; rua=mailto:dmarc@example.com"}},
		{MailOptions{Provider: "custom", MX: []MailMX{{"mx.example.net", 10}}}, []string{"spf TXT example.com create v=spf1 mx ~all"}},
		{MailOptions{Provider: "google", Domain: "mail"}, []string{"mx MX mail.example.com create 1 smtp.google.com",
			"dmarc TXT _dmarc.mail.example.com create v=DMARC1; p=none"}},
		{MailOptions{Provider: "microsoft", Domain: "news.example.com"}, []string{"mx MX news.example.com create 0 news-example-com.mail.protection.outlook.com"}},
	} {
		p, err := m.PlanMailDNS(ctx, zoneCom, tc.o, "", "")
		if err != nil {
			t.Fatalf("%+v: %v", tc.o, err)
		}
		hasChange(t, p, tc.want...)
		if p.NeedConfirm || p.Conflicts != 0 || p.Hash == "" || p.DKIM != tc.o.Provider {
			t.Errorf("%s: %+v", tc.o.Provider, p)
		}
	}

	// Apply Google's records; a second plan changes nothing.
	m2, fake := dnsTest(t, "good-token")
	o := MailOptions{Provider: "google"}
	var seen []string
	p, _ := m2.PlanMailDNS(ctx, zoneCom, o, "", "")
	n, err := m2.ApplyMailDNS(ctx, zoneCom, o, "", "", p.Hash, func(c MailChange) { seen = append(seen, c.Role) })
	if err != nil || n != 3 || len(seen) != 3 {
		t.Fatalf("applied %d %v: %v", n, seen, err)
	}
	if got := fake.mx("example.com"); len(got) != 1 || got[0] != "1 smtp.google.com" {
		t.Fatalf("mx %v", got)
	}
	if got := fake.list("TXT", "example.com"); len(got) != 1 || got[0] != "v=spf1 include:_spf.google.com ~all" {
		t.Fatalf("spf %v", got)
	}
	if got := fake.list("TXT", "_dmarc.example.com"); len(got) != 1 || got[0] != "v=DMARC1; p=none" {
		t.Fatalf("dmarc %v", got)
	}
	p, _ = m2.PlanMailDNS(ctx, zoneCom, o, "", "")
	if pending(p) != 0 {
		t.Fatalf("second plan: %v", changes(p))
	}
}

func TestMailPlanErrors(t *testing.T) {
	ctx := context.Background()
	m, _ := dnsTest(t, "good-token")
	for code, o := range map[string]MailOptions{
		"dns_mail_provider":        {Provider: "aol"},
		"dns_mail_region":          {Provider: "zoho", Region: "mars"},
		"dns_mail_no_mx":           {Provider: "custom"},
		"dns_mail_too_many_mx":     {Provider: "custom", MX: make([]MailMX, 6)},
		"dns_invalid_mail_server":  {Provider: "custom", MX: []MailMX{{"not a host", 10}}},
		"dns_invalid_priority":     {Provider: "custom", MX: []MailMX{{"mx.example.net", 70000}}},
		"dns_mail_invalid_include": {Provider: "custom", MX: []MailMX{{"mx.example.net", 10}}, SPFInclude: "include:x y"},
		"dns_mail_invalid_ip":      {Provider: "custom", MX: []MailMX{{"mx.example.net", 10}}, SPFIPs: []string{"203.0.113.300"}},
		"dns_mail_invalid_report":  {Provider: "google", DMARCReport: "dmarc at example.com"},
		"dns_mail_dkim_name":       {Provider: "google", DKIM: []MailDKIM{{"google", "TXT", "v=DKIM1; p=abc"}}},
		"dns_mail_dkim_type":       {Provider: "google", DKIM: []MailDKIM{{"google._domainkey", "MX", "x"}}},
		"dns_mail_dkim_twice":      {Provider: "google", DKIM: []MailDKIM{{"s._domainkey", "TXT", "v=DKIM1; p=a"}, {"s._domainkey.example.com", "TXT", "v=DKIM1; p=b"}}},
		"dns_mail_too_many_dkim":   {Provider: "google", DKIM: make([]MailDKIM, 3)},
		"dns_invalid_target":       {Provider: "microsoft", DKIM: []MailDKIM{{"selector1._domainkey", "CNAME", "not a host"}}},
		"dns_txt_invalid":          {Provider: "google", DKIM: []MailDKIM{{"google._domainkey", "TXT", "v=DKIM1;\np=abc"}}},
		"dns_invalid_name":         {Provider: "google", Domain: "bad_name!"},
	} {
		_, err := m.PlanMailDNS(ctx, zoneCom, o, "", "")
		if got, _ := errCode(err); got != code {
			t.Errorf("%s: got %s (%v)", code, got, err)
		}
	}
	if _, err := m.PlanMailDNS(ctx, "ffffffffffffffffffffffffffffffff", MailOptions{Provider: "google"}, "", ""); err == nil {
		t.Error("unknown zone accepted")
	}
}

func TestMailSPFMerge(t *testing.T) {
	ctx := context.Background()
	m, fake := dnsTest(t, "good-token")
	fake.add(zoneCom, "TXT", "example.com", "google-site-verification=abc", false)
	fake.add(zoneCom, "TXT", "example.com", `"v=spf1 ip4:198.51.100.7 include:_spf.google.com -all"`, false)
	o := MailOptions{Provider: "google"}
	p, err := m.PlanMailDNS(ctx, zoneCom, o, "smtp.mailgun.org", "mailgun.org")
	if err != nil {
		t.Fatal(err)
	}
	want := "v=spf1 ip4:198.51.100.7 include:_spf.google.com include:mailgun.org -all"
	hasChange(t, p, "spf TXT example.com update "+want)
	if p.SPF != want || p.Relay != "smtp.mailgun.org" || p.RelayInclude != "mailgun.org" {
		t.Fatalf("plan %+v", p)
	}
	if len(p.Notes) == 0 || p.Notes[0].Code != "relay" {
		t.Fatalf("notes %+v", p.Notes)
	}
	applyMail(t, m, o, "smtp.mailgun.org", "mailgun.org")
	got := fake.list("TXT", "example.com")
	if len(got) != 2 || got[0] != "google-site-verification=abc" || got[1] != want {
		t.Fatalf("TXT %v", got)
	}
	// Idempotent: the same setup again keeps everything.
	p, _ = m.PlanMailDNS(ctx, zoneCom, o, "smtp.mailgun.org", "mailgun.org")
	if pending(p) != 0 {
		t.Fatalf("second run: %v", changes(p))
	}

	// Two SPF records (a broken zone) become one.
	m2, fake2 := dnsTest(t, "good-token")
	fake2.add(zoneCom, "TXT", "example.com", "v=spf1 mx ~all", false)
	fake2.add(zoneCom, "TXT", "example.com", "v=spf1 include:sendgrid.net -all", false)
	p, err = m2.PlanMailDNS(ctx, zoneCom, o, "", "")
	if err != nil {
		t.Fatal(err)
	}
	hasChange(t, p, "spf TXT example.com update v=spf1 mx include:sendgrid.net include:_spf.google.com ~all",
		"spf TXT example.com delete -v=spf1 include:sendgrid.net -all")
	if !slicesHas(p.Notes, MailNote{Code: "spf_merged", Detail: "2"}) {
		t.Fatalf("notes %+v", p.Notes)
	}
	applyMail(t, m2, o, "", "")
	if got := fake2.list("TXT", "example.com"); len(got) != 1 || got[0] != "v=spf1 mx include:sendgrid.net include:_spf.google.com ~all" {
		t.Fatalf("merged %v", got)
	}

	// A redirect= without "all" stays; too many lookups are reported.
	m3, fake3 := dnsTest(t, "good-token")
	fake3.add(zoneCom, "TXT", "example.com", "v=spf1 a mx include:a.example include:b.example include:c.example include:d.example include:e.example include:f.example include:g.example redirect=_spf.example.net", false)
	p, err = m3.PlanMailDNS(ctx, zoneCom, o, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(p.SPF, " include:_spf.google.com redirect=_spf.example.net") || !slicesHas(p.Notes, MailNote{Code: "spf_lookups", Detail: "11"}) {
		t.Fatalf("redirect: %q %+v", p.SPF, p.Notes)
	}
}

func slicesHas(notes []MailNote, n MailNote) bool {
	for _, x := range notes {
		if x == n {
			return true
		}
	}
	return false
}

func TestMailMX(t *testing.T) {
	ctx := context.Background()
	m, fake := dnsTest(t, "good-token")
	fake.addMX("example.com", "mail.other.net", 10)
	fake.addMX("example.com", "mx.zoho.com", 30)
	o := MailOptions{Provider: "zoho"}
	p, err := m.PlanMailDNS(ctx, zoneCom, o, "", "")
	if err != nil {
		t.Fatal(err)
	}
	hasChange(t, p, "mx MX example.com conflict -10 mail.other.net", "mx MX example.com update 10 mx.zoho.com",
		"mx MX example.com create 20 mx2.zoho.com", "mx MX example.com create 50 mx3.zoho.com")
	if !p.NeedConfirm || p.Conflicts != 1 {
		t.Fatalf("confirm %v %d", p.NeedConfirm, p.Conflicts)
	}
	if _, err := m.ApplyMailDNS(ctx, zoneCom, o, "", "", p.Hash, func(MailChange) {}); err == nil {
		t.Fatal("applied over a conflict")
	} else if code, detail := errCode(err); code != "dns_mail_mx_conflict" || detail != "1" {
		t.Fatalf("conflict: %s %s", code, detail)
	}
	if got := fake.mx("example.com"); len(got) != 2 {
		t.Fatalf("changed before refusing: %v", got)
	}
	// The old hash does not apply with replace_mx: the plan differs.
	o.ReplaceMX = true
	if _, err := m.ApplyMailDNS(ctx, zoneCom, o, "", "", p.Hash, func(MailChange) {}); err == nil {
		t.Fatal("applied a plan the admin did not see")
	} else if code, _ := errCode(err); code != "dns_plan_changed" {
		t.Fatalf("hash: %s", code)
	}
	p, _ = m.PlanMailDNS(ctx, zoneCom, o, "", "")
	hasChange(t, p, "mx MX example.com delete -10 mail.other.net")
	if p.NeedConfirm {
		t.Fatal("still needs confirmation")
	}
	applyMail(t, m, o, "", "")
	if got := strings.Join(fake.mx("example.com"), ","); got != "10 mx.zoho.com,20 mx2.zoho.com,50 mx3.zoho.com" {
		t.Fatalf("mx after: %s", got)
	}

	// Google's older records and a tenant's Microsoft 365 host are kept
	// as they are: nothing is added next to them.
	m2, fake2 := dnsTest(t, "good-token")
	fake2.addMX("example.com", "aspmx.l.google.com", 1)
	fake2.addMX("example.com", "alt1.aspmx.l.google.com", 5)
	p, _ = m2.PlanMailDNS(ctx, zoneCom, MailOptions{Provider: "google"}, "", "")
	for _, c := range p.Changes {
		if c.Role == "mx" && c.Action != "keep" {
			t.Fatalf("legacy Google MX: %v", changes(p))
		}
	}
	m3, fake3 := dnsTest(t, "good-token")
	fake3.addMX("example.com", "example-com01e.mail.protection.outlook.com", 0)
	p, _ = m3.PlanMailDNS(ctx, zoneCom, MailOptions{Provider: "microsoft"}, "", "")
	for _, c := range p.Changes {
		if c.Role == "mx" && c.Action != "keep" {
			t.Fatalf("tenant MX: %v", changes(p))
		}
	}
	// Cloudflare chooses its priorities: an existing record keeps its own.
	m4, fake4 := dnsTest(t, "good-token")
	fake4.addMX("example.com", "route1.mx.cloudflare.net", 69)
	p, _ = m4.PlanMailDNS(ctx, zoneCom, MailOptions{Provider: "cloudflare"}, "", "")
	hasChange(t, p, "mx MX example.com keep 69 route1.mx.cloudflare.net", "mx MX example.com create 20 route2.mx.cloudflare.net")
}

func TestMailDMARCAndDKIM(t *testing.T) {
	ctx := context.Background()
	m, fake := dnsTest(t, "good-token")
	fake.add(zoneCom, "TXT", "_dmarc.example.com", "v=DMARC1; p=reject; rua=mailto:x@example.com", false)
	fake.add(zoneCom, "TXT", "selector1._domainkey.example.com", "v=DKIM1; p=old", false)
	o := MailOptions{Provider: "microsoft", DMARCReport: "dmarc@example.com", DKIM: []MailDKIM{
		{"selector1._domainkey", "CNAME", "selector1-example-com._domainkey.contoso.onmicrosoft.com"},
		{"selector2._domainkey.example.com.", "cname", "selector2-example-com._domainkey.contoso.onmicrosoft.com."},
	}}
	p, err := m.PlanMailDNS(ctx, zoneCom, o, "", "")
	if err != nil {
		t.Fatal(err)
	}
	hasChange(t, p, "dmarc TXT _dmarc.example.com keep v=DMARC1; p=reject; rua=mailto:x@example.com",
		"dkim TXT selector1._domainkey.example.com delete -v=DKIM1; p=old",
		"dkim CNAME selector1._domainkey.example.com create selector1-example-com._domainkey.contoso.onmicrosoft.com",
		"dkim CNAME selector2._domainkey.example.com create selector2-example-com._domainkey.contoso.onmicrosoft.com")
	if !slicesHas(p.Notes, MailNote{Code: "dmarc_kept"}) {
		t.Fatalf("notes %+v", p.Notes)
	}
	applyMail(t, m, o, "", "")
	if got := fake.list("CNAME", "selector1._domainkey.example.com"); len(got) != 1 {
		t.Fatalf("cname %v", got)
	}
	if got := fake.list("TXT", "selector1._domainkey.example.com"); len(got) != 0 {
		t.Fatalf("old DKIM left: %v", got)
	}
	if got := fake.list("TXT", "_dmarc.example.com"); len(got) != 1 || got[0] != "v=DMARC1; p=reject; rua=mailto:x@example.com" {
		t.Fatalf("dmarc changed: %v", got)
	}

	// A TXT key: created, then updated when the provider gives a new one.
	m2, fake2 := dnsTest(t, "good-token")
	o = MailOptions{Provider: "google", DKIM: []MailDKIM{{"google._domainkey", "TXT", "v=DKIM1; k=rsa; p=MIIBone"}}}
	applyMail(t, m2, o, "", "")
	o.DKIM[0].Value = "v=DKIM1; k=rsa; p=MIIBtwo"
	p, _ = m2.PlanMailDNS(ctx, zoneCom, o, "", "")
	hasChange(t, p, "dkim TXT google._domainkey.example.com update v=DKIM1; k=rsa; p=MIIBtwo")
	applyMail(t, m2, o, "", "")
	if got := fake2.list("TXT", "google._domainkey.example.com"); len(got) != 1 || got[0] != "v=DKIM1; k=rsa; p=MIIBtwo" {
		t.Fatalf("dkim %v", got)
	}
	p, _ = m2.PlanMailDNS(ctx, zoneCom, o, "", "")
	if pending(p) != 0 {
		t.Fatalf("again: %v", changes(p))
	}
}

func TestMailApplyRefusals(t *testing.T) {
	ctx := context.Background()
	m, fake := dnsTest(t, "good-token")
	o := MailOptions{Provider: "google"}
	p, _ := m.PlanMailDNS(ctx, zoneCom, o, "", "")
	if _, err := m.ApplyMailDNS(ctx, zoneCom, o, "", "", "", func(MailChange) {}); err == nil {
		t.Fatal("applied without a hash")
	}
	// The zone changed after the preview.
	fake.add(zoneCom, "TXT", "example.com", "v=spf1 -all", false)
	if _, err := m.ApplyMailDNS(ctx, zoneCom, o, "", "", p.Hash, func(MailChange) {}); err == nil {
		t.Fatal("applied a stale plan")
	} else if code, _ := errCode(err); code != "dns_plan_changed" {
		t.Fatalf("stale: %s", code)
	}
	// A relay set up meanwhile changes the plan too.
	p, _ = m.PlanMailDNS(ctx, zoneCom, o, "", "")
	if _, err := m.ApplyMailDNS(ctx, zoneCom, o, "smtp.sendgrid.net", "sendgrid.net", p.Hash, func(MailChange) {}); err == nil {
		t.Fatal("relay change not noticed")
	}

	// A token that can read but not edit: the plan works, applying names
	// the missing permission.
	ro, _ := dnsTest(t, "read-token")
	p, err := ro.PlanMailDNS(ctx, zoneCom, o, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ro.ApplyMailDNS(ctx, zoneCom, o, "", "", p.Hash, func(MailChange) {}); err == nil {
		t.Fatal("read-only token applied")
	} else if code, detail := errCode(err); code != "cloudflare_permission" || detail != permDNSEdit {
		t.Fatalf("read-only: %s %s", code, detail)
	}
}

func TestMergeSPF(t *testing.T) {
	for _, tc := range []struct {
		in    []string
		extra []string
		want  string
	}{
		{nil, []string{"include:_spf.google.com"}, "v=spf1 include:_spf.google.com ~all"},
		{[]string{"v=spf1 +mx -all"}, []string{"mx", "include:x.example"}, "v=spf1 +mx include:x.example -all"},
		{[]string{"v=spf1 ?all"}, []string{"include:x.example"}, "v=spf1 include:x.example ?all"},
		{[]string{"V=SPF1 Include:X.example all"}, []string{"include:x.example"}, "v=spf1 Include:X.example all"},
		{[]string{"v=spf1"}, nil, "v=spf1 ~all"},
	} {
		if got := mergeSPF(tc.in, tc.extra); got != tc.want {
			t.Errorf("mergeSPF(%q, %q) = %q, want %q", tc.in, tc.extra, got, tc.want)
		}
	}
	if !hasTag("v=spf1", "v=spf1") || hasTag("v=spf10 a", "v=spf1") || !hasTag("v=DMARC1;p=none", "v=DMARC1") {
		t.Fatal("hasTag")
	}
	if got := txtText(`"v=spf1 include:a.example " "-all"`); got != "v=spf1 include:a.example -all" {
		t.Fatalf("txtText %q", got)
	}
}
