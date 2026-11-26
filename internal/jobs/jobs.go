// Package jobs runs long tasks, such as installing packages or issuing a
// certificate, in the background and keeps their log so the UI can follow
// them. Only one job runs at a time: they all change the same server.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/nefocp/nefo/internal/secure"
)

// ErrBusy is returned by Start while another job is running.
var ErrBusy = errors.New("another task is running")

const (
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"

	maxLines   = 2000
	maxLineLen = 500
	keepJobs   = 20
)

// Job is one background task.
type Job struct {
	mu       sync.Mutex
	id       string
	kind     string
	target   string
	status   string
	lines    []string
	dropped  int
	partial  []byte
	err      string
	started  time.Time
	finished time.Time
}

// Snapshot is a copy of a job's state, ready to be sent as JSON.
type Snapshot struct {
	ID       string     `json:"id"`
	Kind     string     `json:"kind"`
	Target   string     `json:"target"`
	Status   string     `json:"status"`
	Log      []string   `json:"log"`
	Error    string     `json:"error,omitempty"`
	Started  time.Time  `json:"started"`
	Finished *time.Time `json:"finished,omitempty"`
}

// Logf adds a line to the job's log.
func (j *Job) Logf(format string, args ...any) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.flushPartial()
	j.addLine(fmt.Sprintf(format, args...))
}

// Write adds program output to the log, one line at a time. It makes Job an
// io.Writer that can be given to run.Runner.Stream.
func (j *Job) Write(p []byte) (int, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, c := range p {
		switch c {
		case '\n', '\r': // apt redraws progress with \r: treat it as a new line
			if len(j.partial) > 0 {
				j.addLine(string(j.partial))
				j.partial = j.partial[:0]
			}
		default:
			if len(j.partial) < maxLineLen {
				j.partial = append(j.partial, c)
			}
		}
	}
	return len(p), nil
}

func (j *Job) flushPartial() {
	if len(j.partial) > 0 {
		j.addLine(string(j.partial))
		j.partial = j.partial[:0]
	}
}

func (j *Job) addLine(s string) {
	s = strings.TrimRight(s, " \t")
	if s == "" {
		return
	}
	if len(s) > maxLineLen {
		s = s[:maxLineLen]
	}
	j.lines = append(j.lines, s)
	if len(j.lines) > maxLines {
		n := len(j.lines) - maxLines
		j.lines = append(j.lines[:0:0], j.lines[n:]...)
		j.dropped += n
	}
}

// ID returns the job's ID.
func (j *Job) ID() string { return j.id }

// Snapshot returns a copy of the job's current state.
func (j *Job) Snapshot() Snapshot {
	j.mu.Lock()
	defer j.mu.Unlock()
	s := Snapshot{
		ID:      j.id,
		Kind:    j.kind,
		Target:  j.target,
		Status:  j.status,
		Error:   j.err,
		Started: j.started,
	}
	if j.dropped > 0 {
		s.Log = append(s.Log, fmt.Sprintf("… %d earlier lines not shown", j.dropped))
	}
	s.Log = append(s.Log, j.lines...)
	if len(j.partial) > 0 {
		s.Log = append(s.Log, string(j.partial))
	}
	if s.Log == nil {
		s.Log = []string{}
	}
	if !j.finished.IsZero() {
		f := j.finished
		s.Finished = &f
	}
	return s
}

func (j *Job) finish(err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.flushPartial()
	j.finished = time.Now()
	if err != nil {
		j.status = StatusFailed
		j.err = err.Error()
		return
	}
	j.status = StatusSucceeded
}

// Func is the work a job does. It reports progress through j.
type Func func(ctx context.Context, j *Job) error

// Manager starts jobs and remembers the most recent ones.
type Manager struct {
	log     *slog.Logger
	mu      sync.Mutex
	jobs    []*Job // oldest first
	running *Job
	wg      sync.WaitGroup
}

// NewManager returns a Manager.
func NewManager(log *slog.Logger) *Manager {
	return &Manager{log: log}
}

// Start runs fn in the background with the given timeout. It returns ErrBusy
// if another job is still running.
func (m *Manager) Start(kind, target string, timeout time.Duration, fn Func) (*Job, error) {
	m.mu.Lock()
	if m.running != nil {
		m.mu.Unlock()
		return nil, ErrBusy
	}
	j := &Job{
		id:      secure.String(16, secure.LowerAlnum),
		kind:    kind,
		target:  target,
		status:  StatusRunning,
		started: time.Now(),
	}
	m.running = j
	m.jobs = append(m.jobs, j)
	if len(m.jobs) > keepJobs {
		m.jobs = append(m.jobs[:0:0], m.jobs[len(m.jobs)-keepJobs:]...)
	}
	m.wg.Add(1)
	m.mu.Unlock()

	go func() {
		defer m.wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		err := runSafely(ctx, j, fn)
		j.finish(err)
		if err != nil {
			m.log.Warn("job failed", "kind", kind, "target", target, "err", err)
		} else {
			m.log.Info("job finished", "kind", kind, "target", target)
		}
		m.mu.Lock()
		m.running = nil
		m.mu.Unlock()
		// Jobs such as downloads grow the heap; give the memory back to the
		// system at once, which matters on small servers.
		debug.FreeOSMemory()
	}()
	return j, nil
}

func runSafely(ctx context.Context, j *Job, fn Func) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("internal error: %v", r)
		}
	}()
	return fn(ctx, j)
}

// Get returns the job with the given ID.
func (m *Manager) Get(id string) (*Job, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, j := range m.jobs {
		if j.id == id {
			return j, true
		}
	}
	return nil, false
}

// Recent returns the remembered jobs, newest first.
func (m *Manager) Recent() []Snapshot {
	m.mu.Lock()
	jobs := append([]*Job(nil), m.jobs...)
	m.mu.Unlock()
	out := make([]Snapshot, 0, len(jobs))
	for i := len(jobs) - 1; i >= 0; i-- {
		out = append(out, jobs[i].Snapshot())
	}
	return out
}

// Wait blocks until running jobs have finished or ctx is done.
func (m *Manager) Wait(ctx context.Context) {
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
	}
}
