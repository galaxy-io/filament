package paths

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/galaxy-io/filament/connectors/http/errs"
)

func decode(t *testing.T, raw string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return v
}

func TestAsStringArrayIndex(t *testing.T) {
	doc := decode(t, `{"data":[{"id":"a"},{"id":"b"},{"id":"c"}]}`)

	tests := []struct {
		path string
		want string
	}{
		{"data.0.id", "a"},
		{"data.2.id", "c"},
		{"data.-1.id", "c"},
		{"data.-3.id", "a"},
	}
	for _, tc := range tests {
		got, present, err := AsString(doc, tc.path)
		if err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		if !present || got != tc.want {
			t.Fatalf("%s = %q (present %v), want %q", tc.path, got, present, tc.want)
		}
	}
}

// A negative index past the start of the array, and any index into an empty
// array, must report missing rather than wrapping around — cursor pagination
// reads a missing cursor as "last page", so a wrap would resurrect an already
// consumed record and loop forever.
func TestAsStringStrictNegativeIndexOutOfRange(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		path string
	}{
		{"before start", `{"data":[{"id":"a"}]}`, "data.-2.id"},
		{"empty array", `{"data":[]}`, "data.-1.id"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := AsStringStrict(decode(t, tc.doc), tc.path)
			if !errors.Is(err, errs.ErrPathMissing) {
				t.Fatalf("err = %v, want ErrPathMissing", err)
			}
		})
	}
}
