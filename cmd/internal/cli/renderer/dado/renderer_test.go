package dado

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/atterpac/dado/inline"

	"github.com/galaxy-io/filament"
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
		{args: []string{"run"}, section: "run", routed: true},
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
