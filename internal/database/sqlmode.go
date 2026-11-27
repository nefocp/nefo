package database

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"regexp"
)

// MariaDB's default sql_mode has NO_AUTO_CREATE_USER, which MySQL 8 removed
// and refuses ("Variable 'sql_mode' can't be set to the value of
// 'NO_AUTO_CREATE_USER'"). A dump writes the sql_mode each trigger,
// routine and event was made with on a line of its own before it:
//
//	/*!50003 SET sql_mode              = 'STRICT_TRANS_TABLES,…,NO_AUTO_CREATE_USER,…' */ ;
//
// noAutoCreateUser drops the mode from those lines while the dump streams
// into MySQL. Other lines, however long, pass through unchanged.
type noAutoCreateUser struct {
	src  *bufio.Reader
	out  bytes.Buffer
	mid  bool // in the middle of a line too long to look at
	err  error
	seen int // lines changed
}

var (
	sqlModeLineRe = regexp.MustCompile(`^/\*!50003 SET sql_mode\s*=\s*'`)
	noAutoRe      = regexp.MustCompile(`,NO_AUTO_CREATE_USER\b|\bNO_AUTO_CREATE_USER,?`)
)

func newNoAutoCreateUser(r io.Reader) *noAutoCreateUser {
	return &noAutoCreateUser{src: bufio.NewReaderSize(r, 64<<10)}
}

func (f *noAutoCreateUser) Read(p []byte) (int, error) {
	for f.out.Len() == 0 {
		if f.err != nil {
			return 0, f.err
		}
		line, err := f.src.ReadSlice('\n')
		switch {
		case errors.Is(err, bufio.ErrBufferFull):
			f.out.Write(line)
			f.mid = true
			continue
		case err != nil:
			f.err = err
		}
		if !f.mid && sqlModeLineRe.Match(line) {
			if fixed := noAutoRe.ReplaceAll(line, nil); len(fixed) != len(line) {
				f.seen++
				line = fixed
			}
		}
		f.out.Write(line)
		f.mid = false
	}
	return f.out.Read(p)
}
