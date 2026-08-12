package runner

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament"
)

// ensureSchemas drives a Schematized sink's DDL from a SchemaProvider source for
// each named resource. It is a no-op when the sink isn't schema-aware. A
// schema-aware sink paired with a source that can't supply schemas is a
// misconfiguration and fails the run before any data moves.
func ensureSchemas(ctx context.Context, src filament.Source, snk filament.Sink, spec filament.RunSpec) error {
	sch, ok := snk.(filament.Schematized)
	if !ok {
		return nil
	}
	prov, ok := src.(filament.SchemaProvider)
	if !ok {
		return fmt.Errorf("sink %q requires a schema but source %q provides none", spec.Sink.Provider, spec.Source.Provider)
	}
	for _, res := range spec.Resources {
		schema, err := prov.Schema(ctx, res)
		if err != nil {
			return fmt.Errorf("schema for %q: %w", res, err)
		}
		if err := sch.EnsureSchema(ctx, res, schema); err != nil {
			return fmt.Errorf("ensure schema for %q: %w", res, err)
		}
	}
	return nil
}
