package main

import (
	"context"

	"github.com/spf13/cobra"

	textrenderer "github.com/galaxy-io/filament/cmd/internal/cli/renderer/text"
)

func (a *cliApp) runsCommand() *cobra.Command {
	return &cobra.Command{
		Use:               "runs [pipeline]",
		Short:             "List runs",
		Args:              cobra.MaximumNArgs(1),
		PersistentPreRunE: a.prepareTarget,
		RunE: func(cmd *cobra.Command, args []string) error {
			pipeline := ""
			if len(args) == 1 {
				pipeline = args[0]
			}
			return a.listRuns(cmd.Context(), pipeline)
		},
	}
}

func (a *cliApp) listRuns(ctx context.Context, pipeline string) error {
	result, err := a.service.Runs(ctx, pipeline)
	if err != nil {
		return err
	}
	return textrenderer.Runs(a.stdout, result, a.target.Name)
}
