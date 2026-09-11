// Package bigquery registers the Google BigQuery sink with the default
// registry. Enable it with a blank import:
//
//	import _ "github.com/galaxy-io/filament/connectors/bigquery"
package bigquery

import (
	"github.com/galaxy-io/filament"
	sink "github.com/galaxy-io/filament/connectors/bigquery/sink"
	"github.com/galaxy-io/filament/registry"
)

func init() {
	registry.RegisterSink("bigquery", filament.MaturityAlpha, func() filament.Sink { return sink.New() })
}
