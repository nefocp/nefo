package seccheck

import (
	"context"
	"sync"
	"time"
)

// Source is one part of the facts. Read reads it within its time limit
// and returns what to set in Facts; Keys are the checks that need it.
type Source struct {
	Keys []string
	Read func(ctx context.Context) func(*Facts)
}

// Gather reads the sources at the same time, each within timeout, and
// returns the facts. The programs behind them (ufw, fail2ban-client,
// docker, sshd -T) usually answer in a fraction of a second, but one can
// wait on a busy server: fail2ban-client while fail2ban reads the journal
// at start, docker while dockerd creates containers. A source that has
// not answered in time is left out, and its checks are listed in
// Facts.Partial instead of being run on half the facts.
func Gather(ctx context.Context, timeout time.Duration, sources ...Source) *Facts {
	var (
		mu     sync.Mutex
		f      = &Facts{}
		closed bool
		done   = make([]bool, len(sources))
		wg     sync.WaitGroup
	)
	for i, src := range sources {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			set := src.Read(sctx)
			if sctx.Err() != nil {
				return // what it read after its time is not to be trusted
			}
			mu.Lock()
			defer mu.Unlock()
			if !closed {
				if set != nil {
					set(f)
				}
				done[i] = true
			}
		}()
	}
	all := make(chan struct{})
	go func() { wg.Wait(); close(all) }()
	// A source that ignores its context (a file system that does not
	// answer) gets a moment more, then is left behind.
	select {
	case <-all:
	case <-time.After(timeout + lateGrace):
	}
	mu.Lock()
	defer mu.Unlock()
	closed = true
	for i, src := range sources {
		if !done[i] {
			f.Partial = append(f.Partial, src.Keys...)
		}
	}
	return f
}
