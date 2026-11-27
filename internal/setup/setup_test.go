package setup

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/usererr"
)

var phpVersions = []string{"7.4", "8.0", "8.1", "8.2", "8.3", "8.4", "8.5"}

// facts is an Ubuntu 24.04 server with nothing installed.
func facts(memMB, usedMB int) Facts {
	return Facts{
		Supported:   true,
		Server:      Server{CPUs: 1, MemTotalMB: memMB, MemUsedMB: usedMB, DiskTotalMB: 25000, DiskFreeMB: 20000, Arch: "amd64"},
		DistroPHP:   "8.3",
		PHPRepo:     "ppa:ondrej/php",
		PHPVersions: phpVersions,
		OLS:         true,
		Installed:   map[string]bool{},
		Blocked:     map[string]Reason{},
		RAM:         map[string]int{},
		RAMMax:      map[string]bool{"redis": true},
	}
}

func ids(e Env) []string {
	var out []string
	for _, p := range e.Parts {
		out = append(out, p.ID)
	}
	return out
}

func stepIDs(p Plan) []string {
	var out []string
	for _, s := range p.Steps {
		out = append(out, s.ID)
	}
	return out
}

func TestNewEnvOffersWhatTheServerSupports(t *testing.T) {
	all := NewEnv(facts(2048, 300))
	want := []string{"nginx", "apache", "openlitespeed", "php7.4", "php8.0", "php8.1", "php8.2", "php8.3", "php8.4", "php8.5",
		"mariadb", "mysql", "postgresql", "redis", "memcached", "nodejs", "python", "docker", "adminer", "waf", "clamav"}
	if got := ids(all); !reflect.DeepEqual(got, want) {
		t.Fatalf("parts on a 2 GB amd64 server:\n got %v\nwant %v", got, want)
	}

	// A CPU without Node.js builds or LiteSpeed packages, and 1 GB of RAM.
	f := facts(960, 250)
	f.Server.Arch = "riscv64"
	got := ids(NewEnv(f))
	for _, id := range []string{"openlitespeed", "nodejs", "clamav"} {
		if slices.Contains(got, id) {
			t.Errorf("%s is offered on riscv64 with 960 MB: %v", id, got)
		}
	}
	if !slices.Contains(got, "docker") || !slices.Contains(got, "php8.5") {
		t.Errorf("parts missing: %v", got)
	}

	// A release LiteSpeed has no packages for, on arm64.
	f = facts(4096, 300)
	f.Server.Arch, f.OLS = "arm64", false
	got = ids(NewEnv(f))
	if slices.Contains(got, "openlitespeed") || !slices.Contains(got, "nodejs") || !slices.Contains(got, "clamav") {
		t.Errorf("arm64 without LiteSpeed packages: %v", got)
	}

	// Versions past their end of life are left to the Software page,
	// unless one is installed.
	f = facts(2048, 300)
	f.PHPVersions = append([]string{"5.6", "7.3"}, phpVersions...)
	f.EOL = map[string]bool{"php5.6": true, "php7.3": true}
	f.Installed["php7.3"] = true
	got = ids(NewEnv(f))
	if slices.Contains(got, "php5.6") || !slices.Contains(got, "php7.3") {
		t.Errorf("end-of-life versions: %v", got)
	}
	if p, _ := NewEnv(f).Part("php7.3"); !p.EOL {
		t.Errorf("PHP 7.3 is not marked: %+v", p)
	}

	// Something installed is always shown, even if it would not be offered.
	f = facts(960, 250)
	f.Installed["clamav"] = true
	if !slices.Contains(ids(NewEnv(f)), "clamav") {
		t.Error("installed ClamAV is hidden on a small server")
	}

	// Only what the Software page lists: a panel whose catalog has no MySQL,
	// PostgreSQL or Memcached does not offer them.
	f = facts(2048, 300)
	f.Catalog = []string{"php8.3", "mariadb", "redis", "nodejs", "python", "docker", "adminer", "waf", "clamav"}
	got = ids(NewEnv(f))
	for _, id := range []string{"mysql", "postgresql", "memcached"} {
		if slices.Contains(got, id) {
			t.Errorf("%s is offered without a Software item: %v", id, got)
		}
	}
	if !slices.Contains(got, "mariadb") || !slices.Contains(got, "nginx") || !slices.Contains(got, "php8.5") {
		t.Errorf("parts missing with a catalog: %v", got)
	}

	// One database server per server.
	f = facts(2048, 300)
	f.Installed["mariadb"] = true
	if p, _ := NewEnv(f).Part("mysql"); p.Blocked == nil || p.Blocked.Code != "setup_db_conflict" || p.Blocked.Detail != "MariaDB" {
		t.Errorf("MySQL next to MariaDB: %+v", p)
	}

	if e := NewEnv(Facts{Supported: false, PHPVersions: phpVersions}); len(e.Parts) != 0 {
		t.Errorf("an unsupported server offers %v", ids(e))
	}
}

func TestNewEnvCosts(t *testing.T) {
	f := facts(2048, 300)
	f.RAM["mariadb"], f.RAM["redis"] = 356, 128
	e := NewEnv(f)
	nginx, _ := e.Part("nginx")
	if nginx.RAMMB != 15+25 || nginx.DiskMB != diskMB[Nginx]+diskMB["php"] || nginx.PHP != "8.3" {
		t.Errorf("Nginx with the distribution's PHP: %+v", nginx)
	}
	if p, _ := e.Part("mariadb"); p.RAMMB != 356 {
		t.Errorf("MariaDB's figure from the Software page: %+v", p)
	}
	if p, _ := e.Part("redis"); p.RAMMB != 128 || !p.RAMMax {
		t.Errorf("Redis: %+v", p)
	}
	if p, _ := e.Part("php8.5"); !p.ThirdParty || p.RAMMB != 25 {
		t.Errorf("PHP 8.5: %+v", p)
	}
	if p, _ := e.Part("php8.3"); p.ThirdParty {
		t.Errorf("the distribution's PHP is third party: %+v", p)
	}
	if p, _ := e.Part("waf"); !p.NeedsWeb || p.RAMMB != 115 {
		t.Errorf("WAF: %+v", p)
	}
	// Once the distribution's PHP is installed, the web server costs only itself.
	f.Installed["php8.3"] = true
	if p, _ := NewEnv(f).Part("nginx"); p.RAMMB != 15 || p.DiskMB != diskMB[Nginx] {
		t.Errorf("Nginx when PHP 8.3 is installed: %+v", p)
	}
}

func TestDefaultsFollowTheRAM(t *testing.T) {
	for _, tc := range []struct {
		name        string
		mem, used   int
		web         string
		php, extras []string
		db          string
		installed   []string
		blocked     string
		managedWeb  string
		eol         []string
		noDistro    bool
	}{
		{name: "1 GB", mem: 960, used: 250, web: "nginx", php: []string{"8.3"}, db: "mariadb", extras: []string{}},
		{name: "2 GB", mem: 1950, used: 300, web: "nginx", php: []string{"8.3"}, db: "mariadb", extras: []string{"redis"}},
		{name: "8 GB", mem: 7900, used: 400, web: "nginx", php: []string{"8.3"}, db: "mariadb", extras: []string{"redis"}},
		// 512 MB: Nginx with its PHP is all that fits.
		{name: "512 MB", mem: 480, used: 150, web: "nginx", php: []string{}, extras: []string{}},
		// Nginx can't be installed next to an Apache that the panel doesn't manage.
		{name: "Apache already there", mem: 960, used: 250, blocked: "nginx", web: "apache", php: []string{"8.3"}, db: "mariadb", extras: []string{}},
		{name: "a managed web server", mem: 960, used: 250, managedWeb: "openlitespeed", installed: []string{"openlitespeed", "php8.3"},
			web: "openlitespeed", php: []string{"8.3"}, db: "mariadb", extras: []string{}},
		// The database server that is installed stays the choice.
		{name: "MySQL installed", mem: 1950, used: 300, installed: []string{"mysql"}, web: "nginx", php: []string{"8.3"}, db: "mysql", extras: []string{"redis"}},
		// The distribution's PHP is the choice, even when newer versions are
		// there; without one, the newest that is not end of life.
		{name: "newest is end of life", mem: 960, used: 250, eol: []string{"php8.5", "php8.0"}, web: "nginx", php: []string{"8.3"}, db: "mariadb", extras: []string{}},
		{name: "no distribution PHP", mem: 960, used: 250, noDistro: true, eol: []string{"php8.5", "php8.0"}, web: "nginx", php: []string{"8.4"}, db: "mariadb", extras: []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := facts(tc.mem, tc.used)
			f.WebServer = tc.managedWeb
			if tc.noDistro {
				f.DistroPHP = ""
			}
			for _, id := range tc.installed {
				f.Installed[id] = true
			}
			f.EOL = map[string]bool{}
			for _, id := range tc.eol {
				f.EOL[id] = true
			}
			if tc.blocked != "" {
				f.Blocked[tc.blocked] = Reason{Code: "web_server_conflict", Detail: "apache"}
				f.Blocked["openlitespeed"] = Reason{Code: "web_server_conflict", Detail: "apache"}
			}
			if tc.mem >= 1500 {
				f.RAM["mariadb"], f.RAM["redis"] = 356, 128
			}
			e := NewEnv(f)
			c := e.Defaults()
			if c.Web != tc.web || !reflect.DeepEqual(c.PHP, tc.php) || c.DB != tc.db || !reflect.DeepEqual(c.Extras, tc.extras) {
				t.Fatalf("defaults %+v, want web %q php %v db %q extras %v", c, tc.web, tc.php, tc.db, tc.extras)
			}
			if slices.Contains(c.Extras, "docker") {
				t.Fatal("Docker is pre-selected")
			}
			p, err := e.Plan(c)
			if err != nil {
				t.Fatal(err)
			}
			if slices.Contains(p.Warnings, "ram") {
				t.Fatalf("the recommended choice does not fit: %+v", p)
			}
		})
	}
	if c := NewEnv(Facts{}).Defaults(); c.Web != "" || len(c.PHP)+len(c.Extras) != 0 {
		t.Errorf("defaults on an unsupported server: %+v", c)
	}
}

func TestPlanOrdersAndMarksInstalledParts(t *testing.T) {
	f := facts(2048, 300)
	f.Installed["php8.4"], f.Installed["redis"] = true, true
	e := NewEnv(f)
	p, err := e.Plan(Choice{Web: "nginx", PHP: []string{"8.4", " 8.3", "8.4", "7.4"}, DB: "mariadb", Extras: []string{"waf", "docker", "redis"}})
	if err != nil {
		t.Fatal(err)
	}
	// PHP 8.3 comes with Nginx; the rest in a fixed order.
	want := []string{"nginx", "php7.4", "php8.4", "mariadb", "redis", "docker", "waf"}
	if got := stepIDs(p); !reflect.DeepEqual(got, want) {
		t.Fatalf("steps %v, want %v", got, want)
	}
	if p.Steps[0].Name != "Nginx and PHP 8.3" || !p.Steps[2].Installed || !p.Steps[4].Installed || p.Steps[1].Installed {
		t.Fatalf("steps %+v", p.Steps)
	}
	// Installed parts cost nothing more.
	wantRAM := (15 + 25) + 25 + 228 + 90 + 115
	wantDisk := (diskMB[Nginx] + diskMB["php"]) + diskMB["php"] + diskMB["mariadb"] + diskMB["docker"] + diskMB["waf"]
	if p.RAMMB != wantRAM || p.DiskMB != wantDisk || len(p.Warnings) != 0 {
		t.Fatalf("ram %d (want %d), disk %d (want %d), warnings %v", p.RAMMB, wantRAM, p.DiskMB, wantDisk, p.Warnings)
	}
	if p.Target() != "Nginx and PHP 8.3, PHP 7.4, PHP 8.4, MariaDB, Redis, Docker, Web firewall (ModSecurity)" {
		t.Errorf("target %q", p.Target())
	}

	// Without a web server, the distribution's PHP is a step of its own.
	p, err = e.Plan(Choice{Web: "none", PHP: []string{"8.3"}, Extras: []string{"docker"}})
	if err != nil || !reflect.DeepEqual(stepIDs(p), []string{"php8.3", "docker"}) {
		t.Fatalf("no web server: %v %v", stepIDs(p), err)
	}

	// Once Nginx is installed (a retry), its step is skipped and PHP 8.3 is
	// a step again, installed with it.
	f.WebServer, f.Installed["nginx"], f.Installed["php8.3"] = "nginx", true, true
	p, err = NewEnv(f).Plan(Choice{Web: "nginx", PHP: []string{"8.3"}, Extras: []string{"adminer"}})
	if err != nil || !reflect.DeepEqual(stepIDs(p), []string{"nginx", "php8.3", "adminer"}) || p.Steps[0].Name != "Nginx" {
		t.Fatalf("retry: %+v %v", p.Steps, err)
	}
	if !p.Steps[0].Installed || !p.Steps[1].Installed || p.RAMMB != 0 {
		t.Fatalf("retry: %+v", p)
	}
	// Adminer and the WAF need a web server: an installed one counts.
	if _, err := NewEnv(f).Plan(Choice{Extras: []string{"waf"}}); err != nil {
		t.Fatalf("WAF with Nginx installed: %v", err)
	}
	// A database server among the extras counts as the database.
	p, err = NewEnv(f).Plan(Choice{Extras: []string{"redis", "mysql"}})
	if err != nil || !reflect.DeepEqual(stepIDs(p), []string{"mysql", "redis"}) {
		t.Fatalf("MySQL as an extra: %v %v", stepIDs(p), err)
	}
}

func TestPlanRefusals(t *testing.T) {
	code := func(err error) string {
		var ue *usererr.Error
		if errors.As(err, &ue) {
			return ue.Code + ":" + ue.Detail
		}
		return fmt.Sprint(err)
	}
	small := facts(960, 250)
	small.Server.Arch = "riscv64"
	apache := facts(2048, 300)
	apache.WebServer, apache.Installed["apache"] = "apache", true
	apache.Blocked["nginx"] = Reason{Code: "web_server_conflict", Detail: "apache"}
	maria := facts(2048, 300)
	maria.Installed["mariadb"] = true
	noMySQL := facts(2048, 300)
	noMySQL.Catalog = []string{"mariadb", "redis"}
	for _, tc := range []struct {
		f    Facts
		c    Choice
		want string
	}{
		{Facts{Supported: false}, Choice{Web: "nginx"}, "unsupported:"},
		{facts(2048, 300), Choice{Web: "none"}, "setup_empty:"},
		{facts(2048, 300), Choice{Web: "caddy"}, "invalid_web_server:caddy"},
		{small, Choice{Web: "openlitespeed"}, "setup_not_offered:OpenLiteSpeed"},
		{small, Choice{Web: "nginx", Extras: []string{"nodejs"}}, "setup_not_offered:Node.js"},
		{small, Choice{Web: "nginx", Extras: []string{"clamav"}}, "setup_not_offered:ClamAV"},
		{facts(2048, 300), Choice{Extras: []string{"docker", "waf"}}, "setup_needs_web:Web firewall (ModSecurity)"},
		{facts(2048, 300), Choice{Web: "none", Extras: []string{"adminer"}}, "setup_needs_web:Adminer"},
		{facts(2048, 300), Choice{Web: "nginx", PHP: []string{"5.6"}}, "setup_not_offered:PHP 5.6"},
		{facts(2048, 300), Choice{Web: "nginx", Extras: []string{"postfix"}}, "unknown_software:postfix"},
		// One web server per server, and one database server.
		{apache, Choice{Web: "nginx"}, "web_server_conflict:apache"},
		{maria, Choice{DB: "mysql"}, "setup_db_conflict:MariaDB"},
		{facts(2048, 300), Choice{DB: "mysql", Extras: []string{"mariadb"}}, "setup_db_conflict:MySQL"},
		{facts(2048, 300), Choice{DB: "postgres"}, "unknown_software:postgres"},
		{noMySQL, Choice{DB: "mysql"}, "setup_not_offered:MySQL"},
		{noMySQL, Choice{Web: "nginx", Extras: []string{"memcached"}}, "setup_not_offered:Memcached"},
	} {
		_, err := NewEnv(tc.f).Plan(tc.c)
		if got := code(err); got != tc.want {
			t.Errorf("%+v: %s, want %s", tc.c, got, tc.want)
		}
	}
	if _, err := NewEnv(apache).Plan(Choice{Web: "apache", Extras: []string{"waf"}}); err != nil {
		t.Errorf("the managed web server again: %v", err)
	}
}

func TestPlanWarnsWhenItDoesNotFit(t *testing.T) {
	f := facts(960, 400)
	f.Server.DiskFreeMB = 1500
	p, err := NewEnv(f).Plan(Choice{Web: "nginx", PHP: []string{"8.4"}, DB: "mariadb", Extras: []string{"redis", "docker"}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Warnings, []string{"ram", "disk"}) {
		t.Fatalf("warnings %v (ram %d, disk %d)", p.Warnings, p.RAMMB, p.DiskMB)
	}
	// Unknown RAM and disk: no warnings.
	f.Server = Server{Arch: "amd64"}
	if p, _ := NewEnv(f).Plan(Choice{Web: "nginx", Extras: []string{"docker"}}); len(p.Warnings) != 0 {
		t.Fatalf("warnings without numbers: %v", p.Warnings)
	}
}

func TestOffered(t *testing.T) {
	if !Offered(true, "", "", 0) {
		t.Error("a new server is not offered the setup")
	}
	for _, tc := range [][4]any{{false, "", "", 0}, {true, "nginx", "", 0}, {true, "", "", 1}, {true, "", StateDone, 0}, {true, "", StateSkipped, 0}} {
		if Offered(tc[0].(bool), tc[1].(string), tc[2].(string), tc[3].(int)) {
			t.Errorf("offered with %v", tc)
		}
	}
	v := NewView(NewEnv(facts(960, 250)), "", 0)
	if !v.Offered || v.RAMBudgetMB == nil || *v.RAMBudgetMB != 960*85/100-250 || v.DiskBudgetMB == nil || *v.DiskBudgetMB != 20000-1024 {
		t.Errorf("view %+v", v)
	}
	if v.Defaults.Web != "nginx" {
		t.Errorf("view defaults %+v", v.Defaults)
	}
}

// ---- Run ----

type fakeInstaller struct {
	installed map[string]bool
	fail      map[string]error
	calls     []string
}

func (f *fakeInstaller) Installed(_ context.Context, id string) bool { return f.installed[id] }

func (f *fakeInstaller) Install(_ context.Context, id string, lg Logger) error {
	f.calls = append(f.calls, id)
	lg.Logf("installing %s", id)
	if err := f.fail[id]; err != nil {
		return err
	}
	f.installed[id] = true
	if id == "nginx" {
		f.installed["php8.3"] = true // InstallServer brings the distribution's PHP
	}
	return nil
}

type logBuf struct{ strings.Builder }

func (l *logBuf) Logf(format string, args ...any) { fmt.Fprintf(l, format+"\n", args...) }

func TestRunStopsAtTheFirstFailureAndRetries(t *testing.T) {
	e := NewEnv(facts(960, 250))
	plan, err := e.Plan(Choice{Web: "nginx", PHP: []string{"8.3", "8.4"}, DB: "mariadb", Extras: []string{"redis"}})
	if err != nil {
		t.Fatal(err)
	}
	in := &fakeInstaller{installed: map[string]bool{"redis": true}, fail: map[string]error{"php8.4": errors.New("apt-get: exit status 100")}}
	var lg logBuf
	err = Run(context.Background(), in, plan, &lg)
	var se *StepError
	if !errors.As(err, &se) || se.Step.ID != "php8.4" || !reflect.DeepEqual(se.Installed, []string{"Nginx and PHP 8.3"}) {
		t.Fatalf("error %v", err)
	}
	if !reflect.DeepEqual(in.calls, []string{"nginx", "php8.4"}) {
		t.Fatalf("calls after the failure: %v", in.calls)
	}
	for _, want := range []string{"PHP 8.4 could not be installed: apt-get: exit status 100", "Installed before it stopped: Nginx and PHP 8.3.", "run the setup again"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
	if !strings.Contains(lg.String(), "==> [1/4] Installing Nginx and PHP 8.3") || !strings.Contains(lg.String(), "The setup stopped here") {
		t.Errorf("log:\n%s", lg.String())
	}

	// The problem is fixed: the same plan runs again and skips what is installed.
	delete(in.fail, "php8.4")
	in.calls = nil
	lg.Reset()
	if err := Run(context.Background(), in, plan, &lg); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in.calls, []string{"php8.4", "mariadb"}) {
		t.Fatalf("calls on the retry: %v", in.calls)
	}
	for _, want := range []string{"[1/4] Nginx and PHP 8.3 is installed already: skipped", "[4/4] Redis is installed already: skipped", "The setup has finished"} {
		if !strings.Contains(lg.String(), want) {
			t.Errorf("log lacks %q:\n%s", want, lg.String())
		}
	}

	// A third run installs nothing.
	in.calls = nil
	if err := Run(context.Background(), in, plan, &lg); err != nil || len(in.calls) != 0 {
		t.Fatalf("third run: %v %v", in.calls, err)
	}

	// A failure at the first step.
	in = &fakeInstaller{installed: map[string]bool{}, fail: map[string]error{"nginx": errors.New("no network")}}
	err = Run(context.Background(), in, plan, &lg)
	if !errors.As(err, &se) || len(se.Installed) != 0 || !strings.Contains(err.Error(), "Nothing was installed") {
		t.Fatalf("first step: %v", err)
	}

	// A cancelled context stops before the next step.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	in = &fakeInstaller{installed: map[string]bool{}}
	if err := Run(ctx, in, plan, &lg); err == nil || len(in.calls) != 0 {
		t.Fatalf("cancelled: %v %v", in.calls, err)
	}
}

func TestName(t *testing.T) {
	for id, want := range map[string]string{"php8.4": "PHP 8.4", "nodejs": "Node.js", "openlitespeed": "OpenLiteSpeed", "x": "x"} {
		if got := Name(id); got != want {
			t.Errorf("Name(%q) = %q", id, got)
		}
	}
	if compareVersions("8.10", "8.9") <= 0 || compareVersions("7.4", "8.0") >= 0 {
		t.Error("compareVersions")
	}
}

func TestDiskFigures(t *testing.T) {
	// A full install measured +1.1 GB while the setup said 365 MB: the
	// figures now count apt's kept downloads and the data files (see the
	// comment of diskMB). The recommended choice on Ubuntu 24.04:
	f := facts(2048, 300)
	f.OSID, f.Codename = "ubuntu", "noble"
	e := NewEnv(f)
	p, err := e.Plan(Choice{Web: "nginx", DB: "mariadb", Extras: []string{"redis"}})
	if err != nil {
		t.Fatal(err)
	}
	if p.DiskMB < 450 || p.AptListsMB != 0 {
		t.Fatalf("Nginx, PHP, MariaDB and Redis: %d MB (lists %d)", p.DiskMB, p.AptListsMB)
	}
	// A server that never downloaded apt's lists: the first install does.
	f.AptListsMissing = true
	p2, _ := NewEnv(f).Plan(Choice{Web: "nginx", DB: "mariadb", Extras: []string{"redis"}})
	if p2.AptListsMB != aptListsMB["noble"] || p2.DiskMB != p.DiskMB+aptListsMB["noble"] {
		t.Fatalf("with the lists: %d MB (lists %d)", p2.DiskMB, p2.AptListsMB)
	}
	if NewEnv(f).AptListsMB != aptListsMB["noble"] {
		t.Fatal("env without the lists figure")
	}
	// A release without its own figure: the distribution's.
	f.Codename = "future"
	if NewEnv(f).AptListsMB != aptListsMB["ubuntu"] {
		t.Fatal("distribution's lists figure")
	}
	f.Codename = "noble"
	// Nothing to install: nothing to download either.
	f.Installed["nginx"], f.Installed["php8.3"], f.Installed["mariadb"], f.Installed["redis"] = true, true, true, true
	p3, _ := NewEnv(f).Plan(Choice{Web: "nginx", DB: "mariadb", Extras: []string{"redis"}})
	if p3.DiskMB != 0 || p3.AptListsMB != 0 {
		t.Fatalf("all installed: %d MB", p3.DiskMB)
	}
	// A release with its own figure.
	diskByRelease["testrel"] = map[string]int{"mariadb": 123}
	defer delete(diskByRelease, "testrel")
	f = facts(2048, 300)
	f.Codename = "testrel"
	if pt, _ := NewEnv(f).Part("mariadb"); pt.DiskMB != 123 {
		t.Fatalf("release figure: %+v", pt)
	}
	if pt, _ := NewEnv(f).Part("redis"); pt.DiskMB != diskMB["redis"] {
		t.Fatalf("default figure: %+v", pt)
	}
}

func TestMariaDBSeries(t *testing.T) {
	code := func(err error) string {
		var ue *usererr.Error
		if errors.As(err, &ue) {
			return ue.Code + ":" + ue.Detail
		}
		return fmt.Sprint(err)
	}
	f := facts(2048, 300)
	// Without the Software page's say, no series is offered.
	if _, ok := NewEnv(f).Part("mariadb-11.4"); ok {
		t.Fatal("a series offered without the catalog")
	}
	f.Catalog = []string{"php8.3", "mariadb", "mariadb-11.4", "mariadb-11.8", "mysql", "redis"}
	f.Versions = map[string]string{"mariadb": "10.11", "mariadb-11.4": "11.4", "mariadb-11.8": "11.8"}
	f.Until = map[string]string{"mariadb-11.4": "2029-05-29", "mariadb-11.8": "2028-06-04"}
	f.RAM["mariadb"], f.RAM["mariadb-11.4"] = 356, 356
	e := NewEnv(f)
	var dbs []string
	for _, p := range e.Parts {
		if p.Group == "db" {
			dbs = append(dbs, p.ID)
		}
	}
	if !reflect.DeepEqual(dbs, []string{"mariadb", "mariadb-11.4", "mariadb-11.8", "mysql"}) {
		t.Fatalf("database parts %v", dbs)
	}
	p, _ := e.Part("mariadb-11.4")
	if p.Version != "11.4" || p.Until != "2029-05-29" || p.RAMMB != 356 || p.DiskMB != diskMB["mariadb"] {
		t.Errorf("part %+v", p)
	}
	if d := e.Defaults(); d.DB != "mariadb" {
		t.Errorf("default %q", d.DB)
	}
	plan, err := e.Plan(Choice{Web: "nginx", DB: "mariadb-11.4"})
	if err != nil {
		t.Fatal(err)
	}
	if got := stepIDs(plan); !reflect.DeepEqual(got, []string{"nginx", "mariadb-11.4"}) || plan.Steps[1].Name != "MariaDB 11.4" {
		t.Errorf("plan %v %+v", got, plan.Steps)
	}
	if _, err := e.Plan(Choice{DB: "mariadb-11.4", Extras: []string{"mysql"}}); code(err) != "setup_db_conflict:MariaDB 11.4" {
		t.Errorf("two servers: %v", err)
	}
	if _, err := e.Plan(Choice{DB: "mariadb-12.9"}); code(err) != "setup_not_offered:MariaDB 12.9" {
		t.Errorf("unknown series: %v", err)
	}
	// Installed, it is the one the setup keeps.
	f.Installed["mariadb-11.8"] = true
	if d := NewEnv(f).Defaults(); d.DB != "mariadb-11.8" {
		t.Errorf("default with 11.8 installed: %q", d.DB)
	}
	if !IsDatabase("mariadb-11.4") || IsDatabase("redis") {
		t.Error("IsDatabase")
	}
}
