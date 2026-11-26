package siteimport

import (
	"regexp"
	"strings"

	"github.com/nefocp/nefo/internal/wptoolkit"
)

// wpInfo is what a wp-config.php says about its database.
type wpInfo struct {
	DBName, DBUser, DBPassword, DBHost string
	Multisite                          bool
	readable                           bool // DB_NAME is a plain string
}

var multisiteRe = regexp.MustCompile(`(?i:define)\s*\(\s*['"]MULTISITE['"]\s*,\s*true\s*\)`)

const maxConfig = 1 << 20

func parseWPConfig(b []byte) *wpInfo {
	cfg := string(b)
	w := &wpInfo{Multisite: multisiteRe.MatchString(cfg)}
	w.DBName, w.readable = wptoolkit.ConfigDefine(cfg, "DB_NAME")
	w.DBUser, _ = wptoolkit.ConfigDefine(cfg, "DB_USER")
	w.DBPassword, _ = wptoolkit.ConfigDefine(cfg, "DB_PASSWORD")
	w.DBHost, _ = wptoolkit.ConfigDefine(cfg, "DB_HOST")
	return w
}

// localHost tells whether DB_HOST is the server's own MariaDB or MySQL.
func localHost(h string) bool {
	host, rest, _ := strings.Cut(strings.ToLower(strings.TrimSpace(h)), ":")
	if h == "" {
		return true
	}
	if host != "localhost" && host != "127.0.0.1" {
		return false
	}
	return rest == "" || rest == "3306" || strings.HasPrefix(rest, "/")
}

// openBasedirRoot returns the first folder of an open_basedir setting in a
// .user.ini (aaPanel writes open_basedir=/www/wwwroot/example.com/:/tmp/),
// which tells where the site was.
func openBasedirRoot(ini []byte) string {
	for _, line := range strings.Split(string(ini), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || strings.TrimSpace(strings.ToLower(k)) != "open_basedir" {
			continue
		}
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		first, _, _ := strings.Cut(v, ":")
		first = strings.TrimRight(first, "/")
		if strings.HasPrefix(first, "/") && len(first) > 1 && first != "/tmp" && !strings.ContainsAny(first, " \t\r") {
			return first
		}
	}
	return ""
}
