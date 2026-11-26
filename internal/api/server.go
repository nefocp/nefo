// Package api serves the panel's HTTP API and the embedded user interface.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/nefocp/nefo/internal/auth"
	"github.com/nefocp/nefo/internal/backup"
	"github.com/nefocp/nefo/internal/config"
	"github.com/nefocp/nefo/internal/cron"
	"github.com/nefocp/nefo/internal/database"
	"github.com/nefocp/nefo/internal/docker"
	"github.com/nefocp/nefo/internal/files"
	"github.com/nefocp/nefo/internal/gitdeploy"
	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/jobs"
	"github.com/nefocp/nefo/internal/mailrelay"
	"github.com/nefocp/nefo/internal/monitor"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/security"
	"github.com/nefocp/nefo/internal/services"
	"github.com/nefocp/nefo/internal/software"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/system"
	"github.com/nefocp/nefo/internal/systools"
	"github.com/nefocp/nefo/internal/uptime"
	"github.com/nefocp/nefo/internal/web"
)

// Server holds the dependencies of the HTTP handlers.
type Server struct {
	cfg      *config.Config
	store    *store.Store
	sys      *system.Collector
	limiter  *auth.Limiter
	hosting  *hosting.Manager
	db       *database.Manager
	files    *files.Manager
	backup   *backup.Manager
	cron     *cron.Manager
	services *services.Manager
	security *security.Manager
	software *software.Manager
	monitor  *monitor.Monitor
	uptime   *uptime.Service
	docker   *docker.Manager
	mail     *mailrelay.Manager
	systools *systools.Manager
	jobs     *jobs.Manager
	log      *slog.Logger
	now      func() time.Time // replaceable in tests
	started  time.Time        // when the panel started

	totpMu   sync.Mutex
	totpUsed map[int64]uint64 // last accepted TOTP time step per user, to stop replays
}

// New returns a Server.
func New(cfg *config.Config, st *store.Store, log *slog.Logger) *Server {
	s := &Server{
		cfg:   cfg,
		store: st,
		sys:   system.NewCollector(),
		// 5 failures in 15 minutes locks the IP (or IPv6 /64) out for 15 minutes.
		limiter: auth.NewLimiter(5, 15*time.Minute, 15*time.Minute),
		hosting: hosting.New(hosting.DefaultPaths(cfg.DataDir), run.System{}, st, log,
			cfg.ACMEDirectoryURL()),
		db:       database.New(run.System{}, st, log),
		jobs:     jobs.NewManager(log),
		log:      log,
		now:      time.Now,
		started:  time.Now(),
		totpUsed: map[int64]uint64{},
	}
	s.files = files.New(s.fileScope)
	s.backup = backup.New(st, s.db, s.hosting, backup.DefaultDir, log)
	s.backup.ProtectFolders(cfg.DataDir) // folder backups never touch the panel's own data
	s.cron = cron.New(st, run.System{}, log, s.cronUsers)
	s.services = services.New(run.System{})
	s.security = security.New(run.System{}, cfg.Port())
	s.db.SetFirewall(s.security) // remote database access opens and closes port 3306
	s.security.SetWebPorts(s.hosting.FirewallPorts)
	s.docker = docker.New(run.System{})
	s.mail = mailrelay.New(run.System{}, st)
	s.hosting.SetReservedPorts(cfg.Port(), gitdeploy.HookPort)
	s.monitor = monitor.New(st, s.sys, log, s.watchedServices)
	s.uptime = uptime.New(st, s.monitor.Notify, log)
	s.software = software.New(run.System{}, st, func(ctx context.Context) string { return s.hosting.Status(ctx).DefaultPHP })
	s.systools = systools.New(run.System{}, st)
	return s
}

// Handler returns the complete HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.Handle("POST /api/auth/logout", s.requireAuth(s.handleLogout))
	mux.Handle("GET /api/auth/me", s.requireAuth(s.handleMe))
	mux.Handle("POST /api/auth/password", s.requireAuth(s.handleChangePassword))
	mux.Handle("POST /api/auth/2fa/setup", s.requireAuth(s.handleTOTPSetup))
	mux.Handle("POST /api/auth/2fa/enable", s.requireAuth(s.handleTOTPEnable))
	mux.Handle("POST /api/auth/2fa/disable", s.requireAuth(s.handleTOTPDisable))

	mux.Handle("GET /api/system/info", s.requireAuth(s.handleSystemInfo))
	mux.Handle("GET /api/system/stats", s.requireAuth(s.handleSystemStats))
	mux.Handle("GET /api/audit", s.requireAuth(s.handleAudit))

	mux.Handle("GET /api/web/status", s.requireAuth(s.handleWebStatus))
	mux.Handle("POST /api/web/install", s.requireAuth(s.handleWebInstall))
	mux.Handle("POST /api/web/remove", s.requireAuth(s.handleWebRemove))
	mux.Handle("GET /api/sites", s.requireAuth(s.handleListSites))
	mux.Handle("POST /api/sites", s.requireAuth(s.handleCreateSite))
	mux.Handle("DELETE /api/sites/{id}", s.requireAuth(s.handleDeleteSite))
	mux.Handle("POST /api/sites/{id}/ssl", s.requireAuth(s.handleIssueSSL))
	mux.Handle("POST /api/sites/{id}/force-https", s.requireAuth(s.handleForceHTTPS))
	mux.Handle("DELETE /api/sites/{id}/ssl", s.requireAuth(s.handleRemoveSSL))
	mux.Handle("PUT /api/sites/{id}/hsts", s.requireAuth(s.handleSetHSTS))
	mux.Handle("PUT /api/sites/{id}/page-cache", s.requireAuth(s.handleSetPageCache))
	mux.Handle("POST /api/sites/{id}/page-cache/purge", s.requireAuth(s.handlePurgePageCache))
	mux.Handle("POST /api/sites/{id}/wordpress", s.requireAuth(s.handleInstallApp))
	mux.Handle("POST /api/sites/{id}/apps/{app}", s.requireAuth(s.handleInstallApp))
	mux.Handle("GET /api/apps", s.requireAuth(s.handleAppCatalog))
	mux.Handle("POST /api/apps/docker", s.requireAuth(s.handleInstallDockerApp))
	mux.Handle("GET /api/apps/logos/{key}", s.requireAuth(s.handleAppLogo))
	mux.Handle("POST /api/apps/catalog/refresh", s.requireAuth(s.handleRefreshCatalog))
	mux.Handle("POST /api/apps/{id}/check", s.requireAuth(s.handleCheckApp))
	mux.Handle("POST /api/apps/{id}/install", s.requireAuth(s.handleInstallCatalogApp))
	mux.Handle("GET /api/apps/installed", s.requireAuth(s.handleInstalledApps))
	mux.Handle("GET /api/apps/installed/{name}/credentials", s.requireAuth(s.handleAppCredentials))
	mux.Handle("GET /api/apps/installed/{name}/logs", s.requireAuth(s.handleInstanceLogs))
	mux.Handle("POST /api/apps/installed/{name}/{action}", s.requireAuth(s.handleInstanceAction))
	mux.Handle("DELETE /api/apps/installed/{name}", s.requireAuth(s.handleUninstallApp))
	mux.Handle("PUT /api/sites/{id}/settings", s.requireAuth(s.handleSiteSettings))
	mux.Handle("PUT /api/sites/{id}/access", s.requireAuth(s.handleSiteAccess))
	mux.Handle("PUT /api/sites/{id}/options", s.requireAuth(s.handleSiteOptions))
	mux.Handle("PUT /api/sites/{id}/note", s.requireAuth(s.handleSiteNote))
	mux.Handle("POST /api/sites/{id}/clone", s.requireAuth(s.handleSiteClone))
	mux.Handle("GET /api/sites/{id}/sftp", s.requireAuth(s.handleGetSFTP))
	mux.Handle("GET /api/sites/{id}/nginx", s.requireAuth(s.handleGetNginx))
	mux.Handle("GET /api/sites/{id}/firewall", s.requireAuth(s.handleGetFirewall))
	mux.Handle("GET /api/sites/{id}/scan", s.requireAuth(s.handleScanReport))
	mux.Handle("POST /api/sites/{id}/scan", s.requireAuth(s.handleScan))
	mux.Handle("POST /api/sites/{id}/quarantine", s.requireAuth(s.handleQuarantine))
	mux.Handle("POST /api/sites/{id}/quarantine/{qid}/restore", s.requireAuth(s.handleQuarantineAction))
	mux.Handle("DELETE /api/sites/{id}/quarantine/{qid}", s.requireAuth(s.handleQuarantineAction))
	mux.Handle("PUT /api/sites/{id}/firewall", s.requireAuth(s.handleSetFirewall))
	mux.Handle("POST /api/sites/{id}/certificate", s.requireAuth(s.handleInstallCertificate))
	mux.Handle("GET /api/settings/cloudflare", s.requireAuth(s.handleGetCloudflare))
	mux.Handle("GET /api/mail-relay", s.requireAuth(s.handleMailRelayStatus))
	mux.Handle("GET /api/php/{version}", s.requireAuth(s.handlePHPConfig))
	mux.Handle("GET /api/node-versions", s.requireAuth(s.handleNodeVersions))
	mux.Handle("POST /api/node-versions/{major}", s.requireAuth(s.handleNodeVersion))
	mux.Handle("DELETE /api/node-versions/{major}", s.requireAuth(s.handleNodeVersion))
	mux.Handle("POST /api/sites/{id}/app/deps", s.requireAuth(s.handleAppDeps))
	mux.Handle("POST /api/php/{version}/extensions/{ext}", s.requireAuth(s.handlePHPExtension))
	mux.Handle("DELETE /api/php/{version}/extensions/{ext}", s.requireAuth(s.handlePHPExtension))
	mux.Handle("PUT /api/php/{version}/ini", s.requireAuth(s.handlePHPIni))
	mux.Handle("PUT /api/mail-relay", s.requireAuth(s.handleMailRelaySave))
	mux.Handle("POST /api/mail-relay/test", s.requireAuth(s.handleMailRelayTest))
	mux.Handle("DELETE /api/mail-relay", s.requireAuth(s.handleMailRelayRemove))
	mux.Handle("PUT /api/settings/cloudflare", s.requireAuth(s.handleSetCloudflare))
	// The DNS page and "point DNS here" through the Cloudflare token (dns.go).
	mux.Handle("GET /api/dns/zones", s.requireAuth(s.handleDNSZones))
	mux.Handle("GET /api/dns/zones/{zone}/records", s.requireAuth(s.handleDNSRecords))
	mux.Handle("POST /api/dns/zones/{zone}/records", s.requireAuth(s.handleCreateDNSRecord))
	mux.Handle("PUT /api/dns/zones/{zone}/records/{id}", s.requireAuth(s.handleUpdateDNSRecord))
	mux.Handle("DELETE /api/dns/zones/{zone}/records/{id}", s.requireAuth(s.handleDeleteDNSRecord))
	mux.Handle("POST /api/dns/zones/{zone}/purge", s.requireAuth(s.handlePurgeDNSCache))
	mux.Handle("POST /api/sites/{id}/dns/plan", s.requireAuth(s.handlePointDNSPlan))
	mux.Handle("POST /api/sites/{id}/dns/apply", s.requireAuth(s.handlePointDNSApply))
	mux.Handle("PUT /api/sites/{id}/nginx", s.requireAuth(s.handleSetNginx))
	mux.Handle("PUT /api/sites/{id}/sftp", s.requireAuth(s.handleSetSFTP))
	mux.Handle("GET /api/databases/{id}/export", s.requireAuth(s.handleExportDatabase))
	mux.Handle("POST /api/databases/{id}/import", s.requireAuth(s.handleImportDatabase))
	mux.Handle("GET /api/sites/{id}/logs", s.requireAuth(s.handleSiteLog))
	mux.Handle("GET /api/sites/{id}/stats", s.requireAuth(s.handleSiteStats))
	mux.Handle("GET /api/sites/{id}/git", s.requireAuth(s.handleGetGit))
	mux.Handle("PUT /api/sites/{id}/git", s.requireAuth(s.handleSaveGit))
	mux.Handle("DELETE /api/sites/{id}/git", s.requireAuth(s.handleDeleteGit))
	mux.Handle("POST /api/sites/{id}/git/deploy", s.requireAuth(s.handleGitDeploy))
	mux.Handle("POST /api/sites/{id}/git/key", s.requireAuth(s.handleGitKey))
	mux.Handle("POST /api/sites/{id}/git/secret", s.requireAuth(s.handleGitSecret))
	// Website monitoring (uptime.go).
	mux.Handle("GET /api/sites/{id}/monitor", s.requireAuth(s.handleGetSiteMonitor))
	mux.Handle("PUT /api/sites/{id}/monitor", s.requireAuth(s.handleSaveSiteMonitor))
	mux.Handle("GET /api/uptime", s.requireAuth(s.handleUptimeSummary))

	mux.Handle("GET /api/wordpress", s.requireAuth(s.handleWPList))
	mux.Handle("GET /api/wordpress/{id}", s.requireAuth(s.handleWPStatus))
	mux.Handle("POST /api/wordpress/{id}/update", s.requireAuth(s.handleWPUpdate))
	mux.Handle("PUT /api/wordpress/{id}/maintenance", s.requireAuth(s.handleWPMaintenance))
	mux.Handle("PUT /api/wordpress/{id}/debug", s.requireAuth(s.handleWPDebug))
	mux.Handle("PUT /api/wordpress/{id}/auto-updates", s.requireAuth(s.handleWPAutoUpdates))
	mux.Handle("GET /api/wordpress/{id}/admins", s.requireAuth(s.handleWPAdmins))
	mux.Handle("POST /api/wordpress/{id}/password", s.requireAuth(s.handleWPPassword))
	mux.Handle("POST /api/wordpress/{id}/staging", s.requireAuth(s.handleWPStaging))
	mux.Handle("GET /api/wordpress/{id}/push", s.requireAuth(s.handleWPPushInfo))
	mux.Handle("POST /api/wordpress/{id}/push", s.requireAuth(s.handleWPPush))

	mux.Handle("GET /api/db/status", s.requireAuth(s.handleDBStatus))
	mux.Handle("POST /api/db/install", s.requireAuth(s.handleDBInstall))
	mux.Handle("GET /api/databases", s.requireAuth(s.handleListDatabases))
	mux.Handle("POST /api/databases", s.requireAuth(s.handleCreateDatabase))
	mux.Handle("GET /api/databases/{id}/credentials", s.requireAuth(s.handleDBCredentials))
	mux.Handle("POST /api/databases/{id}/password", s.requireAuth(s.handleDBPassword))
	mux.Handle("DELETE /api/databases/{id}", s.requireAuth(s.handleDeleteDatabase))
	mux.Handle("GET /api/databases/{id}/remote", s.requireAuth(s.handleDBRemote))
	mux.Handle("PUT /api/databases/{id}/remote", s.requireAuth(s.handleSetDBRemote))
	mux.Handle("POST /api/db/restart", s.requireAuth(s.handleDBRestart))
	mux.Handle("GET /api/db/slowlog", s.requireAuth(s.handleDBSlowLog))
	mux.Handle("PUT /api/db/slowlog", s.requireAuth(s.handleSetDBSlowLog))
	mux.Handle("DELETE /api/db/slowlog", s.requireAuth(s.handleClearDBSlowLog))

	mux.Handle("GET /api/redis", s.requireAuth(s.handleRedisInfo))
	mux.Handle("GET /api/redis/keys", s.requireAuth(s.handleRedisScan))
	mux.Handle("GET /api/redis/key", s.requireAuth(s.handleRedisKey))
	mux.Handle("DELETE /api/redis/key", s.requireAuth(s.handleRedisDeleteKey))
	mux.Handle("POST /api/redis/flush", s.requireAuth(s.handleRedisFlush))

	mux.Handle("GET /api/files", s.requireAuth(s.handleListFiles))
	mux.Handle("GET /api/files/content", s.requireAuth(s.handleReadFile))
	mux.Handle("PUT /api/files/content", s.requireAuth(s.handleWriteFile))
	mux.Handle("POST /api/files/mkdir", s.requireAuth(s.handleMkdir))
	mux.Handle("POST /api/files/rename", s.requireAuth(s.handleRenameFile))
	mux.Handle("POST /api/files/delete", s.requireAuth(s.handleDeleteFiles))
	mux.Handle("POST /api/files/chmod", s.requireAuth(s.handleChmod))
	mux.Handle("POST /api/files/upload", s.requireAuth(s.handleUpload))
	mux.Handle("GET /api/files/download", s.requireAuth(s.handleDownload))
	mux.Handle("POST /api/files/compress", s.requireAuth(s.handleCompress))
	mux.Handle("POST /api/files/extract", s.requireAuth(s.handleExtract))
	mux.Handle("POST /api/files/fetch", s.requireAuth(s.handleFetchURL))
	mux.Handle("GET /api/files/search", s.requireAuth(s.handleSearchFiles))
	mux.Handle("POST /api/files/delete-permanently", s.requireAuth(s.handleDeletePermanently))
	mux.Handle("GET /api/files/trash", s.requireAuth(s.handleListTrash))
	mux.Handle("POST /api/files/trash/restore", s.requireAuth(s.handleRestoreTrash))
	mux.Handle("POST /api/files/trash/delete", s.requireAuth(s.handleDeleteTrash))
	mux.Handle("POST /api/files/trash/empty", s.requireAuth(s.handleEmptyTrash))

	mux.Handle("GET /api/backups", s.requireAuth(s.handleListBackups))
	mux.Handle("POST /api/backups", s.requireAuth(s.handleCreateBackup))
	mux.Handle("POST /api/backups/{id}/restore", s.requireAuth(s.handleRestoreBackup))
	mux.Handle("POST /api/backups/{id}/upload", s.requireAuth(s.handleUploadBackup))
	mux.Handle("GET /api/backups/{id}/download", s.requireAuth(s.handleDownloadBackup))
	mux.Handle("DELETE /api/backups/{id}", s.requireAuth(s.handleDeleteBackup))
	mux.Handle("POST /api/backups/schedules", s.requireAuth(s.handleCreateSchedule))
	mux.Handle("DELETE /api/backups/schedules/{id}", s.requireAuth(s.handleDeleteSchedule))
	mux.Handle("PUT /api/backups/storage", s.requireAuth(s.handleSaveStorage))
	mux.Handle("DELETE /api/backups/storage", s.requireAuth(s.handleRemoveStorage))
	mux.Handle("PUT /api/backups/sftp", s.requireAuth(s.handleSaveSFTP))
	mux.Handle("DELETE /api/backups/sftp", s.requireAuth(s.handleRemoveSFTP))

	mux.Handle("GET /api/cron", s.requireAuth(s.handleListCron))
	mux.Handle("POST /api/cron", s.requireAuth(s.handleSaveCron))
	mux.Handle("PUT /api/cron/{id}", s.requireAuth(s.handleSaveCron))
	mux.Handle("DELETE /api/cron/{id}", s.requireAuth(s.handleDeleteCron))
	mux.Handle("POST /api/cron/{id}/run", s.requireAuth(s.handleRunCron))
	mux.Handle("GET /api/cron/{id}/log", s.requireAuth(s.handleCronLog))

	mux.Handle("GET /api/services", s.requireAuth(s.handleListServices))
	mux.Handle("POST /api/services/{name}/{action}", s.requireAuth(s.handleServiceAction))
	mux.Handle("GET /api/services/{name}/logs", s.requireAuth(s.handleServiceLogs))

	mux.Handle("GET /api/system/processes", s.requireAuth(s.handleProcesses))
	mux.Handle("POST /api/system/processes/{pid}/kill", s.requireAuth(s.handleKillProcess))
	mux.Handle("GET /api/system/ports", s.requireAuth(s.handlePorts))
	mux.Handle("GET /api/system/ssh-logins", s.requireAuth(s.handleSSHLogins))

	mux.Handle("GET /api/daemons", s.requireAuth(s.handleListDaemons))
	mux.Handle("POST /api/daemons", s.requireAuth(s.handleSaveDaemon))
	mux.Handle("PUT /api/daemons/{id}", s.requireAuth(s.handleSaveDaemon))
	mux.Handle("DELETE /api/daemons/{id}", s.requireAuth(s.handleDeleteDaemon))
	mux.Handle("POST /api/daemons/{id}/{action}", s.requireAuth(s.handleDaemonAction))
	mux.Handle("GET /api/daemons/{id}/logs", s.requireAuth(s.handleDaemonLogs))

	mux.Handle("GET /api/security", s.requireAuth(s.handleSecurityStatus))
	mux.Handle("POST /api/security/firewall/rules", s.requireAuth(s.handleAddRule))
	mux.Handle("POST /api/security/firewall/rules/delete", s.requireAuth(s.handleDeleteRule))
	mux.Handle("POST /api/security/firewall/enable", s.requireAuth(s.handleEnableFirewall))
	mux.Handle("PUT /api/security/ssh", s.requireAuth(s.handleSetSSH))
	mux.Handle("GET /api/security/ssh/keys", s.requireAuth(s.handleSSHKeys))
	mux.Handle("POST /api/security/ssh/keys", s.requireAuth(s.handleAddSSHKey))
	mux.Handle("POST /api/security/ssh/keys/delete", s.requireAuth(s.handleRemoveSSHKey))
	mux.Handle("POST /api/security/fail2ban/install", s.requireAuth(s.handleInstallFail2ban))
	mux.Handle("POST /api/security/fail2ban/unban", s.requireAuth(s.handleUnban))

	mux.Handle("GET /api/panel", s.requireAuth(s.handlePanelSettings))
	mux.Handle("POST /api/panel/certificate", s.requireAuth(s.handlePanelCertificate))
	mux.Handle("DELETE /api/panel/certificate", s.requireAuth(s.handleRemovePanelCertificate))
	mux.Handle("GET /api/panel/update", s.requireAuth(s.handleCheckUpdate))
	mux.Handle("POST /api/panel/update", s.requireAuth(s.handleApplyUpdate))

	mux.Handle("GET /api/features", s.requireAuth(s.handleFeatures))
	mux.Handle("GET /api/software", s.requireAuth(s.handleSoftware))
	mux.Handle("POST /api/software/{id}/install", s.requireAuth(s.handleSoftwareInstall))
	mux.Handle("POST /api/software/{id}/remove", s.requireAuth(s.handleSoftwareRemove))

	mux.Handle("GET /api/system/history", s.requireAuth(s.handleHistory))
	mux.Handle("GET /api/alerts", s.requireAuth(s.handleGetAlerts))
	mux.Handle("PUT /api/alerts", s.requireAuth(s.handleSaveAlerts))
	mux.Handle("POST /api/alerts/test", s.requireAuth(s.handleTestAlerts))

	mux.Handle("POST /api/sites/{id}/app/{action}", s.requireAuth(s.handleAppAction))
	mux.Handle("GET /api/sites/{id}/app/logs", s.requireAuth(s.handleAppLogs))
	mux.Handle("GET /api/docker", s.requireAuth(s.handleDocker))
	mux.Handle("POST /api/docker/run", s.requireAuth(s.handleDockerRun))
	mux.Handle("POST /api/docker/containers/{id}/{action}", s.requireAuth(s.handleDockerAction))
	mux.Handle("GET /api/docker/containers/{id}/logs", s.requireAuth(s.handleDockerLogs))
	mux.Handle("GET /api/docker/containers/{id}/terminal", s.requireAuth(s.handleContainerTerminal))
	mux.Handle("GET /api/docker/images", s.requireAuth(s.handleListImages))
	mux.Handle("POST /api/docker/images/remove", s.requireAuth(s.handleRemoveImage))
	mux.Handle("POST /api/docker/images/prune", s.requireAuth(s.handlePruneImages))
	mux.Handle("GET /api/docker/compose", s.requireAuth(s.handleListCompose))
	mux.Handle("POST /api/docker/compose", s.requireAuth(s.handleSaveCompose))
	mux.Handle("GET /api/docker/compose/{name}", s.requireAuth(s.handleGetCompose))
	mux.Handle("PUT /api/docker/compose/{name}", s.requireAuth(s.handleSaveCompose))
	mux.Handle("DELETE /api/docker/compose/{name}", s.requireAuth(s.handleDeleteCompose))
	mux.Handle("POST /api/docker/compose/{name}/{action}", s.requireAuth(s.handleComposeAction))
	mux.Handle("GET /api/docker/compose/{name}/logs", s.requireAuth(s.handleComposeLogs))

	mux.Handle("GET /api/terminal", s.requireAuth(s.handleTerminal))

	mux.Handle("GET /api/jobs", s.requireAuth(s.handleListJobs))
	mux.Handle("GET /api/jobs/{id}", s.requireAuth(s.handleGetJob))
	// The UI frame: running job without logs, and this server's addresses (ui_summary.go).
	mux.Handle("GET /api/jobs/summary", s.requireAuth(s.handleJobSummary))
	mux.Handle("GET /api/system/addresses", s.requireAuth(s.handleAddresses))
	// Reboot the server, restart the panel (power.go).
	mux.Handle("POST /api/system/reboot", s.requireAuth(s.handleReboot))
	mux.Handle("POST /api/panel/restart", s.requireAuth(s.handlePanelRestart))
	// The System page: OS updates, swap, time and name, BBR, cleanup (ostools.go).
	mux.Handle("GET /api/system/updates", s.requireAuth(s.handleOSUpdates))
	mux.Handle("POST /api/system/updates/check", s.requireAuth(s.handleOSUpdatesCheck))
	mux.Handle("POST /api/system/updates/install", s.requireAuth(s.handleOSUpgrade))
	mux.Handle("PUT /api/system/updates/auto", s.requireAuth(s.handleOSAutoUpdates))
	mux.Handle("GET /api/system/swap", s.requireAuth(s.handleSwap))
	mux.Handle("POST /api/system/swap", s.requireAuth(s.handleCreateSwap))
	mux.Handle("DELETE /api/system/swap", s.requireAuth(s.handleRemoveSwap))
	mux.Handle("PUT /api/system/swap/swappiness", s.requireAuth(s.handleSwappiness))
	mux.Handle("GET /api/system/time", s.requireAuth(s.handleSystemTime))
	mux.Handle("GET /api/system/timezones", s.requireAuth(s.handleTimezones))
	mux.Handle("PUT /api/system/timezone", s.requireAuth(s.handleSetTimezone))
	mux.Handle("PUT /api/system/ntp", s.requireAuth(s.handleSetNTP))
	mux.Handle("PUT /api/system/hostname", s.requireAuth(s.handleSetHostname))
	mux.Handle("GET /api/system/network", s.requireAuth(s.handleNetwork))
	mux.Handle("PUT /api/system/network/bbr", s.requireAuth(s.handleSetBBR))
	mux.Handle("GET /api/system/cleanup", s.requireAuth(s.handleCleanup))
	mux.Handle("POST /api/system/cleanup/{item}", s.requireAuth(s.handleClean))

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found")
	})
	mux.Handle("/adminer/", s.requireAuth(s.handleAdminer))
	mux.Handle("/", s.staticHandler())

	var h http.Handler = mux
	h = requireCustomHeader(h)
	h = http.NewCrossOriginProtection().Handler(h)
	h = securityHeaders(h)
	return s.entrance(h)
}

// WaitJobs waits for running background jobs, such as a package
// installation, until ctx is done.
func (s *Server) WaitJobs(ctx context.Context) { s.jobs.Wait(ctx) }

// RefreshConfig brings the web server configuration up to date with this
// version's templates. It is called once at start-up.
func (s *Server) RefreshConfig(ctx context.Context) {
	n, err := s.hosting.Refresh(ctx)
	if err != nil {
		s.log.Error("refresh web server configuration", "err", err)
	} else if n > 0 {
		s.log.Info("updated web server configuration files", "files", n)
	}
	// fail2ban's jails follow the SSH and panel ports.
	if changed, err := s.security.RefreshFail2ban(ctx); err != nil {
		s.log.Error("refresh fail2ban configuration", "err", err)
	} else if changed {
		s.log.Info("updated the fail2ban jails")
	}
}

// RunHtaccessWatch applies changed .htaccess rewrite rules of the websites
// on an OpenLiteSpeed server, until ctx is done (it idles on Nginx and
// Apache servers).
func (s *Server) RunHtaccessWatch(ctx context.Context) { s.hosting.WatchHtaccess(ctx) }

// RunMaintenance removes expired sessions and old audit entries and renews
// website certificates every hour until ctx is cancelled.
func (s *Server) RunMaintenance(ctx context.Context) {
	// Warm asks apt a slow question (seconds); in its own goroutine, so
	// it does not hold up the first maintenance pass (expired sites etc.).
	go s.hosting.Warm(ctx)
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		now := time.Now()
		if _, err := s.store.PurgeExpiredSessions(ctx, now); err != nil && ctx.Err() == nil {
			s.log.Warn("purge sessions", "err", err)
		}
		if _, err := s.store.PurgeAuditBefore(ctx, now.AddDate(0, 0, -90)); err != nil && ctx.Err() == nil {
			s.log.Warn("purge audit log", "err", err)
		}
		s.renewCertificates(ctx)
		s.expireSites(ctx)
		s.cron.TrimLogs()
		if n, err := s.files.PurgeTrash(ctx, files.TrashMaxAge); err != nil && ctx.Err() == nil {
			s.log.Warn("purge trash", "err", err)
		} else if n > 0 {
			s.log.Info("removed old trash entries", "entries", n)
		}
		s.renewPanelCertificate(ctx)
		s.refreshCatalog(ctx, false)
		s.checkOSUpdates(ctx) // apt-get update at most every few hours (ostools.go)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// entrance hides everything outside /<entrance>/. Requests elsewhere get a
// bare 404, so the server does not reveal that a panel is running.
func (s *Server) entrance(next http.Handler) http.Handler {
	prefix := "/" + s.cfg.Entrance
	inner := http.StripPrefix(prefix, next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == prefix:
			http.Redirect(w, r, prefix+"/", http.StatusFound)
		case strings.HasPrefix(r.URL.Path, prefix+"/"):
			inner.ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		// no-referrer also keeps the secret entrance out of other sites' logs.
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		// Adminer sends its own, nonce-based policy.
		if !isAdminer(r) {
			h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; "+
				"style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; "+
				"object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

// requireCustomHeader is a second CSRF defence next to
// http.CrossOriginProtection: browsers cannot send a custom header to another
// origin without a CORS preflight, and the panel never answers preflights.
func requireCustomHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet, r.Method == http.MethodHead, r.Method == http.MethodOptions:
		case isAdminer(r):
			// Adminer's plain HTML forms cannot send the header; it has its own
			// CSRF token, and http.CrossOriginProtection still applies.
		default:
			if r.Header.Get("X-Requested-With") != "panel" {
				writeError(w, http.StatusForbidden, "csrf")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// staticHandler serves the embedded single-page app.
func (s *Server) staticHandler() http.Handler {
	files := web.FS()
	_, err := fs.Stat(files, "index.html")
	hasUI := err == nil
	fileServer := http.FileServerFS(files)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !hasUI {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(uiNotBuilt))
			return
		}
		switch {
		case r.URL.Path == "/":
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFileFS(w, r, files, "index.html")
		case strings.HasSuffix(r.URL.Path, "/"):
			http.NotFound(w, r) // no directory listings
		default:
			if strings.HasPrefix(r.URL.Path, "/assets/") {
				// Vite puts a content hash in every asset name.
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			fileServer.ServeHTTP(w, r)
		}
	})
}

const uiNotBuilt = `<!doctype html><meta charset="utf-8"><title>Nefo</title>
<body style="font-family:system-ui,sans-serif;max-width:40rem;margin:4rem auto;padding:0 1rem">
<h1>The API is running, but the UI is not built</h1>
<p>Run <code>npm ci &amp;&amp; npm run build</code> in <code>web/</code>, then rebuild the binary.
During development, use <code>npm run dev</code> in <code>web/</code> instead.</p>`

// ---- helpers ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

func (s *Server) internalError(w http.ResponseWriter, r *http.Request, err error) {
	s.log.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	writeError(w, http.StatusInternalServerError, "internal")
}

// decodeJSON reads a small JSON body into v and rejects unknown fields.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	return decodeJSONLimit(w, r, v, 1<<20)
}

// decodeJSONLimit is decodeJSON with a custom body size limit.
func decodeJSONLimit(w http.ResponseWriter, r *http.Request, v any, limit int64) bool {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request")
		return false
	}
	return true
}

// clientIP is the TCP peer address. X-Forwarded-For is deliberately ignored:
// the panel is reached directly, and trusting that header would let anyone
// dodge the login rate limit.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// limiterKey groups IPv6 clients by /64, since one user usually controls a whole /64.
func limiterKey(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return ip
	}
	if addr.Is6() && !addr.Is4In6() {
		if p, err := addr.Prefix(64); err == nil {
			return p.String()
		}
	}
	return addr.Unmap().String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func (s *Server) audit(ctx context.Context, username, action, ip, detail string) {
	s.log.Info("audit", "user", username, "action", action, "ip", ip)
	if text, ok := alertEvents[action]; ok {
		s.monitor.Event(ctx, text+": "+detail)
	}
	if err := s.store.Audit(ctx, truncate(username, 64), action, ip, detail); err != nil {
		s.log.Warn("write audit log", "err", err)
	}
}

// rateLimited answers 429 and returns true when the client is locked out.
func (s *Server) rateLimited(w http.ResponseWriter, r *http.Request) bool {
	ok, wait := s.limiter.Check(limiterKey(clientIP(r)), s.now())
	if ok {
		return false
	}
	// Only in the journal, not the audit log, which a flood would fill:
	// fail2ban's panel jail counts these, and bans an address that keeps
	// trying while it is locked out.
	s.log.Info("audit", "user", "", "action", "login_blocked", "ip", clientIP(r))
	secs := int(wait.Seconds()) + 1
	w.Header().Set("Retry-After", itoa(secs))
	writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "too_many_attempts", "retry_after": secs})
	return true
}

func (s *Server) recordFailure(r *http.Request) {
	s.limiter.Fail(limiterKey(clientIP(r)), s.now())
}

var errNoSession = errors.New("no session")

func sprintf(format string, args ...any) string { return fmt.Sprintf(format, args...) }
