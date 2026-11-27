package main

import (
	"bytes"
	"errors"
	"flag"
	"reflect"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/api"
	"github.com/nefocp/nefo/internal/setup"
	"github.com/nefocp/nefo/internal/usererr"
)

func TestParseSetupFlags(t *testing.T) {
	for _, tc := range []struct {
		args  []string
		want  setup.Choice
		chose bool
	}{
		{[]string{"--web", "nginx", "--php", "8.3,8.4", "--mariadb", "--redis", "--node", "--docker", "--yes"},
			setup.Choice{Web: "nginx", PHP: []string{"8.3", "8.4"}, DB: "mariadb", Extras: []string{"redis", "nodejs", "docker"}}, true},
		// One dash or two, = or a space; the order of the extras is the setup's.
		{[]string{"-web=apache", "--waf", "--adminer", "-python", "--db", "mysql", "--clamav", "--php", " 8.4 , 8.4"},
			setup.Choice{Web: "apache", PHP: []string{"8.4"}, DB: "mysql", Extras: []string{"python", "adminer", "waf", "clamav"}}, true},
		{[]string{"--web", "none", "--docker", "-y"}, setup.Choice{PHP: []string{}, Extras: []string{"docker"}}, true},
		{[]string{"--web", "NGINX", "--php", "none", "--db", "none"}, setup.Choice{Web: "nginx", PHP: []string{}, Extras: []string{}}, true},
		{[]string{"--db", "mariadb-11.4"}, setup.Choice{PHP: []string{}, DB: "mariadb-11.4", Extras: []string{}}, true},
		{[]string{"--db", "mariadb", "--mariadb", "--postgresql", "--memcached"},
			setup.Choice{PHP: []string{}, DB: "mariadb", Extras: []string{"postgresql", "memcached"}}, true},
		// Nothing named: the recommended choice.
		{[]string{"--yes", "--wait", "30s"}, setup.Choice{PHP: []string{}, Extras: []string{}}, false},
		{nil, setup.Choice{PHP: []string{}, Extras: []string{}}, false},
	} {
		o, err := parseSetupFlags(tc.args, &bytes.Buffer{})
		if err != nil {
			t.Errorf("%q: %v", tc.args, err)
			continue
		}
		if !reflect.DeepEqual(o.choice, tc.want) || o.chose != tc.chose {
			t.Errorf("%q: %+v (chose %v), want %+v (chose %v)", tc.args, o.choice, o.chose, tc.want, tc.chose)
		}
	}

	o, err := parseSetupFlags([]string{"--yes", "--wait", "1m", "--dry-run", "--config", "/tmp/c.json"}, &bytes.Buffer{})
	if err != nil || !o.yes || !o.dryRun || o.wait.Seconds() != 60 || o.cfgPath != "/tmp/c.json" || o.skip || o.check {
		t.Fatalf("options %+v %v", o, err)
	}
	if o, err := parseSetupFlags([]string{"--check", "--web", "nginx"}, &bytes.Buffer{}); err != nil || !o.check {
		t.Fatalf("--check: %+v %v", o, err)
	}
	if o, err := parseSetupFlags([]string{"--skip"}, &bytes.Buffer{}); err != nil || !o.skip {
		t.Fatalf("--skip: %+v %v", o, err)
	}
}

func TestParseSetupFlagsRefusals(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string // in the error; "" for a flag package error
	}{
		{[]string{"--web", "caddy"}, `unknown web server "caddy"`},
		{[]string{"--php", "8.3,5.2"}, `unknown PHP version "5.2"`},
		{[]string{"--php", "8.3;rm"}, `unknown PHP version "8.3;rm"`},
		{[]string{"--db", "postgres"}, `unknown database server "postgres"`},
		{[]string{"--db", "mysql", "--mariadb"}, "disagree"},
		{[]string{"--web", "nginx", "extra"}, `unexpected argument "extra"`},
		{[]string{"--skip", "--web", "nginx"}, "--skip installs nothing"},
		{[]string{"--skip", "--dry-run"}, "--skip installs nothing"},
		{[]string{"--web", "none", "--php", "none"}, "nothing to install"},
		{[]string{"--nginx"}, ""},
		{[]string{"--wait", "soon"}, ""},
	} {
		var out bytes.Buffer
		_, err := parseSetupFlags(tc.args, &out)
		if err == nil {
			t.Errorf("%q accepted", tc.args)
			continue
		}
		var ue usageError
		if tc.want == "" {
			if errors.As(err, &ue) || !strings.Contains(out.String(), "Usage: nefo setup") {
				t.Errorf("%q: %v, output %q", tc.args, err, out.String())
			}
			continue
		}
		if !errors.As(err, &ue) || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%q: %v, want %q", tc.args, err, tc.want)
		}
	}
	if _, err := parseSetupFlags([]string{"-h"}, &bytes.Buffer{}); !errors.Is(err, flag.ErrHelp) {
		t.Errorf("-h: %v", err)
	}
}

func TestSetupErrorText(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{&api.LocalError{Code: "busy"}, "another task is running"},
		{&api.LocalError{Code: "setup_needs_web", Detail: "Adminer"}, "Adminer needs a web server: add --web nginx"},
		{usererr.New("web_server_conflict", "apache"), "apache is installed on this server"},
		{usererr.New("setup_db_conflict", "MariaDB"), "use --db mariadb"},
		{&api.LocalError{Code: "something_new", Detail: "x"}, "something_new: x"},
		{errors.New("plain"), "plain"},
	} {
		if got := setupError(tc.err).Error(); !strings.Contains(got, tc.want) {
			t.Errorf("%v: %q, want %q", tc.err, got, tc.want)
		}
	}
}

func TestPrintSetupPlan(t *testing.T) {
	env := setup.NewEnv(setup.Facts{
		Supported:   true,
		Server:      setup.Server{CPUs: 2, MemTotalMB: 960, MemUsedMB: 700, DiskTotalMB: 25000, DiskFreeMB: 20000, Arch: "amd64"},
		DistroPHP:   "8.3",
		PHPRepo:     "ppa:ondrej/php",
		PHPVersions: []string{"8.3", "8.5"},
		Installed:   map[string]bool{"redis": true},
	})
	v := setup.NewView(env, "", 0)
	p, err := v.Plan(setup.Choice{Web: "nginx", PHP: []string{"8.5"}, DB: "mariadb", Extras: []string{"redis"}})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	printSetupPlan(&out, v, p)
	for _, want := range []string{
		"This server: 2 CPUs, 960 MB of RAM (700 MB in use), 19.5 GB of free disk.",
		"1. Nginx and PHP 8.3  about 40 MB of RAM, 135 MB of disk",
		"2. PHP 8.5            about 25 MB of RAM, 130 MB of disk, from ppa:ondrej/php",
		"4. Redis              installed already: skipped",
		"In all: about 293 MB more RAM and 665 MB of disk.",
		"Warning: that is more memory than this server has to spare (about 116 MB)",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("plan lacks %q:\n%s", want, out.String())
		}
	}
	if sizeMB(1536) != "1.5 GB" || sizeMB(0) != "0 MB" {
		t.Error("sizeMB")
	}
}
