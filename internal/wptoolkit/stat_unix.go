//go:build unix

package wptoolkit

import "syscall"

// openNonBlock keeps an open from waiting on a FIFO a site's user created.
const openNonBlock = syscall.O_NONBLOCK
