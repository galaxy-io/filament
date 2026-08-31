// Package app contains target-neutral CLI use cases.
package app

import (
	"context"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// Target supplies operations for one selected local or remote Filament target.
type Target interface {
	Catalog(context.Context) (model.Catalog, error)
	Configuration(context.Context) (model.Document, error)
	ListConnections(context.Context, string) (model.ConnectionList, error)
	ListPipelines(context.Context) (model.PipelineList, error)
	PutConnection(context.Context, string, string, model.Connection) error
	DeleteConnection(context.Context, string, string) error
	PutPipeline(context.Context, string, model.Pipeline) error
	DeletePipeline(context.Context, string) error
	Discover(context.Context, model.DiscoverRequest) (model.ResourceList, error)
	Run(context.Context, filament.RunSpec, func(model.RunEvent)) (model.RunResult, error)
	ConfigurationLocation() string
	ReadConfiguration(context.Context) ([]byte, error)
	WriteConfiguration(context.Context, []byte) error
}

// Service exposes target-neutral use cases to CLI renderers and input adapters.
type Service struct {
	target Target
}

// NewService binds use cases to the selected target.
func NewService(target Target) *Service {
	return &Service{target: target}
}

func (s *Service) Catalog(ctx context.Context) (model.Catalog, error) {
	return s.target.Catalog(ctx)
}

func (s *Service) Configuration(ctx context.Context) (model.Document, error) {
	return s.target.Configuration(ctx)
}

// Connections lists connections of one kind from the selected target.
func (s *Service) Connections(ctx context.Context, kind string) (model.ConnectionList, error) {
	return s.target.ListConnections(ctx, kind)
}

// Pipelines lists pipelines from the selected target.
func (s *Service) Pipelines(ctx context.Context) (model.PipelineList, error) {
	return s.target.ListPipelines(ctx)
}

func (s *Service) PutConnection(ctx context.Context, kind, name string, connection model.Connection) error {
	return s.target.PutConnection(ctx, kind, name, connection)
}

func (s *Service) DeleteConnection(ctx context.Context, kind, name string) error {
	return s.target.DeleteConnection(ctx, kind, name)
}

func (s *Service) PutPipeline(ctx context.Context, name string, pipeline model.Pipeline) error {
	return s.target.PutPipeline(ctx, name, pipeline)
}

func (s *Service) DeletePipeline(ctx context.Context, name string) error {
	return s.target.DeletePipeline(ctx, name)
}

func (s *Service) Discover(ctx context.Context, request model.DiscoverRequest) (model.ResourceList, error) {
	return s.target.Discover(ctx, request)
}

func (s *Service) Run(ctx context.Context, spec filament.RunSpec, observe func(model.RunEvent)) (model.RunResult, error) {
	return s.target.Run(ctx, spec, observe)
}

func (s *Service) ConfigurationLocation() string { return s.target.ConfigurationLocation() }

func (s *Service) ReadConfiguration(ctx context.Context) ([]byte, error) {
	return s.target.ReadConfiguration(ctx)
}

func (s *Service) WriteConfiguration(ctx context.Context, data []byte) error {
	return s.target.WriteConfiguration(ctx, data)
}
