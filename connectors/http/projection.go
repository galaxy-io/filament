package httpapi

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/http/errs"
	"github.com/galaxy-io/filament/connectors/http/internal/paths"
	"github.com/galaxy-io/filament/connectors/http/manifest"
)

func schemaFields(res manifest.Resource) []filament.SchemaField {
	if len(res.Fields) == 0 {
		fields := make([]filament.SchemaField, 0, len(res.PrimaryKey)+1)
		for _, key := range res.PrimaryKey {
			fields = append(fields, filament.SchemaField{
				Name:     key,
				Nullable: false,
				Logical:  filament.LogicalString,
				Native:   "string",
			})
		}
		fields = append(fields, filament.SchemaField{
			Name:     "data",
			Nullable: false,
			Logical:  filament.LogicalJSON,
			Native:   "json",
		})
		return fields
	}
	fields := make([]filament.SchemaField, 0, len(res.Fields))
	for _, f := range res.Fields {
		fields = append(fields, filament.SchemaField{
			Name:     f.Name,
			Nullable: f.Nullable,
			Logical:  logicalType(f.Type),
			Native:   f.Type,
		})
	}
	return fields
}

func projectRecord(res manifest.Resource, record map[string]any, parent Capture) (map[string]any, bool, error) {
	if len(res.Fields) == 0 {
		return record, false, nil
	}
	out := make(map[string]any, len(res.Fields))
	var remainder map[string]any
	for _, field := range res.Fields {
		if field.Mode == "remainder" {
			if remainder == nil {
				remainder = recordRemainder(record, res.Fields)
			}
			out[field.Name] = remainder
			continue
		}
		if len(field.Shape) > 0 {
			value, ok, err := shapedValue(field, record, parent)
			if err != nil {
				return nil, false, fmt.Errorf("project field %q: %w", field.Name, err)
			}
			if !ok && field.Nullable {
				out[field.Name] = nil
			} else {
				out[field.Name] = value
			}
			continue
		}
		value, err := projectedValue(field, record, parent)
		if err != nil {
			if field.Nullable && (errors.Is(err, errs.ErrPathMissing) || errors.Is(err, errs.ErrPathNull)) {
				out[field.Name] = nil
				continue
			}
			return nil, false, fmt.Errorf("project field %q: %w", field.Name, err)
		}
		out[field.Name] = value
	}
	return out, true, nil
}

func recordRemainder(record map[string]any, fields []manifest.FieldSpec) map[string]any {
	out := deepCopyMap(record)
	for _, field := range fields {
		if field.Mode == "remainder" || strings.HasPrefix(field.Path, "parent.") {
			continue
		}
		if len(field.Shape) > 0 {
			for _, path := range field.Shape {
				deleteProjectedPath(out, path)
			}
		} else {
			deleteProjectedPath(out, field.Path)
		}
	}
	return out
}

func deleteProjectedPath(out map[string]any, path string) {
	if path == "" || path == "$" || strings.HasPrefix(path, "parent.") {
		return
	}
	deletePath(out, paths.Split(path))
}

func deepCopyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = deepCopyValue(v)
	}
	return out
}

func deepCopyValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		return deepCopyMap(x)
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = deepCopyValue(item)
		}
		return out
	default:
		return v
	}
}

func deletePath(node map[string]any, segs []string) {
	if len(segs) == 0 {
		return
	}
	if len(segs) == 1 {
		delete(node, segs[0])
		return
	}
	next, ok := node[segs[0]].(map[string]any)
	if !ok {
		return
	}
	deletePath(next, segs[1:])
	if len(next) == 0 {
		delete(node, segs[0])
	}
}

func projectedValue(field manifest.FieldSpec, record map[string]any, parent Capture) (any, error) {
	if strings.HasPrefix(field.Path, "parent.") {
		key := strings.TrimPrefix(field.Path, "parent.")
		value, ok := parent[key]
		if !ok || value == "" {
			return nil, fmt.Errorf("%w: %s", errs.ErrPathMissing, field.Path)
		}
		return coerceValue(field.Type, value)
	}
	path := field.Path
	if path == "$" {
		path = ""
	}
	value, err := paths.Value(record, path)
	if err != nil {
		return nil, err
	}
	return coerceValue(field.Type, value)
}

func shapedValue(field manifest.FieldSpec, record map[string]any, parent Capture) (map[string]any, bool, error) {
	out := make(map[string]any, len(field.Shape))
	for name, path := range field.Shape {
		value, err := projectedPathValue(path, record, parent)
		if err != nil {
			if errors.Is(err, errs.ErrPathMissing) || errors.Is(err, errs.ErrPathNull) {
				continue
			}
			return nil, false, fmt.Errorf("%s: %w", path, err)
		}
		out[name] = value
	}
	return out, len(out) > 0, nil
}

func projectedPathValue(path string, record map[string]any, parent Capture) (any, error) {
	if strings.HasPrefix(path, "parent.") {
		key := strings.TrimPrefix(path, "parent.")
		value, ok := parent[key]
		if !ok || value == "" {
			return nil, fmt.Errorf("%w: %s", errs.ErrPathMissing, path)
		}
		return value, nil
	}
	if path == "$" {
		path = ""
	}
	return paths.Value(record, path)
}

func coerceValue(fieldType string, value any) (any, error) {
	switch fieldType {
	case "string", "date", "time", "timestamp", "timestamptz", "uuid", "decimal":
		return scalarString(value)
	case "bool":
		if b, ok := value.(bool); ok {
			return b, nil
		}
		if s, ok := value.(string); ok {
			return strconv.ParseBool(s)
		}
		return nil, fmt.Errorf("%w: expected bool, got %T", errs.ErrPathType, value)
	case "int16", "int32", "int64":
		return scalarInt(value)
	case "float32", "float64":
		return scalarFloat(value)
	case "json":
		return value, nil
	default:
		return value, nil
	}
}

func scalarString(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case bool:
		return strconv.FormatBool(v), nil
	case int:
		return strconv.Itoa(v), nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	default:
		return "", fmt.Errorf("%w: expected scalar, got %T", errs.ErrPathType, value)
	}
}

func scalarInt(value any) (int64, error) {
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	case float64:
		i := int64(v)
		if float64(i) != v {
			return 0, fmt.Errorf("%w: number %v is not an integer", errs.ErrPathType, v)
		}
		return i, nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	default:
		return 0, fmt.Errorf("%w: expected integer, got %T", errs.ErrPathType, value)
	}
}

func scalarFloat(value any) (float64, error) {
	switch v := value.(type) {
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case float64:
		return v, nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("%w: expected float, got %T", errs.ErrPathType, value)
	}
}

func logicalType(fieldType string) filament.LogicalType {
	switch fieldType {
	case "bool":
		return filament.LogicalBool
	case "int16":
		return filament.LogicalInt16
	case "int32":
		return filament.LogicalInt32
	case "int64":
		return filament.LogicalInt64
	case "float32":
		return filament.LogicalFloat32
	case "float64":
		return filament.LogicalFloat64
	case "decimal":
		return filament.LogicalDecimal
	case "date":
		return filament.LogicalDate
	case "time":
		return filament.LogicalTime
	case "timestamp":
		return filament.LogicalTimestamp
	case "timestamptz":
		return filament.LogicalTimestampTZ
	case "json":
		return filament.LogicalJSON
	case "uuid":
		return filament.LogicalUUID
	default:
		return filament.LogicalString
	}
}
