package app

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// SavePipeline validates and persists a pipeline through the selected target.
func (s *Service) SavePipeline(ctx context.Context, request SavePipelineRequest) (model.Pipeline, error) {
	document, err := s.target.Configuration(ctx)
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
	if err := ValidateDocument(testDocument, catalog); err != nil {
		return model.Pipeline{}, err
	}
	if err := s.target.PutPipeline(ctx, request.Name, pipeline); err != nil {
		return model.Pipeline{}, err
	}
	return pipeline, nil
}

// BuildPipeline applies a typed request without persisting it.
func BuildPipeline(request SavePipelineRequest, existing *model.Pipeline, document model.Document, catalog model.Catalog) (model.Pipeline, error) {
	pipeline := model.Pipeline{SyncMode: "full", WriteMode: "replace"}
	oldSourceType, oldSinkType := "", ""
	if existing != nil {
		pipeline = *existing
		pipeline.Source.Config = cloneConfigMap(existing.Source.Config)
		pipeline.Sink.Config = cloneConfigMap(existing.Sink.Config)
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
	pipeline.Source.Config, err = normalizeSavedSecretReferences(catalog.Sources[source.Type].Config, pipeline.Source.Config)
	if err == nil {
		pipeline.Sink.Config, err = normalizeSavedSecretReferences(catalog.Sinks[sink.Type].Config, pipeline.Sink.Config)
	}
	if err != nil {
		return pipeline, err
	}
	return pipeline, nil
}

// DeleteSavedPipeline verifies existence before persistence.
func (s *Service) DeleteSavedPipeline(ctx context.Context, name string) error {
	document, err := s.target.Configuration(ctx)
	if err != nil {
		return err
	}
	if _, exists := document.Pipelines[name]; !exists {
		return fmt.Errorf("pipeline %q does not exist", name)
	}
	return s.target.DeletePipeline(ctx, name)
}
