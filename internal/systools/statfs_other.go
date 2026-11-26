//go:build !linux

package systools

import "errors"

// diskFree is only available on Linux.
func diskFree(string) (uint64, string, error) {
	return 0, "", errors.New("disk space is only known on Linux")
}
