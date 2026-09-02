package app

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// PrepareRun builds a target submission from a saved or inline run. Inline
// and override runs need an EphemeralTarget: their entities are pushed to the
// deployment for this run only, which only makes sense where deployment state
// dies with the process.
func (s *Service) PrepareRun(ctx context.Context, request RunRequest) (model.RunSubmission, error) {
	catalog, err := s.target.Catalog(ctx)
	if err != nil {
		return model.RunSubmission{}, err
	}
	if request.Inline != nil {
		return s.prepareInlineRun(ctx, *request.Inline, catalog)
	}
	if request.Pipeline == "" {
		return model.RunSubmission{}, fmt.Errorf("pipeline is required")
	}
	document, err := s.Configuration(ctx)
	if err != nil {
		return model.RunSubmission{}, err
	}
	pipeline, ok := document.Pipelines[request.Pipeline]
	if !ok {
		return model.RunSubmission{}, fmt.Errorf("pipeline %q does not exist", request.Pipeline)
	}
	request.Overrides.Name = request.Pipeline
	saved := pipeline
	override := !emptyPipelineRequest(request.Overrides)
	if override {
		pipeline, err = BuildPipeline(request.Overrides, &pipeline, document, catalog)
		if err != nil {
			return model.RunSubmission{}, err
		}
		// Flag parsing prefills the saved refs, so presence alone is not an
		// override; only a run that actually changes the pipeline is.
		override = !model.PipelinesEquivalent(pipeline, saved)
	}
	if override {
		pusher, ok := s.target.(EphemeralTarget)
		if !ok {
			return model.RunSubmission{}, fmt.Errorf("this target does not support override flags; edit the pipeline instead")
		}
		// The override runs as an adhoc copy so the saved pipeline — and the
		// YAML behind it — stays untouched.
		request.Pipeline = model.AdhocPrefix + request.Pipeline
		if pipeline, err = pusher.PushPipeline(ctx, request.Pipeline, pipeline); err != nil {
			return model.RunSubmission{}, err
		}
	}
	source := document.Sources[pipeline.Source.Ref]
	sink := document.Sinks[pipeline.Sink.Ref]
	return model.RunSubmission{
		Pipeline: &model.EntityReference{Name: request.Pipeline, Metadata: pipeline.Metadata},
		Spec: filament.RunSpec{
			PipelineID: request.Pipeline,
			Source:     filament.Ref{Connector: source.Type},
			Sink:       filament.Ref{Connector: sink.Type},
			Resources:  append([]string(nil), pipeline.Resources...),
		},
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

// prepareInlineRun pushes an inline source, sink, and pipeline to an
// ephemeral deployment and submits the pipeline like any saved run. The sink
// schema default and mode validation are the deployment's job.
func (s *Service) prepareInlineRun(ctx context.Context, run InlineRun, catalog model.Catalog) (model.RunSubmission, error) {
	pusher, ok := s.target.(EphemeralTarget)
	if !ok {
		return model.RunSubmission{}, fmt.Errorf("this target runs saved pipelines; create the pipeline first")
	}
	if run.Source.Connector == "" || run.Sink.Connector == "" {
		return model.RunSubmission{}, fmt.Errorf("source and sink connectors are required")
	}
	sourceSpec, sourceOK := catalog.Sources[run.Source.Connector]
	if !sourceOK {
		return model.RunSubmission{}, fmt.Errorf("unknown source connector %q", run.Source.Connector)
	}
	sinkSpec, sinkOK := catalog.Sinks[run.Sink.Connector]
	if !sinkOK {
		return model.RunSubmission{}, fmt.Errorf("unknown sink connector %q", run.Sink.Connector)
	}
	sourceConfig := canonicalizeConfig(sourceSpec.Config, run.Source.Config)
	sourceConfig, err := normalizeSavedSecretReferences(sourceSpec.Config, sourceConfig)
	if err == nil {
		err = validateFields("source connector", sourceSpec.Config.Fields, sourceConfig)
	}
	if err != nil {
		return model.RunSubmission{}, err
	}
	sinkConfig := canonicalizeConfig(sinkSpec.Config, run.Sink.Config)
	sinkConfig, err = normalizeSavedSecretReferences(sinkSpec.Config, sinkConfig)
	if err == nil {
		err = validateFields("sink connector", sinkSpec.Config.Fields, sinkConfig)
	}
	if err != nil {
		return model.RunSubmission{}, err
	}
	sourceName := model.AdhocPrefix + "inline-" + run.Source.Connector
	sinkName := model.AdhocPrefix + "inline-" + run.Sink.Connector
	if _, err := pusher.PushConnection(ctx, "source", sourceName, model.Connection{Type: run.Source.Connector, Config: sourceConfig}); err != nil {
		return model.RunSubmission{}, err
	}
	if _, err := pusher.PushConnection(ctx, "sink", sinkName, model.Connection{Type: run.Sink.Connector, Config: sinkConfig}); err != nil {
		return model.RunSubmission{}, err
	}
	pipeline, err := pusher.PushPipeline(ctx, model.AdhocPrefix+"inline", model.Pipeline{
		Source:    model.PipelineNode{Ref: sourceName},
		Sink:      model.PipelineNode{Ref: sinkName},
		Resources: append([]string(nil), run.Resources...),
		SyncMode:  run.SyncMode,
		WriteMode: run.WriteMode,
	})
	if err != nil {
		return model.RunSubmission{}, err
	}
	return model.RunSubmission{
		Pipeline: &model.EntityReference{Name: model.AdhocPrefix + "inline", Metadata: pipeline.Metadata},
		Spec: filament.RunSpec{
			PipelineID: model.AdhocPrefix + "inline",
			Source:     filament.Ref{Connector: run.Source.Connector},
			Sink:       filament.Ref{Connector: run.Sink.Connector},
			Resources:  append([]string(nil), run.Resources...),
		},
	}, nil
}
