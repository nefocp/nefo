package siteimport

import (
	"crypto/sha1"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"
)

// A reader for cPanel's mysql.sql: the account's database users and what
// they may use. Lines look like
//
//	GRANT USAGE ON *.* TO 'bob_wp'@'localhost' IDENTIFIED BY PASSWORD '*2470C0…';
//	GRANT ALL PRIVILEGES ON `bob\_wp`.* TO 'bob_wp'@'localhost';
//
// and, from MySQL 8 servers, also
//
//	CREATE USER 'bob_wp'@'localhost' IDENTIFIED WITH 'mysql_native_password' AS '*2470C0…';
//	GRANT ALL PRIVILEGES ON `bob\_wp`.* TO `bob_wp`@`localhost`;
//
// The statements are only read, never run.

// dbAccount is a database user with its password hash.
type dbAccount struct {
	Name   string
	Hosts  []string
	Hash   string // as written, from the localhost entry when there is one
	Plugin string // "" for the default (mysql_native_password)
}

// dbGrant is one GRANT on a database (or tables of it).
type dbGrant struct {
	User, Host string
	DB         string // a GRANT pattern: \_ is a literal _, _ and % are wildcards
	Privs      string
}

type grantSet struct {
	accounts map[string]*dbAccount
	grants   []dbGrant
}

const maxGrantStatements = 20000

func parseGrants(b []byte) *grantSet {
	g := &grantSet{accounts: map[string]*dbAccount{}}
	for i, stmt := range splitStatements(string(b)) {
		if i >= maxGrantStatements {
			break
		}
		toks := tokenize(stmt)
		if len(toks) < 2 {
			continue
		}
		switch {
		case toks[0].is("GRANT"):
			g.grant(toks)
		case toks[0].is("CREATE") && toks[1].is("USER"), toks[0].is("ALTER") && toks[1].is("USER"):
			rest := toks[2:]
			if len(rest) >= 3 && rest[0].is("IF") && rest[1].is("NOT") && rest[2].is("EXISTS") {
				rest = rest[3:]
			}
			if user, host, rest, ok := account(rest); ok {
				hash, plugin := identified(rest)
				g.addAccount(user, host, hash, plugin)
			}
		case toks[0].is("SET") && toks[1].is("PASSWORD") && len(toks) > 3 && toks[2].is("FOR"):
			if user, host, rest, ok := account(toks[3:]); ok && len(rest) >= 2 && rest[0].text == "=" && rest[1].quote != 0 {
				g.addAccount(user, host, rest[1].text, "")
			}
		}
	}
	return g
}

func (g *grantSet) grant(toks []token) {
	on := -1
	for i, t := range toks {
		if t.quote == 0 && t.is("ON") {
			on = i
			break
		}
	}
	if on < 2 {
		return
	}
	var privs []string
	for _, t := range toks[1:on] {
		privs = append(privs, t.text)
	}
	rest := toks[on+1:]
	if len(rest) > 0 && rest[0].quote == 0 && (rest[0].is("TABLE") || rest[0].is("FUNCTION") || rest[0].is("PROCEDURE")) {
		rest = rest[1:]
	}
	// db . table
	if len(rest) < 3 || rest[1].text != "." || rest[1].quote != 0 {
		return
	}
	db := rest[0].text
	rest = rest[3:]
	if len(rest) < 1 || !rest[0].is("TO") {
		return
	}
	user, host, rest, ok := account(rest[1:])
	if !ok {
		return
	}
	hash, plugin := identified(rest)
	g.addAccount(user, host, hash, plugin)
	if db == "*" {
		return // global privileges are never imported
	}
	g.grants = append(g.grants, dbGrant{User: user, Host: host, DB: db, Privs: strings.Join(privs, " ")})
}

// account reads 'user'@'host' (or a user without a host).
func account(toks []token) (user, host string, rest []token, ok bool) {
	if len(toks) == 0 || (toks[0].quote == 0 && !isWord(toks[0].text)) {
		return "", "", nil, false
	}
	user, rest = toks[0].text, toks[1:]
	host = "%"
	if len(rest) >= 2 && rest[0].text == "@" && rest[0].quote == 0 {
		host, rest = rest[1].text, rest[2:]
	}
	return user, host, rest, user != ""
}

// identified reads IDENTIFIED BY PASSWORD 'h', IDENTIFIED WITH plugin AS 'h'
// or IDENTIFIED VIA plugin USING 'h'.
func identified(toks []token) (hash, plugin string) {
	for i := 0; i+2 < len(toks); i++ {
		if !toks[i].is("IDENTIFIED") || toks[i].quote != 0 {
			continue
		}
		switch a := toks[i+1]; {
		case a.is("BY") && toks[i+2].is("PASSWORD") && i+3 < len(toks) && toks[i+3].quote != 0:
			return toks[i+3].text, ""
		case (a.is("WITH") || a.is("VIA")) && i+4 < len(toks) && (toks[i+3].is("AS") || toks[i+3].is("USING") || toks[i+3].is("BY")) && toks[i+4].quote != 0:
			return toks[i+4].text, strings.ToLower(toks[i+2].text)
		}
	}
	return "", ""
}

func (g *grantSet) addAccount(user, host, hash, plugin string) {
	a := g.accounts[user]
	if a == nil {
		a = &dbAccount{Name: user}
		g.accounts[user] = a
	}
	if !contains(a.Hosts, host) {
		a.Hosts = append(a.Hosts, host)
	}
	if hash != "" && (a.Hash == "" || host == "localhost") {
		a.Hash, a.Plugin = hash, plugin
	}
}

// usersOf returns the users with privileges on the database name (beyond
// USAGE), sorted.
func (g *grantSet) usersOf(name string) []string {
	var out []string
	for _, gr := range g.grants {
		if strings.EqualFold(strings.TrimSpace(gr.Privs), "USAGE") || !likeMatch(gr.DB, name) {
			continue
		}
		if !contains(out, gr.User) {
			out = append(out, gr.User)
		}
	}
	sort.Strings(out)
	return out
}

// explicit tells whether user has a grant naming the database itself,
// rather than a pattern such as bob\_%.
func (g *grantSet) explicit(user, name string) bool {
	for _, gr := range g.grants {
		if gr.User == user && strings.ReplaceAll(gr.DB, `\_`, "_") == name && !strings.Contains(strings.ReplaceAll(gr.DB, `\_`, ""), "_") {
			return true
		}
	}
	return false
}

var nativeHashRe = regexp.MustCompile(`^\*[0-9A-Fa-f]{40}$`)

// nativeHash returns the user's password hash when MariaDB accepts it as
// it is: a mysql_native_password hash. Other kinds (MySQL 8's
// caching_sha2_password, old 16-digit hashes) give "".
func (g *grantSet) nativeHash(user string) string {
	a := g.accounts[user]
	if a == nil || (a.Plugin != "" && a.Plugin != "mysql_native_password") || !nativeHashRe.MatchString(a.Hash) {
		return ""
	}
	return strings.ToUpper(a.Hash)
}

// NativeHash is MySQL's mysql_native_password hash of a password.
func NativeHash(password string) string {
	a := sha1.Sum([]byte(password))
	b := sha1.Sum(a[:])
	return "*" + strings.ToUpper(hex.EncodeToString(b[:]))
}

// likeMatch matches a GRANT database pattern: % is any run, _ any one
// character, and \ escapes the next one. Names are compared without case,
// as cPanel's are lower case.
func likeMatch(pattern, s string) bool {
	p, n := []rune(strings.ToLower(pattern)), []rune(strings.ToLower(s))
	var match func(i, j int) bool
	steps := 0
	match = func(i, j int) bool {
		if steps++; steps > 100000 {
			return false
		}
		for i < len(p) {
			switch p[i] {
			case '%':
				for k := j; k <= len(n); k++ {
					if match(i+1, k) {
						return true
					}
				}
				return false
			case '_':
				if j >= len(n) {
					return false
				}
			case '\\':
				if i+1 < len(p) {
					i++
				}
				fallthrough
			default:
				if j >= len(n) || n[j] != p[i] {
					return false
				}
			}
			i++
			j++
		}
		return j == len(n)
	}
	return match(0, 0)
}

// ---- statements and tokens ----

type token struct {
	text  string
	quote byte // 0 for a word or symbol, else ', " or `
}

func (t token) is(word string) bool { return t.quote == 0 && strings.EqualFold(t.text, word) }

func isWord(s string) bool {
	for _, c := range s {
		if !(c == '_' || c == '$' || c == '-' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
			return false
		}
	}
	return s != ""
}

// splitStatements splits SQL on semicolons outside quotes and comments.
func splitStatements(s string) []string {
	var out []string
	var q byte
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case q != 0:
			if c == '\\' && q != '`' {
				i++
			} else if c == q {
				q = 0
			}
		case c == '\'' || c == '"' || c == '`':
			q = c
		case c == '-' && strings.HasPrefix(s[i:], "-- "), c == '#':
			if j := strings.IndexByte(s[i:], '\n'); j >= 0 {
				i += j
			} else {
				i = len(s)
			}
		case c == ';':
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if strings.TrimSpace(s[start:]) != "" {
		out = append(out, s[start:])
	}
	return out
}

// tokenize splits a statement into words, quoted strings and the symbols
// @ . , = ( ) *.
func tokenize(s string) []token {
	var out []token
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '\'' || c == '"' || c == '`':
			var b strings.Builder
			j := i + 1
			for ; j < len(s); j++ {
				if s[j] == '\\' && c != '`' && j+1 < len(s) {
					// Keep \_ and \% as written: they matter in GRANT patterns.
					if n := s[j+1]; n == '_' || n == '%' {
						b.WriteByte('\\')
					}
					b.WriteByte(s[j+1])
					j++
					continue
				}
				if s[j] == c {
					if j+1 < len(s) && s[j+1] == c { // doubled quote
						b.WriteByte(c)
						j++
						continue
					}
					break
				}
				b.WriteByte(s[j])
			}
			out = append(out, token{text: b.String(), quote: c})
			i = j + 1
		case strings.IndexByte("@.,=()*;", c) >= 0:
			out = append(out, token{text: string(c)})
			i++
		default:
			j := i
			for j < len(s) && strings.IndexByte(" \t\n\r'\"`@.,=()*;", s[j]) < 0 {
				j++
			}
			out = append(out, token{text: s[i:j]})
			i = j
		}
	}
	return out
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
