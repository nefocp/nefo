package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/database"
	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/jobs"
	"github.com/nefocp/nefo/internal/setup"
	"github.com/nefocp/nefo/internal/software"
	"github.com/nefocp/nefo/internal/usererr"
)

// The setup wizard (/setup in the UI, "nefo setup" on the command line):
// the web server, PHP versions, MariaDB and extras installed one after
// another as one job, with the Software page's own install functions.

// setupTimeout bounds the whole chain; each install takes minutes.
const setupTimeout = 2 * time.Hour

// setupHooks replace the server's facts and its installs in tests.
type setupHooks struct {
	env       func(ctx context.Context) (setup.Env, error)
	installer func(user, ip string) setup.Installer
}

// setupEnv returns what the setup offers on this server now.
func (s *Server) setupEnv(ctx context.Context) (setup.Env, error) {
	if s.setupTest != nil && s.setupTest.env != nil {
		return s.setupTest.env(ctx)
	}
	f := setup.Facts{
		Supported: s.hosting.Supported(),
		Installed: map[string]bool{},
		Blocked:   map[string]setup.Reason{},
		RAM:       map[string]int{},
		RAMMax:    map[string]bool{},
		Versions:  map[string]string{},
		Until:     map[string]string{},
	}
	info := s.sys.Info()
	f.Server = setup.Server{CPUs: info.CPUs, OS: info.OS, Arch: info.Arch}
	if st, err := s.sys.Stats(); err == nil && st.Supported {
		f.Server.MemTotalMB, f.Server.MemUsedMB = int(st.MemTotal>>20), int(st.MemUsed>>20)
		f.Server.DiskTotalMB, f.Server.DiskFreeMB = int(st.DiskTotal>>20), int((st.DiskTotal-st.DiskUsed)>>20)
	}
	if !f.Supported {
		return setup.NewEnv(f), nil
	}
	items, err := s.softwareItems(ctx)
	if err != nil {
		return setup.Env{}, err
	}
	f.EOL = map[string]bool{}
	dbInstalled := false
	for _, it := range items {
		dbInstalled = dbInstalled || (setup.IsDatabase(it.ID) && it.Installed)
	}
	for _, it := range items {
		// What the Software page can't install on this server is not offered.
		if it.NotOffered != "" && !it.Installed {
			continue
		}
		if setup.IsDatabase(it.ID) {
			// Once a database server is installed, the setup only shows it.
			if dbInstalled && it.ID != "mariadb" && it.ID != "mysql" {
				continue
			}
			f.Versions[it.ID], f.Until[it.ID] = it.Version, it.SecurityUntil
		}
		if it.Kind == "php" {
			f.PHPVersions = append(f.PHPVersions, it.Version)
			f.EOL[it.ID] = it.EOL
		}
		// Databases and extras are offered only when the Software page lists
		// them (MariaDB or MySQL, PostgreSQL, Memcached…).
		f.Catalog = append(f.Catalog, it.ID)
		f.Installed[it.ID] = it.Installed
		if it.RAMMB > 0 {
			f.RAM[it.ID] = it.RAMMB
		}
		f.RAMMax[it.ID] = it.RAMMax
	}
	// The web firewall's figure here is the measured one (see setup).
	delete(f.RAM, "waf")
	web := s.hosting.Status(ctx)
	f.WebServer, f.DistroPHP = web.Server, web.DefaultPHP
	f.PHPRepo, f.OLS = software.PHPRepository(), hosting.OLSRelease()
	f.OSID, f.Codename = software.OSRelease()
	f.AptListsMissing = software.AptListsEmpty()
	for _, kind := range setup.WebServers {
		f.Installed[kind] = web.Server == kind && web.Ready
		// One web server per server: CheckInstall says which may be installed.
		if _, err := s.hosting.CheckInstall(ctx, kind); err != nil {
			f.Blocked[kind] = setupReason(err)
		}
	}
	return setup.NewEnv(f), nil
}

// setupReason turns a refusal into the code and detail the UI translates.
func setupReason(err error) setup.Reason {
	var ce *hosting.ConflictError
	var ue *usererr.Error
	switch {
	case errors.As(err, &ce):
		return setup.Reason{Code: ce.Code, Detail: ce.Detail}
	case errors.As(err, &ue):
		return setup.Reason{Code: ue.Code, Detail: ue.Detail}
	case errors.Is(err, hosting.ErrUnsupported):
		return setup.Reason{Code: "unsupported"}
	}
	return setup.Reason{Code: "operation_failed", Detail: err.Error()}
}

// SetupView returns what GET /api/setup answers.
func (s *Server) SetupView(ctx context.Context) (setup.View, error) {
	env, err := s.setupEnv(ctx)
	if err != nil {
		return setup.View{}, err
	}
	state, err := s.store.Setting(ctx, setup.StateKey)
	if err != nil {
		return setup.View{}, err
	}
	sites, err := s.store.ListSites(ctx)
	if err != nil {
		return setup.View{}, err
	}
	return setup.NewView(env, state, len(sites)), nil
}

// setupOffered is setup.Offered for GET /api/features, which the UI asks
// on every page change: the cheap checks come first, and nothing runs.
func (s *Server) setupOffered(ctx context.Context, sites int) bool {
	supported, web := s.hosting.Supported, s.hosting.WebServer
	if s.setupTest != nil && s.setupTest.env != nil {
		env, err := s.setupTest.env(ctx)
		if err != nil {
			return false
		}
		supported, web = func() bool { return env.Supported }, func() string { return env.WebServer }
	}
	if sites > 0 || web() != "" {
		return false
	}
	state, err := s.store.Setting(ctx, setup.StateKey)
	if err != nil || state != "" {
		return false
	}
	return setup.Offered(supported(), "", state, sites)
}

// planSetup checks a choice against the server as it is now.
func (s *Server) planSetup(ctx context.Context, c setup.Choice) (setup.Plan, error) {
	env, err := s.setupEnv(ctx)
	if err != nil {
		return setup.Plan{}, err
	}
	if !env.Supported {
		return setup.Plan{}, hosting.ErrUnsupported
	}
	return env.Plan(c)
}

// startSetup starts the setup as one job.
func (s *Server) startSetup(ctx context.Context, c setup.Choice, user, ip string) (*jobs.Job, setup.Plan, error) {
	plan, err := s.planSetup(ctx, c)
	if err != nil {
		return nil, plan, err
	}
	job, err := s.jobs.Start("setup", plan.Target(), setupTimeout, func(ctx context.Context, j *jobs.Job) error {
		return s.runSetup(ctx, plan, j, user, ip)
	})
	return job, plan, err
}

// RunSetup runs the setup in this process and waits for it: "nefo setup"
// does this when the panel is not running.
func (s *Server) RunSetup(ctx context.Context, c setup.Choice, lg setup.Logger) error {
	plan, err := s.planSetup(ctx, c)
	if err != nil {
		return err
	}
	return s.runSetup(ctx, plan, lg, "root", "cli")
}

// runSetup runs the plan's steps and, when they all are installed, marks
// the wizard done.
func (s *Server) runSetup(ctx context.Context, plan setup.Plan, lg setup.Logger, user, ip string) error {
	var in setup.Installer = setupInstaller{s: s, user: user, ip: ip}
	if s.setupTest != nil && s.setupTest.installer != nil {
		in = s.setupTest.installer(user, ip)
	}
	if err := setup.Run(ctx, in, plan, lg); err != nil {
		return err
	}
	if err := s.store.SetSetting(ctx, setup.StateKey, setup.StateDone); err != nil {
		lg.Logf("Warning: the setup could not be marked as finished: %v", err)
	}
	s.audit(ctx, user, "setup_done", ip, plan.Target())
	return nil
}

// SkipSetup marks the wizard skipped, so it is not opened after signing
// in. A finished setup stays finished.
func (s *Server) SkipSetup(ctx context.Context, user, ip string) error {
	state, err := s.store.Setting(ctx, setup.StateKey)
	if err != nil || state == setup.StateDone {
		return err
	}
	if err := s.store.SetSetting(ctx, setup.StateKey, setup.StateSkipped); err != nil {
		return err
	}
	s.audit(ctx, user, "setup_skipped", ip, "")
	return nil
}

// setupInstaller installs the parts with the panel's own functions.
type setupInstaller struct {
	s        *Server
	user, ip string
}

func (si setupInstaller) Installed(ctx context.Context, id string) bool {
	s := si.s
	switch id {
	case setup.Nginx, setup.Apache, setup.OpenLiteSpeed:
		st := s.hosting.Status(ctx)
		return st.Server == id && st.Ready
	case "mariadb":
		return database.InstalledEngine() == database.MariaDB
	case "mysql":
		return database.InstalledEngine() == database.MySQL
	case "redis":
		return software.RedisInstalled()
	case "nodejs":
		return software.NodeInstalled()
	case "python":
		return software.PythonReady()
	case "docker":
		return software.DockerInstalled()
	case "adminer":
		return s.hosting.AdminerInstalled()
	case "waf":
		return s.hosting.WAFInstalled()
	case "clamav":
		return s.hosting.ClamAVInstalled()
	}
	if v, ok := strings.CutPrefix(id, "php"); ok {
		return software.PHPInstalled(v)
	}
	// Anything else the Software page lists (MySQL, PostgreSQL…): as it shows it.
	items, err := s.softwareItems(ctx)
	if err != nil {
		return false
	}
	for _, it := range items {
		if it.ID == id {
			return it.Installed
		}
	}
	return false
}

func (si setupInstaller) Install(ctx context.Context, id string, lg setup.Logger) error {
	s := si.s
	switch id {
	case setup.Nginx, setup.Apache, setup.OpenLiteSpeed:
		err := s.hosting.InstallServer(ctx, id, lg)
		if err == nil {
			s.audit(ctx, si.user, "web_installed", si.ip, id)
		}
		return err
	}
	err := s.installSoftware(ctx, id, lg)
	if err == nil {
		s.audit(ctx, si.user, "software_installed", si.ip, id)
	}
	return err
}

// ---- HTTP ----

// handleGetSetup answers GET /api/setup: what the setup offers on this
// server, the recommended choice, and whether to open it after signing in.
func (s *Server) handleGetSetup(w http.ResponseWriter, r *http.Request, a *authed) {
	v, err := s.SetupView(r.Context())
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// setupStarted is the job of a setup, with its plan.
type setupStarted struct {
	jobs.Snapshot
	Plan setup.Plan `json:"plan"`
}

// handleStartSetup starts the setup: {"web": "nginx", "php": ["8.4"],
// "db": "mariadb", "extras": ["redis"]}. A choice that can't be installed is
// refused before any job starts.
func (s *Server) handleStartSetup(w http.ResponseWriter, r *http.Request, a *authed) {
	var c setup.Choice
	if !decodeJSON(w, r, &c) {
		return
	}
	job, plan, err := s.startSetup(r.Context(), c, a.user.Username, clientIP(r))
	if err != nil {
		s.webServerError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, setupStarted{job.Snapshot(), plan})
}

// handleSkipSetup answers POST /api/setup/skip.
func (s *Server) handleSkipSetup(w http.ResponseWriter, r *http.Request, a *authed) {
	if err := s.SkipSetup(r.Context(), a.user.Username, clientIP(r)); err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
