package dado

import (
	"context"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/atterpac/dado/inline"
	"github.com/gdamore/tcell/v2"

	"github.com/galaxy-io/filament/cmd/internal/cli/style"
)

// wizardSteps names the stages in the stepper and which one is active.
type wizardSteps struct {
	labels []string
	active int
}

func (s wizardSteps) at(index int) wizardSteps {
	return wizardSteps{labels: s.labels, active: index}
}

var (
	connectionSteps = wizardSteps{labels: []string{"Connector", "Configuration", "Review"}}
	pipelineSteps   = wizardSteps{labels: []string{"Source", "Sink"}}
)

type headerLineKind int

const (
	headerPrompt headerLineKind = iota
	headerHelper
	headerReview
	headerBlank
)

// headerLine is one answered prompt, helper, or review row shown above the live field.
type headerLine struct {
	kind  headerLineKind
	key   string
	value string
}

// wizardHeader stacks the stepper, a rule, and the transcript above a form.
type wizardHeader struct {
	stepper *inline.Stepper
	lines   []headerLine
	theme   inline.InlineTheme
}

func newWizardHeader(steps wizardSteps, lines []headerLine, theme inline.InlineTheme) (*wizardHeader, error) {
	stepper := inline.NewStepper("",
		inline.WithStepOrientation(inline.StepHorizontal),
		inline.WithStepDetails(false),
		inline.WithStepperTheme(theme.Status),
	)
	for index, label := range steps.labels {
		if err := stepper.Add(strconv.Itoa(index), label); err != nil {
			return nil, err
		}
	}
	for index := range steps.labels {
		id := strconv.Itoa(index)
		switch {
		case index < steps.active:
			if err := stepper.Complete(id); err != nil {
				return nil, err
			}
		case index == steps.active:
			if err := stepper.Activate(id); err != nil {
				return nil, err
			}
		}
	}
	return &wizardHeader{stepper: stepper, lines: lines, theme: theme}, nil
}

func (h *wizardHeader) Frame(width int) *inline.Frame {
	rule := inline.NewFrame(width, 1)
	rule.DrawString(0, 0, strings.Repeat("─", width), h.theme.Muted)
	frames := []*inline.Frame{h.stepper.Frame(width), rule}
	if len(h.lines) > 0 {
		frames = append(frames, h.linesFrame(width))
	}
	return inline.StackFrames(0, frames...)
}

func (h *wizardHeader) linesFrame(width int) *inline.Frame {
	keyWidth := 0
	for _, line := range h.lines {
		if line.kind == headerReview {
			keyWidth = max(keyWidth, utf8.RuneCountInString(line.key))
		}
	}
	frame := inline.NewFrame(width, len(h.lines))
	for y, line := range h.lines {
		switch line.kind {
		case headerPrompt:
			x := draw(frame, 0, y, "? ", h.theme.Accent)
			x = draw(frame, x, y, line.key+" ", h.theme.Label)
			x = draw(frame, x, y, "› ", h.theme.Muted)
			draw(frame, x, y, line.value, h.theme.Text)
		case headerHelper:
			draw(frame, 0, y, "· "+line.value, h.theme.Muted)
		case headerReview:
			padding := strings.Repeat(" ", keyWidth-utf8.RuneCountInString(line.key)+style.Gutter)
			x := draw(frame, 0, y, line.key+padding, h.theme.Label)
			draw(frame, x, y, line.value, h.theme.Text)
		}
	}
	return frame
}

func draw(frame *inline.Frame, x, y int, text string, look tcell.Style) int {
	frame.DrawString(x, y, text, look)
	return x + utf8.RuneCountInString(text)
}

// confirmWizard shows the review lines and asks for a final yes or no.
func (r *Renderer) confirmWizard(ctx context.Context, steps wizardSteps, lines []headerLine, label string) (bool, error) {
	lines = append(lines, headerLine{kind: headerBlank})
	header, err := newWizardHeader(steps, lines, r.theme)
	if err != nil {
		return false, err
	}
	form := inline.NewForm("").SetHeader(header).Add(
		inline.NewSelectField("confirmed", label, inline.NewChoice("true", "Yes"), inline.NewChoice("false", "No")).Required(),
	)
	result, err := r.runInteractiveForm(ctx, form)
	if err != nil {
		return false, err
	}
	return result["confirmed"] == "true", nil
}

// announce keeps a success line in scrollback above the live region.
func (r *Renderer) announce(kind, name, verb string) error {
	if r.interactiveRenderer == nil {
		return nil
	}
	p := r.painter()
	return r.interactiveRenderer.Println(strings.Join([]string{
		p.Success("✓"), p.Label(strings.ToUpper(kind[:1]) + kind[1:]), name, p.Label(verb),
	}, " "))
}

func (r *Renderer) painter() style.Painter {
	return r.paint
}
