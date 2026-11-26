package software

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/usererr"
)

// Several Node.js release lines can be installed side by side, each in
// /opt/node-v<version>-linux-<arch>. App sites choose one; the default
// node in /usr/local/bin is the one the "Node.js" item installed.

// NodeVersion is an installed or installable Node.js release line.
type NodeVersion struct {
	Major     int    `json:"major"`
	Version   string `json:"version"`       // installed version, or the newest available
	LTS       string `json:"lts,omitempty"` // codename of an LTS line
	Installed bool   `json:"installed"`
	Path      string `json:"path,omitempty"` // bin folder when installed
	Default   bool   `json:"default"`        // /usr/local/bin/node points here
}

var nodeDirRe = regexp.MustCompile(`^node-v([0-9]+)\.([0-9]+)\.([0-9]+)-linux-(x64|arm64)$`)

// InstalledNode returns the installed Node.js versions by major, newest
// version of each line.
func InstalledNode() map[int]NodeVersion {
	res := map[int]NodeVersion{}
	entries, _ := os.ReadDir("/opt")
	def, _ := os.Readlink("/usr/local/bin/node")
	for _, e := range entries {
		m := nodeDirRe.FindStringSubmatch(e.Name())
		if m == nil || !e.IsDir() {
			continue
		}
		major, _ := strconv.Atoi(m[1])
		v := "v" + m[1] + "." + m[2] + "." + m[3]
		bin := filepath.Join("/opt", e.Name(), "bin")
		if cur, ok := res[major]; ok && versionLess(v, cur.Version) {
			continue
		}
		res[major] = NodeVersion{Major: major, Version: v, Installed: true, Path: bin,
			Default: strings.HasPrefix(def, bin+"/")}
	}
	return res
}

// NodeBin returns the bin folder of an installed Node.js major version.
func NodeBin(major int) (string, bool) {
	v, ok := InstalledNode()[major]
	return v.Path, ok
}

func versionLess(a, b string) bool {
	pa, pb := strings.Split(strings.TrimPrefix(a, "v"), "."), strings.Split(strings.TrimPrefix(b, "v"), ".")
	for i := 0; i < 3 && i < len(pa) && i < len(pb); i++ {
		x, _ := strconv.Atoi(pa[i])
		y, _ := strconv.Atoi(pb[i])
		if x != y {
			return x < y
		}
	}
	return false
}

type nodeRelease struct {
	Version string `json:"version"`
	LTS     any    `json:"lts"`
}

// NodeVersions lists the installed release lines and the current LTS lines
// that can be installed.
func (m *Manager) NodeVersions(ctx context.Context) ([]NodeVersion, error) {
	inst := InstalledNode()
	idx, err := download(ctx, "https://nodejs.org/dist/index.json", 4<<20)
	if err != nil {
		return nil, err
	}
	var rels []nodeRelease
	if err := json.Unmarshal(idx, &rels); err != nil {
		return nil, err
	}
	seen := map[int]bool{}
	var out []NodeVersion
	for _, r := range rels { // newest first
		lts, _ := r.LTS.(string)
		major := majorOf(r.Version)
		if major == 0 || seen[major] || lts == "" {
			continue
		}
		seen[major] = true
		if len(seen) > 3 {
			break // the three newest LTS lines
		}
		v := NodeVersion{Major: major, Version: r.Version, LTS: lts}
		if i, ok := inst[major]; ok {
			v.Version, v.Installed, v.Path, v.Default = i.Version, true, i.Path, i.Default
		}
		out = append(out, v)
	}
	for major, i := range inst {
		if !slices.ContainsFunc(out, func(v NodeVersion) bool { return v.Major == major }) {
			out = append(out, i)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Major > out[b].Major })
	return out, nil
}

func majorOf(v string) int {
	if !regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(v) {
		return 0
	}
	n, _ := strconv.Atoi(strings.SplitN(strings.TrimPrefix(v, "v"), ".", 2)[0])
	return n
}

// InstallNodeMajor installs the newest release of a Node.js line next to
// the others, checked against the published SHA-256. It does not change
// the default node.
func (m *Manager) InstallNodeMajor(ctx context.Context, major int, lg Logger) error {
	arch := map[string]string{"amd64": "x64", "arm64": "arm64"}[runtime.GOARCH]
	if arch == "" {
		return usererr.New("unsupported")
	}
	if major < 18 || major > 99 {
		return usererr.New("invalid_node_version", strconv.Itoa(major))
	}
	idx, err := download(ctx, "https://nodejs.org/dist/index.json", 4<<20)
	if err != nil {
		return err
	}
	var rels []nodeRelease
	if err := json.Unmarshal(idx, &rels); err != nil {
		return err
	}
	ver := ""
	for _, r := range rels {
		if majorOf(r.Version) == major {
			ver = r.Version
			break
		}
	}
	if ver == "" {
		return usererr.New("invalid_node_version", strconv.Itoa(major))
	}
	name := "node-" + ver + "-linux-" + arch
	base := "https://nodejs.org/dist/" + ver + "/"
	sums, err := download(ctx, base+"SHASUMS256.txt", 1<<20)
	if err != nil {
		return err
	}
	want := ""
	sc := bufio.NewScanner(strings.NewReader(string(sums)))
	for sc.Scan() {
		if f := strings.Fields(sc.Text()); len(f) == 2 && f[1] == name+".tar.gz" {
			want = f[0]
		}
	}
	if len(want) != 64 {
		return errors.New("SHASUMS256.txt has no entry for " + name + ".tar.gz")
	}
	lg.Logf("Downloading Node.js %s", ver)
	tmp, err := os.CreateTemp("", "node-*.tar.gz")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	h := sha256.New()
	err = downloadTo(ctx, base+name+".tar.gz", 200<<20, io.MultiWriter(tmp, h))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if hex.EncodeToString(h.Sum(nil)) != want {
		return errors.New("the Node.js download does not match its published checksum")
	}
	lg.Logf("Extracting to /opt/%s", name)
	if _, err := m.run.Output(ctx, "/bin/sh", "-c", `umask 022 && exec tar -xzf "$1" -C /opt --no-same-owner`, "sh", tmp.Name()); err != nil {
		return err
	}
	if err := m.nodeLibraries(ctx, lg); err != nil {
		return err
	}
	lg.Logf("Node.js %s is installed. Choose it for an app website in its settings.", ver)
	return nil
}

// nodeLibraries installs libatomic1, which the nodejs.org builds need and
// minimal systems (containers, some cloud images) do not have. When another
// package brought it in, it is marked as wanted, so that removing that
// package does not take it away from Node.js.
func (m *Manager) nodeLibraries(ctx context.Context, lg Logger) error {
	for _, g := range []string{"/usr/lib/*-linux-gnu/libatomic.so.1", "/lib/*-linux-gnu/libatomic.so.1"} {
		if found, _ := filepath.Glob(g); len(found) > 0 {
			m.run.Output(ctx, "apt-mark", "manual", "libatomic1")
			return nil
		}
	}
	lg.Logf("Installing libatomic1, which Node.js needs")
	if err := run.Apt(ctx, m.run, lg, "update"); err != nil {
		return err
	}
	return run.Apt(ctx, m.run, lg, "install", "--no-install-recommends", "libatomic1")
}

// RemoveNodeMajor removes an installed Node.js line unless it is the
// default or used (inUse lists the majors app sites use).
func (m *Manager) RemoveNodeMajor(ctx context.Context, major int, inUse []int, lg Logger) error {
	v, ok := InstalledNode()[major]
	if !ok {
		return usererr.New("invalid_node_version", strconv.Itoa(major))
	}
	if v.Default {
		return usererr.New("node_is_default")
	}
	if slices.Contains(inUse, major) {
		return usererr.New("node_in_use", strconv.Itoa(major))
	}
	dir := filepath.Dir(v.Path)
	if !nodeDirRe.MatchString(filepath.Base(dir)) || filepath.Dir(dir) != "/opt" {
		return fmt.Errorf("refusing to remove %s", dir)
	}
	lg.Logf("Removing %s", dir)
	return os.RemoveAll(dir)
}

// PythonReady reports whether Python virtual environments with pip can be
// created for the default python3. The venv module itself is always there
// (libpython3.X-stdlib); on Debian and Ubuntu, ensurepip, which puts pip into
// a new environment, only comes with python3.X-venv. Without it
// "python3 -m venv" fails halfway.
func PythonReady() bool {
	bin, err := filepath.EvalSymlinks("/usr/bin/python3")
	if err != nil {
		return false
	}
	return fileExists(filepath.Join("/usr/lib", filepath.Base(bin), "ensurepip", "__init__.py"))
}

// InstallPython installs what Python apps need: venv and pip.
func (m *Manager) InstallPython(ctx context.Context, lg Logger) error {
	lg.Logf("Installing python3-venv and python3-pip")
	return run.Apt(ctx, m.run, lg, "install", "-y", "--no-install-recommends", "python3-venv", "python3-pip")
}
