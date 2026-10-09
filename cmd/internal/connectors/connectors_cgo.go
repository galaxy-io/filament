//go:build cgo

package connectors

import (
	// Connectors that link native libraries register only in cgo builds, so
	// the static binaries never see them.
	_ "github.com/galaxy-io/filament/connectors/motherduck"
)
