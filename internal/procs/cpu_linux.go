//go:build linux

package procs

import (
	"os"
	"strconv"
)

// readTicks reads the CPU time of every process from /proc.
func readTicks() map[int]cpuSample {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	out := make(map[int]cpuSample, len(entries))
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 0 {
			continue
		}
		b, err := os.ReadFile("/proc/" + e.Name() + "/stat")
		if err != nil {
			continue // it has exited
		}
		if s, ok := parseStat(string(b)); ok {
			out[pid] = s
		}
	}
	return out
}
