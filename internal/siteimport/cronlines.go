package siteimport

import (
	"regexp"
	"sort"
	"strings"

	"github.com/nefocp/nefo/internal/cron"
)

// CronLine is a line of the old account's crontab.
//
// Cron lines are shown, not installed by default: their paths and
// programs are the old server's. Ticked ones are installed as cron jobs of
// the website's own Linux user (never root), with the old document roots
// and cPanel's PHP binaries replaced by this server's; one that still
// names the old home folder is marked so.
type CronLine struct {
	Raw        string   `json:"raw"`
	Schedule   string   `json:"schedule"`
	Command    string   `json:"command"`     // on the old server
	NewCommand string   `json:"new_command"` // as it would be installed
	Domain     string   `json:"domain"`      // the website it runs as
	OK         bool     `json:"ok"`          // it can be installed
	Notes      []string `json:"notes"`       // old_home: names the old home folder
}

const maxCronLines = 500

var (
	envLineRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*\s*=`)
	// cPanel's PHP binaries: /opt/cpanel/ea-php81/root/usr/bin/php,
	// /usr/local/bin/ea-php81, /opt/alt/php81/usr/bin/php (CloudLinux).
	eaPHPRe = regexp.MustCompile(`/opt/cpanel/ea-php[0-9]+/root/usr/bin/php(?:-cgi)?|/usr/local/bin/ea-php[0-9]+|/opt/alt/php[0-9]+/usr/bin/php(?:-cgi)?|/usr/local/bin/php(?:-cli)?`)
)

// cronLines reads a crontab and rewrites its commands for this server.
// roots maps each old document root to its website's domain and new
// public folder; main is the domain lines without a root belong to; php
// gives each domain's PHP version here.
func cronLines(text, homeDir, main string, roots map[string][2]string, php map[string]string) []*CronLine {
	var olds []string
	for r := range roots {
		olds = append(olds, r)
	}
	// Longest first: /home/bob/public_html/shop before /home/bob/public_html.
	sort.Slice(olds, func(i, j int) bool { return len(olds[i]) > len(olds[j]) })
	out := []*CronLine{}
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "#") || envLineRe.MatchString(line) {
			continue
		}
		if len(out) >= maxCronLines {
			break
		}
		c := &CronLine{Raw: line, Notes: []string{}}
		f := strings.Fields(line)
		n := 5
		if strings.HasPrefix(line, "@") {
			n = 1
		}
		if len(f) <= n {
			out = append(out, c)
			continue
		}
		c.Schedule = strings.Join(f[:n], " ")
		// The command keeps its own spacing.
		rest := line
		for i := 0; i < n; i++ {
			rest = strings.TrimLeft(rest, " \t")
			rest = rest[strings.IndexAny(rest+" ", " \t"):]
		}
		c.Command = strings.TrimSpace(rest)
		c.Domain = main
		cmd := c.Command
		for _, o := range olds {
			if strings.Contains(cmd, o) {
				if c.Domain == main {
					c.Domain = roots[o][0]
				}
				cmd = replacePath(cmd, o, roots[o][1])
			}
		}
		if v := php[c.Domain]; v != "" {
			cmd = eaPHPRe.ReplaceAllString(cmd, "/usr/bin/php"+v)
		}
		c.NewCommand = cmd
		if homeDir != "" && (strings.Contains(cmd, homeDir+"/") || strings.HasSuffix(cmd, homeDir)) {
			c.Notes = append(c.Notes, "old_home")
		}
		c.OK = cron.ValidSchedule(c.Schedule) && c.NewCommand != "" && len(c.NewCommand) <= 2000 && c.Domain != ""
		out = append(out, c)
	}
	return out
}

// replacePath replaces the folder old with new where it is a whole path:
// /home/bob/public_html/x, but not /home/bob/public_html2.
func replacePath(s, old, new string) string {
	var b strings.Builder
	for {
		i := strings.Index(s, old)
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		end := i + len(old)
		b.WriteString(s[:i])
		if end < len(s) && isPathByte(s[end]) {
			b.WriteString(old)
		} else {
			b.WriteString(new)
		}
		s = s[end:]
	}
}

func isPathByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '-' || c == '_'
}
