package database

import (
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Rotating the slow query log, without restarting the server: the panel's
// maintenance loop (every hour) renames the log once it is larger than
// SlowRotateMB or its oldest entry is SlowRotateDays old, has the server
// reopen it (FLUSH SLOW LOGS, which creates a new file), compresses the old
// one next to it as mariadb-slow.log-YYYYMMDD-HHMMSS.gz (mysql-slow.log-…
// on MySQL) and keeps the newest SlowRotateKeep of those.
//
// The panel does this itself rather than with a logrotate file: logrotate
// is not on every server, and its rules can't cover the same file twice
// (MariaDB's package already has /var/log/mysql/*.log, monthly with a
// minimum size of 50 MB). Those rotated names never match that rule's
// pattern, so the two can't rename the same files. At rest it costs one
// stat of the log an hour.
const (
	SlowRotateMB   = 50
	SlowRotateDays = 7
	SlowRotateKeep = 4
)

// slowRotated returns the rotated logs of the slow log, oldest first.
func (m *Manager) slowRotated() []string {
	base := m.slowLogPath()
	list, _ := filepath.Glob(base + "-[0-9]*.gz")
	sort.Strings(list) // the names hold the time
	return list
}

// SlowRotation describes the rotated slow logs, for the viewer.
type SlowRotation struct {
	MB    int   `json:"mb"`
	Days  int   `json:"days"`
	Keep  int   `json:"keep"`
	Files int   `json:"files"`
	Size  int64 `json:"size"`
}

func (m *Manager) slowRotation() SlowRotation {
	r := SlowRotation{MB: SlowRotateMB, Days: SlowRotateDays, Keep: SlowRotateKeep}
	for _, f := range m.slowRotated() {
		if fi, err := os.Lstat(f); err == nil && fi.Mode().IsRegular() {
			r.Files++
			r.Size += fi.Size()
		}
	}
	return r
}

// RotateSlowLog rotates the slow query log when it is due (see above) and
// reports whether it did. It is cheap to call often.
func (m *Manager) RotateSlowLog(ctx context.Context, now time.Time) (bool, error) {
	if m.detected() == "" {
		return false, nil
	}
	name := m.slowLogPath()
	fi, err := os.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !fi.Mode().IsRegular() || fi.Size() == 0 {
		return false, nil
	}
	due := fi.Size() >= SlowRotateMB<<20
	if !due {
		if first, ok := m.firstSlowEntry(); ok && now.Sub(first) >= SlowRotateDays*24*time.Hour {
			due = true
		}
	}
	if !due {
		return false, nil
	}
	// An upgrade or a switch holds the lock for a long time: the next
	// hour's pass rotates then, the maintenance loop does not wait.
	if !m.mu.TryLock() {
		return false, nil
	}
	defer m.mu.Unlock()
	f, err := m.openSlowLog(os.O_RDONLY) // checks it is still a plain file
	if err != nil {
		return false, err
	}
	f.Close()
	rotated := name + "-" + now.Format("20060102-150405")
	for i := 2; fileExists(rotated) || fileExists(rotated+".gz"); i++ {
		rotated = name + "-" + now.Format("20060102-150405") + "-" + strconv.Itoa(i)
	}
	if err := os.Rename(name, rotated); err != nil {
		return false, err
	}
	// The server writes on into the renamed file until it reopens the log;
	// stopped, it creates the file when it starts.
	if m.Status(ctx).Running {
		if _, err := m.sql(ctx, "FLUSH SLOW LOGS;"); err != nil {
			return true, fmt.Errorf("the slow query log was renamed to %s, but the server did not reopen it: %w", rotated, err)
		}
	}
	if err := gzipFile(rotated); err != nil {
		return true, err
	}
	list := m.slowRotated()
	for len(list) > SlowRotateKeep {
		os.Remove(list[0])
		list = list[1:]
	}
	if m.log != nil {
		m.log.Info("rotated the slow query log", "file", rotated+".gz", "size", fi.Size())
	}
	return true, nil
}

// detected is the installed engine as m's detect hook sees it
// (InstalledEngine on a real server), "" for none, without running
// anything.
func (m *Manager) detected() Engine {
	if m.detect != nil {
		return m.detect()
	}
	return hostEngine()
}

// firstSlowEntry returns the time of the oldest entry in the slow log.
func (m *Manager) firstSlowEntry() (time.Time, bool) {
	f, err := m.openSlowLog(os.O_RDONLY)
	if err != nil {
		return time.Time{}, false
	}
	defer f.Close()
	sc := bufio.NewScanner(io.LimitReader(f, 256<<10))
	sc.Buffer(make([]byte, 64<<10), 64<<10)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if t, ok := strings.CutPrefix(line, "# Time: "); ok {
			if tm, ok := parseSlowTime(t); ok {
				return tm, true
			}
		}
		if mm := timestampRe.FindStringSubmatch(line); mm != nil {
			if ts, err := strconv.ParseInt(mm[1], 10, 64); err == nil {
				return time.Unix(ts, 0), true
			}
		}
	}
	return time.Time{}, false
}

// gzipFile compresses path into path.gz and removes path.
func gzipFile(path string) error {
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	tmp := path + ".gz.tmp"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o640)
	if err != nil {
		in.Close()
		return err
	}
	gz := gzip.NewWriter(out)
	_, err = io.Copy(gz, in)
	in.Close()
	if cerr := gz.Close(); err == nil {
		err = cerr
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, path+".gz")
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Remove(path)
}
