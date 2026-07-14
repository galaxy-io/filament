package server

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"

	ingestion "github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

const providerRPCTimeout = 5 * time.Second

// ListProviders returns the registered source and sink specs, optionally
// filtered by kind.
func (a *Server) ListProviders(_ context.Context, req *connect.Request[ingestionv1.ListProvidersRequest]) (*connect.Response[ingestionv1.ListProvidersResponse], error) {
	var providers []*ingestionv1.ProviderSpec
	if req.Msg.GetKind() == ingestionv1.ProviderKind_PROVIDER_KIND_UNSPECIFIED || req.Msg.GetKind() == ingestionv1.ProviderKind_PROVIDER_KIND_SOURCE {
		for _, spec := range a.sources.Specs() {
			providers = append(providers, sourceSpecToProto(spec))
		}
	}
	if req.Msg.GetKind() == ingestionv1.ProviderKind_PROVIDER_KIND_UNSPECIFIED || req.Msg.GetKind() == ingestionv1.ProviderKind_PROVIDER_KIND_SINK {
		for _, spec := range a.sinks.Specs() {
			providers = append(providers, sinkSpecToProto(spec))
		}
	}
	return connect.NewResponse(&ingestionv1.ListProvidersResponse{Providers: providers}), nil
}

// ValidateConfig checks a provider config against its schema, optionally
// testing the live connection.
func (a *Server) ValidateConfig(ctx context.Context, req *connect.Request[ingestionv1.ValidateConfigRequest]) (*connect.Response[ingestionv1.ValidateConfigResponse], error) {
	ctx, cancel := context.WithTimeout(ctx, providerRPCTimeout)
	defer cancel()

	cfg := ingestion.NewConfig(structMap(req.Msg.GetConfig()))
	switch req.Msg.GetKind() {
	case ingestionv1.ProviderKind_PROVIDER_KIND_SOURCE:
		source, err := a.sources.Resolve(req.Msg.GetProvider())
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
	case ingestionv1.ProviderKind_PROVIDER_KIND_SINK:
		sink, err := a.sinks.Resolve(req.Msg.GetProvider())
		if err != nil {
			return nil, err
		}
		if err := validateConfigSchema(sink.Spec().Config, cfg); err != nil {
			return connect.NewResponse(validationError(err.Error())), nil
		}
	default:
		return connect.NewResponse(validationError("provider kind is required")), nil
	}
	return connect.NewResponse(&ingestionv1.ValidateConfigResponse{Valid: true}), nil
}

// DiscoverResources configures the source and lists its selectable resources.
func (a *Server) DiscoverResources(ctx context.Context, req *connect.Request[ingestionv1.DiscoverResourcesRequest]) (*connect.Response[ingestionv1.DiscoverResourcesResponse], error) {
	ctx, cancel := context.WithTimeout(ctx, providerRPCTimeout)
	defer cancel()

	source, err := a.sources.Resolve(req.Msg.GetProvider())
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
		return nil, fmt.Errorf("provider %q does not support discovery", req.Msg.GetProvider())
	}
	result, err := discoverable.Discover(ctx, ingestion.DiscoverOpts{Refresh: req.Msg.GetRefresh()})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resourcesToProto(result.Resources)), nil
}
