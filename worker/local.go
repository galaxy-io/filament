// Package worker answers connector questions for the control services. Local
// serves them from in-process registries; the same contract is served over the
// wire by a persistent worker.
package worker

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/galaxy-io/filament"
)

// Local answers from the given registries in process.
func Local(sources filament.SourceRegistry, sinks filament.SinkRegistry) filament.Worker {
	return local{sources: sources, sinks: sinks}
}

type local struct {
	sources filament.SourceRegistry
	sinks   filament.SinkRegistry
}

func (l local) Describe(context.Context) (filament.Catalog, error) {
	return filament.Catalog{Sources: l.sources.Specs(), Sinks: l.sinks.Specs()}, nil
}

func (l local) SourceSpec(_ context.Context, name string) (filament.ConnectorSpec, error) {
	spec, err := l.sources.Spec(name)
	if err != nil {
		return filament.ConnectorSpec{}, notFound(err)
	}
	return spec, nil
}

func (l local) SinkSpec(_ context.Context, name string) (filament.SinkSpec, error) {
	spec, err := l.sinks.Spec(name)
	if err != nil {
		return filament.SinkSpec{}, notFound(err)
	}
	return spec, nil
}

func (l local) Validate(_ context.Context, ref filament.ConnectorRef, cfg filament.Config) error {
	switch ref.Kind {
	case filament.ConnectorKindSource:
		source, err := l.source(ref.Name)
		if err != nil {
			return err
		}
		return rejected(source.Validate(cfg))
	case filament.ConnectorKindSink:
		sink, err := l.sink(ref.Name)
		if err != nil {
			return err
		}
		if validator, ok := sink.(filament.ConfigValidatable); ok {
			return rejected(validator.Validate(cfg))
		}
		return nil
	default:
		return errors.New("connector kind is required")
	}
}

func (l local) TestConnection(ctx context.Context, ref filament.ConnectorRef, cfg filament.Config) error {
	var connector any
	switch ref.Kind {
	case filament.ConnectorKindSource:
		source, err := l.source(ref.Name)
		if err != nil {
			return err
		}
		connector = source
	case filament.ConnectorKindSink:
		sink, err := l.sink(ref.Name)
		if err != nil {
			return err
		}
		connector = sink
	default:
		return errors.New("connector kind is required")
	}
	live, ok := connector.(filament.LiveValidatable)
	if !ok {
		return filament.ErrUnsupported
	}
	return rejected(live.TestConnection(ctx, cfg))
}

func (l local) Discover(ctx context.Context, name string, cfg filament.Config, opts filament.DiscoverOpts) ([]filament.Resource, error) {
	source, teardown, err := l.configure(ctx, name, cfg)
	if err != nil {
		return nil, err
	}
	defer teardown()
	discoverable, ok := source.(filament.Discoverable)
	if !ok {
		return nil, filament.ErrUnsupported
	}
	result, err := discoverable.Discover(ctx, opts)
	if err != nil {
		return nil, err
	}
	return result.Resources, nil
}

func (l local) Inspect(ctx context.Context, name string, cfg filament.Config, resources []string) ([]filament.Inspection, error) {
	source, teardown, err := l.configure(ctx, name, cfg)
	if err != nil {
		return nil, err
	}
	defer teardown()
	// discover lists the source's resources once; nil when it cannot.
	var discover func() ([]filament.Resource, error)
	if discoverable, ok := source.(filament.Discoverable); ok {
		var discovered []filament.Resource
		discover = func() ([]filament.Resource, error) {
			if discovered != nil {
				return discovered, nil
			}
			result, err := discoverable.Discover(ctx, filament.DiscoverOpts{})
			if err != nil {
				return nil, fmt.Errorf("discover resources: %w", err)
			}
			discovered = result.Resources
			return discovered, nil
		}
	}
	if len(resources) == 0 {
		if discover == nil {
			return nil, filament.ErrUnsupported
		}
		all, err := discover()
		if err != nil {
			return nil, err
		}
		for _, resource := range all {
			if resource.Selectable {
				resources = append(resources, resource.Name)
			}
		}
	}
	out := make([]filament.Inspection, 0, len(resources))
	for _, resource := range resources {
		out = append(out, inspectResource(ctx, source, resource, discover))
	}
	return out, nil
}

// inspectResource reports one resource from an already configured source. The
// primary key comes from the schema when the source has one, else from
// discovery. The schema supplies the columns, because a source may rank only
// the cursor it supports; ranked cursor columns are laid over them.
func inspectResource(ctx context.Context, source filament.Source, resource string, discover func() ([]filament.Resource, error)) filament.Inspection {
	inspection := filament.Inspection{Name: resource}
	if managed, ok := source.(filament.ManagedIncrementalSource); ok {
		inspection.ManagedIncremental = managed.ManagedIncremental(resource)
	}
	failed := func(err error) filament.Inspection {
		inspection.Status, inspection.Message = filament.InspectFailed, err.Error()
		return inspection
	}
	schemas, hasSchema := source.(filament.SchemaProvider)
	if hasSchema {
		schema, err := schemas.Schema(ctx, resource)
		if err != nil {
			return failed(fmt.Errorf("schema for %q: %w", resource, err))
		}
		schema.Resource = resource
		inspection.Schema, inspection.PrimaryKey = &schema, schema.PrimaryKey
	} else if discover != nil {
		all, err := discover()
		if err != nil {
			return failed(err)
		}
		for _, candidate := range all {
			if candidate.Name == resource {
				inspection.PrimaryKey = candidate.PrimaryKey
				break
			}
		}
	}
	var ranked []filament.CursorColumn
	if cursors, ok := source.(filament.CursorColumnProvider); ok {
		columns, err := cursors.CursorColumns(ctx, resource)
		if err != nil {
			return failed(fmt.Errorf("resource columns %q: %w", resource, err))
		}
		ranked, inspection.Ranked = columns, true
	}
	if !hasSchema {
		if !inspection.Ranked {
			inspection.Status = filament.InspectUnsupported
		}
		inspection.Columns = ranked
		return inspection
	}
	for _, field := range inspection.Schema.Fields {
		column := filament.CursorColumn{SchemaField: field}
		if i := slices.IndexFunc(ranked, func(c filament.CursorColumn) bool { return c.Name == field.Name }); i >= 0 {
			column = ranked[i]
			ranked = slices.Delete(ranked, i, i+1)
		}
		column.PrimaryKey = column.PrimaryKey || slices.Contains(inspection.Schema.PrimaryKey, field.Name)
		inspection.Columns = append(inspection.Columns, column)
	}
	inspection.Columns = append(inspection.Columns, ranked...)
	return inspection
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

// configure resolves and configures a fresh source; the returned teardown
// releases it.
func (l local) configure(ctx context.Context, name string, cfg filament.Config) (filament.Source, func(), error) {
	source, err := l.source(name)
	if err != nil {
		return nil, nil, err
	}
	if err := source.Configure(ctx, cfg); err != nil {
		return nil, nil, rejected(err)
	}
	return source, func() { _ = source.Teardown(ctx) }, nil
}

func (l local) source(name string) (filament.Source, error) {
	source, err := l.sources.Resolve(name)
	if err != nil {
		return nil, notFound(err)
	}
	return source, nil
}

func (l local) sink(name string) (filament.Sink, error) {
	sink, err := l.sinks.Resolve(name)
	if err != nil {
		return nil, notFound(err)
	}
	return sink, nil
}

// notFound marks a registry miss with the shared sentinel, keeping its text.
func notFound(err error) error {
	return fmt.Errorf("%w: %w", filament.ErrNotFound, err)
}

// rejected marks a connector's own verdict on its config; nil stays nil.
func rejected(err error) error {
	if err == nil {
		return nil
	}
	return &filament.ConfigureError{Reason: err}
}
