package mailrelay

import (
	"context"
	"testing"
)

type memStore map[string]string

func (m memStore) Setting(_ context.Context, k string) (string, error) { return m[k], nil }
func (m memStore) SetSetting(_ context.Context, k, v string) error     { m[k] = v; return nil }

func TestSPFInclude(t *testing.T) {
	for host, want := range map[string]string{
		"smtp.gmail.com":                     "_spf.google.com",
		" SMTP.Office365.com. ":              "spf.protection.outlook.com",
		"smtp.sendgrid.net":                  "sendgrid.net",
		"smtp.eu.mailgun.org":                "mailgun.org",
		"email-smtp.us-east-1.amazonaws.com": "amazonses.com",
		"email-smtp.eu-west-1.amazonaws.com": "amazonses.com",
		"smtp-relay.brevo.com":               "spf.brevo.com",
		"smtppro.zoho.eu":                    "zohomail.eu",
		"smtp.zoho.com":                      "zohomail.com",
		"smtp.zoho.example":                  "",
		"smtp.example.com":                   "",
		"localhost":                          "",
		"127.0.0.1":                          "",
		"amazonaws.com":                      "",
	} {
		if got := SPFInclude(host); got != want {
			t.Errorf("SPFInclude(%q) = %q, want %q", host, got, want)
		}
	}
}

func TestRelaySPF(t *testing.T) {
	ctx := context.Background()
	st := memStore{}
	m := New(nil, st)
	if h, inc := m.RelaySPF(ctx); h != "" || inc != "" {
		t.Fatalf("no relay: %q %q", h, inc)
	}
	st[settingKey] = `{"host":"smtp.mailgun.org","port":587,"security":"starttls"}`
	if h, inc := m.RelaySPF(ctx); h != "smtp.mailgun.org" || inc != "mailgun.org" {
		t.Fatalf("mailgun: %q %q", h, inc)
	}
}
