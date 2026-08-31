package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/galaxy-io/filament"
	climodel "github.com/galaxy-io/filament/cmd/internal/cli/model"
	"github.com/galaxy-io/filament/internal/naming"
)

func (a *cliApp) runCommand(ctx context.Context, args []string) error {
	if len(args) == 0 || helpRequested(args) {
		return a.printRunHelp(ctx, args)
	}
	parsed, err := a.parseCommandArgs(args)
	if err != nil {
		return err
	}

	name := firstPositional(parsed)
	var spec filament.RunSpec
	if name == "" {
		spec, err = a.directRunSpec(parsed.flags)
	} else {
		spec, err = a.savedRunSpec(ctx, name, parsed.flags)
	}
	if err != nil {
		return err
	}

	result, err := a.service.Run(ctx, spec, nil)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(a.statusWriter(), "Run %s completed: %d records, %d bytes.\n", spec.Run, result.Records, result.Bytes)
	return err
}

func (a *cliApp) directRunSpec(flags map[string][]string) (filament.RunSpec, error) {
	sourceName := lastFlag(flags, "source-connector")
	sinkName := lastFlag(flags, "sink-connector")
	if sourceName == "" || sinkName == "" {
		return filament.RunSpec{}, fmt.Errorf("--source-connector and --sink-connector are required; example: filament run --source-connector postgres --source-dsn postgres://... --sink-connector stdout")
	}
	source, sourceOK := a.catalog.Sources[sourceName]
	if !sourceOK {
		return filament.RunSpec{}, fmt.Errorf("unknown source connector %q", sourceName)
	}
	sink, sinkOK := a.catalog.Sinks[sinkName]
	if !sinkOK {
		return filament.RunSpec{}, fmt.Errorf("unknown sink connector %q", sinkName)
	}

	allowed := map[string]bool{
		"source-connector": true,
		"sink-connector":   true,
		"resources":        true,
		"sync-mode":        true,
		"write-mode":       true,
	}
	sourceConfig, err := directConnectorConfig("source", source.Config, flags, allowed)
	if err != nil {
		return filament.RunSpec{}, err
	}
	sinkConfig, err := directConnectorConfig("sink", sink.Config, flags, allowed)
	if err != nil {
		return filament.RunSpec{}, err
	}
	sinkConfig = applySinkSchemaDefault(sinkConfig, sink, sourceName)
	if err := rejectUnknownFlags(flags, allowed); err != nil {
		return filament.RunSpec{}, err
	}

	resources := []string(nil)
	if raw, present := flagValue(flags, "resources"); present {
		resources = splitComma(raw)
		if len(resources) == 0 {
			return filament.RunSpec{}, fmt.Errorf("--resources must contain at least one resource")
		}
	}
	return makeRunSpec("", sourceName, sinkName, sourceConfig, sinkConfig, resources, lastFlag(flags, "sync-mode"), lastFlag(flags, "write-mode"))
}

func (a *cliApp) savedRunSpec(ctx context.Context, name string, flags map[string][]string) (filament.RunSpec, error) {
	for _, topologyFlag := range []string{"source", "sink", "source-connector", "sink-connector"} {
		if _, present := flags[topologyFlag]; present {
			return filament.RunSpec{}, fmt.Errorf("named pipeline %q cannot be combined with --%s", name, topologyFlag)
		}
	}

	doc, err := a.service.Configuration(ctx)
	if err != nil {
		return filament.RunSpec{}, err
	}
	if err := validateDocument(doc, a.catalog); err != nil {
		return filament.RunSpec{}, err
	}
	p, ok := doc.Pipelines[name]
	if !ok {
		return filament.RunSpec{}, fmt.Errorf("pipeline %q does not exist", name)
	}
	if len(flags) > 0 {
		p, err = a.pipelineFromFlags(name, &p, flags, doc)
		if err != nil {
			return filament.RunSpec{}, err
		}
		testDoc := doc
		testDoc.Pipelines = cloneMap(doc.Pipelines)
		testDoc.Pipelines[name] = p
		if err := validateDocument(testDoc, a.catalog); err != nil {
			return filament.RunSpec{}, err
		}
	}

	source := doc.Sources[p.Source.Ref]
	sink := doc.Sinks[p.Sink.Ref]
	sourceConfig, err := resolvedConnectionConfig(source, p.Source.Config, a.catalog.Sources[source.Type].Config)
	if err != nil {
		return filament.RunSpec{}, fmt.Errorf("source %q: %w", p.Source.Ref, err)
	}
	sinkConfig, err := resolvedConnectionConfig(sink, p.Sink.Config, a.catalog.Sinks[sink.Type].Config)
	if err != nil {
		return filament.RunSpec{}, fmt.Errorf("sink %q: %w", p.Sink.Ref, err)
	}
	sinkConfig = applySinkSchemaDefault(sinkConfig, a.catalog.Sinks[sink.Type], p.Source.Ref)
	return makeRunSpec(name, source.Type, sink.Type, sourceConfig, sinkConfig, p.Resources, p.SyncMode, p.WriteMode)
}

func directConnectorConfig(kind string, schema filament.ConfigSchema, flags map[string][]string, allowed map[string]bool) (map[string]any, error) {
	config := map[string]any{}
	prefix := kind + "-"
	for _, field := range schema.Fields {
		name := prefix + strings.ReplaceAll(field.Name, "_", "-")
		allowed[name] = true
		if isSecretField(field) {
			envName := name + "-env"
			allowed[envName] = true
			raw, direct := flagValue(flags, name)
			envVar, fromEnv := flagValue(flags, envName)
			if direct && fromEnv {
				return nil, fmt.Errorf("--%s and --%s cannot be combined", name, envName)
			}
			if fromEnv {
				variable, err := normalizeEnvironmentName(envVar)
				if err != nil {
					return nil, fmt.Errorf("--%s: %w", envName, err)
				}
				raw = "env:" + variable
			}
			if direct || fromEnv {
				resolved, err := resolveEnvironmentReference(raw)
				if err != nil {
					return nil, fmt.Errorf("--%s: %w", name, err)
				}
				config[field.Name] = resolved
			}
			continue
		}
		if raw, present := flagValue(flags, name); present {
			value, err := parseFlagValue(field, raw)
			if err != nil {
				return nil, fmt.Errorf("--%s: %w", name, err)
			}
			config[field.Name] = value
		}
	}

	config = canonicalizeConfig(schema, config)
	config, err := resolveConfigSecretReferences(schema, config)
	if err != nil {
		return nil, err
	}
	if err := validateFields(kind+" connector", schema.Fields, config); err != nil {
		return nil, err
	}
	return config, nil
}

func resolvedConnectionConfig(conn climodel.Connection, scoped map[string]any, schema filament.ConfigSchema) (map[string]any, error) {
	config := cloneConfigMap(conn.Config)
	for field, value := range scoped {
		config[field] = cloneConfigValue(value)
	}
	config = canonicalizeConfig(schema, config)
	return resolveConfigSecretReferences(schema, config)
}

func applySinkSchemaDefault(config map[string]any, spec filament.SinkSpec, sourceName string) map[string]any {
	result := cloneConfigMap(config)
	if spec.SchemaField == "" || !isEmpty(result[spec.SchemaField]) {
		return result
	}
	if value := naming.Normalize(sourceName); value != "" {
		result[spec.SchemaField] = value
	}
	return result
}

func makeRunSpec(pipelineID, sourceName, sinkName string, sourceConfig, sinkConfig map[string]any, resources []string, syncName, writeName string) (filament.RunSpec, error) {
	syncMode := filament.ModeFull
	if syncName != "" && syncName != "full" {
		return filament.RunSpec{}, fmt.Errorf("sync mode %q is not available for local runs; use full", syncName)
	}
	writeMode := filament.WriteMode(writeName)
	ingestionType, err := filament.IngestionFor(syncMode, writeMode)
	if err != nil {
		return filament.RunSpec{}, err
	}
	runID := filament.RunID("cli-" + strconv.FormatInt(time.Now().UnixNano(), 36))
	return filament.RunSpec{
		Tenant:         "local",
		Run:            runID,
		PipelineID:     pipelineID,
		Source:         filament.Ref{Connector: sourceName, Config: sourceConfig},
		Sink:           filament.Ref{Connector: sinkName, Config: sinkConfig},
		Resources:      append([]string(nil), resources...),
		IngestionTypes: map[string]filament.IngestionType{"": ingestionType},
	}, nil
}
