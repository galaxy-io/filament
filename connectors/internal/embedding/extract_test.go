package embedding_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/connectors/internal/embedding"
	et "github.com/galaxy-io/filament/connectors/internal/embedding/embeddingtest"
	"github.com/galaxy-io/filament/rowmodel"
)

func TestExtractMixedOperations(t *testing.T) {
	b := et.Batch(t, "articles",
		et.Row{Op: rowmodel.OpInsert, ID: "a", Title: "First", Tag: "news", Views: 1, Vector: []float32{0.1, 0.2}},
		et.Row{Op: rowmodel.OpDelete, ID: "b"},
		et.Row{Op: rowmodel.OpUpdate, ID: "c", Title: "Third", Views: 3, Vector: []float32{0.3, 0.4}},
	)

	got, err := embedding.Extract(b, et.Config())
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	want := []embedding.Mutation{
		{Op: embedding.OpUpsert, Doc: embedding.Document{
			ID: "a", Text: "First", Vector: []float32{0.1, 0.2},
			Metadata: map[string]any{"title": "First", "tag": "news", "views": int64(1)},
		}},
		{Op: embedding.OpDelete, Doc: embedding.Document{ID: "b"}},
		{Op: embedding.OpUpsert, Doc: embedding.Document{
			ID: "c", Text: "Third", Vector: []float32{0.3, 0.4},
			Metadata: map[string]any{"title": "Third", "tag": nil, "views": int64(3)},
		}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Extract =\n%#v\nwant\n%#v", got, want)
	}
}

func TestExtractWithoutVectorField(t *testing.T) {
	b := et.Batch(t, "articles", et.Row{Op: rowmodel.OpInsert, ID: "a", Title: "First"})
	cfg := embedding.Config{PrimaryKey: et.ColumnID}

	got, err := embedding.Extract(b, cfg)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(got) != 1 || got[0].Doc.Vector != nil || got[0].Doc.Text != "" {
		t.Fatalf("Extract = %#v, want one upsert with no vector or text", got)
	}
	if _, ok := got[0].Doc.Metadata[et.ColumnEmbedding]; !ok {
		t.Fatal("unconfigured embedding column should be kept as metadata")
	}
}

func TestExtractMarkerBatch(t *testing.T) {
	marker := arrowbatch.NewMarker()
	defer marker.Release()

	got, err := embedding.Extract(marker, et.Config())
	if err != nil || got != nil {
		t.Fatalf("Extract(marker) = %v, %v; want nil, nil", got, err)
	}
}

func TestExtractRejectsBadBatches(t *testing.T) {
	good := et.Row{Op: rowmodel.OpInsert, ID: "ok", Vector: []float32{1, 2}}

	tests := []struct {
		name string
		cfg  embedding.Config
		row  et.Row
		want string
	}{
		{"missing primary key config", embedding.Config{}, good, "primary key is required"},
		{"vector field is primary key", embedding.Config{PrimaryKey: et.ColumnID, VectorField: et.ColumnID}, good, "cannot be the primary key"},
		{"blank text field", embedding.Config{PrimaryKey: et.ColumnID, TextFields: []string{" "}}, good, "text fields cannot be blank"},
		{"primary key column absent", embedding.Config{PrimaryKey: "nope"}, good, `column "nope" not found`},
		{"vector column absent", embedding.Config{PrimaryKey: et.ColumnID, VectorField: "nope"}, good, `column "nope" not found`},
		{"vector column not text", embedding.Config{PrimaryKey: et.ColumnID, VectorField: et.ColumnViews}, good, "must be text"},
		{"text column absent", embedding.Config{PrimaryKey: et.ColumnID, TextFields: []string{"nope"}}, good, `column "nope" not found`},
		{"empty primary key", et.Config(), et.Row{Op: rowmodel.OpInsert, Vector: []float32{1, 2}}, "row 1: primary key is empty"},
		{"null vector", et.Config(), et.Row{Op: rowmodel.OpInsert, ID: "x"}, "row 1: vector is null"},
		{"malformed vector", et.Config(), et.Row{Op: rowmodel.OpInsert, ID: "x", RawVector: "[0.1,,0.5]"}, "row 1: vector is not a JSON array"},
		{"dimension mismatch", et.Config(), et.Row{Op: rowmodel.OpInsert, ID: "x", Vector: []float32{1, 2, 3}}, "row 1: vector has 3 dimensions, earlier rows have 2"},
		{"unknown operation", et.Config(), et.Row{Op: rowmodel.Operation(99), ID: "x", Vector: []float32{1, 2}}, "row 1: unsupported operation 99"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The bad row comes second, so a passing first row proves the
			// whole batch is rejected rather than partly returned.
			b := et.Batch(t, "articles", good, tt.row)
			got, err := embedding.Extract(b, tt.cfg)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Extract error = %v, want it to contain %q", err, tt.want)
			}
			if got != nil {
				t.Fatalf("Extract returned %d mutations alongside an error", len(got))
			}
		})
	}
}

func TestExtractDeleteNeedsNoVector(t *testing.T) {
	b := et.Batch(t, "articles", et.Row{Op: rowmodel.OpDelete, ID: "gone"})
	got, err := embedding.Extract(b, et.Config())
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	want := []embedding.Mutation{{Op: embedding.OpDelete, Doc: embedding.Document{ID: "gone"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Extract = %#v, want %#v", got, want)
	}
}
