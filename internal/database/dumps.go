package database

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/usererr"
)

// Before the panel upgrades MariaDB or switches between MariaDB and MySQL,
// it exports every database into a folder of DefaultDumpsDir, one gzipped
// SQL file per database and a manifest.json that describes them. The
// folders stay until the administrator deletes them (DeleteDump).

// DefaultDumpsDir is where those folders go, next to the panel's backups.
const DefaultDumpsDir = "/var/backups/panel/db-server"

// Dump is one folder of dumps, for the Software page.
type Dump struct {
	Name      string    `json:"name"` // the folder's name
	Kind      string    `json:"kind"` // "upgrade" or "switch"
	From      string    `json:"from"` // "MariaDB 10.11.14"
	To        string    `json:"to"`   // "MariaDB 11.4", "MySQL 8.4"
	Created   time.Time `json:"created"`
	Size      int64     `json:"size"`
	Databases []string  `json:"databases"`
	Path      string    `json:"path"`
	// Result is "done", "rolled_back" or "failed" once the job ended; ""
	// while it runs (or if the panel stopped in the middle).
	Result string `json:"result"`
}

// manifest describes a folder of dumps (manifest.json).
type manifest struct {
	Kind      string                  `json:"kind"`
	From      string                  `json:"from"`
	To        string                  `json:"to"`
	Created   time.Time               `json:"created"`
	Databases []dumpedDB              `json:"databases"`
	Accounts  []string                `json:"accounts,omitempty"` // user@host, without credentials
	Result    string                  `json:"result,omitempty"`
	Error     string                  `json:"error,omitempty"`
	Counts    map[string]objectCounts `json:"counts,omitempty"`
}

type dumpedDB struct {
	Name      string `json:"name"`
	Charset   string `json:"charset"`
	Collation string `json:"collation"`
	File      string `json:"file"`
	Size      int64  `json:"size"`
}

var dumpNameRe = regexp.MustCompile(`^[0-9]{8}-[0-9]{6}-[a-z0-9.-]{1,60}$`)

func (m *Manager) dumpsPath() string {
	if m.dumpsDir != "" {
		return m.dumpsDir
	}
	return DefaultDumpsDir
}

// newDumpDir makes a new, empty folder for dumps, readable by root only.
func (m *Manager) newDumpDir(what string) (string, error) {
	base := m.dumpsPath()
	if err := fsutil.MkdirAll(base, 0o700); err != nil {
		return "", err
	}
	name := time.Now().Format("20060102-150405") + "-" + what
	dir := filepath.Join(base, name)
	for i := 2; ; i++ {
		err := os.Mkdir(dir, 0o700)
		if err == nil {
			return dir, nil
		}
		if !errors.Is(err, fs.ErrExist) || i > 9 {
			return "", err
		}
		dir = filepath.Join(base, name+"-"+strconv.Itoa(i))
	}
}

func writeManifest(dir string, mf *manifest) error {
	b, err := json.MarshalIndent(mf, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(filepath.Join(dir, "manifest.json"), append(b, '\n'), 0o600)
}

func readManifest(dir string) (*manifest, error) {
	b, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	var mf manifest
	if err := json.Unmarshal(b, &mf); err != nil {
		return nil, err
	}
	return &mf, nil
}

// ListDumps returns the folders of dumps, newest first.
func (m *Manager) ListDumps() ([]Dump, error) {
	base := m.dumpsPath()
	entries, err := os.ReadDir(base)
	if errors.Is(err, fs.ErrNotExist) {
		return []Dump{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Dump{}
	for _, e := range entries {
		if !e.IsDir() || !dumpNameRe.MatchString(e.Name()) {
			continue
		}
		dir := filepath.Join(base, e.Name())
		mf, err := readManifest(dir)
		if err != nil {
			continue
		}
		d := Dump{Name: e.Name(), Kind: mf.Kind, From: mf.From, To: mf.To, Created: mf.Created, Path: dir,
			Result: mf.Result, Databases: []string{}}
		for _, db := range mf.Databases {
			d.Databases = append(d.Databases, db.Name)
		}
		d.Size = dirSize(dir)
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name > out[j].Name })
	return out, nil
}

// DeleteDump deletes a folder of dumps. It is refused while an upgrade or
// a switch runs.
func (m *Manager) DeleteDump(name string) error {
	if !dumpNameRe.MatchString(name) {
		return usererr.New("not_found")
	}
	if !m.mu.TryLock() {
		return usererr.New("db_busy")
	}
	defer m.mu.Unlock()
	dir := filepath.Join(m.dumpsPath(), name)
	if _, err := readManifest(dir); err != nil {
		return usererr.New("not_found")
	}
	return os.RemoveAll(dir)
}

// dirSize adds up the sizes of the regular files under dir.
func dirSize(dir string) int64 {
	var n int64
	filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if fi, err := d.Info(); err == nil {
				n += fi.Size()
			}
		}
		return nil
	})
	return n
}

// sqlString quotes s as an SQL string literal.
func sqlString(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `''`).Replace(s) + "'"
}

// sqlIdent quotes s as an SQL name.
func sqlIdent(s string) string { return "`" + strings.ReplaceAll(s, "`", "``") + "`" }

func sqlList(names []string) string {
	q := make([]string, len(names))
	for i, n := range names {
		q[i] = sqlString(n)
	}
	return strings.Join(q, ",")
}

// hexRows runs a query whose every column is HEX(…) and returns its rows
// decoded, so that names with tabs or newlines can't break the parsing.
func (m *Manager) hexRows(ctx context.Context, e Engine, query string) ([][]string, error) {
	out, err := m.sqlWith(ctx, e, query)
	if err != nil {
		return nil, err
	}
	var rows [][]string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimRight(line, "\r"); line == "" {
			continue
		}
		var row []string
		for _, f := range strings.Split(line, "\t") {
			if f == "NULL" {
				row = append(row, "")
				continue
			}
			b, err := hex.DecodeString(f)
			if err != nil {
				return nil, fmt.Errorf("unexpected answer from %s: %q", e.Name(), line)
			}
			row = append(row, string(b))
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// schemaCharsets returns the default character set and collation of each
// database.
func (m *Manager) schemaCharsets(ctx context.Context, e Engine, dbs []string) (map[string][2]string, error) {
	out := map[string][2]string{}
	if len(dbs) == 0 {
		return out, nil
	}
	rows, err := m.hexRows(ctx, e, "SELECT HEX(SCHEMA_NAME), HEX(DEFAULT_CHARACTER_SET_NAME), HEX(DEFAULT_COLLATION_NAME) "+
		"FROM information_schema.SCHEMATA WHERE SCHEMA_NAME IN ("+sqlList(dbs)+");")
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		if len(r) == 3 {
			out[r[0]] = [2]string{r[1], r[2]}
		}
	}
	return out, nil
}

// dumpDatabases exports each database into dir as name.sql.gz with the
// engine's dump program, with routines, triggers and events, and returns
// what it wrote.
func (m *Manager) dumpDatabases(ctx context.Context, e Engine, dir string, dbs []string, lg Logger) ([]dumpedDB, error) {
	cs, err := m.schemaCharsets(ctx, e, dbs)
	if err != nil {
		return nil, err
	}
	out := []dumpedDB{}
	for i, name := range dbs {
		file := fmt.Sprintf("%03d.sql.gz", i+1) // names may hold characters a file name can't
		lg.Logf("Exporting %s", name)
		size, err := m.dumpOne(ctx, e, name, filepath.Join(dir, file))
		if err != nil {
			return nil, fmt.Errorf("export %s: %w", name, err)
		}
		c := cs[name]
		out = append(out, dumpedDB{Name: name, Charset: c[0], Collation: c[1], File: file, Size: size})
	}
	return out, nil
}

func (m *Manager) dumpOne(ctx context.Context, e Engine, name, path string) (int64, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return 0, err
	}
	bw := bufio.NewWriterSize(f, 256<<10)
	gz, _ := gzip.NewWriterLevel(bw, gzip.BestSpeed)
	args := []string{"--single-transaction", "--quick", "--routines", "--triggers", "--events", "--default-character-set=utf8mb4"}
	if e == MySQL {
		args = append(args, "--set-gtid-purged=OFF")
	}
	err = m.run.Pipe(ctx, nil, gz, e.dumper(), append(args, name)...)
	if cerr := gz.Close(); err == nil {
		err = cerr
	}
	if ferr := bw.Flush(); err == nil {
		err = ferr
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return 0, err
	}
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// objectCounts is what a database holds, to check that a dump arrived
// whole: the rows of each table, and how many views, routines, triggers
// and events it has.
type objectCounts struct {
	Rows     map[string]int64 `json:"rows"`
	Views    int              `json:"views"`
	Routines int              `json:"routines"`
	Triggers int              `json:"triggers"`
	Events   int              `json:"events"`
}

// countObjects counts what each database holds (see objectCounts). It
// reads every table, so it takes as long as reading the data once.
func (m *Manager) countObjects(ctx context.Context, e Engine, dbs []string) (map[string]objectCounts, error) {
	out := map[string]objectCounts{}
	for _, db := range dbs {
		out[db] = objectCounts{Rows: map[string]int64{}}
	}
	if len(dbs) == 0 {
		return out, nil
	}
	list := sqlList(dbs)
	rows, err := m.hexRows(ctx, e, "SELECT HEX(TABLE_SCHEMA), HEX(TABLE_NAME), HEX(TABLE_TYPE) FROM information_schema.TABLES "+
		"WHERE TABLE_SCHEMA IN ("+list+") ORDER BY TABLE_SCHEMA, TABLE_NAME;")
	if err != nil {
		return nil, err
	}
	type table struct{ db, name string }
	var tables []table
	for _, r := range rows {
		if len(r) != 3 {
			continue
		}
		c := out[r[0]]
		switch r[2] {
		case "VIEW":
			c.Views++
		default:
			tables = append(tables, table{r[0], r[1]})
		}
		out[r[0]] = c
	}
	for _, q := range []struct {
		sql string
		add func(*objectCounts, int)
	}{
		{"SELECT HEX(ROUTINE_SCHEMA), HEX(CONCAT(COUNT(*))) FROM information_schema.ROUTINES WHERE ROUTINE_SCHEMA IN (" + list + ") GROUP BY ROUTINE_SCHEMA;",
			func(c *objectCounts, n int) { c.Routines = n }},
		{"SELECT HEX(TRIGGER_SCHEMA), HEX(CONCAT(COUNT(*))) FROM information_schema.TRIGGERS WHERE TRIGGER_SCHEMA IN (" + list + ") GROUP BY TRIGGER_SCHEMA;",
			func(c *objectCounts, n int) { c.Triggers = n }},
		{"SELECT HEX(EVENT_SCHEMA), HEX(CONCAT(COUNT(*))) FROM information_schema.EVENTS WHERE EVENT_SCHEMA IN (" + list + ") GROUP BY EVENT_SCHEMA;",
			func(c *objectCounts, n int) { c.Events = n }},
	} {
		rows, err := m.hexRows(ctx, e, q.sql)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			if len(r) != 2 {
				continue
			}
			n, _ := strconv.Atoi(r[1])
			c := out[r[0]]
			q.add(&c, n)
			out[r[0]] = c
		}
	}
	// COUNT(*) of every table, many tables to a statement.
	for start := 0; start < len(tables); start += 100 {
		end := min(start+100, len(tables))
		var parts []string
		for i := start; i < end; i++ {
			parts = append(parts, fmt.Sprintf("SELECT HEX(CONCAT(%d)), HEX(CONCAT(COUNT(*))) FROM %s.%s", i, sqlIdent(tables[i].db), sqlIdent(tables[i].name)))
		}
		rows, err := m.hexRows(ctx, e, strings.Join(parts, " UNION ALL ")+";")
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			if len(r) != 2 {
				continue
			}
			i, err1 := strconv.Atoi(r[0])
			n, err2 := strconv.ParseInt(r[1], 10, 64)
			if err1 != nil || err2 != nil || i < start || i >= end {
				return nil, fmt.Errorf("unexpected answer from %s while counting rows", e.Name())
			}
			out[tables[i].db].Rows[tables[i].name] = n
		}
	}
	return out, nil
}

// compareCounts returns what differs between two countObjects results,
// as sentences for the job log; nil when nothing does.
func compareCounts(before, after map[string]objectCounts) []string {
	var diff []string
	dbs := make([]string, 0, len(before))
	for db := range before {
		dbs = append(dbs, db)
	}
	sort.Strings(dbs)
	for _, db := range dbs {
		b, a := before[db], after[db]
		names := make([]string, 0, len(b.Rows))
		for t := range b.Rows {
			names = append(names, t)
		}
		sort.Strings(names)
		for _, t := range names {
			n, ok := a.Rows[t]
			switch {
			case !ok:
				diff = append(diff, fmt.Sprintf("%s: the table %s is missing", db, t))
			case n != b.Rows[t]:
				diff = append(diff, fmt.Sprintf("%s: the table %s has %d rows instead of %d", db, t, n, b.Rows[t]))
			}
		}
		for t := range a.Rows {
			if _, ok := b.Rows[t]; !ok {
				diff = append(diff, fmt.Sprintf("%s: the table %s appeared", db, t))
			}
		}
		for _, c := range []struct {
			what string
			b, a int
		}{{"views", b.Views, a.Views}, {"routines", b.Routines, a.Routines}, {"triggers", b.Triggers, a.Triggers}, {"events", b.Events, a.Events}} {
			if c.a != c.b {
				diff = append(diff, fmt.Sprintf("%s: %d %s instead of %d", db, c.a, c.what, c.b))
			}
		}
	}
	return diff
}

// pkgVersion is an installed package and its version.
type pkgVersion struct{ Name, Version string }

// installedPackages returns the installed packages matching the dpkg-query
// patterns, with their versions.
func (m *Manager) installedPackages(ctx context.Context, patterns ...string) []pkgVersion {
	out, _ := m.run.Output(ctx, "dpkg-query", append([]string{"-W", "-f", "${Package}\t${Version}\t${db:Status-Status}\n"}, patterns...)...)
	var pkgs []pkgVersion
	seen := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Split(strings.TrimSpace(line), "\t")
		if len(f) != 3 || f[2] != "installed" || seen[f[0]] || !pkgNameRe.MatchString(f[0]) || !pkgVersionRe.MatchString(f[1]) {
			continue
		}
		seen[f[0]] = true
		pkgs = append(pkgs, pkgVersion{f[0], f[1]})
	}
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].Name < pkgs[j].Name })
	return pkgs
}

var (
	pkgNameRe    = regexp.MustCompile(`^[a-z0-9][a-z0-9+.-]{0,99}$`)
	pkgVersionRe = regexp.MustCompile(`^[0-9A-Za-z.+~:-]{1,100}$`)
)

// copyTree copies a folder with its owners, modes and links (cp -a).
func (m *Manager) copyTree(ctx context.Context, from, to string) error {
	if out, err := m.run.Output(ctx, "cp", "-a", "--", from, to); err != nil {
		return fmt.Errorf("copy %s to %s: %s", from, to, strings.TrimSpace(out))
	}
	return nil
}

// gunzipFile opens a gzipped dump for reading.
func gunzipFile(path string) (io.Reader, func(), error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	gz, err := gzip.NewReader(bufio.NewReaderSize(f, 256<<10))
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	return gz, func() { gz.Close(); f.Close() }, nil
}

// diskCheck returns db_disk_space when the file system of path has less
// than needMB free.
func diskCheck(path string, needMB int64) error {
	free := freeDiskMB(path)
	if free < 0 || free >= needMB {
		return nil
	}
	return usererr.New("db_disk_space", fmt.Sprintf("%d MB, %d MB", needMB, free))
}

// existingParent returns path or its nearest folder that exists.
func existingParent(path string) string {
	for p := path; ; p = filepath.Dir(p) {
		if _, err := os.Stat(p); err == nil || p == filepath.Dir(p) {
			return p
		}
	}
}
