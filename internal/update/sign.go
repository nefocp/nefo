package update

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	_ "embed"
	"encoding/pem"
	"errors"
)

// releasePub is the project's Ed25519 release signing key in PEM form (see
// RELEASING.md). Releases sign checksums.txt with the matching private key.
// An empty file (development builds) turns signature checks off.
//
//go:embed release.pub
var releasePub []byte

// signingKey returns the embedded public key, or nil when there is none.
func signingKey() (ed25519.PublicKey, error) {
	b := bytes.TrimSpace(releasePub)
	if len(b) == 0 {
		return nil, nil
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, errors.New("release.pub is not a PEM file")
	}
	k, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	pub, ok := k.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("release.pub is not an Ed25519 key")
	}
	return pub, nil
}

// HasSigningKey reports whether this build has the project's release key.
// Development builds do not, and then nothing signed is accepted from the
// network (see Verify).
func HasSigningKey() bool {
	k, err := signingKey()
	return err == nil && k != nil
}

// ErrNoSigningKey is returned by Verify in builds without a release key.
var ErrNoSigningKey = errors.New("this build has no release signing key")

// Verify checks an Ed25519 signature made with the release key over data,
// for files other than releases (the app store's catalog). Unlike release
// checks, a build without a key refuses everything.
func Verify(data, sig []byte) error {
	k, err := signingKey()
	if err != nil {
		return err
	}
	if k == nil {
		return ErrNoSigningKey
	}
	if len(sig) != ed25519.SignatureSize || !ed25519.Verify(k, data, sig) {
		return errors.New("the signature does not match the project's release key")
	}
	return nil
}

// checkSignature verifies an Ed25519 signature (as made by
// "openssl pkeyutl -sign -rawin") over data.
func checkSignature(pub ed25519.PublicKey, data, sig []byte) error {
	if len(sig) != ed25519.SignatureSize || !ed25519.Verify(pub, data, sig) {
		return errors.New("checksums.txt has no valid signature from the project: the update was not installed")
	}
	return nil
}
