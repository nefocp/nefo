package jobs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func newManager() *Manager {
	return NewManager(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func waitDone(t *testing.T, j *Job) Snapshot {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if s := j.Snapshot(); s.Status != StatusRunning {
			return s
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("job did not finish")
	return Snapshot{}
}

func TestJobLogAndStatus(t *testing.T) {
	m := newManager()
	j, err := m.Start("test", "x", time.Minute, func(ctx context.Context, j *Job) error {
		j.Logf("step %d", 1)
		io.WriteString(j, "line a\nline b\rprogress 50%\rprogress 100%\npartial")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	s := waitDone(t, j)
	want := []string{"step 1", "line a", "line b", "progress 50%", "progress 100%", "partial"}
	if strings.Join(s.Log, "|") != strings.Join(want, "|") {
		t.Fatalf("log = %q, want %q", s.Log, want)
	}
	if s.Status != StatusSucceeded || s.Finished == nil || s.Error != "" {
		t.Fatalf("snapshot = %+v", s)
	}
}

func TestOnlyOneJobAtATime(t *testing.T) {
	m := newManager()
	release := make(chan struct{})
	j, err := m.Start("slow", "", time.Minute, func(ctx context.Context, j *Job) error {
		<-release
		return errors.New("boom")
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start("other", "", time.Minute, func(context.Context, *Job) error { return nil }); !errors.Is(err, ErrBusy) {
		t.Fatalf("second Start: err = %v, want ErrBusy", err)
	}
	close(release)
	s := waitDone(t, j)
	if s.Status != StatusFailed || s.Error != "boom" {
		t.Fatalf("snapshot = %+v", s)
	}
	m.Wait(context.Background())
	if _, err := m.Start("other", "", time.Minute, func(context.Context, *Job) error { return nil }); err != nil {
		t.Fatalf("Start after the first job finished: %v", err)
	}
	m.Wait(context.Background())
	if got := m.Recent(); len(got) != 2 || got[0].Kind != "other" {
		t.Fatalf("Recent() = %+v", got)
	}
}

func TestPanicBecomesFailure(t *testing.T) {
	m := newManager()
	j, _ := m.Start("panic", "", time.Minute, func(context.Context, *Job) error { panic("oops") })
	if s := waitDone(t, j); s.Status != StatusFailed || !strings.Contains(s.Error, "oops") {
		t.Fatalf("snapshot = %+v", s)
	}
}

func TestLogIsCapped(t *testing.T) {
	j := &Job{}
	for i := 0; i < maxLines+10; i++ {
		j.Logf("line %d", i)
	}
	j.Logf("%s", strings.Repeat("x", maxLineLen+100))
	s := j.Snapshot()
	if len(s.Log) != maxLines+1 || !strings.Contains(s.Log[0], "earlier lines") {
		t.Fatalf("len = %d, first = %q", len(s.Log), s.Log[0])
	}
	if l := s.Log[len(s.Log)-1]; len(l) != maxLineLen {
		t.Fatalf("long line kept %d bytes", len(l))
	}
}

func TestLogFromFollowsALongJob(t *testing.T) {
	m := newManager()
	release := make(chan struct{})
	j, err := m.Start("long", "", time.Minute, func(ctx context.Context, j *Job) error {
		for i := 0; i < maxLines+5; i++ {
			j.Logf("line %d", i)
		}
		io.WriteString(j, "half a line")
		<-release
		return errors.New("boom")
	})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, next := j.LogFrom(0); next == maxLines+5 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the log did not grow")
		}
		time.Sleep(5 * time.Millisecond)
	}
	// The first 5 lines were dropped: asking from 0 starts at the oldest kept.
	lines, next := j.LogFrom(0)
	if len(lines) != maxLines || lines[0] != "line 5" || next != maxLines+5 {
		t.Fatalf("from 0: %d lines, first %q, next %d", len(lines), lines[0], next)
	}
	if lines, next := j.LogFrom(maxLines + 3); len(lines) != 2 || lines[1] != fmt.Sprintf("line %d", maxLines+4) || next != maxLines+5 {
		t.Fatalf("from the end: %q %d", lines, next)
	}
	if lines, _ := j.LogFrom(next + 10); len(lines) != 0 {
		t.Fatalf("past the end: %q", lines)
	}
	if st, _ := j.State(); st != StatusRunning {
		t.Fatalf("state %s", st)
	}
	close(release)
	waitDone(t, j)
	// Finishing completes the half line.
	if lines, _ := j.LogFrom(next); len(lines) != 1 || lines[0] != "half a line" {
		t.Fatalf("after the end: %q", lines)
	}
	if st, e := j.State(); st != StatusFailed || e != "boom" {
		t.Fatalf("state %s %q", st, e)
	}
}
