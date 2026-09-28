package streamkit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

// MessageColumns projects JSON object members into source-owned columns. Other
// messages retain their bytes in a source-owned payload column. A resource keeps
// the first message's schema; drift fails before any row values are appended.
type MessageColumns struct {
	fields []rowmodel.Field
	raw    bool
}

func decodeObject(payload []byte) (map[string]json.RawMessage, bool, error) {
	if !json.Valid(payload) || len(bytes.TrimSpace(payload)) == 0 || bytes.TrimSpace(payload)[0] != '{' {
		return nil, false, nil
	}
	// Reject duplicate keys rather than silently losing the earlier value.
	d := json.NewDecoder(bytes.NewReader(payload))
	_, _ = d.Token()
	values := map[string]json.RawMessage{}
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return nil, true, err
		}
		name := token.(string)
		if _, exists := values[name]; exists {
			return nil, true, fmt.Errorf("duplicate JSON field %q", name)
		}
		var value json.RawMessage
		if err := d.Decode(&value); err != nil {
			return nil, true, err
		}
		values[name] = value
	}
	return values, true, nil
}

func messageType(raw json.RawMessage) rowmodel.LogicalType {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return rowmodel.LogicalJSON
	}
	switch raw[0] {
	case '"':
		return rowmodel.LogicalString
	case 't', 'f':
		return rowmodel.LogicalBool
	case '{', '[':
		return rowmodel.LogicalJSON
	default:
		// JSON storage preserves number precision, including integers above 2^53
		// and arbitrary decimals, without float conversion or later type promotion.
		return rowmodel.LogicalJSON
	}
}

// NewMessageColumns infers a deterministic schema and rejects collisions with
// transport columns and Filament's reserved namespace.
func NewMessageColumns(base rowmodel.Schema, payload []byte) (*MessageColumns, rowmodel.Schema, error) {
	values, object, err := decodeObject(payload)
	if err != nil {
		return nil, rowmodel.Schema{}, err
	}
	columns := &MessageColumns{raw: !object}
	occupied := map[string]bool{}
	for _, field := range base.Fields {
		occupied[strings.ToLower(field.Name)] = true
	}
	if !object {
		columns.fields = []rowmodel.Field{{Name: "payload", Logical: rowmodel.LogicalBytes, Nullable: true}}
	} else {
		names := make([]string, 0, len(values))
		for name := range values {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			lower := strings.ToLower(name)
			if name == "" || strings.HasPrefix(lower, "_filament_") || occupied[lower] {
				return nil, rowmodel.Schema{}, fmt.Errorf("message field %q collides with transport metadata, another field, or the reserved Filament namespace", name)
			}
			occupied[lower] = true
			columns.fields = append(columns.fields, rowmodel.Field{Name: name, Logical: messageType(values[name]), Nullable: true})
		}
	}
	base = base.Clone()
	base.Fields = append(base.Fields, columns.fields...)
	schema, err := rowmodel.WithEventMetadataFields(base)
	return columns, schema, err
}

// Prepare validates the entire message before the caller appends transport
// fields. The returned closure writes only source-owned message columns.
func (c *MessageColumns) Prepare(payload []byte) (func(arrowbatch.RowWriter), error) {
	values, object, err := decodeObject(payload)
	if err != nil {
		return nil, err
	}
	if c.raw {
		if object {
			return nil, fmt.Errorf("message schema changed from raw payload to JSON object")
		}
		return func(w arrowbatch.RowWriter) {
			if payload == nil {
				w.Null()
			} else {
				w.Bytes(payload)
			}
		}, nil
	}
	// A Kafka tombstone has no values, but retains the inferred JSON schema.
	if !object && payload != nil {
		return nil, fmt.Errorf("expected a JSON object for decoded message schema")
	}
	known := map[string]bool{}
	for _, field := range c.fields {
		known[field.Name] = true
		raw := values[field.Name]
		if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			continue
		}
		if field.Logical != rowmodel.LogicalJSON && messageType(raw) != field.Logical {
			return nil, fmt.Errorf("message field %q changed type", field.Name)
		}
	}
	for name := range values {
		if !known[name] {
			return nil, fmt.Errorf("message schema added field %q; restart with a compatible schema", name)
		}
	}
	return func(w arrowbatch.RowWriter) {
		for _, field := range c.fields {
			raw := values[field.Name]
			if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
				w.Null()
				continue
			}
			switch field.Logical {
			case rowmodel.LogicalString:
				var value string
				_ = json.Unmarshal(raw, &value)
				w.String(value)
			case rowmodel.LogicalBool:
				w.Bool(bytes.Equal(bytes.TrimSpace(raw), []byte("true")))
			default:
				w.StringBytes(raw)
			}
		}
	}, nil
}
