package wptoolkit

import (
	"encoding/json"
	"slices"
	"testing"
	"time"
)

// Output samples have the shape wp-cli 2.12 prints: JSON on one line,
// without a newline at the end.

const pluginListJSON = `[{"name":"akismet","title":"Akismet Anti-spam: Spam Protection","status":"inactive","update":"available","version":"5.3.3","update_version":"5.4","auto_update":"off"},` +
	`{"name":"hello","title":"Hello Dolly","status":"inactive","update":"none","version":"1.7.2","update_version":"","auto_update":"off"},` +
	`{"name":"woocommerce","title":"WooCommerce","status":"active","update":"unavailable","version":"9.1.0","update_version":"9.4.0","auto_update":"on","requires":"6.5","requires_php":"8.4"},` +
	`{"name":"panel-tweaks","title":"","status":"must-use","update":"none","version":"","update_version":"","auto_update":"off"},` +
	`{"name":"object-cache.php","title":"Redis Object Cache Drop-In","status":"dropin","update":"none","version":"2.5.0","update_version":null,"auto_update":"off"}]`

const themeListJSON = `[{"name":"twentytwentyfive","title":"Twenty Twenty-Five","status":"active","update":"available","version":"1.2","update_version":"1.3","auto_update":"off"},` +
	`{"name":"twentytwentyfour","title":"Twenty Twenty-Four","status":"inactive","update":"none","version":"1.3","update_version":"","auto_update":"off"}]`

const coreUpdateJSON = `[{"version":"6.9","update_type":"major","package_url":"https:\/\/downloads.wordpress.org\/release\/wordpress-6.9.zip"},` +
	`{"version":"6.8.3","update_type":"minor","package_url":"https:\/\/downloads.wordpress.org\/release\/wordpress-6.8.3-partial-1.zip"}]`

func TestParseCoreUpdates(t *testing.T) {
	got, err := parseCoreUpdates(coreUpdateJSON)
	if err != nil || newestVersion(got) != "6.9" {
		t.Fatalf("got %v, %v", got, err)
	}
	for _, none := range []string{"[]", "", "Success: WordPress is at the latest version.\n"} {
		if got, err := parseCoreUpdates(none); err != nil || len(got) != 0 {
			t.Errorf("%q: got %v, %v", none, got, err)
		}
	}
	// A must-use plugin that prints something first.
	if got, err := parseCoreUpdates("Notice from a plugin\n" + coreUpdateJSON); err != nil || len(got) != 2 {
		t.Fatalf("with noise: %v, %v", got, err)
	}
	if _, err := parseCoreUpdates("<html>Fatal error</html>"); err == nil {
		t.Fatal("accepted HTML")
	}
}

func TestParseExtensions(t *testing.T) {
	list, err := parseExtensions(pluginListJSON)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 5 {
		t.Fatalf("got %d plugins", len(list))
	}
	a := list[0]
	if a.Name != "akismet" || a.Update != "available" || a.Version != "5.3.3" || a.UpdateVersion != "5.4" || a.Title != "Akismet Anti-spam: Spam Protection" {
		t.Fatalf("akismet = %+v", a)
	}
	if list[2].AutoUpdate != "on" || list[3].Status != "must-use" || list[4].UpdateVersion != "" {
		t.Fatalf("list = %+v", list)
	}
	// Unexpected types do not make the list unreadable.
	list, err = parseExtensions(`[{"name":"x","version":1.5,"update":false,"title":"a\u0007b"}]`)
	if err != nil || list[0].Version != "1.5" || list[0].Update != "false" || list[0].Title != "a b" {
		t.Fatalf("odd types: %+v, %v", list, err)
	}
	if _, err := parseExtensions("Error: no"); err == nil {
		t.Fatal("accepted text")
	}
}

func TestParseConfig(t *testing.T) {
	cfg, err := parseConfig(`[{"name":"WP_DEBUG","value":true,"type":"constant"},{"name":"WP_AUTO_UPDATE_CORE","value":"minor","type":"constant"},{"name":"table_prefix","value":"wp_","type":"variable"}]`)
	if err != nil {
		t.Fatal(err)
	}
	if !truthy(cfg["WP_DEBUG"]) || string(cfg["WP_AUTO_UPDATE_CORE"]) != `"minor"` {
		t.Fatalf("cfg = %v", cfg)
	}
	if _, ok := cfg["table_prefix"]; ok {
		t.Fatal("variables are not constants")
	}
	for raw, want := range map[string]bool{
		`true`: true, `false`: false, `1`: true, `0`: false, `"1"`: true, `"0"`: false, `""`: false,
		`"false"`: true, // as in PHP
		`null`:    false,
	} {
		if truthy(json.RawMessage(raw)) != want {
			t.Errorf("truthy(%s) = %v", raw, !want)
		}
	}
	if truthy(cfg["MISSING"]) {
		t.Error("a missing constant is not true")
	}
}

func TestParseUsers(t *testing.T) {
	users, err := parseUsers(`[{"ID":1,"user_login":"admin","user_email":"admin@example.com","display_name":"admin"},` +
		`{"ID":"7","user_login":"boss","user_email":"boss@example.com","display_name":"The Boss"},{"ID":0,"user_login":"bad"}]`)
	if err != nil {
		t.Fatal(err)
	}
	want := []User{{1, "admin", "admin@example.com", "admin"}, {7, "boss", "boss@example.com", "The Boss"}}
	if !slices.Equal(users, want) {
		t.Fatalf("users = %+v", users)
	}
}

func TestParseURL(t *testing.T) {
	for out, want := range map[string]string{
		`"https:\/\/example.com"`:      "https://example.com",
		`"http:\/\/example.com\/blog"`: "http://example.com/blog",
		"http://example.com\n":         "http://example.com",
		`"javascript:alert(1)"`:        "",
		`"https:\/\/user:pw@evil.com"`: "",
		`"\/relative"`:                 "",
		`""`:                           "",
	} {
		if got := parseURL(out); got != want {
			t.Errorf("parseURL(%s) = %q, want %q", out, got, want)
		}
	}
}

func TestCleanVersion(t *testing.T) {
	for out, want := range map[string]string{"6.8.3\n": "6.8.3", "6.9-RC1": "6.9-RC1", "PHP Notice: x\n6.7": "6.7", "<b>6.8</b>": ""} {
		if got := cleanVersion(out); got != want {
			t.Errorf("cleanVersion(%q) = %q", out, got)
		}
	}
	if compareVersions("6.10", "6.9.4") <= 0 || compareVersions("6.8", "6.8.0") != 0 || compareVersions("6.8.3", "6.9") >= 0 {
		t.Fatal("compareVersions")
	}
}

func TestMaintenanceActive(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	for content, want := range map[string]bool{
		"<?php $upgrading = 1799999940; ?>": true,  // wp-cli, a minute ago
		"<?php $upgrading = 1799999300; ?>": false, // older than 10 minutes
		maintenanceFile:                     true,
		"<?php $upgrading = time(); ?>":     true,
		"":                                  false,
	} {
		if got := maintenanceActive([]byte(content), now); got != want {
			t.Errorf("%q: %v", content, got)
		}
	}
	// wp-cli reads the panel's file with this expression.
	if !upgradingRe.MatchString(maintenanceFile) {
		t.Fatal("wp-cli would not understand the pinned file")
	}
}

func TestLastErrorLine(t *testing.T) {
	out := "Warning: Some notice\nError: <h1>Error establishing a database connection</h1>\n"
	if got := lastErrorLine(out); got != "Error establishing a database connection" {
		t.Fatalf("got %q", got)
	}
	if err := cliErr([]string{"core", "check-update", "--format=json"}, cliFail("No luck.")); ErrorText(err) != "No luck." ||
		err.Error() != "wp core check-update: No luck." {
		t.Fatalf("cliErr = %v", err)
	}
}
