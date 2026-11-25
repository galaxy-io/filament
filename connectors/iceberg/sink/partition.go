package iceberg

import (
	"fmt"
	"strings"

	iceberg "github.com/apache/iceberg-go"

	"github.com/galaxy-io/filament"
)

// partitionField is one partition_by entry: a source column and its transform.
type partitionField struct {
	column    string
	transform iceberg.Transform
}

// partitionBy reads partition_by from a resource config. Each entry is a
// column, partitioned by identity, or transform(column) with an Iceberg
// transform name such as day or bucket[16].
func partitionBy(cfg filament.Config) ([]partitionField, error) {
	var entries []string
	switch raw := cfg.Raw()["partition_by"].(type) {
	case nil:
		return nil, nil
	case []string:
		entries = raw
	case []any:
		for _, entry := range raw {
			text, ok := entry.(string)
			if !ok {
				return nil, fmt.Errorf("partition_by entry %v is not a string", entry)
			}
			entries = append(entries, text)
		}
	default:
		return nil, fmt.Errorf("partition_by must be a list, got %T", raw)
	}
	fields := make([]partitionField, 0, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		field := partitionField{column: entry, transform: iceberg.IdentityTransform{}}
		if name, column, ok := strings.Cut(strings.TrimSuffix(entry, ")"), "("); ok && strings.HasSuffix(entry, ")") {
			transform, err := iceberg.ParseTransform(strings.TrimSpace(name))
			if err != nil {
				return nil, fmt.Errorf("partition_by entry %q: %w", entry, err)
			}
			if _, unknown := transform.(iceberg.UnknownTransform); unknown {
				return nil, fmt.Errorf("partition_by entry %q: unknown transform %q", entry, name)
			}
			field = partitionField{column: strings.TrimSpace(column), transform: transform}
		}
		if field.column == "" || strings.ContainsAny(field.column, "()") {
			return nil, fmt.Errorf("partition_by entry %q: want a column or transform(column)", entry)
		}
		fields = append(fields, field)
	}
	return fields, nil
}

// partitionSpec binds fields to the table schema. A field is named after its
// column and transform, as Iceberg engines name them.
func partitionSpec(schema *iceberg.Schema, fields []partitionField) (iceberg.PartitionSpec, error) {
	opts := make([]iceberg.PartitionOption, len(fields))
	for i, field := range fields {
		name := field.column
		switch field.transform.(type) {
		case iceberg.IdentityTransform:
		case iceberg.BucketTransform:
			name += "_bucket"
		case iceberg.TruncateTransform:
			name += "_trunc"
		default:
			name += "_" + field.transform.String()
		}
		opts[i] = iceberg.AddPartitionFieldByName(field.column, name, field.transform, schema, nil)
	}
	return iceberg.NewPartitionSpecOpts(opts...)
}
