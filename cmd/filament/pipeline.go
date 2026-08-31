package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/galaxy-io/filament"
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
	doc, err := a.service.Configuration(ctx)
	if err != nil {
		return err
	}
	if helpRequested(args[1:]) {
		return a.printPipelineOperationHelp(args[0], args[1:], doc)
	}
	switch args[0] {
	case "create", "edit":
		return a.changePipeline(ctx, args[0], args[1:], doc)
	case "delete":
		return a.deletePipeline(ctx, args[1:], doc)
	default:
		return fmt.Errorf("unknown pipeline operation %q", args[0])
	}
}

func (a *cliApp) changePipeline(ctx context.Context, operation string, args []string, doc climodel.Document) error {
	parsed, err := a.parseCommandArgs(args)
	if err != nil {
		return err
	}
	name := firstPositional(parsed)
	var existing *climodel.Pipeline
	if operation == "edit" {
		if name == "" {
			return fmt.Errorf("usage: filament pipeline edit <name> [flags]")
		}
		current, ok := doc.Pipelines[name]
		if !ok {
			return fmt.Errorf("pipeline %q does not exist", name)
		}
		existing = &current
	} else if name != "" {
		if _, duplicate := doc.Pipelines[name]; duplicate {
			return fmt.Errorf("pipeline %q already exists", name)
		}
	}
	if name == "" {
		return fmt.Errorf("pipeline name is required; example: filament pipeline create users-copy --source production --sink warehouse --resources users,audit --sync-mode full --write-mode replace")
	}
	p, err := a.pipelineFromFlags(name, existing, parsed.flags, doc)
	if err != nil {
		return err
	}
	testDoc := doc
	testDoc.Pipelines = cloneMap(doc.Pipelines)
	testDoc.Pipelines[name] = p
	if err := validateDocument(testDoc, a.catalog); err != nil {
		return err
	}
	if err := a.service.PutPipeline(ctx, name, p); err != nil {
		return err
	}
	return printSuccess(a.statusWriter(), fmt.Sprintf("%s %s pipeline", pastTense(operation), name))
}

func (a *cliApp) deletePipeline(ctx context.Context, args []string, doc climodel.Document) error {
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
	if _, ok := doc.Pipelines[name]; !ok {
		return fmt.Errorf("pipeline %q does not exist", name)
	}
	if !force {
		confirmed, err := a.confirmDelete("pipeline", name)
		if err != nil {
			return err
		}
		if !confirmed {
			return nil
		}
	}
	if err := a.service.DeletePipeline(ctx, name); err != nil {
		return err
	}
	_, err = fmt.Fprintf(a.statusWriter(), "Deleted pipeline %q.\n", name)
	return err
}

func (a *cliApp) pipelineFromFlags(name string, existing *climodel.Pipeline, flags map[string][]string, doc climodel.Document) (climodel.Pipeline, error) {
	p := climodel.Pipeline{SyncMode: "full", WriteMode: "replace"}
	oldSourceType, oldSinkType := "", ""
	if existing != nil {
		p = *existing
		p.Source.Config = cloneConfigMap(existing.Source.Config)
		p.Sink.Config = cloneConfigMap(existing.Sink.Config)
		oldSourceType = doc.Sources[existing.Source.Ref].Type
		oldSinkType = doc.Sinks[existing.Sink.Ref].Type
	}
	if value := lastFlag(flags, "source"); value != "" {
		p.Source.Ref = value
	}
	if value := lastFlag(flags, "sink"); value != "" {
		p.Sink.Ref = value
	}
	if raw, ok := flagValue(flags, "resources"); ok {
		p.Resources = splitComma(raw)
	}
	if value := lastFlag(flags, "sync-mode"); value != "" {
		p.SyncMode = value
	}
	if value := lastFlag(flags, "write-mode"); value != "" {
		p.WriteMode = value
	}
	if p.Source.Ref == "" || p.Sink.Ref == "" {
		return p, fmt.Errorf("pipeline %q: --source and --sink are required", name)
	}
	source, sourceOK := doc.Sources[p.Source.Ref]
	sink, sinkOK := doc.Sinks[p.Sink.Ref]
	if !sourceOK || !sinkOK {
		return p, fmt.Errorf("pipeline %q: --source and --sink must name saved connections", name)
	}
	if existing != nil && oldSourceType != source.Type {
		p.Source.Config = map[string]any{}
	}
	if existing != nil && oldSinkType != sink.Type {
		p.Sink.Config = map[string]any{}
	}
	allowed := map[string]bool{"source": true, "sink": true, "resources": true, "sync-mode": true, "write-mode": true}
	var err error
	p.Source.Config, err = overlayScopedFlags(p.Source.Config, "source-", a.catalog.Sources[source.Type].Config, flags, allowed)
	if err == nil {
		p.Sink.Config, err = overlayScopedFlags(p.Sink.Config, "sink-", a.catalog.Sinks[sink.Type].Config, flags, allowed)
	}
	if err != nil {
		return p, err
	}
	unsetTargets := map[string]func(){}
	for _, item := range []struct {
		prefix string
		schema filament.ConfigSchema
		config map[string]any
	}{
		{prefix: "source-", schema: a.catalog.Sources[source.Type].Config, config: p.Source.Config},
		{prefix: "sink-", schema: a.catalog.Sinks[sink.Type].Config, config: p.Sink.Config},
	} {
		for _, field := range orderedFields(item.schema, filament.ScopePipeline) {
			flagName := item.prefix + strings.ReplaceAll(field.Name, "_", "-")
			config, fieldName := item.config, field.Name
			unsetTargets[flagName] = func() { delete(config, fieldName) }
		}
	}
	if err := applyFieldUnsets(flags, unsetTargets, allowed); err != nil {
		return p, err
	}
	if err := rejectUnknownFlags(flags, allowed); err != nil {
		return p, err
	}
	p.Source.Config, err = normalizeSavedSecretReferences(a.catalog.Sources[source.Type].Config, p.Source.Config)
	if err == nil {
		p.Sink.Config, err = normalizeSavedSecretReferences(a.catalog.Sinks[sink.Type].Config, p.Sink.Config)
	}
	if err != nil {
		return p, err
	}
	return p, nil
}
