// Package ws is a small WebSocket server (RFC 6455): enough for the panel's
// terminal, without a third-party dependency. It handles the handshake,
// masked client frames, fragmentation, ping/pong and close.
package ws

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Message types.
const (
	Text   = 1
	Binary = 2
	closeF = 8
	pingF  = 9
	pongF  = 10
)

const maxMessage = 1 << 20

// ErrClosed is returned once the peer has closed the connection.
var ErrClosed = errors.New("websocket closed")

// Conn is a server-side WebSocket connection.
type Conn struct {
	conn      net.Conn
	r         *bufio.Reader
	wmu       sync.Mutex
	closeOnce sync.Once
}

func headerHas(h http.Header, name, token string) bool {
	for _, v := range h.Values(name) {
		for _, t := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(t), token) {
				return true
			}
		}
	}
	return false
}

// Upgrade completes the handshake. The caller must have checked the Origin.
func Upgrade(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	key := r.Header.Get("Sec-WebSocket-Key")
	if r.Method != http.MethodGet || !headerHas(r.Header, "Connection", "upgrade") ||
		!headerHas(r.Header, "Upgrade", "websocket") || r.Header.Get("Sec-WebSocket-Version") != "13" || key == "" {
		http.Error(w, "websocket handshake expected", http.StatusBadRequest)
		return nil, errors.New("not a websocket handshake")
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "websocket not supported", http.StatusInternalServerError)
		return nil, errors.New("response cannot be hijacked")
	}
	conn, rw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}
	// The server's read and write timeouts still apply to a hijacked
	// connection; the caller sets its own deadlines from here on.
	conn.SetDeadline(time.Time{})
	sum := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	resp := "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + base64.StdEncoding.EncodeToString(sum[:]) + "\r\n\r\n"
	if _, err := conn.Write([]byte(resp)); err != nil {
		conn.Close()
		return nil, err
	}
	return &Conn{conn: conn, r: rw.Reader}, nil
}

// SetReadDeadline sets the deadline for the next ReadMessage.
func (c *Conn) SetReadDeadline(t time.Time) error { return c.conn.SetReadDeadline(t) }

// ReadMessage returns the next data message. Pings are answered and a close
// frame ends the connection with ErrClosed.
func (c *Conn) ReadMessage() (int, []byte, error) {
	var msg []byte
	op := 0
	for {
		fin, opcode, payload, err := c.readFrame()
		if err != nil {
			return 0, nil, err
		}
		switch opcode {
		case pingF:
			c.write(pongF, payload)
			continue
		case pongF:
			continue
		case closeF:
			c.Close()
			return 0, nil, ErrClosed
		case 0: // continuation
			if op == 0 {
				return 0, nil, errors.New("unexpected continuation frame")
			}
		case Text, Binary:
			if op != 0 {
				return 0, nil, errors.New("interleaved data frames")
			}
			op = opcode
		default:
			return 0, nil, errors.New("unknown opcode")
		}
		msg = append(msg, payload...)
		if len(msg) > maxMessage {
			return 0, nil, errors.New("message too large")
		}
		if fin {
			return op, msg, nil
		}
	}
}

func (c *Conn) readFrame() (fin bool, opcode int, payload []byte, err error) {
	var h [2]byte
	if _, err = io.ReadFull(c.r, h[:]); err != nil {
		return
	}
	fin, opcode = h[0]&0x80 != 0, int(h[0]&0x0f)
	if h[0]&0x70 != 0 {
		return false, 0, nil, errors.New("reserved bits set")
	}
	if h[1]&0x80 == 0 {
		return false, 0, nil, errors.New("client frames must be masked")
	}
	n := uint64(h[1] & 0x7f)
	switch n {
	case 126:
		var b [2]byte
		if _, err = io.ReadFull(c.r, b[:]); err != nil {
			return
		}
		n = uint64(binary.BigEndian.Uint16(b[:]))
	case 127:
		var b [8]byte
		if _, err = io.ReadFull(c.r, b[:]); err != nil {
			return
		}
		n = binary.BigEndian.Uint64(b[:])
	}
	if n > maxMessage || (opcode >= 8 && (n > 125 || !fin)) {
		return false, 0, nil, errors.New("frame too large")
	}
	var mask [4]byte
	if _, err = io.ReadFull(c.r, mask[:]); err != nil {
		return
	}
	payload = make([]byte, n)
	if _, err = io.ReadFull(c.r, payload); err != nil {
		return
	}
	for i := range payload {
		payload[i] ^= mask[i%4]
	}
	return fin, opcode, payload, nil
}

// WriteMessage sends one unfragmented message. It is safe for concurrent use.
func (c *Conn) WriteMessage(op int, data []byte) error { return c.write(op, data) }

func (c *Conn) write(op int, data []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	hdr := []byte{0x80 | byte(op)}
	switch n := len(data); {
	case n < 126:
		hdr = append(hdr, byte(n))
	case n <= 0xffff:
		hdr = append(hdr, 126, byte(n>>8), byte(n))
	default:
		hdr = append(hdr, 127)
		hdr = binary.BigEndian.AppendUint64(hdr, uint64(n))
	}
	c.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
	if _, err := c.conn.Write(append(hdr, data...)); err != nil {
		return err
	}
	return nil
}

// Close sends a close frame and closes the connection.
func (c *Conn) Close() error { return c.CloseWith(0, "") }

// CloseWith sends a close frame carrying a status code and a short reason,
// which the browser sees in its close event, then closes the connection.
// Only the first Close or CloseWith sends a frame.
func (c *Conn) CloseWith(code int, reason string) error {
	err := net.ErrClosed
	c.closeOnce.Do(func() {
		var p []byte
		if code != 0 {
			if len(reason) > 123 { // a control frame payload is at most 125 bytes
				reason = reason[:123]
			}
			p = append(binary.BigEndian.AppendUint16(nil, uint16(code)), reason...)
		}
		c.write(closeF, p)
		err = c.conn.Close()
	})
	return err
}
