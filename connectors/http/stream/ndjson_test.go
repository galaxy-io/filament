package stream

import (
	"errors"
	"strings"
	"testing"

	"github.com/galaxy-io/filament/connectors/http/errs"
	"github.com/galaxy-io/filament/connectors/http/manifest"
)

func readNDJSON(t *testing.T, spec *manifest.StreamSpec, body string) (int, error) {
	t.Helper()
	r, err := New(spec)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	err = r.Read(t.Context(), strings.NewReader(body), func(map[string]any) error {
		n++
		return nil
	})
	return n, err
}

// Mixpanel's raw export answers a window with no events with the body
// "terminated early" instead of an empty response.
func TestNDJSONEmptyBodyMarker(t *testing.T) {
	spec := &manifest.StreamSpec{Type: "ndjson", EmptyBody: "terminated early"}
	for _, body := range []string{"terminated early", "terminated early\n", "\n  terminated early \r\n"} {
		n, err := readNDJSON(t, spec, body)
		if err != nil || n != 0 {
			t.Fatalf("body %q: records = %d, err = %v; want zero records", body, n, err)
		}
	}
}

func TestNDJSONEmptyBodyMarkerKeepsRecords(t *testing.T) {
	spec := &manifest.StreamSpec{Type: "ndjson", EmptyBody: "terminated early"}
	n, err := readNDJSON(t, spec, `{"event":"a"}`+"\n"+`{"event":"b"}`+"\n")
	if err != nil || n != 2 {
		t.Fatalf("records = %d, err = %v; want 2", n, err)
	}
	// A large body passes the peek untouched.
	big := strings.Repeat(`{"event":"a"}`+"\n", 5000)
	n, err = readNDJSON(t, spec, big)
	if err != nil || n != 5000 {
		t.Fatalf("records = %d, err = %v; want 5000", n, err)
	}
}

// Only the exact marker is treated as empty. Anything else still meets the
// strict line parser.
func TestNDJSONEmptyBodyMarkerIsExact(t *testing.T) {
	spec := &manifest.StreamSpec{Type: "ndjson", EmptyBody: "terminated early"}
	for _, body := range []string{
		"terminated early\nterminated early\n",
		`{"event":"a"}` + "\nterminated early\n",
		"terminated earlyish\n",
		"error: terminated early\n",
	} {
		if _, err := readNDJSON(t, spec, body); !errors.Is(err, errs.ErrMalformedLine) {
			t.Fatalf("body %q: err = %v, want a malformed line", body, err)
		}
	}
}

// Without empty_body the marker is a malformed line, as before.
func TestNDJSONWithoutEmptyBodyIsStrict(t *testing.T) {
	for _, spec := range []*manifest.StreamSpec{nil, {Type: "ndjson"}} {
		if _, err := readNDJSON(t, spec, "terminated early\n"); !errors.Is(err, errs.ErrMalformedLine) {
			t.Fatalf("spec %+v: err = %v, want a malformed line", spec, err)
		}
	}
	if n, err := readNDJSON(t, &manifest.StreamSpec{Type: "ndjson"}, ""); n != 0 || err != nil {
		t.Fatalf("empty body: records = %d, err = %v", n, err)
	}
}
