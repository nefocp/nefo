// Package cron manages scheduled commands. The panel writes them to
// /etc/cron.d/panel, so the system's cron runs them even when the panel is
// stopped. Each job writes its output to its own log file.
//
// A cron job's command is a shell command written by the administrator, who
// is root on this server: it is run by /bin/sh like a line typed over SSH.
// The panel only makes sure it fits on one cron line.
package cron

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
)

// Manager manages cron jobs.
type Manager struct {
	store  *store.Store
	run    run.Runner
	log    *slog.Logger
	file   string // /etc/cron.d/panel
	logDir string // /var/log/panel-cron
	mu     sync.Mutex
	chown  func(path string, uid, gid int) error
	users  func(ctx context.Context) (map[string][2]int, error) // allowed users -> uid, gid
}

// New returns a Manager. users lists the accounts jobs may run as, with
// their numeric IDs.
func New(st *store.Store, r run.Runner, log *slog.Logger, users func(ctx context.Context) (map[string][2]int, error)) *Manager {
	return &Manager{store: st, run: r, log: log, file: "/etc/cron.d/panel", logDir: "/var/log/panel-cron",
		chown: os.Chown, users: users}
}

var (
	fieldRe  = regexp.MustCompile(`^[0-9A-Za-z*/,\-]{1,64}$`)
	presets  = map[string]bool{"@reboot": true, "@yearly": true, "@annually": true, "@monthly": true, "@weekly": true, "@daily": true, "@midnight": true, "@hourly": true}
	nameRe   = regexp.MustCompile(`^[^\x00-\x1f]{1,100}$`)
	unitsMax = []int{59, 23, 31, 12, 7}
)

// ValidSchedule checks a cron schedule: a preset such as @daily, or five
// fields (minute hour day month weekday) of digits, names, * / , and -.
func ValidSchedule(s string) bool {
	s = strings.TrimSpace(s)
	if presets[s] {
		return true
	}
	f := strings.Fields(s)
	if len(f) != 5 {
		return false
	}
	for i, x := range f {
		if !fieldRe.MatchString(x) {
			return false
		}
		// Plain numbers must be in range, to catch typos such as 60 minutes.
		for _, part := range strings.FieldsFunc(x, func(r rune) bool { return r == ',' || r == '-' || r == '/' }) {
			if n, err := strconv.Atoi(part); err == nil && n > unitsMax[i] && !strings.Contains(x, "/"+part) {
				return false
			}
		}
	}
	return true
}

func validCommand(c string) bool {
	return c != "" && len(c) <= 2000 && !strings.ContainsAny(c, "\n\r\x00")
}

// Check validates a job before it is saved.
func (m *Manager) Check(ctx context.Context, c *store.CronJob) error {
	c.Name, c.Schedule, c.Command = strings.TrimSpace(c.Name), strings.Join(strings.Fields(c.Schedule), " "), strings.TrimSpace(c.Command)
	if !nameRe.MatchString(c.Name) {
		return usererr.New("invalid_cron_name")
	}
	if !ValidSchedule(c.Schedule) {
		return usererr.New("invalid_cron_schedule", c.Schedule)
	}
	if !validCommand(c.Command) {
		return usererr.New("invalid_cron_command")
	}
	users, err := m.users(ctx)
	if err != nil {
		return err
	}
	if _, ok := users[c.RunAs]; !ok || validate.SystemUsername(c.RunAs) != nil {
		return usererr.New("invalid_cron_user", c.RunAs)
	}
	return nil
}

// Save stores a job and rewrites the cron file.
func (m *Manager) Save(ctx context.Context, c *store.CronJob) error {
	if err := m.Check(ctx, c); err != nil {
		return err
	}
	if err := m.store.SaveCronJob(ctx, c); err != nil {
		return err
	}
	return m.Sync(ctx)
}

// Delete removes a job, its log and its cron line.
func (m *Manager) Delete(ctx context.Context, id int64) error {
	if err := m.store.DeleteCronJob(ctx, id); err != nil {
		return err
	}
	os.Remove(m.logFile(id))
	return m.Sync(ctx)
}

func (m *Manager) logFile(id int64) string {
	return filepath.Join(m.logDir, strconv.FormatInt(id, 10)+".log")
}

// Sync writes /etc/cron.d/panel from the stored jobs. Jobs of users that no
// longer exist are left out.
func (m *Manager) Sync(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	jobs, err := m.store.ListCronJobs(ctx)
	if err != nil {
		return err
	}
	users, err := m.users(ctx)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(m.logDir, 0o755); err != nil {
		return err
	}
	os.Chmod(m.logDir, 0o755)
	var b strings.Builder
	b.WriteString("# Managed by the panel. Do not edit: the panel rewrites this file.\n")
	b.WriteString("SHELL=/bin/sh\nPATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\n")
	for _, j := range jobs {
		ids, ok := users[j.RunAs]
		if !ok || !ValidSchedule(j.Schedule) || !validCommand(j.Command) || validate.SystemUsername(j.RunAs) != nil {
			continue
		}
		// The log file belongs to the job's user, so a site's job can write it.
		lf := m.logFile(j.ID)
		if f, err := os.OpenFile(lf, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600); err == nil {
			f.Close()
			m.chown(lf, ids[0], ids[1])
		}
		fmt.Fprintf(&b, "\n# %d: %s\n", j.ID, strings.ReplaceAll(j.Name, "\n", " "))
		prefix := ""
		if !j.Enabled {
			prefix = "# (disabled) "
		}
		// In a crontab, % means newline unless escaped.
		cmd := strings.ReplaceAll(j.Command, "%", `\%`)
		fmt.Fprintf(&b, "%s%s %s ( %s ) >> %s 2>&1\n", prefix, j.Schedule, j.RunAs, cmd, lf)
	}
	return fsutil.WriteFileAtomic(m.file, []byte(b.String()), 0o644)
}

// RunNow runs a job at once and copies its output to w.
func (m *Manager) RunNow(ctx context.Context, id int64, w io.Writer) error {
	j, err := m.store.CronJobByID(ctx, id)
	if err != nil {
		return err
	}
	if err := m.Check(ctx, j); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Hour)
	defer cancel()
	if j.RunAs == "root" {
		return m.run.Stream(ctx, w, "/bin/sh", "-c", j.Command)
	}
	return m.run.Stream(ctx, w, "runuser", "-u", j.RunAs, "--", "/bin/sh", "-c", j.Command)
}

// Log returns the end of a job's log.
func (m *Manager) Log(ctx context.Context, id int64) (string, error) {
	f, err := os.Open(m.logFile(id))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return "", err
	}
	start := fi.Size() - 64<<10
	if start < 0 {
		start = 0
	}
	b := make([]byte, fi.Size()-start)
	if _, err := f.ReadAt(b, start); err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return string(b), nil
}

// TrimLogs cuts job logs larger than 5 MB down to their last 1 MB.
func (m *Manager) TrimLogs() {
	entries, _ := os.ReadDir(m.logDir)
	for _, e := range entries {
		p := filepath.Join(m.logDir, e.Name())
		fi, err := e.Info()
		if err != nil || !fi.Mode().IsRegular() || fi.Size() < 5<<20 {
			continue
		}
		f, err := os.OpenFile(p, os.O_RDWR, 0)
		if err != nil {
			continue
		}
		keep := make([]byte, 1<<20)
		n, _ := f.ReadAt(keep, fi.Size()-int64(len(keep)))
		f.Truncate(0)
		f.WriteAt(keep[:n], 0)
		f.Close()
	}
}
