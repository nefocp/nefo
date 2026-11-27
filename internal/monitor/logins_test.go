package monitor

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/procs"
	"github.com/nefocp/nefo/internal/store"
)

// fakeJournal is the SSH journal: entries with cursors c1, c2…
type fakeJournal struct {
	mu      sync.Mutex
	entries []procs.Login
	calls   []string // "after=<cursor>" or "since=<unix>"
	history []procs.Login
}

func (j *fakeJournal) add(l procs.Login) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.entries = append(j.entries, l)
}

func (j *fakeJournal) read(_ context.Context, cursor string, since time.Time) ([]procs.Login, string, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	start := 0
	if cursor != "" {
		j.calls = append(j.calls, "after="+cursor)
		var n int
		for i := range j.entries {
			if "c"+itoa(i+1) == cursor {
				n = i + 1
			}
		}
		start = n
	} else {
		j.calls = append(j.calls, "since="+itoa(int(since.Unix())))
		for start < len(j.entries) && j.entries[start].Time.Before(since) {
			start++
		}
	}
	if start >= len(j.entries) {
		return nil, "", nil
	}
	return append([]procs.Login(nil), j.entries[start:]...), "c" + itoa(len(j.entries)), nil
}

func itoa(n int) string { return strconv.Itoa(n) }

type loginTest struct {
	m     *Monitor
	j     *fakeJournal
	st    *store.Store
	clock time.Time
	mu    sync.Mutex
	sent  []string
}

func newLoginTest(t *testing.T, s Settings) *loginTest {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	lt := &loginTest{st: st, j: &fakeJournal{}, clock: time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)}
	lt.m = &Monitor{store: st, log: slog.New(slog.DiscardHandler), active: map[string]time.Time{},
		logins: &loginWatch{now: func() time.Time { return lt.clock }}}
	lt.m.logins.send = func(_ context.Context, _ Settings, text string) {
		lt.mu.Lock()
		lt.sent = append(lt.sent, text)
		lt.mu.Unlock()
	}
	lt.m.SetSSHSource(lt.j.read, func(context.Context) ([]procs.Login, error) { return lt.j.history, nil })
	lt.save(t, s)
	return lt
}

func (lt *loginTest) save(t *testing.T, s Settings) {
	t.Helper()
	b, _ := json.Marshal(s)
	if err := lt.st.SetSetting(context.Background(), settingsKey, string(b)); err != nil {
		t.Fatal(err)
	}
}

// tick is one minute of Run.
func (lt *loginTest) tick() []string {
	lt.clock = lt.clock.Add(time.Minute)
	lt.m.watchSSH(context.Background())
	lt.mu.Lock()
	defer lt.mu.Unlock()
	out := lt.sent
	lt.sent = nil
	return out
}

func (lt *loginTest) login(user, ip, method string) {
	lt.j.add(procs.Login{Time: lt.clock.Add(10 * time.Second), Result: "accepted", User: user, IP: ip, Method: method})
}

func (lt *loginTest) fail(ip string, n int) {
	for i := 0; i < n; i++ {
		lt.j.add(procs.Login{Time: lt.clock.Add(time.Duration(i) * time.Millisecond), Result: "failed", User: "root", IP: ip, Method: "password"})
	}
}

const hook = "https://hooks.example.net/x"

func TestSSHLoginAlerts(t *testing.T) {
	lt := newLoginTest(t, Settings{WebhookURL: hook, SSHLogins: LoginsAll})
	lt.j.history = []procs.Login{{Time: lt.clock.Add(-48 * time.Hour), Result: "accepted", User: "root", IP: "203.0.113.10"}}
	// A sign-in before the alerts were on is never reported.
	lt.login("root", "198.51.100.99", "password")
	if got := lt.tick(); len(got) != 0 {
		t.Fatalf("first tick sent %v", got)
	}
	if len(lt.j.calls) != 0 {
		t.Fatalf("the first tick read the journal: %v", lt.j.calls)
	}
	if got := lt.tick(); len(got) != 0 {
		t.Fatalf("old sign-in reported: %v", got)
	}

	lt.login("root", "203.0.113.10", "publickey")
	got := lt.tick()
	if len(got) != 1 || !strings.Contains(got[0], "SSH sign-in: root from 203.0.113.10 with a key") || strings.Contains(got[0], "new address") {
		t.Fatalf("known address: %v", got)
	}
	lt.login("deploy", "198.51.100.7", "password")
	lt.login("root", "2001:db8:1:2::5", "keyboard-interactive/pam")
	got = lt.tick()
	if len(got) != 1 || !strings.Contains(got[0], "2 SSH sign-ins:") || !strings.Contains(got[0], "deploy from 198.51.100.7 (new address) with a password") ||
		!strings.Contains(got[0], "keyboard-interactive") {
		t.Fatalf("two sign-ins: %v", got)
	}
	if got := lt.tick(); len(got) != 0 {
		t.Fatalf("nothing new, sent %v", got)
	}

	// Only new addresses; the same /64 is not new.
	lt.save(t, Settings{WebhookURL: hook, SSHLogins: LoginsNewIP})
	lt.login("root", "203.0.113.10", "publickey")
	lt.login("root", "2001:db8:1:2::99", "publickey")
	if got := lt.tick(); len(got) != 0 {
		t.Fatalf("known addresses reported: %v", got)
	}
	lt.login("root", "192.0.2.50", "publickey")
	if got := lt.tick(); len(got) != 1 || !strings.Contains(got[0], "root from 192.0.2.50 (new address)") {
		t.Fatalf("new address: %v", got)
	}

	// Off: the place is forgotten; on again, it starts from then.
	lt.save(t, Settings{WebhookURL: hook})
	lt.login("root", "192.0.2.77", "publickey")
	lt.tick()
	if v, _ := lt.st.Setting(context.Background(), watchKey); v != "" {
		t.Fatalf("cursor kept while off: %s", v)
	}
	lt.save(t, Settings{WebhookURL: hook, SSHLogins: LoginsAll})
	lt.tick()
	lt.tick()
	if got := lt.tick(); len(got) != 0 {
		t.Fatalf("sign-in from while off reported: %v", got)
	}

	// Without any channel nothing is read.
	lt.save(t, Settings{SSHLogins: LoginsAll})
	n := len(lt.j.calls)
	lt.tick()
	if len(lt.j.calls) != n {
		t.Fatal("read the journal without a channel")
	}
}

func TestSSHStaleCursor(t *testing.T) {
	lt := newLoginTest(t, Settings{WebhookURL: hook, SSHLogins: LoginsAll})
	lt.tick()
	lt.login("root", "203.0.113.10", "publickey")
	lt.tick()
	// The panel was stopped for three hours: only the last hour is read.
	lt.clock = lt.clock.Add(3 * time.Hour)
	lt.m.logins.readAt = time.Time{}
	lt.j.add(procs.Login{Time: lt.clock.Add(-2 * time.Hour), Result: "accepted", User: "root", IP: "192.0.2.1"})
	lt.j.add(procs.Login{Time: lt.clock.Add(-10 * time.Minute), Result: "accepted", User: "root", IP: "192.0.2.2"})
	got := lt.tick()
	if len(got) != 1 || strings.Contains(got[0], "192.0.2.1") || !strings.Contains(got[0], "192.0.2.2") {
		t.Fatalf("after a pause: %v", got)
	}
	if last := lt.j.calls[len(lt.j.calls)-1]; !strings.HasPrefix(last, "since=") {
		t.Fatalf("read %s", last)
	}
}

func TestSSHFailedBurst(t *testing.T) {
	lt := newLoginTest(t, Settings{WebhookURL: hook, SSHFailed: 20})
	lt.tick()
	lt.fail("203.0.113.5", 12)
	if got := lt.tick(); len(got) != 0 {
		t.Fatalf("12 attempts: %v", got)
	}
	lt.fail("203.0.113.5", 3)
	lt.fail("198.51.100.7", 6)
	got := lt.tick()
	if len(got) != 1 || !strings.Contains(got[0], "21 failed SSH sign-in attempts in the last 10 minutes, from 203.0.113.5 (15), 198.51.100.7 (6)") {
		t.Fatalf("burst: %v", got)
	}
	// Once an hour at most.
	lt.fail("203.0.113.5", 30)
	if got := lt.tick(); len(got) != 0 {
		t.Fatalf("second burst within the hour: %v", got)
	}
	// Attempts older than ten minutes are forgotten.
	for i := 0; i < 70; i++ {
		lt.tick()
	}
	lt.fail("203.0.113.5", 19)
	if got := lt.tick(); len(got) != 0 {
		t.Fatalf("old attempts counted: %v", got)
	}
	lt.fail("203.0.113.5", 1)
	if got := lt.tick(); len(got) != 1 {
		t.Fatalf("new burst: %v", got)
	}
	// Sign-ins are not reported with only the burst alert on.
	lt.login("root", "192.0.2.9", "publickey")
	if got := lt.tick(); len(got) != 0 {
		t.Fatalf("sign-in reported: %v", got)
	}
}

func TestPanelLoginAlerts(t *testing.T) {
	ctx := context.Background()
	lt := newLoginTest(t, Settings{WebhookURL: hook, PanelLogins: LoginsNewIP})
	lt.st.Audit(ctx, "admin", "login", "203.0.113.10", "")
	sent := make(chan string, 4)
	lt.m.logins.send = func(_ context.Context, _ Settings, text string) { sent <- text }
	wait := func() string {
		select {
		case s := <-sent:
			return s
		case <-time.After(200 * time.Millisecond):
			return ""
		}
	}
	lt.m.PanelLogin(ctx, "admin", "203.0.113.10")
	if s := wait(); s != "" {
		t.Fatalf("address from the audit log reported: %s", s)
	}
	lt.m.PanelLogin(ctx, "admin", "198.51.100.7")
	if s := wait(); !strings.Contains(s, "Panel sign-in: admin from 198.51.100.7 (new address)") {
		t.Fatalf("new address: %q", s)
	}
	lt.m.PanelLogin(ctx, "admin", "198.51.100.7")
	if s := wait(); s != "" {
		t.Fatalf("second time: %s", s)
	}
	lt.save(t, Settings{WebhookURL: hook, PanelLogins: LoginsAll})
	lt.m.PanelLogin(ctx, "admin", "198.51.100.7")
	if s := wait(); !strings.Contains(s, "Panel sign-in: admin from 198.51.100.7,") {
		t.Fatalf("all: %q", s)
	}
	lt.save(t, Settings{WebhookURL: hook})
	lt.m.PanelLogin(ctx, "admin", "192.0.2.1")
	if s := wait(); s != "" {
		t.Fatalf("off: %s", s)
	}
}

func TestLoginSettings(t *testing.T) {
	for _, s := range []Settings{{SSHLogins: "yes"}, {PanelLogins: "x"}, {SSHFailed: 2}, {SSHFailed: -1}, {SSHFailed: 100001}} {
		if code(s.Validate()) != "invalid_login_alerts" {
			t.Errorf("accepted %+v", s)
		}
	}
	for _, s := range []Settings{{SSHLogins: LoginsAll, SSHFailed: 20, PanelLogins: LoginsNewIP}, {SSHLogins: LoginsNewIP, SSHFailed: 3}} {
		if err := s.Validate(); err != nil {
			t.Errorf("refused %+v: %v", s, err)
		}
	}
}

func TestPrune(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	set := map[string]int64{"old": now.Add(-200 * 24 * time.Hour).Unix()}
	for i := 0; i < knownMax+10; i++ {
		set["k"+itoa(i)] = now.Add(-time.Duration(i) * time.Minute).Unix()
	}
	prune(set, now)
	if len(set) != knownMax || set["old"] != 0 || set["k0"] == 0 || set["k"+itoa(knownMax+5)] != 0 {
		t.Fatalf("%d left", len(set))
	}
}
