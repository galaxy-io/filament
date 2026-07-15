package server

import (
	"context"
	"net/http"

	ingestion "github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/api/ingestion/v1/ingestionv1connect"
	"github.com/galaxy-io/filament/eventbus"
)

type runSubmitter interface {
	Submit(context.Context, ingestion.RunRequest) (ingestion.RunID, error)
}

// Server implements the ingestion Connect API over the registries, store,
// orchestrator, and bus.
type Server struct {
	sources ingestion.SourceRegistry
	sinks   ingestion.SinkRegistry
	store   ingestion.DataStore
	orch    runSubmitter
	bus     eventbus.Bus
	secrets ingestion.Secrets
}

type Option func(*Server)

func WithSecrets(secrets ingestion.Secrets) Option { return func(s *Server) { s.secrets = secrets } }

// New returns a Server wired to the given providers.
func New(sources ingestion.SourceRegistry, sinks ingestion.SinkRegistry, store ingestion.DataStore, orch runSubmitter, bus eventbus.Bus, opts ...Option) *Server {
	s := &Server{
		sources: sources,
		sinks:   sinks,
		store:   store,
		orch:    orch,
		bus:     bus,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Mount registers the Connect handler on mux.
func (a *Server) Mount(mux *http.ServeMux) {
	path, handler := ingestionv1connect.NewIngestionServiceHandler(a)
	mux.Handle(path, withCORS(handler))
}

var _ ingestionv1connect.IngestionServiceHandler = (*Server)(nil)
