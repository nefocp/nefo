package apt

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

// fake answers apt-get -s and dpkg-query from canned output and records
// what it ran for real (through Stream, as run.Apt does).
type fake struct {
	sim       map[string]string // "purge a b" -> simulated output
	orphans   []string          // what autoremove would remove, in turn
	installed string
	ran       []string
}

func (f *fake) Output(ctx context.Context, name string, args ...string) (string, error) {
	switch name {
	case "dpkg-query":
		return f.installed, errors.New("exit status 1") // a pattern matched nothing
	case "apt-get":
		rest := strings.Join(args[2:], " ") // after -s -q
		if rest == "autoremove" {
			if len(f.orphans) == 0 {
				return "", nil
			}
			o := f.orphans[0]
			f.orphans = f.orphans[1:]
			return o, nil
		}
		return f.sim[rest], nil
	}
	return "", nil
}

func (f *fake) Stream(ctx context.Context, w io.Writer, name string, args ...string) error {
	// /bin/sh -c '... apt-get "$@"' apt-get -q -y -o … -o … -o … <verb> <pkgs>
	f.ran = append(f.ran, strings.Join(args[11:], " "))
	return nil
}
func (f *fake) Input(context.Context, []byte, string, ...string) (string, error) { return "", nil }
func (f *fake) Pipe(context.Context, io.Reader, io.Writer, string, ...string) error {
	return nil
}

type log struct{ strings.Builder }

func (l *log) Logf(format string, args ...any) {}

func TestInstalled(t *testing.T) {
	f := &fake{installed: "redis-server\tinstalled\nphp8.3-redis\tinstalled\nphp8.1-redis\tconfig-files\nredis\tnot-installed\ndpkg-query: no packages found matching php7.4-redis\n"}
	got, err := Installed(context.Background(), f, false, "redis-server", "redis", "php*-redis")
	if err != nil || strings.Join(got, " ") != "php8.3-redis redis-server" {
		t.Fatalf("got %q, %v", got, err)
	}
	got, _ = Installed(context.Background(), f, true, "php*-redis")
	if strings.Join(got, " ") != "php8.1-redis php8.3-redis redis-server" {
		t.Fatalf("with config files: %q", got)
	}
	if _, err := Installed(context.Background(), f, false, "a; rm -rf /"); err == nil {
		t.Fatal("bad name accepted")
	}
}

func TestRemoveRefusesToTakeMore(t *testing.T) {
	f := &fake{sim: map[string]string{
		"purge python3-pip": "Purg python3-pip [24.0]\nPurg some-tool:amd64 [1.0]\n",
	}}
	err := Remove(context.Background(), f, &log{}, true, "python3-pip")
	if !errors.Is(err, ErrWouldRemoveMore) || !strings.Contains(err.Error(), "some-tool") {
		t.Fatalf("err = %v", err)
	}
	if len(f.ran) != 0 {
		t.Fatalf("apt ran anyway: %q", f.ran)
	}
}

func TestRemoveCleansOnlyNewLeftovers(t *testing.T) {
	f := &fake{
		sim: map[string]string{
			"remove docker.io":       "Remv docker.io [26.1]\n",
			"remove containerd":      "Remv containerd [1.7]\n",
			"remove runc":            "Remv containerd [1.7]\nRemv runc [1.1]\n", // containerd needs runc
			"remove containerd runc": "Remv containerd [1.7]\nRemv runc [1.1]\n",
		},
		orphans: []string{
			"Remv linux-image-6.8.0-31-generic [6.8.0-31]\nRemv old-lib [1]\n", // before: not ours to remove
			"Remv linux-image-6.8.0-31-generic [6.8.0-31]\nRemv old-lib [1]\nRemv containerd [1.7]\nRemv runc [1.1]\nRemv rsync [3.2]\n",
		},
	}
	if err := Remove(context.Background(), f, &log{}, false, "docker.io"); err != nil {
		t.Fatal(err)
	}
	// rsync became unused too, but it is protected.
	if strings.Join(f.ran, " | ") != "remove docker.io | remove containerd runc" {
		t.Fatalf("ran %q", f.ran)
	}
}

func TestRemoveKeepsWhatProtectedPackagesNeed(t *testing.T) {
	// perl came with MariaDB and is unused now, but it is protected; the
	// libraries perl needs must stay with it, the rest goes.
	f := &fake{
		sim: map[string]string{
			"purge mariadb-server":                       "Purg mariadb-server [1]\n",
			"purge galera-4":                             "Purg galera-4 [26]\n",
			"purge libperl5.38t64":                       "Purg libperl5.38t64 [5.38]\nPurg perl [5.38]\n",
			"purge perl-modules-5.38":                    "Purg perl-modules-5.38 [5.38]\nPurg libperl5.38t64 [5.38]\nPurg perl [5.38]\n",
			"purge mysql-common":                         "Purg mysql-common [1]\nPurg mariadb-common [1]\n",
			"purge mariadb-common":                       "Purg mariadb-common [1]\n",
			"purge galera-4 mariadb-common mysql-common": "Purg galera-4\nPurg mariadb-common\nPurg mysql-common\n",
			"purge galera-4 libperl5.38t64 mariadb-common mysql-common perl-modules-5.38": "Purg galera-4\nPurg libperl5.38t64\n" +
				"Purg mariadb-common\nPurg mysql-common\nPurg perl\nPurg perl-modules-5.38\n",
		},
		orphans: []string{
			"",
			"Remv galera-4 [26]\nRemv libperl5.38t64 [5.38]\nRemv mariadb-common [1]\nRemv mysql-common [1]\nRemv perl [5.38]\nRemv perl-modules-5.38 [5.38]\n",
		},
	}
	if err := Remove(context.Background(), f, &log{}, true, "mariadb-server"); err != nil {
		t.Fatal(err)
	}
	if strings.Join(f.ran, " | ") != "purge mariadb-server | purge galera-4 mariadb-common mysql-common" {
		t.Fatalf("ran %q", f.ran)
	}
}

func TestRemoveRefusesProtected(t *testing.T) {
	f := &fake{}
	for _, p := range []string{"python3", "python3.12", "systemd", "openssh-server"} {
		if err := Remove(context.Background(), f, &log{}, true, p); err == nil {
			t.Errorf("%s: removal accepted", p)
		}
	}
	if !protected.MatchString("python3.12-minimal") || !protected.MatchString("libatomic1") ||
		protected.MatchString("python3.12-venv") || protected.MatchString("python3-pip") {
		t.Error("protected list is wrong")
	}
	if len(f.ran) != 0 {
		t.Fatalf("ran %q", f.ran)
	}
}
