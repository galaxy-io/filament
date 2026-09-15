package dado

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/atterpac/dado/inline"
	"github.com/gdamore/tcell/v2"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

func TestInteractiveEntryRoutes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		args      []string
		section   string
		operation string
		routed    bool
	}{
		{routed: true},
		{args: []string{"source"}, section: "source", routed: true},
		{args: []string{"source", "list"}},
		{args: []string{"source", "create"}, section: "source", operation: "create", routed: true},
		{args: []string{"pipeline", "list"}},
		{args: []string{"config"}, section: "config", routed: true},
		{args: []string{"run"}, section: "run", operation: "run", routed: true},
		{args: []string{"source", "create", "production"}},
		{args: []string{"source", "discover"}},
		{args: []string{"source", "--help"}},
	}
	for _, test := range tests {
		entry, routed := interactiveEntryForArgs(test.args)
		if routed != test.routed || entry.section != test.section || entry.operation != test.operation {
			t.Errorf("interactiveEntryForArgs(%v) = %#v, %v", test.args, entry, routed)
		}
	}
}

func TestInteractiveFormStaysInline(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	inlineRenderer := inline.NewRenderer(inline.WithOutput(&output), inline.WithTerminalOutput(true))
	renderer := Renderer{
		stdin: strings.NewReader("\r"), stdout: &output, stderr: &output,
		interactiveRenderer: inlineRenderer,
	}
	form := inline.NewForm("Filament").Add(
		inline.NewSelectField("selection", "Choose", inline.NewChoice("run", "Run")).Required(),
	)
	result, err := renderer.runInteractiveForm(context.Background(), form)
	if err != nil {
		t.Fatal(err)
	}
	if result["selection"] != "run" {
		t.Fatalf("selection = %#v", result["selection"])
	}
	if err := inlineRenderer.Close(); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "\x1b[?1049") {
		t.Fatalf("Dado entered the alternate screen: %q", output.String())
	}
}

func TestInteractiveFormEnablesBoundaryNavigation(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	inlineRenderer := inline.NewRenderer(inline.WithOutput(&output), inline.WithTerminalOutput(false))
	renderer := Renderer{
		stdin:               iotest.OneByteReader(strings.NewReader("\x1b[Z")),
		stdout:              &output,
		stderr:              &output,
		interactiveRenderer: inlineRenderer,
	}
	form := inline.NewForm("Configure").Add(inline.NewTextField("name", "Name"))
	if _, err := renderer.runInteractiveForm(context.Background(), form); !interactivePrevious(err) {
		t.Fatalf("runInteractiveForm() error = %v, want previous form", err)
	}
	if err := inlineRenderer.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSchemaWizardNavigatesBackAcrossForms(t *testing.T) {
	t.Parallel()
	wizard := newSchemaWizard(filament.ConfigSchema{Fields: []filament.ConfigField{
		{Name: "first", Type: filament.FieldBool, Scope: filament.ScopePipeline},
		{Name: "second", Type: filament.FieldBool, Scope: filament.ScopePipeline},
	}}, filament.ScopePipeline, nil)

	// Submit the first field, go back from the second, change the first to true,
	// then advance through both fields.
	input := "\t\x1b[Z\x1b[B\t\t"
	var output bytes.Buffer
	inlineRenderer := inline.NewRenderer(inline.WithOutput(&output), inline.WithTerminalOutput(false))
	renderer := Renderer{
		stdin:               iotest.OneByteReader(strings.NewReader(input)),
		stdout:              &output,
		stderr:              &output,
		interactiveRenderer: inlineRenderer,
	}
	if err := wizard.run(context.Background(), &renderer, pipelineSteps.at(0)); err != nil {
		t.Fatal(err)
	}
	if err := inlineRenderer.Close(); err != nil {
		t.Fatal(err)
	}
	if got := wizard.patch().Values["first"]; got != true {
		t.Fatalf("first = %v, want true after backward navigation", got)
	}
}

func TestPreferredChoiceIsFirst(t *testing.T) {
	t.Parallel()
	choices := preferredChoices("postgres", []interactiveOption{
		{label: "Sample", value: "sample"},
		{label: "Postgres", value: "postgres"},
	})
	if len(choices) != 2 || choices[0].Value != "postgres" {
		t.Fatalf("choices = %#v", choices)
	}
}

func TestConnectionOptionsShowConnectorDisplayName(t *testing.T) {
	t.Parallel()
	renderer := Renderer{catalog: model.Catalog{Sources: map[string]filament.ConnectorSpec{
		"github": {DisplayName: "GitHub"},
	}}}
	options := renderer.connectionOptions("source", map[string]model.Connection{
		"production": {Type: "github"},
	})
	if len(options) != 1 || options[0].label != "production (GitHub)" {
		t.Fatalf("connectionOptions() = %#v, want production (GitHub)", options)
	}
}

func TestPipelineModeOptionsUseDisplayLabels(t *testing.T) {
	t.Parallel()
	modes := model.PipelineModes{
		ReadModes:  []string{"incremental", "cdc"},
		WriteModes: []string{"append", "upsert"},
	}
	readOptions := pipelineReadModeOptions(modes)
	if readOptions[0].label != "Incremental" || readOptions[0].value != "incremental" || readOptions[1].label != "CDC" {
		t.Fatalf("pipelineReadModeOptions() = %#v", readOptions)
	}
	writeOptions := pipelineWriteModeOptions(modes, "incremental")
	if writeOptions[0].label != "Append" || writeOptions[0].value != "append" {
		t.Fatalf("pipelineWriteModeOptions() = %#v", writeOptions)
	}
}

func TestDefaultPipelineResources(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		resources []model.ResourceSummary
		want      []string
		wantNil   bool
	}{
		{
			name: "all enabled uses all-resources representation",
			resources: []model.ResourceSummary{
				{Name: "users", Selectable: true, Enabled: true},
				{Name: "orders", Selectable: true, Enabled: true},
			},
			wantNil: true,
		},
		{
			name: "disabled resources are omitted",
			resources: []model.ResourceSummary{
				{Name: "users", Selectable: true, Enabled: true},
				{Name: "audit", Selectable: true},
				{Name: "internal", Enabled: true},
			},
			want: []string{"users"},
		},
		{
			name: "none enabled remains an explicit selection",
			resources: []model.ResourceSummary{
				{Name: "users", Selectable: true},
			},
			want: []string{},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := defaultPipelineResources(test.resources)
			if (got == nil) != test.wantNil {
				t.Fatalf("defaultPipelineResources() = %#v, want nil=%v", got, test.wantNil)
			}
			if strings.Join(got, ",") != strings.Join(test.want, ",") {
				t.Fatalf("defaultPipelineResources() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestLongDiscoveredResourceSelectionUsesBoundedViewport(t *testing.T) {
	t.Parallel()
	resources := make([]model.ResourceSummary, resourcePickerVisibleChoices*3)
	initial := make([]string, 0, len(resources))
	for index := range resources {
		name := fmt.Sprintf("resource_%02d", index)
		resources[index] = model.ResourceSummary{Name: name, Selectable: true}
		initial = append(initial, name)
	}

	field := discoveredResourceField(initial, resources)
	form := discoveredResourceForm(field, len(resources), filamentTheme(false))
	text := frameText(form.Frame(80))
	if !strings.Contains(text, fmt.Sprintf("Resources (1–%d of %d)", resourcePickerVisibleChoices, len(resources))) {
		t.Fatalf("resource overflow hint missing:\n%s", text)
	}
	if !strings.Contains(text, fmt.Sprintf("1–%d of %d", resourcePickerVisibleChoices, len(resources))) {
		t.Fatalf("resource viewport missing range:\n%s", text)
	}
	if strings.Contains(text, "resource_10") {
		t.Fatalf("resource outside initial viewport rendered:\n%s", text)
	}

	for range 17 {
		form.HandleKey(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	}
	text = frameText(form.Frame(80))
	if !strings.Contains(text, fmt.Sprintf("Resources (9–%d of %d)", resourcePickerVisibleChoices+8, len(resources))) {
		t.Fatalf("resource label did not follow active viewport:\n%s", text)
	}
}

func TestResourceSelectionNavigatesBackToMode(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	inlineRenderer := inline.NewRenderer(inline.WithOutput(&output), inline.WithTerminalOutput(false))
	renderer := Renderer{
		// Choose resources, return to the mode form, move to all resources,
		// and advance with Tab.
		stdin:               iotest.OneByteReader(strings.NewReader("\t\x1b[Z\x1b[B\t")),
		stdout:              &output,
		stderr:              &output,
		interactiveRenderer: inlineRenderer,
	}
	selected, err := renderer.selectPipelineResources(context.Background(), []string{"users"}, []model.ResourceSummary{
		{Name: "users", Selectable: true, Enabled: true},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if selected != nil {
		t.Fatalf("selected = %v, want all resources", selected)
	}
	if err := inlineRenderer.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSchemaWizardProducesTypedPatch(t *testing.T) {
	t.Parallel()
	wizard := newSchemaWizard(filament.ConfigSchema{Fields: []filament.ConfigField{
		{Name: "rows", Type: filament.FieldInt, Scope: filament.ScopePipeline},
		{Name: "label", Type: filament.FieldString, Scope: filament.ScopePipeline},
		{Name: "enabled", Type: filament.FieldBool, Scope: filament.ScopePipeline},
	}}, filament.ScopePipeline, map[string]any{
		"rows": 2, "label": "old", "enabled": true,
	})
	wizard.fields[0].text = "7"
	wizard.fields[1].text = ""
	wizard.fields[2].boolean = false

	patch := wizard.patch()
	if patch.Values["rows"] != 7 || patch.Values["enabled"] != false {
		t.Fatalf("values = %#v", patch.Values)
	}
	if len(patch.Unset) != 1 || patch.Unset[0] != "label" {
		t.Fatalf("unset = %#v", patch.Unset)
	}
}

func TestRunUpdatesTrackRecords(t *testing.T) {
	t.Parallel()
	row := &runRow{name: "users"}
	applyRunUpdate(row, resourceProgressUpdate{resource: "users", status: "running", records: 4, bytes: 128})
	if row.state != rowRunning || row.records != 4 {
		t.Fatalf("row = %#v", row)
	}
	applyRunUpdate(row, resourceProgressUpdate{resource: "users", status: "complete", records: 10, bytes: 320, final: true})
	if row.state != rowDone || row.records != 10 || row.bytes != 320 {
		t.Fatalf("row = %#v", row)
	}
	applyRunUpdate(row, resourceProgressUpdate{resource: "users", status: "failed", err: "boom"})
	if row.state != rowFailed || row.err != "boom" {
		t.Fatalf("row = %#v", row)
	}
}

func TestInterruptAndQuitCloseApplication(t *testing.T) {
	t.Parallel()
	for _, input := range []string{"\x03", "q"} {
		var output bytes.Buffer
		renderer := Renderer{stdin: strings.NewReader(input), stdout: &output, stderr: &output}
		if err := renderer.runInteractiveAt(context.Background(), interactiveEntry{}); err != nil {
			t.Errorf("input %q: %v", input, err)
		}
	}
}
