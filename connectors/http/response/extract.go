// Package response decodes API responses into records, captures parent fields,
// and surfaces 200-wrapped errors.
//
// Path semantics for records_path / error.path use gjson syntax (dot-path
// with optional array indexing). Empty records_path means records live at
// the document root.
//
// Capture field paths use the connector's shared paths package: dotted keys,
// numeric segments index into arrays (e.g. `tags.0.name`), and backslash-
// escaping of literal dots inside a key.
package response

import (
	"errors"
	"fmt"

	"github.com/tidwall/gjson"

	"github.com/galaxy-io/filament/connectors/http/errs"
	"github.com/galaxy-io/filament/connectors/http/internal/paths"
	"github.com/galaxy-io/filament/connectors/http/manifest"
)

// Extractor pulls records, captures, and errors out of raw response bytes.
type Extractor struct {
	spec manifest.ResponseSpec
}

// New returns an Extractor for the given response spec.
func New(spec manifest.ResponseSpec) *Extractor {
	return &Extractor{spec: spec}
}

// Records returns the records array from raw. When `records_path` is empty:
//   - root=array → entire body is the array
//   - root=object (or empty) → error, nothing to extract
func (e *Extractor) Records(raw []byte) ([]map[string]any, error) {
	if !gjson.ValidBytes(raw) {
		return nil, fmt.Errorf("response: body is not valid JSON")
	}

	var arr gjson.Result
	if e.spec.RecordsPath == "" {
		if e.spec.Root != "array" {
			return nil, fmt.Errorf("response: records_path is empty but root is not \"array\"")
		}
		arr = gjson.ParseBytes(raw)
	} else {
		arr = gjson.GetBytes(raw, e.spec.RecordsPath)
	}

	if !arr.Exists() {
		return nil, nil
	}
	if !arr.IsArray() {
		return nil, fmt.Errorf("response: records_path %q does not resolve to an array", e.spec.RecordsPath)
	}

	items := arr.Array()
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		m, err := resultToMap(it)
		if err != nil {
			return nil, err
		}
		if m != nil {
			out = append(out, m)
		}
	}
	return out, nil
}

// CheckError inspects raw for an API-style 200-wrapped error envelope. Returns
// nil when no error.spec is configured or the spec doesn't fire.
func (e *Extractor) CheckError(raw []byte) error {
	if e.spec.Error == nil || e.spec.Error.Path == "" {
		return nil
	}
	if !gjson.ValidBytes(raw) {
		return nil
	}
	v := gjson.GetBytes(raw, e.spec.Error.Path)
	// `when_present`: any non-null/non-empty existence triggers the error.
	if e.spec.Error.WhenPresent {
		if !v.Exists() || isEmpty(v) {
			return nil
		}
	} else {
		if !v.Exists() {
			return nil
		}
	}

	msg := v.String()
	if e.spec.Error.MessagePath != "" {
		if mv := gjson.GetBytes(raw, e.spec.Error.MessagePath); mv.Exists() {
			msg = mv.String()
		}
	}
	if e.spec.Error.CodePath != "" {
		if cv := gjson.GetBytes(raw, e.spec.Error.CodePath); cv.Exists() {
			return fmt.Errorf("api error %s: %s", cv.String(), msg)
		}
	}
	return fmt.Errorf("api error: %s", msg)
}

// Capture pulls the configured fields out of one record into a flat string map
// suitable for the `parent.*` template scope. Missing or null leaves yield an
// empty string. Non-scalar leaves (arrays, objects) yield "" — Capture is for
// flat scope values, not embedded structures.
func (e *Extractor) Capture(record map[string]any, fields map[string]string) map[string]string {
	if len(fields) == 0 {
		return nil
	}
	out := make(map[string]string, len(fields))
	for k, path := range fields {
		s, _, err := paths.AsString(record, path)
		if err != nil && !errors.Is(err, errs.ErrPathMissing) && !errors.Is(err, errs.ErrPathNull) {
			// Non-scalar (object/array) at the leaf: leave the field empty
			// rather than emit a JSON-stringified blob, which would smuggle
			// structured data into a flat scope and confuse downstream
			// templates. The miss is intentional and silent.
			out[k] = ""
			continue
		}
		out[k] = s
	}
	return out
}

// resultToMap converts a gjson object into a map[string]any. Non-object items
// are skipped (caller iterates an array of objects).
func resultToMap(r gjson.Result) (map[string]any, error) {
	if !r.IsObject() {
		return nil, nil
	}
	v := r.Value()
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("response: gjson value not a map (got %T)", v)
	}
	return m, nil
}

func isEmpty(v gjson.Result) bool {
	switch v.Type {
	case gjson.Null:
		return true
	case gjson.String:
		return v.Str == ""
	}
	if v.IsArray() && len(v.Array()) == 0 {
		return true
	}
	if v.IsObject() && len(v.Map()) == 0 {
		return true
	}
	return false
}
