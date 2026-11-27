package backup

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/nefocp/nefo/internal/hosting"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
)

// recreateSite creates a website that a backup holds but this server does
// not have: the same domain, aliases, PHP version (or the newest installed
// one), app and rewrite rules. The restore then puts back its files and
// creates its databases. What a backup does not hold (the certificate,
// cron jobs, SFTP accounts…) is not created.
func (m *Manager) recreateSite(ctx context.Context, man manifest, lg Logger) (*store.Site, error) {
	if m.hosting == nil {
		return nil, errors.New("this panel cannot create websites")
	}
	lg.Logf("The website %s does not exist on this server: creating it from the backup", man.Domain)
	st := m.hosting.Status(ctx)
	if !st.Ready {
		return nil, errors.New("no web server is installed yet: install one (Websites shows how), then restore again")
	}
	php := man.PHPVersion
	if php != "" {
		installed, newest := false, ""
		for _, p := range st.PHP {
			installed = installed || p.Version == php
			if newest == "" || newerVersion(p.Version, newest) {
				newest = p.Version
			}
		}
		switch {
		case installed:
		case newest != "":
			lg.Logf("Warning: PHP %s is not installed: the website gets PHP %s (install PHP %s under Software and switch the website to it if it needs that version)", php, newest, php)
			php = newest
		default:
			return nil, errors.New("the website uses PHP " + php + ", and no PHP is installed: install PHP under Software, then restore again")
		}
	}
	req := hosting.CreateRequest{Domain: man.Domain, Aliases: man.Aliases, PHPVersion: php}
	site, err := m.hosting.CreateSite(ctx, req)
	if err != nil && len(req.Aliases) > 0 {
		lg.Logf("Warning: the website could not be created with its aliases (%s): %v. Creating it without them", strings.Join(req.Aliases, ", "), err)
		req.Aliases = nil
		site, err = m.hosting.CreateSite(ctx, req)
	}
	if err != nil {
		return nil, err
	}
	rewrite := man.Rewrite
	if rewrite == "" && man.App == hosting.AppWordPress {
		rewrite = "wordpress" // older backups do not record it
	}
	if man.App != "" {
		site.App = man.App
		if err := m.store.UpdateSite(ctx, site); err != nil {
			return nil, err
		}
	}
	if rewrite != "" || man.NginxCustom != "" {
		if _, err := m.hosting.SetNginxConfig(ctx, site.ID, rewrite, man.NginxCustom); err != nil {
			lg.Logf("Warning: the rewrite rules or own web server settings of the backup were not accepted: %v", err)
		}
	}
	if php == "" {
		lg.Logf("Website %s created (static)", site.Domain)
	} else {
		lg.Logf("Website %s created with PHP %s", site.Domain, php)
	}
	lg.Logf("Not in the backup, so not created: the SSL certificate (get one under the website's SSL), " +
		"cron jobs, SFTP accounts, background programs, redirects, password protection and the website's other settings")
	return m.store.SiteByID(ctx, site.ID)
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
