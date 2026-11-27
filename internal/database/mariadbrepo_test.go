package database

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/pgpkey"
)

// mariaRunner is mysqlRunner playing a server with MariaDB's repository:
// it answers apt-cache policy with candidate and mariadbd --version with
// version.
type mariaRunner struct {
	*mysqlRunner
	candidate string
	maria     string // mariadbd --version
}

func (r *mariaRunner) Output(ctx context.Context, name string, args ...string) (string, error) {
	switch {
	case name == "apt-cache":
		return "mariadb-server:\n  Installed: (none)\n  Candidate: " + r.candidate + "\n  Version table:\n", nil
	case name == "mariadbd" && r.maria != "":
		return r.maria, nil
	}
	return r.mysqlRunner.Output(ctx, name, args...)
}

// newMariaTest is newMySQLTest for MariaDB, with MariaDB's key served
// from testdata.
func newMariaTest(t *testing.T, installed Engine) (*Manager, *mariaRunner, string) {
	t.Helper()
	m, mr, dir := newMySQLTest(t, installed)
	f := &mariaRunner{mysqlRunner: mr, candidate: "1:11.4.14+maria~ubu2404"}
	m.run = f
	srv := keyServer(t)
	m.mariaKeyURLs = []string{srv.URL + "/MariaDB-Server-GPG-KEY"}
	m.repo.client = srv.Client()
	os.MkdirAll(filepath.Join(m.etcDir, "mariadb.conf.d"), 0o755)
	return m, f, dir
}

func TestMariaDBChoices(t *testing.T) {
	m, _, _ := newMariaTest(t, "")
	for _, c := range []struct {
		id, codename, arch string
		want               []string // distro series, then each series' error code ("" when offered)
	}{
		{"ubuntu", "jammy", "amd64", []string{"10.6", "", "", ""}},
		{"ubuntu", "noble", "amd64", []string{"10.11", "", "", ""}},
		{"ubuntu", "noble", "arm64", []string{"10.11", "", "", ""}},
		{"debian", "bookworm", "amd64", []string{"10.11", "", "", ""}},
		// 11.4 is not published for the newest releases, whose own MariaDB is
		// 11.8 already: 12.3 is the only series there.
		{"ubuntu", "resolute", "amd64", []string{"11.8", "mariadb_series_unavailable:11.4 (ubuntu resolute)", "mariadb_series_not_newer:11.8", ""}},
		{"debian", "trixie", "amd64", []string{"11.8", "mariadb_series_unavailable:11.4 (debian trixie)", "mariadb_series_not_newer:11.8", ""}},
		{"ubuntu", "focal", "amd64", []string{"", "mariadb_series_unavailable:11.4 (ubuntu focal)", "mariadb_series_unavailable:11.8 (ubuntu focal)", "mariadb_series_unavailable:12.3 (ubuntu focal)"}},
		{"ubuntu", "noble", "riscv64", []string{"10.11", "mariadb_series_unavailable:11.4 (riscv64)", "mariadb_series_unavailable:11.8 (riscv64)", "mariadb_series_unavailable:12.3 (riscv64)"}},
	} {
		m.repo.osRelease = func() (string, string) { return c.id, c.codename }
		m.repo.arch = func() string { return c.arch }
		ch := m.MariaDBChoices()
		got := []string{ch[0].Series}
		for _, x := range ch[1:] {
			got = append(got, errCode(x.Unavailable))
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("%s %s %s: %q, want %q", c.id, c.codename, c.arch, got, c.want)
		}
		if ch[0].ID != "mariadb" || ch[1].ID != "mariadb-11.4" || !ch[1].Repo || ch[1].Until != "2029-05-29" || ch[3].ID != "mariadb-12.3" || ch[3].Until != "2029-06-12" {
			t.Errorf("choices: %+v", ch)
		}
	}
}

func TestSeries(t *testing.T) {
	for v, want := range map[string]string{"11.4.14": "11.4", "1:11.4.14+maria~ubu2404": "11.4", "1:10.11.14-0ubuntu0.24.04.1": "10.11", "8.4.7": "8.4", "": "", "x": ""} {
		if got := SeriesOf(v); got != want {
			t.Errorf("SeriesOf(%q) = %q, want %q", v, got, want)
		}
	}
	if !(compareSeries("10.6", "10.11") < 0 && compareSeries("10.11", "11.4") < 0 && compareSeries("11.8", "11.4") > 0 && compareSeries("12.3", "11.8") > 0 && compareSeries("11.4", "11.4") == 0) {
		t.Error("compareSeries")
	}
	for id, want := range map[string]string{"mariadb": "", "mariadb-11.4": "11.4", "mariadb-11.8": "11.8", "mariadb-12.3": "12.3"} {
		if got, ok := ParseMariaDBID(id); !ok || got != want {
			t.Errorf("ParseMariaDBID(%q) = %q %v", id, got, ok)
		}
	}
	for _, id := range []string{"mariadb-9.9", "mariadb-", "mysql", "mariadb-11.4;"} {
		if _, ok := ParseMariaDBID(id); ok {
			t.Errorf("ParseMariaDBID(%q) accepted", id)
		}
	}
}

func TestInstallMariaDBSeries(t *testing.T) {
	m, f, _ := newMariaTest(t, "")
	if err := m.InstallMariaDB(context.Background(), "11.4", &nopLog{}); err != nil {
		t.Fatal(err)
	}
	list, _ := os.ReadFile(m.mariaList)
	want := "deb [signed-by=" + m.mariaKeyring + "] https://dlm.mariadb.com/repo/mariadb-server/11.4/repo/ubuntu noble main\n"
	if !strings.HasSuffix(string(list), want) {
		t.Errorf("source list:\n%s\nwant it to end with:\n%s", list, want)
	}
	key, _ := os.ReadFile(m.mariaKeyring)
	if fp, err := pgpkey.Fingerprint(key); err != nil || fp != mariadbKeyFingerprint {
		t.Errorf("keyring holds %s (%v)", fp, err)
	}
	if !slices.Equal(f.apt, []string{"update", "install --no-install-recommends mariadb-server mariadb-client"}) {
		t.Errorf("apt: %q", f.apt)
	}
	if b, _ := os.ReadFile(filepath.Join(m.etcDir, "mariadb.conf.d", panelConfName)); !strings.Contains(string(b), "innodb_buffer_pool_size") {
		t.Errorf("tuning: %s", b)
	}
	if m.MariaDBRepoSeries() != "11.4" {
		t.Errorf("repo series %q", m.MariaDBRepoSeries())
	}
}

func TestInstallMariaDBSeriesNeedsMariaDBsPackages(t *testing.T) {
	// MariaDB's repository unreachable: apt would install the distribution's.
	m, f, _ := newMariaTest(t, "")
	f.candidate = "1:10.11.14-0ubuntu0.24.04.1"
	err := m.InstallMariaDB(context.Background(), "11.4", &nopLog{})
	if err == nil || !strings.Contains(err.Error(), "dlm.mariadb.com") {
		t.Fatalf("err = %v", err)
	}
	if fileExists(m.mariaList) || fileExists(m.mariaKeyring) {
		t.Error("repository left behind")
	}
	if slices.Contains(f.apt, "install --no-install-recommends mariadb-server mariadb-client") {
		t.Error("installed anyway")
	}
}

func TestInstallMariaDBRefusesOtherKey(t *testing.T) {
	m, _, _ := newMariaTest(t, "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := os.ReadFile(filepath.Join("testdata", "RPM-GPG-KEY-mysql-2025")) // a real key, but not MariaDB's
		w.Write(b)
	}))
	defer srv.Close()
	m.mariaKeyURLs = []string{srv.URL + "/key"}
	m.repo.client = srv.Client()
	err := m.InstallMariaDB(context.Background(), "11.8", &nopLog{})
	if err == nil || !strings.Contains(err.Error(), "not the expected "+mariadbKeyFingerprint) {
		t.Fatalf("err = %v", err)
	}
	if fileExists(m.mariaList) || fileExists(m.mariaKeyring) {
		t.Error("repository written for the wrong key")
	}
}

func TestInstallDistroMariaDBDropsLeftoverRepo(t *testing.T) {
	m, f, _ := newMariaTest(t, "")
	os.MkdirAll(filepath.Dir(m.mariaList), 0o755)
	os.WriteFile(m.mariaList, []byte("deb [signed-by=x] https://dlm.mariadb.com/repo/mariadb-server/11.4/repo/ubuntu noble main\n"), 0o644)
	if err := m.Install(context.Background(), &nopLog{}); err != nil {
		t.Fatal(err)
	}
	if fileExists(m.mariaList) {
		t.Error("the leftover repository would win over the distribution's packages")
	}
	if !slices.Equal(f.apt, []string{"update", "install --no-install-recommends mariadb-server mariadb-client"}) {
		t.Errorf("apt: %q", f.apt)
	}
}

func TestInstallMariaDBRefusals(t *testing.T) {
	ctx := context.Background()
	// Another series over an installed MariaDB is an upgrade.
	m, _, _ := newMariaTest(t, MariaDB)
	if err := m.CheckInstallMariaDB(ctx, "11.4"); errCode(err) != "mariadb_installed:MariaDB" {
		t.Errorf("11.4 over the distribution's: %v", err)
	}
	if err := m.CheckInstallMariaDB(ctx, ""); err != nil {
		t.Errorf("the distribution's again (repairs it): %v", err)
	}
	// A series older than or as old as the distribution's own.
	m, _, _ = newMariaTest(t, "")
	m.repo.osRelease = func() (string, string) { return "debian", "trixie" }
	if err := m.CheckInstallMariaDB(ctx, "11.8"); errCode(err) != "mariadb_series_not_newer:11.8" {
		t.Errorf("11.8 on trixie: %v", err)
	}
	if err := m.CheckInstallMariaDB(ctx, "12.3"); err != nil {
		t.Errorf("12.3 on trixie: %v", err)
	}
	// The data of a newer MariaDB.
	m, _, _ = newMariaTest(t, "")
	os.MkdirAll(m.dataDir, 0o755)
	os.WriteFile(filepath.Join(m.dataDir, "aria_log_control"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(m.dataDir, "mariadb_upgrade_info"), []byte("11.8.10-MariaDB"), 0o644)
	if err := m.Install(ctx, &nopLog{}); errCode(err) != "mariadb_datadir_newer:11.8" {
		t.Errorf("10.11 over 11.8's data: %v", err)
	}
	if err := m.CheckInstallMariaDB(ctx, "11.4"); errCode(err) != "mariadb_datadir_newer:11.8" {
		t.Errorf("11.4 over 11.8's data: %v", err)
	}
	if err := m.CheckInstallMariaDB(ctx, "11.8"); err != nil {
		t.Errorf("11.8 over 11.8's data: %v", err)
	}
	// Without the upgrade file, the redo log's header says who made it.
	os.Remove(filepath.Join(m.dataDir, "mariadb_upgrade_info"))
	os.WriteFile(filepath.Join(m.dataDir, "ib_logfile0"), append([]byte("Phys\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00MariaDB 11.8.10"), make([]byte, 100)...), 0o644)
	if got := m.dataDirSeries(); got != "11.8" {
		t.Errorf("series from the redo log: %q", got)
	}
}

func TestRemoveMariaDBRemovesItsRepo(t *testing.T) {
	m, f, _ := newRemoveTest(t)
	f.installed = "mariadb-server\tinstalled\nmariadb-client\tinstalled\n"
	os.MkdirAll(filepath.Dir(m.mariaList), 0o755)
	os.MkdirAll(filepath.Dir(m.mariaKeyring), 0o755)
	os.WriteFile(m.mariaList, []byte("deb x\n"), 0o644)
	os.WriteFile(m.mariaKeyring, []byte("key"), 0o644)
	if err := m.Remove(context.Background(), false, "", &nopLog{}); err != nil {
		t.Fatal(err)
	}
	if fileExists(m.mariaList) || fileExists(m.mariaKeyring) {
		t.Error("MariaDB's repository stayed")
	}
}

var _ io.Writer = (*nopLog)(nil)

func TestStatusSource(t *testing.T) {
	m, f, _ := newMariaTest(t, MariaDB)
	for out, want := range map[string]string{
		"mariadbd  Ver 11.4.14-MariaDB-ubu2404 for debian-linux-gnu on x86_64 (mariadb.org binary distribution)": "mariadb",
		"mariadbd  Ver 11.8.10-MariaDB-deb12 for debian-linux-gnu on x86_64":                                     "mariadb",
		"mariadbd  Ver 10.11.14-MariaDB-0ubuntu0.24.04.1 for debian-linux-gnu on x86_64 (Ubuntu 24.04)":          "distribution",
		"mariadbd  Ver 10.11.19-MariaDB-0+deb12u1 for debian-linux-gnu on x86_64 (Debian 12)":                    "distribution",
	} {
		f.maria = out
		if st := m.Status(context.Background()); st.Source != want || st.Series != SeriesOf(st.Version) || st.Series == "" {
			t.Errorf("%s: %+v", out, st)
		}
	}
}
