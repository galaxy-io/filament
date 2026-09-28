// Package databricks wires the Databricks sinks onto the default registry.
// Enable them with a blank import:
//
//	import _ "github.com/galaxy-io/filament/connectors/databricks"
//
// The module groups every Databricks write mechanism; the Zerobus Ingest sink
// registers as "databrickszerobus". Future mechanisms (Delta via SQL warehouse,
// Lakebase) register their own names from this package.
package databricks

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/databricks/zerobus"
	"github.com/galaxy-io/filament/registry"
)

// init registers the Databricks sinks on the default registry when this package
// is imported.
func init() {
	registry.RegisterSink("databrickszerobus", filament.MaturityAlpha, func() filament.Sink { return zerobus.New() })
}
