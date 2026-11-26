package hosting

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// "systemctl reload nginx" only sends a signal: for a short while after it
// returns, the old worker processes still answer with the old configuration.
// A request made at once, such as the domain check before a certificate
// order, would then miss a site that was just created. So after a reload the
// panel waits until new workers run and no old worker accepts requests.

const (
	workerTitle       = "nginx: worker process"
	workerShutdownTag = "is shutting down"
)

// activeNginxWorkers returns the PIDs of Nginx workers that accept requests.
// Workers that are draining after a reload rename themselves "… is shutting
// down" and are left out.
func activeNginxWorkers() map[int]bool {
	pids := map[int]bool{}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return pids
	}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		cmd, err := os.ReadFile(filepath.Join("/proc", e.Name(), "cmdline"))
		if err != nil {
			continue
		}
		if bytes.HasPrefix(cmd, []byte(workerTitle)) && !bytes.Contains(cmd, []byte(workerShutdownTag)) {
			pids[pid] = true
		}
	}
	return pids
}

// waitNginxReload waits until the workers in before have stopped accepting
// requests and at least one new worker runs, for at most timeout.
func waitNginxReload(before map[int]bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		now := activeNginxWorkers()
		fresh, stale := false, false
		for pid := range now {
			if before[pid] {
				stale = true
			} else {
				fresh = true
			}
		}
		if fresh && !stale {
			return true
		}
		time.Sleep(25 * time.Millisecond)
	}
	return false
}

// waitForFile waits until path exists, for at most timeout.
func waitForFile(path string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fileExists(path) {
			return true
		}
		time.Sleep(25 * time.Millisecond)
	}
	return false
}
