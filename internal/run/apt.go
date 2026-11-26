package run

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// aptMu makes the panel's own apt-get runs wait for each other: the hourly
// check for system updates runs "apt-get update", which holds apt's lists
// lock, and an installation started at the same moment would otherwise fail.
var aptMu sync.Mutex

// Waiting for another program's apt or dpkg lock: up to aptLockTries tries,
// aptLockWait apart (5 minutes). Variables so tests don't wait.
var (
	aptLockTries = 60
	aptLockWait  = 5 * time.Second
)

// Apt runs apt-get non-interactively with a normal umask. The panel's
// service runs with umask 077, which would make files created by package
// scripts unreadable. The shell script is a constant and the arguments are
// passed separately, so no input ever reaches the shell.
//
// The panel's own runs never overlap, and when another program holds apt's
// locks (unattended-upgrades, apt-daily, an administrator's apt in a
// shell), Apt waits for it rather than failing. DPkg::Lock::Timeout covers
// dpkg's lock, but not the package lists' lock that "apt-get update" takes.
func Apt(ctx context.Context, r Runner, w io.Writer, args ...string) error {
	aptMu.Lock()
	defer aptMu.Unlock()
	base := []string{"-c", `umask 022 && exec apt-get "$@"`, "apt-get",
		"-q", "-y", "-o", "DPkg::Lock::Timeout=300",
		"-o", "Dpkg::Options::=--force-confdef", "-o", "Dpkg::Options::=--force-confold"}
	for try := 1; ; try++ {
		err := r.Stream(ctx, w, "/bin/sh", append(base, args...)...)
		if err == nil || !aptLocked(err) || try >= aptLockTries {
			return err
		}
		fmt.Fprintf(w, "Another program is using the package manager; waiting for it to finish…\n")
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(aptLockWait):
		}
	}
}

// aptLocked reports whether apt-get failed because another program held one
// of its locks.
func aptLocked(err error) bool {
	s := err.Error()
	return strings.Contains(s, "Could not get lock") || strings.Contains(s, "Unable to lock directory") ||
		strings.Contains(s, "Unable to acquire the dpkg frontend lock")
}
