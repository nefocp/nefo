package backup

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/appstore"
	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// App backups hold an app store instance: its folder in /var/lib/panel-apps
// (compose file, secrets, the containers' data, with owners and modes)
// under files/, and database dumps under dumps/. The recipe says how to
// keep them consistent: "pause" freezes the containers while their files
// are copied (SQLite and the like recover from that like after a power
// cut), "dump" writes the databases out with their own tools first and
// leaves their raw files out, "none" copies as is.

// KindApp is an app store instance.
const KindApp = "app"

var appNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,40}$`)

// BackupApp backs up an app instance.
func (m *Manager) BackupApp(ctx context.Context, name string, scheduleID int64, remote bool, lg Logger) (*store.Backup, error) {
	return m.backupApp(ctx, name, scheduleID, remote, "", lg)
}

func (m *Manager) backupApp(ctx context.Context, name string, scheduleID int64, remote bool, label string, lg Logger) (*store.Backup, error) {
	plan, err := m.hosting.AppBackup(ctx, name)
	if err != nil {
		return nil, err
	}
	man := manifest{Format: 1, Kind: KindApp, App: plan.AppID, Path: plan.Dir, Label: label}
	for _, d := range plan.Dumps {
		man.Databases = append(man.Databases, d.File)
	}
	return m.make(ctx, KindApp, name, man, scheduleID, remote, lg, func(tw *tar.Writer) error {
		return m.addApp(ctx, tw, plan, lg)
	})
}

// appState is app.json in an app backup: the instance's secrets, which go
// with the restored data (the archive already holds them in compose.json
// and secrets/; it is root-only like every backup).
type appState struct {
	Secrets map[string]string `json:"secrets"`
}

// addApp writes the instance's secrets and dumps (while the app runs),
// then the folder (paused if the recipe says so).
func (m *Manager) addApp(ctx context.Context, tw *tar.Writer, plan *hosting.AppBackupPlan, lg Logger) error {
	state, err := json.Marshal(appState{Secrets: plan.Secrets})
	if err != nil {
		return err
	}
	if err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: "app.json", Size: int64(len(state)), Mode: 0o600, ModTime: time.Now()}); err != nil {
		return err
	}
	if _, err := tw.Write(state); err != nil {
		return err
	}
	for _, d := range plan.Dumps {
		lg.Logf("Dumping %s", d.File)
		tmp, err := os.CreateTemp(m.dir, ".dump-*")
		if err != nil {
			return err
		}
		err = m.hosting.AppDump(ctx, plan.Name, d, tmp)
		if err == nil {
			err = addFile(tw, "dumps/"+d.File, tmp)
		}
		tmp.Close()
		os.Remove(tmp.Name())
		if err != nil {
			return fmt.Errorf("dump %s: %w", d.File, err)
		}
	}
	if plan.Consistency == "pause" {
		lg.Logf("Pausing the app while its files are copied")
		unpause, err := m.hosting.AppPause(ctx, plan.Name)
		if err != nil {
			lg.Logf("Warning: could not pause the app: %v", err)
		}
		defer unpause()
	}
	root, err := os.OpenRoot(plan.Dir)
	if err != nil {
		return err
	}
	defer root.Close()
	excluded := func(p string) bool {
		for _, v := range plan.Exclude {
			if p == "data/"+v || strings.HasPrefix(p, "data/"+v+"/") {
				return true
			}
		}
		return false
	}
	lg.Logf("Packing %s", plan.Dir)
	n, err := addFolderTreeSkip(ctx, tw, root, "files", excluded)
	if err != nil {
		return err
	}
	lg.Logf("%d files packed", n)
	return nil
}

// addFile adds a temporary file (whose size tar needs first).
func addFile(tw *tar.Writer, name string, f *os.File) error {
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if err := tw.WriteHeader(&tar.Header{Typeflag: tar.TypeReg, Name: name, Size: fi.Size(), Mode: 0o600, ModTime: fi.ModTime()}); err != nil {
		return err
	}
	_, err = io.Copy(tw, io.LimitReader(f, fi.Size()))
	return err
}

// restoreApp puts an instance back: containers stopped, folder replaced,
// containers started, dumps loaded.
func (m *Manager) restoreApp(ctx context.Context, b *store.Backup, man manifest, tr *tar.Reader, lg Logger) error {
	plan, err := m.hosting.AppBackup(ctx, b.Target)
	if err != nil {
		return err
	}
	if man.Kind != KindApp || man.Path != plan.Dir || man.App != plan.AppID {
		return fmt.Errorf("the archive is not a backup of %s", b.Target)
	}
	lg.Logf("Stopping the app")
	if err := m.hosting.AppStop(ctx, plan.Name); err != nil {
		lg.Logf("Warning: %v", err)
	}
	dumps := map[string]string{}
	defer func() {
		for _, f := range dumps {
			os.Remove(f)
		}
	}()
	var state *appState
	err = restoreTree(ctx, plan.Dir, tr, lg, func(name string, r io.Reader) error {
		if name == "app.json" {
			state = &appState{}
			return json.NewDecoder(io.LimitReader(r, 1<<20)).Decode(state)
		}
		file, ok := strings.CutPrefix(name, "dumps/")
		if !ok || !slices.ContainsFunc(plan.Dumps, func(d appstore.Dump) bool { return d.File == file }) {
			return nil
		}
		tmp, err := os.CreateTemp(m.dir, ".restore-*")
		if err != nil {
			return err
		}
		dumps[file] = tmp.Name()
		_, err = io.Copy(tmp, r)
		if cerr := tmp.Close(); err == nil {
			err = cerr
		}
		return err
	})
	if err != nil {
		return err
	}
	var saved map[string]string
	if state != nil {
		saved = state.Secrets
	}
	if err := m.hosting.AppRestored(ctx, plan.Name, saved, logWriter{lg}); err != nil {
		return err
	}
	lg.Logf("Starting the app")
	if err := m.hosting.AppStart(ctx, plan.Name, logWriter{lg}); err != nil {
		return err
	}
	for _, d := range plan.Dumps {
		f, ok := dumps[d.File]
		if !ok {
			continue
		}
		lg.Logf("Loading %s", d.File)
		r, err := os.Open(f)
		if err != nil {
			return err
		}
		err = m.hosting.AppRestoreDump(ctx, plan.Name, d, r)
		r.Close()
		if err != nil {
			return fmt.Errorf("load %s: %w", d.File, err)
		}
	}
	return nil
}

// checkAppName checks the target of an app backup.
func checkAppName(name string) error {
	if !appNameRe.MatchString(name) {
		return usererr.New("invalid_target", name)
	}
	return nil
}

// addFolderTreeSkip is addFolderTree leaving out the paths skip names.
func addFolderTreeSkip(ctx context.Context, tw *tar.Writer, root *os.Root, prefix string, skip func(string) bool) (int, error) {
	return addFolderTreeFiltered(ctx, tw, root, prefix, skip)
}

// logWriter lets a Logger take program output too.
type logWriter struct{ Logger }

func (w logWriter) Write(p []byte) (int, error) {
	for _, l := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		if strings.TrimSpace(l) != "" {
			w.Logf("%s", l)
		}
	}
	return len(p), nil
}
