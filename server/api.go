package server

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/vanguard"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/api/ingestion/v1/ingestionv1connect"
	"github.com/galaxy-io/filament/eventbus"
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
}

// Option configures a Server.
type Option func(*Server)

// WithSecrets sets the secrets provider used to resolve secret refs.
func WithSecrets(secrets filament.Secrets) Option { return func(s *Server) { s.secrets = secrets } }

// New returns a Server wired to the given providers.
func New(sources filament.SourceRegistry, sinks filament.SinkRegistry, store filament.DataStore, orch runSubmitter, bus eventbus.Bus, opts ...Option) *Server {
	s := &Server{
		sources: sources,
		sinks:   sinks,
		store:   store,
		orch:    orch,
		bus:     bus,
	}
	if schedules, ok := store.(filament.PipelineScheduleStore); ok {
		s.schedules = schedules
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Mount registers the Connect handler on mux, plus REST routes under /v1
// transcoded from the google.api.http annotations.
func (a *Server) Mount(mux *http.ServeMux) {
	path, handler := ingestionv1connect.NewIngestionServiceHandler(a)
	transcoder, err := vanguard.NewTranscoder([]*vanguard.Service{
		vanguard.NewService(path, handler),
	})
	if err != nil {
		panic(fmt.Sprintf("server: build transcoder: %v", err))
	}
	h := withCORS(transcoder)
	mux.Handle(path, h)
	mux.Handle("/v1/", h)
}

var _ ingestionv1connect.IngestionServiceHandler = (*Server)(nil)
