//go:build !linux

package siteimport

import (
	"errors"
	"io/fs"
	"os"
)

const openFlags = 0

type fileID struct{ dev, ino uint64 }

func idOf(fs.FileInfo) fileID { return fileID{} }

func openedPath(*os.File) (string, bool) { return "", false }

// DiskFree is only known on Linux.
func DiskFree(string) (int64, uint64, error) {
	return 0, 0, errors.New("free space is only known on Linux")
}
