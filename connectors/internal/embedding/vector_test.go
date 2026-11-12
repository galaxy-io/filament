package embedding_test

import (
	"reflect"
	"testing"

	"github.com/galaxy-io/filament/connectors/internal/embedding"
)

func TestParseVector(t *testing.T) {
	got, err := embedding.ParseVector(" [0.5, -1, 2e3] ")
	if err != nil {
		t.Fatalf("ParseVector: %v", err)
	}
	if want := []float32{0.5, -1, 2000}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseVector = %v, want %v", got, want)
	}
}

func TestParseVectorRejectsBadInput(t *testing.T) {
	for _, in := range []string{
		"",
		"[]",
		"[0.1,,0.5]",
		"[0.1, null]",
		`[0.1, "0.2"]`,
		"[NaN]",
		"[1e39]", // overflows float32
		"0.1, 0.2",
		`{"v": [1]}`,
	} {
		if got, err := embedding.ParseVector(in); err == nil {
			t.Errorf("ParseVector(%q) = %v, want an error", in, got)
		}
	}
}
