// Package config loads and saves the panel's configuration file.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/secure"
)

// DefaultPath is where the installer writes the configuration file.
const DefaultPath = "/etc/nefo/config.json"

// TLS configures HTTPS for the panel itself.
type TLS struct {
	Enabled bool `json:"enabled"`
	// Cert and Key are optional. When empty, a self-signed certificate is
	// created in the data directory on first start.
	Cert string `json:"cert,omitempty"`
	Key  string `json:"key,omitempty"`
}

// Config is the content of config.json.
type Config struct {
	// Listen is the address the panel listens on, such as ":28443".
	Listen string `json:"listen"`
	// Entrance is a secret first path segment. Every URL outside
	// /<entrance>/ answers 404, so scanners cannot even find the login page.
	Entrance string `json:"entrance"`
	// DataDir holds the database and generated certificates.
	DataDir string `json:"data_dir"`
	TLS     TLS    `json:"tls"`
	// ACMEDirectory is the certificate authority for website certificates.
	// Empty means Let's Encrypt. Set it to LetsEncryptStaging while testing,
	// so failed attempts do not count against the real rate limits.
	ACMEDirectory string `json:"acme_directory,omitempty"`
	// UpdateURL is where updates are downloaded from. Empty means the
	// project's GitHub releases.
	UpdateURL string `json:"update_url,omitempty"`
	// Dev relaxes some checks for local development. Never use it on a server.
	Dev bool `json:"dev,omitempty"`
}

// ACME directories of Let's Encrypt.
const (
	LetsEncrypt        = "https://acme-v02.api.letsencrypt.org/directory"
	LetsEncryptStaging = "https://acme-staging-v02.api.letsencrypt.org/directory"
)

// ACMEDirectoryURL returns the ACME directory to use.
func (c *Config) ACMEDirectoryURL() string {
	if c.ACMEDirectory == "" {
		return LetsEncrypt
	}
	return c.ACMEDirectory
}

var entranceRe = regexp.MustCompile(`^[a-z0-9]{3,32}$`)

// Validate checks that the configuration is usable.
func (c *Config) Validate() error {
	if _, _, err := net.SplitHostPort(c.Listen); err != nil {
		return fmt.Errorf("listen %q: %w", c.Listen, err)
	}
	if !entranceRe.MatchString(c.Entrance) {
		return errors.New("entrance must be 3-32 lowercase letters or digits")
	}
	if !c.Dev && len(c.Entrance) < 8 {
		return errors.New("entrance must be at least 8 characters outside dev mode")
	}
	if c.DataDir == "" {
		return errors.New("data_dir is required")
	}
	if c.ACMEDirectory != "" {
		if u, err := url.Parse(c.ACMEDirectory); err != nil || u.Scheme != "https" || u.Host == "" {
			return errors.New("acme_directory must be an https:// URL")
		}
	}
	return nil
}

// Port returns the port number from Listen.
func (c *Config) Port() int {
	_, p, _ := net.SplitHostPort(c.Listen)
	n, _ := strconv.Atoi(p)
	return n
}

// Load reads and validates the configuration file at path.
func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if c.DataDir != "" && !filepath.IsAbs(c.DataDir) {
		c.DataDir = filepath.Join(filepath.Dir(path), c.DataDir)
	}
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &c, nil
}

// Save validates c and writes it to path, readable by the owner only.
func Save(path string, c *Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, append(b, '\n'), 0o600)
}

// RandomEntrance returns a new random entrance path segment.
func RandomEntrance() string {
	return secure.String(12, secure.LowerAlnum)
}

// RandomPort returns a random port in [20000, 50000). A non-standard port
// keeps the panel out of most automated scans.
func RandomPort() int {
	return 20000 + secure.Intn(30000)
}
