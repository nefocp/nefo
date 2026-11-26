//go:build unix

package logstats

import (
	"errors"
	"os"
	"syscall"
)

// openLog opens a log file for reading without following a symbolic link,
// which could point outside the log folder. O_NONBLOCK keeps a planted FIFO
// from blocking; the caller then sees that it is not a regular file.
func openLog(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}

// isSymlinkErr reports whether openLog failed because path is a symlink.
func isSymlinkErr(err error) bool { return errors.Is(err, syscall.ELOOP) }
