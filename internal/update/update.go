// Package update replaces the panel binary with the latest release.
//
// A release publishes VERSION, checksums.txt and nefo-linux-<arch>. The
// downloaded binary must match its SHA-256 in checksums.txt, and is moved
// into place atomically before the service restarts.
package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/version"
)

// DefaultBase is where releases are downloaded from.
var DefaultBase = "https://github.com/" + version.Repo + "/releases/latest/download"

// Logger receives progress messages.
type Logger interface {
	Logf(format string, args ...any)
}

// Updater checks for and installs releases.
type Updater struct {
	base   string
	binary string
	run    run.Runner
	client *http.Client
}

// New returns an Updater. base must be https (http is allowed for localhost,
// for testing). binary is the installed panel executable.
func New(base, binary string, r run.Runner) (*Updater, error) {
	if base == "" {
		base = DefaultBase
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost"))) {
		return nil, fmt.Errorf("update_url must be an https:// address")
	}
	return &Updater{base: strings.TrimRight(base, "/"), binary: binary, run: r, client: &http.Client{Timeout: 10 * time.Minute}}, nil
}

var versionRe = regexp.MustCompile(`^v?[0-9A-Za-z.+-]{1,40}$`)

func (u *Updater) get(ctx context.Context, name string, max int64, w io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.base+"/"+name, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", version.AppName+"/"+version.Version)
	resp, err := u.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", name, resp.StatusCode)
	}
	n, err := io.Copy(w, io.LimitReader(resp.Body, max+1))
	if err == nil && n > max {
		err = fmt.Errorf("download %s: larger than expected", name)
	}
	return err
}

// Latest returns the version of the latest release.
func (u *Updater) Latest(ctx context.Context) (string, error) {
	var b strings.Builder
	if err := u.get(ctx, "VERSION", 100, &b); err != nil {
		return "", err
	}
	v := strings.TrimSpace(b.String())
	if !versionRe.MatchString(v) {
		return "", errors.New("the release has an invalid VERSION file")
	}
	return v, nil
}

// Apply downloads the latest binary, checks it and replaces the installed
// one, then asks systemd to restart the panel.
func (u *Updater) Apply(ctx context.Context, lg Logger) (string, error) {
	latest, err := u.Latest(ctx)
	if err != nil {
		return "", err
	}
	asset := "nefo-linux-" + runtime.GOARCH
	var sums strings.Builder
	if err := u.get(ctx, "checksums.txt", 64<<10, &sums); err != nil {
		return "", err
	}
	pub, err := signingKey()
	if err != nil {
		return "", err
	}
	if pub != nil {
		var sig bytes.Buffer
		if err := u.get(ctx, "checksums.txt.sig", 1024, &sig); err != nil {
			return "", fmt.Errorf("the release has no signature: %w", err)
		}
		if err := checkSignature(pub, []byte(sums.String()), sig.Bytes()); err != nil {
			return "", err
		}
		lg.Logf("Release signature verified")
	} else {
		lg.Logf("Development build: the release signature is not checked")
	}
	want := ""
	for _, line := range strings.Split(sums.String(), "\n") {
		if f := strings.Fields(line); len(f) == 2 && strings.TrimPrefix(f[1], "*") == asset {
			want = f[0]
		}
	}
	if len(want) != 64 {
		return "", fmt.Errorf("checksums.txt has no entry for %s", asset)
	}

	lg.Logf("Downloading %s %s", asset, latest)
	dir := filepath.Dir(u.binary)
	tmp, err := os.CreateTemp(dir, ".nefo-update-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	h := sha256.New()
	err = u.get(ctx, asset, 200<<20, io.MultiWriter(tmp, h))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", err
	}
	if hex.EncodeToString(h.Sum(nil)) != want {
		return "", errors.New("the download does not match checksums.txt: not installed")
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return "", err
	}
	// Make sure the new binary runs on this machine before switching.
	out, err := u.run.Output(ctx, tmp.Name(), "version")
	if err != nil {
		return "", fmt.Errorf("the new binary does not run here: %v", err)
	}
	lg.Logf("Checked: %s", strings.TrimSpace(out))
	if err := os.Rename(tmp.Name(), u.binary); err != nil {
		return "", err
	}
	lg.Logf("Installed. The panel restarts now; reload the page in a few seconds.")
	_, err = u.run.Output(ctx, "systemctl", "--no-block", "restart", "nefo.service")
	return latest, err
}
