package app

import (
	"context"
	"fmt"
	"maps"

	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// SavePipeline validates and persists a pipeline through the selected target.
func (s *Service) SavePipeline(ctx context.Context, request SavePipelineRequest) (model.Pipeline, error) {
	document, err := s.Configuration(ctx)
	if err != nil {
		return model.Pipeline{}, err
	}
	catalog, err := s.target.Catalog(ctx)
	if err != nil {
		return model.Pipeline{}, err
	}
	existing, exists := document.Pipelines[request.Name]
	if request.Create && exists {
		return model.Pipeline{}, fmt.Errorf("pipeline %q already exists", request.Name)
	}
	if !request.Create && !exists {
		return model.Pipeline{}, fmt.Errorf("pipeline %q does not exist", request.Name)
	}
	if exists && existing.Info.EditBlockedReason != "" {
		return model.Pipeline{}, fmt.Errorf("pipeline %q cannot be edited by the CLI without losing data: %s; edit it in the UI", request.Name, existing.Info.EditBlockedReason)
	}
	var base *model.Pipeline
	if exists {
		base = &existing
	}
	pipeline, err := BuildPipeline(request, base, document, catalog)
	if err != nil {
		return model.Pipeline{}, err
	}
	testDocument := document
	testDocument.Pipelines = cloneMap(document.Pipelines)
	testDocument.Pipelines[request.Name] = pipeline
	if err := s.target.ValidateConfiguration(ctx, testDocument); err != nil {
		return model.Pipeline{}, err
	}
	if request.Create {
		return s.target.CreatePipeline(ctx, request.Name, pipeline)
	}
	return s.target.UpdatePipeline(ctx, request.Name, pipeline)
}

// BuildPipeline applies a typed request without persisting it.
func BuildPipeline(request SavePipelineRequest, existing *model.Pipeline, document model.Document, catalog model.Catalog) (model.Pipeline, error) {
	pipeline := model.Pipeline{SyncMode: "full", WriteMode: "replace"}
	oldSourceType, oldSinkType := "", ""
	if existing != nil {
		pipeline = *existing
		pipeline.Source.Config = cloneConfigMap(existing.Source.Config)
		pipeline.Source.SecretRefs = maps.Clone(existing.Source.SecretRefs)
		pipeline.Sink.Config = cloneConfigMap(existing.Sink.Config)
		pipeline.Sink.SecretRefs = maps.Clone(existing.Sink.SecretRefs)
		oldSourceType = document.Sources[existing.Source.Ref].Type
		oldSinkType = document.Sinks[existing.Sink.Ref].Type
	}
	if request.Source != "" {
		pipeline.Source.Ref = request.Source
	}
	if request.Sink != "" {
		pipeline.Sink.Ref = request.Sink
	}
	if request.Resources != nil {
		pipeline.Resources = append([]string(nil), (*request.Resources)...)
	}
	if request.SyncMode != "" {
		pipeline.SyncMode = request.SyncMode
	}
	if request.WriteMode != "" {
		pipeline.WriteMode = request.WriteMode
	}
	if pipeline.Source.Ref == "" || pipeline.Sink.Ref == "" {
		return pipeline, fmt.Errorf("pipeline %q: source and sink are required", request.Name)
	}
	source, sourceOK := document.Sources[pipeline.Source.Ref]
	sink, sinkOK := document.Sinks[pipeline.Sink.Ref]
	if !sourceOK || !sinkOK {
		return pipeline, fmt.Errorf("pipeline %q: source and sink must name saved connections", request.Name)
	}
	if existing != nil && oldSourceType != source.Type {
		pipeline.Source.Config = map[string]any{}
	}
	if existing != nil && oldSinkType != sink.Type {
		pipeline.Sink.Config = map[string]any{}
	}
	pipeline.Source.Config = applyConfigPatch(pipeline.Source.Config, request.SourceConfig)
	pipeline.Sink.Config = applyConfigPatch(pipeline.Sink.Config, request.SinkConfig)
	var err error
	pipeline.Source.SecretRefs, err = UpdateSecretReferences(catalog.Sources[source.Type].Config, request.SourceConfig, pipeline.Source.SecretRefs)
	if err == nil {
		pipeline.Sink.SecretRefs, err = UpdateSecretReferences(catalog.Sinks[sink.Type].Config, request.SinkConfig, pipeline.Sink.SecretRefs)
	}
	if err != nil {
		return pipeline, err
	}
	pipeline.Source.Config, err = normalizeSavedSecretReferences(catalog.Sources[source.Type].Config, pipeline.Source.Config)
	if err == nil {
		pipeline.Sink.Config, err = normalizeSavedSecretReferences(catalog.Sinks[sink.Type].Config, pipeline.Sink.Config)
	}
	if err != nil {
		return pipeline, err
	}
	graph := model.SimplePipelineGraph(pipeline)
	pipeline.Graph = &graph
	return pipeline, nil
}

// DeleteSavedPipeline verifies existence before persistence.
func (s *Service) DeleteSavedPipeline(ctx context.Context, name string) error {
	document, err := s.Configuration(ctx)
	if err != nil {
		return err
	}
	if _, exists := document.Pipelines[name]; !exists {
		return fmt.Errorf("pipeline %q does not exist", name)
	}
	return s.target.DeletePipeline(ctx, name, document.Pipelines[name].Metadata)
}
