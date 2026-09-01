package app

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// DiscoverSource resolves and validates a typed discovery request before
// delegating connector execution to the target.
func (s *Service) DiscoverSource(ctx context.Context, request DiscoverSourceRequest) (model.ResourceList, error) {
	catalog, err := s.target.Catalog(ctx)
	if err != nil {
		return model.ResourceList{}, err
	}
	connector, label := request.Connector, request.Connector
	var config map[string]any
	if request.Source != "" {
		connection, getErr := s.target.GetConnection(ctx, "source", request.Source)
		if getErr != nil {
			return model.ResourceList{}, getErr
		}
		connector, label = connection.Type, request.Source
		schema, schemaErr := catalog.ConnectionSchema("source", connector)
		if schemaErr != nil {
			return model.ResourceList{}, schemaErr
		}
		config, err = ResolvedConnectionConfig(connection, request.Config.Values, schema)
	} else {
		if connector == "" {
			return model.ResourceList{}, fmt.Errorf("a saved source or connector is required")
		}
		schema, schemaErr := catalog.ConnectionSchema("source", connector)
		if schemaErr != nil {
			return model.ResourceList{}, schemaErr
		}
		config = canonicalizeConfig(schema, applyConfigPatch(nil, request.Config))
		config, err = normalizeSavedSecretReferences(schema, config)
		if err == nil {
			err = validateFields("source connector", schema.Fields, config)
		}
	}
	if err != nil {
		return model.ResourceList{}, fmt.Errorf("source %q: %w", label, err)
	}
	return s.target.Discover(ctx, model.DiscoverRequest{
		Connector: connector, Source: label, Config: config, Refresh: request.Refresh,
	})
}
