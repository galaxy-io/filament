package runner

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/transform"
)

// resourceError names the resource a pre-extraction failure belongs to, so the
// run can report that one resource as the cause and the rest as aborted.
type resourceError struct {
	resource string
	err      error
}

func (e *resourceError) Error() string { return e.err.Error() }
func (e *resourceError) Unwrap() error { return e.err }

// ensureSchemas drives a Schematized sink's DDL from a SchemaProvider source for
// each named resource. It is a no-op when the sink isn't schema-aware. A
// schema-aware sink paired with a source that can't supply schemas is a
// misconfiguration and fails the run before any data moves. A transform is
// applied to the source schema before the CDC and audit shaping, in the same
// order the pipeline inlet applies it to rows.
func ensureSchemas(ctx context.Context, src filament.Source, snk filament.Sink, spec filament.RunSpec, def *transform.Definition) error {
	sch, ok := snk.(filament.Schematized)
	if !ok {
		return nil
	}
	prov, ok := src.(filament.SchemaProvider)
	if !ok {
		return fmt.Errorf("sink %q requires a schema but source %q provides none", spec.Sink.Connector, spec.Source.Connector)
	}
	for _, res := range spec.Resources {
		if err := ensureSchema(ctx, prov, sch, spec, def, res); err != nil {
			return &resourceError{resource: res, err: err}
		}
	}
	return nil
}

func ensureSchema(ctx context.Context, prov filament.SchemaProvider, sch filament.Schematized, spec filament.RunSpec, def *transform.Definition, res string) error {
	schema, err := prov.Schema(ctx, res)
	if err != nil {
		return fmt.Errorf("schema for %q: %w", res, err)
	}
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
	schema, err = rowmodel.WithAuditFields(schema, filament.IsCDCIngestion(ingestionType))
	if err != nil {
		return fmt.Errorf("schema for %q: %w", res, err)
	}
	if err := sch.EnsureSchema(ctx, res, schema); err != nil {
		return fmt.Errorf("ensure schema for %q: %w", res, err)
	}
	return nil
}
