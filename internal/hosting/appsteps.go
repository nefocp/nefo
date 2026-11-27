package hosting

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/nefocp/nefo/internal/appstore"
	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/store"
)

// Set-up steps are data from the recipe: an HTTP request to the app on
// 127.0.0.1, or a program run in a container with its input on stdin.
// Passwords are only ever in request bodies and on stdin, never on a
// command line.

// stepRetryFor is how long an HTTP step waits for an app that is still
// loading; tests shorten it.
var stepRetryFor, stepRetryWait = 3 * time.Minute, 3 * time.Second

// stepJars keeps cookies between the HTTP steps of one instance's set-up
// (Vaultwarden: log in to /admin, then invite).
var stepJars = map[string]http.CookieJar{}

func (m *Manager) runStep(ctx context.Context, inst *store.App, env *appstore.Env, st appstore.Step, lg Logger) error {
	switch {
	case st.HTTP != nil:
		return m.httpStep(ctx, inst, env, st.HTTP, lg)
	case st.Exec != nil:
		return m.execStep(ctx, inst, env, st.Exec, false, lg)
	case st.Run != nil:
		return m.execStep(ctx, inst, env, st.Run, true, lg)
	}
	return errors.New("empty step")
}

func (m *Manager) httpStep(ctx context.Context, inst *store.App, env *appstore.Env, h *appstore.HTTPStep, lg Logger) error {
	path, err := env.Expand(h.Path)
	if err != nil {
		return err
	}
	lg.Logf("Setting up the app (%s %s)", h.Method, path)
	var body []byte
	ctype := ""
	switch {
	case h.Form != nil:
		v := url.Values{}
		for k, x := range h.Form {
			s, err := env.Expand(x)
			if err != nil {
				return err
			}
			v.Set(k, s)
		}
		body, ctype = []byte(v.Encode()), "application/x-www-form-urlencoded"
	case h.JSON != nil:
		x, err := expandJSON(env, h.JSON)
		if err != nil {
			return err
		}
		b, err := json.Marshal(x)
		if err != nil {
			return err
		}
		body, ctype = b, "application/json"
	}
	// A step uses the cookies of the steps before it; keep_cookies passes
	// its own on to the next step (Vaultwarden: log in, then invite).
	jar := stepJars[inst.Name]
	if jar == nil {
		jar, _ = cookiejar.New(nil)
		stepJars[inst.Name] = jar
	}
	defer func() {
		if !h.KeepCookies {
			delete(stepJars, inst.Name)
		}
	}()
	client := &http.Client{Timeout: time.Minute, Jar: jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	headers := http.Header{}
	host := ""
	if ctype != "" {
		headers.Set("Content-Type", ctype)
	}
	for k, x := range h.Headers {
		v, err := env.Expand(x)
		if err != nil {
			return err
		}
		if strings.EqualFold(k, "Host") {
			host = v
		} else {
			headers.Set(k, v)
		}
	}
	// Some apps answer their health check before they take requests
	// (Jellyfin answers 503 while it loads): a refused connection or a
	// 502/503/504 is tried again for a while, since the app did not handle
	// the request. Any other answer counts.
	var status int
	var answer []byte
	for deadline := time.Now().Add(stepRetryFor); ; {
		req, err := http.NewRequestWithContext(ctx, h.Method, "http://127.0.0.1:"+strconv.Itoa(inst.Port)+path, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header = headers.Clone()
		if host != "" {
			req.Host = host
		}
		resp, err := client.Do(req)
		if err == nil {
			status = resp.StatusCode
			answer, _ = io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			resp.Body.Close()
			if status != http.StatusBadGateway && status != http.StatusServiceUnavailable && status != http.StatusGatewayTimeout {
				break
			}
		}
		if time.Now().After(deadline) {
			if err != nil {
				return fmt.Errorf("set-up request %s: %w", path, err)
			}
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(stepRetryWait):
		}
	}
	if !slices.Contains(h.Expect, status) {
		return fmt.Errorf("set-up request %s answered HTTP %d", path, status)
	}
	if len(h.Save) > 0 {
		var doc any
		if err := json.Unmarshal(answer, &doc); err != nil {
			return fmt.Errorf("set-up request %s: the answer is not JSON", path)
		}
		if env.Steps == nil {
			env.Steps = map[string]string{}
		}
		for name, p := range h.Save {
			v, ok := jsonPath(doc, p)
			if !ok {
				return fmt.Errorf("set-up request %s: no %s in the answer", path, p)
			}
			env.Steps[name] = v
		}
	}
	return nil
}

// expandJSON fills the placeholders of every string in a JSON body.
func expandJSON(env *appstore.Env, v any) (any, error) {
	switch x := v.(type) {
	case string:
		return env.Expand(x)
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			y, err := expandJSON(env, e)
			if err != nil {
				return nil, err
			}
			out[k] = y
		}
		return out, nil
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			y, err := expandJSON(env, e)
			if err != nil {
				return nil, err
			}
			out[i] = y
		}
		return out, nil
	}
	return v, nil
}

// jsonPath returns the string or number at a dotted path ("data.token") of
// a JSON answer.
func jsonPath(doc any, p string) (string, bool) {
	for _, k := range strings.Split(p, ".") {
		m, ok := doc.(map[string]any)
		if !ok {
			return "", false
		}
		if doc, ok = m[k]; !ok {
			return "", false
		}
	}
	switch x := doc.(type) {
	case string:
		return x, x != ""
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), true
	}
	return "", false
}

func (m *Manager) execStep(ctx context.Context, inst *store.App, env *appstore.Env, e *appstore.ExecStep, oneOff bool, lg Logger) error {
	var argv []string
	for _, a := range e.Argv {
		s, err := env.Expand(a)
		if err != nil {
			return err
		}
		argv = append(argv, s)
	}
	stdin, err := env.Expand(e.Stdin)
	if err != nil {
		return err
	}
	timeout := time.Duration(e.TimeoutS) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var args []string
	if oneOff {
		lg.Logf("Setting up the app (one-off %s container)", e.Service)
		args = composeArgs(inst.Name, append([]string{"run", "--rm", "--no-deps", "-T", e.Service}, argv...)...)
	} else {
		lg.Logf("Setting up the app (in the %s container)", e.Service)
		args = composeArgs(inst.Name, append([]string{"exec", "-T", e.Service}, argv...)...)
	}
	out, err := m.run.Input(ctx, []byte(stdin), "docker", args...)
	if err != nil {
		return fmt.Errorf("set-up step in %s failed: %w", e.Service, err)
	}
	if e.ExpectStdout != "" && !strings.Contains(out, e.ExpectStdout) {
		return fmt.Errorf("set-up step in %s did not finish:\n%s", e.Service, run.Tail(out, 10))
	}
	return nil
}

// waitHTTPPath waits until 127.0.0.1:port answers path below HTTP 500.
func waitHTTPPath(ctx context.Context, port int, path string, limit time.Duration) error {
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	deadline := time.Now().Add(limit)
	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(port)+path, nil)
		if resp, err := client.Do(req); err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return errors.New("no answer in time")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

// waitHTTP waits until something answers HTTP on 127.0.0.1:port.
func waitHTTP(ctx context.Context, port int, limit time.Duration) error {
	return waitHTTPPath(ctx, port, "/", limit)
}

func bcryptHash(pw string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), 10)
	return string(h), err
}
