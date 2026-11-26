package appstore

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"

	"github.com/nefocp/nefo/internal/secure"
)

const (
	alnum      = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
	lowerAlnum = "abcdefghijkmnpqrstuvwxyz23456789"
)

// GenerateSecrets returns the recipe's secrets: those already in have are
// kept (an update or a re-render), the others are generated. Hashes are
// made after the values they hash.
func GenerateSecrets(specs map[string]*SecretSpec, have map[string]string) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range have {
		if _, ok := specs[k]; ok {
			out[k] = v
		}
	}
	for _, name := range sortedKeys(specs) {
		s := specs[name]
		if _, ok := out[name]; ok {
			continue
		}
		switch s.Gen {
		case "random":
			chars := alnum
			if s.Chars == "lower-alnum" {
				chars = lowerAlnum
			}
			out[name] = secure.String(s.Length, chars)
		case "hex":
			out[name] = hex.EncodeToString(secure.Bytes((s.Length + 1) / 2))[:s.Length]
		}
	}
	for _, name := range sortedKeys(specs) {
		s := specs[name]
		if _, ok := out[name]; ok {
			continue
		}
		src, ok := out[s.From]
		if !ok {
			return nil, fmt.Errorf("secret %s: %s missing", name, s.From)
		}
		switch s.Gen {
		case "bcrypt":
			cost := s.Cost
			if cost == 0 {
				cost = 10
			}
			h, err := bcrypt.GenerateFromPassword([]byte(src), cost)
			if err != nil {
				return nil, err
			}
			out[name] = string(h)
		case "argon2id":
			out[name] = Argon2id(src)
		default:
			return nil, fmt.Errorf("secret %s: unknown gen %q", name, s.Gen)
		}
	}
	return out, nil
}

// Argon2id returns a PHC string ($argon2id$v=19$m=65536,t=3,p=4$salt$hash),
// the form Vaultwarden's ADMIN_TOKEN and code-server's HASHED_PASSWORD read.
func Argon2id(password string) string {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	const m, t, p = 64 * 1024, 3, 4
	key := argon2.IDKey([]byte(password), salt, t, m, p, 32)
	enc := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, m, t, p, enc.EncodeToString(salt), enc.EncodeToString(key))
}
