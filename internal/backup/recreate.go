package backup

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/siteclone"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
)

// siteManifest is the manifest of a website's backup: what the website
// needs to be created again (see recreateSite) besides its files and
// databases.
func siteManifest(site *store.Site, label string) manifest {
	man := manifest{Format: 1, Kind: KindSite, Domain: site.Domain, PHPVersion: site.PHPVersion, App: site.App,
		Aliases: site.Aliases, Rewrite: site.Rewrite, NginxCustom: site.NginxCustom, Label: label}
	switch {
	case site.ProxyURL != "":
		opts := site.ProxyOpts
		man.ProxyURL, man.ProxyOpts = site.ProxyURL, &opts
	case site.ProxyPort != 0:
		man.ProxyPort, man.AppCommand, man.AppRuntime, man.AppMemory = site.ProxyPort, site.AppCommand, site.AppRuntime, site.AppMemory
	}
	return man
}

// appStart is the program of an app website created again: it is set up
// once the website's files are back, as a copy of a site does.
type appStart struct {
	command, runtime string
	memory           int
}

// dockerSiteError is why a website that runs a Docker app is not created
// again: the app store creates the website with the app (its containers,
// their port and data), and the app's own backup (made with the website's)
// puts its data back once it is installed.
func dockerSiteError(man manifest) error {
	app := strings.TrimPrefix(man.App, "docker:")
	return fmt.Errorf("%s ran the Docker app %s, which a website backup cannot create again: install %s from the App store "+
		"for %s first, then restore the app's backup (made with the website's, listed under Apps in Backups)", man.Domain, app, app, man.Domain)
}

// recreateSite creates a website that a backup holds but this server does
// not have: the same domain, aliases, kind (PHP with the same version or
// the newest installed one, static, an app with its runtime, a reverse
// proxy with its address), app and rewrite rules. The restore then puts
// back its files and creates its databases; an app's program is set up
// after that (start). What a backup does not hold (the certificate, cron
// jobs, SFTP accounts…) is not created. A Docker app's website is not
// created (dockerSiteError).
func (m *Manager) recreateSite(ctx context.Context, man manifest, lg Logger) (site *store.Site, start *appStart, err error) {
	if strings.HasPrefix(man.App, "docker:") {
		return nil, nil, dockerSiteError(man)
	}
	if m.hosting == nil {
		return nil, nil, errors.New("this panel cannot create websites")
	}
	lg.Logf("The website %s does not exist on this server: creating it from the backup", man.Domain)
	st := m.hosting.Status(ctx)
	if !st.Ready {
		return nil, nil, errors.New("no web server is installed yet: install one (Websites shows how), then restore again")
	}
	req := hosting.CreateRequest{Domain: man.Domain, Aliases: man.Aliases}
	kind := ""
	switch {
	case man.ProxyURL != "":
		req.ProxyURL = man.ProxyURL
		if man.ProxyOpts != nil {
			req.ProxyOpts = *man.ProxyOpts
		}
		kind = "a reverse proxy to " + man.ProxyURL
	case man.ProxyPort != 0:
		name := siteclone.RuntimeName(man.AppRuntime)
		if man.AppRuntime != "" {
			if m.hosting.CheckRuntime(man.AppRuntime) != nil {
				return nil, nil, fmt.Errorf("the website's app runs on %s, which is not set up on this server: install it on the Software page, then restore again", name)
			}
		}
		if req.ProxyPort, err = m.appPort(ctx, man.ProxyPort, lg); err != nil {
			return nil, nil, err
		}
		start = &appStart{command: man.AppCommand, runtime: man.AppRuntime, memory: man.AppMemory}
		kind = "a " + name + " app on port " + strconv.Itoa(req.ProxyPort)
	default:
		if req.PHPVersion, err = m.recreatePHP(st, man.PHPVersion, lg); err != nil {
			return nil, nil, err
		}
		kind = "static"
		if req.PHPVersion != "" {
			kind = "PHP " + req.PHPVersion
		}
	}
	site, err = m.hosting.CreateSite(ctx, req)
	if err != nil && len(req.Aliases) > 0 {
		lg.Logf("Warning: the website could not be created with its aliases (%s): %v. Creating it without them", strings.Join(req.Aliases, ", "), err)
		req.Aliases = nil
		site, err = m.hosting.CreateSite(ctx, req)
	}
	if err != nil {
		return nil, nil, err
	}
	rewrite := man.Rewrite
	if rewrite == "" && man.App == hosting.AppWordPress {
		rewrite = "wordpress" // older backups do not record it
	}
	if man.App != "" {
		site.App = man.App
		if err := m.store.UpdateSite(ctx, site); err != nil {
			return nil, nil, err
		}
	}
	if rewrite != "" || man.NginxCustom != "" {
		if _, err := m.hosting.SetNginxConfig(ctx, site.ID, rewrite, man.NginxCustom); err != nil {
			lg.Logf("Warning: the rewrite rules or own web server settings of the backup were not accepted: %v", err)
		}
	}
	lg.Logf("Website %s created (%s)", site.Domain, kind)
	lg.Logf("Not in the backup, so not created: the SSL certificate (get one under the website's SSL), " +
		"cron jobs, SFTP accounts, background programs, redirects, password protection and the website's other settings")
	site, err = m.store.SiteByID(ctx, site.ID)
	return site, start, err
}

// recreatePHP is the PHP version a website created again gets: its own if
// it is installed, else the newest installed one ("" for a static site).
func (m *Manager) recreatePHP(st hosting.Status, php string, lg Logger) (string, error) {
	if php == "" {
		return "", nil
	}
	installed, newest := false, ""
	for _, p := range st.PHP {
		installed = installed || p.Version == php
		if newest == "" || newerVersion(p.Version, newest) {
			newest = p.Version
		}
	}
	switch {
	case installed:
		return php, nil
	case newest != "":
		lg.Logf("Warning: PHP %s is not installed: the website gets PHP %s (install PHP %s under Software and switch the website to it if it needs that version)", php, newest, php)
		return newest, nil
	}
	return "", errors.New("the website uses PHP " + php + ", and no PHP is installed: install PHP under Software, then restore again")
}

// appPort is the port of an app website created again: its old one when
// no other website or program uses it here, else a free one (the program
// gets it in PORT, as always).
func (m *Manager) appPort(ctx context.Context, port int, lg Logger) (int, error) {
	if m.hosting.CheckAppPort(ctx, port) == nil && portFree(port) {
		return port, nil
	}
	free, err := m.hosting.FreeAppPort(ctx)
	if err != nil {
		return 0, err
	}
	lg.Logf("Warning: port %d is used on this server: the app gets port %d (it is in the PORT variable; change the app's settings if it reads its port from elsewhere)", port, free)
	return free, nil
}

// portFree reports whether nothing listens on 127.0.0.1:port.
func portFree(port int) bool {
	l, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		return false
	}
	l.Close()
	return true
}

// startApp sets up the program of an app website created again, now that
// its files are back: its start command, runtime and memory limit, which
// write and start its service. A failure leaves the website without a
// program, as the log says.
func (m *Manager) startApp(ctx context.Context, site *store.Site, a *appStart, lg Logger) {
	if a.command == "" && a.runtime == "" && a.memory == 0 {
		return // an app whose program the panel does not run
	}
	lg.Logf("Setting up the app's program (%s)", siteclone.RuntimeName(a.runtime))
	_, _, err := m.hosting.UpdateSettings(ctx, site.ID, hosting.Settings{
		Aliases: site.Aliases, PHPMemory: site.PHPMemory, PHPUpload: site.PHPUpload, PHPTime: site.PHPTime,
		ProxyPort: site.ProxyPort, AppCommand: a.command, AppRuntime: a.runtime, AppMemory: a.memory,
	})
	if err != nil {
		lg.Logf("Warning: the app's program was not set up (%v): set its start command in the website's settings", err)
		return
	}
	if a.command != "" {
		lg.Logf("The app's program runs: %s", a.command)
	}
}

// newerVersion compares PHP versions such as "8.3" and "8.10".
func newerVersion(a, b string) bool {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		x, _ := strconv.Atoi(as[i])
		y, _ := strconv.Atoi(bs[i])
		if x != y {
			return x > y
		}
	}
	return len(as) > len(bs)
}

// usablePassword is a password from a backup that the panel accepts for a
// database user, or "" for a new random one.
func usablePassword(p string) string {
	if validate.DBPassword(p) != nil {
		return ""
	}
	return p
}

func userCodeOf(err error) string {
	var ue *usererr.Error
	if errors.As(err, &ue) {
		return ue.Code
	}
	return ""
}
