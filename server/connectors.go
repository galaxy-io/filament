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
	"github.com/galaxy-io/filament/internal/convert"
)

const (
	connectorRPCTimeout       = 5 * time.Second
	resourceColumnsRPCTimeout = 30 * time.Second
)

// ListConnectors returns the registered source and sink specs, optionally
// filtered by kind.
func (a *Server) ListConnectors(ctx context.Context, req *connect.Request[ingestionv1.ListConnectorsRequest]) (*connect.Response[ingestionv1.ListConnectorsResponse], error) {
	options, err := listOptionsOf(req.Msg.GetPagination(), req.Msg.GetSearch(), req.Msg.GetSorting(), map[ingestionv1.SortBy]string{
		ingestionv1.SortBy_SORT_BY_NAME: "name",
	}, "registry", false)
	if err != nil {
		return nil, err
	}
	catalog, err := a.worker.Describe(ctx)
	if err != nil {
		return nil, connectorError(err)
	}
	var connectors []*ingestionv1.ConnectorSpec
	if req.Msg.GetKind() == ingestionv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED || req.Msg.GetKind() == ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE {
		for _, spec := range catalog.Sources {
			connectors = append(connectors, convert.SourceSpecToProto(spec))
		}
	}
	if req.Msg.GetKind() == ingestionv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED || req.Msg.GetKind() == ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK {
		for _, spec := range catalog.Sinks {
			connectors = append(connectors, convert.SinkSpecToProto(spec))
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
func (a *Server) GetConnector(ctx context.Context, req *connect.Request[ingestionv1.GetConnectorRequest]) (*connect.Response[ingestionv1.GetConnectorResponse], error) {
	var spec *ingestionv1.ConnectorSpec
	switch req.Msg.GetKind() {
	case ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE:
		sourceSpec, err := a.worker.SourceSpec(ctx, req.Msg.GetConnector())
		if err != nil {
			return nil, connectorError(err)
		}
		spec = convert.SourceSpecToProto(sourceSpec)
	case ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK:
		sinkSpec, err := a.worker.SinkSpec(ctx, req.Msg.GetConnector())
		if err != nil {
			return nil, connectorError(err)
		}
		spec = convert.SinkSpecToProto(sinkSpec)
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("connector kind is required"))
	}
	return connect.NewResponse(&ingestionv1.GetConnectorResponse{Connector: spec}), nil
}

// ValidateConfig checks a connector config against its schema and, when the
// connector can probe connectivity, performs one live request with it so a
// bad credential is reported here rather than on the first run.
func (a *Server) ValidateConfig(ctx context.Context, req *connect.Request[ingestionv1.ValidateConfigRequest]) (*connect.Response[ingestionv1.ValidateConfigResponse], error) {
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, connectorRPCTimeout)
	defer cancel()

	config := convert.StructMap(req.Msg.GetConfig())
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
	ref, ok := connectorRefFromProto(req.Msg.GetKind(), req.Msg.GetConnector())
	if !ok {
		return connect.NewResponse(validationError("connector kind is required")), nil
	}
	schema, err := a.schemaFor(ctx, req.Msg.GetKind(), ref.Name)
	if err != nil {
		return nil, connectorError(err)
	}
	canonicalizeConnectionConfig(schema, config, nil)
	cfg := filament.NewConfig(config)
	if err := validateConfigSchema(schema, cfg); err != nil {
		return connect.NewResponse(schemaValidationError(err)), nil
	}
	if err := a.worker.Validate(ctx, ref, cfg); err != nil {
		if errors.Is(err, filament.ErrConfigure) {
			return connect.NewResponse(validationError(err.Error())), nil
		}
		return nil, connectorError(err)
	}
	// Connectors without a probe pass on structural validation alone.
	if err := a.worker.TestConnection(ctx, ref, cfg); err != nil && !errors.Is(err, filament.ErrUnsupported) {
		if errors.Is(err, filament.ErrConfigure) {
			return connect.NewResponse(validationError(err.Error())), nil
		}
		return nil, connectorError(err)
	}
	return connect.NewResponse(&ingestionv1.ValidateConfigResponse{Valid: true}), nil
}

func (a *Server) validateConnectionConnectorConfig(ctx context.Context, kind ingestionv1.ConnectorKind, connector string, cfg filament.Config) error {
	ref, ok := connectorRefFromProto(kind, connector)
	if !ok {
		return fmt.Errorf("connector kind is required")
	}
	return a.worker.Validate(ctx, ref, cfg)
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
	config := convert.StructMap(req.Msg.GetConfig())
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

	resources, err := a.worker.Discover(ctx, connector, filament.NewConfig(config), filament.DiscoverOpts{Refresh: req.Msg.GetRefresh()})
	if err != nil {
		if errors.Is(err, filament.ErrUnsupported) {
			return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("connector %q does not support discovery", connector))
		}
		return nil, connectorError(err)
	}
	return connect.NewResponse(&ingestionv1.DiscoverResourcesResponse{Resources: convert.ResourcesToProto(resources)}), nil
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
	config := convert.StructMap(req.Msg.GetConfig())
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
	resources := append([]string(nil), req.Msg.GetResources()...)
	if len(resources) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("at least one resource is required"))
	}
	inspections, err := a.worker.Inspect(ctx, connector, filament.NewConfig(config), resources)
	if err != nil {
		return nil, connectorError(err)
	}
	response := &ingestionv1.GetResourceColumnsResponse{}
	for _, inspection := range inspections {
		switch inspection.Status {
		case filament.InspectUnsupported:
			return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("connector %q does not provide resource columns", connector))
		case filament.InspectFailed:
			return nil, connect.NewError(connect.CodeInternal, errors.New(inspection.Message))
		}
		response.Resources = append(response.Resources, &ingestionv1.ResourceColumns{
			Resource:           inspection.Name,
			Columns:            convert.CursorColumnsToProto(inspection.Columns),
			ManagedIncremental: inspection.ManagedIncremental,
		})
	}
	return connect.NewResponse(response), nil
}

// connectorRefFromProto names a connector for the worker; ok is false when the
// kind is unspecified.
func connectorRefFromProto(kind ingestionv1.ConnectorKind, name string) (filament.ConnectorRef, bool) {
	switch kind {
	case ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE:
		return filament.ConnectorRef{Kind: filament.ConnectorKindSource, Name: name}, true
	case ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK:
		return filament.ConnectorRef{Kind: filament.ConnectorKindSink, Name: name}, true
	default:
		return filament.ConnectorRef{}, false
	}
}

// connectorError maps a worker error onto a Connect code: an unknown connector
// is NotFound, a missing optional contract is Unimplemented, a connector
// rejecting its config is FailedPrecondition, and anything else is Internal.
func connectorError(err error) error {
	switch {
	case errors.Is(err, filament.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, filament.ErrUnsupported):
		return connect.NewError(connect.CodeUnimplemented, err)
	case errors.Is(err, filament.ErrConfigure):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}
