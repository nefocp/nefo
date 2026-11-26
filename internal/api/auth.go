package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/nefocp/nefo/internal/auth"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/validate"
	"github.com/nefocp/nefo/internal/version"
)

// authed is who made a request: a browser session (tokenHash) or an API key
// (key, see apikeys.go). For a key, user.Username also names the key, so
// the audit entries of the request show which key it was.
type authed struct {
	user      *store.User
	tokenHash string
	key       *store.APIKey
}

type authedHandler func(w http.ResponseWriter, r *http.Request, a *authed)

type userResponse struct {
	Username    string `json:"username"`
	TOTPEnabled bool   `json:"totp_enabled"`
}

func toUserResponse(u *store.User) userResponse {
	return userResponse{Username: u.Username, TOTPEnabled: u.TOTPEnabled}
}

func itoa(n int) string { return strconv.Itoa(n) }

// requireAuth only calls h for a request with a valid session cookie, or
// with a valid API key that allows the request (apikeys.go).
func (s *Server) requireAuth(h authedHandler) http.Handler { return s.authenticate(h, false) }

// sessionOnly is requireAuth for the routes an API key must never reach,
// even a full one: they answer a key with 403 api_key_forbidden.
func (s *Server) sessionOnly(h authedHandler) http.Handler { return s.authenticate(h, true) }

func (s *Server) authenticate(h authedHandler, sessionOnly bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A request with a Bearer token is never a cookie session.
		if token, ok := bearerToken(r); ok {
			s.serveWithKey(w, r, token, h, sessionOnly || sessionOnlyPath(r.URL.Path))
			return
		}
		a, err := s.session(r)
		if errors.Is(err, errNoSession) || errors.Is(err, store.ErrNotFound) {
			http.SetCookie(w, s.cookie("", -1))
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		h(w, r, a)
	})
}

func (s *Server) session(r *http.Request) (*authed, error) {
	c, err := r.Cookie(auth.CookieName)
	if err != nil || c.Value == "" {
		return nil, errNoSession
	}
	hash := auth.HashToken(c.Value)
	sess, err := s.store.ValidSession(r.Context(), hash, s.now())
	if err != nil {
		return nil, err
	}
	u, err := s.store.UserByID(r.Context(), sess.UserID)
	if err != nil {
		return nil, err
	}
	return &authed{user: u, tokenHash: hash}, nil
}

func (s *Server) cookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     auth.CookieName,
		Value:    value,
		Path:     "/" + s.cfg.Entrance + "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   s.cfg.TLS.Enabled,
		SameSite: http.SameSiteStrictMode,
	}
}

func (s *Server) startSession(w http.ResponseWriter, r *http.Request, u *store.User) error {
	token, hash := auth.NewSessionToken()
	now := s.now()
	err := s.store.CreateSession(r.Context(), &store.Session{
		TokenHash: hash,
		UserID:    u.ID,
		CreatedAt: now,
		ExpiresAt: now.Add(auth.SessionTTL),
		IP:        clientIP(r),
		UserAgent: truncate(r.UserAgent(), 256),
	})
	if err != nil {
		return err
	}
	http.SetCookie(w, s.cookie(token, int(auth.SessionTTL.Seconds())))
	return nil
}

// checkTOTP verifies a code and refuses to accept the same code twice.
func (s *Server) checkTOTP(u *store.User, code string) bool {
	counter, ok := auth.VerifyTOTP(u.TOTPSecret, code, s.now())
	if !ok {
		return false
	}
	s.totpMu.Lock()
	defer s.totpMu.Unlock()
	if last, seen := s.totpUsed[u.ID]; seen && counter <= last {
		return false
	}
	s.totpUsed[u.ID] = counter
	return true
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	// A request with an API key never starts a session (and skipped the
	// X-Requested-With check).
	if _, ok := bearerToken(r); ok {
		writeError(w, http.StatusForbidden, "api_key_forbidden")
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if !decodeJSON(w, r, &req) || s.rateLimited(w, r) {
		return
	}
	ctx, ip := r.Context(), clientIP(r)

	u, err := s.store.UserByName(ctx, req.Username)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.internalError(w, r, err)
		return
	}
	ok := false
	if u != nil {
		ok = auth.CheckPassword(u.PasswordHash, req.Password)
	} else {
		auth.BurnPasswordCheck(req.Password)
	}
	if !ok {
		s.recordFailure(r)
		s.audit(ctx, req.Username, "login_failed", ip, "wrong username or password")
		writeError(w, http.StatusUnauthorized, "invalid_credentials")
		return
	}

	if u.TOTPEnabled {
		if strings.TrimSpace(req.Code) == "" {
			writeError(w, http.StatusUnauthorized, "totp_required")
			return
		}
		if !s.checkTOTP(u, req.Code) {
			s.recordFailure(r)
			s.audit(ctx, u.Username, "login_failed", ip, "wrong 2FA code")
			writeError(w, http.StatusUnauthorized, "invalid_totp")
			return
		}
	}

	s.limiter.Reset(limiterKey(ip))
	if err := s.startSession(w, r, u); err != nil {
		s.internalError(w, r, err)
		return
	}
	s.audit(ctx, u.Username, "login", ip, "")
	writeJSON(w, http.StatusOK, toUserResponse(u))
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request, a *authed) {
	if err := s.store.DeleteSession(r.Context(), a.tokenHash); err != nil {
		s.internalError(w, r, err)
		return
	}
	http.SetCookie(w, s.cookie("", -1))
	s.audit(r.Context(), a.user.Username, "logout", clientIP(r), "")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request, a *authed) {
	writeJSON(w, http.StatusOK, toUserResponse(a.user))
}

// Errors on authenticated forms use 403, not 401: the UI treats 401 as
// "signed out" and goes back to the login page.

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Current string `json:"current_password"`
		New     string `json:"new_password"`
	}
	if !decodeJSON(w, r, &req) || s.rateLimited(w, r) {
		return
	}
	if !auth.CheckPassword(a.user.PasswordHash, req.Current) {
		s.recordFailure(r)
		writeError(w, http.StatusForbidden, "invalid_password")
		return
	}
	if validate.Password(req.New) != nil {
		writeError(w, http.StatusBadRequest, "weak_password")
		return
	}
	hash, err := auth.HashPassword(req.New)
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	// Signs out every session, including this one.
	if err := s.store.SetPassword(r.Context(), a.user.ID, hash); err != nil {
		s.internalError(w, r, err)
		return
	}
	http.SetCookie(w, s.cookie("", -1))
	s.audit(r.Context(), a.user.Username, "password_changed", clientIP(r), "")
	writeJSON(w, http.StatusOK, map[string]bool{"relogin": true})
}

func (s *Server) handleTOTPSetup(w http.ResponseWriter, r *http.Request, a *authed) {
	if a.user.TOTPEnabled {
		writeError(w, http.StatusConflict, "totp_already_enabled")
		return
	}
	secret := auth.NewTOTPSecret()
	if err := s.store.SetTOTP(r.Context(), a.user.ID, secret, false); err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"secret": secret,
		"uri":    auth.TOTPURI(version.AppName, a.user.Username, secret),
	})
}

func (s *Server) handleTOTPEnable(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Code string `json:"code"`
	}
	if !decodeJSON(w, r, &req) || s.rateLimited(w, r) {
		return
	}
	if a.user.TOTPEnabled {
		writeError(w, http.StatusConflict, "totp_already_enabled")
		return
	}
	if a.user.TOTPSecret == "" {
		writeError(w, http.StatusBadRequest, "totp_not_setup")
		return
	}
	if !s.checkTOTP(a.user, req.Code) {
		s.recordFailure(r)
		writeError(w, http.StatusForbidden, "invalid_totp")
		return
	}
	if err := s.store.SetTOTP(r.Context(), a.user.ID, a.user.TOTPSecret, true); err != nil {
		s.internalError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "totp_enabled", clientIP(r), "")
	a.user.TOTPEnabled = true
	writeJSON(w, http.StatusOK, toUserResponse(a.user))
}

func (s *Server) handleTOTPDisable(w http.ResponseWriter, r *http.Request, a *authed) {
	var req struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if !decodeJSON(w, r, &req) || s.rateLimited(w, r) {
		return
	}
	if !a.user.TOTPEnabled {
		writeJSON(w, http.StatusOK, toUserResponse(a.user))
		return
	}
	if !auth.CheckPassword(a.user.PasswordHash, req.Password) {
		s.recordFailure(r)
		writeError(w, http.StatusForbidden, "invalid_password")
		return
	}
	if !s.checkTOTP(a.user, req.Code) {
		s.recordFailure(r)
		writeError(w, http.StatusForbidden, "invalid_totp")
		return
	}
	if err := s.store.SetTOTP(r.Context(), a.user.ID, "", false); err != nil {
		s.internalError(w, r, err)
		return
	}
	s.audit(r.Context(), a.user.Username, "totp_disabled", clientIP(r), "")
	a.user.TOTPEnabled = false
	writeJSON(w, http.StatusOK, toUserResponse(a.user))
}
