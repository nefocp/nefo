package software

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/nefocp/nefo/internal/docker"
	"github.com/nefocp/nefo/internal/software/apt"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// Uninstalling. Data is never deleted silently: a removal is refused while
// websites, containers or Compose projects still use the software, and
// Docker's images and volumes stay unless the administrator asks for them
// to be deleted and types the name to confirm.

// RemoveOptions are the administrator's choices for an uninstall.
type RemoveOptions struct {
	// DeleteData also deletes the data folder (Docker: images, volumes and
	// build cache). Confirm must then be the item's ID.
	DeleteData bool   `json:"delete_data"`
	Confirm    string `json:"confirm"`
}

// Where the software lives; tests point these elsewhere.
var (
	redisBin    = "/usr/bin/redis-server"
	dockerBin   = "/usr/bin/docker"
	nodeLinkDir = "/usr/local/bin"
	optDir      = "/opt"
	composeDir  = docker.ComposeDir
	// dockerDataDirs hold images, containers, volumes and build cache.
	dockerDataDirs      = []string{"/var/lib/docker", "/var/lib/containerd"}
	dockerContainersDir = "/var/lib/docker/containers"
)

// RedisInstalled reports whether the Redis server is installed.
func RedisInstalled() bool { return exists(redisBin) }

// NodeInstalled reports whether a default node is in /usr/local/bin.
func NodeInstalled() bool { return exists(filepath.Join(nodeLinkDir, "node")) }

// DockerInstalled reports whether Docker is installed.
func DockerInstalled() bool { return exists(dockerBin) }

// CheckRemove returns why an item cannot be uninstalled now (a usererr),
// or nil. It changes nothing.
func (m *Manager) CheckRemove(ctx context.Context, id string, opts RemoveOptions) error {
	if v, ok := strings.CutPrefix(id, "php"); ok {
		return m.phpRemovable(ctx, v)
	}
	sites, err := m.store.ListSites(ctx)
	if err != nil {
		return err
	}
	switch id {
	case "redis":
		if !RedisInstalled() {
			return usererr.New("software_not_installed", "Redis")
		}
		return nil
	case "nodejs":
		_, major, err := defaultNode()
		if err != nil {
			return err
		}
		return nodeBlocker(major, defaultNodeUsers(sites))
	case "python":
		if !PythonReady() {
			return usererr.New("software_not_installed", "Python")
		}
		return pythonBlocker(pythonUsers(sites))
	case "docker":
		if !DockerInstalled() {
			return usererr.New("software_not_installed", "Docker")
		}
		if opts.DeleteData && strings.TrimSpace(opts.Confirm) != "docker" {
			return usererr.New("software_confirm_mismatch", "docker")
		}
		return m.dockerUsage(ctx, sites).blocker()
	}
	return usererr.New("unknown_software", id)
}

// Remove uninstalls an item after checking that it may be (see CheckRemove).
func (m *Manager) Remove(ctx context.Context, id string, opts RemoveOptions, lg Logger) error {
	if v, ok := strings.CutPrefix(id, "php"); ok {
		return m.RemovePHP(ctx, v, lg)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.CheckRemove(ctx, id, opts); err != nil {
		return err
	}
	switch id {
	case "redis":
		return m.removeRedis(ctx, lg)
	case "nodejs":
		return m.removeDefaultNode(ctx, lg)
	case "python":
		return m.removePython(ctx, lg)
	case "docker":
		return m.removeDocker(ctx, opts.DeleteData, lg)
	}
	return usererr.New("unknown_software", id)
}

// ---- Redis ----

var phpRedisRe = regexp.MustCompile(`^php([0-9]+\.[0-9]+)-redis$`)

// removeRedis purges Redis and the PHP extensions for it, then restarts
// the PHP versions that had the extension loaded.
func (m *Manager) removeRedis(ctx context.Context, lg Logger) error {
	pkgs, err := apt.Installed(ctx, m.run, true, "redis-server", "redis-tools", "redis", "php*-redis")
	if err != nil {
		return err
	}
	var versions []string
	for _, p := range pkgs {
		if mm := phpRedisRe.FindStringSubmatch(p); mm != nil && phpInstalled(mm[1]) {
			versions = append(versions, mm[1])
		}
	}
	if err := apt.Check(ctx, m.run, true, pkgs...); err != nil {
		return err
	}
	// The package scripts stop it too, unless a policy forbids them.
	lg.Logf("Stopping Redis")
	m.run.Output(ctx, "systemctl", "disable", "--now", "redis-server")
	lg.Logf("Removing Redis and the PHP extensions for it")
	if err := apt.Remove(ctx, m.run, lg, true, pkgs...); err != nil {
		return err
	}
	for _, v := range versions {
		lg.Logf("Restarting PHP %s without the Redis extension", v)
		if out, err := m.run.Output(ctx, "systemctl", "restart", "php"+v+"-fpm"); err != nil {
			lg.Logf("Warning: restarting php%s-fpm failed: %s", v, strings.TrimSpace(out))
		}
	}
	lg.Logf("Redis is removed. Websites that used it as an object cache now work without one.")
	return nil
}

// ---- Node.js ----

// defaultNode returns the folder the default node in /usr/local/bin links
// into, and its major version, when the panel installed it there.
func defaultNode() (dir string, major int, err error) {
	link := filepath.Join(nodeLinkDir, "node")
	target, rerr := os.Readlink(link)
	if rerr != nil {
		if exists(link) {
			return "", 0, usererr.New("node_not_managed")
		}
		return "", 0, usererr.New("software_not_installed", "Node.js")
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(nodeLinkDir, target)
	}
	bin := filepath.Dir(filepath.Clean(target))
	dir = filepath.Dir(bin)
	mm := nodeDirRe.FindStringSubmatch(filepath.Base(dir))
	if mm == nil || filepath.Base(bin) != "bin" || filepath.Dir(dir) != filepath.Clean(optDir) {
		return "", 0, usererr.New("node_not_managed")
	}
	major, _ = strconv.Atoi(mm[1])
	return dir, major, nil
}

// defaultNodeUsers counts the app websites that run with the server's
// default node: app sites (a proxy port) with no runtime chosen, other
// than Docker apps, which bring their own.
func defaultNodeUsers(sites []*store.Site) int {
	n := 0
	for _, s := range sites {
		if s.ProxyPort > 0 && s.AppRuntime == "" && !strings.HasPrefix(s.App, "docker:") {
			n++
		}
	}
	return n
}

func nodeBlocker(major, users int) error {
	if users > 0 {
		return usererr.New("node_in_use", strconv.Itoa(major))
	}
	return nil
}

// removeDefaultNode removes node, npm, npx and corepack from /usr/local/bin
// and the release they point to, unless app websites chose that release
// line ("node:22"): then the folder stays as one of the extra lines.
func (m *Manager) removeDefaultNode(ctx context.Context, lg Logger) error {
	dir, major, err := defaultNode()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(nodeLinkDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		p := filepath.Join(nodeLinkDir, e.Name())
		if t, err := os.Readlink(p); err == nil && strings.HasPrefix(filepath.Clean(t), dir+string(filepath.Separator)) {
			if err := os.Remove(p); err != nil {
				return err
			}
			lg.Logf("Removed %s", p)
		}
	}
	sites, err := m.store.ListSites(ctx)
	if err != nil {
		return err
	}
	for _, s := range sites {
		if s.AppRuntime == "node:"+strconv.Itoa(major) {
			lg.Logf("Kept %s: app websites chose Node.js %d in their settings. Remove it under Node.js versions when they no longer need it.", dir, major)
			return nil
		}
	}
	lg.Logf("Removing %s", dir)
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	lg.Logf("Node.js is removed. Other Node.js versions installed for app websites stay.")
	return nil
}

// ---- Python ----

// pythonVersion returns the default python3's version, such as "3.12".
func pythonVersion() string {
	bin, err := filepath.EvalSymlinks("/usr/bin/python3")
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(filepath.Base(bin), "python")
}

func pythonUsers(sites []*store.Site) int {
	n := 0
	for _, s := range sites {
		if s.AppRuntime == "python" {
			n++
		}
	}
	return n
}

func pythonBlocker(users int) error {
	if users > 0 {
		return usererr.New("python_in_use", strconv.Itoa(users))
	}
	return nil
}

var pyVersionRe = regexp.MustCompile(`^3\.[0-9]{1,2}$`)

// removePython removes venv and pip. python3 itself stays: the system
// needs it.
func (m *Manager) removePython(ctx context.Context, lg Logger) error {
	names := []string{"python3-venv", "python3-pip"}
	if v := pythonVersion(); pyVersionRe.MatchString(v) {
		names = append(names, "python"+v+"-venv")
	}
	pkgs, err := apt.Installed(ctx, m.run, true, names...)
	if err != nil {
		return err
	}
	lg.Logf("Removing venv and pip (python3 itself stays)")
	if err := apt.Remove(ctx, m.run, lg, true, pkgs...); err != nil {
		return err
	}
	lg.Logf("Python app support is removed.")
	return nil
}

// ---- Docker ----

// dockerPackages are Docker's packages from the distribution (docker.io)
// and from Docker's own repository (docker-ce).
var dockerPackages = []string{"docker.io", "docker-compose-v2", "docker-buildx", "docker-compose", "docker-doc",
	"docker-ce", "docker-ce-cli", "docker-ce-rootless-extras", "containerd.io",
	"docker-buildx-plugin", "docker-compose-plugin", "docker-model-plugin"}

type dockerUse struct{ apps, projects, containers int }

// dockerUsage counts the Docker app websites, Compose projects and
// containers (running or not).
func (m *Manager) dockerUsage(ctx context.Context, sites []*store.Site) dockerUse {
	var u dockerUse
	for _, s := range sites {
		if strings.HasPrefix(s.App, "docker:") {
			u.apps++
		}
	}
	if entries, err := os.ReadDir(composeDir); err == nil {
		for _, e := range entries {
			if e.IsDir() && docker.ValidProject(e.Name()) == nil {
				u.projects++
			}
		}
	}
	u.containers = m.dockerContainers(ctx)
	return u
}

var containerIDRe = regexp.MustCompile(`^[0-9a-f]{12,64}$`)

// dockerContainers counts all containers. When the Docker service is not
// running, it counts the containers' folders instead: asking the docker
// command would start the service through its socket.
func (m *Manager) dockerContainers(ctx context.Context) int {
	n := 0
	if active, _ := m.run.Output(ctx, "systemctl", "is-active", "docker"); strings.TrimSpace(active) == "active" {
		if out, err := m.run.Output(ctx, "docker", "ps", "-aq", "--no-trunc"); err == nil {
			for _, l := range strings.Fields(out) {
				if containerIDRe.MatchString(l) {
					n++
				}
			}
			return n
		}
	}
	entries, _ := os.ReadDir(dockerContainersDir)
	for _, e := range entries {
		if e.IsDir() && containerIDRe.MatchString(e.Name()) {
			n++
		}
	}
	return n
}

func (u dockerUse) blocker() error {
	switch {
	case u.apps > 0:
		return usererr.New("docker_apps_in_use", strconv.Itoa(u.apps))
	case u.projects > 0:
		return usererr.New("docker_compose_in_use", strconv.Itoa(u.projects))
	case u.containers > 0:
		return usererr.New("docker_in_use", strconv.Itoa(u.containers))
	}
	return nil
}

// removeDocker removes Docker. Its images, volumes and build cache stay in
// /var/lib/docker unless deleteData is set; then the packages are purged
// and the folders deleted.
func (m *Manager) removeDocker(ctx context.Context, deleteData bool, lg Logger) error {
	pkgs, err := apt.Installed(ctx, m.run, deleteData, dockerPackages...)
	if err != nil {
		return err
	}
	if len(pkgs) == 0 {
		return fmt.Errorf("%s was not installed from a Debian package; remove it on the command line", dockerBin)
	}
	if err := apt.Check(ctx, m.run, deleteData, pkgs...); err != nil {
		return err
	}
	// The socket first: it would start the service again.
	lg.Logf("Stopping Docker")
	m.run.Output(ctx, "systemctl", "disable", "--now", "docker.socket", "docker.service")
	lg.Logf("Removing Docker")
	// apt-get remove keeps the data; purging docker.io runs a script that
	// deletes /var/lib/docker.
	if err := apt.Remove(ctx, m.run, lg, deleteData, pkgs...); err != nil {
		return err
	}
	if !deleteData {
		// docker.io leaves that script behind for a later purge: without it,
		// purging the remains of the package cannot delete the kept data.
		nuke := filepath.Join(dockerDataDirs[0], "nuke-graph-directory.sh")
		if exists(nuke) {
			os.Remove(nuke)
		}
		lg.Logf("Docker is removed. Its images, volumes and build cache were kept in %s; installing Docker again picks them up.", dockerDataDirs[0])
		return nil
	}
	for _, d := range dockerDataDirs {
		if !exists(d) {
			continue
		}
		lg.Logf("Deleting %s", d)
		if err := os.RemoveAll(d); err != nil {
			return err
		}
	}
	lg.Logf("Docker is removed, with its images, volumes and build cache.")
	return nil
}
