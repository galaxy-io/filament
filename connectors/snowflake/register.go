// Package snowflake registers the Snowflake sink with the default registry.
// Enable it with a blank import:
//
//	import _ "github.com/galaxy-io/filament/connectors/snowflake"
package snowflake

import (
	"github.com/galaxy-io/filament"
	sink "github.com/galaxy-io/filament/connectors/snowflake/sink"
	"github.com/galaxy-io/filament/registry"
)

func init() {
	registry.RegisterSink("snowflake", filament.MaturityAlpha, func() filament.Sink { return sink.New() })
}
