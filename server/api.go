package server

import (
	"context"
	"net/http"
	"sync"

	"connectrpc.com/grpcreflect"

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

	mu          sync.RWMutex
	pipelines   map[string]*ingestionv1.Pipeline
	connections map[string]*ingestionv1.Connection
	nextID      int64
	nextConnID  int64
}

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

type MountOptions struct {
	Reflection bool
}

type MountOption func(*MountOptions)

func WithReflection(enabled bool) MountOption {
	return func(o *MountOptions) { o.Reflection = enabled }
}

func (a *Server) Mount(mux *http.ServeMux, opts ...MountOption) {
	var options MountOptions
	for _, opt := range opts {
		opt(&options)
	}

	path, handler := ingestionv1connect.NewIngestionServiceHandler(a)
	mux.Handle(path, withCORS(handler))
	if options.Reflection {
		reflector := grpcreflect.NewStaticReflector(ingestionv1connect.IngestionServiceName)
		path, handler = grpcreflect.NewHandlerV1(reflector)
		mux.Handle(path, withCORS(handler))
		path, handler = grpcreflect.NewHandlerV1Alpha(reflector)
		mux.Handle(path, withCORS(handler))
	}
}

var _ ingestionv1connect.IngestionServiceHandler = (*Server)(nil)
