package jobs

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCancelStopsTheRunningJob(t *testing.T) {
	m := newManager()
	j, err := m.Start("slow", "", time.Minute, func(ctx context.Context, j *Job) error {
		<-ctx.Done()
		return ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	if m.Cancel("someone-else") {
		t.Fatal("cancelled a job that is not running")
	}
	if !m.Cancel(j.ID()) {
		t.Fatal("Cancel did not find the running job")
	}
	s := waitDone(t, j)
	if s.Status != StatusFailed || s.Error != context.Canceled.Error() {
		t.Fatalf("snapshot = %+v", s)
	}
	if m.Cancel(j.ID()) {
		t.Fatal("cancelled a finished job")
	}
	// The next job starts as usual.
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, err := m.Start("next", "", time.Minute, func(context.Context, *Job) error { return nil })
		if err == nil {
			break
		}
		if !errors.Is(err, ErrBusy) || time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
