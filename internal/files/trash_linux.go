//go:build linux

package files

import (
	"errors"
	"os"
	"syscall"
)

// renameEntry renames name in folder from to newName in folder to. The
// names are looked up from the open folders (renameat), so a symbolic link
// put in a parent folder meanwhile cannot redirect the move.
func renameEntry(from *os.File, name string, to *os.File, newName string) error {
	fc, err := from.SyscallConn()
	if err != nil {
		return err
	}
	tc, err := to.SyscallConn()
	if err != nil {
		return err
	}
	var rerr error
	err = fc.Control(func(ffd uintptr) {
		err := tc.Control(func(tfd uintptr) {
			rerr = syscall.Renameat(int(ffd), name, int(tfd), newName)
		})
		if rerr == nil {
			rerr = err
		}
	})
	if err != nil {
		return err
	}
	if rerr != nil {
		return &os.LinkError{Op: "renameat", Old: name, New: newName, Err: rerr}
	}
	return nil
}

// openNonblock is added when opening files to read, so a FIFO swapped in
// for a regular file cannot block the panel.
const openNonblock = syscall.O_NONBLOCK

// openDirectory makes opening anything but a folder fail at once.
const openDirectory = syscall.O_DIRECTORY

// isCrossDevice reports whether a rename failed because the two folders
// are on different file systems.
func isCrossDevice(err error) bool { return errors.Is(err, syscall.EXDEV) }
