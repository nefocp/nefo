package procs

import (
	"context"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/usererr"
)

type fakeRunner struct {
	calls   []string
	outputs map[string]string // by program name, or "name firstArg"
	fail    map[string]error
}

func (f *fakeRunner) Output(_ context.Context, name string, args ...string) (string, error) {
	f.calls = append(f.calls, strings.TrimSpace(name+" "+strings.Join(args, " ")))
	key := name
	if len(args) > 0 {
		if _, ok := f.outputs[name+" "+args[0]]; ok {
			key = name + " " + args[0]
		}
	}
	return f.outputs[key], f.fail[key]
}

func (f *fakeRunner) Input(ctx context.Context, _ []byte, name string, args ...string) (string, error) {
	return f.Output(ctx, name, args...)
}

func (f *fakeRunner) Stream(ctx context.Context, _ io.Writer, name string, args ...string) error {
	_, err := f.Output(ctx, name, args...)
	return err
}

func (f *fakeRunner) Pipe(ctx context.Context, _ io.Reader, _ io.Writer, name string, args ...string) error {
	_, err := f.Output(ctx, name, args...)
	return err
}

func code(err error) string {
	var ue *usererr.Error
	if errors.As(err, &ue) {
		return ue.Code
	}
	if err == nil {
		return ""
	}
	return "other: " + err.Error()
}

// From Ubuntu 24.04 (procps-ng 4.0.4).
const psOut = ` 119003   26680 root                              6.7  0.5 10700       1 sshd: root@notty
  43314       1 root                              2.5  1.8 37652   11990 /usr/bin/containerd
  20062       1 mysql                             0.4  9.7 196872  15256 /usr/sbin/mariadbd
      1       0 root                              0.3  0.6 12900   20838 /sbin/init splash
      2       0 root                              0.0  0.0     0   20838 [kthreadd]
  86659       2 root                              0.2  0.0     0    2748 [kworker/u4:6-events_power_efficient]
    983       1 messagebus                        0.0  0.2  5264   20802 @dbus-daemon --system --address=systemd: --nofork
 118334       1 root                              0.0  1.1 22628     824 /usr/local/bin/nefo serve --config /etc/nefo/config.json
 119052  118334 root                              150  0.2  4560       0 ps -e -ww --no-headers -o pid,ppid,user:32,pcpu,pmem,rss,etimes,args
   6666    5555 w_example_com                      1.0  0.1  2048      60 [kworker/0:0]
   7777       1 1234567                           0.0  0.0   100      10 php  -r   'sleep(1);'
garbage line
`

func newTest() (*Manager, *fakeRunner) {
	r := &fakeRunner{outputs: map[string]string{}, fail: map[string]error{}}
	m := New(r)
	m.self, m.parent = 118334, 1
	m.ticks = func() map[int]cpuSample { return nil }
	m.sleep = func(time.Duration) {}
	return m, r
}

func find(list []Process, pid int) *Process {
	for i := range list {
		if list[i].PID == pid {
			return &list[i]
		}
	}
	return nil
}

func TestList(t *testing.T) {
	m, r := newTest()
	r.outputs["ps"] = psOut
	list, total, err := m.List(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if total != 10 || len(list) != 10 {
		t.Fatalf("total %d, %d listed", total, len(list))
	}
	if find(list, 119052) != nil {
		t.Fatal("the panel's own ps is listed")
	}
	if list[0].PID != 119003 || list[1].PID != 43314 {
		t.Fatalf("not sorted by CPU: %d %d", list[0].PID, list[1].PID)
	}
	maria := find(list, 20062)
	if maria.User != "mysql" || maria.CPU != 0.4 || maria.Mem != 9.7 || maria.RSSMB != 192.3 || maria.Elapsed != 15256 || maria.Command != "/usr/sbin/mariadbd" {
		t.Fatalf("mariadb: %+v", maria)
	}
	if p := find(list, 7777); p.User != "1234567" || p.Command != "php  -r   'sleep(1);'" {
		t.Fatalf("spaces in the command line: %+v", p)
	}
	if !find(list, 2).Kernel || !find(list, 86659).Kernel {
		t.Fatal("kernel threads not marked")
	}
	if find(list, 6666).Kernel {
		t.Fatal("a program named [kworker/0:0] is not a kernel thread")
	}
	if !find(list, 1).Protected || !find(list, 118334).Protected || find(list, 20062).Protected {
		t.Fatal("protected flags wrong")
	}
	if !slices.Contains(r.calls, "ps -e -ww --no-headers -o pid,ppid,user:32,pcpu,pmem,rss,etimes,args") {
		t.Fatalf("calls %q", r.calls)
	}

	list, _, _ = m.List(context.Background(), "mem")
	if list[0].PID != 20062 {
		t.Fatalf("not sorted by memory: %d", list[0].PID)
	}
	list, _, _ = m.List(context.Background(), "pid")
	if list[0].PID != 1 || list[1].PID != 2 {
		t.Fatalf("not sorted by PID: %d %d", list[0].PID, list[1].PID)
	}
}

func TestListLimitAndTruncate(t *testing.T) {
	m, r := newTest()
	var b strings.Builder
	for i := 1; i <= 400; i++ {
		b.WriteString(strings.Repeat(" ", 3))
		b.WriteString(strconv.Itoa(i + 1000))
		b.WriteString(" 1 root 0.0 0.0 100 5 /bin/x ")
		b.WriteString(strings.Repeat("é", 200))
		b.WriteString("\n")
	}
	r.outputs["ps"] = b.String()
	list, total, err := m.List(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if total != 400 || len(list) != MaxProcesses {
		t.Fatalf("total %d, listed %d", total, len(list))
	}
	c := list[0].Command
	if !strings.HasSuffix(c, "…") || len(c) > maxCommand+len("…") || strings.ContainsRune(c, '�') {
		t.Fatalf("command not cut cleanly: %d bytes %q", len(c), c[len(c)-10:])
	}
}

func TestKill(t *testing.T) {
	m, r := newTest()
	ctx := context.Background()
	r.outputs["ps -p"] = "  20062       1 mysql   0.4  9.7 196872  15256 /usr/sbin/mariadbd\n"
	p, err := m.Kill(ctx, 20062, "TERM")
	if err != nil || p.User != "mysql" {
		t.Fatalf("kill: %+v %v", p, err)
	}
	if r.calls[len(r.calls)-1] != "kill -s TERM 20062" {
		t.Fatalf("calls %q", r.calls)
	}
	if _, err := m.Kill(ctx, 20062, "KILL"); err != nil || r.calls[len(r.calls)-1] != "kill -s KILL 20062" {
		t.Fatalf("KILL: %v %q", err, r.calls)
	}

	for _, c := range []struct {
		pid    int
		signal string
		code   string
	}{
		{20062, "HUP", "invalid_signal"},
		{20062, "", "invalid_signal"},
		{20062, "-9", "invalid_signal"},
		{1, "KILL", "kill_init"},
		{118334, "TERM", "kill_panel"},
		{0, "TERM", "process_not_found"},
		{-5, "TERM", "process_not_found"},
	} {
		r.calls = nil
		if _, err := m.Kill(ctx, c.pid, c.signal); code(err) != c.code {
			t.Errorf("Kill(%d, %q) = %v, want %s", c.pid, c.signal, err, c.code)
		}
		if slices.ContainsFunc(r.calls, func(s string) bool { return strings.HasPrefix(s, "kill") }) {
			t.Errorf("Kill(%d, %q) sent a signal", c.pid, c.signal)
		}
	}
	m.parent = 5555
	if _, err := m.Kill(ctx, 5555, "TERM"); code(err) != "kill_panel" {
		t.Fatalf("parent: %v", err)
	}

	r.outputs["ps -p"] = "  86659       2 root  0.2  0.0  0  2748 [kworker/u4:6-events_power_efficient]\n"
	if _, err := m.Kill(ctx, 86659, "KILL"); code(err) != "kill_kernel_thread" {
		t.Fatalf("kernel thread: %v", err)
	}
	r.outputs["ps -p"] = ""
	r.fail["ps -p"] = errors.New("exit status 1")
	if _, err := m.Kill(ctx, 99999, "TERM"); code(err) != "process_not_found" {
		t.Fatalf("missing process: %v", err)
	}

	r.outputs["ps -p"] = "  4242       1 root  0.0  0.0  0  1 sleep 300\n"
	delete(r.fail, "ps -p")
	r.outputs["kill"] = "kill: (4242): No such process\n"
	r.fail["kill"] = errors.New("exit status 1")
	if _, err := m.Kill(ctx, 4242, "TERM"); code(err) != "process_not_found" {
		t.Fatalf("exited meanwhile: %v", err)
	}
}

func TestParseStat(t *testing.T) {
	line := "4242 (a (weird) name) S 1 4242 4242 0 -1 4194560 120 0 0 0 250 75 0 0 20 0 1 0 98765 1000000 200 18446744073709551615\n"
	s, ok := parseStat(line)
	if !ok || s.ticks != 325 || s.start != 98765 {
		t.Fatalf("parseStat = %+v %v", s, ok)
	}
	if _, ok := parseStat("4242 (x) S 1 2"); ok {
		t.Fatal("short line accepted")
	}
}

func TestRecentCPU(t *testing.T) {
	m, r := newTest()
	r.outputs["ps"] = psOut
	now := time.Unix(1_000_000, 0)
	m.now = func() time.Time { return now }
	samples := []map[int]cpuSample{
		{20062: {ticks: 1000, start: 5}, 43314: {ticks: 10, start: 7}},
		{20062: {ticks: 1050, start: 5}, 43314: {ticks: 10, start: 7}, 6666: {ticks: 20, start: 9}},
		{20062: {ticks: 1550, start: 5}, 43314: {ticks: 5, start: 99}},
	}
	m.ticks = func() map[int]cpuSample {
		s := samples[0]
		if len(samples) > 1 {
			samples = samples[1:]
		}
		return s
	}
	slept := time.Duration(0)
	m.sleep = func(d time.Duration) { slept += d; now = now.Add(d) }

	// First call: a 0.5 s sample. MariaDB used 50 ticks = 0.5 s of CPU: 100%.
	list, _, err := m.List(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if slept != 500*time.Millisecond {
		t.Fatalf("slept %v", slept)
	}
	if p := find(list, 20062); p.CPU != 100 || list[0].PID != 20062 {
		t.Fatalf("mariadb %v, first %d", p.CPU, list[0].PID)
	}
	if find(list, 43314).CPU != 0 || find(list, 6666).CPU != 40 {
		t.Fatalf("containerd %v, new process %v", find(list, 43314).CPU, find(list, 6666).CPU)
	}
	// Without a sample, ps's own figure stays.
	if find(list, 119003).CPU != 6.7 {
		t.Fatalf("sshd %v", find(list, 119003).CPU)
	}

	// Five seconds later, compared with the previous call: no new sample.
	now = now.Add(5 * time.Second)
	list, _, _ = m.List(context.Background(), "")
	if slept != 500*time.Millisecond {
		t.Fatal("sampled again")
	}
	if find(list, 20062).CPU != 100 {
		t.Fatalf("mariadb %v", find(list, 20062).CPU)
	}
	// containerd's PID was reused by a new process: its 5 ticks in 5 s.
	if find(list, 43314).CPU != 1 {
		t.Fatalf("reused PID %v", find(list, 43314).CPU)
	}
}

// From Ubuntu 24.04 (iproute2 6.1.0).
const ssOut = `udp UNCONN 0      0                 127.0.0.54:53    0.0.0.0:* users:(("systemd-resolve",pid=604,fd=16))
udp UNCONN 0      0      198.51.100.42%enp1s0:68    0.0.0.0:* users:(("systemd-network",pid=890,fd=21))
tcp LISTEN 0      511                127.0.0.1:6379  0.0.0.0:* users:(("redis-server",pid=41850,fd=6))
tcp LISTEN 0      511                  0.0.0.0:443   0.0.0.0:* users:(("nginx",pid=118072,fd=9),("nginx",pid=118071,fd=9),("nginx",pid=81058,fd=9))
tcp LISTEN 0      128                  0.0.0.0:22    0.0.0.0:* users:(("sshd",pid=26680,fd=3))
tcp LISTEN 0      4096                       *:46949       *:* users:(("panel",pid=118334,fd=15))
tcp LISTEN 0      511                     [::]:443      [::]:* users:(("nginx",pid=118072,fd=10),("nginx",pid=81058,fd=10))
tcp LISTEN 0      511                    [::1]:6379     [::]:* users:(("redis-server",pid=41850,fd=7))
tcp LISTEN 0      64                   0.0.0.0:2049  0.0.0.0:*
tcp LISTEN 0      5                    0.0.0.0:9000  0.0.0.0:* users:(("my \"odd\" app",pid=7,fd=3))
`

func TestParseSS(t *testing.T) {
	list := parseSS(ssOut)
	if len(list) != 10 {
		t.Fatalf("%d ports", len(list))
	}
	got := []string{}
	for _, p := range list {
		got = append(got, p.Proto+" "+p.Address+" "+strconv.Itoa(p.Port)+" "+p.Process+" "+strconv.Itoa(p.PID)+map[bool]string{true: " local"}[p.Local])
	}
	want := []string{
		"tcp 0.0.0.0 22 sshd 26680",
		"udp 127.0.0.54 53 systemd-resolve 604 local",
		"udp 198.51.100.42%enp1s0 68 systemd-network 890",
		"tcp 0.0.0.0 443 nginx 81058",
		"tcp :: 443 nginx 81058",
		"tcp 0.0.0.0 2049  0",
		"tcp 127.0.0.1 6379 redis-server 41850 local",
		"tcp ::1 6379 redis-server 41850 local",
		`tcp 0.0.0.0 9000 my \"odd\" app 7`,
		"tcp * 46949 panel 118334",
	}
	slices.Sort(got)
	slices.Sort(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if list[0].Port != 22 || list[len(list)-1].Port != 46949 {
		t.Fatalf("not sorted by port: %d … %d", list[0].Port, list[len(list)-1].Port)
	}
	for _, p := range list {
		if p.Port == 443 && p.PID != 81058 {
			t.Fatalf("nginx PID %d, want the master 81058", p.PID)
		}
	}
}

func TestSSHLogins(t *testing.T) {
	m, r := newTest()
	r.outputs["journalctl"] = `{"MESSAGE":"Accepted publickey for root from 203.0.113.35 port 55671 ssh2: ED25519 SHA256:2rDy0kN4YzVt0nqKk8pW3sHq9xGv7LbJmE5uTcAf1Zo","_PID":"119394","__REALTIME_TIMESTAMP":"1795554925495839","SYSLOG_IDENTIFIER":"sshd"}
{"MESSAGE":"Failed password for root from 193.32.162.84 port 36730 ssh2","_PID":"1298","__REALTIME_TIMESTAMP":"1795554000000000"}
{"MESSAGE":"Invalid user sb from 109.160.32.138 port 38148","_PID":"2346","__REALTIME_TIMESTAMP":"1795554100000000"}
{"MESSAGE":"Failed password for invalid user sb from 109.160.32.138 port 38148 ssh2","_PID":"2346","__REALTIME_TIMESTAMP":"1795554104000000"}
{"MESSAGE":"Failed password for invalid user sb from 109.160.32.138 port 38148 ssh2","_PID":"2346","__REALTIME_TIMESTAMP":"1795554108000000"}
{"MESSAGE":"Invalid user  from 109.160.32.138 port 57218","_PID":"2340","__REALTIME_TIMESTAMP":"1795554050000000"}
{"MESSAGE":"Invalid user x from 1.2.3.4 port 1 from 10.0.0.9 port 22","_PID":"5","__REALTIME_TIMESTAMP":"1795554060000000"}
{"MESSAGE":[73,110,118,97,108,105,100,32,117,115,101,114,32,255,97,32,102,114,111,109,32,50,48,48,49,58,100,98,56,58,58,49,32,112,111,114,116,32,52,52],"_PID":"6","__REALTIME_TIMESTAMP":"1795554070000000"}
{"MESSAGE":"Accepted password for deploy from ::ffff:198.51.100.7 port 4000 ssh2","_PID":"7","__REALTIME_TIMESTAMP":"1795554080000000"}
{"MESSAGE":"Failed password for root from not-an-ip port 1 ssh2","_PID":"8","__REALTIME_TIMESTAMP":"1795554090000000"}
{"MESSAGE":"pam_unix(sshd:session): session opened for user root(uid=0) by root(uid=0)","_PID":"9","__REALTIME_TIMESTAMP":"1795554095000000"}
not json
`
	list, err := m.SSHLogins(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, l := range list {
		got = append(got, l.Result+" "+l.User+" "+l.IP+" "+l.Method)
	}
	want := []string{
		"accepted root 203.0.113.35 publickey",
		"invalid_user sb 109.160.32.138 password",
		"invalid_user sb 109.160.32.138 password",
		"accepted deploy 198.51.100.7 password",
		"invalid_user ?a 2001:db8::1 ",
		"invalid_user x from 1.2.3.4 port 1 10.0.0.9 ",
		"invalid_user  109.160.32.138 ",
		"failed root 193.32.162.84 password",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if !list[0].Time.Equal(time.UnixMicro(1795554925495839)) {
		t.Fatalf("time %v", list[0].Time)
	}
	call := r.calls[0]
	for _, part := range []string{"-u ssh.service", "-u sshd.service", "--since=-30d", "-o json", "-g ^(Accepted |Failed password |Invalid user )"} {
		if !strings.Contains(call, part) {
			t.Errorf("journalctl call lacks %q: %s", part, call)
		}
	}

	// Without --grep support, journalctl is asked again without it.
	r.calls = nil
	r.fail["journalctl"] = errors.New("exit status 1")
	if _, err := m.SSHLogins(context.Background()); err == nil || len(r.calls) != 2 || strings.Contains(r.calls[1], "-g") {
		t.Fatalf("fallback: %v %q", err, r.calls)
	}
}

func TestLoginLimit(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 300; i++ {
		b.WriteString(`{"MESSAGE":"Failed password for root from 192.0.2.1 port 1 ssh2","_PID":"1","__REALTIME_TIMESTAMP":"`)
		b.WriteString(strconv.Itoa(i) + "000000")
		b.WriteString("\"}\n")
	}
	list := parseLogins(b.String())
	if len(list) != MaxLogins || list[0].Time.Unix() != 299 {
		t.Fatalf("%d logins, newest %v", len(list), list[0].Time)
	}
}
