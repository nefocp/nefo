// Package secure provides cryptographically secure random values.
package secure

import (
	"crypto/rand"
	"math/big"
)

const (
	// LowerAlnum is used for values that appear in URLs, such as the entrance path.
	LowerAlnum = "abcdefghijklmnopqrstuvwxyz0123456789"
	// PasswordChars leaves out characters that are easy to misread (0 O 1 l I).
	PasswordChars = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
)

// String returns n characters picked uniformly from alphabet.
func String(n int, alphabet string) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[Intn(len(alphabet))]
	}
	return string(b)
}

// Intn returns a uniform random integer in [0, n).
func Intn(n int) int {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return int(v.Int64())
}

// Bytes returns n random bytes.
func Bytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return b
}
