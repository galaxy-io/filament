package main

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	climodel "github.com/galaxy-io/filament/cmd/internal/cli/model"
	textrenderer "github.com/galaxy-io/filament/cmd/internal/cli/renderer/text"
)

func (a *cliApp) runPipelineCommand(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		_, err := fmt.Fprint(a.stdout, pipelineHelp)
		return err
	}
	if args[0] == "list" && helpRequested(args[1:]) {
		return a.printPipelineOperationHelp(args[0], args[1:], climodel.NewDocument())
	}
	if args[0] == "list" {
		if len(args) != 1 {
			return fmt.Errorf("usage: filament pipeline list")
		}
		result, err := a.service.Pipelines(ctx)
		if err != nil {
			return err
		}
		return textrenderer.Pipelines(a.stdout, result)
	}
	document, err := a.service.Configuration(ctx)
	if err != nil {
		return err
	}
	if helpRequested(args[1:]) {
		return a.printPipelineOperationHelp(args[0], args[1:], document)
	}
	switch args[0] {
	case "create", "edit":
		return a.changePipeline(ctx, args[0], args[1:], document)
	case "delete":
		return a.deletePipeline(ctx, args[1:], document)
	default:
		return fmt.Errorf("unknown pipeline operation %q", args[0])
	}
}

func (a *cliApp) changePipeline(ctx context.Context, operation string, args []string, document climodel.Document) error {
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

func (a *cliApp) deletePipeline(ctx context.Context, args []string, document climodel.Document) error {
	parsed, err := a.parseCommandArgs(args)
	if err != nil {
		return err
	}
	name := firstPositional(parsed)
	if name == "" {
		return fmt.Errorf("usage: filament pipeline delete <name> [--force]")
	}
	force, err := parseForceFlag(parsed.flags)
	if err != nil {
		return err
	}
	if err := rejectUnknownFlags(parsed.flags, map[string]bool{"force": true}); err != nil {
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
