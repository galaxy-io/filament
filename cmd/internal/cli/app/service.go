// Package app contains target-neutral CLI use cases.
package app

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// Target supplies operations for one selected local or remote Filament target.
type Target interface {
	Catalog(context.Context) (model.Catalog, error)
	Configuration(context.Context) (model.Document, error)
	PutConnection(context.Context, string, string, model.Connection) error
	DeleteConnection(context.Context, string, string) error
	PutPipeline(context.Context, string, model.Pipeline) error
	DeletePipeline(context.Context, string) error
	Discover(context.Context, model.DiscoverRequest) (model.ResourceList, error)
	Run(context.Context, filament.RunSpec, func(model.RunEvent)) (model.RunResult, error)
}

// RawConfigurationTarget is an optional target capability for byte-preserving
// configuration editing. Remote targets do not need to expose it.
type RawConfigurationTarget interface {
	ConfigurationLocation() string
	ReadConfiguration(context.Context) ([]byte, error)
	WriteConfiguration(context.Context, []byte) error
}

// ErrRawConfigurationUnsupported indicates that the selected target only
// supports structured configuration operations.
var ErrRawConfigurationUnsupported = errors.New("target does not support raw configuration editing")

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
	doc, err := s.target.Configuration(ctx)
	if err != nil {
		return model.ConnectionList{}, err
	}
	catalog, err := s.target.Catalog(ctx)
	if err != nil {
		return model.ConnectionList{}, err
	}
	var connections map[string]model.Connection
	switch kind {
	case "source":
		connections = doc.Sources
	case "sink":
		connections = doc.Sinks
	default:
		return model.ConnectionList{}, fmt.Errorf("unknown connection kind %q", kind)
	}
	names := make([]string, 0, len(connections))
	for name := range connections {
		names = append(names, name)
	}
	sort.Strings(names)
	result := model.ConnectionList{Kind: kind, Items: make([]model.ConnectionSummary, 0, len(names))}
	for _, name := range names {
		connection := connections[name]
		description, _ := catalog.Description(kind, connection.Type)
		result.Items = append(result.Items, model.ConnectionSummary{
			Name: name, Connector: connection.Type, Description: description,
		})
	}
	return result, nil
}

// Pipelines lists pipelines from the selected target.
func (s *Service) Pipelines(ctx context.Context) (model.PipelineList, error) {
	doc, err := s.target.Configuration(ctx)
	if err != nil {
		return model.PipelineList{}, err
	}
	names := make([]string, 0, len(doc.Pipelines))
	for name := range doc.Pipelines {
		names = append(names, name)
	}
	sort.Strings(names)
	result := model.PipelineList{Items: make([]model.PipelineSummary, 0, len(names))}
	for _, name := range names {
		pipeline := doc.Pipelines[name]
		result.Items = append(result.Items, model.PipelineSummary{
			Name: name, Source: pipeline.Source.Ref, Sink: pipeline.Sink.Ref,
			ResourceCount: len(pipeline.Resources), AllResources: len(pipeline.Resources) == 0,
			SyncMode: pipeline.SyncMode, WriteMode: pipeline.WriteMode,
		})
	}
	return result, nil
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

func (s *Service) ConfigurationLocation() string {
	if target, ok := s.target.(RawConfigurationTarget); ok {
		return target.ConfigurationLocation()
	}
	return ""
}

func (s *Service) ReadConfiguration(ctx context.Context) ([]byte, error) {
	target, ok := s.target.(RawConfigurationTarget)
	if !ok {
		return nil, ErrRawConfigurationUnsupported
	}
	return target.ReadConfiguration(ctx)
}

func (s *Service) WriteConfiguration(ctx context.Context, data []byte) error {
	target, ok := s.target.(RawConfigurationTarget)
	if !ok {
		return ErrRawConfigurationUnsupported
	}
	return target.WriteConfiguration(ctx, data)
}
