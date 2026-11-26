package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/nefocp/nefo/internal/setup"
)

// The local socket lets "nefo setup" ask the running panel to do the
// installs, so they run as one of the panel's own jobs: one at a time with
// its other jobs, and with the panel's view of the server (which web server
// it manages, for example) kept up to date. Without it, the command and
// the panel would only have dpkg's locks between them.
//
// The socket is in the data folder (readable by root only) and has mode
// 0600: whoever can open it is root, the same as for "nefo reset-password".

// LocalSocket is the path of the panel's local socket.
func LocalSocket(dataDir string) string { return filepath.Join(dataDir, "nefo.sock") }

// ListenLocal creates the local socket. The panel calls it before it
// listens on its port, so a command that finds the port open finds the
// socket too.
func (s *Server) ListenLocal() (net.Listener, error) {
	path := LocalSocket(s.cfg.DataDir)
	// A panel that was killed leaves its socket behind.
	if fi, err := os.Lstat(path); err == nil && fi.Mode()&os.ModeSocket != 0 {
		os.Remove(path)
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		return nil, err
	}
	return ln, nil
}

// ServeLocal answers on the local socket until ctx is done. Closing the
// listener removes the socket file.
func (s *Server) ServeLocal(ctx context.Context, ln net.Listener) {
	hs := &http.Server{Handler: s.localHandler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		hs.Close()
	}()
	if err := hs.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		s.log.Warn("local socket", "err", err)
	}
}

// LocalLog is a part of a job's log, for a command that follows the job.
type LocalLog struct {
	Status string   `json:"status"`
	Error  string   `json:"error,omitempty"`
	Lines  []string `json:"lines"`
	Next   int      `json:"next"`
}

// localHandler serves the local socket: the setup and its job's log.
// Audit entries name the user "root", from "cli".
func (s *Server) localHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /setup", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.SetupView(r.Context())
		if err != nil {
			s.opError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, v)
	})
	mux.HandleFunc("POST /setup", func(w http.ResponseWriter, r *http.Request) {
		var c setup.Choice
		if !decodeJSON(w, r, &c) {
			return
		}
		job, plan, err := s.startSetup(r.Context(), c, "root", "cli")
		if err != nil {
			s.webServerError(w, r, err)
			return
		}
		writeJSON(w, http.StatusAccepted, setupStarted{job.Snapshot(), plan})
	})
	mux.HandleFunc("POST /setup/skip", func(w http.ResponseWriter, r *http.Request) {
		if err := s.SkipSetup(r.Context(), "root", "cli"); err != nil {
			s.opError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	mux.HandleFunc("GET /jobs/{id}", func(w http.ResponseWriter, r *http.Request) {
		j, ok := s.jobs.Get(r.PathValue("id"))
		if !ok {
			writeError(w, http.StatusNotFound, "not_found")
			return
		}
		from, _ := strconv.Atoi(r.URL.Query().Get("from"))
		// The state first: once it says finished, the lines are complete.
		status, errText := j.State()
		lines, next := j.LogFrom(from)
		writeJSON(w, http.StatusOK, LocalLog{Status: status, Error: errText, Lines: lines, Next: next})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { writeError(w, http.StatusNotFound, "not_found") })
	return mux
}

// ---- client ----

// ErrNoPanel means no panel answers on the local socket.
var ErrNoPanel = errors.New("the panel is not running")

// LocalError is a refusal from the panel: an error code of the API, such
// as "busy", and its detail.
type LocalError struct {
	Code   string
	Detail string
}

func (e *LocalError) Error() string {
	if e.Detail != "" {
		return e.Code + ": " + e.Detail
	}
	return e.Code
}

// LocalClient talks to a running panel through its local socket.
type LocalClient struct{ hc *http.Client }

// NewLocalClient returns a client for the socket at path.
func NewLocalClient(path string) *LocalClient {
	tr := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			c, err := d.DialContext(ctx, "unix", path)
			if err != nil {
				return nil, fmt.Errorf("%w (%v)", ErrNoPanel, err)
			}
			return c, nil
		},
		DisableKeepAlives: true,
	}
	return &LocalClient{hc: &http.Client{Transport: tr, Timeout: time.Minute}}
}

func (c *LocalClient) do(ctx context.Context, method, path string, body, out any) error {
	var rd *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://nefo"+path, rd)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		var e struct {
			Error  string `json:"error"`
			Detail string `json:"detail"`
		}
		if json.NewDecoder(resp.Body).Decode(&e) != nil || e.Error == "" {
			e.Error = "http_" + strconv.Itoa(resp.StatusCode)
		}
		return &LocalError{Code: e.Error, Detail: e.Detail}
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// SetupView returns what the setup offers.
func (c *LocalClient) SetupView(ctx context.Context) (setup.View, error) {
	var v setup.View
	err := c.do(ctx, http.MethodGet, "/setup", nil, &v)
	return v, err
}

// StartSetup starts the setup in the panel and returns its job's ID.
func (c *LocalClient) StartSetup(ctx context.Context, ch setup.Choice) (string, error) {
	var res setupStarted
	err := c.do(ctx, http.MethodPost, "/setup", ch, &res)
	return res.ID, err
}

// SkipSetup marks the wizard skipped.
func (c *LocalClient) SkipSetup(ctx context.Context) error {
	var res map[string]bool
	return c.do(ctx, http.MethodPost, "/setup/skip", struct{}{}, &res)
}

// JobLog returns the job's state and its log lines from line from on.
func (c *LocalClient) JobLog(ctx context.Context, id string, from int) (LocalLog, error) {
	var l LocalLog
	err := c.do(ctx, http.MethodGet, "/jobs/"+id+"?from="+strconv.Itoa(from), nil, &l)
	return l, err
}
