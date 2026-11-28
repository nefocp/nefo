package systools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/usererr"
)

// ---- swap ----

// SwapSizes are the swap file sizes offered, in MB.
var SwapSizes = []int{512, 1024, 2048, 4096, 8192}

// swapMargin is the disk space that must stay free next to a new swap file.
const swapMargin = 1 << 30

// SwapDevice is one line of /proc/swaps.
type SwapDevice struct {
	Path     string `json:"path"`
	Type     string `json:"type"` // "file" or "partition"
	SizeKB   int64  `json:"size_kb"`
	UsedKB   int64  `json:"used_kb"`
	Priority int    `json:"priority"`
	Panel    bool   `json:"panel"` // the panel's own swap file
}

// Swap is the swap and memory state.
type Swap struct {
	Devices        []SwapDevice `json:"devices"`
	TotalKB        int64        `json:"total_kb"`
	FreeKB         int64        `json:"free_kb"`
	MemTotalKB     int64        `json:"mem_total_kb"`
	MemAvailableKB int64        `json:"mem_available_kb"`
	Swappiness     int          `json:"swappiness"`       // -1 when unknown
	SwappinessSet  bool         `json:"swappiness_panel"` // the panel's sysctl file sets it
	File           string       `json:"file"`             // the panel's swap file path
	FileExists     bool         `json:"file_exists"`
	FileActive     bool         `json:"file_active"`
	// Foreign is a swap file that the panel did not create; the panel does
	// not add a second one next to it.
	Foreign    string `json:"foreign,omitempty"`
	DiskFreeMB int64  `json:"disk_free_mb"` // -1 when unknown
	Sizes      []int  `json:"sizes"`
}

// Swap reads /proc/swaps and /proc/meminfo.
func (m *Manager) Swap() Swap {
	s := Swap{Devices: []SwapDevice{}, File: m.p.SwapFile, Swappiness: -1, DiskFreeMB: -1, Sizes: SwapSizes}
	s.Devices = parseSwaps(readTrim(m.p.ProcSwaps), m.p.SwapFile)
	for _, d := range s.Devices {
		if d.Panel {
			s.FileActive = true
		} else if d.Type == "file" && s.Foreign == "" {
			s.Foreign = d.Path
		}
	}
	mem := parseMeminfo(readTrim(m.p.Meminfo))
	s.TotalKB, s.FreeKB = mem["SwapTotal"], mem["SwapFree"]
	s.MemTotalKB, s.MemAvailableKB = mem["MemTotal"], mem["MemAvailable"]
	// In a container whose /proc/meminfo is the host's: the container's
	// memory, as the dashboard counts it (see meminfo).
	if m.memory != nil {
		if mm, err := m.memory(); err == nil && mm.Limited {
			s.MemTotalKB, s.MemAvailableKB = int64(mm.Total>>10), int64(mm.Available>>10)
		}
	}
	if v, err := strconv.Atoi(readTrim(m.p.Swappiness)); err == nil {
		s.Swappiness = v
	}
	s.SwappinessSet = exists(m.p.SysctlSwap)
	s.FileExists = exists(m.p.SwapFile)
	if free, _, err := m.diskFree(filepath.Dir(m.p.SwapFile)); err == nil {
		s.DiskFreeMB = int64(free >> 20)
	}
	return s
}

// parseSwaps reads /proc/swaps: a heading, then
// "Filename  Type  Size  Used  Priority" with sizes in KB.
func parseSwaps(text, panelFile string) []SwapDevice {
	out := []SwapDevice{}
	for i, line := range strings.Split(text, "\n") {
		f := strings.Fields(line)
		if i == 0 || len(f) < 5 {
			continue
		}
		d := SwapDevice{Path: unescapeOctal(f[0]), Type: f[1]}
		d.SizeKB, _ = strconv.ParseInt(f[2], 10, 64)
		d.UsedKB, _ = strconv.ParseInt(f[3], 10, 64)
		d.Priority, _ = strconv.Atoi(f[4])
		// A deleted file shows as "/swapfile (deleted)".
		d.Path = strings.TrimSuffix(d.Path, " (deleted)")
		d.Panel = d.Path == panelFile
		out = append(out, d)
	}
	return out
}

// unescapeOctal decodes the \040-style escapes of /proc files.
func unescapeOctal(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			if n, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// parseMeminfo reads /proc/meminfo into KB values.
func parseMeminfo(text string) map[string]int64 {
	out := map[string]int64{}
	for _, line := range strings.Split(text, "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		f := strings.Fields(v)
		if len(f) == 0 {
			continue
		}
		if n, err := strconv.ParseInt(f[0], 10, 64); err == nil {
			out[strings.TrimSpace(k)] = n
		}
	}
	return out
}

// CheckCreateSwap returns the error that creating a swap file of sizeMB
// would get, so the API can answer before starting a job.
func (m *Manager) CheckCreateSwap(sizeMB int) error {
	if !slices.Contains(SwapSizes, sizeMB) {
		return usererr.New("swap_size_invalid")
	}
	s := m.Swap()
	if s.FileActive || s.FileExists {
		return usererr.New("swap_panel_exists", m.p.SwapFile)
	}
	if s.Foreign != "" {
		return usererr.New("swap_exists", s.Foreign)
	}
	free, fsType, err := m.diskFree(filepath.Dir(m.p.SwapFile))
	if err != nil {
		return err
	}
	if fsType == "zfs" || fsType == "tmpfs" {
		return usererr.New("swap_unsupported_fs", fsType)
	}
	if need := uint64(sizeMB)<<20 + swapMargin; free < need {
		return usererr.New("swap_no_space", fmt.Sprintf("%d MB / %d MB", need>>20, free>>20))
	}
	return nil
}

// CreateSwap creates the panel's swap file of sizeMB, turns it on and adds
// it to /etc/fstab. fallocate makes the file at once; where swapon refuses
// such a file (some file systems), it is written with dd instead.
func (m *Manager) CreateSwap(ctx context.Context, sizeMB int, lg Logger) (err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.CheckCreateSwap(sizeMB); err != nil {
		return err
	}
	path := m.p.SwapFile
	_, fsType, _ := m.diskFree(filepath.Dir(path))
	active := false
	defer func() {
		if err == nil {
			return
		}
		if active {
			m.run.Output(context.WithoutCancel(ctx), "swapoff", path)
		}
		os.Remove(path)
		lg.Logf("Removed %s again", path)
	}()

	lg.Logf("Creating %s (%d MB)", path, sizeMB)
	usedDD, err := m.newSwapFile(ctx, path, fsType, sizeMB, false, lg)
	if err != nil {
		return err
	}
	for {
		if err := m.mkswap(ctx, path, lg); err != nil {
			return err
		}
		out, err := m.run.Output(ctx, "swapon", path)
		if err == nil {
			break
		}
		switch {
		case notPermitted(out):
			return usererr.New("swap_not_permitted")
		case usedDD && strings.Contains(out, "Invalid argument"):
			return usererr.New("swap_unsupported_fs", fsType)
		case usedDD:
			return err
		}
		// A preallocated file can look like a file with holes to swapon.
		lg.Logf("swapon refused the file (%s); writing it with dd instead", run.Tail(out, 1))
		if usedDD, err = m.newSwapFile(ctx, path, fsType, sizeMB, true, lg); err != nil {
			return err
		}
	}
	active = true
	lg.Logf("Adding %s to %s", path, m.p.Fstab)
	if err := m.editFstab(func(lines []string) []string {
		if slices.ContainsFunc(lines, func(l string) bool { return fstabTarget(l) == path }) {
			return lines
		}
		return append(lines, fstabEscape(path)+" none swap sw 0 0")
	}); err != nil {
		return err
	}
	lg.Logf("Swap is on: %d MB", sizeMB)
	return nil
}

// newSwapFile writes an empty swap file of sizeMB, readable only by root,
// with fallocate or else dd, and reports whether it used dd. On btrfs the
// file must not be copy-on-write, so it is marked first.
func (m *Manager) newSwapFile(ctx context.Context, path, fsType string, sizeMB int, useDD bool, lg Logger) (bool, error) {
	os.Remove(path)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return false, err
	}
	f.Close()
	if err := os.Chmod(path, 0o600); err != nil {
		return false, err
	}
	if fsType == "btrfs" {
		if out, err := m.run.Output(ctx, "chattr", "+C", path); err != nil {
			lg.Logf("chattr +C failed: %s", run.Tail(out, 1))
		}
	}
	if !useDD {
		out, err := m.run.Output(ctx, "fallocate", "-l", strconv.FormatInt(int64(sizeMB)<<20, 10), path)
		if err == nil {
			return false, nil
		}
		lg.Logf("fallocate failed (%s); writing the file with dd instead", run.Tail(out, 1))
	}
	lg.Logf("Writing %d MB of zeros, this can take a minute", sizeMB)
	return true, m.run.Stream(ctx, lg, "dd", "if=/dev/zero", "of="+path, "bs=1M", "count="+strconv.Itoa(sizeMB), "status=none")
}

func (m *Manager) mkswap(ctx context.Context, path string, lg Logger) error {
	out, err := m.run.Output(ctx, "mkswap", path)
	if err != nil {
		return err
	}
	if t := run.Tail(out, 1); t != "" {
		lg.Logf("%s", t)
	}
	return nil
}

// notPermitted reports whether a program failed because the kernel did not
// allow it, as in a container.
func notPermitted(out string) bool {
	return strings.Contains(out, "Operation not permitted") || strings.Contains(out, "Permission denied") ||
		strings.Contains(out, "Read-only file system")
}

// CheckRemoveSwap returns the error that removing the panel's swap file
// would get.
func (m *Manager) CheckRemoveSwap() error {
	s := m.Swap()
	if !s.FileExists && !s.FileActive && !m.inFstab() {
		return usererr.New("swap_not_found")
	}
	for _, d := range s.Devices {
		// Turning swap off moves its pages back into memory: refuse when
		// they would not fit, rather than wake the OOM killer.
		if d.Panel && d.UsedKB > 0 && d.UsedKB+64<<10 > s.MemAvailableKB {
			return usererr.New("swap_in_use", fmt.Sprintf("%d MB / %d MB", d.UsedKB>>10, s.MemAvailableKB>>10))
		}
	}
	return nil
}

func (m *Manager) inFstab() bool {
	b, _ := os.ReadFile(m.p.Fstab)
	return slices.ContainsFunc(strings.Split(string(b), "\n"), func(l string) bool { return fstabTarget(l) == m.p.SwapFile })
}

// RemoveSwap turns the panel's swap file off and deletes it and its fstab
// line.
func (m *Manager) RemoveSwap(ctx context.Context, lg Logger) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.CheckRemoveSwap(); err != nil {
		return err
	}
	path := m.p.SwapFile
	if m.Swap().FileActive {
		lg.Logf("Turning off %s", path)
		if err := m.run.Stream(ctx, lg, "swapoff", path); err != nil {
			return err
		}
	}
	lg.Logf("Removing %s from %s", path, m.p.Fstab)
	if err := m.editFstab(func(lines []string) []string {
		return slices.DeleteFunc(lines, func(l string) bool { return fstabTarget(l) == path })
	}); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	lg.Logf("Deleted %s", path)
	return nil
}

// fstabEscape writes a path as fstab needs it, with octal escapes for
// spaces and backslashes.
func fstabEscape(p string) string {
	return strings.NewReplacer(`\`, `\134`, " ", `\040`, "\t", `\011`, "\n", `\012`).Replace(p)
}

// fstabTarget returns the device or file of an fstab line ("" for
// comments and blank lines).
func fstabTarget(line string) string {
	f := strings.Fields(line)
	if len(f) < 3 || strings.HasPrefix(f[0], "#") {
		return ""
	}
	return unescapeOctal(f[0])
}

// editFstab changes /etc/fstab line by line and replaces it atomically
// when something changed.
func (m *Manager) editFstab(fn func([]string) []string) error {
	b, err := os.ReadFile(m.p.Fstab)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	text := strings.TrimRight(string(b), "\n")
	var lines []string
	if text != "" {
		lines = strings.Split(text, "\n")
	}
	before := strings.Join(lines, "\n")
	lines = fn(slices.Clone(lines))
	after := strings.Join(lines, "\n")
	if after == before {
		return nil
	}
	return writeFile(m.p.Fstab, []byte(after+"\n"), 0o644)
}

// SetSwappiness sets vm.swappiness now and at every boot (a sysctl file of
// the panel's).
func (m *Manager) SetSwappiness(v int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if v < 0 || v > 100 {
		return usererr.New("swappiness_invalid")
	}
	old := readOld(m.p.SysctlSwap)
	text := fmt.Sprintf("# Written by the panel (System → Swap).\nvm.swappiness = %d\n", v)
	if err := writeFile(m.p.SysctlSwap, []byte(text), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(m.p.Swappiness, []byte(strconv.Itoa(v)+"\n"), 0o644); err != nil {
		restore(m.p.SysctlSwap, old)
		if errors.Is(err, os.ErrPermission) || strings.Contains(err.Error(), "read-only") {
			return usererr.New("sysctl_not_permitted")
		}
		return err
	}
	return nil
}
