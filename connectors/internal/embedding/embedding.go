// Package embedding provides shared utilities for extracting text payloads,
// primary keys, pre-computed vectors, and metadata from Arrow record batches
// for vector database sink connectors.
package embedding

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/galaxy-io/filament/rowmodel"
)

// VectorDoc represents a single document payload for vector store ingestion.
type VectorDoc struct {
	// ID is the unique primary key of the vector document.
	ID string `json:"id"`

	// Vector contains the floating point array representation of the document embedding.
	Vector []float32 `json:"vector,omitempty"`

	// Document contains the original textual content of the payload.
	Document string `json:"document,omitempty"`

	// Metadata holds arbitrary key-value attributes associated with the document.
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Operation specifies the change operation kind ("insert", "update", or "delete").
	Operation string `json:"operation"`
}

// OpString converts a rowmodel.Operation enum to its string representation ("insert", "update", "delete").
// Returns an error if the operation kind is unknown.
func OpString(op rowmodel.Operation) (string, error) {
	switch op {
	case rowmodel.OpInsert:
		return "insert", nil
	case rowmodel.OpUpdate:
		return "update", nil
	case rowmodel.OpDelete:
		return "delete", nil
	default:
		return "", fmt.Errorf("unsupported row operation kind: %d", op)
	}
}

// FindColumnIndex locates a column index by name in schema, returning -1 if absent.
func FindColumnIndex(schema *arrow.Schema, targetCol string) int {
	if schema == nil || strings.TrimSpace(targetCol) == "" {
		return -1
	}

	for i, field := range schema.Fields() {
		if strings.EqualFold(field.Name, targetCol) {
			return i
		}
	}

	return -1
}

// ExtractDocID extracts the non-empty primary key string value from the specified column for row i.
func ExtractDocID(rows arrow.RecordBatch, rowIdx int, colIdx int) (string, error) {
	if rows == nil || colIdx < 0 || colIdx >= int(rows.NumCols()) {
		return "", fmt.Errorf("invalid column index %d for schema", colIdx)
	}

	col := rows.Column(colIdx)
	if col == nil || col.IsNull(rowIdx) {
		return "", fmt.Errorf("primary key value at row %d is null", rowIdx)
	}

	valStr := strings.TrimSpace(col.ValueStr(rowIdx))
	if valStr == "" {
		return "", fmt.Errorf("primary key value at row %d is empty", rowIdx)
	}

	return valStr, nil
}

// ExtractDocumentText concatenates values from specified text fields for row i.
func ExtractDocumentText(rows arrow.RecordBatch, rowIdx int, textFields []string) string {
	if rows == nil {
		return ""
	}

	schema := rows.Schema()
	var parts []string

	if len(textFields) > 0 {
		for _, fieldName := range textFields {
			for idx, f := range schema.Fields() {
				if strings.EqualFold(f.Name, fieldName) {
					col := rows.Column(idx)
					if col != nil && !col.IsNull(rowIdx) {
						parts = append(parts, col.ValueStr(rowIdx))
					}
					break
				}
			}
		}
	} else {
		// Fallback: concatenate all string columns
		for idx, f := range schema.Fields() {
			if f.Type.ID() == arrow.STRING {
				col := rows.Column(idx)
				if col != nil && !col.IsNull(rowIdx) {
					parts = append(parts, col.ValueStr(rowIdx))
				}
			}
		}
	}

	return strings.Join(parts, " ")
}

// ExtractEmbeddingVector extracts a []float32 array from the embedding column if present.
func ExtractEmbeddingVector(rows arrow.RecordBatch, rowIdx int, embeddingCol string) ([]float32, error) {
	if rows == nil || embeddingCol == "" {
		return nil, nil
	}

	schema := rows.Schema()
	for idx, f := range schema.Fields() {
		if strings.EqualFold(f.Name, embeddingCol) {
			col := rows.Column(idx)
			if col == nil || col.IsNull(rowIdx) {
				return nil, nil
			}
			valStr := col.ValueStr(rowIdx)
			return ParseFloats(valStr)
		}
	}

	return nil, nil
}

// ExtractMetadataMap extracts non-vector column values into a key-value metadata map.
func ExtractMetadataMap(rows arrow.RecordBatch, rowIdx int, embeddingCol string, resource string, seq uint64) map[string]interface{} {
	meta := map[string]interface{}{
		"resource": resource,
		"seq":      seq,
	}

	if rows == nil {
		return meta
	}

	schema := rows.Schema()
	for idx, f := range schema.Fields() {
		if strings.EqualFold(f.Name, embeddingCol) {
			continue
		}
		col := rows.Column(idx)
		if col != nil && !col.IsNull(rowIdx) {
			meta[f.Name] = col.ValueStr(rowIdx)
		}
	}

	return meta
}

// ParseFloats converts a string formatted float list (e.g. "[0.1, 0.2]") into a []float32 slice.
// Returns an error if any float token fails to parse.
func ParseFloats(s string) ([]float32, error) {
	clean := strings.Trim(s, "[]{} ")
	if clean == "" {
		return nil, nil
	}

	tokens := strings.Split(clean, ",")
	res := make([]float32, 0, len(tokens))

	for _, tok := range tokens {
		t := strings.TrimSpace(tok)
		if t == "" {
			continue
		}
		val, err := strconv.ParseFloat(t, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid float value %q in embedding vector %q: %w", t, s, err)
		}
		res = append(res, float32(val))
	}

	return res, nil
}
