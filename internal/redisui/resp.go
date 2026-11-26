package redisui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

// A minimal RESP2 client: commands go out as arrays of bulk strings, and
// replies are read with limits, so a huge value cannot exhaust the panel's
// memory.

// allowed are the only commands the panel sends. Anything else is refused
// before it reaches the network.
var allowed = map[string]bool{
	"AUTH": true, "SELECT": true, "INFO": true, "SCAN": true,
	"TYPE": true, "PTTL": true, "STRLEN": true, "GETRANGE": true,
	"HLEN": true, "HSCAN": true, "LLEN": true, "LRANGE": true,
	"SCARD": true, "SSCAN": true, "ZCARD": true, "ZRANGE": true,
	"DEL": true, "FLUSHDB": true,
}

const (
	maxBulkKeep = 64 << 10 // bytes kept of one bulk string; the rest is read and dropped
	maxArray    = 1 << 16  // elements in one array reply
	maxLine     = 64 << 10 // simple strings, errors and length lines
)

// value is a RESP reply.
type value struct {
	kind      byte // '+', '-', ':', '$', '*'
	str       string
	num       int64
	null      bool
	truncated bool // a bulk string longer than maxBulkKeep
	size      int64
	array     []value
}

// Error is an error reply from Redis, such as "WRONGTYPE ...".
type Error struct{ Msg string }

func (e *Error) Error() string { return "Redis: " + e.Msg }

type conn struct {
	c       net.Conn
	r       *bufio.Reader
	w       *bufio.Writer
	timeout time.Duration
}

func dial(ctx context.Context, addr string, timeout time.Duration) (*conn, error) {
	d := net.Dialer{Timeout: timeout}
	c, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	return &conn{c: c, r: bufio.NewReaderSize(c, 32<<10), w: bufio.NewWriter(c), timeout: timeout}, nil
}

func (c *conn) Close() error { return c.c.Close() }

// deadline gives every exchange its own timeout, shortened by ctx.
func (c *conn) deadline(ctx context.Context) {
	dl := time.Now().Add(c.timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(dl) {
		dl = d
	}
	c.c.SetDeadline(dl)
}

func encode(w *bufio.Writer, args []string) error {
	if len(args) == 0 || !allowed[strings.ToUpper(args[0])] {
		name := ""
		if len(args) > 0 {
			name = args[0]
		}
		return fmt.Errorf("the panel does not send the Redis command %q", name)
	}
	fmt.Fprintf(w, "*%d\r\n", len(args))
	for _, a := range args {
		fmt.Fprintf(w, "$%d\r\n", len(a))
		w.WriteString(a)
		w.WriteString("\r\n")
	}
	return nil
}

// do sends one command and reads its reply. An error reply is returned as
// *Error.
func (c *conn) do(ctx context.Context, args ...string) (value, error) {
	vs, err := c.pipeline(ctx, [][]string{args})
	if err != nil {
		return value{}, err
	}
	if vs[0].kind == '-' {
		return vs[0], &Error{vs[0].str}
	}
	return vs[0], nil
}

// pipeline sends several commands at once and reads their replies in order.
// Error replies stay in the result as values of kind '-'.
func (c *conn) pipeline(ctx context.Context, cmds [][]string) ([]value, error) {
	for _, args := range cmds {
		if err := encode(c.w, args); err != nil {
			c.w.Reset(c.c)
			return nil, err
		}
	}
	c.deadline(ctx)
	if err := c.w.Flush(); err != nil {
		return nil, err
	}
	out := make([]value, 0, len(cmds))
	for range cmds {
		v, err := readValue(c.r, 0)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func readLine(r *bufio.Reader) (string, error) {
	var b []byte
	for {
		chunk, isPrefix, err := r.ReadLine()
		if err != nil {
			return "", err
		}
		b = append(b, chunk...)
		if len(b) > maxLine {
			return "", errors.New("Redis sent a line that is too long")
		}
		if !isPrefix {
			return string(b), nil
		}
	}
}

func readValue(r *bufio.Reader, depth int) (value, error) {
	if depth > 4 {
		return value{}, errors.New("Redis sent replies nested too deeply")
	}
	line, err := readLine(r)
	if err != nil {
		return value{}, err
	}
	if line == "" {
		return value{}, errors.New("Redis sent an empty line")
	}
	v := value{kind: line[0]}
	rest := line[1:]
	switch v.kind {
	case '+', '-':
		v.str = rest
	case ':':
		if v.num, err = strconv.ParseInt(rest, 10, 64); err != nil {
			return value{}, fmt.Errorf("Redis sent a bad integer %q", rest)
		}
	case '$':
		n, err := strconv.ParseInt(rest, 10, 64)
		if err != nil || n < -1 || n > 512<<20 {
			return value{}, fmt.Errorf("Redis sent a bad length %q", rest)
		}
		if n == -1 {
			v.null = true
			return v, nil
		}
		v.size = n
		keep := min(n, maxBulkKeep)
		buf := make([]byte, keep)
		if _, err := io.ReadFull(r, buf); err != nil {
			return value{}, err
		}
		if n > keep {
			v.truncated = true
			if _, err := io.CopyN(io.Discard, r, n-keep); err != nil {
				return value{}, err
			}
		}
		var crlf [2]byte
		if _, err := io.ReadFull(r, crlf[:]); err != nil {
			return value{}, err
		}
		if crlf != [2]byte{'\r', '\n'} {
			return value{}, errors.New("Redis sent a bulk string without its line end")
		}
		v.str = string(buf)
	case '*':
		n, err := strconv.ParseInt(rest, 10, 64)
		if err != nil || n < -1 || n > maxArray {
			return value{}, fmt.Errorf("Redis sent a bad array length %q", rest)
		}
		if n == -1 {
			v.null = true
			return v, nil
		}
		v.array = make([]value, 0, n)
		for range n {
			e, err := readValue(r, depth+1)
			if err != nil {
				return value{}, err
			}
			v.array = append(v.array, e)
		}
	default:
		return value{}, fmt.Errorf("Redis sent an unknown reply type %q", v.kind)
	}
	return v, nil
}
