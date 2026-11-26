//go:build linux

package terminal

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

func TestStartRunsCommand(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("the terminal starts in /root, which needs root")
	}
	s, err := Start(24, 80, "/bin/sh", "-c", `stty size; echo "tty:$(tty)"`)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var out bytes.Buffer
	buf := make([]byte, 4096)
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(out.String(), "tty:") && time.Now().Before(deadline) {
		n, err := s.Read(buf)
		out.Write(buf[:n])
		if err != nil {
			break
		}
	}
	if got := out.String(); !strings.Contains(got, "24 80") || !strings.Contains(got, "tty:/dev/pts/") {
		t.Errorf("output %q", got)
	}
	select {
	case <-s.Done():
	case <-time.After(5 * time.Second):
		t.Error("the command did not exit")
	}
}

func TestStartMissingProgram(t *testing.T) {
	if _, err := Start(24, 80, "/nonexistent/program"); err == nil {
		t.Error("started a missing program")
	}
}
