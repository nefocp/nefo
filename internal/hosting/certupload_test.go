package hosting

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/usererr"
)

type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

func newTestCA(t *testing.T) *testCA {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Test CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(365 * 24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := x509.ParseCertificate(der)
	return &testCA{c, key}
}

// leaf issues a certificate for names; it returns the leaf PEM, the CA PEM
// and the leaf's key PEM (PKCS#1 for RSA, SEC 1 for EC).
func (ca *testCA) leaf(t *testing.T, names []string, notAfter time.Time, rsaKey bool) (string, string, string) {
	t.Helper()
	var pub any
	var keyPEM []byte
	if rsaKey {
		k, _ := rsa.GenerateKey(rand.Reader, 2048)
		pub = &k.PublicKey
		keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)})
	} else {
		k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		pub = &k.PublicKey
		der, _ := x509.MarshalECPrivateKey(k)
		keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()), Subject: pkix.Name{CommonName: names[0]},
		DNSNames: names, NotBefore: time.Now().Add(-time.Hour), NotAfter: notAfter,
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, pub, ca.key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
		string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.cert.Raw})),
		string(keyPEM)
}

func wantCode(t *testing.T, err error, code string) {
	t.Helper()
	var ue *usererr.Error
	if !errors.As(err, &ue) || ue.Code != code {
		t.Fatalf("err = %v, want %s", err, code)
	}
}

func TestParseCustomCert(t *testing.T) {
	ca := newTestCA(t)
	now := time.Now()
	year := now.Add(300 * 24 * time.Hour)
	names := []string{"example.com", "www.example.com"}

	leaf, caPEM, key := ca.leaf(t, names, year, true)
	// The CA first: the leaf (matching the key) must be moved to the front.
	p, err := parseCustomCert(caPEM+leaf, key, names, now)
	if err != nil {
		t.Fatal(err)
	}
	if p.leaf.Subject.CommonName != "example.com" {
		t.Fatalf("leaf = %s", p.leaf.Subject.CommonName)
	}
	blocks := strings.Count(string(p.chainPEM), "BEGIN CERTIFICATE")
	first, _ := pem.Decode(p.chainPEM)
	if c, _ := x509.ParseCertificate(first.Bytes); blocks != 2 || c.Subject.CommonName != "example.com" {
		t.Fatalf("chain not reordered (%d blocks)", blocks)
	}
	if !strings.HasPrefix(string(p.keyPEM), "-----BEGIN PRIVATE KEY-----") {
		t.Fatal("key not stored as PKCS#8")
	}
	if len(p.warnings) != 1 || p.warnings[0].Code != "untrusted" {
		t.Fatalf("warnings = %+v", p.warnings)
	}

	// EC key, wildcard alias.
	wl, _, wk := ca.leaf(t, []string{"example.com", "*.example.com"}, year, false)
	if _, err := parseCustomCert(wl, wk, []string{"example.com", "*.example.com", "shop.example.com"}, now); err != nil {
		t.Fatalf("wildcard: %v", err)
	}
	// A wildcard alias needs the wildcard itself in the certificate.
	_, err = parseCustomCert(leaf, key, []string{"example.com", "*.example.com"}, now)
	wantCode(t, err, "certificate_names")

	other, _, otherKey := ca.leaf(t, names, year, false)
	_, err = parseCustomCert(leaf, otherKey, names, now)
	wantCode(t, err, "cert_key_mismatch")

	_, err = parseCustomCert(other, otherKey, []string{"example.com", "shop.example.org"}, now)
	wantCode(t, err, "certificate_names")

	_, err = parseCustomCert(leaf, key, names, year.Add(time.Hour))
	wantCode(t, err, "certificate_expired")

	soon, _, soonKey := ca.leaf(t, names, now.Add(5*24*time.Hour), false)
	p, err = parseCustomCert(soon, soonKey, names, now)
	if err != nil || len(p.warnings) != 2 || p.warnings[1].Code != "expires_soon" {
		t.Fatalf("expiring: %v %+v", err, p)
	}

	_, err = parseCustomCert("not a certificate", key, names, now)
	wantCode(t, err, "invalid_certificate")
	_, err = parseCustomCert(leaf+key, key, names, now)
	wantCode(t, err, "invalid_certificate")
	_, err = parseCustomCert(leaf, "-----BEGIN ENCRYPTED PRIVATE KEY-----\nAAAA\n-----END ENCRYPTED PRIVATE KEY-----\n", names, now)
	wantCode(t, err, "encrypted_private_key")
	_, err = parseCustomCert(leaf, "garbage", names, now)
	wantCode(t, err, "invalid_private_key")
}
