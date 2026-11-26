package docker

import (
	"context"
	"errors"
	"strings"

	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/usererr"
)

// Shell returns the command that opens an interactive shell in a running
// container, for the web terminal, and the container's name for the audit
// log. The shell is bash when the image has it, otherwise sh.
func (m *Manager) Shell(ctx context.Context, id string) (argv []string, name string, err error) {
	if err := ValidContainer(id); err != nil {
		return nil, "", err
	}
	out, err := m.run.Output(ctx, "docker", "container", "inspect", "--format", "{{.State.Running}} {{.Name}}", id)
	if err != nil {
		var re *run.Error
		if errors.As(err, &re) && strings.Contains(re.Output, "No such container") {
			return nil, "", usererr.New("container_not_found", id)
		}
		return nil, "", err
	}
	running, name, _ := strings.Cut(strings.TrimSpace(out), " ")
	name = strings.TrimPrefix(name, "/")
	if running != "true" {
		return nil, "", usererr.New("container_not_running", name)
	}
	shell := "/bin/sh"
	if _, err := m.run.Output(ctx, "docker", "exec", id, "test", "-x", "/bin/bash"); err == nil {
		shell = "/bin/bash"
	}
	return []string{"docker", "exec", "-it", "-e", "TERM=xterm-256color", id, shell}, name, nil
}
