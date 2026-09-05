package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	climodel "github.com/galaxy-io/filament/cmd/internal/cli/model"
)

func (a *cliApp) runCommandDefinition() *cobra.Command {
	cmd := a.dynamicCommand(
		"run <pipeline> [flags] | run --source-connector NAME --sink-connector NAME [flags]",
		"Run a saved pipeline or an inline transfer, or list past runs", a.printRunHelp, a.runCommand,
	)
	cmd.PersistentPreRunE = a.prepareTarget
	cmd.AddCommand(a.runListCommand())
	return cmd
}

func (a *cliApp) runCommand(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return a.printRunHelp(ctx, args)
	}
	parsed, err := a.parseCommandArgs(args)
	if err != nil {
		return err
	}
	name := firstPositional(parsed)
	var request cliapp.RunRequest
	if name == "" {
		request, err = a.directRunRequest(parsed.flags)
	} else {
		request, err = a.savedRunRequest(ctx, name, parsed.flags)
	}
	if err != nil {
		return err
	}
	if interactive := a.renderer(); interactive.Interactive() {
		return interactive.RunRequest(ctx, request)
	}
	spec, result, err := a.service.ExecuteRun(ctx, request, nil)
	if err != nil {
		return err
	}
	runIDs := make([]string, 0, len(result.Runs))
	for _, run := range result.Runs {
		runIDs = append(runIDs, run.ID)
	}
	if len(runIDs) == 0 && spec.Run != "" {
		runIDs = append(runIDs, string(spec.Run))
	}
	_, err = fmt.Fprintf(a.statusWriter(), "Run %s completed: %d records, %d bytes.\n", strings.Join(runIDs, ", "), result.Records, result.Bytes)
	return err
}

func (a *cliApp) directRunRequest(flags map[string][]string) (cliapp.RunRequest, error) {
	sourceConnector := lastFlag(flags, "source-connector")
	sinkConnector := lastFlag(flags, "sink-connector")
	request := cliapp.RunRequest{Inline: &cliapp.InlineRun{
		Source: cliapp.InlineConnector{Connector: sourceConnector},
		Sink:   cliapp.InlineConnector{Connector: sinkConnector},
	}}
	if sourceConnector == "" || sinkConnector == "" {
		return request, fmt.Errorf("--source-connector and --sink-connector are required; example: filament run --source-connector postgres --source-dsn postgres://... --sink-connector stdout")
	}
	source, sourceOK := a.catalog.Sources[sourceConnector]
	if !sourceOK {
		return request, fmt.Errorf("unknown source connector %q", sourceConnector)
	}
	sink, sinkOK := a.catalog.Sinks[sinkConnector]
	if !sinkOK {
		return request, fmt.Errorf("unknown sink connector %q", sinkConnector)
	}
	allowed := map[string]bool{
		"source-connector": true, "sink-connector": true, "resources": true,
		"sync-mode": true, "write-mode": true,
	}
	sourcePatch, _, err := configPatchFromAllFlags(source.Config, "source-", flags, allowed, true)
	if err != nil {
		return request, err
	}
	sinkPatch, _, err := configPatchFromAllFlags(sink.Config, "sink-", flags, allowed, true)
	if err != nil {
		return request, err
	}
	if err := rejectUnknownFlags(flags, allowed); err != nil {
		return request, err
	}
	request.Inline.Source.Config = sourcePatch.Values
	request.Inline.Sink.Config = sinkPatch.Values
	if raw, present := flagValue(flags, "resources"); present {
		request.Inline.Resources = splitComma(raw)
		if len(request.Inline.Resources) == 0 {
			return request, fmt.Errorf("--resources must contain at least one resource")
		}
	}
	request.Inline.SyncMode = lastFlag(flags, "sync-mode")
	request.Inline.WriteMode = lastFlag(flags, "write-mode")
	return request, nil
}

func (a *cliApp) savedRunRequest(ctx context.Context, name string, flags map[string][]string) (cliapp.RunRequest, error) {
	request := cliapp.RunRequest{Pipeline: name}
	for _, topologyFlag := range []string{"source", "sink", "source-connector", "sink-connector"} {
		if _, present := flags[topologyFlag]; present {
			return request, fmt.Errorf("named pipeline %q cannot be combined with --%s", name, topologyFlag)
		}
	}
	document, err := a.service.Configuration(ctx)
	if err != nil {
		return request, err
	}
	pipeline, ok := document.Pipelines[name]
	if !ok {
		return request, fmt.Errorf("pipeline %q does not exist", name)
	}
	overrides, err := a.pipelineRequestFromFlags("edit", name, &pipeline, flags, document)
	if err != nil {
		return request, err
	}
	// Topology flags are rejected above, so the seeded refs are the saved
	// pipeline's own; clearing them keeps a flagless run a plain saved run.
	overrides.Source, overrides.Sink = "", ""
	request.Overrides = overrides
	return request, nil
}

// runListCommand shows one page of run history: run list [pipeline] [--limit N] [--next CURSOR].
func (a *cliApp) runListCommand() *cobra.Command {
	var page listPageFlags
	cmd := &cobra.Command{
		Use:     "list [pipeline]",
		Aliases: []string{"ls"},
		Short:   "List past runs",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			request, err := page.request()
			if err != nil {
				return err
			}
			pipeline := ""
			if len(args) == 1 {
				pipeline = args[0]
			}
			result, err := a.service.Runs(cmd.Context(), climodel.RunListRequest{Pipeline: pipeline, PageRequest: request})
			if err != nil {
				return err
			}
			return a.text().Runs(result, strings.TrimSpace("filament run list "+pipeline))
		},
	}
	page.add(cmd)
	return cmd
}
