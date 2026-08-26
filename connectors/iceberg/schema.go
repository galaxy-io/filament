package iceberg

import (
	"context"

	iceberg "github.com/apache/iceberg-go"
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
