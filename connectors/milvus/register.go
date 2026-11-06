// Package milvus registers the Milvus vector store sink with the default registry.
// Enable with:
//
//	import _ "github.com/galaxy-io/filament/connectors/milvus"
package milvus

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/milvus/sink"
	"github.com/galaxy-io/filament/registry"
)

// init registers the Milvus sink with the default registry on package import.
func init() {
	registry.RegisterSink("milvus", filament.MaturityAlpha, func() filament.Sink { return sink.New() })
}
