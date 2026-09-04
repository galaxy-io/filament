// Package app contains target-neutral CLI use cases.
package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// CatalogTarget supplies connector metadata for one selected target.
type CatalogTarget interface {
	Catalog(context.Context) (model.Catalog, error)
}

// ConnectionTarget supplies explicit connection queries and mutations.
type ConnectionTarget interface {
	ListConnections(context.Context, string, model.PageRequest) (model.Page[model.NamedConnection], error)
	GetConnection(context.Context, string, string) (model.Connection, error)
	CreateConnection(context.Context, string, string, model.Connection) (model.Connection, error)
	UpdateConnection(context.Context, string, string, model.Connection) (model.Connection, error)
	DeleteConnection(context.Context, string, string, model.EntityMetadata) error
}

// PipelineTarget supplies explicit pipeline queries and mutations.
type PipelineTarget interface {
	ListPipelines(context.Context, model.PageRequest) (model.Page[model.NamedPipeline], error)
	GetPipeline(context.Context, string) (model.Pipeline, error)
	CreatePipeline(context.Context, string, model.Pipeline) (model.Pipeline, error)
	UpdatePipeline(context.Context, string, model.Pipeline) (model.Pipeline, error)
	DeletePipeline(context.Context, string, model.EntityMetadata) error
	ValidateConfiguration(context.Context, model.Document) error
}

// DiscoveryTarget executes connector discovery in the target environment.
type DiscoveryTarget interface {
	Discover(context.Context, model.DiscoverRequest) (model.ResourceList, error)
}

// RunTarget separates run submission, progress streaming, and lifecycle
// commands so local and RPC adapters expose the same interaction model.
type RunTarget interface {
	SubmitRun(context.Context, model.RunSubmission) (model.RunGroup, error)
	TailRun(context.Context, model.RunGroup, func(model.RunEvent)) (model.RunResult, error)
	SignalRun(context.Context, model.RunRef, filament.Signal) error
}

// Target supplies all operations for one selected local or remote Filament
// target. The smaller interfaces document adapter responsibilities and can be
// used independently by focused tests.
type Target interface {
	CatalogTarget
	ConnectionTarget
	PipelineTarget
	DiscoveryTarget
	RunTarget
}

// RunHistoryTarget is an optional target capability that lists past runs.
type RunHistoryTarget interface {
	ListRuns(context.Context, model.RunListRequest) (model.RunList, error)
}

// PipelineModesTarget is an optional target capability that resolves the
// read and write modes a proposed route supports.
type PipelineModesTarget interface {
	PipelineModes(context.Context, model.Pipeline) (model.PipelineModes, error)
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

// Catalog returns connector metadata from the selected target.
func (s *Service) Catalog(ctx context.Context) (model.Catalog, error) {
	return s.target.Catalog(ctx)
}

// Configuration assembles the selected target's connections and pipelines.
func (s *Service) Configuration(ctx context.Context) (model.Document, error) {
	document := model.NewDocument()
	for _, kind := range []string{"source", "sink"} {
		connections, err := s.allConnections(ctx, kind)
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
	pipelines, err := s.allPipelines(ctx)
	if err != nil {
		return model.Document{}, err
	}
	for _, item := range pipelines {
		document.Pipelines[item.Name] = item.Pipeline
	}
	return document, nil
}

func (s *Service) allConnections(ctx context.Context, kind string) ([]model.NamedConnection, error) {
	return model.DrainPages(func(request model.PageRequest) (model.Page[model.NamedConnection], error) {
		return s.target.ListConnections(ctx, kind, request)
	})
}

func (s *Service) allPipelines(ctx context.Context) ([]model.NamedPipeline, error) {
	return model.DrainPages(func(request model.PageRequest) (model.Page[model.NamedPipeline], error) {
		return s.target.ListPipelines(ctx, request)
	})
}

// Connections lists every connection of one kind.
func (s *Service) Connections(ctx context.Context, kind string) (model.ConnectionList, error) {
	connections, err := s.allConnections(ctx, kind)
	if err != nil {
		return model.ConnectionList{}, err
	}
	return s.connectionList(ctx, kind, connections, model.PageInfo{})
}

// ConnectionPage lists one page of connections of one kind.
func (s *Service) ConnectionPage(ctx context.Context, kind string, request model.PageRequest) (model.ConnectionList, error) {
	page, err := s.target.ListConnections(ctx, kind, request)
	if err != nil {
		return model.ConnectionList{}, err
	}
	return s.connectionList(ctx, kind, page.Items, page.PageInfo)
}

func (s *Service) connectionList(ctx context.Context, kind string, connections []model.NamedConnection, info model.PageInfo) (model.ConnectionList, error) {
	if kind != "source" && kind != "sink" {
		return model.ConnectionList{}, fmt.Errorf("unknown connection kind %q", kind)
	}
	catalog, err := s.target.Catalog(ctx)
	if err != nil {
		return model.ConnectionList{}, err
	}
	result := model.ConnectionList{Kind: kind, Items: make([]model.ConnectionSummary, 0, len(connections)), Page: info}
	for _, item := range connections {
		description, _ := catalog.Description(kind, item.Connection.Type)
		result.Items = append(result.Items, model.ConnectionSummary{
			Name: item.Name, Connector: item.Connection.Type, Description: description,
		})
	}
	return result, nil
}

// Pipelines lists every pipeline.
func (s *Service) Pipelines(ctx context.Context) (model.PipelineList, error) {
	pipelines, err := s.allPipelines(ctx)
	if err != nil {
		return model.PipelineList{}, err
	}
	return pipelineList(pipelines, model.PageInfo{}), nil
}

// PipelinePage lists one page of pipelines.
func (s *Service) PipelinePage(ctx context.Context, request model.PageRequest) (model.PipelineList, error) {
	page, err := s.target.ListPipelines(ctx, request)
	if err != nil {
		return model.PipelineList{}, err
	}
	return pipelineList(page.Items, page.PageInfo), nil
}

func pipelineList(pipelines []model.NamedPipeline, info model.PageInfo) model.PipelineList {
	result := model.PipelineList{Items: make([]model.PipelineSummary, 0, len(pipelines)), Page: info}
	for _, item := range pipelines {
		pipeline := item.Pipeline
		result.Items = append(result.Items, model.PipelineSummary{
			Name: item.Name, Source: pipeline.Source.Ref, Sink: pipeline.Sink.Ref,
			ResourceCount: len(pipeline.Resources), AllResources: len(pipeline.Resources) == 0,
			SyncMode: pipeline.SyncMode, WriteMode: pipeline.WriteMode,
		})
	}
	return result
}

// Runs lists one page of run history when the target keeps any.
func (s *Service) Runs(ctx context.Context, request model.RunListRequest) (model.RunList, error) {
	history, ok := s.target.(RunHistoryTarget)
	if !ok {
		return model.RunList{}, errors.New("this target keeps no run history")
	}
	return history.ListRuns(ctx, request)
}

// Discover returns resources available for a connector request.
func (s *Service) Discover(ctx context.Context, request model.DiscoverRequest) (model.ResourceList, error) {
	return s.target.Discover(ctx, request)
}

// SubmitRun starts a prepared run on the selected target.
func (s *Service) SubmitRun(ctx context.Context, submission model.RunSubmission) (model.RunGroup, error) {
	return s.target.SubmitRun(ctx, submission)
}

// TailRun streams normalized progress for a submitted run group.
func (s *Service) TailRun(ctx context.Context, group model.RunGroup, observe func(model.RunEvent)) (model.RunResult, error) {
	return s.target.TailRun(ctx, group, observe)
}

// SignalRun sends a lifecycle command to one target-side run.
func (s *Service) SignalRun(ctx context.Context, run model.RunRef, signal filament.Signal) error {
	return s.target.SignalRun(ctx, run, signal)
}

// ConfigurationLocation returns the raw configuration path when supported.
func (s *Service) ConfigurationLocation() string {
	if target, ok := s.target.(RawConfigurationTarget); ok {
		return target.ConfigurationLocation()
	}
	return ""
}

// ReadConfiguration returns the raw configuration when supported.
func (s *Service) ReadConfiguration(ctx context.Context) ([]byte, error) {
	target, ok := s.target.(RawConfigurationTarget)
	if !ok {
		return nil, ErrRawConfigurationUnsupported
	}
	return target.ReadConfiguration(ctx)
}

// WriteConfiguration replaces the raw configuration when supported.
func (s *Service) WriteConfiguration(ctx context.Context, data []byte) error {
	target, ok := s.target.(RawConfigurationTarget)
	if !ok {
		return ErrRawConfigurationUnsupported
	}
	return target.WriteConfiguration(ctx, data)
}
