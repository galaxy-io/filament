package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/memory"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/inproc"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/runner"
)

type runResult struct {
	records int64
	bytes   int64
}

type runExecutor func(context.Context, filament.RunSpec) (runResult, error)

func (a *cliApp) runCommand(ctx context.Context, args []string) error {
	if len(args) == 0 || helpRequested(args) {
		return a.printRunHelp(args)
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
		spec, err = a.savedRunSpec(name, parsed.flags)
	}
	if err != nil {
		return err
	}

	execute := a.executeRun
	if execute == nil {
		execute = executeLocalRun
	}
	result, err := execute(ctx, spec)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(a.statusWriter(), "Run %s completed: %d records, %d bytes.\n", spec.Run, result.records, result.bytes)
	return err
}

func (a *cliApp) directRunSpec(flags map[string][]string) (filament.RunSpec, error) {
	sourceName := lastFlag(flags, "source-connector")
	sinkName := lastFlag(flags, "sink-connector")
	if sourceName == "" || sinkName == "" {
		return filament.RunSpec{}, fmt.Errorf("--source-connector and --sink-connector are required; example: filament run --source-connector postgres --source-dsn postgres://... --sink-connector stdout")
	}
	source, sourceOK := a.catalog.sources[sourceName]
	if !sourceOK {
		return filament.RunSpec{}, fmt.Errorf("unknown source connector %q", sourceName)
	}
	sink, sinkOK := a.catalog.sinks[sinkName]
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

func (a *cliApp) savedRunSpec(name string, flags map[string][]string) (filament.RunSpec, error) {
	for _, topologyFlag := range []string{"source", "sink", "source-connector", "sink-connector"} {
		if _, present := flags[topologyFlag]; present {
			return filament.RunSpec{}, fmt.Errorf("named pipeline %q cannot be combined with --%s", name, topologyFlag)
		}
	}

	doc, _, err := (configStore{path: a.configPath}).load()
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
	sourceConfig, err := resolvedConnectionConfig(source, p.Source.Config, a.catalog.sources[source.Type].Config)
	if err != nil {
		return filament.RunSpec{}, fmt.Errorf("source %q: %w", p.Source.Ref, err)
	}
	sinkConfig, err := resolvedConnectionConfig(sink, p.Sink.Config, a.catalog.sinks[sink.Type].Config)
	if err != nil {
		return filament.RunSpec{}, fmt.Errorf("sink %q: %w", p.Sink.Ref, err)
	}
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

	values := cloneMap(config)
	for _, field := range schema.Fields {
		if field.Default != nil {
			if _, present := values[field.Name]; !present {
				values[field.Name] = field.Default
			}
		}
	}
	for _, field := range schema.Fields {
		if field.Required && field.Default == nil && fieldVisible(field, values) && isEmpty(config[field.Name]) {
			name := prefix + strings.ReplaceAll(field.Name, "_", "-")
			return nil, fmt.Errorf("--%s is required", name)
		}
	}
	return config, nil
}

func resolvedConnectionConfig(conn connection, scoped map[string]any, schema filament.ConfigSchema) (map[string]any, error) {
	config := cloneMap(conn.Config)
	for _, field := range schema.Fields {
		if !isSecretField(field) {
			continue
		}
		value, _ := config[field.Name].(string)
		resolved, err := resolveEnvironmentReference(value)
		if err != nil {
			return nil, err
		}
		config[field.Name] = resolved
	}
	for field, value := range scoped {
		config[field] = value
	}
	return config, nil
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
		Source:         filament.Ref{Provider: sourceName, Config: sourceConfig},
		Sink:           filament.Ref{Provider: sinkName, Config: sinkConfig},
		Resources:      append([]string(nil), resources...),
		IngestionTypes: map[string]filament.IngestionType{"": ingestionType},
	}, nil
}

func executeLocalRun(ctx context.Context, spec filament.RunSpec) (runResult, error) {
	bus := inproc.New()
	defer func() { _ = bus.Close() }()

	completed, err := bus.Subscribe(events.SubjectPattern(events.RunCompleted), eventbus.SubOpts{})
	if err != nil {
		return runResult{}, err
	}
	failed, err := bus.Subscribe(events.SubjectPattern(events.RunFailed), eventbus.SubOpts{})
	if err != nil {
		return runResult{}, err
	}
	partial, err := bus.Subscribe(events.SubjectPattern(events.RunPartial), eventbus.SubOpts{})
	if err != nil {
		return runResult{}, err
	}

	runner.RunOne(ctx, runner.Deps{
		Bus:       bus,
		DataStore: memory.New(),
		Sources:   registry.DefaultSources,
		Sinks:     registry.DefaultSinks,
	}, spec)

	select {
	case message := <-completed.C():
		fact, decodeErr := events.Decode(message)
		_ = message.Ack()
		if decodeErr != nil {
			return runResult{}, decodeErr
		}
		result := fact.Data.(events.RunCompletedEvent)
		return runResult{records: result.Records, bytes: result.Bytes}, nil
	case message := <-failed.C():
		fact, decodeErr := events.Decode(message)
		_ = message.Ack()
		if decodeErr != nil {
			return runResult{}, decodeErr
		}
		return runResult{}, fmt.Errorf("run failed: %s", fact.Data.(events.RunFailedEvent).Error)
	case message := <-partial.C():
		fact, decodeErr := events.Decode(message)
		_ = message.Ack()
		if decodeErr != nil {
			return runResult{}, decodeErr
		}
		return runResult{}, fmt.Errorf("run partial: %s", fact.Data.(events.RunPartialEvent).Error)
	case <-ctx.Done():
		return runResult{}, ctx.Err()
	}
}
