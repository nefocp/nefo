// Package system reads information and live statistics about the server.
package system

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Info describes the server. It changes rarely.
type Info struct {
	Hostname  string `json:"hostname"`
	OS        string `json:"os"`
	Kernel    string `json:"kernel"`
	Arch      string `json:"arch"`
	CPUs      int    `json:"cpus"`
	Uptime    uint64 `json:"uptime_seconds"`
	Version   string `json:"panel_version"`
	Supported bool   `json:"supported"`
}

// Stats is a snapshot of resource usage.
type Stats struct {
	CPUPercent float64 `json:"cpu_percent"`
	MemTotal   uint64  `json:"mem_total"`
	MemUsed    uint64  `json:"mem_used"`
	DiskTotal  uint64  `json:"disk_total"`
	DiskUsed   uint64  `json:"disk_used"`
	Load1      float64 `json:"load1"`
	Load5      float64 `json:"load5"`
	Load15     float64 `json:"load15"`
	// Supported is false on platforms where live stats are not implemented
	// (anything but Linux, for example when developing on Windows or macOS).
	Supported bool `json:"supported"`
}

// Collector computes CPU usage from the difference between two samples, so it
// keeps the previous sample.
type Collector struct {
	mu        sync.Mutex
	prevIdle  uint64
	prevTotal uint64
	lastPct   float64
	lastAt    time.Time
}

// NewCollector returns a Collector.
func NewCollector() *Collector { return &Collector{} }

// parseCPULine parses the aggregate "cpu" line of /proc/stat and returns
// idle and total jiffies.
func parseCPULine(line string) (idle, total uint64, err error) {
	f := strings.Fields(line)
	if len(f) < 5 || f[0] != "cpu" {
		return 0, 0, errors.New("unexpected /proc/stat format")
	}
	vals := make([]uint64, 0, len(f)-1)
	for _, s := range f[1:] {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return 0, 0, err
		}
		vals = append(vals, v)
	}
	// Fields: user nice system idle iowait irq softirq steal guest guest_nice.
	// guest and guest_nice are already included in user and nice.
	n := min(len(vals), 8)
	for _, v := range vals[:n] {
		total += v
	}
	idle = vals[3]
	if len(vals) > 4 {
		idle += vals[4] // iowait counts as idle
	}
	return idle, total, nil
}

func cpuPercent(prevIdle, prevTotal, idle, total uint64) float64 {
	if total <= prevTotal {
		return 0
	}
	dt := total - prevTotal
	var di uint64
	if idle > prevIdle {
		di = idle - prevIdle
	}
	if di > dt {
		di = dt
	}
	return float64(dt-di) / float64(dt) * 100
}

// parseMeminfo returns MemTotal and MemAvailable from /proc/meminfo, in bytes.
func parseMeminfo(r io.Reader) (total, available uint64, err error) {
	var haveTotal, haveAvail bool
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		key, rest, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		v, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			continue
		}
		switch key {
		case "MemTotal":
			total, haveTotal = v*1024, true
		case "MemAvailable":
			available, haveAvail = v*1024, true
		}
	}
	if err := sc.Err(); err != nil {
		return 0, 0, err
	}
	if !haveTotal || !haveAvail {
		return 0, 0, errors.New("MemTotal or MemAvailable missing from /proc/meminfo")
	}
	return total, available, nil
}

func parseLoadavg(s string) (l1, l5, l15 float64, err error) {
	f := strings.Fields(s)
	if len(f) < 3 {
		return 0, 0, 0, errors.New("unexpected /proc/loadavg format")
	}
	if l1, err = strconv.ParseFloat(f[0], 64); err != nil {
		return
	}
	if l5, err = strconv.ParseFloat(f[1], 64); err != nil {
		return
	}
	l15, err = strconv.ParseFloat(f[2], 64)
	return
}

func parseUptime(s string) (uint64, error) {
	f := strings.Fields(s)
	if len(f) == 0 {
		return 0, errors.New("empty /proc/uptime")
	}
	v, err := strconv.ParseFloat(f[0], 64)
	return uint64(v), err
}

// parseOSRelease returns PRETTY_NAME from /etc/os-release.
func parseOSRelease(r io.Reader) string {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "PRETTY_NAME="); ok {
			if u, err := strconv.Unquote(v); err == nil {
				return u
			}
			return strings.Trim(v, `"'`)
		}
	}
	return ""
}
