package server

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/internal/compile"
)

const (
	connectorRPCTimeout       = 5 * time.Second
	resourceColumnsRPCTimeout = 30 * time.Second
)

// ListConnectors returns the registered source and sink specs, optionally
// filtered by kind.
func (a *Server) ListConnectors(_ context.Context, req *connect.Request[ingestionv1.ListConnectorsRequest]) (*connect.Response[ingestionv1.ListConnectorsResponse], error) {
	options, err := listOptionsOf(req.Msg.GetPagination(), req.Msg.GetSearch(), req.Msg.GetSorting(), map[ingestionv1.SortBy]string{
		ingestionv1.SortBy_SORT_BY_NAME: "name",
	}, "registry", false)
	if err != nil {
		return nil, err
	}
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
	search := strings.ToLower(options.Search)
	connectors = slices.DeleteFunc(connectors, func(spec *ingestionv1.ConnectorSpec) bool {
		if search == "" {
			return false
		}
		for _, field := range []string{spec.GetName(), spec.GetDisplayName(), spec.GetDescription()} {
			if strings.Contains(strings.ToLower(field), search) {
				return false
			}
		}
		return true
	})
	slices.SortStableFunc(connectors, func(a, b *ingestionv1.ConnectorSpec) int {
		var comparison int
		switch options.SortBy {
		case "name":
			comparison = strings.Compare(strings.ToLower(a.GetName()), strings.ToLower(b.GetName()))
		default:
			comparison = cmp.Compare(a.GetKind(), b.GetKind())
		}
		if comparison == 0 {
			comparison = strings.Compare(a.GetName(), b.GetName())
		}
		if options.SortDescending {
			return -comparison
		}
		return comparison
	})
	total := len(connectors)
	page := connectors
	if req.Msg.GetPagination() != nil {
		if options.Offset >= len(page) {
			page = nil
		} else {
			page = page[options.Offset:]
			if len(page) > options.Limit {
				page = page[:options.Limit]
			}
		}
	}
	return connect.NewResponse(&ingestionv1.ListConnectorsResponse{Connectors: page, Pagination: paginationOf(req.Msg.GetPagination(), options, total)}), nil
}

// GetConnector returns the spec for one registered connector.
func (a *Server) GetConnector(_ context.Context, req *connect.Request[ingestionv1.GetConnectorRequest]) (*connect.Response[ingestionv1.GetConnectorResponse], error) {
	var spec *ingestionv1.ConnectorSpec
	switch req.Msg.GetKind() {
	case ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE:
		if catalog, ok := a.sources.(interface {
			Spec(string) (filament.ConnectorSpec, error)
		}); ok {
			sourceSpec, err := catalog.Spec(req.Msg.GetConnector())
			if err != nil {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			spec = sourceSpecToProto(sourceSpec)
		} else {
			source, err := a.sources.Resolve(req.Msg.GetConnector())
			if err != nil {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			spec = sourceSpecToProto(source.Spec())
		}
	case ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK:
		if catalog, ok := a.sinks.(interface {
			Spec(string) (filament.SinkSpec, error)
		}); ok {
			sinkSpec, err := catalog.Spec(req.Msg.GetConnector())
			if err != nil {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			spec = sinkSpecToProto(sinkSpec)
		} else {
			sink, err := a.sinks.Resolve(req.Msg.GetConnector())
			if err != nil {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			spec = sinkSpecToProto(sink.Spec())
		}
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("connector kind is required"))
	}
	return connect.NewResponse(&ingestionv1.GetConnectorResponse{Connector: spec}), nil
}

// ValidateConfig checks a connector config against its schema.
func (a *Server) ValidateConfig(ctx context.Context, req *connect.Request[ingestionv1.ValidateConfigRequest]) (*connect.Response[ingestionv1.ValidateConfigResponse], error) {
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, connectorRPCTimeout)
	defer cancel()

	config := structMap(req.Msg.GetConfig())
	if id := req.Msg.GetConnectionId(); id != "" {
		conn, err := a.store.LoadConnection(ctx, tenant, id)
		if err != nil {
			if errors.Is(err, filament.ErrNotFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if err := a.resolveConnectionSecrets(ctx, conn, conn.Config); err != nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		config = overlayConfig(conn.Config, config)
	}
	switch req.Msg.GetKind() {
	case ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE:
		source, err := a.sources.Resolve(req.Msg.GetConnector())
		if err != nil {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		canonicalizeConnectionConfig(source.Spec().Config, config, nil)
		cfg := filament.NewConfig(config)
		if err := validateConfigSchema(source.Spec().Config, cfg); err != nil {
			return connect.NewResponse(schemaValidationError(err)), nil
		}
		if err := source.Validate(cfg); err != nil {
			return connect.NewResponse(validationError(err.Error())), nil
		}
	case ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK:
		sink, err := a.sinks.Resolve(req.Msg.GetConnector())
		if err != nil {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		canonicalizeConnectionConfig(sink.Spec().Config, config, nil)
		cfg := filament.NewConfig(config)
		if err := validateConfigSchema(sink.Spec().Config, cfg); err != nil {
			return connect.NewResponse(schemaValidationError(err)), nil
		}
		if validator, ok := sink.(filament.ConfigValidatable); ok {
			if err := validator.Validate(cfg); err != nil {
				return connect.NewResponse(validationError(err.Error())), nil
			}
		}
	default:
		return connect.NewResponse(validationError("connector kind is required")), nil
	}
	return connect.NewResponse(&ingestionv1.ValidateConfigResponse{Valid: true}), nil
}

func (a *Server) validateConnectionConnectorConfig(kind ingestionv1.ConnectorKind, connector string, cfg filament.Config) error {
	switch kind {
	case ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE:
		source, err := a.sources.Resolve(connector)
		if err != nil {
			return err
		}
		return source.Validate(cfg)
	case ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK:
		sink, err := a.sinks.Resolve(connector)
		if err != nil {
			return err
		}
		if validator, ok := sink.(filament.ConfigValidatable); ok {
			return validator.Validate(cfg)
		}
		return nil
	default:
		return fmt.Errorf("connector kind is required")
	}
}

// DiscoverResources configures the source and lists its selectable resources.
func (a *Server) DiscoverResources(ctx context.Context, req *connect.Request[ingestionv1.DiscoverResourcesRequest]) (*connect.Response[ingestionv1.DiscoverResourcesResponse], error) {
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, connectorRPCTimeout)
	defer cancel()

	connector := req.Msg.GetConnector()
	config := structMap(req.Msg.GetConfig())
	if id := req.Msg.GetConnectionId(); id != "" {
		conn, err := a.store.LoadConnection(ctx, tenant, id)
		if err != nil {
			if errors.Is(err, filament.ErrNotFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if connector == "" {
			connector = conn.Connector
		}
		if err := a.resolveConnectionSecrets(ctx, conn, conn.Config); err != nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		config = overlayConfig(conn.Config, config)
	}

	source, err := a.sources.Resolve(connector)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	cfg := filament.NewConfig(config)
	if err := source.Configure(ctx, cfg); err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	defer func() { _ = source.Teardown(ctx) }()

	discoverable, ok := source.(filament.Discoverable)
	if !ok {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("connector %q does not support discovery", connector))
	}
	result, err := discoverable.Discover(ctx, filament.DiscoverOpts{Refresh: req.Msg.GetRefresh()})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resourcesToProto(result.Resources)), nil
}

// GetResourceColumns configures one source and returns schemas for every
// requested resource, avoiding one connector pool per resource in the editor.
func (a *Server) GetResourceColumns(ctx context.Context, req *connect.Request[ingestionv1.GetResourceColumnsRequest]) (*connect.Response[ingestionv1.GetResourceColumnsResponse], error) {
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, resourceColumnsRPCTimeout)
	defer cancel()

	connector := req.Msg.GetConnector()
	config := structMap(req.Msg.GetConfig())
	if id := req.Msg.GetConnectionId(); id != "" {
		conn, err := a.store.LoadConnection(ctx, tenant, id)
		if err != nil {
			if errors.Is(err, filament.ErrNotFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if connector == "" {
			connector = conn.Connector
		}
		config = compile.MergeConfig(conn.Config, config)
		if err := a.resolveConnectionSecrets(ctx, conn, config); err != nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
	}
	source, err := a.sources.Resolve(connector)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err := source.Configure(ctx, filament.NewConfig(config)); err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
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
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("resource columns %q: %w", resource, err))
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
			IsNullable: column.Nullable, IsPrimaryKey: column.PrimaryKey,
			IsCursorEligible: column.Eligible, IsCursorRecommended: column.Recommended,
			RecommendationRank: int32(column.Rank), Warning: column.Warning, //nolint:gosec // tiny rank
			IsConfigurable: column.Configurable, SupportsLookback: column.SupportsLookback,
		})
	}
	return out
}
