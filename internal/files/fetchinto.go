package files

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
)

// CheckURL validates a download address before a job starts: an http or
// https URL whose host resolves to public addresses only (as for "Download
// from URL"). The dialer checks every address it connects to again.
func (m *Manager) CheckURL(ctx context.Context, raw string) (*url.URL, error) {
	u, err := parseFetchURL(raw)
	if err != nil {
		return nil, err
	}
	if err := checkFetchHost(ctx, u, m.checkAddr); err != nil {
		return nil, err
	}
	return u, nil
}

// FetchInto downloads u into dir, a folder of the panel's own (not a
// website's), with the same address checks as Fetch, and at most limit
// bytes. The file belongs to root with mode 0640; its name comes from the
// server or the URL and never replaces an existing file. It returns the
// new file's path.
func (m *Manager) FetchInto(ctx context.Context, dir string, u *url.URL, limit int64, lg Logger) (string, error) {
	if runtime.GOOS != "linux" {
		return "", ErrUnsupported
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return "", err
	}
	defer root.Close()
	client, tr := newFetchClient(m.checkAddr)
	defer tr.CloseIdleConnections()
	rel, n, err := fetchTo(ctx, client, u, root, ".", -1, -1, limit, lg)
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, rel)
	if lg != nil {
		lg.Logf("Saved %s (%s)", p, humanBytes(n))
	}
	return p, nil
}
