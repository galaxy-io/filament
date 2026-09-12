package bigquery

import (
	"github.com/galaxy-io/filament"
	bigqueryconnection "github.com/galaxy-io/filament/connectors/bigquery/internal/connection"
)

// Spec describes the sink's connection configuration. It intentionally
// advertises no write policies until the write implementation is available.
func (*Sink) Spec() filament.SinkSpec {
	return filament.SinkSpec{
		Name:         sinkName,
		DisplayName:  "Google BigQuery",
		Description:  "Serverless Google Cloud data warehouse for large-scale analytics.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-bigquery-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-bigquery-light.svg",
		Version:      "1",
		Config: filament.ConfigSchema{Fields: append(bigqueryconnection.Fields(), filament.ConfigField{
			Name: "dataset", Type: filament.FieldString, Scope: filament.ScopePipeline,
			Help: "Destination dataset. Empty defaults to the normalized source connection name.",
		})},
		SchemaField: "dataset",
	}
}
