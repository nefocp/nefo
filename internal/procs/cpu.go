package procs

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// ps reports a process's CPU use averaged over its whole life, which hides
// a long-running program that is busy right now. Where /proc is available,
// the panel compares the CPU time of every process between two moments:
// the previous page refresh, or a short sample on the first visit.

// clockTicks is USER_HZ, the unit of the CPU times in /proc/<pid>/stat. It
// is 100 on every Linux architecture.
const clockTicks = 100

// cpuSample is a process's CPU time so far, and when it started (to notice
// a process ID that was reused).
type cpuSample struct {
	ticks uint64
	start uint64
}

// parseStat reads utime, stime and starttime from /proc/<pid>/stat. The
// second field, the name, is in parentheses and may contain spaces and
// parentheses itself, so the fields are counted from the last ")".
func parseStat(data string) (cpuSample, bool) {
	i := strings.LastIndexByte(data, ')')
	if i < 0 {
		return cpuSample{}, false
	}
	f := strings.Fields(data[i+1:])
	// f[0] is field 3 (state): utime is field 14, stime 15, starttime 22.
	if len(f) < 20 {
		return cpuSample{}, false
	}
	utime, err1 := strconv.ParseUint(f[11], 10, 64)
	stime, err2 := strconv.ParseUint(f[12], 10, 64)
	start, err3 := strconv.ParseUint(f[19], 10, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return cpuSample{}, false
	}
	return cpuSample{ticks: utime + stime, start: start}, true
}

// recentCPU returns each process's CPU use in percent of one core since the
// previous call, or nil where it cannot be measured.
func (m *Manager) recentCPU() map[int]float64 {
	if m.ticks == nil {
		return nil
	}
	m.cpuMu.Lock()
	defer m.cpuMu.Unlock()
	now := m.now()
	if m.lastCPU != nil && now.Sub(m.prevAt) < time.Second {
		return m.lastCPU // asked again a moment later: same answer
	}
	cur := m.ticks()
	if cur == nil {
		return nil
	}
	if m.prev == nil || now.Sub(m.prevAt) > time.Minute {
		// Nothing recent to compare with: take a short sample now.
		m.prev, m.prevAt = cur, now
		m.sleep(500 * time.Millisecond)
		now = m.now()
		if cur = m.ticks(); cur == nil {
			return nil
		}
	}
	secs := now.Sub(m.prevAt).Seconds()
	if secs <= 0 {
		return nil
	}
	out := make(map[int]float64, len(cur))
	for pid, c := range cur {
		used := c.ticks
		if p, ok := m.prev[pid]; ok && p.start == c.start && p.ticks <= c.ticks {
			used = c.ticks - p.ticks
		}
		// A process that is new since the previous sample used all its
		// CPU time in between.
		out[pid] = math.Round(float64(used)/clockTicks/secs*1000) / 10
	}
	m.prev, m.prevAt, m.lastCPU = cur, now, out
	return out
}
