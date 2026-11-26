//go:build unix

package wptoolkit

import (
	"io/fs"
	"syscall"
)

// openNonBlock keeps an open from waiting on a FIFO a site's user created.
const openNonBlock = syscall.O_NONBLOCK

// fileOwner returns a file's owner and number of hard links.
func fileOwner(fi fs.FileInfo) (uid int, links uint64) {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return int(st.Uid), uint64(st.Nlink)
	}
	return -1, 1
}
