// Package scalar provides the canonical conversion of HTTP connector scalar
// values to strings.
package scalar

import "strconv"

// String converts a supported scalar value to its string representation.
// The supported numeric types cover values produced by JSON decoding and
// manifest/config decoding. Nil is intentionally left to callers because its
// meaning differs by context (empty form field, missing path, or invalid
// projection).
func String(value any) (string, bool) {
	switch value := value.(type) {
	case string:
		return value, true
	case bool:
		return strconv.FormatBool(value), true
	case int:
		return strconv.Itoa(value), true
	case int32:
		return strconv.FormatInt(int64(value), 10), true
	case int64:
		return strconv.FormatInt(value, 10), true
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64), true
	default:
		return "", false
	}
}
