package api

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/jobs"
)

type powerFake struct{ calls chan string }

func (f powerFake) Output(_ context.Context, name string, args ...string) (string, error) {
	f.calls <- name + " " + strings.Join(args, " ")
	return "", nil
}
func (powerFake) Stream(context.Context, io.Writer, string, ...string) error { return nil }
func (powerFake) Input(context.Context, []byte, string, ...string) (string, error) {
	return "", nil
}
func (powerFake) Pipe(context.Context, io.Reader, io.Writer, string, ...string) error { return nil }

func TestPowerActions(t *testing.T) {
	f := powerFake{calls: make(chan string, 4)}
	oldRun, oldDelay, oldUnit := powerRun, powerDelay, panelUnit
	powerRun, powerDelay = f, 0
	panelUnit = func() string { return "nefo.service" }
	t.Cleanup(func() { powerRun, powerDelay, panelUnit = oldRun, oldDelay, oldUnit })
	ran := func() string {
		select {
		case c := <-f.calls:
			return c
		case <-time.After(2 * time.Second):
			return ""
		}
	}

	e := newEnv(t)
	if res, _ := e.do(t, http.MethodPost, "/api/system/reboot", map[string]string{"confirm": "x"}, nil); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("reboot without a session: %d", res.StatusCode)
	}
	e.login(t)
	host, _ := os.Hostname()

	// The wrong name is refused and nothing runs.
	res, body := e.do(t, http.MethodPost, "/api/system/reboot", map[string]string{"confirm": host + "-not"}, nil)
	if res.StatusCode != http.StatusBadRequest || body["error"] != "reboot_confirm_mismatch" {
		t.Fatalf("wrong name: %d %v", res.StatusCode, body)
	}

	// Nothing while a job runs.
	release := make(chan struct{})
	if _, err := e.srv.jobs.Start("test", "", time.Minute, func(ctx context.Context, j *jobs.Job) error {
		<-release
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if res, body := e.do(t, http.MethodPost, "/api/system/reboot", map[string]string{"confirm": host}, nil); res.StatusCode != http.StatusConflict {
		t.Fatalf("reboot during a job: %d %v", res.StatusCode, body)
	}
	if res, body := e.do(t, http.MethodPost, "/api/panel/restart", map[string]string{}, nil); res.StatusCode != http.StatusConflict {
		t.Fatalf("restart during a job: %d %v", res.StatusCode, body)
	}
	close(release)
	for i := 0; i < 100 && e.srv.jobRunning(); i++ {
		time.Sleep(10 * time.Millisecond)
	}
	if c := ran(); c != "" {
		t.Fatalf("ran %q while refusing", c)
	}

	// The right name reboots, case does not matter.
	res, body = e.do(t, http.MethodPost, "/api/system/reboot", map[string]string{"confirm": " " + strings.ToUpper(host) + " "}, nil)
	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("reboot: %d %v", res.StatusCode, body)
	}
	if c := ran(); c != "systemctl --no-block reboot" {
		t.Fatalf("reboot ran %q", c)
	}

	res, body = e.do(t, http.MethodPost, "/api/panel/restart", map[string]string{}, nil)
	if res.StatusCode != http.StatusAccepted {
		t.Fatalf("restart: %d %v", res.StatusCode, body)
	}
	if c := ran(); c != "systemctl --no-block restart nefo.service" {
		t.Fatalf("restart ran %q", c)
	}

	entries, err := e.store.RecentAudit(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, en := range entries {
		seen[en.Action] = true
	}
	if !seen["server_reboot"] || !seen["panel_restart"] {
		t.Fatalf("audit log: %+v", entries)
	}

	// Not a service: no restart.
	panelUnit = func() string { return "" }
	if res, body := e.do(t, http.MethodPost, "/api/panel/restart", map[string]string{}, nil); body["error"] != "panel_not_service" {
		t.Fatalf("not a service: %d %v", res.StatusCode, body)
	}
}
