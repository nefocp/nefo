package database

import (
	"context"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nefocp/nefo/internal/store"
)

func TestPlanAccount(t *testing.T) {
	known := map[string]string{"shop": "pw"}
	long := strings.Repeat("a", 33)
	for _, c := range []struct {
		user, plugin, hash string
		to                 Engine
		how, why           string
	}{
		{"shop", "mysql_native_password", "*" + strings.Repeat("A", 40), MySQL, HowPassword, ""},
		{"shop", "caching_sha2_password", "$A$005$x", MariaDB, HowPassword, ""},
		{"reporter", "mysql_native_password", "*" + strings.Repeat("A", 40), MariaDB, HowHash, ""},
		{"reporter", "mysql_native_password", "*" + strings.Repeat("A", 40), MySQL, HowNew, "mysql_native_password"},
		{"reporter", "", "*" + strings.Repeat("B", 40), MySQL, HowNew, "mysql_native_password"},
		{"reporter", "caching_sha2_password", "$A$005$x", MariaDB, HowNew, "caching_sha2_password"},
		{"reporter", "ed25519", "x", MySQL, HowNew, "ed25519"},
		{"empty", "mysql_native_password", "", MariaDB, HowNew, "mysql_native_password"},
		{"ops", "unix_socket", "", MySQL, HowSocket, ""},
		{"ops", "auth_socket", "", MariaDB, HowSocket, ""},
		{importUserPrefix + "shop", "mysql_native_password", "*" + strings.Repeat("C", 40), MySQL, HowKept, ""},
		{importUserPrefix + "a_very_long_database_name", "mysql_native_password", "", MySQL, HowSkip, "too_long"},
		{long, "mysql_native_password", "", MySQL, HowSkip, "too_long"},
		{long, "mysql_native_password", "*" + strings.Repeat("A", 40), MariaDB, HowHash, ""},
	} {
		how, why := planAccount(c.user, c.plugin, c.hash, c.to, known)
		if how != c.how || why != c.why {
			t.Errorf("%s (%s) to %s: %s %q, want %s %q", c.user, c.plugin, c.to, how, why, c.how, c.why)
		}
	}
	for _, u := range []string{"root", "mysql", "mariadb.sys", "mysql.sys", "mysql.session", "mysql.infoschema", "debian-sys-maint", "", "PUBLIC"} {
		if !systemAccount(u) {
			t.Errorf("%q is the server's own", u)
		}
	}
}

func TestCarriedGrants(t *testing.T) {
	keep, skipped := carriedGrants([]string{
		"GRANT USAGE ON *.* TO `shop`@`localhost` IDENTIFIED BY PASSWORD '*ABCDEF'",
		"GRANT ALL PRIVILEGES ON `shop`.* TO `shop`@`localhost`",
		"GRANT SELECT, INSERT ON `shop\\_db`.`items` TO `shop`@`localhost` WITH GRANT OPTION",
		"GRANT USAGE ON *.* TO `ops`@`localhost` IDENTIFIED VIA unix_socket WITH MAX_QUERIES_PER_HOUR 10",
		"GRANT PROXY ON ''@'%' TO 'ops'@'localhost'",
		"GRANT `editor` TO `shop`@`localhost`",
		"GRANT RELOAD, PROCESS ON *.* TO `ops`@`localhost` IDENTIFIED BY PASSWORD '*AB' WITH GRANT OPTION",
		"",
	})
	wantKeep := []string{
		"GRANT ALL PRIVILEGES ON `shop`.* TO `shop`@`localhost`;",
		"GRANT SELECT, INSERT ON `shop\\_db`.`items` TO `shop`@`localhost` WITH GRANT OPTION;",
		"GRANT RELOAD, PROCESS ON *.* TO `ops`@`localhost` WITH GRANT OPTION;",
	}
	if !slices.Equal(keep, wantKeep) {
		t.Errorf("kept:\n%s", strings.Join(keep, "\n"))
	}
	if len(skipped) != 2 || !strings.HasPrefix(skipped[0], "GRANT PROXY") || !strings.Contains(skipped[1], "`editor`") {
		t.Errorf("skipped: %q", skipped)
	}
	for _, g := range keep {
		if strings.Contains(g, "IDENTIFIED") {
			t.Errorf("credentials carried: %s", g)
		}
	}
}

func TestCreateAccountSQL(t *testing.T) {
	a := account{AccountPlan: AccountPlan{User: "o'neil", Host: "203.0.113.5", How: HowPassword}, password: `pa\ss'w`}
	q, _ := createAccountSQL(a, MySQL)
	if q != `CREATE USER 'o''neil'@'203.0.113.5' IDENTIFIED BY 'pa\\ss''w';` {
		t.Errorf("password: %s", q)
	}
	a = account{AccountPlan: AccountPlan{User: "r", Host: "localhost", How: HowHash}, hash: "*abcdef", locked: true}
	q, _ = createAccountSQL(a, MariaDB)
	if q != "CREATE USER 'r'@'localhost' IDENTIFIED BY PASSWORD '*ABCDEF';\nALTER USER 'r'@'localhost' ACCOUNT LOCK;" {
		t.Errorf("hash: %s", q)
	}
	a = account{AccountPlan: AccountPlan{User: "ops", Host: "localhost", How: HowSocket}}
	if q, _ = createAccountSQL(a, MySQL); q != "CREATE USER 'ops'@'localhost' IDENTIFIED WITH auth_socket;" {
		t.Errorf("socket to MySQL: %s", q)
	}
	if q, _ = createAccountSQL(a, MariaDB); q != "CREATE USER 'ops'@'localhost' IDENTIFIED VIA unix_socket;" {
		t.Errorf("socket to MariaDB: %s", q)
	}
	a = account{AccountPlan: AccountPlan{User: "r", Host: "localhost", How: HowNew}}
	if q, _ = createAccountSQL(a, MySQL); !strings.HasPrefix(q, "CREATE USER 'r'@'localhost' IDENTIFIED BY '") || !strings.HasSuffix(q, "' ACCOUNT LOCK;") {
		t.Errorf("new: %s", q)
	}
	a.How = HowSkip
	if _, err := createAccountSQL(a, MySQL); err == nil {
		t.Error("a skipped account was created")
	}
}

func TestCompareCounts(t *testing.T) {
	before := map[string]objectCounts{"shop": {Rows: map[string]int64{"items": 5, "log": 2}, Views: 1, Triggers: 1}}
	same := map[string]objectCounts{"shop": {Rows: map[string]int64{"items": 5, "log": 2}, Views: 1, Triggers: 1}}
	if d := compareCounts(before, same); d != nil {
		t.Errorf("same: %q", d)
	}
	after := map[string]objectCounts{"shop": {Rows: map[string]int64{"items": 4, "extra": 1}, Views: 1}}
	d := compareCounts(before, after)
	want := []string{"shop: the table items has 4 rows instead of 5", "shop: the table log is missing", "shop: the table extra appeared", "shop: 0 triggers instead of 1"}
	if !slices.Equal(d, want) {
		t.Errorf("diff:\n%s", strings.Join(d, "\n"))
	}
}

func TestDumpsListAndDelete(t *testing.T) {
	m, _ := newTest(t)
	dir, err := m.newDumpDir("mariadb-10.11-to-11.4")
	if err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(dir); runtime.GOOS == "linux" && fi.Mode().Perm() != 0o700 {
		t.Errorf("mode %v", fi.Mode())
	}
	mf := &manifest{Kind: "upgrade", From: "MariaDB 10.11.14", To: "MariaDB 11.4", Created: time.Now().UTC(), Result: "done",
		Databases: []dumpedDB{{Name: "shop", File: "001.sql.gz"}}}
	writeManifest(dir, mf)
	os.WriteFile(filepath.Join(dir, "001.sql.gz"), []byte("12345"), 0o600)
	os.Mkdir(filepath.Join(m.dumpsPath(), "not-a-dump"), 0o700)
	list, err := m.ListDumps()
	if err != nil || len(list) != 1 {
		t.Fatalf("list %+v %v", list, err)
	}
	if d := list[0]; d.Kind != "upgrade" || d.Result != "done" || !slices.Equal(d.Databases, []string{"shop"}) || d.Size < 5 || d.Path != dir {
		t.Errorf("dump %+v", d)
	}
	for _, bad := range []string{"../etc", "not-a-dump", "20261127-053012-x/../../etc"} {
		if err := m.DeleteDump(bad); errCode(err) != "not_found:" {
			t.Errorf("delete %q: %v", bad, err)
		}
	}
	m.mu.Lock()
	if err := m.DeleteDump(list[0].Name); errCode(err) != "db_busy:" {
		t.Errorf("delete while busy: %v", err)
	}
	m.mu.Unlock()
	if err := m.DeleteDump(list[0].Name); err != nil || fileExists(dir) {
		t.Errorf("delete: %v", err)
	}
}

// hexOut is a row of hexRows' answer.
func hexOut(rows ...[]string) string {
	var b strings.Builder
	for _, r := range rows {
		for i, f := range r {
			if i > 0 {
				b.WriteByte('\t')
			}
			b.WriteString(strings.ToUpper(hex.EncodeToString([]byte(f))))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func TestCountObjects(t *testing.T) {
	m, f := newTest(t)
	f.answer["FROM information_schema.TABLES"] = hexOut([]string{"shop", "items", "BASE TABLE"}, []string{"shop", "cheap", "VIEW"}, []string{"shop", "we`ird\tname", "BASE TABLE"})
	f.answer["information_schema.TRIGGERS"] = hexOut([]string{"shop", "1"})
	f.answer["UNION ALL"] = hexOut([]string{"0", "42"}, []string{"1", "7"})
	f.answer["COUNT(*))) FROM `shop`.`items`"] = hexOut([]string{"0", "42"}, []string{"1", "7"})
	got, err := m.countObjects(context.Background(), MariaDB, []string{"shop"})
	if err != nil {
		t.Fatal(err)
	}
	c := got["shop"]
	if c.Views != 1 || c.Triggers != 1 || c.Rows["items"] != 42 || c.Rows["we`ird\tname"] != 7 || len(c.Rows) != 2 {
		t.Errorf("counts %+v", c)
	}
	last := f.sql[len(f.sql)-1]
	if !strings.Contains(last, "FROM `shop`.`we``ird\tname`") {
		t.Errorf("a name with a backtick is not quoted: %s", last)
	}
}

func TestCheckSwitchProblems(t *testing.T) {
	m, f := newTest(t)
	m.detect = func() Engine { return MariaDB }
	m.repo.osRelease = func() (string, string) { return "ubuntu", "noble" }
	m.repo.arch = func() string { return "amd64" }
	m.dataDir = t.TempDir()
	m.etcDir = t.TempDir()
	f.answer["SHOW DATABASES"] = "information_schema\nmysql\nshop\n-odd\n"
	f.answer["FROM mysql.user"] = hexOut([]string{"shop", "localhost", "mysql_native_password", "*" + strings.Repeat("A", 40)},
		[]string{"legacy", "localhost", "mysql_native_password", "*" + strings.Repeat("B", 40)}, []string{"root", "localhost", "", ""})
	f.answer["TABLE_TYPE <> 'VIEW'"] = hexOut([]string{"shop", "cache", "Aria", "BASE TABLE"}, []string{"shop", "seq", "", "SEQUENCE"}, []string{"shop", "items", "InnoDB", "BASE TABLE"})
	f.answer["information_schema.COLUMNS"] = hexOut([]string{"shop", "hosts", "ip", "inet6"})
	f.answer["information_schema.VIEWS"] = hexOut([]string{"shop", strings.Repeat("u", 40) + "@localhost"})
	ctx := context.Background()
	// legacy is a database of the panel whose password only its hash knows.
	if err := m.store.CreateDatabase(ctx, &store.Database{Name: "legacy", Username: "legacy"}); err != nil {
		t.Fatal(err)
	}
	if err := m.store.CreateDatabase(ctx, &store.Database{Name: "shop", Username: "shop", Password: "Secret-1"}); err != nil {
		t.Fatal(err)
	}
	p, err := m.CheckSwitch(ctx, MySQL)
	if err != nil {
		t.Fatal(err)
	}
	var codes []string
	for _, pr := range p.Problems {
		codes = append(codes, pr.Code+":"+pr.Detail)
	}
	want := []string{
		"switch_db_name:-odd",
		"switch_password_unknown:legacy",
		"switch_mysql_tables:shop.cache (Aria), shop.seq (sequence)",
		"switch_mysql_columns:shop.hosts.ip (INET6)",
		"switch_mysql_definers:shop: " + strings.Repeat("u", 40),
	}
	if !slices.Equal(codes, want) {
		t.Errorf("problems:\n%s", strings.Join(codes, "\n"))
	}
	if p.From != MariaDB || p.To != MySQL || p.ToVersion != "8.4" || len(p.Databases) != 2 {
		t.Errorf("plan %+v", p)
	}
	hows := map[string]string{}
	for _, a := range p.Accounts {
		hows[a.User] = a.How
	}
	if hows["shop"] != HowPassword || hows["legacy"] != HowNew || len(hows) != 2 {
		t.Errorf("accounts %v", hows)
	}
	if err := m.Switch(ctx, MySQL, nil, &nopLog{}); errCode(err) != "switch_db_name:-odd" {
		t.Errorf("switch with problems: %v", err)
	}
	if _, err := m.CheckSwitch(ctx, MariaDB); errCode(err) != "db_switch_same:MariaDB" {
		t.Errorf("to itself: %v", err)
	}
	m.repo.arch = func() string { return "arm64" }
	if _, err := m.CheckSwitch(ctx, MySQL); errCode(err) != "mysql_unsupported_arch:arm64" {
		t.Errorf("MySQL on arm64: %v", err)
	}
}
