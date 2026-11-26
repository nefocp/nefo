// Package web embeds the built user interface (web/ builds into dist/) so the
// panel ships as one file.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// FS returns the built UI. It contains only .gitkeep until `npm run build`
// has been run in web/.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
