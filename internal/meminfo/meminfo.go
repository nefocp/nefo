// Package meminfo reads the server's memory as the panel should count it.
//
// On a VPS that is a container (LXC, OpenVZ, Docker), /proc/meminfo may
// show the host's memory rather than the container's: a 1 GB container on
// a 64 GB host would look like a 64 GB server. The container's limit is
// then in the memory cgroup the panel sees as its root (/sys/fs/cgroup):
// memory.max with cgroup v2, memory/memory.limit_in_bytes with v1. Read
// takes the smaller of the two totals, and the used memory that goes with
// it, counted as on a server of its own: the page cache and the slab the
// kernel frees when programs need the room are not in use (see
// cgroup.used). Containers whose /proc/meminfo is already the container's
// own (LXCFS, OpenVZ's) give the same figures either way.
package meminfo

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Memory is in bytes.
type Memory struct {
	Total     uint64
	Available uint64
	SwapTotal uint64
	SwapFree  uint64
	// Limited means Total is a container's memory limit, smaller than
	// what /proc/meminfo shows.
	Limited bool
}

// Used is the memory in use: Total less Available.
func (m Memory) Used() uint64 {
	if m.Available >= m.Total {
		return 0
	}
	return m.Total - m.Available
}

// root is "/" on a server; tests point it at a folder with fake files.
var root = "/"

func path(p string) string { return filepath.Join(root, p) }

// Read returns the server's memory.
func Read() (Memory, error) {
	b, err := os.ReadFile(path("proc/meminfo"))
	if err != nil {
		return Memory{}, err
	}
	m, err := parseMeminfo(b)
	if err != nil {
		return Memory{}, err
	}
	if c, ok := cgroupMemory(); ok && c.limit > 0 && c.limit < m.Total {
		m.Total, m.Limited = c.limit, true
		used := c.used()
		avail := uint64(0)
		if used < c.limit {
			avail = c.limit - used
		}
		// The host can run short before the container reaches its limit.
		m.Available = min(avail, m.Available)
		if c.swapLimit >= 0 && uint64(c.swapLimit) < m.SwapTotal {
			m.SwapTotal = uint64(c.swapLimit)
			m.SwapFree = 0
			if c.swapUsed < m.SwapTotal {
				m.SwapFree = m.SwapTotal - c.swapUsed
			}
		}
	}
	return m, nil
}

// Total returns the server's total memory in bytes, 0 when unknown.
func Total() int64 {
	m, err := Read()
	if err != nil {
		return 0
	}
	return int64(m.Total)
}

// AvailableBytes returns the memory available for new programs, 0 when
// unknown.
func AvailableBytes() int64 {
	m, err := Read()
	if err != nil {
		return 0
	}
	return int64(m.Available)
}

func parseMeminfo(b []byte) (Memory, error) {
	var m Memory
	var haveTotal, haveAvail bool
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		key, rest, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		f := strings.Fields(rest)
		if len(f) == 0 {
			continue
		}
		v, err := strconv.ParseUint(f[0], 10, 64)
		if err != nil {
			continue
		}
		v *= 1024 // kB
		switch key {
		case "MemTotal":
			m.Total, haveTotal = v, true
		case "MemAvailable":
			m.Available, haveAvail = v, true
		case "SwapTotal":
			m.SwapTotal = v
		case "SwapFree":
			m.SwapFree = v
		}
	}
	if !haveTotal || !haveAvail {
		return Memory{}, errors.New("MemTotal or MemAvailable missing from /proc/meminfo")
	}
	return m, nil
}

// cgroup is the memory cgroup the panel sees as its root.
type cgroup struct {
	limit       uint64 // 0: no limit
	usage       uint64 // memory.current or memory.usage_in_bytes
	reclaimable uint64 // page cache and slab the kernel frees when it needs room
	swapLimit   int64  // -1: none or unknown
	swapUsed    uint64
}

// used is the memory the container's programs use: its usage without the
// page cache (active_file and inactive_file) and the reclaimable slab
// (cached folder entries and inodes). A container's usage counts every
// file it has read: after a backup, a walk of the websites or a big
// download, most of it is cache. MemAvailable, which the panel counts on
// a server of its own (and free shows), leaves the same out. docker stats
// subtracts only inactive_file, which still counts the cache read lately:
// on a small container that is hundreds of MB more "used" than its
// programs take. Shared memory (tmpfs) is on the anon lists and stays
// counted, as it must: it cannot be dropped.
func (c cgroup) used() uint64 {
	if c.reclaimable < c.usage {
		return c.usage - c.reclaimable
	}
	return 0
}

// cgroupMemory reads the limit of /sys/fs/cgroup (v2) or
// /sys/fs/cgroup/memory (v1). The host's own root cgroup has no limit
// file (v2) or an unlimited one (v1).
func cgroupMemory() (cgroup, bool) {
	c := cgroup{swapLimit: -1}
	if limit, unlimited, ok := readLimit("sys/fs/cgroup/memory.max"); ok {
		if unlimited || limit == 0 {
			return c, false
		}
		c.limit = limit
		c.usage, _ = readUint("sys/fs/cgroup/memory.current")
		st := statValues("sys/fs/cgroup/memory.stat")
		c.reclaimable = st["active_file"] + st["inactive_file"] + st["slab_reclaimable"]
		if s, unlimited, ok := readLimit("sys/fs/cgroup/memory.swap.max"); ok && !unlimited {
			c.swapLimit = int64(min(s, unlimitedV1))
			c.swapUsed, _ = readUint("sys/fs/cgroup/memory.swap.current")
		}
		return c, true
	}
	if limit, unlimited, ok := readLimit("sys/fs/cgroup/memory/memory.limit_in_bytes"); ok && !unlimited && limit > 0 {
		c.limit = limit
		c.usage, _ = readUint("sys/fs/cgroup/memory/memory.usage_in_bytes")
		// v1's memory.stat has no slab figures; total_ counts the child
		// cgroups too, as usage_in_bytes does.
		st := statValues("sys/fs/cgroup/memory/memory.stat")
		c.reclaimable = st["total_active_file"] + st["total_inactive_file"]
		return c, true
	}
	return c, false
}

// unlimitedV1 is about what cgroup v1 shows without a limit
// (9223372036854771712: the largest page-aligned int64).
const unlimitedV1 = 1 << 62

// readLimit reads a limit file: a number of bytes, or "max". unlimited
// is true for "max" and for v1's huge number; ok is false when the file is
// missing or unreadable.
func readLimit(p string) (v uint64, unlimited, ok bool) {
	b, err := os.ReadFile(path(p))
	if err != nil {
		return 0, false, false
	}
	s := strings.TrimSpace(string(b))
	if s == "max" {
		return 0, true, true
	}
	v, err = strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, false, false
	}
	if v >= unlimitedV1 {
		return 0, true, true
	}
	return v, false, true
}

func readUint(p string) (uint64, bool) {
	b, err := os.ReadFile(path(p))
	if err != nil {
		return 0, false
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
	return v, err == nil
}

// statValues reads memory.stat: "key value" lines, in bytes (nothing when
// the file is missing).
func statValues(p string) map[string]uint64 {
	out := map[string]uint64{}
	b, err := os.ReadFile(path(p))
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(b), "\n") {
		if k, v, ok := strings.Cut(line, " "); ok {
			if n, err := strconv.ParseUint(strings.TrimSpace(v), 10, 64); err == nil {
				out[k] = n
			}
		}
	}
	return out
}
