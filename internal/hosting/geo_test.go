package hosting

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/geoip"
	"github.com/nefocp/nefo/internal/store"
)

// testGeoCSV builds a database in DB-IP's format that covers every
// address: n IPv4 and n IPv6 ranges cycling through the countries, with
// 1.0.1.0/24 in CN, 8.8.8.0/24 in US, 14.160.0.0/16 in VN and 10.0.0.0/8
// reserved (ZZ).
func testGeoCSV(n int) []byte {
	var b bytes.Buffer
	cc := geoip.Countries()
	fixed := map[string][2]uint32{"CN": {0x01000100, 0x010001ff}, "US": {0x08080800, 0x080808ff}, "VN": {0x0ea00000, 0x0ea0ffff}, "ZZ": {0x0a000000, 0x0affffff}}
	v4 := func(x uint32) netip.Addr {
		var a [4]byte
		binary.BigEndian.PutUint32(a[:], x)
		return netip.AddrFrom4(a)
	}
	// The fixed ranges, then filler between them.
	type rr struct {
		lo, hi uint32
		cc     string
	}
	var list []rr
	for c, r := range fixed {
		list = append(list, rr{r[0], r[1], c})
	}
	sortRR := func() {
		for i := range list {
			for j := i + 1; j < len(list); j++ {
				if list[j].lo < list[i].lo {
					list[i], list[j] = list[j], list[i]
				}
			}
		}
	}
	sortRR()
	step := uint32((1 << 32) / uint64(n))
	cur := uint32(0)
	i := 0
	emit := func(lo, hi uint32, c string) { fmt.Fprintf(&b, "%s,%s,%s\n", v4(lo), v4(hi), c) }
	for _, f := range list {
		for cur < f.lo {
			hi := cur + step - 1
			if hi >= f.lo || hi < cur {
				hi = f.lo - 1
			}
			emit(cur, hi, cc[i%len(cc)])
			i++
			cur = hi + 1
		}
		emit(f.lo, f.hi, f.cc)
		cur = f.hi + 1
	}
	for {
		hi := cur + step - 1
		if hi < cur {
			hi = 1<<32 - 1
		}
		emit(cur, hi, cc[i%len(cc)])
		i++
		if hi == 1<<32-1 {
			break
		}
		cur = hi + 1
	}
	v6 := func(x uint64) netip.Addr {
		var a [16]byte
		binary.BigEndian.PutUint64(a[:8], x)
		return netip.AddrFrom16(a)
	}
	step6 := uint64(1<<64-1) / uint64(n)
	for j := 0; j < n; j++ {
		lo := v6(uint64(j) * step6)
		hi := netip.MustParseAddr("ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff")
		if j < n-1 {
			a := v6(uint64(j+1)*step6 - 1).As16()
			for k := 8; k < 16; k++ {
				a[k] = 0xff
			}
			hi = netip.AddrFrom16(a)
		}
		fmt.Fprintf(&b, "%s,%s,%s\n", lo, hi, cc[(j+3)%len(cc)])
	}
	return b.Bytes()
}

func gz(b []byte) []byte {
	var out bytes.Buffer
	w := gzip.NewWriter(&out)
	w.Write(b)
	w.Close()
	return out.Bytes()
}

// geoTestManager is a Nginx manager with a stored country database.
func geoTestManager(t *testing.T) *Manager {
	t.Helper()
	m := newTestManager(t)
	m.run = &fakeRunner{outputs: map[string]fakeResult{"nginx": {out: "nginx version: nginx/1.24.0\n"}}}
	m.paths.GeoIP = t.TempDir()
	m.paths.Nginx = t.TempDir()
	if err := m.setServerKind(context.Background(), ServerNginx); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(m.geoDataFile("csv"), gz(testGeoCSV(60000)), 0o644)
	meta := map[string]*geoMeta{"csv": {Month: "2026-11", Format: "csv", Fetched: time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)}}
	if err := m.writeGeoMeta(meta); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestGeoFilesNginx(t *testing.T) {
	m := geoTestManager(t)
	ctx := context.Background()
	prot := []*store.SiteProtect{
		{SiteID: 1, GeoMode: GeoBlock, GeoCountries: []string{"CN"}},
		{SiteID: 2, GeoMode: GeoAllow, GeoCountries: []string{"VN"}},
		{SiteID: 3, GeoMode: GeoBlock}, // no countries: off
	}
	files, remove, err := m.geoFiles(ctx, prot)
	if err != nil {
		t.Fatal(err)
	}
	if len(remove) != 0 || len(files) != 3 {
		t.Fatalf("files %d, remove %v", len(files), remove)
	}
	conf := string(files[m.nginxGeoConf()])
	mustContain(t, conf, "data=2026-11 countries=CN,VN,ZZ", "geo $panel_geo_cf {", "173.245.48.0/20 1;",
		"1       $http_cf_connecting_ip;", "geo $panel_geo_addr $panel_geo_v4 {", "include "+m.nginxGeoList("v4")+";",
		"include "+m.nginxGeoList("v6")+";",
		`"~^(\S+) \1$" 1;`, `"~^0(?<panel_cc>[A-Z]{2})$" $panel_cc;`, "IP Geolocation by DB-IP (2026-11, CC BY 4.0)")
	// The synthetic data is many small ranges: "ranges" mode is smaller.
	if !strings.Contains(conf, "ranges=1") || !strings.Contains(conf, "    ranges;\n") {
		t.Fatalf("not in ranges mode:\n%s", conf)
	}
	v4 := string(files[m.nginxGeoList("v4")])
	mustContain(t, v4, "1.0.1.0-1.0.1.255 CN;\n", "14.160.0.0-14.160.255.255 VN;\n", "10.0.0.0-10.255.255.255 ZZ;\n")
	mustNotContain(t, v4, " US;", "224.0.0.0", "255.255.255.255")
	for _, l := range strings.Split(strings.TrimSpace(string(files[m.nginxGeoList("v6")])), "\n") {
		if !strings.HasPrefix(l, "#") && !regexp.MustCompile(`^[0-9a-f:]+/[0-9]+ (CN|VN|ZZ);$`).MatchString(l) {
			t.Fatalf("v6 line %q", l)
		}
	}
	if geoHeaderInt([]byte(conf), "memory_kb") <= 0 {
		t.Fatal("no memory estimate")
	}
	// Written, the same settings build nothing new.
	for p, d := range files {
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, d, 0o644)
	}
	again, _, err := m.geoFiles(ctx, prot)
	if err != nil || !bytes.Equal(again[m.nginxGeoList("v4")], files[m.nginxGeoList("v4")]) {
		t.Fatalf("again: %v", err)
	}
	// No site left: the three files go.
	files, remove, err = m.geoFiles(ctx, prot[2:])
	if err != nil || len(files) != 0 || len(remove) != 3 {
		t.Fatalf("off: %d files, remove %v, %v", len(files), remove, err)
	}
}

func TestGeoFilesApache(t *testing.T) {
	m := geoTestManager(t)
	m.paths.Apache = t.TempDir()
	ctx := context.Background()
	if err := m.setServerKind(ctx, ServerApache); err != nil {
		t.Fatal(err)
	}
	files, _, err := m.geoFiles(ctx, []*store.SiteProtect{{SiteID: 1, GeoMode: GeoAllow, GeoCountries: []string{"VN"}}})
	if err != nil {
		t.Fatal(err)
	}
	vn := string(files[m.apacheGeoFile("VN")])
	mustContain(t, vn, "Require ip ", " 14.160.0.0/16")
	if zz := string(files[m.apacheGeoFile("ZZ")]); !strings.Contains(zz, "10.0.0.0/8") {
		t.Fatalf("ZZ:\n%s", zz)
	}
	if n := m.apacheGeoPrefixes([]string{"VN", "ZZ"}); n < 2 {
		t.Fatalf("prefixes %d", n)
	}
}

func TestGeoVhost(t *testing.T) {
	m := geoTestManager(t)
	ctx := context.Background()
	site := &store.Site{Domain: "geo.example.com", SysUser: "geo_example_com"}
	if err := m.store.CreateSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	var d vhostData
	m.geoVhost(ctx, site, &d)
	if d.GeoMode != "" {
		t.Fatal("geo without settings")
	}
	m.store.SetSiteProtect(ctx, &store.SiteProtect{SiteID: site.ID, GeoMode: GeoAllow, GeoCountries: []string{"VN", "US"}})
	m.geoVhost(ctx, site, &d)
	if d.GeoMode != GeoAllow || d.GeoRegex != "US|VN|ZZ" || len(d.GeoFiles) != 0 {
		t.Fatalf("%+v", d)
	}
	d.Domain, d.Public, d.LogDir, d.SnippetDir = site.Domain, "/var/www/x/public", "/var/log/nginx", "/etc/nginx/panel"
	out, err := renderVhost(d)
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, string(out), `if ($panel_country !~ "^(?:US|VN|ZZ)$") {`, "return 403;",
		`if ($uri ~ "^/(?:\.well-known/acme-challenge/|\.panel/deploy/|403\.html$)") {`)
	// A value that could inject directives is refused.
	d.GeoRegex = `VN") { return 200; } #`
	if _, err := renderVhost(d); err == nil {
		t.Fatal("bad regex rendered")
	}
}

func TestGeoDownload(t *testing.T) {
	csv := gz(testGeoCSV(60000))
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		switch r.URL.Path {
		case "/dbip-country-lite-2026-10.csv.gz":
			w.Write(csv)
		case "/bad-2026-11.csv.gz":
			w.Write(gz([]byte(strings.Repeat("<html>not found</html>\n", 100000))))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	oldURL, oldMin := geoCSVURL, geoMinDownload
	defer func() { geoCSVURL, geoMinDownload = oldURL, oldMin }()
	geoCSVURL, geoMinDownload = srv.URL+"/dbip-country-lite-%s.csv.gz", 1000

	m := newTestManager(t)
	m.paths.GeoIP = filepath.Join(t.TempDir(), "geo")
	// Early in November the file of October is the newest.
	meta, err := m.geoDownload(context.Background(), "csv", time.Date(2026, 11, 1, 2, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if meta.Month != "2026-10" || meta.Stats.V4 < 50000 || meta.Stats.Countries != 250 || len(asked) != 2 {
		t.Fatalf("%+v asked %v", meta, asked)
	}
	if st, err := os.Stat(m.paths.GeoIP); err != nil || (runtime.GOOS == "linux" && st.Mode().Perm() != 0o755) {
		t.Fatalf("folder %v %v", st, err)
	}
	geoCSVURL = srv.URL + "/bad-%s.csv.gz"
	if _, err := m.geoDownload(context.Background(), "csv", time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("HTML accepted as the database")
	}
	// The stored file is still the good one.
	b, _ := os.ReadFile(m.geoDataFile("csv"))
	if !bytes.Equal(b, csv) {
		t.Fatal("stored file replaced by a bad download")
	}
}

func TestNormalizeCountries(t *testing.T) {
	got, err := normalizeCountries([]string{" vn", "CN", "VN", ""})
	if err != nil || strings.Join(got, ",") != "CN,VN" {
		t.Fatalf("%v %v", got, err)
	}
	for _, bad := range [][]string{{"ZZ"}, {"EU"}, {"V N"}, {"VNM"}} {
		if _, err := normalizeCountries(bad); err == nil {
			t.Errorf("accepted %v", bad)
		}
	}
	many := geoip.Countries()[:101]
	if _, err := normalizeCountries(many); err == nil {
		t.Error("101 countries accepted")
	}
}

func TestAddrRegex(t *testing.T) {
	re := regexp.MustCompile(`^(?:` + addrRegex(geoip.Cloudflare) + `)$`)
	for _, ok := range []string{"173.245.48.1", "173.245.63.255", "104.16.0.1", "104.23.255.254", "162.159.1.1", "198.41.200.3",
		"2400:cb00:2048:1::c629:d7a2", "2606:4700::6810:84e5", "2a06:98c7:ffff::1", "2c0f:f248::"} {
		if !re.MatchString(ok) {
			t.Errorf("%s not matched", ok)
		}
	}
	for _, no := range []string{"173.245.64.0", "173.245.47.255", "104.24.0.0x", "104.15.255.255", "1.2.3.4", "1173.245.48.1",
		"2400:cb01::1", "2a06:98c8::1", "2606:47000::1", "x2606:4700::1"} {
		if re.MatchString(no) {
			t.Errorf("%s matched", no)
		}
	}
	self := regexp.MustCompile(`^(?:` + addrRegex([]string{"203.0.113.10/32", "2001:db8::10/128"}) + `)$`)
	if !self.MatchString("203.0.113.10") || !self.MatchString("2001:db8::10") || self.MatchString("203.0.113.1") || self.MatchString("203x0.113.10") {
		t.Error("single addresses")
	}
	if !olsAddrRegexRe.MatchString(addrRegex(geoip.Cloudflare)) {
		t.Error("the Cloudflare regex has characters the template refuses")
	}
}

func TestDigitRange(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 400; i++ {
		lo, hi := r.Intn(256), r.Intn(256)
		if lo > hi {
			lo, hi = hi, lo
		}
		base := 10
		top := 255
		if i%2 == 1 {
			base, top = 16, 0xffff
			lo, hi = r.Intn(top+1), r.Intn(top+1)
			if lo > hi {
				lo, hi = hi, lo
			}
		}
		re := regexp.MustCompile(`^` + digitRange(lo, hi, base) + `$`)
		for v := 0; v <= top; v += 1 + top/600 {
			s := strconv.FormatInt(int64(v), base)
			if re.MatchString(s) != (v >= lo && v <= hi) {
				t.Fatalf("base %d range %d-%d: %s → %v (%s)", base, lo, hi, s, re.MatchString(s), digitRange(lo, hi, base))
			}
		}
		for _, v := range []int{lo, hi} {
			if !re.MatchString(strconv.FormatInt(int64(v), base)) {
				t.Fatalf("range %d-%d misses %d", lo, hi, v)
			}
		}
	}
}

func TestGeoMetaRoundTrip(t *testing.T) {
	m := newTestManager(t)
	m.paths.GeoIP = t.TempDir()
	if m.geoReady() {
		t.Fatal("ready without a database")
	}
	m.writeGeoMeta(map[string]*geoMeta{"csv": {Month: "2026-11"}})
	os.WriteFile(m.geoDataFile("csv"), []byte("x"), 0o644)
	if !m.geoReady() {
		t.Fatal("not ready")
	}
	b, _ := os.ReadFile(filepath.Join(m.paths.GeoIP, geoMetaFile))
	var back map[string]geoMeta
	if json.Unmarshal(b, &back) != nil || back["csv"].Month != "2026-11" {
		t.Fatalf("%s", b)
	}
}

func TestGeoRenderChecks(t *testing.T) {
	d := baseVhost()
	d.PHPSocket = "/run/php/panel-example_com.sock"
	d.GeoMode, d.GeoRegex = GeoBlock, "CN"
	// OpenLiteSpeed needs Cloudflare's addresses to tell its requests apart.
	v := olsVhost{vhostData: d, ErrorDir: "/usr/local/lsws/panel-errors"}
	v.LogDir, v.EmptyRoot, v.User = "/var/log/openlitespeed", "/var/lib/panel-acme", "example_com"
	if _, err := renderOLSVhost(v, ""); err == nil {
		t.Fatal("OpenLiteSpeed rules rendered without Cloudflare's addresses")
	}
	v.GeoCF = addrRegex(geoip.Cloudflare)
	out, err := renderOLSVhost(v, "")
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, string(out), "enableIpGeo             1", "RewriteCond %{ENV:GEOIP_COUNTRY_CODE} ^(?:CN)$", "RewriteCond %{HTTP:CF-IPCountry} ^(?:CN)$")
	v.GeoCF = "x\nEND_rules"
	if _, err := renderOLSVhost(v, ""); err == nil {
		t.Fatal("a regex that ends the rules block was rendered")
	}
	// Apache: an allow list lets the server itself and the listed files in.
	d.GeoMode, d.GeoRegex, d.GeoFiles = GeoAllow, "VN|ZZ", []string{"/etc/apache2/panel/geo/VN.conf", "/etc/apache2/panel/geo/ZZ.conf"}
	d.LogDir, d.SnippetDir, d.EmptyRoot = "/var/log/apache2", "/etc/apache2/panel", "/var/lib/panel-acme"
	a, err := renderApacheVhost(d, "")
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, string(a), "<RequireAny>\n                Require local\n                Include /etc/apache2/panel/geo/VN.conf", "<Location /.well-known/acme-challenge/>", "<Location /403.html>")
	d.GeoFiles = []string{"/etc/apache2/panel/geo/VN.conf\nRequire all granted"}
	if _, err := renderApacheVhost(d, ""); err == nil {
		t.Fatal("a file name with a directive was rendered")
	}
}

func TestGeoTooLarge(t *testing.T) {
	for _, c := range []struct {
		counts map[string]int
		want   string
	}{
		{map[string]int{"US": 271881}, "US:271881"},
		{map[string]int{"US": 271881, "CN": 15189, "ZZ": 18}, "US:271881"},
		{map[string]int{"DE": 55789, "AU": 159920, "VN": 3284}, "AU:159920,DE:55789"},
		{map[string]int{"CN": 30000, "RU": 23054}, "CN:30000"},
		{map[string]int{"CN": 15189, "VN": 3284}, ""},
	} {
		if got := geoTooLarge(c.counts); got != c.want {
			t.Errorf("%v: %q, want %q", c.counts, got, c.want)
		}
	}
}
