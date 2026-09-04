package dado

import (
	"strings"
	"testing"
	"time"

	"github.com/atterpac/dado/inline"

	"github.com/galaxy-io/filament/cmd/internal/cli/model"
	"github.com/galaxy-io/filament/cmd/internal/cli/style"
)

func frameText(frame *inline.Frame) string {
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
	return strings.Join(lines, "\n")
}

func TestWizardConfigurationStepRendersTranscriptAboveField(t *testing.T) {
	t.Parallel()
	theme := filamentTheme(true)
	header, err := newWizardHeader(connectionSteps.at(1), []headerLine{
		{kind: headerPrompt, key: "DSN", value: "$POSTGRES_DSN"},
		{kind: headerHelper, value: "How changes are read from the source"},
	}, theme)
	if err != nil {
		t.Fatal(err)
	}
	form := inline.NewForm("").SetHeader(header).SetTheme(theme).Add(
		inline.NewSelectField("replication", "Replication",
			inline.Choice{Value: "standard", Label: "standard", Description: "Standard"},
			inline.Choice{Value: "cdc", Label: "cdc", Description: "Change Data Capture (CDC)"},
		).Required(),
	)
	text := frameText(form.Frame(70))
	t.Logf("\n%s", text)
	for _, expected := range []string{"✓ Connector", "● Configuration", "○ Review", "Step 2 of 3", "? DSN › $POSTGRES_DSN", "· How changes are read", "Replication", "standard", "cdc"} {
		if !strings.Contains(text, expected) {
			t.Errorf("missing %q", expected)
		}
	}
}

func TestWizardFirstStepRendersNameAndConnector(t *testing.T) {
	t.Parallel()
	theme := filamentTheme(true)
	header, err := newWizardHeader(connectionSteps.at(0), nil, theme)
	if err != nil {
		t.Fatal(err)
	}
	form := inline.NewForm("").SetHeader(header).SetTheme(theme).Add(
		inline.NewTextField("name", "Name").SetValue("prod-postgres").Required(),
		inline.NewSelectField("connector", "Connector",
			inline.Choice{Value: "postgres", Label: "postgres", Description: "Popular open-source relational database"},
			inline.Choice{Value: "mysql", Label: "mysql", Description: "Widely-used open-source relational database"},
		).Required(),
	)
	t.Logf("\n%s", frameText(form.Frame(70)))
}

func TestWizardReviewStepPadsKeys(t *testing.T) {
	t.Parallel()
	theme := filamentTheme(true)
	header, err := newWizardHeader(connectionSteps.at(2), []headerLine{
		{kind: headerReview, key: "Name", value: "prod-postgres"},
		{kind: headerReview, key: "Connector", value: "postgres"},
		{kind: headerReview, key: "Replication", value: "cdc"},
		{kind: headerBlank},
	}, theme)
	if err != nil {
		t.Fatal(err)
	}
	form := inline.NewForm("").SetHeader(header).SetTheme(theme).Add(
		inline.NewSelectField("confirmed", "Create source", inline.NewChoice("true", "Yes"), inline.NewChoice("false", "No")).Required(),
	)
	text := frameText(form.Frame(70))
	t.Logf("\n%s", text)
	if !strings.Contains(text, "Name            prod-postgres") || !strings.Contains(text, "Replication     cdc") {
		t.Errorf("keys not padded:\n%s", text)
	}
}

func TestRunProgressViewRendersGrid(t *testing.T) {
	t.Parallel()
	view := &runProgressView{theme: filamentTheme(true), rows: []*runRow{
		{name: "customers", records: 1_800_000, state: rowDone},
		{name: "orders", records: 600_000, state: rowRunning},
		{name: "audit_log", state: rowPending},
	}}
	text := frameText(view.Frame(60))
	t.Logf("\n%s", text)
	for _, expected := range []string{"  Resource      Rows     Status", "  customers     1.8M     ✓ Done", "  orders        600K     ⠋ Running", "  audit_log        –     ○ Waiting"} {
		if !strings.Contains(text, expected) {
			t.Errorf("missing %q in\n%s", expected, text)
		}
	}
	var p style.Painter
	lines := view.lines(p)
	t.Logf("\n%s", lines)
	if !strings.Contains(lines, "  customers     1.8M     ✓ Done") {
		t.Errorf("scrollback lines =\n%s", lines)
	}
	summary := runSummaryLine(p, 2, model.RunResult{Records: 2_400_000}, 39*time.Second, nil)
	if summary != "✓ Synced 2 resources · 2.4M rows · 39s" {
		t.Errorf("summary = %q", summary)
	}
}
