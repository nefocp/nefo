//go:build !linux

package terminal

// Start is only available on Linux.
func Start(rows, cols int, argv ...string) (*Session, error) { return nil, ErrUnsupported }

// Resize is only available on Linux.
func (s *Session) Resize(rows, cols int) error { return ErrUnsupported }

// Close is only available on Linux.
func (s *Session) Close() {}
