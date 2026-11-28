package meminfo

import (
	"os"
	"path/filepath"
	"testing"
)

const hostMeminfo = `MemTotal:       65536000 kB
MemFree:         1000000 kB
MemAvailable:   40000000 kB
Buffers:          100000 kB
SwapTotal:       4194300 kB
SwapFree:        4194300 kB
`

// fakeRoot writes files (relative path -> content) under a new root.
func fakeRoot(t *testing.T, files map[string]string) {
	t.Helper()
	dir := t.TempDir()
	for p, c := range files {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	old := root
	root = dir
	t.Cleanup(func() { root = old })
}

const mb = 1 << 20

func TestHostWithoutLimit(t *testing.T) {
	// A server of its own: cgroup v2 has no memory.max in the root cgroup.
	fakeRoot(t, map[string]string{
		"proc/meminfo":                  hostMeminfo,
		"sys/fs/cgroup/memory.stat":     "inactive_file 100\n",
		"sys/fs/cgroup/cgroup.procs":    "1\n",
		"sys/fs/cgroup/memory.pressure": "",
	})
	m, err := Read()
	if err != nil {
		t.Fatal(err)
	}
	if m.Total != 65536000*1024 || m.Available != 40000000*1024 || m.Limited || m.SwapTotal != 4194300*1024 {
		t.Fatalf("%+v", m)
	}
	if m.Used() != (65536000-40000000)*1024 {
		t.Fatalf("used %d", m.Used())
	}
}

func TestCgroupV2Limit(t *testing.T) {
	// A 1 GB container on a 64 GB host: 400 MB in use, of which 100 MB is
	// page cache read lately, 50 MB older page cache and 20 MB cached
	// folder entries and inodes.
	fakeRoot(t, map[string]string{
		"proc/meminfo":                 hostMeminfo,
		"sys/fs/cgroup/memory.max":     "1073741824\n",
		"sys/fs/cgroup/memory.current": "419430400\n",
		"sys/fs/cgroup/memory.stat": "anon 200000000\nfile 160000000\nactive_file 104857600\ninactive_file 52428800\nshmem 2000000\n" +
			"slab_reclaimable 20971520\nslab_unreclaimable 9000000\n",
		"sys/fs/cgroup/memory.swap.max":     "536870912\n",
		"sys/fs/cgroup/memory.swap.current": "104857600\n",
	})
	m, err := Read()
	if err != nil {
		t.Fatal(err)
	}
	if !m.Limited || m.Total != 1024*mb {
		t.Fatalf("total %d limited %v", m.Total>>20, m.Limited)
	}
	if m.Used() != 230*mb || m.Available != 794*mb {
		t.Fatalf("used %d MB, available %d MB", m.Used()>>20, m.Available>>20)
	}
	if m.SwapTotal != 512*mb || m.SwapFree != 412*mb {
		t.Fatalf("swap %d/%d MB", m.SwapFree>>20, m.SwapTotal>>20)
	}
	if Total() != 1024*mb || AvailableBytes() != 794*mb {
		t.Fatalf("Total %d AvailableBytes %d", Total(), AvailableBytes())
	}
}

func TestCgroupV2PageCacheIsNotUsed(t *testing.T) {
	// The screenshot sandbox: a 900 MB container whose programs take about
	// 250 MB, its page cache full after seeding. memory.current less
	// inactive_file (what docker stats shows) was 677 MB.
	fakeRoot(t, map[string]string{
		"proc/meminfo":                 hostMeminfo,
		"sys/fs/cgroup/memory.max":     "943718400\n",
		"sys/fs/cgroup/memory.current": "912261120\n", // 870 MB
		"sys/fs/cgroup/memory.stat": "anon 209715200\nfile 587202560\nkernel 104857600\nshmem 10485760\n" +
			"active_file 383778816\ninactive_file 202375168\nslab_reclaimable 73400320\nslab_unreclaimable 20971520\n",
	})
	m, _ := Read()
	if m.Total != 900*mb || m.Used() != 241*mb || m.Available != 659*mb {
		t.Fatalf("total %d used %d avail %d MB", m.Total>>20, m.Used()>>20, m.Available>>20)
	}
	// All of it cache: nothing in use, not an underflow.
	fakeRoot(t, map[string]string{
		"proc/meminfo":                 hostMeminfo,
		"sys/fs/cgroup/memory.max":     "943718400\n",
		"sys/fs/cgroup/memory.current": "104857600\n",
		"sys/fs/cgroup/memory.stat":    "active_file 104857600\ninactive_file 1048576\n",
	})
	if m, _ := Read(); m.Used() != 0 || m.Available != 900*mb {
		t.Fatalf("used %d avail %d", m.Used(), m.Available)
	}
	// Without memory.stat, the whole usage counts.
	fakeRoot(t, map[string]string{
		"proc/meminfo":                 hostMeminfo,
		"sys/fs/cgroup/memory.max":     "943718400\n",
		"sys/fs/cgroup/memory.current": "104857600\n",
	})
	if m, _ := Read(); m.Used() != 100*mb {
		t.Fatalf("no memory.stat: used %d MB", m.Used()>>20)
	}
}

func TestCgroupV2Unlimited(t *testing.T) {
	// A container without --memory, and one whose limit is above what
	// meminfo shows (LXCFS already gives the container's own figures).
	for _, max := range []string{"max\n", "137438953472\n"} {
		fakeRoot(t, map[string]string{
			"proc/meminfo":                 hostMeminfo,
			"sys/fs/cgroup/memory.max":     max,
			"sys/fs/cgroup/memory.current": "419430400\n",
		})
		m, _ := Read()
		if m.Limited || m.Total != 65536000*1024 || m.Available != 40000000*1024 {
			t.Errorf("memory.max %q: %+v", max, m)
		}
	}
}

func TestCgroupNoSwapAllowed(t *testing.T) {
	fakeRoot(t, map[string]string{
		"proc/meminfo":                  hostMeminfo,
		"sys/fs/cgroup/memory.max":      "536870912",
		"sys/fs/cgroup/memory.current":  "104857600",
		"sys/fs/cgroup/memory.swap.max": "0",
	})
	m, _ := Read()
	if m.Total != 512*mb || m.Available != 412*mb || m.SwapTotal != 0 || m.SwapFree != 0 {
		t.Fatalf("%+v", m)
	}
}

func TestCgroupV1Limit(t *testing.T) {
	fakeRoot(t, map[string]string{
		"proc/meminfo": hostMeminfo,
		"sys/fs/cgroup/memory/memory.limit_in_bytes": "2147483648\n",
		"sys/fs/cgroup/memory/memory.usage_in_bytes": "1073741824\n",
		// The cgroup's own figures, then the total_ ones with its children.
		"sys/fs/cgroup/memory/memory.stat": "cache 800000000\nactive_file 1\ninactive_file 1\ntotal_active_file 268435456\ntotal_inactive_file 536870912\n",
	})
	m, _ := Read()
	if !m.Limited || m.Total != 2048*mb || m.Used() != 256*mb || m.Available != 1792*mb {
		t.Fatalf("total %d used %d avail %d", m.Total>>20, m.Used()>>20, m.Available>>20)
	}
	// Swap: v1 is left to meminfo.
	if m.SwapTotal != 4194300*1024 {
		t.Fatalf("swap %d", m.SwapTotal)
	}
}

func TestCgroupV1Unlimited(t *testing.T) {
	fakeRoot(t, map[string]string{
		"proc/meminfo": hostMeminfo,
		"sys/fs/cgroup/memory/memory.limit_in_bytes": "9223372036854771712\n",
		"sys/fs/cgroup/memory/memory.usage_in_bytes": "1073741824\n",
	})
	m, _ := Read()
	if m.Limited || m.Total != 65536000*1024 {
		t.Fatalf("%+v", m)
	}
}

func TestHostShortOfMemory(t *testing.T) {
	// The container may use 8 GB, but the host has only 1 GB left: the
	// smaller figure counts.
	fakeRoot(t, map[string]string{
		"proc/meminfo":                 "MemTotal: 16000000 kB\nMemAvailable: 1048576 kB\n",
		"sys/fs/cgroup/memory.max":     "8589934592",
		"sys/fs/cgroup/memory.current": "1073741824",
	})
	m, _ := Read()
	if m.Total != 8192*mb || m.Available != 1024*mb {
		t.Fatalf("total %d avail %d", m.Total>>20, m.Available>>20)
	}
}

func TestBadMeminfo(t *testing.T) {
	fakeRoot(t, map[string]string{"proc/meminfo": "MemFree: 12 kB\n"})
	if _, err := Read(); err == nil {
		t.Fatal("no error without MemTotal")
	}
	if Total() != 0 || AvailableBytes() != 0 {
		t.Fatal("unknown memory should be 0")
	}
	fakeRoot(t, map[string]string{})
	if _, err := Read(); err == nil {
		t.Fatal("no error without /proc/meminfo")
	}
}
