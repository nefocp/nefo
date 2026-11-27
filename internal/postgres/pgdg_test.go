package postgres

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/pgpkey"
)

// pgdgServer serves a key file as the PGDG key URL.
func pgdgServer(t *testing.T, m *Manager, file string) {
	t.Helper()
	key, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(key) }))
	t.Cleanup(srv.Close)
	m.pgdg.keyURL = srv.URL + "/ACCC4CF8.asc"
	m.pgdg.client = srv.Client()
}

// The pinned fingerprint is the key in testdata, as downloaded from
// https://www.postgresql.org/media/keys/ACCC4CF8.asc, which never expires.
func TestPGDGKey(t *testing.T) {
	armored, err := os.ReadFile(filepath.Join("testdata", "ACCC4CF8.asc"))
	if err != nil {
		t.Fatal(err)
	}
	key, err := pgpkey.Dearmor(armored)
	if err != nil {
		t.Fatal(err)
	}
	if fp, err := pgpkey.Fingerprint(key); err != nil || fp != pgdgFingerprint {
		t.Fatalf("%s %v", fp, err)
	}
	if _, never, err := pgpkey.Expiry(key); err != nil || !never {
		t.Fatalf("expiry: %v %v", never, err)
	}

	m, _, _ := newTestManager(t)
	pgdgServer(t, m, filepath.Join("testdata", "ACCC4CF8.asc"))
	if err := m.addPGDGRepo(context.Background()); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(m.pgdg.keyring); string(b) != string(key) {
		t.Fatal("keyring is not the binary key")
	}
	list, _ := os.ReadFile(m.pgdg.list)
	if want := "deb [arch=amd64 signed-by=" + m.pgdg.keyring + "] https://apt.postgresql.org/pub/repos/apt noble-pgdg main\n"; !strings.HasSuffix(string(list), want) {
		t.Fatalf("list:\n%s", list)
	}
	if err := m.removePGDGRepo(testLog{t}); err != nil || m.hasPGDGRepo() {
		t.Fatalf("not removed: %v", err)
	}
	if _, err := os.Stat(m.pgdg.keyring); !os.IsNotExist(err) {
		t.Fatal("keyring kept")
	}

	// Another key is refused, and nothing is written.
	m2, _, _ := newTestManager(t)
	pgdgServer(t, m2, filepath.Join("..", "database", "testdata", "RPM-GPG-KEY-mysql-2025"))
	if err := m2.addPGDGRepo(context.Background()); err == nil || !strings.Contains(err.Error(), "not the expected") {
		t.Fatalf("%v", err)
	}
	if m2.hasPGDGRepo() {
		t.Fatal("list written for a wrong key")
	}
}

func TestVersions(t *testing.T) {
	ctx := context.Background()
	m, _, _ := newTestManager(t)
	for codename, want := range map[string]struct {
		distro string
		pgdg   []string
	}{
		"jammy":    {"14", []string{"15", "16", "17", "18", "19"}},
		"noble":    {"16", []string{"15", "17", "18", "19"}},
		"resolute": {"18", []string{"15", "16", "17", "19"}},
		"bookworm": {"15", []string{"16", "17", "18", "19"}},
		"trixie":   {"17", []string{"15", "16", "18", "19"}},
	} {
		id := "ubuntu"
		if codename == "bookworm" || codename == "trixie" {
			id = "debian"
		}
		m.pgdg.osRelease = func() (string, string) { return id, codename }
		v := m.Versions(ctx)
		if v.Distribution != want.distro || !slices.Equal(v.PGDG, want.pgdg) || v.PGDGUnavailable != "" {
			t.Errorf("%s: %+v", codename, v)
		}
	}
	// 16 is installed (newTestManager): upgrades go to newer ones only.
	m.pgdg.osRelease = func() (string, string) { return "ubuntu", "noble" }
	if v := m.Versions(ctx); v.Release != "Ubuntu 24.04" || v.Installed != "16" || v.Source != "distribution" || !slices.Equal(v.Upgrades, []string{"17", "18", "19"}) || v.UpgradeBlocked != "" {
		t.Fatalf("installed: %+v", v)
	}
	m.pgdg.osRelease = func() (string, string) { return "ubuntu", "plucky" }
	if v := m.Versions(ctx); len(v.PGDG) != 0 || v.PGDGUnavailable != "pg_pgdg_unsupported_os" || v.PGDGDetail != "ubuntu plucky" || len(v.Upgrades) != 0 {
		t.Fatalf("unknown release: %+v", v)
	}
	m.pgdg.osRelease = func() (string, string) { return "debian", "noble" }
	if v := m.Versions(ctx); v.PGDGUnavailable != "pg_pgdg_unsupported_os" {
		t.Fatalf("mismatched ID: %+v", v)
	}
	m.pgdg.osRelease = func() (string, string) { return "ubuntu", "noble" }
	m.pgdg.arch = func() string { return "riscv64" }
	if v := m.Versions(ctx); v.PGDGUnavailable != "pg_pgdg_unsupported_arch" || v.Distribution != "16" {
		t.Fatalf("arch: %+v", v)
	}
}

// installStream fakes apt-get: installing postgresql-N (or the
// metapackage, as major) adds its binaries and a main cluster.
func installStream(m *Manager, metaMajor string) func(name string, args []string) error {
	return func(name string, args []string) error {
		if name != "/bin/sh" {
			return nil
		}
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, " install ") {
			return nil
		}
		pkg := args[len(args)-1]
		v := strings.TrimPrefix(pkg, "postgresql-")
		if pkg == "postgresql" {
			v = metaMajor
		}
		for _, p := range []string{filepath.Join(m.libDir, v, "bin", "postgres"), filepath.Join(m.mainDir(v), "postgresql.conf")} {
			os.MkdirAll(filepath.Dir(p), 0o755)
			os.WriteFile(p, []byte("port = 5432\n"), 0o644)
		}
		return nil
	}
}

func aptInstalls(f *fakeRunner) []string {
	var pkgs []string
	for _, c := range f.all() {
		if c.name == "/bin/sh" && slices.Contains(c.args, "install") {
			pkgs = append(pkgs, c.args[len(c.args)-1])
		}
	}
	return pkgs
}

func TestInstallPGDG(t *testing.T) {
	ctx := context.Background()
	m, f, _ := newTestManager(t)
	os.RemoveAll(m.libDir)
	os.RemoveAll(m.etcDir)
	m.portFree = func(int) bool { return true }
	pgdgServer(t, m, filepath.Join("testdata", "ACCC4CF8.asc"))
	f.sql = func(stdin string) (string, error) {
		if strings.HasPrefix(stdin, "SHOW server_encoding") {
			return "UTF8\n", nil
		}
		return "", nil
	}
	f.stream = installStream(m, "16")
	if err := m.Install(ctx, "18", testLog{t}); err != nil {
		t.Fatal(err)
	}
	if got := aptInstalls(f); !slices.Equal(got, []string{"postgresql-18"}) {
		t.Fatalf("installed %v, want postgresql-18 by name (never the metapackage)", got)
	}
	if !m.hasPGDGRepo() {
		t.Fatal("no repository")
	}
	if _, err := os.Stat(filepath.Join(m.mainDir("18"), "conf.d", confName)); err != nil {
		t.Fatal("not tuned")
	}

	// One major at a time.
	if err := m.Install(ctx, "17", testLog{t}); userCode(err) != "pg_other_major" {
		t.Fatalf("second major: %v", err)
	}
	// Installing again without a version repairs 18, from PGDG.
	f.calls = nil
	if err := m.Install(ctx, "", testLog{t}); err != nil {
		t.Fatal(err)
	}
	if got := aptInstalls(f); !slices.Equal(got, []string{"postgresql-18"}) {
		t.Fatalf("repair installed %v", got)
	}
	// Unknown versions are refused.
	if err := m.Install(ctx, "13", testLog{t}); userCode(err) != "pg_version_unknown" {
		t.Fatalf("13: %v", err)
	}
}

func TestInstallDistributionDropsPGDG(t *testing.T) {
	ctx := context.Background()
	m, f, _ := newTestManager(t)
	os.RemoveAll(m.libDir)
	os.RemoveAll(m.etcDir)
	m.portFree = func(int) bool { return true }
	pgdgServer(t, m, filepath.Join("testdata", "ACCC4CF8.asc"))
	if err := m.addPGDGRepo(ctx); err != nil {
		t.Fatal(err)
	}
	f.stream = installStream(m, "16")
	if err := m.Install(ctx, "", testLog{t}); err != nil {
		t.Fatal(err)
	}
	if got := aptInstalls(f); !slices.Equal(got, []string{"postgresql"}) {
		t.Fatalf("installed %v", got)
	}
	if m.hasPGDGRepo() {
		t.Fatal("the PGDG list stayed for a distribution install")
	}
}

// After a rolled-back upgrade the distribution's 16 runs next to the
// repository: repairing it installs postgresql-16 by name and keeps the
// repository.
func TestInstallRepairNextToPGDG(t *testing.T) {
	ctx := context.Background()
	m, f, _ := newTestManager(t)
	pgdgServer(t, m, filepath.Join("testdata", "ACCC4CF8.asc"))
	if err := m.addPGDGRepo(ctx); err != nil {
		t.Fatal(err)
	}
	f.stream = installStream(m, "16")
	if err := m.Install(ctx, "", testLog{t}); err != nil {
		t.Fatal(err)
	}
	if got := aptInstalls(f); !slices.Equal(got, []string{"postgresql-16"}) {
		t.Fatalf("installed %v", got)
	}
	if !m.hasPGDGRepo() {
		t.Fatal("repository removed")
	}
}

// A cluster of another major left on disk (uninstalled without deleting
// the data) is not silently replaced by a new one.
func TestInstallRefusesOldCluster(t *testing.T) {
	m, f, _ := newTestManager(t)
	os.RemoveAll(m.libDir)
	os.RemoveAll(m.etcDir)
	os.MkdirAll(filepath.Join(m.dataDir, "14", "main"), 0o700)
	os.WriteFile(filepath.Join(m.dataDir, "14", "main", "PG_VERSION"), []byte("14\n"), 0o600)
	if err := m.Install(context.Background(), "", testLog{t}); userCode(err) != "pg_old_cluster" {
		t.Fatalf("%v", err)
	}
	if len(f.all()) != 0 {
		t.Fatalf("ran %v", f.all())
	}
}

// Uninstalling removes the repository with the packages.
func TestRemoveDropsPGDG(t *testing.T) {
	ctx := context.Background()
	m, f, _ := newTestManager(t)
	pgdgServer(t, m, filepath.Join("testdata", "ACCC4CF8.asc"))
	if err := m.addPGDGRepo(ctx); err != nil {
		t.Fatal(err)
	}
	f.sql = func(stdin string) (string, error) { return "postgres\ntemplate0\ntemplate1\n", nil }
	f.output = func(name string, args []string) (string, error) {
		switch {
		case name == "systemctl" && args[0] == "is-active":
			return "active\n", nil
		case name == "dpkg-query":
			return "postgresql-16\tinstalled\npostgresql\tinstalled\n", nil
		case name == "pgrep":
			return "", nil
		}
		return "", nil
	}
	if err := m.Remove(ctx, false, "", testLog{t}); err != nil {
		t.Fatal(err)
	}
	if m.hasPGDGRepo() || fileThere(m.pgdg.keyring) {
		t.Fatal("repository kept")
	}
}

func fileThere(p string) bool { _, err := os.Stat(p); return err == nil }

func TestDistVersion(t *testing.T) {
	madison := `    libpq5 | 19.0-2.pgdg12+1 | https://apt.postgresql.org/pub/repos/apt bookworm-pgdg/main amd64 Packages
    libpq5 | 15.20-0+deb12u1 | http://deb.debian.org/debian-security bookworm-security/main amd64 Packages
    libpq5 | 15.14-0+deb12u1 | http://deb.debian.org/debian bookworm/main amd64 Packages
`
	if v := distVersion(madison); v != "15.20-0+deb12u1" {
		t.Fatalf("%q", v)
	}
	if v := distVersion("    libpq5 | 19.0-2.pgdg12+1 | https://apt.postgresql.org/pub/repos/apt bookworm-pgdg/main amd64 Packages\n"); v != "" {
		t.Fatalf("a PGDG build picked: %q", v)
	}
	if v := distVersion("    libpq5 | 1.0; rm -rf / | http://deb.debian.org/debian bookworm/main amd64 Packages\n"); v != "" {
		t.Fatalf("junk accepted: %q", v)
	}
}

// Uninstalling a PGDG PostgreSQL puts libpq5 back to the distribution's
// build when it stays installed.
func TestRemoveRestoresLibpq(t *testing.T) {
	ctx := context.Background()
	m, f, _ := newTestManager(t)
	pgdgServer(t, m, filepath.Join("testdata", "ACCC4CF8.asc"))
	if err := m.addPGDGRepo(ctx); err != nil {
		t.Fatal(err)
	}
	f.sql = func(stdin string) (string, error) { return "postgres\ntemplate0\ntemplate1\n", nil }
	f.output = func(name string, args []string) (string, error) {
		switch {
		case name == "systemctl" && args[0] == "is-active":
			return "active\n", nil
		case name == "dpkg-query" && slices.Contains(args, "libpq5"):
			return "19.0-2.pgdg24.04+1", nil
		case name == "dpkg-query":
			return "postgresql-18\tinstalled\n", nil
		case name == "apt-cache":
			return " libpq5 | 16.10-0ubuntu0.24.04.1 | http://archive.ubuntu.com/ubuntu noble-updates/main amd64 Packages\n", nil
		}
		return "", nil
	}
	if err := m.Remove(ctx, false, "", testLog{t}); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range f.all() {
		if c.name == "/bin/sh" && slices.Contains(c.args, "--allow-downgrades") {
			got = c.args[len(c.args)-1:]
		}
	}
	if !slices.Equal(got, []string{"libpq5=16.10-0ubuntu0.24.04.1"}) {
		t.Fatalf("libpq5 install: %v", got)
	}
}
