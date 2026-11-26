package appstore

import (
	"embed"
	"io/fs"
	"slices"
	"strings"
)

// The logos are sanitized SVG files (plain shapes only: no scripts, styles,
// external references or embedded images); logos/SOURCES.md records where
// each came from and its licence. Besides the apps, the software page uses
// them (nginx, php, mariadb…).
//
//go:embed logos/*.svg
var logoFS embed.FS

// Logo returns the SVG for a logo key.
func Logo(key string) ([]byte, bool) {
	if !logoRe.MatchString(key) {
		return nil, false
	}
	b, err := fs.ReadFile(logoFS, "logos/"+key+".svg")
	return b, err == nil
}

// LogoExists reports whether a logo key has a file.
func LogoExists(key string) bool {
	_, ok := Logo(key)
	return ok
}

// Logos lists the logo keys.
func Logos() []string {
	entries, _ := fs.ReadDir(logoFS, "logos")
	var out []string
	for _, e := range entries {
		if k, ok := strings.CutSuffix(e.Name(), ".svg"); ok {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}
