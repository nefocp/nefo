package postgres

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/store"
)

// connParam returns a parameter of the libpq connection string in args.
func connParam(args []string, key string) string {
	for _, a := range args {
		a = strings.TrimPrefix(a, "--dbname=")
		if !strings.Contains(a, "passfile=") {
			continue
		}
		for _, kv := range strings.Fields(a) {
			if v, ok := strings.CutPrefix(kv, key+"="); ok {
				return strings.Trim(v, "'")
			}
		}
	}
	return ""
}

func addPGDatabase(t *testing.T, st *store.Store, name, password string) *store.Database {
	t.Helper()
	d := &store.Database{Name: name, Username: name, Password: password, Engine: store.EnginePostgres}
	if err := st.CreateDatabase(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestImportRunsAsOwner(t *testing.T) {
	ctx := context.Background()
	m, f, st := newTestManager(t)
	addPGDatabase(t, st, "shop", "Pass:word\\x-123")
	var got call
	var passfile string
	f.pipe = func(c call, stdout io.Writer) error {
		got = c
		b, err := os.ReadFile(connParam(c.args, "passfile"))
		if err != nil {
			t.Errorf("password file: %v", err)
		}
		passfile = string(b)
		return nil
	}
	dump := "--\n-- PostgreSQL database dump\n--\n\n\\restrict AbC123xyz\n\nSET transaction_timeout = 0;\nSET client_encoding = 'UTF8';\n" +
		"CREATE TABLE t (a text);\nCOPY public.t (a) FROM stdin;\nx\n\\.\n\n\\unrestrict AbC123xyz\n"
	if err := m.Import(ctx, "shop", strings.NewReader(dump)); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(filepath.ToSlash(got.name), "/16/bin/psql") {
		t.Fatalf("ran %s", got)
	}
	args := " " + strings.Join(got.args, " ") + " "
	for _, want := range []string{" -X ", " ON_ERROR_STOP=1 ", " --single-transaction ", " -f - ", " --no-password "} {
		if !strings.Contains(args, want) {
			t.Errorf("psql lacks %q: %s", want, got)
		}
	}
	if u := connParam(got.args, "user"); u != "shop" {
		t.Fatalf("imported as %q", u)
	}
	if strings.Contains(args, "postgres") && !strings.Contains(args, "/var/run/postgresql") {
		t.Fatalf("as postgres: %s", got)
	}
	if passfile != "*:*:shop:shop:Pass\\:word\\\\x-123\n" {
		t.Fatalf("password file %q", passfile)
	}
	if _, err := os.Stat(connParam(got.args, "passfile")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the password file was left behind")
	}
	lines := strings.Split(got.stdin, "\n")
	if !strings.HasPrefix(lines[0], `\restrict `) || len(lines[0]) != len(`\restrict `)+32 {
		t.Fatalf("first line %q", lines[0])
	}
	// The dump's own \restrict, \unrestrict and SET transaction_timeout
	// lines are blank, so the lines still count the same.
	body := strings.Join(lines[1:], "\n")
	if strings.Contains(body, "restrict") || strings.Contains(body, "transaction_timeout") {
		t.Fatalf("not filtered:\n%s", body)
	}
	if strings.Count(body, "\n") != strings.Count(dump, "\n") || !strings.Contains(body, "COPY public.t (a) FROM stdin;\nx\n\\.\n") {
		t.Fatalf("changed:\n%s", body)
	}
	if strings.Contains(got.stdin, "Pass:word") || strings.Contains(got.String(), "Pass:word") {
		t.Fatal("the password is in psql's input or arguments")
	}
	// Afterwards the role gets the panel's password again.
	calls := f.all()
	if last := calls[len(calls)-1]; !last.asPostgres() || !strings.HasPrefix(last.stdin, `ALTER ROLE "shop" PASSWORD 'SCRAM-SHA-256$`) {
		t.Fatalf("last call %s %q", last, last.stdin)
	}
}

func TestDumpRunsAsOwner(t *testing.T) {
	m, f, st := newTestManager(t)
	addPGDatabase(t, st, "shop", "Secret-Pass_123")
	f.pipe = func(c call, stdout io.Writer) error {
		io.WriteString(stdout, "-- dump of "+connParam(c.args, "dbname")+" as "+connParam(c.args, "user")+"\n")
		return nil
	}
	var buf bytes.Buffer
	if err := m.Dump(context.Background(), "shop", &buf); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "-- dump of shop as shop\n" {
		t.Fatalf("%q", buf.String())
	}
	c := f.all()[len(f.all())-1]
	args := strings.Join(c.args, " ")
	if !strings.HasSuffix(filepath.ToSlash(c.name), "/16/bin/pg_dump") || !strings.Contains(args, "--no-owner") || !strings.Contains(args, "--no-privileges") ||
		!strings.Contains(args, "--format=plain") || strings.Contains(args, "Secret") {
		t.Fatalf("%s", c)
	}
	if err := m.Dump(context.Background(), "missing", &buf); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("unknown database: %v", err)
	}
	if err := m.Dump(context.Background(), "../x", &buf); userCode(err) != "invalid_db_name" {
		t.Fatalf("bad name: %v", err)
	}
}

func TestImportRefusals(t *testing.T) {
	ctx := context.Background()
	m, f, st := newTestManager(t)
	addPGDatabase(t, st, "shop", "Secret-Pass_123")
	for _, tc := range []struct {
		stderr, code, detail string
	}{
		{"psql:<stdin>:6: ERROR:  permission denied to create role", "pg_import_role", "5: permission denied to create role"},
		{"psql:<stdin>:3: ERROR:  must be superuser to create procedural language \"plperlu\"", "pg_import_role", ""},
		{"psql:<stdin>:40: ERROR:  role \"bob\" does not exist", "pg_import_role", "39: role \"bob\" does not exist"},
		{"psql:<stdin>:12: ERROR:  must be owner of extension plpgsql", "pg_import_role", ""},
		{"psql:<stdin>:2: ERROR:  permission denied to set role \"postgres\"", "pg_import_role", ""},
		{"psql:<stdin>:4: ERROR:  must be able to SET ROLE \"postgres\"", "pg_import_role", ""},
		{"psql:<stdin>:4: ERROR:  must be member of role \"postgres\"", "pg_import_role", ""},
		{"psql:<stdin>:5: ERROR:  permission denied to COPY to or from an external program", "pg_import_role", ""},
		{"psql:<stdin>:5: ERROR:  permission denied for function lo_import", "pg_import_role", ""},
		{"psql:<stdin>:8: ERROR:  permission denied to create database", "pg_import_role", ""},
		{"psql:<stdin>:9: error: backslash commands are restricted; only \\unrestrict is allowed", "pg_import_meta", "8: backslash commands are restricted; only \\unrestrict is allowed"},
		{"psql:<stdin>:1: error: invalid command \\restrict", "pg_import_psql_old", ""},
		{"psql:<stdin>:7: ERROR:  syntax error at or near \"CREAT\"", "pg_import_failed", "6: syntax error at or near \"CREAT\""},
	} {
		f.pipe = func(c call, stdout io.Writer) error {
			return &run.Error{Cmd: c.name, Err: errors.New("exit status 3"), Output: "NOTICE:  x\n" + tc.stderr + "\n"}
		}
		err := m.Import(ctx, "shop", strings.NewReader("SELECT 1;\n"))
		if userCode(err) != tc.code {
			t.Errorf("%s: %v", tc.stderr, err)
			continue
		}
		var ie *ImportError
		if !errors.As(err, &ie) || (tc.detail != "" && ie.ue.Detail != tc.detail) {
			t.Errorf("%s: detail %+v", tc.stderr, ie)
		}
	}
	// The password is not set again after a failed import.
	n := 0
	for _, c := range f.all() {
		if c.asPostgres() && strings.HasPrefix(c.stdin, "ALTER ROLE") {
			n++
		}
	}
	if n != 0 {
		t.Fatalf("%d password resets", n)
	}
}

func TestImportFilter(t *testing.T) {
	long := strings.Repeat("x", 200<<10)
	in := "\\restrict Abc\nSET transaction_timeout = 0;\n" + long + "\n\\restrict Abc\n\\unrestrict Abc\n\\! rm -rf /\nlast"
	read := func(major int) string {
		r := importFilter(strings.NewReader(in), major)
		defer r.Close()
		b, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	got := read(16)
	want := "\n\n" + long + "\n\n\n\\! rm -rf /\nlast"
	if got != want {
		t.Fatalf("got %d bytes, want %d: %.80q", len(got), len(want), got)
	}
	// Servers from 17 on know transaction_timeout.
	if got := read(17); !strings.HasPrefix(got, "\nSET transaction_timeout = 0;\n") {
		t.Fatalf("%.80q", got)
	}
	// \restrict further down is left for psql to refuse.
	var b strings.Builder
	for i := 0; i < headerLines; i++ {
		b.WriteString("SELECT 1;\n")
	}
	b.WriteString("\\restrict Abc\n")
	r := importFilter(strings.NewReader(b.String()), 16)
	out, _ := io.ReadAll(r)
	r.Close()
	if !strings.HasSuffix(string(out), "\\restrict Abc\n") {
		t.Fatal("a late \\restrict was taken out")
	}
	// Closing early stops the copy.
	r = importFilter(strings.NewReader(strings.Repeat("SELECT 1;\n", 100000)), 16)
	buf := make([]byte, 10)
	io.ReadFull(r, buf)
	r.Close()
}
