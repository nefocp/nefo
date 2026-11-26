package api

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/database"
	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/jobs"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/wptoolkit"
)

// Staging copies remember their live site, so they can be pushed back to
// it. The link is a setting per staging site; it names both sites by ID,
// Linux user and creation time, so a site made later with a reused ID is
// never taken for either of them.

func stagingKey(id int64) string { return "wp_staging:" + strconv.FormatInt(id, 10) }

type stagingLink struct {
	LiveID         int64  `json:"live_id"`
	LiveUser       string `json:"live_user"`
	LiveCreated    int64  `json:"live_created"`
	StagingUser    string `json:"staging_user"`
	StagingCreated int64  `json:"staging_created"`
}

// saveStagingLink records that staging (by ID) is a staging copy of live.
func (s *Server) saveStagingLink(ctx context.Context, live *store.Site, stagingID int64) error {
	staging, err := s.store.SiteByID(ctx, stagingID)
	if err != nil {
		return err
	}
	b, err := json.Marshal(stagingLink{
		LiveID: live.ID, LiveUser: live.SysUser, LiveCreated: live.CreatedAt.Unix(),
		StagingUser: staging.SysUser, StagingCreated: staging.CreatedAt.Unix(),
	})
	if err != nil {
		return err
	}
	return s.store.SetSetting(ctx, stagingKey(staging.ID), string(b))
}

// stagingLive returns the live site of a staging copy, or nil when site is
// not a staging copy or its live site is gone.
func (s *Server) stagingLive(ctx context.Context, site *store.Site) (*store.Site, error) {
	v, err := s.store.Setting(ctx, stagingKey(site.ID))
	if err != nil || v == "" {
		return nil, err
	}
	var l stagingLink
	if json.Unmarshal([]byte(v), &l) != nil || l.StagingUser != site.SysUser || l.StagingCreated != site.CreatedAt.Unix() {
		return nil, nil
	}
	live, err := s.store.SiteByID(ctx, l.LiveID)
	if err != nil || live.SysUser != l.LiveUser || live.CreatedAt.Unix() != l.LiveCreated {
		return nil, nil
	}
	return live, nil
}

// forgetStagingLink removes the link of a deleted site.
func (s *Server) forgetStagingLink(ctx context.Context, id int64) {
	if err := s.store.SetSetting(ctx, stagingKey(id), ""); err != nil {
		s.log.Warn("forget the staging link", "site", id, "err", err)
	}
}

func wpSiteJSON(site *store.Site) wpSiteResponse {
	return wpSiteResponse{ID: site.ID, Domain: site.Domain, PHPVersion: site.PHPVersion,
		SSL: site.SSL, Disabled: site.Disabled, URL: siteBaseURL(site)}
}

// handleWPPushInfo tells whether a WordPress site is a staging copy, and
// of which live site.
func (s *Server) handleWPPushInfo(w http.ResponseWriter, r *http.Request, a *authed) {
	site, ok := s.wpSite(w, r)
	if !ok {
		return
	}
	live, err := s.stagingLive(r.Context(), site)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if live == nil || live.App != hosting.AppWordPress {
		writeJSON(w, http.StatusOK, map[string]any{"live": nil})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"live": wpSiteJSON(live)})
}

// handleWPPush copies a staging copy over its live site, as a job:
// {"files": bool, "database": bool, "confirm": "<live domain>"}. The live
// site is backed up first. Mistakes are reported before the job starts.
func (s *Server) handleWPPush(w http.ResponseWriter, r *http.Request, a *authed) {
	site, ok := s.wpSite(w, r)
	if !ok {
		return
	}
	var req struct {
		Files    bool   `json:"files"`
		Database bool   `json:"database"`
		Confirm  string `json:"confirm"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	ctx := r.Context()
	live, err := s.stagingLive(ctx, site)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if live == nil {
		s.opError(w, r, usererr.New("wp_not_staging", site.Domain))
		return
	}
	if live.App != hosting.AppWordPress || live.PHPVersion == "" {
		s.opError(w, r, usererr.New("wp_not_wordpress", live.Domain))
		return
	}
	if strings.TrimSuffix(strings.ToLower(strings.TrimSpace(req.Confirm)), ".") != live.Domain {
		s.opError(w, r, usererr.New("confirm_mismatch"))
		return
	}
	opts := wptoolkit.PushOptions{Files: req.Files, Database: req.Database}
	if opts.Database && !s.db.Status(ctx).Running {
		s.opError(w, r, database.ErrNotInstalled)
		return
	}
	kit := s.wpkit()
	plan, err := kit.PreparePush(site, live, opts)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	// The live database is emptied before the import, which needs it to be
	// one of the panel's databases.
	if opts.Database {
		if _, err := s.store.DatabaseByName(ctx, plan.LiveDB); err != nil {
			s.opError(w, r, usererr.New("wp_push_db_unknown", plan.LiveDB))
			return
		}
	}
	what := map[[2]bool]string{{true, true}: "files and database", {true, false}: "files", {false, true}: "database"}[[2]bool{opts.Files, opts.Database}]
	user, ip, detail := a.user.Username, clientIP(r), site.Domain+" to "+live.Domain+" ("+what+")"
	job, err := s.jobs.Start("wp_push", live.Domain, 3*time.Hour, func(ctx context.Context, j *jobs.Job) error {
		if err := kit.Push(ctx, site, live, opts, s.wpPushHost(), j); err != nil {
			s.audit(ctx, user, "wp_push_failed", ip, detail)
			return err
		}
		s.audit(ctx, user, "wp_pushed", ip, detail)
		// Visitors must see the pushed pages, not the live site's cached ones.
		if cur, err := s.store.SiteByID(ctx, live.ID); err == nil && cur.PageCache {
			if _, err := s.hosting.PurgePageCache(ctx, live.ID); err != nil {
				j.Logf("The page cache of %s could not be purged: %v. Purge it on the website's Page cache tab.", live.Domain, err)
			} else {
				j.Logf("Purged the page cache of %s", live.Domain)
			}
		}
		return nil
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job": job.Snapshot(), "live": wpSiteJSON(live)})
}

// wpPushHost gives a push the panel's backups and databases.
func (s *Server) wpPushHost() wptoolkit.PushHost {
	return wptoolkit.PushHost{
		Backup: func(ctx context.Context, live *store.Site, dbName string, lg wptoolkit.Logger) (string, error) {
			b, err := s.backup.BackupSite(ctx, live.Domain, 0, false, lg)
			if err != nil {
				return "", err
			}
			desc := b.File
			if dbName == "" {
				return desc, nil
			}
			// A site backup holds the databases linked to the site; WordPress
			// may use another one.
			linked, err := s.store.DatabasesForSite(ctx, live.ID)
			if err != nil {
				return "", err
			}
			if !slices.ContainsFunc(linked, func(d *store.Database) bool { return d.Name == dbName }) {
				lg.Logf("Backing up the database %s, which is not linked to %s", dbName, live.Domain)
				db, err := s.backup.BackupDatabase(ctx, dbName, 0, false, lg)
				if err != nil {
					return "", err
				}
				desc += " and " + db.File
			}
			return desc, nil
		},
		ResetDB: s.db.Reset,
		CopyDB:  s.wpCopyDB,
	}
}
