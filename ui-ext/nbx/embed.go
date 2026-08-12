// Package nbx embeds the NextBase superuser UI extension files.
package nbx

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distDir embed.FS

// DistDirFS contains the embedded extension dist directory files
// (without the "dist" prefix).
var DistDirFS, _ = fs.Sub(distDir, "dist")
