package siteimport

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/binary"
	"io"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/validate"
)

// SiteArchive is what the analysis of a website's files archive found.
type SiteArchive struct {
	arch    *Archive
	strip   string // the single top folder, "" when there is none
	root    string // the folder that becomes public/, relative to the archive
	dirs    map[string]*dirAgg
	all     dirAgg
	topDirs map[string]bool
	topFile bool               // a file at the top of the archive
	direct  map[string]bool    // folders holding index.php, index.html or wp-config.php
	wp      map[string]*wpInfo // by folder
	ini     map[string][]byte  // .user.ini by folder
	entries int
}

// siteFolders are the usual document root folders inside a home folder
// (cPanel, HestiaCP, CyberPanel: public_html; Plesk: httpdocs; XAMPP:
// htdocs).
var siteFolders = []string{"public_html", "httpdocs", "htdocs"}

// AnalyseSite reads a .tar.gz, .tar or .zip of a website's files.
func AnalyseSite(ctx context.Context, f *os.File, a *Archive, lim Limits, lg Logger) (*SiteArchive, error) {
	if a.Format != FormatTarGz && a.Format != FormatTar && a.Format != FormatZip {
		return nil, usererr.New("import_unknown_format", a.Name)
	}
	s := &SiteArchive{arch: a, dirs: map[string]*dirAgg{}, topDirs: map[string]bool{}, direct: map[string]bool{},
		wp: map[string]*wpInfo{}, ini: map[string][]byte{}}
	w := &walker{lim: lim}
	next := time.Now().Add(5 * time.Second)
	err := w.walk(ctx, f, a, func(e *entry) error {
		name, ok := cleanName(e.Name)
		if !ok || name == "" {
			return nil
		}
		first, rest, _ := strings.Cut(name, "/")
		if rest == "" && e.Kind != kindDir {
			s.topFile = true
		} else {
			s.topDirs[first] = true
		}
		if e.Kind != kindFile {
			return nil
		}
		php := strings.HasSuffix(strings.ToLower(name), ".php")
		s.all.add(e.Size, php)
		for _, d := range ancestors(name, 3) {
			a := s.dirs[d]
			if a == nil {
				a = &dirAgg{}
				s.dirs[d] = a
			}
			a.add(e.Size, php)
		}
		if time.Now().After(next) {
			next = time.Now().Add(5 * time.Second)
			lg.Logf("Read %d files (%s) of the archive", s.all.files, humanBytes(s.all.bytes))
		}
		dir, base := path.Split(name)
		dir = strings.TrimSuffix(dir, "/")
		if depth(name) > 3 {
			return nil
		}
		switch base {
		case "index.php", "index.html", "index.htm", "wp-config.php":
			s.direct[dir] = true
		}
		switch {
		case base == "wp-config.php" && e.Size <= maxConfig:
			if b := readSmall(e.R, maxConfig); b != nil {
				s.wp[dir] = parseWPConfig(b)
			}
		case base == ".user.ini" && e.Size <= 64<<10:
			s.ini[dir] = readSmall(e.R, 64<<10)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.entries = w.entries
	if s.all.files == 0 {
		return nil, usererr.New("import_empty", a.Name)
	}
	// One folder at the top, as aaPanel's backups have: its content is the
	// website.
	if !s.topFile && len(s.topDirs) == 1 {
		for d := range s.topDirs {
			s.strip = d
		}
	}
	s.root = s.strip
	// A home folder with public_html in it (and no website of its own at
	// its top): public_html is the website.
	if !s.direct[s.root] {
		var found []string
		for _, sf := range siteFolders {
			p := path.Join(s.root, sf)
			if s.dirs[p] != nil {
				found = append(found, p)
			}
		}
		if len(found) == 1 {
			s.root = found[0]
		}
	}
	lg.Logf("Read the whole archive: %d files, %s", s.all.files, humanBytes(s.all.bytes))
	return s, nil
}

var aaPanelSiteRe = regexp.MustCompile(`^web_(.+?)_[0-9]{8}_[0-9]{6}`)

// suggestDomain guesses the website's domain from the archive's name
// (aaPanel's web_DOMAIN_DATE_TIME…) or its top folder.
func (s *SiteArchive) suggestDomain() string {
	if m := aaPanelSiteRe.FindStringSubmatch(s.arch.Name); m != nil {
		if d := cleanDomain(m[1]); validate.Domain(d) == nil {
			return d
		}
	}
	if d := cleanDomain(s.strip); validate.Domain(d) == nil {
		return d
	}
	return ""
}

// Plan says what an import of the archive and dumps would do.
func (s *SiteArchive) Plan(env Env, dumps []*Dump) *Plan {
	p := env.base(KindArchive, s.arch)
	site := &ArchiveSite{Root: s.root, Domain: s.suggestDomain()}
	agg := s.dirs[s.root]
	if s.root == "" {
		agg = &s.all
	}
	if agg != nil {
		site.Files, site.Size = agg.files, agg.bytes
	}
	w := s.wp[s.root]
	switch {
	case w != nil:
		site.App, site.DB = "wordpress", w.DBName
	case agg != nil && agg.php > 0:
		site.App = "php"
	default:
		site.App = "static"
	}
	if site.App != "static" {
		site.PHP = env.choosePHP("")
		if site.PHP == "" {
			p.Notes = append(p.Notes, Note{Code: "php_missing"})
		}
	}
	if ini := s.ini[s.root]; ini != nil {
		site.OldRoot = openBasedirRoot(ini)
	}
	if site.Domain != "" && env.DomainTaken != nil && env.DomainTaken(site.Domain) {
		p.Notes = append(p.Notes, Note{Code: "domain_exists", Detail: site.Domain})
	}
	p.Site = site
	used := map[string]bool{}
	for _, d := range dumps {
		db := &Database{Name: d.DB, Size: d.Size, Users: []string{}, UsedBy: []string{}, Source: d.Archive.Path,
			Password: "new", format: d.Archive.Format}
		if db.Name == "" {
			db.Name = dbSafe(strings.TrimSuffix(strings.TrimSuffix(d.Archive.Name, ".gz"), ".sql"))
		}
		db.NewName = env.freeDBName(db.Name, used)
		if db.NewName == "" {
			db.Blocked = "no_name"
		}
		used[db.NewName] = true
		db.User = db.NewName
		if w != nil && (w.DBName == d.DB || len(dumps) == 1) {
			db.UsedBy = []string{"wordpress"}
		}
		db.ConfigChange = true
		p.Databases = append(p.Databases, db)
	}
	if !env.DBReady && len(dumps) > 0 {
		p.Notes = append(p.Notes, Note{Code: "db_not_running"})
	}
	return p
}

// Dump is a separate SQL dump of a site archive import.
type Dump struct {
	Archive *Archive
	DB      string // the database it was made from, when it says so
	Size    int64  // about how large it is unpacked
}

var (
	dumpDBRe     = regexp.MustCompile("(?m)^-- (?:Host: [^\\n]*?\\s+)?Database: `?([A-Za-z0-9_$-]+)`?\\s*$")
	aaPanelDBRe  = regexp.MustCompile(`^db_(.+?)_[0-9]{8}_[0-9]{6}`)
	dumpHeadSize = 64 << 10
)

// AnalyseDump reads the start of an SQL dump for the name of its database
// ("-- Host: localhost    Database: shop", or phpMyAdmin's "-- Database:
// `shop`"), else takes it from aaPanel's file name db_NAME_DATE_TIME….
func AnalyseDump(f *os.File, a *Archive) (*Dump, error) {
	d := &Dump{Archive: a, Size: a.Size}
	if a.Format == FormatSQLGz {
		// The size gzip records is modulo 4 GB: a dump smaller than its
		// archive went past it.
		if n := gzipSize(f, a.Size); n >= a.Size {
			d.Size = n
		} else {
			d.Size = a.Size * 8
		}
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	var r io.Reader = f
	if a.Format == FormatSQLGz {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return nil, usererr.New("invalid_sql_file", a.Name)
		}
		defer gz.Close()
		r = gz
	}
	head := make([]byte, dumpHeadSize)
	n, _ := io.ReadFull(bufio.NewReader(r), head)
	head = head[:n]
	if m := dumpDBRe.FindSubmatch(head); m != nil {
		d.DB = string(m[1])
	} else if m := aaPanelDBRe.FindStringSubmatch(a.Name); m != nil {
		d.DB = m[1]
	}
	if n == 0 {
		return nil, usererr.New("invalid_sql_file", a.Name)
	}
	return d, nil
}

// gzipSize is the uncompressed size a .gz file declares (modulo 4 GB).
func gzipSize(f *os.File, size int64) int64 {
	if size < 18 {
		return 0
	}
	var b [4]byte
	if _, err := f.ReadAt(b[:], size-4); err != nil {
		return 0
	}
	return int64(binary.LittleEndian.Uint32(b[:]))
}

// sortedKeys returns a map's keys, sorted.
func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
