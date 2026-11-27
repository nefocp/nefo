package files

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/secure"
	"github.com/nefocp/nefo/internal/usererr"
)

const (
	// MaxFetchSize is the largest file "Download from URL" saves.
	MaxFetchSize = 2 << 30
	maxRedirects = 5
	maxURLLength = 2048
	maxNameBytes = 200 // leaves room for " (12)" and the temporary name
	fetchLogStep = 3 * time.Second
)

// blockedPrefixes are address ranges a download must never reach, besides
// those netip already names (loopback, private, link-local, multicast and
// unspecified addresses).
var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),      // "this network"
	netip.MustParsePrefix("100.64.0.0/10"),  // carrier-grade NAT
	netip.MustParsePrefix("192.0.0.0/24"),   // IETF protocol assignments
	netip.MustParsePrefix("198.18.0.0/15"),  // benchmarking, often used inside networks
	netip.MustParsePrefix("240.0.0.0/4"),    // reserved, and the broadcast address
	netip.MustParsePrefix("::/96"),          // IPv4-compatible addresses
	netip.MustParsePrefix("64:ff9b:1::/48"), // local-use NAT64
	netip.MustParsePrefix("fec0::/10"),      // old site-local addresses
}

// nat64 holds IPv4 addresses inside IPv6 ones; the IPv4 address is checked.
var nat64 = netip.MustParsePrefix("64:ff9b::/96")

// internalAddr reports whether a is an address of the server's own
// networks: loopback, private, link-local, carrier-grade NAT, unspecified
// and similar ranges.
func internalAddr(a netip.Addr) bool {
	a = a.Unmap()
	if !a.IsValid() || a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() || a.IsLinkLocalMulticast() ||
		a.IsInterfaceLocalMulticast() || a.IsMulticast() || a.IsUnspecified() {
		return true
	}
	for _, p := range blockedPrefixes {
		if p.Contains(a) {
			return true
		}
	}
	if nat64.Contains(a) {
		b := a.As16()
		return internalAddr(netip.AddrFrom4([4]byte(b[12:])))
	}
	return false
}

// publicAddr allows connections to public addresses only. It also refuses
// the addresses of the server's own network interfaces, which reach the
// services listening on them.
func publicAddr(ap netip.AddrPort) error {
	a := ap.Addr().Unmap()
	if internalAddr(a) || localAddr(a) {
		return fmt.Errorf("%s is an internal address: the panel does not download from the server's own network", a)
	}
	return nil
}

func localAddr(a netip.Addr) bool {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, ia := range addrs {
		if n, ok := ia.(*net.IPNet); ok {
			if ip, ok := netip.AddrFromSlice(n.IP); ok && ip.Unmap() == a {
				return true
			}
		}
	}
	return false
}

// newFetchClient returns an HTTP client whose every connection, including
// those for redirects, goes to an address that check allows. The check runs
// on the address actually dialled, after DNS resolution, so a name that
// resolves to an internal address (or changes its answer) is refused too.
func newFetchClient(check func(netip.AddrPort) error) (*http.Client, *http.Transport) {
	d := &net.Dialer{
		Timeout:   20 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			ap, err := netip.ParseAddrPort(address)
			if err != nil {
				return fmt.Errorf("unexpected address %q", address)
			}
			return check(ap)
		},
	}
	tr := &http.Transport{
		Proxy:                 nil, // a proxy would make the address check meaningless
		DialContext:           d.DialContext,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   20 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		// Keep the bytes exactly as served: a .gz file sent with
		// "Content-Encoding: gzip" must stay compressed.
		DisableCompression: true,
		MaxIdleConns:       1,
		IdleConnTimeout:    10 * time.Second,
	}
	c := &http.Client{
		Transport: tr,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("refused a redirect to a %s: address", req.URL.Scheme)
			}
			return nil
		},
	}
	return c, tr
}

// parseFetchURL checks that raw is an http or https URL with a host.
func parseFetchURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > maxURLLength {
		return nil, usererr.New("invalid_url", truncateText(raw, 100))
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.Opaque != "" {
		return nil, usererr.New("invalid_url", truncateText(raw, 100))
	}
	return u, nil
}

// checkFetchHost resolves the URL's host and refuses it if any of its
// addresses is not allowed. It only gives a clear error early: the dialer
// checks the address it connects to again.
func checkFetchHost(ctx context.Context, u *url.URL, check func(netip.AddrPort) error) error {
	host := u.Hostname()
	port := 80
	if u.Scheme == "https" {
		port = 443
	}
	if p := u.Port(); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			return usererr.New("invalid_url", u.Redacted())
		}
		port = n
	}
	var addrs []netip.Addr
	if a, err := netip.ParseAddr(host); err == nil {
		addrs = []netip.Addr{a}
	} else {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		addrs, err = net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil || len(addrs) == 0 {
			return usererr.New("url_host_unknown", host)
		}
	}
	for _, a := range addrs {
		if check(netip.AddrPortFrom(a.Unmap(), uint16(port))) != nil {
			return usererr.New("url_not_allowed", host)
		}
	}
	return nil
}

// CheckFetch validates a "Download from URL" request before it starts: the
// URL must be http or https and its host must resolve to public addresses,
// and dir must be a folder.
func (m *Manager) CheckFetch(ctx context.Context, dir, raw string) (*url.URL, error) {
	u, err := parseFetchURL(raw)
	if err != nil {
		return nil, err
	}
	t, err := m.open(ctx, dir)
	if err != nil {
		return nil, err
	}
	defer t.Close()
	if err := t.writable(true); err != nil {
		return nil, err
	}
	fi, err := t.root.Stat(t.rel)
	if err != nil {
		return nil, mapErr(err, t.abs)
	}
	if !fi.IsDir() {
		return nil, usererr.New("not_a_folder", t.abs)
	}
	if err := checkFetchHost(ctx, u, m.checkAddr); err != nil {
		return nil, err
	}
	return u, nil
}

// Fetch downloads u into folder dir and returns the new file's path. The
// name comes from the server (Content-Disposition) or the URL, an existing
// file is never replaced (" (1)" is added instead), and the file gets the
// folder's owner and group with mode 0640.
func (m *Manager) Fetch(ctx context.Context, dir string, u *url.URL, lg Logger) (string, error) {
	t, err := m.open(ctx, dir)
	if err != nil {
		return "", err
	}
	defer t.Close()
	if err := t.writable(true); err != nil {
		return "", err
	}
	fi, err := t.root.Stat(t.rel)
	if err != nil {
		return "", mapErr(err, t.abs)
	}
	if !fi.IsDir() {
		return "", usererr.New("not_a_folder", t.abs)
	}
	uid, gid := fsutil.Owner(fi)
	client, tr := newFetchClient(m.checkAddr)
	defer tr.CloseIdleConnections()
	rel, n, err := fetchTo(ctx, client, u, t.root, t.rel, uid, gid, MaxFetchSize, lg)
	if err != nil {
		return "", err
	}
	abs := path.Join(t.scope.Root, rel)
	if lg != nil {
		lg.Logf("Saved %s (%s)", abs, humanBytes(n))
	}
	return abs, nil
}

type discard struct{}

func (discard) Logf(string, ...any) {}

// fetchTo downloads u into folder dir of root, at most limit bytes, and
// returns the file's path relative to root and its size. The data goes to
// a hidden temporary file first, which is removed if anything fails.
func fetchTo(ctx context.Context, client *http.Client, u *url.URL, root *os.Root, dir string, uid, gid int, limit int64, lg Logger) (string, int64, error) {
	if lg == nil {
		lg = discard{}
	}
	lg.Logf("Downloading %s", u.Redacted())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", 0, usererr.New("invalid_url", u.Redacted())
	}
	req.Header.Set("User-Agent", "nefo-file-manager")
	req.Header.Set("Accept", "*/*")
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fetchError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("the server answered %s", resp.Status)
	}
	if final := resp.Request.URL; final.String() != u.String() {
		lg.Logf("Redirected to %s", final.Redacted())
	}
	if resp.ContentLength > limit {
		return "", 0, fmt.Errorf("the file is %s, more than the %s limit", humanBytes(resp.ContentLength), humanBytes(limit))
	}
	name := downloadName(resp.Header.Get("Content-Disposition"), u, resp.Request.URL)

	tmp := path.Join(dir, ".panel-download-"+secure.String(12, secure.LowerAlnum)+".part")
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", 0, mapErr(err, tmp)
	}
	done := false
	defer func() {
		if !done {
			f.Close()
			root.Remove(tmp)
		}
	}()
	n, err := copyWithProgress(ctx, f, io.LimitReader(resp.Body, limit+1), resp.ContentLength, lg)
	if err != nil {
		if ctx.Err() != nil {
			return "", n, fmt.Errorf("download stopped: %w", ctx.Err())
		}
		return "", n, fmt.Errorf("download interrupted after %s: %w", humanBytes(n), err)
	}
	if n > limit {
		return "", n, fmt.Errorf("the file is larger than the %s limit", humanBytes(limit))
	}
	if uid >= 0 {
		if err := f.Chown(uid, gid); err != nil {
			return "", n, err
		}
	}
	if err := f.Chmod(0o640); err != nil {
		return "", n, err
	}
	if err := f.Close(); err != nil {
		root.Remove(tmp)
		done = true
		return "", n, err
	}
	done = true
	final, err := linkUnique(root, dir, name, tmp)
	if err != nil {
		root.Remove(tmp)
		return "", n, err
	}
	return final, n, nil
}

// fetchError turns a client error into a plain sentence; the URL is left
// out because it may hold a password.
func fetchError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	return fmt.Errorf("download failed: %w", err)
}

func copyWithProgress(ctx context.Context, w io.Writer, r io.Reader, total int64, lg Logger) (int64, error) {
	buf := make([]byte, 256<<10)
	var n int64
	next := time.Now().Add(fetchLogStep)
	for {
		if err := ctx.Err(); err != nil {
			return n, err
		}
		k, rerr := r.Read(buf)
		if k > 0 {
			if _, err := w.Write(buf[:k]); err != nil {
				return n, err
			}
			n += int64(k)
		}
		if time.Now().After(next) {
			next = time.Now().Add(fetchLogStep)
			if total > 0 {
				lg.Logf("%s of %s (%d%%)", humanBytes(n), humanBytes(total), n*100/total)
			} else {
				lg.Logf("%s", humanBytes(n))
			}
		}
		if rerr == io.EOF {
			return n, nil
		}
		if rerr != nil {
			return n, rerr
		}
	}
}

// linkUnique gives the temporary file tmp its final name in dir: name, or
// "name (1).ext" and so on when it is taken. A hard link fails instead of
// replacing a file that appeared in the meantime.
func linkUnique(root *os.Root, dir, name, tmp string) (string, error) {
	for i := 0; i < 1000; i++ {
		p := path.Join(dir, numberedName(name, i))
		err := root.Link(tmp, p)
		if err == nil {
			root.Remove(tmp)
			return p, nil
		}
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		// No hard links on this file system: check, then rename.
		if _, lerr := root.Lstat(p); lerr == nil {
			continue
		}
		if err := root.Rename(tmp, p); err != nil {
			return "", mapErr(err, p)
		}
		return p, nil
	}
	return "", usererr.New("path_exists", path.Join(dir, name))
}

// downloadName picks the file name from a Content-Disposition header, else
// from the URLs' paths (the one asked for first, then where it led).
func downloadName(disposition string, urls ...*url.URL) string {
	if disposition != "" {
		if _, params, err := mime.ParseMediaType(disposition); err == nil {
			if n := sanitizeName(params["filename"]); n != "" {
				return n
			}
		}
	}
	for _, u := range urls {
		if u != nil {
			if n := sanitizeName(path.Base(u.Path)); n != "" {
				return n
			}
		}
	}
	return "download"
}

// sanitizeName turns a name chosen by a remote server into a plain base
// name: no folders, no control or invisible characters, not hidden (no
// leading dot, so it cannot become .htaccess or .user.ini), and at most
// 200 bytes. It returns "" when nothing usable is left.
func sanitizeName(s string) string {
	s = strings.ReplaceAll(s, "\\", "/")
	s = strings.TrimRight(s, "/")
	if i := strings.LastIndexByte(s, '/'); i >= 0 {
		s = s[i+1:]
	}
	s = strings.ToValidUTF8(s, "_")
	s = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		if unicode.Is(unicode.Cc, r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, s)
	s = strings.TrimLeft(strings.TrimSpace(s), ". ")
	s = strings.TrimRight(s, ". ")
	if len(s) > maxNameBytes {
		stem, ext := splitExt(s)
		if len(ext) > 20 {
			stem, ext = s, ""
		}
		stem = cutBytes(stem, maxNameBytes-len(ext))
		s = strings.TrimRight(stem, ". ") + ext
	}
	if !ValidName(s) {
		return ""
	}
	return s
}

// splitExt splits a name into stem and extension, keeping double
// extensions such as .tar.gz together.
func splitExt(name string) (string, string) {
	lower := strings.ToLower(name)
	for _, d := range []string{".tar.gz", ".tar.bz2", ".tar.xz", ".tar.zst"} {
		if strings.HasSuffix(lower, d) && len(name) > len(d) {
			return name[:len(name)-len(d)], name[len(name)-len(d):]
		}
	}
	ext := path.Ext(name)
	if ext == name {
		return name, ""
	}
	return name[:len(name)-len(ext)], ext
}

// numberedName returns name for i = 0, else "stem (i).ext".
func numberedName(name string, i int) string {
	if i == 0 {
		return name
	}
	stem, ext := splitExt(name)
	return fmt.Sprintf("%s (%d)%s", stem, i, ext)
}

// cutBytes shortens s to at most n bytes without splitting a character.
func cutBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !isRuneStart(s[n]) {
		n--
	}
	return s[:n]
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }

func truncateText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return cutBytes(s, n) + "…"
}

// humanBytes formats a size like the UI does: 1.5 MB.
func humanBytes(n int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	v := float64(n)
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if i == 0 || v >= 100 {
		return fmt.Sprintf("%.0f %s", v, units[i])
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}
