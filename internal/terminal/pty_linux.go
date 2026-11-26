//go:build linux

package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"unsafe"
)

func ioctl(f *os.File, req uintptr, arg unsafe.Pointer) error {
	// SyscallConn keeps the file in non-blocking mode (unlike Fd), so
	// closing it interrupts a pending Read.
	rc, err := f.SyscallConn()
	if err != nil {
		return err
	}
	var errno syscall.Errno
	if err := rc.Control(func(fd uintptr) {
		_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg))
	}); err != nil {
		return err
	}
	if errno != 0 {
		return errno
	}
	return nil
}

// Start runs a program in a new pseudo-terminal: argv when it is given
// (such as a docker exec command), otherwise a login shell for root.
func Start(rows, cols int, argv ...string) (*Session, error) {
	if len(argv) == 0 {
		argv = []string{"/bin/bash", "-l"}
	}
	// Resolve the program before opening the terminal, so a missing one
	// is reported as an error rather than as a shell that exits at once.
	prog, err := exec.LookPath(argv[0])
	if err != nil {
		return nil, err
	}
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	var unlock int32
	if err := ioctl(master, syscall.TIOCSPTLCK, unsafe.Pointer(&unlock)); err != nil {
		master.Close()
		return nil, fmt.Errorf("unlock pty: %w", err)
	}
	var n uint32
	if err := ioctl(master, syscall.TIOCGPTN, unsafe.Pointer(&n)); err != nil {
		master.Close()
		return nil, fmt.Errorf("pty number: %w", err)
	}
	slave, err := os.OpenFile("/dev/pts/"+strconv.Itoa(int(n)), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		master.Close()
		return nil, err
	}
	defer slave.Close()

	cmd := exec.Command(prog, argv[1:]...)
	cmd.Dir = "/root"
	cmd.Env = []string{
		"TERM=xterm-256color", "COLORTERM=truecolor", "HOME=/root", "USER=root", "LOGNAME=root",
		"SHELL=/bin/bash", "LANG=C.UTF-8", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		master.Close()
		return nil, err
	}
	s := &Session{pty: master, cmd: cmd, done: make(chan struct{})}
	s.Resize(rows, cols)
	go func() {
		cmd.Wait()
		close(s.done)
	}()
	return s, nil
}

// Resize tells the shell the terminal's size.
func (s *Session) Resize(rows, cols int) error {
	if rows < 1 || cols < 1 || rows > 500 || cols > 1000 {
		return nil
	}
	ws := struct{ Row, Col, X, Y uint16 }{uint16(rows), uint16(cols), 0, 0}
	return ioctl(s.pty, syscall.TIOCSWINSZ, unsafe.Pointer(&ws))
}

// Close ends the shell and everything it started.
func (s *Session) Close() {
	s.once.Do(func() {
		if s.cmd.Process != nil {
			syscall.Kill(-s.cmd.Process.Pid, syscall.SIGHUP)
			syscall.Kill(-s.cmd.Process.Pid, syscall.SIGKILL)
		}
		s.pty.Close()
	})
}
