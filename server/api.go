package server

import (
	"context"
	"net/http"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/api/ingestion/v1/ingestionv1connect"
	"github.com/galaxy-io/filament/api/metrics/v1/metricsv1connect"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/internal/compile"
)

type runSubmitter interface {
	Submit(context.Context, filament.RunRequest) (filament.RunID, error)
}

// Server implements the ingestion Connect API over the registries, store,
// orchestrator, and bus.
type Server struct {
	sources   filament.SourceRegistry
	sinks     filament.SinkRegistry
	store     filament.DataStore
	orch      runSubmitter
	bus       eventbus.Bus
	secrets   filament.Secrets
	schedules filament.PipelineScheduleStore
	compiler  *compile.Compiler
	metrics   filament.MetricsStore
	auth      connect.Interceptor
}

// Option configures a Server.
type Option func(*Server)

// WithSecrets sets the secrets provider used to resolve secret refs.
func WithSecrets(secrets filament.Secrets) Option { return func(s *Server) { s.secrets = secrets } }

// WithMetricsStore sets the run metrics query backend. Unset leaves
// MetricsService unimplemented.
func WithMetricsStore(ms filament.MetricsStore) Option { return func(s *Server) { s.metrics = ms } }

// WithAuth installs an interceptor on every RPC handler. Unset leaves the
// API unauthenticated.
func WithAuth(ic connect.Interceptor) Option { return func(s *Server) { s.auth = ic } }

// New returns a Server wired to the given providers.
func New(sources filament.SourceRegistry, sinks filament.SinkRegistry, store filament.DataStore, orch runSubmitter, bus eventbus.Bus, opts ...Option) *Server {
	s := &Server{
		sources:  sources,
		sinks:    sinks,
		store:    store,
		orch:     orch,
		bus:      bus,
		compiler: &compile.Compiler{Store: store, Sources: sources, Sinks: sinks, DefaultTenant: defaultTenant("")},
	}
	if schedules, ok := store.(filament.PipelineScheduleStore); ok {
		s.schedules = schedules
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Mount registers the Connect handlers on mux.
func (a *Server) Mount(mux *http.ServeMux) {
	var opts []connect.HandlerOption
	if a.auth != nil {
		opts = append(opts, connect.WithInterceptors(a.auth))
	}
	path, handler := ingestionv1connect.NewIngestionServiceHandler(a, opts...)
	mux.Handle(path, withCORS(handler))
	path, handler = metricsv1connect.NewMetricsServiceHandler(a, opts...)
	mux.Handle(path, withCORS(handler))
}

var _ ingestionv1connect.IngestionServiceHandler = (*Server)(nil)
