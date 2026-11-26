//go:build !linux

package files

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// renameEntry renames name in folder from to newName in folder to. Only
// Linux resolves the names from the open folders; elsewhere (development
// machines) the folders' paths are used.
func renameEntry(from *os.File, name string, to *os.File, newName string) error {
	return os.Rename(filepath.Join(from.Name(), name), filepath.Join(to.Name(), newName))
}

// openNonblock is only needed on Linux.
const openNonblock = 0

// openDirectory is only needed on Linux.
const openDirectory = 0

// isCrossDevice reports whether a rename failed because the two folders
// are on different file systems.
func isCrossDevice(err error) bool { return errors.Is(err, syscall.EXDEV) }
