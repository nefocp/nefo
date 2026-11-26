package hosting

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nefocp/nefo/internal/usererr"
	"github.com/nefocp/nefo/internal/version"
)

// DNS-01 validation through Cloudflare's API: needed for wildcard names, and
// useful for sites that are not reachable over HTTP yet. The API token needs
// the "Edit zone DNS" permission (Zone → DNS → Edit) for the zones in use.
// The DNS page uses the same client and token (cloudflare_dns.go).

// CloudflareTokenKey is the setting that holds the API token.
const CloudflareTokenKey = "cloudflare_token"

// cloudflareAPI is the API base URL; tests point it at a fake server.
var cloudflareAPI = "https://api.cloudflare.com/client/v4"

// lookupTXT asks one name server for TXT records; tests replace it.
var lookupTXT = func(ctx context.Context, server, name string) ([]string, error) {
	if _, _, err := net.SplitHostPort(server); err != nil {
		server = net.JoinHostPort(server, "53")
	}
	r := &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, network, server)
	}}
	return r.LookupTXT(ctx, name)
}

type cloudflare struct {
	token string
	http  *http.Client
}

func newCloudflare(token string) *cloudflare {
	return &cloudflare{token: token, http: &http.Client{Timeout: 30 * time.Second}}
}

// cfError is an error answer from Cloudflare's API. Its text holds
// Cloudflare's messages and codes, never the token.
type cfError struct {
	Status int
	Codes  []int
	Text   string
}

func (e *cfError) Error() string { return "Cloudflare API: " + e.Text }

// denied reports whether Cloudflare refused the request for the token's
// permissions: 10000 "Authentication error" or 9109 "Unauthorized to
// access requested resource", usually with HTTP 403.
func (e *cfError) denied() bool {
	return e.has(10000, 9109) || (e.Status == http.StatusForbidden && !e.has(1000, 6003, 6111))
}

// badToken reports whether Cloudflare does not know the token at all:
// 1000 "Invalid API Token", or 6003/6111 for a malformed one.
func (e *cfError) badToken() bool {
	return e.has(1000, 6003, 6111) || (e.Status == http.StatusUnauthorized && !e.has(10000, 9109))
}

func (e *cfError) has(codes ...int) bool {
	for _, c := range e.Codes {
		if slices.Contains(codes, c) {
			return true
		}
	}
	return false
}

// cfPage is the paging information of a list answer.
type cfPage struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Count      int `json:"count"`
	TotalCount int `json:"total_count"`
	TotalPages int `json:"total_pages"`
}

// do calls the API and decodes "result" into out.
func (c *cloudflare) do(ctx context.Context, method, path string, in, out any) error {
	_, err := c.call(ctx, method, path, in, out)
	return err
}

// call is do that also returns the paging information of a list.
func (c *cloudflare) call(ctx context.Context, method, path string, in, out any) (*cfPage, error) {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, cloudflareAPI+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", version.AppName+"/"+version.Version)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Cloudflare API: %w", err)
	}
	defer resp.Body.Close()
	type cfMsg struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Chain   []struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error_chain"`
	}
	var env struct {
		Success bool            `json:"success"`
		Errors  []cfMsg         `json:"errors"`
		Result  json.RawMessage `json:"result"`
		Info    *cfPage         `json:"result_info"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&env); err != nil {
		return nil, fmt.Errorf("Cloudflare API: HTTP %d, unreadable answer", resp.StatusCode)
	}
	if !env.Success {
		e := &cfError{Status: resp.StatusCode}
		msgs := []string{}
		for _, m := range env.Errors {
			e.Codes = append(e.Codes, m.Code)
			msgs = append(msgs, fmt.Sprintf("%s (%d)", truncateText(m.Message, 300), m.Code))
			for _, c := range m.Chain {
				e.Codes = append(e.Codes, c.Code)
				msgs = append(msgs, fmt.Sprintf("%s (%d)", truncateText(c.Message, 300), c.Code))
			}
			if len(msgs) >= 10 {
				break
			}
		}
		if len(msgs) == 0 {
			msgs = append(msgs, fmt.Sprintf("HTTP %d", resp.StatusCode))
		}
		e.Text = strings.Join(msgs, "; ")
		return nil, e
	}
	if out != nil {
		if err := json.Unmarshal(env.Result, out); err != nil {
			return nil, fmt.Errorf("Cloudflare API: unexpected answer: %w", err)
		}
	}
	return env.Info, nil
}

// truncateText shortens s to at most n bytes, on a character boundary.
func truncateText(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + "…"
}

// Verify checks that the token is valid and active.
func (c *cloudflare) Verify(ctx context.Context) error {
	var r struct {
		Status string `json:"status"`
	}
	if err := c.do(ctx, http.MethodGet, "/user/tokens/verify", nil, &r); err != nil {
		return err
	}
	if r.Status != "active" {
		return fmt.Errorf("the token is %s", r.Status)
	}
	return nil
}

type cfZone struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	NameServers []string `json:"name_servers"`
	Plan        struct {
		Name string `json:"name"`
	} `json:"plan"`
}

// zoneFor finds the Cloudflare zone that holds name, trying the name and
// then each parent domain.
func (c *cloudflare) zoneFor(ctx context.Context, name string) (*cfZone, error) {
	labels := strings.Split(strings.TrimPrefix(name, "*."), ".")
	for i := 0; i < len(labels)-1; i++ {
		cand := strings.Join(labels[i:], ".")
		var zones []cfZone
		if err := c.do(ctx, http.MethodGet, "/zones?status=active&name="+url.QueryEscape(cand), nil, &zones); err != nil {
			return nil, err
		}
		if len(zones) > 0 && zones[0].ID != "" {
			return &zones[0], nil
		}
	}
	// Certificates are issued in a job, whose errors are shown as text.
	return nil, fmt.Errorf("none of the Cloudflare zones this API token can use holds %s", name)
}

// Zones lists the zones the token can see, for the settings page.
func (c *cloudflare) Zones(ctx context.Context) ([]string, error) {
	var zones []cfZone
	if err := c.do(ctx, http.MethodGet, "/zones?status=active&per_page=50", nil, &zones); err != nil {
		return nil, err
	}
	names := []string{}
	for _, z := range zones {
		names = append(names, z.Name)
	}
	return names, nil
}

// Present creates the TXT record fqdn=value, waits until the zone's name
// servers answer with it, and returns a function that deletes it.
func (c *cloudflare) Present(ctx context.Context, fqdn, value string, lg Logger) (func(), error) {
	zone, err := c.zoneFor(ctx, strings.TrimPrefix(fqdn, "_acme-challenge."))
	if err != nil {
		return nil, err
	}
	var rec struct {
		ID string `json:"id"`
	}
	if err := c.do(ctx, http.MethodPost, "/zones/"+url.PathEscape(zone.ID)+"/dns_records",
		map[string]any{"type": "TXT", "name": fqdn, "content": value, "ttl": 120}, &rec); err != nil {
		return nil, err
	}
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if err := c.do(ctx, http.MethodDelete, "/zones/"+url.PathEscape(zone.ID)+"/dns_records/"+url.PathEscape(rec.ID), nil, nil); err != nil {
			lg.Logf("Warning: could not delete the DNS record %s: %v", fqdn, err)
		}
	}
	lg.Logf("Waiting until %s answers with the new DNS record", strings.Join(zone.NameServers, ", "))
	if err := waitTXT(ctx, zone.NameServers, fqdn, value, 3*time.Minute); err != nil {
		cleanup()
		return nil, err
	}
	return cleanup, nil
}

// waitTXT polls every name server until each answers with value.
func waitTXT(ctx context.Context, servers []string, name, value string, limit time.Duration) error {
	if len(servers) == 0 {
		return nil
	}
	deadline := time.Now().Add(limit)
	for {
		missing := ""
		for _, ns := range servers {
			qctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			vals, _ := lookupTXT(qctx, ns, name)
			cancel()
			found := false
			for _, v := range vals {
				if v == value {
					found = true
				}
			}
			if !found {
				missing = ns
				break
			}
		}
		if missing == "" {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("the DNS record %s did not appear on %s in time", name, missing)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}

// dnsSolver publishes DNS-01 challenge records.
type dnsSolver interface {
	Present(ctx context.Context, fqdn, value string, lg Logger) (cleanup func(), err error)
}

// cloudflareSolver returns the configured Cloudflare client.
func (m *Manager) cloudflareSolver(ctx context.Context) (*cloudflare, error) {
	token, _ := m.store.Setting(ctx, CloudflareTokenKey)
	if token == "" {
		return nil, errors.New("DNS validation needs a Cloudflare API token: add one in Settings → Cloudflare DNS")
	}
	return newCloudflare(token), nil
}

// CloudflareStatus describes the saved Cloudflare token.
type CloudflareStatus struct {
	Configured bool     `json:"configured"`
	Hint       string   `json:"hint,omitempty"` // last characters of the token
	Zones      []string `json:"zones"`
	Error      string   `json:"error,omitempty"`
}

// Cloudflare returns the state of the Cloudflare token, with its zones.
func (m *Manager) Cloudflare(ctx context.Context) CloudflareStatus {
	token, _ := m.store.Setting(ctx, CloudflareTokenKey)
	st := CloudflareStatus{Configured: token != "", Zones: []string{}}
	if token == "" {
		return st
	}
	if len(token) > 4 {
		st.Hint = "…" + token[len(token)-4:]
	}
	zones, err := newCloudflare(token).Zones(ctx)
	if err != nil {
		st.Error = err.Error()
	} else {
		st.Zones = zones
	}
	return st
}

// SetCloudflare checks and saves the Cloudflare token; "" removes it.
func (m *Manager) SetCloudflare(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return m.store.SetSetting(ctx, CloudflareTokenKey, "")
	}
	if len(token) > 200 || strings.ContainsAny(token, " \t\r\n") {
		return usererr.New("cloudflare_token_invalid", "")
	}
	if err := newCloudflare(token).Verify(ctx); err != nil {
		var ue *usererr.Error
		if errors.As(err, &ue) {
			return err
		}
		return usererr.New("cloudflare_token_invalid", err.Error())
	}
	return m.store.SetSetting(ctx, CloudflareTokenKey, token)
}
