package paths

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/galaxy-io/filament/connectors/http/errs"
)

func decode(t *testing.T, raw string) any {
	t.Helper()
	var out any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return out
}

// Cursor pagination over an API that returns no next-page token reads the id
// off the last record on the page. A negative index expresses that without
// hardcoding the page size, which would silently truncate the walk whenever a
// short page arrives with more results still pending.
func TestAsStringArrayIndexing(t *testing.T) {
	doc := decode(t, `{"data":[{"id":"a"},{"id":"b"},{"id":"c"}]}`)

	for _, tc := range []struct {
		path string
		want string
	}{
		{"data.0.id", "a"},
		{"data.2.id", "c"},
		{"data.-1.id", "c"},
		{"data.-2.id", "b"},
		{"data.-3.id", "a"},
	} {
		got, ok, err := AsString(doc, tc.path)
		if err != nil {
			t.Fatalf("AsString(%q): %v", tc.path, err)
		}
		if !ok || got != tc.want {
			t.Fatalf("AsString(%q) = %q (ok=%v), want %q", tc.path, got, ok, tc.want)
		}
	}
}

// AsStringStrict, not AsString: the cursor paginator dispatches on the typed
// error, and AsString swallows missing by contract.
func TestAsStringStrictArrayIndexOutOfRange(t *testing.T) {
	doc := decode(t, `{"data":[{"id":"a"},{"id":"b"}],"empty":[]}`)

	for _, path := range []string{
		"data.2.id",  // past the end
		"data.-3.id", // negative past the start
		"empty.0.id",
		// -1 on an empty array must not wrap to the end: the cursor paginator
		// reads ErrPathMissing as a terminator, which is right for a final
		// page that came back empty.
		"empty.-1.id",
	} {
		_, ok, err := AsStringStrict(doc, path)
		if ok || !errors.Is(err, errs.ErrPathMissing) {
			t.Fatalf("AsStringStrict(%q) = (ok=%v, err=%v), want ErrPathMissing", path, ok, err)
		}
	}
}

func TestAsStringNonNumericArraySegment(t *testing.T) {
	doc := decode(t, `{"data":[{"id":"a"}]}`)
	_, _, err := AsString(doc, "data.first.id")
	if !errors.Is(err, errs.ErrPathType) {
		t.Fatalf("AsString on a non-numeric array segment = %v, want ErrPathType", err)
	}
}

func TestSplitHonoursEscapedDotsAndNegativeSegments(t *testing.T) {
	for _, tc := range []struct {
		path string
		want []string
	}{
		{"a.b.c", []string{"a", "b", "c"}},
		{"data.-1.id", []string{"data", "-1", "id"}},
		{`meta.sub\.key`, []string{"meta", "sub.key"}},
		{"", nil},
	} {
		got := Split(tc.path)
		if len(got) != len(tc.want) {
			t.Fatalf("Split(%q) = %#v, want %#v", tc.path, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("Split(%q) = %#v, want %#v", tc.path, got, tc.want)
			}
		}
	}
}
