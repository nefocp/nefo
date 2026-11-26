package wptoolkit

import (
	"context"
	"strings"
	"testing"
)

const importedConfig = `<?php
define( 'DB_NAME', 'bob_wp' );
define("DB_USER", "bob_wpuser");
define( 'DB_PASSWORD', 'it\'s-old' );
// define( 'DB_NAME', 'commented_out' );
define( 'DB_HOST', 'localhost:/var/lib/mysql/mysql.sock' );
$table_prefix = 'wp_';
`

func TestConfigDefineAndSetConfigDB(t *testing.T) {
	if v, ok := ConfigDefine(importedConfig, "DB_PASSWORD"); !ok || v != "it's-old" {
		t.Fatalf("DB_PASSWORD = %q %v", v, ok)
	}
	if v, _ := ConfigDefine(importedConfig, "DB_USER"); v != "bob_wpuser" {
		t.Fatalf("DB_USER = %q", v)
	}
	out, err := SetConfigDB(importedConfig, "example_wp", "example_wp", "New-Password-2026")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"define( 'DB_NAME', 'example_wp' );", "define( 'DB_USER', 'example_wp' );",
		"define( 'DB_PASSWORD', 'New-Password-2026' );", "define( 'DB_HOST', 'localhost' );",
		"// define( 'DB_NAME', 'commented_out' );", "$table_prefix = 'wp_';"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	// A kept password (only the hash is known) stays as it is.
	out, err = SetConfigDB(importedConfig, "bob_wp2", "bob_wpuser", "")
	if err != nil || !strings.Contains(out, `'it\'s-old'`) || !strings.Contains(out, "'bob_wp2'") {
		t.Fatalf("kept password: %v\n%s", err, out)
	}
	for _, bad := range [][3]string{{"a'b", "u", ""}, {"db", "u", "x' . system('id') . '"}} {
		if _, err := SetConfigDB(importedConfig, bad[0], bad[1], bad[2]); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if _, err := SetConfigDB("<?php\n", "db", "db", "New-Password-2026"); err == nil {
		t.Error("a config without DB_NAME was accepted")
	}
}

func TestReplaceInDB(t *testing.T) {
	r := &fakeRunner{}
	k, _ := newKit(t, r)
	site := newSite(t, k, "example.com", "example_com")
	if err := k.ReplaceInDB(context.Background(), site, "/home/bob/public_html", "/var/www/example.com/public", &testLog{}); err != nil {
		t.Fatal(err)
	}
	wantLines(t, r.wpLines(), []string{
		"search-replace /home/bob/public_html /var/www/example.com/public --all-tables --precise --report-changed-only",
		`search-replace \/home\/bob\/public_html \/var\/www\/example.com\/public --all-tables --precise --report-changed-only`,
	})
	for _, c := range r.calls {
		if c.user() != "example_com" {
			t.Fatalf("wp-cli ran as %q", c.user())
		}
	}
}
