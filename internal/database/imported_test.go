package database

import (
	"context"
	"strings"
	"testing"
)

const testHash = "*2470C0C06DEE42FD1618BB99005ADCA2EC9D1E19" // the hash of "password"

func TestCreateWithUserKeepsTheHash(t *testing.T) {
	m, f := newTest(t)
	ctx := context.Background()
	d, err := m.CreateWithUser(ctx, NewDB{Name: "bob_wp", User: "bob_wpuser", Hash: strings.ToLower(testHash)})
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "bob_wp" || d.Username != "bob_wpuser" || d.Password != "" {
		t.Fatalf("created %+v", d)
	}
	last := f.sql[len(f.sql)-1]
	for _, want := range []string{
		"CREATE DATABASE `bob_wp`",
		"CREATE USER 'bob_wpuser'@'localhost' IDENTIFIED BY PASSWORD '" + testHash + "';",
		"GRANT ALL PRIVILEGES ON `bob\\_wp`.* TO 'bob_wpuser'@'localhost';",
	} {
		if !strings.Contains(last, want) {
			t.Errorf("missing %q in:\n%s", want, last)
		}
	}
	// The user name is taken now, for a database and for a user.
	if _, err := m.CreateWithUser(ctx, NewDB{Name: "bob_wpuser"}); !isCode(err, "db_exists") {
		t.Fatalf("user name as a database: %v", err)
	}
	if _, err := m.CreateWithUser(ctx, NewDB{Name: "other", User: "bob_wp"}); !isCode(err, "db_exists") {
		t.Fatalf("database name as a user: %v", err)
	}
	// Deleting drops both.
	if err := m.Delete(ctx, d.ID, "bob_wp"); err != nil {
		t.Fatal(err)
	}
	last = f.sql[len(f.sql)-1]
	if !strings.Contains(last, "DROP DATABASE IF EXISTS `bob_wp`") || !strings.Contains(last, "DROP USER IF EXISTS 'bob_wpuser'@'localhost'") {
		t.Fatalf("delete ran:\n%s", last)
	}
}

func TestCreateWithUserPasswords(t *testing.T) {
	m, f := newTest(t)
	ctx := context.Background()
	d, err := m.CreateWithUser(ctx, NewDB{Name: "shop", Password: "Known-Password-2026", Hash: testHash})
	if err != nil {
		t.Fatal(err)
	}
	if d.Username != "shop" || d.Password != "Known-Password-2026" ||
		!strings.Contains(f.sql[len(f.sql)-1], "IDENTIFIED BY 'Known-Password-2026'") {
		t.Fatalf("known password: %+v\n%s", d, f.sql[len(f.sql)-1])
	}
	d, err = m.CreateWithUser(ctx, NewDB{Name: "blog"})
	if err != nil || len(d.Password) != 24 {
		t.Fatalf("random password: %+v %v", d, err)
	}
	n := len(f.sql)
	for _, bad := range []NewDB{
		{Name: "x1", Hash: "*' OR 1=1 --"},
		{Name: "x2", Hash: "$A$005$abc"},
		{Name: "x3", Password: "short"},
		{Name: "x4", User: "a'b"},
		{Name: "x5", Password: "has'quote-0123456"},
	} {
		if _, err := m.CreateWithUser(ctx, bad); err == nil {
			t.Errorf("%+v accepted", bad)
		}
	}
	if len(f.sql) != n {
		t.Fatalf("SQL ran for invalid input: %q", f.sql[n:])
	}
}

func TestTakenNames(t *testing.T) {
	m, f := newTest(t)
	ctx := context.Background()
	if _, err := m.CreateWithUser(ctx, NewDB{Name: "shop", User: "shop_user"}); err != nil {
		t.Fatal(err)
	}
	f.answer["UNION SELECT User FROM mysql.user"] = "wordpress\nroot\nShop_Old\n"
	taken, err := m.TakenNames(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"shop", "shop_user", "wordpress", "root", "shop_old"} {
		if !taken[n] {
			t.Errorf("%s is free", n)
		}
	}
	if taken["free_name"] {
		t.Error("free_name is taken")
	}
}
