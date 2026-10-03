//go:build cgo

// The Databricks connector wraps the Zerobus Go SDK, which links a prebuilt
// Rust static library through cgo. The cli and binaries build targets set
// CGO_ENABLED=0, so registering it there would fail to link. Guard the import
// behind the cgo build tag: the connector is available in cgo-enabled builds
// and simply absent otherwise.
package connectors

import (
	// Blank import registers the Databricks connector's sinks with the default
	// registry from init(), matching the other connectors in connectors.go.
	_ "github.com/galaxy-io/filament/connectors/databricks"
)
