package redisui

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/usererr"
)

// fakeRedis is a small RESP2 server with in-memory data, enough for the
// commands the panel sends.
type fakeRedis struct {
	ln       net.Listener
	password string
	silent   bool // accept connections but never answer

	mu   sync.Mutex
	cmds []string                  // every command received, "SELECT 2"
	data map[int]map[string]any    // db -> key -> string, []string (list), map[string]string (hash), set, zset
	ttl  map[int]map[string]int64  // milliseconds
	sets map[int]map[string]bool   // keys that are sets ([]string)
	zset map[int]map[string][]pair // sorted sets
}

type pair struct{ member, score string }

func newFake(t *testing.T) *fakeRedis {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeRedis{ln: ln, data: map[int]map[string]any{}, ttl: map[int]map[string]int64{},
		sets: map[int]map[string]bool{}, zset: map[int]map[string][]pair{}}
	for i := range Databases {
		f.data[i], f.ttl[i], f.sets[i], f.zset[i] = map[string]any{}, map[string]int64{}, map[string]bool{}, map[string][]pair{}
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(c)
		}
	}()
	return f
}

func (f *fakeRedis) client(t *testing.T) *Client {
	bin := filepath.Join(t.TempDir(), "redis-server")
	os.WriteFile(bin, nil, 0o600)
	return &Client{Addr: f.ln.Addr().String(), ConfPath: filepath.Join(t.TempDir(), "none.conf"), Binary: bin, Timeout: 2 * time.Second}
}

func (f *fakeRedis) commands() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.cmds...)
}

func readCommand(r *bufio.Reader) ([]string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(line, "*") {
		return nil, fmt.Errorf("not an array: %q", line)
	}
	n, _ := strconv.Atoi(strings.TrimSpace(line[1:]))
	args := make([]string, n)
	for i := range args {
		l, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		size, _ := strconv.Atoi(strings.TrimSpace(l[1:]))
		buf := make([]byte, size+2)
		if _, err := io.ReadFull(r, buf); err != nil {
			return nil, err
		}
		args[i] = string(buf[:size])
	}
	return args, nil
}

func bulk(s string) string { return fmt.Sprintf("$%d\r\n%s\r\n", len(s), s) }

func array(items []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(items))
	for _, it := range items {
		b.WriteString(bulk(it))
	}
	return b.String()
}

func (f *fakeRedis) serve(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	db, authed := 0, f.password == ""
	for {
		args, err := readCommand(r)
		if err != nil {
			return
		}
		f.mu.Lock()
		f.cmds = append(f.cmds, strings.Join(args, " "))
		silent := f.silent
		f.mu.Unlock()
		if silent {
			continue
		}
		var reply string
		cmd := strings.ToUpper(args[0])
		switch {
		case cmd == "AUTH":
			if args[1] == f.password {
				authed, reply = true, "+OK\r\n"
			} else {
				reply = "-WRONGPASS invalid username-password pair or user is disabled.\r\n"
			}
		case !authed:
			reply = "-NOAUTH Authentication required.\r\n"
		case cmd == "SELECT":
			db, _ = strconv.Atoi(args[1])
			reply = "+OK\r\n"
		default:
			f.mu.Lock()
			reply = f.answer(db, cmd, args[1:])
			f.mu.Unlock()
		}
		c.Write([]byte(reply))
	}
}

func (f *fakeRedis) typeOf(db int, key string) string {
	switch v := f.data[db][key].(type) {
	case nil:
		return "none"
	case string:
		return "string"
	case map[string]string:
		return "hash"
	case []string:
		if f.sets[db][key] {
			return "set"
		}
		return "list"
	case []pair:
		return "zset"
	default:
		_ = v
		return "stream"
	}
}

// page returns a SCAN-like page of 150 elements per call.
func page(items []string, cursor string, per int) (string, []string) {
	start, _ := strconv.Atoi(cursor)
	end := min(start+per, len(items))
	next := strconv.Itoa(end)
	if end >= len(items) {
		next = "0"
	}
	return next, items[start:end]
}

func (f *fakeRedis) answer(db int, cmd string, a []string) string {
	data := f.data[db]
	switch cmd {
	case "INFO":
		switch a[0] {
		case "server":
			return bulk("# Server\r\nredis_version:7.0.15\r\nredis_mode:standalone\r\nuptime_in_seconds:12155\r\n")
		case "memory":
			return bulk("# Memory\r\nused_memory:1013424\r\nused_memory_human:989.67K\r\nmaxmemory:67108864\r\n")
		case "keyspace":
			var b strings.Builder
			b.WriteString("# Keyspace\r\n")
			for i := range Databases {
				if n := len(f.data[i]); n > 0 {
					fmt.Fprintf(&b, "db%d:keys=%d,expires=%d,avg_ttl=0\r\n", i, n, len(f.ttl[i]))
				}
			}
			return bulk(b.String())
		}
	case "SCAN":
		var keys []string
		for k := range data {
			if ok, _ := path.Match(a[2], k); ok {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		next, p := page(keys, a[0], 2)
		return "*2\r\n" + bulk(next) + array(p)
	case "TYPE":
		return "+" + f.typeOf(db, a[0]) + "\r\n"
	case "PTTL":
		if data[a[0]] == nil {
			return ":-2\r\n"
		}
		if ms, ok := f.ttl[db][a[0]]; ok {
			return fmt.Sprintf(":%d\r\n", ms)
		}
		return ":-1\r\n"
	case "STRLEN":
		s, _ := data[a[0]].(string)
		return fmt.Sprintf(":%d\r\n", len(s))
	case "GETRANGE":
		s, _ := data[a[0]].(string)
		start, _ := strconv.Atoi(a[1])
		end, _ := strconv.Atoi(a[2])
		end = min(end, len(s)-1)
		return bulk(s[start : end+1])
	case "LLEN", "SCARD":
		l, _ := data[a[0]].([]string)
		return fmt.Sprintf(":%d\r\n", len(l))
	case "LRANGE":
		l, _ := data[a[0]].([]string)
		end, _ := strconv.Atoi(a[2])
		return array(l[:min(end+1, len(l))])
	case "HLEN":
		h, _ := data[a[0]].(map[string]string)
		return fmt.Sprintf(":%d\r\n", len(h))
	case "HSCAN":
		h, _ := data[a[0]].(map[string]string)
		var fields []string
		for k := range h {
			fields = append(fields, k)
		}
		sort.Strings(fields)
		next, p := page(fields, a[1], 150)
		var flat []string
		for _, k := range p {
			flat = append(flat, k, h[k])
		}
		return "*2\r\n" + bulk(next) + array(flat)
	case "SSCAN":
		l, _ := data[a[0]].([]string)
		next, p := page(l, a[1], 150)
		return "*2\r\n" + bulk(next) + array(p)
	case "ZCARD":
		z, _ := data[a[0]].([]pair)
		return fmt.Sprintf(":%d\r\n", len(z))
	case "ZRANGE":
		z, _ := data[a[0]].([]pair)
		end, _ := strconv.Atoi(a[2])
		var flat []string
		for _, p := range z[:min(end+1, len(z))] {
			flat = append(flat, p.member, p.score)
		}
		return array(flat)
	case "DEL":
		if _, ok := data[a[0]]; ok {
			delete(data, a[0])
			return ":1\r\n"
		}
		return ":0\r\n"
	case "FLUSHDB":
		f.data[db] = map[string]any{}
		return "+OK\r\n"
	}
	return "-ERR unknown command '" + cmd + "'\r\n"
}

func isCode(err error, code string) bool {
	var ue *usererr.Error
	return errors.As(err, &ue) && ue.Code == code
}

func TestInfoWithPassword(t *testing.T) {
	f := newFake(t)
	f.password = `pa ss"word`
	f.data[0]["a"], f.data[3]["b"], f.data[3]["c"] = "1", "2", "3"
	f.ttl[3]["b"] = 5000
	c := f.client(t)
	os.WriteFile(c.ConfPath, []byte("# requirepass commented\nbind 127.0.0.1 -::1\nrequirepass old\nrequirepass \"pa ss\\\"word\"\n"), 0o600)

	info, err := c.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !info.Installed || !info.Running || info.Version != "7.0.15" || info.UsedMemory != 1013424 ||
		info.MaxMemory != 67108864 || info.Uptime != 12155 {
		t.Errorf("info: %+v", info)
	}
	if len(info.Databases) != Databases || info.Databases[0].Keys != 1 || info.Databases[3].Keys != 2 ||
		info.Databases[3].Expires != 1 || info.Databases[5].Keys != 0 {
		t.Errorf("keyspace: %+v", info.Databases)
	}
	if cmds := f.commands(); cmds[0] != `AUTH pa ss"word` {
		t.Errorf("commands: %q", cmds)
	}

	os.WriteFile(c.ConfPath, []byte("requirepass wrong\n"), 0o600)
	if _, err := c.Info(context.Background()); !isCode(err, "redis_auth_failed") {
		t.Errorf("wrong password: %v", err)
	}
	os.Remove(c.ConfPath)
	if _, err := c.Scan(context.Background(), 0, "", ""); !isCode(err, "redis_auth_failed") {
		t.Errorf("no password: %v", err)
	}
}

func TestNotInstalledOrNotRunning(t *testing.T) {
	f := newFake(t)
	c := f.client(t)
	c.Binary = filepath.Join(t.TempDir(), "missing")
	if info, err := c.Info(context.Background()); err != nil || info.Installed || info.Running {
		t.Errorf("not installed: %+v %v", info, err)
	}
	if _, err := c.Scan(context.Background(), 0, "*", "0"); !isCode(err, "redis_not_installed") {
		t.Errorf("scan: %v", err)
	}

	c = f.client(t)
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	c.Addr = ln.Addr().String()
	ln.Close() // nothing listens there now
	info, err := c.Info(context.Background())
	if err != nil || !info.Installed || info.Running || info.Error == "" {
		t.Errorf("not running: %+v %v", info, err)
	}
	if err := c.Delete(context.Background(), 0, "k"); !isCode(err, "redis_not_running") {
		t.Errorf("delete: %v", err)
	}
}

func TestScan(t *testing.T) {
	f := newFake(t)
	c := f.client(t)
	f.data[2]["user:1"] = "alice"
	f.data[2]["user:2"] = map[string]string{"name": "bob"}
	f.data[2]["user:3"] = []string{"x"}
	f.data[2]["session:9"] = "s"
	f.data[2]["user:\xff\xfe"] = "binary key"
	f.ttl[2]["user:1"] = 1500

	ctx := context.Background()
	var all []KeyInfo
	cursor := "0"
	for range 10 {
		res, err := c.Scan(ctx, 2, "user:*", cursor)
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, res.Keys...)
		if cursor = res.Cursor; cursor == "0" {
			break
		}
	}
	if len(all) != 4 {
		t.Fatalf("keys: %+v", all)
	}
	byName := map[string]KeyInfo{}
	for _, k := range all {
		byName[k.Name] = k
	}
	if k := byName["user:1"]; k.Type != "string" || k.TTL != 2 {
		t.Errorf("user:1: %+v", k)
	}
	if k := byName["user:2"]; k.Type != "hash" || k.TTL != -1 {
		t.Errorf("user:2: %+v", k)
	}
	if k := byName["user:3"]; k.Type != "list" {
		t.Errorf("user:3: %+v", k)
	}
	bin := byName["user:�"] // one replacement character for the run of bad bytes
	if raw, err := DecodeKey(bin.ID); err != nil || raw != "user:\xff\xfe" {
		t.Errorf("binary key ID: %q %v", raw, err)
	}
	if cmds := f.commands(); cmds[0] != "SELECT 2" || cmds[1] != "SCAN 0 MATCH user:* COUNT 100" {
		t.Errorf("commands: %q", cmds[:2])
	}

	for _, bad := range []struct{ pattern, cursor string }{{"a\nb", "0"}, {strings.Repeat("*", 300), "0"}, {"*", "-1"}, {"*", "abc"}} {
		if _, err := c.Scan(ctx, 0, bad.pattern, bad.cursor); err == nil {
			t.Errorf("accepted %+v", bad)
		}
	}
	if _, err := c.Scan(ctx, 16, "*", "0"); !isCode(err, "redis_invalid_db") {
		t.Errorf("db 16: %v", err)
	}
}

func TestGet(t *testing.T) {
	f := newFake(t)
	c := f.client(t)
	ctx := context.Background()
	f.data[0]["short"] = "hello"
	f.data[0]["big"] = strings.Repeat("a", MaxString+5000)
	hash := map[string]string{}
	for i := range 250 {
		hash[fmt.Sprintf("f%03d", i)] = fmt.Sprintf("v%d", i)
	}
	f.data[0]["hash"] = hash
	var list []string
	for i := range 300 {
		list = append(list, strconv.Itoa(i))
	}
	f.data[0]["list"] = list
	f.data[0]["set"] = []string{"a", "b", "c"}
	f.sets[0]["set"] = true
	f.data[0]["zset"] = []pair{{"low", "1"}, {"high", "2.5"}}
	f.data[0]["stream"] = struct{}{}

	k, err := c.Get(ctx, 0, "short")
	if err != nil || k.Value != "hello" || k.Size != 5 || k.Truncated || k.Type != "string" {
		t.Errorf("short: %+v %v", k, err)
	}
	k, err = c.Get(ctx, 0, "big")
	if err != nil || len(k.Value) != MaxString || !k.Truncated || k.Size != MaxString+5000 {
		t.Errorf("big: len=%d truncated=%v size=%d %v", len(k.Value), k.Truncated, k.Size, err)
	}
	k, err = c.Get(ctx, 0, "hash")
	if err != nil || len(k.Items) != MaxItems || k.Size != 250 || !k.Truncated || k.Items[0].Field != "f000" || k.Items[0].Value != "v0" {
		t.Errorf("hash: items=%d size=%d %v", len(k.Items), k.Size, err)
	}
	k, err = c.Get(ctx, 0, "list")
	if err != nil || len(k.Items) != MaxItems || k.Size != 300 || k.Items[199].Value != "199" {
		t.Errorf("list: %d %v", len(k.Items), err)
	}
	k, err = c.Get(ctx, 0, "set")
	if err != nil || len(k.Items) != 3 || k.Truncated || k.Type != "set" {
		t.Errorf("set: %+v %v", k, err)
	}
	k, err = c.Get(ctx, 0, "zset")
	if err != nil || len(k.Items) != 2 || k.Items[1].Value != "high" || k.Items[1].Score != "2.5" {
		t.Errorf("zset: %+v %v", k, err)
	}
	k, err = c.Get(ctx, 0, "stream")
	if err != nil || k.Type != "stream" || len(k.Items) != 0 {
		t.Errorf("stream: %+v %v", k, err)
	}
	if _, err := c.Get(ctx, 0, "missing"); !isCode(err, "redis_key_not_found") {
		t.Errorf("missing: %v", err)
	}
	if cmds := f.commands(); !contains(cmds, "LRANGE list 0 199") || !contains(cmds, "ZRANGE zset 0 199 WITHSCORES") ||
		!contains(cmds, "GETRANGE big 0 65535") || !contains(cmds, "HSCAN hash 150 COUNT 200") {
		t.Errorf("commands: %q", cmds)
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestDeleteAndFlush(t *testing.T) {
	f := newFake(t)
	c := f.client(t)
	ctx := context.Background()
	f.data[4]["k"] = "v"
	f.data[4]["other"] = "v"
	if err := c.Delete(ctx, 4, "k"); err != nil {
		t.Fatal(err)
	}
	if err := c.Delete(ctx, 4, "k"); !isCode(err, "redis_key_not_found") {
		t.Errorf("second delete: %v", err)
	}
	if err := c.Flush(ctx, 4, "3"); !isCode(err, "redis_flush_confirm") {
		t.Errorf("wrong confirm: %v", err)
	}
	if err := c.Flush(ctx, 4, ""); !isCode(err, "redis_flush_confirm") {
		t.Errorf("empty confirm: %v", err)
	}
	if contains(f.commands(), "FLUSHDB") {
		t.Fatal("FLUSHDB sent without confirmation")
	}
	if err := c.Flush(ctx, 4, " 4 "); err != nil {
		t.Fatal(err)
	}
	if len(f.data[4]) != 0 || !contains(f.commands(), "FLUSHDB") {
		t.Error("database not flushed")
	}
}

func TestOnlyAllowedCommands(t *testing.T) {
	f := newFake(t)
	cn, err := dial(context.Background(), f.ln.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer cn.Close()
	for _, cmd := range [][]string{{"CONFIG", "GET", "*"}, {"FLUSHALL"}, {"EVAL", "return 1", "0"}, {"SET", "a", "b"}, {}} {
		if _, err := cn.do(context.Background(), cmd...); err == nil {
			t.Errorf("%q was sent", cmd)
		}
	}
	// A refused command in a pipeline sends nothing at all.
	if _, err := cn.pipeline(context.Background(), [][]string{{"TYPE", "a"}, {"KEYS", "*"}}); err == nil {
		t.Error("pipeline with KEYS was sent")
	}
	if _, err := cn.do(context.Background(), "TYPE", "a"); err != nil {
		t.Fatalf("connection unusable after a refusal: %v", err)
	}
	if cmds := f.commands(); len(cmds) != 1 || cmds[0] != "TYPE a" {
		t.Errorf("server received %q", cmds)
	}
}

func TestTimeout(t *testing.T) {
	f := newFake(t)
	f.silent = true
	c := f.client(t)
	c.Timeout = 200 * time.Millisecond
	start := time.Now()
	_, err := c.Scan(context.Background(), 0, "*", "0")
	if err == nil {
		t.Fatal("no error from a server that never answers")
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("took %v", d)
	}
	var ne net.Error
	if !errors.As(err, &ne) || !ne.Timeout() {
		t.Errorf("err = %v, want a timeout", err)
	}
}

func TestReadValueLimits(t *testing.T) {
	big := strings.Repeat("z", maxBulkKeep+100)
	in := bulk(big) + ":42\r\n" + "*2\r\n$-1\r\n+OK\r\n" + "-ERR boom\r\n"
	r := bufio.NewReader(strings.NewReader(in))
	v, err := readValue(r, 0)
	if err != nil || !v.truncated || len(v.str) != maxBulkKeep || v.size != int64(len(big)) {
		t.Fatalf("big bulk: len=%d truncated=%v %v", len(v.str), v.truncated, err)
	}
	if v, err := readValue(r, 0); err != nil || v.num != 42 {
		t.Fatalf("integer after a truncated bulk: %+v %v", v, err)
	}
	if v, err := readValue(r, 0); err != nil || len(v.array) != 2 || !v.array[0].null || v.array[1].str != "OK" {
		t.Fatalf("array: %+v %v", v, err)
	}
	if v, err := readValue(r, 0); err != nil || v.kind != '-' || v.str != "ERR boom" {
		t.Fatalf("error: %+v %v", v, err)
	}
	for _, bad := range []string{"*99999999\r\n", "$-5\r\n", "$3\r\nabcXY", "?x\r\n", "\r\n", ":12a\r\n"} {
		if _, err := readValue(bufio.NewReader(strings.NewReader(bad)), 0); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
	deep := strings.Repeat("*1\r\n", 10) + ":1\r\n"
	if _, err := readValue(bufio.NewReader(strings.NewReader(deep)), 0); err == nil {
		t.Error("accepted deep nesting")
	}
}

func TestSplitConfArgs(t *testing.T) {
	cases := map[string][]string{
		`requirepass secret`:                 {"requirepass", "secret"},
		`requirepass   "a b\"c\\d"`:          {"requirepass", `a b"c\d`},
		`requirepass 'it\'s'`:                {"requirepass", "it's"},
		`requirepass "\x41\x42"`:             {"requirepass", "AB"},
		"requirepass\tx":                     {"requirepass", "x"},
		`requirepass ""`:                     {"requirepass", ""},
		`save 900 1`:                         {"save", "900", "1"},
		`user default on nopass ~* &* +@all`: {"user", "default", "on", "nopass", "~*", "&*", "+@all"},
	}
	for in, want := range cases {
		got := splitConfArgs(in)
		if strings.Join(got, "|") != strings.Join(want, "|") || len(got) != len(want) {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "redis.conf")
	os.WriteFile(p, []byte("# requirepass foobared\nrequirepass first\n  requirepass second  \n"), 0o600)
	if pw := readPassword(p); pw != "second" {
		t.Errorf("password %q", pw)
	}
	os.WriteFile(p, []byte("# requirepass foobared\n"), 0o600)
	if pw := readPassword(p); pw != "" {
		t.Errorf("password %q", pw)
	}
}
