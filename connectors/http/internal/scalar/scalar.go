// Package scalar provides the canonical conversion of HTTP connector scalar
// values to strings.
package scalar

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"

	"github.com/galaxy-io/filament/connectors/http/errs"
)

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
	case json.Number:
		return value.String(), true
	default:
		return "", false
	}
}

// NumberInt64 accepts integral JSON numbers, including decimal and exponent
// notation, without rounding long identifiers through float64.
func NumberInt64(value json.Number) (int64, error) {
	if n, err := value.Int64(); err == nil {
		return n, nil
	}
	n, ok := new(big.Rat).SetString(value.String())
	if !ok || !n.IsInt() || !n.Num().IsInt64() {
		return 0, fmt.Errorf("%w: number %s is not an int64", errs.ErrPathType, value)
	}
	return n.Num().Int64(), nil
}
