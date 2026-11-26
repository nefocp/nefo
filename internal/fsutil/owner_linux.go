//go:build linux

package fsutil

import (
	"io/fs"
	"syscall"
)

// Owner returns the numeric owner and group of a file, or -1, -1 where the
// platform does not have them.
func Owner(fi fs.FileInfo) (int, int) {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return int(st.Uid), int(st.Gid)
	}
	return -1, -1
}
