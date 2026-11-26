//go:build linux

package procs

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/run"
)

// TestLive reads this server's real processes, ports and SSH log. It only
// reads, but needs root to see every process and the journal.
func TestLive(t *testing.T) {
	if os.Getenv("NEFO_LIVE_TEST") == "" {
		t.Skip("set NEFO_LIVE_TEST=1 to read this server's processes, ports and SSH log")
	}
	ctx := context.Background()
	m := New(run.System{})
	start := time.Now()
	list, total, err := m.List(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d processes (%d listed) in %v", total, len(list), time.Since(start))
	if total < 10 || len(list) == 0 {
		t.Fatal("too few processes")
	}
	kernel, protected := 0, 0
	for i, p := range list {
		if i < 8 {
			t.Logf("  %6d %-12s cpu %5.1f mem %4.1f %7.1f MB %6ds %s", p.PID, p.User, p.CPU, p.Mem, p.RSSMB, p.Elapsed, p.Command)
		}
		if p.Kernel {
			kernel++
		}
		if p.Protected {
			protected++
		}
	}
	if kernel == 0 || protected == 0 {
		t.Fatalf("%d kernel threads, %d protected", kernel, protected)
	}
	if _, err := m.Get(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Kill(ctx, 2, "TERM"); code(err) != "kill_kernel_thread" {
		t.Fatalf("kthreadd: %v", err)
	}

	ports, err := m.Ports(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range ports {
		t.Logf("  %s %s:%d %s (%d) local=%v", p.Proto, p.Address, p.Port, p.Process, p.PID, p.Local)
	}
	if len(ports) == 0 {
		t.Fatal("no ports")
	}

	start = time.Now()
	logins, err := m.SSHLogins(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d sign-ins in %v", len(logins), time.Since(start))
	for i, l := range logins {
		if i < 8 {
			t.Logf("  %s %-12s %-14s %-16s %s", l.Time.Format(time.RFC3339), l.Result, l.User, l.IP, l.Method)
		}
	}
}
