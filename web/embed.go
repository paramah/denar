// Package web carries the built panel so the server ships as one binary.
package web

import (
	"embed"
	"io/fs"
)

// dist holds the output of the web build. Only the .gitkeep placeholder is
// tracked, so a checkout compiles before the panel has been built.
//
//go:embed all:dist
var dist embed.FS

// Dist returns the built panel rooted at dist: index.html sits at the top.
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		// Unreachable: "dist" is a valid path and the directive embeds it.
		panic(err)
	}
	return sub
}
