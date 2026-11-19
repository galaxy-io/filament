//go:build embedui

package ui

import (
	"embed"
	"io/fs"
	"net/http"
)

// buildFS embeds the Vite build output. Build the UI first:
//
//	cd ui && pnpm install && pnpm build
//
//go:embed all:build
var buildFS embed.FS

// Handler serves the embedded web UI with an SPA fallback to index.html,
// rendered with cfg.
func Handler(cfg Config) http.Handler {
	build, err := fs.Sub(buildFS, "build")
	if err != nil {
		panic("ui: embedded build missing: " + err.Error())
	}
	handler, err := spaHandler(build, cfg)
	if err != nil {
		panic("ui: embedded build unusable: " + err.Error())
	}
	return handler
}
