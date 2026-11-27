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
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeDrive is Google's OAuth endpoints and enough of the Drive API v3 for
// the panel, over TLS, with faults that tests can switch on.
type fakeDrive struct {
	t   *testing.T
	srv *httptest.Server

	mu sync.Mutex
	// OAuth
	clientID, secret string
	devicePolls      []string // answers to the device flow's polls, in order: pending, slow, denied, expired, ok, noscope
	polls            int
	refreshToken     string
	revoked          bool
	access           map[string]bool
	refreshes        int
	email            string
	limit, usage     int64
	// Drive
	files    map[string]*fakeFile
	nextID   int
	pageSize int
	sessions map[string]*fakeSession
	started  int // upload sessions
	// Faults
	chunkFault  map[int]string // by number of the upload PUT (from 1): half, drop, gone, busy
	chunkPuts   int
	statusAsks  int
	cutDownload int64 // the next download stops after this many bytes
	rateLimit   int   // so many API requests get "403 userRateLimitExceeded"
	uploadQuota bool  // the upload start answers "403 storageQuotaExceeded"
	requests    []string
	clock       time.Time
	slept       []time.Duration
}

type fakeFile struct {
	id, name, parent, mime string
	data                   []byte
	trashed                bool
	order                  int
}

type fakeSession struct {
	name, parent string
	size         int64
	data         []byte
}

// useFakeDrive points the Drive client at a new fake and makes waits instant
// (they are recorded, and advance a fake clock).
func useFakeDrive(t *testing.T) *fakeDrive {
	t.Helper()
	f := &fakeDrive{t: t, clientID: "123-abc.apps.googleusercontent.com", secret: "GOCSPX-test-secret",
		refreshToken: "1//refresh-token", access: map[string]bool{}, email: "owner@example.com",
		files: map[string]*fakeFile{}, pageSize: 2, sessions: map[string]*fakeSession{}, chunkFault: map[int]string{},
		clock: time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)}
	f.srv = httptest.NewTLSServer(http.HandlerFunc(f.serve))
	oldEP, oldHTTP, oldSleep, oldNow, oldUnit, oldChunk := driveEP, driveHTTP, driveSleep, driveNow, drivePollUnit, driveChunk
	u := f.srv.URL
	driveEP = driveEndpoints{DeviceCode: u + "/device/code", Token: u + "/token", Revoke: u + "/revoke",
		API: u + "/drive/v3", Upload: u + "/upload/drive/v3"}
	driveHTTP = &http.Client{Transport: f.srv.Client().Transport, CheckRedirect: oldHTTP.CheckRedirect}
	driveSleep = func(ctx context.Context, d time.Duration) error {
		f.mu.Lock()
		f.slept = append(f.slept, d)
		f.clock = f.clock.Add(d)
		f.mu.Unlock()
		return ctx.Err()
	}
	driveNow = func() time.Time {
		f.mu.Lock()
		defer f.mu.Unlock()
		return f.clock
	}
	drivePollUnit = time.Second
	driveChunk = 1000
	driveTokens.Lock()
	driveTokens.m = map[string]cachedToken{}
	driveTokens.Unlock()
	t.Cleanup(func() {
		driveEP, driveHTTP, driveSleep, driveNow, drivePollUnit, driveChunk = oldEP, oldHTTP, oldSleep, oldNow, oldUnit, oldChunk
		f.srv.Close()
	})
	return f
}

func (f *fakeDrive) set(fn func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn()
}

func writeJSONBody(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func driveErr(w http.ResponseWriter, status int, reason, msg string) {
	writeJSONBody(w, status, map[string]any{"error": map[string]any{"code": status, "message": msg,
		"errors": []map[string]string{{"reason": reason, "message": msg}}}})
}

// drop closes the connection without an answer: a network failure.
func drop(w http.ResponseWriter) {
	if hj, ok := w.(http.Hijacker); ok {
		if c, _, err := hj.Hijack(); err == nil {
			c.Close()
		}
	}
}

func (f *fakeDrive) fileJSON(ff *fakeFile) map[string]any {
	sum := sha256.Sum256(ff.data)
	m := map[string]any{"id": ff.id, "name": ff.name, "mimeType": ff.mime, "trashed": ff.trashed,
		"modifiedTime": "2026-11-27T10:00:00.000Z"}
	if ff.mime != driveFolderMime {
		m["size"] = strconv.Itoa(len(ff.data))
		m["sha256Checksum"] = hex.EncodeToString(sum[:])
	}
	return m
}

func (f *fakeDrive) newID(prefix string) string {
	f.nextID++
	return fmt.Sprintf("%s%d", prefix, f.nextID)
}

var (
	qParent = regexp.MustCompile(`^'((?:[^'\\]|\\.)*)' in parents and trashed = false`)
	qName   = regexp.MustCompile(` and name = '((?:[^'\\]|\\.)*)'`)
	qMime   = regexp.MustCompile(` and mimeType (!=|=) '((?:[^'\\]|\\.)*)'`)
)

func unquoteQ(s string) string { return strings.NewReplacer(`\'`, `'`, `\\`, `\`).Replace(s) }

func (f *fakeDrive) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, r.Method+" "+r.URL.Path+" "+r.Header.Get("Content-Range")+" "+r.Header.Get("Range"))
	p := r.URL.Path
	switch {
	case p == "/device/code" || p == "/token" || p == "/revoke":
		f.oauth(w, r)
		return
	case strings.HasPrefix(p, "/drive/v3/") || strings.HasPrefix(p, "/upload/drive/v3/"):
	default:
		http.NotFound(w, r)
		return
	}
	if !f.access[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")] {
		driveErr(w, http.StatusUnauthorized, "authError", "Invalid Credentials")
		return
	}
	if f.rateLimit > 0 && !strings.HasPrefix(p, "/upload/") {
		f.rateLimit--
		driveErr(w, http.StatusForbidden, "userRateLimitExceeded", "User Rate Limit Exceeded")
		return
	}
	switch {
	case p == "/drive/v3/about":
		writeJSONBody(w, 200, map[string]any{"user": map[string]any{"emailAddress": f.email},
			"storageQuota": map[string]any{"limit": strconv.FormatInt(f.limit, 10), "usage": strconv.FormatInt(f.usage, 10)}})
	case p == "/drive/v3/files" && r.Method == http.MethodGet:
		f.list(w, r)
	case p == "/drive/v3/files" && r.Method == http.MethodPost:
		var in struct {
			Name    string   `json:"name"`
			Mime    string   `json:"mimeType"`
			Parents []string `json:"parents"`
		}
		json.NewDecoder(r.Body).Decode(&in)
		if in.Mime != driveFolderMime || len(in.Parents) != 1 {
			driveErr(w, 400, "badRequest", "only folders are made this way")
			return
		}
		ff := &fakeFile{id: f.newID("folder"), name: in.Name, parent: in.Parents[0], mime: driveFolderMime, order: f.nextID}
		f.files[ff.id] = ff
		writeJSONBody(w, 200, map[string]any{"id": ff.id})
	case strings.HasPrefix(p, "/drive/v3/files/"):
		ff := f.files[strings.TrimPrefix(p, "/drive/v3/files/")]
		if ff == nil {
			driveErr(w, 404, "notFound", "File not found: "+p)
			return
		}
		switch {
		case r.Method == http.MethodDelete:
			delete(f.files, ff.id)
			w.WriteHeader(http.StatusNoContent)
		case r.URL.Query().Get("alt") == "media":
			f.download(w, r, ff)
		default:
			writeJSONBody(w, 200, f.fileJSON(ff))
		}
	case p == "/upload/drive/v3/files" && r.Method == http.MethodPost:
		f.startUpload(w, r)
	case p == "/upload/drive/v3/files" && r.Method == http.MethodPut:
		f.putChunk(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeDrive) oauth(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	oauthErr := func(status int, code, desc string) {
		writeJSONBody(w, status, map[string]string{"error": code, "error_description": desc})
	}
	newAccess := func() string {
		tok := f.newID("ya29.access-")
		f.access[tok] = true
		return tok
	}
	switch r.URL.Path {
	case "/device/code":
		if r.Form.Get("client_id") != f.clientID {
			oauthErr(401, "invalid_client", "The OAuth client was not found.")
			return
		}
		if r.Form.Get("scope") != driveScope {
			oauthErr(400, "invalid_scope", "bad scope")
			return
		}
		writeJSONBody(w, 200, map[string]any{"device_code": "DEVICE-CODE", "user_code": "ABCD-EFGH",
			"verification_url": "https://www.google.com/device", "expires_in": 1800, "interval": 5})
	case "/revoke":
		if r.Form.Get("token") == f.refreshToken {
			f.revoked = true
		}
		w.WriteHeader(200)
	case "/token":
		if r.Form.Get("client_id") != f.clientID || r.Form.Get("client_secret") != f.secret {
			oauthErr(401, "invalid_client", "Unauthorized")
			return
		}
		switch r.Form.Get("grant_type") {
		case "urn:ietf:params:oauth:grant-type:device_code":
			ans := "pending"
			if f.polls < len(f.devicePolls) {
				ans = f.devicePolls[f.polls]
			}
			f.polls++
			switch ans {
			case "pending":
				oauthErr(428, "authorization_pending", "Precondition Required")
			case "slow":
				oauthErr(403, "slow_down", "Forbidden")
			case "denied":
				oauthErr(403, "access_denied", "Forbidden")
			case "expired":
				oauthErr(400, "expired_token", "Bad Request")
			case "ok", "noscope":
				scope := driveScope
				if ans == "noscope" {
					scope = "openid"
				}
				writeJSONBody(w, 200, map[string]any{"access_token": newAccess(), "expires_in": 3599,
					"refresh_token": f.refreshToken, "scope": scope, "token_type": "Bearer"})
			}
		case "refresh_token":
			if f.revoked || r.Form.Get("refresh_token") != f.refreshToken {
				oauthErr(400, "invalid_grant", "Token has been expired or revoked.")
				return
			}
			f.refreshes++
			writeJSONBody(w, 200, map[string]any{"access_token": newAccess(), "expires_in": 3599, "scope": driveScope, "token_type": "Bearer"})
		default:
			oauthErr(400, "unsupported_grant_type", "")
		}
	}
}

func (f *fakeDrive) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	pm := qParent.FindStringSubmatch(q)
	if pm == nil {
		driveErr(w, 400, "invalid", "Invalid Value: "+q)
		return
	}
	parent := unquoteQ(pm[1])
	var out []*fakeFile
	for _, ff := range f.files {
		if ff.parent != parent || ff.trashed {
			continue
		}
		if nm := qName.FindStringSubmatch(q); nm != nil && ff.name != unquoteQ(nm[1]) {
			continue
		}
		if mm := qMime.FindStringSubmatch(q); mm != nil && (ff.mime == unquoteQ(mm[2])) != (mm[1] == "=") {
			continue
		}
		out = append(out, ff)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].order < out[j].order })
	start, _ := strconv.Atoi(r.URL.Query().Get("pageToken"))
	end := min(start+f.pageSize, len(out))
	page := []map[string]any{}
	for _, ff := range out[start:end] {
		page = append(page, f.fileJSON(ff))
	}
	res := map[string]any{"files": page}
	if end < len(out) {
		res["nextPageToken"] = strconv.Itoa(end)
	}
	writeJSONBody(w, 200, res)
}

func (f *fakeDrive) download(w http.ResponseWriter, r *http.Request, ff *fakeFile) {
	data, start := ff.data, 0
	if rg := r.Header.Get("Range"); rg != "" {
		s, _ := strings.CutSuffix(strings.TrimPrefix(rg, "bytes="), "-")
		start, _ = strconv.Atoi(s)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(data)-1, len(data)))
		w.Header().Set("Content-Length", strconv.Itoa(len(data)-start))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(200)
	}
	if cut := f.cutDownload; cut > 0 && int(cut) < len(data)-start {
		f.cutDownload = 0
		w.Write(data[start : start+int(cut)])
		w.(http.Flusher).Flush()
		drop(w)
		return
	}
	w.Write(data[start:])
}

func (f *fakeDrive) startUpload(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("uploadType") != "resumable" {
		driveErr(w, 400, "badRequest", "resumable only")
		return
	}
	var in struct {
		Name    string   `json:"name"`
		Parents []string `json:"parents"`
	}
	json.NewDecoder(r.Body).Decode(&in)
	size, err := strconv.ParseInt(r.Header.Get("X-Upload-Content-Length"), 10, 64)
	if err != nil || len(in.Parents) != 1 || f.files[in.Parents[0]] == nil {
		driveErr(w, 400, "badRequest", "bad upload start")
		return
	}
	if f.uploadQuota {
		driveErr(w, 403, "storageQuotaExceeded", "The user's Drive storage quota has been exceeded.")
		return
	}
	f.started++
	id := f.newID("session")
	f.sessions[id] = &fakeSession{name: in.Name, parent: in.Parents[0], size: size}
	w.Header().Set("Location", f.srv.URL+"/upload/drive/v3/files?uploadType=resumable&upload_id="+id)
	w.WriteHeader(200)
}

var contentRange = regexp.MustCompile(`^bytes (?:(\d+)-(\d+)|\*)/(\d+)$`)

func (f *fakeDrive) putChunk(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("upload_id")
	s := f.sessions[id]
	if s == nil {
		driveErr(w, 404, "notFound", "upload session not found")
		return
	}
	m := contentRange.FindStringSubmatch(r.Header.Get("Content-Range"))
	if m == nil {
		driveErr(w, 400, "badRequest", "bad Content-Range "+r.Header.Get("Content-Range"))
		return
	}
	body, _ := io.ReadAll(r.Body)
	if m[1] == "" {
		f.statusAsks++
	} else {
		f.chunkPuts++
		from, _ := strconv.ParseInt(m[1], 10, 64)
		to, _ := strconv.ParseInt(m[2], 10, 64)
		if from != int64(len(s.data)) || to-from+1 != int64(len(body)) {
			driveErr(w, 400, "badRequest", fmt.Sprintf("chunk %d-%d does not follow %d bytes", from, to, len(s.data)))
			return
		}
		switch f.chunkFault[f.chunkPuts] {
		case "half": // half of it arrives, then Google fails
			s.data = append(s.data, body[:len(body)/2]...)
			driveErr(w, 503, "backendError", "Backend Error")
			return
		case "drop":
			s.data = append(s.data, body[:len(body)/3]...)
			drop(w)
			return
		case "busy":
			driveErr(w, 500, "backendError", "Backend Error")
			return
		case "gone":
			delete(f.sessions, id)
			driveErr(w, 404, "notFound", "session expired")
			return
		}
		s.data = append(s.data, body...)
	}
	if int64(len(s.data)) < s.size {
		if len(s.data) > 0 {
			w.Header().Set("Range", fmt.Sprintf("bytes=0-%d", len(s.data)-1))
		}
		w.WriteHeader(http.StatusPermanentRedirect)
		return
	}
	delete(f.sessions, id)
	ff := &fakeFile{id: f.newID("file"), name: s.name, parent: s.parent, mime: "application/octet-stream", data: s.data, order: f.nextID}
	f.files[ff.id] = ff
	f.usage += int64(len(s.data))
	writeJSONBody(w, 200, f.fileJSON(ff))
}

// byPath finds a file or folder by its path from My Drive.
func (f *fakeDrive) byPath(p string) *fakeFile {
	f.mu.Lock()
	defer f.mu.Unlock()
	parent := "root"
	var cur *fakeFile
	for _, name := range strings.Split(p, "/") {
		cur = nil
		for _, ff := range f.files {
			if ff.parent == parent && ff.name == name && !ff.trashed && (cur == nil || ff.order < cur.order) {
				cur = ff
			}
		}
		if cur == nil {
			return nil
		}
		parent = cur.id
	}
	return cur
}

func (f *fakeDrive) count(name string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, ff := range f.files {
		if ff.name == name {
			n++
		}
	}
	return n
}

// connect signs the manager in through the device flow and waits until the
// settings are saved.
func (f *fakeDrive) connect(t *testing.T, m *Manager, c DriveConfig) *DriveView {
	t.Helper()
	f.set(func() { f.devicePolls = append(f.devicePolls[:f.polls], "pending", "ok") })
	c.ClientID, c.ClientSecret = f.clientID, f.secret
	st, err := m.StartDriveConnect(context.Background(), c, nil)
	if err != nil {
		t.Fatal(err)
	}
	if st.State != "waiting" || st.UserCode != "ABCD-EFGH" || st.VerificationURL != "https://www.google.com/device" {
		t.Fatalf("start = %+v", st)
	}
	for i := 0; i < 6000; i++ { // up to a minute: scrypt is slow under -race
		st = m.DriveConnectStatus()
		if st != nil && st.State != "waiting" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if st == nil || st.State != "done" {
		t.Fatalf("connect = %+v", st)
	}
	return st.Drive
}

func TestDriveDevicePolling(t *testing.T) {
	f := useFakeDrive(t)
	ctx := context.Background()
	dc, err := requestDeviceCode(ctx, f.clientID)
	if err != nil {
		t.Fatal(err)
	}
	s := time.Second
	cases := []struct {
		polls  []string
		err    error
		sleeps []time.Duration
	}{
		{[]string{"pending", "pending", "slow", "pending", "ok"}, nil, []time.Duration{5 * s, 5 * s, 5 * s, 10 * s, 10 * s}},
		{[]string{"pending", "denied"}, errDeviceDenied, nil},
		{[]string{"pending", "expired"}, errDeviceExpired, nil},
		{[]string{"ok"}, nil, []time.Duration{5 * s}},
		{[]string{"noscope"}, errDriveScope, nil},
	}
	for i, c := range cases {
		f.set(func() { f.devicePolls, f.polls, f.slept = c.polls, 0, nil })
		tok, err := pollDeviceToken(ctx, f.clientID, f.secret, dc)
		if !errors.Is(err, c.err) {
			t.Errorf("case %d: err %v, want %v", i, err, c.err)
			continue
		}
		if err == nil && (tok.RefreshToken != f.refreshToken || tok.AccessToken == "") {
			t.Errorf("case %d: token %+v", i, tok)
		}
		if c.sleeps != nil && !slices.Equal(f.slept, c.sleeps) {
			t.Errorf("case %d: waited %v, want %v", i, f.slept, c.sleeps)
		}
	}

	// The code expires while nobody enters it: the panel stops by itself.
	f.set(func() { f.devicePolls, f.polls = nil, 0 })
	short := *dc
	short.ExpiresIn = 12
	if _, err := pollDeviceToken(ctx, f.clientID, f.secret, &short); !errors.Is(err, errDeviceExpired) || f.polls != 2 {
		t.Errorf("expiry: %v after %d polls", err, f.polls)
	}
	// A wrong secret is the client's fault.
	if _, err := pollDeviceToken(ctx, f.clientID, "wrong", dc); userCode(driveUserError(err)) != "gdrive_client" {
		t.Errorf("wrong secret: %v", err)
	}
	// An unknown client is refused at once.
	if _, err := requestDeviceCode(ctx, "999-nope.apps.googleusercontent.com"); userCode(driveUserError(err)) != "gdrive_client" {
		t.Errorf("unknown client: %v", err)
	}
	// Google is unreachable.
	driveEP.DeviceCode = "https://127.0.0.1:1/device/code"
	if _, err := requestDeviceCode(ctx, f.clientID); userCode(driveUserError(err)) != "gdrive_unreachable" {
		t.Errorf("unreachable: %v", err)
	}
}

func TestDriveConnectAndOperations(t *testing.T) {
	f := useFakeDrive(t)
	m, st := newTestManager(t)
	ctx := context.Background()

	// Bad settings are refused before Google is asked.
	for code, c := range map[string]DriveConfig{
		"gdrive_client_id":     {ClientID: "not-a-client-id", ClientSecret: "x"},
		"gdrive_client_secret": {ClientID: f.clientID},
		"gdrive_folder":        {ClientID: f.clientID, ClientSecret: "x", Folder: "a/../b"},
		"backup_passphrase_invalid": {ClientID: f.clientID, ClientSecret: "x",
			Encryption: Encryption{Encrypt: true, EncryptPassphrase: "short"}},
	} {
		if _, err := m.StartDriveConnect(ctx, c, nil); userCode(err) != code {
			t.Errorf("%s: %v", code, err)
		}
	}

	view := f.connect(t, m, DriveConfig{Folder: " Nefo backups / test-host/ "})
	if view.Account != "owner@example.com" || view.Folder != "Nefo backups/test-host" || !strings.HasPrefix(view.FolderURL, "https://drive.google.com/drive/folders/folder") {
		t.Fatalf("view = %+v", view)
	}
	root := f.byPath("Nefo backups/test-host")
	if root == nil || root.mime != driveFolderMime {
		t.Fatal("backup folder not created")
	}
	if f.count(".panel-test") != 0 {
		t.Fatal("test file left behind")
	}
	stored, _ := st.Setting(ctx, driveSettingKey)
	if !strings.Contains(stored, f.refreshToken) || !strings.Contains(stored, f.secret) {
		t.Fatalf("stored = %s", stored)
	}
	v, _ := m.DriveStorage(ctx)
	vb, _ := json.Marshal(v)
	if strings.Contains(string(vb), f.secret) || strings.Contains(string(vb), f.refreshToken) {
		t.Fatalf("view leaks a secret: %s", vb)
	}
	if names, _ := m.ConfiguredRemotes(ctx); !slices.Equal(names, []string{RemoteDrive}) {
		t.Fatalf("remotes = %v", names)
	}

	r, _, err := m.openRemote(ctx, RemoteDrive)
	if err != nil {
		t.Fatal(err)
	}
	c := r.(*driveClient)
	key := "site/example.com/example.com_20261127-030000.tar.gz"
	body := randomBytes(4500) // five chunks of 1000 bytes
	sum := sha256.Sum256(body)
	if err := c.Put(ctx, key, bytes.NewReader(body), int64(len(body)), hex.EncodeToString(sum[:])); err != nil {
		t.Fatal(err)
	}
	ff := f.byPath("Nefo backups/test-host/site/example.com/example.com_20261127-030000.tar.gz")
	if ff == nil || !bytes.Equal(ff.data, body) {
		t.Fatal("file not stored in its sub-folder")
	}
	if f.chunkPuts != 5+1 { // and one chunk for the test file
		t.Errorf("%d chunk requests", f.chunkPuts)
	}
	var got bytes.Buffer
	if err := c.Get(ctx, key, &got); err != nil || !bytes.Equal(got.Bytes(), body) {
		t.Fatalf("Get: %v", err)
	}

	// Uploading under the same key replaces the file.
	if err := c.Put(ctx, key, strings.NewReader("v2"), 2, ""); err != nil {
		t.Fatal(err)
	}
	if n := f.count("example.com_20261127-030000.tar.gz"); n != 1 {
		t.Fatalf("%d copies after replacing", n)
	}
	got.Reset()
	if err := c.Get(ctx, key, &got); err != nil || got.String() != "v2" {
		t.Fatalf("Get after replace = %q, %v", got.String(), err)
	}

	// Listing goes through every page (the fake has pages of 2).
	for i := 0; i < 4; i++ {
		k := fmt.Sprintf("site/example.com/example.com_2026112%d-030000.tar.gz", i)
		if err := c.Put(ctx, k, strings.NewReader("x"), 1, ""); err != nil {
			t.Fatal(err)
		}
	}
	files, err := c.List(ctx, "site/example.com")
	if err != nil || len(files) != 5 || files[0].Name != "example.com_20261120-030000.tar.gz" || files[4].Name != "example.com_20261127-030000.tar.gz" || files[4].Size != 2 {
		t.Fatalf("List = %+v, %v", files, err)
	}
	if files, err := c.List(ctx, "site/missing.example"); err != nil || len(files) != 0 {
		t.Fatalf("List of a missing folder = %+v, %v", files, err)
	}
	pages := 0
	for _, rq := range f.requests {
		if strings.HasPrefix(rq, "GET /drive/v3/files ") {
			pages++
		}
	}
	if pages < 3 {
		t.Errorf("listing used %d requests", pages)
	}

	if err := c.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if f.count("example.com_20261127-030000.tar.gz") != 0 {
		t.Fatal("file not deleted")
	}
	if err := c.Delete(ctx, key); err != nil {
		t.Fatalf("deleting a missing file: %v", err)
	}
	if err := c.Get(ctx, key, io.Discard); err == nil || !strings.Contains(err.Error(), "missing in Google Drive") {
		t.Fatalf("Get after delete: %v", err)
	}

	// Removing the storage gives up the access.
	if err := m.RemoveDrive(ctx); err != nil {
		t.Fatal(err)
	}
	if !f.revoked {
		t.Error("refresh token not revoked")
	}
	if v, _ := m.DriveStorage(ctx); v != nil {
		t.Error("settings kept")
	}
}

func TestDriveConnectFailures(t *testing.T) {
	f := useFakeDrive(t)
	m, _ := newTestManager(t)
	ctx := context.Background()
	wait := func() *DriveConnect {
		for i := 0; i < 6000; i++ { // up to a minute: scrypt is slow under -race
			if st := m.DriveConnectStatus(); st != nil && st.State != "waiting" {
				return st
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("still waiting")
		return nil
	}
	f.set(func() { f.devicePolls = []string{"pending", "denied"} })
	if _, err := m.StartDriveConnect(ctx, DriveConfig{ClientID: f.clientID, ClientSecret: f.secret}, nil); err != nil {
		t.Fatal(err)
	}
	if st := wait(); st.State != "failed" || st.Error != "gdrive_denied" || st.UserCode != "" {
		t.Fatalf("denied: %+v", st)
	}
	// The quota is full: the test upload fails, nothing is saved.
	f.set(func() { f.devicePolls, f.polls, f.uploadQuota = []string{"ok"}, 0, true })
	if _, err := m.StartDriveConnect(ctx, DriveConfig{ClientID: f.clientID, ClientSecret: f.secret}, nil); err != nil {
		t.Fatal(err)
	}
	if st := wait(); st.State != "failed" || st.Error != "gdrive_quota" {
		t.Fatalf("quota: %+v", st)
	}
	if v, _ := m.DriveStorage(ctx); v != nil {
		t.Fatal("saved although the test failed")
	}
	// Cancelling forgets the sign-in.
	f.set(func() { f.devicePolls, f.polls, f.uploadQuota = nil, 0, false })
	if _, err := m.StartDriveConnect(ctx, DriveConfig{ClientID: f.clientID, ClientSecret: f.secret}, nil); err != nil {
		t.Fatal(err)
	}
	m.CancelDriveConnect()
	if st := m.DriveConnectStatus(); st != nil {
		t.Fatalf("after cancel: %+v", st)
	}
	// The default folder is named after the server.
	view := f.connect(t, m, DriveConfig{})
	if view.Folder != DefaultDriveFolder() || !strings.HasPrefix(view.Folder, "Nefo backups/") {
		t.Fatalf("default folder %q", view.Folder)
	}
	// Connecting again without the secret keeps the saved one.
	f.set(func() { f.devicePolls, f.polls = []string{"ok"}, 0 })
	if _, err := m.StartDriveConnect(ctx, DriveConfig{ClientID: f.clientID}, nil); err != nil {
		t.Fatalf("connect again without the secret: %v", err)
	}
	if st := wait(); st.State != "done" {
		t.Fatalf("again: %+v", st)
	}
	if n := f.count("test-host"); n > 1 {
		t.Fatal("folder made twice")
	}
}

func TestDriveResumableUpload(t *testing.T) {
	f := useFakeDrive(t)
	m, _ := newTestManager(t)
	ctx := context.Background()
	f.connect(t, m, DriveConfig{Folder: "Backups"})
	r, _, _ := m.openRemote(ctx, RemoteDrive)
	c := r.(*driveClient)
	body := randomBytes(5500)

	// Chunk 2 arrives half and Google fails, chunk 4 is cut by the network:
	// the client asks what arrived and goes on from there.
	f.set(func() {
		f.chunkPuts, f.statusAsks, f.chunkFault = 0, 0, map[int]string{2: "half", 4: "drop", 5: "busy"}
	})
	if err := c.Put(ctx, "a/b.tar.gz", bytes.NewReader(body), int64(len(body)), ""); err != nil {
		t.Fatal(err)
	}
	if ff := f.byPath("Backups/a/b.tar.gz"); ff == nil || !bytes.Equal(ff.data, body) {
		t.Fatal("resumed upload stored wrong data")
	}
	if f.statusAsks != 3 {
		t.Errorf("%d status requests, want 3", f.statusAsks)
	}
	if f.started != 2 { // the connection test and this one
		t.Errorf("%d sessions", f.started)
	}

	// The session expires in the middle: the upload starts again.
	f.set(func() { f.chunkPuts, f.chunkFault = 0, map[int]string{3: "gone"} })
	if err := c.Put(ctx, "a/c.tar.gz", bytes.NewReader(body), int64(len(body)), ""); err != nil {
		t.Fatal(err)
	}
	if ff := f.byPath("Backups/a/c.tar.gz"); ff == nil || !bytes.Equal(ff.data, body) || f.started != 4 {
		t.Fatalf("restarted upload: %d sessions", f.started)
	}

	// Google keeps failing: the upload gives up after the retries.
	faults := map[int]string{}
	for i := 1; i <= 40; i++ {
		faults[i] = "busy"
	}
	f.set(func() { f.chunkPuts, f.chunkFault = 0, faults })
	err := c.Put(ctx, "a/d.tar.gz", bytes.NewReader(body), int64(len(body)), "")
	if err == nil || !strings.Contains(err.Error(), "failed 11 times in a row") {
		t.Fatalf("failing Google: %v", err)
	}
	if f.byPath("Backups/a/d.tar.gz") != nil {
		t.Fatal("a failed upload left a file")
	}

	// A checksum that does not match: the copy is removed.
	f.set(func() { f.chunkFault = map[int]string{} })
	if err := c.Put(ctx, "a/e.tar.gz", bytes.NewReader(body), int64(len(body)), strings.Repeat("0", 64)); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("bad checksum: %v", err)
	}
	if f.byPath("Backups/a/e.tar.gz") != nil {
		t.Fatal("damaged copy kept")
	}

	// A broken download goes on from where it stopped.
	f.set(func() { f.cutDownload = 1700 })
	var got bytes.Buffer
	if err := c.Get(ctx, "a/b.tar.gz", &got); err != nil || !bytes.Equal(got.Bytes(), body) {
		t.Fatalf("resumed download: %v (%d bytes)", err, got.Len())
	}
	resumed := false
	for _, rq := range f.requests {
		resumed = resumed || strings.Contains(rq, "bytes=1700-")
	}
	if !resumed {
		t.Error("download did not resume with a range")
	}
}

func TestDriveTokensAndErrors(t *testing.T) {
	f := useFakeDrive(t)
	m, _ := newTestManager(t)
	ctx := context.Background()
	f.connect(t, m, DriveConfig{Folder: "Backups/srv"})
	open := func() *driveClient {
		r, _, err := m.openRemote(ctx, RemoteDrive)
		if err != nil {
			t.Fatal(err)
		}
		return r.(*driveClient)
	}

	// The access token is kept between operations.
	before := f.refreshes
	if _, err := open().List(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := open().List(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if f.refreshes != before {
		t.Errorf("%d token renewals for cached tokens", f.refreshes-before)
	}
	// Google drops the access token: 401, one renewal, and it works.
	f.set(func() { f.access = map[string]bool{} })
	if err := open().Put(ctx, "x/y", strings.NewReader("data"), 4, ""); err != nil {
		t.Fatal(err)
	}
	if f.refreshes != before+1 {
		t.Errorf("%d renewals after a 401", f.refreshes-before)
	}
	// Rate limits are waited out.
	f.set(func() { f.rateLimit, f.slept = 2, nil })
	if _, err := open().List(ctx, "x"); err != nil {
		t.Fatalf("rate limited: %v", err)
	}
	if len(f.slept) != 2 {
		t.Errorf("waited %v", f.slept)
	}

	// Quota: refused before the upload when about says so, and when the
	// upload itself is refused.
	f.set(func() { f.limit, f.usage = 1000, 990 })
	if err := open().Put(ctx, "x/big", strings.NewReader(strings.Repeat("x", 50)), 50, ""); !errors.Is(err, errDriveFull) {
		t.Fatalf("pre-check: %v", err)
	}
	f.set(func() { f.limit, f.uploadQuota = 0, true })
	if err := open().Put(ctx, "x/big", strings.NewReader("x"), 1, ""); !errors.Is(err, errDriveFull) || !strings.Contains(err.Error(), "full") {
		t.Fatalf("upload refused: %v", err)
	}
	f.set(func() { f.uploadQuota = false })

	// The folder goes to the trash, then is deleted for good.
	root := f.byPath("Backups/srv")
	f.set(func() { root.trashed = true })
	if err := open().Put(ctx, "x/z", strings.NewReader("x"), 1, ""); userCode(driveUserError(err)) != "gdrive_folder_trashed" || !strings.Contains(err.Error(), "trash") {
		t.Fatalf("trashed folder: %v", err)
	}
	f.set(func() { delete(f.files, root.id) })
	if err := open().Put(ctx, "x/z", strings.NewReader("x"), 1, ""); userCode(driveUserError(err)) != "gdrive_folder_missing" || !strings.Contains(err.Error(), "deleted") {
		t.Fatalf("deleted folder: %v", err)
	}
	// Saving the settings again makes a new folder.
	v, err := m.SaveDrive(ctx, DriveConfig{Folder: "Backups/srv"})
	if err != nil {
		t.Fatal(err)
	}
	if nr := f.byPath("Backups/srv"); nr == nil || nr.id == root.id || !strings.HasSuffix(v.FolderURL, nr.id) {
		t.Fatalf("folder not made again: %+v", v)
	}
	if err := open().Put(ctx, "x/z", strings.NewReader("x"), 1, ""); err != nil {
		t.Fatalf("after mending: %v", err)
	}

	// The owner removes the panel's access in their Google account.
	f.set(func() { f.revoked, f.access = true, map[string]bool{} })
	err = open().Put(ctx, "x/w", strings.NewReader("x"), 1, "")
	if !errors.Is(err, errDriveRevoked) || userCode(driveUserError(err)) != "gdrive_revoked" || !strings.Contains(err.Error(), "Connect Google Drive again") {
		t.Fatalf("revoked: %v", err)
	}
	if _, err := m.SaveDrive(ctx, DriveConfig{Folder: "Backups/srv"}); userCode(err) != "gdrive_revoked" {
		t.Fatalf("save while revoked: %v", err)
	}
}

func TestDriveQuery(t *testing.T) {
	if got := quoteQuery(`it's a \ test`); got != `'it\'s a \\ test'` {
		t.Fatal(got)
	}
	for in, want := range map[string]int64{"": 0, "bytes=0-0": 1, "bytes=0-1048575": 1048576} {
		if got, err := receivedRange(in); err != nil || got != want {
			t.Errorf("%q: %d %v", in, got, err)
		}
	}
	if _, err := receivedRange("bytes=5-9"); err == nil {
		t.Error("range not from 0 accepted")
	}
	for in, ok := range map[string]bool{"Nefo backups/srv1": true, "a": true, "": true, "a/../b": false, "a\\b": false, "x\ny": false} {
		if _, err := cleanDriveFolder(in); (err == nil) != ok {
			t.Errorf("%q: %v", in, err)
		}
	}
	if err := validDriveClient("123-abc.apps.googleusercontent.com", "GOCSPX-x"); err != nil {
		t.Error(err)
	}
	u, _ := url.Parse(googleEndpoints.Upload)
	if u.Host != "www.googleapis.com" || builtinDriveClientID != "" {
		t.Error("production endpoints changed")
	}
}
