package api

import (
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"net/netip"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/nefocp/nefo/internal/auth"
	"github.com/nefocp/nefo/internal/secure"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// API keys let scripts (CI deploys, backups, monitoring) use the API without
// a browser session: "Authorization: Bearer nefo_…", under the entrance path
// like every other request. A key is "nefo_" and 32 random bytes in
// base64url; the panel keeps only its SHA-256 hash.
const (
	apiKeyPrefix       = "nefo_"
	apiKeyRandomBytes  = 32
	apiKeyLen          = len(apiKeyPrefix) + 43 // 32 bytes in unpadded base64url
	apiKeyShown        = len(apiKeyPrefix) + 6  // kept to tell the keys apart
	maxAPIKeys         = 20
	maxAPIKeyAddresses = 20
	maxAPIKeyName      = 32
	// A key's last use is written at most once a minute, not on each request.
	apiKeyTouchEvery = time.Minute
)

// sessionOnlyPrefixes are refused to API keys whatever their route's
// wrapper (s.sessionOnly or s.requireAuth): the database tools proxied
// behind the sign-in, the account's own routes and the API keys. Add a
// prefix here for a new proxied tool; the routes that are not under one
// prefix use s.sessionOnly.
var sessionOnlyPrefixes = []string{"/adminer/", "/phpmyadmin/", "/api/auth/", "/api/settings/api-keys"}

func sessionOnlyPath(path string) bool {
	for _, p := range sessionOnlyPrefixes {
		if strings.HasPrefix(path, p) || path+"/" == p {
			return true
		}
	}
	return false
}

// readOnlyRefused are the reads a read-only key may not make, by route
// pattern: they reveal passwords and secrets, or whole files and
// databases, which would make a "read-only" key as good as the
// administrator's. A full key may make them.
var readOnlyRefused = map[string]bool{
	"GET /api/apps/installed/{name}/credentials": true,
	"GET /api/databases/{id}/credentials":        true,
	"GET /api/databases/{id}/export":             true,
	"GET /api/backups/{id}/download":             true,
	"GET /api/files":                             true,
	"GET /api/files/content":                     true,
	"GET /api/files/download":                    true,
	"GET /api/files/search":                      true,
	"GET /api/files/trash":                       true,
	"GET /api/redis/key":                         true, // values: sessions, tokens
	"GET /api/sites/{id}/git":                    true, // the webhook secret
}

// The expiry choices, in days; 0 is never.
var apiKeyExpiryDays = map[int]bool{0: true, 30: true, 90: true, 365: true}

// bearerToken returns the token of the request's "Authorization: Bearer"
// header, and whether it has one. Keys are read from this header only, never
// from a cookie or the query string: a browser never adds the header by
// itself, and a page on another site cannot set it without a CORS preflight,
// which the panel never answers. Two Bearer headers give an empty token,
// which fails.
func bearerToken(r *http.Request) (string, bool) {
	token, found := "", false
	for _, v := range r.Header.Values("Authorization") {
		scheme, rest, _ := strings.Cut(strings.TrimSpace(v), " ")
		if !strings.EqualFold(scheme, "Bearer") {
			continue
		}
		if found {
			return "", true
		}
		token, found = strings.TrimSpace(rest), true
	}
	return token, found
}

func hasBearer(r *http.Request) bool {
	_, ok := bearerToken(r)
	return ok
}

// serveWithKey calls h for a request that carries an API key, if the key
// is valid and allows the request. Every kind of failure (unknown, revoked,
// expired, wrong address) gets the same 401, counts against the sign-in
// limit, and is audited as a failed sign-in, so fail2ban's panel jail bans
// an address that guesses keys.
func (s *Server) serveWithKey(w http.ResponseWriter, r *http.Request, token string, h authedHandler, sessionOnly bool) {
	if s.rateLimited(w, r) {
		return
	}
	ctx, ip, now := r.Context(), clientIP(r), s.now()
	a, failure, err := s.keyAuth(r, token, now)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	if failure != "" {
		s.recordFailure(r)
		name := ""
		if a != nil {
			name = a.user.Username
		}
		s.audit(ctx, name, "login_failed", ip, failure)
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeError(w, http.StatusUnauthorized, "invalid_api_key")
		return
	}
	if now.Sub(a.key.LastUsedAt) >= apiKeyTouchEvery {
		if _, err := s.store.TouchAPIKey(ctx, a.key.ID, now, ip, apiKeyTouchEvery); err != nil {
			s.log.Warn("record API key use", "err", err)
		}
	}

	what := r.Method + " " + truncate(r.URL.Path, 200)
	reads := r.Method == http.MethodGet || r.Method == http.MethodHead
	refusal := ""
	switch {
	case sessionOnly:
		refusal = "api_key_forbidden"
	case !reads && a.key.ReadOnly():
		refusal = "api_key_read_only"
	case a.key.ReadOnly() && readOnlyRefused[r.Pattern]:
		refusal = "api_key_read_only"
	}
	if refusal != "" {
		s.audit(ctx, a.user.Username, "api_key_refused", ip, what)
		writeError(w, http.StatusForbidden, refusal)
		return
	}
	// Each change is in the audit log, also those whose handler logs nothing.
	if !reads {
		s.audit(ctx, a.user.Username, "api_request", ip, what)
	}
	h(w, r, a)
}

// keyAuth finds the key of token and its user. failure says why the key
// may not be used (for the audit log only); a is set when the key exists.
func (s *Server) keyAuth(r *http.Request, token string, now time.Time) (a *authed, failure string, err error) {
	ctx := r.Context()
	keys, err := s.store.APIKeys(ctx)
	if err != nil {
		return nil, "", err
	}
	var k *store.APIKey
	if validAPIKeyFormat(token) {
		// Compared with every key, in constant time: how long it takes
		// says nothing about which key, if any, matched.
		sum := []byte(auth.HashToken(token))
		for _, c := range keys {
			if subtle.ConstantTimeCompare([]byte(c.Hash), sum) == 1 {
				k = c
			}
		}
	}
	if k == nil {
		return nil, "unknown API key", nil
	}
	u, err := s.store.UserByID(ctx, k.UserID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, "unknown API key", nil
	}
	if err != nil {
		return nil, "", err
	}
	// The handlers audit as a.user.Username: "admin (key deploy-bot)".
	u.Username = keyUsername(u.Username, k.Name)
	a = &authed{user: u, key: k}
	switch {
	case !k.ExpiresAt.IsZero() && !now.Before(k.ExpiresAt):
		return a, "API key expired", nil
	case !addressAllowed(k.Addresses, clientIP(r)):
		return a, "API key not allowed from this address", nil
	}
	return a, "", nil
}

func keyUsername(user, key string) string { return user + " (key " + key + ")" }

func newAPIKey() (token, hash string) {
	token = apiKeyPrefix + base64.RawURLEncoding.EncodeToString(secure.Bytes(apiKeyRandomBytes))
	return token, auth.HashToken(token)
}

func validAPIKeyFormat(token string) bool {
	if len(token) != apiKeyLen || !strings.HasPrefix(token, apiKeyPrefix) {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(token[len(apiKeyPrefix):])
	return err == nil
}

// addressAllowed reports whether ip is one of allowed (addresses and CIDR
// networks); an empty list allows any address.
func addressAllowed(allowed []string, ip string) bool {
	if len(allowed) == 0 {
		return true
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	addr = addr.WithZone("").Unmap()
	for _, a := range allowed {
		if p, err := netip.ParsePrefix(a); err == nil {
			if p.Contains(addr) {
				return true
			}
		} else if b, err := netip.ParseAddr(a); err == nil && b == addr {
			return true
		}
	}
	return false
}

// parseKeyAddresses checks and normalises the addresses a key may be used
// from: IPv4 or IPv6 addresses, or networks such as 203.0.113.0/24.
func parseKeyAddresses(in []string) ([]string, error) {
	out, seen := []string{}, map[string]bool{}
	for _, raw := range in {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		var norm string
		if strings.Contains(v, "/") {
			p, err := netip.ParsePrefix(v)
			if err != nil || p.Addr().Is4In6() {
				return nil, usererr.New("invalid_ip", truncate(v, 50))
			}
			norm = p.Masked().String()
		} else {
			a, err := netip.ParseAddr(v)
			if err != nil || a.Zone() != "" {
				return nil, usererr.New("invalid_ip", truncate(v, 50))
			}
			norm = a.Unmap().String()
		}
		if !seen[norm] {
			seen[norm] = true
			out = append(out, norm)
		}
	}
	if len(out) > maxAPIKeyAddresses {
		return nil, usererr.New("too_many_key_addresses", itoa(maxAPIKeyAddresses))
	}
	return out, nil
}

// validKeyName allows letters, digits, spaces, dots, hyphens and
// underscores: the name shows in the audit log and the service log.
func validKeyName(name string) bool {
	if name == "" || utf8.RuneCountInString(name) > maxAPIKeyName {
		return false
	}
	for _, c := range name {
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) && !strings.ContainsRune(" .-_", c) {
			return false
		}
	}
	return true
}

type apiKeyResponse struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Access     string     `json:"access"`
	Addresses  []string   `json:"addresses"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
	Expired    bool       `json:"expired"`
	LastUsedAt *time.Time `json:"last_used_at"`
	LastUsedIP string     `json:"last_used_ip"`
}

func timeOrNil(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func (s *Server) apiKeyJSON(k *store.APIKey) apiKeyResponse {
	return apiKeyResponse{
		ID: k.ID, Name: k.Name, Prefix: k.Prefix, Access: k.Access, Addresses: k.Addresses,
		CreatedAt: k.CreatedAt, ExpiresAt: timeOrNil(k.ExpiresAt),
		Expired:    !k.ExpiresAt.IsZero() && !s.now().Before(k.ExpiresAt),
		LastUsedAt: timeOrNil(k.LastUsedAt), LastUsedIP: k.LastUsedIP,
	}
}

func (s *Server) handleListAPIKeys(w http.ResponseWriter, r *http.Request, a *authed) {
	keys, err := s.store.APIKeys(r.Context())
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	out := make([]apiKeyResponse, 0, len(keys))
	for _, k := range keys {
		out = append(out, s.apiKeyJSON(k))
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": out, "max": maxAPIKeys})
}

// handleCreateAPIKey answers the new key once; it cannot be shown again.
func (s *Server) handleCreateAPIKey(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Name        string   `json:"name"`
		Access      string   `json:"access"`
		Addresses   []string `json:"addresses"`
		ExpiresDays int      `json:"expires_days"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if (req.Access != store.APIKeyRead && req.Access != store.APIKeyFull) || !apiKeyExpiryDays[req.ExpiresDays] {
		writeError(w, http.StatusBadRequest, "bad_request")
		return
	}
	if !validKeyName(name) {
		s.opError(w, r, usererr.New("invalid_key_name"))
		return
	}
	addrs, err := parseKeyAddresses(req.Addresses)
	if err != nil {
		s.opError(w, r, err)
		return
	}
	token, hash := newAPIKey()
	now := s.now()
	k := &store.APIKey{UserID: a.user.ID, Name: name, Prefix: token[:apiKeyShown], Hash: hash,
		Access: req.Access, Addresses: addrs, CreatedAt: now}
	if req.ExpiresDays > 0 {
		k.ExpiresAt = now.AddDate(0, 0, req.ExpiresDays)
	}
	switch err := s.store.CreateAPIKey(r.Context(), k, maxAPIKeys); {
	case errors.Is(err, store.ErrConflict):
		s.opError(w, r, usererr.New("api_key_exists", name))
		return
	case errors.Is(err, store.ErrTooManyKeys):
		s.opError(w, r, usererr.New("too_many_api_keys", itoa(maxAPIKeys)))
		return
	case err != nil:
		s.internalError(w, r, err)
		return
	}
	detail := name + " (" + k.Access
	if len(addrs) > 0 {
		detail += ", " + strings.Join(addrs, " ")
	}
	if req.ExpiresDays > 0 {
		detail += ", " + itoa(req.ExpiresDays) + " days"
	}
	s.audit(r.Context(), a.user.Username, "api_key_created", clientIP(r), truncate(detail+")", 500))
	writeJSON(w, http.StatusCreated, map[string]any{"key": s.apiKeyJSON(k), "token": token})
}

// handleDeleteAPIKey revokes a key: the next request with it gets 401.
func (s *Server) handleDeleteAPIKey(w http.ResponseWriter, r *http.Request, a *authed) {
	id, ok := pathID(r)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found")
		return
	}
	ctx := r.Context()
	k, err := s.store.APIKeyByID(ctx, id)
	if err == nil {
		err = s.store.DeleteAPIKey(ctx, id)
	}
	if err != nil {
		s.opError(w, r, err)
		return
	}
	s.audit(ctx, a.user.Username, "api_key_revoked", clientIP(r), k.Name)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
