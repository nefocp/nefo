package database

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/usererr"
)

const (
	// slowConfName keeps the slow query log settings across restarts, in
	// the engine's drop-in folder (see Manager.slowConfPath).
	slowConfName = "99-panel-slowlog.cnf"
	// slowLogFile is MariaDB's, in the directory Debian's logrotate rule for
	// MariaDB covers (/var/log/mysql/*.log); see Engine.slowLogFile.
	slowLogFile = "/var/log/mysql/mariadb-slow.log"

	// MinSlowThreshold and MaxSlowThreshold bound the slow query threshold,
	// in seconds.
	MinSlowThreshold = 0.1
	MaxSlowThreshold = 60.0

	// SlowQueryLimit is the number of entries the viewer shows.
	SlowQueryLimit = 200
	// slowQueryMax is the length of a query's text in the viewer, in characters.
	slowQueryMax = 2000
	// slowTail is how much of the end of the log is read for the viewer.
	slowTail = 4 << 20
)

// SlowLog describes the database server's slow query log.
type SlowLog struct {
	Running   bool    `json:"running"`
	Enabled   bool    `json:"enabled"`
	Threshold float64 `json:"threshold"` // seconds
	File      string  `json:"file"`
	Size      int64   `json:"size"`
}

// SlowLogStatus reads the slow query log settings: from the server while
// it runs, otherwise from the panel's drop-in.
func (m *Manager) SlowLogStatus(ctx context.Context) (*SlowLog, error) {
	if err := m.ready(ctx); err != nil {
		return nil, err
	}
	file := m.engine().slowLogFile()
	sl := &SlowLog{File: file, Threshold: 10}
	if fi, err := os.Lstat(m.slowLogPath()); err == nil && fi.Mode().IsRegular() {
		sl.Size = fi.Size()
	}
	if b, err := os.ReadFile(m.slowConfPath()); err == nil {
		sl.Enabled, sl.Threshold = parseSlowConf(string(b))
	}
	sl.Running = m.Status(ctx).Running
	if !sl.Running {
		return sl, nil
	}
	out, err := m.sql(ctx, "SELECT @@global.slow_query_log, @@global.long_query_time, @@global.slow_query_log_file;")
	if err != nil {
		return nil, err
	}
	f := strings.Split(strings.TrimSpace(out), "\t")
	if len(f) != 3 {
		return nil, fmt.Errorf("unexpected answer from %s: %q", m.engine().Name(), out)
	}
	sl.Threshold, _ = strconv.ParseFloat(f[1], 64)
	// The log only counts as on when it is written to the panel's file,
	// which is the one the viewer reads.
	sl.Enabled = f[0] == "1" && f[2] == file
	return sl, nil
}

func parseSlowConf(s string) (enabled bool, threshold float64) {
	threshold = 10
	for _, line := range strings.Split(s, "\n") {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "slow_query_log":
			enabled = strings.TrimSpace(v) == "1"
		case "long_query_time":
			if t, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
				threshold = t
			}
		}
	}
	return enabled, threshold
}

// formatThreshold checks a threshold in seconds and formats it for SQL and
// the drop-in, rounded to milliseconds.
func formatThreshold(t float64) (string, error) {
	if !(t >= MinSlowThreshold && t <= MaxSlowThreshold) { // also refuses NaN
		return "", usererr.New("invalid_slow_threshold", strconv.FormatFloat(t, 'f', -1, 64))
	}
	return strconv.FormatFloat(math.Round(t*1000)/1000, 'f', -1, 64), nil
}

func slowConf(enabled bool, threshold string) string {
	return slowConfFor(slowLogFile, enabled, threshold)
}

// slowConfFor is the drop-in for a server that writes the log to file.
func slowConfFor(file string, enabled bool, threshold string) string {
	on := "0"
	if enabled {
		on = "1"
	}
	return `# Managed by the panel. Do not edit: the panel rewrites this file.
[mysqld]
slow_query_log = ` + on + `
slow_query_log_file = ` + file + `
long_query_time = ` + threshold + "\n"
}

// slowLogSQL returns the statements that apply the settings to the running
// server. long_query_time applies to connections opened afterwards.
func slowLogSQL(enabled bool, threshold string) string {
	return slowLogSQLFor(slowLogFile, enabled, threshold)
}

func slowLogSQLFor(file string, enabled bool, threshold string) string {
	q := "SET GLOBAL long_query_time = " + threshold + ";\n"
	if enabled {
		return q + "SET GLOBAL slow_query_log_file = '" + file + "';\nSET GLOBAL slow_query_log = ON;\n"
	}
	return q + "SET GLOBAL slow_query_log = OFF;\n"
}

// SetSlowLog turns the slow query log on or off with a threshold in
// seconds. The change applies at once and is kept in a drop-in, so it
// survives restarts.
func (m *Manager) SetSlowLog(ctx context.Context, enabled bool, threshold float64) error {
	t, err := formatThreshold(threshold)
	if err != nil {
		return err
	}
	if err := m.ready(ctx); err != nil {
		return err
	}
	if !m.Status(ctx).Running {
		return usererr.New("db_not_running")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	file := m.engine().slowLogFile()
	if enabled {
		// The server writes the log as the mysql user. These are the owner
		// and mode Debian's MariaDB package gives the directory; MySQL's
		// own error log in it stays writable for it.
		if _, err := m.run.Output(ctx, "install", "-d", "-o", "mysql", "-g", "adm", "-m", "2750", path.Dir(file)); err != nil {
			return err
		}
	}
	if _, err := m.sql(ctx, slowLogSQLFor(file, enabled, t)); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(m.slowConfPath(), []byte(slowConfFor(file, enabled, t)), 0o644)
}

// openSlowLog opens the slow log for reading or writing. The directory
// belongs to the mysql user, so the file must be a plain file, not a link
// that points the root-owned panel somewhere else.
func (m *Manager) openSlowLog(flag int) (*os.File, error) {
	name := m.slowLogPath()
	before, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", name)
	}
	f, err := os.OpenFile(name, flag, 0)
	if err != nil {
		return nil, err
	}
	after, err := f.Stat()
	if err != nil || !os.SameFile(before, after) {
		f.Close()
		return nil, fmt.Errorf("%s changed while it was opened", name)
	}
	return f, nil
}

// SlowQueries returns the newest entries of the slow query log, newest
// first, at most limit of them.
func (m *Manager) SlowQueries(ctx context.Context, limit int) ([]SlowQuery, error) {
	f, err := m.openSlowLog(os.O_RDONLY)
	if errors.Is(err, fs.ErrNotExist) {
		return []SlowQuery{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	start := max(fi.Size()-slowTail, 0)
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	return parseSlowLog(io.LimitReader(f, slowTail), start > 0, limit), nil
}

// ClearSlowLog empties the slow query log. The server appends to the file, so
// it carries on at the start; FLUSH SLOW LOGS reopens it to be sure.
func (m *Manager) ClearSlowLog(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, err := m.openSlowLog(os.O_WRONLY)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	err = f.Truncate(0)
	f.Close()
	if err != nil {
		return err
	}
	if m.Status(ctx).Running {
		if _, err := m.sql(ctx, "FLUSH SLOW LOGS;"); err != nil {
			return err
		}
	}
	return nil
}

// SlowQuery is one entry of the slow query log.
type SlowQuery struct {
	Time         string  `json:"time"` // RFC 3339, UTC
	User         string  `json:"user"` // user@host
	QueryTime    float64 `json:"query_time"`
	LockTime     float64 `json:"lock_time"`
	RowsSent     int64   `json:"rows_sent"`
	RowsExamined int64   `json:"rows_examined"`
	Database     string  `json:"database"`
	Query        string  `json:"query"`
	Truncated    bool    `json:"truncated"`
}

var (
	// "root[root] @ localhost []", "shop[shop] @  [203.0.113.5]"
	userHostRe = regexp.MustCompile(`^(\S*)\[([^\]]*)\]\s+@\s+(\S*)\s*\[([^\]]*)\]`)
	// "Query_time: 0.300212", "Schema: shop", "Schema: " (no database)
	slowFieldRe = regexp.MustCompile(`([A-Za-z_]+): (\S*)`)
	timestampRe = regexp.MustCompile(`^SET (?:.*,)?timestamp=([0-9]+)[,;]`)
	useRe       = regexp.MustCompile("^use `?([^`;]+)`?;$")
)

// parseSlowLog parses MariaDB's slow query log format (MySQL's differs in
// its "# Time: 2025-11-25T14:03:07.123456Z" line and has no "Schema:"):
//
//	# Time: 251125 14:03:07
//	# User@Host: root[root] @ localhost []
//	# Thread_id: 41  Schema: shop  QC_hit: No
//	# Query_time: 0.300321  Lock_time: 0.000000  Rows_sent: 1  Rows_examined: 0
//	# Rows_affected: 0  Bytes_sent: 64
//	use shop;
//	SET timestamp=1764079387;
//	SELECT SLEEP(0.3);
//
// "# Time" is only written when the second changes, "use" when the database
// does. Each start of the server adds a three-line header. mid means r
// starts in the middle of the file, so everything before the first entry is
// skipped. The newest limit entries are returned, newest first.
func parseSlowLog(r io.Reader, mid bool, limit int) []SlowQuery {
	var (
		all      []SlowQuery
		cur      *SlowQuery
		inQuery  bool // past the header lines of cur
		lastTime time.Time
		query    strings.Builder
		long     bool // the query is longer than slowQueryMax
	)
	finish := func() {
		if cur == nil {
			return
		}
		q := strings.TrimRight(query.String(), " \t\r\n")
		q = strings.TrimSuffix(q, ";")
		if utf8.RuneCountInString(q) > slowQueryMax {
			q = string([]rune(q)[:slowQueryMax])
			long = true
		}
		cur.Query, cur.Truncated = strings.ToValidUTF8(q, "�"), long
		if cur.Time == "" && !lastTime.IsZero() {
			cur.Time = lastTime.UTC().Format(time.RFC3339)
		}
		all = append(all, *cur)
		if len(all) > 2*limit {
			all = append(all[:0], all[len(all)-limit:]...)
		}
		cur, inQuery, long = nil, false, false
		query.Reset()
	}

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	sc.Split(scanLinesLong)
	skipping := mid // in a server header, or before the first entry
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		switch {
		case strings.HasPrefix(line, "# Time: "):
			finish()
			skipping = false
			if t, ok := parseSlowTime(strings.TrimPrefix(line, "# Time: ")); ok {
				lastTime = t
			}
			continue
		case strings.HasPrefix(line, "# User@Host: "):
			finish()
			skipping = false
			cur = &SlowQuery{User: slowUser(strings.TrimPrefix(line, "# User@Host: "))}
			continue
		case strings.HasSuffix(line, ". started with:"):
			// The server (re)opened the log: a header follows.
			finish()
			skipping = true
			continue
		}
		if skipping || cur == nil {
			continue
		}
		if !inQuery {
			switch {
			case strings.HasPrefix(line, "# "):
				slowFields(cur, line)
				continue
			case useRe.MatchString(line):
				if cur.Database == "" {
					cur.Database = useRe.FindStringSubmatch(line)[1]
				}
				continue
			case timestampRe.MatchString(line):
				if ts, err := strconv.ParseInt(timestampRe.FindStringSubmatch(line)[1], 10, 64); err == nil {
					cur.Time = time.Unix(ts, 0).UTC().Format(time.RFC3339)
				}
				inQuery = true
				continue
			}
			inQuery = true // no SET timestamp line: the query starts here
		}
		if query.Len() > 4*slowQueryMax {
			long = true
			continue
		}
		if query.Len() > 0 {
			query.WriteByte('\n')
		}
		query.WriteString(line)
	}
	finish()

	if len(all) > limit {
		all = all[len(all)-limit:]
	}
	out := make([]SlowQuery, len(all))
	for i, q := range all {
		out[len(all)-1-i] = q
	}
	return out
}

// scanLinesLong is bufio.ScanLines that cuts lines longer than the buffer
// into pieces instead of failing: a huge INSERT must not end the parsing.
func scanLinesLong(data []byte, atEOF bool) (int, []byte, error) {
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	if len(data) >= 512<<10 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// parseSlowTime parses "251125 14:03:07" or "251125  9:03:07" (MariaDB,
// server time) or "2025-11-25T14:03:07.123456Z" (MySQL, UTC by default).
func parseSlowTime(s string) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(s)); err == nil {
		return t, true
	}
	f := strings.Fields(s)
	if len(f) != 2 {
		return time.Time{}, false
	}
	if len(f[1]) == 7 { // 9:03:07
		f[1] = "0" + f[1]
	}
	t, err := time.ParseInLocation("060102 15:04:05", f[0]+" "+f[1], time.Local)
	return t, err == nil
}

// slowUser turns "shop[shop] @  [203.0.113.5]" into "shop@203.0.113.5".
func slowUser(s string) string {
	mm := userHostRe.FindStringSubmatch(s)
	if mm == nil {
		return strings.TrimSpace(s)
	}
	user, host := mm[2], mm[3]
	if user == "" {
		user = mm[1]
	}
	if host == "" {
		host = mm[4]
	}
	return user + "@" + host
}

func slowFields(q *SlowQuery, line string) {
	for _, mm := range slowFieldRe.FindAllStringSubmatch(line, -1) {
		v := mm[2]
		switch mm[1] {
		case "Schema":
			q.Database = v
		case "Query_time":
			q.QueryTime, _ = strconv.ParseFloat(v, 64)
		case "Lock_time":
			q.LockTime, _ = strconv.ParseFloat(v, 64)
		case "Rows_sent":
			q.RowsSent, _ = strconv.ParseInt(v, 10, 64)
		case "Rows_examined":
			q.RowsExamined, _ = strconv.ParseInt(v, 10, 64)
		}
	}
}
