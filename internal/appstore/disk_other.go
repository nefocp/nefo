//go:build !linux && !darwin

package appstore

// freeDiskMB is unknown outside Unix (development builds).
func freeDiskMB(string) int { return -1 }
