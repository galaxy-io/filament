package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	climodel "github.com/galaxy-io/filament/cmd/internal/cli/model"
)

func (a *cliApp) runCommandDefinition() *cobra.Command {
	cmd := a.dynamicCommand(
		"run <pipeline> [flags] | run list [pipeline] [flags] | run --source-connector NAME --sink-connector NAME [flags]",
		"Run pipelines and inspect history", a.printRunHelp, a.runCommand,
	)
	cmd.PersistentPreRunE = a.prepareTarget
	return cmd
}

func (a *cliApp) runCommand(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return a.printRunHelp(ctx, args)
	}
	if args[0] == "ls" || args[0] == "list" {
		parsed, err := a.parseCommandArgs(args[1:])
		if err != nil {
			return err
		}
		allowed := map[string]bool{"limit": true, "next": true}
		if err := rejectUnknownFlags(parsed.flags, allowed); err != nil {
			return err
		}
		pageSize := int64(defaultListLimit)
		if raw, present := flagValue(parsed.flags, "limit"); present {
			pageSize, err = strconv.ParseInt(raw, 10, 32)
			if err != nil || pageSize <= 0 {
				return fmt.Errorf("--limit must be a positive integer")
			}
		}
		return a.listRuns(ctx, climodel.RunListRequest{
			Pipeline: firstPositional(parsed), PageSize: int32(pageSize), Cursor: lastFlag(parsed.flags, "next"),
		})
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
	request.Overrides = overrides
	return request, nil
}
