package app

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
	"github.com/galaxy-io/filament/internal/naming"
)

// PrepareRun builds a target submission from a saved or inline run.
func (s *Service) PrepareRun(ctx context.Context, request RunRequest) (model.RunSubmission, error) {
	catalog, err := s.target.Catalog(ctx)
	if err != nil {
		return model.RunSubmission{}, err
	}
	if request.Inline != nil {
		spec, prepareErr := prepareInlineRun(*request.Inline, catalog)
		return model.RunSubmission{Spec: spec}, prepareErr
	}
	if request.Pipeline == "" {
		return model.RunSubmission{}, fmt.Errorf("pipeline is required")
	}
	document, err := s.Configuration(ctx)
	if err != nil {
		return model.RunSubmission{}, err
	}
	if err := s.target.ValidateConfiguration(ctx, document); err != nil {
		return model.RunSubmission{}, err
	}
	pipeline, ok := document.Pipelines[request.Pipeline]
	if !ok {
		return model.RunSubmission{}, fmt.Errorf("pipeline %q does not exist", request.Pipeline)
	}
	request.Overrides.Name = request.Pipeline
	override := !emptyPipelineRequest(request.Overrides)
	if override {
		pipeline, err = BuildPipeline(request.Overrides, &pipeline, document, catalog)
		if err != nil {
			return model.RunSubmission{}, err
		}
		testDocument := document
		testDocument.Pipelines = cloneMap(document.Pipelines)
		testDocument.Pipelines[request.Pipeline] = pipeline
		if err := s.target.ValidateConfiguration(ctx, testDocument); err != nil {
			return model.RunSubmission{}, err
		}
	}
	source := document.Sources[pipeline.Source.Ref]
	sink := document.Sinks[pipeline.Sink.Ref]
	sourceConfig, err := ResolvedConnectionConfig(source, pipeline.Source.Config, catalog.Sources[source.Type].Config)
	if err != nil {
		return model.RunSubmission{}, fmt.Errorf("source %q: %w", pipeline.Source.Ref, err)
	}
	sinkConfig, err := ResolvedConnectionConfig(sink, pipeline.Sink.Config, catalog.Sinks[sink.Type].Config)
	if err != nil {
		return model.RunSubmission{}, fmt.Errorf("sink %q: %w", pipeline.Sink.Ref, err)
	}
	sinkConfig = applySinkSchemaDefault(sinkConfig, catalog.Sinks[sink.Type], pipeline.Source.Ref)
	spec, err := makeRunSpec(request.Pipeline, source.Type, sink.Type, sourceConfig, sinkConfig, pipeline.Resources, pipeline.SyncMode, pipeline.WriteMode)
	if err != nil {
		return model.RunSubmission{}, err
	}
	return model.RunSubmission{
		Pipeline: &model.EntityReference{Name: request.Pipeline, Metadata: pipeline.Metadata},
		Spec:     spec,
		Override: override,
	}, nil
}

// ExecuteRun prepares and runs a typed request through the selected target.
func (s *Service) ExecuteRun(ctx context.Context, request RunRequest, observe func(model.RunEvent)) (filament.RunSpec, model.RunResult, error) {
	submission, err := s.PrepareRun(ctx, request)
	if err != nil {
		return filament.RunSpec{}, model.RunResult{}, err
	}
	group, err := s.SubmitRun(ctx, submission)
	if err != nil {
		return submission.Spec, model.RunResult{}, err
	}
	result, err := s.TailRun(ctx, group, observe)
	return submission.Spec, result, err
}

func prepareInlineRun(run InlineRun, catalog model.Catalog) (filament.RunSpec, error) {
	if run.Source.Connector == "" || run.Sink.Connector == "" {
		return filament.RunSpec{}, fmt.Errorf("source and sink connectors are required")
	}
	sourceSpec, sourceOK := catalog.Sources[run.Source.Connector]
	if !sourceOK {
		return filament.RunSpec{}, fmt.Errorf("unknown source connector %q", run.Source.Connector)
	}
	sinkSpec, sinkOK := catalog.Sinks[run.Sink.Connector]
	if !sinkOK {
		return filament.RunSpec{}, fmt.Errorf("unknown sink connector %q", run.Sink.Connector)
	}
	sourceConfig := canonicalizeConfig(sourceSpec.Config, run.Source.Config)
	sourceConfig, err := normalizeSavedSecretReferences(sourceSpec.Config, sourceConfig)
	if err == nil {
		err = validateFields("source connector", sourceSpec.Config.Fields, sourceConfig)
	}
	if err != nil {
		return filament.RunSpec{}, err
	}
	sinkConfig := canonicalizeConfig(sinkSpec.Config, run.Sink.Config)
	sinkConfig, err = normalizeSavedSecretReferences(sinkSpec.Config, sinkConfig)
	if err == nil {
		err = validateFields("sink connector", sinkSpec.Config.Fields, sinkConfig)
	}
	if err != nil {
		return filament.RunSpec{}, err
	}
	sinkConfig = applySinkSchemaDefault(sinkConfig, sinkSpec, run.Source.Connector)
	return makeRunSpec("", run.Source.Connector, run.Sink.Connector, sourceConfig, sinkConfig, run.Resources, run.SyncMode, run.WriteMode)
}

func makeRunSpec(pipelineID, sourceName, sinkName string, sourceConfig, sinkConfig map[string]any, resources []string, syncName, writeName string) (filament.RunSpec, error) {
	syncMode := filament.ModeFull
	if syncName != "" && syncName != "full" {
		return filament.RunSpec{}, fmt.Errorf("sync mode %q is not supported; use full", syncName)
	}
	writeMode := filament.WriteMode(writeName)
	ingestionType, err := filament.IngestionFor(syncMode, writeMode)
	if err != nil {
		return filament.RunSpec{}, err
	}
	return filament.RunSpec{
		PipelineID:     pipelineID,
		Source:         filament.Ref{Connector: sourceName, Config: sourceConfig},
		Sink:           filament.Ref{Connector: sinkName, Config: sinkConfig},
		Resources:      append([]string(nil), resources...),
		IngestionTypes: map[string]filament.IngestionType{"": ingestionType},
	}, nil
}

func applySinkSchemaDefault(config map[string]any, spec filament.SinkSpec, sourceName string) map[string]any {
	result := model.CloneConfig(config)
	if spec.SchemaField == "" || !isEmpty(result[spec.SchemaField]) {
		return result
	}
	if value := naming.Normalize(sourceName); value != "" {
		result[spec.SchemaField] = value
	}
	return result
}
