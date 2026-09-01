package dado

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/atterpac/dado/inline"

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
		{args: []string{"source", "list"}, section: "source", operation: "list", routed: true},
		{args: []string{"source", "create"}, section: "source", operation: "create", routed: true},
		{args: []string{"pipeline", "list"}, section: "pipeline", operation: "list", routed: true},
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

func TestResourceProgressTracksRecords(t *testing.T) {
	t.Parallel()
	progress := inline.NewMultiProgress("Run")
	states := map[string]*resourceProgressState{}
	add := func(name string, estimated int64) error {
		if _, exists := states[name]; !exists {
			states[name] = &resourceProgressState{estimated: estimated}
			return progress.Add(name, name, estimated)
		}
		return nil
	}
	if err := add("users", 10); err != nil {
		t.Fatal(err)
	}
	if err := applyResourceProgress(progress, states, add, resourceProgressUpdate{
		resource: "users", status: "running", records: 4, bytes: 128,
	}); err != nil {
		t.Fatal(err)
	}
	if err := applyResourceProgress(progress, states, add, resourceProgressUpdate{
		resource: "users", status: "complete", records: 10, bytes: 320, final: true,
	}); err != nil {
		t.Fatal(err)
	}
	tasks := progress.Tasks()
	if len(tasks) != 1 || tasks[0].State != inline.TaskComplete || tasks[0].Current != 10 {
		t.Fatalf("tasks = %#v", tasks)
	}
	if !strings.Contains(tasks[0].Detail, "10 records") {
		t.Fatalf("detail = %q", tasks[0].Detail)
	}
}

func TestRunSummaryShowsPerformance(t *testing.T) {
	t.Parallel()
	frame := (runSummaryView{
		pipeline: "daily-sync", source: "postgres", sink: "stdout", resources: 3,
		result:  model.RunResult{Records: 350025, Bytes: 8 * 1024 * 1024},
		elapsed: 2 * time.Second,
	}).Frame(80)
	lines := make([]string, frame.Height())
	for y := 0; y < frame.Height(); y++ {
		var line strings.Builder
		for x := 0; x < frame.Width(); {
			value, _, width := frame.Cell(x, y)
			if width < 1 {
				width = 1
			}
			line.WriteString(value)
			x += width
		}
		lines[y] = strings.TrimRight(line.String(), " ")
	}
	text := strings.Join(lines, "\n")
	for _, expected := range []string{"Run completed", "daily-sync", "postgres → stdout", "350,025", "8.0 MiB"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("summary missing %q:\n%s", expected, text)
		}
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
