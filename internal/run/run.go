// Package run executes system programs.
//
// Every program gets each argument as a separate value and never goes through
// a shell with user input (see CONTRIBUTING.md). Output is in English and in
// the C locale, so the panel can parse it on any server.
package run

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Runner runs programs. Tests replace it with a fake.
type Runner interface {
	// Output runs a program and returns what it printed on stdout and stderr.
	Output(ctx context.Context, name string, args ...string) (string, error)
	// Stream runs a program and copies its output to w while it runs.
	Stream(ctx context.Context, w io.Writer, name string, args ...string) error
	// Input runs a program with stdin as its standard input and returns its
	// output. Secrets such as passwords go through stdin, never through
	// arguments, which every user on the server can read in the process list.
	Input(ctx context.Context, stdin []byte, name string, args ...string) (string, error)
	// Pipe runs a program reading stdin (may be nil) and writing its standard
	// output to stdout, for large data such as database dumps. Standard
	// error is kept for the error message.
	Pipe(ctx context.Context, stdin io.Reader, stdout io.Writer, name string, args ...string) error
}

// System runs programs on this machine.
type System struct{}

// Error describes a program that failed, with the end of its output.
type Error struct {
	Cmd    string
	Err    error
	Output string
}

func (e *Error) Error() string {
	out := Tail(e.Output, 15)
	if out == "" {
		return fmt.Sprintf("%s: %v", e.Cmd, e.Err)
	}
	return fmt.Sprintf("%s: %v\n%s", e.Cmd, e.Err, out)
}

func (e *Error) Unwrap() error { return e.Err }

func command(ctx context.Context, name string, args []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(),
		"LC_ALL=C", "LANG=C",
		"DEBIAN_FRONTEND=noninteractive",
		// needrestart would otherwise restart services in the middle of an apt run.
		"NEEDRESTART_SUSPEND=1",
	)
	cmd.Stdin = nil // /dev/null: nothing may wait for input
	cmd.WaitDelay = 10 * time.Second
	return cmd
}

// Output implements Runner.
func (System) Output(ctx context.Context, name string, args ...string) (string, error) {
	var buf bytes.Buffer
	cmd := command(ctx, name, args)
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	if err != nil {
		return buf.String(), &Error{Cmd: name, Err: err, Output: buf.String()}
	}
	return buf.String(), nil
}

// Input implements Runner.
func (System) Input(ctx context.Context, stdin []byte, name string, args ...string) (string, error) {
	var buf bytes.Buffer
	cmd := command(ctx, name, args)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return buf.String(), &Error{Cmd: name, Err: err, Output: buf.String()}
	}
	return buf.String(), nil
}

// Pipe implements Runner.
func (System) Pipe(ctx context.Context, stdin io.Reader, stdout io.Writer, name string, args ...string) error {
	var stderr tailBuffer
	cmd := command(ctx, name, args)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return &Error{Cmd: name, Err: err, Output: stderr.String()}
	}
	return nil
}

// Stream implements Runner.
func (System) Stream(ctx context.Context, w io.Writer, name string, args ...string) error {
	var tail tailBuffer
	out := io.MultiWriter(w, &tail)
	cmd := command(ctx, name, args)
	// The same writer for both: exec then calls Write from one goroutine at a time.
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return &Error{Cmd: name, Err: err, Output: tail.String()}
	}
	return nil
}

// Tail returns the last n non-empty lines of s.
func Tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	var keep []string
	for i := len(lines) - 1; i >= 0 && len(keep) < n; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			keep = append(keep, lines[i])
		}
	}
	for i, j := 0, len(keep)-1; i < j; i, j = i+1, j-1 {
		keep[i], keep[j] = keep[j], keep[i]
	}
	return strings.Join(keep, "\n")
}

// tailBuffer keeps only the last 8 KB written to it.
type tailBuffer struct{ b []byte }

func (t *tailBuffer) Write(p []byte) (int, error) {
	const max = 8 << 10
	t.b = append(t.b, p...)
	if len(t.b) > max {
		t.b = t.b[len(t.b)-max:]
	}
	return len(p), nil
}

func (t *tailBuffer) String() string { return string(t.b) }
