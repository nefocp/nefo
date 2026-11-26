package software

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nefocp/nefo/internal/store"
)

// composerSandbox points Composer's paths into a temporary folder.
func composerSandbox(t *testing.T) string {
	t.Helper()
	dir := sandbox(t)
	oldPhar, oldApt, oldSite := composerPhar, composerApt, composerSite
	composerPhar = filepath.Join(dir, "usr/local/bin/composer")
	composerApt = filepath.Join(dir, "usr/bin/composer")
	t.Cleanup(func() { composerPhar, composerApt, composerSite = oldPhar, oldApt, oldSite })
	return dir
}

// fakePhar looks like the inside of Composer's phar.
func fakePhar(version string) []byte {
	return []byte("#!/usr/bin/env php\n<?php namespace Composer;class Composer{public const VERSION = '" + version +
		"';\npublic const BRANCH_ALIAS_VERSION = '';public const RELEASE_DATE = '2026-11-24 10:14:18';}")
}

func writeTestFile(t *testing.T, p string, b []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestComposerPathAndVersion(t *testing.T) {
	composerSandbox(t)
	m, f, st := newTestManager(t)
	ctx := context.Background()
	if ComposerPath() != "" || m.ComposerItem(ctx).Installed {
		t.Fatal("installed in an empty sandbox")
	}
	// The distribution's package.
	writeTestFile(t, composerApt, []byte("#!/usr/bin/env php\n<?php require '/usr/share/php/Composer/autoload.php';"))
	f.answers["dpkg-query -W -f=${Version} composer"] = "1:2.2.6-2ubuntu1"
	it := m.ComposerItem(ctx)
	if ComposerPath() != composerApt || !it.Installed || it.Source != "apt" || it.Version != "2.2.6" || !it.Updatable || !it.Removable {
		t.Fatalf("apt: %q %+v", ComposerPath(), it)
	}
	// The official phar in /usr/local/bin comes first, as in a shell's PATH.
	writeTestFile(t, composerPhar, fakePhar("2.11.3"))
	it = m.ComposerItem(ctx)
	if ComposerPath() != composerPhar || it.Source != "official" || it.Version != "2.11.3" {
		t.Fatalf("phar: %q %+v", ComposerPath(), it)
	}

	// What uses it: Laravel (only to install) and Git deploy scripts (every deploy).
	addSite(t, st, &store.Site{Domain: "lara.test", PHPVersion: "8.3", App: "laravel"})
	addSite(t, st, &store.Site{Domain: "git.test", PHPVersion: "8.3"})
	addSite(t, st, &store.Site{Domain: "json.test", PHPVersion: "8.3"})
	sites, _ := st.ListSites(ctx)
	for _, s := range sites {
		script := map[string]string{
			"git.test":  "git submodule update\ncomposer install --no-dev\nphp artisan migrate --force",
			"json.test": "cp composer.json.dist composer.json\nphp composer.phar install",
		}[s.Domain]
		if script != "" {
			if err := st.SaveGitDeploy(ctx, &store.GitDeploy{SiteID: s.ID, Repo: "https://example.com/r.git", Branch: "main", Target: "root", Script: script, HookSecret: "x"}); err != nil {
				t.Fatal(err)
			}
		}
	}
	it = m.ComposerItem(ctx)
	if it.Usage["apps"] != 1 || it.Usage["deploys"] != 1 || it.Removable || it.Blocked != "composer_in_use" || it.BlockedDetail != "1" {
		t.Fatalf("usage: %+v", it)
	}
	if err := m.Remove(ctx, "composer", RemoveOptions{}, &nopLog{}); code(err) != "composer_in_use:1" || !exists(composerPhar) {
		t.Fatalf("remove while a deploy runs it: %v", err)
	}
	for _, s := range []string{"composer install", "  /usr/local/bin/composer dump-autoload -o", "a && composer update"} {
		if !composerCallRe.MatchString(s) {
			t.Errorf("%q runs composer", s)
		}
	}
	for _, s := range []string{"cat composer.json", "php composer.phar install", "composer-setup.php"} {
		if composerCallRe.MatchString(s) {
			t.Errorf("%q does not run the server's composer", s)
		}
	}
}

func TestRemoveComposer(t *testing.T) {
	composerSandbox(t)
	m, f, _ := newTestManager(t)
	ctx := context.Background()
	if err := m.CheckRemove(ctx, "composer", RemoveOptions{}); code(err) != "software_not_installed:Composer" {
		t.Fatalf("not installed: %v", err)
	}
	writeTestFile(t, composerApt, []byte("x"))
	writeTestFile(t, composerPhar, fakePhar("2.11.3"))
	f.answers["dpkg-query -W -f ${Package}"] = "composer\tinstalled\n"
	f.answers["apt-get -s -q remove"] = "Remv composer [2.2.6-2]\n"
	if err := m.Remove(ctx, "composer", RemoveOptions{}, &nopLog{}); err != nil {
		t.Fatal(err)
	}
	if exists(composerPhar) || strings.Join(f.apt, "|") != "remove composer" {
		t.Fatalf("phar left: %v, apt %q", exists(composerPhar), f.apt)
	}
}

// fakeComposerSite answers like getcomposer.org on 127.0.0.1.
type fakeComposerSite struct {
	version, path string
	minPHP        int
	phar          []byte
	sum           string
	downloads     int
}

func newFakeComposerSite(t *testing.T, version string, phar []byte) *fakeComposerSite {
	t.Helper()
	c := &fakeComposerSite{version: version, path: "/download/" + version + "/composer.phar", minPHP: 70205, phar: phar}
	s := sha256.Sum256(phar)
	c.sum = hex.EncodeToString(s[:])
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/versions":
			fmt.Fprintf(w, `{"stable":[{"path":%q,"version":%q,"min-php":%d,"lts":false},{"path":"/download/2.2.33/composer.phar","version":"2.2.33","min-php":50300,"lts":true}]}`,
				c.path, c.version, c.minPHP)
		case "/download/" + c.version + "/composer.phar":
			c.downloads++
			w.Write(c.phar)
		case "/download/" + c.version + "/composer.phar.sha256sum":
			fmt.Fprintf(w, "%s  composer.phar\n", c.sum)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	composerSite = srv.URL
	return c
}

func TestUpdateComposer(t *testing.T) {
	composerSandbox(t)
	m, _, _ := newTestManager(t)
	ctx := context.Background()
	installPHPForTest(t, "8.3")
	old := fakePhar("2.10.0")
	writeTestFile(t, composerPhar, old)
	next := fakePhar("2.11.3")
	c := newFakeComposerSite(t, "2.11.3", next)

	// A checksum that does not match: refused, the old phar stays, no
	// temporary file is left.
	c.sum = strings.Repeat("0", 64)
	if err := m.UpdateComposer(ctx, &nopLog{}); err == nil || !strings.Contains(err.Error(), "does not match its published SHA-256") {
		t.Fatalf("mismatch: %v", err)
	}
	if b, _ := os.ReadFile(composerPhar); string(b) != string(old) {
		t.Fatal("a download that does not match replaced Composer")
	}
	if entries, _ := os.ReadDir(filepath.Dir(composerPhar)); len(entries) != 1 {
		t.Fatalf("left behind: %v", entries)
	}
	// A checksum file that is not one.
	c.sum = "<html>"
	if err := m.UpdateComposer(ctx, &nopLog{}); err == nil || !strings.Contains(err.Error(), "not a SHA-256") {
		t.Fatalf("bad checksum file: %v", err)
	}
	// A release listed somewhere unexpected is refused before any download.
	s := sha256.Sum256(next)
	c.sum, c.path, c.downloads = hex.EncodeToString(s[:]), "/elsewhere/composer.phar", 0
	if err := m.UpdateComposer(ctx, &nopLog{}); err == nil || c.downloads != 0 {
		t.Fatalf("unexpected path: %v, %d downloads", err, c.downloads)
	}
	c.path = "/download/2.11.3/composer.phar"
	// A release that needs a newer PHP than any installed.
	c.minPHP = 90000
	if err := m.UpdateComposer(ctx, &nopLog{}); code(err) != "composer_needs_php:9.0.0" {
		t.Fatalf("min-php: %v", err)
	}
	c.minPHP = 70205

	// The right one replaces the file in one step.
	if err := m.UpdateComposer(ctx, &nopLog{}); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(composerPhar); string(b) != string(next) || pharVersion(composerPhar) != "2.11.3" {
		t.Fatalf("installed %q", b)
	}
	if fi, _ := os.Stat(composerPhar); os.PathSeparator == '/' && fi.Mode().Perm() != 0o755 {
		t.Fatalf("mode %v", fi.Mode())
	}
	if entries, _ := os.ReadDir(filepath.Dir(composerPhar)); len(entries) != 1 {
		t.Fatalf("left behind: %v", entries)
	}
	// Up to date: nothing downloaded.
	c.downloads = 0
	if err := m.UpdateComposer(ctx, &nopLog{}); err != nil || c.downloads != 0 {
		t.Fatalf("again: %v, %d downloads", err, c.downloads)
	}
	if ComposerPath() != composerPhar {
		t.Fatal("the panel does not run the updated Composer")
	}
}
