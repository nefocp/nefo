package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/usererr"
)

// Compose projects are groups of containers described by a compose.yaml
// that the administrator writes. Each is kept in its own folder and runs as
// the Compose project "panel-<name>". Before every start the panel asks
// Compose for the final configuration (with includes, extends and variables
// resolved) and refuses what would weaken the server: ports published
// beyond 127.0.0.1, because Docker's port publishing bypasses ufw;
// privileged containers; added capabilities, devices, changed security
// options (seccomp, AppArmor) and kernel settings (sysctls); the host's
// network, PID, IPC, user or cgroup namespace; and mounts of system folders
// or of the Docker socket, which give a container control of the server.
// The app store's projects go through the same checks (CheckConfig), with
// no exceptions.

const (
	// ComposeDir holds one folder per project.
	ComposeDir = "/var/lib/panel-compose"
	// MaxComposeSize is the largest compose.yaml the panel accepts.
	MaxComposeSize = 64 << 10
	projectPrefix  = "panel-"
	composeFile    = "compose.yaml"
)

var projectRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)

// ErrNoProject is returned for a Compose project that does not exist.
var ErrNoProject = errors.New("no such compose project")

// Logger receives a job's progress messages and program output.
type Logger interface {
	io.Writer
	Logf(format string, args ...any)
}

// Project is one Compose project.
type Project struct {
	Name       string    `json:"name"`
	State      string    `json:"state"`  // running, partial, stopped, down, unknown
	Status     string    `json:"status"` // as Compose reports it, such as "running(2)"
	Running    int       `json:"running"`
	Containers int       `json:"containers"`
	Updated    time.Time `json:"updated"` // when compose.yaml was last saved
}

// ProjectContainer is a container of a Compose project.
type ProjectContainer struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Service string `json:"service"`
	Image   string `json:"image"`
	State   string `json:"state"`
	Status  string `json:"status"`
	Health  string `json:"health"`
	Ports   string `json:"ports"`
}

// ValidProject checks a project name.
func ValidProject(name string) error {
	if !projectRe.MatchString(name) {
		return usererr.New("invalid_project", name)
	}
	return nil
}

func (m *Manager) projectDir(name string) string { return filepath.Join(m.composeDir, name) }
func (m *Manager) projectFile(name string) string {
	return filepath.Join(m.projectDir(name), composeFile)
}

func (m *Manager) projectExists(name string) bool {
	_, err := os.Stat(m.projectFile(name))
	return err == nil
}

// existing checks the name of a project that must exist.
func (m *Manager) existing(name string) error {
	if err := ValidProject(name); err != nil {
		return err
	}
	if !m.projectExists(name) {
		return ErrNoProject
	}
	return nil
}

// composeArgs builds a docker compose command line for a project. The
// project directory is set explicitly, so relative paths in the file
// always mean the project's folder, even while a draft is checked.
func (m *Manager) composeArgs(name, file string, args ...string) []string {
	return append([]string{"compose", "--ansi", "never", "--project-directory", m.projectDir(name),
		"-f", file, "-p", projectPrefix + name}, args...)
}

// ComposeInstalled reports whether the docker compose plugin works.
func (m *Manager) ComposeInstalled(ctx context.Context) bool {
	_, err := m.run.Output(ctx, "docker", "compose", "version")
	return err == nil
}

// EnsureCompose installs the Compose plugin if it is missing: the
// distribution's docker-compose-v2, or Docker's docker-compose-plugin when
// Docker itself comes from Docker's repository. apt may not remove any
// package to do it.
func (m *Manager) EnsureCompose(ctx context.Context, lg Logger) error {
	if m.ComposeInstalled(ctx) {
		return nil
	}
	pkg := "docker-compose-v2"
	if out, _ := m.run.Output(ctx, "dpkg-query", "-W", "-f", "${Status}", "docker-ce"); strings.Contains(out, "install ok installed") {
		pkg = "docker-compose-plugin"
	}
	lg.Logf("Installing the Docker Compose plugin (%s)", pkg)
	if err := run.Apt(ctx, m.run, lg, "update"); err != nil {
		return err
	}
	if err := run.Apt(ctx, m.run, lg, "install", "-y", "--no-remove", pkg); err != nil {
		return err
	}
	if !m.ComposeInstalled(ctx) {
		return fmt.Errorf("docker compose still does not work after installing %s", pkg)
	}
	return nil
}

// ---- listing ----

// Projects lists the projects with the state of their containers.
func (m *Manager) Projects(ctx context.Context) ([]Project, error) {
	entries, err := os.ReadDir(m.composeDir)
	if errors.Is(err, fs.ErrNotExist) {
		return []Project{}, nil
	}
	if err != nil {
		return nil, err
	}
	// Without the Compose plugin the projects are listed without a state.
	out, lsErr := m.stdout(ctx, "compose", "ls", "--all", "--format", "json")
	status := map[string]string{}
	if lsErr == nil {
		status, lsErr = parseComposeLs(out)
	}
	list := []Project{}
	for _, e := range entries {
		if !e.IsDir() || ValidProject(e.Name()) != nil {
			continue
		}
		info, err := os.Stat(m.projectFile(e.Name()))
		if err != nil {
			continue
		}
		p := Project{Name: e.Name(), Updated: info.ModTime().UTC(), State: "unknown"}
		if lsErr == nil {
			p.Status = status[projectPrefix+e.Name()]
			p.Running, p.Containers = countStatus(p.Status)
			p.State = projectState(p.Running, p.Containers)
		}
		list = append(list, p)
	}
	return list, nil
}

// parseComposeLs reads `docker compose ls --format json`:
// [{"Name":"panel-web","Status":"running(2)","ConfigFiles":"…"}].
func parseComposeLs(out string) (map[string]string, error) {
	status := map[string]string{}
	if strings.TrimSpace(out) == "" {
		return status, nil
	}
	var list []struct{ Name, Status string }
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &list); err != nil {
		return nil, fmt.Errorf("read docker compose ls: %w", err)
	}
	for _, p := range list {
		status[p.Name] = p.Status
	}
	return status, nil
}

var statusCountRe = regexp.MustCompile(`([a-z]+)\((\d+)\)`)

// countStatus reads a status such as "running(2), exited(1)".
func countStatus(s string) (running, total int) {
	for _, m := range statusCountRe.FindAllStringSubmatch(s, -1) {
		n, _ := strconv.Atoi(m[2])
		total += n
		if m[1] == "running" {
			running += n
		}
	}
	return running, total
}

func projectState(running, total int) string {
	switch {
	case total == 0:
		return "down"
	case running == total:
		return "running"
	case running == 0:
		return "stopped"
	}
	return "partial"
}

// ProjectFile returns a project's compose.yaml.
func (m *Manager) ProjectFile(name string) (string, error) {
	if err := m.existing(name); err != nil {
		return "", err
	}
	b, err := os.ReadFile(m.projectFile(name))
	return string(b), err
}

// ProjectContainers lists a project's containers, stopped ones included.
func (m *Manager) ProjectContainers(ctx context.Context, name string) ([]ProjectContainer, error) {
	if err := m.existing(name); err != nil {
		return nil, err
	}
	out, err := m.stdout(ctx, m.composeArgs(name, m.projectFile(name), "ps", "--all", "--format", "json")...)
	if err != nil {
		return nil, err
	}
	return parseComposePs(out)
}

// parseComposePs reads `docker compose ps --format json`: one object per
// line since Compose 2.21, a JSON array before.
func parseComposePs(out string) ([]ProjectContainer, error) {
	list := []ProjectContainer{}
	err := eachJSON(out, func(b []byte) error {
		var c struct {
			ID, Name, Service, Image, State, Status, Health string
			Ports                                           json.RawMessage
			Publishers                                      []struct {
				URL           string
				TargetPort    int
				PublishedPort int
				Protocol      string
			}
		}
		if err := json.Unmarshal(b, &c); err != nil {
			return fmt.Errorf("read docker compose ps: %w", err)
		}
		pc := ProjectContainer{ID: shortID(c.ID), Name: c.Name, Service: c.Service, Image: c.Image,
			State: c.State, Status: c.Status, Health: c.Health}
		var published []string
		for _, p := range c.Publishers {
			if p.PublishedPort != 0 {
				published = append(published, fmt.Sprintf("%s:%d->%d/%s", p.URL, p.PublishedPort, p.TargetPort, p.Protocol))
			}
		}
		pc.Ports = strings.Join(slices.Compact(published), ", ")
		if pc.Ports == "" {
			json.Unmarshal(c.Ports, &pc.Ports) // a string in older versions
		}
		list = append(list, pc)
		return nil
	})
	return list, err
}

// ProjectLogs returns the last 200 lines of each of a project's containers.
func (m *Manager) ProjectLogs(ctx context.Context, name string) (string, error) {
	if err := m.existing(name); err != nil {
		return "", err
	}
	return m.run.Output(ctx, "docker", m.composeArgs(name, m.projectFile(name), "logs", "--no-color", "--timestamps", "--tail", "200")...)
}

// ---- saving ----

// ValidateProject checks a name and a compose.yaml before a job saves them.
// create means the project must not exist yet; otherwise it must.
func (m *Manager) ValidateProject(name, text string, create bool) error {
	if err := ValidProject(name); err != nil {
		return err
	}
	if len(text) > MaxComposeSize {
		return usererr.New("compose_too_large")
	}
	if strings.TrimSpace(text) == "" {
		return usererr.New("compose_empty")
	}
	switch exists := m.projectExists(name); {
	case create && exists:
		return usererr.New("project_exists", name)
	case !create && !exists:
		return ErrNoProject
	}
	return nil
}

// CheckText runs the safety checks on a draft compose.yaml for a project,
// without saving it. It needs the Compose plugin.
func (m *Manager) CheckText(ctx context.Context, name, text string) error {
	if err := ValidProject(name); err != nil {
		return err
	}
	if err := ensureDir(m.composeDir); err != nil {
		return err
	}
	f, err := os.CreateTemp(m.composeDir, ".check-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.WriteString(text)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return m.checkFile(ctx, name, f.Name())
}

// checkFile asks Compose for a file's final configuration and checks it.
func (m *Manager) checkFile(ctx context.Context, name, file string) error {
	out, err := m.stdout(ctx, m.composeArgs(name, file, "config", "--format", "json")...)
	if err != nil {
		msg := err.Error()
		var re *run.Error
		if errors.As(err, &re) {
			msg = run.Tail(re.Output, 6)
		}
		// Name the file as the user knows it, not the draft's temporary name.
		return usererr.New("compose_invalid", strings.ReplaceAll(msg, file, composeFile))
	}
	return CheckConfig([]byte(out))
}

// SaveProject writes a project's compose.yaml, creating its folder.
func (m *Manager) SaveProject(name, text string) error {
	if err := ValidProject(name); err != nil {
		return err
	}
	if len(text) > MaxComposeSize {
		return usererr.New("compose_too_large")
	}
	if err := ensureDir(m.composeDir); err != nil {
		return err
	}
	if err := ensureDir(m.projectDir(name)); err != nil {
		return err
	}
	return writeFile(m.projectFile(name), []byte(text))
}

// ensureDir creates a folder that only root can enter.
func ensureDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.Chmod(dir, 0o700)
}

// writeFile replaces a file atomically with one that only root can read.
func writeFile(name string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(name), ".save-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name()) // fails harmlessly after the rename
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), name)
}

// ---- actions ----

var projectActions = []string{"start", "stop", "restart", "down", "update"}

// CheckAction checks a project action before a job runs it.
func (m *Manager) CheckAction(name, action string) error {
	if err := m.existing(name); err != nil {
		return err
	}
	if !slices.Contains(projectActions, action) {
		return usererr.New("action_not_allowed", action)
	}
	return nil
}

// Up checks a saved project and starts it, creating or recreating the
// containers whose configuration changed and pulling missing images.
func (m *Manager) Up(ctx context.Context, name string, lg Logger) error {
	if err := m.existing(name); err != nil {
		return err
	}
	file := m.projectFile(name)
	lg.Logf("Checking %s", composeFile)
	if err := m.checkFile(ctx, name, file); err != nil {
		return err
	}
	lg.Logf("Starting the containers")
	return m.run.Stream(ctx, lg, "docker", m.composeArgs(name, file, "up", "--detach", "--remove-orphans")...)
}

// ProjectAction runs start, stop, restart, down (stop and remove the
// containers, keeping volumes) or update (pull newer images and recreate).
func (m *Manager) ProjectAction(ctx context.Context, name, action string, lg Logger) error {
	if err := m.CheckAction(name, action); err != nil {
		return err
	}
	file := m.projectFile(name)
	compose := func(args ...string) error {
		return m.run.Stream(ctx, lg, "docker", m.composeArgs(name, file, args...)...)
	}
	switch action {
	case "start":
		return m.Up(ctx, name, lg)
	case "update":
		// Check before pulling: images of a refused project are not needed.
		lg.Logf("Checking %s", composeFile)
		if err := m.checkFile(ctx, name, file); err != nil {
			return err
		}
		lg.Logf("Pulling images")
		if err := compose("pull"); err != nil {
			return err
		}
		return m.Up(ctx, name, lg)
	case "stop":
		return compose("stop")
	case "restart":
		return compose("restart")
	}
	return compose("down", "--remove-orphans")
}

// DeleteProject removes a project's containers and networks, and its
// named volumes too when volumes is set, then its folder.
func (m *Manager) DeleteProject(ctx context.Context, name string, volumes bool, lg Logger) error {
	if err := m.existing(name); err != nil {
		return err
	}
	args := []string{"down", "--remove-orphans"}
	if volumes {
		args = append(args, "--volumes")
	}
	lg.Logf("Removing the containers")
	if err := m.run.Stream(ctx, lg, "docker", m.composeArgs(name, m.projectFile(name), args...)...); err != nil {
		return err
	}
	lg.Logf("Removing %s", m.projectDir(name))
	return os.RemoveAll(m.projectDir(name))
}

// ---- safety checks ----

// flexString accepts a JSON string or number: Compose versions differ in
// how they print ports.
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*f = flexString(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*f = flexString(n.String())
	return nil
}

// composeConfig is the part of `docker compose config --format json` that
// the checks read.
type composeConfig struct {
	Services map[string]struct {
		Privileged        bool              `json:"privileged"`
		NetworkMode       string            `json:"network_mode"`
		PID               string            `json:"pid"`
		IPC               string            `json:"ipc"`
		UsernsMode        string            `json:"userns_mode"`
		Cgroup            string            `json:"cgroup"`
		CapAdd            []string          `json:"cap_add"`
		Devices           json.RawMessage   `json:"devices"`
		DeviceCgroupRules []string          `json:"device_cgroup_rules"`
		SecurityOpt       []string          `json:"security_opt"`
		Sysctls           map[string]string `json:"sysctls"`
		Ports             []struct {
			HostIP    string     `json:"host_ip"`
			Target    flexString `json:"target"`
			Published flexString `json:"published"`
			Protocol  string     `json:"protocol"`
		} `json:"ports"`
		Volumes []struct {
			Type     string `json:"type"`
			Source   string `json:"source"`
			ReadOnly bool   `json:"read_only"`
		} `json:"volumes"`
	} `json:"services"`
	Volumes map[string]struct {
		DriverOpts map[string]string `json:"driver_opts"`
	} `json:"volumes"`
	Secrets map[string]struct {
		File string `json:"file"`
	} `json:"secrets"`
	Configs map[string]struct {
		File string `json:"file"`
	} `json:"configs"`
}

// protectedPaths may not be mounted into a container, nor anything inside
// them.
var protectedPaths = []string{"/etc", "/root", "/boot", "/proc", "/sys", "/dev", "/var/run/docker.sock", "/run/docker.sock"}

// readOnlyAllowed are files in /etc that containers commonly mount, read
// only, to use the server's time zone.
var readOnlyAllowed = []string{"/etc/localtime", "/etc/timezone"}

// resolveLink finds where a host path really points; tests replace it.
var resolveLink = filepath.EvalSymlinks

// within reports whether p is dir or inside it.
func within(p, dir string) bool {
	return p == dir || dir == "/" || strings.HasPrefix(p, dir+"/")
}

// forbiddenMount reports whether mounting the host path src gives a
// container a system folder or the Docker socket. A folder that contains
// the socket, such as /run or the root folder, counts as the socket. The
// path is checked as written and after following symbolic links.
func forbiddenMount(src string, readOnly bool) bool {
	if !strings.HasPrefix(src, "/") {
		return false // a named volume, or a path Docker would refuse anyway
	}
	paths := []string{path.Clean(src)}
	if real, err := resolveLink(src); err == nil && strings.HasPrefix(real, "/") {
		paths = append(paths, path.Clean(real))
	}
	for _, p := range paths {
		if readOnly && slices.Contains(readOnlyAllowed, p) {
			continue
		}
		for _, dir := range protectedPaths {
			if within(p, dir) {
				return true
			}
			if strings.HasSuffix(dir, ".sock") && within(dir, p) {
				return true
			}
		}
	}
	return false
}

func describePort(hostIP, published, target, protocol string) string {
	s := target + "/" + protocol
	if published != "" {
		s = published + "->" + s
	}
	if hostIP != "" {
		s = hostIP + ":" + s
	}
	return s
}

// noDevices reports whether a service's devices list is absent or empty.
func noDevices(raw json.RawMessage) bool {
	t := strings.TrimSpace(string(raw))
	return t == "" || t == "null" || t == "[]"
}

// CheckConfig refuses a project configuration (the output of "docker
// compose config --format json") that would weaken the server. Problems
// are reported one at a time, in a stable order.
func CheckConfig(data []byte) error {
	var c composeConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return fmt.Errorf("read docker compose config: %w", err)
	}
	names := make([]string, 0, len(c.Services))
	for name := range c.Services {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		svc := c.Services[name]
		if svc.Privileged {
			return usererr.New("compose_privileged", name)
		}
		for _, ns := range []struct{ key, value string }{{"network_mode", svc.NetworkMode}, {"pid", svc.PID}, {"ipc", svc.IPC},
			{"userns_mode", svc.UsernsMode}, {"cgroup", svc.Cgroup}} {
			if ns.value == "host" {
				return usererr.New("compose_host_namespace", name+": "+ns.key+": host")
			}
		}
		if len(svc.CapAdd) > 0 {
			return usererr.New("compose_capability", name+": "+svc.CapAdd[0])
		}
		if !noDevices(svc.Devices) || len(svc.DeviceCgroupRules) > 0 {
			return usererr.New("compose_device", name)
		}
		// Only no-new-privileges, which takes privileges away, may be set:
		// seccomp=unconfined, apparmor=unconfined or label changes weaken
		// the container's confinement.
		for _, o := range svc.SecurityOpt {
			switch strings.ReplaceAll(strings.TrimSpace(o), "=", ":") {
			case "no-new-privileges", "no-new-privileges:true":
			default:
				return usererr.New("compose_security_opt", name+": "+o)
			}
		}
		if keys := sortedKeys(svc.Sysctls); len(keys) > 0 {
			return usererr.New("compose_sysctl", name+": "+keys[0])
		}
		for _, p := range svc.Ports {
			// Without host_ip Docker listens on every address, and without
			// a published port it picks a random one, also on every address.
			if p.HostIP != "127.0.0.1" && p.HostIP != "::1" {
				return usererr.New("compose_public_port", name+": "+describePort(p.HostIP, string(p.Published), string(p.Target), p.Protocol))
			}
		}
		for _, v := range svc.Volumes {
			if v.Type == "bind" && forbiddenMount(v.Source, v.ReadOnly) {
				return usererr.New("compose_forbidden_mount", name+": "+v.Source)
			}
		}
	}
	// A named volume can mount a host folder too, and file-based secrets
	// and configs are mounted from the host (read only).
	for _, name := range sortedKeys(c.Volumes) {
		if dev := c.Volumes[name].DriverOpts["device"]; forbiddenMount(dev, false) {
			return usererr.New("compose_forbidden_mount", "volume "+name+": "+dev)
		}
	}
	for _, name := range sortedKeys(c.Secrets) {
		if f := c.Secrets[name].File; forbiddenMount(f, true) {
			return usererr.New("compose_forbidden_mount", "secret "+name+": "+f)
		}
	}
	for _, name := range sortedKeys(c.Configs) {
		if f := c.Configs[name].File; forbiddenMount(f, true) {
			return usererr.New("compose_forbidden_mount", "config "+name+": "+f)
		}
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
