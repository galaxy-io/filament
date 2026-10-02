package paths

import (
	"encoding/json"
	"errors"
	"strings"
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

// A FHIR R4 searchset Bundle carries its page links as an array of
// {"relation","url"} objects. The [key=value] selector picks the next link
// without hardcoding an array position, which the server is free to reorder.
func TestAsStringElementSelector(t *testing.T) {
	doc := decode(t, `{"link":[
		{"relation":"self","url":"https://hapi.fhir.org/baseR4/Patient?_count=100"},
		{"relation":"next","url":"https://hapi.fhir.org/baseR4/Patient?_count=100&searchId=abc"}
	]}`)

	got, ok, err := AsString(doc, "link[relation=next].url")
	if err != nil || !ok {
		t.Fatalf("selector = (%q, ok=%v, err=%v), want the next URL", got, ok, err)
	}
	if want := "https://hapi.fhir.org/baseR4/Patient?_count=100&searchId=abc"; got != want {
		t.Fatalf("selector = %q, want %q", got, want)
	}

	// The first element matches too; selection is positional only as a tiebreak.
	got, _, err = AsString(doc, "link[relation=self].url")
	if err != nil || !strings.HasSuffix(got, "_count=100") {
		t.Fatalf("self selector = (%q, err=%v)", got, err)
	}
}

// A bare [k=v] segment applies to the current node, which must be an array.
func TestAsStringBareElementSelector(t *testing.T) {
	doc := decode(t, `[{"relation":"next","url":"u1"},{"relation":"self","url":"u2"}]`)
	got, ok, err := AsString(doc, "[relation=self].url")
	if err != nil || !ok || got != "u2" {
		t.Fatalf("bare selector = (%q, ok=%v, err=%v), want u2", got, ok, err)
	}
}

// No matching element is ErrPathMissing, which next_url pagination reads as
// a clean terminator on the final page (no "next" link present).
func TestAsStringElementSelectorNoMatchIsMissing(t *testing.T) {
	doc := decode(t, `{"link":[{"relation":"self","url":"u1"}]}`)
	_, ok, err := AsStringStrict(doc, "link[relation=next].url")
	if ok || !errors.Is(err, errs.ErrPathMissing) {
		t.Fatalf("no-match selector = (ok=%v, err=%v), want ErrPathMissing", ok, err)
	}
}

// Selecting from a non-array is a loud type error, not a silent terminator.
func TestAsStringElementSelectorOnNonArray(t *testing.T) {
	doc := decode(t, `{"link":{"relation":"next","url":"u1"}}`)
	_, _, err := AsString(doc, "link[relation=next].url")
	if !errors.Is(err, errs.ErrPathType) {
		t.Fatalf("selector on object = %v, want ErrPathType", err)
	}
}

// Segments that merely contain brackets without the key=value shape keep
// their old meaning: literal key lookup on objects, index error on arrays.
func TestAsStringBracketSegmentWithoutSelectorShape(t *testing.T) {
	doc := decode(t, `{"data":[{"id":"a"}],"weird[0]":"literal"}`)
	got, ok, err := AsString(doc, `weird[0]`)
	if err != nil || !ok || got != "literal" {
		t.Fatalf("bracket literal = (%q, ok=%v, err=%v), want literal", got, ok, err)
	}
	_, ok, err = AsStringStrict(doc, "data[0].id")
	if ok || !errors.Is(err, errs.ErrPathMissing) {
		t.Fatalf("data[0] on a map = (ok=%v, err=%v), want ErrPathMissing (literal key, not a selector)", ok, err)
	}
}

// The selector compares the key's string rendering, so numeric and boolean
// element fields match their textual form.
func TestAsStringElementSelectorCoercesScalars(t *testing.T) {
	doc := decode(t, `{"items":[{"n":2,"v":"two"},{"n":10,"v":"ten"}]}`)
	got, _, err := AsString(doc, "items[n=10].v")
	if err != nil || got != "ten" {
		t.Fatalf("numeric selector = (%q, err=%v), want ten", got, err)
	}
}
