package rowmodel

// Filament reserves this namespace for row lineage materialized after source
// fields. Audit fields remain nullable so existing destination tables can
// evolve without rewriting historical rows.
const (
	AuditRunIDField          = "_filament_run_id"
	AuditRunStartedAtField   = "_filament_run_started_at"
	AuditOperationField      = "_filament_operation"
	AuditSourcePositionField = "_filament_source_position"
	AuditSequenceField       = "_filament_sequence"
)

// WithAuditFields returns an independently owned destination schema with
// Filament's universal lineage fields and, for CDC, change-stream fields.
func WithAuditFields(schema Schema, cdc bool) (Schema, error) {
	out := schema.Clone()
	if err := ValidateReservedFields(out); err != nil {
		return Schema{}, err
	}
	out.Fields = append(out.Fields,
		Field{Name: AuditRunIDField, Logical: LogicalString, Nullable: true},
		Field{Name: AuditRunStartedAtField, Logical: LogicalTimestampTZ, Nullable: true},
	)
	if cdc {
		out.Fields = append(out.Fields,
			Field{Name: AuditOperationField, Logical: LogicalString, Nullable: true},
			Field{Name: AuditSourcePositionField, Logical: LogicalString, Nullable: true},
			Field{Name: AuditSequenceField, Logical: LogicalInt64, Nullable: true},
		)
	}
	return out, nil
}

// AsCDCAppendHistory returns the sink schema used to retain every CDC event.
// Source keys may repeat and deletes may contain only key values, so the
// destination has no primary key and every source field is nullable.
func AsCDCAppendHistory(schema Schema) Schema {
	out := schema.Clone()
	out.PrimaryKey = nil
	for i := range out.Fields {
		out.Fields[i].Nullable = true
	}
	return out
}
