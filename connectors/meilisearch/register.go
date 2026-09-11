// Package meilisearch registers the Meilisearch sink with the default registry.
// Enable with:
//
//	import _ "github.com/galaxy-io/filament/connectors/meilisearch"
package meilisearch

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/meilisearch/sink"
	"github.com/galaxy-io/filament/registry"
)

func init() {
	registry.RegisterSink("meilisearch", filament.MaturityAlpha, func() filament.Sink { return sink.New() })
}
