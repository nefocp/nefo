package software

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCandidates(t *testing.T) {
	out := `php8.3-imagick:
  Installed: (none)
  Candidate: 3.7.0-4ubuntu3
  Version table:
     3.7.0-4ubuntu3 500
php8.3-mongodb:
  Installed: (none)
  Candidate: (none)
php8.3-redis:
  Installed: 5.3.7+4.3.0-3
  Candidate: 5.3.7+4.3.0-3
`
	got := parseCandidates(out)
	if !got["php8.3-imagick"] || got["php8.3-mongodb"] || !got["php8.3-redis"] {
		t.Fatalf("got %v", got)
	}
}

func TestIniRules(t *testing.T) {
	old := fileExists
	defer func() { fileExists = old }()
	fileExists = func(p string) bool { return p == "/usr/share/zoneinfo/Asia/Ho_Chi_Minh" }
	for k, cases := range map[string]map[string]bool{
		"max_input_vars":      {"5000": true, "50": false, "5000; x": false},
		"date.timezone":       {"Asia/Ho_Chi_Minh": true, "Mars/Base": false, "../../etc/passwd": false},
		"display_errors":      {"On": true, "on": false, "1": false},
		"realpath_cache_size": {"4096K": true, "16M": true, "1G": false, "4096K\nx=1": false},
	} {
		for v, ok := range cases {
			if iniRules[k].check(v) != ok {
				t.Errorf("%s = %q: got %v", k, v, !ok)
			}
		}
	}
}

func TestReadIni(t *testing.T) {
	p := filepath.Join(t.TempDir(), "99-panel.ini")
	os.WriteFile(p, []byte("; header\nmax_input_vars = 5000\nunknown_key = 1\n;opcache.enable = 0\ndate.timezone = \"UTC\"\n"), 0o644)
	got := readIni(p)
	if len(got) != 2 || got["max_input_vars"] != "5000" || got["date.timezone"] != "UTC" {
		t.Fatalf("got %v", got)
	}
}
