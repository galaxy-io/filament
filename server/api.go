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
	Submit(context.Context, filament.RunSubmission) (filament.RunID, error)
}

// Server implements the ingestion Connect API over the worker, store,
// orchestrator, and bus.
type Server struct {
	worker    filament.Worker
	store     filament.DataStore
	orch      runSubmitter
	bus       eventbus.Bus
	secrets   filament.Secrets
	schedules filament.PipelineScheduleStore
	compiler  *compile.Compiler
	metrics   filament.MetricsStore
	identity  identity.Authenticator
	// defaultTenant scopes every request when identity is disabled.
	defaultTenant filament.TenantID
	log           filament.Logger
}

// Option configures a Server.
type Option func(*Server)

// WithSecrets sets the secrets provider used to resolve secret refs.
func WithSecrets(secrets filament.Secrets) Option { return func(s *Server) { s.secrets = secrets } }

// WithMetricsStore sets the run metrics query backend. Unset leaves
// MetricsService unimplemented.
func WithMetricsStore(ms filament.MetricsStore) Option { return func(s *Server) { s.metrics = ms } }

// WithIdentity sets the authenticator. Unset leaves the API unauthenticated
// and AuthService unimplemented apart from its config document, the same way
// an unset metrics store leaves MetricsService unimplemented. A full
// identity.Provider also serves AuthService; any other Authenticator leaves
// sign-in upstream and AuthService answers only its config document.
func WithIdentity(a identity.Authenticator) Option { return func(s *Server) { s.identity = a } }

// WithDefaultTenant sets the only tenant available when authentication is
// disabled. Authenticated requests always use the identity provider's tenant.
func WithDefaultTenant(tenant filament.TenantID) Option {
	return func(s *Server) {
		if tenant != "" {
			s.defaultTenant = tenant
		}
	}
}

// WithLogger sets the structured logger used for API diagnostics.
func WithLogger(log filament.Logger) Option {
	return func(s *Server) {
		if log != nil {
			s.log = log.With(filament.Field{Key: "component", Value: "api"})
		}
	}
}

// New returns a Server wired to the given providers. worker answers every
// connector question; the server never holds a driver.
func New(worker filament.Worker, store filament.DataStore, orch runSubmitter, bus eventbus.Bus, opts ...Option) *Server {
	s := &Server{
		worker:        worker,
		store:         store,
		orch:          orch,
		bus:           bus,
		defaultTenant: filament.DefaultTenantID,
		compiler:      &compile.Compiler{Store: store, Worker: worker},
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
	opts = append(opts, connect.WithInterceptors(&authInterceptor{
		provider: a.identity, store: a.store, defaultTenant: a.defaultTenant,
	}))
	path, handler := ingestionv1connect.NewIngestionServiceHandler(a, opts...)
	mux.Handle(path, withCORS(handler))
	path, handler = metricsv1connect.NewMetricsServiceHandler(a, opts...)
	mux.Handle(path, withCORS(handler))
	// A provider serves AuthService directly. Any other authenticator leaves
	// sign-in upstream, so a stub reports where; without one, a stub keeps
	// the config document answering so the UI can tell auth is off.
	var authHandler authv1connect.AuthServiceHandler = disabledAuth{}
	switch p := a.identity.(type) {
	case identity.Provider:
		authHandler = p
	case identity.ExternalLogin:
		authHandler = externalAuth{loginURL: p.LoginURL()}
	case identity.Authenticator:
		authHandler = externalAuth{}
	}
	path, handler = authv1connect.NewAuthServiceHandler(authHandler, opts...)
	mux.Handle(path, withCORS(handler))
}

var _ ingestionv1connect.IngestionServiceHandler = (*Server)(nil)
