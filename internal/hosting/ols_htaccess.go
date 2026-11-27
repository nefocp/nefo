package hosting

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/store"
)

// .htaccess files on OpenLiteSpeed.
//
// OpenLiteSpeed reads a folder's .htaccess rewrite rules once, the first
// time the folder is used, and only notices changes after a restart. It also
// trusts them completely: the environment a rule sets ([E=NAME:value]) is
// sent to PHP-FPM as FastCGI parameters, so a .htaccess file could pass
// PHP_ADMIN_VALUE (extension=/path/evil.so, sendmail_path, open_basedir) to
// the site's pool, or replace SCRIPT_FILENAME.
//
// So OpenLiteSpeed never reads them itself (autoLoadHtaccess 0). The panel
// reads them, keeps the rewrite directives OpenLiteSpeed understands without
// the dangerous ones, and writes them into the site's virtual host as the
// rules of the folder's context, which is what OpenLiteSpeed's own .htaccess
// support does. WatchHtaccess rewrites the virtual host when a file
// changes, so edits apply within seconds, without a restart by hand.

const (
	maxHtaccessSize     = 256 << 10 // a larger .htaccess file is ignored
	maxHtaccessContexts = 64        // folders with rules, per site
	maxHtaccessEntries  = 50000     // files and folders looked at, per site
	maxHtaccessLine     = 4096
)

// htaccessState is what the watcher knows about a site's .htaccess files.
type htaccessState struct {
	files map[string]fileStamp // path relative to public → size and time
	found time.Time            // when the tree was last walked
}

type fileStamp struct {
	size int64
	mod  time.Time
}

// scanHtaccess reads the .htaccess files below public and returns their
// filtered rewrite rules. It reads inside an os.Root, follows no symlink and
// skips folders whose names the configuration cannot carry.
func (m *Manager) scanHtaccess(id int64, public string) olsHtaccess {
	var ht olsHtaccess
	stamps := map[string]fileStamp{}
	defer func() {
		m.htaccessMu.Lock()
		if m.htaccess == nil {
			m.htaccess = map[int64]htaccessState{}
		}
		m.htaccess[id] = htaccessState{files: stamps, found: time.Now()}
		m.htaccessMu.Unlock()
	}()
	root, err := os.OpenRoot(public)
	if err != nil {
		return ht
	}
	defer root.Close()
	entries := 0
	fs.WalkDir(root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		entries++
		if entries > maxHtaccessEntries {
			return fs.SkipAll
		}
		if d.IsDir() {
			if p != "." && (strings.HasPrefix(d.Name(), ".") || olsRelPath(p) == "") {
				return fs.SkipDir
			}
			return nil
		}
		if d.Name() != ".htaccess" || !d.Type().IsRegular() {
			return nil
		}
		dir := filepath.Dir(filepath.FromSlash(p))
		uri := olsRelPath(dir)
		if uri == "" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		stamps[filepath.ToSlash(p)] = fileStamp{info.Size(), info.ModTime()}
		if info.Size() > maxHtaccessSize {
			m.log.Warn("htaccess file too large, ignored", "file", filepath.Join(public, p))
			return nil
		}
		b, err := readRootFile(root, p)
		if err != nil {
			return nil
		}
		rules := sanitizeHtaccess(b)
		if rules == "" {
			return nil
		}
		if uri == "/" {
			ht.Root = rules
			return nil
		}
		if len(ht.Contexts) >= maxHtaccessContexts {
			return nil
		}
		ht.Contexts = append(ht.Contexts, olsHtContext{URI: uri, Location: public + uri, Rules: rules})
		return nil
	})
	sort.Slice(ht.Contexts, func(i, j int) bool { return ht.Contexts[i].URI < ht.Contexts[j].URI })
	return ht
}

// readRootFile reads a regular file inside root, refusing symlinks.
func readRootFile(root *os.Root, name string) ([]byte, error) {
	fi, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, maxHtaccessSize+1))
}

var (
	htIfModuleRe   = regexp.MustCompile(`^</?IfModule(?:\s+[!A-Za-z0-9_.-]+)?\s*>$`)
	htEngineRe     = regexp.MustCompile(`(?i)^RewriteEngine\s+(on|off)$`)
	htBaseRe       = regexp.MustCompile(`(?i)^RewriteBase\s+(/[A-Za-z0-9._~%/@+-]*)$`)
	htEnvNameRe    = regexp.MustCompile(`^!?[A-Za-z0-9_.-]+$`)
	htDirectiveRe  = regexp.MustCompile(`(?i)^(RewriteCond|RewriteRule|CacheKeyModify)\s`)
	htForbiddenEnv = []string{
		// PHP-FPM reads them as php.ini settings, PHP_ADMIN_VALUE with
		// system rights.
		"php_value", "php_admin_value", "php_flag", "php_admin_flag",
		// PHP-FPM picks the script with these.
		"script_filename", "script_name", "path_info", "path_translated", "document_root",
		"orig_path_info", "orig_script_name", "orig_script_filename", "context_document_root",
		// Internal flags of OpenLiteSpeed.
		"proxy-host", "modsecurity",
	}
)

// sanitizeHtaccess keeps the lines of a .htaccess file that OpenLiteSpeed's
// rewrite engine reads (RewriteEngine, RewriteBase, RewriteCond,
// RewriteRule, CacheKeyModify and IfModule sections) and drops:
//   - rules that set a forbidden environment variable, or one whose name is
//     not a constant (E=%1:... could name any);
//   - rules that proxy ([P]): they could reach any app's backend by name;
//   - RewriteFile, which reads rules from any file, and everything else.
func sanitizeHtaccess(b []byte) string {
	if len(b) > maxHtaccessSize {
		return ""
	}
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
	var keep, conds []string
	size := 0
	add := func(lines ...string) bool {
		for _, l := range lines {
			if size+len(l)+1 > olsMaxRules {
				return false
			}
			size += len(l) + 1
			keep = append(keep, l)
		}
		return true
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") || len(line) > maxHtaccessLine ||
			strings.ContainsAny(line, "\x00\r") {
			continue
		}
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "rewritecond") && htDirectiveRe.MatchString(line):
			// A condition belongs to the next rule: it is kept or dropped
			// with it.
			conds = append(conds, line)
			continue
		case strings.HasPrefix(lower, "rewriterule") && htDirectiveRe.MatchString(line):
			pending := conds
			conds = nil
			if !safeRewriteRule(line) {
				continue
			}
			if !add(append(pending, line)...) {
				return strings.Join(keep, "\n")
			}
			continue
		case htIfModuleRe.MatchString(line):
		case htEngineRe.MatchString(line):
			line = "RewriteEngine " + strings.ToLower(htEngineRe.FindStringSubmatch(line)[1])
		case htBaseRe.MatchString(line):
		case htDirectiveRe.MatchString(line): // CacheKeyModify
		default:
			continue
		}
		// Conditions not followed by their rule are dropped, as OpenLiteSpeed
		// would ignore them.
		conds = nil
		if !add(line) {
			break
		}
	}
	return strings.Join(keep, "\n")
}

// safeRewriteRule checks the flags of a RewriteRule line.
func safeRewriteRule(line string) bool {
	args, ok := rewriteArgs(line)
	if !ok || len(args) < 3 || len(args) > 4 {
		return false
	}
	if len(args) == 3 {
		return true
	}
	flags := args[3]
	if !strings.HasPrefix(flags, "[") || !strings.HasSuffix(flags, "]") {
		return false
	}
	for _, f := range strings.Split(flags[1:len(flags)-1], ",") {
		f = strings.TrimSpace(f)
		name, value, _ := strings.Cut(f, "=")
		switch strings.ToLower(name) {
		case "p", "proxy":
			return false
		case "e", "env":
			value = strings.Trim(value, `"'`)
			key, _, _ := strings.Cut(value, ":")
			key = strings.TrimSpace(key)
			if !htEnvNameRe.MatchString(key) {
				return false
			}
			if slices.Contains(htForbiddenEnv, strings.ToLower(strings.TrimPrefix(key, "!"))) {
				return false
			}
		}
	}
	return true
}

// rewriteArgs splits a rewrite directive like OpenLiteSpeed does: by spaces,
// with "double" or 'single' quotes around an argument.
func rewriteArgs(line string) ([]string, bool) {
	var args []string
	i := 0
	for i < len(line) {
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		if i >= len(line) {
			break
		}
		if q := line[i]; q == '"' || q == '\'' {
			end := strings.IndexByte(line[i+1:], q)
			if end < 0 {
				return nil, false
			}
			args = append(args, line[i+1:i+1+end])
			i += end + 2
			continue
		}
		start := i
		for i < len(line) && line[i] != ' ' && line[i] != '\t' {
			if line[i] == '\\' && i+1 < len(line) {
				i++
			}
			i++
		}
		args = append(args, line[start:i])
	}
	return args, true
}

// ---- the watcher ----

// WatchHtaccess keeps the OpenLiteSpeed sites' rewrite rules in line with
// their .htaccess files until ctx is done. Every 10 seconds it looks at the
// files it knows (and the root .htaccess of each site); every 2 minutes it
// walks the sites' folders for new ones. A site whose rules changed gets
// its virtual host rewritten and OpenLiteSpeed restarts gracefully. On
// Nginx and Apache servers it does nothing.
func (m *Manager) WatchHtaccess(ctx context.Context) {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for tick := 0; ; tick++ {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if m.serverKind() != ServerOLS || !m.Supported() || !fileExists(m.web().hookFile()) {
			continue
		}
		m.checkHtaccess(ctx, tick%12 == 0)
	}
}

// checkHtaccess applies changed .htaccess rules. With full, every site's
// tree is walked again.
func (m *Manager) checkHtaccess(ctx context.Context, full bool) {
	sites, err := m.store.ListSites(ctx)
	if err != nil {
		return
	}
	var due []*store.Site
	for _, s := range sites {
		if s.Disabled || s.RedirectURL != "" || s.ProxyPort != 0 || s.ProxyURL != "" {
			continue
		}
		if full || m.htaccessChanged(s) {
			due = append(due, s)
		}
	}
	// Render first, without the lock: the panel's own changes get it with
	// tryLock, and most checks find nothing to do.
	var stale []*store.Site
	for _, s := range due {
		if m.vhostStale(ctx, s) {
			stale = append(stale, s)
		}
	}
	if len(stale) == 0 {
		return
	}
	// The flag first: a change that finds the lock taken by the watcher
	// waits for it instead of failing (tryLock).
	m.htaccessBusy.Store(true)
	if !m.ops.TryLock() {
		m.htaccessBusy.Store(false)
		// Busy: look again at the next tick.
		m.htaccessMu.Lock()
		for _, s := range stale {
			delete(m.htaccess, s.ID)
		}
		m.htaccessMu.Unlock()
		return
	}
	defer func() {
		m.ops.Unlock()
		m.htaccessBusy.Store(false)
	}()
	type change struct {
		path string
		old  []byte
	}
	var changed []change
	for _, s := range stale {
		site, err := m.store.SiteByID(ctx, s.ID) // it may have changed meanwhile
		if err != nil {
			continue
		}
		data, err := m.vhostBytes(ctx, site)
		if err != nil {
			continue
		}
		file := m.vhostFile(site.Domain)
		old, err := os.ReadFile(file)
		if err != nil || bytes.Equal(old, data) {
			continue
		}
		if err := writeFile(file, data, 0o644); err != nil {
			m.log.Warn("htaccess rules", "domain", site.Domain, "err", err)
			continue
		}
		changed = append(changed, change{file, old})
		m.log.Info("applying changed .htaccess rules", "domain", site.Domain)
	}
	if len(changed) == 0 {
		return
	}
	if err := m.reloadWeb(ctx); err != nil {
		m.log.Warn("htaccess rules refused, previous rules kept", "err", err)
		for _, c := range changed {
			writeFile(c.path, c.old, 0o644)
		}
		m.reloadWeb(context.WithoutCancel(ctx))
	}
}

// vhostStale reports whether the site's virtual host, rendered now, differs
// from its file.
func (m *Manager) vhostStale(ctx context.Context, s *store.Site) bool {
	data, err := m.vhostBytes(ctx, s)
	if err != nil {
		return false
	}
	old, err := os.ReadFile(m.vhostFile(s.Domain))
	return err == nil && !bytes.Equal(old, data)
}

// htaccessChanged reports whether a known .htaccess file of the site, or
// its root one, changed since the site's rules were last rendered.
func (m *Manager) htaccessChanged(s *store.Site) bool {
	m.htaccessMu.Lock()
	st, ok := m.htaccess[s.ID]
	m.htaccessMu.Unlock()
	if !ok {
		return true
	}
	public := filepath.Join(m.paths.siteRoot(s.Domain), "public")
	check := map[string]bool{".htaccess": true}
	for p := range st.files {
		check[p] = true
	}
	for p := range check {
		fi, err := os.Lstat(filepath.Join(public, filepath.FromSlash(p)))
		old, known := st.files[p]
		switch {
		case err != nil && known, err == nil && !known:
			return true
		case err == nil && (fi.Size() != old.size || !fi.ModTime().Equal(old.mod)):
			return true
		}
	}
	return false
}
