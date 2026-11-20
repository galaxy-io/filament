package embedding

import (
	"errors"
	"fmt"
	"strings"

	"github.com/apache/arrow-go/v18/arrow"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

// columns holds the position of each configured column in a batch schema.
type columns struct {
	key    int
	vector int // -1 when no vector field is configured
	text   []int
	meta   []int // every column except the key and the vector
}

// Extract converts every row of b into a Mutation, in row order.
//
// It validates the whole batch before returning: an unknown operation, a
// missing or empty primary key, or a missing or malformed vector on any row
// fails the batch with an error naming the row, and no mutations are returned.
// Vectors must also share one dimension across the batch. A marker or empty
// batch returns no mutations and no error.
func Extract(b *arrowbatch.Batch, cfg Config) ([]Mutation, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	n := b.NumRows()
	if n == 0 {
		return nil, nil
	}
	rows := b.Rows()
	cols, err := resolveColumns(rows.Schema(), cfg)
	if err != nil {
		return nil, fmt.Errorf("embedding: resource %q: %w", b.Resource, err)
	}

	out := make([]Mutation, 0, n)
	dim := 0
	for i := 0; i < n; i++ {
		m, err := extractRow(rows, b.Op(i), i, cols)
		if err != nil {
			return nil, fmt.Errorf("embedding: resource %q row %d: %w", b.Resource, i, err)
		}
		if m.Doc.Vector != nil {
			if dim == 0 {
				dim = len(m.Doc.Vector)
			} else if len(m.Doc.Vector) != dim {
				return nil, fmt.Errorf("embedding: resource %q row %d: vector has %d dimensions, earlier rows have %d",
					b.Resource, i, len(m.Doc.Vector), dim)
			}
		}
		out = append(out, m)
	}
	return out, nil
}

func resolveColumns(schema *arrow.Schema, cfg Config) (columns, error) {
	cols := columns{key: -1, vector: -1}
	index := func(name string) (int, error) {
		idx := schema.FieldIndices(name)
		if len(idx) == 0 {
			return -1, fmt.Errorf("column %q not found", name)
		}
		return idx[0], nil
	}

	var err error
	if cols.key, err = index(cfg.PrimaryKey); err != nil {
		return cols, fmt.Errorf("primary key: %w", err)
	}
	if cfg.VectorField != "" {
		if cols.vector, err = index(cfg.VectorField); err != nil {
			return cols, fmt.Errorf("vector field: %w", err)
		}
		switch schema.Field(cols.vector).Type.ID() {
		case arrow.STRING, arrow.LARGE_STRING:
		default:
			return cols, fmt.Errorf("vector field %q must be text holding a JSON array, got %s",
				cfg.VectorField, schema.Field(cols.vector).Type)
		}
	}
	for _, name := range cfg.TextFields {
		idx, err := index(name)
		if err != nil {
			return cols, fmt.Errorf("text field: %w", err)
		}
		cols.text = append(cols.text, idx)
	}
	for i := 0; i < schema.NumFields(); i++ {
		if i != cols.key && i != cols.vector {
			cols.meta = append(cols.meta, i)
		}
	}
	return cols, nil
}

func extractRow(rows arrow.RecordBatch, op rowmodel.Operation, i int, cols columns) (Mutation, error) {
	key := rows.Column(cols.key)
	if key.IsNull(i) {
		return Mutation{}, errors.New("primary key is null")
	}
	id := key.ValueStr(i)
	if id == "" {
		return Mutation{}, errors.New("primary key is empty")
	}

	switch op {
	case rowmodel.OpDelete:
		return Mutation{Op: OpDelete, Doc: Document{ID: id}}, nil
	case rowmodel.OpInsert, rowmodel.OpUpdate:
	default:
		return Mutation{}, fmt.Errorf("unsupported operation %d", op)
	}

	doc := Document{ID: id, Metadata: make(map[string]any, len(cols.meta))}
	if cols.vector >= 0 {
		col := rows.Column(cols.vector)
		if col.IsNull(i) {
			return Mutation{}, errors.New("vector is null")
		}
		vec, err := ParseVector(col.ValueStr(i))
		if err != nil {
			return Mutation{}, err
		}
		doc.Vector = vec
	}

	var text []string
	for _, idx := range cols.text {
		if col := rows.Column(idx); !col.IsNull(i) {
			text = append(text, col.ValueStr(i))
		}
	}
	doc.Text = strings.Join(text, "\n")

	for _, idx := range cols.meta {
		doc.Metadata[rows.Schema().Field(idx).Name] = rows.Column(idx).GetOneForMarshal(i)
	}
	return Mutation{Op: OpUpsert, Doc: doc}, nil
}
