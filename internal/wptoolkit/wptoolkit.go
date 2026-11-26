// Package wptoolkit manages WordPress sites with wp-cli: update checks and
// updates, maintenance and debug mode, automatic updates, administrator
// passwords and staging copies.
//
// wp-cli always runs as the site's own Linux user with the site's PHP
// version, like everything else on the site, so a site that was broken into
// cannot use it to reach other sites. Secrets never go on a command line:
// PHP code that carries a password is sent on stdin (wp eval-file -).
//
// What wp-cli prints is data from the site and is treated as untrusted: it
// is size-limited, parsed as JSON and checked before the panel shows it.
package wptoolkit

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/validate"
	"github.com/nefocp/nefo/internal/version"
)

const (
	// wp-cli comes from its latest GitHub release (see cli.go).
	wpcliMaxSize = 32 << 20

	// PharPath is where the panel keeps wp-cli. Site users read it, so it
	// and its folder are world-readable; only root can change them.
	PharPath = "/usr/local/lib/panel/wp-cli.phar"

	// maxOutput limits what the panel reads from one wp-cli command: a
	// must-use plugin (they load even with --skip-plugins) could print
	// without end.
	maxOutput = 4 << 20

	readTimeout   = 2 * time.Minute
	statusTimeout = 50 * time.Second // the panel's HTTP write timeout is 60 seconds
	updateTimeout = 20 * time.Minute
	keepBackups   = 3
	maxStatusRuns = 3 // status checks running at once; each starts several PHP processes
)

// Logger receives progress messages for a job log.
type Logger interface {
	io.Writer
	Logf(format string, args ...any)
}

// Toolkit runs wp-cli for the panel's WordPress sites.
type Toolkit struct {
	run  run.Runner
	root func(domain string) string // the site's folder; WordPress is in <root>/public

	// replaceable in tests
	phar      string
	latestURL string // the GitHub API's latest wp-cli release
	assetURL  string // the release downloads: <assetURL>v<version>/<file>
	lookupIDs func(name string) (uid, gid int, err error)
	now       func() time.Time

	installMu sync.Mutex
	installed bool

	sumMu  sync.Mutex
	sumKey string // the phar's size and time when sum was computed
	sum    string

	reads chan struct{}
}

// New returns a Toolkit. root returns a site's folder, as hosting.Root does.
func New(r run.Runner, root func(domain string) string) *Toolkit {
	return &Toolkit{
		run: r, root: root,
		phar: PharPath, latestURL: wpcliLatestURL, assetURL: wpcliAssetURL, lookupIDs: lookupIDs, now: time.Now,
		reads: make(chan struct{}, maxStatusRuns),
	}
}

// Installed reports whether wp-cli is on the server (it is installed on
// first use).
func (t *Toolkit) Installed() bool {
	_, err := os.Stat(t.phar)
	return err == nil
}

// Ensure installs the latest wp-cli release if wp-cli is missing or is not
// a file the panel installed (see CLIVersion).
func (t *Toolkit) Ensure(ctx context.Context) error {
	t.installMu.Lock()
	defer t.installMu.Unlock()
	if t.installed && t.Installed() {
		return nil
	}
	if t.CLIVersion() == "" {
		if err := t.installLatest(ctx, discardLog{}); err != nil {
			return err
		}
	}
	t.installed = true
	return nil
}

func fileSHA512(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha512.New()
	if _, err := io.Copy(h, io.LimitReader(f, wpcliMaxSize+1)); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// fetch downloads a file of at most max bytes over HTTPS.
func fetch(ctx context.Context, url string, max int64) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", version.AppName+"/"+version.Version)
	req.Header.Set("Accept", "application/vnd.github+json, application/octet-stream;q=0.9, */*;q=0.8")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("download %s: larger than expected", url)
	}
	return data, nil
}

func lookupIDs(name string) (int, int, error) {
	u, err := user.Lookup(name)
	if err != nil {
		return 0, 0, err
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return 0, 0, err
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return 0, 0, err
	}
	return uid, gid, nil
}

// checkSite refuses a site whose names could not safely appear in a
// command line or a path.
func checkSite(site *store.Site) error {
	if site == nil || validate.Domain(site.Domain) != nil || validate.SystemUsername(site.SysUser) != nil ||
		validate.PHPVersion(site.PHPVersion) != nil {
		return errors.New("invalid site")
	}
	return nil
}

func (t *Toolkit) public(site *store.Site) string {
	return filepath.Join(t.root(site.Domain), "public")
}
func (t *Toolkit) tmp(site *store.Site) string { return filepath.Join(t.root(site.Domain), "tmp") }

// command builds "runuser -u <user> -- sh -c 'umask … && exec "$@"' sh env …
// php<version> wp-cli.phar --path=<root>/public --skip-plugins … <args>".
// The shell only sets the umask; the command comes as separate arguments.
// Plugins and themes are not loaded, so a broken or hostile plugin cannot
// stop the panel from managing the site.
func (t *Toolkit) command(site *store.Site, umask string, args ...string) ([]string, error) {
	if err := checkSite(site); err != nil {
		return nil, err
	}
	tmp := t.tmp(site)
	full := []string{"-u", site.SysUser, "--", "sh", "-c", "umask " + umask + " && exec \"$@\"", "sh",
		"env", "HOME=" + tmp, "TMPDIR=" + tmp, "PATH=/usr/local/bin:/usr/bin:/bin",
		"WP_CLI_CACHE_DIR=" + filepath.Join(tmp, ".wp-cli", "cache"), "WP_CLI_DISABLE_AUTO_CHECK_UPDATE=1",
		"php" + site.PHPVersion, "-d", "memory_limit=512M", "-d", "sendmail_path=/bin/true",
		t.phar, "--path=" + t.public(site), "--skip-plugins", "--skip-themes", "--skip-packages",
	}
	return append(full, args...), nil
}

// wp runs a wp-cli command and returns its standard output. Standard error
// (warnings and PHP notices, which wp-cli sends there) only goes into the
// error message.
func (t *Toolkit) wp(ctx context.Context, site *store.Site, stdin []byte, args ...string) (string, error) {
	full, err := t.command(site, "027", args...)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, readTimeout)
	defer cancel()
	var out capped
	var in io.Reader
	if stdin != nil {
		in = bytes.NewReader(stdin)
	}
	err = t.run.Pipe(ctx, in, &out, "runuser", full...)
	if out.over {
		return "", fmt.Errorf("wp %s: more output than expected", strings.Join(args[:min(2, len(args))], " "))
	}
	if err != nil {
		return out.String(), cliErr(args, err)
	}
	return out.String(), nil
}

// stream runs a wp-cli command for a job, with its output in the job log.
func (t *Toolkit) stream(ctx context.Context, site *store.Site, lg Logger, umask string, args ...string) error {
	full, err := t.command(site, umask, args...)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, updateTimeout)
	defer cancel()
	if err := t.run.Stream(ctx, lg, "runuser", full...); err != nil {
		return cliErr(args, err)
	}
	return nil
}

// capped is a buffer that stops growing at maxOutput. The buffer is not
// embedded: its ReadFrom, which io.Copy prefers, would get around the limit.
type capped struct {
	buf  bytes.Buffer
	over bool
}

func (c *capped) Write(p []byte) (int, error) {
	if c.buf.Len()+len(p) > maxOutput {
		c.over = true
		return 0, errors.New("too much output")
	}
	return c.buf.Write(p)
}

func (c *capped) String() string { return c.buf.String() }

// CLIError is a failed wp-cli command with wp-cli's own explanation, such
// as "Error establishing a database connection.".
type CLIError struct {
	Command string
	Message string
	Err     error
}

func (e *CLIError) Error() string { return "wp " + e.Command + ": " + e.Message }
func (e *CLIError) Unwrap() error { return e.Err }

func cliErr(args []string, err error) error {
	out := ""
	var re *run.Error
	if errors.As(err, &re) {
		out = re.Output
	}
	cmd := strings.Join(args[:min(2, len(args))], " ")
	msg := lastErrorLine(out)
	if msg == "" {
		msg = strings.TrimSpace(run.Tail(out, 5))
	}
	if msg == "" {
		msg = err.Error()
	}
	return &CLIError{Command: cmd, Message: msg, Err: err}
}

// lastErrorLine returns the last "Error: …" line wp-cli printed, without
// the prefix and without HTML (WordPress database errors contain some).
func lastErrorLine(out string) string {
	lines := strings.Split(out, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if msg, ok := strings.CutPrefix(strings.TrimSpace(lines[i]), "Error: "); ok {
			return truncate(stripTags(msg), 500)
		}
	}
	return ""
}

func stripTags(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
		case r == '>' && in:
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.ToValidUTF8(s[:n], "")
}

// hasMessage reports whether err is a wp-cli error containing text.
func hasMessage(err error, text string) bool {
	var ce *CLIError
	return errors.As(err, &ce) && strings.Contains(ce.Message, text)
}
