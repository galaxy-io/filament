package dado

import (
	"context"
)

func (r *Renderer) manageInteractiveConfig(ctx context.Context) error {
	for {
		title, description := "Configuration", r.configPath
		options := []interactiveOption{{label: "Validate", value: "validate"}}
		if r.configPath != "" && r.openConfigurationEditor != nil {
			options = append(options,
				interactiveOption{label: "Edit", value: "edit"},
				interactiveOption{label: "Show path", value: "path"},
			)
		} else {
			title, description = r.titled("Configuration"), ""
		}
		options = append(options, interactiveOption{label: "Back", value: interactiveBack})
		action, err := r.chooseInteractive(ctx, title, description, options)
		if err != nil || action == interactiveBack {
			return err
		}
		switch action {
		case "validate":
			if validateErr := r.service.ValidateConfiguration(ctx); validateErr != nil {
				if err := r.notice(false, validateErr.Error()); err != nil {
					return err
				}
			} else if err := r.notice(true, "Configuration is valid"); err != nil {
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
			if err := r.notice(true, r.configPath); err != nil {
				return err
			}
		}
	}
}
