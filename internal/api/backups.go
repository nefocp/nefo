package api

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"time"

	"github.com/nefocp/nefo/internal/backup"
	"github.com/nefocp/nefo/internal/jobs"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

type backupResponse struct {
	ID        int64     `json:"id"`
	Kind      string    `json:"kind"`
	Target    string    `json:"target"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	Local     bool      `json:"local"`
	Remote    bool      `json:"remote"`
	Remotes   []string  `json:"remotes"`   // "s3", "sftp", "gdrive": where copies are
	Encrypted []string  `json:"encrypted"` // the remotes whose copy is encrypted
	Scheduled bool      `json:"scheduled"`
	CreatedAt time.Time `json:"created_at"`
	App       string    `json:"app,omitempty"` // app store app of an app backup
	// Exists tells whether the website, database or app is on this
	// server; a restore creates a missing website or database again.
	Exists bool `json:"exists"`
}

type scheduleResponse struct {
	ID        int64      `json:"id"`
	Kind      string     `json:"kind"`
	Target    string     `json:"target"`
	Frequency string     `json:"frequency"`
	Hour      int        `json:"hour"`
	Weekday   int        `json:"weekday"`
	Keep      int        `json:"keep"`
	Remote    bool       `json:"remote"`
	LastRunAt *time.Time `json:"last_run_at,omitempty"`
	LastError string     `json:"last_error,omitempty"`
}

func (s *Server) handleListBackups(w http.ResponseWriter, r *http.Request, a *authed) {
	ctx := r.Context()
	list, err := s.store.ListBackups(ctx)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	sites, _ := s.store.ListSites(ctx)
	dbs, _ := s.store.ListDatabases(ctx)
	if pgdbs, err := s.store.ListPGDatabases(ctx); err == nil {
		dbs = append(dbs, pgdbs...)
	}
	apps, _ := s.store.ListApps(ctx)
	exists := map[string]bool{}
	for _, site := range sites {
		exists[backup.KindSite+" "+site.Domain] = true
	}
	for _, d := range dbs {
		exists[backup.KindDatabase+" "+d.Name] = true
	}
	for _, in := range apps {
		exists[backup.KindApp+" "+in.Name] = true
	}
	backups := make([]backupResponse, 0, len(list))
	folders := map[string]bool{}
	for _, b := range list {
		_, statErr := os.Stat(b.File)
		backups = append(backups, backupResponse{b.ID, b.Kind, b.Target, filepath.Base(b.File), b.Size,
			statErr == nil, b.RemoteKey != "", backup.Remotes(b), backup.EncryptedRemotes(b), b.ScheduleID != 0, b.CreatedAt, b.AppID,
			b.Kind == backup.KindFolder || exists[b.Kind+" "+b.Target]})
		if b.Kind == backup.KindFolder {
			folders[b.Target] = true
		}
	}
	scheds, err := s.store.ListSchedules(ctx)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	schedules := make([]scheduleResponse, 0, len(scheds))
	for _, sc := range scheds {
		sr := scheduleResponse{sc.ID, sc.Kind, sc.Target, sc.Frequency, sc.Hour, sc.Weekday, sc.Keep, sc.Remote, nil, sc.LastError}
		if !sc.LastRunAt.IsZero() {
			t := sc.LastRunAt
			sr.LastRunAt = &t
		}
		schedules = append(schedules, sr)
		if sc.Kind == backup.KindFolder {
			folders[sc.Target] = true
		}
	}
	storage, err := s.backup.Storage(ctx)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	sftpStorage, err := s.backup.SFTPStorage(ctx)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	driveStorage, err := s.backup.DriveStorage(ctx)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	remotes, err := s.backup.ConfiguredRemotes(ctx)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	// Folders have no list of their own: those backed up before are suggested.
	targets := map[string][]string{"site": {}, "database": {}, "folder": {}, "app": {}}
	for f := range folders {
		targets["folder"] = append(targets["folder"], f)
	}
	sort.Strings(targets["folder"])
	for _, site := range sites {
		targets["site"] = append(targets["site"], site.Domain)
	}
	for _, d := range dbs {
		targets["database"] = append(targets["database"], d.Name)
	}
	sort.Strings(targets["database"]) // MariaDB and PostgreSQL together
	// App store instances (Docker and service apps), by instance name.
	for _, in := range apps {
		targets["app"] = append(targets["app"], in.Name)
	}
	sort.Strings(targets["app"])
	_, offset := time.Now().Zone()
	writeJSON(w, http.StatusOK, map[string]any{
		"backups": backups, "schedules": schedules, "storage": storage, "sftp": sftpStorage, "remotes": remotes,
		"gdrive": driveStorage, "gdrive_default_folder": backup.DefaultDriveFolder(),
		"gdrive_builtin_client": backup.BuiltinDriveClient(),
		"targets":               targets,
		"server_time":           time.Now(), "server_utc_offset": offset,
	})
}

// runBackup makes one backup; for a schedule it also prunes old backups and
// records the result.
func (s *Server) runBackup(ctx context.Context, kind, target string, sc *store.Schedule, remote bool, lg backup.Logger) error {
	var scheduleID int64
	if sc != nil {
		scheduleID = sc.ID
	}
	b, err := s.backup.Backup(ctx, kind, target, scheduleID, remote, lg)
	if sc != nil {
		if b != nil {
			if perr := s.backup.Prune(ctx, sc, lg); perr != nil && err == nil {
				err = perr
			}
		}
		msg := ""
		if err != nil {
			msg = truncate(err.Error(), 1000)
		}
		s.store.ScheduleRan(context.WithoutCancel(ctx), sc.ID, time.Now(), msg)
	}
	return err
}

func (s *Server) handleCreateBackup(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Kind   string `json:"kind"`
		Target string `json:"target"`
		Remote bool   `json:"remote"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Kind != backup.KindSite && req.Kind != backup.KindDatabase && req.Kind != backup.KindFolder && req.Kind != backup.KindApp {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	if err := s.backup.CheckTarget(r.Context(), req.Kind, req.Target); err != nil {
		s.opError(w, r, err)
		return
	}
	user, ip := a.user.Username, clientIP(r)
	job, err := s.jobs.Start("backup", req.Target, 6*time.Hour, func(ctx context.Context, j *jobs.Job) error {
		err := s.runBackup(ctx, req.Kind, req.Target, nil, req.Remote, j)
		action := "backup_created"
		if err != nil {
			action = "backup_failed"
		}
		s.audit(ctx, user, action, ip, req.Target)
		return err
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

func (s *Server) handleRestoreBackup(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	var req struct {
		Confirm string `json:"confirm"`
		// Passphrase decrypts an encrypted remote copy (optional: the
		// storage's own passphrases are tried too).
		Passphrase string `json:"passphrase"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	b, err := s.store.BackupByID(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	if req.Confirm != b.Target {
		s.opError(w, r, usererr.New("confirm_mismatch"))
		return
	}
	user, ip := a.user.Username, clientIP(r)
	job, err := s.jobs.Start("restore", b.Target, 6*time.Hour, func(ctx context.Context, j *jobs.Job) error {
		// A website whose files are locked is unlocked for the restore and
		// locked again after it (see hosting/filelock.go).
		restore := func() error {
			return s.backup.RestoreWith(ctx, id, backup.RestoreOptions{Passphrase: req.Passphrase}, j)
		}
		var err error
		switch b.Kind {
		case "site":
			if site, serr := s.siteByDomain(ctx, b.Target); serr == nil {
				err = s.hosting.WithUnlocked(ctx, site.ID, j, restore)
			} else {
				err = restore()
			}
		case "folder":
			err = s.hosting.UnlockedFor(ctx, b.Target, j, restore)
		default:
			err = restore()
		}
		action := "backup_restored"
		if err != nil {
			action = "restore_failed"
		}
		s.audit(ctx, user, action, ip, b.Target+" "+filepath.Base(b.File))
		return err
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

func (s *Server) handleUploadBackup(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	b, err := s.store.BackupByID(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	job, err := s.jobs.Start("upload_backup", b.Target, 6*time.Hour, func(ctx context.Context, j *jobs.Job) error {
		return s.backup.Upload(ctx, id, j)
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

func (s *Server) handleDownloadBackup(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	b, err := s.store.BackupByID(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	f, err := os.Open(b.File)
	if err != nil {
		s.opError(w, r, usererr.New("backup_file_missing", b.File))
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		s.opError(w, r, err)
		return
	}
	http.NewResponseController(w).SetWriteDeadline(time.Now().Add(6 * time.Hour))
	h := w.Header()
	h.Set("Content-Type", "application/octet-stream")
	h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(b.File)}))
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	s.audit(r.Context(), a.user.Username, "backup_downloaded", clientIP(r), filepath.Base(b.File))
	http.ServeContent(w, r, "", fi.ModTime(), f)
}

func (s *Server) handleDeleteBackup(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	b, err := s.store.BackupByID(r.Context(), id)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	if err := s.backup.Delete(r.Context(), id); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "backup_deleted", clientIP(r), filepath.Base(b.File))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleCreateSchedule(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Kind      string `json:"kind"`
		Target    string `json:"target"`
		Frequency string `json:"frequency"`
		Hour      int    `json:"hour"`
		Weekday   int    `json:"weekday"`
		Keep      int    `json:"keep"`
		Remote    bool   `json:"remote"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	sc := &store.Schedule{Kind: req.Kind, Target: req.Target, Frequency: req.Frequency, Hour: req.Hour,
		Weekday: req.Weekday, Keep: req.Keep, Remote: req.Remote}
	if err := s.backup.CheckSchedule(r.Context(), sc); err != nil {
		s.opError(w, r, err)
		return
	}
	if err := s.store.CreateSchedule(r.Context(), sc); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "schedule_created", clientIP(r), sc.Target)
	writeJSON(w, http.StatusCreated, map[string]int64{"id": sc.ID})
}

func (s *Server) handleDeleteSchedule(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	if err := s.store.DeleteSchedule(r.Context(), id); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "schedule_deleted", clientIP(r), "")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSaveStorage(w http.ResponseWriter, r *http.Request, a *authed) {
	var req backup.S3Config
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.backup.SaveStorage(r.Context(), req); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "storage_saved", clientIP(r), "S3 "+req.Bucket)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleRemoveStorage(w http.ResponseWriter, r *http.Request, a *authed) {
	if err := s.backup.RemoveStorage(r.Context()); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "storage_removed", clientIP(r), "S3")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleSaveSFTP is "Test and save" for SFTP storage: it connects, trusts
// the host key the server presents, tests a file and stores the settings.
func (s *Server) handleSaveSFTP(w http.ResponseWriter, r *http.Request, a *authed) {
	var req backup.SFTPConfig
	if !decodeJSON(w, r, &req) {
		return
	}
	// Connecting and testing can take longer than the server's write timeout.
	http.NewResponseController(w).SetWriteDeadline(time.Now().Add(2 * time.Minute))
	res, err := s.backup.SaveSFTP(r.Context(), req)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	v := res.SFTP
	detail := "SFTP " + v.Username + "@" + v.Host + " " + v.HostKeyFingerprint
	if res.PreviousFingerprint != "" {
		detail += " (host key changed from " + res.PreviousFingerprint + ")"
	}
	s.audit(r.Context(), a.user.Username, "storage_saved", clientIP(r), detail)
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleRemoveSFTP(w http.ResponseWriter, r *http.Request, a *authed) {
	if err := s.backup.RemoveSFTP(r.Context()); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "storage_removed", clientIP(r), "SFTP")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Backups found on a storage (a new server recovering an earlier one's):
//
//	POST /api/backups/remote-scan    {remote, passphrase}: lists and reads them, as a job
//	GET  /api/backups/remote-scan?remote=sftp  the last scan's result
//	POST /api/backups/remote-import  {remote, keys}: adds them to the backup list

func (s *Server) handleRemoteScan(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Remote     string `json:"remote"`
		Passphrase string `json:"passphrase"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if !slices.Contains(backup.RemoteNames(), req.Remote) {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	names, err := s.backup.ConfiguredRemotes(r.Context())
	if err != nil {
		s.opError(w, r, err)
		return
	}
	if !slices.Contains(names, req.Remote) {
		s.opError(w, r, usererr.New("storage_not_configured"))
		return
	}
	job, err := s.jobs.Start("scan_storage", backup.RemoteLabel(req.Remote), time.Hour, func(ctx context.Context, j *jobs.Job) error {
		res, err := s.backup.ScanRemote(ctx, req.Remote, req.Passphrase, j)
		if err != nil {
			return err
		}
		readable := 0
		for _, f := range res.Found {
			if f.Readable {
				readable++
			}
		}
		j.Logf("Found %d backups that are not in the list; %d of them could be read", len(res.Found), readable)
		return nil
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, job.Snapshot())
}

func (s *Server) handleRemoteFound(w http.ResponseWriter, r *http.Request, a *authed) {
	l := s.backup.FoundOn(r.URL.Query().Get("remote"))
	if l == nil {
		writeJSON(w, http.StatusOK, map[string]any{"found": nil})
		return
	}
	writeJSON(w, http.StatusOK, l)
}

func (s *Server) handleRemoteImport(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Remote string   `json:"remote"`
		Keys   []string `json:"keys"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.Keys) == 0 || len(req.Keys) > 2000 {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	ids, err := s.backup.ImportFound(r.Context(), req.Remote, req.Keys)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "backups_imported", clientIP(r), fmt.Sprintf("%d from %s", len(ids), req.Remote))
	writeJSON(w, http.StatusOK, map[string]any{"ids": ids})
}

// Google Drive storage:
//
//	POST   /api/backups/gdrive/connect  {client_id, client_secret, folder, encrypt, encrypt_passphrase}
//	                                    asks Google for a code to enter at google.com/device
//	GET    /api/backups/gdrive/connect  the sign-in's state; once done, the settings are saved
//	DELETE /api/backups/gdrive/connect  stops waiting for the code
//	PUT    /api/backups/gdrive          {folder, encrypt, encrypt_passphrase}: changes without signing in again
//	DELETE /api/backups/gdrive          forgets the settings and gives up the access

func (s *Server) handleDriveConnect(w http.ResponseWriter, r *http.Request, a *authed) {
	var req backup.DriveConfig
	if !decodeJSON(w, r, &req) {
		return
	}
	user, ip := a.user.Username, clientIP(r)
	st, err := s.backup.StartDriveConnect(r.Context(), req, func(v *backup.DriveView) {
		s.audit(context.Background(), user, "storage_saved", ip, "Google Drive "+v.Account+" "+v.Folder)
	})
	if err != nil {
		s.opError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleDriveConnectStatus(w http.ResponseWriter, r *http.Request, a *authed) {
	st := s.backup.DriveConnectStatus()
	if st == nil {
		writeJSON(w, http.StatusOK, map[string]string{"state": "none"})
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleDriveConnectCancel(w http.ResponseWriter, r *http.Request, a *authed) {
	s.backup.CancelDriveConnect()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSaveDrive(w http.ResponseWriter, r *http.Request, a *authed) {
	var req backup.DriveConfig
	if !decodeJSON(w, r, &req) {
		return
	}
	http.NewResponseController(w).SetWriteDeadline(time.Now().Add(4 * time.Minute))
	v, err := s.backup.SaveDrive(r.Context(), req)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "storage_saved", clientIP(r), "Google Drive "+v.Account+" "+v.Folder)
	writeJSON(w, http.StatusOK, map[string]any{"gdrive": v})
}

func (s *Server) handleRemoveDrive(w http.ResponseWriter, r *http.Request, a *authed) {
	if err := s.backup.RemoveDrive(r.Context()); err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "storage_removed", clientIP(r), "Google Drive")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// RunScheduler starts due scheduled backups every minute until ctx is done.
// When another job is running, a due backup waits for the next minute.
func (s *Server) RunScheduler(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		scheds, err := s.store.ListSchedules(ctx)
		if err != nil {
			continue
		}
		now := time.Now()
		for _, sc := range scheds {
			if !backup.Due(sc, now) {
				continue
			}
			sc := sc
			_, err := s.jobs.Start("backup", sc.Target, 6*time.Hour, func(ctx context.Context, j *jobs.Job) error {
				j.Logf("Scheduled backup (%s at %02d:00)", sc.Frequency, sc.Hour)
				err := s.runBackup(ctx, sc.Kind, sc.Target, sc, sc.Remote, j)
				action := "backup_created"
				if err != nil {
					action = "backup_failed"
				}
				s.audit(ctx, "system", action, "", sc.Target)
				return err
			})
			if err != nil {
				break // busy: try again next minute
			}
		}
	}
}
