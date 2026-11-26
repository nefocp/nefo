package hosting

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
)

// Settings are the changeable settings of a site.
type Settings struct {
	Aliases    []string
	PHPVersion string
	PHPMemory  int // MB
	PHPUpload  int // MB
	PHPTime    int // seconds
	ProxyPort  int
	AppCommand string
	// AppRuntime: "" (default node), "node:<major>" or "python".
	AppRuntime string
	// PHP-FPM workers and pool mode, and the normally disabled functions
	// the site may use (from HardenedFunctions).
	PHPWorkers int
	PHPPM      string
	PHPAllow   []string
}

// UpdateSettings changes a site's aliases, PHP version and PHP limits. If
// the web server or PHP-FPM rejects the result, the old configuration is put back.
// reissue reports that the site's certificate does not cover the new names.
func (m *Manager) UpdateSettings(ctx context.Context, id int64, s Settings) (site *store.Site, reissue bool, err error) {
	if !m.Supported() {
		return nil, false, ErrUnsupported
	}
	if err := m.tryLock(); err != nil {
		return nil, false, err
	}
	defer m.ops.Unlock()

	site, err = m.store.SiteByID(ctx, id)
	if err != nil {
		return nil, false, err
	}
	old := *site
	old.Aliases = slices.Clone(site.Aliases)

	_, aliases, err := m.checkNames(ctx, site.Domain, s.Aliases, site.ID)
	if err != nil {
		return nil, false, err
	}
	switch {
	case site.PHPVersion == "" && s.PHPVersion != "", site.PHPVersion != "" && s.PHPVersion == "":
		return nil, false, usererr.New("cannot_change_type")
	case s.PHPVersion != "" && (validate.PHPVersion(s.PHPVersion) != nil || !m.phpInstalled(s.PHPVersion)):
		return nil, false, usererr.New("php_not_installed", s.PHPVersion)
	}
	s.AppCommand = strings.TrimSpace(s.AppCommand)
	if (site.ProxyPort != 0) != (s.ProxyPort != 0) {
		return nil, false, usererr.New("cannot_change_type")
	}
	if s.ProxyPort != 0 {
		if err := m.checkApp(ctx, s.ProxyPort, s.AppCommand, site.ID); err != nil {
			return nil, false, err
		}
		if err := checkRuntime(s.AppRuntime); err != nil {
			return nil, false, err
		}
		site.AppRuntime = s.AppRuntime
	} else if s.AppCommand != "" {
		return nil, false, usererr.New("invalid_app")
	}
	if s.PHPMemory < 32 || s.PHPMemory > 4096 || s.PHPUpload < 1 || s.PHPUpload > 4096 || s.PHPTime < 10 || s.PHPTime > 3600 {
		return nil, false, usererr.New("invalid_php_limits")
	}
	site.Aliases, site.PHPVersion = aliases, s.PHPVersion
	site.PHPMemory, site.PHPUpload, site.PHPTime = s.PHPMemory, s.PHPUpload, s.PHPTime
	if s.PHPWorkers != 0 || s.PHPPM != "" {
		if s.PHPWorkers < 1 || s.PHPWorkers > 200 || (s.PHPPM != "ondemand" && s.PHPPM != "dynamic") {
			return nil, false, usererr.New("invalid_php_workers")
		}
		site.PHPWorkers, site.PHPPM = s.PHPWorkers, s.PHPPM
	}
	if s.PHPAllow != nil {
		var allow []string
		for _, f := range s.PHPAllow {
			if !slices.Contains(HardenedFunctions, f) {
				return nil, false, usererr.New("invalid_php_function", f)
			}
			if !slices.Contains(allow, f) {
				allow = append(allow, f)
			}
		}
		site.PHPAllow = strings.Join(allow, " ")
	}
	site.ProxyPort, site.AppCommand = s.ProxyPort, s.AppCommand

	bg := context.WithoutCancel(ctx)
	restore := func() {
		if old.PHPVersion != "" {
			if site.PHPVersion != old.PHPVersion {
				removeIfExists(m.paths.poolFile(site.PHPVersion, site.SysUser))
				m.reloadPHP(bg, site.PHPVersion)
			}
			if m.writePool(&old) == nil {
				m.reloadPHP(bg, old.PHPVersion)
			}
		}
		if m.writeVhost(bg, &old) == nil {
			m.reloadWeb(bg)
		}
	}

	if site.PHPVersion != "" {
		// Both versions would use the same socket: take the old pool down first.
		if site.PHPVersion != old.PHPVersion {
			removeIfExists(m.paths.poolFile(old.PHPVersion, site.SysUser))
			if err := m.reloadPHP(ctx, old.PHPVersion); err != nil {
				restore()
				return nil, false, err
			}
			waitGone(m.paths.phpSocket(site.SysUser), 5*time.Second)
		}
		if err := m.writePool(site); err != nil {
			restore()
			return nil, false, err
		}
		if err := m.reloadPHP(ctx, site.PHPVersion); err != nil {
			restore()
			return nil, false, err
		}
		waitForFile(m.paths.phpSocket(site.SysUser), 5*time.Second)
	}
	if err := m.writeVhost(ctx, site); err != nil {
		restore()
		return nil, false, err
	}
	if err := m.reloadWeb(ctx); err != nil {
		restore()
		return nil, false, err
	}
	if site.ProxyPort != 0 && (site.ProxyPort != old.ProxyPort || site.AppCommand != old.AppCommand || site.AppRuntime != old.AppRuntime) {
		if err := m.writeApp(ctx, site); err != nil {
			restore()
			m.writeApp(bg, &old)
			return nil, false, err
		}
	}
	if err := m.store.UpdateSite(ctx, site); err != nil {
		restore()
		return nil, false, err
	}
	reissue = site.SSL && !slices.Equal(sorted(old.Aliases), sorted(site.Aliases))
	return site, reissue, nil
}

func sorted(s []string) []string {
	c := slices.Clone(s)
	slices.Sort(c)
	return c
}

func waitGone(path string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) && fileExists(path) {
		time.Sleep(25 * time.Millisecond)
	}
}

// Log kinds for SiteLog.
var logKinds = map[string]bool{"access": true, "error": true, "php": true}

// SiteLog returns the last lines of a site's access or error log, or
// of its PHP error log.
func (m *Manager) SiteLog(ctx context.Context, id int64, kind string, lines int) (string, error) {
	if !logKinds[kind] {
		return "", usererr.New("invalid_log")
	}
	if lines < 1 || lines > 2000 {
		lines = 200
	}
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return "", err
	}
	if validate.Domain(site.Domain) != nil {
		return "", errors.New("invalid site")
	}
	var f *os.File
	switch kind {
	case "access", "error":
		f, err = os.Open(filepath.Join(m.web().logDir(), site.Domain+"."+kind+".log"))
	case "php":
		// The logs folder belongs to the site: read it inside an os.Root so a
		// planted symlink cannot point the panel elsewhere.
		var root *os.Root
		if root, err = os.OpenRoot(m.paths.siteRoot(site.Domain)); err == nil {
			defer root.Close()
			f, err = root.Open("logs/php-error.log")
		}
	}
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	defer f.Close()
	return tail(f, lines)
}

// tail returns the last n lines of f, reading at most 1 MB from its end.
func tail(f *os.File, n int) (string, error) {
	fi, err := f.Stat()
	if err != nil {
		return "", err
	}
	size := fi.Size()
	start := size - 1<<20
	if start < 0 {
		start = 0
	}
	buf := make([]byte, size-start)
	if _, err := f.ReadAt(buf, start); err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	buf = bytes.TrimRight(buf, "\n")
	lines := bytes.Split(buf, []byte("\n"))
	if start > 0 && len(lines) > 1 {
		lines = lines[1:] // the first line is probably cut
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return string(bytes.Join(lines, []byte("\n"))), nil
}
