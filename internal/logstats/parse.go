package logstats

import (
	"bytes"
	"time"
)

// entry is one parsed access log line. The byte slices point into the line
// and are only valid until the next line is read.
type entry struct {
	ip     []byte
	unix   int64  // seconds since the epoch
	path   []byte // without the query string; nil if the request line is not a request
	status int
	bytes  int64
	ref    []byte // nil for "-"
	agent  []byte // nil for "-" or ""
}

// acmePrefix starts the paths of certificate validation requests, which are
// not visits and are left out of the statistics.
var acmePrefix = []byte("/.well-known/acme-challenge/")

// maxPathLen shortens very long paths, which are mostly attacks, so that
// they cannot use much memory.
const maxPathLen = 200

// parseLine reads a line in Nginx's "combined" format:
//
//	$remote_addr - $remote_user [$time_local] "$request" $status $body_bytes_sent "$http_referer" "$http_user_agent"
//
// Nginx escapes double quotes inside the quoted fields (as \x22), so a quote
// always ends a field. The referrer and user agent may be missing (the
// "common" format), and anything after them is ignored.
func parseLine(line []byte, e *entry) bool {
	*e = entry{}
	sp := bytes.IndexByte(line, ' ')
	if sp <= 0 || !looksLikeIP(line[:sp]) {
		return false
	}
	e.ip = line[:sp]
	rest := line[sp+1:]

	// "- $remote_user " then the time, which has a fixed width.
	lb := bytes.IndexByte(rest, '[')
	if lb < 0 || len(rest) < lb+28 || rest[lb+27] != ']' {
		return false
	}
	t, ok := parseTime(rest[lb+1 : lb+27])
	if !ok {
		return false
	}
	e.unix = t
	rest = rest[lb+28:]

	req, rest, ok := quoted(rest)
	if !ok {
		return false
	}
	e.path = requestPath(req)

	// " $status $body_bytes_sent"
	if len(rest) < 5 || rest[0] != ' ' {
		return false
	}
	rest = rest[1:]
	if rest[0] < '1' || rest[0] > '9' || !isDigit(rest[1]) || !isDigit(rest[2]) || (len(rest) > 3 && rest[3] != ' ') {
		return false
	}
	e.status = int(rest[0]-'0')*100 + int(rest[1]-'0')*10 + int(rest[2]-'0')
	rest = rest[3:]
	if len(rest) < 2 {
		return false
	}
	rest = rest[1:]
	n := 0
	if rest[0] == '-' {
		n = 1
	} else {
		for n < len(rest) && isDigit(rest[n]) {
			if n >= 18 { // cannot overflow int64
				return false
			}
			e.bytes = e.bytes*10 + int64(rest[n]-'0')
			n++
		}
		if n == 0 {
			return false
		}
	}
	rest = rest[n:]
	if len(rest) == 0 || isSpaceOnly(rest) {
		return true // the "common" format
	}

	var ref, agent []byte
	if ref, rest, ok = quoted(rest); !ok {
		return false
	}
	if agent, _, ok = quoted(rest); !ok {
		return false
	}
	if !(len(ref) == 1 && ref[0] == '-') && len(ref) > 0 {
		e.ref = ref
	}
	if !(len(agent) == 1 && agent[0] == '-') && len(agent) > 0 {
		e.agent = agent
	}
	return true
}

// quoted reads ` "value"` from the start of b and returns the value and what
// follows the closing quote.
func quoted(b []byte) (value, rest []byte, ok bool) {
	if len(b) < 3 || b[0] != ' ' || b[1] != '"' {
		return nil, nil, false
	}
	b = b[2:]
	q := bytes.IndexByte(b, '"')
	if q < 0 {
		return nil, nil, false
	}
	return b[:q], b[q+1:], true
}

// requestPath returns the path of a request line such as
// "GET /blog/?page=2 HTTP/1.1", without the query string. It returns nil for
// lines that are not requests, such as the bytes of a TLS handshake sent to
// the HTTP port.
func requestPath(req []byte) []byte {
	sp := bytes.IndexByte(req, ' ')
	if sp <= 0 {
		return nil
	}
	uri := req[sp+1:]
	if sp2 := bytes.IndexByte(uri, ' '); sp2 >= 0 {
		uri = uri[:sp2]
	}
	// An absolute URI ("GET http://host/path"), as sent to proxies.
	if i := bytes.Index(uri, []byte("://")); i > 0 && i < 8 {
		uri = uri[i+3:]
		if j := bytes.IndexByte(uri, '/'); j >= 0 {
			uri = uri[j:]
		} else {
			uri = []byte("/")
		}
	}
	if len(uri) == 0 || uri[0] != '/' {
		return nil
	}
	if q := bytes.IndexAny(uri, "?#"); q >= 0 {
		uri = uri[:q]
	}
	if len(uri) > maxPathLen {
		uri = uri[:maxPathLen]
	}
	return uri
}

func looksLikeIP(b []byte) bool {
	if len(b) < 2 || len(b) > 45 {
		return false
	}
	sep := false
	for _, c := range b {
		switch {
		case c == '.' || c == ':':
			sep = true
		case isDigit(c), c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return sep
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isSpaceOnly(b []byte) bool {
	for _, c := range b {
		if c != ' ' && c != '\t' && c != '\r' {
			return false
		}
	}
	return true
}

var months = map[string]time.Month{
	"Jan": time.January, "Feb": time.February, "Mar": time.March, "Apr": time.April,
	"May": time.May, "Jun": time.June, "Jul": time.July, "Aug": time.August,
	"Sep": time.September, "Oct": time.October, "Nov": time.November, "Dec": time.December,
}

// parseTime reads Nginx's $time_local, "02/Jan/2006:15:04:05 -0700", and
// returns it as Unix seconds. It is much faster than time.Parse.
func parseTime(b []byte) (int64, bool) {
	if len(b) != 26 || b[2] != '/' || b[6] != '/' || b[11] != ':' || b[14] != ':' || b[17] != ':' || b[20] != ' ' {
		return 0, false
	}
	num := func(s []byte) (int, bool) {
		n := 0
		for _, c := range s {
			if !isDigit(c) {
				return 0, false
			}
			n = n*10 + int(c-'0')
		}
		return n, true
	}
	day, ok1 := num(b[0:2])
	month, ok2 := months[string(b[3:6])]
	year, ok3 := num(b[7:11])
	hour, ok4 := num(b[12:14])
	minute, ok5 := num(b[15:17])
	sec, ok6 := num(b[18:20])
	oh, ok7 := num(b[22:24])
	om, ok8 := num(b[24:26])
	if !(ok1 && ok2 && ok3 && ok4 && ok5 && ok6 && ok7 && ok8) ||
		day < 1 || day > 31 || hour > 23 || minute > 59 || sec > 60 || oh > 14 || om > 59 ||
		(b[21] != '+' && b[21] != '-') {
		return 0, false
	}
	offset := int64(oh*3600 + om*60)
	if b[21] == '-' {
		offset = -offset
	}
	return time.Date(year, month, day, hour, minute, sec, 0, time.UTC).Unix() - offset, true
}
