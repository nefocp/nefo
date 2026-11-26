package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/nefocp/nefo/internal/software/apt"
	"github.com/nefocp/nefo/internal/usererr"
)

// Uninstalling PostgreSQL, the same way as MariaDB: refused while the
// server holds any database other than its own (postgres, template0,
// template1), whoever made it, as the server itself says. The clusters'
// data and configuration stay unless the administrator asks for them to be
// deleted and types "postgresql": the packages are then removed without
// purging, which keeps /etc/postgresql and /var/lib/postgresql, and
// installing PostgreSQL again starts the same cluster.

// systemDatabases are PostgreSQL's own.
var systemDatabases = map[string]bool{"postgres": true, "template0": true, "template1": true}

// UserDatabases lists the databases on the server other than PostgreSQL's
// own. It needs the server running (its data folder names databases by
// number), unless there is no cluster at all.
func (m *Manager) UserDatabases(ctx context.Context) ([]string, error) {
	names := []string{}
	c, ok := m.cluster()
	if !ok {
		if m.hasClusterData() {
			return nil, usererr.New("pg_not_running")
		}
		return names, nil
	}
	if !m.Status(ctx).Running {
		return nil, usererr.New("pg_not_running")
	}
	out, err := m.sql(ctx, c, "postgres", "SELECT datname FROM pg_database;")
	if err != nil {
		return nil, err
	}
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimSpace(l); l != "" && !systemDatabases[l] {
			names = append(names, l)
		}
	}
	sort.Strings(names)
	return names, nil
}

// hasClusterData reports whether a cluster's data folder exists.
func (m *Manager) hasClusterData() bool {
	matches, _ := filepath.Glob(filepath.Join(m.dataDir, "*", "*", "PG_VERSION"))
	return len(matches) > 0
}

// HasData reports whether PostgreSQL's data folder holds a cluster.
func (m *Manager) HasData() bool { return m.hasClusterData() }

// DataDir is where the clusters' data lives.
func (m *Manager) DataDir() string { return m.dataDir }

// CheckRemove returns why PostgreSQL cannot be uninstalled now (a
// usererr), or nil. deleteData asks for the data and the clusters'
// configuration to be deleted too, which confirm must allow by being
// "postgresql".
func (m *Manager) CheckRemove(ctx context.Context, deleteData bool, confirm string) error {
	if !m.supported() {
		return usererr.New("unsupported")
	}
	if !m.Status(ctx).Installed {
		return usererr.New("software_not_installed", "PostgreSQL")
	}
	if deleteData && strings.TrimSpace(confirm) != "postgresql" {
		return usererr.New("software_confirm_mismatch", "postgresql")
	}
	dbs, err := m.UserDatabases(ctx)
	if err != nil {
		return err
	}
	if len(dbs) > 0 {
		list := dbs
		if len(list) > 8 {
			list = append(list[:8:8], "…")
		}
		return usererr.New("pg_has_databases", strings.Join(list, ", "))
	}
	return nil
}

var serverPkgRe = regexp.MustCompile(`^postgresql-([0-9]+)$`)

// Remove uninstalls the PostgreSQL packages. Without deleteData they are
// removed but not purged, so the clusters' configuration and data stay.
func (m *Manager) Remove(ctx context.Context, deleteData bool, confirm string, lg Logger) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.CheckRemove(ctx, deleteData, confirm); err != nil {
		return err
	}
	pkgs, err := apt.Installed(ctx, m.run, deleteData, "postgresql*")
	if err != nil {
		return err
	}
	// Purging a server package may delete its clusters (a debconf question
	// that defaults to yes). Only a purge asked for gets there; say no to
	// it otherwise, so an earlier answer cannot delete them.
	if !deleteData {
		var answers strings.Builder
		for _, p := range pkgs {
			if mm := serverPkgRe.FindStringSubmatch(p); mm != nil {
				fmt.Fprintf(&answers, "%s %s/postrm_purge_data boolean false\n", p, p)
			}
		}
		if answers.Len() > 0 {
			if out, err := m.run.Input(ctx, []byte(answers.String()), "debconf-set-selections"); err != nil {
				lg.Logf("Warning: debconf-set-selections: %s", strings.TrimSpace(out))
			}
		}
	}
	if err := apt.Check(ctx, m.run, deleteData, pkgs...); err != nil {
		return err
	}
	lg.Logf("Stopping PostgreSQL")
	if out, err := m.run.Output(ctx, "systemctl", "stop", "postgresql"); err != nil {
		return fmt.Errorf("stopping PostgreSQL failed: %s", strings.TrimSpace(out))
	}
	lg.Logf("Removing PostgreSQL")
	if err := apt.Remove(ctx, m.run, lg, deleteData, pkgs...); err != nil {
		return err
	}
	if !deleteData {
		if m.hasClusterData() {
			lg.Logf("PostgreSQL is removed. Its data (%s) and settings (%s) were kept; installing PostgreSQL again uses them.", m.dataDir, m.etcDir)
		} else {
			lg.Logf("PostgreSQL is removed.")
		}
		return nil
	}
	if out, err := m.run.Output(ctx, "pgrep", "-x", "postgres"); err == nil && strings.TrimSpace(out) != "" {
		return errors.New("PostgreSQL is still running, so its data was not deleted")
	}
	for _, dir := range []string{m.dataDir, m.etcDir} {
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		lg.Logf("Deleting %s", dir)
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}
	lg.Logf("PostgreSQL is removed, with its data.")
	return nil
}
