package validate

import (
	"strings"
	"testing"
)

func TestDomain(t *testing.T) {
	good := []string{
		"example.com",
		"www.example.com",
		"a-b.example.co.uk",
		"xn--bcher-kva.example",
		"1password.com",
		strings.Repeat("a", 63) + ".com",
	}
	bad := []string{
		"",
		"localhost",
		"Example.com",
		"example.com.",
		".example.com",
		"a..b.com",
		"-a.com",
		"a-.com",
		"exa_mple.com",
		"example.com; rm -rf /",
		"example.com/../../etc",
		"$(whoami).com",
		"example.com\n",
		"1.2.3.4",
		"bücher.example",
		strings.Repeat("a", 64) + ".com",
		strings.Repeat("a.", 127) + "com",
	}
	for _, d := range good {
		if err := Domain(d); err != nil {
			t.Errorf("Domain(%q) = %v, want ok", d, err)
		}
	}
	for _, d := range bad {
		if Domain(d) == nil {
			t.Errorf("Domain(%q) accepted", d)
		}
	}
}

func TestSystemUsername(t *testing.T) {
	for _, u := range []string{"site1", "_svc", "web-user", "a"} {
		if err := SystemUsername(u); err != nil {
			t.Errorf("%q: %v", u, err)
		}
	}
	for _, u := range []string{"", "1site", "Root", "a b", "a;b", "../x", strings.Repeat("a", 33)} {
		if SystemUsername(u) == nil {
			t.Errorf("%q accepted", u)
		}
	}
}

func TestPassword(t *testing.T) {
	if Password("short") == nil {
		t.Error("short password accepted")
	}
	if Password(strings.Repeat("x", 73)) == nil {
		t.Error("73-byte password accepted")
	}
	if err := Password("twelve chars"); err != nil {
		t.Error(err)
	}
}

func TestPHPVersion(t *testing.T) {
	for _, v := range []string{"8.3", "7.4", "8.10"} {
		if err := PHPVersion(v); err != nil {
			t.Errorf("%q: %v", v, err)
		}
	}
	for _, v := range []string{"", "8", "8.3.1", "83", "8.3;rm", "../8.3", " 8.3", "8.3\n"} {
		if PHPVersion(v) == nil {
			t.Errorf("%q accepted", v)
		}
	}
}

func TestDBName(t *testing.T) {
	for _, n := range []string{"wp", "shop_db", "a1"} {
		if err := DBName(n); err != nil {
			t.Errorf("%q: %v", n, err)
		}
	}
	for _, n := range []string{"", "1db", "Db", "a-b", "a`b", "a'b", "mysql", "test", strings.Repeat("a", 33), "x;drop"} {
		if DBName(n) == nil {
			t.Errorf("%q accepted", n)
		}
	}
}

func TestDBPassword(t *testing.T) {
	if err := DBPassword("Abc123-xyz_789"); err != nil {
		t.Error(err)
	}
	for _, p := range []string{"short", "has space 12345", "quote'12345678", `back\slash1234`, "dollar$1234567", "semi;colon12345", strings.Repeat("a", 65)} {
		if DBPassword(p) == nil {
			t.Errorf("%q accepted", p)
		}
	}
}
