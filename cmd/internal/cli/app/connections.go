package app

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// SaveConnection validates and persists a connection through the selected
// target.
func (s *Service) SaveConnection(ctx context.Context, request SaveConnectionRequest) (model.Connection, error) {
	document, err := s.target.Configuration(ctx)
	if err != nil {
		return model.Connection{}, err
	}
	catalog, err := s.target.Catalog(ctx)
	if err != nil {
		return model.Connection{}, err
	}
	connections, err := connectionMap(request.Kind, document)
	if err != nil {
		return model.Connection{}, err
	}
	existing, exists := connections[request.Name]
	if request.Create && exists {
		return model.Connection{}, fmt.Errorf("%s %q already exists", request.Kind, request.Name)
	}
	if !request.Create && !exists {
		return model.Connection{}, fmt.Errorf("%s %q does not exist", request.Kind, request.Name)
	}
	var base *model.Connection
	if exists {
		base = &existing
	}
	connection, err := BuildConnection(request, base, catalog)
	if err != nil {
		return model.Connection{}, err
	}
	testDocument := document
	if request.Kind == "source" {
		testDocument.Sources = cloneMap(document.Sources)
		testDocument.Sources[request.Name] = connection
	} else {
		testDocument.Sinks = cloneMap(document.Sinks)
		testDocument.Sinks[request.Name] = connection
	}
	if err := ValidateDocument(testDocument, catalog); err != nil {
		return model.Connection{}, err
	}
	if err := s.target.PutConnection(ctx, request.Kind, request.Name, connection); err != nil {
		return model.Connection{}, err
	}
	return connection, nil
}

// BuildConnection applies a typed request without persisting it. It is useful
// to render previews and compose larger use cases.
func BuildConnection(request SaveConnectionRequest, existing *model.Connection, catalog model.Catalog) (model.Connection, error) {
	connection := model.Connection{Config: map[string]any{}}
	if existing != nil {
		connection = *existing
		connection.Config = cloneConfigMap(existing.Config)
	}
	connector := request.Connector
	if connector == "" {
		connector = connection.Type
	}
	if connector == "" {
		return connection, fmt.Errorf("%s %q: connector is required", request.Kind, request.Name)
	}
	if existing != nil && connector != existing.Type {
		connection.Config = map[string]any{}
	}
	connection.Type = connector
	schema, err := catalog.ConnectionSchema(request.Kind, connector)
	if err != nil {
		return connection, err
	}
	connection.Config = applyConfigPatch(connection.Config, request.Config)
	connection.Config = canonicalizeConfig(schema, connection.Config)
	connection.Config, err = normalizeSavedSecretReferences(schema, connection.Config)
	if err != nil {
		return connection, fmt.Errorf("%s %q: %w", request.Kind, request.Name, err)
	}
	return connection, nil
}

// DeleteConnection rejects dangling pipeline references before persistence.
func (s *Service) DeleteSavedConnection(ctx context.Context, kind, name string) error {
	document, err := s.target.Configuration(ctx)
	if err != nil {
		return err
	}
	connections, err := connectionMap(kind, document)
	if err != nil {
		return err
	}
	if _, exists := connections[name]; !exists {
		return fmt.Errorf("%s %q does not exist", kind, name)
	}
	for pipelineName, pipeline := range document.Pipelines {
		if (kind == "source" && pipeline.Source.Ref == name) || (kind == "sink" && pipeline.Sink.Ref == name) {
			return fmt.Errorf("%s %q is referenced by pipeline %q; delete or edit that pipeline first", kind, name, pipelineName)
		}
	}
	return s.target.DeleteConnection(ctx, kind, name)
}
