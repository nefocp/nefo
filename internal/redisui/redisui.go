// Package redisui lets the administrator look into the local Redis server:
// its version, memory and keys, one key's value, deleting a key and
// emptying a database. It speaks just enough RESP2 over TCP and only sends
// the commands listed in allowed, each with a timeout.
package redisui

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nefocp/nefo/internal/usererr"
)

const (
	// Databases is the number of databases the panel offers (0 to 15, the
	// Redis default).
	Databases = 16
	// MaxString is how much of a string value is shown.
	MaxString = 64 << 10
	// MaxItems is how many items of a hash, list, set or sorted set are shown.
	MaxItems = 200
	// maxItemValue is how much of one item is shown.
	maxItemValue = 4 << 10
	scanCount    = "100"
	maxKeyLen    = 4 << 10
	maxPattern   = 256
)

// Client talks to the Redis server on this machine.
type Client struct {
	Addr     string        // 127.0.0.1:6379
	ConfPath string        // where requirepass is read from
	Binary   string        // present when Redis is installed
	Timeout  time.Duration // for connecting and for each exchange
}

// New returns a Client for the Redis server the panel installs.
func New() *Client {
	return &Client{Addr: "127.0.0.1:6379", ConfPath: "/etc/redis/redis.conf",
		Binary: "/usr/bin/redis-server", Timeout: 5 * time.Second}
}

// Installed reports whether Redis is installed.
func (c *Client) Installed() bool {
	_, err := os.Stat(c.Binary)
	return err == nil
}

// open connects, signs in with the configured password and selects db.
func (c *Client) open(ctx context.Context, db int) (*conn, error) {
	if !c.Installed() {
		return nil, usererr.New("redis_not_installed")
	}
	if db < 0 || db >= Databases {
		return nil, usererr.New("redis_invalid_db", strconv.Itoa(db))
	}
	cn, err := dial(ctx, c.Addr, c.Timeout)
	if err != nil {
		return nil, usererr.New("redis_not_running", err.Error())
	}
	fail := func(err error) (*conn, error) {
		cn.Close()
		var re *Error
		if errors.As(err, &re) && (strings.HasPrefix(re.Msg, "NOAUTH") || strings.HasPrefix(re.Msg, "WRONGPASS") ||
			strings.HasPrefix(re.Msg, "ERR invalid password")) {
			return nil, usererr.New("redis_auth_failed", re.Msg)
		}
		return nil, err
	}
	if pw := readPassword(c.ConfPath); pw != "" {
		if _, err := cn.do(ctx, "AUTH", pw); err != nil {
			return fail(err)
		}
	}
	if _, err := cn.do(ctx, "SELECT", strconv.Itoa(db)); err != nil {
		return fail(err)
	}
	return cn, nil
}

// ---- info ----

// Info describes the Redis server.
type Info struct {
	Installed  bool   `json:"installed"`
	Running    bool   `json:"running"`
	Error      string `json:"error,omitempty"` // why the panel could not connect
	Version    string `json:"version"`
	UsedMemory int64  `json:"used_memory"`
	MaxMemory  int64  `json:"max_memory"` // 0: no limit
	Uptime     int64  `json:"uptime"`     // seconds
	Databases  []DB   `json:"databases"`
}

// DB is the number of keys in one database.
type DB struct {
	Index   int   `json:"db"`
	Keys    int64 `json:"keys"`
	Expires int64 `json:"expires"`
}

// Info reads the server's version, memory use, uptime and keys per database.
func (c *Client) Info(ctx context.Context) (*Info, error) {
	info := &Info{Installed: c.Installed(), Databases: make([]DB, Databases)}
	for i := range info.Databases {
		info.Databases[i].Index = i
	}
	if !info.Installed {
		return info, nil
	}
	cn, err := c.open(ctx, 0)
	if err != nil {
		var ue *usererr.Error
		if errors.As(err, &ue) && ue.Code == "redis_not_running" {
			info.Error = ue.Detail
			return info, nil
		}
		return nil, err
	}
	defer cn.Close()
	// One section per command: Redis 6 takes only one, and the full report
	// can be long.
	vs, err := cn.pipeline(ctx, [][]string{{"INFO", "server"}, {"INFO", "memory"}, {"INFO", "keyspace"}})
	if err != nil {
		return nil, err
	}
	info.Running = true
	for _, v := range vs {
		if v.kind == '-' {
			return nil, &Error{v.str}
		}
		parseInfo(v.str, info)
	}
	return info, nil
}

func parseInfo(s string, info *Info) {
	for _, line := range strings.Split(s, "\n") {
		k, v, ok := strings.Cut(strings.TrimRight(line, "\r"), ":")
		if !ok {
			continue
		}
		switch {
		case k == "redis_version":
			info.Version = v
		case k == "used_memory":
			info.UsedMemory, _ = strconv.ParseInt(v, 10, 64)
		case k == "maxmemory":
			info.MaxMemory, _ = strconv.ParseInt(v, 10, 64)
		case k == "uptime_in_seconds":
			info.Uptime, _ = strconv.ParseInt(v, 10, 64)
		case strings.HasPrefix(k, "db"):
			// db0:keys=3,expires=1,avg_ttl=0
			n, err := strconv.Atoi(k[2:])
			if err != nil || n < 0 || n >= Databases {
				continue
			}
			for _, kv := range strings.Split(v, ",") {
				name, num, _ := strings.Cut(kv, "=")
				x, _ := strconv.ParseInt(num, 10, 64)
				switch name {
				case "keys":
					info.Databases[n].Keys = x
				case "expires":
					info.Databases[n].Expires = x
				}
			}
		}
	}
}

// ---- keys ----

// KeyInfo is a key in a list.
type KeyInfo struct {
	// ID is the key's bytes in base64: keys need not be valid UTF-8, and
	// requests about one key name it by ID.
	ID   string `json:"id"`
	Name string `json:"name"` // for display
	Type string `json:"type"` // string, hash, list, set, zset, stream…
	TTL  int64  `json:"ttl"`  // seconds; -1: no expiry, -2: gone
}

// ScanResult is one page of keys.
type ScanResult struct {
	Cursor string    `json:"cursor"` // "0" when the scan is complete
	Keys   []KeyInfo `json:"keys"`
}

func keyInfo(raw string) KeyInfo {
	return KeyInfo{ID: base64.StdEncoding.EncodeToString([]byte(raw)), Name: display(raw)}
}

func display(s string) string { return strings.ToValidUTF8(s, "�") }

// DecodeKey turns a key ID back into the key.
func DecodeKey(id string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(id)
	if err != nil || len(b) == 0 || len(b) > maxKeyLen {
		return "", usererr.New("redis_invalid_key")
	}
	return string(b), nil
}

func checkPattern(p string) (string, error) {
	if p == "" {
		return "*", nil
	}
	if len(p) > maxPattern {
		return "", usererr.New("redis_invalid_pattern")
	}
	for _, r := range p {
		if r < 0x20 || r == 0x7f {
			return "", usererr.New("redis_invalid_pattern")
		}
	}
	return p, nil
}

func checkCursor(s string) (string, error) {
	if s == "" {
		return "0", nil
	}
	if _, err := strconv.ParseUint(s, 10, 64); err != nil {
		return "", usererr.New("redis_invalid_cursor")
	}
	return s, nil
}

func ttlSeconds(ms int64) int64 {
	if ms < 0 {
		return ms // -1 or -2
	}
	return (ms + 999) / 1000
}

// Scan returns one page of the keys matching pattern (Redis glob syntax),
// starting at cursor ("0" for the first page), with each key's type and
// time to live.
func (c *Client) Scan(ctx context.Context, db int, pattern, cursor string) (*ScanResult, error) {
	pattern, err := checkPattern(pattern)
	if err != nil {
		return nil, err
	}
	if cursor, err = checkCursor(cursor); err != nil {
		return nil, err
	}
	cn, err := c.open(ctx, db)
	if err != nil {
		return nil, err
	}
	defer cn.Close()
	v, err := cn.do(ctx, "SCAN", cursor, "MATCH", pattern, "COUNT", scanCount)
	if err != nil {
		return nil, err
	}
	if len(v.array) != 2 {
		return nil, errors.New("Redis sent an unexpected SCAN reply")
	}
	res := &ScanResult{Cursor: v.array[0].str, Keys: []KeyInfo{}}
	var cmds [][]string
	for _, k := range v.array[1].array {
		if k.truncated {
			continue // a key over 64 KB cannot be named again
		}
		res.Keys = append(res.Keys, keyInfo(k.str))
		cmds = append(cmds, []string{"TYPE", k.str}, []string{"PTTL", k.str})
	}
	if len(cmds) == 0 {
		return res, nil
	}
	replies, err := cn.pipeline(ctx, cmds)
	if err != nil {
		return nil, err
	}
	for i := range res.Keys {
		res.Keys[i].Type = replies[2*i].str
		res.Keys[i].TTL = ttlSeconds(replies[2*i+1].num)
	}
	return res, nil
}

// Item is one element of a hash, list, set or sorted set.
type Item struct {
	Field     string `json:"field,omitempty"` // hash field
	Value     string `json:"value"`
	Score     string `json:"score,omitempty"` // sorted set
	Truncated bool   `json:"truncated,omitempty"`
}

// Key is one key with (the start of) its value.
type Key struct {
	KeyInfo
	// Size is the length of a string in bytes, or the number of items.
	Size   int64  `json:"size"`
	Value  string `json:"value,omitempty"`
	Binary bool   `json:"binary,omitempty"` // the string is not valid UTF-8
	Items  []Item `json:"items"`
	// Truncated means only part of the value is shown.
	Truncated bool `json:"truncated"`
}

func item(v value) Item {
	s, cut := v.str, v.truncated
	if len(s) > maxItemValue {
		s, cut = s[:maxItemValue], true
	}
	return Item{Value: display(s), Truncated: cut}
}

// Get returns a key's type, time to live and value: up to MaxString bytes
// of a string, or the first MaxItems items of a hash, list, set or sorted
// set.
func (c *Client) Get(ctx context.Context, db int, key string) (*Key, error) {
	if key == "" || len(key) > maxKeyLen {
		return nil, usererr.New("redis_invalid_key")
	}
	cn, err := c.open(ctx, db)
	if err != nil {
		return nil, err
	}
	defer cn.Close()
	r, err := cn.pipeline(ctx, [][]string{{"TYPE", key}, {"PTTL", key}})
	if err != nil {
		return nil, err
	}
	k := &Key{KeyInfo: keyInfo(key), Items: []Item{}}
	k.Type, k.TTL = r[0].str, ttlSeconds(r[1].num)
	if k.Type == "none" || k.TTL == -2 {
		return nil, usererr.New("redis_key_not_found")
	}
	switch k.Type {
	case "string":
		r, err := cn.pipeline(ctx, [][]string{{"STRLEN", key}, {"GETRANGE", key, "0", strconv.Itoa(MaxString - 1)}})
		if err != nil {
			return nil, err
		}
		k.Size = r[0].num
		k.Binary = !utf8.ValidString(r[1].str)
		k.Value = display(r[1].str)
		k.Truncated = k.Size > int64(len(r[1].str))
		return k, nil
	case "list":
		r, err := cn.pipeline(ctx, [][]string{{"LLEN", key}, {"LRANGE", key, "0", strconv.Itoa(MaxItems - 1)}})
		if err != nil {
			return nil, err
		}
		k.Size = r[0].num
		for _, v := range r[1].array {
			k.Items = append(k.Items, item(v))
		}
	case "zset":
		r, err := cn.pipeline(ctx, [][]string{{"ZCARD", key}, {"ZRANGE", key, "0", strconv.Itoa(MaxItems - 1), "WITHSCORES"}})
		if err != nil {
			return nil, err
		}
		k.Size = r[0].num
		a := r[1].array
		for i := 0; i+1 < len(a); i += 2 {
			it := item(a[i])
			it.Score = a[i+1].str
			k.Items = append(k.Items, it)
		}
	case "hash", "set":
		count, scan := "SCARD", "SSCAN"
		if k.Type == "hash" {
			count, scan = "HLEN", "HSCAN"
		}
		n, err := cn.do(ctx, count, key)
		if err != nil {
			return nil, err
		}
		k.Size = n.num
		if k.Items, err = scanItems(ctx, cn, scan, key, k.Type == "hash"); err != nil {
			return nil, err
		}
	default:
		// Streams and module types: only the type and time to live.
		k.Truncated = true
		return k, nil
	}
	if k.Size > int64(len(k.Items)) {
		k.Truncated = true
	}
	for _, it := range k.Items {
		k.Truncated = k.Truncated || it.Truncated
	}
	return k, nil
}

// scanItems reads up to MaxItems items with HSCAN or SSCAN. COUNT is only a
// hint, so it loops, a bounded number of times.
func scanItems(ctx context.Context, cn *conn, cmd, key string, pairs bool) ([]Item, error) {
	items := []Item{}
	seen := map[string]bool{}
	cursor := "0"
	for range 20 {
		v, err := cn.do(ctx, cmd, key, cursor, "COUNT", strconv.Itoa(MaxItems))
		if err != nil {
			return nil, err
		}
		if len(v.array) != 2 {
			return nil, errors.New("Redis sent an unexpected " + cmd + " reply")
		}
		a := v.array[1].array
		step := 1
		if pairs {
			step = 2
		}
		for i := 0; i+step-1 < len(a) && len(items) < MaxItems; i += step {
			if seen[a[i].str] {
				continue // SCAN may return an element twice
			}
			seen[a[i].str] = true
			if pairs {
				it := item(a[i+1])
				f := item(a[i])
				it.Field, it.Truncated = f.Value, it.Truncated || f.Truncated
				items = append(items, it)
			} else {
				items = append(items, item(a[i]))
			}
		}
		cursor = v.array[0].str
		if cursor == "0" || len(items) >= MaxItems {
			break
		}
	}
	return items, nil
}

// Delete deletes a key.
func (c *Client) Delete(ctx context.Context, db int, key string) error {
	if key == "" || len(key) > maxKeyLen {
		return usererr.New("redis_invalid_key")
	}
	cn, err := c.open(ctx, db)
	if err != nil {
		return err
	}
	defer cn.Close()
	v, err := cn.do(ctx, "DEL", key)
	if err != nil {
		return err
	}
	if v.num == 0 {
		return usererr.New("redis_key_not_found")
	}
	return nil
}

// Flush deletes every key in a database. confirm must be the database's
// number, typed again.
func (c *Client) Flush(ctx context.Context, db int, confirm string) error {
	if db < 0 || db >= Databases {
		return usererr.New("redis_invalid_db", strconv.Itoa(db))
	}
	if strings.TrimSpace(confirm) != strconv.Itoa(db) {
		return usererr.New("redis_flush_confirm")
	}
	cn, err := c.open(ctx, db)
	if err != nil {
		return err
	}
	defer cn.Close()
	_, err = cn.do(ctx, "FLUSHDB")
	return err
}

// ---- configuration ----

// readPassword returns the last requirepass value in Redis's configuration
// file, or "" when there is none. Root can read the file.
func readPassword(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	pw := ""
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}
		if f := splitConfArgs(line); len(f) >= 2 && strings.EqualFold(f[0], "requirepass") {
			pw = f[1]
		}
	}
	return pw
}

// splitConfArgs splits a configuration line like Redis does: words are
// separated by spaces, "double quotes" allow \" \\ \n \r \t and \xHH
// escapes, and 'single quotes' allow \'.
func splitConfArgs(line string) []string {
	var out []string
	i := 0
	for {
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		if i >= len(line) {
			return out
		}
		var b strings.Builder
		switch line[i] {
		case '"':
			i++
			for i < len(line) && line[i] != '"' {
				if line[i] == '\\' && i+1 < len(line) {
					i++
					switch line[i] {
					case 'n':
						b.WriteByte('\n')
					case 'r':
						b.WriteByte('\r')
					case 't':
						b.WriteByte('\t')
					case 'x':
						if i+2 < len(line) {
							if x, err := strconv.ParseUint(line[i+1:i+3], 16, 8); err == nil {
								b.WriteByte(byte(x))
								i += 2
								break
							}
						}
						b.WriteByte('x')
					default:
						b.WriteByte(line[i])
					}
				} else {
					b.WriteByte(line[i])
				}
				i++
			}
			i++ // closing quote
		case '\'':
			i++
			for i < len(line) && line[i] != '\'' {
				if line[i] == '\\' && i+1 < len(line) && line[i+1] == '\'' {
					i++
				}
				b.WriteByte(line[i])
				i++
			}
			i++
		default:
			for i < len(line) && line[i] != ' ' && line[i] != '\t' {
				b.WriteByte(line[i])
				i++
			}
		}
		out = append(out, b.String())
	}
}
