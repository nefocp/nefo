package hosting

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"golang.org/x/crypto/acme"

	"github.com/nefocp/nefo/internal/config"
	"github.com/nefocp/nefo/internal/secure"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/validate"
	"github.com/nefocp/nefo/internal/version"
)

// Certificates come from Let's Encrypt (or another ACME CA) with HTTP-01
// challenges. Every site's web server configuration serves
// /.well-known/acme-challenge/ from one panel-owned folder, so validation
// works whatever the site's own files do.

var challengeTokenRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,256}$`)

// IssueOptions change how a certificate is requested.
type IssueOptions struct {
	// DNS, if set, turns DNS validation (through Cloudflare) on or off for
	// this site and its renewals.
	DNS *bool
}

// IssueCertificate gets a certificate for all names of a site and turns on
// HTTPS. lg receives progress messages.
func (m *Manager) IssueCertificate(ctx context.Context, id int64, opts IssueOptions, lg Logger) error {
	if !m.Supported() {
		return ErrUnsupported
	}
	m.ops.Lock()
	defer m.ops.Unlock()

	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return err
	}
	if opts.DNS != nil && *opts.DNS != site.SSLDNS {
		site.SSLDNS = *opts.DNS
		if err := m.store.UpdateSite(ctx, site); err != nil {
			return err
		}
	}
	if err := m.obtainAndInstall(ctx, site, lg); err != nil {
		m.recordSSLFailure(context.WithoutCancel(ctx), id, err)
		return err
	}
	return nil
}

// RenewRetry is how long a failed automatic renewal waits before the next
// attempt, for the websites' certificates and the panel's own. Let's
// Encrypt pauses a name after many failed validations in a row, which
// hourly retries of a name that stays broken would reach in a few weeks.
const RenewRetry = 12 * time.Hour

// RenewDue renews the certificates that are in the last third of their
// lifetime (30 days for a 90-day certificate). A failed renewal is retried
// after RenewRetry. report is called for every attempt, and once a day, with
// an error, for a certificate the administrator installed that expires
// within two weeks.
func (m *Manager) RenewDue(ctx context.Context, report func(domain string, err error)) {
	if !m.Supported() {
		return
	}
	sites, err := m.store.ListSites(ctx)
	if err != nil {
		m.log.Warn("list sites for renewal", "err", err)
		return
	}
	now := m.now()
	for _, s := range sites {
		if s.SSL && s.SSLSource == SSLCustom {
			// Not ours to renew: remind once a day in the last two weeks
			// (the audit log and the alerts). It is not a failed renewal,
			// so the site's certificate error stays as it is; the UI warns
			// from the source and the expiry date.
			if s.CertNotAfter.Sub(now) < 14*24*time.Hour && now.Sub(s.SSLAttemptAt) > 24*time.Hour {
				m.recordCustomReminder(ctx, s.ID)
				report(s.Domain, fmt.Errorf("the installed certificate expires on %s", s.CertNotAfter.UTC().Format("2006-01-02")))
			}
			continue
		}
		if !s.SSL || !renewalDue(s, now) {
			continue
		}
		if s.SSLError != "" && now.Sub(s.SSLAttemptAt) < RenewRetry {
			continue
		}
		m.ops.Lock()
		site, err := m.store.SiteByID(ctx, s.ID) // it may have changed meanwhile
		if err == nil && (!site.SSL || site.SSLSource == SSLCustom) {
			m.ops.Unlock() // HTTPS turned off, or a certificate installed
			continue
		}
		if err == nil {
			err = m.obtainAndInstall(ctx, site, slogLogger{m})
			if err != nil {
				m.recordSSLFailure(context.WithoutCancel(ctx), s.ID, err)
			}
		}
		m.ops.Unlock()
		if ctx.Err() != nil {
			return
		}
		if !errors.Is(err, store.ErrNotFound) {
			report(s.Domain, err)
		}
	}
}

func renewalDue(s *store.Site, now time.Time) bool {
	life := s.CertNotAfter.Sub(s.CertNotBefore)
	if s.CertNotBefore.IsZero() || life <= 0 {
		life = 90 * 24 * time.Hour
	}
	return s.CertNotAfter.Sub(now) < life/3
}

func (m *Manager) recordSSLFailure(ctx context.Context, id int64, cause error) {
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return
	}
	msg := cause.Error()
	if len(msg) > 2000 {
		msg = msg[:2000]
	}
	site.SSLError = msg
	site.SSLAttemptAt = m.now()
	if err := m.store.UpdateSite(ctx, site); err != nil {
		m.log.Warn("save certificate error", "domain", site.Domain, "err", err)
	}
}

// legacyCustomReminder starts the English reminder that older versions
// stored as the certificate error of a site with an installed certificate.
const legacyCustomReminder = "the certificate you installed expires on "

// recordCustomReminder notes when the administrator was last reminded that
// their certificate expires, and clears an older version's reminder text.
func (m *Manager) recordCustomReminder(ctx context.Context, id int64) {
	m.ops.Lock()
	defer m.ops.Unlock()
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return
	}
	if strings.HasPrefix(site.SSLError, legacyCustomReminder) {
		site.SSLError = ""
	}
	site.SSLAttemptAt = m.now()
	if err := m.store.UpdateSite(ctx, site); err != nil {
		m.log.Warn("save certificate reminder", "domain", site.Domain, "err", err)
	}
}

// usesDNS reports whether a site's certificate needs DNS validation:
// wildcard names can only be validated that way.
func usesDNS(site *store.Site) bool {
	return site.SSLDNS || slices.ContainsFunc(site.Aliases, validate.IsWildcard)
}

func (m *Manager) obtainAndInstall(ctx context.Context, site *store.Site, lg Logger) error {
	names := site.Names()
	var solver dnsSolver
	if usesDNS(site) {
		cf, err := m.cloudflareSolver(ctx)
		if err != nil {
			return err
		}
		solver = cf
		lg.Logf("Validating %s through DNS (Cloudflare)", strings.Join(names, ", "))
	} else {
		lg.Logf("Checking that %s reach this server", strings.Join(names, ", "))
		if err := m.precheck(ctx, names); err != nil {
			return err
		}
	}

	lg.Logf("Requesting a certificate from %s", m.caName())
	chain, key, leaf, err := m.obtain(ctx, names, solver, lg)
	if err != nil {
		return err
	}

	dir := m.paths.certDir(site.Domain)
	if err := mkdir(dir, 0o700); err != nil {
		return err
	}
	// Key first: the web server is only reloaded after both files are in place.
	if err := writeFile(filepath.Join(dir, "privkey.pem"), key, 0o600); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(dir, "fullchain.pem"), chain, 0o600); err != nil {
		return err
	}

	old := *site
	site.SSL, site.SSLSource = true, ""
	site.CertNotBefore, site.CertNotAfter = leaf.NotBefore, leaf.NotAfter
	site.SSLError, site.SSLAttemptAt = "", m.now()
	lg.Logf("Turning on HTTPS in %s", m.web().title())
	if err := m.updateSite(ctx, site, &old); err != nil {
		return err
	}
	if !old.SSL {
		m.refreshDockerApp(ctx, site, lg)
	}
	if site.App == AppWordPress && !old.SSL {
		lg.Logf("Switching the WordPress address to https://%s", site.Domain)
		if err := m.wpSetURL(ctx, site, "https://"+site.Domain); err != nil {
			lg.Logf("Warning: %v", err)
		}
	}
	lg.Logf("Done. The certificate is valid until %s.", leaf.NotAfter.UTC().Format("2006-01-02"))
	return nil
}

func (m *Manager) caName() string {
	switch m.acmeDirectory {
	case config.LetsEncrypt:
		return "Let's Encrypt"
	case config.LetsEncryptStaging:
		return "Let's Encrypt (staging: the certificate will not be trusted by browsers)"
	}
	return m.acmeDirectory
}

// precheck makes sure every name reaches this server over HTTP before asking
// the CA, which limits failed validations per hour. It serves a random file
// from the challenge folder and fetches it through each name.
func (m *Manager) precheck(ctx context.Context, names []string) error {
	file := "panel-check-" + secure.String(24, secure.LowerAlnum)
	want := secure.String(32, secure.LowerAlnum)
	path := filepath.Join(m.paths.challengeDir(), file)
	if err := writeFile(path, []byte(want), 0o644); err != nil {
		return err
	}
	defer os.Remove(path)

	var problems []string
	for _, n := range names {
		if p := m.probeName(ctx, n, file, want); p != "" {
			problems = append(problems, p)
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("these names do not reach this server yet, so no certificate was requested:\n- %s",
			strings.Join(problems, "\n- "))
	}
	return nil
}

func (m *Manager) probeName(ctx context.Context, name, file, want string) string {
	addrs, err := net.DefaultResolver.LookupHost(ctx, name)
	if err != nil || len(addrs) == 0 {
		return name + ": no DNS record. Add an A record that points to this server's IP address."
	}
	where := name + " (" + strings.Join(addrs, ", ") + ")"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+name+"/.well-known/acme-challenge/"+file, nil)
	if err != nil {
		return where + ": " + err.Error()
	}
	req.Header.Set("User-Agent", version.AppName+"/"+version.Version+" (domain check)")
	resp, err := m.probe.Do(req)
	if err != nil {
		reason := err.Error()
		var uerr *url.Error
		if errors.As(err, &uerr) {
			reason = uerr.Err.Error()
		}
		if errors.Is(err, io.EOF) {
			reason = "the connection was closed without an answer"
		}
		return where + ": no answer over HTTP on port 80 (" + reason + "). Check the DNS record and the firewall."
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if resp.StatusCode != http.StatusOK || strings.TrimSpace(string(body)) != want {
		return fmt.Sprintf("%s answered HTTP %d, but not from this server. Check that the DNS record points here.",
			where, resp.StatusCode)
	}
	return ""
}

// ---- ACME ----

func (m *Manager) acmeClient(ctx context.Context) (*acme.Client, error) {
	m.acmeMu.Lock()
	defer m.acmeMu.Unlock()
	key, err := m.accountKey()
	if err != nil {
		return nil, err
	}
	c := &acme.Client{Key: key, DirectoryURL: m.acmeDirectory, UserAgent: version.AppName + "/" + version.Version}
	if !m.acmeRegistered {
		// Registering an existing key returns ErrAccountAlreadyExists, which is fine.
		if _, err := c.Register(ctx, &acme.Account{}, acme.AcceptTOS); err != nil && !errors.Is(err, acme.ErrAccountAlreadyExists) {
			return nil, fmt.Errorf("create the ACME account: %w", err)
		}
		m.acmeRegistered = true
	}
	return c, nil
}

// accountKey loads the ACME account key, creating it on first use.
func (m *Manager) accountKey() (*ecdsa.PrivateKey, error) {
	path := filepath.Join(m.paths.ACME, "account.key")
	if b, err := os.ReadFile(path); err == nil {
		block, _ := pem.Decode(b)
		if block == nil {
			return nil, fmt.Errorf("%s is not a PEM file", path)
		}
		return x509.ParseECPrivateKey(block.Bytes)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	if err := mkdir(m.paths.ACME, 0o700); err != nil {
		return nil, err
	}
	if err := writeFile(path, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

// obtain runs an ACME order for names and returns the certificate chain and
// private key in PEM form.
func (m *Manager) obtain(ctx context.Context, names []string, solver dnsSolver, lg Logger) (chainPEM, keyPEM []byte, leaf *x509.Certificate, err error) {
	c, err := m.acmeClient(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	order, err := c.AuthorizeOrder(ctx, acme.DomainIDs(names...))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("start the order: %w", err)
	}
	for _, u := range order.AuthzURLs {
		if err := m.authorize(ctx, c, u, solver, lg); err != nil {
			return nil, nil, nil, err
		}
	}
	if order, err = c.WaitOrder(ctx, order.URI); err != nil {
		return nil, nil, nil, fmt.Errorf("wait for the order: %w", err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}
	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{DNSNames: names}, key)
	if err != nil {
		return nil, nil, nil, err
	}
	der, _, err := c.CreateOrderCert(ctx, order.FinalizeURL, csr, true)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("finalize the order: %w", err)
	}
	if len(der) == 0 {
		return nil, nil, nil, errors.New("the CA returned no certificate")
	}
	if leaf, err = x509.ParseCertificate(der[0]); err != nil {
		return nil, nil, nil, err
	}
	var chain bytes.Buffer
	for _, b := range der {
		pem.Encode(&chain, &pem.Block{Type: "CERTIFICATE", Bytes: b})
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, nil, err
	}
	return chain.Bytes(), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), leaf, nil
}

func (m *Manager) authorize(ctx context.Context, c *acme.Client, url string, solver dnsSolver, lg Logger) error {
	z, err := c.GetAuthorization(ctx, url)
	if err != nil {
		return fmt.Errorf("get the authorization: %w", err)
	}
	name := z.Identifier.Value
	if z.Wildcard {
		name = "*." + name
	}
	if z.Status == acme.StatusValid {
		lg.Logf("%s was validated recently", name)
		return nil
	}
	if solver != nil {
		return m.authorizeDNS(ctx, c, z, name, solver, lg)
	}
	var chal *acme.Challenge
	for _, ch := range z.Challenges {
		if ch.Type == "http-01" {
			chal = ch
		}
	}
	if chal == nil {
		return fmt.Errorf("the CA offers no HTTP-01 challenge for %s", name)
	}
	if !challengeTokenRe.MatchString(chal.Token) {
		return fmt.Errorf("the CA sent an unexpected challenge token for %s", name)
	}
	body, err := c.HTTP01ChallengeResponse(chal.Token)
	if err != nil {
		return err
	}
	path := filepath.Join(m.paths.challengeDir(), chal.Token)
	if err := writeFile(path, []byte(body), 0o644); err != nil {
		return err
	}
	defer os.Remove(path)

	lg.Logf("Validating %s", name)
	if _, err := c.Accept(ctx, chal); err != nil {
		return fmt.Errorf("start the validation of %s: %w", name, err)
	}
	if _, err := c.WaitAuthorization(ctx, z.URI); err != nil {
		return fmt.Errorf("%s could not be validated: %w", name, err)
	}
	return nil
}

// authorizeDNS answers a DNS-01 challenge: a TXT record at
// _acme-challenge.<name> (the same record name for "*.example.com").
func (m *Manager) authorizeDNS(ctx context.Context, c *acme.Client, z *acme.Authorization, name string, solver dnsSolver, lg Logger) error {
	var chal *acme.Challenge
	for _, ch := range z.Challenges {
		if ch.Type == "dns-01" {
			chal = ch
		}
	}
	if chal == nil {
		return fmt.Errorf("the CA offers no DNS-01 challenge for %s", name)
	}
	value, err := c.DNS01ChallengeRecord(chal.Token)
	if err != nil {
		return err
	}
	fqdn := "_acme-challenge." + z.Identifier.Value
	lg.Logf("Adding the DNS record %s", fqdn)
	cleanup, err := solver.Present(ctx, fqdn, value, lg)
	if err != nil {
		return err
	}
	defer cleanup()
	lg.Logf("Validating %s", name)
	if _, err := c.Accept(ctx, chal); err != nil {
		return fmt.Errorf("start the validation of %s: %w", name, err)
	}
	if _, err := c.WaitAuthorization(ctx, z.URI); err != nil {
		return fmt.Errorf("%s could not be validated: %w", name, err)
	}
	return nil
}

// slogLogger sends job-style progress messages to the panel log, for
// renewals that run without a job.
type slogLogger struct{ m *Manager }

func (l slogLogger) Write(p []byte) (int, error) { return len(p), nil }
func (l slogLogger) Logf(format string, args ...any) {
	l.m.log.Info("certificate renewal: " + fmt.Sprintf(format, args...))
}
