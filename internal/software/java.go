package software

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/software/apt"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// Java for app websites: OpenJDK from the distribution, the headless JDK
// (java, and javac, jar and the rest for builds in Git deploy scripts),
// several long-term support versions side by side in
// /usr/lib/jvm/java-<major>-openjdk-<arch>. An app website picks one
// ("java:21"); its service and deploy scripts get that version's JAVA_HOME,
// with its bin folder first in PATH. Which versions a system has depends
// on the release: Debian 12 has 17, Debian 13 has 21 and 25, Ubuntu has
// 17, 21 and 25. Nothing runs while no website uses it.

// JVMDir is where the OpenJDK packages install; tests (also hosting's)
// point it elsewhere.
var JVMDir = "/usr/lib/jvm"

var jvmDirRe = regexp.MustCompile(`^java-([0-9]{1,3})-openjdk-([a-z0-9]+)$`)

// JavaVersion is an installed or installable Java version.
type JavaVersion struct {
	Major int `json:"major"`
	// Version is the installed release (such as "21.0.9"), or the one the
	// distribution offers.
	Version   string `json:"version,omitempty"`
	Installed bool   `json:"installed"`
	Home      string `json:"home,omitempty"` // JAVA_HOME when installed
	// Available: the distribution has it (openjdk-<major>-jdk-headless).
	Available bool `json:"available"`
	LTS       bool `json:"lts"`
}

// javaLTS reports whether a Java major version gets long-term support:
// 17, 21, 25, then every fourth (8 and 11 were too, but are old).
func javaLTS(major int) bool { return major >= 17 && (major-17)%4 == 0 }

func javaPackage(major int) string { return "openjdk-" + strconv.Itoa(major) + "-jdk-headless" }

// InstalledJava returns the installed OpenJDK versions by major.
func InstalledJava() map[int]JavaVersion {
	res := map[int]JavaVersion{}
	entries, _ := os.ReadDir(JVMDir)
	for _, e := range entries {
		m := jvmDirRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		home := filepath.Join(JVMDir, e.Name())
		// A symlink such as default-java is not a version; the real
		// folder (or one that dpkg left without a java) is skipped too.
		if fi, err := os.Lstat(home); err != nil || !fi.IsDir() || !exists(filepath.Join(home, "bin", "java")) {
			continue
		}
		major, _ := strconv.Atoi(m[1])
		if _, ok := res[major]; ok && m[2] != debArch() {
			continue // another architecture's (multiarch): this one's wins
		}
		res[major] = JavaVersion{Major: major, Version: javaRelease(home), Installed: true, Home: home, LTS: javaLTS(major)}
	}
	return res
}

// JavaHome returns the JAVA_HOME of an installed Java major version.
func JavaHome(major int) (string, bool) {
	v, ok := InstalledJava()[major]
	return v.Home, ok
}

// javaRelease reads JAVA_VERSION from a JDK's release file ("21.0.9").
func javaRelease(home string) string {
	b, _ := os.ReadFile(filepath.Join(home, "release"))
	v := strings.Trim(osField(b, "JAVA_VERSION"), `"`)
	if !regexp.MustCompile(`^[0-9][0-9.+_a-z-]{0,30}$`).MatchString(v) {
		return ""
	}
	return v
}

// debArch is the Debian name of this machine's architecture, as in the
// JDK folder names.
func debArch() string {
	switch a := legacyArch; a {
	case "amd64", "arm64":
		return a
	case "arm":
		return "armhf"
	case "386":
		return "i386"
	default:
		return a
	}
}

// javaPolicyRe finds the package sections of "apt-cache policy" output.
var javaPolicyRe = regexp.MustCompile(`^openjdk-([0-9]{1,3})-jdk-headless:`)

// javaCandCache keeps apt's answer until its package lists change (apt
// takes seconds on a small server).
var javaCandCache struct {
	sync.Mutex
	key  string
	cand map[int]string
}

// aptListsStamp changes whenever "apt-get update" changed the lists.
func aptListsStamp() string {
	dir := sysPath("/var/lib/apt/lists")
	fi, err := os.Stat(dir)
	if err != nil {
		return ""
	}
	return dir + "|" + fi.ModTime().Format(time.RFC3339Nano)
}

// javaCandidates asks apt which OpenJDK versions the configured
// repositories have: the long-term ones from 17 on, by major, with the
// version a new install would get.
func (m *Manager) javaCandidates(ctx context.Context) map[int]string {
	stamp := aptListsStamp()
	javaCandCache.Lock()
	defer javaCandCache.Unlock()
	if stamp != "" && javaCandCache.key == stamp && javaCandCache.cand != nil {
		return javaCandCache.cand
	}
	cand := m.askJavaCandidates(ctx)
	if len(cand) > 0 {
		javaCandCache.key, javaCandCache.cand = stamp, cand
	}
	return cand
}

func (m *Manager) askJavaCandidates(ctx context.Context) map[int]string {
	majors := []int{17, 21, 25}
	// Newer long-term versions are found by name, so 29 shows up by itself.
	if out, err := m.run.Output(ctx, "apt-cache", "pkgnames", "openjdk-"); err == nil {
		for _, name := range strings.Fields(out) {
			if mm := regexp.MustCompile(`^openjdk-([0-9]{1,3})-jdk-headless$`).FindStringSubmatch(name); mm != nil {
				if n, _ := strconv.Atoi(mm[1]); javaLTS(n) && !slices.Contains(majors, n) {
					majors = append(majors, n)
				}
			}
		}
	}
	names := []string{}
	for _, n := range majors {
		names = append(names, javaPackage(n))
	}
	out, _ := m.run.Output(ctx, "apt-cache", append([]string{"policy"}, names...)...)
	return parseJavaPolicy(out)
}

// parseJavaPolicy reads the candidate of each openjdk-<major>-jdk-headless
// section of "apt-cache policy" output.
func parseJavaPolicy(out string) map[int]string {
	res := map[int]string{}
	major := 0
	for _, line := range strings.Split(out, "\n") {
		if mm := javaPolicyRe.FindStringSubmatch(line); mm != nil {
			major, _ = strconv.Atoi(mm[1])
			continue
		}
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "Candidate:"); ok && major > 0 {
			if v = strings.TrimSpace(v); v != "" && v != "(none)" {
				res[major] = upstreamVersion(v)
			}
			major = 0
		}
	}
	return res
}

// upstreamVersion turns a Debian version such as "21.0.9+10-1~24.04" or
// "1:17.0.17+10-1" into "21.0.9" or "17.0.17".
func upstreamVersion(v string) string {
	if _, after, ok := strings.Cut(v, ":"); ok {
		v = after
	}
	if i := strings.IndexAny(v, "+-~"); i > 0 {
		v = v[:i]
	}
	return v
}

// InstalledJavaVersions lists the installed Java versions, newest first.
func InstalledJavaVersions() []JavaVersion {
	out := []JavaVersion{}
	for _, v := range InstalledJava() {
		out = append(out, v)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Major > out[b].Major })
	return out
}

// JavaVersions lists the installed Java versions and the long-term ones
// the distribution offers, newest first.
func (m *Manager) JavaVersions(ctx context.Context) []JavaVersion {
	inst := InstalledJava()
	cand := m.javaCandidates(ctx)
	out := []JavaVersion{}
	for major, v := range cand {
		jv := JavaVersion{Major: major, Version: v, Available: true, LTS: javaLTS(major)}
		if i, ok := inst[major]; ok {
			jv.Version, jv.Installed, jv.Home = i.Version, true, i.Home
		}
		out = append(out, jv)
	}
	for major, i := range inst {
		if _, ok := cand[major]; !ok {
			out = append(out, i)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Major > out[b].Major })
	return out
}

// javaUsers counts the app websites that run with Java, by major.
func javaUsers(sites []*store.Site) map[int]int {
	users := map[int]int{}
	for _, s := range sites {
		if major, ok := JavaMajor(s.AppRuntime); ok {
			users[major]++
		}
	}
	return users
}

// JavaMajor returns the major version of a "java:<major>" runtime.
func JavaMajor(runtime string) (int, bool) {
	n, ok := strings.CutPrefix(runtime, "java:")
	if !ok {
		return 0, false
	}
	major, err := strconv.Atoi(n)
	return major, err == nil && major > 0
}

// JavaItem is the Java card of the Software page: installed when at least
// one version is, with the newest one's release as its version.
func (m *Manager) javaItem(sites []*store.Site) Item {
	inst := InstalledJava()
	it := Item{ID: "java", Kind: "java", Group: "runtimes", Installed: len(inst) > 0, Lines: len(inst), Source: "apt"}
	if !it.Installed {
		return it
	}
	newest := 0
	for major := range inst {
		newest = max(newest, major)
	}
	it.Version = inst[newest].Version
	n := 0
	for _, c := range javaUsers(sites) {
		n += c
	}
	it.Usage = map[string]int{"sites": n}
	it.Block(javaBlocker(0, n))
	return it
}

// javaBlocker: a version app websites use stays (major 0: any version).
func javaBlocker(major, users int) error {
	if users == 0 {
		return nil
	}
	if major == 0 {
		return usererr.New("java_in_use_sites", strconv.Itoa(users))
	}
	return usererr.New("java_in_use", strconv.Itoa(major))
}

// InstallJava installs a Java version from the distribution (major 0: the
// newest long-term version it has).
func (m *Manager) InstallJava(ctx context.Context, major int, lg Logger) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.installJava(ctx, major, lg)
}

func (m *Manager) installJava(ctx context.Context, major int, lg Logger) error {
	if major != 0 && (major < 8 || major > 999) {
		return usererr.New("invalid_java_version", strconv.Itoa(major))
	}
	if err := run.Apt(ctx, m.run, lg, "update"); err != nil {
		return err
	}
	cand := m.javaCandidates(ctx)
	if major == 0 {
		for n := range cand {
			if javaLTS(n) {
				major = max(major, n)
			}
		}
		if major == 0 {
			b, _ := os.ReadFile(osReleaseFile)
			return usererr.New("java_not_published", osName(b))
		}
	}
	if _, ok := cand[major]; !ok {
		b, _ := os.ReadFile(osReleaseFile)
		return usererr.New("java_not_published", "Java "+strconv.Itoa(major)+", "+osName(b))
	}
	lg.Logf("Installing Java %d (OpenJDK %s from the distribution, without a desktop)", major, cand[major])
	if err := run.Apt(ctx, m.run, lg, "install", "--no-install-recommends", javaPackage(major)); err != nil {
		return err
	}
	home, ok := JavaHome(major)
	if !ok {
		return fmt.Errorf("%s is installed, but %s has no java-%d-openjdk folder with bin/java", javaPackage(major), JVMDir, major)
	}
	lg.Logf("Java %d is installed in %s. Choose it for an app website in its settings.", major, home)
	return nil
}

// javaPackages are the packages of a Java version that a removal takes.
func javaPackages(major int) []string {
	n := strconv.Itoa(major)
	var pkgs []string
	for _, p := range []string{"jdk-headless", "jre-headless", "jdk", "jre", "source", "doc", "demo", "dbg", "jre-zero"} {
		pkgs = append(pkgs, "openjdk-"+n+"-"+p)
	}
	return pkgs
}

// RemoveJava removes a Java version no app website uses, with the
// packages only it needed.
func (m *Manager) RemoveJava(ctx context.Context, major int, lg Logger) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.removeJava(ctx, major, lg)
}

func (m *Manager) removeJava(ctx context.Context, major int, lg Logger) error {
	if _, ok := InstalledJava()[major]; !ok {
		return usererr.New("software_not_installed", "Java "+strconv.Itoa(major))
	}
	if err := m.javaRemovable(ctx, major); err != nil {
		return err
	}
	pkgs, err := apt.Installed(ctx, m.run, true, javaPackages(major)...)
	if err != nil {
		return err
	}
	if len(pkgs) == 0 {
		return fmt.Errorf("Java %d was not installed from packages (no openjdk-%d package), so the panel does not remove it", major, major)
	}
	lg.Logf("Removing Java %d", major)
	if err := apt.Remove(ctx, m.run, lg, true, pkgs...); err != nil {
		return err
	}
	lg.Logf("Java %d is removed.", major)
	return nil
}

// javaRemovable returns why Java major (0: every version) cannot be
// removed now, or nil.
func (m *Manager) javaRemovable(ctx context.Context, major int) error {
	sites, err := m.store.ListSites(ctx)
	if err != nil {
		return err
	}
	users := javaUsers(sites)
	if major != 0 {
		return javaBlocker(major, users[major])
	}
	n := 0
	for _, c := range users {
		n += c
	}
	return javaBlocker(0, n)
}

// CheckRemoveJava returns why Java major cannot be removed now (a
// usererr), or nil. It changes nothing.
func (m *Manager) CheckRemoveJava(ctx context.Context, major int) error {
	if _, ok := InstalledJava()[major]; !ok {
		return usererr.New("software_not_installed", "Java "+strconv.Itoa(major))
	}
	return m.javaRemovable(ctx, major)
}

// removeAllJava is the card's uninstall: every installed version.
func (m *Manager) removeAllJava(ctx context.Context, lg Logger) error {
	inst := InstalledJava()
	majors := []int{}
	for major := range inst {
		majors = append(majors, major)
	}
	sort.Ints(majors)
	var errs []error
	for _, major := range majors {
		if err := m.removeJava(ctx, major, lg); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// JavaInUse lists the Java majors app websites use.
func JavaInUse(sites []*store.Site) []int {
	out := []int{}
	for major := range javaUsers(sites) {
		out = append(out, major)
	}
	sort.Ints(out)
	return out
}
