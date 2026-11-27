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

	"github.com/nefocp/nefo/internal/run"
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

// dropAllRemote forgets the remote access of every PostgreSQL database and
// removes its ufw rules and the listen drop-in. m.mu is held.
func (m *Manager) dropAllRemote(ctx context.Context, lg Logger) error {
	all, err := m.store.AllPGRemoteHosts(ctx)
	if err != nil {
		return err
	}
	c, _ := m.cluster()
	for id := range all {
		d, err := m.store.PGDatabaseByID(ctx, id)
		if err != nil {
			continue
		}
		lg.Logf("Ending the remote access of %s", d.Name)
		if err := m.store.SetPGRemoteHosts(ctx, d.ID, nil); err != nil {
			return err
		}
		if err := m.syncRemoteFirewall(ctx, d.Name, c.port, nil); err != nil {
			return err
		}
	}
	if c.version != "" {
		if err := os.Remove(m.remoteConfPath(c)); err == nil {
			lg.Logf("Removed %s", m.remoteConfPath(c))
		}
	}
	return nil
}

// distLibpq puts libpq5 back to the distribution's version when the
// repository's build of it stays installed (PHP's pgsql extension needs
// it), so that it gets the distribution's security updates again. A
// failure is only logged: PostgreSQL is removed either way.
func (m *Manager) distLibpq(ctx context.Context, lg Logger) {
	cur, err := m.run.Output(ctx, "dpkg-query", "-W", "-f", "${Version}", "libpq5")
	if cur = strings.TrimSpace(cur); err != nil || !strings.Contains(cur, "pgdg") {
		return
	}
	lg.Logf("libpq5 %s came from the PostgreSQL project's repository: going back to the distribution's", cur)
	if err := run.Apt(ctx, m.run, lg, "update"); err != nil {
		lg.Logf("Warning: %v", err)
		return
	}
	out, err := m.run.Output(ctx, "apt-cache", "madison", "libpq5")
	if err != nil {
		lg.Logf("Warning: apt-cache madison libpq5: %v", err)
		return
	}
	v := distVersion(out)
	if v == "" {
		lg.Logf("Warning: the distribution has no libpq5 to go back to")
		return
	}
	if err := run.Apt(ctx, m.run, lg, "install", "--allow-downgrades", "--no-install-recommends", "libpq5="+v); err != nil {
		lg.Logf("Warning: libpq5 stays at %s: %v", cur, err)
	}
}

var debVersionRe = regexp.MustCompile(`^[0-9][0-9A-Za-z.+~:-]{0,80}$`)

// distVersion returns the newest version "apt-cache madison" lists that is
// not a PGDG build (madison lists the newest first).
func distVersion(madison string) string {
	for _, l := range strings.Split(madison, "\n") {
		f := strings.Split(l, "|")
		if len(f) < 3 {
			continue
		}
		v := strings.TrimSpace(f[1])
		if debVersionRe.MatchString(v) && !strings.Contains(v, "pgdg") && !strings.Contains(f[2], "apt.postgresql.org") {
			return v
		}
	}
	return ""
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
	// Remote access left by databases the server lost: close the port.
	if err := m.dropAllRemote(ctx, lg); err != nil {
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
	// The PostgreSQL project's repository was only there for PostgreSQL.
	hadRepo := m.hasPGDGRepo()
	if err := m.removePGDGRepo(lg); err != nil {
		return err
	}
	if hadRepo {
		m.distLibpq(ctx, lg)
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
