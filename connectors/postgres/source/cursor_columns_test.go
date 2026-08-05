package postgres

import "testing"

func TestTimestampCursorTypes(t *testing.T) {
	for _, typ := range []string{"timestamp with time zone", "timestamp without time zone", "timestamp(3) with time zone", "timestamptz", "timestamptz(6)"} {
		if !isTimestampType(typ) {
			t.Errorf("%q should be eligible", typ)
		}
	}
	for _, typ := range []string{"bigint", "numeric(12,2)", "uuid", "text", "date", "jsonb"} {
		if isTimestampType(typ) {
			t.Errorf("%q should not be eligible", typ)
		}
	}
}
