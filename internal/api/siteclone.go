package api

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/database"
	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/jobs"
	"github.com/nefocp/nefo/internal/siteclone"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
)

// handleSiteClone copies a PHP or static website to a new domain, as a
// job: {"domain": "copy.example.com", "copy_databases": true}. A
// WordPress site is copied like a staging copy, database included; other
// sites by package siteclone. App websites are refused. Mistakes are
// reported before the job starts.
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
	if err := siteclone.Check(site); err != nil {
		s.opError(w, r, err)
		return
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
	if wordpress {
		if !s.db.Status(ctx).Running {
			s.opError(w, r, database.ErrNotInstalled)
			return
		}
		if _, err := s.wpkit().PrepareClone(site); err != nil {
			s.opError(w, r, err)
			return
		}
	} else if req.CopyDatabases {
		if dbs, err = s.store.DatabasesForSite(ctx, site.ID); err != nil {
			s.internalError(w, r, err)
			return
		}
		if len(dbs) > 0 && !s.db.Status(ctx).Running {
			s.opError(w, r, database.ErrNotInstalled)
			return
		}
	}
	user, ip, detail := a.user.Username, clientIP(r), site.Domain+" to "+domain
	job, err := s.jobs.Start("site_clone", domain, 3*time.Hour, func(ctx context.Context, j *jobs.Job) error {
		var err error
		if wordpress {
			j.Logf("%s is a WordPress site: it is copied like a staging copy, with its database", site.Domain)
			var to *store.Site
			if to, err = s.wpkit().Clone(ctx, site, domain, s.wpCloneHost(user, ip), j); err == nil {
				directives := siteclone.ReplaceRoot(site.NginxCustom, s.hosting.Root(site.Domain), s.hosting.Root(domain))
				if serr := s.cloneSettings(ctx, site, to, directives, j); serr != nil {
					j.Logf("Warning: %v", serr)
				}
				j.Logf("Search engines are asked not to index the copy. If it is a website of its own, change that in WordPress under Settings → Reading.")
			}
		} else {
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

// siteCloneHost gives a clone the panel's website and database functions.
func (s *Server) siteCloneHost(user, ip string) siteclone.Host {
	wp := s.wpCloneHost(user, ip)
	return siteclone.Host{
		Root:      s.hosting.Root,
		LookupIDs: hosting.LookupIDs,
		CreateSite: func(ctx context.Context, from *store.Site, domain string) (*store.Site, error) {
			return wp.CreateSite(ctx, domain, from.PHPVersion)
		},
		DeleteSite:   wp.DeleteSite,
		CopySettings: s.cloneSettings,
		CreateDB: func(ctx context.Context, site *store.Site, from *store.Database) (*store.Database, error) {
			prefix := "db"
			if site.App != "" {
				prefix = hosting.AppDBPrefix(site.App)
			}
			return s.createDBWithFreeName(ctx, appDBBase(prefix, site.Domain), site.ID)
		},
		DeleteDB: wp.DeleteDB,
		CopyDB:   s.wpCopyDB,
	}
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
