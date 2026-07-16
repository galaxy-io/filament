package iceberg

import (
	"context"
	"strconv"
	"strings"

	iceberg "github.com/apache/iceberg-go"
	icetable "github.com/apache/iceberg-go/table"

	"github.com/galaxy-io/filament"
)

// buildIcebergSchema maps a RecordSchema to an Iceberg schema. Field IDs are
// allocated from a single monotonic counter that also covers nested element IDs,
// so every ID is unique across the tree — iceberg-go panics on a duplicate
// (e.g. a list element colliding with a column).
func buildIcebergSchema(schema filament.RecordSchema) *iceberg.Schema {
	var nextID int
	next := func() int { nextID++; return nextID }

	fields := make([]iceberg.NestedField, len(schema.Fields))
	for i, f := range schema.Fields {
		fields[i] = iceberg.NestedField{
			ID:       next(),
			Name:     f.Name,
			Type:     logicalToIceType(f.Logical, f.Native, next),
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
// superset schema, but a narrowing change could break readers). New column IDs —
// including nested element IDs — are assigned by iceberg-go.
func evolveSchema(ctx context.Context, tbl *icetable.Table, schema filament.RecordSchema) error {
	existing := tbl.Schema()
	txn := tbl.NewTransaction()
	us := txn.UpdateSchema(false, false)
	changed := false
	for _, f := range schema.Fields {
		if _, found := existing.FindFieldByName(f.Name); found {
			continue
		}
		// nil id allocator: the create-path collision concern doesn't apply here
		// because UpdateSchema assigns fresh IDs for added (and nested) fields.
		us.AddColumn([]string{f.Name}, logicalToIceType(f.Logical, f.Native, nil), "", !f.Nullable, nil)
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

// logicalToIceType maps a portable LogicalType to an Iceberg type. native carries
// the source's own spelling (e.g. "numeric(12,2)") and refines types that need
// parameters. nextID allocates IDs for nested elements; it may be nil on the
// evolve path, where iceberg-go assigns nested IDs itself.
func logicalToIceType(l filament.LogicalType, native string, nextID func() int) iceberg.Type {
	switch l {
	case filament.LogicalBool:
		return iceberg.PrimitiveTypes.Bool
	case filament.LogicalInt16, filament.LogicalInt32:
		return iceberg.PrimitiveTypes.Int32
	case filament.LogicalInt64:
		return iceberg.PrimitiveTypes.Int64
	case filament.LogicalFloat32:
		return iceberg.PrimitiveTypes.Float32
	case filament.LogicalFloat64:
		return iceberg.PrimitiveTypes.Float64
	case filament.LogicalDecimal:
		prec, scale := decimalPrecScale(native)
		return iceberg.DecimalTypeOf(prec, scale)
	case filament.LogicalString, filament.LogicalJSON:
		return iceberg.PrimitiveTypes.String
	case filament.LogicalBytes:
		return iceberg.PrimitiveTypes.Binary
	case filament.LogicalDate:
		return iceberg.PrimitiveTypes.Date
	case filament.LogicalTime:
		return iceberg.PrimitiveTypes.Time
	case filament.LogicalTimestamp:
		return iceberg.PrimitiveTypes.Timestamp
	case filament.LogicalTimestampTZ:
		return iceberg.PrimitiveTypes.TimestampTz
	case filament.LogicalUUID:
		return iceberg.PrimitiveTypes.UUID
	case filament.LogicalArray:
		elemID := -1 // unassigned placeholder; iceberg-go fills it in on evolve
		if nextID != nil {
			elemID = nextID()
		}
		return &iceberg.ListType{ElementID: elemID, Element: iceberg.PrimitiveTypes.String, ElementRequired: false}
	default:
		return iceberg.PrimitiveTypes.String
	}
}

const (
	defaultDecimalPrec  = 38
	defaultDecimalScale = 9
)

// decimalPrecScale extracts precision and scale from a native type spelling such
// as "numeric(12,2)" or "decimal(20)". Falls back to (38,9) when absent or
// unparseable — Iceberg requires concrete values.
func decimalPrecScale(native string) (prec, scale int) {
	prec, scale = defaultDecimalPrec, defaultDecimalScale
	open := strings.IndexByte(native, '(')
	if open < 0 || !strings.HasSuffix(native, ")") {
		return prec, scale
	}
	args := native[open+1 : len(native)-1]
	parts := strings.SplitN(args, ",", 2)
	p, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || p <= 0 {
		return prec, scale // unparseable precision → keep both defaults
	}
	prec = p
	scale = 0 // SQL: an explicit precision with no scale means scale 0
	if len(parts) == 2 {
		if s, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil && s >= 0 {
			scale = s
		}
	}
	return prec, scale
}
