package database

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/usererr"
)

// Upgrading MariaDB in place to a newer LTS series from MariaDB's
// repository (mariadbrepo.go), such as the distribution's 10.11 to 11.4:
//
//  1. every database is exported first (dumps.go), and the packages of
//     both versions are downloaded, so going back needs no network;
//  2. the server is made read-only, its rows counted, and stopped; its
//     data directory and /etc/mysql are copied;
//  3. the new packages replace the old ones and start the server on the
//     same data, still read-only; mariadb-upgrade upgrades its system
//     tables;
//  4. the new server must have the new version, the same tables with the
//     same rows, views, routines, triggers and events, and the panel's
//     database users must sign in. Then it accepts writes again and the
//     copy of the data is deleted; the dumps stay.
//
// When a step of 3 or 4 fails, the old packages come back with the copied
// data directory and configuration: the server is as it was before. Going
// back to an older series (a downgrade) is refused: MariaDB can't read the
// data files and system tables a newer version upgraded.

// readOnlyConfName keeps a server read-only while the panel upgrades or
// switches it, until its data is checked.
const readOnlyConfName = "99-panel-readonly.cnf"

const readOnlyConf = `# Managed by the panel: the database server is being upgraded or replaced.
# The panel removes this file when it is done.
[mysqld]
read_only = ON
`

// UpgradePlan describes an upgrade of MariaDB, for the UI.
type UpgradePlan struct {
	From       string   `json:"from"` // the installed version, "10.11.14"
	FromSeries string   `json:"from_series"`
	To         string   `json:"to"` // the series, "11.4"
	Until      string   `json:"until"`
	Databases  []string `json:"databases"`
	DataMB     int64    `json:"data_mb"` // the data directory
	NeedMB     int64    `json:"need_mb"` // free disk space the upgrade needs
	FreeMB     int64    `json:"free_mb"` // -1 when unknown
	DumpsDir   string   `json:"dumps_dir"`
}

// upgradeExtraMB is roughly the disk the downloaded packages of both
// versions take.
const upgradeExtraMB = 400

// CheckUpgrade returns the plan of an upgrade of the installed MariaDB to
// series, or why it can't be done (a usererr).
func (m *Manager) CheckUpgrade(ctx context.Context, series string) (*UpgradePlan, error) {
	if !m.supported() {
		return nil, usererr.New("unsupported")
	}
	st := m.Status(ctx)
	if !st.Installed || st.Engine != MariaDB {
		return nil, usererr.New("software_not_installed", "MariaDB")
	}
	info, ok := seriesInfo(series)
	if !ok {
		return nil, usererr.New("unknown_software", SeriesID(series))
	}
	switch c := compareSeries(series, st.Series); {
	case c < 0:
		return nil, usererr.New("mariadb_downgrade", st.Series)
	case c == 0:
		return nil, usererr.New("mariadb_upgrade_same", st.Series)
	}
	if err := m.seriesAvailable(series); err != nil {
		return nil, err
	}
	if !st.Running {
		return nil, usererr.New("db_not_running")
	}
	if m.customDataDir() {
		return nil, usererr.New("db_datadir_custom")
	}
	dbs, err := m.UserDatabases(ctx)
	if err != nil {
		return nil, err
	}
	data := dirSize(m.dataDirPath()) >> 20
	p := &UpgradePlan{From: st.Version, FromSeries: st.Series, To: series, Until: info.Until, Databases: dbs, DataMB: data,
		NeedMB: data + data/2 + upgradeExtraMB, FreeMB: freeDiskMB(m.dataDirPath()), DumpsDir: m.dumpsPath()}
	if err := diskCheck(m.dataDirPath(), p.NeedMB); err != nil {
		return p, err
	}
	return p, nil
}

// upgradePackages are the packages a MariaDB upgrade replaces, which going
// back installs again in their old versions.
var upgradePackages = []string{"mariadb-*", "mysql-common", "libmariadb3", "galera-4"}

// UpgradeMariaDB upgrades the installed MariaDB to series (see the comment
// at the top of this file).
func (m *Manager) UpgradeMariaDB(ctx context.Context, series string, lg Logger) error {
	plan, err := m.CheckUpgrade(ctx, series)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.busy.Store(true)
	defer m.busy.Store(false)
	from, to := "MariaDB "+plan.From, "MariaDB "+series
	dir, err := m.newDumpDir(fmt.Sprintf("mariadb-%s-to-%s", plan.FromSeries, series))
	if err != nil {
		return err
	}
	mf := &manifest{Kind: "upgrade", From: from, To: to, Created: time.Now().UTC()}
	finish := func(result string, err error) error {
		mf.Result = result
		if err != nil {
			mf.Error = err.Error()
		}
		if werr := writeManifest(dir, mf); werr != nil {
			lg.Logf("Warning: %v", werr)
		}
		return err
	}
	if err := writeManifest(dir, mf); err != nil {
		return err
	}
	lg.Logf("Upgrading %s to %s. First, every database is exported into %s.", from, to, dir)
	if mf.Databases, err = m.dumpDatabases(ctx, MariaDB, dir, plan.Databases, lg); err != nil {
		return finish("failed", err)
	}
	if err := writeManifest(dir, mf); err != nil {
		return finish("failed", err)
	}

	old := m.installedPackages(ctx, upgradePackages...)
	repo := m.saveMariaDBRepo()
	lg.Logf("Adding MariaDB's repository for MariaDB %s (signing key %s)", series, mariadbKeyFingerprint)
	if err := m.addMariaDBRepo(ctx, series); err != nil {
		repo.restore(m)
		return finish("failed", err)
	}
	if err := m.fetchPackages(ctx, series, old, lg); err != nil {
		repo.restore(m)
		return finish("failed", err)
	}

	lg.Logf("Making MariaDB read-only and counting the rows of every table")
	if _, err := m.sqlWith(ctx, MariaDB, "SET GLOBAL read_only = ON;"); err != nil {
		repo.restore(m)
		return finish("failed", err)
	}
	writable := func() { m.sqlWith(context.WithoutCancel(ctx), MariaDB, "SET GLOBAL read_only = OFF;") }
	counts, err := m.countObjects(ctx, MariaDB, plan.Databases)
	if err != nil {
		writable()
		repo.restore(m)
		return finish("failed", err)
	}
	mf.Counts = counts
	lg.Logf("Stopping MariaDB %s", plan.From)
	if out, err := m.run.Output(ctx, "systemctl", "stop", MariaDB.service()); err != nil {
		writable()
		repo.restore(m)
		return finish("failed", fmt.Errorf("stopping MariaDB failed: %s", strings.TrimSpace(out)))
	}
	saved, err := m.saveServerFiles(ctx, dir, "before-"+series, lg)
	if err != nil {
		m.run.Output(context.WithoutCancel(ctx), "systemctl", "start", MariaDB.service())
		repo.restore(m)
		return finish("failed", err)
	}

	err = m.upgradeSteps(ctx, plan, counts, lg)
	if err == nil {
		lg.Logf("Deleting the copy of the data directory from before the upgrade (%s)", saved.data)
		if rerr := os.RemoveAll(saved.data); rerr != nil {
			lg.Logf("Warning: %v", rerr)
		}
		os.RemoveAll(saved.etc)
		lg.Logf("MariaDB is upgraded to %s. The dumps from before the upgrade stay in %s until you delete them.", m.Status(ctx).Version, dir)
		return finish("done", nil)
	}
	lg.Logf("The upgrade failed: %v", err)
	lg.Logf("Going back to %s with the data from before the upgrade", from)
	if rerr := m.restoreEngine(ctx, MariaDB, plan.FromSeries, old, repo, saved, lg); rerr != nil {
		return finish("failed", fmt.Errorf("%w. Going back to %s failed as well: %v. The data directory from before the upgrade is %s, "+
			"its configuration %s, and the dumps are in %s", err, from, rerr, saved.data, saved.etc, dir))
	}
	return finish("rolled_back", fmt.Errorf("the upgrade to %s failed, and %s is back as it was: %w", to, from, err))
}

// fetchPackages updates the package lists and downloads the new series'
// packages, and the installed ones (old) for going back.
func (m *Manager) fetchPackages(ctx context.Context, series string, old []pkgVersion, lg Logger) error {
	lg.Logf("Updating the package lists")
	if err := run.Apt(ctx, m.run, lg, "update"); err != nil {
		return err
	}
	if err := m.checkCandidate(ctx, series); err != nil {
		return err
	}
	lg.Logf("Downloading MariaDB %s", series)
	if err := run.Apt(ctx, m.run, lg, "install", "--download-only", "--no-install-recommends", "mariadb-server", "mariadb-client"); err != nil {
		return err
	}
	m.keepOldPackages(ctx, old, lg)
	return nil
}

// keepOldPackages downloads the installed packages into apt's cache, so
// that going back works without the network.
func (m *Manager) keepOldPackages(ctx context.Context, old []pkgVersion, lg Logger) {
	if len(old) == 0 {
		return
	}
	args := []string{"install", "--download-only", "--reinstall", "--allow-downgrades", "--no-install-recommends"}
	for _, p := range old {
		args = append(args, p.Name+"="+p.Version)
	}
	if err := run.Apt(ctx, m.run, lg, args...); err != nil {
		lg.Logf("Warning: the installed packages could not be kept for going back (%v); going back would download them again", err)
	}
}

// savedRepo is the panel's MariaDB repository before an upgrade or switch.
type savedRepo struct {
	list, key []byte // nil when there was none
}

func (m *Manager) saveMariaDBRepo() savedRepo {
	keyring, list := m.mariaRepoPaths()
	var r savedRepo
	if b, err := os.ReadFile(list); err == nil {
		r.list = b
		r.key, _ = os.ReadFile(keyring)
	}
	return r
}

// restore puts the repository back as it was.
func (r savedRepo) restore(m *Manager) {
	if r.list == nil {
		m.removeMariaDBRepo(nil)
		return
	}
	keyring, list := m.mariaRepoPaths()
	if r.key != nil {
		fsutil.WriteFileAtomic(keyring, r.key, 0o644)
	}
	fsutil.WriteFileAtomic(list, r.list, 0o644)
}

// savedFiles are the copies of a stopped server's data directory and
// configuration that going back restores.
type savedFiles struct {
	data  string // the data directory's copy, or the moved directory
	etc   string // the copy of /etc/mysql
	moved bool   // data was moved, not copied
}

// saveServerFiles copies the stopped server's configuration into dir, and
// its data directory next to it (as /var/lib/mysql.<tag>-<time>).
func (m *Manager) saveServerFiles(ctx context.Context, dir, tag string, lg Logger) (savedFiles, error) {
	s := savedFiles{data: m.dataDirPath() + "." + tag + "-" + time.Now().Format("20060102-150405"), etc: filepath.Join(dir, "etc-mysql")}
	lg.Logf("Copying the configuration %s to %s", m.etcDirPath(), s.etc)
	if err := m.copyTree(ctx, m.etcDirPath(), s.etc); err != nil {
		return s, err
	}
	lg.Logf("Copying the data directory %s to %s", m.dataDirPath(), s.data)
	if err := m.copyTree(ctx, m.dataDirPath(), s.data); err != nil {
		os.RemoveAll(s.data)
		return s, err
	}
	return s, nil
}

// upgradeSteps installs the new series over the stopped server and checks
// it (steps 3 and 4).
func (m *Manager) upgradeSteps(ctx context.Context, plan *UpgradePlan, counts map[string]objectCounts, lg Logger) error {
	roConf := filepath.Join(m.confDir(MariaDB), readOnlyConfName)
	if err := fsutil.WriteFileAtomic(roConf, []byte(readOnlyConf), 0o644); err != nil {
		return err
	}
	lg.Logf("Installing MariaDB %s from MariaDB's repository", plan.To)
	if err := run.Apt(ctx, m.run, lg, "install", "--no-install-recommends", "mariadb-server", "mariadb-client"); err != nil {
		return err
	}
	if _, err := m.run.Output(ctx, "systemctl", "enable", MariaDB.service()); err != nil {
		return err
	}
	if out, err := m.run.Output(ctx, "systemctl", "restart", MariaDB.service()); err != nil {
		return fmt.Errorf("MariaDB %s does not start: %s", plan.To, strings.TrimSpace(out))
	}
	if err := m.waitReady(ctx, MariaDB); err != nil {
		return err
	}
	// The package's start script runs mariadb-upgrade in the background on
	// a new version; the panel runs it again once that one is done.
	m.waitNoProcess(ctx, "mariadb-upgrade", 10*time.Minute)
	lg.Logf("Running mariadb-upgrade")
	if err := m.run.Stream(ctx, lg, "mariadb-upgrade"); err != nil {
		return fmt.Errorf("mariadb-upgrade failed: %w", err)
	}
	if st := m.Status(ctx); st.Series != plan.To {
		return fmt.Errorf("MariaDB reports version %s instead of %s", st.Version, plan.To)
	}
	if err := m.checkData(ctx, MariaDB, counts, lg); err != nil {
		return err
	}
	if err := os.Remove(roConf); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if _, err := m.sqlWith(ctx, MariaDB, "SET GLOBAL read_only = OFF;"); err != nil {
		return err
	}
	return nil
}

// checkData compares the databases on engine e with the counts taken
// before (see objectCounts), and signs in as each of the panel's database
// users.
func (m *Manager) checkData(ctx context.Context, e Engine, before map[string]objectCounts, lg Logger) error {
	lg.Logf("Checking the data: tables, rows, views, routines, triggers and events")
	dbs := make([]string, 0, len(before))
	for db := range before {
		dbs = append(dbs, db)
	}
	after, err := m.countObjects(ctx, e, dbs)
	if err != nil {
		return err
	}
	if diff := compareCounts(before, after); len(diff) > 0 {
		if len(diff) > 10 {
			diff = append(diff[:10], fmt.Sprintf("and %d more differences", len(diff)-10))
		}
		return fmt.Errorf("the data differs from before: %s", strings.Join(diff, "; "))
	}
	tables := 0
	for _, c := range after {
		tables += len(c.Rows)
	}
	lg.Logf("The %d databases have the same %d tables with the same rows", len(dbs), tables)
	return m.checkLogins(ctx, e, lg)
}

// checkLogins signs in as each database user of the panel whose password
// it knows.
func (m *Manager) checkLogins(ctx context.Context, e Engine, lg Logger) error {
	dbs, err := m.store.ListDatabases(ctx)
	if err != nil {
		return err
	}
	socket, err := m.sqlWith(ctx, e, "SELECT @@socket;")
	if err != nil {
		return err
	}
	n := 0
	for _, d := range dbs {
		if d.Password == "" {
			continue
		}
		cnf, err := writeClientConf(d.Username, d.Password, strings.TrimSpace(socket))
		if err != nil {
			return err
		}
		_, err = m.run.Output(ctx, e.client(), "--defaults-file="+cnf, "--batch", "--skip-column-names", "-e", "SELECT 1", d.Name)
		os.Remove(cnf)
		if err != nil {
			return fmt.Errorf("the database user %s can't sign in to %s: %w", d.Username, d.Name, err)
		}
		n++
	}
	if n > 0 {
		lg.Logf("The panel's %d database users sign in with their passwords", n)
	}
	return nil
}

// waitReady waits until the server answers, for up to two minutes.
func (m *Manager) waitReady(ctx context.Context, e Engine) error {
	var err error
	for i := 0; i < 120; i++ {
		if _, err = m.sqlWith(ctx, e, "SELECT 1;"); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return fmt.Errorf("%s does not answer: %w", e.Name(), err)
}

// waitNoProcess waits until no process named name runs, at most max.
func (m *Manager) waitNoProcess(ctx context.Context, name string, max time.Duration) {
	deadline := time.Now().Add(max)
	for time.Now().Before(deadline) {
		if _, err := m.run.Output(ctx, "pgrep", "-x", name); err != nil {
			return // exit status 1: none
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

// restoreEngine brings engine e back as it was before an upgrade or a
// switch: whatever server is installed now is removed, the saved
// configuration and data directory return, and the packages old are
// installed again (in their versions if apt still has them, else in their
// current ones of the same series). series is the series it must report.
func (m *Manager) restoreEngine(ctx context.Context, e Engine, series string, old []pkgVersion, repo savedRepo, saved savedFiles, lg Logger) error {
	ctx = context.WithoutCancel(ctx) // going back must finish even when the job's time ran out
	for _, x := range []Engine{MariaDB, MySQL} {
		pkgs := m.installedPackages(ctx, enginePackages(x)...)
		if len(pkgs) == 0 {
			continue
		}
		if err := m.removeEnginePackages(ctx, x, false, lg); err != nil {
			return err
		}
	}
	repo.restore(m)
	lg.Logf("Restoring %s", m.etcDirPath())
	if err := os.RemoveAll(m.etcDirPath()); err != nil {
		return err
	}
	if err := m.copyTree(ctx, saved.etc, m.etcDirPath()); err != nil {
		return err
	}
	data := m.dataDirPath()
	failed := ""
	if _, err := os.Lstat(data); err == nil {
		failed = data + ".failed-" + time.Now().Format("20060102-150405")
		if err := os.Rename(data, failed); err != nil {
			return err
		}
	}
	lg.Logf("Restoring the data directory %s", data)
	if err := os.Rename(saved.data, data); err != nil {
		return err
	}
	if e == MySQL {
		if err := m.seedMySQL(ctx); err != nil {
			return err
		}
	}
	lg.Logf("Installing %s %s again", e.Name(), series)
	args := []string{"install", "--no-install-recommends", "--allow-downgrades"}
	for _, p := range old {
		args = append(args, p.Name+"="+p.Version)
	}
	if err := run.Apt(ctx, m.run, lg, args...); err != nil {
		lg.Logf("Those versions are not available any more; installing the current ones of %s %s", e.Name(), series)
		main := []string{"install", "--no-install-recommends", "mariadb-server", "mariadb-client"}
		if e == MySQL {
			main = []string{"install", "--no-install-recommends", "mysql-community-server", "mysql-community-client"}
		}
		if err := run.Apt(ctx, m.run, lg, main...); err != nil {
			return err
		}
	}
	m.run.Output(ctx, "systemctl", "enable", e.service())
	if out, err := m.run.Output(ctx, "systemctl", "restart", e.service()); err != nil {
		return fmt.Errorf("%s does not start: %s", e.Name(), strings.TrimSpace(out))
	}
	if err := m.waitReady(ctx, e); err != nil {
		return err
	}
	if st := m.Status(ctx); st.Engine != e || st.Series != series {
		return fmt.Errorf("%s reports version %s instead of %s", e.Name(), st.Version, series)
	}
	if failed != "" {
		lg.Logf("Deleting %s, the data of the failed attempt (it was read-only, so it holds nothing new)", failed)
		if err := os.RemoveAll(failed); err != nil {
			lg.Logf("Warning: %v", err)
		}
	}
	lg.Logf("%s %s runs again with the data from before", e.Name(), m.Status(ctx).Version)
	return nil
}
