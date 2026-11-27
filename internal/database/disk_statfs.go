//go:build linux || darwin

package database

import "syscall"

// freeDiskMB returns the space available at path, or -1 when it cannot be
// read.
func freeDiskMB(path string) int64 {
	var st syscall.Statfs_t
	if err := syscall.Statfs(existingParent(path), &st); err != nil {
		return -1
	}
	return int64(uint64(st.Bavail) * uint64(st.Bsize) >> 20)
}
