// Package terminal runs a root shell, or another program such as a shell
// in a container, in a pseudo-terminal for the panel's web terminal. The
// administrator is root on the server, so the shell has the same power as
// an SSH session.
package terminal

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
)

// ErrUnsupported is returned on systems without Linux pseudo-terminals.
var ErrUnsupported = errors.New("the terminal needs Linux")

// Session is a running shell.
type Session struct {
	pty  *os.File
	cmd  *exec.Cmd
	once sync.Once
	done chan struct{}
}

// Read reads the shell's output.
func (s *Session) Read(p []byte) (int, error) { return s.pty.Read(p) }

// Write sends keystrokes to the shell.
func (s *Session) Write(p []byte) (int, error) { return s.pty.Write(p) }

// Done is closed when the shell has exited.
func (s *Session) Done() <-chan struct{} { return s.done }

var _ io.ReadWriter = (*Session)(nil)
