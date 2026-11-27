package mailrelay

import (
	"context"
	"strings"
)

// spfIncludes maps the SMTP hosts of common providers to the domain their
// customers' SPF records include, so that mail the relay passes on is
// allowed by the sending domain's SPF record.
var spfIncludes = map[string]string{
	"smtp.gmail.com":            "_spf.google.com",
	"smtp-relay.gmail.com":      "_spf.google.com",
	"smtp.googlemail.com":       "_spf.google.com",
	"smtp.office365.com":        "spf.protection.outlook.com",
	"smtp-mail.outlook.com":     "spf.protection.outlook.com",
	"smtp.sendgrid.net":         "sendgrid.net",
	"smtp.mailgun.org":          "mailgun.org",
	"smtp.eu.mailgun.org":       "mailgun.org",
	"smtp-relay.brevo.com":      "spf.brevo.com",
	"smtp-relay.sendinblue.com": "spf.brevo.com",
	"smtp.postmarkapp.com":      "spf.mtasv.net",
	"in-v3.mailjet.com":         "spf.mailjet.com",
	"smtp.sparkpostmail.com":    "sparkpostmail.com",
	"mail.smtp2go.com":          "spf.smtp2go.com",
}

// zohoSMTP are Zoho Mail's SMTP hosts per data centre (smtp. for personal
// accounts, smtppro. for organisations) and the SPF include of each.
var zohoSMTP = map[string]string{"com": "zohomail.com", "eu": "zohomail.eu", "in": "zohomail.in", "com.au": "zohomail.com.au"}

// SPFInclude returns the SPF include for an SMTP relay host, or "" when the
// provider is not known (or the relay is on this server).
func SPFInclude(host string) string {
	h := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	if inc, ok := spfIncludes[h]; ok {
		return inc
	}
	if strings.HasPrefix(h, "email-smtp.") && strings.HasSuffix(h, ".amazonaws.com") {
		return "amazonses.com"
	}
	for _, prefix := range []string{"smtp.zoho.", "smtppro.zoho."} {
		if r, ok := strings.CutPrefix(h, prefix); ok {
			return zohoSMTP[r]
		}
	}
	return ""
}

// RelaySPF returns the configured relay's host and its SPF include ("" and
// "" without a relay; the host and "" for a provider SPFInclude does not
// know).
func (m *Manager) RelaySPF(ctx context.Context) (host, include string) {
	s, err := m.load(ctx)
	if err != nil || s == nil {
		return "", ""
	}
	return s.Host, SPFInclude(s.Host)
}
