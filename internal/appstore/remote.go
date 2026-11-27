package appstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/version"
)

// A newer catalog can be published next to the releases as
// appstore-catalog.json with appstore-catalog.json.sig, an Ed25519
// signature over the file's exact bytes made with the release key (the
// same "openssl pkeyutl -sign -rawin" as checksums.txt). The panel fetches
// it once a day and accepts it only when the signature is valid, the format
// is one it reads, and its catalog_version is higher than both the embedded
// catalog's and the last one accepted (so an old signed file cannot be
// replayed). Each entry must pass the same checks as the embedded file;
// one that does not (a recipe feature of a newer panel) is left out, and
// the embedded entry of the same app is kept (see ParseRemote).
//
// Builds without a release key never fetch anything.

const (
	remoteName   = "appstore-catalog.json"
	maxRemote    = 1 << 20
	acceptedFile = "accepted_version"
)

// ErrRemoteDisabled is returned when this build cannot check signatures.
var ErrRemoteDisabled = errors.New("the remote catalog is off: this build has no release signing key")

// Remote fetches and verifies the signed catalog.
type Remote struct {
	Base     string                       // release download base (update_url)
	CacheDir string                       // /var/lib/nefo/appstore
	Verify   func(data, sig []byte) error // nil turns the remote catalog off
	Client   *http.Client
}

// Enabled reports whether a remote catalog can be used at all.
func (r *Remote) Enabled() bool { return r != nil && r.Verify != nil && r.Base != "" }

// LoadCache puts a previously fetched catalog in use at start-up, after
// checking it again.
func (r *Remote) LoadCache() error {
	if !r.Enabled() {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(r.CacheDir, remoteName))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	sig, err := os.ReadFile(filepath.Join(r.CacheDir, remoteName+".sig"))
	if err != nil {
		return err
	}
	c, sk, err := r.check(data, sig, true)
	if err != nil {
		return err
	}
	setCurrent(c, "remote", sk)
	return nil
}

// Refresh downloads the remote catalog and uses it if it is newer. It
// returns the catalog in use afterwards.
func (r *Remote) Refresh(ctx context.Context) (*Catalog, error) {
	if !r.Enabled() {
		return Current(), ErrRemoteDisabled
	}
	base := strings.TrimRight(r.Base, "/")
	data, err := r.get(ctx, base+"/"+remoteName)
	if err != nil {
		return Current(), err
	}
	sig, err := r.get(ctx, base+"/"+remoteName+".sig")
	if err != nil {
		return Current(), err
	}
	c, sk, err := r.check(data, sig, false)
	if err != nil {
		return Current(), err
	}
	if err := os.MkdirAll(r.CacheDir, 0o700); err != nil {
		return Current(), err
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(r.CacheDir, remoteName), data, 0o600); err != nil {
		return Current(), err
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(r.CacheDir, remoteName+".sig"), sig, 0o600); err != nil {
		return Current(), err
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(r.CacheDir, acceptedFile), []byte(strconv.FormatInt(c.CatalogVersion, 10)), 0o600); err != nil {
		return Current(), err
	}
	setCurrent(c, "remote", sk)
	return c, nil
}

// check verifies a signed catalog. cached allows the version that was
// last accepted (the file we wrote ourselves); a download must be newer.
func (r *Remote) check(data, sig []byte, cached bool) (*Catalog, []Skipped, error) {
	if err := r.Verify(data, sig); err != nil {
		return nil, nil, fmt.Errorf("remote catalog: %w", err)
	}
	c, sk, err := ParseRemote(data, version.Version)
	if err != nil {
		return nil, nil, fmt.Errorf("remote catalog: %w", err)
	}
	if c.MinPanel != "" && version.Version != "dev" && !versionAtLeast(version.Version, c.MinPanel) {
		return nil, nil, fmt.Errorf("remote catalog needs panel %s or newer", c.MinPanel)
	}
	if c.CatalogVersion <= Embedded().CatalogVersion {
		return nil, nil, errNotNewer
	}
	last := r.accepted()
	if c.CatalogVersion < last || (!cached && c.CatalogVersion == last) {
		return nil, nil, errNotNewer
	}
	return c, sk, nil
}

var errNotNewer = errors.New("the remote catalog is not newer than the one in use")

// IsNotNewer reports whether Refresh found nothing newer.
func IsNotNewer(err error) bool { return errors.Is(err, errNotNewer) }

func (r *Remote) accepted() int64 {
	b, err := os.ReadFile(filepath.Join(r.CacheDir, acceptedFile))
	if err != nil {
		return 0
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	return n
}

func (r *Remote) get(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", version.AppName+"/"+version.Version)
	client := r.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxRemote+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxRemote {
		return nil, fmt.Errorf("%s is larger than 1 MB", url)
	}
	return b, nil
}
