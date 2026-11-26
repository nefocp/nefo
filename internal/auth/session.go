package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"

	"github.com/nefocp/nefo/internal/secure"
)

const (
	// SessionTTL is how long a sign-in lasts.
	SessionTTL = 12 * time.Hour
	// CookieName is the session cookie.
	CookieName = "panel_session"
)

// NewSessionToken returns a random token for the cookie and the hash to store.
func NewSessionToken() (token, hash string) {
	token = base64.RawURLEncoding.EncodeToString(secure.Bytes(32))
	return token, HashToken(token)
}

// HashToken returns the stored form of a session token.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
