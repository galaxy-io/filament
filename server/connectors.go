package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

const connectorRPCTimeout = 5 * time.Second
const resourceColumnsRPCTimeout = 30 * time.Second

// ListConnectors returns the registered source and sink specs, optionally
// filtered by kind.
func (a *Server) ListConnectors(_ context.Context, req *connect.Request[ingestionv1.ListConnectorsRequest]) (*connect.Response[ingestionv1.ListConnectorsResponse], error) {
	var connectors []*ingestionv1.ConnectorSpec
	if req.Msg.GetKind() == ingestionv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED || req.Msg.GetKind() == ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE {
		for _, spec := range a.sources.Specs() {
			connectors = append(connectors, sourceSpecToProto(spec))
		}
	}
	if req.Msg.GetKind() == ingestionv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED || req.Msg.GetKind() == ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK {
		for _, spec := range a.sinks.Specs() {
			connectors = append(connectors, sinkSpecToProto(spec))
		}
	}
	return connect.NewResponse(&ingestionv1.ListConnectorsResponse{Connectors: connectors}), nil
}

// ValidateConfig checks a connector config against its schema, optionally
// testing the live connection.
func (a *Server) ValidateConfig(ctx context.Context, req *connect.Request[ingestionv1.ValidateConfigRequest]) (*connect.Response[ingestionv1.ValidateConfigResponse], error) {
	ctx, cancel := context.WithTimeout(ctx, connectorRPCTimeout)
	defer cancel()

	cfg := filament.NewConfig(structMap(req.Msg.GetConfig()))
	switch req.Msg.GetKind() {
	case ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE:
		source, err := a.sources.Resolve(req.Msg.GetConnector())
		if err != nil {
			return nil, err
		}
		if err := source.Validate(cfg); err != nil {
			return connect.NewResponse(validationError(err.Error())), nil
		}
		if req.Msg.GetLive() {
			if live, ok := source.(filament.LiveValidatable); ok {
				if err := live.TestConnection(ctx, cfg); err != nil {
					return connect.NewResponse(validationError(err.Error())), nil
				}
			}
		}
	case ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK:
		sink, err := a.sinks.Resolve(req.Msg.GetConnector())
		if err != nil {
			return nil, err
		}
		if err := validateConfigSchema(sink.Spec().Config, cfg, filament.ScopeConnection); err != nil {
			return connect.NewResponse(validationError(err.Error())), nil
		}
	default:
		return connect.NewResponse(validationError("connector kind is required")), nil
	}
	return connect.NewResponse(&ingestionv1.ValidateConfigResponse{Valid: true}), nil
}

// DiscoverResources configures the source and lists its selectable resources.
func (a *Server) DiscoverResources(ctx context.Context, req *connect.Request[ingestionv1.DiscoverResourcesRequest]) (*connect.Response[ingestionv1.DiscoverResourcesResponse], error) {
	ctx, cancel := context.WithTimeout(ctx, connectorRPCTimeout)
	defer cancel()

	connector := req.Msg.GetConnector()
	config := structMap(req.Msg.GetConfig())
	if id := req.Msg.GetConnectionId(); id != "" {
		conn, err := a.store.LoadConnection(ctx, id)
		if err != nil {
			if errors.Is(err, filament.ErrNotFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if connector == "" {
			connector = conn.Connector
		}
		config = mergeConfig(conn.Config, config)
		if err := a.resolveConnectionSecrets(ctx, conn, config); err != nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
	}

	source, err := a.sources.Resolve(connector)
	if err != nil {
		return nil, err
	}
	cfg := filament.NewConfig(config)
	if err := source.Configure(ctx, cfg); err != nil {
		return nil, err
	}
	defer func() { _ = source.Teardown(ctx) }()

	discoverable, ok := source.(filament.Discoverable)
	if !ok {
		return nil, fmt.Errorf("connector %q does not support discovery", req.Msg.GetConnector())
	}
	result, err := discoverable.Discover(ctx, filament.DiscoverOpts{Refresh: req.Msg.GetRefresh()})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resourcesToProto(result.Resources)), nil
}

// GetResourceColumns configures one source and returns schemas for every
// requested resource, avoiding one connector pool per resource in the editor.
func (a *Server) GetResourceColumns(ctx context.Context, req *connect.Request[ingestionv1.GetResourceColumnsRequest]) (*connect.Response[ingestionv1.GetResourceColumnsResponse], error) {
	ctx, cancel := context.WithTimeout(ctx, resourceColumnsRPCTimeout)
	defer cancel()

	connector := req.Msg.GetConnector()
	config := structMap(req.Msg.GetConfig())
	if id := req.Msg.GetConnectionId(); id != "" {
		conn, err := a.store.LoadConnection(ctx, id)
		if err != nil {
			if errors.Is(err, filament.ErrNotFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if connector == "" {
			connector = conn.Connector
		}
		config = mergeConfig(conn.Config, config)
		if err := a.resolveConnectionSecrets(ctx, conn, config); err != nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
	}
	source, err := a.sources.Resolve(connector)
	if err != nil {
		return nil, err
	}
	if err := source.Configure(ctx, filament.NewConfig(config)); err != nil {
		return nil, err
	}
	defer func() { _ = source.Teardown(ctx) }()

	resources := append([]string(nil), req.Msg.GetResources()...)
	if len(resources) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("at least one resource is required"))
	}
	cursorProvider, cursorOK := source.(filament.CursorColumnProvider)
	schemaProvider, schemaOK := source.(filament.SchemaProvider)
	if !cursorOK && !schemaOK {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("connector %q does not provide resource columns", connector))
	}
	response := &ingestionv1.GetResourceColumnsResponse{}
	for _, resource := range resources {
		var columns []filament.CursorColumn
		if cursorOK {
			columns, err = cursorProvider.CursorColumns(ctx, resource)
		} else {
			var schema filament.RecordSchema
			schema, err = schemaProvider.Schema(ctx, resource)
			for _, field := range schema.Fields {
				columns = append(columns, filament.CursorColumn{SchemaField: field})
			}
		}
		if err != nil {
			return nil, fmt.Errorf("resource columns %q: %w", resource, err)
		}
		out := cursorColumnsToProto(columns)
		response.Resources = append(response.Resources, &ingestionv1.ResourceColumns{Resource: resource, Columns: out})
	}
	return connect.NewResponse(response), nil
}

func cursorColumnsToProto(columns []filament.CursorColumn) []*ingestionv1.ResourceColumn {
	out := make([]*ingestionv1.ResourceColumn, 0, len(columns))
	for _, column := range columns {
		out = append(out, &ingestionv1.ResourceColumn{
			Name: column.Name, LogicalType: string(column.Logical), NativeType: column.Native,
			Nullable: column.Nullable, PrimaryKey: column.PrimaryKey,
			CursorEligible: column.Eligible, CursorRecommended: column.Recommended,
			RecommendationRank: int32(column.Rank), Warning: column.Warning, //nolint:gosec // tiny rank
			Configurable: column.Configurable, SupportsLookback: column.SupportsLookback,
		})
	}
	return out
}
