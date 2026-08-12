// Package paths is the canonical dot-path lookup for the httpapi connector.
// Every sub-package that needs to walk a decoded JSON document uses it so
// syntax, error semantics, and typed sentinels stay consistent.
//
// Path syntax:
//
//	a.b.c          — nested object keys
//	items.0.name   — numeric segments index into arrays
//	data.-1.id     — negative indices count from the end (-1 is the last)
//	meta.sub\.key  — backslash escapes a literal dot inside a key
//
// The package exposes three accessors. Each returns a typed error
// (errs.ErrPathMissing, errs.ErrPathNull, or errs.ErrPathType) so callers
// can dispatch on outcome without parsing error strings:
//
//	paths.AsString — best-effort scalar → string (coerces number, bool, null)
//	paths.Int      — int64 with exact-integer check
//	paths.Bool     — bool, strict type match
//
// The low-level lookup/value/Kind machinery is intentionally unexported —
// the three helpers above cover every production call site.
package paths

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/galaxy-io/filament/connectors/http/errs"
)

// kind classifies the resolved value's runtime type.
type kind int

const (
	kindMissing kind = iota
	kindNull
	kindString
	kindNumber
	kindBool
	kindArray
	kindObject
	kindOther
)

func (k kind) String() string {
	switch k {
	case kindMissing:
		return "missing"
	case kindNull:
		return "null"
	case kindString:
		return "string"
	case kindNumber:
		return "number"
	case kindBool:
		return "bool"
	case kindArray:
		return "array"
	case kindObject:
		return "object"
	default:
		return "other"
	}
}

// value wraps the result of a successful lookup.
type value struct {
	kind kind
	raw  any
}

func (v value) asString() (string, bool, error) {
	switch x := v.raw.(type) {
	case string:
		return x, true, nil
	case nil:
		return "", false, nil
	case bool:
		return strconv.FormatBool(x), true, nil
	case int:
		return strconv.Itoa(x), true, nil
	case int32:
		return strconv.FormatInt(int64(x), 10), true, nil
	case int64:
		return strconv.FormatInt(x, 10), true, nil
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), true, nil
	}
	return "", false, typeErr("scalar", v.kind)
}

func (v value) toInt() (int64, error) {
	switch x := v.raw.(type) {
	case int:
		return int64(x), nil
	case int32:
		return int64(x), nil
	case int64:
		return x, nil
	case float64:
		i := int64(x)
		if float64(i) != x {
			return 0, fmt.Errorf("%w: number %v is not an integer", errs.ErrPathType, x)
		}
		return i, nil
	}
	return 0, typeErr("integer", v.kind)
}

func (v value) toBool() (bool, error) {
	if b, ok := v.raw.(bool); ok {
		return b, nil
	}
	return false, typeErr("bool", v.kind)
}

// Int resolves path to an int64. Returns ErrPathMissing/ErrPathNull when the
// path doesn't resolve; ErrPathType when it resolves to a non-integer value.
func Int(data any, path string) (int64, error) {
	v, err := lookup(data, path)
	if err != nil {
		return 0, err
	}
	return v.toInt()
}

// Bool resolves path to a bool. Non-bool types (even truthy-ish values like
// "true" or 1) return ErrPathType — callers that want coercion use AsString.
func Bool(data any, path string) (bool, error) {
	v, err := lookup(data, path)
	if err != nil {
		return false, err
	}
	return v.toBool()
}

// Value resolves path and returns the raw JSON-decoded value. Empty path returns
// the root value. Missing/null/type errors follow the same semantics as the
// typed helpers.
func Value(data any, path string) (any, error) {
	v, err := lookup(data, path)
	if err != nil {
		return nil, err
	}
	return v.raw, nil
}

// AsString returns a best-effort string rendering of any scalar at path. Null
// and missing both yield ("", false, nil); non-scalar (array/object) leaves
// return ErrPathType so the caller can decide whether to surface or swallow.
//
// Use AsStringStrict when null vs missing must be distinguished (e.g. cursor
// pagination terminating on missing but erroring on explicit null).
func AsString(data any, path string) (string, bool, error) {
	v, err := lookup(data, path)
	if err != nil {
		if errors.Is(err, errs.ErrPathMissing) || errors.Is(err, errs.ErrPathNull) {
			return "", false, nil
		}
		return "", false, err
	}
	return v.asString()
}

// AsStringStrict is AsString that preserves the missing-vs-null distinction.
// Returns:
//
//   - present scalar  → (value, true, nil)
//   - missing key     → ("", false, wrapped errs.ErrPathMissing)
//   - explicit null   → ("", false, wrapped errs.ErrPathNull)
//   - non-scalar leaf → ("", false, wrapped errs.ErrPathType)
//
// Callers dispatch on outcome via errors.Is.
func AsStringStrict(data any, path string) (string, bool, error) {
	v, err := lookup(data, path)
	if err != nil {
		return "", false, err
	}
	return v.asString()
}

// lookup walks data along path and returns the resolved value plus an error
// classifying the outcome. Empty path returns the root.
func lookup(data any, path string) (value, error) {
	if path == "" {
		return classify(data), nil
	}
	segs := split(path)
	cur := data
	for i, seg := range segs {
		switch node := cur.(type) {
		case nil:
			return value{kind: kindMissing}, missingAt(segs, i)
		case map[string]any:
			next, ok := node[seg]
			if !ok {
				return value{kind: kindMissing}, missingAt(segs, i)
			}
			cur = next
		case map[string]string:
			next, ok := node[seg]
			if !ok {
				return value{kind: kindMissing}, missingAt(segs, i)
			}
			cur = next
		case []any:
			idx, err := strconv.Atoi(seg)
			if err != nil {
				return value{kind: kindOther}, fmt.Errorf("%w: segment %q is not an array index at %s",
					errs.ErrPathType, seg, joinPrefix(segs, i))
			}
			// Negative indices count back from the end, so `data.-1.id` reads
			// the last record without hardcoding a page size. An out-of-range
			// index (including -1 on an empty array) stays ErrPathMissing,
			// which cursor pagination reads as a clean terminator.
			at := idx
			if at < 0 {
				at += len(node)
			}
			if at < 0 || at >= len(node) {
				return value{kind: kindMissing}, fmt.Errorf("%w: index %d out of range at %s",
					errs.ErrPathMissing, idx, joinPrefix(segs, i))
			}
			cur = node[at]
		default:
			return value{kind: kindOther}, fmt.Errorf("%w: cannot descend into %T at %s",
				errs.ErrPathType, node, joinPrefix(segs, i))
		}
	}
	v := classify(cur)
	if v.kind == kindNull {
		return v, fmt.Errorf("%w: %s", errs.ErrPathNull, path)
	}
	return v, nil
}

// Split parses a dot-path into segments, honoring backslash-escapes for
// literal dots inside keys. Empty path returns nil. Exported for callers
// (e.g. the streaming chunked_array reader) that walk a token stream rather
// than a decoded tree but still need the same segmentation semantics.
func Split(path string) []string { return split(path) }

// split parses a dot-path into segments, honoring backslash-escapes for
// literal dots inside keys. Empty path returns nil.
func split(path string) []string {
	if path == "" {
		return nil
	}
	var out []string
	var cur strings.Builder
	escape := false
	for i := 0; i < len(path); i++ {
		c := path[i]
		switch {
		case escape:
			cur.WriteByte(c)
			escape = false
		case c == '\\':
			escape = true
		case c == '.':
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	if escape {
		cur.WriteByte('\\')
	}
	out = append(out, cur.String())
	return out
}

func classify(v any) value {
	switch x := v.(type) {
	case nil:
		return value{kind: kindNull, raw: nil}
	case string:
		return value{kind: kindString, raw: x}
	case bool:
		return value{kind: kindBool, raw: x}
	case float64, int, int32, int64:
		return value{kind: kindNumber, raw: x}
	case []any:
		return value{kind: kindArray, raw: x}
	case map[string]any:
		return value{kind: kindObject, raw: x}
	case map[string]string:
		return value{kind: kindObject, raw: x}
	default:
		return value{kind: kindOther, raw: x}
	}
}

func typeErr(want string, got kind) error {
	return fmt.Errorf("%w: want %s, got %s", errs.ErrPathType, want, got)
}

func missingAt(segs []string, i int) error {
	return fmt.Errorf("%w: %s", errs.ErrPathMissing, joinPrefix(segs, i+1))
}

// joinPrefix re-renders the first n segments as a dot-path for error messages.
// Single dots in segments are escaped so the rendered path is unambiguous.
func joinPrefix(segs []string, n int) string {
	if n > len(segs) {
		n = len(segs)
	}
	parts := make([]string, n)
	for i := 0; i < n; i++ {
		parts[i] = strings.ReplaceAll(segs[i], ".", `\.`)
	}
	return strings.Join(parts, ".")
}
