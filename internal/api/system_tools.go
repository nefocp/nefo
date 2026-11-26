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
	"github.com/nefocp/nefo/internal/software"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
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
	if v, ok := strings.CutPrefix(id, "php"); ok && slices.Contains(software.PHPVersions, v) {
		return "PHP " + v, true
	}
	name, ok := map[string]string{
		"mariadb": "MariaDB", "redis": "Redis", "adminer": "Adminer", "nodejs": "Node.js", "python": "Python",
		"docker": "Docker", "waf": "ModSecurity", "clamav": "ClamAV",
	}[id]
	return name, ok
}

func (s *Server) handleSoftware(w http.ResponseWriter, r *http.Request, a *authed) {
	ctx := r.Context()
	items, err := s.software.Catalog(ctx)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	st := s.db.Status(ctx)
	maria := software.Item{ID: "mariadb", Kind: "mariadb", Group: "data", Version: st.Version, Installed: st.Installed,
		RAMMB: database.MemoryMB(), DataDir: "/var/lib/mysql", HasData: softwarePathExists("/var/lib/mysql")}
	if maria.Installed {
		if dbs, err := s.db.UserDatabases(ctx); err == nil {
			maria.Usage = map[string]int{"databases": len(dbs)}
		}
		maria.Block(s.db.CheckRemove(ctx, false, ""))
		maria.Stopped = !st.Running
	}
	adminer := software.Item{ID: "adminer", Kind: "adminer", Group: "data", Version: hosting.AdminerVersion, Installed: s.hosting.AdminerInstalled()}
	if adminer.Installed {
		adminer.Block(nil)
	}
	waf := software.Item{ID: "waf", Kind: "waf", Group: "security", Version: "OWASP CRS 3", Installed: s.hosting.WAFInstalled(), RAMMB: 40}
	waf.Block(nil)
	clam := software.Item{ID: "clamav", Kind: "clamav", Group: "security", Installed: s.hosting.ClamAVInstalled(), RAMMB: 15}
	clam.Block(nil)
	items = append(items, maria, adminer, waf, clam)
	sites, err := s.store.ListSites(ctx)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "supported": s.hosting.Supported(), "sites": len(sites)})
}

func (s *Server) handleSoftwareInstall(w http.ResponseWriter, r *http.Request, a *authed) {
	id, user, ip := r.PathValue("id"), a.user.Username, clientIP(r)
	name, ok := softwareName(id)
	if !ok {
		s.opError(w, r, usererr.New("unknown_software", id))
		return
	}
	if !s.hosting.Supported() {
		s.opError(w, r, hosting.ErrUnsupported)
		return
	}
	job, err := s.jobs.Start("install_software", name, 30*time.Minute, func(ctx context.Context, j *jobs.Job) error {
		var err error
		switch id {
		case "mariadb":
			err = s.db.Install(ctx, j)
		case "adminer":
			err = s.hosting.InstallAdminer(ctx, j)
		case "waf":
			err = s.hosting.InstallWAF(ctx, j)
		case "clamav":
			err = s.hosting.InstallClamAV(ctx, j)
		case "python":
			err = s.software.InstallPython(ctx, j)
		default:
			err = s.software.Install(ctx, id, j)
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
// "confirm": ""}. delete_data (Docker and MariaDB) also deletes the data
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
	case "mariadb":
		check = s.db.CheckRemove(ctx, opts.DeleteData, opts.Confirm)
		do = func(ctx context.Context, j *jobs.Job) error {
			return s.db.Remove(ctx, opts.DeleteData, opts.Confirm, j)
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
