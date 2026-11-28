package hosting

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/nefocp/nefo/internal/appstore"
	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// A Docker app's website is cloned with its app: a new website and app
// instance of the same version, with the same passwords (the copied data
// was made with them) and a copy of the instance's data folder, taken
// while the app is stopped for a moment. The copy gets its own container
// names, ports and address: the recipe's ${domain} and ${url} name the new
// website.

// cloneRefused are the apps a copy of which would do wrong on its own,
// with the reason shown: n8n runs its active workflows (schedules,
// polling triggers) in both copies, and Odoo its scheduled actions and
// mail servers (the copy would fetch and delete the same mailbox's mail,
// and send the same reminders).
var cloneRefused = map[string]string{
	"n8n":  "clone_app_workflows",
	"odoo": "clone_app_jobs",
}

// CloneRefusal returns the reason code a Docker app cannot be cloned for,
// or "".
func CloneRefusal(appID string) string { return cloneRefused[appID] }

// CheckCloneDockerApp says whether the Docker app of a website can be
// copied to a new website at domain.
func (m *Manager) CheckCloneDockerApp(ctx context.Context, from *store.Site, domain string) (*appstore.App, *store.App, error) {
	app, ok := siteDockerApp(from)
	if !ok {
		return nil, nil, usererr.New("unknown_app", from.App)
	}
	if code := cloneRefused[app.ID]; code != "" {
		return nil, nil, usererr.New(code, app.Name)
	}
	if !m.Supported() {
		return nil, nil, ErrUnsupported
	}
	if !dockerReady() {
		return nil, nil, usererr.New("docker_needed")
	}
	inst, err := m.store.AppForSite(ctx, from.ID)
	if errors.Is(err, store.ErrNotFound) {
		// Installed before the app store: one update converts it.
		return nil, nil, usererr.New("clone_app_legacy", app.Name)
	}
	if err != nil {
		return nil, nil, err
	}
	if inst.State != "ready" {
		return nil, nil, usererr.New("app_busy", inst.Name)
	}
	if _, _, err := m.checkNames(ctx, domain, nil, 0); err != nil {
		return nil, nil, err
	}
	if res := m.CheckResources(ctx, app); !res.OK() {
		p := res.Refusals[0]
		return nil, nil, usererr.New(p.Code, p.Detail)
	}
	if dir, err := instanceDir(inst.Name); err == nil {
		need := dirSizeMB(filepath.Join(dir, "data")) + 512
		if free := appstore.ReadSystem(AppsDir).FreeDiskMB; free >= 0 && free < need {
			return nil, nil, usererr.New("clone_disk_space", strconv.Itoa(need)+" MB")
		}
	}
	return app, inst, nil
}

// dirSizeMB adds up the sizes of the files under dir.
func dirSizeMB(dir string) int {
	var n int64
	filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if fi, err := d.Info(); err == nil {
				n += fi.Size()
			}
		}
		return nil
	})
	return int(n >> 20)
}

// CloneDockerApp copies a website with its Docker app to a new website at
// domain. If anything fails, what it created is removed again; the
// original app runs again in any case.
func (m *Manager) CloneDockerApp(ctx context.Context, from *store.Site, domain string, lg Logger) (*store.Site, error) {
	app, inst, err := m.CheckCloneDockerApp(ctx, from, domain)
	if err != nil {
		return nil, err
	}
	appOps.Lock()
	defer appOps.Unlock()

	bg := context.WithoutCancel(ctx)
	var site *store.Site
	clone := &store.App{AppID: inst.AppID, Kind: inst.Kind, Version: inst.Version, Images: inst.Images,
		Inputs: maps.Clone(inst.Inputs), AdminEmail: inst.AdminEmail, ExtraPorts: map[int]int{}}
	ok, owned := false, false
	defer func() {
		if ok {
			return
		}
		lg.Logf("Removing what was created for the copy")
		if owned {
			m.removeInstance(bg, clone.Name, lg)
			m.store.DeleteApp(bg, clone.ID)
		}
		if site != nil {
			for i := 0; ; i++ {
				_, err := m.DeleteSite(bg, site.ID, site.Domain)
				if err == nil {
					break
				}
				if !errors.Is(err, ErrBusy) || i == 60 {
					lg.Logf("Warning: %v", err)
					break
				}
				time.Sleep(time.Second)
			}
		}
	}()

	used, err := m.usedAppPorts(ctx)
	if err != nil {
		return nil, err
	}
	pick := func() (int, error) {
		for range 50 {
			p := freePort()
			if !used[p] && m.checkApp(ctx, p, "", 0) == nil {
				used[p] = true
				return p, nil
			}
		}
		return 0, errors.New("no free port for the app")
	}
	port, err := pick()
	if err != nil {
		return nil, err
	}
	lg.Logf("Creating the website %s (closed until its app runs)", domain)
	if site, err = m.CreateSite(ctx, CreateRequest{Domain: domain, ProxyPort: port, Suspended: true}); err != nil {
		site = nil
		return nil, err
	}
	clone.Name, clone.SiteID, clone.Port = site.SysUser, site.ID, port
	for c := range inst.ExtraPorts {
		if clone.ExtraPorts[c], err = pick(); err != nil {
			return nil, err
		}
	}
	src, err := instanceDir(inst.Name)
	if err != nil {
		return nil, err
	}
	dst, err := instanceDir(clone.Name)
	if err != nil {
		return nil, err
	}
	if _, err := os.Lstat(dst); err == nil {
		return nil, fmt.Errorf("%s exists already: remove it first", dst)
	}
	if err := m.store.CreateApp(ctx, clone); err != nil {
		if errors.Is(err, store.ErrConflict) {
			return nil, usererr.New("app_installed", clone.Name)
		}
		return nil, err
	}
	owned = true
	// The copied data was made with the original's passwords.
	secrets, err := m.store.AppSecrets(ctx, inst.ID)
	if err != nil {
		return nil, err
	}
	if err := m.store.SetAppSecrets(ctx, clone.ID, secrets); err != nil {
		return nil, err
	}
	if err := mkdir(AppsDir, 0o700); err != nil {
		return nil, err
	}
	if err := mkdir(dst, 0o700); err != nil {
		return nil, err
	}
	if err := m.copyAppData(ctx, from, site, inst.Name, src, dst, lg); err != nil {
		return nil, err
	}

	env, err := m.appEnv(app, clone, site, secrets, false)
	if err != nil {
		return nil, err
	}
	// The original's images, also when the catalog has newer ones.
	pinned := m.pinnedApp(app, clone)
	if err := m.prepareInstance(ctx, pinned, env, lg); err != nil {
		return nil, err
	}
	if err := m.upInstance(ctx, pinned, clone, lg); err != nil {
		return nil, err
	}
	clone.State = "ready"
	if err := m.store.UpdateApp(ctx, clone); err != nil {
		return nil, err
	}
	lg.Logf("Opening the website")
	if site, err = m.openAppSite(ctx, site.ID, app, secrets); err != nil {
		return nil, err
	}
	ok = true
	lg.Logf("The copy of %s runs at http://%s/ once its domain points at this server, with the same accounts and passwords as %s.",
		app.Name, site.Domain, from.Domain)
	lg.Logf("If %s keeps its own address in its settings (not only in the panel's), change it there to the new one.", app.Name)
	return site, nil
}

// copyAppData copies an instance's data folder into a new one, with the
// original stopped meanwhile (and started again right after), then gives
// what belonged to the original website's user to the new one.
func (m *Manager) copyAppData(ctx context.Context, from, to *store.Site, name, src, dst string, lg Logger) error {
	data := filepath.Join(src, "data")
	entries, err := os.ReadDir(data)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && len(entries) == 0) {
		lg.Logf("%s has no data to copy", name)
		return nil
	}
	if err != nil {
		return err
	}
	status, _ := m.containerStates(ctx, false)
	running := status[name] == "running" || status[name] == "partial"
	if running {
		lg.Logf("Stopping %s while its data is copied", from.Domain)
		if _, err := m.run.Output(ctx, "docker", composeArgs(name, "stop", "--timeout", "30")...); err != nil {
			m.run.Output(context.WithoutCancel(ctx), "docker", composeArgs(name, "start")...)
			return fmt.Errorf("stop the app: %w", err)
		}
	}
	started := !running
	restart := func() {
		if started {
			return
		}
		started = true
		lg.Logf("Starting %s again", from.Domain)
		if _, err := m.run.Output(context.WithoutCancel(ctx), "docker", composeArgs(name, "start")...); err != nil {
			lg.Logf("Warning: %s did not start again: %v", from.Domain, err)
		}
	}
	defer restart()
	lg.Logf("Copying %s (%d MB)", data, dirSizeMB(data))
	t0 := time.Now()
	if _, err := m.run.Output(ctx, "cp", "-a", "--reflink=auto", "--", data, dst+"/"); err != nil {
		return fmt.Errorf("copy the data: %w", err)
	}
	restart()
	lg.Logf("Copied in %s", time.Since(t0).Round(time.Second))
	oldUID, oldGID, err := lookupIDs(from.SysUser)
	if err != nil {
		return err
	}
	uid, gid, err := lookupIDs(to.SysUser)
	if err != nil {
		return err
	}
	return chownOwned(filepath.Join(dst, "data"), oldUID, oldGID, uid, gid)
}

// chownOwned gives what belongs to oldUID (and oldGID) under dir to uid
// (and gid), without following symbolic links. dir is root's alone.
func chownOwned(dir string, oldUID, oldGID, uid, gid int) error {
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		fi, err := os.Lstat(p)
		if err != nil {
			return err
		}
		u, g := fsutil.Owner(fi)
		nu, ng := -1, -1
		if u == oldUID {
			nu = uid
		}
		if g == oldGID {
			ng = gid
		}
		if nu == -1 && ng == -1 {
			return nil
		}
		return os.Lchown(p, nu, ng)
	})
}

// FreeAppPort returns a port on 127.0.0.1 that no website, app or other
// program uses, for the copy of an app website.
func (m *Manager) FreeAppPort(ctx context.Context) (int, error) {
	used, err := m.usedAppPorts(ctx)
	if err != nil {
		return 0, err
	}
	for range 50 {
		if p := freePort(); !used[p] && m.checkApp(ctx, p, "", 0) == nil {
			return p, nil
		}
	}
	return 0, errors.New("no free port for the app")
}

// CheckAppPort checks that an app website may use port: a port the panel
// allows that no other website uses.
func (m *Manager) CheckAppPort(ctx context.Context, port int) error {
	return m.checkApp(ctx, port, "", 0)
}

// CheckRuntime checks that an app runtime ("node:22", "python"…) can run
// apps on this server.
func (m *Manager) CheckRuntime(rt string) error { return checkRuntime(rt) }
