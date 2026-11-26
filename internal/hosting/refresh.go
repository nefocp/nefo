package hosting

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Refresh rewrites every configuration file the panel manages (the web
// server's shared files, each site's vhost and PHP-FPM pool) from the
// current templates, so a panel upgrade brings existing sites up to date.
// Files that are already current are left alone. If the web server rejects
// the result, all old files are put back. It runs when the panel starts.
func (m *Manager) Refresh(ctx context.Context) (changed int, err error) {
	ws := m.web()
	if !m.Supported() || !fileExists(ws.hookFile()) {
		return 0, nil
	}
	m.ops.Lock()
	defer m.ops.Unlock()
	if err := ws.prepare(ctx); err != nil {
		return 0, err
	}

	// Website accounts may not sign in over SSH (see sftp.go).
	if err := m.ensureSSHD(ctx); err != nil {
		m.log.Warn("ssh settings for websites", "err", err)
	}

	type saved struct {
		data    []byte
		existed bool
		mode    os.FileMode
	}
	backup := map[string]saved{}
	phpChanged := map[string]bool{}
	writeMode := func(path string, data []byte, mode os.FileMode) error {
		old, err := os.ReadFile(path)
		if err == nil && bytes.Equal(old, data) {
			return nil
		}
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		backup[path] = saved{old, err == nil, mode}
		return writeFile(path, data, mode)
	}
	write := func(path string, data []byte) error { return writeMode(path, data, 0o644) }
	restore := func() {
		for path, s := range backup {
			if s.existed {
				writeFile(path, s.data, s.mode)
			} else {
				os.Remove(path)
			}
		}
	}

	base, err := ws.baseFiles(ctx)
	if err != nil {
		return 0, err
	}
	for path, data := range base {
		if err := write(path, data); err != nil {
			restore()
			return 0, err
		}
	}
	sites, err := m.store.ListSites(ctx)
	if err != nil {
		restore()
		return 0, err
	}
	for _, site := range sites {
		if site.NginxCustom != "" {
			if err := mkdir(filepath.Dir(m.customFile(site.Domain)), 0o755); err != nil {
				restore()
				return 0, err
			}
			if err := write(m.customFile(site.Domain), ws.customBytes(site)); err != nil {
				restore()
				return 0, err
			}
		}
		v, err := m.vhostBytes(ctx, site)
		if err == nil {
			err = write(m.vhostFile(site.Domain), v)
		}
		if err != nil {
			restore()
			return 0, err
		}
		if site.PHPVersion != "" && m.phpInstalled(site.PHPVersion) {
			before := len(backup)
			p, err := m.poolBytes(site)
			if err == nil {
				err = write(m.paths.poolFile(site.PHPVersion, site.SysUser), p)
			}
			if err != nil {
				restore()
				return 0, err
			}
			if len(backup) > before {
				phpChanged[site.PHPVersion] = true
			}
		}
	}
	// Adminer's internal server: follows PHP upgrades and template changes.
	// A missing PHP is not fatal here; the old file is left in place.
	if m.AdminerInstalled() {
		if conf, err := m.adminerConfBytes(ctx); err == nil && conf != nil {
			if err := writeMode(m.adminerConf(), conf, 0o600); err != nil {
				restore()
				return 0, err
			}
			if err := m.writeAdminerIni(); err != nil {
				m.log.Warn("adminer php limits", "err", err)
			}
		}
	}
	if len(backup) == 0 {
		return 0, nil
	}
	if err := ws.reload(ctx); err != nil {
		restore()
		ws.reload(context.WithoutCancel(ctx))
		return 0, err
	}
	for v := range phpChanged {
		if err := m.reloadPHP(ctx, v); err != nil {
			restore()
			ws.reload(context.WithoutCancel(ctx))
			m.reloadPHP(context.WithoutCancel(ctx), v)
			return 0, err
		}
	}
	return len(backup), nil
}
