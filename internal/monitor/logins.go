package monitor

// Sign-in alerts: SSH sign-ins and bursts of failed SSH attempts, read
// from the SSH server's journal once a minute in Run, from a cursor kept in
// the settings (nothing follows the journal in between), and sign-ins to
// the panel itself (PanelLogin).

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nefocp/nefo/internal/procs"
	"github.com/nefocp/nefo/internal/usererr"
)

// Sign-in alert modes (Settings.SSHLogins and PanelLogins).
const (
	LoginsOff   = ""
	LoginsAll   = "all"    // every sign-in
	LoginsNewIP = "new_ip" // only from an address the account has not signed in from before
)

// Limits of the sign-in alerts.
const (
	burstWindow    = 10 * time.Minute // failed attempts are counted over this long
	burstEvery     = time.Hour        // at most one burst alert this often
	staleCursor    = time.Hour        // after a longer pause, only the last hour is read
	knownMax       = 500              // addresses remembered per kind (ssh, panel)
	knownFor       = 180 * 24 * time.Hour
	maxLoginLines  = 10 // sign-ins listed in one alert
	journalTimeout = 20 * time.Second
)

// SSHSource reads the SSH server's journal after cursor, or from since on
// without one (procs.Manager.SSHJournal).
type SSHSource func(ctx context.Context, cursor string, since time.Time) ([]procs.Login, string, error)

// SSHHistory returns the SSH sign-ins of the last weeks
// (procs.Manager.SSHSignIns), from which the known addresses are learnt
// when the alerts are turned on.
type SSHHistory func(ctx context.Context) ([]procs.Login, error)

const (
	watchKey = "ssh_alert_cursor" // JSON of watchState
	knownKey = "login_known_ips"  // JSON of map kind → "user address" → last sign-in (Unix)
)

type watchState struct {
	Cursor string    `json:"cursor,omitempty"`
	Since  time.Time `json:"since"` // the start, before the first entry was read
	At     time.Time `json:"at"`    // when Cursor was read
}

// loginWatch is the in-memory part of the sign-in alerts.
type loginWatch struct {
	mu      sync.Mutex
	source  SSHSource
	history SSHHistory
	readAt  time.Time
	failed  map[time.Time]map[string]int // minute → address → failed attempts
	burstAt time.Time
	now     func() time.Time
	// send delivers an alert; tests replace it.
	send func(ctx context.Context, s Settings, text string)
}

// SetSSHSource gives the monitor the SSH journal to watch.
func (m *Monitor) SetSSHSource(src SSHSource, hist SSHHistory) {
	m.logins.mu.Lock()
	defer m.logins.mu.Unlock()
	m.logins.source, m.logins.history = src, hist
}

func validLoginMode(s string) bool { return s == LoginsOff || s == LoginsAll || s == LoginsNewIP }

// validateLogins checks the sign-in alert settings.
func (s *Settings) validateLogins() error {
	if !validLoginMode(s.SSHLogins) || !validLoginMode(s.PanelLogins) || (s.SSHFailed != 0 && (s.SSHFailed < 3 || s.SSHFailed > 100000)) {
		return usererr.New("invalid_login_alerts")
	}
	return nil
}

func (m *Monitor) alert(ctx context.Context, s Settings, text string) {
	if m.logins.send != nil {
		m.logins.send(ctx, s, text)
		return
	}
	m.send(ctx, s, text)
}

func (m *Monitor) loadJSON(ctx context.Context, key string, v any) bool {
	raw, err := m.store.Setting(ctx, key)
	return err == nil && raw != "" && json.Unmarshal([]byte(raw), v) == nil
}

func (m *Monitor) saveJSON(ctx context.Context, key string, v any) {
	b, _ := json.Marshal(v)
	if err := m.store.SetSetting(ctx, key, string(b)); err != nil {
		m.log.Warn("save sign-in alert state", "err", err)
	}
}

// watchSSH reads what the SSH server logged since the last minute and
// sends the alerts it calls for. With the SSH alerts off it reads nothing
// and forgets its place, so turning them on never reports old sign-ins.
func (m *Monitor) watchSSH(ctx context.Context) {
	s, err := m.settings(ctx)
	if err != nil {
		return
	}
	// Sent after the lock is released: a slow channel must not hold up a
	// panel sign-in (PanelLogin takes the lock too).
	for _, text := range m.readSSH(ctx, s) {
		m.alert(ctx, s, text)
	}
}

// readSSH does watchSSH's reading and returns the alerts to send.
func (m *Monitor) readSSH(ctx context.Context, s Settings) []string {
	lw := m.logins
	lw.mu.Lock()
	defer lw.mu.Unlock()
	var st watchState
	saved := m.loadJSON(ctx, watchKey, &st)
	if lw.source == nil || !s.HasChannel() || (s.SSHLogins == LoginsOff && s.SSHFailed == 0) {
		if saved {
			m.store.SetSetting(ctx, watchKey, "")
		}
		lw.failed, lw.readAt = nil, time.Time{}
		return nil
	}
	now := lw.now()
	if !saved {
		// Turned on: start from now, knowing the addresses of the last weeks.
		m.learnHistory(ctx)
		m.saveJSON(ctx, watchKey, watchState{Since: now})
		lw.readAt = now
		return nil
	}
	cursor, since := st.Cursor, st.Since
	last := st.At
	if lw.readAt.After(last) {
		last = lw.readAt
	}
	if cursor != "" && now.Sub(last) > staleCursor {
		// The panel was stopped for a while: an alert about every sign-in of
		// a long pause would come too late to help, so read the last hour.
		cursor, since = "", now.Add(-staleCursor)
	}
	rctx, cancel := context.WithTimeout(ctx, journalTimeout)
	defer cancel()
	logins, next, err := lw.source(rctx, cursor, since)
	if err != nil {
		m.log.Debug("read the SSH journal", "err", err)
		return nil
	}
	lw.readAt = now
	if next != "" && next != st.Cursor {
		m.saveJSON(ctx, watchKey, watchState{Cursor: next, At: now})
	}

	var send []string
	var accepted []procs.Login
	for _, l := range logins {
		switch l.Result {
		case "accepted":
			accepted = append(accepted, l)
		case "failed", "invalid_user":
			lw.countFailure(l)
		}
	}
	if s.SSHLogins != LoginsOff && len(accepted) > 0 {
		var lines []string
		for _, l := range accepted {
			isNew := m.learn(ctx, "ssh", l.User, l.IP, l.Time)
			if s.SSHLogins == LoginsNewIP && !isNew {
				continue
			}
			lines = append(lines, sshLine(l, isNew))
		}
		if text := loginText("SSH sign-in", "SSH sign-ins", "🔑", lines); text != "" {
			send = append(send, text)
		}
	} else {
		for _, l := range accepted {
			m.learn(ctx, "ssh", l.User, l.IP, l.Time)
		}
	}
	if s.SSHFailed > 0 {
		if text := lw.burst(now, s.SSHFailed); text != "" {
			send = append(send, text)
		}
	}
	return send
}

// countFailure adds a failed attempt to its minute.
func (lw *loginWatch) countFailure(l procs.Login) {
	if lw.failed == nil {
		lw.failed = map[time.Time]map[string]int{}
	}
	minute := l.Time.Truncate(time.Minute)
	if lw.failed[minute] == nil {
		lw.failed[minute] = map[string]int{}
	}
	lw.failed[minute][l.IP]++
}

// burst forgets attempts older than burstWindow and returns an alert when
// the others reach limit (at most once per burstEvery).
func (lw *loginWatch) burst(now time.Time, limit int) string {
	byIP := map[string]int{}
	total := 0
	for minute, ips := range lw.failed {
		if now.Sub(minute) > burstWindow {
			delete(lw.failed, minute)
			continue
		}
		for ip, n := range ips {
			byIP[ip] += n
			total += n
		}
	}
	if total < limit || now.Sub(lw.burstAt) < burstEvery {
		return ""
	}
	lw.burstAt = now
	ips := make([]string, 0, len(byIP))
	for ip := range byIP {
		ips = append(ips, ip)
	}
	sort.Slice(ips, func(i, j int) bool {
		if byIP[ips[i]] != byIP[ips[j]] {
			return byIP[ips[i]] > byIP[ips[j]]
		}
		return ips[i] < ips[j]
	})
	var top []string
	for i, ip := range ips {
		if i == 5 {
			top = append(top, fmt.Sprintf("+%d more", len(ips)-5))
			break
		}
		top = append(top, fmt.Sprintf("%s (%d)", ip, byIP[ip]))
	}
	return fmt.Sprintf("⚠️ %d failed SSH sign-in attempts in the last %d minutes, from %s.", total, int(burstWindow.Minutes()), strings.Join(top, ", "))
}

func method(m string) string {
	switch {
	case m == "publickey":
		return "with a key"
	case m == "password":
		return "with a password"
	case strings.HasPrefix(m, "keyboard-interactive"):
		return "with a password (keyboard-interactive)"
	case m == "":
		return ""
	}
	return "with " + m
}

func when(t time.Time) string { return t.In(time.Local).Format("2006-01-02 15:04:05 MST") }

func sshLine(l procs.Login, isNew bool) string {
	parts := []string{l.User + " from " + l.IP}
	if isNew {
		parts[0] += " (new address)"
	}
	if m := method(l.Method); m != "" {
		parts = append(parts, m)
	}
	return strings.Join(parts, " ") + ", " + when(l.Time)
}

// loginText makes one alert of the sign-in lines.
func loginText(one, many, icon string, lines []string) string {
	switch len(lines) {
	case 0:
		return ""
	case 1:
		return icon + " " + one + ": " + lines[0]
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s %d %s:", icon, len(lines), many)
	for i, l := range lines {
		if i == maxLoginLines {
			fmt.Fprintf(&b, "\n… and %d more", len(lines)-maxLoginLines)
			break
		}
		b.WriteString("\n- " + l)
	}
	return b.String()
}

// ---- known addresses ----

// addrKey groups IPv6 addresses by /64, which usually belongs to one
// person (their addresses change inside it).
func addrKey(ip string) string {
	a, err := netip.ParseAddr(ip)
	if err != nil {
		return ip
	}
	a = a.Unmap()
	if a.Is6() {
		if p, err := a.Prefix(64); err == nil {
			return p.String()
		}
	}
	return a.String()
}

// learn records a sign-in of user from ip and reports whether that address
// is new for the account.
func (m *Monitor) learn(ctx context.Context, kind, user, ip string, at time.Time) bool {
	known := map[string]map[string]int64{}
	m.loadJSON(ctx, knownKey, &known)
	if known[kind] == nil {
		known[kind] = map[string]int64{}
	}
	k := user + " " + addrKey(ip)
	_, seen := known[kind][k]
	known[kind][k] = at.Unix()
	prune(known[kind], at)
	m.saveJSON(ctx, knownKey, known)
	return !seen
}

// prune forgets addresses not seen for knownFor, and the oldest beyond
// knownMax.
func prune(set map[string]int64, now time.Time) {
	for k, t := range set {
		if now.Sub(time.Unix(t, 0)) > knownFor {
			delete(set, k)
		}
	}
	if len(set) <= knownMax {
		return
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return set[keys[i]] > set[keys[j]] })
	for _, k := range keys[knownMax:] {
		delete(set, k)
	}
}

// learnHistory remembers the addresses of the SSH sign-ins of the last
// weeks, when the SSH alerts are turned on, so the first sign-in from the
// administrator's usual address is not reported as new.
func (m *Monitor) learnHistory(ctx context.Context) {
	if m.logins.history == nil {
		return
	}
	hist, err := m.logins.history(ctx)
	if err != nil {
		return
	}
	known := map[string]map[string]int64{}
	m.loadJSON(ctx, knownKey, &known)
	if known["ssh"] == nil {
		known["ssh"] = map[string]int64{}
	}
	for _, l := range hist {
		if k := l.User + " " + addrKey(l.IP); l.Result == "accepted" && known["ssh"][k] < l.Time.Unix() {
			known["ssh"][k] = l.Time.Unix()
		}
	}
	prune(known["ssh"], m.logins.now())
	m.saveJSON(ctx, knownKey, known)
}

// PanelLogin reports a sign-in to the panel, when its alerts are on, and
// returns without waiting for the channels. It is called before the
// sign-in is written to the audit log, from which the known addresses are
// learnt the first time.
func (m *Monitor) PanelLogin(ctx context.Context, user, ip string) {
	s, err := m.settings(ctx)
	if err != nil || !s.HasChannel() || s.PanelLogins == LoginsOff {
		return
	}
	m.logins.mu.Lock()
	known := map[string]map[string]int64{}
	if m.loadJSON(ctx, knownKey, &known); known["panel"] == nil {
		known["panel"] = map[string]int64{}
		if entries, err := m.store.RecentAudit(ctx, 2000); err == nil {
			for _, e := range entries {
				if e.Action == "login" {
					if k := e.Username + " " + addrKey(e.IP); known["panel"][k] < e.Time.Unix() {
						known["panel"][k] = e.Time.Unix()
					}
				}
			}
		}
		m.saveJSON(ctx, knownKey, known)
	}
	now := m.logins.now()
	isNew := m.learn(ctx, "panel", user, ip, now)
	m.logins.mu.Unlock()
	if s.PanelLogins == LoginsNewIP && !isNew {
		return
	}
	line := user + " from " + ip
	if isNew {
		line += " (new address)"
	}
	// The sign-in must not wait for the channels.
	go m.alert(context.WithoutCancel(ctx), s, "🔐 Panel sign-in: "+line+", "+when(now))
}
