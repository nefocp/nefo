package postgres

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"time"

	"github.com/nefocp/nefo/internal/usererr"
)

// The dumps Upgrade makes (one gzipped pg_dumpall file per upgrade) stay in
// backupDir until the administrator deletes them on the Databases page.

// UpgradeDump is one of them.
type UpgradeDump struct {
	Name    string    `json:"name"` // the file's name
	From    string    `json:"from"` // the major version before the upgrade ("15")
	To      string    `json:"to"`   // and after it ("17")
	Created time.Time `json:"created"`
	Size    int64     `json:"size"`
	Path    string    `json:"path"`
}

// upgradeDumpRe matches the names Upgrade gives its dumps.
var upgradeDumpRe = regexp.MustCompile(`^postgresql-([0-9]+(?:\.[0-9]+)?)-before-([0-9]+(?:\.[0-9]+)?)-([0-9]{8}-[0-9]{6})\.sql\.gz$`)

// UpgradeDumps lists the kept dumps, newest first.
func (m *Manager) UpgradeDumps() []UpgradeDump {
	out := []UpgradeDump{}
	entries, err := os.ReadDir(m.backupDir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		sm := upgradeDumpRe.FindStringSubmatch(e.Name())
		if sm == nil || !e.Type().IsRegular() {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		created, err := time.ParseInLocation("20060102-150405", sm[3], time.Local)
		if err != nil {
			created = fi.ModTime()
		}
		out = append(out, UpgradeDump{Name: e.Name(), From: sm[1], To: sm[2], Created: created, Size: fi.Size(),
			Path: filepath.Join(m.backupDir, e.Name())})
	}
	slices.SortFunc(out, func(a, b UpgradeDump) int { return b.Created.Compare(a.Created) })
	return out
}

// DeleteUpgradeDump deletes one kept dump by its file name. It waits for an
// upgrade that is running (which may be writing one).
func (m *Manager) DeleteUpgradeDump(name string) error {
	if !upgradeDumpRe.MatchString(name) {
		return usererr.New("not_found")
	}
	if !m.mu.TryLock() {
		return usererr.New("busy")
	}
	defer m.mu.Unlock()
	p := filepath.Join(m.backupDir, name)
	if fi, err := os.Lstat(p); err != nil || !fi.Mode().IsRegular() {
		return usererr.New("not_found")
	}
	return os.Remove(p)
}
