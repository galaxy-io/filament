package server

import (
	"fmt"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// schemaFor resolves a connector's config schema by kind + name from the source
// and sink registries.
func (a *Server) schemaFor(kind ingestionv1.ConnectorKind, connector string) (filament.ConfigSchema, error) {
	switch kind {
	case ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE:
		src, err := a.sources.Resolve(connector)
		if err != nil {
			return filament.ConfigSchema{}, err
		}
		return src.Spec().Config, nil
	case ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK:
		sink, err := a.sinks.Resolve(connector)
		if err != nil {
			return filament.ConfigSchema{}, err
		}
		return sink.Spec().Config, nil
	default:
		return filament.ConfigSchema{}, fmt.Errorf("connector kind is required")
	}
}

// Field-scope validation (connection vs pipeline overlay) lives in
// internal/compile alongside the overlay merge it guards.
