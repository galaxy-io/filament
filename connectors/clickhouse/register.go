// Package clickhouse wires the ClickHouse sink onto the default registry.
// Enable it with a blank import:
//
//	import _ "github.com/galaxy-io/filament/connectors/clickhouse"
package clickhouse

import (
	"github.com/galaxy-io/filament"
	sink "github.com/galaxy-io/filament/connectors/clickhouse/sink"
	"github.com/galaxy-io/filament/registry"
)

func init() {
	registry.RegisterSink("clickhouse", filament.MaturityAlpha, func() filament.Sink { return sink.New() })
}
