package hosting

import (
	"context"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// redirectRe allows absolute http(s) URLs without characters Nginx would
// interpret (quotes, spaces, semicolons, $, braces).
var redirectRe = regexp.MustCompile(`^https?://[a-z0-9.-]{1,253}(:[0-9]{1,5})?(/[A-Za-z0-9._~%/-]*)?$`)

var authUserRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

func (m *Manager) authFile(domain string) string {
	return filepath.Join(m.web().snippetDir(), "auth", domain+".htpasswd")
}

// Access are a site's access settings.
type Access struct {
	Disabled     bool
	RedirectURL  string
	AuthUser     string // "" turns password protection off
	AuthPassword string // "" keeps the current password
}

// SetAccess suspends or resumes a site, redirects it, or protects it with
// a password (HTTP basic authentication; Let's Encrypt validation still
// works). If the web server rejects the result, the old configuration is
// put back.
func (m *Manager) SetAccess(ctx context.Context, id int64, a Access) (*store.Site, error) {
	if !m.Supported() {
		return nil, ErrUnsupported
	}
	if err := m.tryLock(); err != nil {
		return nil, err
	}
	defer m.ops.Unlock()
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return nil, err
	}
	old := *site
	old.Aliases = slices.Clone(site.Aliases)

	a.RedirectURL = strings.TrimRight(strings.TrimSpace(a.RedirectURL), "/")
	if a.RedirectURL != "" && !redirectRe.MatchString(a.RedirectURL) {
		return nil, usererr.New("invalid_redirect")
	}
	site.Disabled, site.RedirectURL = a.Disabled, a.RedirectURL

	switch {
	case a.AuthUser == "":
		site.AuthUser, site.AuthHash = "", ""
	case !authUserRe.MatchString(a.AuthUser):
		return nil, usererr.New("invalid_auth_user")
	case a.AuthPassword == "" && site.AuthHash == "":
		return nil, usererr.New("invalid_auth_password")
	case a.AuthPassword != "":
		if len(a.AuthPassword) < 8 || len(a.AuthPassword) > 72 {
			return nil, usererr.New("invalid_auth_password")
		}
		h, err := bcrypt.GenerateFromPassword([]byte(a.AuthPassword), 10)
		if err != nil {
			return nil, err
		}
		site.AuthUser, site.AuthHash = a.AuthUser, string(h)
	default:
		site.AuthUser = a.AuthUser
	}

	restore := func() {
		bg := context.WithoutCancel(ctx)
		m.writeAuth(&old)
		if m.writeVhost(bg, &old) == nil {
			m.reloadWeb(bg)
		}
	}
	if err := m.writeAuth(site); err != nil {
		restore()
		return nil, err
	}
	if err := m.writeVhost(ctx, site); err != nil {
		restore()
		return nil, err
	}
	if err := m.reloadWeb(ctx); err != nil {
		restore()
		return nil, err
	}
	if err := m.store.UpdateSite(ctx, site); err != nil {
		restore()
		return nil, err
	}
	return site, nil
}

// writeAuth writes (or removes) the site's password file. The web server's
// workers read it as www-data, so it is 0640 root:www-data. Nginx and
// Apache both accept the bcrypt ($2a$) hashes.
func (m *Manager) writeAuth(site *store.Site) error {
	path := m.authFile(site.Domain)
	if site.AuthUser == "" {
		return removeIfExists(path)
	}
	if err := mkdir(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := writeFile(path, []byte(site.AuthUser+":"+site.AuthHash+"\n"), 0o640); err != nil {
		return err
	}
	if g, err := user.LookupGroup("www-data"); err == nil {
		if gid, err := strconv.Atoi(g.Gid); err == nil {
			return os.Chown(path, 0, gid)
		}
	}
	return nil
}
