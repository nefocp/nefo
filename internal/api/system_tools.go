package api

import (
	"context"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/database"
	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/jobs"
	"github.com/nefocp/nefo/internal/postgres"
	"github.com/nefocp/nefo/internal/software"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
)

// cronUsers are the accounts a cron job may run as: root and the sites' users.
func (s *Server) cronUsers(ctx context.Context) (map[string][2]int, error) {
	users := map[string][2]int{"root": {0, 0}}
	sites, err := s.store.ListSites(ctx)
	if err != nil {
		return nil, err
	}
	for _, site := range sites {
		if uid, gid, err := hosting.LookupIDs(site.SysUser); err == nil {
			users[site.SysUser] = [2]int{uid, gid}
		}
	}
	return users, nil
}

type cronResponse struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Schedule string `json:"schedule"`
	Command  string `json:"command"`
	RunAs    string `json:"run_as"`
	Enabled  bool   `json:"enabled"`
	Site     string `json:"site,omitempty"`
}

func (s *Server) handleListCron(w http.ResponseWriter, r *http.Request, a *authed) {
	ctx := r.Context()
	list, err := s.store.ListCronJobs(ctx)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	sites, _ := s.store.ListSites(ctx)
	userSite := map[string]string{}
	users := []map[string]string{{"user": "root", "site": ""}}
	for _, site := range sites {
		userSite[site.SysUser] = site.Domain
		users = append(users, map[string]string{"user": site.SysUser, "site": site.Domain, "php": site.PHPVersion})
	}
	out := make([]cronResponse, 0, len(list))
	for _, c := range list {
		out = append(out, cronResponse{c.ID, c.Name, c.Schedule, c.Command, c.RunAs, c.Enabled, userSite[c.RunAs]})
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": out, "users": users})
}

func (s *Server) handleSaveCron(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Name     string `json:"name"`
		Schedule string `json:"schedule"`
		Command  string `json:"command"`
		RunAs    string `json:"run_as"`
		Enabled  bool   `json:"enabled"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	c := &store.CronJob{Name: req.Name, Schedule: req.Schedule, Command: req.Command, RunAs: req.RunAs, Enabled: req.Enabled}
	if r.PathValue("id") != "" {
		id, ok := pathID(r)
		if !ok {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}
		c.ID = id
	}
	if err := s.cron.Save(r.Context(), c); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "cron_saved", clientIP(r), truncate(c.Name+": "+c.Command, 500))
	writeJSON(w, http.StatusOK, map[string]int64{"id": c.ID})
}

func (s *Server) handleDeleteCron(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err := s.cron.Delete(r.Context(), id); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "cron_deleted", clientIP(r), "")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleRunCron(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	c, err := s.store.CronJobByID(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	user, ip := a.user.Username, clientIP(r)
	job, err := s.jobs.Start("cron_run", c.Name, time.Hour, func(ctx context.Context, j *jobs.Job) error {
		s.audit(ctx, user, "cron_run", ip, c.Name)
		return s.cron.RunNow(ctx, id, j)
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

func (s *Server) handleCronLog(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	text, err := s.cron.Log(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"log": text})
}

func (s *Server) handleListServices(w http.ResponseWriter, r *http.Request, a *authed) {
	writeJSON(w, http.StatusOK, map[string]any{"services": s.services.List(r.Context())})
}

func (s *Server) handleServiceAction(w http.ResponseWriter, r *http.Request, a *authed) {
	name, action := r.PathValue("name"), r.PathValue("action")
	if err := s.services.Do(r.Context(), name, action); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "service_"+action, clientIP(r), name)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleServiceLogs(w http.ResponseWriter, r *http.Request, a *authed) {
	text, err := s.services.Logs(r.Context(), r.PathValue("name"))
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"log": text})
}

// softwareName returns the name a job shows for an item of the Software
// page, and whether the ID is one.
func softwareName(id string) (string, bool) {
	if v, ok := strings.CutPrefix(id, "php"); ok && validate.PHPVersion(v) == nil && software.PHPVersionListed(v) {
		return "PHP " + v, true
	}
	if series, ok := database.ParseMariaDBID(id); ok && series != "" {
		return "MariaDB " + series, true
	}
	name, ok := map[string]string{
		"mariadb": "MariaDB", "redis": "Redis", "adminer": "Adminer", "nodejs": "Node.js", "python": "Python",
		"docker": "Docker", "waf": "ModSecurity", "clamav": "ClamAV",
		"composer": "Composer", "wpcli": "WP-CLI",
		"postgresql": "PostgreSQL", "memcached": "Memcached",
		"mysql": "MySQL", "phpmyadmin": "phpMyAdmin",
		"java": "Java", "ioncube": "ionCube Loader",
	}[id]
	return name, ok
}

func (s *Server) handleSoftware(w http.ResponseWriter, r *http.Request, a *authed) {
	ctx := r.Context()
	items, err := s.softwareItems(ctx)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	sites, err := s.store.ListSites(ctx)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "supported": s.hosting.Supported(), "sites": len(sites)})
}

// softwareItems lists what the Software page offers: the software
// manager's items, then MariaDB, Adminer, the web firewall and ClamAV. The
// setup wizard offers the same.
func (s *Server) softwareItems(ctx context.Context) ([]software.Item, error) {
	items, err := s.software.Catalog(ctx)
	if err != nil {
		return nil, err
	}
	items = append(items, s.toolItems(ctx)...) // Composer and WP-CLI (tools.go)
	st := s.db.Status(ctx)
	maria, series := s.mariadbItems(ctx, st)         // MariaDB and its series from MariaDB's repository (dbengine.go)
	items = append(items, s.dbToolItems(ctx, st)...) // MySQL and phpMyAdmin (dbengine.go)
	items = append(items, series...)
	adminer := software.Item{ID: "adminer", Kind: "adminer", Group: "data", Version: hosting.AdminerVersion, Installed: s.hosting.AdminerInstalled()}
	if adminer.Installed {
		adminer.Block(nil)
	}
	waf := software.Item{ID: "waf", Kind: "waf", Group: "security", Version: "OWASP CRS 3", Installed: s.hosting.WAFInstalled(), RAMMB: 40}
	waf.Block(nil)
	// Not every distribution packages ModSecurity for every web server
	// (Ubuntu 22.04 has none for Nginx); the setup then doesn't offer it
	// either.
	if kind := s.hosting.WebServer(); kind != "" && !waf.Installed && !s.hosting.WAFPackaged(ctx, kind) {
		waf.NotOffered, waf.NotOfferedDetail = "waf_not_packaged", s.hosting.ServerTitle(kind)
	}
	clam := software.Item{ID: "clamav", Kind: "clamav", Group: "security", Installed: s.hosting.ClamAVInstalled(), RAMMB: 15}
	clam.Block(nil)
	return append(items, maria, s.postgresItem(ctx), s.software.MemcachedItem(ctx), adminer, waf, clam), nil
}

// postgresItem is PostgreSQL's item, which can be removed like MariaDB's.
func (s *Server) postgresItem(ctx context.Context) software.Item {
	st := s.pg.Status(ctx)
	it := software.Item{ID: "postgresql", Kind: "postgresql", Group: "data", Version: st.Version, Installed: st.Installed,
		RAMMB: postgres.MemoryMB(), DataDir: s.pg.DataDir(), HasData: s.pg.HasData(), ThirdParty: st.Source == "pgdg"}
	if it.Installed {
		if dbs, err := s.pg.UserDatabases(ctx); err == nil {
			it.Usage = map[string]int{"databases": len(dbs)}
		}
		it.Block(s.pg.CheckRemove(ctx, false, ""))
		it.Stopped = !st.Running
	}
	return it
}

// installSoftware installs an item of the Software page (not the web
// server): the job of POST /api/software/{id}/install, and a step of the
// setup. A new item's install goes here, so the setup can install it too.
func (s *Server) installSoftware(ctx context.Context, id string, lg software.Logger) error {
	if series, ok := database.ParseMariaDBID(id); ok {
		return s.db.InstallMariaDB(ctx, series, lg)
	}
	switch id {
	case "mysql":
		return s.db.InstallMySQL(ctx, lg)
	case "phpmyadmin":
		return s.hosting.InstallPhpMyAdmin(ctx, lg)
	case "adminer":
		if err := s.hosting.InstallAdminer(ctx, lg); err != nil {
			return err
		}
		return s.adminerPostgres(ctx, lg) // postgres.go
	case "postgresql":
		return s.installPostgres(ctx, "", lg)
	case "memcached":
		return s.software.InstallMemcached(ctx, lg)
	case "waf":
		return s.hosting.InstallWAF(ctx, lg)
	case "clamav":
		return s.hosting.InstallClamAV(ctx, lg)
	case "python":
		return s.software.InstallPython(ctx, lg)
	case "wpcli":
		return s.wpkit().InstallCLI(ctx, lg)
	}
	return s.software.Install(ctx, id, lg)
}

func (s *Server) handleSoftwareInstall(w http.ResponseWriter, r *http.Request, a *authed) {
	id, user, ip := r.PathValue("id"), a.user.Username, clientIP(r)
	name, ok := softwareName(id)
	if !ok {
		s.opError(w, r, usererr.New("unknown_software", id))
		return
	}
	// {"accept_eol": true}: an old PHP version is installed only after the
	// administrator confirmed it gets no security updates. {"version":
	// "17"}: PostgreSQL's major version from apt.postgresql.org ("" for the
	// distribution's).
	var req struct {
		AcceptEOL bool   `json:"accept_eol"`
		Version   string `json:"version"`
	}
	if r.ContentLength != 0 && !decodeJSON(w, r, &req) {
		return
	}
	if !s.hosting.Supported() {
		s.opError(w, r, hosting.ErrUnsupported)
		return
	}
	if err := s.software.CheckInstall(r.Context(), id, req.AcceptEOL); err != nil {
		s.opError(w, r, err)
		return
	}
	// The web firewall needs a ModSecurity package for this web server.
	if id == "waf" {
		if kind := s.hosting.WebServer(); kind != "" && !s.hosting.WAFPackaged(r.Context(), kind) {
			s.opError(w, r, usererr.New("waf_not_packaged", s.hosting.ServerTitle(kind)))
			return
		}
	}
	// MariaDB or MySQL: one per server (dbengine.go).
	if err := s.checkDBInstall(r.Context(), id); err != nil {
		s.opError(w, r, err)
		return
	}
	if req.Version != "" {
		if id != "postgresql" {
			writeError(w, http.StatusBadRequest, "bad_request") // only PostgreSQL has versions to choose
			return
		}
		if v := s.pg.Versions(r.Context()); !slices.Contains(v.PGDG, req.Version) && req.Version != v.Distribution {
			if v.PGDGUnavailable != "" {
				s.opError(w, r, usererr.New(v.PGDGUnavailable, v.PGDGDetail))
			} else {
				s.opError(w, r, usererr.New("pg_version_unknown", req.Version))
			}
			return
		}
		name += " " + req.Version
	}
	if id == "postgresql" {
		if err := s.pg.CheckInstall(req.Version); err != nil {
			s.opError(w, r, err)
			return
		}
	}
	job, err := s.jobs.Start("install_software", name, 30*time.Minute, func(ctx context.Context, j *jobs.Job) error {
		var err error
		if id == "postgresql" {
			err = s.installPostgres(ctx, req.Version, j)
		} else {
			err = s.installSoftware(ctx, id, j)
		}
		if err == nil {
			s.audit(ctx, user, "software_installed", ip, id)
		}
		return err
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

// handleSoftwareRemove uninstalls an item: {"delete_data": false,
// "confirm": ""}. delete_data (Docker, MariaDB, MySQL, PostgreSQL) also deletes the data
// folder, and needs confirm to be the item's ID. A removal that a guard
// refuses (websites, containers or databases still use it) is answered
// with the guard's error before any job starts.
func (s *Server) handleSoftwareRemove(w http.ResponseWriter, r *http.Request, a *authed) {
	id, user, ip := r.PathValue("id"), a.user.Username, clientIP(r)
	var opts software.RemoveOptions
	if r.ContentLength != 0 && !decodeJSON(w, r, &opts) {
		return
	}
	name, ok := softwareName(id)
	if !ok {
		s.opError(w, r, usererr.New("unknown_software", id))
		return
	}
	if !s.hosting.Supported() {
		s.opError(w, r, hosting.ErrUnsupported)
		return
	}
	ctx := r.Context()
	var do func(ctx context.Context, j *jobs.Job) error
	var check error
	switch id {
	case "waf":
		if !s.hosting.WAFInstalled() {
			check = usererr.New("software_not_installed", name)
		}
		do = func(ctx context.Context, j *jobs.Job) error { return s.hosting.RemoveWAF(ctx, j) }
	case "clamav":
		if !s.hosting.ClamAVInstalled() {
			check = usererr.New("software_not_installed", name)
		}
		do = func(ctx context.Context, j *jobs.Job) error { return s.hosting.RemoveClamAV(ctx, j) }
	case "adminer":
		if !s.hosting.AdminerInstalled() {
			check = usererr.New("software_not_installed", name)
		}
		do = func(ctx context.Context, j *jobs.Job) error { return s.hosting.RemoveAdminer(ctx, j) }
	case "mariadb", "mysql":
		e, _ := dbEngineOf(id)
		check = s.db.CheckRemoveEngine(ctx, e, opts.DeleteData, opts.Confirm)
		do = func(ctx context.Context, j *jobs.Job) error {
			return s.db.RemoveEngine(ctx, e, opts.DeleteData, opts.Confirm, j)
		}
	case "phpmyadmin":
		if !s.hosting.PhpMyAdminInstalled() {
			check = usererr.New("software_not_installed", name)
		}
		do = func(ctx context.Context, j *jobs.Job) error { return s.hosting.RemovePhpMyAdmin(ctx, j) }
	case "postgresql":
		check = s.pg.CheckRemove(ctx, opts.DeleteData, opts.Confirm)
		do = func(ctx context.Context, j *jobs.Job) error {
			return s.pg.Remove(ctx, opts.DeleteData, opts.Confirm, j)
		}
	case "memcached":
		check = s.software.CheckRemoveMemcached(ctx)
		do = func(ctx context.Context, j *jobs.Job) error { return s.software.RemoveMemcached(ctx, j) }
	case "wpcli":
		check = s.wpcliRemovable(ctx)
		do = func(ctx context.Context, j *jobs.Job) error {
			if err := s.wpcliRemovable(ctx); err != nil {
				return err
			}
			return s.wpkit().RemoveCLI(j)
		}
	default:
		check = s.software.CheckRemove(ctx, id, opts)
		do = func(ctx context.Context, j *jobs.Job) error { return s.software.Remove(ctx, id, opts, j) }
	}
	if check != nil {
		s.opError(w, r, check)
		return
	}
	detail := id
	if opts.DeleteData {
		detail += " (data deleted)"
	}
	job, err := s.jobs.Start("remove_software", name, 30*time.Minute, func(ctx context.Context, j *jobs.Job) error {
		err := do(ctx, j)
		if err == nil {
			s.audit(ctx, user, "software_removed", ip, detail)
		}
		return err
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

func softwarePathExists(p string) bool { _, err := os.Stat(p); return err == nil }
