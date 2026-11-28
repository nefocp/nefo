package seccheck

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nefocp/nefo/internal/usererr"
)

// Settings of the security check.
type Settings struct {
	// Daily runs the check once a day in the panel's hourly maintenance.
	Daily bool `json:"daily"`
	// Notify sends an alert (Settings → Alerts) when a daily run finds a
	// critical problem or a warning that the previous run did not have.
	Notify bool `json:"notify"`
}

// Store keeps the settings, the last report and the accepted findings
// (the panel's settings table).
type Store interface {
	Setting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
}

const (
	keyReport   = "security_check"
	keyAccepted = "security_check_accepted" // finding ID → its Hash when accepted
	keySettings = "security_check_settings"
)

// DailyEvery is how old the last report may get before a daily run.
const DailyEvery = 23 * time.Hour

// Service runs the check, one run at a time, and keeps its results.
type Service struct {
	store   Store
	mu      sync.Mutex
	running atomic.Bool
}

// NewService returns a Service that keeps its data in st.
func NewService(st Store) *Service { return &Service{store: st} }

// Settings returns the settings: by default the check runs every day and
// sends no alert.
func (s *Service) Settings(ctx context.Context) (Settings, error) {
	st := Settings{Daily: true}
	v, err := s.store.Setting(ctx, keySettings)
	if err != nil || v == "" {
		return st, err
	}
	err = json.Unmarshal([]byte(v), &st)
	return st, err
}

// SaveSettings stores the settings.
func (s *Service) SaveSettings(ctx context.Context, st Settings) error {
	b, _ := json.Marshal(st)
	return s.store.SetSetting(ctx, keySettings, string(b))
}

// Running reports whether a run is under way.
func (s *Service) Running() bool { return s.running.Load() }

// Last returns the last report with the accepted findings marked, or nil
// before the first run.
func (s *Service) Last(ctx context.Context) (*Report, error) {
	rep, err := s.load(ctx)
	if err != nil || rep == nil {
		return nil, err
	}
	acc, err := s.accepted(ctx)
	if err != nil {
		return nil, err
	}
	mark(rep, acc)
	return rep, nil
}

func (s *Service) load(ctx context.Context) (*Report, error) {
	v, err := s.store.Setting(ctx, keyReport)
	if err != nil || v == "" {
		return nil, err
	}
	var rep Report
	if err := json.Unmarshal([]byte(v), &rep); err != nil {
		return nil, nil // an unreadable report is as good as none
	}
	return &rep, nil
}

func (s *Service) accepted(ctx context.Context) (map[string]string, error) {
	acc := map[string]string{}
	v, err := s.store.Setting(ctx, keyAccepted)
	if err != nil || v == "" {
		return acc, err
	}
	json.Unmarshal([]byte(v), &acc)
	return acc, nil
}

func mark(rep *Report, acc map[string]string) {
	for i := range rep.Findings {
		f := &rep.Findings[i]
		f.Accepted = acc[f.ID] != "" && acc[f.ID] == f.Hash
	}
}

// Due reports whether a daily run is due at now.
func (s *Service) Due(ctx context.Context, now time.Time) bool {
	st, err := s.Settings(ctx)
	if err != nil || !st.Daily || s.Running() {
		return false
	}
	rep, err := s.load(ctx)
	return err == nil && (rep == nil || now.Sub(rep.CheckedAt) >= DailyEvery)
}

// ErrRunning is returned while another run is under way.
var ErrRunning = usererr.New("security_check_running")

// Run gathers the facts, runs the check and keeps the report. It also
// returns the findings that are new since the previous report: critical
// ones and warnings, not accepted, whose ID or facts the previous report
// did not have. Accepted findings that are gone are forgotten, so a
// problem that comes back after a fix shows again.
func (s *Service) Run(ctx context.Context, facts func(context.Context) *Facts, env Env) (*Report, []Finding, error) {
	if !s.running.CompareAndSwap(false, true) {
		return nil, nil, ErrRunning
	}
	defer s.running.Store(false)
	start := time.Now()
	rep := Run(ctx, facts(ctx), env)
	rep.Millis = time.Since(start).Milliseconds() // with gathering the facts

	// A run that reached its time limit is kept as it is (partial): the
	// settings table is written without that limit.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, err := s.load(ctx)
	if err != nil {
		return nil, nil, err
	}
	acc, err := s.accepted(ctx)
	if err != nil {
		return nil, nil, err
	}
	kept := map[string]string{}
	for _, f := range rep.Findings {
		if h, ok := acc[f.ID]; ok {
			kept[f.ID] = h
		}
	}
	mark(rep, kept)
	b, err := json.Marshal(rep)
	if err != nil {
		return nil, nil, err
	}
	if err := s.store.SetSetting(ctx, keyReport, string(b)); err != nil {
		return nil, nil, err
	}
	if len(kept) != len(acc) {
		if err := s.saveAccepted(ctx, kept); err != nil {
			return nil, nil, err
		}
	}
	before := map[string]string{}
	if prev != nil {
		for _, f := range prev.Findings {
			before[f.ID] = f.Hash
		}
	}
	var fresh []Finding
	for _, f := range rep.Findings {
		if f.Accepted || f.Severity == Info || before[f.ID] == f.Hash {
			continue
		}
		fresh = append(fresh, f)
	}
	return rep, fresh, nil
}

func (s *Service) saveAccepted(ctx context.Context, acc map[string]string) error {
	if len(acc) == 0 {
		return s.store.SetSetting(ctx, keyAccepted, "")
	}
	b, _ := json.Marshal(acc)
	return s.store.SetSetting(ctx, keyAccepted, string(b))
}

// Accept marks a finding of the last report as accepted (it is counted
// apart and hidden until its facts change), or shows it again. It returns
// the report with the change.
func (s *Service) Accept(ctx context.Context, id string, accept bool) (*Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rep, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	var hash string
	if rep != nil {
		for _, f := range rep.Findings {
			if f.ID == id {
				hash = f.Hash
			}
		}
	}
	if hash == "" {
		return nil, usererr.New("security_finding_gone")
	}
	acc, err := s.accepted(ctx)
	if err != nil {
		return nil, err
	}
	if accept {
		acc[id] = hash
	} else {
		delete(acc, id)
	}
	if err := s.saveAccepted(ctx, acc); err != nil {
		return nil, err
	}
	mark(rep, acc)
	return rep, nil
}
