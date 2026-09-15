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

const resourcePickerVisibleChoices = 10

type pipelineConfigurationStage int

const (
	pipelineSourceConfiguration pipelineConfigurationStage = iota
	pipelineSinkConfiguration
	pipelineReadMode
	pipelineWriteMode
	pipelineResourceSelection
)

type pipelineConfigurationState struct {
	request      cliapp.SavePipelineRequest
	base         *model.Pipeline
	existing     *model.Pipeline
	doc          model.Document
	sourceWizard *schemaWizard
	sinkWizard   *schemaWizard
	modes        model.PipelineModes
	stage        pipelineConfigurationStage
	history      []pipelineConfigurationStage
	fromEnd      bool
}

type pipelineStageResult struct {
	next        pipelineConfigurationStage
	interactive bool
	done        bool
	err         error
}

func (r *Renderer) managePipelines(ctx context.Context) error {
	for {
		done, err := r.managePipelinesOnce(ctx)
		if done || err != nil {
			return err
		}
	}
}

// managePipelinesOnce runs one pass of the pipelines menu; done reports that
// the user backed out.
func (r *Renderer) managePipelinesOnce(ctx context.Context) (bool, error) {
	listed, err := r.service.Pipelines(ctx)
	if err != nil {
		return true, err
	}
	description := "No pipelines yet."
	var options []interactiveOption
	if len(listed.Items) > 0 {
		description = ""
		options = append(options, r.pipelineMenuOptions(listed.Items)...)
		options = append(options, spacer)
	}
	options = append(options, interactiveOption{label: "Create a pipeline", value: "__create__", tone: inline.ChoiceToneSuccess})
	selected, err := r.chooseInteractive(ctx, "Pipelines", description, options)
	if interactiveCancelled(err) {
		return true, nil
	}
	if err != nil {
		return true, err
	}
	if selected == "__create__" {
		if err := r.pipelineWizard(ctx, "", nil); err != nil && !interactiveCancelled(err) {
			_ = r.notice(false, "Unable to create pipeline: "+err.Error())
		}
		return false, nil
	}
	doc, err := r.service.Configuration(ctx)
	if err != nil {
		return true, err
	}
	if err := r.managePipeline(ctx, selected, doc); err != nil && !interactiveCancelled(err) {
		_ = r.notice(false, "Unable to manage pipeline: "+err.Error())
	}
	return false, nil
}

func (r *Renderer) managePipeline(ctx context.Context, name string, doc model.Document) error {
	pipeline := doc.Pipelines[name]
	action, err := r.chooseInteractive(ctx, name, fmt.Sprintf("%s → %s", pipeline.Source.Ref, pipeline.Sink.Ref), []interactiveOption{
		{label: "Run", value: "run"},
		{label: "View", value: "view"},
		{label: "Edit", value: "edit"},
		{label: "Delete", value: "delete"},
	})
	if err != nil {
		return err
	}
	switch action {
	case "run":
		return r.runInteractivePipeline(ctx, name)
	case "view":
		return r.showDetail(ctx, name, pipelinePairs(pipeline))
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
	if existing != nil && existing.Info.EditBlockedReason != "" {
		return fmt.Errorf("pipeline %q cannot be edited by the CLI without losing data: %s; edit it in the UI", name, existing.Info.EditBlockedReason)
	}
	doc, err := r.service.Configuration(ctx)
	if err != nil {
		return err
	}
	if len(doc.Sources) == 0 || len(doc.Sinks) == 0 {
		return fmt.Errorf("create at least one source and one sink before creating a pipeline")
	}
	var draft *cliapp.SavePipelineRequest
	for {
		name, sourceRef, sinkRef, err := r.pipelineTopologyForm(ctx, name, existing, doc, draft)
		if interactivePrevious(err) {
			return inline.ErrFormCancelled
		}
		if err != nil {
			return err
		}
		if existing == nil {
			if _, exists := doc.Pipelines[name]; exists {
				return fmt.Errorf("pipeline %q already exists", name)
			}
		}

		request, err := r.configurePipeline(ctx, name, sourceRef, sinkRef, existing, doc, draft)
		if interactivePrevious(err) {
			draft = &request
			continue
		}
		if err != nil {
			return err
		}
		if _, err := r.service.SavePipeline(ctx, request); err != nil {
			return err
		}
		verb := "created"
		if existing != nil {
			verb = "updated"
		}
		return r.announce("pipeline", request.Name, verb)
	}
}

func (r *Renderer) configurePipeline(
	ctx context.Context,
	name, sourceRef, sinkRef string,
	existing *model.Pipeline,
	doc model.Document,
	draft *cliapp.SavePipelineRequest,
) (cliapp.SavePipelineRequest, error) {
	state := r.newPipelineConfigurationState(name, sourceRef, sinkRef, existing, doc, draft)
	for {
		result := r.runPipelineConfigurationStage(ctx, state)
		if interactivePrevious(result.err) {
			if !state.previousStage() {
				return state.request, result.err
			}
			continue
		}
		if result.err != nil {
			return state.request, result.err
		}
		if result.done {
			return state.request, nil
		}
		state.advance(result.next, result.interactive)
	}
}

func (r *Renderer) newPipelineConfigurationState(
	name, sourceRef, sinkRef string,
	existing *model.Pipeline,
	doc model.Document,
	draft *cliapp.SavePipelineRequest,
) *pipelineConfigurationState {
	source, sink := doc.Sources[sourceRef], doc.Sinks[sinkRef]
	var sourceInitial, sinkInitial map[string]any
	base := existing
	if existing != nil {
		updated := *existing
		if existing.Source.Ref == sourceRef {
			sourceInitial = cliapp.ConfigWithSecretPlaceholders(existing.Source.Config, existing.Source.SecretRefs)
		} else {
			updated.Source.Config = nil
		}
		if existing.Sink.Ref == sinkRef {
			sinkInitial = cliapp.ConfigWithSecretPlaceholders(existing.Sink.Config, existing.Sink.SecretRefs)
		} else {
			updated.Sink.Config = nil
		}
		base = &updated
	}
	if draft != nil && draft.Source == sourceRef {
		sourceInitial = draft.SourceConfig.Values
	}
	if draft != nil && draft.Sink == sinkRef {
		sinkInitial = draft.SinkConfig.Values
	}
	sourceWizard := newSchemaWizard(r.catalog.Sources[source.Type].Config, filament.ScopePipeline, sourceInitial)
	sinkWizard := newSchemaWizard(r.catalog.Sinks[sink.Type].Config, filament.ScopePipeline, sinkInitial)

	syncMode := "full"
	if existing != nil && existing.SyncMode != "" {
		syncMode = existing.SyncMode
	}
	writeMode := "replace"
	if existing != nil && existing.WriteMode != "" {
		writeMode = existing.WriteMode
	}
	if draft != nil && draft.Source == sourceRef && draft.Sink == sinkRef {
		if draft.SyncMode != "" {
			syncMode = draft.SyncMode
		}
		if draft.WriteMode != "" {
			writeMode = draft.WriteMode
		}
	}
	request := cliapp.SavePipelineRequest{
		Create: existing == nil, Name: name, Source: sourceRef, Sink: sinkRef,
		SyncMode: syncMode, WriteMode: writeMode,
	}
	if draft != nil && draft.Source == sourceRef && draft.Sink == sinkRef && draft.Resources != nil {
		resources := append([]string(nil), (*draft.Resources)...)
		request.Resources = &resources
	}
	return &pipelineConfigurationState{
		request:      request,
		base:         base,
		existing:     existing,
		doc:          doc,
		sourceWizard: sourceWizard,
		sinkWizard:   sinkWizard,
		stage:        pipelineSourceConfiguration,
	}
}

func (s *pipelineConfigurationState) previousStage() bool {
	if len(s.history) == 0 {
		return false
	}
	s.stage = s.history[len(s.history)-1]
	s.history = s.history[:len(s.history)-1]
	s.fromEnd = true
	return true
}

func (s *pipelineConfigurationState) advance(next pipelineConfigurationStage, interactive bool) {
	if interactive {
		s.history = append(s.history, s.stage)
	}
	s.stage = next
	s.fromEnd = false
}

func (r *Renderer) runPipelineConfigurationStage(ctx context.Context, state *pipelineConfigurationState) pipelineStageResult {
	switch state.stage {
	case pipelineSourceConfiguration:
		return r.configurePipelineSourceStage(ctx, state)
	case pipelineSinkConfiguration:
		return r.configurePipelineSinkStage(ctx, state)
	case pipelineReadMode:
		return r.configurePipelineReadModeStage(ctx, state)
	case pipelineWriteMode:
		return r.configurePipelineWriteModeStage(ctx, state)
	case pipelineResourceSelection:
		return r.configurePipelineResourcesStage(ctx, state)
	default:
		return pipelineStageResult{err: fmt.Errorf("unknown pipeline configuration stage %d", state.stage)}
	}
}

func (r *Renderer) configurePipelineSourceStage(ctx context.Context, state *pipelineConfigurationState) pipelineStageResult {
	interactive := len(state.sourceWizard.steps()) > 0
	err := state.sourceWizard.runFrom(ctx, r, pipelineSteps.at(0), state.fromEnd)
	state.request.SourceConfig = state.sourceWizard.patch()
	return pipelineStageResult{next: pipelineSinkConfiguration, interactive: interactive, err: err}
}

func (r *Renderer) configurePipelineSinkStage(ctx context.Context, state *pipelineConfigurationState) pipelineStageResult {
	interactive := len(state.sinkWizard.steps()) > 0
	err := state.sinkWizard.runFrom(ctx, r, pipelineSteps.at(1), state.fromEnd)
	state.request.SinkConfig = state.sinkWizard.patch()
	return pipelineStageResult{next: pipelineReadMode, interactive: interactive, err: err}
}

func (r *Renderer) configurePipelineReadModeStage(ctx context.Context, state *pipelineConfigurationState) pipelineStageResult {
	preview, err := cliapp.BuildPipeline(state.request, state.base, state.doc, r.catalog)
	if err != nil {
		return pipelineStageResult{err: err}
	}
	state.modes, err = r.service.PipelineModes(ctx, preview)
	if err != nil {
		return pipelineStageResult{err: err}
	}
	interactive := len(pipelineReadModeOptions(state.modes)) > 1
	state.request.SyncMode, err = r.choosePipelineReadMode(
		ctx, state.request.Source, state.modes, state.request.SyncMode,
	)
	return pipelineStageResult{next: pipelineWriteMode, interactive: interactive, err: err}
}

func (r *Renderer) configurePipelineWriteModeStage(ctx context.Context, state *pipelineConfigurationState) pipelineStageResult {
	interactive := len(pipelineWriteModeOptions(state.modes, state.request.SyncMode)) > 1
	writeMode, err := r.choosePipelineWriteMode(
		ctx, state.request.Sink, state.modes, state.request.SyncMode, state.request.WriteMode,
	)
	state.request.WriteMode = writeMode
	return pipelineStageResult{next: pipelineResourceSelection, interactive: interactive, err: err}
}

func (r *Renderer) configurePipelineResourcesStage(ctx context.Context, state *pipelineConfigurationState) pipelineStageResult {
	preview, err := cliapp.BuildPipeline(state.request, state.base, state.doc, r.catalog)
	if err != nil {
		return pipelineStageResult{err: err}
	}
	resources, discoverErr := r.discoverPipelineResources(ctx, preview)
	initial := existingResources(state.existing)
	if state.request.Resources != nil {
		initial = append([]string(nil), (*state.request.Resources)...)
	} else if state.existing == nil && discoverErr == nil {
		initial = defaultPipelineResources(resources)
	}
	selected, err := r.selectPipelineResources(ctx, initial, resources, discoverErr)
	if err != nil {
		return pipelineStageResult{err: err}
	}
	state.request.Resources = &selected
	return pipelineStageResult{done: true}
}

func (r *Renderer) choosePipelineReadMode(ctx context.Context, sourceRef string, modes model.PipelineModes, preferred string) (string, error) {
	options := pipelineReadModeOptions(modes)
	if len(options) == 0 {
		return "", fmt.Errorf("source %q has no supported read mode", sourceRef)
	}
	if len(options) == 1 {
		return options[0].value, nil
	}
	result, err := r.runInteractiveForm(ctx, inline.NewForm("Transfer behavior").Add(
		inline.NewSelectField("sync-mode", "Read mode", preferredChoices(preferred, options)...).Required(),
	))
	if err != nil {
		return "", err
	}
	mode, _ := result["sync-mode"].(string)
	return mode, nil
}

func pipelineReadModeOptions(modes model.PipelineModes) []interactiveOption {
	options := make([]interactiveOption, 0, len(modes.ReadModes))
	for _, mode := range modes.ReadModes {
		options = append(options, interactiveOption{label: pipelineModeLabel(mode), value: mode})
	}
	return options
}

func (r *Renderer) choosePipelineWriteMode(ctx context.Context, sinkRef string, modes model.PipelineModes, syncMode, preferred string) (string, error) {
	writeOptions := pipelineWriteModeOptions(modes, syncMode)
	if len(writeOptions) == 0 {
		return "", fmt.Errorf("sink %q has no write mode compatible with %s reads", sinkRef, syncMode)
	}
	if len(writeOptions) == 1 {
		return writeOptions[0].value, nil
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

func pipelineWriteModeOptions(modes model.PipelineModes, syncMode string) []interactiveOption {
	writeOptions := []interactiveOption{}
	for _, mode := range filament.WriteModesFor(readMode(syncMode)) {
		for _, supported := range modes.WriteModes {
			if string(mode) == supported {
				writeOptions = append(writeOptions, interactiveOption{label: pipelineModeLabel(supported), value: supported})
				break
			}
		}
	}
	return writeOptions
}

func pipelineModeLabel(mode string) string {
	if strings.EqualFold(mode, "cdc") {
		return "CDC"
	}
	return present.TitleCase(mode)
}

func readMode(value string) filament.ReadMode {
	if value == "incremental" {
		return filament.ModeIncremental
	}
	if value == model.SyncModeCDC {
		return filament.ModeCDC
	}
	return filament.ModeFull
}

func (r *Renderer) pipelineTopologyForm(
	ctx context.Context,
	name string,
	existing *model.Pipeline,
	doc model.Document,
	draft *cliapp.SavePipelineRequest,
) (string, string, string, error) {
	sourceRef, sinkRef := "", ""
	if existing != nil {
		sourceRef, sinkRef = existing.Source.Ref, existing.Sink.Ref
	}
	if draft != nil {
		sourceRef, sinkRef = draft.Source, draft.Sink
	}
	sourceOptions := r.connectionOptions("source", doc.Sources)
	sinkOptions := r.connectionOptions("sink", doc.Sinks)
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
	if initial != nil {
		selectionMode = "select"
	}
	description := "Discover every available resource at run time"
	if discoverErr != nil {
		description = "Discovery unavailable; enter resource names manually"
	}
	for {
		modeResult, err := r.runInteractiveForm(ctx, inline.NewForm("Resource selection").Add(
			inline.NewSelectField("mode", description, preferredChoices(selectionMode, []interactiveOption{
				{label: "All discovered resources", value: "all"},
				{label: "Choose resources", value: "select"},
			})...).Required(),
		))
		if err != nil {
			return nil, err
		}
		selectionMode, _ = modeResult["mode"].(string)
		if selectionMode == "all" {
			return nil, nil
		}

		if discoverErr == nil {
			field := discoveredResourceField(initial, resources)
			if len(discoveredResourceChoices(resources)) == 0 {
				return nil, fmt.Errorf("source discovery returned no selectable resources")
			}
			form := discoveredResourceForm(field, len(discoveredResourceChoices(resources)), r.theme)
			result, err := r.runInteractiveForm(ctx, form)
			if interactivePrevious(err) {
				initial, _ = field.Value().([]string)
				continue
			}
			if err != nil {
				return nil, err
			}
			selected, _ := result["resources"].([]string)
			return selected, nil
		}

		field := inline.NewTextField("resources", "Resources").SetValue(strings.Join(initial, ",")).
			SetPlaceholder("comma-separated resource names").Required().Validate(func(value string) error {
			selected := splitComma(value)
			if len(selected) == 0 {
				return fmt.Errorf("select at least one resource or choose all resources")
			}
			return nil
		})
		result, err := r.runInteractiveForm(ctx, inline.NewForm("Enter resource names").Add(field))
		if interactivePrevious(err) {
			value, _ := field.Value().(string)
			initial = splitComma(value)
			continue
		}
		if err != nil {
			return nil, err
		}
		value, _ := result["resources"].(string)
		return splitComma(value), nil
	}
}

// defaultPipelineResources mirrors the UI's initial resource selection. A nil
// result retains the compact "all resources" representation; an explicitly
// empty result keeps the chooser open so the user must enable a resource.
func defaultPipelineResources(resources []model.ResourceSummary) []string {
	selected := make([]string, 0, len(resources))
	selectable := 0
	for _, resource := range resources {
		if !resource.Selectable {
			continue
		}
		selectable++
		if resource.Enabled {
			selected = append(selected, resource.Name)
		}
	}
	if len(selected) == selectable {
		return nil
	}
	return selected
}

func discoveredResourceField(initial []string, resources []model.ResourceSummary) *inline.MultiSelectField {
	choices := discoveredResourceChoices(resources)
	label := "Resources"
	if len(choices) > resourcePickerVisibleChoices {
		label = ""
	}
	return inline.NewMultiSelectField("resources", label, choices...).
		SetSelectedValues(initial...).
		SetIndicatorGlyphs("✔", "○", "–").
		MaxVisibleChoices(resourcePickerVisibleChoices).
		MinSelected(1)
}

func discoveredResourceForm(field *inline.MultiSelectField, total int, theme inline.InlineTheme) *inline.Form {
	const title = "Choose discovered resources"
	if total <= resourcePickerVisibleChoices {
		return inline.NewForm(title).Add(field)
	}

	form := inline.NewForm("").SetTheme(theme).Add(field)
	header := &resourcePickerHeader{form: form, title: title, total: total, theme: theme}
	return form.SetHeader(header).SetHeaderGap(0)
}

// resourcePickerHeader reads the viewport text Dado has already rendered so
// the Filament-owned label always describes the same active range.
type resourcePickerHeader struct {
	form  *inline.Form
	title string
	total int
	theme inline.InlineTheme
}

func (h *resourcePickerHeader) Frame(width int) *inline.Frame {
	if h.form.Done() {
		return inline.NewFrame(width, 0)
	}

	h.form.SetHeader(nil)
	body := h.form.Frame(width)
	h.form.SetHeader(h)

	rangeText := resourcePickerRange(body, h.total)
	frame := inline.NewFrame(width, 3)
	frame.DrawString(0, 0, h.title, h.theme.Accent.Bold(true))
	frame.DrawString(0, 2, fmt.Sprintf("Resources (%s)", rangeText), h.theme.Label.Bold(true))
	return frame
}

func resourcePickerRange(frame *inline.Frame, total int) string {
	totalText := fmt.Sprint(total)
	for y := frame.Height() - 1; y >= 0; y-- {
		fields := strings.Fields(frameRowText(frame, y))
		if len(fields) == 3 && fields[1] == "of" && fields[2] == totalText && strings.Contains(fields[0], "–") {
			return strings.Join(fields, " ")
		}
	}
	return fmt.Sprintf("1–%d of %d", min(resourcePickerVisibleChoices, total), total)
}

func frameRowText(frame *inline.Frame, y int) string {
	var line strings.Builder
	for x := 0; x < frame.Width(); {
		value, _, width := frame.Cell(x, y)
		if width < 1 {
			width = 1
		}
		line.WriteString(value)
		x += width
	}
	return strings.TrimSpace(line.String())
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

func (r *Renderer) connectionOptions(kind string, connections map[string]model.Connection) []interactiveOption {
	options := make([]interactiveOption, 0, len(connections))
	for _, name := range sortedKeys(connections) {
		options = append(options, interactiveOption{
			label: fmt.Sprintf("%s (%s)", name, r.connectorDisplayName(kind, connections[name].Type)),
			value: name,
		})
	}
	return options
}

func existingResources(pipeline *model.Pipeline) []string {
	if pipeline == nil {
		return nil
	}
	return pipeline.Resources
}

func pipelinePairs(pipeline model.Pipeline) [][2]string {
	resources := "all discovered resources"
	if len(pipeline.Resources) > 0 {
		resources = strings.Join(pipeline.Resources, ", ")
	}
	return [][2]string{
		{"Source", pipeline.Source.Ref},
		{"Sink", pipeline.Sink.Ref},
		{"Resources", resources},
		{"Sync mode", pipeline.SyncMode},
		{"Write mode", pipeline.WriteMode},
	}
}

// pipelineMenuOptions lists pipelines as the boxed table every renderer
// shows.
func (r *Renderer) pipelineMenuOptions(items []model.PipelineSummary) []interactiveOption {
	if len(items) == 0 {
		return nil
	}
	rows := present.PipelineRows(items)
	return r.tableMenu(present.Titles(present.PipelineColumns()), present.Cells(rows), present.Keys(rows))
}
