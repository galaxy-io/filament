package dado

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/atterpac/dado/inline"
	"golang.org/x/term"

	textrenderer "github.com/galaxy-io/filament/cmd/internal/cli/renderer/text"
	"github.com/galaxy-io/filament/cmd/internal/cli/style"
)

const interactiveBack = "__back__"

type interactiveOption struct {
	label       string
	value       string
	description string
	tone        inline.ChoiceTone
	disabled    bool
}

// boxedMenu renders header and rows as the same boxed table the list
// commands print; borders and the header are unselectable rows.
func boxedMenu(headers []string, rows [][]string, values []string) []interactiveOption {
	widths := style.Widths(append([][]string{headers}, rows...))
	rule := func(left, junction, right string) string {
		parts := make([]string, len(widths))
		for index, width := range widths {
			parts[index] = strings.Repeat("─", width+2)
		}
		return left + strings.Join(parts, junction) + right
	}
	line := func(cells []string) string {
		parts := make([]string, len(cells))
		for index, cell := range cells {
			parts[index] = " " + pad(cell, widths[index]) + " "
		}
		return "│" + strings.Join(parts, "│") + "│"
	}
	options := make([]interactiveOption, 0, len(rows)+4)
	options = append(options,
		interactiveOption{label: rule("╭", "┬", "╮"), disabled: true},
		interactiveOption{label: line(headers), disabled: true},
		interactiveOption{label: rule("├", "┼", "┤"), disabled: true},
	)
	for index, row := range rows {
		options = append(options, interactiveOption{label: line(row), value: values[index]})
	}
	options = append(options, interactiveOption{label: rule("╰", "┴", "╯"), disabled: true})
	return options
}

type interactiveEntry struct {
	section   string
	operation string
}

func (r *Renderer) interactiveAvailable() bool {
	in, inputOK := r.stdin.(*os.File)
	out, outputOK := r.statusWriter().(*os.File)
	return inputOK && outputOK && term.IsTerminal(int(in.Fd())) && term.IsTerminal(int(out.Fd()))
}

func interactiveEntryForArgs(args []string) (interactiveEntry, bool) {
	if len(args) == 0 {
		return interactiveEntry{}, true
	}
	if len(args) == 1 {
		switch args[0] {
		case "source", "sink", "pipeline", "config", "run":
			return interactiveEntry{section: args[0]}, true
		}
		return interactiveEntry{}, false
	}
	if len(args) == 2 {
		switch args[0] {
		case "source", "sink", "pipeline":
			if args[1] == "create" {
				return interactiveEntry{section: args[0], operation: args[1]}, true
			}
		}
	}
	return interactiveEntry{}, false
}

func (r *Renderer) runInteractiveAt(ctx context.Context, entry interactiveEntry) (runErr error) {
	renderer := inline.NewRenderer(inline.WithOutput(r.statusWriter()))
	r.interactiveRenderer = renderer
	defer func() {
		r.flushNotice()
		runErr = errors.Join(runErr, renderer.Clear(), renderer.Close())
		r.interactiveRenderer = nil
	}()
	if entry.section != "" {
		if err := r.runInteractiveEntry(ctx, entry); err != nil {
			if interactiveInterrupted(err) {
				return nil
			}
			if acknowledgeErr := r.acknowledgeInteractiveError(err); acknowledgeErr != nil {
				if interactiveInterrupted(acknowledgeErr) {
					return nil
				}
				return acknowledgeErr
			}
			if entry.operation != "" && !interactiveCancelled(err) {
				return err
			}
		}
		if entry.operation != "" {
			return nil
		}
	}

	for {
		action, err := r.chooseInteractive(ctx, r.titled("Filament"), "", []interactiveOption{
			{label: "Run", value: "run"},
			{label: "Sources", value: "sources"},
			{label: "Sinks", value: "sinks"},
			{label: "Pipelines", value: "pipelines"},
			{label: "Runs", value: "runs"},
			{label: "Configuration", value: "config"},
			{label: "Exit", value: interactiveBack},
		})
		if interactiveInterrupted(err) {
			return nil
		}
		if interactiveCancelled(err) || action == interactiveBack {
			return nil
		}
		if err != nil {
			return err
		}

		switch action {
		case "run":
			err = r.interactiveRun(ctx)
		case "sources":
			err = r.manageConnections(ctx, "source")
		case "sinks":
			err = r.manageConnections(ctx, "sink")
		case "pipelines":
			err = r.managePipelines(ctx)
		case "runs":
			err = r.showRuns(ctx)
		case "config":
			err = r.manageInteractiveConfig(ctx)
		}
		if interactiveInterrupted(err) {
			return nil
		}
		if acknowledgeErr := r.acknowledgeInteractiveError(err); acknowledgeErr != nil {
			if interactiveInterrupted(acknowledgeErr) {
				return nil
			}
			return acknowledgeErr
		}
	}
}

func (r *Renderer) runInteractiveEntry(ctx context.Context, entry interactiveEntry) error {
	switch entry.section {
	case "source", "sink":
		if entry.operation == "create" {
			err := r.connectionWizard(ctx, entry.section, "", nil)
			if interactiveCancelled(err) {
				return nil
			}
			return err
		}
		return r.manageConnections(ctx, entry.section)
	case "pipeline":
		if entry.operation == "create" {
			err := r.pipelineWizard(ctx, "", nil)
			if interactiveCancelled(err) {
				return nil
			}
			return err
		}
		return r.managePipelines(ctx)
	case "config":
		return r.manageInteractiveConfig(ctx)
	case "run":
		return r.interactiveRun(ctx)
	default:
		return nil
	}
}

func (r *Renderer) acknowledgeInteractiveError(err error) error {
	if err == nil || interactiveCancelled(err) {
		return nil
	}
	if interactiveInterrupted(err) {
		return err
	}
	return r.notice(false, err.Error())
}

// titled appends the target name to a menu title so the header stays one line.
func (r *Renderer) titled(base string) string {
	if r.targetName == "" {
		return base
	}
	return base + " (" + r.targetName + ")"
}

// menuHeader is the heading above a menu form: an accent title and, when an
// operation just finished, its outcome line.
type menuHeader struct {
	theme    inline.InlineTheme
	title    string
	notice   string
	noticeOK bool
	// gap appends a blank row so a following field label is not glued to
	// the heading.
	gap bool
}

func (h menuHeader) Frame(width int) *inline.Frame {
	height := 1
	if h.notice != "" {
		height++
	}
	if h.gap {
		height++
	}
	frame := inline.NewFrame(max(width, 0), height)
	draw(frame, 0, 0, h.title, h.theme.Accent.Bold(true))
	if h.notice != "" {
		marker, tone := "✗", h.theme.Error
		if h.noticeOK {
			marker, tone = "✓", h.theme.Success
		}
		x := draw(frame, 0, 1, marker+" ", tone)
		draw(frame, x, 1, h.notice, h.theme.Text)
	}
	return frame
}

func (r *Renderer) chooseInteractive(ctx context.Context, title, description string, options []interactiveOption) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("%s has no available options", title)
	}
	choices := make([]inline.Choice, 0, len(options))
	for _, option := range options {
		choices = append(choices, inline.Choice{
			Value:       option.value,
			Label:       option.label,
			Description: option.description,
			Tone:        option.tone,
			Disabled:    option.disabled,
		})
	}
	field := inline.NewSelectField("selection", description, choices...).Required()
	// Every menu renders its heading as a header frame: it keeps the layout
	// compact and gives a queued outcome line a stable home under the title.
	notice, noticeOK := r.takeNotice()
	header := menuHeader{theme: r.theme, title: title, notice: notice, noticeOK: noticeOK, gap: description != ""}
	form := inline.NewForm("").SetHeader(header).Add(field)
	result, err := r.runInteractiveForm(ctx, form)
	if err != nil {
		return "", err
	}
	selected, _ := result["selection"].(string)
	return selected, nil
}

// showRuns keeps the target's run history table in scrollback.
func (r *Renderer) showRuns(ctx context.Context) error {
	result, err := r.service.Runs(ctx, "")
	if err != nil {
		return r.notice(false, err.Error())
	}
	var rendered strings.Builder
	if err := textrenderer.Runs(&rendered, result, r.targetName); err != nil {
		return err
	}
	if r.interactiveRenderer == nil {
		return nil
	}
	for line := range strings.SplitSeq(strings.TrimRight(rendered.String(), "\n"), "\n") {
		if err := r.interactiveRenderer.Println(line); err != nil {
			return err
		}
	}
	return r.interactiveRenderer.Println("")
}

// showInteractiveMessage keeps a titled block in scrollback and returns to the menu.
func (r *Renderer) showInteractiveMessage(_ context.Context, title, description string) error {
	if r.interactiveRenderer == nil {
		return nil
	}
	p := r.painter()
	lines := []string{p.Label(title)}
	for _, line := range strings.Split(strings.TrimSpace(description), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, style.Indent+line)
		}
	}
	return r.interactiveRenderer.Println(strings.Join(lines, "\n") + "\n")
}

// notice keeps one status line in scrollback: a green check or a red cross.
// notice queues an outcome line. The next menu renders it inside its header
// instead of scattering it through scrollback; a session that exits first
// flushes it so direct operations still report.
func (r *Renderer) notice(ok bool, message string) error {
	if r.interactiveRenderer == nil {
		return nil
	}
	r.noticeText, r.noticeOK = message, ok
	return nil
}

// takeNotice consumes the pending notice.
func (r *Renderer) takeNotice() (string, bool) {
	text, ok := r.noticeText, r.noticeOK
	r.noticeText, r.noticeOK = "", false
	return text, ok
}

func (r *Renderer) flushNotice() {
	text, ok := r.takeNotice()
	if text == "" || r.interactiveRenderer == nil {
		return
	}
	p := r.painter()
	marker := p.Error("✗")
	if ok {
		marker = p.Success("✓")
	}
	_ = r.interactiveRenderer.Println(marker + " " + text)
}

func (r *Renderer) confirmInteractive(ctx context.Context, title, description string) (bool, error) {
	form := inline.NewForm(title).Add(inline.NewSelectField("confirmed", description,
		inline.NewChoice("false", "No"),
		inline.NewChoice("true", "Yes"),
	).Required())
	result, err := r.runInteractiveForm(ctx, form)
	if err != nil {
		return false, err
	}
	return result["confirmed"] == "true", nil
}

func (r *Renderer) runInteractiveForm(ctx context.Context, form *inline.Form) (inline.FormResult, error) {
	if r.interactiveRenderer == nil {
		return nil, errors.New("interactive renderer is not running")
	}
	options := []inline.SessionOption{}
	if r.stdin != nil {
		options = append(options, inline.WithSessionInput(r.stdin))
	}
	return inline.NewSession(r.interactiveRenderer, options...).Run(ctx, form.SetTheme(r.theme).QuitOnQ(true))
}

func (r *Renderer) clearInteractive() error {
	if r.interactiveRenderer == nil {
		return nil
	}
	return r.interactiveRenderer.Clear()
}

func interactiveCancelled(err error) bool {
	return errors.Is(err, inline.ErrFormCancelled)
}

func interactiveInterrupted(err error) bool {
	return errors.Is(err, inline.ErrFormInterrupted) || errors.Is(err, context.Canceled)
}

func preferredChoices(preferred string, options []interactiveOption) []inline.Choice {
	choices := make([]inline.Choice, 0, len(options))
	appendChoice := func(option interactiveOption) {
		choices = append(choices, inline.Choice{Value: option.value, Label: option.label, Description: option.description, Tone: option.tone, Disabled: option.disabled})
	}
	for _, option := range options {
		if option.value == preferred {
			appendChoice(option)
		}
	}
	for _, option := range options {
		if option.value != preferred {
			appendChoice(option)
		}
	}
	return choices
}

// showPairs keeps a titled key/value block in scrollback.
func (r *Renderer) showPairs(title string, pairs [][2]string) error {
	if r.interactiveRenderer == nil {
		return nil
	}
	p := r.painter()
	width := 0
	for _, pair := range pairs {
		width = max(width, utf8.RuneCountInString(pair[0]))
	}
	lines := []string{p.Bold(title)}
	for _, pair := range pairs {
		padding := strings.Repeat(" ", width-utf8.RuneCountInString(pair[0])+style.Gutter)
		lines = append(lines, style.Indent+p.Label(pair[0])+padding+pair[1])
	}
	return r.interactiveRenderer.Println(strings.Join(lines, "\n") + "\n")
}
