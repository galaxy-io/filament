package runner

import (
	"context"
	"fmt"
	"maps"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/transform"
)

// resourceError names the resource whose preparation failed a run.
type resourceError struct {
	resource string
	err      error
}

func (e *resourceError) Error() string { return e.err.Error() }
func (e *resourceError) Unwrap() error { return e.err }

// ensureSchemas drives a Schematized sink's DDL from the discovered source
// schemas for each named resource. It is a no-op when the sink isn't
// schema-aware. A transform is applied to the source schema before the CDC and
// audit shaping, so the DDL matches what the pipeline writes.
func ensureSchemas(ctx context.Context, snk filament.Sink, spec filament.RunSpec, schemas map[string]rowmodel.Schema, def *transform.Definition) error {
	sch, ok := snk.(filament.Schematized)
	if !ok {
		return nil
	}
	for _, res := range spec.Resources {
		if err := ensureSchema(ctx, sch, spec, schemas[res], def, res); err != nil {
			return &resourceError{resource: res, err: err}
		}
	}
	return nil
}

func ensureSchema(ctx context.Context, sch filament.Schematized, spec filament.RunSpec, schema rowmodel.Schema, def *transform.Definition, res string) error {
	if def != nil {
		if schema.Resource == "" {
			schema.Resource = res
		}
		plan, err := transform.Compile(def, schema)
		if err != nil {
			return fmt.Errorf("transform for %q: %w", res, err)
		}
		schema = plan.Schema()
	}
	ingestionType := filament.TypeFor(spec.IngestionTypes, res)
	if ingestionType == filament.IngestionCDCAppend {
		schema = rowmodel.AsCDCAppendHistory(schema)
	}
	schema, err := rowmodel.WithAuditFields(schema, filament.IsCDCIngestion(ingestionType))
	if err != nil {
		return fmt.Errorf("schema for %q: %w", res, err)
	}
	destination, schema := destinationSchema(res, schema, spec.WritePolicies)
	if err := sch.EnsureSchema(ctx, destination, schema); err != nil {
		return fmt.Errorf("ensure schema for %q: %w", res, err)
	}
	return nil
}

// loadSinkSchemas fetches metadata only for sinks that require typed DDL.
// Reuse supplied schemas, and keep naming independent of metadata discovery.
func loadSinkSchemas(ctx context.Context, src filament.Source, snk filament.Sink, resources []string, schemas map[string]rowmodel.Schema) (map[string]rowmodel.Schema, error) {
	if _, required := snk.(filament.Schematized); !required {
		return schemas, nil
	}
	out := maps.Clone(schemas)
	if out == nil {
		out = make(map[string]rowmodel.Schema, len(resources))
	}
	provider, available := src.(filament.SchemaProvider)
	for _, resource := range resources {
		if _, loaded := out[resource]; loaded {
			continue
		}
		if !available {
			return nil, fmt.Errorf("source %q must provide resource schemas", src.Spec().Name)
		}
		schema, err := provider.Schema(ctx, resource)
		if err != nil {
			return nil, fmt.Errorf("schema for %q: %w", resource, err)
		}
		out[resource] = schema
	}
	return out, nil
}
