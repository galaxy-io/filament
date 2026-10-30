package embedding_test

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/connectors/internal/embedding"
	"github.com/galaxy-io/filament/rowmodel"
)

// Column names in batches built by testBatch.
const (
	colID        = "id"
	colTitle     = "title"
	colTag       = "tag"
	colViews     = "views"
	colEmbedding = "embedding"
)

// testRow is one row of a batch built by testBatch.
type testRow struct {
	op    rowmodel.Operation
	id    string
	title string
	tag   string // stored as null when empty
	views int64
	// vector is encoded as a JSON array; nil is stored as null.
	vector []float32
	// rawVector, when set, is stored verbatim instead of vector, for
	// testing malformed input.
	rawVector string
}

// testConfig is the embedding configuration matching testBatch's schema.
func testConfig() embedding.Config {
	return embedding.Config{
		PrimaryKey:  colID,
		VectorField: colEmbedding,
		TextFields:  []string{colTitle},
	}
}

// testBatch builds an owned batch of rows, released when the test ends.
func testBatch(t *testing.T, rows ...testRow) *arrowbatch.Batch {
	t.Helper()
	schema := rowmodel.Schema{
		Resource:   "articles",
		PrimaryKey: []string{colID},
		Fields: []rowmodel.Field{
			{Name: colID, Logical: rowmodel.LogicalString},
			{Name: colTitle, Logical: rowmodel.LogicalString},
			{Name: colTag, Logical: rowmodel.LogicalString, Nullable: true},
			{Name: colViews, Logical: rowmodel.LogicalInt64},
			{Name: colEmbedding, Logical: rowmodel.LogicalArray, Nullable: true},
		},
	}
	builder := array.NewRecordBuilder(memory.DefaultAllocator, arrowbatch.Schema(schema))
	defer builder.Release()

	id := builder.Field(0).(*array.StringBuilder)
	title := builder.Field(1).(*array.StringBuilder)
	tag := builder.Field(2).(*array.StringBuilder)
	views := builder.Field(3).(*array.Int64Builder)
	vector := builder.Field(4).(*array.StringBuilder)

	ops := make([]rowmodel.Operation, 0, len(rows))
	for _, r := range rows {
		ops = append(ops, r.op)
		id.Append(r.id)
		title.Append(r.title)
		if r.tag == "" {
			tag.AppendNull()
		} else {
			tag.Append(r.tag)
		}
		views.Append(r.views)
		switch {
		case r.rawVector != "":
			vector.Append(r.rawVector)
		case r.vector == nil:
			vector.AppendNull()
		default:
			vector.Append(encodeVector(r.vector))
		}
	}

	b := arrowbatch.NewBatch(builder.NewRecordBatch(), arrowbatch.NewOperations(ops))
	b.Resource = schema.Resource
	t.Cleanup(b.Release)
	return b
}

// encodeVector formats v as a JSON array, the form Filament carries arrays in.
func encodeVector(v []float32) string {
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = strconv.FormatFloat(float64(f), 'g', -1, 32)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func TestExtractMixedOperations(t *testing.T) {
	b := testBatch(t,
		testRow{op: rowmodel.OpInsert, id: "a", title: "First", tag: "news", views: 1, vector: []float32{0.1, 0.2}},
		testRow{op: rowmodel.OpDelete, id: "b"},
		testRow{op: rowmodel.OpUpdate, id: "c", title: "Third", views: 3, vector: []float32{0.3, 0.4}},
	)

	got, err := embedding.Extract(b, testConfig())
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
	b := testBatch(t, testRow{op: rowmodel.OpInsert, id: "a", title: "First"})
	cfg := embedding.Config{PrimaryKey: colID}

	got, err := embedding.Extract(b, cfg)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(got) != 1 || got[0].Doc.Vector != nil || got[0].Doc.Text != "" {
		t.Fatalf("Extract = %#v, want one upsert with no vector or text", got)
	}
	if _, ok := got[0].Doc.Metadata[colEmbedding]; !ok {
		t.Fatal("unconfigured embedding column should be kept as metadata")
	}
}

func TestExtractMarkerBatch(t *testing.T) {
	marker := arrowbatch.NewMarker()
	defer marker.Release()

	got, err := embedding.Extract(marker, testConfig())
	if err != nil || got != nil {
		t.Fatalf("Extract(marker) = %v, %v; want nil, nil", got, err)
	}
}

func TestExtractRejectsBadBatches(t *testing.T) {
	good := testRow{op: rowmodel.OpInsert, id: "ok", vector: []float32{1, 2}}

	tests := []struct {
		name string
		cfg  embedding.Config
		row  testRow
		want string
	}{
		{"missing primary key config", embedding.Config{}, good, "primary key is required"},
		{"vector field is primary key", embedding.Config{PrimaryKey: colID, VectorField: colID}, good, "cannot be the primary key"},
		{"blank text field", embedding.Config{PrimaryKey: colID, TextFields: []string{" "}}, good, "text fields cannot be blank"},
		{"primary key column absent", embedding.Config{PrimaryKey: "nope"}, good, `column "nope" not found`},
		{"vector column absent", embedding.Config{PrimaryKey: colID, VectorField: "nope"}, good, `column "nope" not found`},
		{"vector column not text", embedding.Config{PrimaryKey: colID, VectorField: colViews}, good, "must be text"},
		{"text column absent", embedding.Config{PrimaryKey: colID, TextFields: []string{"nope"}}, good, `column "nope" not found`},
		{"empty primary key", testConfig(), testRow{op: rowmodel.OpInsert, vector: []float32{1, 2}}, "row 1: primary key is empty"},
		{"null vector", testConfig(), testRow{op: rowmodel.OpInsert, id: "x"}, "row 1: vector is null"},
		{"malformed vector", testConfig(), testRow{op: rowmodel.OpInsert, id: "x", rawVector: "[0.1,,0.5]"}, "row 1: vector is not a JSON array"},
		{"dimension mismatch", testConfig(), testRow{op: rowmodel.OpInsert, id: "x", vector: []float32{1, 2, 3}}, "row 1: vector has 3 dimensions, earlier rows have 2"},
		{"unknown operation", testConfig(), testRow{op: rowmodel.Operation(99), id: "x", vector: []float32{1, 2}}, "row 1: unsupported operation 99"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The bad row comes second, so a passing first row proves the
			// whole batch is rejected rather than partly returned.
			b := testBatch(t, good, tt.row)
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
	b := testBatch(t, testRow{op: rowmodel.OpDelete, id: "gone"})
	got, err := embedding.Extract(b, testConfig())
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	want := []embedding.Mutation{{Op: embedding.OpDelete, Doc: embedding.Document{ID: "gone"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Extract = %#v, want %#v", got, want)
	}
}
