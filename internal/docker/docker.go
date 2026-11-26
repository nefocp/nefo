// Package docker shows and controls Docker containers, images and Compose
// projects, and runs new containers from an image, with ports published on
// 127.0.0.1 only: Docker's own port publishing bypasses ufw, so the panel
// never opens a container to the internet directly. Use an app website to
// make it public.
package docker

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/usererr"
)

// Container is one Docker container.
type Container struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Image  string `json:"image"`
	State  string `json:"state"`
	Status string `json:"status"`
	Ports  string `json:"ports"`
}

// Manager runs docker commands.
type Manager struct {
	run        run.Runner
	composeDir string // one folder per Compose project
}

// New returns a Manager.
func New(r run.Runner) *Manager { return &Manager{run: r, composeDir: ComposeDir} }

// Installed reports whether Docker is installed.
func (m *Manager) Installed() bool { _, err := os.Stat("/usr/bin/docker"); return err == nil }

var (
	idRe    = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)
	imageRe = regexp.MustCompile(`^[a-z0-9]+([._/-][a-z0-9]+)*(:[A-Za-z0-9._-]{1,128})?$`)
	envRe   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}=[^\x00\n\r]{0,1000}$`)
)

func eachLine(out string, fn func([]byte) error) error {
	for _, l := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if err := fn([]byte(l)); err != nil {
			return err
		}
	}
	return nil
}

// Containers lists all containers.
func (m *Manager) Containers(ctx context.Context) ([]Container, error) {
	out, err := m.run.Output(ctx, "docker", "ps", "-a", "--no-trunc", "--format", "{{json .}}")
	if err != nil {
		return nil, err
	}
	list := []Container{}
	err = eachLine(out, func(b []byte) error {
		var c struct{ ID, Names, Image, State, Status, Ports string }
		if err := json.Unmarshal(b, &c); err != nil {
			return err
		}
		list = append(list, Container{ID: c.ID[:min(12, len(c.ID))], Name: c.Names, Image: c.Image, State: c.State, Status: c.Status, Ports: c.Ports})
		return nil
	})
	return list, err
}

// ValidContainer checks a container ID or name.
func ValidContainer(id string) error {
	if !idRe.MatchString(id) {
		return usererr.New("invalid_container", id)
	}
	return nil
}

// Action starts, stops, restarts or removes a container.
func (m *Manager) Action(ctx context.Context, id, action string) error {
	if err := ValidContainer(id); err != nil {
		return err
	}
	args := map[string][]string{
		"start": {"start", id}, "stop": {"stop", id}, "restart": {"restart", id}, "remove": {"rm", "-f", id},
	}[action]
	if args == nil {
		return usererr.New("action_not_allowed", action)
	}
	_, err := m.run.Output(ctx, "docker", args...)
	return err
}

// Logs returns the last lines of a container's output.
func (m *Manager) Logs(ctx context.Context, id string) (string, error) {
	if !idRe.MatchString(id) {
		return "", usererr.New("invalid_container", id)
	}
	return m.run.Output(ctx, "docker", "logs", "--tail", "200", "--timestamps", id)
}

// RunRequest describes a new container.
type RunRequest struct {
	Image         string   `json:"image"`
	Name          string   `json:"name"`
	HostPort      int      `json:"host_port"`      // published on 127.0.0.1; 0 for none
	ContainerPort int      `json:"container_port"` // port inside the container
	Env           []string `json:"env"`            // NAME=value
}

// Check validates a request, so mistakes are reported before a job starts.
func (r *RunRequest) Check() error {
	if !imageRe.MatchString(r.Image) || len(r.Image) > 255 {
		return usererr.New("invalid_image", r.Image)
	}
	if !idRe.MatchString(r.Name) {
		return usererr.New("invalid_container", r.Name)
	}
	if r.HostPort != 0 || r.ContainerPort != 0 {
		if r.HostPort < 1024 || r.HostPort > 65535 || r.ContainerPort < 1 || r.ContainerPort > 65535 {
			return usererr.New("invalid_port", strconv.Itoa(r.HostPort))
		}
	}
	for _, e := range r.Env {
		if !envRe.MatchString(e) {
			return usererr.New("invalid_env", e)
		}
	}
	return nil
}

// Run pulls an image if needed and starts a container that restarts with
// the server. Output goes to w.
func (m *Manager) Run(ctx context.Context, r RunRequest, w io.Writer) error {
	if err := r.Check(); err != nil {
		return err
	}
	args := []string{"run", "-d", "--name", r.Name, "--restart", "unless-stopped"}
	if r.HostPort != 0 {
		args = append(args, "-p", "127.0.0.1:"+strconv.Itoa(r.HostPort)+":"+strconv.Itoa(r.ContainerPort))
	}
	for _, e := range r.Env {
		args = append(args, "-e", e)
	}
	args = append(args, r.Image)
	return m.run.Stream(ctx, w, "docker", args...)
}
