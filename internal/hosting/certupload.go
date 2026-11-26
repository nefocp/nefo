package hosting

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
)

// SSLCustom marks a certificate the administrator installed (bought from
// another CA, or a Cloudflare origin certificate). The panel does not renew
// it, but warns before it expires.
const SSLCustom = "custom"

const maxPEM = 64 << 10

// CertWarning is something to know about an installed certificate.
type CertWarning struct {
	Code   string `json:"code"` // untrusted, expires_soon
	Detail string `json:"detail,omitempty"`
}

// parsedCert is a checked certificate and key, ready to install.
type parsedCert struct {
	chainPEM, keyPEM []byte
	leaf             *x509.Certificate
	warnings         []CertWarning
}

// parseCustomCert checks a PEM certificate chain and private key for names
// at time now. The chain is reordered so the certificate that matches the
// key comes first; the key is stored as PKCS#8.
func parseCustomCert(certPEM, keyPEM string, names []string, now time.Time) (*parsedCert, error) {
	if len(certPEM) > maxPEM || len(keyPEM) > maxPEM {
		return nil, usererr.New("invalid_certificate", "too large")
	}
	var certs []*x509.Certificate
	rest := []byte(certPEM)
	for {
		var b *pem.Block
		b, rest = pem.Decode(rest)
		if b == nil {
			break
		}
		if b.Type != "CERTIFICATE" {
			return nil, usererr.New("invalid_certificate", "unexpected "+b.Type+" block")
		}
		c, err := x509.ParseCertificate(b.Bytes)
		if err != nil {
			return nil, usererr.New("invalid_certificate", err.Error())
		}
		certs = append(certs, c)
	}
	if len(certs) == 0 {
		return nil, usererr.New("invalid_certificate", "no CERTIFICATE block")
	}

	kb, _ := pem.Decode([]byte(keyPEM))
	if kb == nil {
		return nil, usererr.New("invalid_private_key", "no PEM block")
	}
	if kb.Type == "ENCRYPTED PRIVATE KEY" || strings.Contains(kb.Headers["Proc-Type"], "ENCRYPTED") {
		return nil, usererr.New("encrypted_private_key")
	}
	key, err := parsePrivateKey(kb)
	if err != nil {
		return nil, usererr.New("invalid_private_key", err.Error())
	}
	pub, ok := key.(interface{ Public() crypto.PublicKey })
	if !ok {
		return nil, usererr.New("invalid_private_key", "unsupported key type")
	}
	type equaler interface{ Equal(crypto.PublicKey) bool }
	i := slices.IndexFunc(certs, func(c *x509.Certificate) bool {
		e, ok := c.PublicKey.(equaler)
		return ok && e.Equal(pub.Public())
	})
	if i < 0 {
		return nil, usererr.New("cert_key_mismatch")
	}
	leaf := certs[i]
	chain := append([]*x509.Certificate{leaf}, slices.Delete(slices.Clone(certs), i, i+1)...)

	if now.Before(leaf.NotBefore) {
		return nil, usererr.New("certificate_not_yet_valid", leaf.NotBefore.UTC().Format("2006-01-02"))
	}
	if now.After(leaf.NotAfter) {
		return nil, usererr.New("certificate_expired", leaf.NotAfter.UTC().Format("2006-01-02"))
	}
	var missing []string
	for _, n := range names {
		var ok bool
		if validate.IsWildcard(n) {
			ok = slices.Contains(leaf.DNSNames, n)
		} else {
			ok = leaf.VerifyHostname(n) == nil
		}
		if !ok {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		return nil, usererr.New("certificate_names", strings.Join(missing, ", "))
	}

	p := &parsedCert{leaf: leaf}
	inter := x509.NewCertPool()
	for _, c := range chain[1:] {
		inter.AddCert(c)
	}
	if _, err := leaf.Verify(x509.VerifyOptions{Intermediates: inter, CurrentTime: now}); err != nil {
		// A Cloudflare origin certificate, for one, is only trusted by
		// Cloudflare: allowed, with a warning.
		p.warnings = append(p.warnings, CertWarning{Code: "untrusted", Detail: err.Error()})
	}
	if leaf.NotAfter.Sub(now) < 14*24*time.Hour {
		p.warnings = append(p.warnings, CertWarning{Code: "expires_soon", Detail: leaf.NotAfter.UTC().Format("2006-01-02")})
	}
	var buf bytes.Buffer
	for _, c := range chain {
		pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: c.Raw})
	}
	p.chainPEM = buf.Bytes()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, usererr.New("invalid_private_key", err.Error())
	}
	p.keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	return p, nil
}

func parsePrivateKey(b *pem.Block) (any, error) {
	if k, err := x509.ParsePKCS8PrivateKey(b.Bytes); err == nil {
		return k, nil
	}
	if k, err := x509.ParsePKCS1PrivateKey(b.Bytes); err == nil {
		return k, nil
	}
	if k, err := x509.ParseECPrivateKey(b.Bytes); err == nil {
		return k, nil
	}
	return nil, errors.New("not a PKCS#8, PKCS#1 or EC private key")
}

// InstallCertificate installs the administrator's own certificate for a
// site and turns on HTTPS. The panel will not renew it.
func (m *Manager) InstallCertificate(ctx context.Context, id int64, certPEM, keyPEM string) (*store.Site, []CertWarning, error) {
	if !m.Supported() {
		return nil, nil, ErrUnsupported
	}
	if err := m.tryLock(); err != nil {
		return nil, nil, err
	}
	defer m.ops.Unlock()
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	p, err := parseCustomCert(certPEM, keyPEM, site.Names(), m.now())
	if err != nil {
		return nil, nil, err
	}

	dir := m.paths.certDir(site.Domain)
	if err := mkdir(dir, 0o700); err != nil {
		return nil, nil, err
	}
	files := map[string][]byte{"privkey.pem": p.keyPEM, "fullchain.pem": p.chainPEM}
	backup := map[string][]byte{}
	for name := range files {
		if b, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
			backup[name] = b
		}
	}
	restore := func() {
		for name := range files {
			if b, ok := backup[name]; ok {
				writeFile(filepath.Join(dir, name), b, 0o600)
			} else {
				os.Remove(filepath.Join(dir, name))
			}
		}
	}
	// Key first: the web server is only reloaded after both files are in place.
	for _, name := range []string{"privkey.pem", "fullchain.pem"} {
		if err := writeFile(filepath.Join(dir, name), files[name], 0o600); err != nil {
			restore()
			return nil, nil, err
		}
	}

	old := *site
	site.SSL, site.SSLSource = true, SSLCustom
	site.CertNotBefore, site.CertNotAfter = p.leaf.NotBefore, p.leaf.NotAfter
	site.SSLError, site.SSLAttemptAt = "", m.now()
	if err := m.updateSite(ctx, site, &old); err != nil {
		restore()
		if old.SSL { // the web server may have loaded the new files meanwhile
			m.reloadWeb(context.WithoutCancel(ctx))
		}
		return nil, nil, err
	}
	if !old.SSL {
		m.refreshDockerApp(ctx, site, slogLogger{m})
	}
	if site.App == AppWordPress && !old.SSL {
		if err := m.wpSetURL(ctx, site, "https://"+site.Domain); err != nil {
			m.log.Warn("switch WordPress to https", "domain", site.Domain, "err", err)
		}
	}
	return site, p.warnings, nil
}
