package hosting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// fakeDNS is a Cloudflare API with zones and records, enough for the DNS
// page. Tokens: "good-token" may do everything; "read-token" may read but
// not change records or purge; "no-zones-token" is refused everywhere.
type fakeDNS struct {
	mu      sync.Mutex
	zones   []map[string]any
	records map[string]map[string]any // id → record, with "zone_id"
	nextID  int
	purges  []map[string]any
	calls   []string
}

const (
	zoneCom = "0000000000000000000000000000c0de"
	zoneOrg = "000000000000000000000000000000f9"
)

func newFakeDNS() *fakeDNS {
	return &fakeDNS{
		zones: []map[string]any{
			{"id": zoneCom, "name": "example.com", "status": "active", "plan": map[string]any{"name": "Free Website"}},
			{"id": zoneOrg, "name": "example.org", "status": "pending", "plan": map[string]any{"name": "Pro Website"}},
		},
		records: map[string]map[string]any{},
	}
}

func (f *fakeDNS) add(zone, typ, name, content string, proxied bool) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.addLocked(zone, map[string]any{"type": typ, "name": name, "content": content, "proxied": proxied, "ttl": 1})
}

func (f *fakeDNS) addLocked(zone string, r map[string]any) string {
	f.nextID++
	id := fmt.Sprintf("%032x", f.nextID)
	r["id"], r["zone_id"] = id, zone
	r["proxiable"] = r["type"] == "A" || r["type"] == "AAAA" || r["type"] == "CNAME"
	if r["type"] == "CAA" {
		d := r["data"].(map[string]any)
		r["content"] = fmt.Sprintf("%v %v %q", d["flags"], d["tag"], d["value"])
	}
	f.records[id] = r
	return id
}

// list returns the records of a name and type, as "content" or
// "content proxied".
func (f *fakeDNS) list(typ, name string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, r := range f.records {
		if r["type"] == typ && r["name"] == name {
			s := r["content"].(string)
			if p, _ := r["proxied"].(bool); p {
				s += " proxied"
			}
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

func (f *fakeDNS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, r.Method+" "+r.URL.Path)
	send := func(status int, v map[string]any) {
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(v)
	}
	fail := func(status, code int, msg string) {
		send(status, map[string]any{"success": false, "errors": []any{map[string]any{"code": code, "message": msg}}})
	}
	ok := func(result any, info map[string]any) {
		v := map[string]any{"success": true, "errors": []any{}, "result": result}
		if info != nil {
			v["result_info"] = info
		}
		send(http.StatusOK, v)
	}
	page := func(items []map[string]any) {
		per, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
		n, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if per <= 0 || per > 100 {
			per = 20
		}
		if n <= 0 {
			n = 1
		}
		lo, hi := min((n-1)*per, len(items)), min(n*per, len(items))
		ok(items[lo:hi], map[string]any{"page": n, "per_page": per, "count": hi - lo, "total_count": len(items), "total_pages": (len(items) + per - 1) / per})
	}

	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	write := r.Method != http.MethodGet
	switch {
	case token == "no-zones-token":
		fail(http.StatusForbidden, 9109, "Unauthorized to access requested resource")
		return
	case token == "read-token" && write:
		fail(http.StatusForbidden, 10000, "Authentication error")
		return
	case token != "good-token" && token != "read-token":
		fail(http.StatusUnauthorized, 1000, "Invalid API Token")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/zones":
		page(f.zones)
	case len(parts) >= 3 && parts[0] == "zones" && parts[2] == "dns_records":
		zone := parts[1]
		id := ""
		if len(parts) == 4 {
			id = parts[3]
		}
		rec := f.records[id]
		if id != "" && (rec == nil || rec["zone_id"] != zone) {
			fail(http.StatusNotFound, 81044, "Record does not exist.")
			return
		}
		switch r.Method {
		case http.MethodGet:
			if id != "" {
				ok(rec, nil)
				return
			}
			name := r.URL.Query().Get("name")
			var items []map[string]any
			for _, x := range f.records {
				if x["zone_id"] == zone && (name == "" || x["name"] == name) {
					items = append(items, x)
				}
			}
			sort.Slice(items, func(i, j int) bool { return items[i]["id"].(string) < items[j]["id"].(string) })
			page(items)
		case http.MethodPost:
			var in map[string]any
			json.NewDecoder(r.Body).Decode(&in)
			for _, x := range f.records {
				if x["name"] == in["name"] && (x["type"] == "CNAME") != (in["type"] == "CNAME") {
					fail(http.StatusBadRequest, 81053, "An A, AAAA, or CNAME record with that host already exists.")
					return
				}
			}
			id := f.addLocked(zone, in)
			ok(f.records[id], nil)
		case http.MethodPatch:
			var in map[string]any
			json.NewDecoder(r.Body).Decode(&in)
			for k, v := range in {
				rec[k] = v
			}
			ok(rec, nil)
		case http.MethodDelete:
			delete(f.records, id)
			ok(map[string]any{"id": id}, nil)
		}
	case r.Method == http.MethodPost && len(parts) == 3 && parts[2] == "purge_cache":
		var in map[string]any
		json.NewDecoder(r.Body).Decode(&in)
		in["zone_id"] = parts[1]
		f.purges = append(f.purges, in)
		ok(map[string]any{"id": parts[1]}, nil)
	default:
		fail(http.StatusNotFound, 7003, "Could not route to "+r.URL.Path)
	}
}

// dnsTest returns a manager whose Cloudflare token is token, talking to
// the fake API.
func dnsTest(t *testing.T, token string) (*Manager, *fakeDNS) {
	t.Helper()
	fake := newFakeDNS()
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	old := cloudflareAPI
	cloudflareAPI = srv.URL
	t.Cleanup(func() { cloudflareAPI = old })
	m := newTestManager(t)
	if err := m.store.SetSetting(context.Background(), CloudflareTokenKey, token); err != nil {
		t.Fatal(err)
	}
	return m, fake
}

func errCode(err error) (string, string) {
	var ue *usererr.Error
	if errors.As(err, &ue) {
		return ue.Code, ue.Detail
	}
	if err == nil {
		return "", ""
	}
	return "other", err.Error()
}

func TestDNSZonesAndRecords(t *testing.T) {
	ctx := context.Background()
	m, fake := dnsTest(t, "good-token")
	zones, err := m.DNSZones(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(zones) != 2 || zones[0].Name != "example.com" || zones[0].Plan != "Free Website" || zones[1].Status != "pending" {
		t.Fatalf("zones = %+v", zones)
	}

	// 250 records: three pages of 100.
	for i := 0; i < 249; i++ {
		fake.add(zoneCom, "A", fmt.Sprintf("h%03d.example.com", i), "203.0.113.1", false)
	}
	fake.add(zoneCom, "MX", "example.com", "mail.example.com", false)
	list, err := m.DNSRecords(ctx, zoneCom)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Records) != 250 || list.More || list.Zone.Name != "example.com" {
		t.Fatalf("got %d records, more %v", len(list.Records), list.More)
	}
	if list.Records[0].Name != "example.com" || list.Records[1].Name != "h000.example.com" {
		t.Fatalf("not sorted: %s, %s", list.Records[0].Name, list.Records[1].Name)
	}
	pages := 0
	for _, c := range fake.calls {
		if c == "GET /zones/"+zoneCom+"/dns_records" {
			pages++
		}
	}
	if pages != 3 {
		t.Fatalf("asked %d pages", pages)
	}

	// A zone larger than the limit: the first maxRecords, and "more".
	for i := 250; i < maxRecords+20; i++ {
		fake.add(zoneCom, "TXT", fmt.Sprintf("t%04d.example.com", i), "x", false)
	}
	list, err = m.DNSRecords(ctx, zoneCom)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Records) != maxRecords || !list.More {
		t.Fatalf("got %d records, more %v", len(list.Records), list.More)
	}

	// Unknown zone IDs, and IDs that are not Cloudflare's.
	if _, err := m.DNSRecords(ctx, "ffffffffffffffffffffffffffffffff"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown zone: %v", err)
	}
	if _, err := m.DNSRecords(ctx, "../user"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("bad zone id: %v", err)
	}
}

func TestDNSRecordChanges(t *testing.T) {
	ctx := context.Background()
	m, fake := dnsTest(t, "good-token")

	rec, err := m.CreateDNSRecord(ctx, zoneCom, RecordInput{Type: "a", Name: "www", Content: " 203.0.113.10 ", Proxied: true, TTL: 300})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Name != "www.example.com" || rec.Content != "203.0.113.10" || !rec.Proxied || rec.TTL != 1 {
		t.Fatalf("created %+v", rec)
	}
	mx, err := m.CreateDNSRecord(ctx, zoneCom, RecordInput{Type: "MX", Name: "@", Content: "Mail.Example.com.", Priority: 10, TTL: 3600})
	if err != nil {
		t.Fatal(err)
	}
	if mx.Name != "example.com" || mx.Content != "mail.example.com" || mx.Priority == nil || *mx.Priority != 10 || mx.TTL != 3600 {
		t.Fatalf("mx %+v", mx)
	}
	caa, err := m.CreateDNSRecord(ctx, zoneCom, RecordInput{Type: "CAA", Name: "example.com", Data: &CAAData{Tag: "ISSUE", Value: "letsencrypt.org"}})
	if err != nil {
		t.Fatal(err)
	}
	if caa.Data == nil || caa.Data.Tag != "issue" || caa.Content != `0 issue "letsencrypt.org"` {
		t.Fatalf("caa %+v", caa)
	}

	old, upd, err := m.UpdateDNSRecord(ctx, zoneCom, rec.ID, RecordInput{Type: "A", Name: "www", Content: "203.0.113.11", TTL: 120})
	if err != nil {
		t.Fatal(err)
	}
	if old.Content != "203.0.113.10" || upd.Content != "203.0.113.11" || upd.Proxied || upd.TTL != 120 {
		t.Fatalf("update %+v → %+v", old, upd)
	}
	if code, _ := errCode(func() error {
		_, _, err := m.UpdateDNSRecord(ctx, zoneCom, rec.ID, RecordInput{Type: "CNAME", Name: "www", Content: "example.com"})
		return err
	}()); code != "dns_type_change" {
		t.Fatalf("type change: %s", code)
	}

	// Records of other types are read-only.
	ns := fake.add(zoneCom, "NS", "sub.example.com", "ns1.other.net", false)
	if _, err := m.DeleteDNSRecord(ctx, zoneCom, ns); err == nil {
		t.Fatal("deleted an NS record")
	} else if code, detail := errCode(err); code != "dns_type_unsupported" || detail != "NS" {
		t.Fatalf("NS: %s %s", code, detail)
	}
	gone, err := m.DeleteDNSRecord(ctx, zoneCom, rec.ID)
	if err != nil || gone.Name != "www.example.com" {
		t.Fatalf("delete: %v %+v", err, gone)
	}
	if got := fake.list("A", "www.example.com"); len(got) != 0 {
		t.Fatalf("left %v", got)
	}
	// Cloudflare's own refusals carry its message.
	if _, err := m.DeleteDNSRecord(ctx, zoneCom, rec.ID); err == nil {
		t.Fatal("deleted twice")
	} else if code, detail := errCode(err); code != "cloudflare_error" || !strings.Contains(detail, "Record does not exist. (81044)") {
		t.Fatalf("second delete: %s %s", code, detail)
	}
}

func TestRecordBody(t *testing.T) {
	long := strings.Repeat("a", 256)
	cases := []struct {
		in   RecordInput
		code string // "" for valid
	}{
		{RecordInput{Type: "A", Name: "@", Content: "203.0.113.5"}, ""},
		{RecordInput{Type: "A", Name: "*", Content: "203.0.113.5", Proxied: true}, ""},
		{RecordInput{Type: "A", Name: "x", Content: "2001:db8::1"}, "dns_invalid_ipv4"},
		{RecordInput{Type: "A", Name: "x", Content: "203.0.113"}, "dns_invalid_ipv4"},
		{RecordInput{Type: "AAAA", Name: "x", Content: "2001:db8::1"}, ""},
		{RecordInput{Type: "AAAA", Name: "x", Content: "203.0.113.5"}, "dns_invalid_ipv6"},
		{RecordInput{Type: "AAAA", Name: "x", Content: "::ffff:203.0.113.5"}, "dns_invalid_ipv6"},
		{RecordInput{Type: "AAAA", Name: "x", Content: "fe80::1%eth0"}, "dns_invalid_ipv6"},
		{RecordInput{Type: "CNAME", Name: "www", Content: "example.com.", Proxied: true}, ""},
		{RecordInput{Type: "CNAME", Name: "www", Content: "www.example.com"}, "dns_cname_self"},
		{RecordInput{Type: "CNAME", Name: "www", Content: "not a host"}, "dns_invalid_target"},
		{RecordInput{Type: "CNAME", Name: "www", Content: "localhost"}, "dns_invalid_target"},
		{RecordInput{Type: "MX", Name: "@", Content: "mail.example.com", Priority: 65535}, ""},
		{RecordInput{Type: "MX", Name: "@", Content: "mail.example.com", Priority: 65536}, "dns_invalid_priority"},
		{RecordInput{Type: "MX", Name: "@", Content: "mail.example.com", Priority: -1}, "dns_invalid_priority"},
		{RecordInput{Type: "MX", Name: "@", Content: "203.0.113.5:25"}, "dns_invalid_mail_server"},
		{RecordInput{Type: "MX", Name: "@", Content: "mail.example.com", Proxied: true}, "dns_proxied_not_allowed"},
		{RecordInput{Type: "TXT", Name: "_dmarc", Content: "v=DMARC1; p=none"}, ""},
		{RecordInput{Type: "TXT", Name: "@", Content: `"v=spf1 -all"`}, ""},
		{RecordInput{Type: "TXT", Name: "@", Content: `"part one" "part \"two\""`}, ""},
		{RecordInput{Type: "TXT", Name: "@", Content: strings.Repeat("k", 2000)}, ""}, // Cloudflare splits it
		{RecordInput{Type: "TXT", Name: "@", Content: strings.Repeat("k", 2049)}, "dns_txt_too_long"},
		{RecordInput{Type: "TXT", Name: "@", Content: `"` + long + `"`}, "dns_txt_string_long"},
		{RecordInput{Type: "TXT", Name: "@", Content: `"unclosed`}, "dns_txt_quotes"},
		{RecordInput{Type: "TXT", Name: "@", Content: `say "hi"`}, "dns_txt_quotes"},
		{RecordInput{Type: "TXT", Name: "@", Content: `"a"b`}, "dns_txt_quotes"},
		{RecordInput{Type: "TXT", Name: "@", Content: "two\nlines"}, "dns_txt_invalid"},
		{RecordInput{Type: "TXT", Name: "@", Content: "  "}, "dns_txt_empty"},
		{RecordInput{Type: "TXT", Name: "@", Content: "v=1", Proxied: true}, "dns_proxied_not_allowed"},
		{RecordInput{Type: "CAA", Name: "@", Data: &CAAData{Flags: 0, Tag: "issue", Value: "letsencrypt.org"}}, ""},
		{RecordInput{Type: "CAA", Name: "@", Data: &CAAData{Flags: 128, Tag: "issuewild", Value: "letsencrypt.org; validationmethods=dns-01"}}, ""},
		{RecordInput{Type: "CAA", Name: "@", Data: &CAAData{Tag: "issue", Value: ";"}}, ""},
		{RecordInput{Type: "CAA", Name: "@", Data: &CAAData{Tag: "iodef", Value: "mailto:security@example.com"}}, ""},
		{RecordInput{Type: "CAA", Name: "@", Data: &CAAData{Tag: "iodef", Value: "https://example.com/caa"}}, ""},
		{RecordInput{Type: "CAA", Name: "@", Data: &CAAData{Tag: "iodef", Value: "security@example.com"}}, "dns_invalid_caa_value"},
		{RecordInput{Type: "CAA", Name: "@", Data: &CAAData{Flags: 1, Tag: "issue", Value: "letsencrypt.org"}}, "dns_invalid_caa_flags"},
		{RecordInput{Type: "CAA", Name: "@", Data: &CAAData{Tag: "tbs", Value: "x"}}, "dns_invalid_caa_tag"},
		{RecordInput{Type: "CAA", Name: "@", Data: &CAAData{Tag: "issue", Value: `"letsencrypt.org"`}}, "dns_invalid_caa_value"},
		{RecordInput{Type: "CAA", Name: "@", Data: &CAAData{Tag: "issue", Value: ""}}, "dns_invalid_caa_value"},
		{RecordInput{Type: "CAA", Name: "@", Data: &CAAData{Tag: "issue", Value: "letsencrypt.org"}, Proxied: true}, "dns_proxied_not_allowed"},
		{RecordInput{Type: "CAA", Name: "@"}, "dns_invalid_caa_tag"},
		{RecordInput{Type: "SRV", Name: "_sip._tcp", Content: "x"}, "dns_type_unsupported"},
		{RecordInput{Type: "NS", Name: "sub", Content: "ns1.example.net"}, "dns_type_unsupported"},
		{RecordInput{Type: "A", Name: "bad name", Content: "203.0.113.5"}, "dns_invalid_name"},
		{RecordInput{Type: "A", Name: "a.*.b", Content: "203.0.113.5"}, "dns_invalid_name"},
		{RecordInput{Type: "A", Name: "x", Content: "203.0.113.5", TTL: 30}, "dns_invalid_ttl"},
		{RecordInput{Type: "A", Name: "x", Content: "203.0.113.5", TTL: 90000}, "dns_invalid_ttl"},
		{RecordInput{Type: "A", Name: "x", Content: "203.0.113.5", TTL: 30, Proxied: true}, ""}, // proxied: automatic
	}
	for _, c := range cases {
		_, err := recordBody("example.com", c.in)
		if code, detail := errCode(err); code != c.code {
			t.Errorf("%+v: got %q (%s), want %q", c.in, code, detail, c.code)
		}
	}

	body, _ := recordBody("example.com", RecordInput{Type: "TXT", Name: "x", Content: "v=1"})
	if _, ok := body["proxied"]; ok {
		t.Error("proxied sent for TXT")
	}
	body, _ = recordBody("example.com", RecordInput{Type: "AAAA", Name: "x", Content: "2001:DB8:0::1", Proxied: true, TTL: 600})
	if body["content"] != "2001:db8::1" || body["proxied"] != true || body["ttl"] != 1 || body["name"] != "x.example.com" {
		t.Errorf("AAAA body %v", body)
	}
	for in, want := range map[string]string{"@": "example.com", "": "example.com", "WWW": "www.example.com",
		"www.example.com.": "www.example.com", "example.com": "example.com", "*.shop": "*.shop.example.com",
		"_dmarc": "_dmarc.example.com", "www.example.org": "www.example.org.example.com"} {
		if got, err := recordName("example.com", in); err != nil || got != want {
			t.Errorf("recordName(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

func TestDNSPermissions(t *testing.T) {
	ctx := context.Background()
	m, fake := dnsTest(t, "read-token")
	id := fake.add(zoneCom, "A", "www.example.com", "203.0.113.1", false)
	if _, err := m.DNSRecords(ctx, zoneCom); err != nil {
		t.Fatalf("reading: %v", err)
	}
	check := func(what string, err error, code, detail string) {
		t.Helper()
		if c, d := errCode(err); c != code || d != detail {
			t.Errorf("%s: got %s %q, want %s %q", what, c, d, code, detail)
		}
	}
	_, err := m.CreateDNSRecord(ctx, zoneCom, RecordInput{Type: "A", Name: "x", Content: "203.0.113.2"})
	check("create", err, "cloudflare_permission", "Zone → DNS → Edit")
	_, _, err = m.UpdateDNSRecord(ctx, zoneCom, id, RecordInput{Type: "A", Name: "www", Content: "203.0.113.2"})
	check("update", err, "cloudflare_permission", "Zone → DNS → Edit")
	_, err = m.DeleteDNSRecord(ctx, zoneCom, id)
	check("delete", err, "cloudflare_permission", "Zone → DNS → Edit")
	_, _, err = m.PurgeCache(ctx, zoneCom, true, nil)
	check("purge", err, "cloudflare_permission", "Zone → Cache Purge → Purge")

	m.store.SetSetting(ctx, CloudflareTokenKey, "no-zones-token")
	_, err = m.DNSZones(ctx)
	check("zones", err, "cloudflare_permission", "Zone → Zone → Read")
	m.store.SetSetting(ctx, CloudflareTokenKey, "wrong-token")
	_, err = m.DNSZones(ctx)
	check("bad token", err, "cloudflare_token_invalid", "Invalid API Token (1000)")
	m.store.SetSetting(ctx, CloudflareTokenKey, "")
	_, err = m.DNSZones(ctx)
	check("no token", err, "cloudflare_not_configured", "")

	// The token never appears in an error.
	m.store.SetSetting(ctx, CloudflareTokenKey, "secret-token-123")
	if _, err := m.DNSRecords(ctx, zoneCom); err == nil || strings.Contains(err.Error(), "secret-token-123") {
		t.Fatalf("error = %v", err)
	}
}

func TestDNSPurge(t *testing.T) {
	ctx := context.Background()
	m, fake := dnsTest(t, "good-token")
	if zone, n, err := m.PurgeCache(ctx, zoneCom, true, nil); err != nil || zone != "example.com" || n != 0 {
		t.Fatalf("purge everything: %v %s %d", err, zone, n)
	}
	urls := []string{"https://example.com/style.css", "", "http://www.example.com/a?b=1", "https://example.com/style.css"}
	if _, n, err := m.PurgeCache(ctx, zoneCom, false, urls); err != nil || n != 2 {
		t.Fatalf("purge files: %v %d", err, n)
	}
	if len(fake.purges) != 2 || fake.purges[0]["purge_everything"] != true || fake.purges[0]["zone_id"] != zoneCom {
		t.Fatalf("purges %v", fake.purges)
	}
	if files, _ := fake.purges[1]["files"].([]any); len(files) != 2 || files[1] != "http://www.example.com/a?b=1" {
		t.Fatalf("files %v", fake.purges[1])
	}
	many := []string{}
	for i := 0; i <= maxPurgeURLs; i++ {
		many = append(many, fmt.Sprintf("https://example.com/%d", i))
	}
	for _, c := range []struct {
		urls []string
		code string
	}{
		{[]string{"https://example.org/x"}, "dns_url_not_in_zone"},
		{[]string{"https://notexample.com/x"}, "dns_url_not_in_zone"},
		{[]string{"ftp://example.com/x"}, "dns_invalid_url"},
		{[]string{"/style.css"}, "dns_invalid_url"},
		{[]string{"https://user:pw@example.com/"}, "dns_invalid_url"},
		{[]string{" ", ""}, "dns_no_urls"},
		{many, "dns_too_many_urls"},
	} {
		if _, _, err := m.PurgeCache(ctx, zoneCom, false, c.urls); err == nil {
			t.Errorf("%v accepted", c.urls)
		} else if code, _ := errCode(err); code != c.code {
			t.Errorf("%v: %s, want %s", c.urls, code, c.code)
		}
	}
	if len(fake.purges) != 2 {
		t.Fatalf("refused purges reached Cloudflare: %v", fake.purges)
	}
}

func TestServerAddrsDetect(t *testing.T) {
	p := netip.MustParseAddr
	for _, c := range []struct {
		addrs            ServerAddrs
		v4, v6, private4 string
	}{
		{ServerAddrs{Route4: "203.0.113.10", Route6: "2a01:4f8::10", Local: []netip.Addr{p("172.17.0.1"), p("203.0.113.10")}}, "203.0.113.10", "2a01:4f8::10", ""},
		// Behind NAT: only private and CGNAT addresses.
		{ServerAddrs{Route4: "10.0.0.5", Local: []netip.Addr{p("10.0.0.5"), p("100.64.1.2"), p("fd00::5")}}, "", "", "10.0.0.5"},
		// No default route: the interfaces' public address.
		{ServerAddrs{Local: []netip.Addr{p("192.168.1.2"), p("198.51.100.20"), p("2001:db8:1::20")}}, "198.51.100.20", "2001:db8:1::20", ""},
	} {
		v4, v6, priv := c.addrs.detect()
		if v4 != c.v4 || v6 != c.v6 || priv != c.private4 {
			t.Errorf("%+v: got %q %q %q", c.addrs, v4, v6, priv)
		}
	}
}

func TestPointDNS(t *testing.T) {
	ctx := context.Background()
	m, fake := dnsTest(t, "good-token")
	site := &store.Site{Domain: "example.com", SysUser: "example_com", Aliases: []string{
		"www.example.com", "shop.example.com", "api.example.com", "cdn.example.com", "*.example.com", "example.net"}}
	if err := m.store.CreateSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	fake.add(zoneCom, "A", "example.com", "198.51.100.7", false)      // the old server: updated
	fake.add(zoneCom, "AAAA", "example.com", "2001:db8::7", false)    // updated
	fake.add(zoneCom, "MX", "example.com", "mail.example.com", false) // another type: kept
	fake.add(zoneCom, "CNAME", "www.example.com", "example.com", false)
	fake.add(zoneCom, "A", "api.example.com", "198.51.100.8", false) // deleted
	fake.add(zoneCom, "A", "api.example.com", "203.0.113.10", false) // already here: kept
	fake.add(zoneCom, "CNAME", "cdn.example.com", "x.netlify.app", true)
	fake.add(zoneCom, "A", "blog.example.com", "198.51.100.9", false) // not a name of the site
	addrs := ServerAddrs{Route4: "203.0.113.10", Route6: "2a01:4f8::10", Local: []netip.Addr{netip.MustParseAddr("203.0.113.10"), netip.MustParseAddr("2a01:4f8::10")}}

	plan, err := m.PlanPointDNS(ctx, site.ID, PointOptions{}, addrs)
	if err != nil {
		t.Fatal(err)
	}
	if plan.IPv4 != "203.0.113.10" || plan.IPv6 != "2a01:4f8::10" || len(plan.Zones) != 1 || plan.Hash == "" {
		t.Fatalf("plan %+v", plan)
	}
	if len(plan.Skipped) != 1 || plan.Skipped[0].Name != "example.net" || plan.Skipped[0].Reason != "no_zone" {
		t.Fatalf("skipped %+v", plan.Skipped)
	}
	got := map[string]int{}
	var lines []string
	for _, ch := range plan.Changes {
		got[ch.Action]++
		lines = append(lines, fmt.Sprintf("%s %s %s %s→%s", ch.Action, ch.Type, ch.Name, ch.Old, ch.New))
	}
	sort.Strings(lines)
	want := []string{
		"create A *.example.com →203.0.113.10",
		"create A cdn.example.com →203.0.113.10",
		"create A shop.example.com →203.0.113.10",
		"create AAAA *.example.com →2a01:4f8::10",
		"create AAAA api.example.com →2a01:4f8::10",
		"create AAAA cdn.example.com →2a01:4f8::10",
		"create AAAA shop.example.com →2a01:4f8::10",
		"delete A api.example.com 198.51.100.8→",
		"delete CNAME cdn.example.com x.netlify.app→",
		"keep A api.example.com 203.0.113.10→203.0.113.10",
		"keep CNAME www.example.com example.com→example.com",
		"update A example.com 198.51.100.7→203.0.113.10",
		"update AAAA example.com 2001:db8::7→2a01:4f8::10",
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Fatalf("plan:\n%s", strings.Join(lines, "\n"))
	}

	// A plan the admin did not see is refused.
	if _, err := m.ApplyPointDNS(ctx, site.ID, PointOptions{}, addrs, "stale", func(PointChange) {}); err == nil {
		t.Fatal("applied a stale plan")
	} else if code, _ := errCode(err); code != "dns_plan_changed" {
		t.Fatalf("stale: %v", err)
	}
	var done []string
	n, err := m.ApplyPointDNS(ctx, site.ID, PointOptions{}, addrs, plan.Hash, func(ch PointChange) { done = append(done, ch.Action) })
	if err != nil {
		t.Fatal(err)
	}
	if n != 11 || len(done) != 11 || done[0] != "delete" || done[len(done)-1] != "create" {
		t.Fatalf("applied %d: %v", n, done)
	}
	for _, c := range []struct{ typ, name, want string }{
		{"A", "example.com", "203.0.113.10"},
		{"AAAA", "example.com", "2a01:4f8::10"},
		{"MX", "example.com", "mail.example.com"},
		{"CNAME", "www.example.com", "example.com"},
		{"A", "www.example.com", ""},
		{"A", "api.example.com", "203.0.113.10"},
		{"A", "cdn.example.com", "203.0.113.10"},
		{"CNAME", "cdn.example.com", ""},
		{"A", "*.example.com", "203.0.113.10"},
		{"A", "blog.example.com", "198.51.100.9"},
	} {
		if got := strings.Join(fake.list(c.typ, c.name), ","); got != c.want {
			t.Errorf("%s %s = %q, want %q", c.typ, c.name, got, c.want)
		}
	}

	// Again: nothing to do.
	plan, err = m.PlanPointDNS(ctx, site.ID, PointOptions{}, addrs)
	if err != nil {
		t.Fatal(err)
	}
	for _, ch := range plan.Changes {
		if ch.Action != "keep" {
			t.Errorf("second plan: %+v", ch)
		}
	}

	// Proxied: every kept record is updated, the CNAME too.
	plan, err = m.PlanPointDNS(ctx, site.ID, PointOptions{Proxied: true}, addrs)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.ApplyPointDNS(ctx, site.ID, PointOptions{Proxied: true}, addrs, plan.Hash, func(PointChange) {}); err != nil {
		t.Fatal(err)
	}
	if got := fake.list("CNAME", "www.example.com"); len(got) != 1 || got[0] != "example.com proxied" {
		t.Fatalf("www = %v", got)
	}
	if got := fake.list("A", "shop.example.com"); len(got) != 1 || got[0] != "203.0.113.10 proxied" {
		t.Fatalf("shop = %v", got)
	}

	// Without IPv6: AAAA records that point elsewhere go; this server's stay.
	fake.add(zoneCom, "AAAA", "shop.example.com", "2001:db8::99", false)
	plan, err = m.PlanPointDNS(ctx, site.ID, PointOptions{NoIPv6: true, Proxied: true}, addrs)
	if err != nil {
		t.Fatal(err)
	}
	if plan.IPv6 != "" {
		t.Fatalf("ipv6 %q", plan.IPv6)
	}
	var shop6 []string
	for _, ch := range plan.Changes {
		if ch.Type == "AAAA" && ch.Name == "shop.example.com" {
			shop6 = append(shop6, ch.Action+" "+ch.Old)
		}
	}
	sort.Strings(shop6)
	if strings.Join(shop6, ",") != "delete 2001:db8::99,keep 2a01:4f8::10" {
		t.Fatalf("shop AAAA: %v", shop6)
	}
}

func TestPointDNSAddresses(t *testing.T) {
	ctx := context.Background()
	m, _ := dnsTest(t, "good-token")
	site := &store.Site{Domain: "example.com", SysUser: "example_com"}
	if err := m.store.CreateSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	nat := ServerAddrs{Route4: "10.0.0.5", Local: []netip.Addr{netip.MustParseAddr("10.0.0.5")}}
	check := func(opts PointOptions, code, detail string) {
		t.Helper()
		_, err := m.PlanPointDNS(ctx, site.ID, opts, nat)
		if c, d := errCode(err); c != code || d != detail {
			t.Errorf("%+v: got %s %q, want %s %q", opts, c, d, code, detail)
		}
	}
	check(PointOptions{}, "dns_need_ipv4", "10.0.0.5")
	check(PointOptions{IPv4: "10.0.0.5"}, "dns_ip_not_public", "10.0.0.5")
	check(PointOptions{IPv4: "2001:db8::1"}, "dns_invalid_ipv4", "2001:db8::1")
	check(PointOptions{IPv4: "203.0.113.10", IPv6: "fd00::1"}, "dns_ip_not_public", "fd00::1")
	check(PointOptions{IPv4: "203.0.113.10", IPv6: "fd00::1", NoIPv6: true}, "", "")
	plan, err := m.PlanPointDNS(ctx, site.ID, PointOptions{IPv4: "203.0.113.10"}, nat)
	if err != nil {
		t.Fatal(err)
	}
	if plan.IPv6 != "" || len(plan.Changes) != 1 || plan.Changes[0].Action != "create" || plan.Changes[0].Type != "A" {
		t.Fatalf("plan %+v", plan)
	}

	// A site none of whose names is in a zone the token can see.
	other := &store.Site{Domain: "example.net", SysUser: "example_net", Aliases: []string{"www.example.net"}}
	m.store.CreateSite(ctx, other)
	if _, err := m.PlanPointDNS(ctx, other.ID, PointOptions{IPv4: "203.0.113.10"}, nat); err == nil {
		t.Fatal("planned without a zone")
	} else if code, detail := errCode(err); code != "dns_no_zone" || detail != "example.net, www.example.net" {
		t.Fatalf("no zone: %s %s", code, detail)
	}
}
