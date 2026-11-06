// Package embedding turns Filament row batches into vector-store mutations.
//
// It is shared by the vector database sinks (Milvus, Qdrant, Pinecone, ...) so
// they agree on how a row becomes a document: which column is the identity,
// which holds the embedding, which text is indexed, and how metadata is kept
// from overwriting the fields a store reserves for itself.
//
// Extract validates a whole batch before returning anything, so a sink can
// finish every check before it makes its first destructive call.
package embedding

import (
	"errors"
	"strings"
)

// Op is the vector-store action for one row.
type Op uint8

// Vector-store actions. Inserts and updates both become upserts, because a
// vector store replaces a point by ID rather than patching it.
const (
	OpUpsert Op = iota + 1
	OpDelete
)

// Document is one row prepared for a vector store.
type Document struct {
	// ID is the source primary key as text. Sinks map it to the identifier
	// their store accepts with the helpers in ids.go.
	ID string
	// Text is the configured text fields joined by newlines. Empty when no
	// text fields are configured.
	Text string
	// Vector is the row's embedding. Nil for deletes and when no vector
	// field is configured.
	Vector []float32
	// Metadata holds every other column, keyed by column name, with values
	// ready to marshal as JSON. Nil for deletes.
	Metadata map[string]any
}

// Mutation is one validated change to apply to a vector store, in row order.
type Mutation struct {
	Op  Op
	Doc Document
}

// Config names the columns that play a role in a vector document.
type Config struct {
	// PrimaryKey is the column whose value identifies the document. Required.
	PrimaryKey string
	// VectorField is the column holding a precomputed embedding as a JSON
	// array of numbers. When set, every upserted row must have one.
	VectorField string
	// TextFields are the columns joined into Document.Text, in order.
	TextFields []string
}

// Validate reports whether the configuration can be used to extract rows.
func (c Config) Validate() error {
	if strings.TrimSpace(c.PrimaryKey) == "" {
		return errors.New("embedding: primary key is required")
	}
	if c.VectorField != "" && c.VectorField == c.PrimaryKey {
		return errors.New("embedding: vector field cannot be the primary key")
	}
	for _, f := range c.TextFields {
		if strings.TrimSpace(f) == "" {
			return errors.New("embedding: text fields cannot be blank")
		}
	}
	return nil
}
