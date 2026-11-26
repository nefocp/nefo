package logstats

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Cache keeps reports for a short time, so that opening the statistics again
// (or switching back to a period) does not read the logs again. Requests for
// a report that is being computed wait for it instead of computing it twice.
type Cache struct {
	TTL time.Duration
	// Workers is the number of reports computed at the same time (default 2).
	Workers int

	mu      sync.Mutex
	entries map[string]*cacheEntry
	sem     chan struct{}
	now     func() time.Time
}

type cacheEntry struct {
	done    chan struct{}
	report  *Report
	err     error
	expires time.Time
}

// NewCache returns a Cache that keeps reports for ttl.
func NewCache(ttl time.Duration) *Cache {
	return &Cache{TTL: ttl, entries: map[string]*cacheEntry{}, now: time.Now}
}

// Get returns the report cached under key, or computes and caches it. Errors
// are not cached.
func (c *Cache) Get(ctx context.Context, key string, compute func() (*Report, error)) (rep *Report, err error) {
	c.mu.Lock()
	if c.sem == nil {
		n := c.Workers
		if n < 1 {
			n = 2
		}
		c.sem = make(chan struct{}, n)
	}
	now := c.now()
	for k, e := range c.entries {
		if !e.expires.IsZero() && now.After(e.expires) {
			delete(c.entries, k)
		}
	}
	e, ok := c.entries[key]
	if !ok {
		e = &cacheEntry{done: make(chan struct{})}
		c.entries[key] = e
	}
	c.mu.Unlock()

	if ok {
		select {
		case <-e.done:
			return e.report, e.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// The computation goes on if this request is cancelled: others may be
	// waiting for it, and its result is cached.
	c.sem <- struct{}{}
	defer func() {
		<-c.sem
		c.mu.Lock()
		e.report, e.err = rep, err
		if err != nil {
			delete(c.entries, key)
		} else {
			e.expires = c.now().Add(c.TTL)
		}
		c.mu.Unlock()
		close(e.done)
	}()
	err = errPanicked // replaced unless compute panics
	return compute()
}

var errPanicked = errors.New("statistics computation failed")
