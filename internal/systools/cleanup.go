package systools

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/usererr"
)

// ---- disk cleanup ----

// journalKeep is what the journal is vacuumed down to.
const (
	journalKeep    = 100 << 20
	journalKeepArg = "--vacuum-size=100M"
)

// CleanItems are the cleanups, in the order the page shows them.
var CleanItems = []string{"apt", "journal", "logs", "autoremove", "docker"}

// CleanItem is something that can be cleaned up and the space it frees.
type CleanItem struct {
	ID       string    `json:"id"`
	Bytes    int64     `json:"bytes"`           // what cleaning it frees
	Total    int64     `json:"total,omitempty"` // journal: its whole size
	Files    int       `json:"files,omitempty"` // rotated logs, cached packages
	Packages []PkgSize `json:"packages,omitempty"`
	Error    string    `json:"error,omitempty"` // it could not be measured
}

// PkgSize is a package that autoremove would remove.
type PkgSize struct {
	Name  string `json:"name"`
	Bytes int64  `json:"bytes"`
}

// Cleanup measures what each cleanup would free. Docker is only listed
// when it is installed.
func (m *Manager) Cleanup(ctx context.Context) []CleanItem {
	out := []CleanItem{}
	for _, id := range CleanItems {
		if id == "docker" && !exists(m.p.DockerBin) {
			continue
		}
		out = append(out, m.measure(ctx, id))
	}
	return out
}

func (m *Manager) measure(ctx context.Context, id string) CleanItem {
	it := CleanItem{ID: id}
	var err error
	switch id {
	case "apt":
		it.Bytes, it.Files = m.aptCache()
	case "journal":
		it.Total, err = m.journalUsage(ctx)
		it.Bytes = max(it.Total-journalKeep, 0)
	case "logs":
		files := m.rotatedLogs()
		it.Files = len(files)
		for _, f := range files {
			it.Bytes += f.size
		}
	case "autoremove":
		it.Packages, err = m.autoremovable(ctx)
		for _, p := range it.Packages {
			it.Bytes += p.Bytes
		}
	case "docker":
		// A daemon that hangs must not hold up the page.
		dctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		it.Bytes, err = m.dockerReclaimable(dctx)
		cancel()
	}
	if err != nil {
		it.Error = run.Tail(err.Error(), 3)
	}
	return it
}

// CheckClean returns the error that cleaning id would get.
func (m *Manager) CheckClean(id string) error {
	if !slices.Contains(CleanItems, id) {
		return usererr.New("cleanup_unknown", id)
	}
	if id == "docker" && !exists(m.p.DockerBin) {
		return usererr.New("software_not_installed", "Docker")
	}
	return nil
}

// Clean runs one cleanup and returns the bytes it freed. For autoremove,
// confirm is the list of packages the administrator saw: when apt would
// now remove anything else, nothing is removed.
func (m *Manager) Clean(ctx context.Context, id string, confirm []string, lg Logger) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.CheckClean(id); err != nil {
		return 0, err
	}
	switch id {
	case "apt":
		before, _ := m.aptCache()
		lg.Logf("Deleting downloaded package files")
		if err := run.Apt(ctx, m.run, lg, "clean"); err != nil {
			return 0, err
		}
		after, _ := m.aptCache()
		return max(before-after, 0), nil

	case "journal":
		before, err := m.journalUsage(ctx)
		if err != nil {
			return 0, err
		}
		lg.Logf("Shrinking the system journal to %s", MB(journalKeep))
		// Only finished journal files are vacuumed: start new ones first.
		if err := m.run.Stream(ctx, lg, "journalctl", "--rotate"); err != nil {
			lg.Logf("journalctl --rotate failed: %v", err)
		}
		if err := m.run.Stream(ctx, lg, "journalctl", journalKeepArg); err != nil {
			return 0, err
		}
		after, err := m.journalUsage(ctx)
		if err != nil {
			return 0, nil
		}
		return max(before-after, 0), nil

	case "logs":
		var freed int64
		n := 0
		for _, f := range m.rotatedLogs() {
			if err := os.Remove(f.path); err != nil {
				lg.Logf("Could not delete %s: %v", f.path, err)
				continue
			}
			freed += f.size
			n++
		}
		lg.Logf("Deleted %d old log files", n)
		return freed, nil

	case "autoremove":
		pkgs, err := m.autoremovable(ctx)
		if err != nil {
			return 0, err
		}
		if len(pkgs) == 0 {
			lg.Logf("No packages to remove.")
			return 0, nil
		}
		var freed int64
		for _, p := range pkgs {
			if !slices.Contains(confirm, p.Name) {
				return 0, usererr.New("autoremove_changed", p.Name)
			}
			freed += p.Bytes
		}
		lg.Logf("Removing %d packages that nothing needs any more", len(pkgs))
		if err := run.Apt(ctx, m.run, lg, "autoremove"); err != nil {
			return 0, err
		}
		return freed, nil

	case "docker":
		before, err := m.dockerReclaimable(ctx)
		if err != nil {
			return 0, err
		}
		lg.Logf("Removing images that no container uses")
		if err := m.run.Stream(ctx, lg, "docker", "image", "prune", "--all", "--force"); err != nil {
			return 0, err
		}
		lg.Logf("Removing the build cache")
		if err := m.run.Stream(ctx, lg, "docker", "builder", "prune", "--all", "--force"); err != nil {
			return 0, err
		}
		after, err := m.dockerReclaimable(ctx)
		if err != nil {
			return before, nil
		}
		return max(before-after, 0), nil
	}
	return 0, nil
}

// aptCache returns the size and number of the downloaded package files
// that apt-get clean deletes.
func (m *Manager) aptCache() (int64, int) {
	var size int64
	n := 0
	for _, dir := range []string{m.p.AptArchives, filepath.Join(m.p.AptArchives, "partial")} {
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if !e.Type().IsRegular() || (dir == m.p.AptArchives && !strings.HasSuffix(e.Name(), ".deb")) {
				continue
			}
			if fi, err := e.Info(); err == nil {
				size += fi.Size()
				n++
			}
		}
	}
	return size, n
}

// "Archived and active journals take up 120.0M in the file system."
var journalRe = regexp.MustCompile(`take up ([0-9]+(?:\.[0-9]+)?)([BKMGTPE])`)

func (m *Manager) journalUsage(ctx context.Context) (int64, error) {
	out, err := m.run.Output(ctx, "journalctl", "--disk-usage")
	if err != nil {
		return 0, err
	}
	mm := journalRe.FindStringSubmatch(out)
	if mm == nil {
		return 0, &run.Error{Cmd: "journalctl --disk-usage", Err: fs.ErrInvalid, Output: out}
	}
	return scaled(mm[1], strings.Index("BKMGTPE", mm[2]), 1024), nil
}

// scaled returns number × base^power.
func scaled(number string, power int, base float64) int64 {
	f, _ := strconv.ParseFloat(number, 64)
	for range power {
		f *= base
	}
	return int64(f)
}

type logFile struct {
	path string
	size int64
}

// rotatedName matches logs that logrotate has already rotated:
// syslog.1, syslog.2.gz, dpkg.log.3.gz…
var rotatedName = regexp.MustCompile(`\.(gz|[0-9]+)$`)

// rotatedLogs lists the rotated logs in /var/log, without the web server
// logs (the traffic statistics read them) and only in root's own folders.
func (m *Manager) rotatedLogs() []logFile {
	var out []logFile
	keep := map[string]bool{}
	for _, k := range m.p.KeepLogs {
		keep[filepath.Clean(k)] = true
	}
	filepath.WalkDir(m.p.VarLog, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			fi, err := d.Info()
			if err != nil || keep[filepath.Clean(path)] || !m.trustedDir(fi) {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !rotatedName.MatchString(d.Name()) {
			return nil
		}
		if fi, err := d.Info(); err == nil {
			out = append(out, logFile{path, fi.Size()})
		}
		return nil
	})
	return out
}

// autoremovable lists the packages apt-get autoremove would remove, with
// their installed size.
func (m *Manager) autoremovable(ctx context.Context) ([]PkgSize, error) {
	out, err := m.run.Output(ctx, "apt-get", "-s", "autoremove")
	if err != nil {
		return nil, err
	}
	pkgs := []PkgSize{}
	var names []string
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && f[0] == "Remv" && pkgRe.MatchString(f[1]) && !slices.Contains(names, f[1]) {
			names = append(names, f[1])
		}
	}
	if len(names) == 0 {
		return pkgs, nil
	}
	sizes := map[string]int64{}
	// Installed-Size is in KB. dpkg-query prints the name without the
	// architecture.
	q, _ := m.run.Output(ctx, "dpkg-query", append([]string{"-W", "-f", "${Package}\t${Installed-Size}\n"}, names...)...)
	for _, line := range strings.Split(q, "\n") {
		n, s, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if kb, err := strconv.ParseInt(s, 10, 64); ok && err == nil {
			sizes[n] = kb << 10
		}
	}
	for _, n := range names {
		base, _, _ := strings.Cut(n, ":")
		pkgs = append(pkgs, PkgSize{Name: n, Bytes: sizes[base]})
	}
	return pkgs, nil
}

// "13.26kB (100%)"
var dockerSizeRe = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?)\s*([kKMGTP]?)B`)

// dockerReclaimable is what pruning unused images and the build cache
// frees, from docker system df.
func (m *Manager) dockerReclaimable(ctx context.Context) (int64, error) {
	out, err := m.run.Output(ctx, "docker", "system", "df", "--format", "{{json .}}")
	if err != nil {
		return 0, err
	}
	var total int64
	for _, line := range strings.Split(out, "\n") {
		var row struct{ Type, Reclaimable string }
		if json.Unmarshal([]byte(strings.TrimSpace(line)), &row) != nil {
			continue
		}
		if row.Type != "Images" && row.Type != "Build Cache" {
			continue
		}
		if mm := dockerSizeRe.FindStringSubmatch(strings.TrimSpace(row.Reclaimable)); mm != nil {
			power := 0
			if mm[2] != "" {
				power = strings.Index("KMGTP", strings.ToUpper(mm[2])) + 1
			}
			// Docker counts in powers of 1000.
			total += scaled(mm[1], power, 1000)
		}
	}
	return total, nil
}
