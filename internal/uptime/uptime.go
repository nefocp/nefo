package uptime

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nefocp/nefo/internal/store"
)

const (
	downAfter   = 2                   // failed checks in a row before a website is down
	repeatEvery = 6 * time.Hour       // the alert again while a website stays down
	certWarn    = 14 * 24 * time.Hour // a served certificate ending sooner is reported…
	certRepeat  = 24 * time.Hour      // …at most once a day
	keep        = 7 * 24 * time.Hour  // check history
	parallel    = 4                   // checks at the same time, at most
)

// Status is where a monitored website stands. State is "pending" (not
// checked yet), "up" or "down"; Since is when the failures began (set from
// the first failed check, also before the website counts as down); Fails is
// how many checks in a row failed.
type Status struct {
	SiteID int64              `json:"site_id"`
	State  string             `json:"state"`
	Since  *time.Time         `json:"since,omitempty"`
	Fails  int                `json:"fails"`
	Last   *store.UptimeCheck `json:"last,omitempty"`
	// CertExpires is the expiry of the certificate the website served.
	CertExpires *time.Time `json:"cert_expires,omitempty"`
}

// Service checks the monitored websites in one loop and keeps their state.
type Service struct {
	store  *store.Store
	check  *Checker
	notify func(ctx context.Context, text string)
	log    *slog.Logger
	now    func() time.Time
	tick   time.Duration // how often the loop looks for due checks

	reloadMu sync.Mutex
	mu       sync.Mutex
	watches  map[int64]*watch
}

// watch is a monitored website: its settings, when it is checked next, and
// its state.
type watch struct {
	site    *store.Site
	mon     store.SiteMonitor
	next    time.Time
	running bool

	fails   int
	down    bool
	since   time.Time // first failed check of the current failures
	alerted time.Time // last "down" alert
	last    *store.UptimeCheck
}

// New returns a Service. notify sends an alert through every configured
// channel and returns when it is sent.
func New(st *store.Store, notify func(ctx context.Context, text string), log *slog.Logger) *Service {
	return &Service{store: st, check: NewChecker(), notify: notify, log: log, now: time.Now, tick: 5 * time.Second,
		watches: map[int64]*watch{}}
}

// Run checks the monitored websites, each at its interval, until ctx is
// done. The first checks are spread over the interval, so they don't all
// run at once, and a few run at the same time at most.
func (s *Service) Run(ctx context.Context) {
	sem := make(chan struct{}, parallel)
	var wg sync.WaitGroup
	defer wg.Wait()
	s.Reload(ctx)
	t := time.NewTicker(s.tick)
	defer t.Stop()
	reloaded, purged := s.now(), time.Time{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		now := s.now()
		// Websites added, changed or deleted elsewhere: within a minute.
		if now.Sub(reloaded) >= time.Minute {
			s.Reload(ctx)
			reloaded = now
		}
		if now.Sub(purged) >= time.Hour {
			if err := s.store.PurgeUptimeBefore(ctx, now.Add(-keep)); err != nil && ctx.Err() == nil {
				s.log.Warn("purge uptime checks", "err", err)
			}
			purged = now
		}
		for _, w := range s.due(now) {
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				s.mu.Lock()
				w.running = false
				s.mu.Unlock()
				return
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				s.run(ctx, w)
			}()
		}
	}
}

// due marks the watches whose check is due as running and returns them.
func (s *Service) due(now time.Time) []*watch {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*watch
	for _, w := range s.watches {
		if !w.running && !now.Before(w.next) {
			w.running = true
			out = append(out, w)
		}
	}
	return out
}

// spread is when a website is first checked after now: at a point of its
// interval that depends on its ID, so the websites take turns.
func spread(now time.Time, siteID int64, interval int) time.Time {
	period := int64(interval) * 60
	return now.Add(time.Duration((siteID*7919)%period) * time.Second)
}

// Reload reads the monitored websites again. A website is watched while
// its monitoring is on and it is not suspended.
func (s *Service) Reload(ctx context.Context) {
	// One at a time: a reload that read the settings before a change must
	// not undo one that read them after.
	s.reloadMu.Lock()
	defer s.reloadMu.Unlock()
	sites, err := s.store.ListSites(ctx)
	if err != nil {
		return
	}
	mons, err := s.store.SiteMonitors(ctx)
	if err != nil {
		return
	}
	byID := map[int64]*store.Site{}
	for _, site := range sites {
		byID[site.ID] = site
	}
	now := s.now()
	keepIDs := map[int64]bool{}
	for _, m := range mons {
		site := byID[m.SiteID]
		if !m.Enabled || site == nil || site.Disabled {
			continue
		}
		keepIDs[m.SiteID] = true
		s.mu.Lock()
		w := s.watches[m.SiteID]
		s.mu.Unlock()
		if w == nil {
			w = &watch{mon: *m, next: spread(now, m.SiteID, m.Interval)}
			s.restore(ctx, w)
		}
		s.mu.Lock()
		if old := s.watches[m.SiteID]; old != nil {
			w = old
		}
		if w.mon.Interval != m.Interval {
			w.next = spread(now, m.SiteID, m.Interval)
		}
		// The certificate fields live here once loaded: the checks change them.
		w.site = site
		w.mon.Enabled, w.mon.Interval, w.mon.Path, w.mon.Keyword, w.mon.Strict4xx = m.Enabled, m.Interval, m.Path, m.Keyword, m.Strict4xx
		s.watches[m.SiteID] = w
		s.mu.Unlock()
	}
	s.mu.Lock()
	for id := range s.watches {
		if !keepIDs[id] {
			delete(s.watches, id)
		}
	}
	s.mu.Unlock()
}

// restore picks up the state of a website from its history, so a restart
// of the panel neither forgets that it is down nor repeats the alert.
func (s *Service) restore(ctx context.Context, w *watch) {
	last, err := s.store.UptimeChecks(ctx, w.mon.SiteID, s.now().Add(-keep), 1, false)
	if err != nil || len(last) == 0 || s.now().Sub(last[0].Time) > time.Hour {
		return // never checked, or the monitoring was off for a while: start afresh
	}
	w.last = &last[0]
	fails, since, err := s.store.UptimeStreak(ctx, w.mon.SiteID)
	if err != nil {
		return
	}
	w.fails, w.since = fails, since
	if fails >= downAfter {
		w.down, w.alerted = true, s.now()
	}
}

// CheckNow checks a monitored website at once, after its settings were
// saved, and returns when the check is done. It does nothing for a website
// that is not monitored, or whose check is already running.
func (s *Service) CheckNow(ctx context.Context, siteID int64) {
	s.Reload(ctx)
	s.mu.Lock()
	w := s.watches[siteID]
	if w == nil || w.running {
		s.mu.Unlock()
		return
	}
	w.running = true
	s.mu.Unlock()
	s.run(ctx, w)
}

// run checks a website once, records the result and sends the alerts it
// calls for.
func (s *Service) run(ctx context.Context, w *watch) {
	s.mu.Lock()
	site, mon := w.site, w.mon
	s.mu.Unlock()
	start := s.now()
	r := s.check.Check(ctx, targetFor(site, mon))
	r.SiteID, r.Time = site.ID, start
	if ctx.Err() != nil {
		// Stopped half-way (the panel is shutting down): not the website's fault.
		s.mu.Lock()
		w.running = false
		s.mu.Unlock()
		return
	}
	if err := s.store.AddUptimeCheck(ctx, r.UptimeCheck); err != nil {
		s.log.Warn("store uptime check", "site", site.Domain, "err", err)
	}
	now := s.now()
	s.mu.Lock()
	msgs := w.record(site.Domain, r.UptimeCheck, now)
	certMsg, certSave := w.cert(site.Domain, r.CertNotAfter, now)
	notAfter, warned := w.mon.CertNotAfter, w.mon.CertWarnedAt
	w.running = false
	w.next = now.Add(time.Duration(mon.Interval) * time.Minute)
	s.mu.Unlock()
	if certSave {
		if err := s.store.SetMonitorCert(ctx, site.ID, notAfter, warned); err != nil {
			s.log.Warn("store certificate expiry", "site", site.Domain, "err", err)
		}
	}
	if certMsg != "" {
		msgs = append(msgs, certMsg)
	}
	if len(msgs) > 0 && !s.stillWatched(ctx, w) {
		return // deleted, suspended or turned off during the check
	}
	for _, m := range msgs {
		s.log.Info("uptime alert", "site", site.Domain, "text", m)
		s.notify(ctx, m)
	}
}

// stillWatched reports whether the website is still monitored, and still
// runs, now that its check is done.
func (s *Service) stillWatched(ctx context.Context, w *watch) bool {
	s.mu.Lock()
	id := w.mon.SiteID
	current := s.watches[id] == w
	s.mu.Unlock()
	if !current {
		return false
	}
	site, err := s.store.SiteByID(ctx, id)
	if err != nil || site.Disabled {
		return false
	}
	m, err := s.store.SiteMonitor(ctx, id)
	return err == nil && m.Enabled
}

// record adds a check to the state and returns the alerts it calls for: down
// after downAfter failures in a row (again every repeatEvery while it
// lasts), up again after one success.
func (w *watch) record(domain string, c store.UptimeCheck, now time.Time) []string {
	w.last = &c
	if c.OK {
		var msgs []string
		if w.down {
			msgs = append(msgs, fmt.Sprintf("✅ %s is up again after %s.", domain, human(now.Sub(w.since))))
		}
		w.fails, w.down, w.since = 0, false, time.Time{}
		return msgs
	}
	w.fails++
	if w.fails == 1 {
		w.since = c.Time
	}
	switch {
	case !w.down && w.fails >= downAfter:
		w.down, w.alerted = true, now
		return []string{fmt.Sprintf("🔴 %s is down (%s, %d checks in a row).", domain, c.Error, w.fails)}
	case w.down && now.Sub(w.alerted) >= repeatEvery:
		w.alerted = now
		return []string{fmt.Sprintf("🔴 %s is still down (%s), for %s now.", domain, c.Error, human(now.Sub(w.since)))}
	}
	return nil
}

// cert notes the expiry of the certificate the website served. It returns
// the alert to send when it ends within certWarn (once a day at most), and
// whether the certificate fields changed.
func (w *watch) cert(domain string, notAfter, now time.Time) (string, bool) {
	if notAfter.IsZero() {
		return "", false
	}
	changed := !notAfter.Equal(w.mon.CertNotAfter)
	w.mon.CertNotAfter = notAfter
	left := notAfter.Sub(now)
	if left >= certWarn || now.Sub(w.mon.CertWarnedAt) < certRepeat {
		return "", changed
	}
	w.mon.CertWarnedAt = now
	days := int(left / (24 * time.Hour))
	when := "in " + plural(days, "day")
	if days == 0 {
		when = "today"
	}
	return fmt.Sprintf("⚠️ The certificate that %s serves ends %s (%s). Renew it, or check why the renewal fails.",
		domain, when, notAfter.UTC().Format("2 Jan 2006 15:04 UTC")), true
}

// Status returns the status of a monitored website, and false when it is
// not monitored (off, suspended, or the loop does not run).
func (s *Service) Status(siteID int64) (Status, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w := s.watches[siteID]
	if w == nil {
		return Status{}, false
	}
	return w.status(), true
}

// Summary returns the status of every monitored website.
func (s *Service) Summary() []Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Status, 0, len(s.watches))
	for _, w := range s.watches {
		out = append(out, w.status())
	}
	return out
}

func (w *watch) status() Status {
	st := Status{SiteID: w.mon.SiteID, State: "up", Fails: w.fails}
	switch {
	case w.down:
		st.State = "down"
	case w.last == nil:
		st.State = "pending"
	}
	if !w.since.IsZero() {
		t := w.since
		st.Since = &t
	}
	if w.last != nil {
		c := *w.last
		st.Last = &c
	}
	if !w.mon.CertNotAfter.IsZero() {
		t := w.mon.CertNotAfter
		st.CertExpires = &t
	}
	return st
}

// human writes a duration for an alert: "12 minutes", "3 hours 5 minutes",
// "2 days 4 hours".
func human(d time.Duration) string {
	m := int(d.Round(time.Minute) / time.Minute)
	switch {
	case m < 1:
		return "less than a minute"
	case m < 60:
		return plural(m, "minute")
	case m < 24*60:
		s := plural(m/60, "hour")
		if m%60 > 0 {
			s += " " + plural(m%60, "minute")
		}
		return s
	}
	s := plural(m/(24*60), "day")
	if h := m / 60 % 24; h > 0 {
		s += " " + plural(h, "hour")
	}
	return s
}

func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return fmt.Sprintf("%d %ss", n, unit)
}
