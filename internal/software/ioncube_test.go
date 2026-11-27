package software

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fakeIoncubeArchive is laid out like ionCube's: ioncube/ with a loader per
// PHP version (and _ts builds), LICENSE.txt and other files.
func fakeIoncubeArchive(t *testing.T, versions ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	add := func(name, body string, typ byte) {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o664, Size: int64(len(body)), Typeflag: typ}); err != nil {
			t.Fatal(err)
		}
		tw.Write([]byte(body))
	}
	add("ioncube/", "", tar.TypeDir)
	add("ioncube/LICENSE.txt", "ionCube licence", tar.TypeReg)
	add("ioncube/loader-wizard.php", "<?php", tar.TypeReg)
	add("../evil.so", "x", tar.TypeReg)
	for _, v := range versions {
		add("ioncube/ioncube_loader_lin_"+v+".so", "ELF "+v, tar.TypeReg)
		add("ioncube/ioncube_loader_lin_"+v+"_ts.so", "ELF ts "+v, tar.TypeReg)
	}
	tw.Close()
	zw.Close()
	return buf.Bytes()
}

// ioncubeSandbox serves archive, pins its checksum and puts PHP 8.0, 8.3
// and 8.6 on the fake server.
func ioncubeSandbox(t *testing.T, archive []byte) (*Manager, *fakeRunner) {
	t.Helper()
	dir := sandbox(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/loaders/ioncube_loaders_lin_x86-64_15.5.1.tar.gz" {
			http.NotFound(w, r)
			return
		}
		w.Write(archive)
	}))
	t.Cleanup(srv.Close)
	sum := sha256.Sum256(archive)
	oldBase, oldArc, oldDir, oldArch := ioncubeBase, ioncubeArchives, ioncubeDir, legacyArch
	ioncubeBase = srv.URL + "/loaders/"
	ioncubeArchives = map[string]ioncubeArchive{"amd64": {"ioncube_loaders_lin_x86-64_15.5.1.tar.gz", hex.EncodeToString(sum[:])}}
	ioncubeDir = filepath.Join(dir, "usr/local/lib/ioncube")
	legacyArch = "amd64"
	t.Cleanup(func() { ioncubeBase, ioncubeArchives, ioncubeDir, legacyArch = oldBase, oldArc, oldDir, oldArch })
	for _, v := range []string{"8.0", "8.3", "8.6"} {
		fakePHP(t, v)
	}
	m, f, _ := newTestManager(t)
	f.answers["php8.3 -v"] = "PHP 8.3.6 (cli) (built: Apr 15 2026)\nZend Engine v4.3.6, Copyright (c) Zend Technologies\n    with the ionCube PHP Loader v15.5.1, Copyright (c) 2002-2026, by ionCube Ltd.\n"
	f.answers["php8.2 -v"] = "PHP 8.2.29 (cli)\n    with the ionCube PHP Loader v15.5.1\n"
	return m, f
}

func fakePHP(t *testing.T, v string) {
	t.Helper()
	touch(t, filepath.Join(phpEtc, v, "fpm", "php-fpm.conf"))
	for _, sapi := range []string{"fpm", "cli"} {
		os.MkdirAll(filepath.Join(phpEtc, v, sapi, "conf.d"), 0o755)
	}
}

func states(it Item) string {
	var s []string
	for _, p := range it.PHP {
		s = append(s, p.Version+"="+p.State)
	}
	return strings.Join(s, " ")
}

func TestIoncubeInstall(t *testing.T) {
	m, f := ioncubeSandbox(t, fakeIoncubeArchive(t, append([]string{"4.1"}, ioncubePHP...)...))
	ctx := context.Background()
	if it := m.IoncubeItem(); it.Installed || states(it) != "8.0=none 8.3=off 8.6=not_yet" {
		t.Fatalf("before: %+v %s", it, states(it))
	}
	if err := m.Install(ctx, "ioncube", &nopLog{}); err != nil {
		t.Fatal(err)
	}
	// Only the loaders of the PHP versions the panel offers, and the licence.
	entries, _ := os.ReadDir(ioncubeDir)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	want := "LICENSE.txt VERSION"
	for _, v := range ioncubePHP {
		want += " ioncube_loader_lin_" + v + ".so"
	}
	if strings.Join(names, " ") != want {
		t.Fatalf("files: %v", names)
	}
	for _, p := range ioncubeConfs("8.3") {
		ini, err := os.ReadFile(p)
		if err != nil || filepath.Base(p) != "00-ioncube.ini" || !strings.Contains(string(ini), "zend_extension="+ioncubeSO("8.3")+"\n") || !strings.HasPrefix(string(ini), ioncubeMark) {
			t.Fatalf("%s: %q %v", p, ini, err)
		}
	}
	for _, v := range []string{"8.0", "8.6"} {
		if exists(ioncubeConfs(v)[0]) || exists(ioncubeConfs(v)[1]) {
			t.Errorf("PHP %s got an ionCube ini", v)
		}
	}
	if !slices.Contains(f.cmds, "systemctl try-reload-or-restart php8.3-fpm") || slices.Contains(f.cmds, "systemctl try-reload-or-restart php8.6-fpm") {
		t.Fatalf("reloads: %v", f.cmds)
	}
	it := m.IoncubeItem()
	if !it.Installed || it.Version != IoncubeVersion || !it.Removable || it.Updatable || states(it) != "8.0=none 8.3=enabled 8.6=not_yet" {
		t.Fatalf("after: %+v %s", it, states(it))
	}

	// A PHP version installed later gets it too.
	fakePHP(t, "8.2")
	m.ioncubeForNewPHP(ctx, "8.2", &nopLog{})
	if !ioncubeEnabled("8.2") {
		t.Fatal("PHP 8.2 installed later has no loader")
	}

	// Uninstall: the panel's files go, an administrator's own ini stays.
	own := filepath.Join(phpEtc, "8.0", "fpm", "conf.d", "01-mine.ini")
	os.WriteFile(own, []byte("zend_extension=other.so\n"), 0o644)
	f.cmds = nil
	if err := m.Remove(ctx, "ioncube", RemoveOptions{}, &nopLog{}); err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"8.2", "8.3"} {
		for _, p := range ioncubeConfs(v) {
			if _, err := os.Lstat(p); err == nil {
				t.Errorf("%s is still there", p)
			}
		}
		if !slices.Contains(f.cmds, "systemctl try-reload-or-restart php"+v+"-fpm") {
			t.Errorf("PHP %s not reloaded: %v", v, f.cmds)
		}
	}
	if exists(ioncubeDir) || !exists(own) || IoncubeInstalled() {
		t.Fatal("uninstall left the loaders or took the administrator's file")
	}
	if code(m.CheckRemove(ctx, "ioncube", RemoveOptions{})) != "software_not_installed:ionCube Loader" {
		t.Fatal("removed twice")
	}
}

func TestIoncubeArchiveChecked(t *testing.T) {
	archive := fakeIoncubeArchive(t, ioncubePHP...)
	m, _ := ioncubeSandbox(t, archive)
	ctx := context.Background()
	// Another archive than the pinned one: nothing is installed.
	ioncubeArchives["amd64"] = ioncubeArchive{ioncubeArchives["amd64"].name, strings.Repeat("0", 64)}
	err := m.InstallIoncube(ctx, &nopLog{})
	if err == nil || !strings.Contains(err.Error(), "does not match the checksum") || exists(ioncubeDir) {
		t.Fatalf("checksum: %v", err)
	}
	// An archive without the loader of a version the panel expects.
	short := fakeIoncubeArchive(t, "8.3")
	sum := sha256.Sum256(short)
	m, _ = ioncubeSandbox(t, short)
	ioncubeArchives["amd64"] = ioncubeArchive{ioncubeArchives["amd64"].name, hex.EncodeToString(sum[:])}
	if err := m.InstallIoncube(ctx, &nopLog{}); err == nil || !strings.Contains(err.Error(), "has no ioncube_loader_lin_") {
		t.Fatalf("incomplete archive: %v", err)
	}
	// No loaders for this machine's architecture.
	legacyArch = "riscv64"
	if code(m.InstallIoncube(ctx, &nopLog{})) != "unsupported:riscv64" || m.IoncubeItem().NotOffered != "unsupported" {
		t.Fatal("riscv64")
	}
}

func TestIoncubePHPDoesNotStart(t *testing.T) {
	m, f := ioncubeSandbox(t, fakeIoncubeArchive(t, ioncubePHP...))
	f.answers["php8.3 -v"] = "PHP Fatal error: The file ioncube_loader_lin_8.3.so was encoded by ...\n"
	err := m.InstallIoncube(context.Background(), &nopLog{})
	if err == nil || !strings.Contains(err.Error(), "PHP 8.3") {
		t.Fatalf("got %v", err)
	}
	// Taken out again, so PHP-FPM keeps working.
	if ioncubeEnabled("8.3") || exists(ioncubeConfs("8.3")[1]) || slices.Contains(f.cmds, "systemctl try-reload-or-restart php8.3-fpm") {
		t.Fatal("a loader PHP cannot start with stayed on")
	}
	if it := m.IoncubeItem(); !it.Installed || states(it) != "8.0=none 8.3=off 8.6=not_yet" {
		t.Fatalf("%s", states(it))
	}
}

func TestIoncubeStates(t *testing.T) {
	for v, want := range map[string]string{"5.6": "off", "7.4": "off", "8.0": "none", "8.5": "off", "8.6": "not_yet", "9.0": "not_yet", "5.5": "none"} {
		if got := ioncubeStateFor(v); got != want {
			t.Errorf("PHP %s: %s, want %s", v, got, want)
		}
	}
}
