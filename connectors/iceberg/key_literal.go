package iceberg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	iceberg "github.com/apache/iceberg-go"
	"github.com/google/uuid"

	ingestion "github.com/galaxy-io/filament"
)

type keyTuple struct {
	values map[string]any
}

func (k keyTuple) key() string {
	var b strings.Builder
	for _, name := range sortedKeys(k.values) {
		if b.Len() > 0 {
			b.WriteByte('\x1f')
		}
		b.WriteString(name)
		b.WriteByte('=')
		enc, _ := json.Marshal(k.values[name])
		b.Write(enc)
	}
	return b.String()
}

func extractKeyTuple(data json.RawMessage, it *iceTable, keys []string) (keyTuple, error) {
	var row map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&row); err != nil {
		return keyTuple{}, fmt.Errorf("parse key row: %w", err)
	}
	values := make(map[string]any, len(keys))
	for _, key := range keys {
		value, ok := row[key]
		if !ok || value == nil {
			return keyTuple{}, fmt.Errorf("primary key %q missing from record", key)
		}
		lit, err := literalValue(it, key, value)
		if err != nil {
			return keyTuple{}, err
		}
		values[key] = lit
	}
	return keyTuple{values: values}, nil
}

func keyFilter(_ *iceTable, keys []string, tuples []keyTuple) (iceberg.BooleanExpression, error) {
	var out iceberg.BooleanExpression
	for _, tuple := range tuples {
		var row iceberg.BooleanExpression
		for _, key := range keys {
			expr, err := equalExpr(key, tuple.values[key])
			if err != nil {
				return nil, err
			}
			if row == nil {
				row = expr
			} else {
				row = iceberg.NewAnd(row, expr)
			}
		}
		if row == nil {
			return nil, fmt.Errorf("empty primary key for iceberg table")
		}
		if out == nil {
			out = row
		} else {
			out = iceberg.NewOr(out, row)
		}
	}
	if out == nil {
		return nil, fmt.Errorf("empty mutation filter for iceberg table")
	}
	return out, nil
}

func equalExpr(key string, value any) (iceberg.BooleanExpression, error) {
	ref := iceberg.Reference(key)
	switch v := value.(type) {
	case bool:
		return iceberg.EqualTo(ref, v), nil
	case int32:
		return iceberg.EqualTo(ref, v), nil
	case int64:
		return iceberg.EqualTo(ref, v), nil
	case float32:
		return iceberg.EqualTo(ref, v), nil
	case float64:
		return iceberg.EqualTo(ref, v), nil
	case string:
		return iceberg.EqualTo(ref, v), nil
	case []byte:
		return iceberg.EqualTo(ref, v), nil
	case uuid.UUID:
		return iceberg.EqualTo(ref, v), nil
	default:
		return nil, fmt.Errorf("unsupported iceberg key literal %T for %q", value, key)
	}
}

func literalValue(it *iceTable, key string, value any) (any, error) {
	field, ok := fieldByName(it.record, key)
	if !ok {
		return nil, fmt.Errorf("primary key %q is not in iceberg schema", key)
	}
	switch field.Logical {
	case ingestion.LogicalBool:
		v, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("primary key %q: expected bool, got %T", key, value)
		}
		return v, nil
	case ingestion.LogicalInt16, ingestion.LogicalInt32:
		n, err := jsonNumber(value)
		if err != nil {
			return nil, fmt.Errorf("primary key %q: %w", key, err)
		}
		if n < minInt32 || n > maxInt32 {
			return nil, fmt.Errorf("primary key %q: integer %d overflows int32", key, n)
		}
		return int32(n), nil
	case ingestion.LogicalInt64:
		n, err := jsonNumber(value)
		if err != nil {
			return nil, fmt.Errorf("primary key %q: %w", key, err)
		}
		return n, nil
	case ingestion.LogicalFloat32:
		n, err := jsonFloat(value)
		if err != nil {
			return nil, fmt.Errorf("primary key %q: %w", key, err)
		}
		return float32(n), nil
	case ingestion.LogicalFloat64:
		n, err := jsonFloat(value)
		if err != nil {
			return nil, fmt.Errorf("primary key %q: %w", key, err)
		}
		return n, nil
	case ingestion.LogicalBytes:
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("primary key %q: expected base64/string bytes, got %T", key, value)
		}
		return []byte(s), nil
	case ingestion.LogicalUUID:
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("primary key %q: expected uuid string, got %T", key, value)
		}
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, fmt.Errorf("primary key %q: parse uuid: %w", key, err)
		}
		return id, nil
	default:
		return fmt.Sprint(value), nil
	}
}

const (
	minInt32 = -1 << 31
	maxInt32 = 1<<31 - 1
)

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func fieldByName(schema ingestion.RecordSchema, name string) (ingestion.SchemaField, bool) {
	for _, field := range schema.Fields {
		if field.Name == name {
			return field, true
		}
	}
	return ingestion.SchemaField{}, false
}

func jsonNumber(value any) (int64, error) {
	switch v := value.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case json.Number:
		return v.Int64()
	case float64:
		if math.Trunc(v) != v {
			return 0, fmt.Errorf("expected integer-compatible value, got fractional %v", v)
		}
		return int64(v), nil
	case string:
		n := json.Number(v)
		return n.Int64()
	default:
		return 0, fmt.Errorf("expected integer-compatible value, got %T", value)
	}
}

func jsonFloat(value any) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case int:
		return float64(v), nil
	case json.Number:
		return v.Float64()
	case string:
		n := json.Number(v)
		return n.Float64()
	default:
		return 0, fmt.Errorf("expected numeric value, got %T", value)
	}
}
