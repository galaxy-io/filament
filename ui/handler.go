// Package ui serves the Filament web UI.
//
// The UI is a Vite + React app (see this directory) consuming @galaxy-io/dls.
// By default Handler returns a small placeholder page so `go build ./...`
// works without Node or a UI build. To embed the real UI into a binary:
//
//	cd ui && pnpm install && pnpm build   # produces ui/dist
//	go build -tags embedui ./examples/ingestiond
//
// Deployable binaries opt in by mounting the handler, e.g.:
//
//	app.Run(ctx, app.WithUI(ui.Handler()))
package ui

import (
	"io/fs"
	"net/http"
	"strings"
)

// spaHandler serves static files from dist, falling back to index.html for
// any path that doesn't match a file, so client-side routes deep-link cleanly.
func spaHandler(dist fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if f, err := dist.Open(path); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		// SPA fallback: serve index.html for unknown paths (client routes).
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/"
		r2.URL.RawPath = ""
		fileServer.ServeHTTP(w, r2)
	})
}
