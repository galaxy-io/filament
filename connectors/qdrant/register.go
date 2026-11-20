// Package qdrant registers the Qdrant vector store sink with the default registry.
// Enable with:
//
//	import _ "github.com/galaxy-io/filament/connectors/qdrant"
package qdrant

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/qdrant/sink"
	"github.com/galaxy-io/filament/registry"
)

// init registers the Qdrant sink with the default registry on package import.
func init() {
	registry.RegisterSink("qdrant", filament.MaturityAlpha, func() filament.Sink { return sink.New() })
}
