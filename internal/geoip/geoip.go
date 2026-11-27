// Package geoip reads DB-IP's free "IP to Country Lite" database and turns
// the part of it the websites need into web server configuration.
//
// The data: DB-IP's IP to Country Lite (https://db-ip.com/db/lite.php),
// updated monthly, IPv4 and IPv6, under the Creative Commons Attribution 4.0
// International licence (https://creativecommons.org/licenses/by/4.0/).
// The licence allows commercial use and redistribution; it asks for
// attribution, which the panel shows wherever country blocking is set:
// "IP Geolocation by DB-IP" with a link to https://db-ip.com.
//
// The other free databases were set aside: MaxMind's GeoLite2 needs an
// account and a licence key and comes under an end-user licence agreement
// (no anonymous download); IP2Location LITE needs a registered token and is
// CC BY-SA; the regional registries' delegation files are free of terms but
// give the country an address block was registered in, not where it is
// used, and come as five files to merge.
//
// The CSV has one line per range, "first,last,CC", sorted, IPv4 first, then
// IPv6; together the ranges cover every address, reserved ones (private
// networks, loopback, multicast…) with the code ZZ.
package geoip

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/bits"
	"net/netip"
	"slices"
	"strings"
)

const (
	// CSVURL and MMDBURL are the monthly files; %s is the month, 2026-11.
	CSVURL  = "https://download.db-ip.com/free/dbip-country-lite-%s.csv.gz"
	MMDBURL = "https://download.db-ip.com/free/dbip-country-lite-%s.mmdb.gz"

	Attribution    = "IP Geolocation by DB-IP"
	AttributionURL = "https://db-ip.com"
	License        = "CC BY 4.0"
	LicenseURL     = "https://creativecommons.org/licenses/by/4.0/"

	// Reserved is DB-IP's code for addresses in no country.
	Reserved = "ZZ"

	// Size limits of a download, compressed and not.
	MaxGzip     = 64 << 20
	MaxCSV      = 512 << 20
	MaxMMDB     = 256 << 20
	minCSVGzip  = 1 << 20
	minMMDB     = 1 << 20
	maxCSVLines = 5_000_000
	// A real file has about 360,000 IPv4 and 370,000 IPv6 ranges in
	// some 250 countries.
	minRanges    = 50_000
	minCountries = 200
)

// Range is one line of the database: the addresses Lo to Hi (inclusive)
// are in the country CC.
type Range struct {
	Lo, Hi netip.Addr
	CC     string
}

// Stats count what a database holds.
type Stats struct {
	V4        int `json:"v4"`
	V6        int `json:"v6"`
	Countries int `json:"countries"`
}

var errFormat = errors.New("not DB-IP's IP to Country Lite CSV format")

// Scan reads the CSV (uncompressed), checks every line, and calls keep with
// the ranges of the countries in want (nil: all). The ranges must be valid,
// sorted and must not overlap; IPv4 comes before IPv6.
func Scan(r io.Reader, want map[string]bool, keep func(Range)) (Stats, error) {
	var st Stats
	countries := map[string]bool{}
	sc := bufio.NewScanner(io.LimitReader(r, MaxCSV))
	sc.Buffer(make([]byte, 4096), 4096)
	var prev Range
	have := false
	lines := 0
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		if lines++; lines > maxCSVLines {
			return st, fmt.Errorf("%w: more than %d lines", errFormat, maxCSVLines)
		}
		a, rest, ok1 := bytes.Cut(line, []byte{','})
		b, cc, ok2 := bytes.Cut(rest, []byte{','})
		if !ok1 || !ok2 || len(cc) != 2 || cc[0] < 'A' || cc[0] > 'Z' || cc[1] < 'A' || cc[1] > 'Z' {
			return st, fmt.Errorf("%w: line %d", errFormat, lines)
		}
		lo, err1 := netip.ParseAddr(string(a))
		hi, err2 := netip.ParseAddr(string(b))
		if err1 != nil || err2 != nil || lo.Is4() != hi.Is4() || lo.Zone() != "" || hi.Zone() != "" ||
			lo.Is4In6() || hi.Is4In6() || hi.Less(lo) {
			return st, fmt.Errorf("%w: line %d", errFormat, lines)
		}
		if have {
			switch {
			case prev.Lo.Is4() && !lo.Is4():
				// IPv6 starts.
			case prev.Lo.Is4() != lo.Is4() || !prev.Hi.Less(lo):
				return st, fmt.Errorf("%w: line %d is out of order", errFormat, lines)
			}
		}
		r := Range{Lo: lo, Hi: hi, CC: string(cc)}
		if lo.Is4() {
			st.V4++
		} else {
			st.V6++
		}
		countries[r.CC] = true
		if want == nil || want[r.CC] {
			keep(r)
		}
		prev, have = r, true
	}
	if err := sc.Err(); err != nil {
		return st, fmt.Errorf("%w: %v", errFormat, err)
	}
	delete(countries, Reserved)
	st.Countries = len(countries)
	if st.V4 < minRanges || st.V6 < minRanges || st.Countries < minCountries {
		return st, fmt.Errorf("%w: only %d IPv4 and %d IPv6 ranges in %d countries", errFormat, st.V4, st.V6, st.Countries)
	}
	return st, nil
}

// Subset reads the CSV and returns the ranges of the countries in want,
// IPv4 and IPv6 apart, with adjacent ranges of one country merged.
func Subset(r io.Reader, want map[string]bool) (v4, v6 []Range, st Stats, err error) {
	add := func(l []Range, x Range) []Range {
		if n := len(l); n > 0 && l[n-1].CC == x.CC && l[n-1].Hi.Next() == x.Lo {
			l[n-1].Hi = x.Hi
			return l
		}
		return append(l, x)
	}
	st, err = Scan(r, want, func(x Range) {
		if x.Lo.Is4() {
			v4 = add(v4, x)
		} else {
			v6 = add(v6, x)
		}
	})
	if err != nil {
		return nil, nil, st, err
	}
	return v4, v6, st, nil
}

// CIDRs splits the range lo-hi into the fewest prefixes.
func CIDRs(lo, hi netip.Addr) []netip.Prefix {
	var out []netip.Prefix
	bits := lo.BitLen()
	for {
		// The shortest prefix that starts at lo and ends at or before hi.
		l := bits
		for l > 0 {
			p := netip.PrefixFrom(lo, l-1)
			if p.Masked().Addr() != lo || hi.Less(lastAddr(p)) {
				break
			}
			l--
		}
		p := netip.PrefixFrom(lo, l)
		out = append(out, p)
		last := lastAddr(p)
		if !last.Less(hi) {
			return out
		}
		lo = last.Next()
	}
}

// lastAddr returns the last address of a prefix.
func lastAddr(p netip.Prefix) netip.Addr {
	a := p.Addr().AsSlice()
	for i := p.Bits(); i < len(a)*8; i++ {
		a[i/8] |= 0x80 >> (i % 8)
	}
	r, _ := netip.AddrFromSlice(a)
	return r
}

// ---- web server configuration ----

// Nginx's geo module keeps IPv4 lists two ways, with different costs,
// measured in a sandbox (Nginx 1.24, memory of all its processes after a
// restart):
//
//   - "ranges" mode (IPv4 only): while parsing, a 1 KB array for every /16
//     that a range covers, which the master process keeps after the parse
//     (China alone: +7.4 MB; the reserved ranges' 9,000 /16s: +10 MB);
//   - CIDR mode: a 32-byte radix tree node per prefix bit, about three
//     times the bare count with the allocator's overhead (China: +5 MB,
//     Viet Nam: +1.8 MB; the United States, Germany and Japan: +73 MB,
//     against +41 MB in ranges mode).
//
// NginxRangesMode picks the smaller for an IPv4 list. IPv6 is always CIDR.
func NginxRangesMode(v4 []Range) bool {
	buckets := 0
	last := -1
	for _, r := range v4 {
		if !keepV4(r.Lo) {
			continue
		}
		a, b := r.Lo.As4(), r.Hi.As4()
		lo, hi := int(a[0])<<8|int(a[1]), int(b[0])<<8|int(b[1])
		if lo == last {
			lo++
		}
		if hi >= lo {
			buckets += hi - lo + 1
		}
		last = hi
	}
	return int64(buckets)*1100 < 3*NginxMemory(v4)
}

// keepV4 leaves out reserved IPv4 space from 224.0.0.0 (multicast, future
// use) and 0.0.0.0/8: no client has such an address, and Nginx looks up an
// IPv6 address in a ranges block, or an address it cannot parse, as
// 255.255.255.255, which must get the block's default.
func keepV4(a netip.Addr) bool {
	b := a.As4()
	return b[0] != 0 && b[0] < 224
}

// NginxRanges renders IPv4 ranges for a geo block in "ranges" mode, one
// "first-last CC;" line each.
func NginxRanges(v4 []Range) []byte {
	var b bytes.Buffer
	for _, r := range v4 {
		if r.Lo.Is4() && keepV4(r.Lo) {
			hi := r.Hi
			if !keepV4(hi) {
				hi = netip.AddrFrom4([4]byte{223, 255, 255, 255})
			}
			fmt.Fprintf(&b, "%s-%s %s;\n", r.Lo, hi, r.CC)
		}
	}
	return b.Bytes()
}

// NginxCIDRs renders ranges as prefixes for a geo block in its default
// (CIDR) mode, one "prefix CC;" line each.
func NginxCIDRs(ranges []Range) []byte {
	var b bytes.Buffer
	for _, r := range ranges {
		for _, p := range CIDRs(r.Lo, r.Hi) {
			if p.Addr().Is4() && !keepV4(p.Addr()) {
				continue
			}
			fmt.Fprintf(&b, "%s %s;\n", p, r.CC)
		}
	}
	return b.Bytes()
}

// apachePerLine keeps each "Require ip" line well below Apache's limit
// for a configuration line (8 KB in older versions).
const apachePerLine = 200

// ApacheRequire renders the ranges as "Require ip" lines and returns how
// many prefixes they hold.
func ApacheRequire(ranges []Range) ([]byte, int) {
	var b bytes.Buffer
	n, col := 0, 0
	for _, r := range ranges {
		for _, p := range CIDRs(r.Lo, r.Hi) {
			if col == 0 {
				b.WriteString("Require ip")
			}
			b.WriteByte(' ')
			if p.Bits() == p.Addr().BitLen() {
				b.WriteString(p.Addr().String())
			} else {
				b.WriteString(p.String())
			}
			n++
			if col++; col == apachePerLine {
				b.WriteByte('\n')
				col = 0
			}
		}
	}
	if col > 0 {
		b.WriteByte('\n')
	}
	return b.Bytes(), n
}

// CountPrefixes returns how many prefixes the ranges make.
func CountPrefixes(ranges []Range) int {
	n := 0
	for _, r := range ranges {
		n += len(CIDRs(r.Lo, r.Hi))
	}
	return n
}

// NginxMemory estimates the memory (in bytes) the geo block of NginxCIDRs
// takes in each Nginx process: a 32-byte tree node per prefix bit not
// shared with another prefix. Ranges come sorted per family, so each prefix
// shares most with the one before it.
func NginxMemory(ranges []Range) int64 {
	var nodes int64
	var prev netip.Prefix
	for _, r := range ranges {
		for _, p := range CIDRs(r.Lo, r.Hi) {
			shared := 0
			if prev.IsValid() && prev.Addr().Is4() == p.Addr().Is4() {
				shared = min(commonBits(prev.Addr(), p.Addr()), prev.Bits(), p.Bits())
			}
			nodes += int64(p.Bits() - shared)
			prev = p
		}
	}
	return 32 * nodes
}

// commonBits returns how many leading bits two addresses share.
func commonBits(a, b netip.Addr) int {
	x, y := a.AsSlice(), b.AsSlice()
	for i := range x {
		if d := x[i] ^ y[i]; d != 0 {
			return i*8 + bits.LeadingZeros8(d)
		}
	}
	return len(x) * 8
}

// ---- Cloudflare ----

// Cloudflare are the addresses of Cloudflare's proxy
// (https://www.cloudflare.com/ips/, also at
// https://api.cloudflare.com/client/v4/ips), as of November 2026. Requests
// from them carry the visitor's address in CF-Connecting-IP.
var Cloudflare = []string{
	"173.245.48.0/20", "103.21.244.0/22", "103.22.200.0/22", "103.31.4.0/22", "141.101.64.0/18",
	"108.162.192.0/18", "190.93.240.0/20", "188.114.96.0/20", "197.234.240.0/22", "198.41.128.0/17",
	"162.158.0.0/15", "104.16.0.0/13", "104.24.0.0/14", "172.64.0.0/13", "131.0.72.0/22",
	"2400:cb00::/32", "2606:4700::/32", "2803:f800::/32", "2405:b500::/32", "2405:8100::/32",
	"2a06:98c0::/29", "2c0f:f248::/32",
}

// CheckPrefixes returns the prefixes in canonical form, or an error if one
// is not a public network prefix or the list is implausibly short or long.
func CheckPrefixes(in []string) ([]string, error) {
	var out []string
	for _, s := range in {
		p, err := netip.ParsePrefix(strings.TrimSpace(s))
		if err != nil || p != p.Masked() || p.Addr().Is4In6() || p.Addr().Zone() != "" || p.Bits() < 8 ||
			p.Addr().IsPrivate() || p.Addr().IsLoopback() || p.Addr().IsUnspecified() {
			return nil, fmt.Errorf("not a public network prefix: %q", s)
		}
		if !slices.Contains(out, p.String()) {
			out = append(out, p.String())
		}
	}
	if len(out) < 5 || len(out) > 200 {
		return nil, fmt.Errorf("%d prefixes", len(out))
	}
	return out, nil
}

// ---- MaxMind DB files (OpenLiteSpeed) ----

var mmdbMarker = []byte("\xab\xcd\xefMaxMind.com")

// CheckMMDB checks that data is a MaxMind DB file with country data for
// IPv4 and IPv6, as DB-IP's .mmdb download is: its metadata names a country
// database, IP version 6, and a search tree that fits in the file.
func CheckMMDB(data []byte) error {
	if len(data) < minMMDB || len(data) > MaxMMDB {
		return fmt.Errorf("not a country database: %d bytes", len(data))
	}
	tail := data[max(0, len(data)-128<<10):]
	i := bytes.LastIndex(tail, mmdbMarker)
	if i < 0 {
		return errors.New("not a MaxMind DB file: no metadata")
	}
	d := &mmdbDecoder{buf: tail[i+len(mmdbMarker):]}
	meta, err := d.value(0)
	if err != nil {
		return fmt.Errorf("not a MaxMind DB file: %v", err)
	}
	m, ok := meta.(map[string]any)
	if !ok {
		return errors.New("not a MaxMind DB file: bad metadata")
	}
	typ, _ := m["database_type"].(string)
	ipv, _ := m["ip_version"].(uint64)
	nodes, _ := m["node_count"].(uint64)
	rec, _ := m["record_size"].(uint64)
	if !strings.Contains(strings.ToLower(typ), "country") || ipv != 6 || nodes < 100_000 || (rec != 24 && rec != 28 && rec != 32) {
		return fmt.Errorf("not a country database for IPv4 and IPv6 (%q, IPv%d, %d nodes)", typ, ipv, nodes)
	}
	if tree := nodes * rec * 2 / 8; tree+16 > uint64(len(data)) {
		return errors.New("damaged MaxMind DB file: the search tree is longer than the file")
	}
	return nil
}

// mmdbDecoder reads the metadata section of a MaxMind DB file
// (https://maxmind.github.io/MaxMind-DB/): maps, arrays, strings and
// numbers, no pointers.
type mmdbDecoder struct {
	buf []byte
	off int
}

func (d *mmdbDecoder) next(n int) ([]byte, error) {
	if n < 0 || d.off+n > len(d.buf) {
		return nil, errors.New("truncated")
	}
	b := d.buf[d.off : d.off+n]
	d.off += n
	return b, nil
}

func (d *mmdbDecoder) value(depth int) (any, error) {
	if depth > 8 {
		return nil, errors.New("nested too deeply")
	}
	c, err := d.next(1)
	if err != nil {
		return nil, err
	}
	typ, size := int(c[0]>>5), int(c[0]&0x1f)
	if typ == 0 {
		e, err := d.next(1)
		if err != nil {
			return nil, err
		}
		typ = 7 + int(e[0])
	}
	if typ == 1 {
		return nil, errors.New("pointer in metadata")
	}
	switch size {
	case 29, 30, 31:
		ext, err := d.next(size - 28)
		if err != nil {
			return nil, err
		}
		v := 0
		for _, x := range ext {
			v = v<<8 | int(x)
		}
		size = []int{29, 285, 65821}[size-29] + v
	}
	switch typ {
	case 2, 4: // UTF-8 string, bytes
		b, err := d.next(size)
		return string(b), err
	case 3: // double
		_, err := d.next(8)
		return nil, err
	case 15: // float
		_, err := d.next(4)
		return nil, err
	case 5, 6, 8, 9, 10: // uint16, uint32, int32, uint64, uint128
		b, err := d.next(size)
		if err != nil || size > 8 {
			return nil, err
		}
		var v [8]byte
		copy(v[8-size:], b)
		return binary.BigEndian.Uint64(v[:]), nil
	case 14: // boolean: the size is the value
		return size != 0, nil
	case 7: // map
		m := map[string]any{}
		for range size {
			k, err := d.value(depth + 1)
			if err != nil {
				return nil, err
			}
			ks, ok := k.(string)
			if !ok {
				return nil, errors.New("map key is not a string")
			}
			if m[ks], err = d.value(depth + 1); err != nil {
				return nil, err
			}
		}
		return m, nil
	case 11: // array
		var a []any
		for range size {
			v, err := d.value(depth + 1)
			if err != nil {
				return nil, err
			}
			a = append(a, v)
		}
		return a, nil
	}
	return nil, fmt.Errorf("unknown type %d", typ)
}
