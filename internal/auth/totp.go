package auth

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/secure"
)

// TOTP as in RFC 6238, with the settings every authenticator app supports.
const (
	totpPeriod = 30
	totpDigits = 6
	totpSkew   = 1 // also accept the previous and next code, for clock drift
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewTOTPSecret returns a new random base32 secret (160 bits).
func NewTOTPSecret() string {
	return b32.EncodeToString(secure.Bytes(20))
}

func totpCode(key []byte, counter uint64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], counter)
	m := hmac.New(sha1.New, key)
	m.Write(msg[:])
	sum := m.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	v := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%0*d", totpDigits, v%1_000_000)
}

// VerifyTOTP checks code against secret at time t. It returns the time-step
// counter of the matching code, so callers can refuse to accept the same
// code twice.
func VerifyTOTP(secret, code string, t time.Time) (uint64, bool) {
	code = strings.ReplaceAll(strings.TrimSpace(code), " ", "")
	if len(code) != totpDigits {
		return 0, false
	}
	key, err := b32.DecodeString(strings.ToUpper(secret))
	if err != nil || len(key) == 0 {
		return 0, false
	}
	now := t.Unix() / totpPeriod
	for d := int64(-totpSkew); d <= totpSkew; d++ {
		c := uint64(now + d)
		if subtle.ConstantTimeCompare([]byte(totpCode(key, c)), []byte(code)) == 1 {
			return c, true
		}
	}
	return 0, false
}

// GenerateTOTP returns the code for secret at time t. The panel itself only
// verifies codes; this is for tests and tooling.
func GenerateTOTP(secret string, t time.Time) (string, error) {
	key, err := b32.DecodeString(strings.ToUpper(secret))
	if err != nil {
		return "", err
	}
	return totpCode(key, uint64(t.Unix()/totpPeriod)), nil
}

// TOTPURI returns the otpauth:// URI that authenticator apps read from a QR code.
func TOTPURI(issuer, account, secret string) string {
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprint(totpDigits))
	q.Set("period", fmt.Sprint(totpPeriod))
	return "otpauth://totp/" + url.PathEscape(issuer+":"+account) + "?" + q.Encode()
}
