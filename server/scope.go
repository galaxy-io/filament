package server

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// schemaFor resolves a connector's config schema by kind + name from the
// catalog.
func (a *Server) schemaFor(ctx context.Context, kind ingestionv1.ConnectorKind, connector string) (filament.ConfigSchema, error) {
	switch kind {
	case ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE:
		spec, err := a.catalog.SourceSpec(ctx, connector)
		if err != nil {
			return filament.ConfigSchema{}, err
		}
		return spec.Config, nil
	case ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK:
		spec, err := a.catalog.SinkSpec(ctx, connector)
		if err != nil {
			return filament.ConfigSchema{}, err
		}
		return spec.Config, nil
	default:
		return filament.ConfigSchema{}, fmt.Errorf("connector kind is required")
	}
}

// Field-scope validation (connection vs pipeline overlay) lives in
// internal/compile alongside the overlay merge it guards.
