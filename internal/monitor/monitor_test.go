package monitor

import "testing"

func TestValidate(t *testing.T) {
	good := Settings{TelegramToken: "123456789:AAH4v_abcdefghijklmnopqrstuvwxyz12", TelegramChat: "-1001234567890",
		SMTPHost: "smtp.example.com", SMTPPort: 587, EmailFrom: "panel@example.com", EmailTo: "me@example.com", CPUPercent: 90}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
	bad := []Settings{
		{TelegramToken: "nope", TelegramChat: "1"},
		{TelegramToken: good.TelegramToken, TelegramChat: "1; rm"},
		{SMTPHost: "smtp.example.com", SMTPPort: 587, EmailFrom: "a@b.c", EmailTo: "x@y.z\r\nBcc: evil@z.z"},
		{SMTPHost: "smtp example", SMTPPort: 587, EmailFrom: "a@b.c", EmailTo: "x@y.z"},
		{CPUPercent: 5},
	}
	for _, s := range bad {
		if s.Validate() == nil {
			t.Errorf("accepted %+v", s)
		}
	}
}
