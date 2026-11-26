//go:build !unix

package wptoolkit

import "io/fs"

const openNonBlock = 0

// fileOwner returns -1 (unknown owner) and one link where the platform has
// no such information.
func fileOwner(fs.FileInfo) (uid int, links uint64) { return -1, 1 }
