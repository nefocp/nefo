package auth

import (
	"sync"
	"time"
)

// Limiter locks a key (normally a client IP) out for a while after too many
// failed attempts within a window.
type Limiter struct {
	mu      sync.Mutex
	max     int
	window  time.Duration
	lockout time.Duration
	entries map[string]*limitEntry
}

type limitEntry struct {
	failures    int
	windowStart time.Time
	lockedUntil time.Time
}

// maxEntries bounds memory use when many different IPs fail at once.
const maxEntries = 50_000

// NewLimiter allows max failures per window, then locks the key for lockout.
func NewLimiter(max int, window, lockout time.Duration) *Limiter {
	return &Limiter{max: max, window: window, lockout: lockout, entries: map[string]*limitEntry{}}
}

// Check reports whether key may make an attempt at now, and if not, how long it must wait.
func (l *Limiter) Check(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.entries[key]
	if e != nil && now.Before(e.lockedUntil) {
		return false, e.lockedUntil.Sub(now)
	}
	return true, 0
}

// Fail records a failed attempt for key.
func (l *Limiter) Fail(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.entries) >= maxEntries {
		l.purge(now)
	}
	e := l.entries[key]
	if e == nil || now.Sub(e.windowStart) > l.window {
		e = &limitEntry{windowStart: now}
		l.entries[key] = e
	}
	e.failures++
	if e.failures >= l.max {
		e.lockedUntil = now.Add(l.lockout)
		e.failures = 0
		e.windowStart = now
	}
}

// Reset forgets failures for key, after a successful attempt.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

func (l *Limiter) purge(now time.Time) {
	for k, e := range l.entries {
		if !now.Before(e.lockedUntil) && now.Sub(e.windowStart) > l.window {
			delete(l.entries, k)
		}
	}
	// Still full: everything is recent. Dropping state is better than
	// growing without bound; locked-out keys are kept.
	if len(l.entries) >= maxEntries {
		for k, e := range l.entries {
			if !now.Before(e.lockedUntil) {
				delete(l.entries, k)
			}
		}
	}
}
