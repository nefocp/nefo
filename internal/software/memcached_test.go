package software

import (
	"bufio"
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMemcachedConfig(t *testing.T) {
	for _, tc := range []struct {
		mem       int64
		mb, conns string
	}{
		{1 << 30, "-m 64\n", "-c 512\n"},
		{2 << 30, "-m 128\n", "-c 1024\n"},
		{8 << 30, "-m 256\n", "-c 1024\n"},
	} {
		conf := MemcachedConfig(tc.mem)
		for _, want := range []string{tc.mb, tc.conns, "-U 0\n", "-l 127.0.0.1\n", "-p 11211\n", "-u memcache\n"} {
			if !strings.Contains(conf, want) {
				t.Errorf("%d MB: lacks %q:\n%s", tc.mem>>20, want, conf)
			}
		}
		// One address only, and nothing that turns UDP back on.
		var listen []string
		for _, l := range strings.Split(conf, "\n") {
			if f := strings.Fields(l); len(f) == 2 && (f[0] == "-l" || f[0] == "-U") {
				listen = append(listen, l)
			}
			if strings.HasPrefix(l, "-d") {
				t.Errorf("daemon flag: %q", l)
			}
		}
		if strings.Join(listen, ",") != "-U 0,-l 127.0.0.1" {
			t.Errorf("listen lines %v", listen)
		}
	}
}

// fakeMemcached answers "stats" and "stats settings" on 127.0.0.1.
func fakeMemcached(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				r := bufio.NewReader(c)
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					switch strings.TrimSpace(line) {
					case "stats":
						c.Write([]byte("STAT pid 42\r\nSTAT uptime 3600\r\nSTAT version 1.6.24\r\nSTAT curr_connections 3\r\n" +
							"STAT get_hits 75\r\nSTAT get_misses 25\r\nSTAT curr_items 12\r\nSTAT bytes 4096\r\n" +
							"STAT limit_maxbytes 67108864\r\nSTAT evictions 0\r\nEND\r\n"))
					case "stats settings":
						c.Write([]byte("STAT maxbytes 67108864\r\nSTAT maxconns 512\r\nSTAT tcpport 11211\r\nSTAT udpport 0\r\n" +
							"STAT inter 127.0.0.1\r\nEND\r\n"))
					default:
						c.Write([]byte("ERROR\r\n"))
					}
				}
			}()
		}
	}()
	return l.Addr().String()
}

func TestMemcachedStats(t *testing.T) {
	addr := fakeMemcached(t)
	stats, settings, err := memcachedStats(context.Background(), addr)
	if err != nil {
		t.Fatal(err)
	}
	var st MemcachedStats
	fillMemcachedStats(&st, stats, settings)
	if st.Version != "1.6.24" || st.Items != 12 || st.Bytes != 4096 || st.LimitBytes != 64<<20 || st.HitRate != 75 ||
		st.Connections != 3 || st.MaxConns != 512 || st.Listen != "127.0.0.1" || st.UDPPort != 0 || st.Uptime != 3600 {
		t.Fatalf("%+v", st)
	}
	// No gets yet: no hit rate.
	st = MemcachedStats{HitRate: -1}
	fillMemcachedStats(&st, map[string]string{"get_hits": "0", "get_misses": "0"}, nil)
	if st.HitRate != -1 {
		t.Fatalf("hit rate %v", st.HitRate)
	}
	if _, err := parseMemcachedStats(bufio.NewReader(strings.NewReader("STAT a 1\r\n"))); err == nil {
		t.Fatal("no END accepted")
	}
	if _, err := parseMemcachedStats(bufio.NewReader(strings.NewReader("SERVER_ERROR out of memory\r\n"))); err == nil {
		t.Fatal("an error answer accepted")
	}
	if m, err := parseMemcachedStats(bufio.NewReader(strings.NewReader("STAT version 1.6.9\r\nEND\r\n"))); err != nil || m["version"] != "1.6.9" {
		t.Fatalf("%v, %v", m, err)
	}
}

// The Manager reports a server that does not answer, without failing.
func TestMemcachedNotRunning(t *testing.T) {
	dir := t.TempDir()
	oldBin, oldAddr := memcachedBin, memcachedAddr
	t.Cleanup(func() { memcachedBin, memcachedAddr = oldBin, oldAddr })
	memcachedBin = filepath.Join(dir, "memcached")
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	memcachedAddr = l.Addr().String()
	l.Close() // nothing listens there now
	m := &Manager{}
	if st := m.Memcached(context.Background()); st.Installed {
		t.Fatalf("%+v", st)
	}
	os.WriteFile(memcachedBin, nil, 0o755)
	if st := m.Memcached(context.Background()); !st.Installed || st.Running || st.Error == "" {
		t.Fatalf("%+v", st)
	}
	memcachedAddr = fakeMemcached(t)
	if st := m.Memcached(context.Background()); !st.Running || st.Items != 12 {
		t.Fatalf("%+v", st)
	}
	if err := m.CheckRemoveMemcached(context.Background()); err != nil {
		t.Fatal(err)
	}
}
