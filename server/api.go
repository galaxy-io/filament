package server

import (
	"context"
	"net/http"
	"sync"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/api/ingestion/v1/ingestionv1connect"
	"github.com/galaxy-io/filament/eventbus"
)

type runSubmitter interface {
	Submit(context.Context, ingestion.RunRequest) (ingestion.RunID, error)
}

type Server struct {
	sources ingestion.SourceRegistry
	sinks   ingestion.SinkRegistry
	store   ingestion.DataStore
	orch    runSubmitter
	bus     eventbus.Bus

	mu        sync.RWMutex
	pipelines map[string]*ingestionv1.Pipeline
	nextID    int64
}

func New(sources ingestion.SourceRegistry, sinks ingestion.SinkRegistry, store ingestion.DataStore, orch runSubmitter, bus eventbus.Bus) *Server {
	return &Server{
		sources:   sources,
		sinks:     sinks,
		store:     store,
		orch:      orch,
		bus:       bus,
		pipelines: map[string]*ingestionv1.Pipeline{},
	}
}

func (a *Server) Mount(mux *http.ServeMux) {
	path, handler := ingestionv1connect.NewIngestionServiceHandler(a)
	mux.Handle(path, withCORS(handler))
}

var _ ingestionv1connect.IngestionServiceHandler = (*Server)(nil)
