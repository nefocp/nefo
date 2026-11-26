package wptoolkit

import (
	"bytes"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/store"
)

// call is one command the fake runner was asked to run.
type call struct {
	method string // Output, Input, Stream or Pipe
	name   string
	args   []string
	stdin  string
}

// user is the Linux user a runuser command runs as.
func (c call) user() string {
	if len(c.args) > 1 && c.args[0] == "-u" {
		return c.args[1]
	}
	return ""
}

// wp returns the wp-cli arguments after the global options, or nil when
// the command is not wp-cli.
func (c call) wp() []string {
	i := slices.Index(c.args, "--skip-packages")
	if i < 0 {
		return nil
	}
	return c.args[i+1:]
}

func (c call) wpLine() string { return strings.Join(c.wp(), " ") }

// fakeRunner records commands; answer gives wp-cli's standard output and
// error for each wp-cli command.
type fakeRunner struct {
	mu     sync.Mutex
	calls  []call
	answer func(c call) (string, error)
}

func (f *fakeRunner) do(c call) (string, error) {
	f.mu.Lock()
	f.calls = append(f.calls, c)
	f.mu.Unlock()
	if f.answer == nil {
		return "", nil
	}
	return f.answer(c)
}

func (f *fakeRunner) Output(_ context.Context, name string, args ...string) (string, error) {
	return f.do(call{method: "Output", name: name, args: args})
}

func (f *fakeRunner) Input(_ context.Context, stdin []byte, name string, args ...string) (string, error) {
	return f.do(call{method: "Input", name: name, args: args, stdin: string(stdin)})
}

func (f *fakeRunner) Stream(_ context.Context, w io.Writer, name string, args ...string) error {
	out, err := f.do(call{method: "Stream", name: name, args: args})
	io.WriteString(w, out)
	return err
}

func (f *fakeRunner) Pipe(_ context.Context, stdin io.Reader, stdout io.Writer, name string, args ...string) error {
	var in []byte
	if stdin != nil {
		in, _ = io.ReadAll(stdin)
	}
	out, err := f.do(call{method: "Pipe", name: name, args: args, stdin: string(in)})
	io.WriteString(stdout, out)
	return err
}

// wpLines returns the wp-cli commands that ran, in order.
func (f *fakeRunner) wpLines() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var lines []string
	for _, c := range f.calls {
		if c.wp() != nil {
			lines = append(lines, c.wpLine())
		}
	}
	return lines
}

// cliFail is what the runner returns when wp-cli exits with an error.
func cliFail(msg string) error {
	return &run.Error{Cmd: "runuser", Err: errors.New("exit status 1"), Output: "Warning: something\nError: " + msg + "\n"}
}

type testLog struct{ bytes.Buffer }

func (l *testLog) Logf(format string, args ...any) { fmt.Fprintf(&l.Buffer, format+"\n", args...) }

// newKit returns a Toolkit with sites under a temporary folder and an
// installed wp-cli, so nothing is downloaded.
func newKit(t *testing.T, r *fakeRunner) (*Toolkit, string) {
	t.Helper()
	base := t.TempDir()
	k := New(r, func(domain string) string { return filepath.Join(base, "www", domain) })
	k.phar = filepath.Join(base, "wp-cli.phar")
	data := []byte("<?php // wp-cli")
	if err := os.WriteFile(k.phar, data, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha512.Sum512(data)
	rec, _ := json.Marshal(cliRecord{Version: "2.12.1", SHA512: hex.EncodeToString(sum[:])})
	if err := os.WriteFile(k.recordFile(), rec, 0o644); err != nil {
		t.Fatal(err)
	}
	// Nothing listens there: no downloads in tests.
	k.latestURL, k.assetURL = "http://127.0.0.1:1/latest", "http://127.0.0.1:1/download/"
	k.lookupIDs = func(string) (int, int, error) { return -1, -1, nil }
	k.now = func() time.Time { return time.Date(2026, 11, 25, 10, 30, 0, 0, time.UTC) }
	return k, base
}

// newSite makes a site's folders, as hosting.CreateSite does.
func newSite(t *testing.T, k *Toolkit, domain, user string) *store.Site {
	t.Helper()
	site := &store.Site{ID: int64(len(domain)), Domain: domain, SysUser: user, PHPVersion: "8.3", App: "wordpress"}
	for _, d := range []string{"public", "tmp", "logs"} {
		if err := os.MkdirAll(filepath.Join(k.root(domain), d), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	return site
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func wantLines(t *testing.T, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("wp-cli commands:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}
