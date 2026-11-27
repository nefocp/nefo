package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/daemons"
	"github.com/nefocp/nefo/internal/database"
	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/jobs"
	"github.com/nefocp/nefo/internal/siteclone"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
)

// handleSiteClone copies a website to a new domain, as a job:
// {"domain": "copy.example.com", "copy_databases": true}. A WordPress site
// is copied like a staging copy, database included; a Docker app with its
// data by hosting.CloneDockerApp; other sites (PHP, static, reverse proxy,
// Node.js, Python and other app websites) by package siteclone, with their
// MariaDB, MySQL and PostgreSQL databases if asked. Mistakes are reported
// before the job starts.
func (s *Server) handleSiteClone(w http.ResponseWriter, r *http.Request, a *authed) {
	if !s.hosting.Supported() {
		s.opError(w, r, hosting.ErrUnsupported)
		return
	}
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	var req struct {
		Domain        string `json:"domain"`
		CopyDatabases bool   `json:"copy_databases"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	ctx := r.Context()
	site, err := s.store.SiteByID(ctx, id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	docker := siteclone.IsDocker(site)
	if !docker {
		if err := siteclone.Check(site); err != nil {
			s.opError(w, r, err)
			return
		}
	}
	domain := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(req.Domain)), ".")
	if validate.Domain(domain) != nil {
		s.opError(w, r, usererr.New("invalid_domain", domain))
		return
	}
	sites, err := s.store.ListSites(ctx)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	for _, other := range sites {
		if slices.Contains(other.Names(), domain) {
			s.opError(w, r, usererr.New("domain_exists", domain))
			return
		}
	}
	if root := s.hosting.Root(domain); fileOrDirExists(root) {
		s.opError(w, r, usererr.New("site_dir_exists", root))
		return
	}
	wordpress := site.App == hosting.AppWordPress && site.PHPVersion != ""
	var dbs []*store.Database
	switch {
	case docker:
		if _, _, err := s.hosting.CheckCloneDockerApp(ctx, site, domain); err != nil {
			s.opError(w, r, err)
			return
		}
	case wordpress:
		if !s.db.Status(ctx).Running {
			s.opError(w, r, database.ErrNotInstalled)
			return
		}
		if _, err := s.wpkit().PrepareClone(site); err != nil {
			s.opError(w, r, err)
			return
		}
	case req.CopyDatabases:
		if dbs, err = s.cloneDatabases(ctx, site.ID); err != nil {
			s.opError(w, r, err)
			return
		}
	}
	if siteclone.IsApp(site) && site.AppRuntime != "" {
		// The runtime must still be installed (Software): checked now, not
		// after the files are copied.
		if err := s.hosting.CheckRuntime(site.AppRuntime); err != nil {
			s.opError(w, r, err)
			return
		}
	}
	user, ip, detail := a.user.Username, clientIP(r), site.Domain+" to "+domain
	job, err := s.jobs.Start("site_clone", domain, 3*time.Hour, func(ctx context.Context, j *jobs.Job) error {
		var err error
		switch {
		case docker:
			j.Logf("%s runs a Docker app: the app is copied with its data", site.Domain)
			var to *store.Site
			if to, err = s.hosting.CloneDockerApp(ctx, site, domain, j); err == nil {
				s.audit(ctx, user, "site_created", ip, to.Domain)
			}
		case wordpress:
			j.Logf("%s is a WordPress site: it is copied like a staging copy, with its database", site.Domain)
			var to *store.Site
			if to, err = s.wpkit().Clone(ctx, site, domain, s.wpCloneHost(user, ip), j); err == nil {
				directives := siteclone.ReplaceRoot(site.NginxCustom, s.hosting.Root(site.Domain), s.hosting.Root(domain))
				if serr := s.cloneSettings(ctx, site, to, directives, j); serr != nil {
					j.Logf("Warning: %v", serr)
				}
				j.Logf("Search engines are asked not to index the copy. If it is a website of its own, change that in WordPress under Settings → Reading.")
			}
		default:
			_, err = siteclone.Clone(ctx, site, domain, dbs, s.siteCloneHost(user, ip), j)
		}
		if err != nil {
			s.audit(ctx, user, "site_clone_failed", ip, detail)
			return err
		}
		s.audit(ctx, user, "site_cloned", ip, detail)
		return nil
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job": job.Snapshot(), "domain": domain, "url": "http://" + domain + "/"})
}

// cloneDatabases returns the MariaDB/MySQL and PostgreSQL databases of a
// site, after checking that their database servers run.
func (s *Server) cloneDatabases(ctx context.Context, siteID int64) ([]*store.Database, error) {
	dbs, err := s.store.DatabasesForSite(ctx, siteID)
	if err != nil {
		return nil, err
	}
	if len(dbs) > 0 && !s.db.Status(ctx).Running {
		return nil, database.ErrNotInstalled
	}
	pg, err := s.store.PGDatabasesForSite(ctx, siteID)
	if err != nil {
		return nil, err
	}
	if len(pg) > 0 && !s.pg.Status(ctx).Running {
		return nil, usererr.New("pg_not_running")
	}
	return append(dbs, pg...), nil
}

// siteCloneHost gives a clone the panel's website and database functions.
func (s *Server) siteCloneHost(user, ip string) siteclone.Host {
	wp := s.wpCloneHost(user, ip)
	return siteclone.Host{
		Root:      s.hosting.Root,
		LookupIDs: hosting.LookupIDs,
		CreateSite: func(ctx context.Context, from *store.Site, domain string) (*store.Site, error) {
			req := hosting.CreateRequest{Domain: domain, PHPVersion: from.PHPVersion}
			switch {
			case from.ProxyURL != "":
				req.ProxyURL, req.ProxyOpts = from.ProxyURL, from.ProxyOpts
			case siteclone.IsApp(from):
				// A port of its own; the service comes with StartApp, once
				// the files are there.
				port, err := s.hosting.FreeAppPort(ctx)
				if err != nil {
					return nil, err
				}
				req.ProxyPort = port
			}
			site, err := s.hosting.CreateSite(ctx, req)
			if err == nil {
				s.audit(ctx, user, "site_created", ip, site.Domain)
			}
			return site, err
		},
		DeleteSite: func(ctx context.Context, site *store.Site) error {
			s.removeSiteDaemons(ctx, site, site.Domain)
			return wp.DeleteSite(ctx, site)
		},
		CopySettings: s.cloneSettings,
		StartApp: func(ctx context.Context, from, to *store.Site, command string, lg siteclone.Logger) error {
			_, _, err := s.hosting.UpdateSettings(ctx, to.ID, hosting.Settings{
				Aliases: to.Aliases, PHPMemory: from.PHPMemory, PHPUpload: from.PHPUpload, PHPTime: from.PHPTime,
				ProxyPort: to.ProxyPort, AppCommand: command, AppRuntime: from.AppRuntime,
			})
			return err
		},
		CopyDaemons: func(ctx context.Context, from, to *store.Site, replace func(string) string) (int, error) {
			list, err := s.store.DaemonsForSite(ctx, from.ID)
			if err != nil || len(list) == 0 {
				return 0, err
			}
			off := false
			for i, d := range list {
				env := make([]string, len(d.Env))
				for k, e := range d.Env {
					env[k] = replace(e)
				}
				if _, err := s.daemonsManager().Create(ctx, daemons.Request{Name: d.Name, SiteID: to.ID,
					Command: replace(d.Command), Dir: d.Dir, Env: env, Enabled: &off}); err != nil {
					return i, fmt.Errorf("%s: %w", d.Name, err)
				}
			}
			return len(list), nil
		},
		CreateDB: func(ctx context.Context, site *store.Site, from *store.Database) (*store.Database, error) {
			prefix := "db"
			if site.App != "" && !strings.HasPrefix(site.App, "docker:") {
				prefix = hosting.AppDBPrefix(site.App)
			}
			if from.Engine == store.EnginePostgres {
				return s.createPGWithFreeName(ctx, appDBBase(prefix, site.Domain), site.ID)
			}
			return s.createDBWithFreeName(ctx, appDBBase(prefix, site.Domain), site.ID)
		},
		DeleteDB: func(ctx context.Context, db *store.Database) error {
			if db.Engine == store.EnginePostgres {
				return s.pg.Delete(ctx, db.ID, db.Name)
			}
			return wp.DeleteDB(ctx, db)
		},
		CopyDB: func(ctx context.Context, from, to *store.Database) error {
			if from.Engine == store.EnginePostgres {
				return s.pgCopyDB(ctx, from.Name, to.Name)
			}
			return s.wpCopyDB(ctx, from.Name, to.Name)
		},
	}
}

// createPGWithFreeName creates a PostgreSQL database named base, or base_2…
// when the name is taken.
func (s *Server) createPGWithFreeName(ctx context.Context, base string, siteID int64) (*store.Database, error) {
	for i := 1; i <= 20; i++ {
		name := base
		if i > 1 {
			name = fmt.Sprintf("%s_%d", base, i)
		}
		d, err := s.pg.Create(ctx, name, "", siteID)
		var ue *usererr.Error
		if errors.As(err, &ue) && ue.Code == "db_exists" {
			continue
		}
		return d, err
	}
	return nil, fmt.Errorf("no free database name for %s", base)
}

// pgCopyDB copies a PostgreSQL database into another, through a dump
// without owners or privileges (the copy belongs to its own role).
func (s *Server) pgCopyDB(ctx context.Context, from, to string) error {
	pr, pw := io.Pipe()
	dumped := make(chan error, 1)
	go func() {
		err := s.pg.Dump(ctx, from, pw)
		pw.CloseWithError(err)
		dumped <- err
	}()
	ierr := s.pg.Import(ctx, to, pr)
	pr.CloseWithError(errors.New("the import stopped"))
	derr := <-dumped
	if ierr != nil {
		return fmt.Errorf("import into %s: %w", to, ierr)
	}
	if derr != nil {
		return fmt.Errorf("dump %s: %w", from, derr)
	}
	return nil
}

// cloneSettings gives a copied website the app, PHP settings and rewrite
// preset of the website it copies, and the given own directives. What the
// web server or PHP-FPM refuses is only a warning in the log: the copy
// works without it, and the settings can be changed in its drawer.
func (s *Server) cloneSettings(ctx context.Context, from, to *store.Site, directives string, lg siteclone.Logger) error {
	site, err := s.store.SiteByID(ctx, to.ID)
	if err != nil {
		return err
	}
	// The app sets PHP's max_input_vars and the web firewall's exclusions,
	// which the next two steps write.
	if from.App != "" && site.App != from.App {
		site.App = from.App
		if err := s.store.UpdateSite(ctx, site); err != nil {
			return err
		}
		to.App = from.App
	}
	if from.PHPVersion != "" {
		lg.Logf("Copying the PHP settings")
		set := hosting.Settings{PHPVersion: from.PHPVersion, PHPMemory: from.PHPMemory, PHPUpload: from.PHPUpload,
			PHPTime: from.PHPTime, PHPAllow: strings.Fields(from.PHPAllow)}
		if from.PHPWorkers != 0 && from.PHPPM != "" {
			set.PHPWorkers, set.PHPPM = from.PHPWorkers, from.PHPPM
		}
		if _, _, err := s.hosting.UpdateSettings(ctx, to.ID, set); err != nil {
			lg.Logf("Warning: could not copy the PHP settings: %v", err)
		}
	}
	// A WordPress copy has its preset already.
	rewrite := from.Rewrite
	if rewrite == "" {
		rewrite = site.Rewrite
	}
	if rewrite != site.Rewrite || directives != "" {
		lg.Logf("Copying the rewrite rules and your own directives")
		if _, err := s.hosting.SetNginxConfig(ctx, to.ID, rewrite, directives); err != nil {
			lg.Logf("Warning: could not copy the rewrite rules and directives: %v", err)
		}
	}
	return nil
}
