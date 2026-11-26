package mailrelay

import (
	"errors"
	"testing"

	"github.com/nefocp/nefo/internal/usererr"
)

func code(err error) string {
	var ue *usererr.Error
	if errors.As(err, &ue) {
		return ue.Code
	}
	return ""
}

func TestCheck(t *testing.T) {
	ok := Settings{Host: " SMTP.Gmail.com ", Port: 587, Security: StartTLS, Username: "me@gmail.com", Password: "abcd efgh"}
	if code(ok.Check("")) != "invalid_smtp_credentials" {
		t.Fatal("password with a space accepted (it would break sasl_passwd)")
	}
	ok.Password = "abcdefghijkl"
	if err := ok.Check(""); err != nil || ok.Host != "smtp.gmail.com" {
		t.Fatalf("%v %q", err, ok.Host)
	}
	keep := Settings{Host: "smtp.sendgrid.net", Port: 587, Security: StartTLS, Username: "apikey"}
	if err := keep.Check("old-secret"); err != nil || keep.Password != "old-secret" {
		t.Fatalf("keeping the saved password: %v %q", err, keep.Password)
	}
	for name, s := range map[string]Settings{
		"invalid_smtp_host":        {Host: "smtp.x.com;rm", Port: 587, Security: StartTLS},
		"invalid_port":             {Host: "smtp.x.com", Port: 0, Security: StartTLS},
		"smtp_tls_required":        {Host: "smtp.x.com", Port: 25, Security: None, Username: "u", Password: "p"},
		"invalid_smtp_security":    {Host: "smtp.x.com", Port: 25, Security: "ssl3"},
		"invalid_smtp_credentials": {Host: "smtp.x.com", Port: 587, Security: StartTLS, Username: "u\nrelayhost=evil", Password: "p"},
		"invalid_email":            {Host: "smtp.x.com", Port: 587, Security: StartTLS, From: "not an address"},
	} {
		s := s
		if got := code(s.Check("")); got != name {
			t.Errorf("%s: got %q", name, got)
		}
	}
	local := Settings{Host: "127.0.0.1", Port: 2525, Security: None}
	if err := local.Check(""); err != nil {
		t.Fatalf("plain relay on this server refused: %v", err)
	}
}

func TestDeliveryLog(t *testing.T) {
	log := `postfix/pickup[1]: 4F1A2B3C4D: uid=0 from=<root>
postfix/cleanup[2]: 4F1A2B3C4D: message-id=<panel-test-123@localhost>
postfix/qmgr[3]: 4F1A2B3C4D: from=<root@vps>, size=400, nrcpt=1 (queue active)
postfix/smtp[4]: 4F1A2B3C4D: to=<a@example.com>, relay=smtp.gmail.com[1.2.3.4]:587, delay=1, dsn=2.0.0, status=sent (250 2.0.0 OK  1700000000 abc - gsmtp)
postfix/smtp[5]: 5A5A5A5A5A: to=<b@example.com>, relay=smtp.gmail.com[1.2.3.4]:587, status=deferred (SASL authentication failed; server smtp.gmail.com said: 535-5.7.8 Username and Password not accepted)`
	qid := findQueueID(log, "panel-test-123")
	if qid != "4F1A2B3C4D" {
		t.Fatalf("queue id = %q", qid)
	}
	if got := deliveryResult(log, qid); got != "sent: 250 2.0.0 OK  1700000000 abc - gsmtp" {
		t.Fatalf("result = %q", got)
	}
	if got := deliveryResult(log, "5A5A5A5A5A"); got[:9] != "deferred:" {
		t.Fatalf("deferred = %q", got)
	}
	if deliveryResult(log, "FFFFFF") != "" || findQueueID(log, "panel-test-999") != "" {
		t.Fatal("matched an unknown message")
	}
}
