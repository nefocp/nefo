package backup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/version"
)

// Google Drive storage.
//
// The panel signs in with the OAuth 2.0 device flow (RFC 8628): it shows a
// code, the owner enters it at google.com/device, and Google gives the
// panel a refresh token. The scope is drive.file: the panel sees only the
// files and folders it created itself, nothing else in the account.
// Backups go into a folder of their own ("Nefo backups/<host name>" by
// default), with the same sub-folders as on this server
// (site/example.com/…). Uploads are resumable, in chunks: an interrupted
// upload goes on from what Google received.
//
// Only net/http and JSON are used, no Google library.

const driveSettingKey = "backup_gdrive"

const (
	driveScope      = "https://www.googleapis.com/auth/drive.file"
	driveFolderMime = "application/vnd.google-apps.folder"
	driveFileFields = "id,name,mimeType,size,modifiedTime,sha256Checksum,trashed"
)

// The panel's own OAuth client, of type "TVs and Limited Input devices",
// once the project has one: owners then need not create a client of their
// own. Empty: every owner enters theirs.
const (
	builtinDriveClientID     = ""
	builtinDriveClientSecret = ""
)

type driveEndpoints struct {
	DeviceCode, Token, Revoke, API, Upload string
}

var googleEndpoints = driveEndpoints{
	DeviceCode: "https://oauth2.googleapis.com/device/code",
	Token:      "https://oauth2.googleapis.com/token",
	Revoke:     "https://oauth2.googleapis.com/revoke",
	API:        "https://www.googleapis.com/drive/v3",
	Upload:     "https://www.googleapis.com/upload/drive/v3",
}

// How the panel reaches Google. Only the tests of this package change these
// (to a fake server, and to not really wait): no setting, flag or
// environment variable points the panel elsewhere.
var (
	driveEP    = googleEndpoints
	driveHTTP  = newDriveHTTP()
	driveSleep = func(ctx context.Context, d time.Duration) error {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			return nil
		}
	}
	driveNow = time.Now
	// drivePollUnit is the unit of the device flow's interval and expiry,
	// which Google gives in seconds.
	drivePollUnit = time.Second
	// driveChunk is the size of each upload request: a multiple of 256 KiB,
	// as Google requires. It is read from the backup file as it is sent.
	driveChunk int64 = 8 << 20
	// driveRetries is how many failed requests in a row an operation
	// survives (with growing pauses, about three minutes in all).
	driveRetries = 10
)

func newDriveHTTP() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.ResponseHeaderTimeout = 2 * time.Minute
	return &http.Client{Transport: t, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if req.Method != http.MethodGet || len(via) >= 5 {
			return http.ErrUseLastResponse // an upload's "308 Resume Incomplete" is an answer, not a redirect
		}
		return nil
	}}
}

// driveRetryWait is the pause before retry number n (from 1): 1, 2, 4… up
// to 32 seconds, with some randomness.
func driveRetryWait(n int) time.Duration {
	d := time.Second << min(n-1, 5)
	return d + time.Duration(rand.Int64N(int64(d/4)+1))
}

// DriveConfig describes Google Drive storage.
type DriveConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Account      string `json:"account,omitempty"` // the Google account's e-mail address
	// Folder holds the backups: a path in My Drive, such as
	// "Nefo backups/server1". FolderID is its last folder.
	Folder   string `json:"folder"`
	FolderID string `json:"folder_id,omitempty"`
	Encryption
}

// DriveView is what the UI sees of the Google Drive settings: no secrets.
type DriveView struct {
	ClientID      string `json:"client_id"`
	BuiltinClient bool   `json:"builtin_client"` // the panel's own OAuth client is used
	Account       string `json:"account"`
	Folder        string `json:"folder"`
	FolderURL     string `json:"folder_url,omitempty"`
	Encrypt       bool   `json:"encrypt"`
}

func (c *DriveConfig) view() *DriveView {
	v := &DriveView{ClientID: c.ClientID, Account: c.Account, Folder: c.Folder, Encrypt: c.Encrypt,
		BuiltinClient: builtinDriveClientID != "" && c.ClientID == builtinDriveClientID}
	if c.FolderID != "" {
		v.FolderURL = "https://drive.google.com/drive/folders/" + url.PathEscape(c.FolderID)
	}
	if v.BuiltinClient {
		v.ClientID = ""
	}
	return v
}

// DefaultDriveFolder is the folder suggested for this server's backups.
func DefaultDriveFolder() string {
	h, _ := os.Hostname()
	h = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r < ' ' || r == 0x7f {
			return '-'
		}
		return r
	}, strings.TrimSpace(h))
	if h == "" {
		h = "server"
	}
	return version.AppName + " backups/" + h
}

// BuiltinDriveClient tells whether the panel brings its own OAuth client.
func BuiltinDriveClient() bool { return builtinDriveClientID != "" }

// cleanDriveFolder normalises a folder path: names separated by slashes.
func cleanDriveFolder(f string) (string, error) {
	if strings.TrimSpace(f) == "" {
		return DefaultDriveFolder(), nil
	}
	var out []string
	for _, p := range strings.Split(f, "/") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if p == "." || p == ".." || len(p) > 200 || strings.ContainsFunc(p, func(r rune) bool { return r < ' ' || r == 0x7f || r == '\\' }) {
			return "", usererr.New("gdrive_folder")
		}
		out = append(out, p)
	}
	if len(out) == 0 || len(out) > 8 {
		return "", usererr.New("gdrive_folder")
	}
	return strings.Join(out, "/"), nil
}

func validDriveClient(id, secret string) error {
	if id == "" || len(id) > 200 || !strings.HasSuffix(id, ".apps.googleusercontent.com") ||
		strings.ContainsFunc(id, func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '.' || r == '_')
		}) {
		return usererr.New("gdrive_client_id")
	}
	if secret == "" || len(secret) > 200 || strings.ContainsFunc(secret, func(r rune) bool { return r <= ' ' || r == 0x7f }) {
		return usererr.New("gdrive_client_secret")
	}
	return nil
}

// ---- errors ----

// driveError is a problem with Google Drive. Its message reads well in a
// job log; code lets "Connect" and "Save" answer with a translated message.
type driveError struct {
	code, detail, msg string
}

func (e *driveError) Error() string { return e.msg }

var (
	errDriveRevoked = &driveError{code: "gdrive_revoked", msg: "Google Drive refused the panel's access: it was revoked, " +
		"or it expired (Google lets it expire after 7 days while the OAuth consent screen's publishing status is Testing). " +
		"Connect Google Drive again under Remote storage on the Backups page"}
	errDriveFull = &driveError{code: "gdrive_quota", msg: "Google Drive is full: the account's storage is used up. " +
		"Free up space in the Google account or get more storage, then upload again"}
	errDriveScope = &driveError{code: "gdrive_scope", msg: "Google did not give the panel access to Drive files: " +
		"connect again and allow it to see, edit, create and delete its own Google Drive files"}
	errDeviceDenied  = &driveError{code: "gdrive_denied", msg: "the Google account did not allow access"}
	errDeviceExpired = &driveError{code: "gdrive_expired", msg: "the code expired before it was entered"}
	errSessionGone   = errors.New("the upload session expired")
)

func errDriveClient(detail string) error {
	return &driveError{code: "gdrive_client", detail: detail,
		msg: "Google refused the OAuth client (" + detail + "): check its client ID and secret, and that its type is TVs and Limited Input devices"}
}

func errDriveFolderGone(folder string) error {
	return &driveError{code: "gdrive_folder_missing", detail: folder,
		msg: fmt.Sprintf("the Google Drive folder %q was deleted. Save the Google Drive settings under Remote storage on the Backups page to create it again", folder)}
}

func errDriveFolderTrashed(folder string) error {
	return &driveError{code: "gdrive_folder_trashed", detail: folder,
		msg: fmt.Sprintf("the Google Drive folder %q is in the Drive trash. Restore it from the trash, "+
			"or save the Google Drive settings under Remote storage on the Backups page to create a new one", folder)}
}

// driveUserError turns a failed connection or test into an error the UI
// translates.
func driveUserError(err error) error {
	var de *driveError
	var ue *usererr.Error
	switch {
	case errors.As(err, &de):
		return usererr.New(de.code, de.detail)
	case errors.As(err, &ue):
		return err
	}
	return usererr.New("storage_test_failed", err.Error())
}

// googleError is how Google's APIs describe a failure.
type googleError struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
		Errors  []struct {
			Reason  string `json:"reason"`
			Message string `json:"message"`
		} `json:"errors"`
	} `json:"error"`
}

// driveHTTPError is a Drive answer with an error status.
type driveHTTPError struct {
	status int
	msg    string
}

func (e *driveHTTPError) Error() string { return e.msg }

func isHTTPStatus(err error, status int) bool {
	var he *driveHTTPError
	return errors.As(err, &he) && he.status == status
}

// apiError reads a failed Drive answer. The body is closed.
func apiError(resp *http.Response) error {
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
	var ge googleError
	json.Unmarshal(b, &ge)
	reasons := map[string]bool{}
	for _, e := range ge.Error.Errors {
		reasons[e.Reason] = true
	}
	switch {
	case reasons["storageQuotaExceeded"] || reasons["quotaExceeded"] && strings.Contains(strings.ToLower(ge.Error.Message), "storage"):
		return errDriveFull
	case reasons["insufficientPermissions"] || ge.Error.Status == "PERMISSION_DENIED" && strings.Contains(ge.Error.Message, "scope"):
		return errDriveScope
	}
	msg := ge.Error.Message
	if msg == "" {
		msg = strings.TrimSpace(string(b))
		if len(msg) > 300 {
			msg = msg[:300]
		}
	}
	e := &driveHTTPError{status: resp.StatusCode, msg: fmt.Sprintf("Google Drive answered HTTP %d", resp.StatusCode)}
	if msg != "" {
		e.msg += ": " + msg
	}
	return e
}

// retryable tells whether a failed answer is worth asking again: Google is
// busy or limits the rate of requests.
func retryable(resp *http.Response) bool {
	switch resp.StatusCode {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout, 408:
		return true
	case http.StatusForbidden:
		// Peek at the reason without consuming the body for apiError.
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
		resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(b))
		var ge googleError
		json.Unmarshal(b, &ge)
		for _, e := range ge.Error.Errors {
			if e.Reason == "rateLimitExceeded" || e.Reason == "userRateLimitExceeded" {
				return true
			}
		}
	}
	return false
}

// ---- OAuth ----

type oauthError struct {
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

type oauthToken struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

// postForm posts to an OAuth endpoint. It returns the body of a 200
// answer, or the OAuth error of another one.
func postForm(ctx context.Context, endpoint string, form url.Values) ([]byte, *oauthError, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", version.AppName+"/"+version.Version)
	resp, err := driveHTTP.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode == http.StatusOK {
		return b, nil, nil
	}
	var oe oauthError
	if json.Unmarshal(b, &oe) != nil || oe.Error == "" {
		return nil, nil, fmt.Errorf("Google answered HTTP %d", resp.StatusCode)
	}
	return nil, &oe, nil
}

// clientError explains the OAuth errors that mean the client is wrong.
func clientError(oe *oauthError) error {
	switch oe.Error {
	case "invalid_client", "unauthorized_client", "deleted_client", "disabled_client", "org_internal":
		d := oe.Error
		if oe.Description != "" {
			d += ": " + oe.Description
		}
		return errDriveClient(d)
	}
	return nil
}

// deviceCode is Google's answer to the start of the device flow.
type deviceCode struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURL string `json:"verification_url"` // Google's name
	VerificationURI string `json:"verification_uri"` // RFC 8628's name
	ExpiresIn       int64  `json:"expires_in"`
	Interval        int64  `json:"interval"`
}

func requestDeviceCode(ctx context.Context, clientID string) (*deviceCode, error) {
	b, oe, err := postForm(ctx, driveEP.DeviceCode, url.Values{"client_id": {clientID}, "scope": {driveScope}})
	if err != nil {
		return nil, &driveError{code: "gdrive_unreachable", detail: err.Error(), msg: "could not reach Google: " + err.Error()}
	}
	if oe != nil {
		if ce := clientError(oe); ce != nil {
			return nil, ce
		}
		return nil, fmt.Errorf("Google refused to start the sign-in: %s %s", oe.Error, oe.Description)
	}
	var dc deviceCode
	if err := json.Unmarshal(b, &dc); err != nil || dc.DeviceCode == "" || dc.UserCode == "" {
		return nil, errors.New("Google's answer to the sign-in request is incomplete")
	}
	if dc.VerificationURL == "" {
		dc.VerificationURL = dc.VerificationURI
	}
	if u, err := url.Parse(dc.VerificationURL); err != nil || u.Scheme != "https" {
		dc.VerificationURL = "https://www.google.com/device"
	}
	if dc.ExpiresIn <= 0 {
		dc.ExpiresIn = 1800
	}
	if dc.Interval <= 0 {
		dc.Interval = 5
	}
	return &dc, nil
}

// pollDeviceToken asks for the token until the owner has entered the code
// (or refused, or the code expired), as RFC 8628 describes.
func pollDeviceToken(ctx context.Context, clientID, secret string, dc *deviceCode) (*oauthToken, error) {
	interval := time.Duration(dc.Interval) * drivePollUnit
	deadline := driveNow().Add(time.Duration(dc.ExpiresIn) * drivePollUnit)
	form := url.Values{"client_id": {clientID}, "client_secret": {secret}, "device_code": {dc.DeviceCode},
		"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}}
	netFails := 0
	for {
		if err := driveSleep(ctx, interval); err != nil {
			return nil, err
		}
		if driveNow().After(deadline) {
			return nil, errDeviceExpired
		}
		b, oe, err := postForm(ctx, driveEP.Token, form)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if netFails++; netFails > driveRetries {
				return nil, &driveError{code: "gdrive_unreachable", detail: err.Error(), msg: "could not reach Google: " + err.Error()}
			}
			continue
		}
		netFails = 0
		if oe == nil {
			var tok oauthToken
			if err := json.Unmarshal(b, &tok); err != nil || tok.AccessToken == "" {
				return nil, errors.New("Google's token answer is incomplete")
			}
			if tok.RefreshToken == "" {
				return nil, errors.New("Google gave no refresh token: remove the panel's access in your Google account's security settings, then connect again")
			}
			if !strings.Contains(" "+tok.Scope+" ", " "+driveScope+" ") {
				return nil, errDriveScope
			}
			return &tok, nil
		}
		switch oe.Error {
		case "authorization_pending":
		case "slow_down":
			interval += 5 * drivePollUnit
		case "access_denied":
			return nil, errDeviceDenied
		case "expired_token":
			return nil, errDeviceExpired
		default:
			if ce := clientError(oe); ce != nil {
				return nil, ce
			}
			return nil, fmt.Errorf("Google refused the sign-in: %s %s", oe.Error, oe.Description)
		}
	}
}

// Access tokens last an hour: they are kept in memory, by refresh token,
// so that each operation does not ask for a new one.
var driveTokens = struct {
	sync.Mutex
	m map[string]cachedToken
}{m: map[string]cachedToken{}}

type cachedToken struct {
	access  string
	expires time.Time
}

func tokenCacheKey(cfg *DriveConfig) string {
	s := sha256.Sum256([]byte(cfg.ClientID + "\x00" + cfg.RefreshToken))
	return hex.EncodeToString(s[:])
}

func cacheToken(cfg *DriveConfig, access string, expiresIn int64) {
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	driveTokens.Lock()
	defer driveTokens.Unlock()
	driveTokens.m[tokenCacheKey(cfg)] = cachedToken{access, driveNow().Add(time.Duration(expiresIn)*time.Second - time.Minute)}
}

func forgetToken(cfg *DriveConfig) {
	driveTokens.Lock()
	defer driveTokens.Unlock()
	delete(driveTokens.m, tokenCacheKey(cfg))
}

// revokeDrive tells Google the panel no longer uses the refresh token.
func revokeDrive(ctx context.Context, cfg *DriveConfig) error {
	forgetToken(cfg)
	_, oe, err := postForm(ctx, driveEP.Revoke, url.Values{"token": {cfg.RefreshToken}})
	if err == nil && oe != nil && oe.Error != "invalid_token" {
		err = fmt.Errorf("%s %s", oe.Error, oe.Description)
	}
	return err
}

// ---- the client ----

type driveClient struct {
	cfg DriveConfig
	// dirs are folder IDs by path under the backup folder ("" is the
	// backup folder itself).
	dirs map[string]string
}

func newDriveClient(cfg DriveConfig) *driveClient {
	return &driveClient{cfg: cfg, dirs: map[string]string{}}
}

// Describe names a file for job logs.
func (c *driveClient) Describe(key string) string {
	who := ""
	if c.cfg.Account != "" {
		who = " (" + c.cfg.Account + ")"
	}
	return "Google Drive" + who + " " + c.cfg.Folder + "/" + key
}

// Close does nothing: requests share no connection that needs closing.
func (c *driveClient) Close() error { return nil }

func (c *driveClient) token(ctx context.Context, renew bool) (string, error) {
	if !renew {
		driveTokens.Lock()
		t, ok := driveTokens.m[tokenCacheKey(&c.cfg)]
		driveTokens.Unlock()
		if ok && driveNow().Before(t.expires) {
			return t.access, nil
		}
	}
	form := url.Values{"client_id": {c.cfg.ClientID}, "client_secret": {c.cfg.ClientSecret},
		"refresh_token": {c.cfg.RefreshToken}, "grant_type": {"refresh_token"}}
	var lastErr error
	for n := 1; n <= 4; n++ {
		b, oe, err := postForm(ctx, driveEP.Token, form)
		if err == nil && oe == nil {
			var tok oauthToken
			if err := json.Unmarshal(b, &tok); err != nil || tok.AccessToken == "" {
				return "", errors.New("Google's token answer is incomplete")
			}
			cacheToken(&c.cfg, tok.AccessToken, tok.ExpiresIn)
			return tok.AccessToken, nil
		}
		if oe != nil {
			if oe.Error == "invalid_grant" {
				forgetToken(&c.cfg)
				return "", errDriveRevoked
			}
			if ce := clientError(oe); ce != nil {
				return "", ce
			}
			return "", fmt.Errorf("Google refused to renew the access: %s %s", oe.Error, oe.Description)
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		lastErr = err
		if err := driveSleep(ctx, driveRetryWait(n)); err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("could not reach Google: %w", lastErr)
}

// send makes a request with the access token; a 401 answer renews the
// token once. newBody gives a fresh body for each attempt. With retry,
// network errors and busy answers are retried, and an answer other than
// 2xx or 308 becomes an error. Without retry, every answer is returned and
// only a failed request is an error. The caller closes the body of the
// answer it gets.
func (c *driveClient) send(ctx context.Context, method, u string, header http.Header, newBody func() (io.Reader, int64), retry bool) (*http.Response, error) {
	renewed := false
	for n := 1; ; n++ {
		tok, err := c.token(ctx, false)
		if err != nil {
			return nil, err
		}
		var body io.Reader
		var size int64
		if newBody != nil {
			body, size = newBody()
		}
		req, err := http.NewRequestWithContext(ctx, method, u, body)
		if err != nil {
			return nil, err
		}
		if newBody != nil {
			req.ContentLength = size
			if size == 0 {
				req.Body = http.NoBody
			}
		}
		for k, v := range header {
			req.Header[k] = v
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("User-Agent", version.AppName+"/"+version.Version)
		resp, err := driveHTTP.Do(req)
		switch {
		case err != nil:
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if !retry || n > driveRetries {
				return nil, fmt.Errorf("Google Drive: %w", err)
			}
		case resp.StatusCode == http.StatusUnauthorized:
			resp.Body.Close()
			if renewed {
				return nil, errDriveRevoked
			}
			renewed = true
			if _, err := c.token(ctx, true); err != nil {
				return nil, err
			}
			continue
		case !retry, resp.StatusCode/100 == 2, resp.StatusCode == http.StatusPermanentRedirect:
			return resp, nil
		case n <= driveRetries && retryable(resp):
			resp.Body.Close()
		default:
			return nil, apiError(resp)
		}
		if err := driveSleep(ctx, driveRetryWait(n)); err != nil {
			return nil, err
		}
	}
}

// call makes a JSON API request and decodes the answer into out.
func (c *driveClient) call(ctx context.Context, method, u string, in, out any) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	var newBody func() (io.Reader, int64)
	h := http.Header{}
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		newBody = func() (io.Reader, int64) { return bytes.NewReader(b), int64(len(b)) }
		h.Set("Content-Type", "application/json; charset=UTF-8")
	}
	resp, err := c.send(ctx, method, u, h, newBody, true)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if out == nil {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(out); err != nil {
		return fmt.Errorf("read Google Drive's answer: %w", err)
	}
	return nil
}

// flexInt reads Drive's 64-bit numbers, which come as JSON strings.
type flexInt int64

func (n *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*n = 0
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	*n = flexInt(v)
	return err
}

type driveFile struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	MimeType string    `json:"mimeType"`
	Size     flexInt   `json:"size"`
	Modified time.Time `json:"modifiedTime"`
	SHA256   string    `json:"sha256Checksum"`
	Trashed  bool      `json:"trashed"`
}

func (f *driveFile) folder() bool { return f.MimeType == driveFolderMime }

// quoteQuery quotes a value for a Drive search query.
func quoteQuery(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}

// list returns the files that match a search query, every page of them,
// oldest first.
func (c *driveClient) list(ctx context.Context, q string) ([]driveFile, error) {
	var out []driveFile
	token := ""
	for page := 0; page < 1000; page++ {
		v := url.Values{"q": {q}, "fields": {"nextPageToken,files(" + driveFileFields + ")"},
			"pageSize": {"1000"}, "orderBy": {"createdTime"}, "spaces": {"drive"}}
		if token != "" {
			v.Set("pageToken", token)
		}
		var res struct {
			NextPageToken string      `json:"nextPageToken"`
			Files         []driveFile `json:"files"`
		}
		if err := c.call(ctx, http.MethodGet, driveEP.API+"/files?"+v.Encode(), nil, &res); err != nil {
			return nil, err
		}
		out = append(out, res.Files...)
		if token = res.NextPageToken; token == "" {
			return out, nil
		}
	}
	return nil, errors.New("Google Drive lists too many files")
}

// children lists what a folder holds under a name ("" for everything):
// folders only, or files only.
func (c *driveClient) children(ctx context.Context, parent, name string, folders bool) ([]driveFile, error) {
	q := quoteQuery(parent) + " in parents and trashed = false"
	if name != "" {
		q += " and name = " + quoteQuery(name)
	}
	if folders {
		q += " and mimeType = " + quoteQuery(driveFolderMime)
	} else {
		q += " and mimeType != " + quoteQuery(driveFolderMime)
	}
	return c.list(ctx, q)
}

func (c *driveClient) mkdir(ctx context.Context, parent, name string) (string, error) {
	var f driveFile
	err := c.call(ctx, http.MethodPost, driveEP.API+"/files?fields=id", map[string]any{
		"name": name, "mimeType": driveFolderMime, "parents": []string{parent}}, &f)
	if err == nil && f.ID == "" {
		err = errors.New("Google Drive did not return the new folder's ID")
	}
	return f.ID, err
}

// walk finds the folders of p under parent, one name after the other, and
// creates the missing ones when create is set. It returns "" (and no
// error) for a missing folder when create is not set.
func (c *driveClient) walk(ctx context.Context, parent, p string, create bool) (string, error) {
	id := parent
	for _, name := range strings.Split(p, "/") {
		if name == "" {
			continue
		}
		found, err := c.children(ctx, id, name, true)
		if err != nil {
			return "", err
		}
		switch {
		case len(found) > 0:
			id = found[0].ID // the oldest, if two were made at once
		case create:
			if id, err = c.mkdir(ctx, id, name); err != nil {
				return "", fmt.Errorf("create the folder %q in Google Drive: %w", name, err)
			}
		default:
			return "", nil
		}
	}
	return id, nil
}

// findRoot finds (or creates) the backup folder from its path in My Drive.
func (c *driveClient) findRoot(ctx context.Context) (string, error) {
	id, err := c.walk(ctx, "root", c.cfg.Folder, true)
	if err == nil {
		c.cfg.FolderID = id
		c.dirs[""] = id
	}
	return id, err
}

// root checks that the saved backup folder is still there.
func (c *driveClient) root(ctx context.Context) (string, error) {
	if id, ok := c.dirs[""]; ok {
		return id, nil
	}
	if c.cfg.FolderID == "" {
		return "", errDriveFolderGone(c.cfg.Folder)
	}
	var f driveFile
	err := c.call(ctx, http.MethodGet, driveEP.API+"/files/"+url.PathEscape(c.cfg.FolderID)+"?fields="+driveFileFields, nil, &f)
	if err != nil {
		if isHTTPStatus(err, http.StatusNotFound) {
			return "", errDriveFolderGone(c.cfg.Folder)
		}
		return "", err
	}
	if f.Trashed {
		return "", errDriveFolderTrashed(c.cfg.Folder)
	}
	if !f.folder() {
		return "", errDriveFolderGone(c.cfg.Folder)
	}
	c.dirs[""] = f.ID
	return f.ID, nil
}

// dir finds the folder of a key's directory ("site/example.com"), "" when
// it does not exist and create is not set.
func (c *driveClient) dir(ctx context.Context, d string, create bool) (string, error) {
	d = strings.Trim(d, "/")
	if id, ok := c.dirs[d]; ok {
		return id, nil
	}
	root, err := c.root(ctx)
	if err != nil || d == "" {
		return root, err
	}
	id, err := c.walk(ctx, root, d, create)
	if err == nil && id != "" {
		c.dirs[d] = id
	}
	return id, err
}

// files finds the files stored under key, oldest first.
func (c *driveClient) files(ctx context.Context, key string) ([]driveFile, error) {
	d, name := path.Split(key)
	parent, err := c.dir(ctx, d, false)
	if err != nil || parent == "" {
		return nil, err
	}
	return c.children(ctx, parent, name, false)
}

type driveQuota struct {
	email        string
	limit, usage int64 // limit 0: unlimited
}

func (c *driveClient) about(ctx context.Context) (*driveQuota, error) {
	var a struct {
		User struct {
			Email string `json:"emailAddress"`
		} `json:"user"`
		Quota struct {
			Limit flexInt `json:"limit"`
			Usage flexInt `json:"usage"`
		} `json:"storageQuota"`
	}
	if err := c.call(ctx, http.MethodGet, driveEP.API+"/about?fields=user(emailAddress),storageQuota(limit,usage)", nil, &a); err != nil {
		return nil, err
	}
	return &driveQuota{a.User.Email, int64(a.Quota.Limit), int64(a.Quota.Usage)}, nil
}

// Put uploads a file in chunks; r must also be an io.ReaderAt, so that an
// interrupted upload can go on from where Google stopped receiving. A file
// stored under the same key before is replaced once the upload is done.
func (c *driveClient) Put(ctx context.Context, key string, r io.Reader, size int64, sha256hex string) error {
	src, ok := r.(io.ReaderAt)
	if !ok {
		return errors.New("internal error: a Google Drive upload needs a file it can read at any place")
	}
	d, name := path.Split(key)
	parent, err := c.dir(ctx, d, true)
	if err != nil {
		return err
	}
	old, err := c.children(ctx, parent, name, false)
	if err != nil {
		return err
	}
	// Fail at once, rather than after uploading most of a large file.
	if q, err := c.about(ctx); err == nil && q.limit > 0 && q.limit-q.usage < size {
		return errDriveFull
	}
	f, err := c.upload(ctx, parent, name, src, size)
	if err != nil {
		return err
	}
	if int64(f.Size) != size {
		c.deleteID(ctx, f.ID)
		return fmt.Errorf("Google Drive stored %d bytes instead of %d", f.Size, size)
	}
	if sha256hex != "" && f.SHA256 != "" && !strings.EqualFold(f.SHA256, sha256hex) {
		c.deleteID(ctx, f.ID)
		return errors.New("the copy in Google Drive has another checksum than the backup: it was damaged during the upload")
	}
	for _, o := range old {
		if o.ID != f.ID {
			if err := c.deleteID(ctx, o.ID); err != nil {
				return fmt.Errorf("the backup was uploaded, but the older copy could not be removed: %w", err)
			}
		}
	}
	return nil
}

// upload sends a file with Google's resumable upload protocol. A session
// that expired is started again, at most twice.
func (c *driveClient) upload(ctx context.Context, parent, name string, src io.ReaderAt, size int64) (*driveFile, error) {
	for restart := 0; ; restart++ {
		session, err := c.startUpload(ctx, parent, name, size)
		if err != nil {
			return nil, err
		}
		f, err := c.sendChunks(ctx, session, src, size)
		if errors.Is(err, errSessionGone) && restart < 2 {
			continue
		}
		if errors.Is(err, errSessionGone) {
			return nil, errors.New("Google Drive dropped the upload three times: try again later")
		}
		return f, err
	}
}

func (c *driveClient) startUpload(ctx context.Context, parent, name string, size int64) (string, error) {
	meta, _ := json.Marshal(map[string]any{"name": name, "parents": []string{parent}})
	h := http.Header{}
	h.Set("Content-Type", "application/json; charset=UTF-8")
	h.Set("X-Upload-Content-Type", "application/octet-stream")
	h.Set("X-Upload-Content-Length", strconv.FormatInt(size, 10))
	cctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	resp, err := c.send(cctx, http.MethodPost, driveEP.Upload+"/files?uploadType=resumable&fields="+driveFileFields, h,
		func() (io.Reader, int64) { return bytes.NewReader(meta), int64(len(meta)) }, true)
	if err != nil {
		return "", err
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
	// The session's address gets the access token: it must be Google's.
	loc := resp.Header.Get("Location")
	u, err := url.Parse(loc)
	up, _ := url.Parse(driveEP.Upload)
	if loc == "" || err != nil || u.Scheme != "https" || up == nil || u.Host != up.Host {
		return "", errors.New("Google Drive did not start the upload (no valid session address)")
	}
	return loc, nil
}

// readRecorder remembers an error reading the backup file, which is not
// worth retrying, unlike a network error.
type readRecorder struct {
	r   io.Reader
	err error
}

func (rr *readRecorder) Read(p []byte) (int, error) {
	n, err := rr.r.Read(p)
	if err != nil && err != io.EOF {
		rr.err = err
	}
	return n, err
}

// sendChunks sends the file to an upload session from offset 0. After a
// failure it asks the session how much it received and goes on from there.
func (c *driveClient) sendChunks(ctx context.Context, session string, src io.ReaderAt, size int64) (*driveFile, error) {
	var off int64
	ask := size == 0 // ask the session where it is instead of sending
	fails := 0
	for {
		var rec *readRecorder
		h := http.Header{}
		var newBody func() (io.Reader, int64)
		if ask {
			h.Set("Content-Range", "bytes */"+strconv.FormatInt(size, 10))
			newBody = func() (io.Reader, int64) { return http.NoBody, 0 }
		} else {
			n := min(driveChunk, size-off)
			h.Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", off, off+n-1, size))
			rec = &readRecorder{r: io.NewSectionReader(src, off, n)}
			newBody = func() (io.Reader, int64) { return rec, n }
		}
		f, res, err := c.sendChunk(ctx, session, h, newBody, rec)
		if err != nil || f != nil {
			return f, err
		}
		if res.err == nil {
			// "308 Resume Incomplete": the session tells how much it has.
			if res.got > size {
				return nil, fmt.Errorf("Google Drive says it received %d bytes of %d", res.got, size)
			}
			if res.got > off || !ask {
				fails = 0 // progress
			}
			if res.got < size || size == 0 {
				off, ask = res.got, false
				continue
			}
			// Everything arrived, but the upload is not finished: ask again.
			res.err = errors.New("Google Drive has the whole file but did not finish the upload")
		}
		ask = true
		if fails++; fails > driveRetries {
			return nil, fmt.Errorf("the upload to Google Drive failed %d times in a row: %w", fails, res.err)
		}
		if err := driveSleep(ctx, driveRetryWait(fails)); err != nil {
			return nil, err
		}
	}
}

// chunkResult is the outcome of an upload request that did not end the
// upload: how much the session has (after a 308), or a failure worth
// retrying.
type chunkResult struct {
	got int64
	err error
}

// sendChunk makes one request of an upload. It returns the file when the
// upload is done, an error when it must stop, or else a chunkResult.
func (c *driveClient) sendChunk(ctx context.Context, session string, h http.Header, newBody func() (io.Reader, int64), rec *readRecorder) (*driveFile, chunkResult, error) {
	rctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	resp, err := c.send(rctx, http.MethodPut, session, h, newBody, false)
	if rec != nil && rec.err != nil {
		if resp != nil {
			resp.Body.Close()
		}
		return nil, chunkResult{}, rec.err // reading the backup failed: retrying will not help
	}
	if err != nil {
		var de *driveError
		if errors.As(err, &de) || ctx.Err() != nil {
			if ctx.Err() != nil {
				return nil, chunkResult{}, ctx.Err()
			}
			return nil, chunkResult{}, err
		}
		return nil, chunkResult{err: err}, nil // a network error: ask the session what arrived
	}
	switch {
	case resp.StatusCode == http.StatusPermanentRedirect:
		io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		got, perr := receivedRange(resp.Header.Get("Range"))
		if perr != nil {
			return nil, chunkResult{}, fmt.Errorf("Google Drive answered the upload with an unexpected range %q", resp.Header.Get("Range"))
		}
		return nil, chunkResult{got: got}, nil
	case resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated:
		var f driveFile
		derr := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&f)
		resp.Body.Close()
		if derr != nil || f.ID == "" {
			return nil, chunkResult{}, errors.New("Google Drive finished the upload without describing the file")
		}
		return &f, chunkResult{}, nil
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		resp.Body.Close()
		return nil, chunkResult{}, errSessionGone
	case retryable(resp):
		resp.Body.Close()
		return nil, chunkResult{err: fmt.Errorf("Google Drive answered HTTP %d", resp.StatusCode)}, nil
	}
	return nil, chunkResult{}, apiError(resp)
}

// receivedRange reads the Range header of "308 Resume Incomplete": how many
// bytes the session has ("bytes=0-1048575" means 1048576; none means 0).
func receivedRange(h string) (int64, error) {
	if h == "" {
		return 0, nil
	}
	s, ok := strings.CutPrefix(h, "bytes=0-")
	if !ok {
		return 0, errors.New("unexpected range")
	}
	last, err := strconv.ParseInt(s, 10, 64)
	if err != nil || last < 0 {
		return 0, errors.New("unexpected range")
	}
	return last + 1, nil
}

// countWriter counts what was written, and remembers a failure of w (a
// full disk, a wrong passphrase), which is not worth retrying.
type countWriter struct {
	w   io.Writer
	n   int64
	err error
}

func (cw *countWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	cw.n += int64(n)
	if err != nil {
		cw.err = err
	}
	return n, err
}

// Get downloads a file into w. A broken download goes on from where it
// stopped.
func (c *driveClient) Get(ctx context.Context, key string, w io.Writer) error {
	found, err := c.files(ctx, key)
	if err != nil {
		return err
	}
	if len(found) == 0 {
		return fmt.Errorf("%s is missing in Google Drive", c.Describe(key))
	}
	f := found[len(found)-1] // the newest
	cw := &countWriter{w: w}
	fails := 0
	for {
		h := http.Header{}
		if cw.n > 0 {
			h.Set("Range", fmt.Sprintf("bytes=%d-", cw.n))
		}
		resp, err := c.send(ctx, http.MethodGet, driveEP.API+"/files/"+url.PathEscape(f.ID)+"?alt=media", h, nil, true)
		if err != nil {
			return err
		}
		skip := int64(0)
		switch {
		case resp.StatusCode == http.StatusPartialContent:
			if !strings.HasPrefix(resp.Header.Get("Content-Range"), fmt.Sprintf("bytes %d-", cw.n)) {
				resp.Body.Close()
				return errors.New("Google Drive resumed the download at the wrong place")
			}
		case resp.StatusCode == http.StatusOK:
			skip = cw.n // the whole file again: skip what was written
		default:
			resp.Body.Close()
			return fmt.Errorf("Google Drive answered the download with HTTP %d", resp.StatusCode)
		}
		if skip > 0 {
			if _, err = io.CopyN(io.Discard, resp.Body, skip); err == nil {
				skip = 0
			}
		}
		if err == nil {
			_, err = io.Copy(cw, resp.Body)
		}
		resp.Body.Close()
		if cw.err != nil {
			return cw.err
		}
		if err == nil {
			if cw.n != int64(f.Size) {
				return fmt.Errorf("downloaded %d bytes from Google Drive instead of %d", cw.n, f.Size)
			}
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if fails++; fails > driveRetries {
			return fmt.Errorf("the download from Google Drive failed %d times in a row: %w", fails, err)
		}
		if err := driveSleep(ctx, driveRetryWait(fails)); err != nil {
			return err
		}
	}
}

func (c *driveClient) deleteID(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	resp, err := c.send(ctx, http.MethodDelete, driveEP.API+"/files/"+url.PathEscape(id), nil, nil, true)
	if err != nil {
		if isHTTPStatus(err, http.StatusNotFound) {
			return nil // already gone
		}
		return err
	}
	resp.Body.Close()
	return nil
}

// Delete removes the file (every copy, if there were two) stored under key.
func (c *driveClient) Delete(ctx context.Context, key string) error {
	found, err := c.files(ctx, key)
	if err != nil {
		return err
	}
	for _, f := range found {
		if err := c.deleteID(ctx, f.ID); err != nil {
			return fmt.Errorf("delete %s: %w", c.Describe(key), err)
		}
	}
	return nil
}

// List returns the files (not folders) under prefix, by name.
func (c *driveClient) List(ctx context.Context, prefix string) ([]RemoteFile, error) {
	parent, err := c.dir(ctx, prefix, false)
	if err != nil {
		return nil, err
	}
	out := []RemoteFile{}
	if parent == "" {
		return out, nil
	}
	found, err := c.children(ctx, parent, "", false)
	if err != nil {
		return nil, err
	}
	for _, f := range found {
		out = append(out, RemoteFile{Name: f.Name, Size: int64(f.Size), ModTime: f.Modified})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// ListTree lists the files under prefix, at most two folders deep (the
// layout is kind/target/file). Names are relative to the backup folder.
func (c *driveClient) ListTree(ctx context.Context, prefix string) ([]RemoteFile, error) {
	prefix = strings.Trim(prefix, "/")
	parent, err := c.dir(ctx, prefix, false)
	if err != nil {
		return nil, err
	}
	out := []RemoteFile{}
	if parent == "" {
		return out, nil
	}
	var walk func(id, rel string, depth int) error
	walk = func(id, rel string, depth int) error {
		found, err := c.list(ctx, quoteQuery(id)+" in parents and trashed = false")
		if err != nil {
			return err
		}
		for _, f := range found {
			switch {
			case !f.folder():
				out = append(out, RemoteFile{Name: rel + "/" + f.Name, Size: int64(f.Size), ModTime: f.Modified})
			case depth < 1:
				if err := walk(f.ID, rel+"/"+f.Name, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(parent, prefix, 0); err != nil {
		return nil, err
	}
	return out, nil
}

// ---- settings and connecting ----

func (m *Manager) driveWithSecret(ctx context.Context) (*DriveConfig, error) {
	v, err := m.store.Setting(ctx, driveSettingKey)
	if err != nil || v == "" {
		return nil, err
	}
	var c DriveConfig
	if err := json.Unmarshal([]byte(v), &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// DriveStorage returns the Google Drive settings without their secrets, or
// nil when Google Drive is not connected.
func (m *Manager) DriveStorage(ctx context.Context) (*DriveView, error) {
	c, err := m.driveWithSecret(ctx)
	if c == nil || err != nil {
		return nil, err
	}
	return c.view(), nil
}

// DriveConnect is the state of a Google sign-in.
type DriveConnect struct {
	State           string     `json:"state"` // "waiting", "done" or "failed"
	UserCode        string     `json:"user_code,omitempty"`
	VerificationURL string     `json:"verification_url,omitempty"`
	ExpiresAt       time.Time  `json:"expires_at"`
	Error           string     `json:"error,omitempty"` // a code the UI translates
	Detail          string     `json:"detail,omitempty"`
	Drive           *DriveView `json:"drive,omitempty"` // once done
}

// driveConnect is the sign-in in progress; there is at most one.
type driveConnect struct {
	mu     sync.Mutex
	cur    *DriveConnect
	cancel context.CancelFunc
	gen    int
}

// StartDriveConnect checks the settings, asks Google for a code and waits
// in the background for the owner to enter it. Once Google gives a token,
// it creates the backup folder, tests it and saves the settings; saved is
// then called. An empty client secret keeps the stored one of the same
// client, an empty encryption passphrase the stored one.
func (m *Manager) StartDriveConnect(ctx context.Context, c DriveConfig, saved func(*DriveView)) (*DriveConnect, error) {
	old, err := m.driveWithSecret(ctx)
	if err != nil {
		return nil, err
	}
	c.ClientID, c.ClientSecret = strings.TrimSpace(c.ClientID), strings.TrimSpace(c.ClientSecret)
	switch {
	case c.ClientID == "" && builtinDriveClientID != "":
		c.ClientID, c.ClientSecret = builtinDriveClientID, builtinDriveClientSecret
	case c.ClientSecret == "" && old != nil && old.ClientID == c.ClientID:
		c.ClientSecret = old.ClientSecret
	}
	if err := validDriveClient(c.ClientID, c.ClientSecret); err != nil {
		return nil, err
	}
	if c.Folder, err = cleanDriveFolder(c.Folder); err != nil {
		return nil, err
	}
	var oldEnc Encryption
	if old != nil {
		oldEnc = old.Encryption
	}
	if err := mergeEncryption(&c.Encryption, oldEnc); err != nil {
		return nil, err
	}
	c.RefreshToken, c.FolderID, c.Account = "", "", ""

	dc, err := requestDeviceCode(ctx, c.ClientID)
	if err != nil {
		return nil, driveUserError(err)
	}
	st := &DriveConnect{State: "waiting", UserCode: dc.UserCode, VerificationURL: dc.VerificationURL,
		ExpiresAt: driveNow().Add(time.Duration(dc.ExpiresIn) * drivePollUnit)}

	m.drive.mu.Lock()
	if m.drive.cancel != nil {
		m.drive.cancel()
	}
	m.drive.gen++
	gen := m.drive.gen
	bg, cancel := context.WithDeadline(context.Background(), st.ExpiresAt.Add(5*time.Minute))
	m.drive.cur, m.drive.cancel = st, cancel
	res := *st
	m.drive.mu.Unlock()

	go func() {
		defer cancel()
		view, err := m.finishDriveConnect(bg, c, dc)
		m.drive.mu.Lock()
		defer m.drive.mu.Unlock()
		if m.drive.gen != gen {
			return // cancelled, or another sign-in started
		}
		done := *st
		if err != nil {
			ue := driveUserError(err).(*usererr.Error)
			done.State, done.Error, done.Detail = "failed", ue.Code, ue.Detail
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				done.Error, done.Detail = "gdrive_expired", ""
			}
		} else {
			done.State, done.Drive = "done", view
		}
		done.UserCode = ""
		m.drive.cur, m.drive.cancel = &done, nil
		if err == nil && saved != nil {
			saved(view)
		}
	}()
	return &res, nil
}

func (m *Manager) finishDriveConnect(ctx context.Context, c DriveConfig, dc *deviceCode) (*DriveView, error) {
	tok, err := pollDeviceToken(ctx, c.ClientID, c.ClientSecret, dc)
	if err != nil {
		return nil, err
	}
	c.RefreshToken = tok.RefreshToken
	cacheToken(&c, tok.AccessToken, tok.ExpiresIn)
	return m.testAndSaveDrive(ctx, c)
}

// testAndSaveDrive finds or creates the backup folder, tests it and saves
// the settings.
func (m *Manager) testAndSaveDrive(ctx context.Context, c DriveConfig) (*DriveView, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	cl := newDriveClient(c)
	q, err := cl.about(ctx)
	if err != nil {
		return nil, err
	}
	c.Account = q.email
	if _, err := cl.findRoot(ctx); err != nil {
		return nil, err
	}
	c.FolderID = cl.cfg.FolderID
	if err := testStorage(ctx, cl, c.Encryption); err != nil {
		return nil, err
	}
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}
	if err := m.store.SetSetting(ctx, driveSettingKey, string(b)); err != nil {
		return nil, err
	}
	return c.view(), nil
}

// DriveConnectStatus returns the state of the last sign-in, nil if none.
func (m *Manager) DriveConnectStatus() *DriveConnect {
	m.drive.mu.Lock()
	defer m.drive.mu.Unlock()
	if m.drive.cur == nil {
		return nil
	}
	st := *m.drive.cur
	if st.State != "waiting" && driveNow().After(st.ExpiresAt.Add(10*time.Minute)) {
		return nil
	}
	return &st
}

// CancelDriveConnect stops waiting for the code to be entered.
func (m *Manager) CancelDriveConnect() {
	m.drive.mu.Lock()
	defer m.drive.mu.Unlock()
	if m.drive.cancel != nil {
		m.drive.cancel()
	}
	m.drive.gen++
	m.drive.cur, m.drive.cancel = nil, nil
}

// SaveDrive changes the folder or the encryption of the connected Google
// Drive, without signing in again. The folder is found or created again,
// so saving also mends a folder that was deleted.
func (m *Manager) SaveDrive(ctx context.Context, req DriveConfig) (*DriveView, error) {
	old, err := m.driveWithSecret(ctx)
	if err != nil {
		return nil, err
	}
	if old == nil {
		return nil, usererr.New("gdrive_not_connected")
	}
	c := *old
	if c.Folder, err = cleanDriveFolder(req.Folder); err != nil {
		return nil, err
	}
	c.Encryption = req.Encryption
	if err := mergeEncryption(&c.Encryption, old.Encryption); err != nil {
		return nil, err
	}
	if c.Folder == old.Folder && c.FolderID != "" {
		// Keep the folder while it is there and not in the trash.
		cl := newDriveClient(c)
		if _, err := cl.root(ctx); err != nil {
			var de *driveError
			if !errors.As(err, &de) || de.code != "gdrive_folder_missing" && de.code != "gdrive_folder_trashed" {
				return nil, driveUserError(err)
			}
			c.FolderID = ""
		}
	} else {
		c.FolderID = ""
	}
	if c.FolderID != "" {
		// Test in place.
		ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		defer cancel()
		cl := newDriveClient(c)
		if q, err := cl.about(ctx); err != nil {
			return nil, driveUserError(err)
		} else {
			c.Account = q.email
		}
		if err := testStorage(ctx, cl, c.Encryption); err != nil {
			return nil, driveUserError(err)
		}
		b, err := json.Marshal(c)
		if err != nil {
			return nil, err
		}
		if err := m.store.SetSetting(ctx, driveSettingKey, string(b)); err != nil {
			return nil, err
		}
		return c.view(), nil
	}
	v, err := m.testAndSaveDrive(ctx, c)
	if err != nil {
		return nil, driveUserError(err)
	}
	return v, nil
}

// RemoveDrive forgets the Google Drive settings and gives up the panel's
// access (best effort). Backups already uploaded stay in Google Drive.
func (m *Manager) RemoveDrive(ctx context.Context) error {
	m.CancelDriveConnect()
	old, err := m.driveWithSecret(ctx)
	if err != nil {
		return err
	}
	if err := m.store.SetSetting(ctx, driveSettingKey, ""); err != nil {
		return err
	}
	if old != nil && old.RefreshToken != "" {
		if err := revokeDrive(ctx, old); err != nil {
			m.log.Warn("revoke Google Drive access", "err", err)
		}
	}
	return nil
}
