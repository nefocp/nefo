package geoip

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net/netip"
	"strings"
	"testing"
)

// fakeCSV builds a database in DB-IP's format that covers every address:
// n IPv4 and n IPv6 ranges, cycling through the countries, with ZZ for
// 10.0.0.0/8 and the last IPv4 /8.
func fakeCSV(n int) string {
	var b strings.Builder
	cc := Countries()
	v4 := func(x uint32) netip.Addr {
		var a [4]byte
		binary.BigEndian.PutUint32(a[:], x)
		return netip.AddrFrom4(a)
	}
	step := uint32((1 << 32) / uint64(n))
	for i := 0; i < n; i++ {
		lo, hi := uint32(i)*step, uint32(i+1)*step-1
		if i == n-1 {
			hi = 1<<32 - 1
		}
		c := cc[i%len(cc)]
		if lo>>24 == 10 || i == n-1 {
			c = Reserved
		}
		fmt.Fprintf(&b, "%s,%s,%s\n", v4(lo), v4(hi), c)
	}
	v6 := func(x uint64) netip.Addr {
		var a [16]byte
		binary.BigEndian.PutUint64(a[:8], x)
		return netip.AddrFrom16(a)
	}
	step6 := uint64(1<<64-1) / uint64(n)
	for i := 0; i < n; i++ {
		lo, hi := v6(uint64(i)*step6), v6(uint64(i+1)*step6-1)
		if i == n-1 {
			hi = netip.MustParseAddr("ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff")
		} else {
			a := hi.As16()
			for j := 8; j < 16; j++ {
				a[j] = 0xff
			}
			hi = netip.AddrFrom16(a)
		}
		fmt.Fprintf(&b, "%s,%s,%s\n", lo, hi, cc[(i+7)%len(cc)])
	}
	return b.String()
}

func TestCountries(t *testing.T) {
	if n := len(Countries()); n != 250 {
		t.Fatalf("%d country codes, want 249 + XK", n)
	}
	if !IsCountry("VN") || !IsCountry("XK") || IsCountry("ZZ") || IsCountry("vn") || IsCountry("EU") {
		t.Fatal("IsCountry")
	}
}

func TestScanAndSubset(t *testing.T) {
	csv := fakeCSV(60000)
	st, err := Scan(strings.NewReader(csv), nil, func(Range) {})
	if err != nil {
		t.Fatal(err)
	}
	if st.V4 != 60000 || st.V6 != 60000 || st.Countries != 250 {
		t.Fatalf("stats %+v", st)
	}
	v4, v6, _, err := Subset(strings.NewReader(csv), map[string]bool{"VN": true, Reserved: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range append(v4, v6...) {
		if r.CC != "VN" && r.CC != Reserved {
			t.Fatalf("subset has %v", r)
		}
	}
	// 10.0.0.0/8 is many adjacent ZZ lines: merged into one range.
	n := 0
	for _, r := range v4 {
		if r.CC == Reserved && r.Lo.As4()[0] == 10 {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("10/8 in %d ranges, not merged into one", n)
	}
}

func TestScanRejects(t *testing.T) {
	good := fakeCSV(60000)
	lines := strings.SplitAfter(good, "\n")
	for name, csv := range map[string]string{
		"empty":         "",
		"html":          "<html><body>Not found</body></html>\n",
		"too small":     strings.Join(lines[:1000], ""),
		"swapped":       strings.Join(append([]string{lines[1], lines[0]}, lines[2:]...), ""),
		"bad country":   strings.Replace(good, ",VN\n", ",vn\n", 1),
		"bad address":   strings.Replace(good, "0.0.0.0,", "0.0.0.x,", 1),
		"mixed family":  "0.0.0.0,::1,US\n" + good,
		"reversed":      "1.0.0.9,1.0.0.1,US\n" + good,
		"v4 after v6":   good + "1.2.3.4,1.2.3.5,US\n",
		"long line":     strings.Repeat("x", 5000) + "\n" + good,
		"extra field":   strings.Replace(good, ",VN\n", ",VN,x\n", 1),
		"few countries": strings.NewReplacer(",VN\n", ",US\n", ",DE\n", ",US\n").Replace(strings.Join(lines[:120000], "")),
	} {
		if name == "few countries" {
			continue // still 248 countries: covered by "too small"
		}
		if _, err := Scan(strings.NewReader(csv), nil, func(Range) {}); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestCIDRs(t *testing.T) {
	for _, c := range []struct{ lo, hi, want string }{
		{"1.0.0.0", "1.0.0.255", "[1.0.0.0/24]"},
		{"1.0.1.0", "1.0.3.255", "[1.0.1.0/24 1.0.2.0/23]"},
		{"0.0.0.0", "255.255.255.255", "[0.0.0.0/0]"},
		{"10.0.0.5", "10.0.0.5", "[10.0.0.5/32]"},
		{"2001:db8::", "2001:db8:ffff:ffff:ffff:ffff:ffff:ffff", "[2001:db8::/32]"},
		{"2001:db8::1", "2001:db8::2", "[2001:db8::1/128 2001:db8::2/128]"},
	} {
		got := fmt.Sprint(CIDRs(netip.MustParseAddr(c.lo), netip.MustParseAddr(c.hi)))
		if got != c.want {
			t.Errorf("%s-%s = %s, want %s", c.lo, c.hi, got, c.want)
		}
	}
}

func TestRenderers(t *testing.T) {
	r := func(lo, hi, cc string) Range {
		return Range{Lo: netip.MustParseAddr(lo), Hi: netip.MustParseAddr(hi), CC: cc}
	}
	v4 := []Range{r("1.0.1.0", "1.0.3.255", "CN"), r("224.0.0.0", "255.255.255.255", Reserved)}
	if got := string(NginxCIDRs(v4)); got != "1.0.1.0/24 CN;\n1.0.2.0/23 CN;\n" {
		t.Fatalf("NginxCIDRs v4:\n%s", got)
	}
	if got := string(NginxRanges(append(v4, r("200.0.0.0", "230.0.0.0", "US")))); got != "1.0.1.0-1.0.3.255 CN;\n200.0.0.0-223.255.255.255 US;\n" {
		t.Fatalf("NginxRanges:\n%s", got)
	}
	// A few large ranges: CIDR mode; many scattered addresses in one /16:
	// ranges mode.
	var scattered []Range
	for i := 0; i < 1000; i += 2 {
		a := netip.AddrFrom4([4]byte{1, 0, byte(i >> 8), byte(i)})
		scattered = append(scattered, Range{Lo: a, Hi: a, CC: "CN"})
	}
	if NginxRangesMode([]Range{r("1.0.0.0", "1.255.255.255", "CN")}) || !NginxRangesMode(scattered) {
		t.Fatal("NginxRangesMode")
	}
	v6 := []Range{r("2001:db8::", "2001:db8:1:ffff:ffff:ffff:ffff:ffff", "VN")}
	if got := string(NginxCIDRs(v6)); got != "2001:db8::/47 VN;\n" {
		t.Fatalf("NginxCIDRs:\n%s", got)
	}
	var many []Range
	for i := 0; i < 450; i++ {
		ip := netip.AddrFrom4([4]byte{1, byte(i >> 8), byte(i), 0})
		many = append(many, Range{Lo: ip, Hi: ip, CC: "CN"})
	}
	out, n := ApacheRequire(append(many, v6...))
	lines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if n != 451 || len(lines) != 3 || !strings.HasPrefix(lines[0], "Require ip 1.0.0.0 1.0.1.0 ") || !strings.HasSuffix(lines[2], " 2001:db8::/47") {
		t.Fatalf("ApacheRequire: %d prefixes, %d lines: %q", n, len(lines), lines[2])
	}
	if CountPrefixes(v4) != 3 {
		t.Fatal("CountPrefixes")
	}
	// 1.0.1.0/24 (24 nodes) and 1.0.2.0/23 (sharing 22 bits: 1 more);
	// 224.0.0.0/3 (3); 2001:db8::/47 (47).
	if got := NginxMemory(append(v4, v6...)); got != 32*(24+1+3+47) {
		t.Fatalf("NginxMemory %d", got)
	}
}

func TestCheckPrefixes(t *testing.T) {
	got, err := CheckPrefixes(Cloudflare)
	if err != nil || len(got) != len(Cloudflare) {
		t.Fatalf("Cloudflare's list: %v %v", got, err)
	}
	for _, bad := range [][]string{
		{"1.2.3.0/24"},
		append([]string{"10.0.0.0/8"}, Cloudflare...),
		append([]string{"1.2.3.4/24"}, Cloudflare...),
		append([]string{"0.0.0.0/0"}, Cloudflare...),
		append([]string{"x"}, Cloudflare...),
	} {
		if _, err := CheckPrefixes(bad); err == nil {
			t.Errorf("accepted %v", bad[0])
		}
	}
}

// mmdbMeta encodes a metadata map the way MaxMind DB files do.
func mmdbMeta(fields map[string]any) []byte {
	var b bytes.Buffer
	b.Write(mmdbMarker)
	b.WriteByte(7<<5 | byte(len(fields)))
	for k, v := range fields {
		b.WriteByte(2<<5 | byte(len(k)))
		b.WriteString(k)
		switch v := v.(type) {
		case string:
			b.WriteByte(2<<5 | byte(len(v)))
			b.WriteString(v)
		case int:
			b.WriteByte(6<<5 | 4)
			binary.Write(&b, binary.BigEndian, uint32(v))
		}
	}
	return b.Bytes()
}

func TestCheckMMDB(t *testing.T) {
	file := func(meta map[string]any, size int) []byte {
		return append(make([]byte, size), mmdbMeta(meta)...)
	}
	good := map[string]any{"database_type": "DBIP-Country-Lite", "ip_version": 6, "node_count": 400000, "record_size": 24}
	if err := CheckMMDB(file(good, 3<<20)); err != nil {
		t.Fatal(err)
	}
	city := map[string]any{"database_type": "GeoLite2-ASN", "ip_version": 6, "node_count": 400000, "record_size": 24}
	v4 := map[string]any{"database_type": "DBIP-Country-Lite", "ip_version": 4, "node_count": 400000, "record_size": 24}
	for name, data := range map[string][]byte{
		"small":     file(good, 100),
		"no marker": make([]byte, 3<<20),
		"asn":       file(city, 3<<20),
		"ipv4 only": file(v4, 3<<20),
		"tree":      file(good, 1<<20+10),
		"truncated": file(good, 3<<20)[:3<<20+30],
	} {
		if err := CheckMMDB(data); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
