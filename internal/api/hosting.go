package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/jobs"
	"github.com/nefocp/nefo/internal/store"
)

type siteResponse struct {
	ID           int64      `json:"id"`
	Domain       string     `json:"domain"`
	Aliases      []string   `json:"aliases"`
	SysUser      string     `json:"sys_user"`
	PHPVersion   string     `json:"php_version"`
	Root         string     `json:"root"`
	SSL          bool       `json:"ssl"`
	ForceHTTPS   bool       `json:"force_https"`
	HSTS         bool       `json:"hsts"`
	HSTSSubs     bool       `json:"hsts_subdomains"`
	PageCache    bool       `json:"page_cache"`
	CertExpires  *time.Time `json:"cert_expires,omitempty"`
	SSLError     string     `json:"ssl_error,omitempty"`
	App          string     `json:"app"`
	PHPMemory    int        `json:"php_memory"`
	PHPUpload    int        `json:"php_upload"`
	PHPTime      int        `json:"php_time"`
	PHPWorkers   int        `json:"php_workers"`
	PHPPM        string     `json:"php_pm"`
	PHPAllow     []string   `json:"php_allow"`
	Custom404    bool       `json:"custom_404"`
	Hotlink      bool       `json:"hotlink"`
	HotlinkAllow []string   `json:"hotlink_allow"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	ProxyPort    int        `json:"proxy_port"`
	AppCommand   string     `json:"app_command"`
	AppRuntime   string     `json:"app_runtime"`
	AppStatus    string     `json:"app_status"`
	Disabled     bool       `json:"disabled"`
	RedirectURL  string     `json:"redirect_url"`
	AuthUser     string     `json:"auth_user"`
	SFTP         bool       `json:"sftp"`
	Rewrite      string     `json:"rewrite"`
	NginxCustom  bool       `json:"nginx_custom"`
	SSLSource    string     `json:"ssl_source"`
	SSLDNS       bool       `json:"ssl_dns"`
	WAF          string     `json:"waf"`
	RateLimit    int        `json:"rate_limit"`
	BlockedIPs   int        `json:"blocked_ips"`
	Note         string     `json:"note"`
	CreatedAt    time.Time  `json:"created_at"`
}

func (s *Server) siteJSON(site *store.Site) siteResponse {
	r := siteResponse{
		ID:           site.ID,
		Domain:       site.Domain,
		Aliases:      site.Aliases,
		SysUser:      site.SysUser,
		PHPVersion:   site.PHPVersion,
		Root:         s.hosting.Root(site.Domain),
		SSL:          site.SSL,
		ForceHTTPS:   site.ForceHTTPS,
		HSTS:         site.HSTS,
		HSTSSubs:     site.HSTSSubdomains,
		PageCache:    site.PageCache,
		SSLError:     site.SSLError,
		App:          site.App,
		PHPMemory:    site.PHPMemory,
		PHPUpload:    site.PHPUpload,
		PHPTime:      site.PHPTime,
		PHPWorkers:   site.PHPWorkers,
		PHPPM:        site.PHPPM,
		PHPAllow:     append([]string{}, strings.Fields(site.PHPAllow)...),
		Custom404:    site.Custom404,
		Hotlink:      site.Hotlink,
		HotlinkAllow: append([]string{}, strings.Fields(site.HotlinkAllow)...),
		ProxyPort:    site.ProxyPort,
		AppCommand:   site.AppCommand,
		AppRuntime:   site.AppRuntime,
		Disabled:     site.Disabled,
		RedirectURL:  site.RedirectURL,
		AuthUser:     site.AuthUser,
		SFTP:         site.SFTP,
		Rewrite:      site.Rewrite,
		NginxCustom:  site.NginxCustom != "",
		SSLSource:    site.SSLSource,
		SSLDNS:       site.SSLDNS,
		WAF:          site.WAF,
		RateLimit:    site.RateLimit,
		BlockedIPs:   len(strings.Fields(site.BlockedIPs)),
		Note:         site.Note,
		CreatedAt:    site.CreatedAt,
	}
	if !site.CertNotAfter.IsZero() {
		t := site.CertNotAfter
		r.CertExpires = &t
	}
	if !site.ExpiresAt.IsZero() {
		t := site.ExpiresAt
		r.ExpiresAt = &t
	}
	if site.ProxyPort != 0 {
		r.AppStatus = s.hosting.AppStatus(context.Background(), site)
	}
	return r
}

func siteID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

func (s *Server) handleWebStatus(w http.ResponseWriter, r *http.Request, a *authed) {
	writeJSON(w, http.StatusOK, s.hosting.Status(r.Context()))
}

// handleWebInstall installs the web server: {"server":
// "nginx"|"apache"|"openlitespeed"}; no body or "" means Nginx, as before
// the choice was offered.
func (s *Server) handleWebInstall(w http.ResponseWriter, r *http.Request, a *authed) {
	if !s.hosting.Supported() {
		s.opError(w, r, hosting.ErrUnsupported)
		return
	}
	var req struct {
		Server string `json:"server"`
	}
	if r.ContentLength != 0 && !decodeJSON(w, r, &req) {
		return
	}
	kind, err := s.hosting.CheckInstall(r.Context(), req.Server)
	if err != nil {
		s.webServerError(w, r, err)
		return
	}
	user, ip := a.user.Username, clientIP(r)
	job, err := s.jobs.Start("install_web", kind, 30*time.Minute, func(ctx context.Context, j *jobs.Job) error {
		err := s.hosting.InstallServer(ctx, kind, j)
		if err == nil {
			s.audit(ctx, user, "web_installed", ip, kind)
		}
		return err
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

func (s *Server) handleListSites(w http.ResponseWriter, r *http.Request, a *authed) {
	sites, err := s.hosting.ListSites(r.Context())
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	out := make([]siteResponse, 0, len(sites))
	for _, site := range sites {
		out = append(out, s.siteJSON(site))
	}
	writeJSON(w, http.StatusOK, map[string]any{"sites": out})
}

func (s *Server) handleCreateSite(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Domain     string   `json:"domain"`
		Aliases    []string `json:"aliases"`
		PHPVersion string   `json:"php_version"`
		ProxyPort  int      `json:"proxy_port"`
		AppCommand string   `json:"app_command"`
		AppRuntime string   `json:"app_runtime"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	site, err := s.hosting.CreateSite(r.Context(), hosting.CreateRequest{
		Domain: req.Domain, Aliases: req.Aliases, PHPVersion: req.PHPVersion, ProxyPort: req.ProxyPort, AppCommand: req.AppCommand,
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "site_created", clientIP(r), site.Domain)
	writeJSON(w, http.StatusCreated, s.siteJSON(site))
}

func (s *Server) handleDeleteSite(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := siteID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "site_not_found")
		return
	}
	var req struct {
		Confirm         string `json:"confirm"`
		DeleteDatabases bool   `json:"delete_databases"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	site, err := s.store.SiteByID(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	// Read the linked databases first: deleting the site unlinks them.
	linked, err := s.store.DatabasesForSite(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	pgLinked, err := s.store.PGDatabasesForSite(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	// A deploy writes into the site's folder as its user: let it finish first.
	if s.gitDeployRunning(site.Domain) {
		s.opError(w, r, jobs.ErrBusy)
		return
	}
	// Background programs run as the site's user: stop them first, or userdel refuses.
	s.removeSiteDaemons(r.Context(), site, req.Confirm)
	warnings, err := s.hosting.DeleteSite(r.Context(), id, req.Confirm)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "site_deleted", clientIP(r), req.Confirm)
	s.forgetStagingLink(r.Context(), id)
	s.gitDeploy().SiteDeleted(r.Context(), id) // the key went with the site folder
	if err := s.files.DropSiteTrash(site.Domain); err != nil {
		s.log.Warn("remove the site's trash", "site", site.Domain, "err", err)
	}
	s.store.DeleteSchedulesFor(r.Context(), "site", req.Confirm)
	s.store.DeleteSchedulesFor(r.Context(), "app", site.SysUser) // its Docker app went with it
	// Cron jobs of the site's Linux user cannot run any more.
	if jobs, err := s.store.ListCronJobs(r.Context()); err == nil {
		for _, c := range jobs {
			if c.RunAs == site.SysUser {
				s.store.DeleteCronJob(r.Context(), c.ID)
			}
		}
		s.cron.Sync(r.Context())
	}
	if req.DeleteDatabases {
		for _, d := range linked {
			if err := s.db.Delete(r.Context(), d.ID, d.Name); err != nil {
				warnings = append(warnings, fmt.Sprintf("database %s: %v", d.Name, err))
				continue
			}
			s.store.DeleteSchedulesFor(r.Context(), "database", d.Name)
			s.audit(r.Context(), a.user.Username, "db_deleted", clientIP(r), d.Name)
		}
		if len(linked) > 0 {
			s.restartDBIfNeeded(r.Context(), a.user.Username, clientIP(r))
		}
		for _, d := range pgLinked {
			if err := s.pg.Delete(r.Context(), d.ID, d.Name); err != nil {
				warnings = append(warnings, fmt.Sprintf("PostgreSQL database %s: %v", d.Name, err))
				continue
			}
			s.store.DeleteSchedulesFor(r.Context(), "database", d.Name)
			s.audit(r.Context(), a.user.Username, "pg_db_deleted", clientIP(r), d.Name)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"warnings": warnings})
}

func (s *Server) handleIssueSSL(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := siteID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "site_not_found")
		return
	}
	site, err := s.store.SiteByID(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	if !s.hosting.Supported() {
		s.opError(w, r, hosting.ErrUnsupported)
		return
	}
	// {"dns": true} validates through Cloudflare from now on; no body or
	// {} keeps the site's choice.
	var req struct {
		DNS *bool `json:"dns"`
	}
	if r.ContentLength != 0 && !decodeJSON(w, r, &req) {
		return
	}
	user, ip, domain := a.user.Username, clientIP(r), site.Domain
	job, err := s.jobs.Start("issue_ssl", domain, 10*time.Minute, func(ctx context.Context, j *jobs.Job) error {
		err := s.hosting.IssueCertificate(ctx, id, hosting.IssueOptions{DNS: req.DNS}, j)
		action := "ssl_issued"
		if err != nil {
			action = "ssl_failed"
		}
		s.audit(ctx, user, action, ip, domain)
		return err
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

func (s *Server) handleForceHTTPS(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := siteID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "site_not_found")
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	site, err := s.hosting.SetForceHTTPS(r.Context(), id, req.Enabled)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	action := "force_https_off"
	if req.Enabled {
		action = "force_https_on"
	}
	s.audit(r.Context(), a.user.Username, action, clientIP(r), site.Domain)
	writeJSON(w, http.StatusOK, s.siteJSON(site))
}

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request, a *authed) {
	writeJSON(w, http.StatusOK, map[string]any{"jobs": s.jobs.Recent()})
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request, a *authed) {
	j, ok := s.jobs.Get(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusNotFound, "job_not_found")
		return
	}
	writeJSON(w, http.StatusOK, j.Snapshot())
}

func (s *Server) handleSiteOptions(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	var req struct {
		Custom404    bool     `json:"custom_404"`
		Hotlink      bool     `json:"hotlink"`
		HotlinkAllow []string `json:"hotlink_allow"`
		ExpiresAt    string   `json:"expires_at"` // YYYY-MM-DD, or "" for never
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	var expires time.Time
	if req.ExpiresAt != "" {
		t, err := time.ParseInLocation("2006-01-02", req.ExpiresAt, time.Local)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_date")
			return
		}
		expires = t.AddDate(0, 0, 1) // the site works until the end of that day
	}
	site, err := s.hosting.SetSiteOptions(r.Context(), id, hosting.SiteOptions{
		Custom404: req.Custom404, Hotlink: req.Hotlink, HotlinkAllow: req.HotlinkAllow, ExpiresAt: expires,
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "site_options_changed", clientIP(r), site.Domain)
	writeJSON(w, http.StatusOK, s.siteJSON(site))
}

// maxSiteNote is the longest note, in characters.
const maxSiteNote = 200

// cleanSiteNote trims a note and refuses one that is too long or holds
// control characters (it is a single line of plain text).
func cleanSiteNote(note string) (string, bool) {
	note = strings.TrimSpace(note)
	if !utf8.ValidString(note) || utf8.RuneCountInString(note) > maxSiteNote {
		return "", false
	}
	for _, r := range note {
		if unicode.IsControl(r) || r == ' ' || r == ' ' {
			return "", false
		}
	}
	return note, true
}

func (s *Server) handleSiteNote(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	var req struct {
		Note string `json:"note"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	note, ok := cleanSiteNote(req.Note)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_note")
		return
	}
	if err := s.store.SetSiteNote(r.Context(), id, note); err != nil {
		s.opError(w, r, err)
		return
	}
	site, err := s.store.SiteByID(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "site_note_changed", clientIP(r), site.Domain)
	writeJSON(w, http.StatusOK, s.siteJSON(site))
}

// expireSites runs from the hourly maintenance loop.
func (s *Server) expireSites(ctx context.Context) {
	s.hosting.ExpireSites(ctx, func(domain string) {
		s.audit(ctx, "system", "site_expired_suspended", "", domain)
	})
}

// renewCertificates runs from the hourly maintenance loop.
func (s *Server) renewCertificates(ctx context.Context) {
	s.hosting.RenewDue(ctx, func(domain string, err error) {
		action := "ssl_renewed"
		if err != nil {
			action = "ssl_renew_failed"
		}
		s.audit(ctx, "system", action, "", domain)
	})
}

func (s *Server) handleSiteSettings(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	var req struct {
		Aliases    []string `json:"aliases"`
		PHPVersion string   `json:"php_version"`
		PHPMemory  int      `json:"php_memory"`
		PHPUpload  int      `json:"php_upload"`
		PHPTime    int      `json:"php_time"`
		PHPWorkers int      `json:"php_workers"`
		PHPPM      string   `json:"php_pm"`
		PHPAllow   []string `json:"php_allow"`
		ProxyPort  int      `json:"proxy_port"`
		AppCommand string   `json:"app_command"`
		AppRuntime string   `json:"app_runtime"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	site, reissue, err := s.hosting.UpdateSettings(r.Context(), id, hosting.Settings{
		Aliases: req.Aliases, PHPVersion: req.PHPVersion, PHPMemory: req.PHPMemory, PHPUpload: req.PHPUpload, PHPTime: req.PHPTime,
		PHPWorkers: req.PHPWorkers, PHPPM: req.PHPPM, PHPAllow: req.PHPAllow,
		ProxyPort: req.ProxyPort, AppCommand: req.AppCommand, AppRuntime: req.AppRuntime,
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "site_settings_changed", clientIP(r), site.Domain)
	writeJSON(w, http.StatusOK, map[string]any{"site": s.siteJSON(site), "reissue_needed": reissue})
}

func (s *Server) handleSiteLog(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	lines, _ := strconv.Atoi(r.URL.Query().Get("lines"))
	text, err := s.hosting.SiteLog(r.Context(), id, r.URL.Query().Get("type"), lines)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"log": text})
}

func (s *Server) handleSiteAccess(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	var req struct {
		Disabled     bool   `json:"disabled"`
		RedirectURL  string `json:"redirect_url"`
		AuthUser     string `json:"auth_user"`
		AuthPassword string `json:"auth_password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	site, err := s.hosting.SetAccess(r.Context(), id, hosting.Access{
		Disabled: req.Disabled, RedirectURL: req.RedirectURL, AuthUser: req.AuthUser, AuthPassword: req.AuthPassword,
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "site_access_changed", clientIP(r), site.Domain)
	writeJSON(w, http.StatusOK, s.siteJSON(site))
}

func (s *Server) handleInstallCertificate(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	var req struct {
		Cert string `json:"cert"`
		Key  string `json:"key"`
	}
	if !decodeJSONLimit(w, r, &req, 256<<10) {
		return
	}
	site, warnings, err := s.hosting.InstallCertificate(r.Context(), id, req.Cert, req.Key)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "ssl_custom_installed", clientIP(r), site.Domain)
	writeJSON(w, http.StatusOK, map[string]any{"site": s.siteJSON(site), "warnings": warnings})
}

func (s *Server) handleGetCloudflare(w http.ResponseWriter, r *http.Request, a *authed) {
	writeJSON(w, http.StatusOK, s.hosting.Cloudflare(r.Context()))
}

func (s *Server) handleSetCloudflare(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Token string `json:"token"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.hosting.SetCloudflare(r.Context(), req.Token); err != nil {
		s.opError(w, r, err)
		return
	}
	action := "cloudflare_token_saved"
	if strings.TrimSpace(req.Token) == "" {
		action = "cloudflare_token_removed"
	}
	s.audit(r.Context(), a.user.Username, action, clientIP(r), "")
	writeJSON(w, http.StatusOK, s.hosting.Cloudflare(r.Context()))
}

// firewallResponse is a site's firewall settings with its latest events.
type firewallResponse struct {
	hosting.Firewall
	WAFInstalled bool               `json:"waf_installed"`
	Events       []hosting.WAFEvent `json:"events"`
}

func (s *Server) firewallJSON(r *http.Request, site *store.Site) (*firewallResponse, error) {
	events, err := s.hosting.WAFEvents(r.Context(), site.ID, 50)
	if err != nil {
		return nil, err
	}
	ips := strings.Fields(site.BlockedIPs)
	if ips == nil {
		ips = []string{}
	}
	return &firewallResponse{
		Firewall:     hosting.Firewall{WAF: site.WAF, RateLimit: site.RateLimit, BlockedIPs: ips},
		WAFInstalled: s.hosting.WAFInstalled(),
		Events:       events,
	}, nil
}

func (s *Server) handleGetFirewall(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	site, err := s.store.SiteByID(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	res, err := s.firewallJSON(r, site)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleSetFirewall(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	var req hosting.Firewall
	if !decodeJSON(w, r, &req) {
		return
	}
	site, err := s.hosting.SetFirewall(r.Context(), id, req)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "site_firewall_changed", clientIP(r), site.Domain)
	res, err := s.firewallJSON(r, site)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleScanReport(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	rep, err := s.hosting.ScanReport(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	site, err := s.store.SiteByID(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	var req struct {
		ClamAV bool `json:"clamav"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	user, ip, domain := a.user.Username, clientIP(r), site.Domain
	job, err := s.jobs.Start("malware_scan", domain, time.Hour, func(ctx context.Context, j *jobs.Job) error {
		err := s.hosting.ScanSite(ctx, id, req.ClamAV, j)
		if err == nil {
			s.audit(ctx, user, "malware_scanned", ip, domain)
		}
		return err
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

func (s *Server) handleQuarantine(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	var req struct {
		Path string `json:"path"`
		Rule string `json:"rule"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	q, err := s.hosting.Quarantine(r.Context(), id, req.Path, req.Rule)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "file_quarantined", clientIP(r), q.Path)
	writeJSON(w, http.StatusOK, q)
}

func (s *Server) handleQuarantineAction(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	qid, err := strconv.ParseInt(r.PathValue("qid"), 10, 64)
	if !ok || err != nil {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	action := "file_restored"
	if r.Method == http.MethodDelete {
		action = "quarantine_deleted"
		err = s.hosting.DeleteQuarantined(r.Context(), id, qid)
	} else {
		err = s.hosting.RestoreQuarantined(r.Context(), id, qid)
	}
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, action, clientIP(r), strconv.FormatInt(qid, 10))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleGetNginx(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	cfg, err := s.hosting.NginxConfig(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) handleSetNginx(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	var req struct {
		Rewrite string `json:"rewrite"`
		Custom  string `json:"custom"`
	}
	if !decodeJSONLimit(w, r, &req, 256<<10) {
		return
	}
	cfg, err := s.hosting.SetNginxConfig(r.Context(), id, req.Rewrite, req.Custom)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "site_nginx_changed", clientIP(r), strconv.FormatInt(id, 10))
	writeJSON(w, http.StatusOK, cfg)
}

// sftpResponse adds the SSH port to a site's SFTP account.
type sftpResponse struct {
	*hosting.SFTPInfo
	Port int `json:"port"`
}

func (s *Server) handleGetSFTP(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	info, err := s.hosting.SFTP(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, sftpResponse{info, s.security.SSHSettings(r.Context()).Port})
}

func (s *Server) handleSetSFTP(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	var req struct {
		Enabled          bool    `json:"enabled"`
		Password         string  `json:"password"`
		GeneratePassword bool    `json:"generate_password"`
		RemovePassword   bool    `json:"remove_password"`
		Keys             *string `json:"keys"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	info, err := s.hosting.SetSFTP(r.Context(), id, hosting.SFTPSettings{
		Enabled: req.Enabled, Password: req.Password, GeneratePassword: req.GeneratePassword,
		RemovePassword: req.RemovePassword, Keys: req.Keys,
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	action := "sftp_disabled"
	if info.Enabled {
		action = "sftp_changed"
	}
	s.audit(r.Context(), a.user.Username, action, clientIP(r), info.Username)
	writeJSON(w, http.StatusOK, sftpResponse{info, s.security.SSHSettings(r.Context()).Port})
}
