package iceberg

import (
	"context"
	"errors"
	"fmt"

	iceberg "github.com/apache/iceberg-go"
	"github.com/apache/iceberg-go/catalog"
	icetable "github.com/apache/iceberg-go/table"

	"github.com/galaxy-io/filament/rowmodel"
)

// buildIcebergSchema maps a RecordSchema to an Iceberg schema, allocating field
// IDs from one monotonic counter.
func buildIcebergSchema(schema rowmodel.Schema) *iceberg.Schema {
	var nextID int
	next := func() int { nextID++; return nextID }

	fields := make([]iceberg.NestedField, len(schema.Fields))
	for i, f := range schema.Fields {
		fields[i] = iceberg.NestedField{
			ID:       next(),
			Name:     f.Name,
			Type:     logicalToIceType(f),
			Required: !f.Nullable,
		}
	}

	if len(schema.PrimaryKey) > 0 {
		nameToID := make(map[string]int, len(fields))
		for _, f := range fields {
			nameToID[f.Name] = f.ID
		}
		ids := make([]int, 0, len(schema.PrimaryKey))
		for _, k := range schema.PrimaryKey {
			if id, ok := nameToID[k]; ok {
				ids = append(ids, id)
			}
		}
		return iceberg.NewSchemaWithIdentifiers(0, ids, fields...)
	}
	return iceberg.NewSchema(0, fields...)
}

// evolveSchema adds any source columns not yet present on the table. It is
// add-only: type, nullability, and ordering changes on existing columns and
// dropped columns are intentionally NOT applied (an Iceberg promote tolerates a
// superset schema, but a narrowing change could break readers). New column IDs
// are assigned by iceberg-go.
func evolveSchema(ctx context.Context, tbl *icetable.Table, schema rowmodel.Schema) error {
	existing := tbl.Schema()
	txn := tbl.NewTransaction()
	us := txn.UpdateSchema(false, false)
	changed := false
	for _, f := range schema.Fields {
		if _, found := existing.FindFieldByName(f.Name); found {
			continue
		}
		us.AddColumn([]string{f.Name}, logicalToIceType(f), "", !f.Nullable, nil)
		changed = true
	}
	if !changed {
		return nil
	}
	if err := us.Commit(); err != nil {
		return err
	}
	_, err := txn.Commit(ctx)
	return err
}

// EnsureSchema creates the Iceberg table if absent, or evolves it by adding any
// columns not yet present.
func (s *Sink) EnsureSchema(ctx context.Context, resource string, schema rowmodel.Schema) error {
	s.mu.Lock()
	cat := s.cat
	s.mu.Unlock()
	if cat == nil {
		return fmt.Errorf("iceberg sink: EnsureSchema called before Open")
	}

	// Create the namespace first: a strict REST catalog rejects CreateTable with
	// NoSuchNamespace otherwise. Idempotent — an existing namespace is fine.
	nsIdent := catalog.ToIdentifier(splitNamespace(s.namespace)...)
	if err := cat.CreateNamespace(ctx, nsIdent, nil); err != nil &&
		!errors.Is(err, catalog.ErrNamespaceAlreadyExists) {
		return fmt.Errorf("iceberg sink: create namespace %s: %w", s.namespace, err)
	}

	iceSchema := buildIcebergSchema(schema)
	ident := s.tableIdent(resource)
	createOpts := []catalog.CreateTableOpt{
		catalog.WithProperties(iceberg.Properties{"write.format.default": "parquet"}),
	}
	if s.tableLocationRoot != "" {
		location := joinURI(s.tableLocationRoot, namespacePath(s.namespace), tableName(resource))
		createOpts = append(createOpts, catalog.WithLocation(location))
	}
	tbl, err := cat.CreateTable(ctx, ident, iceSchema, createOpts...)
	if errors.Is(err, catalog.ErrTableAlreadyExists) {
		tbl, err = cat.LoadTable(ctx, ident)
		if err != nil {
			return fmt.Errorf("iceberg sink: load table %s: %w", resource, err)
		}
		if err := evolveSchema(ctx, tbl, schema); err != nil {
			return fmt.Errorf("iceberg sink: evolve schema %s: %w", resource, err)
		}
		tbl, err = cat.LoadTable(ctx, ident)
		if err != nil {
			return fmt.Errorf("iceberg sink: reload table %s: %w", resource, err)
		}
	} else if err != nil {
		return fmt.Errorf("iceberg sink: create table %s: %w", resource, err)
	}

	s.mu.Lock()
	s.tables[resource] = &iceTable{
		tbl:        tbl,
		schema:     tbl.Schema(),
		primaryKey: append([]string(nil), schema.PrimaryKey...),
	}
	s.writeModes[resource] = s.writeModeForResource(resource)
	s.mu.Unlock()
	return nil
}
