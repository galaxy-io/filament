package dado

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/atterpac/dado/inline"
	"golang.org/x/term"
)

const interactiveBack = "__back__"

type interactiveOption struct {
	label       string
	value       string
	description string
	tone        inline.ChoiceTone
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
		case "config":
			return interactiveEntry{section: args[0]}, true
		}
		return interactiveEntry{}, false
	}
	return interactiveEntry{}, false
}

func (r *Renderer) runInteractiveAt(ctx context.Context, entry interactiveEntry) (runErr error) {
	renderer := inline.NewRenderer(inline.WithOutput(r.statusWriter()))
	r.interactiveRenderer = renderer
	defer func() {
		runErr = errors.Join(runErr, renderer.Clear(), renderer.Close())
		r.interactiveRenderer = nil
	}()
	if entry.section != "" {
		if err := r.runInteractiveEntry(ctx, entry); err != nil {
			if interactiveInterrupted(err) {
				return nil
			}
			if acknowledgeErr := r.acknowledgeInteractiveError(ctx, err); acknowledgeErr != nil {
				if interactiveInterrupted(acknowledgeErr) {
					return nil
				}
				return acknowledgeErr
			}
		}
	}

	for {
		description := "Selected target"
		if r.targetName != "" {
			description = r.targetName + " target"
		}
		action, err := r.chooseInteractive(ctx, "Filament", description, []interactiveOption{
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
		case "config":
			err = r.manageInteractiveConfig(ctx)
		}
		if interactiveInterrupted(err) {
			return nil
		}
		if acknowledgeErr := r.acknowledgeInteractiveError(ctx, err); acknowledgeErr != nil {
			if interactiveInterrupted(acknowledgeErr) {
				return nil
			}
			return acknowledgeErr
		}
	}
}

func (r *Renderer) runInteractiveEntry(ctx context.Context, entry interactiveEntry) error {
	switch entry.section {
	case "config":
		return r.manageInteractiveConfig(ctx)
	default:
		return nil
	}
}

func (r *Renderer) acknowledgeInteractiveError(ctx context.Context, err error) error {
	if err == nil || interactiveCancelled(err) {
		return nil
	}
	if interactiveInterrupted(err) {
		return err
	}
	acknowledgeErr := r.showInteractiveMessage(ctx, "Something went wrong", err.Error())
	if interactiveCancelled(acknowledgeErr) {
		return nil
	}
	return acknowledgeErr
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
		})
	}
	form := inline.NewForm(title).Add(
		inline.NewSelectField("selection", description, choices...).Required(),
	)
	result, err := r.runInteractiveForm(ctx, form)
	if err != nil {
		return "", err
	}
	selected, _ := result["selection"].(string)
	return selected, nil
}

func (r *Renderer) showInteractiveMessage(ctx context.Context, title, description string) error {
	lines := strings.Split(strings.TrimSpace(description), "\n")
	options := make([]inline.Choice, 0, len(lines)+1)
	for index, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		options = append(options, inline.Choice{
			Value:    fmt.Sprintf("message-%d", index),
			Label:    line,
			Disabled: true,
		})
	}
	options = append(options, inline.NewChoice(interactiveBack, "Back"))
	form := inline.NewForm(title).Add(inline.NewSelectField("selection", "", options...).Required())
	_, err := r.runInteractiveForm(ctx, form)
	return err
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
	return inline.NewSession(r.interactiveRenderer, options...).Run(ctx, form.QuitOnQ(true))
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
		choices = append(choices, inline.Choice{Value: option.value, Label: option.label, Description: option.description, Tone: option.tone})
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
