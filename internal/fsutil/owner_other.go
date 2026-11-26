//go:build !linux

package fsutil

import "io/fs"

// Owner returns the numeric owner and group of a file, or -1, -1 where the
// platform does not have them.
func Owner(fs.FileInfo) (int, int) { return -1, -1 }
