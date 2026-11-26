package hosting

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Purging LiteSpeed's lsphp packages (which openlitespeed depends on)
// damages the distribution's PHP: their postrm scripts fall back to
// Debian's phpdismod once their own copy is gone, and call it as
// "phpdismod 8.3 litespeed <module>" for every module, which Debian's
// phpdismod reads as module names for ALL versions and SAPIs; lsphp83-mysql
// and lsphp83-opcache also run "phpdismod ALL ALL mysqli|pdo_mysql|opcache".
// Every PHP version then starts without extensions, and because the
// modules are marked "disabled by admin", reinstalling PHP does not bring
// them back. Seen in the test sandbox (OpenLiteSpeed 1.9.4, lsphp83
// 8.3.37).
//
// So RemoveServer records the modules' state before the purge and puts it
// back afterwards: the conf.d symlinks of every PHP version and SAPI, and
// php-common's module registry.

const phpModuleRegistry = "/var/lib/php/modules"

type phpModuleState struct {
	links map[string]string // conf.d symlink → target
	marks map[string][]byte // registry file → content
}

// snapshotPHPModules records the enabled modules under phpDir (/etc/php)
// and the registry.
func snapshotPHPModules(phpDir, registry string) phpModuleState {
	st := phpModuleState{links: map[string]string{}, marks: map[string][]byte{}}
	dirs, _ := filepath.Glob(filepath.Join(phpDir, "*", "*", "conf.d"))
	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			p := filepath.Join(d, e.Name())
			if target, err := os.Readlink(p); err == nil {
				st.links[p] = target
			}
		}
	}
	filepath.WalkDir(registry, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if b, err := os.ReadFile(p); err == nil && len(b) < 64<<10 {
				st.marks[p] = b
			}
		}
		return nil
	})
	return st
}

// restore puts the recorded state back and returns how many entries it
// changed.
func (st phpModuleState) restore(phpDir, registry string) int {
	fixed := 0
	now := snapshotPHPModules(phpDir, registry)
	for p, target := range st.links {
		if cur, ok := now.links[p]; ok && cur == target {
			continue
		}
		if !strings.HasPrefix(p, filepath.Clean(phpDir)+string(filepath.Separator)) {
			continue
		}
		os.Remove(p)
		if os.Symlink(target, p) == nil {
			fixed++
		}
	}
	for p := range now.links {
		if _, ok := st.links[p]; !ok && os.Remove(p) == nil {
			fixed++
		}
	}
	for p, b := range st.marks {
		if cur, ok := now.marks[p]; ok && bytes.Equal(cur, b) {
			continue
		}
		if mkdir(filepath.Dir(p), 0o755) == nil && writeFile(p, b, 0o644) == nil {
			fixed++
		}
	}
	for p := range now.marks {
		if _, ok := st.marks[p]; !ok && os.Remove(p) == nil {
			fixed++
		}
	}
	return fixed
}

// phpGuard is a web server whose removal must keep the distribution's PHP
// module settings (OpenLiteSpeed, see above).
type phpGuard interface {
	guardsPHPModules() bool
}

func (o olsServer) guardsPHPModules() bool { return true }
