package run

import (
	"context"
	"io"
)

// Apt runs apt-get non-interactively with a normal umask. The panel's
// service runs with umask 077, which would make files created by package
// scripts unreadable. The shell script is a constant and the arguments are
// passed separately, so no input ever reaches the shell.
func Apt(ctx context.Context, r Runner, w io.Writer, args ...string) error {
	base := []string{"-c", `umask 022 && exec apt-get "$@"`, "apt-get",
		"-q", "-y", "-o", "DPkg::Lock::Timeout=300",
		"-o", "Dpkg::Options::=--force-confdef", "-o", "Dpkg::Options::=--force-confold"}
	return r.Stream(ctx, w, "/bin/sh", append(base, args...)...)
}
