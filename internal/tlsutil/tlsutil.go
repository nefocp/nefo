// Package tlsutil creates the self-signed certificate the panel uses until
// the owner configures a real one.
package tlsutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"regexp"
	"time"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/version"
)

const (
	// 397 days: the longest validity that browsers and macOS accept even
	// when the owner adds the certificate to their trust store.
	validity    = 397 * 24 * time.Hour
	renewBefore = 30 * 24 * time.Hour
)

var hostnameRe = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9.-]*[a-zA-Z0-9])?$`)

// EnsureSelfSigned makes sure a usable certificate exists at certPath and
// keyPath, creating or renewing it when it is missing or close to expiry.
// It reports whether a new certificate was written.
func EnsureSelfSigned(certPath, keyPath string) (bool, error) {
	if fsutil.Exists(certPath) && fsutil.Exists(keyPath) && !expiringSoon(certPath) {
		return false, nil
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return false, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		return false, err
	}

	dns := []string{"localhost"}
	host, _ := os.Hostname()
	if hostnameRe.MatchString(host) && host != "localhost" {
		dns = append(dns, host)
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: host, Organization: []string{version.AppName + " self-signed"}},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(validity),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              dns,
		IPAddresses:           localIPs(),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return false, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return false, err
	}

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	if err := fsutil.WriteFileAtomic(keyPath, keyPEM, 0o600); err != nil {
		return false, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := fsutil.WriteFileAtomic(certPath, certPEM, 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func expiringSoon(certPath string) bool {
	b, err := os.ReadFile(certPath)
	if err != nil {
		return true
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return true
	}
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return true
	}
	return time.Until(c.NotAfter) < renewBefore
}

func localIPs() []net.IP {
	ips := []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ips
	}
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && !n.IP.IsLoopback() && !n.IP.IsLinkLocalUnicast() {
			ips = append(ips, n.IP)
		}
	}
	return ips
}
