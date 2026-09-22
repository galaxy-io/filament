package runner

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
)

// ensureSchemas drives a Schematized sink's DDL from a SchemaProvider source for
// each named resource. It is a no-op when the sink isn't schema-aware. A
// schema-aware sink paired with a source that can't supply schemas is a
// misconfiguration and fails the run before any data moves.
func ensureSchemas(ctx context.Context, snk filament.Sink, spec filament.RunSpec, schemas map[string]rowmodel.Schema) error {
	sch, ok := snk.(filament.Schematized)
	if !ok {
		return nil
	}
	for _, res := range spec.Resources {
		schema := schemas[res]
		var err error
		ingestionType := filament.TypeFor(spec.IngestionTypes, res)
		if ingestionType == filament.IngestionCDCAppend {
			schema = rowmodel.AsCDCAppendHistory(schema)
		}
		schema, err = rowmodel.WithAuditFields(schema, filament.IsCDCIngestion(ingestionType))
		if err != nil {
			return fmt.Errorf("schema for %q: %w", res, err)
		}
		destination, schema := destinationSchema(res, schema, spec.WritePolicies)
		if err := sch.EnsureSchema(ctx, destination, schema); err != nil {
			return fmt.Errorf("ensure schema for %q: %w", res, err)
		}
	}
	return nil
}
