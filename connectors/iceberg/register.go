package iceberg

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/registry"
)

// init registers the iceberg sink on the process-wide default registry. A
// consumer enables it with a blank import:
//
//	import _ "github.com/galaxy-io/filament/connectors/iceberg"
func init() {
	registry.RegisterSink("iceberg", filament.MaturityAlpha, func() filament.Sink { return New() })
}
