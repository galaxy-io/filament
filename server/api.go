package server

import (
	"context"
	"net/http"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/api/auth/v1/authv1connect"
	"github.com/galaxy-io/filament/api/ingestion/v1/ingestionv1connect"
	"github.com/galaxy-io/filament/api/metrics/v1/metricsv1connect"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/identity"
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
	identity  identity.Provider
	log       filament.Logger
}

// Option configures a Server.
type Option func(*Server)

// WithSecrets sets the secrets provider used to resolve secret refs.
func WithSecrets(secrets filament.Secrets) Option { return func(s *Server) { s.secrets = secrets } }

// WithMetricsStore sets the run metrics query backend. Unset leaves
// MetricsService unimplemented.
func WithMetricsStore(ms filament.MetricsStore) Option { return func(s *Server) { s.metrics = ms } }

// WithIdentity sets the authentication provider. Unset leaves the API
// unauthenticated and AuthService unimplemented apart from its config
// document, the same way an unset metrics store leaves MetricsService
// unimplemented.
func WithIdentity(p identity.Provider) Option { return func(s *Server) { s.identity = p } }

// WithLogger sets the structured logger used for API diagnostics.
func WithLogger(log filament.Logger) Option {
	return func(s *Server) {
		if log != nil {
			s.log = log.With(filament.Field{Key: "component", Value: "api"})
		}
	}
}

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

// Mount registers the Connect handlers on mux. AuthService mounts whether
// or not a provider is configured so the UI always has a config document to
// read; with a provider, one interceptor authenticates every procedure that
// is not part of the public session flow.
func (a *Server) Mount(mux *http.ServeMux) {
	var opts []connect.HandlerOption
	if a.log != nil {
		opts = append(opts, connect.WithInterceptors(newLoggingInterceptor(a.log)))
	}
	if a.identity != nil {
		opts = append(opts, connect.WithInterceptors(&authInterceptor{provider: a.identity, store: a.store}))
	}
	path, handler := ingestionv1connect.NewIngestionServiceHandler(a, opts...)
	mux.Handle(path, withCORS(handler))
	path, handler = metricsv1connect.NewMetricsServiceHandler(a, opts...)
	mux.Handle(path, withCORS(handler))
	// The provider serves AuthService directly; without one, a stub keeps
	// the config document answering so the UI can tell auth is off.
	var authHandler authv1connect.AuthServiceHandler = disabledAuth{}
	if a.identity != nil {
		authHandler = a.identity
	}
	path, handler = authv1connect.NewAuthServiceHandler(authHandler, opts...)
	mux.Handle(path, withCORS(handler))
}

var _ ingestionv1connect.IngestionServiceHandler = (*Server)(nil)
