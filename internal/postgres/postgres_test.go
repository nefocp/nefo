package postgres

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/nefocp/nefo/internal/run"
	"github.com/nefocp/nefo/internal/store"
	"github.com/nefocp/nefo/internal/usererr"
)

// call is one program the Manager ran.
type call struct {
	name  string
	args  []string
	stdin string
}

func (c call) String() string { return c.name + " " + strings.Join(c.args, " ") }

// asPostgres reports whether the call ran psql as the postgres OS user.
func (c call) asPostgres() bool {
	return c.name == "runuser" && len(c.args) > 1 && c.args[1] == "postgres"
}

type fakeRunner struct {
	mu    sync.Mutex
	calls []call
	// sql answers what psql run as postgres gets on stdin.
	sql func(stdin string) (string, error)
	// pipe answers the other Pipe calls (psql and pg_dump as a role).
	pipe   func(c call, stdout io.Writer) error
	output func(name string, args []string) (string, error)
}

func (f *fakeRunner) record(c call) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, c)
}

func (f *fakeRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	f.record(call{name: name, args: args})
	if f.output != nil {
		return f.output(name, args)
	}
	switch {
	case strings.HasSuffix(name, "/postgres") && len(args) == 1 && args[0] == "--version":
		return "postgres (PostgreSQL) 16.10 (Ubuntu 16.10-0ubuntu0.24.04.1)\n", nil
	case name == "systemctl" && len(args) == 2 && args[0] == "is-active":
		return "active\n", nil
	}
	return "", nil
}

func (f *fakeRunner) Stream(ctx context.Context, w io.Writer, name string, args ...string) error {
	f.record(call{name: name, args: args})
	return nil
}

func (f *fakeRunner) Input(ctx context.Context, stdin []byte, name string, args ...string) (string, error) {
	f.record(call{name: name, args: args, stdin: string(stdin)})
	return "", nil
}

func (f *fakeRunner) Pipe(ctx context.Context, stdin io.Reader, stdout io.Writer, name string, args ...string) error {
	c := call{name: name, args: args}
	if stdin != nil {
		b, _ := io.ReadAll(stdin)
		c.stdin = string(b)
	}
	f.record(c)
	if c.asPostgres() {
		if f.sql == nil {
			return nil
		}
		out, err := f.sql(c.stdin)
		io.WriteString(stdout, out)
		return err
	}
	if f.pipe != nil {
		return f.pipe(c, stdout)
	}
	return nil
}

func (f *fakeRunner) all() []call {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]call(nil), f.calls...)
}

// newTestManager returns a Manager with PostgreSQL 16 "installed" in
// temporary folders and running.
func newTestManager(t *testing.T) (*Manager, *fakeRunner, *store.Store) {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	root := t.TempDir()
	f := &fakeRunner{}
	m := New(f, st, slog.New(slog.DiscardHandler))
	m.supported = func() bool { return true }
	m.memTotal = func() int64 { return 2 << 30 }
	m.libDir, m.etcDir = filepath.Join(root, "lib"), filepath.Join(root, "etc")
	m.dataDir, m.socketDir, m.tempDir = filepath.Join(root, "data"), "/var/run/postgresql", t.TempDir()
	m.wait = 0
	for _, p := range []string{filepath.Join(m.libDir, "16", "bin", "postgres"), filepath.Join(m.etcDir, "16", "main", "postgresql.conf")} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("port = 5432\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return m, f, st
}

func userCode(err error) string {
	var ue *usererr.Error
	if errors.As(err, &ue) {
		return ue.Code
	}
	return ""
}

func TestTuningByRAM(t *testing.T) {
	for _, tc := range []struct {
		mem              int64
		shared, conns    int
		work, maint, ecs int
	}{
		{512 << 20, 64, 30, 2, 16, 256},
		{1 << 30, 128, 40, 4, 32, 512},
		{0, 128, 40, 4, 32, 512}, // unknown: as 1 GB
		{2 << 30, 256, 50, 4, 64, 1024},
		{4 << 30, 512, 75, 8, 128, 2048},
		{16 << 30, 1024, 100, 16, 256, 4096},
	} {
		got := tuningFor(tc.mem)
		want := tuning{tc.shared, tc.ecs, tc.work, tc.maint, tc.conns}
		if got != want {
			t.Errorf("%d MB: %+v, want %+v", tc.mem>>20, got, want)
		}
	}
	conf := tuningFor(2 << 30).conf()
	for _, want := range []string{"listen_addresses = 'localhost'\n", "shared_buffers = 256MB\n", "effective_cache_size = 1024MB\n",
		"work_mem = 4MB\n", "maintenance_work_mem = 64MB\n", "max_connections = 50\n", "password_encryption = 'scram-sha-256'\n"} {
		if !strings.Contains(conf, want) {
			t.Errorf("conf.d drop-in lacks %q:\n%s", want, conf)
		}
	}
}

func TestHBA(t *testing.T) {
	var rules [][]string
	for _, l := range strings.Split(hbaConf, "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			rules = append(rules, strings.Fields(l))
		}
	}
	for _, r := range rules {
		method := r[len(r)-1]
		switch {
		case method == "peer":
			if r[0] != "local" || r[2] != "postgres" {
				t.Errorf("peer for more than the postgres OS user: %v", r)
			}
		case method != "scram-sha-256":
			t.Errorf("method %s: %v", method, r)
		}
		if r[0] == "host" && r[3] != "127.0.0.1/32" && r[3] != "::1/128" {
			t.Errorf("a rule for other machines: %v", r)
		}
		if r[0] == "hostssl" || r[0] == "hostnossl" || strings.Contains(strings.Join(r, " "), "replication") {
			t.Errorf("unexpected rule: %v", r)
		}
	}
	if len(rules) != 4 || strings.Join(rules[0], " ") != "local all postgres peer" || strings.Join(rules[1], " ") != "local all all scram-sha-256" {
		t.Fatalf("rules: %v", rules)
	}
}

func TestInstallWritesConfig(t *testing.T) {
	m, f, _ := newTestManager(t)
	// The cluster existed before (installing again repairs the settings).
	if err := m.Install(context.Background(), testLog{t}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(m.etcDir, "16", "main")
	b, err := os.ReadFile(filepath.Join(dir, "conf.d", confName))
	if err != nil || !strings.Contains(string(b), "shared_buffers = 256MB") || !strings.Contains(string(b), "listen_addresses = 'localhost'") {
		t.Fatalf("drop-in: %s, %v", b, err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "pg_hba.conf")); err != nil || string(b) != hbaConf {
		t.Fatalf("pg_hba.conf: %s, %v", b, err)
	}
	var restarted, dropped bool
	for _, c := range f.all() {
		s := c.String()
		restarted = restarted || s == "systemctl restart postgresql@16-main"
		dropped = dropped || c.name == "pg_dropcluster"
	}
	if !restarted || dropped {
		t.Fatalf("restarted %v, dropped a cluster that existed before %v: %v", restarted, dropped, f.all())
	}
}

// A first install is refused while another program holds port 5432.
func TestInstallPortBusy(t *testing.T) {
	m, f, _ := newTestManager(t)
	os.RemoveAll(m.etcDir) // no cluster yet
	m.portFree = func(port int) bool { return port != 5432 }
	if err := m.Install(context.Background(), testLog{t}); userCode(err) != "pg_port_busy" {
		t.Fatalf("%v", err)
	}
	if len(f.all()) != 0 {
		t.Fatalf("ran %v", f.all())
	}
}

func TestNames(t *testing.T) {
	for _, ok := range []string{"shop", "app_1", "select", "a"} {
		if q, err := ident(ok); err != nil || q != `"`+ok+`"` {
			t.Errorf("%s: %q, %v", ok, q, err)
		}
	}
	for _, bad := range []string{"", "Shop", `a"b`, "a;b", "a b", "pg_x", "postgres", "public", "template1", "all",
		"1abc", "abcdefghijabcdefghijabcdefghijabc", "mysql", "a-b", "x'"} {
		if _, err := ident(bad); userCode(err) != "invalid_db_name" {
			t.Errorf("%q accepted: %v", bad, err)
		}
	}
	for in, want := range map[string]string{"abc": "'abc'", "it's": "'it''s'", `a\b`: `E'a\\b'`, `'\'`: `E'''\\'''`} {
		if got, err := literal(in); err != nil || got != want {
			t.Errorf("literal(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := literal("a\x00b"); err == nil {
		t.Error("NUL accepted")
	}
}

func TestRoleAndDatabaseSQL(t *testing.T) {
	const pw = "Secret-Pass_123"
	q, err := roleSQL("shop", pw)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`CREATE ROLE "shop" LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS PASSWORD 'SCRAM-SHA-256$4096:`} {
		if !strings.HasPrefix(q, want) {
			t.Fatalf("%s", q)
		}
	}
	if strings.Contains(q, pw) {
		t.Fatal("the password itself is in the SQL")
	}
	db, err := databaseSQL("shop", "shop")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`CREATE DATABASE "shop" OWNER "shop" TEMPLATE template0 ENCODING 'UTF8'`, `REVOKE ALL ON DATABASE "shop" FROM PUBLIC;`} {
		if !strings.Contains(db, want) {
			t.Errorf("lacks %q:\n%s", want, db)
		}
	}
	if d, _ := dropSQL("shop", "shop"); d != "DROP DATABASE IF EXISTS \"shop\" WITH (FORCE);\nDROP ROLE IF EXISTS \"shop\";\n" {
		t.Fatalf("%q", d)
	}
	for _, f := range []func() (string, error){
		func() (string, error) { return roleSQL(`x"; DROP ROLE postgres; --`, pw) },
		func() (string, error) { return databaseSQL("ok", "pg_read_all_data") },
		func() (string, error) { return dropSQL("postgres", "postgres") },
		func() (string, error) { return passwordSQL("Bad", pw) },
	} {
		if q, err := f(); err == nil {
			t.Errorf("accepted: %s", q)
		}
	}
}

// The verifier matches RFC 7677's example: the server signature and the
// client proof of its exchange follow from StoredKey and ServerKey.
func TestSCRAMVerifier(t *testing.T) {
	salt, _ := base64.StdEncoding.DecodeString("W22ZaJ0SNY7soEsUEjb6gQ==")
	v := scramVerifierSalt("pencil", salt, 4096)
	parts := strings.Split(strings.TrimPrefix(v, "SCRAM-SHA-256$"), "$")
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "4096:W22ZaJ0SNY7soEsUEjb6gQ==") {
		t.Fatalf("format: %s", v)
	}
	keys := strings.Split(parts[1], ":")
	storedKey, _ := base64.StdEncoding.DecodeString(keys[0])
	serverKey, _ := base64.StdEncoding.DecodeString(keys[1])
	auth := "n=user,r=rOprNGfwEbeRWgbNEkqO," +
		"r=rOprNGfwEbeRWgbNEkqO%hvYDpWUa2RaTCAfuxFIlj)hNlF$k0,s=W22ZaJ0SNY7soEsUEjb6gQ==,i=4096," +
		"c=biws,r=rOprNGfwEbeRWgbNEkqO%hvYDpWUa2RaTCAfuxFIlj)hNlF$k0"
	if sig := base64.StdEncoding.EncodeToString(hmacSHA256(serverKey, auth)); sig != "6rriTRBi23WpRR/wtup+mMhUZUn/dB5nLTJRsjl95G4=" {
		t.Fatalf("server signature %s", sig)
	}
	// ClientKey = proof XOR ClientSignature, and StoredKey = H(ClientKey).
	proof, _ := base64.StdEncoding.DecodeString("dHzbZapWIk4jUhN+Ute9ytag9zjfMHgsqmmiz7AndVQ=")
	clientSig := hmacSHA256(storedKey, auth)
	clientKey := make([]byte, len(proof))
	for i := range proof {
		clientKey[i] = proof[i] ^ clientSig[i]
	}
	if sum := sha256.Sum256(clientKey); !hmac.Equal(sum[:], storedKey) {
		t.Fatal("the client proof does not match the stored key")
	}
	if a, b := scramVerifier("x-password-123"), scramVerifier("x-password-123"); a == b {
		t.Fatal("the salt is not random")
	}
}

func TestCreateDatabase(t *testing.T) {
	ctx := context.Background()
	m, f, st := newTestManager(t)
	f.sql = func(stdin string) (string, error) {
		if strings.HasPrefix(stdin, "SELECT (SELECT count(*) FROM pg_database") {
			return "0\n", nil
		}
		return "", nil
	}
	d, err := m.Create(ctx, " Shop ", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "shop" || d.Username != "shop" || d.Engine != store.EnginePostgres || len(d.Password) != 24 {
		t.Fatalf("%+v", d)
	}
	var created bool
	for _, c := range f.all() {
		if strings.Contains(c.stdin, d.Password) || strings.Contains(c.String(), d.Password) {
			t.Fatalf("the password reached psql: %s", c)
		}
		if c.asPostgres() && strings.Contains(c.stdin, `CREATE ROLE "shop"`) && strings.Contains(c.stdin, `CREATE DATABASE "shop" OWNER "shop"`) {
			created = true
			if !strings.Contains(c.String(), "-v ON_ERROR_STOP=1") || !strings.Contains(c.String(), "-d postgres") {
				t.Errorf("psql: %s", c)
			}
		}
	}
	if !created {
		t.Fatalf("no CREATE: %v", f.all())
	}
	if got, err := st.PGDatabaseByName(ctx, "shop"); err != nil || got.Password != d.Password {
		t.Fatalf("stored: %+v, %v", got, err)
	}

	// Names are shared with MariaDB, and the server's own roles count too.
	if err := st.CreateDatabase(ctx, &store.Database{Name: "wp", Username: "wp", Password: "x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Create(ctx, "wp", "", 0); userCode(err) != "db_exists" {
		t.Fatalf("a MariaDB name: %v", err)
	}
	f.sql = func(stdin string) (string, error) { return "1\n", nil }
	if _, err := m.Create(ctx, "taken", "", 0); userCode(err) != "db_exists" {
		t.Fatalf("a role that exists: %v", err)
	}
	if _, err := m.Create(ctx, "weak", "short", 0); userCode(err) != "invalid_db_password" {
		t.Fatalf("weak password: %v", err)
	}
	if _, err := m.Create(ctx, "postgres", "", 0); userCode(err) != "invalid_db_name" {
		t.Fatalf("reserved name: %v", err)
	}

	// A failed CREATE DATABASE drops the role again.
	f.sql = func(stdin string) (string, error) {
		if strings.HasPrefix(stdin, "SELECT") {
			return "0\n", nil
		}
		if strings.Contains(stdin, "CREATE DATABASE") {
			return "", &run.Error{Cmd: "runuser", Err: errors.New("exit status 3"), Output: "ERROR:  disk full"}
		}
		return "", nil
	}
	if _, err := m.Create(ctx, "broken", "", 0); err == nil {
		t.Fatal("no error")
	}
	calls := f.all()
	if last := calls[len(calls)-1]; !strings.Contains(last.stdin, `DROP ROLE IF EXISTS "broken"`) {
		t.Fatalf("not undone: %s %q", last, last.stdin)
	}
	if _, err := st.PGDatabaseByName(ctx, "broken"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("stored anyway: %v", err)
	}
}

func TestSetPasswordAndDelete(t *testing.T) {
	ctx := context.Background()
	m, f, st := newTestManager(t)
	d := &store.Database{Name: "app", Username: "app", Password: "Old-Password-1", Engine: store.EnginePostgres}
	if err := st.CreateDatabase(ctx, d); err != nil {
		t.Fatal(err)
	}
	got, err := m.SetPassword(ctx, d.ID, "New-Password-12")
	if err != nil || got.Password != "New-Password-12" {
		t.Fatalf("%+v, %v", got, err)
	}
	calls := f.all()
	if last := calls[len(calls)-1]; !strings.HasPrefix(last.stdin, `ALTER ROLE "app" PASSWORD 'SCRAM-SHA-256$`) || strings.Contains(last.stdin, "New-Password-12") {
		t.Fatalf("%q", last.stdin)
	}
	// A MariaDB database is not reachable through the PostgreSQL manager.
	maria := &store.Database{Name: "wp", Username: "wp", Password: "x"}
	if err := st.CreateDatabase(ctx, maria); err != nil {
		t.Fatal(err)
	}
	if _, err := m.SetPassword(ctx, maria.ID, ""); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("MariaDB database: %v", err)
	}
	if err := m.Delete(ctx, maria.ID, "wp"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("MariaDB database deleted: %v", err)
	}
	if err := m.Delete(ctx, d.ID, "ap"); userCode(err) != "confirm_mismatch" {
		t.Fatalf("wrong confirm: %v", err)
	}
	if err := m.Delete(ctx, d.ID, "app"); err != nil {
		t.Fatal(err)
	}
	calls = f.all()
	if last := calls[len(calls)-1]; !strings.Contains(last.stdin, `DROP DATABASE IF EXISTS "app" WITH (FORCE);`) {
		t.Fatalf("%q", last.stdin)
	}
	if _, err := st.PGDatabaseByID(ctx, d.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("record kept")
	}
}

type testLog struct{ t *testing.T }

func (l testLog) Write(p []byte) (int, error) {
	l.t.Log(strings.TrimSpace(string(p)))
	return len(p), nil
}
func (l testLog) Logf(format string, args ...any) { l.t.Logf(format, args...) }

func TestCheckRemove(t *testing.T) {
	ctx := context.Background()
	m, f, _ := newTestManager(t)
	f.sql = func(stdin string) (string, error) { return "postgres\ntemplate0\ntemplate1\nshop\n", nil }
	if err := m.CheckRemove(ctx, false, ""); userCode(err) != "pg_has_databases" {
		t.Fatalf("with a database: %v", err)
	}
	f.sql = func(stdin string) (string, error) { return "postgres\ntemplate0\ntemplate1\n", nil }
	if err := m.CheckRemove(ctx, false, ""); err != nil {
		t.Fatal(err)
	}
	if err := m.CheckRemove(ctx, true, "mariadb"); userCode(err) != "software_confirm_mismatch" {
		t.Fatalf("confirm: %v", err)
	}
	// Stopped: the server must say which databases it holds.
	f.output = func(name string, args []string) (string, error) {
		if name == "systemctl" {
			return "inactive\n", errors.New("exit status 3")
		}
		return "", nil
	}
	if err := m.CheckRemove(ctx, false, ""); userCode(err) != "pg_not_running" {
		t.Fatalf("stopped: %v", err)
	}
}

func TestParsePort(t *testing.T) {
	if p := parsePort([]byte("#port = 1\nport = 5433 # x\n")); p != 5433 {
		t.Fatal(p)
	}
	if p := parsePort([]byte("listen_addresses = 'x'\n")); p != 5432 {
		t.Fatal(p)
	}
}
