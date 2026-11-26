package postgres

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/secure"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// Imports and exports run as the database's own role. psql and pg_dump get
// a libpq connection string with the socket, the role and a password file
// (passfile=): the password is in neither the arguments nor the
// environment, where other users of the server could read it.
//
// psql runs as root, and an SQL file can hold psql's own commands (lines
// such as \! or \o) that would run shell commands or write files as root.
// So every import starts with \restrict and a random key: psql then refuses
// every backslash command (only \unrestrict with that key would end it,
// and the file can't know the key). pg_dump writes its own \restrict and
// \unrestrict lines since PostgreSQL 17.6, 16.10, 15.14 and 14.19; those
// are taken out (see importFilter). A psql older than that does not know
// \restrict and fails on the first line: the import is refused.

// connString returns the libpq connection string for a role.
func (m *Manager) connString(c cluster, db, user, passfile string) string {
	return fmt.Sprintf("host=%s port=%d dbname=%s user=%s passfile=%s",
		connValue(m.socketDir), c.port, connValue(db), connValue(user), connValue(passfile))
}

// connValue quotes a value of a libpq connection string.
func connValue(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}

// passFile writes a password file for user, readable by root only, and
// returns its path.
func (m *Manager) passFile(db, user, password string) (string, error) {
	f, err := os.CreateTemp(m.tempDir, "nefo-pg-*.pass") // mode 0600
	if err != nil {
		return "", err
	}
	esc := strings.NewReplacer(`\`, `\\`, `:`, `\:`)
	_, err = fmt.Fprintf(f, "*:*:%s:%s:%s\n", db, user, esc.Replace(password))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// owner returns the record of a database and checks its names.
func (m *Manager) owner(ctx context.Context, name string) (*store.Database, error) {
	if err := checkName(name); err != nil {
		return nil, err
	}
	d, err := m.store.PGDatabaseByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if err := checkName(d.Username); err != nil {
		return nil, err
	}
	return d, nil
}

// Dump writes a plain SQL dump of a database to w, as its own role,
// without ownership or privileges, so it can be imported as another role.
func (m *Manager) Dump(ctx context.Context, name string, w io.Writer) error {
	c, err := m.ready(ctx)
	if err != nil {
		return err
	}
	d, err := m.owner(ctx, name)
	if err != nil {
		return err
	}
	pf, err := m.passFile(d.Name, d.Username, d.Password)
	if err != nil {
		return err
	}
	defer os.Remove(pf)
	return m.run.Pipe(ctx, nil, w, m.bin(c, "pg_dump"), "--no-password", "--format=plain",
		"--no-owner", "--no-privileges", "--no-tablespaces", "--no-subscriptions", "--no-security-labels",
		"--dbname="+m.connString(c, d.Name, d.Username, pf))
}

// Import runs an SQL dump (plain pg_dump output or any SQL) against a
// database, as its own role and in a single transaction: a dump that fails
// leaves the database as it was. The role may not create or change roles,
// other databases or anything that needs a superuser, so such statements
// fail; the error says which line (see ImportError).
func (m *Manager) Import(ctx context.Context, name string, r io.Reader) error {
	c, err := m.ready(ctx)
	if err != nil {
		return err
	}
	d, err := m.owner(ctx, name)
	if err != nil {
		return err
	}
	lock := m.importLock(d.Name)
	lock.Lock()
	defer lock.Unlock()

	pf, err := m.passFile(d.Name, d.Username, d.Password)
	if err != nil {
		return err
	}
	defer os.Remove(pf)
	key := secure.String(32, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	body := importFilter(r, c.major())
	defer body.Close()
	in := io.MultiReader(strings.NewReader(`\restrict `+key+"\n"), body)
	err = m.run.Pipe(ctx, in, io.Discard, m.bin(c, "psql"), "-X", "-q", "--no-password",
		"-v", "ON_ERROR_STOP=1", "--single-transaction", "-f", "-",
		"-d", m.connString(c, d.Name, d.Username, pf))
	if err != nil {
		return importError(err)
	}
	// A role may change its own password, and a dump could do so: set the
	// panel's again, so the stored one stays right.
	if q, perr := passwordSQL(d.Username, d.Password); perr == nil {
		if _, err := m.sql(context.WithoutCancel(ctx), c, "postgres", q); err != nil {
			m.log.Warn("set the database password again after an import", "db", d.Name, "err", err)
		}
	}
	return nil
}

// importLock returns the lock for imports into the database name.
func (m *Manager) importLock(name string) *sync.Mutex {
	m.importMu.Lock()
	defer m.importMu.Unlock()
	if m.imports == nil {
		m.imports = map[string]*sync.Mutex{}
	}
	l := m.imports[name]
	if l == nil {
		l = new(sync.Mutex)
		m.imports[name] = l
	}
	return l
}

var (
	// pg_dump's own lines: \restrict <key> near the top, \unrestrict <key> at the end.
	restrictRe   = regexp.MustCompile(`^\\restrict [A-Za-z0-9]+\r?\n?$`)
	unrestrictRe = regexp.MustCompile(`^\\unrestrict [A-Za-z0-9]+\r?\n?$`)
	// pg_dump 17 and later set this, and older servers do not know it.
	txTimeoutRe = regexp.MustCompile(`^SET transaction_timeout = 0;\r?\n?$`)
)

// headerLines is how far into a dump pg_dump's \restrict and SET lines may
// be. Further down, a line could be a row of COPY data.
const headerLines = 64

// importFilter passes a dump through, with pg_dump's \restrict and
// \unrestrict lines (and, for servers before 17, its SET
// transaction_timeout) replaced by empty lines, so psql's line numbers in
// errors still match the file's, plus one for the \restrict line in front.
// Close it when done: that also ends the copy when psql stopped early.
func importFilter(r io.Reader, major int) io.ReadCloser {
	pr, pw := io.Pipe()
	go func() {
		br := bufio.NewReaderSize(r, 64<<10)
		bw := bufio.NewWriterSize(pw, 64<<10)
		n := 0        // lines started so far
		long := false // in the middle of a line longer than the buffer (COPY data, a big INSERT)
		var err error
		for {
			var chunk []byte
			chunk, err = br.ReadSlice('\n')
			full := errors.Is(err, bufio.ErrBufferFull)
			if len(chunk) > 0 {
				if !long {
					n++
					if !full && dropLine(chunk, n, major) {
						chunk = []byte("\n")
					}
				}
				if _, werr := bw.Write(chunk); werr != nil {
					err = werr
					break
				}
			}
			long = full
			if err != nil && !full {
				break
			}
		}
		if errors.Is(err, io.EOF) {
			err = bw.Flush()
		}
		pw.CloseWithError(err)
	}()
	return pr
}

func dropLine(line []byte, n, major int) bool {
	if len(line) == 0 || (line[0] != '\\' && line[0] != 'S') {
		return false
	}
	switch {
	case unrestrictRe.Match(line):
		return true
	case n <= headerLines && restrictRe.Match(line):
		return true
	case n <= headerLines && major < 17 && txTimeoutRe.Match(line):
		return true
	}
	return false
}

// psql:<stdin>:12: ERROR:  must be owner of extension plpgsql
var psqlErrRe = regexp.MustCompile(`(?m)^psql:[^\n]*?:([0-9]+): (?:ERROR|FATAL|error):\s+(.+)$`)

// roleErrRe matches what PostgreSQL answers a dump that tries to manage
// roles, other databases or superuser things.
// The wording changed over the versions: PostgreSQL 16 says "must be able
// to SET ROLE" where 14 said "must be member of role".
var roleErrRe = regexp.MustCompile(`(?i)permission denied to (create|alter|drop|set|grant|change|copy)|must be superuser|` +
	`must be (a )?member of role|must be able to set role|must be owner of|role "[^"]*" does not exist|` +
	`must have (the )?\w+ privilege|only superusers|requires superuser|permission denied for (database|schema|language|function)`)

// ImportError explains a failed import. errors.As finds a *usererr.Error
// in it, for the API; a job log shows the English sentence.
type ImportError struct {
	Line int    // of the SQL file; 0 when unknown
	Msg  string // PostgreSQL's message
	ue   *usererr.Error
}

func (e *ImportError) Error() string {
	where := ""
	if e.Line > 0 {
		where = " at line " + strconv.Itoa(e.Line)
	}
	switch e.ue.Code {
	case "pg_import_psql_old":
		return "psql on this server is too old to import SQL files safely: update the PostgreSQL packages and try again"
	case "pg_import_meta":
		return "the SQL file contains psql commands (lines starting with \\, such as \\connect)" + where +
			", which Nefo does not run: export the database again with pg_dump without --create, or remove those lines"
	case "pg_import_role":
		return "the SQL file changes roles, ownership or other databases, or needs a superuser" + where + " (" + e.Msg +
			"): export it again with pg_dump --no-owner --no-privileges, or remove those statements. Nothing was imported"
	}
	return "the import failed" + where + ": " + e.Msg + ". Nothing was imported"
}

func (e *ImportError) Unwrap() error { return e.ue }

// importError turns psql's error into an *ImportError. The line number is
// the file's: psql counts the \restrict line in front.
func importError(err error) error {
	var re *run.Error
	if !errors.As(err, &re) {
		return err
	}
	out := re.Output
	if strings.Contains(out, `invalid command \restrict`) {
		return &ImportError{ue: &usererr.Error{Code: "pg_import_psql_old"}}
	}
	all := psqlErrRe.FindAllStringSubmatch(out, -1)
	if all == nil {
		return err
	}
	mm := all[len(all)-1]
	line, _ := strconv.Atoi(mm[1])
	if line > 0 {
		line--
	}
	msg := strings.TrimSpace(mm[2])
	if len(msg) > 300 {
		msg = msg[:300] + "…"
	}
	e := &ImportError{Line: line, Msg: msg}
	code := "pg_import_failed"
	switch {
	case strings.Contains(msg, "backslash commands are restricted"):
		code = "pg_import_meta"
	case roleErrRe.MatchString(msg):
		code = "pg_import_role"
	}
	detail := msg
	if line > 0 {
		detail = fmt.Sprintf("%d: %s", line, msg)
	}
	e.ue = &usererr.Error{Code: code, Detail: detail}
	return e
}
