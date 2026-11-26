package gitdeploy

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nefocp/nefo/internal/jobs"
	"github.com/nefocp/nefo/internal/store"
)

// The repository's host cannot reach the panel's own port, which is
// firewalled to the administrator. So each site's Nginx server block sends
// POST /.panel/deploy/<site id> on the site's own domain to a small
// listener on HookAddr, which serves POST /git/<site id>.

const (
	// HookAddr is where the webhook listener runs. Nginx proxies to it.
	HookAddr = "127.0.0.1:7809"
	// HookPort is HookAddr's port, which app sites may not use.
	HookPort = 7809

	maxHookBody = 1 << 20
)

// Trigger describes a push that should be deployed.
type Trigger struct {
	SiteID   int64
	Provider string // "github", "gitlab" or "gitea" (also Forgejo and Gogs)
	Ref      string // refs/heads/<branch>
	Commit   string // the pushed commit, if the host sent it
	IP       string // the address that sent the webhook
}

// DeployFunc starts a deploy job. It must not wait for the job to finish.
// When another job is running it returns an error that matches
// jobs.ErrBusy; the handler then tries again later.
type DeployFunc func(Trigger) error

// HookStore gives a site's Git settings; *store.Store implements it.
type HookStore interface {
	GitDeploy(ctx context.Context, siteID int64) (*store.GitDeploy, error)
}

// HookHandler serves the webhooks of all sites.
type HookHandler struct {
	store  HookStore
	deploy DeployFunc
	log    *slog.Logger

	// While other tasks run, a deploy is tried again every retryEvery
	// for up to retryFor.
	retryEvery time.Duration
	retryFor   time.Duration

	mu      sync.Mutex
	pending map[int64]bool // sites with a deploy waiting for its turn
}

// NewHookHandler returns the webhook handler. deploy is called for a push
// to a site's configured branch.
func NewHookHandler(st HookStore, deploy DeployFunc, log *slog.Logger) *HookHandler {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &HookHandler{
		store: st, deploy: deploy, log: log,
		retryEvery: 15 * time.Second, retryFor: time.Hour,
		pending: map[int64]bool{},
	}
}

// ListenAndServe runs h on addr until ctx is done.
func ListenAndServe(ctx context.Context, addr string, h http.Handler) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       time.Minute,
		MaxHeaderBytes:    64 << 10,
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			srv.Shutdown(sctx)
		case <-done:
		}
	}()
	if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func reply(w http.ResponseWriter, code int, key, value string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{key: value})
}

// ServeHTTP implements http.Handler.
func (h *HookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rest, ok := strings.CutPrefix(r.URL.Path, "/git/")
	id, err := strconv.ParseInt(rest, 10, 64)
	if !ok || err != nil || id <= 0 || strconv.FormatInt(id, 10) != rest {
		reply(w, http.StatusNotFound, "error", "not_found")
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		reply(w, http.StatusMethodNotAllowed, "error", "method_not_allowed")
		return
	}
	g, err := h.store.GitDeploy(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) || (err == nil && g.HookSecret == "") {
		reply(w, http.StatusNotFound, "error", "not_found")
		return
	}
	if err != nil {
		h.log.Error("git webhook", "site", id, "err", err)
		reply(w, http.StatusInternalServerError, "error", "internal")
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxHookBody))
	if err != nil {
		if tooLarge := new(http.MaxBytesError); errors.As(err, &tooLarge) {
			reply(w, http.StatusRequestEntityTooLarge, "error", "too_large")
		} else {
			reply(w, http.StatusBadRequest, "error", "bad_request")
		}
		return
	}
	provider, ok := verify(r.Header, body, g.HookSecret)
	if !ok {
		h.log.Warn("git webhook with a bad signature", "site", id, "ip", remoteIP(r))
		reply(w, http.StatusUnauthorized, "error", "bad_signature")
		return
	}

	switch event := eventName(r.Header); event {
	case "ping": // GitHub's test when the webhook is added
		reply(w, http.StatusOK, "status", "pong")
		return
	case "push", "":
	default:
		reply(w, http.StatusOK, "status", "ignored: "+event+" event")
		return
	}
	p, err := parsePush(r.Header.Get("Content-Type"), body)
	if err != nil {
		reply(w, http.StatusBadRequest, "error", "bad_payload")
		return
	}
	if p.Ref != "refs/heads/"+g.Branch {
		reply(w, http.StatusOK, "status", "ignored: not branch "+g.Branch)
		return
	}
	if p.deleted() {
		reply(w, http.StatusOK, "status", "ignored: branch deleted")
		return
	}

	t := Trigger{SiteID: id, Provider: provider, Ref: p.Ref, Commit: p.After, IP: remoteIP(r)}
	switch err := h.deploy(t); {
	case err == nil:
		reply(w, http.StatusAccepted, "status", "deploying")
	case errors.Is(err, jobs.ErrBusy):
		h.later(t)
		reply(w, http.StatusAccepted, "status", "queued: another task is running")
	default:
		h.log.Warn("git webhook: deploy did not start", "site", id, "err", err)
		reply(w, http.StatusInternalServerError, "error", "deploy_not_started")
	}
}

// later tries a deploy again while other tasks run. Several pushes in the
// meantime need only one deploy: it fetches the branch's newest commit.
func (h *HookHandler) later(t Trigger) {
	h.mu.Lock()
	if h.pending[t.SiteID] {
		h.mu.Unlock()
		return
	}
	h.pending[t.SiteID] = true
	h.mu.Unlock()
	go func() {
		defer func() {
			h.mu.Lock()
			delete(h.pending, t.SiteID)
			h.mu.Unlock()
		}()
		for deadline := time.Now().Add(h.retryFor); time.Now().Before(deadline); {
			time.Sleep(h.retryEvery)
			err := h.deploy(t)
			if errors.Is(err, jobs.ErrBusy) {
				continue
			}
			if err != nil {
				h.log.Warn("git webhook: deploy did not start", "site", t.SiteID, "err", err)
			}
			return
		}
		h.log.Warn("git webhook: other tasks ran too long, deploy skipped", "site", t.SiteID)
	}()
}

// remoteIP is the webhook sender's address: Nginx and Apache pass it in
// X-Real-IP. OpenLiteSpeed cannot set that header; it appends the sender
// to X-Forwarded-For.
func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if real := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); real != nil {
			return real.String()
		}
		if xff := r.Header.Values("X-Forwarded-For"); len(xff) > 0 {
			parts := strings.Split(xff[len(xff)-1], ",")
			if last := net.ParseIP(strings.TrimSpace(parts[len(parts)-1])); last != nil {
				return last.String()
			}
		}
	}
	return host
}

// verify checks a webhook's signature or token and returns who sent it.
// GitHub signs the body with HMAC-SHA256 (X-Hub-Signature-256:
// sha256=<hex>), Gitea, Forgejo and Gogs too (X-Gitea-Signature: <hex>,
// and they also send GitHub's header), and GitLab sends the secret itself
// (X-Gitlab-Token). All comparisons take constant time.
func verify(h http.Header, body []byte, secret string) (provider string, ok bool) {
	if secret == "" {
		return "", false
	}
	sum := func() []byte {
		m := hmac.New(sha256.New, []byte(secret))
		m.Write(body)
		return m.Sum(nil)
	}
	gitea := h.Get("X-Gitea-Event") != "" || h.Get("X-Forgejo-Event") != "" || h.Get("X-Gogs-Event") != ""
	if sig := h.Get("X-Hub-Signature-256"); sig != "" {
		provider = "github"
		if gitea {
			provider = "gitea"
		}
		hexSig, found := strings.CutPrefix(sig, "sha256=")
		got, err := hex.DecodeString(hexSig)
		return provider, found && err == nil && hmac.Equal(got, sum())
	}
	for _, name := range []string{"X-Gitea-Signature", "X-Forgejo-Signature", "X-Gogs-Signature"} {
		if sig := h.Get(name); sig != "" {
			got, err := hex.DecodeString(sig)
			return "gitea", err == nil && hmac.Equal(got, sum())
		}
	}
	if token := h.Get("X-Gitlab-Token"); token != "" {
		// Hash both so the comparison does not depend on their lengths.
		a, b := sha256.Sum256([]byte(token)), sha256.Sum256([]byte(secret))
		return "gitlab", subtle.ConstantTimeCompare(a[:], b[:]) == 1
	}
	return "", false
}

// eventName returns the webhook's event in lower case ("push", "ping"…),
// or "" if no header names it.
func eventName(h http.Header) string {
	for _, name := range []string{"X-GitHub-Event", "X-Gitea-Event", "X-Forgejo-Event", "X-Gogs-Event"} {
		if v := h.Get(name); v != "" {
			return strings.ToLower(strings.TrimSpace(v))
		}
	}
	if v := strings.TrimSpace(h.Get("X-Gitlab-Event")); v != "" {
		// "Push Hook", "Tag Push Hook", "Merge Request Hook"…
		return strings.ReplaceAll(strings.ToLower(strings.TrimSuffix(v, " Hook")), " ", "_")
	}
	return ""
}

type pushPayload struct {
	Ref     string `json:"ref"`
	After   string `json:"after"`
	Deleted bool   `json:"deleted"`
}

// deleted reports a push that deleted the branch: GitHub says so, GitLab
// and Gitea send an all-zero commit.
func (p *pushPayload) deleted() bool {
	return p.Deleted || (len(p.After) >= 40 && strings.Trim(p.After, "0") == "")
}

// parsePush reads a push event, sent as JSON or (GitHub's other choice) as
// a form with the JSON in "payload".
func parsePush(contentType string, body []byte) (*pushPayload, error) {
	if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		v, err := url.ParseQuery(string(body))
		if err != nil {
			return nil, err
		}
		body = []byte(v.Get("payload"))
	}
	var p pushPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, err
	}
	return &p, nil
}
