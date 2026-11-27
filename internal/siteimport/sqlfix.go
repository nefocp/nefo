package siteimport

import (
	"bytes"
	"io"
	"strings"
)

// sqlFixer adapts a dump made by MySQL 8 (or 9) for MariaDB while it
// streams through, without holding it in memory. It reads the SQL as the
// server would, so text inside string literals, quoted names and plain
// comments is never changed:
//
//   - MySQL 8 collations MariaDB does not know (utf8mb4_0900_ai_ci and the
//     other *_0900_* ones) become the closest MariaDB collation;
//   - executable comments for MySQL 8 and later (/*!80016 DEFAULT
//     ENCRYPTION='N' */, /*!80003 SRID 4326 */…), which MariaDB would run
//     because its version number (10.x) compares higher, become plain
//     comments, as a MySQL 5.7 server would read them;
//   - statements MariaDB refuses or the import account may not run are
//     left out: SET @@GLOBAL.GTID_PURGED (MySQL's GTIDs), the
//     SET @@SESSION.SQL_LOG_BIN lines around it, and MariaDB's own
//     SET GLOBAL gtid_slave_pos;
//   - CREATE DATABASE and USE for the dump's own database (a dump made with
//     --databases) are left out, so it can go into a database of another
//     name. A second database stays: the import then stops with a clear
//     error instead of mixing two databases into one.
type sqlFixer struct {
	src io.Reader
	buf []byte
	out bytes.Buffer
	err error // from src, once out is drained

	st        lexState
	escape    bool   // the previous byte in a string was a backslash
	star      bool   // the previous byte in a block comment was *
	word      []byte // identifier being read, while it may be a collation
	wordLong  bool   // the identifier is too long to be one: passed through
	ver       []byte // digits after /*!
	lineStart bool
	head      []byte // start of a line, until it is known to be ordinary
	stmt      []byte // a statement being held back (see headRules)
	stmtRule  int
	stmtQuote byte
	stmtEsc   bool
	firstDB   string
	// mysql: the server is MySQL 8 itself, which knows its collations and
	// comments; only the statements an import account may not run go.
	mysql bool
	// namesOnly: only collation names and executable comments change;
	// no statement is left out (see FixCollations).
	namesOnly bool

	Stats SQLFixStats
}

// SQLFixStats counts what a sqlFixer changed.
type SQLFixStats struct {
	Collations int // collation names replaced
	// MariaDBCollations are MariaDB 10.10+ collation names replaced for
	// MySQL (a dump going into MySQL).
	MariaDBCollations int
	Comments          int // MySQL 8 executable comments made plain
	Statements        int // statements left out (GTID, binary log)
	DBLines           int // CREATE DATABASE and USE lines left out
}

func (s SQLFixStats) String() string {
	var parts []string
	if s.Collations > 0 {
		parts = append(parts, count(int64(s.Collations), "MySQL 8 collation name", "MySQL 8 collation names")+" replaced")
	}
	if s.MariaDBCollations > 0 {
		parts = append(parts, count(int64(s.MariaDBCollations), "MariaDB collation name", "MariaDB collation names")+" replaced")
	}
	if s.Comments > 0 {
		parts = append(parts, count(int64(s.Comments), "MySQL 8 only setting", "MySQL 8 only settings")+" left out (such as DEFAULT ENCRYPTION and SRID)")
	}
	if s.Statements > 0 {
		parts = append(parts, count(int64(s.Statements), "GTID or binary log statement", "GTID and binary log statements")+" left out")
	}
	if s.DBLines > 0 {
		parts = append(parts, count(int64(s.DBLines), "CREATE DATABASE or USE line", "CREATE DATABASE and USE lines")+" left out")
	}
	return strings.Join(parts, ", ")
}

type lexState int

const (
	stNormal    lexState = iota
	stSingle             // '…'
	stDouble             // "…"
	stBacktick           // `…`
	stLine               // -- or # comment
	stBlock              // /* … */
	stDash               // saw -
	stDash2              // saw --
	stSlash              // saw /
	stSlashStar          // saw /*
	stVersion            // saw /*! and reading digits
)

const (
	ruleDrop = 1 // leave the statement out
	ruleDB   = 2 // CREATE DATABASE or USE: left out for the dump's own database
	maxHeld  = 64 << 10
)

// headRules are the statements, at the start of a line, that are held
// back until their end and then left out or kept.
var headRules = []struct {
	prefix string
	rule   int
}{
	{"SET @@GLOBAL.GTID_PURGED", ruleDrop},
	{"SET @@SESSION.SQL_LOG_BIN", ruleDrop},
	{"SET @MYSQLDUMP_TEMP_LOG_BIN", ruleDrop},
	{"SET GLOBAL GTID_SLAVE_POS", ruleDrop}, // MariaDB's own dumps with --gtid
	{"CREATE DATABASE ", ruleDB},
	{"CREATE SCHEMA ", ruleDB},
	{"USE ", ruleDB},
}

const maxHead = 28 // the longest prefix

func newSQLFixer(r io.Reader) *sqlFixer {
	return &sqlFixer{src: r, buf: make([]byte, 64<<10), lineStart: true}
}

func (f *sqlFixer) Read(p []byte) (int, error) {
	for f.out.Len() == 0 {
		if f.err != nil {
			return 0, f.err
		}
		n, err := f.src.Read(f.buf)
		for _, c := range f.buf[:n] {
			f.feed(c)
		}
		if err != nil {
			if err == io.EOF {
				f.flush()
			}
			f.err = err
		}
	}
	return f.out.Read(p)
}

// feed takes one byte of the dump.
func (f *sqlFixer) feed(c byte) {
	switch {
	case f.stmtRule != 0:
		f.hold(c)
	case f.lineStart && f.st == stNormal && !f.namesOnly:
		f.head = append(f.head, c)
		if c == '\n' {
			f.releaseHead()
			f.lineStart = true
			return
		}
		rule, maybe := matchHead(f.head)
		switch {
		case rule != 0:
			f.stmt, f.stmtRule, f.head = append(f.stmt[:0], f.head...), rule, f.head[:0]
			f.lineStart = false
			f.stmtQuote, f.stmtEsc = 0, false
		case !maybe || len(f.head) >= maxHead:
			f.releaseHead()
		}
	default:
		f.lex(c)
	}
}

// matchHead tells whether the start of a line is one of headRules (rule)
// or may still become one (maybe).
func matchHead(h []byte) (rule int, maybe bool) {
	for _, r := range headRules {
		n := min(len(h), len(r.prefix))
		if !strings.EqualFold(string(h[:n]), r.prefix[:n]) {
			continue
		}
		if len(h) >= len(r.prefix) {
			return r.rule, true
		}
		maybe = true
	}
	return 0, maybe
}

// releaseHead passes the start of a line on as ordinary SQL.
func (f *sqlFixer) releaseHead() {
	h := f.head
	f.head = nil
	f.lineStart = false
	for _, c := range h {
		f.lex(c)
	}
	f.head = h[:0]
}

// hold collects a held-back statement up to its semicolon.
func (f *sqlFixer) hold(c byte) {
	f.stmt = append(f.stmt, c)
	switch {
	case f.stmtQuote != 0:
		if f.stmtEsc {
			f.stmtEsc = false
		} else if c == '\\' && f.stmtQuote != '`' {
			f.stmtEsc = true
		} else if c == f.stmtQuote {
			f.stmtQuote = 0
		}
	case c == '\'' || c == '"' || c == '`':
		f.stmtQuote = c
	case c == ';':
		f.endStatement(true)
		return
	}
	if len(f.stmt) > maxHeld {
		f.endStatement(false)
	}
}

// endStatement leaves the held statement out, or passes it on.
func (f *sqlFixer) endStatement(complete bool) {
	stmt, rule := f.stmt, f.stmtRule
	f.stmt, f.stmtRule = stmt[:0], 0
	drop := false
	if complete {
		switch rule {
		case ruleDrop:
			drop = true
			f.Stats.Statements++
		case ruleDB:
			if name := statementDB(string(stmt)); name != "" {
				if f.firstDB == "" {
					f.firstDB = name
				}
				if name == f.firstDB {
					drop = true
					f.Stats.DBLines++
				}
			}
		}
	}
	if drop {
		return
	}
	for _, c := range stmt {
		f.lex(c)
	}
}

// statementDB returns the database a CREATE DATABASE or USE names.
func statementDB(s string) string {
	if i := strings.IndexByte(s, '`'); i >= 0 {
		if j := strings.IndexByte(s[i+1:], '`'); j >= 0 {
			return s[i+1 : i+1+j]
		}
		return ""
	}
	f := strings.Fields(strings.TrimSuffix(strings.TrimSpace(s), ";"))
	if len(f) == 0 {
		return ""
	}
	return strings.Trim(f[len(f)-1], ";")
}

// flush passes on what is still held at the end of the dump.
func (f *sqlFixer) flush() {
	if f.stmtRule != 0 {
		f.endStatement(false)
	}
	if len(f.head) > 0 {
		f.releaseHead()
	}
	switch f.st {
	case stDash:
		f.out.WriteByte('-')
	case stDash2:
		f.out.WriteString("--")
	case stSlash:
		f.out.WriteByte('/')
	case stSlashStar:
		f.out.WriteString("/*")
	case stVersion:
		f.out.WriteString("/*!")
		f.out.Write(f.ver)
	}
	f.st = stNormal
	f.flushWord()
}

func isIdentByte(c byte) bool {
	return c == '_' || c == '$' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// lex runs the SQL reader on one byte.
func (f *sqlFixer) lex(c byte) {
	switch f.st {
	case stNormal:
		if isIdentByte(c) {
			if f.wordLong {
				f.out.WriteByte(c)
				return
			}
			f.word = append(f.word, c)
			if len(f.word) > 64 { // no collation is this long: pass it on (hex blobs)
				f.out.Write(f.word)
				f.word, f.wordLong = f.word[:0], true
			}
			return
		}
		f.flushWord()
		switch c {
		case '\'':
			f.st = stSingle
		case '"':
			f.st = stDouble
		case '`':
			f.st = stBacktick
		case '#':
			f.st = stLine
		case '-':
			f.st = stDash
			return
		case '/':
			f.st = stSlash
			return
		case '\n':
			f.out.WriteByte(c)
			f.lineStart = true
			return
		}
		f.out.WriteByte(c)
	case stSingle, stDouble:
		f.out.WriteByte(c)
		q := byte('\'')
		if f.st == stDouble {
			q = '"'
		}
		switch {
		case f.escape:
			f.escape = false
		case c == '\\':
			f.escape = true
		case c == q:
			f.st = stNormal
		}
	case stBacktick:
		f.out.WriteByte(c)
		if c == '`' {
			f.st = stNormal
		}
	case stLine:
		f.out.WriteByte(c)
		if c == '\n' {
			f.st = stNormal
			f.lineStart = true
		}
	case stBlock:
		f.out.WriteByte(c)
		if f.star && c == '/' {
			f.st = stNormal
		}
		f.star = c == '*'
	case stDash:
		if c == '-' {
			f.st = stDash2
			return
		}
		f.out.WriteByte('-')
		f.st = stNormal
		f.lex(c)
	case stDash2:
		f.out.WriteString("--")
		f.st = stNormal
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			f.st = stLine
		}
		f.lex(c)
	case stSlash:
		if c == '*' {
			f.st = stSlashStar
			return
		}
		f.out.WriteByte('/')
		f.st = stNormal
		f.lex(c)
	case stSlashStar:
		if c == '!' {
			f.st, f.ver = stVersion, f.ver[:0]
			return
		}
		f.out.WriteString("/*")
		f.st, f.star = stBlock, false
		f.lex(c)
	case stVersion:
		if c >= '0' && c <= '9' && len(f.ver) < 6 {
			f.ver = append(f.ver, c)
			return
		}
		if len(f.ver) == 5 && (f.ver[0] == '8' || f.ver[0] == '9') && !f.mysql {
			// MySQL 8.0.16 and later only: a plain comment for MariaDB.
			f.out.WriteString("/* ")
			f.out.Write(f.ver)
			f.Stats.Comments++
			f.st, f.star = stBlock, false
		} else {
			f.out.WriteString("/*!")
			f.out.Write(f.ver)
			f.st = stNormal
		}
		f.lex(c)
	}
}

func (f *sqlFixer) flushWord() {
	f.wordLong = false
	if len(f.word) == 0 {
		return
	}
	if r, ok := mariaCollation(string(f.word)); ok && !f.mysql {
		f.out.WriteString(r)
		f.Stats.Collations++
	} else if r, ok := mysqlCollation(string(f.word)); ok && f.mysql {
		f.out.WriteString(r)
		f.Stats.MariaDBCollations++
	} else {
		f.out.Write(f.word)
	}
	f.word = f.word[:0]
}

// FixDump adapts a dump for the server it goes into while it streams (see
// sqlFixer): for MariaDB (toMySQL false), MySQL 8's collations and
// executable comments; for MySQL, MariaDB 10.10 and later's collations
// (mysqlCollation). Both leave out the statements an import account may
// not run, and the dump's own CREATE DATABASE and USE.
func FixDump(r io.Reader, toMySQL bool) io.Reader {
	f := newSQLFixer(r)
	f.mysql = toMySQL
	return f
}

// FixCollations is FixDump that leaves nothing out: only the other
// server's collation names and, into MariaDB, MySQL 8's executable comments
// change. The panel's own SQL imports and backup restores use it, where a
// USE of another database must still be refused.
func FixCollations(r io.Reader, toMySQL bool) io.Reader {
	f := newSQLFixer(r)
	f.mysql, f.namesOnly = toMySQL, true
	return f
}

// mysqlCollation returns the MySQL 8 collation for a MariaDB one MySQL
// does not have: the UCA 14.0 collations of MariaDB 10.10 and later, such
// as utf8mb4_uca1400_ai_ci (the default for utf8mb4 from MariaDB 11.2),
// their NO PAD and language variants. utf8mb4 ones become MySQL's UCA 9.0
// collations (utf8mb4_0900_ai_ci, utf8mb4_sv_0900_ai_ci…, which are all NO
// PAD), the other character sets their unicode_520 or binary collation.
func mysqlCollation(w string) (string, bool) {
	l := strings.ToLower(w)
	cs, rest, ok := strings.Cut(l, "_uca1400_")
	if !ok || cs == "" {
		return "", false
	}
	switch cs {
	case "utf8mb4", "utf8mb3", "utf8", "ucs2", "utf16", "utf16le", "utf32":
	default:
		return "", false
	}
	parts := strings.Split(rest, "_")
	if len(parts) < 2 {
		return "", false
	}
	kind := parts[len(parts)-2] + "_" + parts[len(parts)-1]
	switch kind {
	case "ai_ci", "ai_cs", "as_ci", "as_cs":
	default:
		return "", false
	}
	lang := strings.Join(parts[:len(parts)-2], "_")
	lang = strings.TrimSuffix(strings.TrimSuffix(lang, "nopad"), "_")
	ci := strings.HasSuffix(kind, "_ci")
	if cs != "utf8mb4" {
		if cs == "utf8" {
			cs = "utf8mb3"
		}
		if ci {
			return cs + "_unicode_520_ci", true
		}
		return cs + "_bin", true
	}
	if lang != "" {
		code := ""
		for c, name := range mysqlLangs {
			if name == lang {
				code = c
			}
		}
		if code == "" {
			lang = ""
		} else {
			// MySQL's language collations are accent and case insensitive,
			// or both sensitive.
			if kind == "ai_ci" {
				return "utf8mb4_" + code + "_0900_ai_ci", true
			}
			return "utf8mb4_" + code + "_0900_as_cs", true
		}
	}
	if kind == "ai_cs" {
		kind = "as_cs"
	}
	return "utf8mb4_0900_" + kind, true
}

// mysqlLangs maps MySQL 8's language codes in collation names to the
// names of MariaDB's matching collations.
var mysqlLangs = map[string]string{
	"cs": "czech", "da": "danish", "de_pb": "german2", "eo": "esperanto", "es": "spanish", "es_trad": "spanish2",
	"et": "estonian", "hr": "croatian", "hu": "hungarian", "is": "icelandic", "la": "roman", "lt": "lithuanian",
	"lv": "latvian", "pl": "polish", "ro": "romanian", "sk": "slovak", "sl": "slovenian", "sv": "swedish",
	"tr": "turkish", "vi": "vietnamese",
}

// mariaCollation returns the MariaDB collation for a MySQL 8 one such as
// utf8mb4_0900_ai_ci, utf8mb4_0900_as_cs or utf8mb4_vi_0900_ai_ci.
// Accent and case insensitive ones become utf8mb4_unicode_520_ci (or the
// language's own), the sensitive and binary ones utf8mb4_bin: MariaDB 10.5
// and 10.6, which Debian 11 and Ubuntu 22.04 ship, have nothing closer.
func mariaCollation(w string) (string, bool) {
	l := strings.ToLower(w)
	rest, ok := strings.CutPrefix(l, "utf8mb4_")
	if !ok || !strings.Contains(rest, "0900_") {
		return "", false
	}
	lang, kind, ok := strings.Cut(rest, "0900_")
	if !ok || (lang != "" && !strings.HasSuffix(lang, "_")) {
		return "", false
	}
	lang = strings.TrimSuffix(lang, "_")
	switch kind {
	case "ai_ci", "as_ci":
		if name, ok := mysqlLangs[lang]; ok {
			return "utf8mb4_" + name + "_ci", true
		}
		return "utf8mb4_unicode_520_ci", true
	case "as_cs", "bin", "ai_cs", "as_cs_ks", "as_ci_ks":
		return "utf8mb4_bin", true
	}
	if strings.HasSuffix(kind, "_ci") {
		return "utf8mb4_unicode_520_ci", true
	}
	return "utf8mb4_bin", true
}
