// Package motherduck wires the MotherDuck sink onto the default registry.
// Enable it with a blank import:
//
//	import _ "github.com/galaxy-io/filament/connectors/motherduck"
package motherduck

import (
	"github.com/galaxy-io/filament"
	sink "github.com/galaxy-io/filament/connectors/motherduck/sink"
	"github.com/galaxy-io/filament/registry"
)

func init() {
	registry.RegisterSink("motherduck", filament.MaturityAlpha, func() filament.Sink { return sink.New() })
}
