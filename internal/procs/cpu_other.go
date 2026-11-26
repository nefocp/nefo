//go:build !linux

package procs

// readTicks needs /proc; elsewhere the CPU use comes from ps alone.
func readTicks() map[int]cpuSample { return nil }
