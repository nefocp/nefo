package hosting

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/geoip"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/version"
)

// Country blocking: a website refuses visitors from a list of countries
// (block), or lets in only those (allow). The countries come from DB-IP's
// free IP to Country Lite database (see package geoip), downloaded over
// HTTPS when a site first uses the feature and refreshed monthly by the
// maintenance loop. Servers where no site uses it download nothing and load
// nothing.
//
//   - Nginx: the built-in geo module. A file in the sites folder
//     (_geo.conf, http level) holds two geo blocks, IPv4 (in "ranges" or
//     CIDR mode, whichever is smaller) and IPv6 (CIDR), with only the
//     countries some site lists, so the memory follows the lists, not the
//     whole world. Each site's server block refuses with 403.
//   - Apache: mod_authz_host's "Require ip" with each listed country's
//     prefixes (a file per country), which Apache checks one by one: a
//     site's lists are limited to apacheGeoMax prefixes.
//   - OpenLiteSpeed: its own GeoIP2 support, with DB-IP's database in
//     MaxMind's format, and a rewrite rule per site.
//
// Never blocked: Let's Encrypt's challenges (/.well-known/acme-challenge/),
// the Git webhook, the site's own /403.html, reserved addresses (private
// networks, loopback: DB-IP's ZZ) and the server's own requests (a site
// calling itself). The panel itself has its own port and is not a website.
//
// Behind Cloudflare's proxy every request comes from Cloudflare: for those
// requests (and only those: the header could be forged otherwise) the
// visitor's address in CF-Connecting-IP is looked up instead.

const (
	GeoOff   = ""
	GeoBlock = "block"
	GeoAllow = "allow"

	maxGeoCountries = 100
	// apacheGeoMax is how many prefixes Apache may check for one site: it
	// tests them one by one on every request, and keeps about 350 bytes
	// per prefix in each process (China's 15,000 cost about 5 MB and
	// 0.2 ms per request in a sandbox; the United States' 272,000, 56 MB).
	apacheGeoMax = 50000

	geoCSVFile  = "dbip-country-lite.csv.gz"
	geoMMDBFile = "dbip-country-lite.mmdb"
	geoMetaFile = "dbip.json"
	geoCFFile   = "cloudflare.json"
)

// Replaceable in tests.
var (
	geoCSVURL        = geoip.CSVURL
	geoMMDBURL       = geoip.MMDBURL
	geoCloudflareURL = "https://api.cloudflare.com/client/v4/ips"
	geoHTTP          = &http.Client{Timeout: 3 * time.Minute}
	geoMinDownload   = 1 << 20 // the real files are 4 to 5 MB
)

// geoMeta describes the stored database.
type geoMeta struct {
	Month   string      `json:"month"` // DB-IP's edition, 2026-11
	Format  string      `json:"format"`
	Fetched time.Time   `json:"fetched"`
	Size    int64       `json:"size"`
	SHA256  string      `json:"sha256"`
	Stats   geoip.Stats `json:"stats"`
	// Tried is the last refresh attempt, successful or not.
	Tried time.Time `json:"tried"`
}

func (m *Manager) geoDir() string { return m.paths.GeoIP }

// geoFormat is the file the managed web server needs: OpenLiteSpeed reads
// MaxMind DB files, the panel turns the CSV into Nginx's and Apache's
// configuration.
func (m *Manager) geoFormat() string {
	if m.serverKind() == ServerOLS {
		return "mmdb"
	}
	return "csv"
}

func (m *Manager) geoDataFile(format string) string {
	if format == "mmdb" {
		return filepath.Join(m.geoDir(), geoMMDBFile)
	}
	return filepath.Join(m.geoDir(), geoCSVFile)
}

func (m *Manager) readGeoMeta() map[string]*geoMeta {
	out := map[string]*geoMeta{}
	b, err := os.ReadFile(filepath.Join(m.geoDir(), geoMetaFile))
	if err == nil {
		json.Unmarshal(b, &out)
	}
	return out
}

func (m *Manager) writeGeoMeta(meta map[string]*geoMeta) error {
	b, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(m.geoDir(), geoMetaFile), b, 0o644)
}

// geoReady reports whether the database the web server needs is stored.
func (m *Manager) geoReady() bool {
	meta := m.readGeoMeta()[m.geoFormat()]
	return meta != nil && fileExists(m.geoDataFile(m.geoFormat()))
}

// geoDownload fetches DB-IP's file of the month of now (or, early in a
// month, of the month before), checks it and stores it. It returns the new
// description.
func (m *Manager) geoDownload(ctx context.Context, format string, now time.Time) (*geoMeta, error) {
	tmpl := geoCSVURL
	if format == "mmdb" {
		tmpl = geoMMDBURL
	}
	var body []byte
	var month string
	for _, mo := range []string{now.UTC().Format("2006-01"), now.UTC().AddDate(0, -1, 0).Format("2006-01")} {
		b, status, err := geoFetch(ctx, fmt.Sprintf(tmpl, mo), geoip.MaxGzip)
		if err != nil {
			return nil, err
		}
		if status == http.StatusNotFound {
			continue // DB-IP publishes on the first days of the month
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("download of the country database: HTTP %d", status)
		}
		body, month = b, mo
		break
	}
	if body == nil {
		return nil, errors.New("download of the country database: no file for this month or the last")
	}
	if len(body) < geoMinDownload {
		return nil, fmt.Errorf("the country database download is too small (%d bytes)", len(body))
	}
	gz, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("the country database download is not gzip: %w", err)
	}
	meta := &geoMeta{Month: month, Format: format, Fetched: now.UTC()}
	if err := os.MkdirAll(m.geoDir(), 0o755); err != nil {
		return nil, err
	}
	os.Chmod(m.geoDir(), 0o755)
	switch format {
	case "mmdb":
		data, err := io.ReadAll(io.LimitReader(gz, geoip.MaxMMDB+1))
		if err != nil {
			return nil, fmt.Errorf("the country database download is damaged: %w", err)
		}
		if err := geoip.CheckMMDB(data); err != nil {
			return nil, err
		}
		meta.Size = int64(len(data))
		sum := sha256.Sum256(data)
		meta.SHA256 = hex.EncodeToString(sum[:])
		if err := writeFile(m.geoDataFile(format), data, 0o644); err != nil {
			return nil, err
		}
	default:
		st, err := geoip.Scan(gz, map[string]bool{}, func(geoip.Range) {})
		if err != nil {
			return nil, err
		}
		meta.Stats, meta.Size = st, int64(len(body))
		sum := sha256.Sum256(body)
		meta.SHA256 = hex.EncodeToString(sum[:])
		if err := writeFile(m.geoDataFile(format), body, 0o644); err != nil {
			return nil, err
		}
	}
	return meta, nil
}

// geoFetch downloads url over HTTPS, at most limit bytes.
func geoFetch(ctx context.Context, url string, limit int64) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", version.AppName+"/"+version.Version)
	resp, err := geoHTTP.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("download of %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, nil
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, 0, fmt.Errorf("download of %s: %w", url, err)
	}
	if int64(len(b)) > limit {
		return nil, 0, fmt.Errorf("download of %s: larger than %d MB", url, limit>>20)
	}
	return b, resp.StatusCode, nil
}

// ensureGeoDB downloads the database the web server needs if it is not
// stored yet (the first site to use country blocking).
func (m *Manager) ensureGeoDB(ctx context.Context) error {
	m.geoMu.Lock()
	defer m.geoMu.Unlock()
	if m.geoReady() {
		return nil
	}
	format := m.geoFormat()
	meta, err := m.geoDownload(ctx, format, m.now())
	if err != nil {
		m.log.Warn("country database", "err", err)
		return usererr.New("geo_download_failed", err.Error())
	}
	meta.Tried = meta.Fetched
	all := m.readGeoMeta()
	all[format] = meta
	if err := m.writeGeoMeta(all); err != nil {
		return err
	}
	m.refreshCloudflare(ctx)
	return nil
}

// geoCloudflare returns Cloudflare's prefixes: the list fetched with the
// database, or the one built into the panel.
func (m *Manager) geoCloudflare() []string {
	var saved []string
	if b, err := os.ReadFile(filepath.Join(m.geoDir(), geoCFFile)); err == nil && json.Unmarshal(b, &saved) == nil {
		if ok, err := geoip.CheckPrefixes(saved); err == nil {
			return ok
		}
	}
	return slices.Clone(geoip.Cloudflare)
}

// refreshCloudflare fetches Cloudflare's current list of addresses. On any
// problem the old (or built-in) list stays.
func (m *Manager) refreshCloudflare(ctx context.Context) {
	b, status, err := geoFetch(ctx, geoCloudflareURL, 1<<20)
	if err != nil || status != http.StatusOK {
		m.log.Warn("cloudflare addresses", "err", err, "status", status)
		return
	}
	var res struct {
		Success bool `json:"success"`
		Result  struct {
			V4 []string `json:"ipv4_cidrs"`
			V6 []string `json:"ipv6_cidrs"`
		} `json:"result"`
	}
	if json.Unmarshal(b, &res) != nil || !res.Success {
		return
	}
	list, err := geoip.CheckPrefixes(append(res.Result.V4, res.Result.V6...))
	if err != nil || len(res.Result.V4) < 5 || len(res.Result.V6) < 2 {
		m.log.Warn("cloudflare addresses", "err", err)
		return
	}
	out, _ := json.Marshal(list)
	writeFile(filepath.Join(m.geoDir(), geoCFFile), out, 0o644)
}

// RefreshGeoDB replaces the database with DB-IP's new monthly edition, once
// it is out, and rewrites the web server's country files. It runs from the
// hourly maintenance loop and does nothing while no site uses country
// blocking; it tries at most every 12 hours.
func (m *Manager) RefreshGeoDB(ctx context.Context) {
	if !m.Supported() || !fileExists(m.web().hookFile()) {
		return
	}
	prot, err := m.store.SiteProtects(ctx)
	if err != nil {
		return
	}
	inUse := false
	for _, p := range prot {
		inUse = inUse || p.GeoOn()
	}
	if !inUse {
		return
	}
	m.geoMu.Lock()
	format := m.geoFormat()
	all := m.readGeoMeta()
	cur := all[format]
	now := m.now()
	if cur != nil && (cur.Month == now.UTC().Format("2006-01") || now.Sub(cur.Tried) < 12*time.Hour) {
		m.geoMu.Unlock()
		return
	}
	meta, err := m.geoDownload(ctx, format, now)
	if err == nil && cur != nil && meta.Month == cur.Month {
		// Last month's file again: this month's is not out yet.
		cur.Tried = now
		m.writeGeoMeta(all)
		m.geoMu.Unlock()
		return
	}
	if err != nil {
		m.log.Warn("country database refresh", "err", err)
		if cur != nil {
			cur.Tried = now
			m.writeGeoMeta(all)
		}
		m.geoMu.Unlock()
		return
	}
	meta.Tried = now
	all[format] = meta
	m.writeGeoMeta(all)
	m.refreshCloudflare(ctx)
	m.geoMu.Unlock()

	m.ops.Lock()
	defer m.ops.Unlock()
	if err := m.applyGeo(ctx, nil); err != nil {
		m.log.Warn("country files after the database refresh", "err", err)
		return
	}
	m.log.Info("country database updated", "month", meta.Month)
}

// ---- settings ----

// CountryBlock are a site's country blocking settings.
type CountryBlock struct {
	Mode      string   `json:"mode"` // "", "block" or "allow"
	Countries []string `json:"countries"`
}

// GeoInfo describes country blocking for the UI.
type GeoInfo struct {
	CountryBlock
	// Supported is false when the web server cannot do it well; Reason is
	// then a usererr code.
	Supported bool   `json:"supported"`
	Reason    string `json:"reason,omitempty"`
	Server    string `json:"server"`
	// The database: its edition and when it was fetched; empty before the
	// first use.
	Month       string    `json:"month,omitempty"`
	Fetched     time.Time `json:"fetched,omitzero"`
	Attribution string    `json:"attribution"`
	AttribURL   string    `json:"attribution_url"`
	License     string    `json:"license"`
	// Sites is how many websites use country blocking; MemoryKB estimates
	// what the lists cost Nginx (per process, shared after a fork).
	Sites    int   `json:"sites"`
	MemoryKB int64 `json:"memory_kb,omitempty"`
	// Prefixes counts the address prefixes of this site's lists (Apache's
	// limit is MaxPrefixes).
	Prefixes    int `json:"prefixes,omitempty"`
	MaxPrefixes int `json:"max_prefixes,omitempty"`
}

// normalizeCountries checks and sorts country codes.
func normalizeCountries(in []string) ([]string, error) {
	out := []string{}
	for _, c := range in {
		c = strings.ToUpper(strings.TrimSpace(c))
		if c == "" {
			continue
		}
		if !geoip.IsCountry(c) {
			return nil, usererr.New("invalid_country", c)
		}
		if !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	if len(out) > maxGeoCountries {
		return nil, usererr.New("too_many_countries")
	}
	sort.Strings(out)
	return out, nil
}

// GeoStatus returns a site's country blocking settings and state.
func (m *Manager) GeoStatus(ctx context.Context, id int64) (*GeoInfo, error) {
	if _, err := m.store.SiteByID(ctx, id); err != nil {
		return nil, err
	}
	p, err := m.store.SiteProtect(ctx, id)
	if err != nil {
		return nil, err
	}
	info := &GeoInfo{CountryBlock: CountryBlock{Mode: p.GeoMode, Countries: p.GeoCountries},
		Supported: true, Server: m.serverKind(),
		Attribution: geoip.Attribution, AttribURL: geoip.AttributionURL, License: geoip.License}
	if info.Countries == nil {
		info.Countries = []string{}
	}
	if meta := m.readGeoMeta()[m.geoFormat()]; meta != nil {
		info.Month, info.Fetched = meta.Month, meta.Fetched
	}
	all, err := m.store.SiteProtects(ctx)
	if err != nil {
		return nil, err
	}
	for _, x := range all {
		if x.GeoOn() {
			info.Sites++
		}
	}
	switch m.serverKind() {
	case ServerNginx:
		if b, err := os.ReadFile(m.nginxGeoConf()); err == nil {
			info.MemoryKB = geoHeaderInt(b, "memory_kb")
		}
	case ServerApache:
		info.MaxPrefixes = apacheGeoMax
		if p.GeoOn() {
			info.Prefixes = m.apacheGeoPrefixes(geoSiteCountries(p))
		}
	case "":
		info.Supported, info.Reason = false, "not_ready"
	}
	return info, nil
}

// geoSiteCountries are the countries whose lists a site needs: the listed
// ones, and the reserved addresses (ZZ) for an allow list.
func geoSiteCountries(p *store.SiteProtect) []string {
	cc := slices.Clone(p.GeoCountries)
	if p.GeoMode == GeoAllow {
		cc = append(cc, geoip.Reserved)
	}
	sort.Strings(cc)
	return cc
}

// SetCountryBlock changes a site's country blocking. The database is
// downloaded first if this is its first use. If the web server rejects the
// result, the old settings are put back.
func (m *Manager) SetCountryBlock(ctx context.Context, id int64, cb CountryBlock) (*GeoInfo, error) {
	if !m.Supported() {
		return nil, ErrUnsupported
	}
	if !fileExists(m.web().hookFile()) {
		return nil, ErrNotReady
	}
	if cb.Mode != GeoOff && cb.Mode != GeoBlock && cb.Mode != GeoAllow {
		return nil, usererr.New("invalid_geo_mode")
	}
	countries, err := normalizeCountries(cb.Countries)
	if err != nil {
		return nil, err
	}
	if cb.Mode == GeoOff {
		countries = nil
	} else if len(countries) == 0 {
		return nil, usererr.New("no_countries")
	}
	if _, err := m.store.SiteByID(ctx, id); err != nil {
		return nil, err
	}
	if cb.Mode != GeoOff {
		if err := m.ensureGeoDB(ctx); err != nil {
			return nil, err
		}
	}
	if err := m.tryLock(); err != nil {
		return nil, err
	}
	defer m.ops.Unlock()
	site, err := m.store.SiteByID(ctx, id)
	if err != nil {
		return nil, err
	}
	p, err := m.store.SiteProtect(ctx, id)
	if err != nil {
		return nil, err
	}
	old := *p
	p.GeoMode, p.GeoCountries = cb.Mode, countries
	if m.serverKind() == ServerApache && p.GeoOn() {
		counts := m.apacheGeoCounts(geoSiteCountries(p))
		n := 0
		for _, c := range counts {
			n += c
		}
		if n > apacheGeoMax {
			return nil, usererr.New("geo_too_large", geoTooLarge(counts))
		}
	}
	if err := m.store.SetSiteProtect(ctx, p); err != nil {
		return nil, err
	}
	restore := func() {
		bg := context.WithoutCancel(ctx)
		if err := m.store.SetSiteProtect(bg, &old); err != nil {
			m.log.Warn("restore country blocking", "domain", site.Domain, "err", err)
		}
		if err := m.applyGeo(bg, site); err != nil {
			m.log.Warn("restore country blocking", "domain", site.Domain, "err", err)
		}
	}
	if err := m.applyGeo(ctx, site); err != nil {
		restore()
		return nil, err
	}
	return m.GeoStatus(ctx, id)
}

// applyGeo brings the web server's country files in line with the stored
// settings, rewrites the vhost of site (if given; nil rewrites every site
// that uses country blocking) and reloads. The caller holds ops.
func (m *Manager) applyGeo(ctx context.Context, site *store.Site) error {
	ws := m.web()
	prot, err := m.store.SiteProtects(ctx)
	if err != nil {
		return err
	}
	shared, remove, err := m.geoFiles(ctx, prot)
	if err != nil {
		return err
	}
	type saved struct {
		data    []byte
		existed bool
	}
	backup := map[string]saved{}
	keep := func(path string) {
		if _, ok := backup[path]; ok {
			return
		}
		old, err := os.ReadFile(path)
		backup[path] = saved{old, err == nil}
	}
	undo := func() {
		for path, s := range backup {
			if s.existed {
				writeFile(path, s.data, 0o644)
			} else {
				os.Remove(path)
			}
		}
	}
	for path, data := range shared {
		if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, data) {
			continue
		}
		keep(path)
		if err := mkdir(filepath.Dir(path), 0o755); err != nil {
			undo()
			return err
		}
		if err := writeFile(path, data, 0o644); err != nil {
			undo()
			return err
		}
		// Nginx keeps a binary copy of a large ranges list next to it.
		os.Remove(path + ".bin")
	}
	var sites []*store.Site
	if site != nil {
		sites = []*store.Site{site}
	} else {
		ids := map[int64]bool{}
		for _, p := range prot {
			ids[p.SiteID] = p.GeoOn()
		}
		all, err := m.store.ListSites(ctx)
		if err != nil {
			undo()
			return err
		}
		for _, s := range all {
			if ids[s.ID] {
				sites = append(sites, s)
			}
		}
	}
	for _, s := range sites {
		data, err := m.vhostBytes(ctx, s)
		if err != nil {
			undo()
			return err
		}
		keep(m.vhostFile(s.Domain))
		if err := writeFile(m.vhostFile(s.Domain), data, 0o644); err != nil {
			undo()
			return err
		}
	}
	// Files no site needs any more go last: the vhosts no longer use them.
	for _, path := range remove {
		if fileExists(path) {
			keep(path)
			os.Remove(path)
			os.Remove(path + ".bin")
		}
	}
	if len(backup) == 0 {
		return nil
	}
	if m.serverKind() == ServerApache && len(shared) > 0 {
		if err := m.apacheEnableRemoteIP(ctx); err != nil {
			undo()
			return err
		}
	}
	if err := ws.reload(ctx); err != nil {
		undo()
		ws.reload(context.WithoutCancel(ctx))
		return err
	}
	return nil
}

// geoFiles renders the web server's shared country files for the stored
// settings, and lists those of its files no site needs.
func (m *Manager) geoFiles(ctx context.Context, prot []*store.SiteProtect) (map[string][]byte, []string, error) {
	want := map[string]bool{}
	for _, p := range prot {
		if p.GeoOn() {
			for _, c := range geoSiteCountries(p) {
				want[c] = true
			}
		}
	}
	ready := m.geoReady()
	switch m.serverKind() {
	case ServerNginx:
		files := []string{m.nginxGeoConf(), m.nginxGeoList("v4"), m.nginxGeoList("v6")}
		if len(want) == 0 || !ready {
			return nil, files, nil
		}
		out, err := m.nginxGeoFiles(want)
		return out, nil, err
	case ServerApache:
		existing, _ := filepath.Glob(filepath.Join(m.apacheGeoDir(), "*.conf"))
		if !ready {
			return nil, existing, nil
		}
		out, err := m.apacheGeoFiles(want)
		if err != nil {
			return nil, nil, err
		}
		var remove []string
		for _, f := range existing {
			if _, ok := out[f]; !ok {
				remove = append(remove, f)
			}
		}
		return out, remove, nil
	case ServerOLS:
		// The server's file loads the database while a site uses it.
		base, err := m.web().baseFiles(ctx)
		if err != nil {
			return nil, nil, err
		}
		hook := m.web().hookFile()
		return map[string][]byte{hook: base[hook]}, nil, nil
	}
	return nil, nil, nil
}

// geoInUse reports whether some site uses country blocking (OpenLiteSpeed's
// server file loads the database only then).
func (m *Manager) geoInUse(ctx context.Context) bool {
	if m.store == nil {
		return false
	}
	prot, err := m.store.SiteProtects(ctx)
	if err != nil {
		return false
	}
	for _, p := range prot {
		if p.GeoOn() {
			return true
		}
	}
	return false
}

// geoHeader is the first line of a generated list: what it was made from,
// so an unchanged one is not rebuilt.
func geoHeader(month string, countries []string, extra ...string) string {
	return fmt.Sprintf("# Managed by %s. Do not edit: the panel rewrites this file.\n# data=%s countries=%s %s\n",
		version.AppName, month, strings.Join(countries, ","), strings.Join(extra, " "))
}

func geoHeaderInt(b []byte, key string) int64 {
	line, _, _ := bytes.Cut(b[min(len(b), bytes.IndexByte(b, '\n')+1):], []byte("\n"))
	for _, f := range strings.Fields(string(line)) {
		if v, ok := strings.CutPrefix(f, key+"="); ok {
			var n int64
			fmt.Sscan(v, &n)
			return n
		}
	}
	return 0
}

// openGeoCSV opens the stored CSV.
func (m *Manager) openGeoCSV() (io.ReadCloser, *geoMeta, error) {
	meta := m.readGeoMeta()["csv"]
	if meta == nil {
		return nil, nil, errors.New("the country database is not downloaded")
	}
	f, err := os.Open(m.geoDataFile("csv"))
	if err != nil {
		return nil, nil, err
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	return struct {
		io.Reader
		io.Closer
	}{gz, f}, meta, nil
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ---- Nginx ----

func (m *Manager) nginxGeoConf() string {
	return filepath.Join(m.server(ServerNginx).sitesDir(), "_geo.conf")
}

func (m *Manager) nginxGeoList(name string) string {
	return filepath.Join(m.server(ServerNginx).snippetDir(), "geo", name+".conf")
}

var nginxGeoTmpl = mustTemplate("nginx-geo", `{{.Header}}# Country blocking (Websites → Country blocking), loaded while a website uses
# it. {{.Attribution}} ({{.Month}}, {{.License}}).

# Requests from Cloudflare's proxy are looked up by the visitor's address in
# CF-Connecting-IP, all others by the address they come from.
geo $panel_geo_cf {
    default 0;
{{- range .Cloudflare}}
    {{.}} 1;
{{- end}}
}
map $panel_geo_cf $panel_geo_addr {
    1       $http_cf_connecting_ip;
    default $remote_addr;
}
# IPv4 in "ranges" or CIDR mode, whichever takes less memory for these
# countries (see geoip.NginxRangesMode); IPv6 as prefixes.
geo $panel_geo_addr $panel_geo_v4 {
{{- if .Ranges}}
    ranges;
{{- end}}
    default "";
    include {{.V4}};
}
geo $panel_geo_addr $panel_geo_v6 {
    default "";
    include {{.V6}};
}
# A website calling itself (WordPress's cron, say) is never blocked.
map "$remote_addr $server_addr" $panel_geo_self {
    "~^(\S+) \1$" 1;
    default       0;
}
# The visitor's country, ZZ for reserved addresses and the server itself,
# "" for the countries no website lists.
map "$panel_geo_self$panel_geo_v4$panel_geo_v6" $panel_country {
    "~^1"                       ZZ;
    "~^0(?<panel_cc>[A-Z]{2})$" $panel_cc;
    default                     "";
}
`)

// nginxGeoFiles renders _geo.conf and its two lists for the countries in
// want. Unchanged lists (same database, same countries) are read back
// instead of rebuilt.
func (m *Manager) nginxGeoFiles(want map[string]bool) (map[string][]byte, error) {
	countries := sortedKeys(want)
	meta := m.readGeoMeta()["csv"]
	if meta == nil {
		return nil, errors.New("the country database is not downloaded")
	}
	v4Path, v6Path := m.nginxGeoList("v4"), m.nginxGeoList("v6")
	cf := m.geoCloudflare()
	var v4b, v6b []byte
	var mem int64
	ranges := false
	prefix := geoHeader(meta.Month, countries)
	if old4, err := os.ReadFile(v4Path); err == nil && bytes.HasPrefix(old4, []byte(prefix)) {
		if old6, err := os.ReadFile(v6Path); err == nil && bytes.HasPrefix(old6, []byte(prefix)) {
			v4b, v6b = old4, old6
			if conf, err := os.ReadFile(m.nginxGeoConf()); err == nil {
				mem = geoHeaderInt(conf, "memory_kb") << 10
				ranges = geoHeaderInt(conf, "ranges") == 1
			}
		}
	}
	if v4b == nil {
		r, _, err := m.openGeoCSV()
		if err != nil {
			return nil, err
		}
		v4, v6, _, err := geoip.Subset(r, want)
		r.Close()
		if err != nil {
			return nil, err
		}
		if ranges = geoip.NginxRangesMode(v4); ranges {
			v4b = append([]byte(prefix), geoip.NginxRanges(v4)...)
		} else {
			v4b = append([]byte(prefix), geoip.NginxCIDRs(v4)...)
		}
		v6b = append([]byte(prefix), geoip.NginxCIDRs(v6)...)
		mem = geoip.NginxMemory(append(v4, v6...))
	}
	for _, p := range cf {
		if !safeCIDR(p) {
			return nil, fmt.Errorf("refusing to render the Cloudflare address %q", p)
		}
	}
	conf, err := render(nginxGeoTmpl, struct {
		Header, Attribution, License, Month, V4, V6 string
		Cloudflare                                  []string
		Ranges                                      bool
	}{geoHeader(meta.Month, countries, fmt.Sprintf("memory_kb=%d ranges=%d", mem>>10, boolInt(ranges))), geoip.Attribution, geoip.License,
		meta.Month, v4Path, v6Path, cf, ranges})
	if err != nil {
		return nil, err
	}
	return map[string][]byte{m.nginxGeoConf(): conf, v4Path: v4b, v6Path: v6b}, nil
}

// safeCIDR allows the characters of an address prefix.
func safeCIDR(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r == '.' || r == ':' || r == '/') {
			return false
		}
	}
	return true
}

// ---- Apache ----

func (m *Manager) apacheGeoDir() string {
	return filepath.Join(m.server(ServerApache).snippetDir(), "geo")
}

func (m *Manager) apacheGeoFile(cc string) string {
	return filepath.Join(m.apacheGeoDir(), cc+".conf")
}

// apacheGeoFiles renders one file of "Require ip" lines per country in
// want; unchanged ones are read back.
func (m *Manager) apacheGeoFiles(want map[string]bool) (map[string][]byte, error) {
	out := map[string][]byte{}
	meta := m.readGeoMeta()["csv"]
	if meta == nil {
		if len(want) == 0 {
			return out, nil
		}
		return nil, errors.New("the country database is not downloaded")
	}
	missing := map[string]bool{}
	for cc := range want {
		prefix := geoHeader(meta.Month, []string{cc})
		if old, err := os.ReadFile(m.apacheGeoFile(cc)); err == nil && bytes.HasPrefix(old, []byte(prefix)) {
			out[m.apacheGeoFile(cc)] = old
		} else {
			missing[cc] = true
		}
	}
	if len(missing) == 0 {
		return out, nil
	}
	r, _, err := m.openGeoCSV()
	if err != nil {
		return nil, err
	}
	v4, v6, _, err := geoip.Subset(r, missing)
	r.Close()
	if err != nil {
		return nil, err
	}
	by := map[string][]geoip.Range{}
	for _, x := range append(v4, v6...) {
		by[x.CC] = append(by[x.CC], x)
	}
	for cc := range missing {
		lines, n := geoip.ApacheRequire(by[cc])
		if n == 0 {
			// A country without addresses: a line that matches nothing
			// keeps the file valid.
			lines = []byte("Require ip 192.0.2.255/32\n")
		}
		out[m.apacheGeoFile(cc)] = append([]byte(geoHeader(meta.Month, []string{cc}, fmt.Sprintf("prefixes=%d", n))), lines...)
	}
	return out, nil
}

// apacheGeoPrefixes counts the prefixes of the countries' lists, building
// the lists that are missing.
func (m *Manager) apacheGeoPrefixes(countries []string) int {
	n := 0
	for _, c := range m.apacheGeoCounts(countries) {
		n += c
	}
	return n
}

// apacheGeoCounts returns the prefixes of each country's list.
func (m *Manager) apacheGeoCounts(countries []string) map[string]int {
	want := map[string]bool{}
	for _, c := range countries {
		want[c] = true
	}
	out := map[string]int{}
	files, err := m.apacheGeoFiles(want)
	if err != nil {
		return out
	}
	for _, c := range countries {
		out[c] = int(geoHeaderInt(files[m.apacheGeoFile(c)], "prefixes"))
	}
	return out
}

// geoTooLarge names the countries an Apache list must do without to fit
// apacheGeoMax, the largest first ("US:271881,DE:55789"), for the UI.
func geoTooLarge(counts map[string]int) string {
	total := 0
	var cc []string
	for c, n := range counts {
		total += n
		if c != geoip.Reserved {
			cc = append(cc, c)
		}
	}
	sort.Slice(cc, func(i, j int) bool {
		return counts[cc[i]] > counts[cc[j]] || (counts[cc[i]] == counts[cc[j]] && cc[i] < cc[j])
	})
	var out []string
	for _, c := range cc {
		if total <= apacheGeoMax {
			break
		}
		out = append(out, fmt.Sprintf("%s:%d", c, counts[c]))
		total -= counts[c]
	}
	return strings.Join(out, ",")
}

// apacheEnableRemoteIP turns on mod_remoteip (part of the apache2
// package), which takes the visitor's address from Cloudflare's header.
func (m *Manager) apacheEnableRemoteIP(ctx context.Context) error {
	if fileExists(filepath.Join(m.paths.Apache, "mods-enabled", "remoteip.load")) {
		return nil
	}
	if out, err := m.run.Output(ctx, "a2enmod", "-q", "remoteip"); err != nil {
		return fmt.Errorf("turn on the Apache module remoteip: %s", run.Tail(out, 3))
	}
	return nil
}

// ---- the site's part ----

// geoVhost fills in a site's country blocking for its vhost, if it uses it
// and the web server has the data.
func (m *Manager) geoVhost(ctx context.Context, site *store.Site, d *vhostData) {
	if m.store == nil {
		return
	}
	p, err := m.store.SiteProtect(ctx, site.ID)
	if err != nil || !p.GeoOn() || !m.geoReady() {
		return
	}
	countries := slices.Clone(p.GeoCountries)
	if p.GeoMode == GeoAllow {
		countries = append(countries, geoip.Reserved)
		sort.Strings(countries)
	}
	d.GeoMode, d.GeoRegex = p.GeoMode, strings.Join(countries, "|")
	switch m.serverKind() {
	case ServerApache:
		for _, c := range countries {
			d.GeoFiles = append(d.GeoFiles, m.apacheGeoFile(c))
		}
		d.GeoTrusted = m.geoCloudflare()
	case ServerOLS:
		d.GeoCF = addrRegex(m.geoCloudflare())
		d.GeoSelf = addrRegex(serverAddrs())
	}
}

// serverAddrs returns the addresses of the server's network interfaces
// (not the loopback interface), as single-address prefixes: a site calling
// itself comes from one of them.
var serverAddrs = func() []string {
	var out []string
	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			n, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip, ok := netip.AddrFromSlice(n.IP)
			if !ok || !ip.IsValid() || ip.Unmap().IsLoopback() || ip.Unmap().IsLinkLocalUnicast() {
				continue
			}
			ip = ip.Unmap()
			if p := netip.PrefixFrom(ip, ip.BitLen()).String(); !slices.Contains(out, p) {
				out = append(out, p)
			}
		}
	}
	sort.Strings(out)
	return out
}

// addrRegex returns a regular expression that matches the text form of
// the addresses in the prefixes, as a web server writes REMOTE_ADDR. IPv6
// prefixes are written as their leading groups, so a prefix whose leading
// groups hold a zero (which the text form may leave out) is skipped.
func addrRegex(prefixes []string) string {
	var alts []string
	for _, s := range prefixes {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			continue
		}
		p = p.Masked()
		if r := prefixRegex(p); r != "" && !slices.Contains(alts, r) {
			alts = append(alts, r)
		}
	}
	return strings.Join(alts, "|")
}

func prefixRegex(p netip.Prefix) string {
	a, bits := p.Addr(), p.Bits()
	if bits == a.BitLen() {
		return regexp.QuoteMeta(a.String())
	}
	if a.Is4() {
		o := a.As4()
		var parts []string
		for i := 0; i < 4; i++ {
			lo, hi := int(o[i]), int(o[i])
			switch left := bits - 8*i; {
			case left >= 8:
			case left <= 0:
				lo, hi = 0, 255
			default:
				hi = lo | (1<<(8-left) - 1)
			}
			if lo == 0 && hi == 255 {
				parts = append(parts, "[0-9]+")
			} else {
				parts = append(parts, digitRange(lo, hi, 10))
			}
		}
		return strings.Join(parts, `\.`)
	}
	if bits < 16 || bits > 64 {
		return ""
	}
	b := a.As16()
	var parts []string
	for i := 0; i*16 < bits; i++ {
		v := int(b[2*i])<<8 | int(b[2*i+1])
		lo, hi := v, v
		if left := bits - 16*i; left < 16 {
			hi = lo | (1<<(16-left) - 1)
		}
		if lo < 0x1000 {
			return "" // leading zeros or a zero group: the text form varies
		}
		parts = append(parts, digitRange(lo, hi, 16))
	}
	return strings.Join(parts, ":") + ":[0-9a-f:.]*"
}

// digitRange returns a regular expression for the numbers lo to hi in base
// 10 or 16, written without leading zeros.
func digitRange(lo, hi, base int) string {
	alts := rangeAlts(strconv.FormatInt(int64(lo), base), strconv.FormatInt(int64(hi), base), base)
	if len(alts) == 1 {
		return alts[0]
	}
	return "(?:" + strings.Join(alts, "|") + ")"
}

func rangeAlts(lo, hi string, base int) []string {
	if len(lo) < len(hi) {
		// Up to the largest number of lo's length, then the rest.
		top := strings.Repeat(string(maxDigit(base)), len(lo))
		n, _ := strconv.ParseInt(top, base, 64)
		return append(rangeAlts(lo, top, base), rangeAlts(strconv.FormatInt(n+1, base), hi, base)...)
	}
	if lo == hi {
		return []string{lo}
	}
	i := 0
	for i < len(lo) && lo[i] == hi[i] {
		i++
	}
	prefix, lo, hi := lo[:i], lo[i:], hi[i:]
	if len(lo) == 1 {
		return []string{prefix + digitClass(lo[0], hi[0])}
	}
	any := digitClass('0', maxDigit(base))
	rest := len(lo) - 1
	lowFull := strings.Trim(lo[1:], "0") == ""
	highFull := strings.Trim(hi[1:], string(maxDigit(base))) == ""
	var out []string
	start, end := lo[0], hi[0]
	if !lowFull {
		for _, r := range rangeAlts(lo, string(lo[0])+strings.Repeat(string(maxDigit(base)), rest), base) {
			out = append(out, prefix+r)
		}
		start = nextDigit(start)
	}
	if !highFull {
		end = prevDigit(end)
	}
	if digitValue(start) <= digitValue(end) {
		out = append(out, prefix+digitClass(start, end)+strings.Repeat(any, rest))
	}
	if !highFull {
		for _, r := range rangeAlts(string(hi[0])+strings.Repeat("0", rest), hi, base) {
			out = append(out, prefix+r)
		}
	}
	return out
}

func maxDigit(base int) byte {
	if base == 16 {
		return 'f'
	}
	return '9'
}

func digitValue(c byte) int {
	if c >= 'a' {
		return int(c-'a') + 10
	}
	return int(c - '0')
}

func digitChar(v int) byte {
	if v >= 10 {
		return byte('a' + v - 10)
	}
	return byte('0' + v)
}

func nextDigit(c byte) byte { return digitChar(digitValue(c) + 1) }
func prevDigit(c byte) byte { return digitChar(digitValue(c) - 1) }

// digitClass matches one digit from a to b.
func digitClass(a, b byte) string {
	switch {
	case a == b:
		return string(a)
	case b <= '9' || a >= 'a':
		return "[" + string(a) + "-" + string(b) + "]"
	case a == '9':
		return "[9a-" + string(b) + "]"
	default:
		return "[" + string(a) + "-9a-" + string(b) + "]"
	}
}

var geoRegexRe = func() func(string) bool {
	return func(s string) bool {
		for _, part := range strings.Split(s, "|") {
			if len(part) != 2 || part[0] < 'A' || part[0] > 'Z' || part[1] < 'A' || part[1] > 'Z' {
				return false
			}
		}
		return true
	}
}()

// checkGeo validates the country fields of vhostData.
func (d *vhostData) checkGeo() error {
	if d.GeoMode == "" {
		if d.GeoRegex != "" || len(d.GeoFiles) > 0 || len(d.GeoTrusted) > 0 {
			return errors.New("refusing to render country lists without a mode")
		}
		return nil
	}
	if (d.GeoMode != GeoBlock && d.GeoMode != GeoAllow) || !geoRegexRe(d.GeoRegex) {
		return fmt.Errorf("refusing to render country blocking %q %q", d.GeoMode, d.GeoRegex)
	}
	for _, f := range d.GeoFiles {
		if !safePath(f) || !strings.HasSuffix(f, ".conf") {
			return fmt.Errorf("refusing to render the country file %q", f)
		}
	}
	for _, c := range d.GeoTrusted {
		if !safeCIDR(c) {
			return fmt.Errorf("refusing to render the trusted address %q", c)
		}
	}
	for _, r := range []string{d.GeoSelf, d.GeoCF} {
		if r != "" && !olsAddrRegexRe.MatchString(r) {
			return fmt.Errorf("refusing to render the addresses %q", r)
		}
	}
	return nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
