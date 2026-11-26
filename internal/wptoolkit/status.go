package wptoolkit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/nefocp/nefo/internal/store"
)

// Status is what the toolkit page shows about one WordPress site.
type Status struct {
	Version       string    `json:"version"`
	CoreUpdate    string    `json:"core_update"` // newest version offered, "" when up to date
	Plugins       int       `json:"plugins"`
	PluginUpdates int       `json:"plugin_updates"`
	Themes        int       `json:"themes"`
	ThemeUpdates  int       `json:"theme_updates"`
	Updates       []Pending `json:"updates"`
	Maintenance   bool      `json:"maintenance"`
	Debug         bool      `json:"debug"`
	AutoUpdates   bool      `json:"auto_updates"`
	URL           string    `json:"url"`
	// Error explains why the rest is missing, for example when the
	// database cannot be reached.
	Error string `json:"error,omitempty"`
}

// Pending is an available update.
type Pending struct {
	Type       string `json:"type"` // core, plugin or theme
	Name       string `json:"name"`
	Title      string `json:"title"`
	Version    string `json:"version"`
	NewVersion string `json:"new_version"`
}

// Extension is a plugin or theme as "wp plugin list" and "wp theme list"
// describe it.
type Extension struct {
	Name          string
	Title         string
	Status        string // active, inactive, active-network, must-use, dropin, parent
	Update        string // none, available, unavailable, "version higher than expected"
	Version       string
	UpdateVersion string
	AutoUpdate    string // on or off
}

// User is a WordPress account.
type User struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

const extFields = "--fields=name,title,status,update,version,update_version,auto_update"

// Status checks a site: its WordPress version and available updates, and
// its maintenance, debug and automatic update settings. Problems with the
// site itself, such as a database that is down, are reported in
// Status.Error; the error return is for problems with the panel.
func (t *Toolkit) Status(ctx context.Context, site *store.Site) (*Status, error) {
	if err := checkSite(site); err != nil {
		return nil, err
	}
	// The answer must come before the HTTP server's write timeout.
	ctx, cancel := context.WithTimeout(ctx, statusTimeout)
	defer cancel()
	if err := t.Ensure(ctx); err != nil {
		return nil, err
	}
	st := &Status{Updates: []Pending{}, Maintenance: t.maintenanceOn(site)}
	fail := func(err error) (*Status, error) {
		st.Error = ErrorText(err)
		if ctx.Err() != nil {
			st.Error = fmt.Sprintf("the check took longer than %d seconds", int(statusTimeout.Seconds()))
		}
		return st, nil
	}
	select {
	case t.reads <- struct{}{}:
	case <-ctx.Done():
		return fail(ctx.Err())
	}
	defer func() { <-t.reads }()

	// These two do not load WordPress.
	out, err := t.wp(ctx, site, nil, "core", "version")
	if err != nil {
		return fail(err)
	}
	st.Version = cleanVersion(out)
	cfg, err := t.config(ctx, site, "WP_DEBUG", "WP_AUTO_UPDATE_CORE")
	if err != nil {
		return fail(err)
	}
	st.Debug = truthy(cfg["WP_DEBUG"])
	st.AutoUpdates = string(cfg["WP_AUTO_UPDATE_CORE"]) == "true"

	// The rest loads WordPress and its database.
	if out, err = t.wp(ctx, site, nil, "option", "get", "home", "--format=json"); err != nil {
		return fail(err)
	}
	st.URL = parseURL(out)

	if out, err = t.wp(ctx, site, nil, "core", "check-update", "--force-check", "--format=json"); err != nil {
		return fail(err)
	}
	core, err := parseCoreUpdates(out)
	if err != nil {
		return fail(err)
	}
	if n := newestVersion(core); n != "" && compareVersions(n, st.Version) > 0 {
		st.CoreUpdate = n
		st.Updates = append(st.Updates, Pending{Type: "core", Name: "wordpress", Title: "WordPress", Version: st.Version, NewVersion: n})
	}

	for _, kind := range []string{"plugin", "theme"} {
		if out, err = t.wp(ctx, site, nil, kind, "list", extFields, "--format=json"); err != nil {
			return fail(err)
		}
		list, err := parseExtensions(out)
		if err != nil {
			return fail(fmt.Errorf("wp %s list: %w", kind, err))
		}
		total, updates := 0, 0
		for _, e := range list {
			if e.Status == "must-use" || e.Status == "dropin" {
				continue // not updated by wp-cli
			}
			total++
			if e.Update == "available" {
				updates++
				st.Updates = append(st.Updates, Pending{Type: kind, Name: e.Name, Title: e.Title, Version: e.Version, NewVersion: e.UpdateVersion})
			}
		}
		if kind == "plugin" {
			st.Plugins, st.PluginUpdates = total, updates
		} else {
			st.Themes, st.ThemeUpdates = total, updates
		}
	}
	return st, nil
}

// ErrorText is the message to show for an error: wp-cli's own words when
// wp-cli failed.
func ErrorText(err error) string {
	var ce *CLIError
	if errors.As(err, &ce) {
		return ce.Message
	}
	return err.Error()
}

// config reads constants from wp-config.php with "wp config list". Missing
// constants are left out of the map.
func (t *Toolkit) config(ctx context.Context, site *store.Site, names ...string) (map[string]json.RawMessage, error) {
	args := append([]string{"config", "list"}, names...)
	out, err := t.wp(ctx, site, nil, append(args, "--strict", "--fields=name,value,type", "--format=json")...)
	if hasMessage(err, "No matching entries found") {
		return map[string]json.RawMessage{}, nil
	}
	if err != nil {
		return nil, err
	}
	return parseConfig(out)
}

// ---- parsing wp-cli output ----

// decodeJSON decodes wp-cli's JSON output. wp-cli prints JSON on one line;
// anything a must-use plugin printed before it is skipped.
func decodeJSON(out string, v any) error {
	s := strings.TrimSpace(out)
	if err := json.Unmarshal([]byte(s), v); err == nil {
		return nil
	}
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		if err := json.Unmarshal([]byte(strings.TrimSpace(s[i+1:])), v); err == nil {
			return nil
		}
	}
	return errors.New("unexpected output from wp-cli")
}

// text decodes a JSON string, number, boolean or null as text, so a field
// of an unexpected type does not make the whole list unreadable.
type text string

func (f *text) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch x := v.(type) {
	case string:
		*f = text(clean(x))
	case float64:
		*f = text(strconv.FormatFloat(x, 'f', -1, 64))
	case bool:
		*f = text(strconv.FormatBool(x))
	default:
		*f = ""
	}
	return nil
}

// clean keeps text from a site short and free of control characters.
func clean(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	return truncate(strings.TrimSpace(s), 200)
}

var versionRe = regexp.MustCompile(`^[0-9]+(\.[0-9]+){0,3}(-[A-Za-z0-9.-]{1,30})?$`)

func cleanVersion(out string) string {
	v := strings.TrimSpace(out)
	if i := strings.LastIndexByte(v, '\n'); i >= 0 {
		v = strings.TrimSpace(v[i+1:])
	}
	if !versionRe.MatchString(v) {
		return ""
	}
	return v
}

// parseCoreUpdates reads "wp core check-update --format=json": a list of
// {"version","update_type","package_url"}, "[]" when WordPress is up to
// date (older wp-cli versions print a success message instead).
func parseCoreUpdates(out string) ([]string, error) {
	s := strings.TrimSpace(out)
	if s == "" || strings.Contains(s, "WordPress is at the latest version") {
		return nil, nil
	}
	var items []struct {
		Version text `json:"version"`
	}
	if err := decodeJSON(s, &items); err != nil {
		return nil, fmt.Errorf("wp core check-update: %w", err)
	}
	var versions []string
	for _, it := range items {
		if v := string(it.Version); versionRe.MatchString(v) {
			versions = append(versions, v)
		}
	}
	return versions, nil
}

// parseExtensions reads "wp plugin list" or "wp theme list" with extFields.
func parseExtensions(out string) ([]Extension, error) {
	var items []struct {
		Name          text `json:"name"`
		Title         text `json:"title"`
		Status        text `json:"status"`
		Update        text `json:"update"`
		Version       text `json:"version"`
		UpdateVersion text `json:"update_version"`
		AutoUpdate    text `json:"auto_update"`
	}
	if err := decodeJSON(out, &items); err != nil {
		return nil, err
	}
	list := make([]Extension, 0, len(items))
	for _, it := range items {
		list = append(list, Extension{
			Name: string(it.Name), Title: string(it.Title), Status: string(it.Status), Update: string(it.Update),
			Version: string(it.Version), UpdateVersion: string(it.UpdateVersion), AutoUpdate: string(it.AutoUpdate),
		})
	}
	return list, nil
}

// parseConfig reads "wp config list --format=json": the constants' values
// as JSON (true, "minor", 0…).
func parseConfig(out string) (map[string]json.RawMessage, error) {
	var items []struct {
		Name  string          `json:"name"`
		Value json.RawMessage `json:"value"`
		Type  string          `json:"type"`
	}
	if err := decodeJSON(out, &items); err != nil {
		return nil, fmt.Errorf("wp config list: %w", err)
	}
	m := map[string]json.RawMessage{}
	for _, it := range items {
		if it.Type == "constant" {
			m[it.Name] = it.Value
		}
	}
	return m, nil
}

// truthy tells whether a constant's value counts as true in PHP.
func truthy(raw json.RawMessage) bool {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return false
	}
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x != "" && x != "0"
	case float64:
		return x != 0
	}
	return false
}

// parseUsers reads "wp user list --fields=ID,user_login,user_email,display_name --format=json".
func parseUsers(out string) ([]User, error) {
	var items []struct {
		ID    text `json:"ID"`
		Login text `json:"user_login"`
		Email text `json:"user_email"`
		Name  text `json:"display_name"`
	}
	if err := decodeJSON(out, &items); err != nil {
		return nil, fmt.Errorf("wp user list: %w", err)
	}
	users := []User{}
	for _, it := range items {
		id, err := strconv.ParseInt(string(it.ID), 10, 64)
		if err != nil || id <= 0 || it.Login == "" {
			continue
		}
		users = append(users, User{ID: id, Login: string(it.Login), Email: string(it.Email), Name: string(it.Name)})
	}
	return users, nil
}

// parseURL reads "wp option get home --format=json". Only http and https
// addresses are kept: the page turns the value into a link.
func parseURL(out string) string {
	var s string
	if decodeJSON(out, &s) != nil {
		s = strings.TrimSpace(out)
	}
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || len(s) > 500 {
		return ""
	}
	return u.String()
}

// newestVersion returns the highest version of a list.
func newestVersion(versions []string) string {
	best := ""
	for _, v := range versions {
		if best == "" || compareVersions(v, best) > 0 {
			best = v
		}
	}
	return best
}

// compareVersions compares dotted version numbers such as 6.8 and 6.8.3;
// a suffix such as -RC1 is ignored.
func compareVersions(a, b string) int {
	pa, pb := versionParts(a), versionParts(b)
	for i := 0; i < max(len(pa), len(pb)); i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

func versionParts(v string) []int {
	v, _, _ = strings.Cut(v, "-")
	var parts []int
	for _, p := range strings.Split(v, ".") {
		n, _ := strconv.Atoi(p)
		parts = append(parts, n)
	}
	return parts
}

// ---- maintenance mode ----

// maintenanceFile is what the panel leaves in .maintenance. wp-cli writes
// the current time there, and WordPress ends maintenance mode 10 minutes
// after that time, because it is meant for updates. A time far in the
// future keeps the site in maintenance until it is turned off again; wp-cli
// still reads the file correctly.
const maintenanceFile = "<?php\n// Maintenance mode was turned on in the server panel. It stays on until it is turned off there.\n$upgrading = 4102444800;\n"

var upgradingRe = regexp.MustCompile(`(?i)upgrading\s*=\s*(\d+)\s*;`)

// maintenanceActive tells whether WordPress shows its maintenance page for
// a .maintenance file, with the same rule as wp_is_maintenance_mode().
func maintenanceActive(content []byte, now time.Time) bool {
	m := upgradingRe.FindSubmatch(content)
	if m == nil {
		// "$upgrading = time();" is a common way to keep it on for good.
		return bytes.Contains(content, []byte("time()"))
	}
	ts, err := strconv.ParseInt(string(m[1]), 10, 64)
	return err == nil && now.Unix()-ts < 600
}

func (t *Toolkit) maintenanceOn(site *store.Site) bool {
	b, err := readSiteFile(t.public(site), ".maintenance", 64<<10)
	return err == nil && maintenanceActive(b, t.now())
}

// readSiteFile reads a small regular file of a site, as root, inside an
// os.Root: a symlink cannot lead out of dir, and a FIFO or device the
// site's user put there is not opened.
func readSiteFile(dir, name string, limit int64) ([]byte, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	fi, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", name)
	}
	f, err := root.OpenFile(name, os.O_RDONLY|openNonBlock, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if fi, err := f.Stat(); err != nil || !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", name)
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("%s is larger than expected", name)
	}
	return b, nil
}
