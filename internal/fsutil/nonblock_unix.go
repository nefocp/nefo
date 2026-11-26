//go:build unix

package fsutil

import "syscall"

// openNonBlock keeps an open from waiting on a FIFO a site's user created.
const openNonBlock = syscall.O_NONBLOCK
