package wptoolkit

import (
	"context"
	"errors"
	"strings"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/validate"
)

// Helpers for websites brought over from another server (package
// siteimport): reading and rewriting the database settings of a
// wp-config.php, and replacing the old server's folder in the database.

// ConfigDefine returns the value of the first active define of a constant
// with a plain string value in a wp-config.php, such as DB_NAME.
func ConfigDefine(cfg, name string) (string, bool) { return defineValue(cfg, name) }

// SetConfigDB points a wp-config.php at a database of this server: DB_NAME,
// DB_USER and, when password is not "", DB_PASSWORD are set, and DB_HOST
// becomes localhost. The values must be the panel's own (validated) names
// and password, which never need escaping.
func SetConfigDB(cfg, name, user, password string) (string, error) {
	if validate.DBName(name) != nil || validate.DBName(user) != nil || (password != "" && validate.DBPassword(password) != nil) {
		return "", errors.New("invalid database settings")
	}
	var err error
	for _, d := range []struct {
		name, value string
		required    bool
	}{
		{"DB_NAME", name, true}, {"DB_USER", user, true}, {"DB_PASSWORD", password, true}, {"DB_HOST", "localhost", false},
	} {
		if d.name == "DB_PASSWORD" && password == "" {
			continue
		}
		if cfg, err = setDefine(cfg, d.name, d.value, d.required); err != nil {
			return "", err
		}
	}
	return cfg, nil
}

// ReplaceInDB replaces from with to in every table of a WordPress site's
// database, also as JSON (with escaped slashes, as page builders save
// paths and addresses), with wp-cli as the site's user.
func (t *Toolkit) ReplaceInDB(ctx context.Context, site *store.Site, from, to string, lg Logger) error {
	if err := checkSite(site); err != nil {
		return err
	}
	if from == "" || from == to {
		return nil
	}
	if err := t.Ensure(ctx); err != nil {
		return err
	}
	pairs := [][2]string{{from, to}}
	if strings.Contains(from, "/") {
		pairs = append(pairs, [2]string{strings.ReplaceAll(from, "/", `\/`), strings.ReplaceAll(to, "/", `\/`)})
	}
	for _, p := range pairs {
		lg.Logf("Replacing %s with %s in the database", p[0], p[1])
		if err := t.stream(ctx, site, lg, "027", "search-replace", p[0], p[1],
			"--all-tables", "--precise", "--report-changed-only"); err != nil {
			return err
		}
	}
	return nil
}
