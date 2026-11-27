package api

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/backup"
	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/jobs"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// Deleting a website: {"confirm": "<domain>", "delete_databases": false,
// "backup_first": true}. Without backup_first it is deleted at once (200,
// with the warnings of cleanup steps that failed). With it, a job first
// makes a local backup of the website (files, databases and its Docker
// app's data, named <domain>_before-delete_<time>) and deletes the website
// only if the backup succeeded (202, with the job). Databases are deleted
// only with delete_databases; otherwise they stay on the Databases page,
// linked to no website.

func (s *Server) handleDeleteSite(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := siteID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "site_not_found")
		return
	}
	var req struct {
		Confirm         string `json:"confirm"`
		DeleteDatabases bool   `json:"delete_databases"`
		BackupFirst     bool   `json:"backup_first"`
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
	if strings.TrimSuffix(strings.ToLower(strings.TrimSpace(req.Confirm)), ".") != site.Domain {
		s.opError(w, r, usererr.New("confirm_mismatch"))
		return
	}
	// A deploy writes into the site's folder as its user: let it finish first.
	if s.gitDeployRunning(site.Domain) {
		s.opError(w, r, jobs.ErrBusy)
		return
	}
	user, ip := a.user.Username, clientIP(r)
	if !req.BackupFirst {
		warnings, err := s.deleteSite(ctx, site, req.DeleteDatabases, user, ip)
		if err != nil {
			s.opError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"warnings": warnings})
		return
	}
	if !s.hosting.Supported() {
		s.opError(w, r, hosting.ErrUnsupported)
		return
	}
	if err := s.backup.CheckTarget(ctx, backup.KindSite, site.Domain); err != nil {
		s.opError(w, r, err)
		return
	}
	domain := site.Domain
	job, err := s.jobs.Start("site_delete", domain, 6*time.Hour, func(ctx context.Context, j *jobs.Job) error {
		j.Logf("Backing up %s before it is deleted", domain)
		list, err := s.backup.BackupBeforeDelete(ctx, domain, j)
		if err != nil {
			s.audit(ctx, user, "backup_failed", ip, domain)
			return fmt.Errorf("the backup failed, so %s was not deleted: %w", domain, err)
		}
		s.audit(ctx, user, "backup_created", ip, domain)
		// Read the site again: it may have changed during the backup.
		site, err := s.store.SiteByID(ctx, id)
		if err != nil || site.Domain != domain {
			return fmt.Errorf("%s changed during the backup: it was not deleted", domain)
		}
		for s.gitDeployRunning(domain) {
			j.Logf("Waiting for the Git deploy of %s to finish", domain)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
			}
		}
		j.Logf("Deleting %s", domain)
		warnings, err := s.deleteSite(ctx, site, req.DeleteDatabases, user, ip)
		for _, w := range warnings {
			j.Logf("Warning: %s", w)
		}
		if err != nil {
			return err
		}
		names := make([]string, len(list))
		for i, b := range list {
			names[i] = filepath.Base(b.File)
		}
		j.Logf("%s was deleted. Its backup is kept until you delete it: %s (Backups).", domain, strings.Join(names, ", "))
		j.Logf("To bring the website back, create %s again (Websites, same type), then restore the backup.", domain)
		return nil
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job": job.Snapshot()})
}

// deleteSite removes a website and what belongs to it: its background
// programs, Docker app, SFTP account (hosting.DeleteSite), cron jobs,
// backup schedules, Git deploy settings, trash, staging link, and its
// databases when deleteDBs is set. Monitoring goes with the site's record.
// The website's backups stay.
func (s *Server) deleteSite(ctx context.Context, site *store.Site, deleteDBs bool, user, ip string) ([]string, error) {
	id := site.ID
	// Read the linked databases first: deleting the site unlinks them.
	linked, err := s.store.DatabasesForSite(ctx, id)
	if err != nil {
		return nil, err
	}
	pgLinked, err := s.store.PGDatabasesForSite(ctx, id)
	if err != nil {
		return nil, err
	}
	// Background programs run as the site's user: stop them first, or userdel refuses.
	s.removeSiteDaemons(ctx, site, site.Domain)
	warnings, err := s.hosting.DeleteSite(ctx, id, site.Domain)
	if err != nil {
		return warnings, err
	}
	s.audit(ctx, user, "site_deleted", ip, site.Domain)
	// Its monitoring went with its record: stop watching it now, not at the
	// next minute's reload.
	s.uptime.Reload(ctx)
	s.forgetStagingLink(ctx, id)
	s.gitDeploy().SiteDeleted(ctx, id) // the key went with the site folder
	if err := s.files.DropSiteTrash(site.Domain); err != nil {
		s.log.Warn("remove the site's trash", "site", site.Domain, "err", err)
	}
	s.store.DeleteSchedulesFor(ctx, "site", site.Domain)
	s.store.DeleteSchedulesFor(ctx, "app", site.SysUser) // its Docker app went with it
	// Cron jobs of the site's Linux user cannot run any more.
	if jobs, err := s.store.ListCronJobs(ctx); err == nil {
		for _, c := range jobs {
			if c.RunAs == site.SysUser {
				s.store.DeleteCronJob(ctx, c.ID)
			}
		}
		s.cron.Sync(ctx)
	}
	if !deleteDBs {
		return warnings, nil
	}
	for _, d := range linked {
		if err := s.db.Delete(ctx, d.ID, d.Name); err != nil {
			warnings = append(warnings, fmt.Sprintf("database %s: %v", d.Name, err))
			continue
		}
		s.store.DeleteSchedulesFor(ctx, "database", d.Name)
		s.audit(ctx, user, "db_deleted", ip, d.Name)
	}
	if len(linked) > 0 {
		s.restartDBIfNeeded(ctx, user, ip)
	}
	for _, d := range pgLinked {
		if err := s.pg.Delete(ctx, d.ID, d.Name); err != nil {
			warnings = append(warnings, fmt.Sprintf("PostgreSQL database %s: %v", d.Name, err))
			continue
		}
		s.store.DeleteSchedulesFor(ctx, "database", d.Name)
		s.audit(ctx, user, "pg_db_deleted", ip, d.Name)
	}
	if len(pgLinked) > 0 {
		s.restartPGIfNeeded(ctx, user, ip)
	}
	return warnings, nil
}

// deleteInfoDB is a database of a website about to be deleted.
type deleteInfoDB struct {
	Name   string `json:"name"`
	Engine string `json:"engine"`
}

// handleSiteDeleteInfo lists what deleting a website removes and keeps,
// for the delete dialog.
func (s *Server) handleSiteDeleteInfo(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := siteID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "site_not_found")
		return
	}
	ctx := r.Context()
	site, err := s.store.SiteByID(ctx, id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	out := map[string]any{"domain": site.Domain, "sftp": site.SFTP}
	dbs := []deleteInfoDB{}
	if list, err := s.store.DatabasesForSite(ctx, id); err == nil {
		for _, d := range list {
			dbs = append(dbs, deleteInfoDB{d.Name, store.EngineMySQL})
		}
	}
	if list, err := s.store.PGDatabasesForSite(ctx, id); err == nil {
		for _, d := range list {
			dbs = append(dbs, deleteInfoDB{d.Name, store.EnginePostgres})
		}
	}
	out["databases"] = dbs
	out["docker_app"] = ""
	if name, ok := strings.CutPrefix(site.App, "docker:"); ok {
		out["docker_app"] = name
	}
	cron := 0
	if list, err := s.store.ListCronJobs(ctx); err == nil {
		for _, c := range list {
			if c.RunAs == site.SysUser {
				cron++
			}
		}
	}
	out["cron_jobs"] = cron
	daemons := 0
	if list, err := s.store.DaemonsForSite(ctx, id); err == nil {
		daemons = len(list)
	}
	out["daemons"] = daemons
	mon, err := s.store.SiteMonitor(ctx, id)
	out["monitor"] = err == nil && mon != nil && mon.Enabled
	g, err := s.store.GitDeploy(ctx, id)
	out["git"] = err == nil && g != nil
	schedules := 0
	if list, err := s.store.ListSchedules(ctx); err == nil {
		for _, sc := range list {
			if (sc.Kind == "site" && sc.Target == site.Domain) || (sc.Kind == "app" && sc.Target == site.SysUser) {
				schedules++
			}
		}
	}
	out["schedules"] = schedules
	backups := 0
	if list, err := s.store.ListBackups(ctx); err == nil {
		for _, b := range list {
			if (b.Kind == "site" && b.Target == site.Domain) || (b.Kind == "app" && b.Target == site.SysUser) {
				backups++
			}
		}
	}
	out["backups"] = backups
	// WordPress staging: this site's live site, and its own staging copies.
	out["staging_of"] = ""
	if live, err := s.stagingLive(ctx, site); err == nil && live != nil {
		out["staging_of"] = live.Domain
	}
	copies := []string{}
	if sites, err := s.store.ListSites(ctx); err == nil {
		for _, other := range sites {
			if other.ID == id {
				continue
			}
			if live, err := s.stagingLive(ctx, other); err == nil && live != nil && live.ID == id {
				copies = append(copies, other.Domain)
			}
		}
	}
	out["staging_copies"] = copies
	writeJSON(w, http.StatusOK, out)
}
