// Package fsutil contains small file helpers.
package fsutil

import (
	"errors"
	"os"
	"path/filepath"
)

// WriteFileAtomic writes data to a temporary file next to path and renames it
// into place, so a crash never leaves a half-written file behind.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp) // no-op once the rename has succeeded

	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// MkdirAll is os.MkdirAll with perm given to every folder it creates,
// whatever the umask: the panel runs with umask 077, so os.MkdirAll alone
// makes folders only root can enter, and a service that reads them as its
// own user (MySQL, PostgreSQL, PHP-FPM, Nginx) silently sees nothing.
// Folders that already exist keep their mode.
func MkdirAll(path string, perm os.FileMode) error {
	path = filepath.Clean(path)
	if fi, err := os.Stat(path); err == nil {
		if !fi.IsDir() {
			return &os.PathError{Op: "mkdir", Path: path, Err: errNotDir}
		}
		return nil
	}
	if parent := filepath.Dir(path); parent != path {
		if err := MkdirAll(parent, perm); err != nil {
			return err
		}
	}
	if err := os.Mkdir(path, perm); err != nil {
		if fi, serr := os.Stat(path); serr == nil && fi.IsDir() {
			return nil // made by someone else meanwhile
		}
		return err
	}
	return os.Chmod(path, perm)
}

var errNotDir = errors.New("not a directory")

// Exists reports whether path exists.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
