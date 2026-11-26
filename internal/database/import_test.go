package database

import (
	"context"
	"errors"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/usererr"
)

// importCall is what the fake mariadb saw during an import.
type importCall struct {
	name, conf, confPath, dump string
	args                       []string
}

// fakeImport makes the fake runner's mariadb record the import and fail
// with output (when not empty). No object in the database uses the import
// account.
func fakeImport(f *fakeRunner, output string) *importCall {
	call := &importCall{}
	f.answer["SELECT @@socket"] = "/run/mysqld/mysqld.sock\n"
	f.answer["information_schema.TRIGGERS"] = "0\n"
	f.onPipe = func(ctx context.Context, stdin io.Reader, name string, args []string) error {
		call.name, call.args = name, args
		if len(args) > 0 {
			call.confPath = strings.TrimPrefix(args[0], "--defaults-file=")
			b, err := os.ReadFile(call.confPath)
			if err != nil {
				return err
			}
			call.conf = string(b)
		}
		b, _ := io.ReadAll(stdin)
		call.dump = string(b)
		if output != "" {
			return &run.Error{Cmd: name, Err: errors.New("exit status 1"), Output: output}
		}
		return nil
	}
	return call
}

const dropShopImporter = "DROP USER IF EXISTS 'nefo-import-shop'@'localhost';"

var createdPassword = regexp.MustCompile(`CREATE USER IF NOT EXISTS 'nefo-import-shop_db'@'localhost' IDENTIFIED BY '([^']+)'`)

func TestImportRunsAsLimitedUser(t *testing.T) {
	m, f := newTest(t)
	call := fakeImport(f, "")
	if err := m.Import(context.Background(), "shop_db", strings.NewReader("INSERT INTO t VALUES (1);")); err != nil {
		t.Fatal(err)
	}
	if len(f.sql) != 5 {
		t.Fatalf("SQL run: %q", f.sql)
	}
	setup := f.sql[0]
	mm := createdPassword.FindStringSubmatch(setup)
	if mm == nil || len(mm[1]) < 24 {
		t.Fatalf("no strong password in:\n%s", setup)
	}
	for _, want := range []string{
		// A leftover account gets the new password too.
		"ALTER USER 'nefo-import-shop_db'@'localhost' IDENTIFIED BY '" + mm[1] + "';",
		"GRANT ALL PRIVILEGES ON `shop\\_db`.* TO 'nefo-import-shop_db'@'localhost';",
	} {
		if !strings.Contains(setup, want) {
			t.Errorf("missing %q in:\n%s", want, setup)
		}
	}
	if strings.Contains(setup, "*.*") {
		t.Errorf("global privileges beyond SET USER:\n%s", setup)
	}
	if f.sql[1] != "ALTER USER 'nefo-import-shop_db'@'localhost' ACCOUNT UNLOCK;" {
		t.Errorf("not unlocked: %q", f.sql[1])
	}
	if f.sql[2] != "GRANT SET USER ON *.* TO 'nefo-import-shop_db'@'localhost';" {
		t.Errorf("definer grant: %q", f.sql[2])
	}
	if !strings.Contains(f.sql[3], "TRIGGER_SCHEMA = 'shop_db' AND DEFINER = 'nefo-import-shop_db@localhost'") {
		t.Errorf("no check for objects that use the account: %q", f.sql[3])
	}
	if f.sql[4] != "DROP USER IF EXISTS 'nefo-import-shop_db'@'localhost';" {
		t.Errorf("not dropped afterwards: %q", f.sql[4])
	}

	if call.name != "mariadb" || len(call.args) != 3 || !strings.HasPrefix(call.args[0], "--defaults-file=") ||
		call.args[1] != "--default-character-set=utf8mb4" || call.args[2] != "shop_db" {
		t.Fatalf("ran %s %q", call.name, call.args)
	}
	if strings.Contains(strings.Join(call.args, " "), mm[1]) {
		t.Fatal("password on the command line")
	}
	wantConf := "[client]\nuser=nefo-import-shop_db\npassword=" + mm[1] + "\nsocket=/run/mysqld/mysqld.sock\n"
	if call.conf != wantConf {
		t.Errorf("option file:\n%s\nwant:\n%s", call.conf, wantConf)
	}
	if call.dump != "INSERT INTO t VALUES (1);" {
		t.Errorf("dump not passed on: %q", call.dump)
	}
	if _, err := os.Stat(call.confPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("option file kept: %v", err)
	}
}

func TestImportFallsBackToSuper(t *testing.T) {
	m, f := newTest(t)
	fakeImport(f, "")
	f.failOn = "GRANT SET USER" // MariaDB before 10.5.2
	if err := m.Import(context.Background(), "shop", strings.NewReader("")); err != nil {
		t.Fatal(err)
	}
	if len(f.sql) != 6 || f.sql[3] != "GRANT SUPER ON *.* TO 'nefo-import-shop'@'localhost';" || f.sql[5] != dropShopImporter {
		t.Fatalf("SQL run: %q", f.sql)
	}
}

func TestImportDropsUserOnFailure(t *testing.T) {
	m, f := newTest(t)
	call := fakeImport(f, "ERROR 1064 (42000) at line 3: You have an error in your SQL syntax")
	err := m.Import(context.Background(), "shop", strings.NewReader("garbage"))
	var ue *usererr.Error
	if err == nil || errors.As(err, &ue) || !strings.Contains(err.Error(), "ERROR 1064") {
		t.Fatalf("err = %v, want mariadb's own error", err)
	}
	if last := f.sql[len(f.sql)-1]; last != dropShopImporter {
		t.Fatalf("not dropped: %q", last)
	}
	if _, err := os.Stat(call.confPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("option file kept: %v", err)
	}

	// The account is dropped when creating it fails half way, too.
	m, f = newTest(t)
	call = fakeImport(f, "")
	f.failOn = "ON *.*" // both definer grants fail
	if err := m.Import(context.Background(), "shop", strings.NewReader("")); err == nil {
		t.Fatal("no error")
	}
	if call.name != "" {
		t.Error("imported without the definer privilege")
	}
	if last := f.sql[len(f.sql)-1]; last != dropShopImporter {
		t.Fatalf("SQL run: %q", f.sql)
	}
}

func TestImportDropsUserWhenCancelled(t *testing.T) {
	m, f := newTest(t)
	call := fakeImport(f, "")
	ctx, cancel := context.WithCancel(context.Background())
	inner := f.onPipe
	f.onPipe = func(c context.Context, stdin io.Reader, name string, args []string) error {
		inner(c, stdin, name, args)
		cancel() // the user went away during the import
		return &run.Error{Cmd: name, Err: context.Canceled}
	}
	if err := m.Import(ctx, "shop", strings.NewReader("")); err == nil {
		t.Fatal("no error")
	}
	if last := f.sql[len(f.sql)-1]; last != dropShopImporter {
		t.Fatalf("not dropped: %q", last)
	}
	if _, err := os.Stat(call.confPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("option file kept: %v", err)
	}
}

// A trigger created without DEFINER runs as the import account, so the
// account stays, locked and with its database only.
func TestImportKeepsUserThatObjectsUse(t *testing.T) {
	m, f := newTest(t)
	fakeImport(f, "")
	f.answer["information_schema.TRIGGERS"] = "2\n"
	if err := m.Import(context.Background(), "shop_db", strings.NewReader("CREATE TRIGGER t ...")); err != nil {
		t.Fatal(err)
	}
	n := len(f.sql)
	want := []string{
		"REVOKE ALL PRIVILEGES, GRANT OPTION FROM 'nefo-import-shop_db'@'localhost';\nGRANT ALL PRIVILEGES ON `shop\\_db`.* TO 'nefo-import-shop_db'@'localhost';",
		"ALTER USER 'nefo-import-shop_db'@'localhost' ACCOUNT LOCK;",
	}
	if n < 2 || f.sql[n-2] != want[0] || f.sql[n-1] != want[1] {
		t.Fatalf("SQL run: %q", f.sql)
	}
	for _, q := range f.sql {
		if strings.HasPrefix(q, "DROP USER") {
			t.Fatalf("dropped: %q", q)
		}
	}

	// When MariaDB can't tell, the account is kept too.
	m, f = newTest(t)
	fakeImport(f, "")
	f.failOn = "information_schema.TRIGGERS"
	if err := m.Import(context.Background(), "shop", strings.NewReader("")); err != nil {
		t.Fatal(err)
	}
	if last := f.sql[len(f.sql)-1]; last != "ALTER USER 'nefo-import-shop'@'localhost' ACCOUNT LOCK;" {
		t.Fatalf("SQL run: %q", f.sql)
	}
}

func TestDeleteDropsImportUser(t *testing.T) {
	m, f := newTest(t)
	d, err := m.Create(context.Background(), "shop", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Delete(context.Background(), d.ID, "shop"); err != nil {
		t.Fatal(err)
	}
	if last := f.sql[len(f.sql)-1]; !strings.HasSuffix(last, dropShopImporter) {
		t.Fatalf("delete ran:\n%s", last)
	}
}

func TestImportOtherDatabase(t *testing.T) {
	for _, tc := range []struct{ output, want string }{
		{"ERROR 1044 (42000) at line 22: Access denied for user 'nefo-import-shop'@'localhost' to database 'wp_shop'", "wp_shop"},
		{"ERROR 1142 (42000) at line 9: INSERT command denied to user 'nefo-import-shop'@'localhost' for table `wp_other`.`wp_posts`", "wp_other"},
		{"ERROR 1142 (42000) at line 9: CREATE VIEW command denied to user 'nefo-import-shop'@'localhost' for table `crm`.`v`", "crm"},
		// Not about another database: mariadb's own message.
		{"ERROR 1044 (42000) at line 1: Access denied for user 'nefo-import-shop'@'localhost' to database 'shop'", ""},
		{"ERROR 1227 (42000) at line 5: Access denied; you need (at least one of) the SUPER privilege(s) for this operation", ""},
	} {
		m, f := newTest(t)
		fakeImport(f, tc.output)
		err := m.Import(context.Background(), "shop", strings.NewReader(""))
		var ue *usererr.Error
		if tc.want == "" {
			if err == nil || errors.As(err, &ue) || !strings.Contains(err.Error(), tc.output) {
				t.Errorf("%s: err = %v", tc.output, err)
			}
			continue
		}
		if !errors.As(err, &ue) || ue.Code != "import_other_db" || ue.Detail != tc.want {
			t.Errorf("%s: err = %#v", tc.output, err)
			continue
		}
		// A job log shows the sentence, not the code.
		if msg := err.Error(); !strings.Contains(msg, "another database, "+tc.want) || !strings.Contains(msg, "USE") {
			t.Errorf("message: %s", msg)
		}
	}
}

func TestImportRejectsInvalidName(t *testing.T) {
	m, f := newTest(t)
	fakeImport(f, "")
	if err := m.Import(context.Background(), "a`b", strings.NewReader("")); err == nil {
		t.Fatal("no error")
	}
	if len(f.sql) != 0 {
		t.Fatalf("SQL ran: %q", f.sql)
	}
}
