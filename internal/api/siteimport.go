package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/nefocp/nefo/internal/database"
	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/jobs"
	"github.com/nefocp/nefo/internal/secure"
	"github.com/nefocp/nefo/internal/siteimport"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
)

// Importing websites from cPanel and other panels (package siteimport):
//
//	POST   /api/import             {"kind": "cpanel"|"archive", "archive": {"path"|"url"}, "sql": [{"path"|"url"}]}
//	                                analyses the archive as a job (downloading it first for a URL)
//	GET    /api/import              the imports of the last hours
//	GET    /api/import/{id}         its state, the plan (once analysed) and the result
//	POST   /api/import/{id}/run     a siteimport.Selection: imports as a job
//	POST   /api/import/{id}/stop    stops the running analysis or import, which cleans up after itself
//	DELETE /api/import/{id}         forgets it, with the files the panel downloaded for it
//
// An import's state lives in memory: after a restart the archive is
// analysed again. Downloads go to importDir, on the same disk as the
// websites.

const (
	importDir     = "/var/tmp/nefo-import"
	importKeep    = 6 * time.Hour
	importMaxSQL  = 10
	importReserve = 1 << 30 // left free on the disk after a download
)

type importSource struct {
	Path string `json:"path"`
	URL  string `json:"url"`
}

type importDump struct {
	arch *siteimport.Archive
	dump *siteimport.Dump
}

// importSession is one import, from its analysis to its result.
type importSession struct {
	ID      string
	Kind    string
	Created time.Time

	mu        sync.Mutex
	status    string // analysing, ready, failed, importing, done
	errText   string
	jobID     string
	dir       string // the session's download folder, "" when nothing was downloaded
	arch      *siteimport.Archive
	dumps     []importDump
	cpanel    *siteimport.CPanel
	site      *siteimport.SiteArchive
	result    *siteimport.Result
	runErr    string
	importJob string
}

type importState struct {
	mu sync.Mutex
	m  map[string]*importSession
}

func (st *importState) get(id string) *importSession {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.m[id]
}

func (st *importState) put(s *importSession) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.m == nil {
		st.m = map[string]*importSession{}
	}
	st.m[s.ID] = s
}

// expire forgets old imports that are not running, with their downloads,
// and removes download folders no import knows (left by a restart).
func (st *importState) expire(running func(id string) bool) {
	st.mu.Lock()
	var gone []*importSession
	known := map[string]bool{}
	for id, s := range st.m {
		s.mu.Lock()
		busy := s.status == "analysing" || s.status == "importing"
		s.mu.Unlock()
		if !busy && time.Since(s.Created) > importKeep && !running(s.jobID) && !running(s.importJob) {
			gone = append(gone, s)
			delete(st.m, id)
			continue
		}
		known[id] = true
	}
	st.mu.Unlock()
	for _, s := range gone {
		s.removeDownloads()
	}
	entries, _ := os.ReadDir(importDir)
	for _, e := range entries {
		if info, err := e.Info(); err == nil && !known[e.Name()] && time.Since(info.ModTime()) > 24*time.Hour {
			os.RemoveAll(filepath.Join(importDir, e.Name()))
		}
	}
}

func (s *importSession) removeDownloads() {
	if s.dir != "" && strings.HasPrefix(s.dir, importDir+"/") {
		os.RemoveAll(s.dir)
	}
}

func (s *Server) jobIsRunning(id string) bool {
	if id == "" {
		return false
	}
	j, ok := s.jobs.Get(id)
	if !ok {
		return false
	}
	return j.Snapshot().Status == jobs.StatusRunning
}

// importDeny are the folders archives are never read from, besides
// siteimport.DeniedPaths: the panel's own data.
func (s *Server) importDeny() []string { return []string{s.cfg.DataDir} }

// handleImportStart checks the sources and starts the analysis.
func (s *Server) handleImportStart(w http.ResponseWriter, r *http.Request, a *authed) {
	if !s.hosting.Supported() {
		s.opError(w, r, hosting.ErrUnsupported)
		return
	}
	var req struct {
		Kind    string         `json:"kind"`
		Archive importSource   `json:"archive"`
		SQL     []importSource `json:"sql"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	ctx := r.Context()
	if req.Kind != siteimport.KindCPanel && req.Kind != siteimport.KindArchive {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	if !s.hosting.Status(ctx).Ready {
		s.opError(w, r, hosting.ErrNotReady)
		return
	}
	if req.Kind == siteimport.KindCPanel {
		req.SQL = nil
	}
	if len(req.SQL) > importMaxSQL {
		s.opError(w, r, usererr.New("import_too_many_dumps"))
		return
	}
	// Mistakes are reported now, before a job starts.
	check := func(src importSource, sql bool) error {
		src.Path, src.URL = strings.TrimSpace(src.Path), strings.TrimSpace(src.URL)
		switch {
		case (src.Path == "") == (src.URL == ""):
			return usererr.New("import_source")
		case src.URL != "":
			_, err := s.files.CheckURL(ctx, src.URL)
			return err
		}
		f, _, err := siteimport.Open(src.Path, sql, s.importDeny()...)
		if err == nil {
			f.Close()
		}
		return err
	}
	if err := check(req.Archive, false); err != nil {
		s.opError(w, r, err)
		return
	}
	for _, src := range req.SQL {
		if err := check(src, true); err != nil {
			s.opError(w, r, err)
			return
		}
	}
	s.imports.expire(s.jobIsRunning)
	sess := &importSession{ID: secure.String(16, secure.LowerAlnum), Kind: req.Kind, Created: time.Now(), status: "analysing"}
	target := strings.TrimSpace(req.Archive.Path)
	if target == "" {
		target = strings.TrimSpace(req.Archive.URL)
	}
	user, ip := a.user.Username, clientIP(r)
	job, err := s.jobs.Start("import_analyse", filepath.Base(target), 3*time.Hour, func(ctx context.Context, j *jobs.Job) error {
		err := s.importAnalyse(ctx, sess, req.Archive, req.SQL, j)
		sess.mu.Lock()
		defer sess.mu.Unlock()
		if err != nil {
			if errors.Is(err, context.Canceled) {
				err = errors.New("the analysis was stopped")
			}
			sess.status, sess.errText = "failed", err.Error()
			sess.removeDownloads()
			return err
		}
		sess.status = "ready"
		return nil
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	sess.jobID = job.ID()
	s.imports.put(sess)
	s.audit(ctx, user, "import_analysed", ip, truncate(target, 300))
	writeJSON(w, http.StatusAccepted, map[string]any{"id": sess.ID, "job": job.Snapshot()})
}

// importAnalyse downloads what has a URL and reads the archive and dumps.
func (s *Server) importAnalyse(ctx context.Context, sess *importSession, arch importSource, sql []importSource, j *jobs.Job) error {
	fetch := func(src importSource, sql bool) (string, error) {
		if strings.TrimSpace(src.URL) == "" {
			return strings.TrimSpace(src.Path), nil
		}
		u, err := s.files.CheckURL(ctx, src.URL)
		if err != nil {
			return "", err
		}
		if sess.dir == "" {
			if err := os.MkdirAll(importDir, 0o700); err != nil {
				return "", err
			}
			os.Chmod(importDir, 0o700)
			sess.dir = filepath.Join(importDir, sess.ID)
			if err := os.Mkdir(sess.dir, 0o700); err != nil {
				return "", err
			}
		}
		limit := int64(200 << 30)
		if free, _, err := siteimport.DiskFree(sess.dir); err == nil {
			limit = min(limit, free-importReserve)
			if limit <= 0 {
				return "", fmt.Errorf("the disk is full: %s free", formatBytes(free))
			}
		}
		return s.files.FetchInto(ctx, sess.dir, u, limit, j)
	}
	p, err := fetch(arch, false)
	if err != nil {
		return err
	}
	f, a, err := siteimport.Open(p, false, s.importDeny()...)
	if err != nil {
		return importErr(err)
	}
	defer f.Close()
	j.Logf("Reading %s (%s, %s)", a.Name, formatBytes(a.Size), a.Format)
	sess.mu.Lock()
	sess.arch = a
	sess.mu.Unlock()
	if sess.Kind == siteimport.KindCPanel {
		c, err := siteimport.AnalyseCPanel(ctx, f, a, siteimport.DefaultLimits, time.Now(), j)
		if err != nil {
			return importErr(err)
		}
		sess.mu.Lock()
		sess.cpanel = c
		sess.mu.Unlock()
		return nil
	}
	site, err := siteimport.AnalyseSite(ctx, f, a, siteimport.DefaultLimits, j)
	if err != nil {
		return importErr(err)
	}
	var dumps []importDump
	for _, src := range sql {
		p, err := fetch(src, true)
		if err != nil {
			return err
		}
		df, da, err := siteimport.Open(p, true, s.importDeny()...)
		if err != nil {
			return importErr(err)
		}
		d, err := siteimport.AnalyseDump(df, da)
		df.Close()
		if err != nil {
			return importErr(err)
		}
		j.Logf("Read the dump %s (%s)%s", da.Name, formatBytes(da.Size), map[bool]string{true: ", database " + d.DB, false: ""}[d.DB != ""])
		dumps = append(dumps, importDump{arch: da, dump: d})
	}
	sess.mu.Lock()
	sess.site, sess.dumps = site, dumps
	sess.mu.Unlock()
	return nil
}

// importErr turns a user error into a sentence for the job log.
func importErr(err error) error {
	var ue *usererr.Error
	if !errors.As(err, &ue) {
		return err
	}
	msg := map[string]string{
		"import_not_cpanel":     "this is not a cPanel full backup (it has no userdata/main): for an archive of a website's files, choose “Website archive”",
		"import_unknown_format": "this is not a .tar.gz, .tar or .zip archive (or an .sql or .sql.gz dump)",
		"import_empty":          "the file is empty",
		"invalid_sql_file":      "this is not an SQL dump",
		"import_not_file":       "this is not a regular file",
		"import_path_denied":    "the panel does not read archives from that folder",
		"path_not_found":        "the file does not exist",
	}[ue.Code]
	if msg == "" {
		return err
	}
	if ue.Detail != "" {
		return fmt.Errorf("%s: %s", ue.Detail, msg)
	}
	return errors.New(msg)
}

func formatBytes(n int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	v := float64(n)
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if i == 0 || v >= 100 {
		return fmt.Sprintf("%.0f %s", v, units[i])
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}

// importEnv is what a plan needs to know about this server.
func (s *Server) importEnv(ctx context.Context) siteimport.Env {
	st := s.hosting.Status(ctx)
	env := siteimport.Env{DefaultPHP: st.DefaultPHP, Now: time.Now(), Root: s.hosting.Root, Free: -1, FreeDB: -1}
	for _, p := range st.PHP {
		env.PHP = append(env.PHP, p.Version)
	}
	names := map[string]bool{}
	if sites, err := s.store.ListSites(ctx); err == nil {
		for _, site := range sites {
			for _, n := range site.Names() {
				names[n] = true
			}
		}
	}
	env.DomainTaken = func(n string) bool { return names[n] }
	env.DirExists = func(d string) bool { return fileOrDirExists(s.hosting.Root(d)) }
	env.DBReady = s.db.Status(ctx).Running
	// MySQL 8.4 cannot take cPanel's mysql_native_password hashes.
	env.NoHashes = !s.db.AcceptsNativeHash(ctx)
	taken, err := s.db.TakenNames(ctx)
	env.DBNameFree = func(n string) bool { return err == nil && !taken[n] }
	www := filepath.Dir(s.hosting.Root("example.com"))
	if free, dev, err := siteimport.DiskFree(www); err == nil {
		env.Free = free
		dbDir := "/var/lib/mysql"
		if !fileOrDirExists(dbDir) {
			dbDir = www
		}
		if freeDB, devDB, err := siteimport.DiskFree(dbDir); err == nil {
			env.FreeDB, env.SameDisk = freeDB, devDB == dev
		}
	}
	return env
}

// importSpace refuses an import that would fill the disk.
func (s *Server) importSpace(files, dbs int64) error {
	const margin = 512 << 20
	www := filepath.Dir(s.hosting.Root("example.com"))
	free, dev, err := siteimport.DiskFree(www)
	if err != nil {
		return nil // not known: go on
	}
	need := files + margin
	if freeDB, devDB, err := siteimport.DiskFree("/var/lib/mysql"); err == nil && devDB != dev {
		if freeDB < dbs+margin {
			return fmt.Errorf("not enough free disk space for the databases: they need about %s, %s is free", formatBytes(dbs+margin), formatBytes(freeDB))
		}
	} else {
		need += dbs
	}
	if free < need {
		return fmt.Errorf("not enough free disk space: the import needs about %s, %s is free", formatBytes(need), formatBytes(free))
	}
	return nil
}

func (sess *importSession) plan(env siteimport.Env) *siteimport.Plan {
	switch {
	case sess.cpanel != nil:
		return sess.cpanel.Plan(env)
	case sess.site != nil:
		var dumps []*siteimport.Dump
		for _, d := range sess.dumps {
			dumps = append(dumps, d.dump)
		}
		return sess.site.Plan(env, dumps)
	}
	return nil
}

type importResponse struct {
	ID        string             `json:"id"`
	Kind      string             `json:"kind"`
	Status    string             `json:"status"`
	Error     string             `json:"error,omitempty"`
	RunError  string             `json:"run_error,omitempty"`
	Name      string             `json:"name,omitempty"`
	Created   time.Time          `json:"created"`
	JobID     string             `json:"job_id"`
	ImportJob string             `json:"import_job_id,omitempty"`
	Plan      *siteimport.Plan   `json:"plan,omitempty"`
	Result    *siteimport.Result `json:"result,omitempty"`
}

func (sess *importSession) response(plan *siteimport.Plan) importResponse {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	r := importResponse{ID: sess.ID, Kind: sess.Kind, Status: sess.status, Error: sess.errText, RunError: sess.runErr,
		Created: sess.Created, JobID: sess.jobID, ImportJob: sess.importJob, Plan: plan, Result: sess.result}
	if sess.arch != nil {
		r.Name = sess.arch.Name
	}
	return r
}

func (s *Server) handleImportList(w http.ResponseWriter, r *http.Request, a *authed) {
	s.imports.mu.Lock()
	list := make([]*importSession, 0, len(s.imports.m))
	for _, sess := range s.imports.m {
		list = append(list, sess)
	}
	s.imports.mu.Unlock()
	slices.SortFunc(list, func(x, y *importSession) int { return y.Created.Compare(x.Created) })
	out := []importResponse{}
	for _, sess := range list {
		out = append(out, sess.response(nil))
	}
	writeJSON(w, http.StatusOK, map[string]any{"imports": out})
}

func (s *Server) importSessionOf(w http.ResponseWriter, r *http.Request) *importSession {
	sess := s.imports.get(r.PathValue("id"))
	if sess == nil {
		writeError(w, http.StatusNotFound, "import_not_found")
	}
	return sess
}

func (s *Server) handleImportGet(w http.ResponseWriter, r *http.Request, a *authed) {
	sess := s.importSessionOf(w, r)
	if sess == nil {
		return
	}
	sess.mu.Lock()
	ready := sess.status == "ready" || sess.status == "importing" || sess.status == "done"
	sess.mu.Unlock()
	var plan *siteimport.Plan
	if ready {
		plan = sess.plan(s.importEnv(r.Context()))
	}
	writeJSON(w, http.StatusOK, sess.response(plan))
}

func (s *Server) handleImportStop(w http.ResponseWriter, r *http.Request, a *authed) {
	sess := s.importSessionOf(w, r)
	if sess == nil {
		return
	}
	sess.mu.Lock()
	ids := []string{sess.jobID, sess.importJob}
	sess.mu.Unlock()
	stopped := false
	for _, id := range ids {
		if id != "" && s.jobs.Cancel(id) {
			stopped = true
		}
	}
	if stopped {
		s.audit(r.Context(), a.user.Username, "import_stopped", clientIP(r), sess.ID)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"stopped": stopped})
}

func (s *Server) handleImportDelete(w http.ResponseWriter, r *http.Request, a *authed) {
	sess := s.importSessionOf(w, r)
	if sess == nil {
		return
	}
	sess.mu.Lock()
	busy := sess.status == "analysing" || sess.status == "importing"
	sess.mu.Unlock()
	if busy {
		s.opError(w, r, jobs.ErrBusy)
		return
	}
	s.imports.mu.Lock()
	delete(s.imports.m, sess.ID)
	s.imports.mu.Unlock()
	sess.removeDownloads()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// placeholderText is in the welcome page of a new website (see
// hosting.renderWelcome): a public folder with only that page is empty.
const placeholderText = "This placeholder page was created by the server's control panel."

// emptySite tells whether a website's public folder holds nothing but its
// welcome page.
func (s *Server) emptySite(site *store.Site) bool {
	return emptyPublic(filepath.Join(s.hosting.Root(site.Domain), "public"))
}

// emptyPublic tells whether a public folder holds nothing but the welcome
// page.
func emptyPublic(public string) bool {
	root, err := os.OpenRoot(public)
	if err != nil {
		return false
	}
	defer root.Close()
	d, err := root.Open(".")
	if err != nil {
		return false
	}
	names, err := d.Readdirnames(100)
	d.Close()
	if err != nil && !errors.Is(err, io.EOF) {
		return false
	}
	for _, n := range names {
		if n != "index.html" {
			return false
		}
		fi, err := root.Lstat(n)
		if err != nil || !fi.Mode().IsRegular() || fi.Size() > 64<<10 {
			return false
		}
		b, err := root.ReadFile(n)
		if err != nil || !strings.Contains(string(b), placeholderText) {
			return false
		}
	}
	return true
}

// handleImportRun checks the selection and starts the import.
func (s *Server) handleImportRun(w http.ResponseWriter, r *http.Request, a *authed) {
	sess := s.importSessionOf(w, r)
	if sess == nil {
		return
	}
	var sel siteimport.Selection
	if !decodeJSON(w, r, &sel) {
		return
	}
	ctx := r.Context()
	sess.mu.Lock()
	status := sess.status
	sess.mu.Unlock()
	if status != "ready" {
		s.opError(w, r, usererr.New("import_not_ready", status))
		return
	}
	env := s.importEnv(ctx)
	plan := sess.plan(env)
	if err := plan.Check(sel); err != nil {
		s.opError(w, r, err)
		return
	}
	// The archive must still be the file that was analysed.
	same := func(a *siteimport.Archive, sql bool) error {
		f, b, err := siteimport.Open(a.Path, sql, s.importDeny()...)
		if err != nil {
			return err
		}
		f.Close()
		if !a.Same(b) {
			return usererr.New("import_changed", a.Name)
		}
		return nil
	}
	if err := same(sess.arch, false); err != nil {
		s.opError(w, r, err)
		return
	}
	var target *store.Site
	if plan.Kind == siteimport.KindArchive {
		for _, d := range sess.dumps {
			if slices.Contains(sel.Databases, d.arch.Path) {
				if err := same(d.arch, true); err != nil {
					s.opError(w, r, err)
					return
				}
			}
		}
		needsPHP := plan.Site.App != "static"
		if sel.SiteID != 0 {
			site, err := s.store.SiteByID(ctx, sel.SiteID)
			if err != nil {
				s.opError(w, r, err)
				return
			}
			switch {
			case site.ProxyPort != 0 || strings.HasPrefix(site.App, "docker:") || site.RedirectURL != "":
				s.opError(w, r, usererr.New("import_site_kind", site.Domain))
				return
			case needsPHP && site.PHPVersion == "":
				s.opError(w, r, usererr.New("import_needs_php", site.Domain))
				return
			case !s.emptySite(site):
				s.opError(w, r, usererr.New("import_site_not_empty", site.Domain))
				return
			}
			target = site
		} else {
			domain := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(sel.Domain)), ".")
			sel.Domain = domain
			names := []string{domain}
			if sel.WWW && !strings.HasPrefix(domain, "www.") {
				names = append(names, "www."+domain)
			}
			for _, n := range names {
				if env.DomainTaken(n) {
					s.opError(w, r, usererr.New("domain_exists", n))
					return
				}
			}
			if env.DirExists(domain) {
				s.opError(w, r, usererr.New("site_dir_exists", s.hosting.Root(domain)))
				return
			}
			if sel.PHP != "" && (validate.PHPVersion(sel.PHP) != nil || !slices.Contains(env.PHP, sel.PHP)) {
				s.opError(w, r, usererr.New("php_not_installed", sel.PHP))
				return
			}
			if needsPHP && sel.PHP == "" {
				s.opError(w, r, usererr.New("import_needs_php", domain))
				return
			}
		}
	}
	// Claim the import: a second request finds it busy.
	sess.mu.Lock()
	if sess.status != "ready" {
		status := sess.status
		sess.mu.Unlock()
		s.opError(w, r, usererr.New("import_not_ready", status))
		return
	}
	sess.status, sess.runErr = "importing", ""
	sess.mu.Unlock()
	user, ip := a.user.Username, clientIP(r)
	name := sess.arch.Name
	job, err := s.jobs.Start("import_run", name, 12*time.Hour, func(ctx context.Context, j *jobs.Job) error {
		res, err := s.importRun(ctx, sess, plan, sel, target, s.importHost(user, ip), j)
		sess.mu.Lock()
		defer sess.mu.Unlock()
		if err != nil {
			sess.status, sess.runErr = "ready", err.Error()
			s.audit(context.WithoutCancel(ctx), user, "import_failed", ip, truncate(name, 300))
			return err
		}
		sess.status, sess.result, sess.runErr = "done", res, ""
		var doms []string
		for _, site := range res.Sites {
			doms = append(doms, site.Domain)
		}
		s.audit(ctx, user, "site_imported", ip, truncate(name+": "+strings.Join(doms, ", "), 500))
		sess.removeDownloads()
		return nil
	})
	if err != nil {
		sess.mu.Lock()
		sess.status = "ready"
		sess.mu.Unlock()
		s.opError(w, r, err)
		return
	}
	sess.mu.Lock()
	sess.importJob = job.ID()
	sess.mu.Unlock()
	writeJSON(w, http.StatusAccepted, map[string]any{"job": job.Snapshot()})
}

func (s *Server) importRun(ctx context.Context, sess *importSession, plan *siteimport.Plan, sel siteimport.Selection,
	target *store.Site, h siteimport.Host, j *jobs.Job) (*siteimport.Result, error) {
	f, a, err := siteimport.Open(sess.arch.Path, false, s.importDeny()...)
	if err != nil {
		return nil, importErr(err)
	}
	defer f.Close()
	if !sess.arch.Same(a) {
		return nil, errors.New("the archive changed since it was analysed: analyse it again")
	}
	if sess.cpanel != nil {
		return siteimport.RunCPanel(ctx, f, sess.cpanel, plan, sel, h, siteimport.DefaultLimits, j)
	}
	dumps := map[string]*os.File{}
	defer func() {
		for _, df := range dumps {
			df.Close()
		}
	}()
	for _, d := range sess.dumps {
		if !slices.Contains(sel.Databases, d.arch.Path) {
			continue
		}
		df, da, err := siteimport.Open(d.arch.Path, true, s.importDeny()...)
		if err != nil {
			return nil, importErr(err)
		}
		dumps[d.arch.Path] = df
		if !d.arch.Same(da) {
			return nil, fmt.Errorf("%s changed since it was analysed: analyse again", da.Name)
		}
	}
	return siteimport.RunSite(ctx, f, sess.site, plan, dumps, sel, target, h, siteimport.DefaultLimits, j)
}

// importHost gives an import the panel's websites, databases,
// certificates and cron jobs.
func (s *Server) importHost(user, ip string) siteimport.Host {
	return siteimport.Host{
		Root:      s.hosting.Root,
		LookupIDs: hosting.LookupIDs,
		CreateSite: func(ctx context.Context, domain string, aliases []string, php string) (*store.Site, error) {
			site, err := s.hosting.CreateSite(ctx, hosting.CreateRequest{Domain: domain, Aliases: aliases, PHPVersion: php, Suspended: true})
			if err == nil {
				s.audit(ctx, user, "site_created", ip, site.Domain)
			}
			return site, err
		},
		DeleteSite: func(ctx context.Context, site *store.Site) error {
			if _, err := s.hosting.DeleteSite(ctx, site.ID, site.Domain); err != nil {
				return err
			}
			s.audit(ctx, user, "site_deleted", ip, site.Domain)
			if list, err := s.store.ListCronJobs(ctx); err == nil {
				for _, c := range list {
					if c.RunAs == site.SysUser {
						s.store.DeleteCronJob(ctx, c.ID)
					}
				}
				s.cron.Sync(ctx)
			}
			return nil
		},
		Suspend: func(ctx context.Context, site *store.Site, on bool) error {
			cur, err := s.store.SiteByID(ctx, site.ID)
			if err != nil {
				return err
			}
			if cur.Disabled == on {
				return nil
			}
			_, err = s.hosting.SetAccess(ctx, site.ID, hosting.Access{Disabled: on, RedirectURL: cur.RedirectURL, AuthUser: cur.AuthUser})
			return err
		},
		SetNote: func(ctx context.Context, site *store.Site, note string) error {
			clean, ok := cleanSiteNote(note)
			if !ok {
				clean, _ = cleanSiteNote(truncate(strings.ToValidUTF8(note, ""), maxSiteNote))
			}
			return s.store.SetSiteNote(ctx, site.ID, clean)
		},
		MarkWordPress: func(ctx context.Context, site *store.Site, lg siteimport.Logger) error {
			cur, err := s.store.SiteByID(ctx, site.ID)
			if err != nil {
				return err
			}
			cur.App = hosting.AppWordPress
			if err := s.store.UpdateSite(ctx, cur); err != nil {
				return err
			}
			rewrite := cur.Rewrite
			if rewrite == "" {
				rewrite = hosting.AppWordPress
			}
			lg.Logf("Applying the %s rewrite rules", rewrite)
			if _, err := s.hosting.SetNginxConfig(ctx, site.ID, rewrite, cur.NginxCustom); err != nil {
				lg.Logf("Warning: could not apply the rewrite rules: %v", err)
			}
			return nil
		},
		CreateDB: s.db.CreateWithUser,
		DeleteDB: func(ctx context.Context, d *store.Database) error {
			return s.db.Delete(ctx, d.ID, d.Name)
		},
		ImportDB: s.db.Import,
		ReplaceInDB: func(ctx context.Context, site *store.Site, from, to string, lg siteimport.Logger) error {
			cur, err := s.store.SiteByID(ctx, site.ID)
			if err != nil {
				return err
			}
			return s.wpkit().ReplaceInDB(ctx, cur, from, to, logWriter{lg})
		},
		InstallCert: func(ctx context.Context, site *store.Site, chain, key []byte) error {
			_, _, err := s.hosting.InstallCertificate(ctx, site.ID, string(chain), string(key))
			return err
		},
		AddCron: func(ctx context.Context, site *store.Site, name, schedule, command string) error {
			return s.cron.Save(ctx, &store.CronJob{Name: name, Schedule: schedule, Command: command, RunAs: site.SysUser, Enabled: true})
		},
		CheckSpace: s.importSpace,
		MySQL:      s.db.Engine(context.Background()) == database.MySQL,
	}
}

// logWriter makes a siteimport.Logger (a job) usable where wp-cli's
// output is streamed.
type logWriter struct{ siteimport.Logger }

func (l logWriter) Write(p []byte) (int, error) {
	if w, ok := l.Logger.(io.Writer); ok {
		return w.Write(p)
	}
	return len(p), nil
}
