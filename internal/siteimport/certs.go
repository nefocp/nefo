package siteimport

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"time"
)

// certPool holds the certificates and keys found in a backup. Keys are
// kept only while the archive is read: the analysis keeps a summary, the
// import installs the chosen certificate with the panel's own checks.
type certPool struct {
	certs []*poolCert
	keys  []crypto.PrivateKey
	files int
}

type poolCert struct {
	cert  *x509.Certificate
	group string // the file (or file name without extension) it came from
}

const (
	maxCertFiles = 2000
	maxPEMFile   = 64 << 10
)

// add reads the PEM blocks of one file.
func (p *certPool) add(group string, b []byte) {
	if p.files++; p.files > maxCertFiles {
		return
	}
	for {
		var blk *pem.Block
		blk, b = pem.Decode(b)
		if blk == nil {
			return
		}
		switch {
		case blk.Type == "CERTIFICATE":
			if c, err := x509.ParseCertificate(blk.Bytes); err == nil {
				p.certs = append(p.certs, &poolCert{cert: c, group: group})
			}
		case strings.HasSuffix(blk.Type, "PRIVATE KEY") && !strings.Contains(blk.Type, "ENCRYPTED") && blk.Headers["Proc-Type"] == "":
			if k := parseKey(blk.Bytes); k != nil {
				p.keys = append(p.keys, k)
			}
		}
	}
}

func parseKey(der []byte) crypto.PrivateKey {
	if k, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		return k
	}
	if k, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return k
	}
	if k, err := x509.ParseECPrivateKey(der); err == nil {
		return k
	}
	return nil
}

func (p *certPool) keyFor(c *x509.Certificate) crypto.PrivateKey {
	type equaler interface{ Equal(crypto.PublicKey) bool }
	pub, ok := c.PublicKey.(equaler)
	if !ok {
		return nil
	}
	for _, k := range p.keys {
		if s, ok := k.(interface{ Public() crypto.PublicKey }); ok && pub.Equal(s.Public()) {
			return k
		}
	}
	return nil
}

// pick is the certificate chosen for a website.
type pick struct {
	summary *Cert
	chain   []byte // PEM: the certificate, then the others of its file
	key     []byte // PEM, PKCS#8
}

// best returns the certificate for domain: one for its name with its key,
// valid now, lasting longest; or, when none is usable, the best of the
// others so the plan can say what is wrong. names are all the website's
// names, which the panel requires it to cover.
func (p *certPool) best(domain string, names []string, now time.Time) *pick {
	var bestC *poolCert
	var bestKey crypto.PrivateKey
	score := func(c *poolCert, k crypto.PrivateKey) int {
		s := 0
		if k != nil {
			s += 4
		}
		if !now.Before(c.cert.NotBefore) && now.Before(c.cert.NotAfter) {
			s += 2
		}
		return s
	}
	for _, c := range p.certs {
		// CA certificates are the chain's, not the website's; but
		// "openssl req -x509" marks a self-signed website certificate as a
		// CA too, so one that signed itself still counts.
		if (c.cert.IsCA && !signedItself(c.cert)) || c.cert.VerifyHostname(domain) != nil {
			continue
		}
		k := p.keyFor(c.cert)
		if bestC == nil || score(c, k) > score(bestC, bestKey) ||
			(score(c, k) == score(bestC, bestKey) && c.cert.NotAfter.After(bestC.cert.NotAfter)) {
			bestC, bestKey = c, k
		}
	}
	if bestC == nil {
		return nil
	}
	leaf := bestC.cert
	s := &Cert{Names: append([]string{}, leaf.DNSNames...), Issuer: issuerName(leaf), NotAfter: leaf.NotAfter}
	inter := x509.NewCertPool()
	var chain bytes.Buffer
	pem.Encode(&chain, &pem.Block{Type: "CERTIFICATE", Bytes: leaf.Raw})
	for _, c := range p.certs {
		if c.group == bestC.group && c != bestC && !bytes.Equal(c.cert.Raw, leaf.Raw) {
			inter.AddCert(c.cert)
			pem.Encode(&chain, &pem.Block{Type: "CERTIFICATE", Bytes: c.cert.Raw})
		}
	}
	_, err := leaf.Verify(x509.VerifyOptions{Intermediates: inter, CurrentTime: now})
	s.Trusted = err == nil
	for _, n := range names {
		if leaf.VerifyHostname(n) != nil {
			s.Missing = append(s.Missing, n)
		}
	}
	switch {
	case bestKey == nil:
		s.Problem = "no_key"
	case now.After(leaf.NotAfter):
		s.Problem = "expired"
	case now.Before(leaf.NotBefore):
		s.Problem = "not_yet_valid"
	case len(s.Missing) > 0:
		s.Problem = "names"
	}
	s.Usable = s.Problem == ""
	pk := &pick{summary: s, chain: chain.Bytes()}
	if bestKey != nil {
		if der, err := x509.MarshalPKCS8PrivateKey(bestKey); err == nil {
			pk.key = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
		}
	}
	return pk
}

// signedItself reports whether c signed itself.
func signedItself(c *x509.Certificate) bool {
	return bytes.Equal(c.RawIssuer, c.RawSubject) && c.CheckSignatureFrom(c) == nil
}

func issuerName(c *x509.Certificate) string {
	if len(c.Issuer.Organization) > 0 {
		return c.Issuer.Organization[0]
	}
	if c.Issuer.CommonName != "" {
		return c.Issuer.CommonName
	}
	return c.Issuer.String()
}

// certGroup names the group of a certificate file: a file of apache_tls/
// is a group of its own; DOMAIN.crt and DOMAIN.cabundle share one.
func certGroup(p string) string {
	if i := strings.LastIndexByte(p, '.'); i > strings.LastIndexByte(p, '/') {
		switch p[i:] {
		case ".crt", ".cabundle", ".key", ".pem", ".csr":
			return p[:i]
		}
	}
	return p
}

// certFile tells whether a backup path (without its top folder) may hold
// certificates or keys.
func certFile(p string) bool {
	switch {
	case strings.HasPrefix(p, "apache_tls/"), strings.HasPrefix(p, "sslcerts/"), strings.HasPrefix(p, "sslkeys/"):
		return true
	case strings.HasPrefix(p, "homedir/ssl/certs/"), strings.HasPrefix(p, "homedir/ssl/keys/"):
		return strings.HasSuffix(p, ".crt") || strings.HasSuffix(p, ".key") || strings.HasSuffix(p, ".cabundle")
	}
	return false
}
