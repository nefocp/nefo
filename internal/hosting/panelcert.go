package hosting

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
)

// The panel's own certificate lives in <Certs>/_panel. The underscore keeps
// it apart from site certificates, whose folders are domain names.

// PanelCertFiles returns the paths of the panel's Let's Encrypt certificate.
func (m *Manager) PanelCertFiles() (cert, key string) {
	dir := filepath.Join(m.paths.Certs, "_panel")
	return filepath.Join(dir, "fullchain.pem"), filepath.Join(dir, "privkey.pem")
}

// IssuePanelCertificate gets a certificate for the panel's own domain. The
// web server answers the challenge on port 80 through its default server,
// so no website is needed for the domain.
func (m *Manager) IssuePanelCertificate(ctx context.Context, domain string, lg Logger) (time.Time, error) {
	if !m.Supported() {
		return time.Time{}, ErrUnsupported
	}
	if err := validate.Domain(domain); err != nil {
		return time.Time{}, usererr.New("invalid_domain", domain)
	}
	if !fileExists(m.web().hookFile()) {
		return time.Time{}, ErrNotReady
	}
	m.ops.Lock()
	defer m.ops.Unlock()

	// Older installations answered nothing on the default server: refresh it.
	if err := m.web().writeBase(ctx); err != nil {
		return time.Time{}, err
	}
	if err := m.reloadWeb(ctx); err != nil {
		return time.Time{}, err
	}
	lg.Logf("Checking that %s reaches this server", domain)
	if err := m.precheck(ctx, []string{domain}); err != nil {
		return time.Time{}, err
	}
	lg.Logf("Requesting a certificate from %s", m.caName())
	chain, key, leaf, err := m.obtain(ctx, []string{domain}, nil, lg)
	if err != nil {
		return time.Time{}, err
	}
	certFile, keyFile := m.PanelCertFiles()
	if err := mkdir(filepath.Dir(certFile), 0o700); err != nil {
		return time.Time{}, err
	}
	if err := writeFile(keyFile, key, 0o600); err != nil {
		return time.Time{}, err
	}
	if err := writeFile(certFile, chain, 0o600); err != nil {
		return time.Time{}, err
	}
	lg.Logf("Done. The panel uses the new certificate within a minute; it is valid until %s.", leaf.NotAfter.UTC().Format("2006-01-02"))
	return leaf.NotAfter, nil
}

// RemovePanelCertificate deletes the panel's certificate; the panel goes back
// to its self-signed one.
func (m *Manager) RemovePanelCertificate() error {
	cert, _ := m.PanelCertFiles()
	return os.RemoveAll(filepath.Dir(cert))
}

// PanelCertInfo describes the panel's Let's Encrypt certificate, if any.
type PanelCertInfo struct {
	Names     []string  `json:"names"`
	Issuer    string    `json:"issuer"`
	NotBefore time.Time `json:"not_before"`
	NotAfter  time.Time `json:"not_after"`
}

// PanelCert reads the panel's certificate. It returns nil when there is none.
func (m *Manager) PanelCert() (*PanelCertInfo, error) {
	cert, _ := m.PanelCertFiles()
	b, err := os.ReadFile(cert)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, errors.New("the panel certificate is not a PEM file")
	}
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	return &PanelCertInfo{Names: c.DNSNames, Issuer: c.Issuer.CommonName, NotBefore: c.NotBefore, NotAfter: c.NotAfter}, nil
}

// PanelCertDue reports whether the panel certificate is in the last third
// of its lifetime.
func PanelCertDue(info *PanelCertInfo, now time.Time) bool {
	if info == nil {
		return false
	}
	life := info.NotAfter.Sub(info.NotBefore)
	return info.NotAfter.Sub(now) < life/3
}
