// Package embeddingtest builds row batches for testing vector-store sinks, so
// every sink is exercised against the same mix of inserts, updates and deletes.
package embeddingtest

import (
	"strconv"
	"strings"
	"testing"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/connectors/internal/embedding"
	"github.com/galaxy-io/filament/rowmodel"
)

// Column names in batches built by Batch.
const (
	ColumnID        = "id"
	ColumnTitle     = "title"
	ColumnTag       = "tag"
	ColumnViews     = "views"
	ColumnEmbedding = "embedding"
)

// Row is one row of a test batch.
type Row struct {
	Op    rowmodel.Operation
	ID    string
	Title string
	// Tag is stored as null when empty.
	Tag   string
	Views int64
	// Vector is encoded as a JSON array. Nil is stored as null.
	Vector []float32
	// RawVector, when set, is stored as the embedding text verbatim instead of
	// Vector, for testing malformed input.
	RawVector string
}

// Schema is the row model of batches built by Batch.
func Schema(resource string) rowmodel.Schema {
	return rowmodel.Schema{
		Resource:   resource,
		PrimaryKey: []string{ColumnID},
		Fields: []rowmodel.Field{
			{Name: ColumnID, Logical: rowmodel.LogicalString},
			{Name: ColumnTitle, Logical: rowmodel.LogicalString},
			{Name: ColumnTag, Logical: rowmodel.LogicalString, Nullable: true},
			{Name: ColumnViews, Logical: rowmodel.LogicalInt64},
			{Name: ColumnEmbedding, Logical: rowmodel.LogicalArray, Nullable: true},
		},
	}
}

// Config is the embedding configuration matching Schema.
func Config() embedding.Config {
	return embedding.Config{
		PrimaryKey:  ColumnID,
		VectorField: ColumnEmbedding,
		TextFields:  []string{ColumnTitle},
	}
}

// Batch builds an owned batch of rows for resource. It is released when the
// test ends.
func Batch(t testing.TB, resource string, rows ...Row) *arrowbatch.Batch {
	t.Helper()
	builder := array.NewRecordBuilder(memory.DefaultAllocator, arrowbatch.Schema(Schema(resource)))
	defer builder.Release()

	id := builder.Field(0).(*array.StringBuilder)
	title := builder.Field(1).(*array.StringBuilder)
	tag := builder.Field(2).(*array.StringBuilder)
	views := builder.Field(3).(*array.Int64Builder)
	vector := builder.Field(4).(*array.StringBuilder)

	ops := make([]rowmodel.Operation, 0, len(rows))
	for _, r := range rows {
		ops = append(ops, r.Op)
		id.Append(r.ID)
		title.Append(r.Title)
		if r.Tag == "" {
			tag.AppendNull()
		} else {
			tag.Append(r.Tag)
		}
		views.Append(r.Views)
		switch {
		case r.RawVector != "":
			vector.Append(r.RawVector)
		case r.Vector == nil:
			vector.AppendNull()
		default:
			vector.Append(EncodeVector(r.Vector))
		}
	}

	b := arrowbatch.NewBatch(builder.NewRecordBatch(), arrowbatch.NewOperations(ops))
	b.Resource = resource
	t.Cleanup(b.Release)
	return b
}

// EncodeVector formats v as a JSON array, the form Filament carries arrays in.
func EncodeVector(v []float32) string {
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = strconv.FormatFloat(float64(f), 'g', -1, 32)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
