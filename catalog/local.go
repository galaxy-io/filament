// Package catalog implements filament.Catalog. Local answers from the
// in-process registries and is what standalone, the CLI and the connector
// host use.
package catalog

import (
	"context"
	"errors"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/registry"
)

type local struct {
	sources filament.SourceRegistry
	sinks   filament.SinkRegistry
}

// Local returns a Catalog backed by the given registries.
func Local(sources filament.SourceRegistry, sinks filament.SinkRegistry) filament.Catalog {
	return local{sources: sources, sinks: sinks}
}

func (l local) SourceSpecs(context.Context) ([]filament.ConnectorSpec, error) {
	return l.sources.Specs(), nil
}

func (l local) SinkSpecs(context.Context) ([]filament.SinkSpec, error) { return l.sinks.Specs(), nil }

func (l local) SourceSpec(_ context.Context, name string) (filament.ConnectorSpec, error) {
	if specs, ok := l.sources.(interface {
		Spec(string) (filament.ConnectorSpec, error)
	}); ok {
		spec, err := specs.Spec(name)
		return spec, unknown(err)
	}
	source, err := l.source(name)
	if err != nil {
		return filament.ConnectorSpec{}, err
	}
	return source.Spec(), nil
}

func (l local) SinkSpec(_ context.Context, name string) (filament.SinkSpec, error) {
	if specs, ok := l.sinks.(interface {
		Spec(string) (filament.SinkSpec, error)
	}); ok {
		spec, err := specs.Spec(name)
		return spec, unknown(err)
	}
	sink, err := l.sink(name)
	if err != nil {
		return filament.SinkSpec{}, err
	}
	return sink.Spec(), nil
}

func (l local) PlanReplicationStream(_ context.Context, name string, req filament.ReplicationStreamPlanningRequest) (filament.ReplicationStreamPlan, error) {
	source, err := l.source(name)
	if err != nil {
		return filament.ReplicationStreamPlan{}, err
	}
	planner, ok := source.(filament.ReplicationStreamPlanner)
	if !ok {
		return filament.ReplicationStreamPlan{}, filament.ErrUnsupported
	}
	return planner.PlanReplicationStream(req)
}

func (l local) SourceContracts(_ context.Context, name string) (filament.SourceContracts, error) {
	source, err := l.source(name)
	if err != nil {
		return filament.SourceContracts{}, err
	}
	return filament.SourceContractsOf(source), nil
}

func (l local) SinkContracts(_ context.Context, name string) (filament.SinkContracts, error) {
	sink, err := l.sink(name)
	if err != nil {
		return filament.SinkContracts{}, err
	}
	return filament.SinkContractsOf(sink), nil
}

func (l local) Validate(_ context.Context, kind filament.ConnectorKind, name string, cfg filament.Config) error {
	switch kind {
	case filament.ConnectorKindSource:
		source, err := l.source(name)
		if err != nil {
			return err
		}
		return source.Validate(cfg)
	case filament.ConnectorKindSink:
		sink, err := l.sink(name)
		if err != nil {
			return err
		}
		if validator, ok := sink.(filament.ConfigValidatable); ok {
			return validator.Validate(cfg)
		}
		return nil
	default:
		return errors.New("connector kind is required")
	}
}

func (l local) TestConnection(ctx context.Context, kind filament.ConnectorKind, name string, cfg filament.Config) error {
	var connector any
	var err error
	switch kind {
	case filament.ConnectorKindSource:
		connector, err = l.source(name)
	case filament.ConnectorKindSink:
		connector, err = l.sink(name)
	default:
		return errors.New("connector kind is required")
	}
	if err != nil {
		return err
	}
	live, ok := connector.(filament.LiveValidatable)
	if !ok {
		return nil
	}
	return live.TestConnection(ctx, cfg)
}

func (l local) Discover(ctx context.Context, name string, cfg filament.Config, opts filament.DiscoverOpts) (filament.DiscoverResult, error) {
	source, teardown, err := l.configure(ctx, name, cfg)
	if err != nil {
		return filament.DiscoverResult{}, err
	}
	defer teardown()
	discoverable, ok := source.(filament.Discoverable)
	if !ok {
		return filament.DiscoverResult{}, fmt.Errorf("%w: %q does not support discovery", filament.ErrUnsupported, name)
	}
	return discoverable.Discover(ctx, opts)
}

func (l local) Inspect(ctx context.Context, name string, cfg filament.Config, resources []string) ([]filament.ResourceInspection, error) {
	source, teardown, err := l.configure(ctx, name, cfg)
	if err != nil {
		return nil, err
	}
	defer teardown()

	// A discovered key is authoritative for sources without a schema, and
	// saves one discovery per resource.
	discoveredKeys := map[string][]string{}
	if len(resources) == 0 {
		discoverable, ok := source.(filament.Discoverable)
		if !ok {
			return nil, fmt.Errorf("%w: %q does not support discovery", filament.ErrUnsupported, name)
		}
		result, err := discoverable.Discover(ctx, filament.DiscoverOpts{})
		if err != nil {
			return nil, err
		}
		for _, resource := range result.Resources {
			if resource.Selectable {
				resources = append(resources, resource.Name)
				discoveredKeys[resource.Name] = resource.PrimaryKey
			}
		}
	}

	schemas, hasSchema := source.(filament.SchemaProvider)
	cursors, hasCursors := source.(filament.CursorColumnProvider)
	out := make([]filament.ResourceInspection, 0, len(resources))
	for _, resource := range resources {
		inspection := filament.ResourceInspection{Name: resource}
		if key, ok := discoveredKeys[resource]; ok && !hasSchema {
			inspection.PrimaryKey = key
		} else {
			inspection.PrimaryKey, inspection.PrimaryKeyErr = filament.PrimaryKeyForResource(ctx, source, resource)
		}
		switch {
		case hasCursors:
			inspection.Ranked = true
			inspection.Columns, inspection.ColumnsErr = cursors.CursorColumns(ctx, resource)
		case hasSchema:
			var schema filament.RecordSchema
			schema, inspection.ColumnsErr = schemas.Schema(ctx, resource)
			for _, field := range schema.Fields {
				inspection.Columns = append(inspection.Columns, filament.CursorColumn{SchemaField: field})
			}
		default:
			inspection.ColumnsErr = fmt.Errorf("%w: %q does not provide resource columns", filament.ErrUnsupported, name)
		}
		if managed, ok := source.(filament.ManagedIncrementalSource); ok {
			inspection.ManagedIncremental = managed.ManagedIncremental(resource)
		}
		out = append(out, inspection)
	}
	return out, nil
}

// configure resolves and configures a source; the returned teardown releases
// it. A refused config is reported as ErrConfigure.
func (l local) configure(ctx context.Context, name string, cfg filament.Config) (filament.Source, func(), error) {
	source, err := l.source(name)
	if err != nil {
		return nil, nil, err
	}
	if err := source.Configure(ctx, cfg); err != nil {
		return nil, nil, fmt.Errorf("%w: %w", filament.ErrConfigure, err)
	}
	return source, func() { _ = source.Teardown(ctx) }, nil
}

func (l local) source(name string) (filament.Source, error) {
	source, err := l.sources.Resolve(name)
	return source, unknown(err)
}

func (l local) sink(name string) (filament.Sink, error) {
	sink, err := l.sinks.Resolve(name)
	return sink, unknown(err)
}

// unknown reports an unregistered connector as ErrNotFound so callers need not
// know the registry.
func unknown(err error) error {
	if errors.Is(err, registry.ErrUnknownProvider) {
		return fmt.Errorf("%w: %w", filament.ErrNotFound, err)
	}
	return err
}
