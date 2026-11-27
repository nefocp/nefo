//go:build linux

package system

import (
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/nefocp/nefo/internal/meminfo"
	"github.com/nefocp/nefo/internal/version"
)

// Info returns information about the server.
func (c *Collector) Info() Info {
	host, _ := os.Hostname()
	info := Info{
		Hostname:  host,
		OS:        "Linux",
		Arch:      runtime.GOARCH,
		CPUs:      runtime.NumCPU(),
		Version:   version.Version,
		Supported: true,
	}
	if f, err := os.Open("/etc/os-release"); err == nil {
		if name := parseOSRelease(f); name != "" {
			info.OS = name
		}
		f.Close()
	}
	if b, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil {
		info.Kernel = strings.TrimSpace(string(b))
	}
	if b, err := os.ReadFile("/proc/uptime"); err == nil {
		info.Uptime, _ = parseUptime(string(b))
	}
	return info
}

// Stats returns current resource usage.
func (c *Collector) Stats() (Stats, error) {
	st := Stats{Supported: true}

	pct, err := c.cpu()
	if err != nil {
		return st, err
	}
	st.CPUPercent = pct

	// In a container, the container's limit when /proc/meminfo shows the
	// host's memory (see meminfo).
	mem, err := meminfo.Read()
	if err != nil {
		return st, err
	}
	st.MemTotal, st.MemUsed, st.MemLimited = mem.Total, mem.Used(), mem.Limited

	var fs syscall.Statfs_t
	if err := syscall.Statfs("/", &fs); err == nil {
		bs := uint64(fs.Bsize)
		st.DiskTotal = fs.Blocks * bs
		st.DiskUsed = (fs.Blocks - fs.Bfree) * bs
	}

	if b, err := os.ReadFile("/proc/loadavg"); err == nil {
		st.Load1, st.Load5, st.Load15, _ = parseLoadavg(string(b))
	}
	return st, nil
}

// cpu returns CPU usage since the previous sample. Samples taken less than a
// second apart reuse the last value, so several open browser tabs do not
// produce noisy numbers.
func (c *Collector) cpu() (float64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if !c.lastAt.IsZero() && now.Sub(c.lastAt) < time.Second {
		return c.lastPct, nil
	}
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, err
	}
	line, _, _ := strings.Cut(string(b), "\n")
	idle, total, err := parseCPULine(line)
	if err != nil {
		return 0, err
	}
	pct := cpuPercent(c.prevIdle, c.prevTotal, idle, total)
	c.prevIdle, c.prevTotal, c.lastPct, c.lastAt = idle, total, pct, now
	return pct, nil
}

// NetCounters returns the byte counters of the internet-facing interfaces
// (those with a default route; every interface but lo if there is none), or
// nil when /proc/net/dev cannot be read.
func NetCounters() map[string]NetCounter {
	dev, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil
	}
	defer dev.Close()
	all := parseNetDev(dev)
	var r4, r6 *os.File
	if f, err := os.Open("/proc/net/route"); err == nil {
		r4 = f
		defer f.Close()
	}
	if f, err := os.Open("/proc/net/ipv6_route"); err == nil {
		r6 = f
		defer f.Close()
	}
	var def map[string]bool
	switch {
	case r4 != nil && r6 != nil:
		def = defaultRouteIfaces(r4, r6)
	case r4 != nil:
		def = defaultRouteIfaces(r4, nil)
	case r6 != nil:
		def = defaultRouteIfaces(nil, r6)
	}
	return pickIfaces(all, def)
}
