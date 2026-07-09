//go:build !embedui

package ui

import "net/http"

const stubPage = `<!doctype html>
<html>
  <head><title>Filament</title></head>
  <body style="font-family: monospace; padding: 24px; background: #131313; color: #e8e8e8;">
    <p>Filament UI is not embedded in this binary.</p>
    <p>Build it with:</p>
    <pre>cd ui && pnpm install && pnpm build
go build -tags embedui ./...</pre>
  </body>
</html>`

// Handler returns a placeholder page. Compile with -tags embedui (after
// building ui/dist) to serve the real web UI instead.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(stubPage))
	})
}
