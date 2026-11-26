package postgres

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/nefocp/nefo/internal/secure"
)

// scramIterations is PostgreSQL's default (scram_iterations).
const scramIterations = 4096

// scramVerifier returns the SCRAM-SHA-256 secret PostgreSQL stores for a
// password (RFC 5802 and 7677), as psql's \password and libpq's
// PQencryptPasswordConn make it. Given in CREATE ROLE or ALTER ROLE, the
// server stores it as it is, so the password itself never reaches the
// server, its statistics or its log. The panel's passwords are printable
// ASCII (validate.DBPassword), which SASLprep leaves as they are.
func scramVerifier(password string) string {
	return scramVerifierSalt(password, secure.Bytes(16), scramIterations)
}

func scramVerifierSalt(password string, salt []byte, iter int) string {
	storedKey, serverKey := scramKeys(password, salt, iter)
	b64 := base64.StdEncoding.EncodeToString
	return fmt.Sprintf("SCRAM-SHA-256$%d:%s$%s:%s", iter, b64(salt), b64(storedKey), b64(serverKey))
}

// scramKeys returns the StoredKey and the ServerKey of a password.
func scramKeys(password string, salt []byte, iter int) (storedKey, serverKey []byte) {
	salted, err := pbkdf2.Key(sha256.New, password, salt, iter, sha256.Size)
	if err != nil {
		panic(err) // only for invalid parameters, which are constant here
	}
	clientKey := hmacSHA256(salted, "Client Key")
	sum := sha256.Sum256(clientKey)
	return sum[:], hmacSHA256(salted, "Server Key")
}

func hmacSHA256(key []byte, msg string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(msg))
	return h.Sum(nil)
}
