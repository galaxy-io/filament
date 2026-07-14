package pipeline

import (
	"encoding/json"
	"fmt"
)

// Operation represents the type of change a record represents.
type Operation int

// Change kinds a record can carry.
const (
	OperationSnapshot Operation = iota
	OperationCreate
	OperationUpdate
	OperationDelete
)

func (o Operation) String() string {
	switch o {
	case OperationSnapshot:
		return "snapshot"
	case OperationCreate:
		return "create"
	case OperationUpdate:
		return "update"
	case OperationDelete:
		return "delete"
	default:
		return "unknown"
	}
}

// StructuredData is the universal data carrier — schema-free map.
type StructuredData map[string]any

// Record is the pipeline's byte-oriented record format.
// All variable-length fields are stored as pre-serialized JSON bytes.
// The pipeline passes these through without deserialization — zero reflection.
type Record struct {
	Resource  string
	Operation Operation

	// Pre-serialized JSON bytes. Set once by the connector, never modified.
	KeyJSON      []byte // e.g. {"id":"abc","org":"xyz"}
	MetadataJSON []byte // e.g. {"source":"hubspot","resource":"contacts"}
	DataJSON     []byte // the payload — full record data as JSON bytes
	Projected    bool   // true when DataJSON already matches the advertised schema

	// Cursor is the pagination cursor at time of emission. Empty if not applicable.
	// Used for resumable extraction — persisted in ChunkCommitment.
	Cursor string
	// Watermarks carries resource-local incremental checkpoint values observed
	// while producing this record, keyed by manifest checkpoint key.
	Watermarks map[string]string

	// Lazily populated — only if field-level access is needed after construction.
	key      StructuredData
	metadata map[string]string
}

// NewRecord creates a Record from pre-serialized JSON bytes.
// Used by JSON-native connectors that already have the bytes.
func NewRecord(op Operation, keyJSON, metadataJSON, dataJSON []byte) Record {
	return Record{
		Operation:    op,
		KeyJSON:      keyJSON,
		MetadataJSON: metadataJSON,
		DataJSON:     dataJSON,
	}
}

// NewRecordFromStructured creates a Record by serializing structured data once.
// Used by connectors that build records from Go types (Google Drive, etc.).
func NewRecordFromStructured(op Operation, key StructuredData, metadata map[string]string, data StructuredData) (Record, error) {
	keyJSON, err := json.Marshal(key)
	if err != nil {
		return Record{}, fmt.Errorf("marshal key: %w", err)
	}
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		return Record{}, fmt.Errorf("marshal metadata: %w", err)
	}
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return Record{}, fmt.Errorf("marshal data: %w", err)
	}
	return Record{
		Operation:    op,
		KeyJSON:      keyJSON,
		MetadataJSON: metaJSON,
		DataJSON:     dataJSON,
	}, nil
}

// Key returns the decoded key, lazily deserializing from KeyJSON on first access.
func (r *Record) Key() (StructuredData, error) {
	if r.key == nil {
		if err := json.Unmarshal(r.KeyJSON, &r.key); err != nil {
			return nil, err
		}
	}
	return r.key, nil
}

// DecodedMetadata returns the decoded metadata, lazily deserializing on first access.
func (r *Record) DecodedMetadata() (map[string]string, error) {
	if r.metadata == nil {
		if err := json.Unmarshal(r.MetadataJSON, &r.metadata); err != nil {
			return nil, err
		}
	}
	return r.metadata, nil
}
