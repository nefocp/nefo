package ws

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// clientFrame builds a masked client frame.
func clientFrame(fin bool, op int, payload []byte) []byte {
	b0 := byte(op)
	if fin {
		b0 |= 0x80
	}
	f := []byte{b0}
	switch n := len(payload); {
	case n < 126:
		f = append(f, 0x80|byte(n))
	case n <= 0xffff:
		f = append(f, 0x80|126, byte(n>>8), byte(n))
	default:
		f = append(f, 0x80|127)
		f = binary.BigEndian.AppendUint64(f, uint64(n))
	}
	var mask [4]byte
	rand.Read(mask[:])
	f = append(f, mask[:]...)
	for i, c := range payload {
		f = append(f, c^mask[i%4])
	}
	return f
}

func TestEcho(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := Upgrade(w, r)
		if err != nil {
			return
		}
		for {
			op, msg, err := c.ReadMessage()
			if err != nil {
				return
			}
			c.WriteMessage(op, append([]byte("echo:"), msg...))
		}
	}))
	defer srv.Close()

	conn, err := net.Dial("tcp", strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n" +
		"Sec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n"))
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil || resp.StatusCode != 101 {
		t.Fatalf("handshake: %v %v", resp, err)
	}
	// The RFC 6455 example key and accept value.
	if got := resp.Header.Get("Sec-WebSocket-Accept"); got != "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=" {
		t.Fatalf("accept = %q", got)
	}

	readMsg := func() string {
		var h [2]byte
		br.Read(h[:1])
		br.Read(h[1:])
		n := int(h[1] & 0x7f)
		if n == 126 {
			var b [2]byte
			br.Read(b[:1])
			br.Read(b[1:])
			n = int(binary.BigEndian.Uint16(b[:]))
		}
		buf := make([]byte, n)
		for got := 0; got < n; {
			k, _ := br.Read(buf[got:])
			got += k
		}
		return string(buf)
	}

	conn.Write(clientFrame(true, Text, []byte("hello")))
	if got := readMsg(); got != "echo:hello" {
		t.Fatalf("got %q", got)
	}
	// A fragmented message with a ping in the middle.
	conn.Write(clientFrame(false, Text, []byte("frag")))
	conn.Write(clientFrame(true, pingF, []byte("p")))
	conn.Write(clientFrame(true, 0, []byte("ment")))
	if got := readMsg(); got != "p" { // the pong comes first
		t.Fatalf("pong = %q", got)
	}
	if got := readMsg(); got != "echo:fragment" {
		t.Fatalf("got %q", got)
	}
}

func TestRejectsUnmasked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := Upgrade(w, r)
		if err != nil {
			return
		}
		defer c.Close()
		if _, _, err := c.ReadMessage(); err == nil {
			t.Error("unmasked frame accepted")
		}
	}))
	defer srv.Close()
	conn, _ := net.Dial("tcp", strings.TrimPrefix(srv.URL, "http://"))
	defer conn.Close()
	conn.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n" +
		"Sec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n"))
	br := bufio.NewReader(conn)
	http.ReadResponse(br, nil)
	conn.Write([]byte{0x81, 0x02, 'h', 'i'})
	buf := make([]byte, 1)
	br.Read(buf)
}

func TestCloseWithReason(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := Upgrade(w, r)
		if err != nil {
			return
		}
		c.CloseWith(4001, "busy")
		c.Close() // must not send a second frame
	}))
	defer srv.Close()
	conn, _ := net.Dial("tcp", strings.TrimPrefix(srv.URL, "http://"))
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	conn.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n" +
		"Sec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n"))
	br := bufio.NewReader(conn)
	http.ReadResponse(br, nil)
	rest, _ := io.ReadAll(br)
	want := []byte{0x88, 6, 0x0f, 0xa1, 'b', 'u', 's', 'y'}
	if !bytes.Equal(rest, want) {
		t.Fatalf("got % x, want % x", rest, want)
	}
}
