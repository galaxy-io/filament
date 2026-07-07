package iceberg

import (
	"encoding/json"
	"fmt"

	"github.com/galaxy-io/filament"
)

// jsonColumns returns the names of schema fields typed LogicalJSON, or nil when
// there are none. These map to Iceberg string columns, so their buffered values
// must be re-encoded as JSON strings before Arrow parsing.
func jsonColumns(schema ingestion.RecordSchema) map[string]bool {
	var cols map[string]bool
	for _, f := range schema.Fields {
		if f.Logical == ingestion.LogicalJSON {
			if cols == nil {
				cols = map[string]bool{}
			}
			cols[f.Name] = true
		}
	}
	return cols
}

// encodeJSONColumns rewrites each record's json-typed columns as JSON strings so
// the payload matches the Arrow string column the sink declares for them.
// Values already strings, nulls, and absent keys pass through untouched.
func encodeJSONColumns(recs []json.RawMessage, cols map[string]bool) ([]json.RawMessage, error) {
	if len(cols) == 0 {
		return recs, nil
	}
	for i, rec := range recs {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(rec, &obj); err != nil {
			return nil, fmt.Errorf("decode record: %w", err)
		}
		changed := false
		for col := range cols {
			raw, ok := obj[col]
			if !ok || isJSONStringOrNull(raw) {
				continue
			}
			enc, err := json.Marshal(string(raw))
			if err != nil {
				return nil, fmt.Errorf("encode json column %q: %w", col, err)
			}
			obj[col] = enc
			changed = true
		}
		if !changed {
			continue
		}
		enc, err := json.Marshal(obj)
		if err != nil {
			return nil, fmt.Errorf("re-encode record: %w", err)
		}
		recs[i] = enc
	}
	return recs, nil
}

// isJSONStringOrNull reports whether the raw value's first token is a string or
// null literal — the two forms that already fit an Arrow string column.
func isJSONStringOrNull(raw json.RawMessage) bool {
	for _, b := range raw {
		switch b {
		case ' ', '\t', '\n', '\r':
			continue
		}
		return b == '"' || b == 'n'
	}
	return true
}
