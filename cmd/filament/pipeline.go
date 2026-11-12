package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/galaxy-io/filament"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	climodel "github.com/galaxy-io/filament/cmd/internal/cli/model"
)

func (a *cliApp) pipelineCommand() *cobra.Command {
	cmd := a.targetGroup(&cobra.Command{
		Use:   "pipeline",
		Short: "Manage saved pipelines",
		Long:  "Manage saved pipelines.\n\n" + pipelineGuidance,
	}, "pipeline")
	change := func(operation string) func(context.Context, []string) error {
		return func(ctx context.Context, args []string) error {
			return a.changePipeline(ctx, operation, args)
		}
	}
	cmd.AddCommand(
		a.dynamicCommand(
			"create <name> --source NAME --sink NAME [--resources LIST] [flags]",
			"Save a pipeline", a.printPipelineOperationHelp, change("create"),
		),
		a.dynamicCommand(
			"edit <name> [flags] [--unset source-FIELD|sink-FIELD]",
			"Change a saved pipeline", a.printPipelineOperationHelp, change("edit"),
		),
		a.pipelineListCommand(),
	)
	remove := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a saved pipeline",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			return a.deletePipeline(cmd.Context(), args[0], force)
		},
	}
	remove.Flags().Bool("force", false, "Skip the confirmation prompt")
	cmd.AddCommand(remove)
	return cmd
}

func (a *cliApp) changePipeline(ctx context.Context, operation string, args []string) error {
	if operation == "create" && len(args) == 0 && a.renderer().Interactive() {
		return a.openInteractive(ctx, "pipeline", operation)
	}
	parsed, err := a.parseCommandArgs(args)
	if err != nil {
		return err
	}
	name := firstPositional(parsed)
	if name == "" {
		return fmt.Errorf("pipeline name is required; example: filament pipeline create users-copy --source production --sink warehouse --resources users,audit --sync-mode full --write-mode replace")
	}
	var existing *climodel.Pipeline
	if operation == "edit" {
		current, err := a.service.Pipeline(ctx, name)
		if err != nil {
			return err
		}
		existing = &current
	}
	request, err := a.pipelineRequestFromFlags(ctx, operation, name, existing, parsed.flags)
	if err != nil {
		return err
	}
	if _, err := a.service.SavePipeline(ctx, request); err != nil {
		return err
	}
	return printSuccess(a.statusWriter(), fmt.Sprintf("%s %s pipeline", pastTense(operation), name))
}

func (a *cliApp) deletePipeline(ctx context.Context, name string, force bool) error {
	if _, err := a.service.Pipeline(ctx, name); err != nil {
		return err
	}
	if !force {
		confirmed, err := a.confirmDelete("pipeline", name)
		if err != nil || !confirmed {
			return err
		}
	}
	if err := a.service.DeleteSavedPipeline(ctx, name); err != nil {
		return err
	}
	_, err := fmt.Fprintf(a.statusWriter(), "Deleted pipeline %q.\n", name)
	return err
}

func (a *cliApp) pipelineRequestFromFlags(ctx context.Context, operation, name string, existing *climodel.Pipeline, flags map[string][]string) (cliapp.SavePipelineRequest, error) {
	request := cliapp.SavePipelineRequest{Create: operation == "create", Name: name}
	if existing != nil {
		request.Source = existing.Source.Ref
		request.Sink = existing.Sink.Ref
	}
	if value := lastFlag(flags, "source"); value != "" {
		request.Source = value
	}
	if value := lastFlag(flags, "sink"); value != "" {
		request.Sink = value
	}
	if raw, present := flagValue(flags, "resources"); present {
		resources := cliapp.SplitComma(raw)
		request.Resources = &resources
	}
	request.SyncMode = lastFlag(flags, "sync-mode")
	request.WriteMode = lastFlag(flags, "write-mode")
	if request.Source == "" || request.Sink == "" {
		return request, fmt.Errorf("pipeline %q: --source and --sink are required", name)
	}
	source, err := a.service.Connection(ctx, "source", request.Source)
	if err != nil {
		return request, fmt.Errorf("pipeline %q: %w", name, err)
	}
	sink, err := a.service.Connection(ctx, "sink", request.Sink)
	if err != nil {
		return request, fmt.Errorf("pipeline %q: %w", name, err)
	}
	allowed := map[string]bool{
		"source": true, "sink": true, "resources": true, "sync-mode": true, "write-mode": true,
	}
	sourcePatch, sourceUnsets, err := configPatchFromFlags(a.catalog.Sources[source.Type].Config, filament.ScopePipeline, "source-", flags, allowed, false)
	if err != nil {
		return request, err
	}
	sinkPatch, sinkUnsets, err := configPatchFromFlags(a.catalog.Sinks[sink.Type].Config, filament.ScopePipeline, "sink-", flags, allowed, false)
	if err != nil {
		return request, err
	}
	unsetTargets := map[string]func(){}
	addUnsetTargets(unsetTargets, sourceUnsets, &sourcePatch)
	addUnsetTargets(unsetTargets, sinkUnsets, &sinkPatch)
	if err := applyFieldUnsets(flags, unsetTargets, allowed); err != nil {
		return request, err
	}
	if err := rejectUnknownFlags(flags, allowed); err != nil {
		return request, err
	}
	request.SourceConfig = sourcePatch
	request.SinkConfig = sinkPatch
	return request, nil
}

func (a *cliApp) pipelineListCommand() *cobra.Command {
	var page listPageFlags
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List saved pipelines",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			request, err := page.request()
			if err != nil {
				return err
			}
			result, err := a.service.PipelinePage(cmd.Context(), request)
			if err != nil {
				return err
			}
			return a.text().Pipelines(result, a.configName(), "filament pipeline list")
		},
	}
	page.add(cmd)
	return cmd
}
