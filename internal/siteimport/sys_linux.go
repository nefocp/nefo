//go:build linux

package siteimport

import (
	"io/fs"
	"os"
	"strconv"
	"syscall"
)

// openFlags keep Open from following a link swapped in at the last moment
// and from waiting on a FIFO.
const openFlags = syscall.O_NOFOLLOW | syscall.O_NONBLOCK

type fileID struct{ dev, ino uint64 }

func idOf(fi fs.FileInfo) fileID {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return fileID{dev: uint64(st.Dev), ino: uint64(st.Ino)}
	}
	return fileID{}
}

// openedPath returns where an open file really is.
func openedPath(f *os.File) (string, bool) {
	p, err := os.Readlink("/proc/self/fd/" + strconv.Itoa(int(f.Fd())))
	return p, err == nil
}

// DiskFree returns the bytes of the file system of dir that are free for
// ordinary users (root could also use the reserved ones, but a full disk
// stops the whole server), and its device number.
func DiskFree(dir string) (int64, uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return 0, 0, err
	}
	var s syscall.Stat_t
	if err := syscall.Stat(dir, &s); err != nil {
		return 0, 0, err
	}
	return int64(st.Bavail) * int64(st.Bsize), uint64(s.Dev), nil
}
