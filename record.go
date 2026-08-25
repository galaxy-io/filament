package filament

import (
	"encoding/binary"
	"hash/crc32"
	"maps"
	"strconv"
	"time"
)

// Record is one row (or change) moving through the pipeline: an opaque Data
// payload plus the identity and resume metadata the engine routes on.
type Record struct {
	Resource string
	ID       string
	Op       Operation
	Data     []byte
	Meta     RecordMeta

	// Part and Key are resume metadata, set only by a keyset (Resumable) read and
	// ignored by the integrity CRC (AppendCanonical covers id/op/data only). Part is
	// the source shard the record came from; Key is its primary-key value(s) as text,
	// the keyset cursor the batcher carries onto the batch so progress can be persisted.
	Part int
	Key  []string

	// Coarse marks a record from a bitmap (or other ack-counted) read: its Part has no
	// monotonic key cursor, so the batcher checkpoints it by counting written rows per
	// part instead of advancing a key. Mutually exclusive with a non-nil Key.
	Coarse bool

	// Drained is a control sentinel, not a data row: it carries Part (and Coarse) but no
	// Data, and signals that the source has pushed every row of that part. The batcher
	// turns it into the part's completion marker (expected-row count) and never writes or
	// CRCs it. Always pushed after all of the part's data records, on one FIFO inlet.
	Drained bool
}

// NewRecord builds an insert record — the common case for snapshot sources.
func NewRecord(resource, id string, data []byte) Record {
	return Record{Resource: resource, ID: id, Op: OpInsert, Data: data}
}

// Operation is the change kind a record carries.
type Operation int

// The record operations.
const (
	OpInsert Operation = iota
	OpUpdate
	OpDelete
)

// RecordMeta carries source-assigned provenance: CDC position, sequence, and
// emit time.
type RecordMeta struct {
	LSN       string
	Seq       uint64
	EmittedAt time.Time
}

// RecordSchema is one resource's column layout, as a source reports it and a
// Schematized sink consumes it.
type RecordSchema struct {
	Resource   string
	Fields     []SchemaField
	PrimaryKey []string // column names in key order; empty when the resource has none
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
}

// Batch is the unit of writing: a sequenced group of one resource's records
// with the read-side CRC the writer verifies against.
type Batch struct {
	Tenant   TenantID
	Run      RunID
	Resource string
	Seq      uint64
	Records  []Record
	ReadCRC  uint32

	// Part is the source shard this batch drains (0 for non-keyset reads). Cursor, when
	// set, is the per-shard checkpoint delta — this shard's advanced keyset position, or a
	// bitmap ack/want count — which the writer carries on the batch.written fact for the
	// tracker to persist.
	Part   int
	Cursor *CheckpointData

	// Drained marks a bitmap completion marker: it carries the part's expected-row count
	// in Cursor but no Records. The writer emits its fact for the tracker and performs no
	// sink write.
	Drained bool
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

// CheckpointKind describes a cursor's shape without exposing its value.
func CheckpointKind(cp Checkpoint) string {
	if cp == nil {
		return "none"
	}
	if mode, ok := cp.Raw()["mode"].(string); ok && mode != "" {
		return mode
	}
	return "cursor"
}

// crcTable uses the Castagnoli polynomial, which has hardware acceleration on
// both ARM64 (CRC32 instructions) and x86 (SSE4.2).
var crcTable = crc32.MakeTable(crc32.Castagnoli)

// The canonical integrity encoding of a record is the byte string
//
//	{"id":<id>,"op":<op>,"data":<data>}
//
// where <id> is JSON-string-escaped, <op> is its decimal digits, and <data> is the
// payload appended verbatim. This is a deterministic, injective hash preimage — fed
// only to CRC32C, never parsed back — so <data> needs no validation or escaping: the
// escaped (so unambiguously terminated) <id> and digit-delimited <op> keep the field
// boundaries unique whatever bytes the payload holds. It is therefore NOT guaranteed
// to be syntactically valid JSON; a sink that needs valid JSON on the wire serializes
// records itself (see the stdout sink).

// AppendCanonical appends the record's canonical integrity encoding to dst and
// returns the extended buffer. With spare capacity in dst it allocates nothing,
// so hashing a batch can drive it from a single reused scratch buffer.
func (r Record) AppendCanonical(dst []byte) []byte {
	dst = append(dst, `{"id":`...)
	dst = appendJSONString(dst, r.ID)
	dst = append(dst, `,"op":`...)
	dst = strconv.AppendInt(dst, int64(r.Op), 10)
	dst = append(dst, `,"data":`...)
	dst = append(dst, r.Data...)
	return append(dst, '}')
}

// CanonicalBytes returns a freshly allocated canonical encoding. Prefer
// AppendCanonical with a reused scratch buffer on hot paths; this allocates a
// right-sized buffer up front so the common (valid-JSON) case is a single alloc.
func (r Record) CanonicalBytes() []byte {
	// Envelope {"id":"","op":0,"data":} is ~23 bytes; 32 covers it plus op digits.
	return r.AppendCanonical(make([]byte, 0, len(r.ID)+len(r.Data)+32))
}

// CRC32C returns the running Castagnoli CRC over records and their total
// canonical byte count. Each record's canonical encoding is fed with a
// little-endian length prefix, so regrouping or reordering records changes the
// result (the chunks ["ab","c"] and ["a","bc"] do not collide).
func CRC32C(records []Record) (crc uint32, bytes int64) {
	var lenBuf [4]byte
	var scratch []byte
	for i := range records {
		scratch = records[i].AppendCanonical(scratch[:0])
		binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(scratch))) //nolint:gosec // one record's encoding, never near 4GiB
		crc = crc32.Update(crc, crcTable, lenBuf[:])
		crc = crc32.Update(crc, crcTable, scratch)
		bytes += int64(len(scratch))
	}
	return crc, bytes
}

const hexDigits = "0123456789abcdef"

// The following functions seem odd but are to avoid constantly json.Marshalling which requires reflection
// and is very much a bottle neck when ran on every batch
func appendJSONString(dst []byte, s string) []byte {
	dst = append(dst, '"')
	start := 0
	for i := range len(s) {
		b := s[i]
		if b >= 0x20 && b != '"' && b != '\\' {
			continue
		}
		dst = append(dst, s[start:i]...)
		dst = appendEscape(dst, b)
		start = i + 1
	}
	dst = append(dst, s[start:]...)
	return append(dst, '"')
}

// appendEscape writes the JSON escape for a single character that needs one.
func appendEscape(dst []byte, b byte) []byte {
	switch b {
	case '"':
		return append(dst, '\\', '"')
	case '\\':
		return append(dst, '\\', '\\')
	case '\n':
		return append(dst, '\\', 'n')
	case '\r':
		return append(dst, '\\', 'r')
	case '\t':
		return append(dst, '\\', 't')
	default:
		return append(dst, '\\', 'u', '0', '0', hexDigits[b>>4], hexDigits[b&0xF])
	}
}
