// Package app contains target-neutral CLI use cases.
package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// Target supplies operations for one selected local or remote Filament target.
type Target interface {
	Catalog(context.Context) (model.Catalog, error)
	ListConnections(context.Context, string) ([]model.NamedConnection, error)
	GetConnection(context.Context, string, string) (model.Connection, error)
	CreateConnection(context.Context, string, string, model.Connection) (model.Connection, error)
	UpdateConnection(context.Context, string, string, model.Connection) (model.Connection, error)
	DeleteConnection(context.Context, string, string, model.EntityMetadata) error
	ListPipelines(context.Context) ([]model.NamedPipeline, error)
	GetPipeline(context.Context, string) (model.Pipeline, error)
	CreatePipeline(context.Context, string, model.Pipeline) (model.Pipeline, error)
	UpdatePipeline(context.Context, string, model.Pipeline) (model.Pipeline, error)
	DeletePipeline(context.Context, string, model.EntityMetadata) error
	ValidateConfiguration(context.Context, model.Document) error
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
	document := model.NewDocument()
	for _, kind := range []string{"source", "sink"} {
		connections, err := s.target.ListConnections(ctx, kind)
		if err != nil {
			return model.Document{}, err
		}
		for _, item := range connections {
			if kind == "source" {
				document.Sources[item.Name] = item.Connection
			} else {
				document.Sinks[item.Name] = item.Connection
			}
		}
	}
	pipelines, err := s.target.ListPipelines(ctx)
	if err != nil {
		return model.Document{}, err
	}
	for _, item := range pipelines {
		document.Pipelines[item.Name] = item.Pipeline
	}
	return document, nil
}

// Connections lists connections of one kind from the selected target.
func (s *Service) Connections(ctx context.Context, kind string) (model.ConnectionList, error) {
	connections, err := s.target.ListConnections(ctx, kind)
	if err != nil {
		return model.ConnectionList{}, err
	}
	catalog, err := s.target.Catalog(ctx)
	if err != nil {
		return model.ConnectionList{}, err
	}
	if kind != "source" && kind != "sink" {
		return model.ConnectionList{}, fmt.Errorf("unknown connection kind %q", kind)
	}
	result := model.ConnectionList{Kind: kind, Items: make([]model.ConnectionSummary, 0, len(connections))}
	for _, item := range connections {
		description, _ := catalog.Description(kind, item.Connection.Type)
		result.Items = append(result.Items, model.ConnectionSummary{
			Name: item.Name, Connector: item.Connection.Type, Description: description,
		})
	}
	return result, nil
}

// Pipelines lists pipelines from the selected target.
func (s *Service) Pipelines(ctx context.Context) (model.PipelineList, error) {
	pipelines, err := s.target.ListPipelines(ctx)
	if err != nil {
		return model.PipelineList{}, err
	}
	result := model.PipelineList{Items: make([]model.PipelineSummary, 0, len(pipelines))}
	for _, item := range pipelines {
		pipeline := item.Pipeline
		result.Items = append(result.Items, model.PipelineSummary{
			Name: item.Name, Source: pipeline.Source.Ref, Sink: pipeline.Sink.Ref,
			ResourceCount: len(pipeline.Resources), AllResources: len(pipeline.Resources) == 0,
			SyncMode: pipeline.SyncMode, WriteMode: pipeline.WriteMode,
		})
	}
	return result, nil
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
