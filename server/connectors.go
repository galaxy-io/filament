package server

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"

	ingestion "github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

const connectorRPCTimeout = 5 * time.Second

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

	cfg := ingestion.NewConfig(structMap(req.Msg.GetConfig()))
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
			if live, ok := source.(ingestion.LiveValidatable); ok {
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
		if err := validateConfigSchema(sink.Spec().Config, cfg); err != nil {
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

	source, err := a.sources.Resolve(req.Msg.GetConnector())
	if err != nil {
		return nil, err
	}
	cfg := ingestion.NewConfig(structMap(req.Msg.GetConfig()))
	if err := source.Configure(ctx, cfg); err != nil {
		return nil, err
	}
	defer func() { _ = source.Teardown(ctx) }()

	discoverable, ok := source.(ingestion.Discoverable)
	if !ok {
		return nil, fmt.Errorf("connector %q does not support discovery", req.Msg.GetConnector())
	}
	result, err := discoverable.Discover(ctx, ingestion.DiscoverOpts{Refresh: req.Msg.GetRefresh()})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resourcesToProto(result.Resources)), nil
}
