// Package mysql wires the mysql source and sink onto the default registry.
// Enable both with a single blank import:
//
//	import _ "github.com/galaxy-io/filament/connectors/mysql"
package mysql

import (
	ingestion "github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/registry"

	sink "github.com/galaxy-io/filament/connectors/mysql/sink"
	source "github.com/galaxy-io/filament/connectors/mysql/source"
)

func init() {
	registry.RegisterSource("mysql", func() ingestion.Source { return source.New() })
	registry.RegisterSink("mysql", func() ingestion.Sink { return sink.New() })
}
