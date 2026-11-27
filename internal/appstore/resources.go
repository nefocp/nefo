package appstore

import (
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/nefocp/nefo/internal/meminfo"
)

// System is what the resource check knows about the server.
type System struct {
	MemTotalMB     int
	MemAvailableMB int
	SwapTotalMB    int
	// FreeDiskMB is the free space where the app's files go: Docker's data
	// folder for Docker apps, /var/www for PHP apps.
	FreeDiskMB int
	Arch       string
	CPUFlags   []string
	// RunningLimitsMB is the sum of the memory limits of the apps already
	// installed.
	RunningLimitsMB int
}

// Problem is one reason not to install, or to think twice.
type Problem struct {
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}

// CheckResult is the outcome of a resource check.
type CheckResult struct {
	Refusals []Problem `json:"refusals"`
	Warnings []Problem `json:"warnings"`
	// MemoryLimitMB is the limit the app's containers will get.
	MemoryLimitMB int `json:"memory_limit_mb"`
}

// OK reports whether nothing refuses the install.
func (r *CheckResult) OK() bool { return len(r.Refusals) == 0 }

func mb(n int) string { return strconv.Itoa(n) + " MB" }

// Check compares an app's needs with the server.
//
// Refusals: less RAM in total than the app's minimum (swap never counts),
// less free disk than its images, data and 1 GB to spare, a missing CPU
// feature or an unsupported architecture. Warnings: less RAM than
// recommended, less memory available now than the app's limit, or all the
// apps' limits together above the RAM plus half the swap.
func Check(app *App, sys System) *CheckResult {
	res := &CheckResult{Refusals: []Problem{}, Warnings: []Problem{}}
	r := app.Resources
	limit := r.MemoryLimitMB
	if r.MemoryAuto && sys.MemTotalMB > 0 {
		limit = AutoMemoryMB(sys.MemTotalMB)
	}
	res.MemoryLimitMB = limit
	if sys.MemTotalMB > 0 && sys.MemTotalMB < r.MinRAMMB {
		res.Refusals = append(res.Refusals, Problem{"app_needs_ram", mb(r.MinRAMMB) + " / " + mb(sys.MemTotalMB)})
	}
	need := r.MinFreeDiskMB
	if need == 0 {
		need = r.ImageMB + r.DiskMB + 1024
	}
	if sys.FreeDiskMB >= 0 && sys.FreeDiskMB < need {
		res.Refusals = append(res.Refusals, Problem{"app_needs_disk", mb(need) + " / " + mb(sys.FreeDiskMB)})
	}
	if len(app.Requires.Arch) > 0 && sys.Arch != "" && !slices.Contains(app.Requires.Arch, sys.Arch) {
		res.Refusals = append(res.Refusals, Problem{"app_needs_arch", strings.Join(app.Requires.Arch, ", ")})
	}
	for _, f := range app.Requires.CPUFlags[sys.Arch] {
		if sys.CPUFlags != nil && !slices.Contains(sys.CPUFlags, f) {
			res.Refusals = append(res.Refusals, Problem{"app_needs_cpu", f})
		}
	}
	if sys.MemTotalMB > 0 && sys.MemTotalMB < r.RecommendedRAMMB && sys.MemTotalMB >= r.MinRAMMB {
		res.Warnings = append(res.Warnings, Problem{"app_ram_below_recommended", mb(r.RecommendedRAMMB) + " / " + mb(sys.MemTotalMB)})
	}
	if limit > 0 && sys.MemAvailableMB > 0 && sys.MemAvailableMB < limit {
		res.Warnings = append(res.Warnings, Problem{"app_low_available_memory", mb(limit) + " / " + mb(sys.MemAvailableMB)})
	}
	if limit > 0 && sys.MemTotalMB > 0 && sys.RunningLimitsMB+limit > sys.MemTotalMB+sys.SwapTotalMB/2 {
		res.Warnings = append(res.Warnings, Problem{"app_memory_overcommit", mb(sys.RunningLimitsMB+limit) + " / " + mb(sys.MemTotalMB)})
	}
	return res
}

// AutoMemoryMB is the limit of an app that may use most of the server's
// memory (Ollama): all of it but 1 GB, in steps of 256 MB.
func AutoMemoryMB(total int) int {
	l := (total - 1024) / 256 * 256
	return max(l, 512)
}

// ReadSystem reads the memory (meminfo.Read) and /proc/cpuinfo, and the free space of
// diskPath. Values it cannot read are left at 0 (memory) or -1 (disk), and
// do not cause refusals.
func ReadSystem(diskPath string) System {
	sys := System{FreeDiskMB: -1, Arch: runtime.GOARCH}
	// In a container, its own memory limit (see meminfo).
	if mem, err := meminfo.Read(); err == nil {
		sys.MemTotalMB, sys.MemAvailableMB, sys.SwapTotalMB = int(mem.Total>>20), int(mem.Available>>20), int(mem.SwapTotal>>20)
	}
	if b, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		sys.CPUFlags = cpuFlags(string(b))
	}
	if diskPath != "" {
		sys.FreeDiskMB = freeDiskMB(diskPath)
	}
	return sys
}

// cpuFlags reads the first "flags" (x86) or "Features" (ARM) line.
func cpuFlags(cpuinfo string) []string {
	for _, l := range strings.Split(cpuinfo, "\n") {
		k, v, ok := strings.Cut(l, ":")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		if k == "flags" || k == "Features" {
			return strings.Fields(v)
		}
	}
	return nil
}
