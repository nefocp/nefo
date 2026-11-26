package siteimport

import (
	"io"
	"slices"
	"sort"
	"strings"
	"testing"
	"testing/iotest"
)

const mainYAML = `---
addon_domains:
  addon.com: addon.example.com
cp_php_magic_include_path.conf: 0
main_domain: example.com
parked_domains:
  - parked.net
sub_domains:
  - addon.example.com
  - blog.example.com
`

const vhostYAML = `---
customlog:
  -
    format: combined
    target: /etc/apache2/logs/domlogs/example.com
  -
    format: "\"%{%s}t %I .\\n%{%s}t %O .\""
    target: /etc/apache2/logs/domlogs/example.com-bytes_log
documentroot: /home/bob/public_html
group: bob
hascgi: 1
homedir: /home/bob
ip: 203.0.113.10
phpopenbasedirprotect: 1
phpversion: ea-php81
port: 80
scriptalias:
  - path: /home/bob/public_html/cgi-bin
    url: /cgi-bin/
serveradmin: webmaster@example.com
serveralias: parked.net www.example.com www.parked.net mail.example.com
servername: example.com
usecanonicalname: 'Off'
user: bob
# a comment
`

func TestYAMLUserdata(t *testing.T) {
	v, err := parseYAML([]byte(mainYAML))
	if err != nil {
		t.Fatal(err)
	}
	m := yMap(v)
	if yString(m, "main_domain") != "example.com" {
		t.Fatalf("main_domain: %#v", m)
	}
	if ad := yMap(m["addon_domains"]); ad["addon.com"] != "addon.example.com" {
		t.Fatalf("addon_domains: %#v", m["addon_domains"])
	}
	if p := yStrings(m, "parked_domains"); !slices.Equal(p, []string{"parked.net"}) {
		t.Fatalf("parked: %q", p)
	}
	if s := yStrings(m, "sub_domains"); len(s) != 2 || s[1] != "blog.example.com" {
		t.Fatalf("subs: %q", s)
	}

	v, err = parseYAML([]byte(vhostYAML))
	if err != nil {
		t.Fatal(err)
	}
	m = yMap(v)
	for k, want := range map[string]string{"documentroot": "/home/bob/public_html", "phpversion": "ea-php81",
		"servername": "example.com", "usecanonicalname": "Off", "user": "bob", "homedir": "/home/bob"} {
		if got := yString(m, k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if a := yStrings(m, "serveralias"); len(a) != 4 || a[1] != "www.example.com" {
		t.Errorf("serveralias: %q", a)
	}
	logs, _ := m["customlog"].([]any)
	if len(logs) != 2 || yString(yMap(logs[1]), "format") != `"%{%s}t %I .\n%{%s}t %O ."` {
		t.Errorf("customlog: %#v", m["customlog"])
	}
	sa, _ := m["scriptalias"].([]any)
	if len(sa) != 1 || yString(yMap(sa[0]), "url") != "/cgi-bin/" {
		t.Errorf("scriptalias: %#v", m["scriptalias"])
	}
}

func TestYAMLFlowAndOddInput(t *testing.T) {
	v, err := parseYAML([]byte("addon_domains: {}\nparked_domains: []\nsub_domains: [a.example.com, 'b.example.com']\nkey:\n- x\n- y\n"))
	if err != nil {
		t.Fatal(err)
	}
	m := yMap(v)
	if len(yMap(m["addon_domains"])) != 0 || len(yStrings(m, "parked_domains")) != 0 {
		t.Fatalf("empty flows: %#v", m)
	}
	if s := yStrings(m, "sub_domains"); !slices.Equal(s, []string{"a.example.com", "b.example.com"}) {
		t.Fatalf("flow list: %q", s)
	}
	if s := yStrings(m, "key"); !slices.Equal(s, []string{"x", "y"}) {
		t.Fatalf("same-indent list: %q", s)
	}
	for _, bad := range []string{"\tkey: v", "key: 'unterminated", "[a, [b]]", "key: \"\\x\""} {
		if _, err := parseYAML([]byte(bad)); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
	// Deep nesting stops instead of overflowing the stack.
	var b strings.Builder
	for i := 0; i < 200; i++ {
		b.WriteString(strings.Repeat(" ", i) + "k:\n")
	}
	if _, err := parseYAML([]byte(b.String())); err == nil {
		t.Error("200 levels parsed")
	}
}

const mysqlSQL = `GRANT USAGE ON *.* TO 'bob'@'localhost' IDENTIFIED BY PASSWORD '*AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA';
GRANT ALL PRIVILEGES ON ` + "`bob\\_%`" + `.* TO 'bob'@'localhost';
GRANT USAGE ON *.* TO 'bob_wpuser'@'localhost' IDENTIFIED BY PASSWORD '*2470c0c06dee42fd1618bb99005adca2ec9d1e19';
GRANT ALL PRIVILEGES ON ` + "`bob\\_wp`" + `.* TO 'bob_wpuser'@'localhost';
GRANT USAGE ON *.* TO 'bob_wpuser'@'cpanel.example.net' IDENTIFIED BY PASSWORD '*BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB';
CREATE USER 'bob_m8'@'localhost' IDENTIFIED WITH 'caching_sha2_password' AS '$A$005$abcdef';
GRANT SELECT, INSERT ON ` + "`bob\\_shop`" + `.* TO ` + "`bob_m8`@`localhost`" + `;
CREATE USER IF NOT EXISTS 'bob_n8'@'localhost' IDENTIFIED WITH 'mysql_native_password' AS '*CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC';
GRANT ALL PRIVILEGES ON ` + "`bob\\_shop`" + `.* TO 'bob_n8'@'localhost';
GRANT ALL PRIVILEGES ON *.* TO 'root'@'localhost' WITH GRANT OPTION;
-- GRANT ALL ON ` + "`other`" + `.* TO 'evil'@'%';
`

func TestGrants(t *testing.T) {
	g := parseGrants([]byte(mysqlSQL))
	if u := g.usersOf("bob_wp"); !slices.Equal(u, []string{"bob", "bob_wpuser"}) {
		t.Errorf("users of bob_wp: %q", u)
	}
	if u := g.usersOf("bob_shop"); !slices.Equal(u, []string{"bob", "bob_m8", "bob_n8"}) {
		t.Errorf("users of bob_shop: %q", u)
	}
	// \_ is a literal underscore: bobxwp is not bob_wp.
	if u := g.usersOf("bobxwp"); len(u) != 0 {
		t.Errorf("users of bobxwp: %q", u)
	}
	if g.usersOf("other") != nil {
		t.Error("a commented-out grant counted")
	}
	if h := g.nativeHash("bob_wpuser"); h != "*2470C0C06DEE42FD1618BB99005ADCA2EC9D1E19" {
		t.Errorf("localhost hash: %q", h)
	}
	if h := g.nativeHash("bob_n8"); h != "*CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC" {
		t.Errorf("MySQL 8 native hash: %q", h)
	}
	if h := g.nativeHash("bob_m8"); h != "" {
		t.Errorf("caching_sha2 hash accepted: %q", h)
	}
	if !g.explicit("bob_wpuser", "bob_wp") || g.explicit("bob", "bob_wp") {
		t.Error("explicit grants")
	}
	if NativeHash("password") != "*2470C0C06DEE42FD1618BB99005ADCA2EC9D1E19" {
		t.Errorf("NativeHash = %s", NativeHash("password"))
	}
	for _, c := range []struct {
		p, s string
		ok   bool
	}{{`bob\_%`, "bob_wp", true}, {`bob\_%`, "bobwp", false}, {"bob_wp", "bobxwp", true}, {`a\%b`, "a%b", true}, {`a\%b`, "axb", false}, {"%", "", true}} {
		if likeMatch(c.p, c.s) != c.ok {
			t.Errorf("likeMatch(%q, %q) != %v", c.p, c.s, c.ok)
		}
	}
}

func fix(t *testing.T, in string) (string, SQLFixStats) {
	t.Helper()
	f := newSQLFixer(iotest.OneByteReader(strings.NewReader(in)))
	b, err := io.ReadAll(iotest.OneByteReader(f))
	if err != nil {
		t.Fatal(err)
	}
	return string(b), f.Stats
}

const mysql8Dump = "-- MySQL dump 10.13  Distrib 8.0.36, for Linux (x86_64)\n" +
	"--\n-- Host: localhost    Database: shop\n" +
	"/*!50503 SET NAMES utf8mb4 */;\n" +
	"SET @MYSQLDUMP_TEMP_LOG_BIN = @@SESSION.SQL_LOG_BIN;\n" +
	"SET @@SESSION.SQL_LOG_BIN= 0;\n" +
	"SET @@GLOBAL.GTID_PURGED=/*!80000 '+'*/ '3e11fa47-71ca-11e1-9e33-c80aa9429562:1-5,\n" +
	"4f22ab58-71ca-11e1-9e33-c80aa9429562:1-3';\n" +
	"CREATE DATABASE /*!32312 IF NOT EXISTS*/ `shop` /*!40100 DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci */ /*!80016 DEFAULT ENCRYPTION='N' */;\n" +
	"USE `shop`;\n" +
	"/*!80013 SET @OLD_SQL_REQUIRE_PRIMARY_KEY=@@SQL_REQUIRE_PRIMARY_KEY, SQL_REQUIRE_PRIMARY_KEY=0 */;\n" +
	"CREATE TABLE `places` (\n" +
	"  `id` int NOT NULL,\n" +
	"  `name` varchar(50) COLLATE utf8mb4_0900_as_cs DEFAULT NULL,\n" +
	"  `vi` varchar(50) COLLATE utf8mb4_vi_0900_ai_ci DEFAULT NULL,\n" +
	"  `pos` point NOT NULL /*!80003 SRID 4326 */,\n" +
	"  `utf8mb4_0900_ai_ci` int\n" +
	") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;\n" +
	"/*!50003 SET collation_connection  = utf8mb4_0900_ai_ci */ ;\n" +
	"INSERT INTO `places` VALUES (1,'utf8mb4_0900_ai_ci and /*!80016 x */ stay',\"it\\'s utf8mb4_0900_ai_ci\",0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef);\n" +
	"-- utf8mb4_0900_ai_ci in a comment\n" +
	"USE `other`;\n" +
	"SET @@SESSION.SQL_LOG_BIN = @MYSQLDUMP_TEMP_LOG_BIN;\n"

func TestSQLFixerMySQL8(t *testing.T) {
	out, st := fix(t, mysql8Dump)
	for _, want := range []string{
		"COLLATE utf8mb4_bin DEFAULT NULL",                    // _as_cs
		"COLLATE utf8mb4_vietnamese_ci DEFAULT NULL",          // a language
		"COLLATE=utf8mb4_unicode_520_ci;",                     // the table
		"SET collation_connection  = utf8mb4_unicode_520_ci",  // inside /*!50003 */
		"/* 80003 SRID 4326 */",                               // MySQL 8 only: a plain comment
		"/* 80013 SET @OLD_SQL_REQUIRE_PRIMARY_KEY",           //
		"`utf8mb4_0900_ai_ci` int",                            // a quoted name stays
		"'utf8mb4_0900_ai_ci and /*!80016 x */ stay'",         // string data stays
		"\"it\\'s utf8mb4_0900_ai_ci\"",                       //
		"-- utf8mb4_0900_ai_ci in a comment",                  //
		"USE `other`;",                                        // another database stays (the import refuses it)
		"/*!50503 SET NAMES utf8mb4 */;",                      // older versions stay executable
		"0x0123456789abcdef0123456789abcdef0123456789abcdef0", // a long hex value passes through
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	for _, gone := range []string{"GTID_PURGED", "SQL_LOG_BIN", "4f22ab58", "CREATE DATABASE", "USE `shop`", "ENCRYPTION='N' */;\n"} {
		if strings.Contains(out, gone) {
			t.Errorf("%q left in:\n%s", gone, out)
		}
	}
	want := SQLFixStats{Collations: 4, Comments: 2, Statements: 4, DBLines: 2}
	if st != want {
		t.Errorf("stats %+v, want %+v", st, want)
	}
	if !strings.Contains(st.String(), "4 MySQL 8 collation names replaced") {
		t.Errorf("stats text: %s", st)
	}
}

func TestSQLFixerLeavesOrdinaryDumpsAlone(t *testing.T) {
	in := "-- MariaDB dump 10.19\n/*!40101 SET NAMES utf8mb4 */;\n/*M!100616 SET @OLD_NOTE_VERBOSITY=@@NOTE_VERBOSITY */;\n" +
		"CREATE TABLE `t` (`a` text) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;\n" +
		"INSERT INTO `t` VALUES ('a--b'),('x/*y'),('USE it'),('-'),('/');\nSELECT 4-2, 6/3 -- end\n# hash comment\nselect 1;/**/\n" +
		"/*! STRAIGHT_JOIN */ /*!100100 x */ /*!8 y */"
	out, st := fix(t, in)
	if out != in {
		t.Fatalf("changed:\n%s\nwant:\n%s", out, in)
	}
	if st != (SQLFixStats{}) {
		t.Fatalf("stats %+v", st)
	}
	// Truncated input ends cleanly.
	for _, tail := range []string{"/", "/*", "/*!8001", "-", "--", "SET @@GLOBAL.GTID", "USE `x"} {
		if out, _ := fix(t, "select 1;\n"+tail); out != "select 1;\n"+tail {
			t.Errorf("tail %q: %q", tail, out)
		}
	}
}

func TestMariaCollation(t *testing.T) {
	for in, want := range map[string]string{
		"utf8mb4_0900_ai_ci": "utf8mb4_unicode_520_ci", "UTF8MB4_0900_AI_CI": "utf8mb4_unicode_520_ci",
		"utf8mb4_0900_as_ci": "utf8mb4_unicode_520_ci", "utf8mb4_0900_bin": "utf8mb4_bin",
		"utf8mb4_de_pb_0900_ai_ci": "utf8mb4_german2_ci", "utf8mb4_ja_0900_as_cs": "utf8mb4_bin",
		"utf8mb4_ru_0900_ai_ci": "utf8mb4_unicode_520_ci",
	} {
		if got, ok := mariaCollation(in); !ok || got != want {
			t.Errorf("%s -> %s %v, want %s", in, got, ok, want)
		}
	}
	for _, keep := range []string{"utf8mb4_unicode_ci", "utf8mb4_general_ci", "x0900_ai_ci", "utf8mb4_x0900_ai_ci"} {
		if _, ok := mariaCollation(keep); ok {
			t.Errorf("%s replaced", keep)
		}
	}
}

func TestCronLines(t *testing.T) {
	text := "MAILTO=\"bob@example.com\"\nSHELL=\"/bin/bash\"\n# comment\n" +
		"*/5 * * * * /usr/local/bin/php /home/bob/public_html/wp-cron.php >/dev/null 2>&1\n" +
		"0 3 * * * /opt/cpanel/ea-php74/root/usr/bin/php /home/bob/addon.com/cron.php\n" +
		"@daily /home/bob/scripts/backup.sh\n" +
		"61 * * * * /bin/true\n"
	roots := map[string][2]string{
		"/home/bob/public_html": {"example.com", "/var/www/example.com/public"},
		"/home/bob/addon.com":   {"addon.com", "/var/www/addon.com/public"},
	}
	lines := cronLines(text, "/home/bob", "example.com", roots, map[string]string{"example.com": "8.3", "addon.com": "8.1"})
	if len(lines) != 4 {
		t.Fatalf("%d lines: %+v", len(lines), lines)
	}
	if l := lines[0]; l.Schedule != "*/5 * * * *" || l.Domain != "example.com" || !l.OK ||
		l.NewCommand != "/usr/bin/php8.3 /var/www/example.com/public/wp-cron.php >/dev/null 2>&1" {
		t.Errorf("line 0: %+v", l)
	}
	if l := lines[1]; l.Domain != "addon.com" || l.NewCommand != "/usr/bin/php8.1 /var/www/addon.com/public/cron.php" {
		t.Errorf("line 1: %+v", l)
	}
	if l := lines[2]; l.Schedule != "@daily" || !slices.Contains(l.Notes, "old_home") || l.Domain != "example.com" {
		t.Errorf("line 2: %+v", l)
	}
	if lines[3].OK {
		t.Errorf("minute 61 accepted: %+v", lines[3])
	}
	if got := replacePath("/home/bob/public_html2 /home/bob/public_html/x", "/home/bob/public_html", "/new"); got != "/home/bob/public_html2 /new/x" {
		t.Errorf("replacePath: %s", got)
	}
}

func TestOldPHPAndNames(t *testing.T) {
	for in, want := range map[string]string{"ea-php81": "8.1", "alt-php74": "7.4", "ea-php56": "5.6", "inherit": "", "": "", "php8.2": "8.2"} {
		if got := oldPHP(in); got != want {
			t.Errorf("oldPHP(%q) = %q", in, got)
		}
	}
	env := Env{PHP: []string{"8.1", "8.3"}, DefaultPHP: "8.3"}
	if env.choosePHP("8.1") != "8.1" || env.choosePHP("7.4") != "8.3" || env.choosePHP("") != "8.3" {
		t.Error("choosePHP")
	}
	if (Env{}).choosePHP("8.1") != "" {
		t.Error("no PHP installed")
	}
	for in, want := range map[string]string{"bob_wp": "bob_wp", "Bob-WP.2": "bob_wp_2", "1db": "db_1db", "a-b-c": "a_b_c"} {
		if got := dbSafe(in); got != want {
			t.Errorf("dbSafe(%q) = %q", in, got)
		}
	}
	taken := map[string]bool{"shop": true}
	if got := (Env{}).freeDBName("shop", taken); got != "shop_2" {
		t.Errorf("freeDBName = %q", got)
	}
	long := strings.Repeat("a", 32)
	if got := (Env{}).freeDBName(long, map[string]bool{long: true}); len(got) > 32 || !strings.HasSuffix(got, "_2") {
		t.Errorf("long freeDBName = %q", got)
	}
	keys := sortedKeys(map[string]int{"b": 1, "a": 2})
	if !sort.StringsAreSorted(keys) {
		t.Error("sortedKeys")
	}
}

func TestSQLFixerForMySQL(t *testing.T) {
	f := newSQLFixer(strings.NewReader(mysql8Dump))
	f.mysql = true
	b, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	out := string(b)
	if !strings.Contains(out, "COLLATE=utf8mb4_0900_ai_ci;") || !strings.Contains(out, "/*!80003 SRID 4326 */") || strings.Contains(out, "GTID_PURGED") {
		t.Fatalf("MySQL mode:\n%s", out)
	}
	if f.Stats.Collations != 0 || f.Stats.Comments != 0 || f.Stats.Statements != 4 {
		t.Fatalf("stats %+v", f.Stats)
	}
}
