// Package backup makes, stores and restores backups of websites, databases
// and folders, on this server and optionally in S3-compatible storage, on
// an SFTP server and in Google Drive, encrypted if the storage asks for it
// (see crypt.go).
//
// Every backup is a .tar.gz archive with a manifest.json, the site's (or
// folder's) files under files/ and database dumps under databases/<name>.sql.
package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/nefocp/nefo/internal/database"
	"github.com/nefocp/nefo/internal/files"
	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
	"github.com/nefocp/nefo/internal/version"
)

// Kinds of backup.
const (
	KindSite     = "site"
	KindDatabase = "database"
	KindFolder   = "folder"
)

// DefaultDir is where backups are kept on the server.
const DefaultDir = "/var/backups/panel"

// Logger receives progress messages for a job log.
type Logger interface {
	Logf(format string, args ...any)
}

// Manager makes and restores backups.
type Manager struct {
	store   *store.Store
	db      *database.Manager
	pg      Postgres // nil: no PostgreSQL databases (see postgres.go)
	hosting *hosting.Manager
	dir     string
	// protected are folders that folder backups may not touch.
	protected []string
	log       *slog.Logger
	now       func() time.Time
	mu        sync.Mutex   // one backup or restore at a time
	drive     driveConnect // a Google Drive sign-in in progress (gdrive.go)
	found     foundScans   // the last scan of each storage (found.go)
}

// New returns a Manager that keeps backups in dir.
func New(st *store.Store, db *database.Manager, h *hosting.Manager, dir string, log *slog.Logger) *Manager {
	m := &Manager{store: st, db: db, hosting: h, dir: dir, log: log, now: time.Now,
		protected: append([]string{}, systemFolders...)}
	m.ProtectFolders(append([]string{defaultDataDir, dir}, panelFolders...)...)
	return m
}

type manifest struct {
	Format       int       `json:"format"`
	Kind         string    `json:"kind"`
	Domain       string    `json:"domain,omitempty"`
	PHPVersion   string    `json:"php_version,omitempty"`
	App          string    `json:"app,omitempty"`
	Path         string    `json:"path,omitempty"` // of a folder backup
	Databases    []string  `json:"databases"`
	PGDatabases  []string  `json:"pg_databases,omitempty"` // PostgreSQL, see postgres.go
	CreatedAt    time.Time `json:"created_at"`
	PanelVersion string    `json:"panel_version"`
	// What a website needs to be created again on another server (see
	// recreateSite). Older backups lack them.
	Aliases     []string `json:"aliases,omitempty"`
	Rewrite     string   `json:"rewrite,omitempty"`
	NginxCustom string   `json:"nginx_custom,omitempty"`
	// Label names why the backup was made ("before-delete"); it is part
	// of the file name.
	Label string `json:"label,omitempty"`
	// DBPasswords are the passwords of the databases' users, so that a
	// database created again keeps its password and the site's own
	// configuration (wp-config.php…) still works. The archive holds that
	// configuration, and the data, anyway.
	DBPasswords map[string]string `json:"db_passwords,omitempty"`
}

// LabelBeforeDelete marks the backup made before a website is deleted.
const LabelBeforeDelete = "before-delete"

// ---- making backups ----

// Backup makes a backup of a website, a database or a folder.
func (m *Manager) Backup(ctx context.Context, kind, target string, scheduleID int64, remote bool, lg Logger) (*store.Backup, error) {
	switch kind {
	case KindSite:
		return m.BackupSite(ctx, target, scheduleID, remote, lg)
	case KindDatabase:
		return m.BackupDatabase(ctx, target, scheduleID, remote, lg)
	case KindFolder:
		return m.BackupFolder(ctx, target, scheduleID, remote, lg)
	case KindApp:
		return m.BackupApp(ctx, target, scheduleID, remote, lg)
	}
	return nil, usererr.New("invalid_target", target)
}

// CheckTarget checks that a backup of target can be made: the website or
// database exists, or the folder exists and may be backed up.
func (m *Manager) CheckTarget(ctx context.Context, kind, target string) error {
	if err := checkTarget(kind, target); err != nil {
		if kind == KindFolder {
			return err
		}
		return usererr.New("invalid_target", target)
	}
	switch kind {
	case KindSite:
		_, err := m.siteByDomain(ctx, target)
		return err
	case KindDatabase:
		_, err := m.store.DatabaseByName(ctx, target)
		if errors.Is(err, store.ErrNotFound) {
			_, err = m.store.PGDatabaseByName(ctx, target)
		}
		return err
	case KindApp:
		_, err := m.hosting.AppBackup(ctx, target)
		return err
	}
	if err := m.checkFolder(target); err != nil {
		return err
	}
	if fi, err := os.Stat(target); err != nil || !fi.IsDir() {
		return usererr.New("folder_missing", target)
	}
	return nil
}

// BackupSite backs up a website's files and its databases. If remote is set
// and storage is configured, the archive is also uploaded; an upload error is
// returned together with the backup, which is kept locally.
func (m *Manager) BackupSite(ctx context.Context, domain string, scheduleID int64, remote bool, lg Logger) (*store.Backup, error) {
	b, _, err := m.backupSite(ctx, domain, scheduleID, remote, "", lg)
	return b, err
}

// BackupBeforeDelete makes the local backup a website's deletion waits
// for: its files and databases (with their passwords), and its Docker
// app's data if it has one, in files named <name>_before-delete_<time>.
// Unlike a scheduled backup, a failed app backup fails it. Like every
// backup made by hand, these are never pruned: schedules only delete
// their own backups (and the website's schedules go with it).
func (m *Manager) BackupBeforeDelete(ctx context.Context, domain string, lg Logger) ([]*store.Backup, error) {
	site, app, err := m.backupSite(ctx, domain, 0, false, LabelBeforeDelete, lg)
	if err != nil {
		return nil, err
	}
	out := []*store.Backup{site}
	if app != nil {
		out = append(out, app)
	}
	return out, nil
}

func (m *Manager) backupSite(ctx context.Context, domain string, scheduleID int64, remote bool, label string, lg Logger) (b, appBackup *store.Backup, err error) {
	site, err := m.siteByDomain(ctx, domain)
	if err != nil {
		return nil, nil, err
	}
	dbs, err := m.store.DatabasesForSite(ctx, site.ID)
	if err != nil {
		return nil, nil, err
	}
	pgdbs, err := m.pgDatabasesForSite(ctx, site.ID)
	if err != nil {
		return nil, nil, err
	}
	man := manifest{Format: 1, Kind: KindSite, Domain: site.Domain, PHPVersion: site.PHPVersion, App: site.App,
		Aliases: site.Aliases, Rewrite: site.Rewrite, NginxCustom: site.NginxCustom, Label: label}
	for _, d := range dbs {
		man.Databases = append(man.Databases, d.Name)
		man.addPassword(d)
	}
	for _, d := range pgdbs {
		man.PGDatabases = append(man.PGDatabases, d.Name)
		man.addPassword(d)
	}
	// A Docker app keeps its data outside the website's folder: it gets a
	// backup of its own next to the site's (same schedule, pruned with it).
	inst := m.hosting.AppNameForSite(ctx, site.ID)
	if inst != "" && label == "" {
		defer func() {
			lg.Logf("Backing up the app of %s", site.Domain)
			if _, err := m.BackupApp(ctx, inst, scheduleID, remote, lg); err != nil {
				lg.Logf("Warning: the app backup failed: %v", err)
			}
		}()
	}
	b, err = m.make(ctx, KindSite, site.Domain, man, scheduleID, remote, lg, func(tw *tar.Writer) error {
		root, err := os.OpenRoot(m.hosting.Root(site.Domain))
		if err != nil {
			return err
		}
		defer root.Close()
		lg.Logf("Packing the files of %s", site.Domain)
		n, err := addTree(ctx, tw, root, "files", func(rel string) bool { return rel == "tmp" || rel == "logs" })
		if err != nil {
			return err
		}
		lg.Logf("%d files packed", n)
		for _, d := range dbs {
			if err := m.addDump(ctx, tw, d.Name, lg); err != nil {
				return err
			}
		}
		for _, d := range pgdbs {
			if err := m.addPGDump(ctx, tw, d.Name, lg); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil || inst == "" || label == "" {
		return b, nil, err
	}
	lg.Logf("Backing up the app of %s", site.Domain)
	if appBackup, err = m.backupApp(ctx, inst, scheduleID, remote, label, lg); err != nil {
		// Without the app's data the website could not be brought back.
		m.remove(context.WithoutCancel(ctx), b, lg)
		return nil, nil, fmt.Errorf("the backup of the app failed: %w", err)
	}
	return b, appBackup, nil
}

func (man *manifest) addPassword(d *store.Database) {
	if d == nil || d.Password == "" {
		return
	}
	if man.DBPasswords == nil {
		man.DBPasswords = map[string]string{}
	}
	man.DBPasswords[d.Name] = d.Password
}

// BackupDatabase backs up one database.
func (m *Manager) BackupDatabase(ctx context.Context, name string, scheduleID int64, remote bool, lg Logger) (*store.Backup, error) {
	var dbPass *store.Database
	if d, err := m.store.DatabaseByName(ctx, name); errors.Is(err, store.ErrNotFound) {
		if pd, perr := m.store.PGDatabaseByName(ctx, name); perr == nil {
			man := manifest{Format: 1, Kind: KindDatabase, PGDatabases: []string{name}}
			man.addPassword(pd)
			return m.make(ctx, KindDatabase, name, man, scheduleID, remote, lg, func(tw *tar.Writer) error {
				return m.addPGDump(ctx, tw, name, lg)
			})
		}
		return nil, err
	} else if err != nil {
		return nil, err
	} else {
		dbPass = d
	}
	man := manifest{Format: 1, Kind: KindDatabase, Databases: []string{name}}
	man.addPassword(dbPass)
	return m.make(ctx, KindDatabase, name, man, scheduleID, remote, lg, func(tw *tar.Writer) error {
		return m.addDump(ctx, tw, name, lg)
	})
}

func (m *Manager) siteByDomain(ctx context.Context, domain string) (*store.Site, error) {
	sites, err := m.store.ListSites(ctx)
	if err != nil {
		return nil, err
	}
	for _, s := range sites {
		if s.Domain == domain {
			return s, nil
		}
	}
	return nil, usererr.New("site_missing", domain)
}

// make writes a backup archive, records it and uploads it if asked.
func (m *Manager) make(ctx context.Context, kind, target string, man manifest, scheduleID int64, remote bool,
	lg Logger, content func(*tar.Writer) error) (*store.Backup, error) {
	if err := checkTarget(kind, target); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.now()
	man.CreatedAt, man.PanelVersion = now.UTC(), version.Version
	if man.Databases == nil {
		man.Databases = []string{}
	}
	dir := filepath.Join(m.dir, kind, storageDir(kind, target))
	if err := mkdirPrivate(dir); err != nil {
		return nil, err
	}
	base := fileBase(kind, target)
	if man.Label != "" {
		if !labelRe.MatchString(man.Label) {
			return nil, fmt.Errorf("invalid backup label %q", man.Label)
		}
		base += "_" + man.Label
	}
	name := fmt.Sprintf("%s_%s.tar.gz", base, now.Format("20060102-150405"))
	file := filepath.Join(dir, name)
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			f.Close()
			os.Remove(file)
		}
	}()

	h := sha256.New()
	gz := gzip.NewWriter(io.MultiWriter(f, h))
	tw := tar.NewWriter(gz)
	mb, _ := json.MarshalIndent(man, "", "  ")
	if err := addBytes(tw, "manifest.json", mb, now); err != nil {
		return nil, err
	}
	if err := content(tw); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	fi, err := os.Stat(file)
	if err != nil {
		return nil, err
	}
	b := &store.Backup{Kind: kind, Target: target, File: file, Size: fi.Size(),
		SHA256: hex.EncodeToString(h.Sum(nil)), ScheduleID: scheduleID, CreatedAt: now}
	if kind == KindApp {
		b.AppID = man.App
	}
	if err := m.store.CreateBackup(ctx, b); err != nil {
		return nil, err
	}
	ok = true
	lg.Logf("Backup saved: %s (%d MB)", file, b.Size>>20)

	if remote {
		if err := m.upload(ctx, b, lg); err != nil {
			return b, fmt.Errorf("the backup was saved on this server, but the upload failed: %w", err)
		}
	}
	return b, nil
}

var labelRe = regexp.MustCompile(`^[a-z][a-z-]{0,30}$`)

func checkTarget(kind, target string) error {
	switch kind {
	case KindSite:
		return validate.Domain(target)
	case KindDatabase:
		return validate.DBName(target)
	case KindFolder:
		return validFolderPath(target)
	case KindApp:
		return checkAppName(target)
	}
	return fmt.Errorf("unknown backup kind %q", kind)
}

func mkdirPrivate(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.Chmod(dir, 0o700)
}

func addBytes(tw *tar.Writer, name string, b []byte, t time.Time) error {
	if err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: name, Size: int64(len(b)), Mode: 0o600, ModTime: t}); err != nil {
		return err
	}
	_, err := tw.Write(b)
	return err
}

// addTree adds the content of root under prefix/, skipping top-level
// entries for which skip returns true. Links and special files are left out.
func addTree(ctx context.Context, tw *tar.Writer, root *os.Root, prefix string, skip func(string) bool) (int, error) {
	count := 0
	err := fs.WalkDir(root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if p == "." {
			return nil
		}
		if !strings.Contains(p, "/") && skip(p) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		name := prefix + "/" + p
		switch {
		case fi.IsDir():
			return tw.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: name + "/", Mode: int64(fi.Mode().Perm()), ModTime: fi.ModTime()})
		case fi.Mode().IsRegular():
			src, err := root.Open(p)
			if err != nil {
				return err
			}
			defer src.Close()
			if err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: name, Size: fi.Size(),
				Mode: int64(fi.Mode().Perm()), ModTime: fi.ModTime()}); err != nil {
				return err
			}
			// A file that grows while it is read must not overflow its header.
			n, err := io.Copy(tw, io.LimitReader(src, fi.Size()))
			if err == nil && n < fi.Size() {
				// It shrank: pad, so the archive stays valid.
				_, err = io.CopyN(tw, zeros{}, fi.Size()-n)
			}
			count++
			return err
		}
		return nil
	})
	return count, err
}

type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

// addDump dumps a database to a temporary file (tar needs the size first)
// and adds it as databases/<name>.sql.
func (m *Manager) addDump(ctx context.Context, tw *tar.Writer, name string, lg Logger) error {
	lg.Logf("Dumping database %s", name)
	tmp, err := os.CreateTemp(m.dir, ".dump-*.sql")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if err := m.db.Dump(ctx, name, tmp); err != nil {
		return err
	}
	fi, err := tmp.Stat()
	if err != nil {
		return err
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: "databases/" + name + ".sql",
		Size: fi.Size(), Mode: 0o600, ModTime: m.now()}); err != nil {
		return err
	}
	_, err = io.Copy(tw, tmp)
	return err
}

// ---- restoring ----

// RestoreOptions change how a backup is restored.
type RestoreOptions struct {
	// Passphrase decrypts an encrypted remote copy. It is tried before the
	// storage's own passphrases, so a copy encrypted with a passphrase
	// that is not saved here can be restored too.
	Passphrase string
}

// Restore puts a backup back. For a site, the site's files are replaced
// (its tmp and logs folders are kept) and its databases are emptied and
// filled from the dumps. The site must exist with the same domain. A folder
// gets back the content it had, with owners and modes.
func (m *Manager) Restore(ctx context.Context, id int64, lg Logger) error {
	return m.RestoreWith(ctx, id, RestoreOptions{}, lg)
}

// RestoreWith is Restore with options.
func (m *Manager) RestoreWith(ctx context.Context, id int64, opt RestoreOptions, lg Logger) error {
	b, err := m.store.BackupByID(ctx, id)
	if err != nil {
		return err
	}
	if err := checkTarget(b.Kind, b.Target); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	f, cleanup, err := m.openArchive(ctx, b, opt.Passphrase, lg)
	if err != nil {
		return err
	}
	defer cleanup()

	// Check the archive's checksum before touching anything. A backup
	// found on a storage has none recorded: it is read through once, which
	// finds a damaged archive, and its checksum is recorded then.
	lg.Logf("Checking the backup")
	h := sha256.New()
	if b.SHA256 == "" {
		if err := checkArchive(io.TeeReader(f, h)); err != nil {
			return fmt.Errorf("the backup file is damaged: %w", err)
		}
		b.SHA256 = hex.EncodeToString(h.Sum(nil))
		if err := m.store.SetBackupChecksum(ctx, b.ID, b.SHA256); err != nil {
			return err
		}
	} else {
		if _, err := io.Copy(h, f); err != nil {
			return err
		}
		if hex.EncodeToString(h.Sum(nil)) != b.SHA256 {
			return errors.New("the backup file is damaged: its checksum does not match")
		}
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	hdr, err := tr.Next()
	if err != nil || hdr.Name != "manifest.json" {
		return errors.New("this is not a backup made by the panel (manifest.json missing)")
	}
	var man manifest
	if err := json.NewDecoder(io.LimitReader(tr, 1<<20)).Decode(&man); err != nil {
		return fmt.Errorf("read manifest.json: %w", err)
	}
	if b.Kind == KindFolder {
		if err := m.restoreFolder(ctx, b, man, tr, lg); err != nil {
			return err
		}
		lg.Logf("Restore finished")
		return nil
	}
	if b.Kind == KindApp {
		if err := m.restoreApp(ctx, b, man, tr, lg); err != nil {
			return err
		}
		lg.Logf("Restore finished")
		return nil
	}

	var site *store.Site
	var x *files.Extractor
	var root *os.Root
	if b.Kind == KindSite {
		if site, err = m.siteByDomain(ctx, b.Target); err != nil {
			if userCodeOf(err) != "site_missing" {
				return err
			}
			// Not on this server (a new server, or the site was deleted):
			// it is created again from the manifest.
			if man.Domain != b.Target {
				return errors.New("the backup's manifest names another website than the backup")
			}
			if site, err = m.recreateSite(ctx, man, lg); err != nil {
				return err
			}
		}
		uid, gid, err := hosting.LookupIDs(site.SysUser)
		if err != nil {
			return err
		}
		if root, err = os.OpenRoot(m.hosting.Root(site.Domain)); err != nil {
			return err
		}
		defer root.Close()
		lg.Logf("Removing the current files of %s", site.Domain)
		entries, err := fs.ReadDir(root.FS(), ".")
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.Name() != "tmp" && e.Name() != "logs" {
				if err := root.RemoveAll(e.Name()); err != nil {
					return err
				}
			}
		}
		if x, err = files.NewExtractor(ctx, root, ".", uid, gid, true, nil); err != nil {
			return err
		}
		lg.Logf("Restoring the files")
	}

	restoredDBs := []string{}
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read backup: %w", err)
		}
		name := path.Clean(hdr.Name)
		switch {
		case strings.HasPrefix(name, "files/") && x != nil:
			rel := strings.TrimPrefix(name, "files/")
			if hdr.Typeflag == tar.TypeDir {
				err = x.Folder(rel)
			} else if hdr.Typeflag == tar.TypeReg {
				err = x.File(rel, tr, fs.FileMode(hdr.Mode))
			}
		case strings.HasPrefix(name, "databases/") && strings.HasSuffix(name, ".sql"):
			dbName := strings.TrimSuffix(strings.TrimPrefix(name, "databases/"), ".sql")
			err = m.restoreDB(ctx, dbName, man.DBPasswords[dbName], site, tr, lg)
			restoredDBs = append(restoredDBs, dbName)
		case strings.HasPrefix(name, pgDir) && strings.HasSuffix(name, ".sql"):
			pgName := strings.TrimSuffix(strings.TrimPrefix(name, pgDir), ".sql")
			err = m.restorePG(ctx, pgName, man.DBPasswords[pgName], site, tr, lg)
		}
		if err != nil {
			return err
		}
	}
	if x != nil {
		lg.Logf("%d files restored", x.Count())
	}

	// wp-config.php from the backup may hold an older database password.
	if site != nil && site.App == hosting.AppWordPress {
		for _, name := range restoredDBs {
			if d, err := m.store.DatabaseByName(ctx, name); err == nil {
				if err := m.hosting.UpdateDBPassword(ctx, site.ID, d.Name, d.Password); err != nil {
					lg.Logf("Warning: could not update wp-config.php: %v", err)
				}
			}
		}
	}
	lg.Logf("Restore finished")
	return nil
}

// restoreDB empties a database and imports a dump into it. A database that
// no longer exists in the panel is created again (linked to the site),
// with the password the backup recorded, or a new one; one that is no
// longer linked to a site is linked to the site restored.
func (m *Manager) restoreDB(ctx context.Context, name, password string, site *store.Site, r io.Reader, lg Logger) error {
	if err := validate.DBName(name); err != nil {
		return fmt.Errorf("backup contains an invalid database name %q", name)
	}
	if d, err := m.store.DatabaseByName(ctx, name); errors.Is(err, store.ErrNotFound) {
		var siteID int64
		if site != nil {
			siteID = site.ID
		}
		password = usablePassword(password)
		if password != "" {
			lg.Logf("Creating database %s again, with the password of its user from the backup", name)
		} else {
			lg.Logf("Creating database %s again, with a new password (the backup has none): "+
				"if the website's own configuration names the old one, change it there (Databases shows the new one)", name)
		}
		if _, err := m.db.Create(ctx, name, password, siteID); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if err := m.relink(ctx, d, site, lg); err != nil {
		return err
	}
	lg.Logf("Restoring database %s", name)
	if err := m.db.Reset(ctx, name); err != nil {
		return err
	}
	return m.db.Import(ctx, name, r)
}

// relink links a database that belongs to no website to the restored one.
func (m *Manager) relink(ctx context.Context, d *store.Database, site *store.Site, lg Logger) error {
	if site == nil || d.SiteID != 0 {
		return nil
	}
	lg.Logf("Linking database %s to %s again", d.Name, site.Domain)
	d.SiteID = site.ID
	return m.store.UpdateDatabase(ctx, d)
}

// openArchive opens the local file, or downloads it from remote storage when
// it is only kept there (decrypting it if needed).
func (m *Manager) openArchive(ctx context.Context, b *store.Backup, passphrase string, lg Logger) (*os.File, func(), error) {
	if f, err := os.Open(b.File); err == nil {
		return f, func() { f.Close() }, nil
	}
	return m.download(ctx, b, passphrase, lg)
}

// ---- deleting and retention ----

// Delete removes a backup from this server and from remote storage.
func (m *Manager) Delete(ctx context.Context, id int64) error {
	b, err := m.store.BackupByID(ctx, id)
	if err != nil {
		return err
	}
	return m.remove(ctx, b, nil)
}

func (m *Manager) remove(ctx context.Context, b *store.Backup, lg Logger) error {
	// Only ever delete files inside the backup folder.
	if rel, err := filepath.Rel(m.dir, b.File); err == nil && !strings.HasPrefix(rel, "..") {
		if err := os.Remove(b.File); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	m.removeRemote(ctx, b, lg)
	return m.store.DeleteBackup(ctx, b.ID)
}

// Prune deletes a schedule's oldest backups beyond the number it keeps.
func (m *Manager) Prune(ctx context.Context, sc *store.Schedule, lg Logger) error {
	list, err := m.store.BackupsFor(ctx, sc.Kind, sc.Target, sc.ID)
	if err != nil {
		return err
	}
	if sc.Kind == KindSite {
		if site, err := m.siteByDomain(ctx, sc.Target); err == nil {
			if inst := m.hosting.AppNameForSite(ctx, site.ID); inst != "" {
				apps, err := m.store.BackupsFor(ctx, KindApp, inst, sc.ID)
				if err != nil {
					return err
				}
				for i, b := range apps {
					if i >= sc.Keep {
						if err := m.remove(ctx, b, lg); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	for i, b := range list {
		if i < sc.Keep {
			continue
		}
		lg.Logf("Deleting old backup %s", filepath.Base(b.File))
		if err := m.remove(ctx, b, lg); err != nil {
			return err
		}
	}
	return nil
}

// ---- S3 remote storage (SFTP is in sftp.go, Google Drive in gdrive.go) ----

const s3SettingKey = "backup_s3"

// Storage returns the S3 storage settings, without the secret key and the
// encryption passphrases.
func (m *Manager) Storage(ctx context.Context) (*S3Config, error) {
	c, err := m.storageWithSecret(ctx)
	if c != nil {
		c.SecretKey, c.EncryptPassphrase, c.PreviousPassphrases = "", "", nil
	}
	return c, err
}

func (m *Manager) storageWithSecret(ctx context.Context) (*S3Config, error) {
	v, err := m.store.Setting(ctx, s3SettingKey)
	if err != nil || v == "" {
		return nil, err
	}
	var c S3Config
	if err := json.Unmarshal([]byte(v), &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// SaveStorage validates, tests and stores remote storage settings. An empty
// secret key or encryption passphrase keeps the stored one.
func (m *Manager) SaveStorage(ctx context.Context, c S3Config) error {
	old, err := m.storageWithSecret(ctx)
	if err != nil {
		return err
	}
	var oldEnc Encryption
	if old != nil {
		oldEnc = old.Encryption
		if c.SecretKey == "" {
			c.SecretKey = old.SecretKey
		}
	}
	if err := c.Validate(); err != nil {
		return err
	}
	if err := mergeEncryption(&c.Encryption, oldEnc); err != nil {
		return err
	}
	if err := testStorage(ctx, newS3Remote(c), c.Encryption); err != nil {
		return usererr.New("storage_test_failed", err.Error())
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return m.store.SetSetting(ctx, s3SettingKey, string(b))
}

// RemoveStorage forgets the S3 storage settings.
func (m *Manager) RemoveStorage(ctx context.Context) error {
	return m.store.SetSetting(ctx, s3SettingKey, "")
}

// Upload copies an existing backup to the remote storages that do not have it.
func (m *Manager) Upload(ctx context.Context, id int64, lg Logger) error {
	b, err := m.store.BackupByID(ctx, id)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.upload(ctx, b, lg)
}

// ---- schedules ----

// Due reports whether a schedule should run at now: its most recent planned
// time has passed and it has not run since (nor was it created after it).
func Due(sc *store.Schedule, now time.Time) bool {
	planned := time.Date(now.Year(), now.Month(), now.Day(), sc.Hour, 0, 0, 0, now.Location())
	step := 24 * time.Hour
	if sc.Frequency == "weekly" {
		back := (int(now.Weekday()) - sc.Weekday + 7) % 7
		planned = planned.AddDate(0, 0, -back)
		step = 7 * 24 * time.Hour
	}
	if planned.After(now) {
		planned = planned.Add(-step)
	}
	since := sc.LastRunAt
	if sc.CreatedAt.After(since) {
		since = sc.CreatedAt
	}
	return since.Before(planned)
}

// CheckSchedule validates a new schedule.
func (m *Manager) CheckSchedule(ctx context.Context, sc *store.Schedule) error {
	if err := m.CheckTarget(ctx, sc.Kind, sc.Target); err != nil {
		return err
	}
	if sc.Frequency != "daily" && sc.Frequency != "weekly" {
		return usererr.New("invalid_schedule")
	}
	if sc.Hour < 0 || sc.Hour > 23 || sc.Weekday < 0 || sc.Weekday > 6 || sc.Keep < 1 || sc.Keep > 365 {
		return usererr.New("invalid_schedule")
	}
	if sc.Remote {
		if names, err := m.ConfiguredRemotes(ctx); err != nil {
			return err
		} else if len(names) == 0 {
			return usererr.New("storage_not_configured")
		}
	}
	return nil
}
