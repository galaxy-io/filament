// Package connectors registers every connector shipped in the filament
// services. Enable the full set with a single blank import.
package connectors

import (
	// Each blank import registers that connector's sources and sinks with
	// the default registry from init().
	_ "github.com/galaxy-io/filament/connectors/http"
	_ "github.com/galaxy-io/filament/connectors/iceberg"
	_ "github.com/galaxy-io/filament/connectors/object"
	_ "github.com/galaxy-io/filament/connectors/postgres"
	_ "github.com/galaxy-io/filament/connectors/sample"
	_ "github.com/galaxy-io/filament/connectors/stdout"
)
