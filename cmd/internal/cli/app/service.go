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
// Listings are cursor-paged; DrainPages collects a full listing.
type ConnectionTarget interface {
	ListConnections(context.Context, string, model.PageRequest) (model.Page[model.NamedConnection], error)
	GetConnection(context.Context, string, string) (model.Connection, error)
	CreateConnection(context.Context, string, string, model.Connection) (model.Connection, error)
	UpdateConnection(context.Context, string, string, model.Connection) (model.Connection, error)
	DeleteConnection(context.Context, string, string, model.EntityMetadata) error
}

// PipelineTarget supplies explicit pipeline queries and mutations.
// Listings are cursor-paged; DrainPages collects a full listing.
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

// PipelineModesTarget optionally asks a deployment to resolve connector
// capabilities for a proposed graph. Remote targets implement this with the
// same ValidatePipeline RPC used by the web canvas.
type PipelineModesTarget interface {
	PipelineModes(context.Context, model.Pipeline) (model.PipelineModes, error)
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

// RunHistoryTarget is an optional target capability: the target retains run
// history that can be listed. In-process targets keep none.
type RunHistoryTarget interface {
	ListRuns(ctx context.Context, request model.RunListRequest) (model.RunList, error)
}

// EphemeralTarget is an optional target capability: the target's deployment
// state dies with the process, so run-scoped entities — inline runs, override
// runs — can be pushed straight to the deployment without touching the
// durable document.
type EphemeralTarget interface {
	PushConnection(ctx context.Context, kind, name string, connection model.Connection) (model.Connection, error)
	PushPipeline(ctx context.Context, name string, pipeline model.Pipeline) (model.Pipeline, error)
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

// Connections lists connections of one kind from the selected target.
func (s *Service) Connections(ctx context.Context, kind string) (model.ConnectionList, error) {
	connections, err := s.allConnections(ctx, kind)
	if err != nil {
		return model.ConnectionList{}, err
	}
	return s.connectionList(ctx, kind, model.Page[model.NamedConnection]{Items: connections, Total: len(connections)})
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

// ConnectionPage lists one page of connections using target-native cursors
// when available.
func (s *Service) ConnectionPage(ctx context.Context, kind string, request model.PageRequest) (model.ConnectionList, error) {
	page, err := s.target.ListConnections(ctx, kind, request)
	if err != nil {
		return model.ConnectionList{}, err
	}
	return s.connectionList(ctx, kind, page)
}

func (s *Service) connectionList(ctx context.Context, kind string, page model.Page[model.NamedConnection]) (model.ConnectionList, error) {
	catalog, err := s.target.Catalog(ctx)
	if err != nil {
		return model.ConnectionList{}, err
	}
	if kind != "source" && kind != "sink" {
		return model.ConnectionList{}, fmt.Errorf("unknown connection kind %q", kind)
	}
	result := model.ConnectionList{
		Kind: kind, Items: make([]model.ConnectionSummary, 0, len(page.Items)), Total: page.Total,
		NextCursor: page.NextCursor, PreviousCursor: page.PreviousCursor,
	}
	for _, item := range page.Items {
		description, _ := catalog.Description(kind, item.Connection.Type)
		result.Items = append(result.Items, model.ConnectionSummary{
			Name: item.Name, Connector: item.Connection.Type, Description: description,
			Replication: item.Connection.Info.Replication,
			CreatedAt:   item.Connection.Info.CreatedAt, UpdatedAt: item.Connection.Info.UpdatedAt,
		})
	}
	return result, nil
}

// Runs lists the selected target's run history, optionally for one pipeline.
func (s *Service) Runs(ctx context.Context, request model.RunListRequest) (model.RunList, error) {
	target, ok := s.target.(RunHistoryTarget)
	if !ok {
		return model.RunList{}, errors.New("this target keeps no run history")
	}
	return target.ListRuns(ctx, request)
}

// Pipelines lists pipelines from the selected target.
func (s *Service) Pipelines(ctx context.Context) (model.PipelineList, error) {
	pipelines, err := s.allPipelines(ctx)
	if err != nil {
		return model.PipelineList{}, err
	}
	return pipelineList(model.Page[model.NamedPipeline]{Items: pipelines, Total: len(pipelines)}), nil
}

// PipelinePage lists one page of pipelines using target-native cursors when
// available.
func (s *Service) PipelinePage(ctx context.Context, request model.PageRequest) (model.PipelineList, error) {
	page, err := s.target.ListPipelines(ctx, request)
	if err != nil {
		return model.PipelineList{}, err
	}
	return pipelineList(page), nil
}

func pipelineList(page model.Page[model.NamedPipeline]) model.PipelineList {
	result := model.PipelineList{
		Items: make([]model.PipelineSummary, 0, len(page.Items)), Total: page.Total,
		NextCursor: page.NextCursor, PreviousCursor: page.PreviousCursor,
	}
	for _, item := range page.Items {
		pipeline := item.Pipeline
		result.Items = append(result.Items, model.PipelineSummary{
			Name: item.Name, Source: pipeline.Source.Ref, Sink: pipeline.Sink.Ref,
			ResourceCount: len(pipeline.Resources), AllResources: len(pipeline.Resources) == 0,
			SyncMode: pipeline.SyncMode, WriteMode: pipeline.WriteMode,
			Schedule: pipeline.Info.Schedule, LastRunStatus: pipeline.Info.LastRunStatus,
			LastRunAt: pipeline.Info.LastRunAt, UpdatedAt: pipeline.Info.UpdatedAt,
		})
	}
	return result
}

// Discover returns resources available for a connector request.
func (s *Service) Discover(ctx context.Context, request model.DiscoverRequest) (model.ResourceList, error) {
	return s.target.Discover(ctx, request)
}

// PipelineModes returns authoritative graph capabilities when the target can
// resolve them, otherwise it derives the local connector catalog's modes.
func (s *Service) PipelineModes(ctx context.Context, pipeline model.Pipeline) (model.PipelineModes, error) {
	if target, ok := s.target.(PipelineModesTarget); ok {
		return target.PipelineModes(ctx, pipeline)
	}
	document, err := s.Configuration(ctx)
	if err != nil {
		return model.PipelineModes{}, err
	}
	catalog, err := s.Catalog(ctx)
	if err != nil {
		return model.PipelineModes{}, err
	}
	source, sourceOK := document.Sources[pipeline.Source.Ref]
	sink, sinkOK := document.Sinks[pipeline.Sink.Ref]
	if !sourceOK || !sinkOK {
		return model.PipelineModes{}, fmt.Errorf("source and sink must name saved connections")
	}
	result := model.PipelineModes{}
	for _, mode := range catalog.Sources[source.Type].Modes {
		result.ReadModes = append(result.ReadModes, mode.String())
	}
	for _, capability := range catalog.Sinks[sink.Type].Capabilities.WritePolicies {
		result.WriteModes = appendUnique(result.WriteModes, string(capability.Mode))
	}
	return result, nil
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

// SubmitRun starts a prepared run on the selected target.
func (s *Service) SubmitRun(ctx context.Context, submission model.RunSubmission) (model.RunGroup, error) {
	return s.target.SubmitRun(ctx, submission)
}

// TailRun streams normalized progress for a submitted run group.
func (s *Service) TailRun(ctx context.Context, group model.RunGroup, observe func(model.RunEvent)) (model.RunResult, error) {
	result, err := s.target.TailRun(ctx, group, observe)
	if err == nil && result.Status != "" && result.Status != "complete" {
		err = fmt.Errorf("run ended with status %s", result.Status)
	}
	return result, err
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
