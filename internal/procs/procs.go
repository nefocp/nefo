// Package procs shows what runs on the server: processes (which the
// administrator can stop), listening ports and SSH sign-ins.
package procs

import (
	"context"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/usererr"
)

// MaxProcesses is the most processes List returns.
const MaxProcesses = 300

const maxCommand = 300

// Process is one running process.
type Process struct {
	PID  int    `json:"pid"`
	PPID int    `json:"ppid"`
	User string `json:"user"`
	// CPU is the share of one CPU core in percent, over the last few seconds
	// where the panel can measure it, otherwise since the process started.
	CPU     float64 `json:"cpu"`
	Mem     float64 `json:"mem"` // percent of the server's memory
	RSSMB   float64 `json:"rss_mb"`
	Elapsed int64   `json:"elapsed"` // seconds since it started
	Command string  `json:"command"`
	Kernel  bool    `json:"kernel,omitempty"`
	// Protected processes cannot be stopped from the panel: init, the panel
	// itself and the process that started it.
	Protected bool `json:"protected,omitempty"`
}

// Manager reads processes, ports and sign-ins.
type Manager struct {
	run          run.Runner
	self, parent int
	now          func() time.Time
	sleep        func(time.Duration)
	ticks        func() map[int]cpuSample // nil result: not available

	cpuMu   sync.Mutex
	prev    map[int]cpuSample
	prevAt  time.Time
	lastCPU map[int]float64
}

// New returns a Manager.
func New(r run.Runner) *Manager {
	return &Manager{run: r, self: os.Getpid(), parent: os.Getppid(), now: time.Now, sleep: time.Sleep, ticks: readTicks}
}

// psFormat lists the columns. Only the last one, the command line, can
// contain spaces; "user:32" prints a number for longer names.
var psFormat = []string{"-ww", "--no-headers", "-o", "pid,ppid,user:32,pcpu,pmem,rss,etimes,args"}

// parsePS reads the output of ps with psFormat.
func parsePS(out string) []Process {
	list := []Process{}
	for _, line := range strings.Split(out, "\n") {
		f, rest := fields(line, 7)
		if len(f) < 7 {
			continue
		}
		var p Process
		var err1, err2, err3, err4, err5, err6 error
		p.PID, err1 = strconv.Atoi(f[0])
		p.PPID, err2 = strconv.Atoi(f[1])
		p.User = f[2]
		p.CPU, err3 = strconv.ParseFloat(f[3], 64)
		p.Mem, err4 = strconv.ParseFloat(f[4], 64)
		rss, err5 := strconv.ParseInt(f[5], 10, 64)
		p.Elapsed, err6 = strconv.ParseInt(f[6], 10, 64)
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil || err5 != nil || err6 != nil || p.PID <= 0 {
			continue
		}
		p.RSSMB = math.Round(float64(rss)/1024*10) / 10
		// Kernel threads have no command line (ps shows their name in
		// brackets) and are children of kthreadd (PID 2). A program that
		// merely names itself "[kworker]" is not one, and can be stopped.
		p.Kernel = strings.HasPrefix(rest, "[") && strings.HasSuffix(rest, "]") && (p.PID == 2 || p.PPID == 2)
		p.Command = truncate(rest, maxCommand)
		list = append(list, p)
	}
	return list
}

// fields splits off the first n space-separated fields of a line and
// returns them with the rest of the line.
func fields(line string, n int) ([]string, string) {
	out := make([]string, 0, n)
	s := strings.TrimRight(line, "\r")
	for len(out) < n {
		s = strings.TrimLeft(s, " \t")
		if s == "" {
			break
		}
		i := strings.IndexAny(s, " \t")
		if i < 0 {
			out = append(out, s)
			s = ""
			break
		}
		out = append(out, s[:i])
		s = s[i:]
	}
	return out, strings.TrimSpace(s)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + "…"
}

func (m *Manager) protect(p *Process) {
	p.Protected = p.PID == 1 || p.PID == m.self || p.PID == m.parent
}

// List returns the busiest processes, at most MaxProcesses, sorted by
// "cpu" (the default), "mem" or "pid", and how many processes there are.
func (m *Manager) List(ctx context.Context, sortBy string) ([]Process, int, error) {
	out, err := m.run.Output(ctx, "ps", append([]string{"-e"}, psFormat...)...)
	if err != nil {
		return nil, 0, err
	}
	list := parsePS(out)
	// Leave out the ps we just ran.
	list = slices.DeleteFunc(list, func(p Process) bool {
		return p.PPID == m.self && strings.HasPrefix(p.Command, "ps -e")
	})
	recent := m.recentCPU()
	for i := range list {
		if c, ok := recent[list[i].PID]; ok {
			list[i].CPU = c
		}
		m.protect(&list[i])
	}
	switch sortBy {
	case "mem":
		slices.SortStableFunc(list, func(a, b Process) int { return cmpDesc(a.RSSMB, b.RSSMB, a.CPU, b.CPU) })
	case "pid":
		slices.SortStableFunc(list, func(a, b Process) int { return a.PID - b.PID })
	default:
		slices.SortStableFunc(list, func(a, b Process) int { return cmpDesc(a.CPU, b.CPU, a.Mem, b.Mem) })
	}
	total := len(list)
	if len(list) > MaxProcesses {
		list = list[:MaxProcesses]
	}
	return list, total, nil
}

func cmpDesc(a, b, a2, b2 float64) int {
	switch {
	case a > b:
		return -1
	case a < b:
		return 1
	case a2 > b2:
		return -1
	case a2 < b2:
		return 1
	}
	return 0
}

// Get returns one process, or a "process_not_found" error.
func (m *Manager) Get(ctx context.Context, pid int) (Process, error) {
	if pid <= 0 {
		return Process{}, usererr.New("process_not_found", strconv.Itoa(pid))
	}
	out, _ := m.run.Output(ctx, "ps", append([]string{"-p", strconv.Itoa(pid)}, psFormat...)...)
	for _, p := range parsePS(out) {
		if p.PID == pid {
			m.protect(&p)
			return p, nil
		}
	}
	return Process{}, usererr.New("process_not_found", strconv.Itoa(pid))
}

// Kill sends TERM (ask the program to exit) or KILL (end it at once) to a
// process, and returns the process it was sent to.
func (m *Manager) Kill(ctx context.Context, pid int, signal string) (Process, error) {
	if signal != "TERM" && signal != "KILL" {
		return Process{}, usererr.New("invalid_signal", signal)
	}
	switch {
	case pid == 1:
		return Process{}, usererr.New("kill_init")
	case pid == m.self || pid == m.parent:
		return Process{}, usererr.New("kill_panel", strconv.Itoa(pid))
	}
	p, err := m.Get(ctx, pid)
	if err != nil {
		return p, err
	}
	if p.Kernel {
		return p, usererr.New("kill_kernel_thread", p.Command)
	}
	if out, err := m.run.Output(ctx, "kill", "-s", signal, strconv.Itoa(pid)); err != nil {
		if strings.Contains(out, "No such process") {
			return p, usererr.New("process_not_found", strconv.Itoa(pid))
		}
		return p, err
	}
	return p, nil
}
