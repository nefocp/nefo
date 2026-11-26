// Package systools runs the server's own upkeep: operating system updates
// (and automatic security updates), a swap file, the time zone, NTP and the
// host name, TCP BBR, and cleaning up disk space.
//
// Every program gets its arguments separately (see package run), and every
// file it changes, such as /etc/fstab or /etc/hosts, is replaced atomically.
// The paths live in Paths, so tests can point them at a temporary folder.
package systools

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/run"
)

// Logger receives progress messages for a job log.
type Logger interface {
	io.Writer
	Logf(format string, args ...any)
}

// Settings stores small values, such as the result of the last update
// check (the panel's settings table).
type Settings interface {
	Setting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
}

// Paths are the files and folders the manager reads and writes.
type Paths struct {
	// Updates
	RebootRequired string // /var/run/reboot-required
	RebootPkgs     string // /var/run/reboot-required.pkgs
	AptLists       string // /var/lib/apt/lists, refreshed by apt-get update
	AutoUpgrades   string // /etc/apt/apt.conf.d/20auto-upgrades
	Unattended     string // /usr/bin/unattended-upgrade
	Boot           string // /boot, for installed kernels
	OSRelease      string // /proc/sys/kernel/osrelease, the running kernel

	// Swap
	SwapFile    string // the panel's swap file
	Fstab       string // /etc/fstab
	ProcSwaps   string // /proc/swaps
	Meminfo     string // /proc/meminfo
	Swappiness  string // /proc/sys/vm/swappiness
	SysctlSwap  string // the panel's vm.swappiness file in /etc/sysctl.d
	SysctlBBR   string // the panel's BBR file in /etc/sysctl.d
	ModulesBBR  string // the panel's tcp_bbr file in /etc/modules-load.d
	ProcCC      string // /proc/sys/net/ipv4/tcp_congestion_control
	ProcCCAvail string // /proc/sys/net/ipv4/tcp_available_congestion_control
	ProcQdisc   string // /proc/sys/net/core/default_qdisc
	Modules     string // /lib/modules, for tcp_bbr.ko

	// Time and host name
	Hosts       string // /etc/hosts
	CloudCfgDir string // /etc/cloud/cloud.cfg.d (cloud-init)

	// Cleanup
	AptArchives string   // /var/cache/apt/archives
	VarLog      string   // /var/log
	KeepLogs    []string // folders in VarLog whose old logs are kept (web server logs: traffic statistics)
	DockerBin   string   // /usr/bin/docker
}

// DefaultPaths are the paths on a Debian or Ubuntu server.
func DefaultPaths() Paths {
	return Paths{
		RebootRequired: "/var/run/reboot-required",
		AptLists:       "/var/lib/apt/lists",
		RebootPkgs:     "/var/run/reboot-required.pkgs",
		AutoUpgrades:   "/etc/apt/apt.conf.d/20auto-upgrades",
		Unattended:     "/usr/bin/unattended-upgrade",
		Boot:           "/boot",
		OSRelease:      "/proc/sys/kernel/osrelease",

		SwapFile:    "/panel.swap",
		Fstab:       "/etc/fstab",
		ProcSwaps:   "/proc/swaps",
		Meminfo:     "/proc/meminfo",
		Swappiness:  "/proc/sys/vm/swappiness",
		SysctlSwap:  "/etc/sysctl.d/90-panel-swap.conf",
		SysctlBBR:   "/etc/sysctl.d/90-panel-bbr.conf",
		ModulesBBR:  "/etc/modules-load.d/panel-bbr.conf",
		ProcCC:      "/proc/sys/net/ipv4/tcp_congestion_control",
		ProcCCAvail: "/proc/sys/net/ipv4/tcp_available_congestion_control",
		ProcQdisc:   "/proc/sys/net/core/default_qdisc",
		Modules:     "/lib/modules",

		Hosts:       "/etc/hosts",
		CloudCfgDir: "/etc/cloud/cloud.cfg.d",

		AptArchives: "/var/cache/apt/archives",
		VarLog:      "/var/log",
		KeepLogs:    []string{"/var/log/nginx", "/var/log/apache2", "/var/log/openlitespeed", "/var/log/journal"},
		DockerBin:   "/usr/bin/docker",
	}
}

// Manager runs the system tools. Changes are serialised.
type Manager struct {
	run      run.Runner
	settings Settings
	p        Paths
	mu       sync.Mutex // one change at a time

	now      func() time.Time
	hostname func() (string, error)
	// diskFree returns the free bytes and the file system type ("ext4",
	// "btrfs"…) of the file system that holds dir.
	diskFree func(dir string) (uint64, string, error)
	// trustedDir reports whether the cleanup may delete old logs in a
	// folder: only root's own folders, so nobody can swap one for a link.
	trustedDir func(fs.FileInfo) bool

	cacheMu sync.Mutex
	cache   *UpdateCheck // the last update check, loaded from settings on first use
	zones   []string     // timedatectl list-timezones, read once
}

// New returns a Manager for this server.
func New(r run.Runner, st Settings) *Manager {
	return &Manager{run: r, settings: st, p: DefaultPaths(),
		now: time.Now, hostname: os.Hostname, diskFree: diskFree, trustedDir: rootOwned}
}

func rootOwned(fi fs.FileInfo) bool {
	uid, _ := fsutil.Owner(fi)
	return uid == 0
}

// ---- small helpers ----

// readTrim returns the content of a small file without surrounding space,
// or "" when it cannot be read.
func readTrim(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func exists(path string) bool { return fsutil.Exists(path) }

// writeFile replaces path atomically with data, keeping its mode when it
// exists (perm otherwise). Folders it creates are readable by everyone, as
// /etc's are.
func writeFile(path string, data []byte, perm os.FileMode) error {
	if fi, err := os.Stat(path); err == nil {
		perm = fi.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	err := fsutil.WriteFileAtomic(path, data, perm)
	if errors.Is(err, syscall.EBUSY) {
		// A file bind-mounted into a container (/etc/hosts, /etc/hostname
		// on LXC or Docker) can't be replaced, only written in place.
		return os.WriteFile(path, data, perm)
	}
	return err
}

// restore puts back a file as it was before a change: old is its previous
// content, or nil when it did not exist.
func restore(path string, old []byte) {
	if old == nil {
		os.Remove(path)
		return
	}
	writeFile(path, old, 0o644)
}

// readOld returns a file's content, or nil when it does not exist.
func readOld(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return b
}

// MB formats a number of bytes as megabytes for logs and audit entries.
func MB(b int64) string {
	switch {
	case b >= 10<<30:
		return fmt.Sprintf("%.0f GB", float64(b)/(1<<30))
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	default:
		return strconv.FormatInt((b+(1<<19))>>20, 10) + " MB"
	}
}

// fields splits a line of a key-value output ("Key=value").
func keyValues(out string) map[string]string {
	kv := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
			kv[k] = v
		}
	}
	return kv
}
