// Package ui serves the Filament web UI.
//
// The UI is a Vite + React app (see this directory) consuming @galaxy-io/dls.
// By default Handler returns a small placeholder page so `go build ./...`
// works without Node or a UI build. To embed the real UI into a binary:
//
//	cd ui && pnpm install && pnpm build   # produces ui/build
//	go build -C cmd/server -tags embedui .
//
// `just binaries` does both. Deployable binaries opt in by mounting the
// handler, e.g.:
//
//	mux.Handle("/", ui.Handler())
package ui
