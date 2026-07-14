package server

import (
	"context"
	"net/http"
	"sync"

	ingestion "github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
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

	mu          sync.RWMutex
	pipelines   map[string]*ingestionv1.Pipeline
	connections map[string]*ingestionv1.Connection
	nextID      int64
	nextConnID  int64
}

// New returns a Server wired to the given providers.
func New(sources ingestion.SourceRegistry, sinks ingestion.SinkRegistry, store ingestion.DataStore, orch runSubmitter, bus eventbus.Bus) *Server {
	return &Server{
		sources:     sources,
		sinks:       sinks,
		store:       store,
		orch:        orch,
		bus:         bus,
		pipelines:   map[string]*ingestionv1.Pipeline{},
		connections: map[string]*ingestionv1.Connection{},
	}
}

// Mount registers the Connect handler on mux.
func (a *Server) Mount(mux *http.ServeMux) {
	path, handler := ingestionv1connect.NewIngestionServiceHandler(a)
	mux.Handle(path, withCORS(handler))
}

var _ ingestionv1connect.IngestionServiceHandler = (*Server)(nil)
