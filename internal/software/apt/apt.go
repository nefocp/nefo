// Package apt removes Debian packages without surprises. apt-get also
// removes every package that depends on the ones it is asked to remove, so
// each removal is simulated first and refused when it would take anything
// else with it. Afterwards only the dependencies that this removal left
// unused are cleaned up, never packages that were unused before.
package apt

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/nefocp/nefo/internal/run"
)

// Logger receives progress messages for a job log.
type Logger interface {
	io.Writer
	Logf(format string, args ...any)
}

var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9+.*-]{0,99}$`)

// Installed returns the packages matching names (dpkg-query patterns such
// as "php8.3-*" are allowed) that are installed. withConfig also returns
// packages that were removed but still have their configuration files.
func Installed(ctx context.Context, r run.Runner, withConfig bool, names ...string) ([]string, error) {
	for _, n := range names {
		if !nameRe.MatchString(n) {
			return nil, fmt.Errorf("invalid package name %q", n)
		}
	}
	if len(names) == 0 {
		return []string{}, nil
	}
	// dpkg-query fails when one pattern matches nothing, but still lists
	// the others, so the output counts and the exit status does not.
	out, _ := r.Output(ctx, "dpkg-query", append([]string{"-W", "-f", "${Package}\t${db:Status-Status}\n"}, names...)...)
	return parseInstalled(out, withConfig), nil
}

func parseInstalled(out string, withConfig bool) []string {
	seen := map[string]bool{}
	pkgs := []string{}
	for _, line := range strings.Split(out, "\n") {
		name, status, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if !ok || seen[name] || !nameRe.MatchString(name) {
			continue
		}
		switch status {
		case "not-installed":
			continue
		case "config-files":
			if !withConfig {
				continue
			}
		}
		seen[name] = true
		pkgs = append(pkgs, name)
	}
	sort.Strings(pkgs)
	return pkgs
}

// simulated returns the packages an apt-get -s run would remove.
func simulated(out string) []string {
	var pkgs []string
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) >= 2 && (f[0] == "Remv" || f[0] == "Purg") {
			name, _, _ := strings.Cut(f[1], ":") // libfoo:amd64
			if !slices.Contains(pkgs, name) {
				pkgs = append(pkgs, name)
			}
		}
	}
	sort.Strings(pkgs)
	return pkgs
}

// orphans returns the packages "apt-get autoremove" would remove now.
func orphans(ctx context.Context, r run.Runner) (map[string]bool, error) {
	out, err := r.Output(ctx, "apt-get", "-s", "-q", "autoremove")
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, p := range simulated(out) {
		set[p] = true
	}
	return set, nil
}

// protected packages are never cleaned up as leftovers, even when nothing
// depends on them any more: administrators and the panel use them directly,
// and the Node.js builds from nodejs.org, which apt does not know, need
// libatomic1.
var protected = regexp.MustCompile(`^(rsync|curl|wget|git|ca-certificates|gnupg|gpg|openssl|lsof|psmisc|socat|pv|iptables|nftables|ufw|sudo|cron|perl|perl-base|python3|python3-minimal|python3\.[0-9]+(-minimal)?|libpython3.*|linux-.*|systemd.*|openssh-.*|libatomic1)$`)

// ErrWouldRemoveMore is returned (wrapped) when a removal would also take
// packages that were not asked for.
var ErrWouldRemoveMore = errors.New("the removal would remove other packages")

// Check returns the error Remove would refuse with, without removing
// anything, so a caller can stop services only when the removal will run.
func Check(ctx context.Context, r run.Runner, purge bool, pkgs ...string) error {
	if len(pkgs) == 0 {
		return nil
	}
	verb := "remove"
	if purge {
		verb = "purge"
	}
	return check(ctx, r, verb, pkgs)
}

// Remove removes the given packages, which must be installed (see
// Installed). purge also deletes their configuration and runs their purge
// scripts. Then the packages that only they needed are removed the same way.
func Remove(ctx context.Context, r run.Runner, lg Logger, purge bool, pkgs ...string) error {
	if len(pkgs) == 0 {
		lg.Logf("No packages to remove")
		return nil
	}
	for _, p := range pkgs {
		if !nameRe.MatchString(p) || strings.Contains(p, "*") {
			return fmt.Errorf("invalid package name %q", p)
		}
		if protected.MatchString(p) {
			return fmt.Errorf("refusing to remove %s", p)
		}
	}
	verb := "remove"
	if purge {
		verb = "purge"
	}
	before, orphErr := orphans(ctx, r)
	if err := check(ctx, r, verb, pkgs); err != nil {
		return err
	}
	lg.Logf("Removing %s", strings.Join(pkgs, " "))
	if err := run.Apt(ctx, r, lg, append([]string{verb}, pkgs...)...); err != nil {
		return err
	}
	if orphErr != nil {
		lg.Logf("Warning: could not list unused packages, so none are cleaned up: %v", orphErr)
		return nil
	}
	after, err := orphans(ctx, r)
	if err != nil {
		lg.Logf("Warning: could not list unused packages, so none are cleaned up: %v", err)
		return nil
	}
	var left []string
	for p := range after {
		if !before[p] && !protected.MatchString(p) {
			left = append(left, p)
		}
	}
	if len(left) == 0 {
		return nil
	}
	sort.Strings(left)
	// A leftover that a kept package (perl, say) needs would take that
	// package with it. Then each one is simulated alone, and only those
	// whose removal stays within the leftovers go.
	safe := left
	if check(ctx, r, verb, left) != nil {
		var kept []string
		safe = nil
		for _, p := range left {
			if check(ctx, r, verb, left, p) == nil {
				safe = append(safe, p)
			} else {
				kept = append(kept, p)
			}
		}
		if len(kept) > 0 {
			lg.Logf("Keeping %s: packages that stay need them", strings.Join(kept, " "))
		}
		if len(safe) == 0 {
			return nil
		}
		if err := check(ctx, r, verb, safe); err != nil {
			lg.Logf("Warning: leaving the unused packages %s in place: %v", strings.Join(safe, " "), err)
			return nil
		}
	}
	lg.Logf("Removing packages that only these needed: %s", strings.Join(safe, " "))
	return run.Apt(ctx, r, lg, append([]string{verb}, safe...)...)
}

// check simulates removing pkgs (all of allowed when none are given) and
// refuses when apt would also remove a package that is not in allowed.
func check(ctx context.Context, r run.Runner, verb string, allowed []string, pkgs ...string) error {
	if len(pkgs) == 0 {
		pkgs = allowed
	}
	out, err := r.Output(ctx, "apt-get", append([]string{"-s", "-q", verb}, pkgs...)...)
	if err != nil {
		return err
	}
	var extra []string
	for _, p := range simulated(out) {
		if !slices.Contains(allowed, p) {
			extra = append(extra, p)
		}
	}
	if len(extra) > 0 {
		return fmt.Errorf("%w: removing %s would also remove %s, which depend on it. Nothing was removed",
			ErrWouldRemoveMore, strings.Join(pkgs, " "), strings.Join(extra, " "))
	}
	return nil
}
