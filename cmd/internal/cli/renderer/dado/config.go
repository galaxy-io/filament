package dado

import (
	"context"
)

func (r *Renderer) manageInteractiveConfig(ctx context.Context) error {
	for {
		description := r.configPath
		options := []interactiveOption{{label: "Validate", value: "validate"}}
		if r.configPath != "" && r.openConfigurationEditor != nil {
			options = append(options,
				interactiveOption{label: "Edit", value: "edit"},
				interactiveOption{label: "Show path", value: "path"},
			)
		} else {
			description = r.targetName + " target"
		}
		options = append(options, interactiveOption{label: "Back", value: interactiveBack})
		action, err := r.chooseInteractive(ctx, "Configuration", description, options)
		if err != nil || action == interactiveBack {
			return err
		}
		switch action {
		case "validate":
			validateErr := r.service.ValidateConfiguration(ctx)
			message := "Configuration is valid."
			if validateErr != nil {
				message = validateErr.Error()
			}
			if err := r.showInteractiveMessage(ctx, "Validation", message); err != nil && !interactiveCancelled(err) {
				return err
			}
		case "edit":
			if err := r.clearInteractive(); err != nil {
				return err
			}
			if err := r.openConfigurationEditor(ctx); err != nil {
				return err
			}
		case "path":
			if err := r.showInteractiveMessage(ctx, "Configuration path", r.configPath); err != nil && !interactiveCancelled(err) {
				return err
			}
		}
	}
}
