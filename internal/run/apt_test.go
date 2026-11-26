package run

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// aptFake fails its first n runs with an apt lock error.
type aptFake struct {
	mu       sync.Mutex
	locked   int
	runs     int
	inFlight int
	overlap  bool
	err      error // returned when not locked
}

func (f *aptFake) Output(context.Context, string, ...string) (string, error) { return "", nil }
func (f *aptFake) Input(context.Context, []byte, string, ...string) (string, error) {
	return "", nil
}
func (f *aptFake) Pipe(context.Context, io.Reader, io.Writer, string, ...string) error { return nil }
func (f *aptFake) Stream(ctx context.Context, w io.Writer, name string, args ...string) error {
	f.mu.Lock()
	f.runs++
	f.inFlight++
	if f.inFlight > 1 {
		f.overlap = true
	}
	locked := f.runs <= f.locked
	f.mu.Unlock()
	time.Sleep(5 * time.Millisecond)
	f.mu.Lock()
	f.inFlight--
	f.mu.Unlock()
	if locked {
		return &Error{Err: errors.New("exit status 100"), Output: "Reading package lists...\nE: Could not get lock /var/lib/apt/lists/lock. It is held by process 152 (apt-get)\nE: Unable to lock directory /var/lib/apt/lists/"}
	}
	return f.err
}

func fastLockWait(t *testing.T, tries int) {
	oldTries, oldWait := aptLockTries, aptLockWait
	aptLockTries, aptLockWait = tries, time.Millisecond
	t.Cleanup(func() { aptLockTries, aptLockWait = oldTries, oldWait })
}

func TestAptWaitsForAnotherProgramsLock(t *testing.T) {
	fastLockWait(t, 10)
	f := &aptFake{locked: 3}
	var log strings.Builder
	if err := Apt(context.Background(), f, &log, "update"); err != nil {
		t.Fatal(err)
	}
	if f.runs != 4 || strings.Count(log.String(), "waiting for it to finish") != 3 {
		t.Fatalf("runs %d, log:\n%s", f.runs, log.String())
	}
}

func TestAptGivesUpOnALockThatStays(t *testing.T) {
	fastLockWait(t, 3)
	f := &aptFake{locked: 100}
	err := Apt(context.Background(), f, io.Discard, "update")
	if err == nil || !aptLocked(err) || f.runs != 3 {
		t.Fatalf("err %v after %d runs", err, f.runs)
	}
}

func TestAptOtherErrorsAreNotRetried(t *testing.T) {
	fastLockWait(t, 10)
	f := &aptFake{err: &Error{Err: errors.New("exit status 100"), Output: "E: Unable to locate package nope"}}
	if err := Apt(context.Background(), f, io.Discard, "install", "nope"); err == nil || f.runs != 1 {
		t.Fatalf("err %v after %d runs", err, f.runs)
	}
}

// The panel's own runs never overlap (the hourly update check and an
// installation started at the same moment).
func TestAptRunsOneAtATime(t *testing.T) {
	f := &aptFake{}
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Apt(context.Background(), f, io.Discard, "update")
		}()
	}
	wg.Wait()
	if f.overlap || f.runs != 4 {
		t.Fatalf("overlap %v, runs %d", f.overlap, f.runs)
	}
}
