package hosting

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/malware"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// Malware scans read a site's folder with the panel's own signatures and,
// if asked and installed, ClamAV. Suspicious files can be quarantined:
// moved to a root-only folder, from where they can be restored or deleted.

// clamMinMemory is the free memory ClamAV needs to load its databases.
const clamMinMemory = 1400 << 20

// ScanReport is a site's latest scan with its quarantined files.
type ScanReport struct {
	FinishedAt  *time.Time           `json:"finished_at"`
	Files       int                  `json:"files"`
	Skipped     int                  `json:"skipped"`
	Findings    []malware.Finding    `json:"findings"`
	ClamAV      string               `json:"clamav"`
	ClamAVReady bool                 `json:"clamav_installed"`
	Quarantined []*store.Quarantined `json:"quarantined"`
}

func clamAVInstalled() bool {
	_, err := exec.LookPath("clamscan")
	return err == nil
}

// ScanReport returns a site's latest scan.
func (m *Manager) ScanReport(ctx context.Context, id int64) (*ScanReport, error) {
	if _, err := m.store.SiteByID(ctx, id); err != nil {
		return nil, err
	}
	r := &ScanReport{Findings: []malware.Finding{}, ClamAVReady: clamAVInstalled()}
	sc, err := m.store.MalwareScan(ctx, id)
	switch {
	case err == nil:
		t := sc.FinishedAt
		r.FinishedAt, r.Files, r.Skipped, r.ClamAV = &t, sc.Files, sc.Skipped, sc.ClamAV
		if err := json.Unmarshal([]byte(sc.Findings), &r.Findings); err != nil {
			return nil, err
		}
	case !errors.Is(err, store.ErrNotFound):
		return nil, err
	}
	if r.Quarantined, err = m.store.ListQuarantined(ctx, id); err != nil {
		return nil, err
	}
	return r, nil
}

var severityRank = map[string]int{malware.Critical: 0, malware.High: 1, malware.Medium: 2}

// ScanSite scans a site's folder and stores the result.
func (m *Manager) ScanSite(ctx context.Context, id int64, clamav bool, lg Logger) error {
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return err
	}
	dir := m.paths.siteRoot(site.Domain)
	lg.Logf("Checking the files in %s", dir)
	res, err := malware.Scan(ctx, dir, func(n int) { lg.Logf("%d files checked", n) })
	if err != nil {
		return err
	}
	note := ""
	if clamav {
		var found []malware.Finding
		found, note = m.clamScan(ctx, dir, lg)
		res.Findings = append(res.Findings, found...)
	}
	slices.SortStableFunc(res.Findings, func(a, b malware.Finding) int {
		if d := severityRank[a.Severity] - severityRank[b.Severity]; d != 0 {
			return d
		}
		return strings.Compare(a.Path, b.Path)
	})
	b, err := json.Marshal(res.Findings)
	if err != nil {
		return err
	}
	if err := m.store.SaveMalwareScan(ctx, &store.MalwareScan{SiteID: id, FinishedAt: m.now(), Files: res.Files,
		Skipped: res.Skipped, Findings: string(b), ClamAV: note}); err != nil {
		return err
	}
	if len(res.Findings) == 0 {
		lg.Logf("Done: %d files checked, nothing suspicious found.", res.Files)
	} else {
		lg.Logf("Done: %d files checked, %d suspicious. Review them in Websites → Malware scan.", res.Files, len(res.Findings))
	}
	return nil
}

// clamScan runs ClamAV over dir when there is memory for it. The note says
// what happened, for the report.
func (m *Manager) clamScan(ctx context.Context, dir string, lg Logger) ([]malware.Finding, string) {
	if !clamAVInstalled() {
		return nil, "not_installed"
	}
	if dbs, _ := filepath.Glob("/var/lib/clamav/*.c[lv]d"); len(dbs) == 0 {
		return nil, "no_database"
	}
	if avail := memAvailable(); avail < clamMinMemory {
		lg.Logf("ClamAV skipped: it needs about %d MB of free memory, %d MB are available", clamMinMemory>>20, avail>>20)
		return nil, fmt.Sprintf("low_memory:%d", avail>>20)
	}
	lg.Logf("Scanning with ClamAV (this takes a few minutes)")
	out, err := m.run.Output(ctx, "nice", "-n", "19", "clamscan", "--recursive", "--infected", "--no-summary",
		"--max-filesize=25M", "--max-scansize=100M", dir)
	var exit *exec.ExitError
	switch {
	case err == nil: // nothing found
	case errors.As(err, &exit) && exit.ExitCode() == 1: // found something
	default:
		lg.Logf("ClamAV failed: %v", err)
		return nil, "failed"
	}
	found := malware.ParseClamscan(out, dir)
	lg.Logf("ClamAV: %d infected files", len(found))
	return found, "ok"
}

// memAvailable reads MemAvailable from /proc/meminfo, in bytes.
func memAvailable() int64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if rest, ok := strings.CutPrefix(sc.Text(), "MemAvailable:"); ok {
			kb, _ := strconv.ParseInt(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(rest), "kB")), 10, 64)
			return kb << 10
		}
	}
	return 0
}

func (m *Manager) quarantineFile(id int64) string {
	return filepath.Join(m.paths.Quarantine, strconv.FormatInt(id, 10))
}

// siteRootFS opens a site's folder so that no path can leave it.
func (m *Manager) siteRootFS(site *store.Site) (*os.Root, error) {
	return os.OpenRoot(m.paths.siteRoot(site.Domain))
}

// Quarantine moves a file of a site (path relative to the site folder) to
// the quarantine.
func (m *Manager) Quarantine(ctx context.Context, id int64, path, rule string) (*store.Quarantined, error) {
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return nil, err
	}
	rel := filepath.FromSlash(path)
	if !filepath.IsLocal(rel) {
		return nil, usererr.New("invalid_path", path)
	}
	root, err := m.siteRootFS(site)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	st, err := root.Lstat(rel)
	if err != nil || !st.Mode().IsRegular() {
		return nil, usererr.New("not_a_file", path)
	}
	uid, gid := fsutil.Owner(st)
	q := &store.Quarantined{SiteID: id, Path: filepath.ToSlash(rel), Rule: rule, Size: st.Size(),
		Mode: uint32(st.Mode().Perm()), UID: uid, GID: gid}
	if err := m.store.AddQuarantined(ctx, q); err != nil {
		return nil, err
	}
	err = func() error {
		if err := mkdir(m.paths.Quarantine, 0o700); err != nil {
			return err
		}
		src, err := root.Open(rel)
		if err != nil {
			return err
		}
		defer src.Close()
		if err := copyTo(m.quarantineFile(q.ID), src, 0o600); err != nil {
			return err
		}
		return root.Remove(rel)
	}()
	if err != nil {
		os.Remove(m.quarantineFile(q.ID))
		m.store.DeleteQuarantined(context.WithoutCancel(ctx), q.ID)
		return nil, err
	}
	m.dropFinding(ctx, id, q.Path)
	return q, nil
}

// dropFinding removes a file from the latest scan's findings, once it has
// been dealt with.
func (m *Manager) dropFinding(ctx context.Context, id int64, path string) {
	sc, err := m.store.MalwareScan(ctx, id)
	if err != nil {
		return
	}
	var fs []malware.Finding
	if json.Unmarshal([]byte(sc.Findings), &fs) != nil {
		return
	}
	fs = slices.DeleteFunc(fs, func(f malware.Finding) bool { return f.Path == path })
	if b, err := json.Marshal(fs); err == nil {
		sc.Findings = string(b)
		m.store.SaveMalwareScan(ctx, sc)
	}
}

// RestoreQuarantined puts a quarantined file back where it was, with its
// owner and mode.
func (m *Manager) RestoreQuarantined(ctx context.Context, id, qid int64) error {
	site, q, err := m.quarantined(ctx, id, qid)
	if err != nil {
		return err
	}
	root, err := m.siteRootFS(site)
	if err != nil {
		return err
	}
	defer root.Close()
	rel := filepath.FromSlash(q.Path)
	if _, err := root.Lstat(rel); err == nil {
		return usererr.New("restore_exists", q.Path)
	}
	if st, err := root.Stat(filepath.Dir(rel)); err != nil || !st.IsDir() {
		return usererr.New("restore_no_folder", filepath.ToSlash(filepath.Dir(rel)))
	}
	src, err := os.Open(m.quarantineFile(q.ID))
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := root.OpenFile(rel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		root.Remove(rel)
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}
	if q.UID >= 0 {
		if err := root.Lchown(rel, q.UID, q.GID); err != nil {
			return err
		}
	}
	if err := root.Chmod(rel, os.FileMode(q.Mode)); err != nil {
		return err
	}
	src.Close()
	os.Remove(m.quarantineFile(q.ID))
	return m.store.DeleteQuarantined(ctx, q.ID)
}

// DeleteQuarantined deletes a quarantined file for good.
func (m *Manager) DeleteQuarantined(ctx context.Context, id, qid int64) error {
	_, q, err := m.quarantined(ctx, id, qid)
	if err != nil {
		return err
	}
	if err := os.Remove(m.quarantineFile(q.ID)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return m.store.DeleteQuarantined(ctx, q.ID)
}

func (m *Manager) quarantined(ctx context.Context, id, qid int64) (*store.Site, *store.Quarantined, error) {
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	q, err := m.store.QuarantinedByID(ctx, qid)
	if err != nil {
		return nil, nil, err
	}
	if q.SiteID != id {
		return nil, nil, store.ErrNotFound
	}
	return site, q, nil
}

// deleteSiteMalware removes a deleted site's quarantine and scan.
func (m *Manager) deleteSiteMalware(ctx context.Context, id int64) error {
	qs, err := m.store.ListQuarantined(ctx, id)
	if err != nil {
		return err
	}
	for _, q := range qs {
		os.Remove(m.quarantineFile(q.ID))
	}
	return m.store.DeleteSiteMalware(ctx, id)
}

func copyTo(path string, src io.Reader, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, src); err != nil {
		f.Close()
		os.Remove(path)
		return err
	}
	return f.Close()
}

// InstallClamAV installs ClamAV. freshclam then downloads its databases in
// the background, which takes a few minutes.
func (m *Manager) InstallClamAV(ctx context.Context, lg Logger) error {
	if !m.Supported() {
		return ErrUnsupported
	}
	lg.Logf("Installing ClamAV")
	if err := run.Apt(ctx, m.run, lg, "install", "-y", "--no-install-recommends", "clamav", "clamav-freshclam"); err != nil {
		return err
	}
	lg.Logf("ClamAV is installed. Its virus databases are downloading now; scans can use it in a few minutes.")
	return nil
}

// RemoveClamAV removes ClamAV and its databases.
func (m *Manager) RemoveClamAV(ctx context.Context, lg Logger) error {
	if !m.Supported() {
		return ErrUnsupported
	}
	lg.Logf("Removing ClamAV")
	return run.Apt(ctx, m.run, lg, "purge", "-y", "clamav", "clamav-freshclam", "clamav-base")
}

// ClamAVInstalled reports whether ClamAV's scanner is installed.
func (m *Manager) ClamAVInstalled() bool { return clamAVInstalled() }
