// Package logstats computes a website's traffic statistics from its Nginx
// access logs: requests, visitors and bandwidth per day, status codes, and
// the most requested pages, the busiest IP addresses, referrers, browsers
// and robots.
//
// It reads <domain>.access.log and the copies that logrotate keeps
// (<domain>.access.log.1, <domain>.access.log.2.gz and so on), streaming them
// line by line, so memory use does not grow with the size of the logs.
package logstats

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/usererr"
)

// DefaultMaxLines is the number of log lines Compute reads at most. Beyond
// it the report is marked Truncated and the oldest lines are left out.
const DefaultMaxLines = 5_000_000

// MaxDays is the longest period Compute accepts.
const MaxDays = 31

// TopN is the length of the lists of pages, IP addresses and so on.
const TopN = 10

// Limits on the number of different values kept in memory. Past them, the
// least frequent values are dropped, so the counts of rare values (never
// those of the top 10 in practice) become approximate.
const (
	maxPaths     = 20_000
	maxReferrers = 5_000
	maxAgents    = 2_000
	maxIPs       = 200_000 // past this, new visitors are not counted
	maxAgentMemo = 5_000
	maxLineBytes = 64 << 10
)

// Counts are the numbers for a day, an hour or the whole period.
type Counts struct {
	Requests int64 `json:"requests"`
	// Visitors is the number of different IP addresses.
	Visitors  int64 `json:"visitors"`
	Bytes     int64 `json:"bytes"`
	Status2xx int64 `json:"status_2xx"`
	Status3xx int64 `json:"status_3xx"`
	Status4xx int64 `json:"status_4xx"`
	Status5xx int64 `json:"status_5xx"`
	// Bots counts the requests of robots and programs (see classify).
	Bots int64 `json:"bots"`
}

// Bucket is the traffic of one day or one hour, starting at T.
type Bucket struct {
	T time.Time `json:"t"`
	Counts
}

// Item is an entry of a top 10 list.
type Item struct {
	Name     string `json:"name"`
	Requests int64  `json:"requests"`
	Bytes    int64  `json:"bytes"`
}

// Report holds a site's statistics for a period.
type Report struct {
	Days int `json:"days"`
	// The period runs from midnight Days-1 days ago to now, in the time zone
	// of the now passed to Compute.
	From   time.Time `json:"from"`
	To     time.Time `json:"to"`
	Totals Counts    `json:"totals"`
	Daily  []Bucket  `json:"daily"`
	// Hourly is set for a one-day period, up to the current hour.
	Hourly    []Bucket `json:"hourly,omitempty"`
	Paths     []Item   `json:"paths"`
	IPs       []Item   `json:"ips"`
	Referrers []Item   `json:"referrers"`
	Browsers  []Item   `json:"browsers"`
	Bots      []Item   `json:"bots"`
	// LogsSince is the time of the oldest line read when the logs begin
	// after From: the site is new, or older logs were deleted.
	LogsSince *time.Time `json:"logs_since,omitempty"`
	// Files is the number of log files found (0: the site has no logs yet),
	// Lines the number of lines read and Malformed the lines that could not
	// be read.
	Files     int   `json:"files"`
	Lines     int64 `json:"lines"`
	Malformed int64 `json:"malformed"`
	// Truncated is set when the logs had more than DefaultMaxLines lines.
	Truncated bool `json:"truncated"`
	// Approximate is set when there were too many different visitors or
	// pages to count them all exactly.
	Approximate bool `json:"approximate"`
}

// Compute reads the access logs of domain in dir and returns the statistics
// of the last days days (today and the days-1 days before), using the time
// zone of now for the day boundaries. names are the site's own host names
// (its domain and aliases), left out of the referrers. A site without logs
// gets an empty report.
func Compute(dir, domain string, names []string, days int, now time.Time) (*Report, error) {
	return compute(dir, domain, names, days, now, DefaultMaxLines)
}

func compute(dir, domain string, names []string, days int, now time.Time, maxLines int64) (*Report, error) {
	if days < 1 || days > MaxDays {
		return nil, usererr.New("invalid_days")
	}
	if domain == "" || strings.ContainsAny(domain, `/\`) || strings.HasPrefix(domain, ".") {
		return nil, fmt.Errorf("invalid domain %q", domain)
	}
	a := newAggregator(names, days, now)
	files, err := logFiles(dir, domain+".access.log")
	if err != nil {
		return nil, err
	}
	a.files = len(files)
	for _, name := range files {
		if err := a.readFile(filepath.Join(dir, name), maxLines); err != nil {
			return nil, err
		}
	}
	return a.report(), nil
}

// logFiles returns the names of the regular files in dir that hold base and
// its rotated copies, newest first: base, base.1, base.2.gz...
func logFiles(dir, base string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	type numbered struct {
		name string
		n    int
	}
	var list []numbered
	for _, e := range entries {
		// The type comes from the directory itself (like Lstat): symbolic
		// links, which could point anywhere, are not regular files.
		if !e.Type().IsRegular() {
			continue
		}
		name := e.Name()
		n := 0
		if name != base {
			s, ok := strings.CutPrefix(name, base+".")
			if !ok {
				continue
			}
			s = strings.TrimSuffix(s, ".gz")
			if n, err = strconv.Atoi(s); err != nil || n < 1 || n > 999 || strconv.Itoa(n) != s {
				continue
			}
		}
		list = append(list, numbered{name, n})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].n != list[j].n {
			return list[i].n < list[j].n
		}
		return list[i].name < list[j].name
	})
	out := make([]string, len(list))
	for i, f := range list {
		out[i] = f.name
	}
	return out, nil
}

var errNotRegular = errors.New("not a regular file")

// readFile adds the lines of one log file, plain or gzip-compressed.
func (a *aggregator) readFile(path string, maxLines int64) error {
	f, err := openLog(path)
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, errNotRegular) || isSymlinkErr(err) {
		return nil // rotated or replaced since the directory was read
	}
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() {
		return nil
	}
	// Nothing was written to the file since the period began.
	if fi.ModTime().Unix() < a.dayStarts[0] {
		a.olderData = true
		return nil
	}
	if a.lines >= maxLines {
		a.truncated = true
		return nil
	}

	br := bufio.NewReaderSize(f, maxLineBytes)
	r := br
	if magic, _ := br.Peek(2); len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		zr, err := gzip.NewReader(br)
		if err != nil {
			a.malformed++ // a damaged copy
			return nil
		}
		defer zr.Close()
		r = bufio.NewReaderSize(zr, maxLineBytes)
	}

	for a.lines < maxLines {
		line, err := r.ReadSlice('\n')
		if errors.Is(err, bufio.ErrBufferFull) {
			// A line too long for any real request: skip the rest of it.
			a.lines++
			a.malformed++
			for errors.Is(err, bufio.ErrBufferFull) {
				_, err = r.ReadSlice('\n')
			}
			if err == nil {
				continue
			}
		} else if line = bytes.TrimRight(line, "\r\n"); len(line) > 0 {
			a.lines++
			a.addLine(line)
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			if _, isPath := err.(*fs.PathError); isPath {
				return err
			}
			// The end of a compressed copy is damaged: keep what was read.
			a.malformed++
			return nil
		}
	}
	if _, err := r.Peek(1); err != io.EOF {
		a.truncated = true
	}
	return nil
}

type counter struct {
	m      map[string]*Item
	max    int
	pruned bool
}

func newCounter(max int) *counter { return &counter{m: map[string]*Item{}, max: max} }

func (c *counter) add(key []byte, n int64) {
	if it, ok := c.m[string(key)]; ok {
		it.Requests++
		it.Bytes += n
		return
	}
	c.insert(string(key), n)
}

func (c *counter) addString(key string, n int64) {
	if it, ok := c.m[key]; ok {
		it.Requests++
		it.Bytes += n
		return
	}
	c.insert(key, n)
}

func (c *counter) insert(key string, n int64) {
	if len(c.m) >= c.max {
		c.prune()
	}
	c.m[key] = &Item{Name: key, Requests: 1, Bytes: n}
}

// prune drops the less frequent half of the values.
func (c *counter) prune() {
	counts := make([]int64, 0, len(c.m))
	for _, it := range c.m {
		counts = append(counts, it.Requests)
	}
	sort.Slice(counts, func(i, j int) bool { return counts[i] > counts[j] })
	limit := counts[len(counts)/2]
	for k, it := range c.m {
		if it.Requests <= limit {
			delete(c.m, k)
		}
	}
	c.pruned = true
}

func (c *counter) top(n int) []Item {
	out := make([]Item, 0, len(c.m))
	for _, it := range c.m {
		out = append(out, *it)
	}
	return topItems(out, n)
}

func topItems(items []Item, n int) []Item {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Requests != items[j].Requests {
			return items[i].Requests > items[j].Requests
		}
		return items[i].Name < items[j].Name
	})
	if len(items) > n {
		items = items[:n]
	}
	return items
}

type ipStat struct {
	requests, bytes int64
	days            uint32 // bit i: seen on day i
	hours           uint32 // bit i: seen in hour i (one-day periods)
}

type agentInfo struct {
	name string
	bot  bool
}

type aggregator struct {
	days      int
	now       time.Time
	dayStarts []int64 // days+1 boundaries, in Unix seconds
	hourStart []int64 // 25 boundaries for a one-day period
	names     map[string]bool
	wildcards []string // ".example.com" for the alias "*.example.com"

	daily  []Counts
	hourly []Counts
	total  Counts

	ips       map[string]*ipStat
	paths     *counter
	referrers *counter
	browsers  *counter
	bots      *counter
	agentMemo map[string]agentInfo
	lower     []byte

	files     int
	lines     int64
	malformed int64
	truncated bool
	ipsFull   bool
	oldest    int64 // the oldest line read, in Unix seconds
	olderData bool  // lines (or files) older than the period were seen
}

func newAggregator(names []string, days int, now time.Time) *aggregator {
	a := &aggregator{
		days:      days,
		now:       now,
		names:     map[string]bool{},
		daily:     make([]Counts, days),
		ips:       map[string]*ipStat{},
		paths:     newCounter(maxPaths),
		referrers: newCounter(maxReferrers),
		browsers:  newCounter(maxAgents),
		bots:      newCounter(maxAgents),
		agentMemo: map[string]agentInfo{},
		oldest:    math.MaxInt64,
	}
	for _, n := range names {
		n = strings.TrimSuffix(strings.ToLower(n), ".")
		if rest, ok := strings.CutPrefix(n, "*."); ok {
			a.wildcards = append(a.wildcards, "."+rest)
		} else if n != "" {
			a.names[n] = true
		}
	}
	// time.Date handles daylight saving time: a day can be 23 or 25 hours.
	loc := now.Location()
	y, m, d := now.Date()
	for i := 0; i <= days; i++ {
		a.dayStarts = append(a.dayStarts, time.Date(y, m, d-(days-1)+i, 0, 0, 0, 0, loc).Unix())
	}
	if days == 1 {
		a.hourly = make([]Counts, 24)
		for h := 0; h < 24; h++ {
			a.hourStart = append(a.hourStart, time.Date(y, m, d, h, 0, 0, 0, loc).Unix())
		}
		a.hourStart = append(a.hourStart, a.dayStarts[1])
	}
	return a
}

// bucket returns i such that b[i] <= t < b[i+1], or -1.
func bucket(b []int64, t int64) int {
	if t < b[0] || t >= b[len(b)-1] {
		return -1
	}
	return sort.Search(len(b), func(i int) bool { return b[i] > t }) - 1
}

func (a *aggregator) addLine(line []byte) {
	var e entry
	if !parseLine(line, &e) {
		a.malformed++
		return
	}
	if e.unix < a.oldest {
		a.oldest = e.unix
	}
	if e.path != nil && bytes.HasPrefix(e.path, acmePrefix) {
		return
	}
	day := bucket(a.dayStarts, e.unix)
	if day < 0 {
		if e.unix < a.dayStarts[0] {
			a.olderData = true
		}
		return
	}
	hour := -1
	if a.hourly != nil {
		hour = bucket(a.hourStart, e.unix)
	}

	ag, ok := a.agentMemo[string(e.agent)]
	if !ok {
		ag.name, ag.bot = classify(e.agent, &a.lower)
		if len(a.agentMemo) >= maxAgentMemo {
			clear(a.agentMemo)
		}
		a.agentMemo[string(e.agent)] = ag
	}

	newDay, newHour := false, false
	ip, ok := a.ips[string(e.ip)]
	if !ok {
		if len(a.ips) < maxIPs {
			ip = &ipStat{}
			a.ips[string(e.ip)] = ip
			a.total.Visitors++
		} else {
			a.ipsFull = true
		}
	}
	if ip != nil {
		ip.requests++
		ip.bytes += e.bytes
		if ip.days&(1<<day) == 0 {
			ip.days |= 1 << day
			newDay = true
		}
		if hour >= 0 && ip.hours&(1<<hour) == 0 {
			ip.hours |= 1 << hour
			newHour = true
		}
	}

	countInto(&a.total, &e, ag.bot, false)
	countInto(&a.daily[day], &e, ag.bot, newDay)
	if hour >= 0 {
		countInto(&a.hourly[hour], &e, ag.bot, newHour)
	}

	if e.path != nil {
		a.paths.add(e.path, e.bytes)
	}
	if h := refHost(e.ref); h != nil && !a.ownName(h) {
		a.referrers.add(h, e.bytes)
	}
	if ag.bot {
		a.bots.addString(ag.name, e.bytes)
	} else {
		a.browsers.addString(ag.name, e.bytes)
	}
}

func countInto(c *Counts, e *entry, bot, newVisitor bool) {
	c.Requests++
	c.Bytes += e.bytes
	switch e.status / 100 {
	case 2:
		c.Status2xx++
	case 3:
		c.Status3xx++
	case 4:
		c.Status4xx++
	case 5:
		c.Status5xx++
	}
	if bot {
		c.Bots++
	}
	if newVisitor {
		c.Visitors++
	}
}

func (a *aggregator) ownName(host []byte) bool {
	if a.names[string(host)] {
		return true
	}
	for _, w := range a.wildcards {
		if len(host) > len(w) && bytes.HasSuffix(host, []byte(w)) {
			return true
		}
	}
	return false
}

// refHost returns the lowercase host name of a referrer URL, or nil.
func refHost(ref []byte) []byte {
	i := bytes.Index(ref, []byte("://"))
	if i <= 0 {
		return nil
	}
	h := ref[i+3:]
	if j := bytes.IndexAny(h, "/?#"); j >= 0 {
		h = h[:j]
	}
	if j := bytes.LastIndexByte(h, '@'); j >= 0 {
		h = h[j+1:]
	}
	if len(h) > 0 && h[0] == '[' { // [2001:db8::1]:8080
		j := bytes.IndexByte(h, ']')
		if j < 0 {
			return nil
		}
		h = h[1:j]
	} else if j := bytes.LastIndexByte(h, ':'); j >= 0 {
		h = h[:j]
	}
	h = bytes.TrimSuffix(h, []byte("."))
	if len(h) == 0 || len(h) > 253 {
		return nil
	}
	for _, c := range h {
		if c >= 'A' && c <= 'Z' {
			return bytes.ToLower(h)
		}
	}
	return h
}

func (a *aggregator) report() *Report {
	loc := a.now.Location()
	r := &Report{
		Days:        a.days,
		From:        time.Unix(a.dayStarts[0], 0).In(loc),
		To:          a.now,
		Totals:      a.total,
		Daily:       make([]Bucket, a.days),
		Paths:       a.paths.top(TopN),
		Referrers:   a.referrers.top(TopN),
		Browsers:    a.browsers.top(TopN),
		Bots:        a.bots.top(TopN),
		Files:       a.files,
		Lines:       a.lines,
		Malformed:   a.malformed,
		Truncated:   a.truncated,
		Approximate: a.ipsFull || a.paths.pruned || a.referrers.pruned,
	}
	for i := range a.daily {
		r.Daily[i] = Bucket{T: time.Unix(a.dayStarts[i], 0).In(loc), Counts: a.daily[i]}
	}
	nowUnix := a.now.Unix()
	for i := range a.hourly {
		if a.hourStart[i] > nowUnix {
			break
		}
		r.Hourly = append(r.Hourly, Bucket{T: time.Unix(a.hourStart[i], 0).In(loc), Counts: a.hourly[i]})
	}
	ips := make([]Item, 0, len(a.ips))
	for k, v := range a.ips {
		ips = append(ips, Item{Name: k, Requests: v.requests, Bytes: v.bytes})
	}
	r.IPs = topItems(ips, TopN)
	if !a.olderData && a.oldest != math.MaxInt64 && a.oldest > a.dayStarts[0] {
		t := time.Unix(a.oldest, 0).In(loc)
		r.LogsSince = &t
	}
	return r
}
