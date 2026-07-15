//go:build embedui

package ui

import (
	"embed"
	"io/fs"
	"net/http"
)

// distFS embeds the Vite build output. Build the UI first:
//
//	cd ui && pnpm install && pnpm build
//
//go:embed all:dist
var distFS embed.FS

// Handler serves the embedded web UI with an SPA fallback to index.html.
func Handler() http.Handler {
	dist, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("ui: embedded dist missing: " + err.Error())
	}
	return spaHandler(dist)
}
