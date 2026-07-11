// Package webui embeds the built TabVM web UI so the agent can serve it as a
// single self-contained binary in production. The dist directory is populated
// by the release build (scripts/build-release.ps1 copies apps/web-ui/dist here);
// a committed placeholder keeps `go build` working before a UI build has run.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var embedded embed.FS

// FS returns the built web UI file system rooted at the dist directory.
func FS() (fs.FS, error) {
	return fs.Sub(embedded, "dist")
}
