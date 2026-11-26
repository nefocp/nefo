// Package auth contains password hashing, TOTP two-factor codes, session
// tokens and login rate limiting.
package auth

import (
	"sync"

	"golang.org/x/crypto/bcrypt"

	"github.com/nefocp/nefo/internal/secure"
)

const bcryptCost = 12

// HashPassword returns a bcrypt hash of pw.
func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcryptCost)
	return string(b), err
}

// CheckPassword reports whether pw matches hash.
func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

var dummyHash = sync.OnceValue(func() []byte {
	h, _ := bcrypt.GenerateFromPassword([]byte("timing-equaliser"), bcryptCost)
	return h
})

// BurnPasswordCheck spends the same time as CheckPassword. Call it when the
// username does not exist, so response times do not reveal valid usernames.
func BurnPasswordCheck(pw string) {
	_ = bcrypt.CompareHashAndPassword(dummyHash(), []byte(pw))
}

// NewPassword returns a random password for new or reset accounts.
func NewPassword() string {
	return secure.String(20, secure.PasswordChars)
}
