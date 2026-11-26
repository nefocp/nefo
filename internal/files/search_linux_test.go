package files

import (
	"context"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// A FIFO, listed or swapped in for a file, must never block a search.
func TestSearchFIFODoesNotBlock(t *testing.T) {
	root, dir := searchTree(t, map[string]string{"a.txt": "needle"})
	if err := syscall.Mkfifo(filepath.Join(dir, "pipe.txt"), 0o644); err != nil {
		t.Skip("mkfifo:", err)
	}
	done := make(chan error, 1)
	go func() {
		if _, err := openRegular(root, "pipe.txt"); code(err) != "not_a_file" {
			done <- err
			return
		}
		_, err := search(context.Background(), root, ".", "/site", "", "needle", defaultSearchLimits)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("reading a FIFO blocked")
	}
}
