// Package seccheck is the security check (other panels call it a risk
// scan): a list of cheap, bounded checks of the server's settings, each
// finding with a severity, the facts behind it and the place in the panel
// that fixes it. Nothing is scanned over the internet; the only network
// requests go to this server's own web server, for files that exist in a
// website's folder.
//
// The panel gathers the facts it already knows (SSH, firewall, fail2ban,
// updates, ports, websites…) into Facts; Run reads a few files itself
// (/etc/passwd, /etc/shadow, php.ini, redis.conf, the websites' folders,
// bounded) and returns a Report. Service keeps the last report and the
// findings the administrator accepted.
package seccheck

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nefocp/nefo/internal/docker"
	"github.com/nefocp/nefo/internal/procs"
	"github.com/nefocp/nefo/internal/security"
	"github.com/nefocp/nefo/internal/systools"
)

// Severities, worst first.
const (
	Critical = "critical"
	Warning  = "warning"
	Info     = "info"
)

// Fix is where in the panel a finding is fixed: a page of the UI (its
// route name) and the query that opens the right tab.
type Fix struct {
	Page  string            `json:"page"`
	Query map[string]string `json:"query,omitempty"`
}

// Finding is one problem the check found.
type Finding struct {
	// ID is stable from one run to the next: the check and its subject,
	// such as "site_no_https:example.com".
	ID string `json:"id"`
	// Check names the texts the UI shows (seccheck.c.<check>.title and
	// .text), with Params as their placeholders.
	Check    string            `json:"check"`
	Severity string            `json:"severity"`
	Params   map[string]string `json:"params,omitempty"`
	// Extra are more placeholders that change without the problem
	// changing (days left, examples), so they are not part of Hash.
	Extra map[string]string `json:"extra,omitempty"`
	Fix   *Fix              `json:"fix,omitempty"`
	// Hash changes when the facts behind the finding change, which brings
	// back a finding the administrator accepted.
	Hash     string `json:"hash"`
	Accepted bool   `json:"accepted"`
}

// Report is the result of one run.
type Report struct {
	CheckedAt time.Time `json:"checked_at"`
	Millis    int64     `json:"millis"`
	Findings  []Finding `json:"findings"`
	// Passed are the checks that ran and found nothing (their texts are
	// seccheck.ok.<key>); Skipped those that do not apply on this server
	// (no Redis, no websites…).
	Passed  []string `json:"passed"`
	Skipped []string `json:"skipped"`
	// Partial names the checks that stopped at their limit (files walked,
	// requests made) before they were done.
	Partial []string `json:"partial,omitempty"`
}

// Counts are the open findings by severity, and the accepted ones.
type Counts struct {
	Critical int `json:"critical"`
	Warning  int `json:"warning"`
	Info     int `json:"info"`
	Accepted int `json:"accepted"`
}

// Counts counts the report's findings.
func (r *Report) Counts() Counts {
	var c Counts
	for _, f := range r.Findings {
		switch {
		case f.Accepted:
			c.Accepted++
		case f.Severity == Critical:
			c.Critical++
		case f.Severity == Warning:
			c.Warning++
		default:
			c.Info++
		}
	}
	return c
}

// ---- facts ----

// Facts are what the panel already knows about the server. A nil pointer
// or slice means the panel could not find out (the program is missing),
// and the checks that need it are skipped.
type Facts struct {
	SSH      *security.SSH      // nil: no SSH server
	Firewall security.Firewall  // Installed false: no ufw
	Fail2ban security.Fail2ban  // Installed false: no fail2ban
	Updates  *systools.Updates  // nil: not an apt system
	Ports    []procs.Port       // nil: ss failed
	Docker   []docker.Container // nil: Docker is not installed or not running
	Panel    Panel
	Sites    []Site
	// WordPress is the WordPress toolkit's last check of each site, by
	// domain; sites it never checked are missing.
	WordPress map[string]WordPress
	// Partial are the checks whose facts could not be read in time (a
	// program that did not answer, see Gather): they are not run, and the
	// report lists them as partial.
	Partial []string
}

// Panel describes the panel itself.
type Panel struct {
	TLS        bool
	Loopback   bool // it listens on 127.0.0.1 only
	SelfSigned bool // its certificate is the self-signed one it made
	Users      []User
}

// User is a panel account.
type User struct {
	Name string
	TOTP bool
}

// Site is a website of the panel.
type Site struct {
	ID         int64
	Domain     string
	Dir        string // the site's folder, /var/www/<domain>; public/ is the web root
	SSL        bool
	ForceHTTPS bool
	CertExpiry time.Time
	CustomCert bool // a certificate the administrator uploaded (not renewed by the panel)
	Disabled   bool
	Password   bool // the site asks for a password (HTTP basic authentication)
	Redirect   bool // the site redirects every request elsewhere
	Proxy      bool // an app the web server forwards requests to
}

// WordPress is the WordPress toolkit's last check of a site.
type WordPress struct {
	CheckedAt     time.Time
	CoreUpdate    string
	PluginUpdates int
	ThemeUpdates  int
	Debug         bool
}

// ---- running ----

// check is one item of the list. run returns its findings, and false when
// it does not apply on this server.
type check struct {
	key string
	run func(c *runner) ([]Finding, bool)
}

// Keys lists every check, in the order they run and the UI lists them.
func Keys() []string {
	keys := make([]string, len(checks))
	for i, c := range checks {
		keys[i] = c.key
	}
	return keys
}

type runner struct {
	ctx     context.Context
	f       *Facts
	env     Env
	mu      sync.Mutex
	partial map[string]bool
}

// stopped marks a check as partial: it stopped at a limit (files walked,
// requests made, time) before it was done. Checks run at the same time,
// so this is the one thing they share.
func (r *runner) stopped(key string) {
	r.mu.Lock()
	r.partial[key] = true
	r.mu.Unlock()
}

func (r *runner) isPartial(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.partial[key]
}

// lateGrace is how long Run waits, once ctx is done, for checks to notice
// it: a check still busy after that (a folder on a file system that does
// not answer) is left out as partial.
var lateGrace = 2 * time.Second

// Run runs every check, all at the same time: most take no time, and the
// slow ones (requests to the web server, walking the websites' folders)
// wait on the disk and the network rather than the processor. It stops
// early when ctx is done; the checks not done by then, and those whose
// facts are missing (Facts.Partial), are listed as partial.
func Run(ctx context.Context, f *Facts, env Env) *Report {
	start := time.Now()
	env = env.withDefaults()
	r := &runner{ctx: ctx, f: f, env: env, partial: map[string]bool{}}
	rep := &Report{CheckedAt: env.Now, Findings: []Finding{}, Passed: []string{}, Skipped: []string{}}

	type result struct {
		found   []Finding
		applies bool
		done    bool
	}
	var mu sync.Mutex
	results := make([]result, len(checks))
	var wg sync.WaitGroup
	for i, c := range checks {
		if slices.Contains(f.Partial, c.key) {
			r.stopped(c.key)
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ctx.Err() != nil {
				r.stopped(c.key)
				return
			}
			found, applies := c.run(r)
			mu.Lock()
			results[i] = result{found: found, applies: applies, done: true}
			mu.Unlock()
		}()
	}
	all := make(chan struct{})
	go func() { wg.Wait(); close(all) }()
	select {
	case <-all:
	case <-ctx.Done():
		select {
		case <-all:
		case <-time.After(lateGrace):
		}
	}
	mu.Lock()
	got := slices.Clone(results) // a check that ends later writes to results, not got
	mu.Unlock()

	for i, c := range checks {
		res := got[i]
		partial := r.isPartial(c.key)
		switch {
		case !res.done:
			if !partial {
				r.stopped(c.key)
			}
			partial = true
		case !res.applies:
			rep.Skipped = append(rep.Skipped, c.key)
		case len(res.found) == 0:
			if !partial {
				rep.Passed = append(rep.Passed, c.key)
			}
		default:
			for j := range res.found {
				res.found[j].Hash = hashOf(res.found[j])
			}
			rep.Findings = append(rep.Findings, res.found...)
		}
		if partial {
			rep.Partial = append(rep.Partial, c.key)
		}
	}
	rank := map[string]int{Critical: 0, Warning: 1, Info: 2}
	sort.SliceStable(rep.Findings, func(i, j int) bool { return rank[rep.Findings[i].Severity] < rank[rep.Findings[j].Severity] })
	rep.Millis = time.Since(start).Milliseconds()
	return rep
}

// hashOf fingerprints a finding's facts: its check, severity and params.
func hashOf(f Finding) string {
	h := sha256.New()
	h.Write([]byte(f.Check + "\n" + f.Severity + "\n"))
	keys := make([]string, 0, len(f.Params))
	for k := range f.Params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		h.Write([]byte(k + "=" + f.Params[k] + "\n"))
	}
	return hex.EncodeToString(h.Sum(nil)[:8])
}

func finding(id, check, sev string, fix *Fix, params ...string) Finding {
	f := Finding{ID: id, Check: check, Severity: sev, Fix: fix}
	if len(params) > 0 {
		f.Params = map[string]string{}
		for i := 0; i+1 < len(params); i += 2 {
			f.Params[params[i]] = params[i+1]
		}
	}
	return f
}

func page(name string, query ...string) *Fix {
	f := &Fix{Page: name}
	if len(query) > 0 {
		f.Query = map[string]string{}
		for i := 0; i+1 < len(query); i += 2 {
			f.Query[query[i]] = query[i+1]
		}
	}
	return f
}

// list joins names for a text, at most n of them, then "+k".
func list(names []string, n int) string {
	if len(names) <= n {
		return strings.Join(names, ", ")
	}
	return strings.Join(names[:n], ", ") + " +" + itoa(len(names)-n)
}
