// Package hubspot wires the HubSpot source onto the default registry.
// Enable it with a blank import:
//
//	import _ "github.com/galaxy-io/filament/connectors/hubspot"
package hubspot

import (
	"github.com/galaxy-io/filament"
	source "github.com/galaxy-io/filament/connectors/hubspot/source"
	"github.com/galaxy-io/filament/registry"
)

func init() {
	registry.RegisterSource("hubspot", filament.MaturityAlpha, func() filament.Source { return source.New() })
}
