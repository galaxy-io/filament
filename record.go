package filament

import (
	"maps"
	"strconv"

	"github.com/apache/arrow-go/v18/arrow"
)

// Operation is the change kind a row carries.
type Operation uint8

// The row operations.
const (
	OpInsert Operation = iota
	OpUpdate
	OpDelete
)

// RecordSchema is one resource's column layout, as a source reports it and a
// Schematized sink consumes it.
type RecordSchema struct {
	Resource   string
	Fields     []SchemaField
	PrimaryKey []string // column names in key order; empty when the resource has none

	// Engine names the source engine that produced the schema ("postgres",
	// "mysql", ...). A sink on the same engine may reuse each field's Native
	// spelling verbatim; any other sink maps from Logical.
	Engine string
}

// SchemaField describes one column: name, nullability, and its portable plus
// native types.
type SchemaField struct {
	Name     string
	Nullable bool

	// Logical is the portable type used for cross-engine mapping; Native is the
	// source's own type spelling (e.g. Postgres format_type "numeric(12,2)"), reused
	// verbatim by a same-engine sink for a perfect round-trip.
	Logical LogicalType
	Native  string

	// Precision and Scale bound a decimal column. Precision 0 means unbounded:
	// the value travels as text and lands as an unbounded numeric.
	Precision int
	Scale     int
}

// Batch is the unit of writing: a sequenced chunk of one resource's rows as an
// Arrow record batch, plus each row's operation.
type Batch struct {
	Tenant   TenantID
	Run      RunID
	Resource string
	Seq      uint64

	// Rows holds the chunk's rows in the schema the source's builder was opened
	// with (see batch.Schema for the LogicalType mapping). The writer releases it
	// after Apply; a sink that keeps rows past Apply must Retain them. Nil on a
	// Drained marker.
	Rows arrow.RecordBatch

	// Ops holds one Operation per row. Nil means every row is an insert.
	Ops []Operation

	// Part is the source shard this batch drains (0 for non-keyset reads). Cursor, when
	// set, is the per-shard checkpoint delta — this shard's advanced keyset position, or a
	// bitmap ack/want count — which the writer carries on the batch.written fact for the
	// tracker to persist.
	Part   int
	Cursor *CheckpointData

	// Drained marks a completion marker: it carries the part's expected-row count
	// or final stream position in Cursor but no Rows. The writer emits its fact for
	// the tracker and performs no sink write.
	Drained bool
}

// NumRows returns the row count, 0 for a marker.
func (b Batch) NumRows() int {
	if b.Rows == nil {
		return 0
	}
	return int(b.Rows.NumRows())
}

// Op returns row i's operation.
func (b Batch) Op(i int) Operation {
	if b.Ops == nil {
		return OpInsert
	}
	return b.Ops[i]
}

// SeqString renders Seq as the decimal token used in subjects and dedup keys.
func (b Batch) SeqString() string { return strconv.FormatUint(b.Seq, 10) }

// WriteReceipt is what a sink returns per Apply: where the data landed, how
// much, and the write-side CRC for integrity comparison.
type WriteReceipt struct {
	URI        string
	Bytes      int64
	Rows       int
	WriteCRC   uint32
	Checkpoint *CheckpointData
}

// IntegrityResult is the outcome of comparing a batch's read and write CRCs.
type IntegrityResult struct {
	OK       bool
	Resource string
	Seq      uint64
	ReadCRC  uint32
	WriteCRC uint32
}

// CheckpointData is the concrete backing for the Checkpoint interface — a
// resumable cursor. It implements Checkpoint with value-copy semantics on Set.
type CheckpointData struct {
	ResourceName string         `json:"resource"`
	Cursor       map[string]any `json:"cursor"` // {"page":12} | {"lsn":"0/1A2B"} | {"updated_at":...}
}

var _ Checkpoint = (*CheckpointData)(nil)

// NewCheckpoint returns an empty checkpoint for a resource.
func NewCheckpoint(resource string) *CheckpointData {
	return &CheckpointData{ResourceName: resource, Cursor: map[string]any{}}
}

// Resource returns the resource this cursor belongs to.
func (c *CheckpointData) Resource() string { return c.ResourceName }

// Int reads key as an int, tolerating the float64/int64 forms JSON round-trips produce.
func (c *CheckpointData) Int(key string) int {
	switch n := c.Cursor[key].(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func (c *CheckpointData) String(key string) string {
	if s, ok := c.Cursor[key].(string); ok {
		return s
	}
	return ""
}

// Set returns a copy with key updated; the receiver is not mutated.
func (c *CheckpointData) Set(key string, v any) Checkpoint {
	next := make(map[string]any, len(c.Cursor)+1)
	maps.Copy(next, c.Cursor)
	next[key] = v
	return &CheckpointData{ResourceName: c.ResourceName, Cursor: next}
}

// Raw exposes the underlying cursor map for persistence.
func (c *CheckpointData) Raw() map[string]any { return c.Cursor }
