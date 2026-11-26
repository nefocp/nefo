package auth

import (
	"strings"
	"testing"
	"time"
)

// RFC 6238 appendix B test vectors (SHA-1), truncated to 6 digits.
func TestTOTPVectors(t *testing.T) {
	secret := b32.EncodeToString([]byte("12345678901234567890"))
	cases := []struct {
		unix int64
		want string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
	}
	for _, c := range cases {
		counter, ok := VerifyTOTP(secret, c.want, time.Unix(c.unix, 0))
		if !ok {
			t.Errorf("t=%d: code %s rejected", c.unix, c.want)
		}
		if want := uint64(c.unix / 30); counter != want {
			t.Errorf("t=%d: counter %d, want %d", c.unix, counter, want)
		}
	}
}

func TestTOTPRejects(t *testing.T) {
	secret := NewTOTPSecret()
	now := time.Unix(1_800_000_000, 0)
	key, _ := b32.DecodeString(secret)
	good := totpCode(key, uint64(now.Unix()/30))

	if _, ok := VerifyTOTP(secret, good, now); !ok {
		t.Fatal("current code rejected")
	}
	if _, ok := VerifyTOTP(secret, good[:3]+" "+good[3:], now); !ok {
		t.Error("code with a space in the middle rejected")
	}
	if _, ok := VerifyTOTP(secret, good, now.Add(3*time.Minute)); ok {
		t.Error("code accepted three minutes later")
	}
	for _, bad := range []string{"", "12345", "1234567", "abcdef"} {
		if _, ok := VerifyTOTP(secret, bad, now); ok {
			t.Errorf("accepted %q", bad)
		}
	}
	if _, ok := VerifyTOTP("not base32!", good, now); ok {
		t.Error("accepted invalid secret")
	}
}

func TestTOTPURI(t *testing.T) {
	uri := TOTPURI("Nefo", "admin", "ABC")
	if !strings.HasPrefix(uri, "otpauth://totp/Nefo:admin?") || !strings.Contains(uri, "secret=ABC") {
		t.Errorf("unexpected URI %s", uri)
	}
}

func TestLimiter(t *testing.T) {
	l := NewLimiter(3, time.Minute, 10*time.Minute)
	now := time.Unix(1000, 0)
	for i := 0; i < 2; i++ {
		l.Fail("ip", now)
	}
	if ok, _ := l.Check("ip", now); !ok {
		t.Fatal("locked after 2 failures, max is 3")
	}
	l.Fail("ip", now)
	ok, wait := l.Check("ip", now)
	if ok || wait != 10*time.Minute {
		t.Fatalf("after 3 failures: ok=%v wait=%v", ok, wait)
	}
	if ok, _ := l.Check("other", now); !ok {
		t.Error("another key was locked")
	}
	if ok, _ := l.Check("ip", now.Add(11*time.Minute)); !ok {
		t.Error("still locked after the lockout")
	}
	// Failures spread over more than the window do not add up.
	l2 := NewLimiter(2, time.Minute, time.Hour)
	l2.Fail("ip", now)
	l2.Fail("ip", now.Add(2*time.Minute))
	if ok, _ := l2.Check("ip", now.Add(2*time.Minute)); !ok {
		t.Error("failures outside the window were counted together")
	}
}

func TestPassword(t *testing.T) {
	h, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(h, "correct horse battery staple") || CheckPassword(h, "wrong") {
		t.Error("CheckPassword gave the wrong answer")
	}
	if p := NewPassword(); len(p) != 20 {
		t.Errorf("NewPassword length %d", len(p))
	}
}
