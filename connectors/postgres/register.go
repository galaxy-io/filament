// Package postgres wires the postgres source and sink onto the default registry.
// Enable both with a single blank import:
//
//	import _ "github.com/galaxy-io/filament/connectors/postgres"
package postgres

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/registry"
	source "github.com/galaxy-io/filament/connectors/postgres/source"
	sink "github.com/galaxy-io/filament/connectors/postgres/sink"
)

func init() {
	registry.RegisterSource("postgres", func() ingestion.Source { return source.New() })
	registry.RegisterSink("postgres", func() ingestion.Sink { return sink.New() })
}
