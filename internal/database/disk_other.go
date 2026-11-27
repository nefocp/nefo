//go:build !linux && !darwin

package database

// freeDiskMB is unknown here (-1): the panel manages databases on Linux.
func freeDiskMB(path string) int64 { return -1 }
