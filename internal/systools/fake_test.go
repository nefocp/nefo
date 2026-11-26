package systools

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/usererr"
)

// fake answers commands from canned output, chosen by the longest prefix
// of the command line that matches. apt-get runs through run.Apt are
// recorded as "APT <args>".
type fake struct {
	mu      sync.Mutex
	calls   []string
	answers map[string]string
	fail    map[string]string                            // prefix → output of a failing run
	fn      map[string]func(line string) (string, error) // dynamic answers
	hook    func(line string)                            // side effects, such as swapon changing /proc/swaps
}

func (f *fake) answer(line string) (string, error) {
	f.mu.Lock()
	f.calls = append(f.calls, line)
	hook := f.hook
	best, kind := "", ""
	for p := range f.answers {
		if strings.HasPrefix(line, p) && len(p) >= len(best) {
			best, kind = p, "answer"
		}
	}
	for p := range f.fail {
		if strings.HasPrefix(line, p) && len(p) >= len(best) {
			best, kind = p, "fail"
		}
	}
	for p := range f.fn {
		if strings.HasPrefix(line, p) && len(p) >= len(best) {
			best, kind = p, "fn"
		}
	}
	var fn func(string) (string, error)
	if kind == "fn" {
		fn = f.fn[best]
	}
	f.mu.Unlock()
	if hook != nil {
		hook(line)
	}
	switch kind {
	case "fail":
		return f.fail[best], errors.New("exit status 1")
	case "fn":
		return fn(line)
	}
	return f.answers[best], nil
}

func cmdline(name string, args []string) string {
	if name == "/bin/sh" && len(args) >= 11 && args[2] == "apt-get" {
		return strings.TrimSpace("APT " + strings.Join(args[11:], " "))
	}
	return strings.Join(append([]string{name}, args...), " ")
}

func (f *fake) Output(_ context.Context, name string, args ...string) (string, error) {
	return f.answer(cmdline(name, args))
}

func (f *fake) Stream(_ context.Context, w io.Writer, name string, args ...string) error {
	out, err := f.answer(cmdline(name, args))
	io.WriteString(w, out)
	return err
}

func (f *fake) Input(_ context.Context, _ []byte, name string, args ...string) (string, error) {
	return f.answer(cmdline(name, args))
}

func (f *fake) Pipe(_ context.Context, _ io.Reader, w io.Writer, name string, args ...string) error {
	out, err := f.answer(cmdline(name, args))
	io.WriteString(w, out)
	return err
}

// ran reports whether a command line starting with prefix ran.
func (f *fake) ran(prefix string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			return true
		}
	}
	return false
}

func (f *fake) reset() {
	f.mu.Lock()
	f.calls = nil
	f.mu.Unlock()
}

// memSettings is an in-memory settings table.
type memSettings struct {
	mu sync.Mutex
	m  map[string]string
}

func (s *memSettings) Setting(_ context.Context, k string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[k], nil
}

func (s *memSettings) SetSetting(_ context.Context, k, v string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v == "" {
		delete(s.m, k)
	} else {
		s.m[k] = v
	}
	return nil
}

// testLog collects a job's log.
type testLog struct{ strings.Builder }

func (l *testLog) Logf(format string, args ...any) { fmt.Fprintf(l, format+"\n", args...) }

type testEnv struct {
	m      *Manager
	f      *fake
	dir    string
	st     *memSettings
	free   uint64
	fsType string
	now    time.Time
}

// newTest returns a manager whose every path is in a temporary folder.
func newTest(t *testing.T) *testEnv {
	t.Helper()
	dir := t.TempDir()
	e := &testEnv{f: &fake{answers: map[string]string{}, fail: map[string]string{}, fn: map[string]func(string) (string, error){}},
		dir: dir, st: &memSettings{m: map[string]string{}}, free: 50 << 30, fsType: "ext4",
		now: time.Date(2026, 11, 26, 8, 0, 0, 0, time.UTC)}
	m := New(e.f, e.st)
	in := func(p string) string { return filepath.Join(dir, filepath.FromSlash(p)) }
	m.p = Paths{
		RebootRequired: in("run/reboot-required"),
		RebootPkgs:     in("run/reboot-required.pkgs"),
		AptLists:       in("var/lib/apt/lists"),
		AutoUpgrades:   in("etc/apt/apt.conf.d/20auto-upgrades"),
		Unattended:     in("usr/bin/unattended-upgrade"),
		Boot:           in("boot"),
		OSRelease:      in("proc/osrelease"),
		SwapFile:       in("panel.swap"),
		Fstab:          in("etc/fstab"),
		ProcSwaps:      in("proc/swaps"),
		Meminfo:        in("proc/meminfo"),
		Swappiness:     in("proc/sys/vm/swappiness"),
		SysctlSwap:     in("etc/sysctl.d/90-panel-swap.conf"),
		SysctlBBR:      in("etc/sysctl.d/90-panel-bbr.conf"),
		ModulesBBR:     in("etc/modules-load.d/panel-bbr.conf"),
		ProcCC:         in("proc/sys/net/ipv4/tcp_congestion_control"),
		ProcCCAvail:    in("proc/sys/net/ipv4/tcp_available_congestion_control"),
		ProcQdisc:      in("proc/sys/net/core/default_qdisc"),
		Modules:        in("lib/modules"),
		Hosts:          in("etc/hosts"),
		CloudCfgDir:    in("etc/cloud/cloud.cfg.d"),
		AptArchives:    in("var/cache/apt/archives"),
		VarLog:         in("var/log"),
		KeepLogs:       []string{in("var/log/nginx"), in("var/log/journal")},
		DockerBin:      in("usr/bin/docker"),
	}
	m.now = func() time.Time { return e.now }
	m.hostname = func() (string, error) { return "old-name", nil }
	m.diskFree = func(string) (uint64, string, error) { return e.free, e.fsType, nil }
	m.trustedDir = func(fs.FileInfo) bool { return true }
	e.m = m
	return e
}

// write creates a file (and its folders) in the test folder.
func (e *testEnv) write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func code(err error) string {
	var ue *usererr.Error
	if errors.As(err, &ue) {
		return ue.Code
	}
	return ""
}
