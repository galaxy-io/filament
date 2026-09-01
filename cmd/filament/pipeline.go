package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/galaxy-io/filament"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	climodel "github.com/galaxy-io/filament/cmd/internal/cli/model"
	textrenderer "github.com/galaxy-io/filament/cmd/internal/cli/renderer/text"
)

func (a *cliApp) pipelineCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "Manage saved pipelines",
		Long: `Manage saved pipelines.

An omitted --resources selection means all resources discovered by the source.
Connector pipeline fields use --source-<field> and --sink-<field>. Repeat
--unset to remove optional connector fields from an existing pipeline.`,
		PersistentPreRunE: a.prepareTarget,
	}
	change := func(operation string) func(context.Context, []string) error {
		return func(ctx context.Context, args []string) error {
			return a.changePipeline(ctx, operation, args)
		}
	}
	help := func(operation string) func(context.Context, []string) error {
		return func(ctx context.Context, args []string) error {
			return a.printPipelineOperationHelp(ctx, operation, args)
		}
	}
	cmd.AddCommand(
		a.dynamicCommand(
			"create <name> --source NAME --sink NAME [--resources LIST] [flags]",
			"Save a pipeline", help("create"), change("create"),
		),
		a.dynamicCommand(
			"edit <name> [flags] [--unset source-FIELD|sink-FIELD]",
			"Change a saved pipeline", help("edit"), change("edit"),
		),
		&cobra.Command{
			Use:     "list",
			Aliases: []string{"ls"},
			Short:   "List saved pipelines",
			Args:    cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				result, err := a.service.Pipelines(cmd.Context())
				if err != nil {
					return err
				}
				return textrenderer.Pipelines(a.stdout, result, a.configName())
			},
		},
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
	document, err := a.service.Configuration(ctx)
	if err != nil {
		return err
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
	if current, ok := document.Pipelines[name]; ok {
		existing = &current
	}
	if operation == "edit" && existing == nil {
		return fmt.Errorf("pipeline %q does not exist", name)
	}
	request, err := a.pipelineRequestFromFlags(operation, name, existing, parsed.flags, document)
	if err != nil {
		return err
	}
	if _, err := a.service.SavePipeline(ctx, request); err != nil {
		return err
	}
	return printSuccess(a.statusWriter(), fmt.Sprintf("%s %s pipeline", pastTense(operation), name))
}

func (a *cliApp) deletePipeline(ctx context.Context, name string, force bool) error {
	document, err := a.service.Configuration(ctx)
	if err != nil {
		return err
	}
	if _, ok := document.Pipelines[name]; !ok {
		return fmt.Errorf("pipeline %q does not exist", name)
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
	_, err = fmt.Fprintf(a.statusWriter(), "Deleted pipeline %q.\n", name)
	return err
}

func (a *cliApp) pipelineRequestFromFlags(operation, name string, existing *climodel.Pipeline, flags map[string][]string, document climodel.Document) (cliapp.SavePipelineRequest, error) {
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
		resources := splitComma(raw)
		request.Resources = &resources
	}
	request.SyncMode = lastFlag(flags, "sync-mode")
	request.WriteMode = lastFlag(flags, "write-mode")
	if request.Source == "" || request.Sink == "" {
		return request, fmt.Errorf("pipeline %q: --source and --sink are required", name)
	}
	source, sourceOK := document.Sources[request.Source]
	sink, sinkOK := document.Sinks[request.Sink]
	if !sourceOK || !sinkOK {
		return request, fmt.Errorf("pipeline %q: --source and --sink must name saved connections", name)
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
	for flagName, configName := range sourceUnsets {
		configName := configName
		unsetTargets[flagName] = func() { sourcePatch.Unset = append(sourcePatch.Unset, configName) }
	}
	for flagName, configName := range sinkUnsets {
		configName := configName
		unsetTargets[flagName] = func() { sinkPatch.Unset = append(sinkPatch.Unset, configName) }
	}
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
