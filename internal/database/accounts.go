package database

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/nefocp/nefo/internal/secure"
)

// The accounts of the database server, carried from MariaDB to MySQL or
// back by a switch (switch.go). The two keep passwords differently:
// MariaDB's users have mysql_native_password hashes, which MySQL 8.4 does
// not accept (its plugin is off, see AcceptsNativeHash), and MySQL's have
// caching_sha2_password hashes, which MariaDB does not know. So:
//
//   - a database user of the panel gets its password again: the panel
//     knows it (store.Database.Password), on every host it had;
//   - another account keeps its mysql_native_password hash into MariaDB,
//     and its socket sign-in (unix_socket, auth_socket) both ways;
//   - any other account is created with a random password nobody knows and
//     locked: its owner sets a new password (ALTER USER … ACCOUNT UNLOCK).
//
// The privileges come from SHOW GRANTS on the old server; one the other
// server refuses is left out and listed in the job's log.

// Ways an account is created on the other server (AccountPlan.How).
const (
	HowPassword = "password" // the panel's database user, with its password
	HowHash     = "hash"     // its mysql_native_password hash
	HowSocket   = "socket"   // signs in through the socket, as before
	HowNew      = "new"      // a random password, locked
	HowKept     = "kept"     // the panel's import account: locked, as it was
	HowSkip     = "skip"     // the other server can't have it
)

// AccountPlan says how a switch carries an account over.
type AccountPlan struct {
	User  string `json:"user"`
	Host  string `json:"host"`
	How   string `json:"how"`
	Panel bool   `json:"panel"` // a database user the panel made
	// Why is the reason for HowNew and HowSkip, for the UI: the old
	// sign-in method ("caching_sha2_password") or "too_long".
	Why string `json:"why,omitempty"`
}

// account is an account of the old server, as the switch reads it.
type account struct {
	AccountPlan
	plugin   string
	hash     string
	locked   bool
	grants   []string
	password string // for HowPassword
}

// systemAccount reports whether user@host belongs to the server itself.
func systemAccount(user string) bool {
	switch user {
	case "", "root", "mysql", "mariadb.sys", "mysql.sys", "mysql.session", "mysql.infoschema", "debian-sys-maint", "PUBLIC":
		return true
	}
	return false
}

// readAccounts lists the accounts of engine e other than its own, with
// their grants and how they go to the other engine.
func (m *Manager) readAccounts(ctx context.Context, e Engine) ([]account, error) {
	to := MySQL
	if e == MySQL {
		to = MariaDB
	}
	q := "SELECT HEX(User), HEX(Host), HEX(IFNULL(plugin, '')), HEX(IFNULL(authentication_string, '')) FROM mysql.user"
	if e == MariaDB {
		q += " WHERE is_role <> 'Y'"
	}
	rows, err := m.hexRows(ctx, e, q+";")
	if err != nil {
		return nil, err
	}
	known := map[string]string{} // the panel's users with their passwords
	panel := map[string]bool{}
	dbs, err := m.store.ListDatabases(ctx)
	if err != nil {
		return nil, err
	}
	for _, d := range dbs {
		panel[d.Username] = true
		if d.Password != "" {
			known[d.Username] = d.Password
		}
	}
	var out []account
	for _, r := range rows {
		if len(r) != 4 || systemAccount(r[0]) {
			continue
		}
		a := account{AccountPlan: AccountPlan{User: r[0], Host: r[1], Panel: panel[r[0]]}, plugin: r[2], hash: r[3]}
		a.How, a.Why = planAccount(a.User, a.plugin, a.hash, to, known)
		if a.How == HowPassword {
			a.password = known[a.User]
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].User != out[j].User {
			return out[i].User < out[j].User
		}
		return out[i].Host < out[j].Host
	})
	for i := range out {
		a := &out[i]
		who := sqlString(a.User) + "@" + sqlString(a.Host)
		pre := ""
		if e == MySQL {
			pre = "SET SESSION print_identified_with_as_hex = ON;\n" // no raw hash bytes in the answer
		}
		res, err := m.sqlWith(ctx, e, pre+"SHOW CREATE USER "+who+";\nSELECT '--nefo-grants--';\nSHOW GRANTS FOR "+who+";")
		if err != nil {
			return nil, fmt.Errorf("read the account %s@%s: %w", a.User, a.Host, err)
		}
		create, grants, _ := strings.Cut(res, "--nefo-grants--\n")
		a.locked = strings.Contains(create, "ACCOUNT LOCK")
		for _, g := range strings.Split(grants, "\n") {
			if g = strings.TrimSpace(g); g != "" {
				a.grants = append(a.grants, g)
			}
		}
	}
	return out, nil
}

// planAccount decides how an account goes to engine to (see the package
// comment above).
func planAccount(user, plugin, hash string, to Engine, known map[string]string) (how, why string) {
	switch {
	case strings.HasPrefix(user, importUserPrefix):
		if to == MySQL && len(user) > mysqlUserMax {
			return HowSkip, "too_long"
		}
		return HowKept, ""
	case to == MySQL && len(user) > mysqlUserMax:
		return HowSkip, "too_long"
	case known[user] != "":
		return HowPassword, ""
	}
	switch plugin {
	case "unix_socket", "auth_socket":
		return HowSocket, ""
	case "mysql_native_password", "":
		if to == MariaDB && nativeHashRe.MatchString(strings.ToUpper(hash)) {
			return HowHash, ""
		}
	}
	if plugin == "" {
		plugin = "mysql_native_password"
	}
	return HowNew, plugin
}

var (
	// The credentials SHOW GRANTS adds on MariaDB, and its resource limits.
	grantAuthRe = regexp.MustCompile(` IDENTIFIED (BY PASSWORD '[^']*'|VIA .*?|WITH .*?)( WITH |$)`)
	grantOnRe   = regexp.MustCompile(`^GRANT .+ ON .+ TO `)
)

// carriedGrants turns an account's SHOW GRANTS lines into the statements
// to run on the other server: without credentials, without USAGE (CREATE
// USER gives it), PROXY and role grants, which the panel does not carry.
func carriedGrants(lines []string) (keep, skipped []string) {
	for _, g := range lines {
		g = strings.TrimSuffix(strings.TrimSpace(g), ";")
		g = grantAuthRe.ReplaceAllString(g, "$2")
		g = strings.TrimSpace(g)
		switch {
		case g == "":
		case strings.HasPrefix(g, "GRANT USAGE ON *.* TO "):
			if strings.Contains(g, " WITH GRANT OPTION") {
				skipped = append(skipped, g)
			}
		case strings.HasPrefix(g, "GRANT PROXY ON "), !grantOnRe.MatchString(g):
			skipped = append(skipped, g)
		default:
			keep = append(keep, g+";")
		}
	}
	return keep, skipped
}

// createAccountSQL returns the statement that creates the account on
// engine to.
func createAccountSQL(a account, to Engine) (string, error) {
	who := sqlString(a.User) + "@" + sqlString(a.Host)
	var auth string
	switch a.How {
	case HowPassword:
		auth = "IDENTIFIED BY " + sqlString(a.password)
	case HowHash:
		auth = "IDENTIFIED BY PASSWORD " + sqlString(strings.ToUpper(a.hash))
	case HowSocket:
		auth = "IDENTIFIED VIA unix_socket"
		if to == MySQL {
			auth = "IDENTIFIED WITH auth_socket"
		}
	case HowNew, HowKept:
		auth = "IDENTIFIED BY " + sqlString(secure.String(32, secure.PasswordChars)) + " ACCOUNT LOCK"
	default:
		return "", fmt.Errorf("the account %s@%s is not carried over", a.User, a.Host)
	}
	s := "CREATE USER " + who + " " + auth + ";"
	if a.locked && a.How != HowNew && a.How != HowKept {
		s += "\nALTER USER " + who + " ACCOUNT LOCK;"
	}
	return s, nil
}

// accountPlans returns the plans of accounts, for the UI.
func accountPlans(accounts []account) []AccountPlan {
	out := make([]AccountPlan, len(accounts))
	for i, a := range accounts {
		out[i] = a.AccountPlan
	}
	return out
}
