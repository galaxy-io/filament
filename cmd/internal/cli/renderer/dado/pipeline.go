package dado

import (
	"context"
	"fmt"
	"strings"

	"github.com/atterpac/dado/inline"

	"github.com/galaxy-io/filament"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
	"github.com/galaxy-io/filament/cmd/internal/cli/renderer/present"
)

func (r *Renderer) managePipelines(ctx context.Context) error {
	for {
		listed, err := r.service.Pipelines(ctx)
		if err != nil {
			return err
		}
		options := make([]interactiveOption, 0, len(listed.Items)+3)
		options = append(options, interactiveOption{label: "+ Create pipeline", value: "__create__", tone: inline.ChoiceToneSuccess})
		options = append(options, pipelineMenuOptions(listed.Items)...)
		options = append(options, interactiveOption{label: "Back", value: interactiveBack})
		selected, err := r.chooseInteractive(ctx, "Pipelines", "Create or manage reusable transfers", options)
		if interactiveCancelled(err) || selected == interactiveBack {
			return nil
		}
		if err != nil {
			return err
		}
		if selected == "__create__" {
			if err := r.pipelineWizard(ctx, "", nil); err != nil && !interactiveCancelled(err) {
				if showErr := r.showInteractiveMessage(ctx, "Unable to create pipeline", err.Error()); showErr != nil && !interactiveCancelled(showErr) {
					return showErr
				}
			}
			continue
		}
		doc, err := r.service.Configuration(ctx)
		if err != nil {
			return err
		}
		if err := r.managePipeline(ctx, selected, doc); err != nil && !interactiveCancelled(err) {
			if showErr := r.showInteractiveMessage(ctx, "Unable to manage pipeline", err.Error()); showErr != nil && !interactiveCancelled(showErr) {
				return showErr
			}
		}
	}
}

func (r *Renderer) managePipeline(ctx context.Context, name string, doc model.Document) error {
	pipeline := doc.Pipelines[name]
	action, err := r.chooseInteractive(ctx, name, fmt.Sprintf("%s → %s", pipeline.Source.Ref, pipeline.Sink.Ref), []interactiveOption{
		{label: "Run", value: "run"},
		{label: "View", value: "view"},
		{label: "Edit", value: "edit"},
		{label: "Delete", value: "delete"},
		{label: "Back", value: interactiveBack},
	})
	if err != nil || action == interactiveBack {
		return err
	}
	switch action {
	case "run":
		return r.runInteractivePipeline(ctx, name)
	case "view":
		return r.showInteractiveMessage(ctx, name, pipelineDescription(pipeline))
	case "edit":
		return r.pipelineWizard(ctx, name, &pipeline)
	case "delete":
		confirmed, err := r.confirmInteractive(ctx, "Delete pipeline?", fmt.Sprintf("Delete %q permanently", name))
		if err != nil || !confirmed {
			return err
		}
		return r.service.DeleteSavedPipeline(ctx, name)
	default:
		return nil
	}
}

func (r *Renderer) pipelineWizard(ctx context.Context, name string, existing *model.Pipeline) error {
	doc, err := r.service.Configuration(ctx)
	if err != nil {
		return err
	}
	if len(doc.Sources) == 0 || len(doc.Sinks) == 0 {
		return fmt.Errorf("create at least one source and one sink before creating a pipeline")
	}
	name, sourceRef, sinkRef, err := r.pipelineTopologyForm(ctx, name, existing, doc)
	if err != nil {
		return err
	}
	if existing == nil {
		if _, exists := doc.Pipelines[name]; exists {
			return fmt.Errorf("pipeline %q already exists", name)
		}
	}

	source := doc.Sources[sourceRef]
	sink := doc.Sinks[sinkRef]
	var sourceInitial, sinkInitial map[string]any
	base := existing
	if existing != nil {
		updated := *existing
		if existing.Source.Ref == sourceRef {
			sourceInitial = existing.Source.Config
		} else {
			updated.Source.Config = nil
		}
		if existing.Sink.Ref == sinkRef {
			sinkInitial = existing.Sink.Config
		} else {
			updated.Sink.Config = nil
		}
		base = &updated
	}
	sourceWizard := newSchemaWizard(r.catalog.Sources[source.Type].Config, filament.ScopePipeline, sourceInitial)
	if err := sourceWizard.run(ctx, r, pipelineSteps.at(0)); err != nil {
		return err
	}
	sinkWizard := newSchemaWizard(r.catalog.Sinks[sink.Type].Config, filament.ScopePipeline, sinkInitial)
	if err := sinkWizard.run(ctx, r, pipelineSteps.at(1)); err != nil {
		return err
	}

	writeMode := "replace"
	if existing != nil && existing.WriteMode != "" {
		writeMode = existing.WriteMode
	}
	writeMode, err = r.choosePipelineWriteMode(ctx, sinkRef, sink, writeMode)
	if err != nil {
		return err
	}

	request := cliapp.SavePipelineRequest{
		Create: existing == nil, Name: name, Source: sourceRef, Sink: sinkRef,
		SyncMode: "full", WriteMode: writeMode,
		SourceConfig: sourceWizard.patch(), SinkConfig: sinkWizard.patch(),
	}
	preview, err := cliapp.BuildPipeline(request, base, doc, r.catalog)
	if err != nil {
		return err
	}
	resources, discoverErr := r.discoverPipelineResources(ctx, preview)
	selected, err := r.selectPipelineResources(ctx, existingResources(existing), resources, discoverErr)
	if err != nil {
		return err
	}
	request.Resources = &selected
	if _, err := r.service.SavePipeline(ctx, request); err != nil {
		return err
	}
	verb := "created"
	if existing != nil {
		verb = "updated"
	}
	return r.announce("pipeline", request.Name, verb)
}

func (r *Renderer) choosePipelineWriteMode(ctx context.Context, sinkRef string, sink model.Connection, preferred string) (string, error) {
	writeOptions := []interactiveOption{}
	for _, mode := range filament.WriteModesFor(filament.ModeFull) {
		if cliapp.SinkSupports(r.catalog.Sinks[sink.Type], mode) {
			writeOptions = append(writeOptions, interactiveOption{label: string(mode), value: string(mode)})
		}
	}
	if len(writeOptions) == 0 {
		return "", fmt.Errorf("sink %q has no full-sync write mode", sinkRef)
	}
	result, err := r.runInteractiveForm(ctx, inline.NewForm("Transfer behavior").Add(
		inline.NewSelectField("write-mode", "Write mode", preferredChoices(preferred, writeOptions)...).Required(),
	))
	if err != nil {
		return "", err
	}
	writeMode, _ := result["write-mode"].(string)
	return writeMode, nil
}

func (r *Renderer) pipelineTopologyForm(ctx context.Context, name string, existing *model.Pipeline, doc model.Document) (string, string, string, error) {
	sourceRef, sinkRef := "", ""
	if existing != nil {
		sourceRef, sinkRef = existing.Source.Ref, existing.Sink.Ref
	}
	sourceOptions := connectionOptions(doc.Sources)
	sinkOptions := connectionOptions(doc.Sinks)
	if sourceRef == "" {
		sourceRef = sourceOptions[0].value
	}
	if sinkRef == "" {
		sinkRef = sinkOptions[0].value
	}
	fields := []inline.FormField{}
	if existing == nil {
		fields = append(fields, inline.NewTextField("name", "Pipeline name").SetValue(name).Required().
			Validate(func(value string) error { return validateName("pipeline", value) }))
	}
	fields = append(fields,
		inline.NewSelectField("source", "Source", preferredChoices(sourceRef, sourceOptions)...).Required(),
		inline.NewSelectField("sink", "Sink", preferredChoices(sinkRef, sinkOptions)...).Required(),
	)
	result, err := r.runInteractiveForm(ctx, inline.NewForm("Pipeline configuration").Add(fields...))
	if err != nil {
		return "", "", "", err
	}
	if existing == nil {
		name, _ = result["name"].(string)
	}
	sourceRef, _ = result["source"].(string)
	sinkRef, _ = result["sink"].(string)
	return name, sourceRef, sinkRef, nil
}

func (r *Renderer) selectPipelineResources(ctx context.Context, initial []string, resources []model.ResourceSummary, discoverErr error) ([]string, error) {
	selectionMode := "all"
	if len(initial) > 0 {
		selectionMode = "select"
	}
	description := "Discover every available resource at run time"
	if discoverErr != nil {
		description = "Discovery unavailable; enter resource names manually"
	}
	modeResult, err := r.runInteractiveForm(ctx, inline.NewForm("Resource selection").Add(
		inline.NewSelectField("mode", description, preferredChoices(selectionMode, []interactiveOption{
			{label: "All discovered resources", value: "all"},
			{label: "Choose resources", value: "select"},
		})...).Required(),
	))
	if err != nil {
		return nil, err
	}
	if modeResult["mode"] == "all" {
		return nil, nil
	}
	if discoverErr == nil {
		field := discoveredResourceField(initial, resources)
		if len(field.Value().([]string)) == 0 && len(discoveredResourceChoices(resources)) == 0 {
			return nil, fmt.Errorf("source discovery returned no selectable resources")
		}
		result, err := r.runInteractiveForm(ctx, inline.NewForm("Choose discovered resources").Add(field))
		if err != nil {
			return nil, err
		}
		selected, _ := result["resources"].([]string)
		return selected, nil
	}

	resourceText := strings.Join(initial, ",")
	field := inline.NewTextField("resources", "Resources").SetValue(resourceText).
		SetPlaceholder("comma-separated resource names").Required().Validate(func(value string) error {
		selected := splitComma(value)
		if len(selected) == 0 {
			return fmt.Errorf("select at least one resource or choose all resources")
		}
		return nil
	})
	result, err := r.runInteractiveForm(ctx, inline.NewForm("Enter resource names").Add(field))
	if err != nil {
		return nil, err
	}
	value, _ := result["resources"].(string)
	return splitComma(value), nil
}

func discoveredResourceField(initial []string, resources []model.ResourceSummary) *inline.MultiSelectField {
	return inline.NewMultiSelectField("resources", "Resources", discoveredResourceChoices(resources)...).
		SetSelectedValues(initial...).
		SetIndicator(inline.IndicatorMinimal).
		MinSelected(1)
}

func discoveredResourceChoices(resources []model.ResourceSummary) []inline.Choice {
	choices := make([]inline.Choice, 0, len(resources))
	for _, resource := range resources {
		if !resource.Selectable {
			continue
		}
		label := resource.DisplayName
		if label == "" {
			label = resource.Name
		}
		choices = append(choices, inline.NewChoice(resource.Name, label))
	}
	return choices
}

func (r *Renderer) discoverPipelineResources(ctx context.Context, pipeline model.Pipeline) ([]model.ResourceSummary, error) {
	resources, err := r.service.DiscoverSource(ctx, cliapp.DiscoverSourceRequest{
		Source: pipeline.Source.Ref,
		Config: cliapp.ConfigPatch{Values: pipeline.Source.Config},
	})
	return resources.Items, err
}

func connectionOptions(connections map[string]model.Connection) []interactiveOption {
	options := make([]interactiveOption, 0, len(connections))
	for _, name := range sortedKeys(connections) {
		options = append(options, interactiveOption{label: fmt.Sprintf("%s  ·  %s", name, connections[name].Type), value: name})
	}
	return options
}

func existingResources(pipeline *model.Pipeline) []string {
	if pipeline == nil {
		return nil
	}
	return pipeline.Resources
}

func pipelineDescription(pipeline model.Pipeline) string {
	resources := "all discovered resources"
	if len(pipeline.Resources) > 0 {
		resources = strings.Join(pipeline.Resources, ", ")
	}
	return fmt.Sprintf("Source: %s\nSink: %s\nResources: %s\nSync mode: %s\nWrite mode: %s",
		pipeline.Source.Ref, pipeline.Sink.Ref, resources, pipeline.SyncMode, pipeline.WriteMode)
}

// pipelineMenuOptions lists pipelines as the boxed table every renderer
// shows.
func pipelineMenuOptions(items []model.PipelineSummary) []interactiveOption {
	if len(items) == 0 {
		return nil
	}
	rows := present.PipelineRows(items)
	return boxedMenu(present.Titles(present.PipelineColumns()), present.Cells(rows), present.Keys(rows))
}
