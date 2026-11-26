//go:build !unix

package logstats

import "os"

// openLog opens a log file for reading if it is a regular file, not a
// symbolic link. (Servers run Linux, where O_NOFOLLOW is used instead.)
func openLog(path string) (*os.File, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, errNotRegular
	}
	return os.Open(path)
}

func isSymlinkErr(error) bool { return false }
