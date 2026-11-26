//go:build linux || darwin

package appstore

import "syscall"

// freeDiskMB returns the space available to unprivileged users at path, or
// -1 when it cannot be read.
func freeDiskMB(path string) int {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return -1
	}
	return int(uint64(st.Bavail) * uint64(st.Bsize) >> 20)
}
