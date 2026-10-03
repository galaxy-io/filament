package zerobus

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
)

// EnsureSchema builds the Arrow schema for a resource and opens its Zerobus
// stream. Zerobus validates the Arrow schema strictly against the destination
// Delta table, so the table must already exist (or be created here when
// create_table is enabled) with a matching column layout before the stream
// opens — Zerobus binds the table name into the stream's authorization at
// creation time.
func (s *Sink) EnsureSchema(ctx context.Context, resource string, schema filament.RecordSchema) error {
	if s.client == nil {
		return fmt.Errorf("databrickszerobus sink: ensure schema before open")
	}
	table := s.qualify(resource)
	if s.create {
		if err := s.createTable(ctx, resource, schema); err != nil {
			return err
		}
	}
	arrowSchema := conformSchema(arrowbatch.Schema(schema))
	ipc, err := schemaIPC(arrowSchema)
	if err != nil {
		return fmt.Errorf("databrickszerobus sink: schema ipc for %q: %w", resource, err)
	}
	stream, err := s.client.OpenStream(ctx, table, ipc)
	if err != nil {
		return fmt.Errorf("databrickszerobus sink: open stream for %s: %w", table, err)
	}
	s.streams[resource] = stream
	s.schemas[resource] = arrowSchema
	return nil
}

// qualify renders the fully qualified Unity Catalog table name for a resource.
func (s *Sink) qualify(resource string) string {
	return s.catalog + "." + s.schema + "." + resource
}
