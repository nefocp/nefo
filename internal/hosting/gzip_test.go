package hosting

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The gzip part of Ubuntu 24.04's /etc/nginx/nginx.conf.
const ubuntuNginxConf = `http {
	##
	# Gzip Settings
	##

	gzip on;

	# gzip_vary on;
	# gzip_proxied any;
	# gzip_comp_level 6;
	# gzip_buffers 16 8k;
	# gzip_http_version 1.1;
	# gzip_types text/plain text/css application/json application/javascript text/xml application/xml application/xml+rss text/javascript;

	include /etc/nginx/conf.d/*.conf;
	include /etc/nginx/sites-enabled/*;
}
`

func names(lines []string) []string {
	var out []string
	for _, l := range lines {
		out = append(out, strings.Fields(l)[0])
	}
	return out
}

func TestGzipDirectives(t *testing.T) {
	all := []string{"gzip", "gzip_vary", "gzip_proxied", "gzip_comp_level", "gzip_min_length", "gzip_types"}
	for _, c := range []struct {
		name, conf string
		want       []string
	}{
		{"Ubuntu and Debian: gzip on, the rest commented out", ubuntuNginxConf, all[1:]},
		{"no gzip at all", "http {\n include /etc/nginx/conf.d/*.conf;\n}\n", all},
		{"the administrator set types and a level", "http {\n gzip on;\n gzip_types text/css; # mine\n\tgzip_comp_level 9;\n}\n",
			[]string{"gzip_vary", "gzip_proxied", "gzip_min_length"}},
		{"gzip off stays the administrator's choice", "gzip off;\n", all[1:]},
		{"a name inside a comment or a longer word does not count", "# gzip_vary on;\nmy_gzip_vary on;\n", all},
	} {
		if got := names(gzipDirectives(c.conf)); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	for _, l := range gzipDirectives("") {
		if !strings.HasSuffix(l, ";") || strings.Contains(l, "text/html") {
			t.Errorf("bad directive %q", l)
		}
	}
}

// The hook file reads nginx.conf and the other files of conf.d, never itself.
func TestNginxHTTPConf(t *testing.T) {
	m := newTestManager(t)
	m.paths.Nginx = t.TempDir() // never the real /etc/nginx
	n := nginxServer{m}
	dir := m.paths.Nginx
	os.MkdirAll(filepath.Join(dir, "conf.d"), 0o755)
	os.WriteFile(filepath.Join(dir, "nginx.conf"), []byte("gzip on;\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "conf.d", "other.conf"), []byte("gzip_types text/css;\n"), 0o644)
	os.WriteFile(n.hookFile(), []byte("gzip_vary on;\n"), 0o644)
	got := names(gzipDirectives(n.httpConf()))
	if want := []string{"gzip_vary", "gzip_proxied", "gzip_comp_level", "gzip_min_length"}; !slices.Equal(got, want) {
		t.Fatalf("%v, want %v", got, want)
	}
}
