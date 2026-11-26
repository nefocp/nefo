package api

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/nefocp/nefo/internal/docker"
	"github.com/nefocp/nefo/internal/terminal"
	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/ws"
)

const (
	maxTerminals    = 4
	terminalIdleFor = 30 * time.Minute
)

// openTerminals counts root and container terminals together.
var openTerminals atomic.Int32

// wsCloseFrame is the WebSocket close opcode; package ws exports only the
// data opcodes.
const wsCloseFrame = 8

// refuse ends a terminal socket with a code and a reason the browser can
// show. Closing the connection right after the close frame, with the
// browser's first message (its size) still unread, makes the kernel reset
// the connection, and the browser may then never see the reason. So the
// socket stays open until the browser answers the close, for a second at
// most.
func refuse(conn *ws.Conn, code int, reason string) {
	conn.WriteMessage(wsCloseFrame, append(binary.BigEndian.AppendUint16(nil, uint16(code)), reason...))
	conn.SetReadDeadline(time.Now().Add(time.Second))
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
	conn.Close()
}

// sameOrigin rejects cross-site WebSocket handshakes. Browsers do not apply
// CORS to WebSockets, so the Origin header must be checked by hand.
func sameOrigin(r *http.Request) bool {
	u, err := url.Parse(r.Header.Get("Origin"))
	return err == nil && u.Host != "" && u.Host == r.Host
}

// handleTerminal opens a root shell over a WebSocket. Messages from the
// browser are JSON: {"t":"i","d":"keys"} or {"t":"r","r":rows,"c":cols};
// the shell's output is sent as binary messages.
func (s *Server) handleTerminal(w http.ResponseWriter, r *http.Request, a *authed) {
	s.serveTerminal(w, r, a, "terminal", func(context.Context) (*terminal.Session, string, error) {
		sess, err := terminal.Start(24, 80)
		return sess, "", err
	})
}

// handleContainerTerminal opens a shell inside a running container, with
// the same protocol and limits as the root terminal.
func (s *Server) handleContainerTerminal(w http.ResponseWriter, r *http.Request, a *authed) {
	id := r.PathValue("id")
	if err := docker.ValidContainer(id); err != nil {
		s.opError(w, r, err)
		return
	}
	s.serveTerminal(w, r, a, "container_terminal", func(ctx context.Context) (*terminal.Session, string, error) {
		if !s.docker.Installed() {
			return nil, "", usererr.New("docker_needed")
		}
		ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		argv, name, err := s.docker.Shell(ctx, id)
		if err != nil {
			return nil, "", err
		}
		sess, err := terminal.Start(24, 80, argv...)
		return sess, name, err
	})
}

// serveTerminal connects a WebSocket to the session that start opens. The
// audit log records kind+"_opened" and kind+"_closed", with the detail
// that start returns (such as a container's name).
func (s *Server) serveTerminal(w http.ResponseWriter, r *http.Request, a *authed, kind string,
	start func(context.Context) (*terminal.Session, string, error)) {
	if !sameOrigin(r) {
		writeError(w, http.StatusForbidden, "csrf")
		return
	}
	// A browser cannot read the body of a refused handshake, so refusals
	// after this point are sent as a close code and reason it can show.
	conn, err := ws.Upgrade(w, r)
	if err != nil {
		return
	}
	defer conn.Close()
	if openTerminals.Add(1) > maxTerminals {
		openTerminals.Add(-1)
		refuse(conn, 4001, "too_many_terminals")
		return
	}
	defer openTerminals.Add(-1)

	sess, detail, err := start(r.Context())
	if err != nil {
		var ue *usererr.Error
		if errors.As(err, &ue) {
			refuse(conn, 4003, ue.Code)
			return
		}
		s.log.Warn("terminal", "kind", kind, "err", err)
		refuse(conn, 4002, "terminal_failed")
		return
	}
	defer sess.Close()
	s.audit(r.Context(), a.user.Username, kind+"_opened", clientIP(r), detail)
	started := time.Now()

	// Shell output → browser.
	go func() {
		buf := make([]byte, 32<<10)
		for {
			n, err := sess.Read(buf)
			if n > 0 {
				if conn.WriteMessage(ws.Binary, buf[:n]) != nil {
					break
				}
			}
			if err != nil {
				break
			}
		}
		conn.Close()
	}()

	// Browser → shell, until the socket closes or stays idle too long.
	for {
		conn.SetReadDeadline(time.Now().Add(terminalIdleFor))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var m struct {
			T string `json:"t"`
			D string `json:"d"`
			R int    `json:"r"`
			C int    `json:"c"`
		}
		if json.Unmarshal(msg, &m) != nil {
			continue
		}
		switch m.T {
		case "i":
			sess.Write([]byte(m.D))
		case "r":
			sess.Resize(m.R, m.C)
		}
	}
	took := time.Since(started).Round(time.Second).String()
	if detail != "" {
		took = detail + " " + took
	}
	// The request context is cancelled when the panel shuts down; the
	// record must still be written.
	s.audit(context.WithoutCancel(r.Context()), a.user.Username, kind+"_closed", clientIP(r), took)
}
