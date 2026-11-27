package procs

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"time"
)

// SSHJournal reads the SSH server's journal after cursor (or, without a
// cursor, from since on) and returns the sign-ins and failed attempts in
// it, oldest first, with the cursor of the last entry read ("" when there
// was none: keep the old one). It is the sign-in alerts' once-a-minute
// read: nothing follows the journal in between.
func (m *Manager) SSHJournal(ctx context.Context, cursor string, since time.Time) ([]Login, string, error) {
	args := slices.Clone(sshJournal[:4]) // -u ssh.service -u sshd.service
	args = append(args, "--no-pager", "-q", "-o", "json", "--output-fields=MESSAGE,_PID")
	if cursor != "" {
		args = append(args, "--after-cursor="+cursor)
	} else {
		args = append(args, "--since=@"+strconv.FormatInt(since.Unix(), 10))
	}
	out, err := m.run.Output(ctx, "journalctl", args...)
	if err != nil {
		return nil, "", err
	}
	logins, last := parseJournal(out)
	return logins, last, nil
}

type cursorLine struct {
	journalLine
	Cursor string `json:"__CURSOR"`
}

// parseJournal reads journalctl's JSON lines: the sign-ins and attempts,
// oldest first, and the last entry's cursor.
func parseJournal(out string) ([]Login, string) {
	var raw []rawLogin
	last := ""
	for _, line := range bytes.Split([]byte(out), []byte("\n")) {
		var jl cursorLine
		if len(bytes.TrimSpace(line)) == 0 || json.Unmarshal(line, &jl) != nil {
			continue
		}
		if jl.Cursor != "" {
			last = jl.Cursor
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
	// As in parseLogins: "Invalid user x" then "Failed password for
	// invalid user x" from the same connection is one attempt.
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
	return list, last
}

// SSHSignIns returns the successful SSH sign-ins of the last 30 days (at
// most the last 2000), oldest first: the addresses the sign-in alerts know
// when they are turned on. Unlike SSHLogins, failed attempts can't push
// them out of the list.
func (m *Manager) SSHSignIns(ctx context.Context) ([]Login, error) {
	out, err := m.run.Output(ctx, "journalctl", append(slices.Clone(sshJournal), "-n", "2000", "-g", "^Accepted ")...)
	if err != nil {
		// journalctl without pattern matching support: filter here.
		if out, err = m.run.Output(ctx, "journalctl", append(slices.Clone(sshJournal), "-n", "20000")...); err != nil {
			return nil, err
		}
	}
	all, _ := parseJournal(out)
	list := all[:0]
	for _, l := range all {
		if l.Result == "accepted" {
			list = append(list, l)
		}
	}
	return list, nil
}
