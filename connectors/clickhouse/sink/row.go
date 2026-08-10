package clickhouse

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/galaxy-io/filament"
)

func decodeRecord(schema filament.RecordSchema, data []byte) ([]any, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, fmt.Errorf("decode JSON object: %w", err)
	}
	if object == nil {
		return nil, fmt.Errorf("record payload is not a JSON object")
	}
	values := make([]any, 0, len(schema.Fields))
	for _, field := range schema.Fields {
		raw, ok := object[field.Name]
		if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			if !field.Nullable {
				return nil, fmt.Errorf("non-nullable field %q is missing or null", field.Name)
			}
			values = append(values, nil)
			continue
		}
		value, err := decodeValue(field, raw)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", field.Name, err)
		}
		values = append(values, value)
	}
	return values, nil
}

func decodeValue(field filament.SchemaField, raw json.RawMessage) (any, error) {
	switch field.Logical {
	case filament.LogicalBool:
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		return value, nil
	case filament.LogicalInt16:
		value, err := parseInt(raw, 16)
		if err != nil {
			return nil, err
		}
		return int16(value), nil //nolint:gosec // ParseInt enforced the Int16 range
	case filament.LogicalInt32:
		value, err := parseInt(raw, 32)
		if err != nil {
			return nil, err
		}
		return int32(value), nil //nolint:gosec // ParseInt enforced the Int32 range
	case filament.LogicalInt64:
		return parseInt(raw, 64)
	case filament.LogicalFloat32:
		value, err := parseFloat(raw, 32)
		return float32(value), err
	case filament.LogicalFloat64:
		return parseFloat(raw, 64)
	case filament.LogicalDecimal:
		value, err := scalarText(raw)
		if err != nil {
			return nil, err
		}
		return decimal.NewFromString(value)
	case filament.LogicalString, filament.LogicalDate, filament.LogicalTime,
		filament.LogicalTimestamp, filament.LogicalTimestampTZ, filament.LogicalUUID:
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		return value, nil
	case filament.LogicalBytes:
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		return decodeBytes(value)
	case filament.LogicalJSON, filament.LogicalArray:
		return string(bytes.TrimSpace(raw)), nil
	case filament.LogicalUnknown:
		return scalarText(raw)
	default:
		return string(bytes.TrimSpace(raw)), nil
	}
}

func scalarText(raw json.RawMessage) (string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var value string
		if err := json.Unmarshal(trimmed, &value); err != nil {
			return "", err
		}
		return value, nil
	}
	return string(trimmed), nil
}

func parseInt(raw json.RawMessage, bits int) (int64, error) {
	value, err := scalarText(raw)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(value, 10, bits)
}

func parseFloat(raw json.RawMessage, bits int) (float64, error) {
	value, err := scalarText(raw)
	if err != nil {
		return 0, err
	}
	switch strings.ToLower(value) {
	case "nan":
		return math.NaN(), nil
	case "infinity", "+infinity", "inf", "+inf":
		return math.Inf(1), nil
	case "-infinity", "-inf":
		return math.Inf(-1), nil
	default:
		return strconv.ParseFloat(value, bits)
	}
}

func decodeBytes(value string) (string, error) {
	if strings.HasPrefix(value, `\x`) {
		decoded, err := hex.DecodeString(value[2:])
		if err != nil {
			return "", fmt.Errorf("decode hex bytes: %w", err)
		}
		return string(decoded), nil
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", fmt.Errorf("decode base64 bytes: %w", err)
	}
	return string(decoded), nil
}
