package software

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/software/apt"
	"github.com/nefocp/nefo/internal/usererr"
)

// Memcached, a small in-memory cache, from the distribution's package. The
// panel writes its whole configuration: it listens on 127.0.0.1 only, over
// TCP; UDP is off (-U 0), so it can never be used to amplify attacks; its
// memory and connections are sized for the server.

// Where Memcached lives; tests point these elsewhere.
var (
	memcachedBin  = "/usr/bin/memcached"
	memcachedConf = "/etc/memcached.conf"
	memcachedAddr = "127.0.0.1:11211"
)

// MemcachedInstalled reports whether Memcached is installed.
func MemcachedInstalled() bool { return exists(memcachedBin) }

// memcachedMemoryMB is the memory Memcached may use on this server.
func memcachedMemoryMB(memTotal int64) int {
	switch {
	case memTotal >= 3<<30:
		return 256
	case memTotal >= 1500<<20:
		return 128
	}
	return 64
}

// MemcachedConfig returns /etc/memcached.conf for a server with memTotal
// bytes of RAM. Debian's start script reads one option per line.
func MemcachedConfig(memTotal int64) string {
	conns := 1024
	if memTotal > 0 && memTotal < 1500<<20 {
		conns = 512
	}
	return fmt.Sprintf(`# Managed by Nefo. Do not edit: the panel rewrites this file.
# Memcached listens on 127.0.0.1 only, over TCP. UDP is off (-U 0), so it
# cannot be used to amplify attacks.
logfile /var/log/memcached.log
-m %d
-p 11211
-U 0
-l 127.0.0.1
-c %d
-u memcache
-P /var/run/memcached/memcached.pid
`, memcachedMemoryMB(memTotal), conns)
}

// MemcachedItem is Memcached's item of the Software page.
func (m *Manager) MemcachedItem(ctx context.Context) Item {
	it := Item{ID: "memcached", Kind: "memcached", Group: "data", Installed: MemcachedInstalled(),
		RAMMB: memcachedMemoryMB(memTotal()), RAMMax: true}
	if it.Installed {
		if out, err := m.run.Output(ctx, memcachedBin, "-V"); err == nil {
			it.Version = firstMatch(`memcached ([0-9.]+)`, out)
		}
		it.Block(nil)
		it.Stopped = !m.unitActive(ctx, "memcached")
	}
	return it
}

// InstallMemcached installs Memcached and the PHP extension for it in
// every installed PHP version, then writes its configuration.
func (m *Manager) InstallMemcached(ctx context.Context, lg Logger) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := run.Apt(ctx, m.run, lg, "update"); err != nil {
		return err
	}
	pkgs := []string{"memcached"}
	versions := installedPHP()
	for _, v := range versions {
		pkgs = append(pkgs, "php"+v+"-memcached")
	}
	lg.Logf("Installing Memcached")
	if err := run.Apt(ctx, m.run, lg, append([]string{"install", "--no-install-recommends"}, pkgs...)...); err != nil {
		return err
	}
	mem := memTotal()
	lg.Logf("Limiting Memcached to %d MB of memory, on 127.0.0.1 only, without UDP", memcachedMemoryMB(mem))
	if err := fsutil.WriteFileAtomic(memcachedConf, []byte(MemcachedConfig(mem)), 0o644); err != nil {
		return err
	}
	if _, err := m.run.Output(ctx, "systemctl", "enable", "memcached"); err != nil {
		return err
	}
	if out, err := m.run.Output(ctx, "systemctl", "restart", "memcached"); err != nil {
		return fmt.Errorf("starting Memcached failed: %s", run.Tail(out, 10))
	}
	for _, v := range versions {
		m.run.Output(ctx, "systemctl", "reload", "php"+v+"-fpm")
	}
	lg.Logf("Memcached is running on 127.0.0.1:11211. PHP can use it with the memcached extension.")
	return nil
}

// CheckRemoveMemcached returns why Memcached cannot be uninstalled now.
// It is a cache: nothing in it needs keeping.
func (m *Manager) CheckRemoveMemcached(ctx context.Context) error {
	if !MemcachedInstalled() {
		return usererr.New("software_not_installed", "Memcached")
	}
	return nil
}

var phpMemcachedRe = regexp.MustCompile(`^php([0-9]+\.[0-9]+)-memcached$`)

// RemoveMemcached purges Memcached and its PHP extensions, then restarts
// the PHP versions that had the extension loaded.
func (m *Manager) RemoveMemcached(ctx context.Context, lg Logger) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.CheckRemoveMemcached(ctx); err != nil {
		return err
	}
	pkgs, err := apt.Installed(ctx, m.run, true, "memcached", "php*-memcached")
	if err != nil {
		return err
	}
	var versions []string
	for _, p := range pkgs {
		if mm := phpMemcachedRe.FindStringSubmatch(p); mm != nil && phpInstalled(mm[1]) {
			versions = append(versions, mm[1])
		}
	}
	if err := apt.Check(ctx, m.run, true, pkgs...); err != nil {
		return err
	}
	lg.Logf("Stopping Memcached")
	m.run.Output(ctx, "systemctl", "disable", "--now", "memcached")
	lg.Logf("Removing Memcached and the PHP extensions for it")
	if err := apt.Remove(ctx, m.run, lg, true, pkgs...); err != nil {
		return err
	}
	for _, v := range versions {
		lg.Logf("Restarting PHP %s without the Memcached extension", v)
		if out, err := m.run.Output(ctx, "systemctl", "restart", "php"+v+"-fpm"); err != nil {
			lg.Logf("Warning: restarting php%s-fpm failed: %s", v, strings.TrimSpace(out))
		}
	}
	lg.Logf("Memcached is removed. Websites that used it as a cache keep working without it.")
	return nil
}

// RestartMemcached restarts Memcached, which empties it.
func (m *Manager) RestartMemcached(ctx context.Context) error {
	if !MemcachedInstalled() {
		return usererr.New("software_not_installed", "Memcached")
	}
	if out, err := m.run.Output(ctx, "systemctl", "restart", "memcached"); err != nil {
		return fmt.Errorf("restarting Memcached failed: %s", run.Tail(out, 10))
	}
	return nil
}

// MemcachedStats is what the Databases page shows about Memcached.
type MemcachedStats struct {
	Installed   bool    `json:"installed"`
	Running     bool    `json:"running"`
	Error       string  `json:"error,omitempty"`
	Version     string  `json:"version,omitempty"`
	Uptime      int64   `json:"uptime"`      // seconds
	Items       int64   `json:"items"`       // stored now
	Bytes       int64   `json:"bytes"`       // memory the items use
	LimitBytes  int64   `json:"limit_bytes"` // -m
	Hits        int64   `json:"hits"`
	Misses      int64   `json:"misses"`
	HitRate     float64 `json:"hit_rate"`    // percent of gets that found their key, -1 before any
	Connections int64   `json:"connections"` // open now
	MaxConns    int64   `json:"max_conns"`   // -c
	Evictions   int64   `json:"evictions"`   // items dropped to make room
	Listen      string  `json:"listen"`      // the interface it listens on, as it says (-l)
	UDPPort     int64   `json:"udp_port"`    // 0: UDP is off
}

// Memcached asks Memcached for its statistics over 127.0.0.1:11211.
func (m *Manager) Memcached(ctx context.Context) MemcachedStats {
	st := MemcachedStats{Installed: MemcachedInstalled(), HitRate: -1}
	if !st.Installed {
		return st
	}
	stats, settings, err := memcachedStats(ctx, memcachedAddr)
	if err != nil {
		st.Error = err.Error()
		return st
	}
	st.Running = true
	fillMemcachedStats(&st, stats, settings)
	return st
}

func fillMemcachedStats(st *MemcachedStats, s, settings map[string]string) {
	n := func(m map[string]string, k string) int64 { v, _ := strconv.ParseInt(m[k], 10, 64); return v }
	st.Version, st.Uptime, st.Items, st.Bytes = s["version"], n(s, "uptime"), n(s, "curr_items"), n(s, "bytes")
	st.LimitBytes, st.Hits, st.Misses = n(s, "limit_maxbytes"), n(s, "get_hits"), n(s, "get_misses")
	st.Connections, st.Evictions = n(s, "curr_connections"), n(s, "evictions")
	st.MaxConns, st.Listen, st.UDPPort = n(settings, "maxconns"), settings["inter"], n(settings, "udpport")
	if st.Hits+st.Misses > 0 {
		st.HitRate = float64(st.Hits) * 100 / float64(st.Hits+st.Misses)
	}
}

// memcachedStats sends "stats" and "stats settings" and returns their STAT
// lines.
func memcachedStats(ctx context.Context, addr string) (stats, settings map[string]string, err error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, nil, err
	}
	defer conn.Close()
	if dl, ok := ctx.Deadline(); ok {
		conn.SetDeadline(dl)
	}
	br := bufio.NewReader(io.LimitReader(conn, 256<<10))
	if _, err := io.WriteString(conn, "stats\r\n"); err != nil {
		return nil, nil, err
	}
	if stats, err = parseMemcachedStats(br); err != nil {
		return nil, nil, err
	}
	if _, err := io.WriteString(conn, "stats settings\r\n"); err != nil {
		return nil, nil, err
	}
	if settings, err = parseMemcachedStats(br); err != nil {
		return nil, nil, err
	}
	return stats, settings, nil
}

// parseMemcachedStats reads an answer to "stats" up to its END line.
func parseMemcachedStats(r *bufio.Reader) (map[string]string, error) {
	out := map[string]string{}
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, errors.New("memcached closed the connection before END")
			}
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "END" {
			return out, nil
		}
		if f := strings.SplitN(line, " ", 3); len(f) == 3 && f[0] == "STAT" {
			out[f[1]] = f[2]
			continue
		}
		if strings.HasPrefix(line, "ERROR") || strings.HasPrefix(line, "SERVER_ERROR") || strings.HasPrefix(line, "CLIENT_ERROR") {
			return nil, fmt.Errorf("memcached answered %q", line)
		}
	}
}
