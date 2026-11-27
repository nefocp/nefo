package postgres

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/software/apt"
	"github.com/nefocp/nefo/internal/usererr"
)

// Upgrading the cluster to a newer major version from apt.postgresql.org,
// with postgresql-common's pg_upgradecluster and pg_upgrade (-m upgrade:
// the data files are copied, so the old cluster stays as it was until the
// end). In order:
//
//  1. checks: the main cluster runs, no other cluster, no extension
//     packages of the old version (pg_upgrade would need them for the new
//     one too), enough disk for a copy of the data and the dump;
//  2. every database is dumped with pg_dumpall into defaultBackupDir (root
//     only, kept afterwards as a fallback);
//  3. the repository is added if needed and postgresql-<new> installed; the
//     empty main cluster its package makes is dropped, as pg_upgradecluster
//     makes its own;
//  4. pg_upgradecluster -m upgrade (PostgreSQL is down meanwhile). The new
//     cluster takes over the port, and pg_hba.conf as the old one had it;
//  5. the new cluster must run and hold the same databases; the panel's
//     settings and remote access are written for it;
//  6. only then is the old cluster dropped and its packages removed.
//
// If any step from 3 to 5 fails, the new cluster and its packages go, the
// old cluster's settings are put back and it starts again on its port:
// the server is as before, plus the dump.

// defaultBackupDir is where Upgrade keeps its dump.
const defaultBackupDir = "/var/backups/nefo-postgresql"

// checkUpgradable returns why the cluster cannot be upgraded now (a
// usererr), or nil.
func (m *Manager) checkUpgradable(ctx context.Context) error {
	c, err := m.ready(ctx)
	if err != nil {
		return err
	}
	if vs := m.clusterVersions(); len(vs) > 1 {
		return usererr.New("pg_upgrade_clusters", strings.Join(vs, ", "))
	}
	if others, _ := filepath.Glob(filepath.Join(m.etcDir, c.version, "*", "postgresql.conf")); len(others) > 1 {
		var names []string
		for _, p := range others {
			if n := filepath.Base(filepath.Dir(p)); n != "main" {
				names = append(names, c.version+"/"+n)
			}
		}
		return usererr.New("pg_upgrade_clusters", strings.Join(names, ", "))
	}
	pkgs, err := apt.Installed(ctx, m.run, false, "postgresql-"+c.version+"-*")
	if err != nil {
		return err
	}
	var ext []string
	for _, p := range pkgs {
		if p != "postgresql-"+c.version+"-jit" { // part of the server, split off in newer packages
			ext = append(ext, p)
		}
	}
	if len(ext) > 0 {
		return usererr.New("pg_upgrade_extensions", strings.Join(ext, ", "))
	}
	return nil
}

// Upgrade moves the main cluster to a newer major version (see above).
func (m *Manager) Upgrade(ctx context.Context, target string, lg Logger) error {
	if !slices.Contains(pgdgMajors, target) {
		return usererr.New("pg_version_unknown", target)
	}
	if err := m.pgdgAvailable(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.checkUpgradable(ctx); err != nil {
		return err
	}
	old, ok := m.cluster()
	if !ok {
		return usererr.New("pg_not_installed")
	}
	newMajor, _ := strconv.Atoi(target)
	if newMajor <= old.major() {
		return usererr.New("pg_upgrade_not_newer", old.version)
	}

	// Disk: pg_upgrade copies the data files, and the dump takes space too.
	size, err := m.clusterSize(ctx, old)
	if err != nil {
		return err
	}
	need := 2*size + 256<<20
	if free, err := m.freeBytes(m.dataDir); err == nil && free < need {
		return usererr.New("pg_upgrade_disk", strconv.FormatUint((need-free)>>20+1, 10))
	}
	dbsBefore, err := m.databaseNames(ctx, old)
	if err != nil {
		return err
	}

	// 2. The dump.
	if err := fsutil.MkdirAll(m.backupDir, 0o700); err != nil {
		return err
	}
	dump := filepath.Join(m.backupDir, fmt.Sprintf("postgresql-%s-before-%s-%s.sql.gz", old.version, target, time.Now().Format("20060102-150405")))
	lg.Logf("Saving every database (%s of data) to %s before the upgrade", sizeText(size), dump)
	if err := m.dumpAll(ctx, old, dump); err != nil {
		os.Remove(dump)
		return fmt.Errorf("the dump before the upgrade failed, so nothing was changed: %w", err)
	}
	if fi, err := os.Stat(dump); err == nil {
		lg.Logf("Saved: %s, compressed", sizeText(uint64(fi.Size())))
	}

	// What a rollback puts back.
	oldDir := m.mainDir(old.version)
	saved := map[string][]byte{}
	for _, n := range []string{"postgresql.conf", "start.conf"} {
		if b, err := os.ReadFile(filepath.Join(oldDir, n)); err == nil {
			saved[n] = b
		}
	}
	oldHBAPath := m.hbaFile(ctx, old)
	oldHBA, err := os.ReadFile(oldHBAPath)
	if err != nil {
		return err
	}
	serversBefore := serverVersions(m.libDir)
	addedRepo, aptRan := !m.hasPGDGRepo(), false
	rollback := func(cause error) error {
		ctx := context.WithoutCancel(ctx)
		lg.Logf("The upgrade failed: %v", cause)
		lg.Logf("Rolling back to PostgreSQL %s", old.version)
		var pkgs []string
		for _, v := range serverVersions(m.libDir) {
			if slices.Contains(serversBefore, v) {
				continue
			}
			if _, ok := m.clusterOf(v); ok {
				if out, err := m.run.Output(ctx, "pg_dropcluster", "--stop", v, "main"); err != nil {
					lg.Logf("Warning: pg_dropcluster %s main: %s", v, run.Tail(out, 5))
				}
			}
			pkgs = append(pkgs, "postgresql-"+v, "postgresql-client-"+v, "postgresql-"+v+"-jit")
		}
		for n, b := range saved {
			p := filepath.Join(oldDir, n)
			if cur, err := os.ReadFile(p); err != nil || string(cur) != string(b) {
				lg.Logf("Putting back %s", p)
				m.owned(old, p, b, 0o644)
			}
		}
		if len(pkgs) > 0 {
			if pkgs, err := apt.Installed(ctx, m.run, false, pkgs...); err == nil && len(pkgs) > 0 {
				if err := apt.Remove(ctx, m.run, lg, false, pkgs...); err != nil {
					lg.Logf("Warning: the new packages could not be removed: %v", err)
				}
			}
		}
		switch {
		case addedRepo && !aptRan:
			m.removePGDGRepo(lg)
		case addedRepo:
			// apt brought postgresql-common and libpq5 from the repository
			// too; without it they would get no more updates.
			lg.Logf("The PostgreSQL project's repository stays: postgresql-common and libpq5 now come from it")
		}
		if out, err := m.run.Output(ctx, "systemctl", "start", old.unit()); err != nil {
			lg.Logf("Warning: starting PostgreSQL %s: %s", old.version, run.Tail(out, 5))
		}
		if err := m.waitReady(ctx, old); err != nil {
			return fmt.Errorf("the upgrade failed (%v) and PostgreSQL %s did not start again: %w. Every database is in %s", cause, old.version, err, dump)
		}
		return fmt.Errorf("the upgrade to PostgreSQL %s failed (%v). PostgreSQL %s runs again with its data as before; a dump of every database is in %s", target, cause, old.version, dump)
	}

	// 3. The new version's packages. The "postgresql" metapackage (and
	// its kin) would follow the newest major of the new repository and bring
	// a second server along: the old version's packages are marked as
	// installed by hand, then the metapackages go.
	if meta, err := apt.Installed(ctx, m.run, false, "postgresql", "postgresql-contrib", "postgresql-client"); err == nil && len(meta) > 0 {
		keep, _ := apt.Installed(ctx, m.run, false, "postgresql-"+old.version, "postgresql-client-"+old.version)
		if len(keep) > 0 {
			if out, err := m.run.Output(ctx, "apt-mark", append([]string{"manual"}, keep...)...); err != nil {
				return rollback(fmt.Errorf("apt-mark manual: %s", run.Tail(out, 5)))
			}
		}
		lg.Logf("Removing %s, which would follow the newest major version of the new repository", strings.Join(meta, " "))
		if err := apt.Remove(ctx, m.run, lg, false, meta...); err != nil {
			return rollback(err)
		}
	}
	if addedRepo {
		lg.Logf("Adding the PostgreSQL project's repository (apt.postgresql.org, signing key %s)", pgdgFingerprint)
		if err := m.addPGDGRepo(ctx); err != nil {
			return rollback(err)
		}
	}
	lg.Logf("Updating the package lists")
	if err := run.Apt(ctx, m.run, lg, "update"); err != nil {
		return rollback(err)
	}
	lg.Logf("Installing PostgreSQL %s from apt.postgresql.org", target)
	aptRan = true
	if err := run.Apt(ctx, m.run, lg, "install", "--no-install-recommends", "postgresql-"+target); err != nil {
		return rollback(err)
	}
	if !slices.Contains(serverVersions(m.libDir), target) {
		return rollback(fmt.Errorf("the postgresql-%s package did not install a server", target))
	}
	for _, v := range serverVersions(m.libDir) {
		if v != target && !slices.Contains(serversBefore, v) {
			return rollback(fmt.Errorf("apt also installed PostgreSQL %s", v))
		}
	}
	if _, ok := m.clusterOf(target); ok {
		lg.Logf("Dropping the empty cluster the package made: the upgrade makes its own")
		if out, err := m.run.Output(ctx, "pg_dropcluster", "--stop", target, "main"); err != nil {
			return rollback(fmt.Errorf("pg_dropcluster %s main: %s", target, run.Tail(out, 5)))
		}
	}

	// 4. The upgrade itself.
	lg.Logf("Upgrading the cluster with pg_upgradecluster: PostgreSQL is stopped until it is done")
	if err := m.run.Stream(ctx, lg, "pg_upgradecluster", "-v", target, "-m", "upgrade", old.version, "main"); err != nil {
		return rollback(fmt.Errorf("pg_upgradecluster: %w", err))
	}

	// 5. The new cluster must run, on the old port, with the same databases.
	nc, ok := m.clusterOf(target)
	if !ok {
		return rollback(fmt.Errorf("there is no PostgreSQL %s cluster after pg_upgradecluster", target))
	}
	if nc.port != old.port {
		return rollback(fmt.Errorf("the new cluster uses port %d instead of %d", nc.port, old.port))
	}
	lg.Logf("Writing the panel's settings for PostgreSQL %s", target)
	if err := m.writeTuning(nc, tuningFor(m.memTotal())); err != nil {
		return rollback(err)
	}
	newHBA := filepath.Join(m.mainDir(target), "pg_hba.conf")
	if err := m.owned(nc, newHBA, oldHBA, 0o640); err != nil {
		return rollback(err)
	}
	if out, err := m.run.Output(ctx, "systemctl", "restart", nc.unit()); err != nil {
		return rollback(fmt.Errorf("starting PostgreSQL %s: %s", target, run.Tail(out, 10)))
	}
	if err := m.waitReady(ctx, nc); err != nil {
		return rollback(err)
	}
	if err := m.applyRemote(ctx, nc); err != nil {
		return rollback(err)
	}
	if m.restartPending(ctx, nc) {
		if out, err := m.run.Output(ctx, "systemctl", "restart", nc.unit()); err != nil {
			return rollback(fmt.Errorf("starting PostgreSQL %s: %s", target, run.Tail(out, 10)))
		}
		if err := m.waitReady(ctx, nc); err != nil {
			return rollback(err)
		}
	}
	dbsAfter, err := m.databaseNames(ctx, nc)
	if err != nil {
		return rollback(err)
	}
	if !slices.Equal(dbsBefore, dbsAfter) {
		return rollback(fmt.Errorf("the new cluster holds %s instead of %s", strings.Join(dbsAfter, ", "), strings.Join(dbsBefore, ", ")))
	}
	for _, db := range dbsAfter {
		if checkName(db) != nil {
			continue // postgres, template1 and names the panel did not make
		}
		if _, err := m.sql(ctx, nc, db, "SELECT 1;\n"); err != nil {
			return rollback(fmt.Errorf("database %s does not answer: %w", db, err))
		}
	}
	lg.Logf("PostgreSQL %s runs on port %d with the %d databases", target, nc.port, len(dbsAfter))

	// 6. The old cluster and its packages go.
	lg.Logf("Dropping the old PostgreSQL %s cluster", old.version)
	if out, err := m.run.Output(ctx, "pg_dropcluster", "--stop", old.version, "main"); err != nil {
		lg.Logf("Warning: pg_dropcluster %s main: %s", old.version, run.Tail(out, 5))
	}
	// pg_dropcluster leaves the files it did not make (the panel's) and the
	// version's folders: take out the panel's, then the folders if empty.
	for _, p := range []string{
		filepath.Join(oldDir, "conf.d", confName), filepath.Join(oldDir, "conf.d", remoteConfName),
		filepath.Join(oldDir, hbaIncludeName), filepath.Join(oldDir, panelCertName), filepath.Join(oldDir, panelKeyName),
		filepath.Join(oldDir, "conf.d"), oldDir, filepath.Join(m.etcDir, old.version),
		filepath.Join(m.dataDir, old.version),
	} {
		os.Remove(p) // a folder only if empty
	}
	pkgs, err := apt.Installed(ctx, m.run, false, "postgresql", "postgresql-contrib", "postgresql-"+old.version, "postgresql-client-"+old.version, "postgresql-"+old.version+"-jit")
	if err == nil && len(pkgs) > 0 {
		if err := apt.Remove(ctx, m.run, lg, false, pkgs...); err != nil {
			lg.Logf("Warning: PostgreSQL %s's packages could not be removed: %v", old.version, err)
		}
	}
	if _, err := m.run.Output(ctx, "systemctl", "enable", "postgresql"); err != nil {
		lg.Logf("Warning: systemctl enable postgresql: %v", err)
	}
	lg.Logf("PostgreSQL %s is ready. The dump made before the upgrade stays in %s: delete it once you are sure everything works.", target, dump)
	return nil
}

// clusterSize returns the size of every database of a running cluster.
func (m *Manager) clusterSize(ctx context.Context, c cluster) (uint64, error) {
	out, err := m.sql(ctx, c, "postgres", "SELECT coalesce(sum(pg_database_size(oid)), 0) FROM pg_database;\n")
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(out), 10, 64)
}

// databaseNames returns the cluster's databases but template0, sorted.
func (m *Manager) databaseNames(ctx context.Context, c cluster) ([]string, error) {
	out, err := m.sql(ctx, c, "postgres", "SELECT datname FROM pg_database WHERE datname <> 'template0' ORDER BY datname;\n")
	if err != nil {
		return nil, err
	}
	names := strings.Fields(out)
	slices.Sort(names)
	return names, nil
}

// dumpAll writes pg_dumpall's output, gzipped, to path (0600).
func (m *Manager) dumpAll(ctx context.Context, c cluster, path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	gz := gzip.NewWriter(f)
	err = m.run.Pipe(ctx, nil, gz, "runuser", "-u", "postgres", "--",
		m.bin(c, "pg_dumpall"), "--no-password", "-h", m.socketDir, "-p", strconv.Itoa(c.port))
	if cerr := gz.Close(); err == nil {
		err = cerr
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		if fi, serr := os.Stat(path); serr == nil && fi.Size() < 64 {
			err = errors.New("pg_dumpall wrote nothing")
		}
	}
	return err
}

// sizeText is a size for a job log: "12 MB" or "300 KB".
func sizeText(n uint64) string {
	if n < 1<<20 {
		return strconv.FormatUint((n+1023)>>10, 10) + " KB"
	}
	return strconv.FormatUint(n>>20, 10) + " MB"
}
