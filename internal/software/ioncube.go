package software

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/nefocp/nefo/internal/fsutil"
	"github.com/nefocp/nefo/internal/usererr"
)

// The ionCube Loader runs PHP files encoded with the ionCube Encoder,
// which commercial scripts (WHMCS, some billing, licensing and shop
// software) ship as; websites moved from cPanel often need it. ionCube
// publishes the loaders free of charge (closed source) under their own
// licence, which allows use and redistribution with the licence included
// (https://www.ioncube.com/legal/loaders/eula, LICENSE.txt in the archive):
// the panel downloads them from ionCube on demand and keeps LICENSE.txt
// next to them.
//
// ionCube publishes no checksums or signatures, but its archives have
// stable versioned names (ioncube_loaders_lin_<arch>_<version>.tar.gz next
// to the unversioned "latest" one), so the panel pins one release and the
// SHA-256 of its archives, measured on 2026-11-27. A new loader release
// (one for a newer PHP version) comes with a panel update.
//
// Each archive has one loader per PHP version (ioncube_loader_lin_8.3.so,
// and _ts builds the distributions' PHP does not use). There has never
// been one for PHP 8.0, and the newest PHP versions get one months after
// their release. The panel puts the loaders into /usr/local/lib/ioncube and
// loads one into each installed PHP version it fits with 00-ioncube.ini in
// /etc/php/<v>/fpm/conf.d and cli/conf.d: a zend_extension that loads
// before OPcache (10-opcache.ini), as ionCube requires. Like the panel's
// 99-panel.ini, these are plain files the panel writes, not a package's.

// IoncubeVersion is the pinned loader release.
const IoncubeVersion = "15.5.1"

type ioncubeArchive struct{ name, sha256 string }

// Where the loaders come from; tests change them.
var (
	ioncubeBase     = "https://downloads.ioncube.com/loader_downloads/"
	ioncubeArchives = map[string]ioncubeArchive{
		"amd64": {"ioncube_loaders_lin_x86-64_15.5.1.tar.gz", "739962bd012e77ee82bf01f77a9641f050e4be057f0a5d1a95fe59ce11fd62ba"},
		"arm64": {"ioncube_loaders_lin_aarch64_15.5.1.tar.gz", "b091974aa80a76bd798d71fd5209797d93749b0be13eaff978a3dbe0106b9e1f"},
	}
	ioncubeDir = "/usr/local/lib/ioncube"
)

// ioncubePHP are the PHP versions the pinned release has loaders for, on
// both architectures, among those the panel offers (the archives also have
// PHP 4 and 5 loaders down to 4.1 on amd64 and 5.5 on arm64).
var ioncubePHP = []string{"5.6", "7.0", "7.1", "7.2", "7.3", "7.4", "8.1", "8.2", "8.3", "8.4", "8.5"}

const ioncubeMaxArchive = 64 << 20

// ioncubeMark is in the files the panel writes, so an uninstall removes
// only its own.
const ioncubeMark = "; Managed by the panel: the ionCube Loader"

// IoncubePHPState is what the loader means for one installed PHP version.
type IoncubePHPState struct {
	Version string `json:"version"`
	// State: "enabled" (loaded), "off" (a loader exists but is not loaded:
	// install again), "none" (ionCube never made one: PHP 8.0) or "not_yet"
	// (a newer PHP than the pinned loaders know).
	State string `json:"state"`
}

func ioncubeSO(v string) string {
	return filepath.Join(ioncubeDir, "ioncube_loader_lin_"+v+".so")
}

// ioncubeConfs are the ini files that load the loader into PHP v.
func ioncubeConfs(v string) []string {
	return []string{
		filepath.Join(phpEtc, v, "fpm", "conf.d", "00-ioncube.ini"),
		filepath.Join(phpEtc, v, "cli", "conf.d", "00-ioncube.ini"),
	}
}

// IoncubeInstalled reports whether the panel's loaders are on this server.
func IoncubeInstalled() bool { return exists(filepath.Join(ioncubeDir, "VERSION")) }

// ioncubeEnabled: PHP-FPM of version v loads the loader (the CLI may not
// be installed).
func ioncubeEnabled(v string) bool { return isIoncubeConf(ioncubeConfs(v)[0]) }

func isIoncubeConf(p string) bool {
	b, err := os.ReadFile(p)
	return err == nil && strings.HasPrefix(string(b), ioncubeMark)
}

// ioncubeStateFor says what the loader can do for PHP v.
func ioncubeStateFor(v string) string {
	switch {
	case slices.Contains(ioncubePHP, v):
		if ioncubeEnabled(v) && exists(ioncubeSO(v)) {
			return "enabled"
		}
		return "off"
	case phpNumber(v) > phpNumber(ioncubePHP[len(ioncubePHP)-1]):
		return "not_yet"
	}
	return "none"
}

// installedPHP lists the installed PHP versions, oldest first: the ones
// the panel offers and any other /etc/php/<v> with PHP-FPM.
func installedPHP() []string {
	var out []string
	entries, _ := os.ReadDir(phpEtc)
	for _, e := range entries {
		if phpDirRe.MatchString(e.Name()) && phpInstalled(e.Name()) {
			out = append(out, e.Name())
		}
	}
	sort.Slice(out, func(a, b int) bool { return phpNumber(out[a]) < phpNumber(out[b]) })
	return out
}

var phpDirRe = regexp.MustCompile(`^[5-9]\.[0-9]{1,2}$`)

// IoncubeItem is the ionCube Loader's card, in the PHP group.
func (m *Manager) IoncubeItem() Item {
	it := Item{ID: "ioncube", Kind: "ioncube", Group: "php", Installed: IoncubeInstalled(), Source: "official"}
	if _, ok := ioncubeArchives[legacyArch]; !ok && !it.Installed {
		it.NotOffered, it.NotOfferedDetail = "unsupported", legacyArch
	}
	if it.Installed {
		b, _ := os.ReadFile(filepath.Join(ioncubeDir, "VERSION"))
		it.Version = strings.TrimSpace(string(b))
		it.Block(nil)
		it.Updatable = it.Version != IoncubeVersion
	}
	for _, v := range installedPHP() {
		it.PHP = append(it.PHP, IoncubePHPState{Version: v, State: ioncubeStateFor(v)})
	}
	return it
}

// InstallIoncube downloads the pinned loaders, checks them against their
// SHA-256 and loads them into every installed PHP version they fit. It
// also updates an older panel-installed release.
func (m *Manager) InstallIoncube(ctx context.Context, lg Logger) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.installIoncube(ctx, lg)
}

func (m *Manager) installIoncube(ctx context.Context, lg Logger) error {
	arc, ok := ioncubeArchives[legacyArch]
	if !ok {
		return usererr.New("unsupported", legacyArch)
	}
	lg.Logf("Downloading the ionCube Loader %s from downloads.ioncube.com", IoncubeVersion)
	// Downloaded to a file while hashing, then unpacked one loader at a
	// time: a small server never holds the archive in memory.
	tmp, err := os.CreateTemp("", "ioncube-*.tar.gz")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	h := sha256.New()
	if err := downloadTo(ctx, ioncubeBase+arc.name, ioncubeMaxArchive, io.MultiWriter(tmp, h)); err != nil {
		return err
	}
	if hex.EncodeToString(h.Sum(nil)) != arc.sha256 {
		return fmt.Errorf("the download of %s does not match the checksum the panel has for it: nothing was installed", arc.name)
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return err
	}
	// Unpacked next to the loaders first, then moved over them one by one,
	// so a PHP-FPM that starts meanwhile always finds a complete file.
	staging := ioncubeDir + ".new"
	os.RemoveAll(staging)
	defer os.RemoveAll(staging)
	if err := fsutil.MkdirAll(staging, 0o755); err != nil {
		return err
	}
	names, err := ioncubeExtract(tmp, staging)
	if err != nil {
		return err
	}
	if err := fsutil.MkdirAll(ioncubeDir, 0o755); err != nil {
		return err
	}
	for _, name := range names {
		if err := os.Rename(filepath.Join(staging, name), filepath.Join(ioncubeDir, name)); err != nil {
			return err
		}
	}
	// Loaders of an older release that this one does not have.
	if old, _ := filepath.Glob(filepath.Join(ioncubeDir, "ioncube_loader_lin_*.so")); len(old) > 0 {
		for _, p := range old {
			if !slices.Contains(names, filepath.Base(p)) {
				os.Remove(p)
			}
		}
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(ioncubeDir, "VERSION"), []byte(IoncubeVersion+"\n"), 0o644); err != nil {
		return err
	}
	lg.Logf("The loaders are in %s, with ionCube's licence (LICENSE.txt)", ioncubeDir)
	var failed []string
	for _, v := range installedPHP() {
		switch ioncubeStateFor(v) {
		case "none":
			lg.Logf("PHP %s: ionCube has no loader for this version", v)
			continue
		case "not_yet":
			lg.Logf("PHP %s: ionCube has no loader for this version yet (loader %s)", v, IoncubeVersion)
			continue
		}
		if err := m.enableIoncube(ctx, v, lg); err != nil {
			lg.Logf("PHP %s: %v", v, err)
			failed = append(failed, v)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("the ionCube Loader could not be loaded into PHP %s (see the log above); the other versions have it", strings.Join(failed, ", "))
	}
	lg.Logf("The ionCube Loader %s is installed.", IoncubeVersion)
	return nil
}

// ioncubeExtract writes the loaders of ioncubePHP and LICENSE.txt from the
// archive into dir, and nothing else, and returns their names. An archive
// without one of them is refused.
func ioncubeExtract(archive io.Reader, dir string) ([]string, error) {
	zr, err := gzip.NewReader(archive)
	if err != nil {
		return nil, err
	}
	want := map[string]bool{"LICENSE.txt": true}
	for _, v := range ioncubePHP {
		want["ioncube_loader_lin_"+v+".so"] = true
	}
	var names []string
	tr := tar.NewReader(zr)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		d, name := filepath.Split(filepath.ToSlash(hdr.Name))
		if hdr.Typeflag != tar.TypeReg || d != "ioncube/" || !want[name] || slices.Contains(names, name) {
			continue
		}
		if hdr.Size > 16<<20 {
			return nil, fmt.Errorf("%s in the ionCube archive is larger than expected", name)
		}
		b, err := io.ReadAll(io.LimitReader(tr, hdr.Size))
		if err != nil {
			return nil, err
		}
		if err := fsutil.WriteFileAtomic(filepath.Join(dir, name), b, 0o644); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	for name := range want {
		if !slices.Contains(names, name) {
			return nil, fmt.Errorf("the ionCube archive has no %s", name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// enableIoncube loads the loader into PHP v (FPM and CLI), checks that the
// CLI starts with it, and reloads PHP-FPM. When PHP does not start with
// it, the configuration is taken out again.
func (m *Manager) enableIoncube(ctx context.Context, v string, lg Logger) error {
	if !slices.Contains(ioncubePHP, v) || !exists(ioncubeSO(v)) {
		return fmt.Errorf("there is no ionCube loader for PHP %s", v)
	}
	ini := fmt.Sprintf("%s %s (Software page).\n; Loads before OPcache (10-opcache.ini), as ionCube requires.\n; priority=00\nzend_extension=%s\n",
		ioncubeMark, IoncubeVersion, ioncubeSO(v))
	for _, p := range ioncubeConfs(v) {
		if !exists(filepath.Dir(p)) {
			continue // no PHP CLI for this version
		}
		if err := fsutil.WriteFileAtomic(p, []byte(ini), 0o644); err != nil {
			m.disableIoncube(v)
			return err
		}
	}
	out, err := m.run.Output(ctx, "php"+v, "-v")
	if err != nil || !strings.Contains(out, "ionCube") {
		m.disableIoncube(v)
		if err == nil {
			err = errors.New("PHP does not report the loader")
		}
		return fmt.Errorf("PHP %s did not start with the ionCube Loader, so it stays off: %v %s", v, err, firstLine(out))
	}
	lg.Logf("PHP %s: %s", v, ioncubeLine(out))
	return m.reloadIoncubeFPM(ctx, v, lg)
}

// reloadIoncubeFPM reloads PHP-FPM of version v if it runs (a reload starts
// its master process again, which loads the extensions anew); a stopped
// one loads the change when it starts.
func (m *Manager) reloadIoncubeFPM(ctx context.Context, v string, lg Logger) error {
	lg.Logf("Reloading PHP %s", v)
	_, err := m.run.Output(ctx, "systemctl", "try-reload-or-restart", "php"+v+"-fpm")
	return err
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}

// ioncubeLine is the line of "php -v" about the loader.
func ioncubeLine(out string) string {
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "ionCube") {
			return strings.TrimSpace(l)
		}
	}
	return "loaded"
}

// disableIoncube removes the panel's ionCube files of PHP v, and reports
// whether there were any.
func (m *Manager) disableIoncube(v string) bool {
	had := false
	for _, p := range ioncubeConfs(v) {
		if isIoncubeConf(p) {
			os.Remove(p)
			had = true
		}
	}
	return had
}

// RemoveIoncube takes the loader out of every PHP version, reloads them
// and deletes the loaders.
func (m *Manager) RemoveIoncube(ctx context.Context, lg Logger) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.removeIoncube(ctx, lg)
}

func (m *Manager) removeIoncube(ctx context.Context, lg Logger) error {
	if !IoncubeInstalled() {
		return usererr.New("software_not_installed", "ionCube Loader")
	}
	entries, _ := os.ReadDir(phpEtc)
	for _, e := range entries {
		v := e.Name()
		if !phpDirRe.MatchString(v) || !m.disableIoncube(v) {
			continue
		}
		lg.Logf("PHP %s: the ionCube Loader is off", v)
		if phpInstalled(v) {
			if err := m.reloadIoncubeFPM(ctx, v, lg); err != nil {
				lg.Logf("Warning: %v", err)
			}
		}
	}
	if err := os.RemoveAll(ioncubeDir); err != nil {
		return err
	}
	lg.Logf("The ionCube Loader is removed.")
	return nil
}

// ioncubeForNewPHP loads the loader into a PHP version just installed,
// when the panel's loaders are on the server and one fits it.
func (m *Manager) ioncubeForNewPHP(ctx context.Context, v string, lg Logger) {
	if !IoncubeInstalled() {
		return
	}
	switch ioncubeStateFor(v) {
	case "off":
		if err := m.enableIoncube(ctx, v, lg); err != nil {
			lg.Logf("Warning: %v", err)
		}
	case "not_yet":
		lg.Logf("The ionCube Loader has no loader for PHP %s yet", v)
	}
}
