package procs

import (
	"bytes"
	"context"
	"encoding/json"
	"net/netip"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// MaxLogins is the most SSH sign-ins SSHLogins returns.
const MaxLogins = 200

// Login is one SSH sign-in, or one attempt.
type Login struct {
	Time time.Time `json:"time"`
	// Result is "accepted", "failed" (wrong password for an existing
	// account) or "invalid_user" (the account does not exist).
	Result string `json:"result"`
	User   string `json:"user"`
	IP     string `json:"ip"`
	Method string `json:"method,omitempty"` // publickey, password, keyboard-interactive/pam…
}

// The messages of OpenSSH's sshd (sshd-session from OpenSSH 9.8). The user
// name is chosen by whoever connects and may contain " from ", so the
// patterns take the last "from <ip> port <n>".
var (
	acceptedRe = regexp.MustCompile(`^Accepted (\S+) for (.*) from (\S+) port (\d+)`)
	failedRe   = regexp.MustCompile(`^Failed password for (invalid user )?(.*) from (\S+) port (\d+)`)
	invalidRe  = regexp.MustCompile(`^Invalid user (.*) from (\S+) port (\d+)`)
)

// The journal's trusted _SYSTEMD_UNIT field, unlike the syslog name, cannot
// be forged by other programs on the server, so only the SSH service's own
// lines count. Ubuntu calls it ssh.service, other systems sshd.service.
var sshJournal = []string{"-u", "ssh.service", "-u", "sshd.service", "--since=-30d",
	"--no-pager", "-q", "-o", "json", "--output-fields=MESSAGE,_PID"}

const sshPattern = "^(Accepted |Failed password |Invalid user )"

// SSHLogins returns the last sign-ins and failed attempts of the past 30
// days, newest first.
func (m *Manager) SSHLogins(ctx context.Context) ([]Login, error) {
	out, err := m.run.Output(ctx, "journalctl", append(slices.Clone(sshJournal), "-n", "2000", "-g", sshPattern)...)
	if err != nil {
		// journalctl without pattern matching support: filter here.
		out, err = m.run.Output(ctx, "journalctl", append(slices.Clone(sshJournal), "-n", "10000")...)
		if err != nil {
			return nil, err
		}
	}
	return parseLogins(out), nil
}

type journalLine struct {
	Message json.RawMessage `json:"MESSAGE"`
	PID     string          `json:"_PID"`
	Time    string          `json:"__REALTIME_TIMESTAMP"`
}

type rawLogin struct {
	Login
	pid, port string
}

// message decodes MESSAGE, which the journal writes as an array of bytes
// when it is not valid UTF-8.
func message(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var b []byte
	var nums []int
	if json.Unmarshal(raw, &nums) == nil {
		for _, n := range nums {
			b = append(b, byte(n))
		}
	}
	return strings.ToValidUTF8(string(b), "?")
}

// parseLogins reads journalctl's JSON lines.
func parseLogins(out string) []Login {
	var raw []rawLogin
	for _, line := range bytes.Split([]byte(out), []byte("\n")) {
		var jl journalLine
		if len(bytes.TrimSpace(line)) == 0 || json.Unmarshal(line, &jl) != nil {
			continue
		}
		usec, err := strconv.ParseInt(jl.Time, 10, 64)
		if err != nil {
			continue
		}
		l, ok := parseMessage(message(jl.Message))
		if !ok {
			continue
		}
		l.Time = time.UnixMicro(usec).UTC()
		l.pid = jl.PID
		raw = append(raw, l)
	}
	slices.SortStableFunc(raw, func(a, b rawLogin) int { return a.Time.Compare(b.Time) })

	// "Invalid user x from ip" is followed by "Failed password for invalid
	// user x from ip" when a password is tried: count that as one attempt.
	pending := map[string]int{}
	list := make([]Login, 0, len(raw))
	for _, l := range raw {
		key := l.pid + " " + l.IP + " " + l.port
		if l.Result == "invalid_user" && l.Method == "password" {
			if i, ok := pending[key]; ok {
				list[i].Method = "password"
				delete(pending, key)
				continue
			}
		}
		if l.Result == "invalid_user" && l.Method == "" {
			pending[key] = len(list)
		}
		list = append(list, l.Login)
	}
	slices.Reverse(list)
	if len(list) > MaxLogins {
		list = list[:MaxLogins]
	}
	return list
}

func parseMessage(msg string) (rawLogin, bool) {
	var l rawLogin
	var ip string
	if mm := acceptedRe.FindStringSubmatch(msg); mm != nil {
		l.Result, l.Method, l.User, ip, l.port = "accepted", mm[1], mm[2], mm[3], mm[4]
	} else if mm := failedRe.FindStringSubmatch(msg); mm != nil {
		l.Result, l.Method, l.User, ip, l.port = "failed", "password", mm[2], mm[3], mm[4]
		if mm[1] != "" {
			l.Result = "invalid_user"
		}
	} else if mm := invalidRe.FindStringSubmatch(msg); mm != nil {
		l.Result, l.User, ip, l.port = "invalid_user", mm[1], mm[2], mm[3]
	} else {
		return l, false
	}
	a, err := netip.ParseAddr(ip)
	if err != nil {
		return l, false
	}
	l.IP = a.Unmap().String()
	l.User = truncate(l.User, 64)
	if len(l.Method) > 40 {
		l.Method = truncate(l.Method, 40)
	}
	return l, true
}
