// Package checkpoint encodes and decodes resumable-read checkpoints:
// keyset (key-order pages), bitmap (unordered sub-range), and ctid (physical block range).
//
//	{
//	  "mode":  "keyset",
//	  "cols":  ["id"],          // primary-key columns, in key order
//	  "types": ["bigint"],      // their Postgres types, for cursor casts
//	  "shards": [
//	    {"lo": ["1"],      "hi": ["250000"], "key": null},        // not started
//	    {"lo": ["250000"], "hi": null,       "key": ["843210"]}   // resumes at 843210; hi open
//	  ]
//	}
//
// lo/hi/key are primary-key tuples rendered as text (one element per column), or null
// for an open bound / un-started shard. A null hi marks the open-ended top shard.
package checkpoint

import (
	"encoding/json"
	"strconv"

	"github.com/galaxy-io/filament"
)

// The checkpoint cursor modes a source can persist.
const (
	ModeKeyset = "keyset"
	ModeBitmap = "bitmap"
	ModeCtid   = "ctid"
	// ModeIncremental is an ordered source watermark followed by a complete
	// primary-key tie-breaker. It is durable across scheduled runs.
	ModeIncremental = "incremental"
	// ModeIncrementalBackfill is a resumable, sharded initial snapshot that
	// promotes to its captured incremental start watermark on completion.
	ModeIncrementalBackfill = "incremental_backfill"
	// ModeStream is a change-stream position cursor: a single monotonic location in
	// the source's replication log (Postgres WAL LSN, MySQL binlog file:pos) plus a
	// per-run sequence guard. Unlike the shard-based cursors above it has no layout;
	// resume simply restarts the stream at the recorded position.
	ModeStream = "stream"
)

// DeltaKind identifies the progress operation carried by a batch checkpoint.
// Plans and durable checkpoints remain connector-shaped; deltas use this small
// typed vocabulary so the pipeline and coordinator do not independently sniff
// sentinel fields.
type DeltaKind uint8

// Supported checkpoint delta kinds.
const (
	DeltaUnknown DeltaKind = iota
	DeltaShard
	DeltaStream
	DeltaCoarse
)

// Delta is the decoded progress operation for one written batch.
type Delta struct {
	Kind    DeltaKind
	Part    int
	Key     []string
	LSN     string
	Seq     uint64
	Ack     int
	Want    int
	HasWant bool
}

// DecodeDelta classifies and decodes a batch checkpoint in one place.
func DecodeDelta(cp filament.Checkpoint) Delta {
	if cp == nil {
		return Delta{}
	}
	raw := cp.Raw()
	switch mode, _ := raw["mode"].(string); mode {
	case ModeStream:
		lsn, seq, ok := ParseStream(cp)
		if ok {
			return Delta{Kind: DeltaStream, LSN: lsn, Seq: seq}
		}
	case coarseDeltaTag:
		part, ack, want, hasWant, ok := CoarseDelta(cp)
		if ok {
			return Delta{Kind: DeltaCoarse, Part: part, Ack: ack, Want: want, HasWant: hasWant}
		}
	case ModeKeyset:
		return Delta{Kind: DeltaShard, Part: anyToInt(raw["part"]), Key: anyToStrs(raw["key"])}
	}
	// Legacy stream deltas carried lsn/seq without a mode.
	if lsn, seq, ok := ParseStream(cp); ok {
		return Delta{Kind: DeltaStream, LSN: lsn, Seq: seq}
	}
	return Delta{}
}

// KeysetShard is one independently-resumable slice of a resource's primary-key space.
type KeysetShard struct {
	Lo  []string
	Hi  []string
	Key []string // resume cursor; nil = not started
	// Done marks a bitmap/ctid shard fully read and written. Resume skips Done shards.
	Done bool
}

// KeysetCheckpoint is the decoded cursor for one resource — keyset, bitmap, or ctid,
// distinguished by Mode. All share the shard layout; per-shard resume is a Key cursor
// (keyset) or a Done flag (bitmap/ctid). Meta carries ctid stamps; nil otherwise.
type KeysetCheckpoint struct {
	Mode   string
	Cols   []string
	Types  []string
	Shards []KeysetShard
	Meta   map[string]string
}

// ToCheckpoint encodes the plan as a CheckpointData for persistence/transport.
func (k KeysetCheckpoint) ToCheckpoint(resource string) *filament.CheckpointData {
	mode := k.Mode
	if mode == "" {
		mode = ModeKeyset
	}
	shards := make([]any, len(k.Shards))
	for i, sh := range k.Shards {
		shards[i] = map[string]any{
			"lo":   strsToAny(sh.Lo),
			"hi":   strsToAny(sh.Hi),
			"key":  strsToAny(sh.Key),
			"done": sh.Done,
		}
	}
	cursor := map[string]any{
		"mode":   mode,
		"cols":   strsToAny(k.Cols),
		"types":  strsToAny(k.Types),
		"shards": shards,
	}
	if len(k.Meta) > 0 {
		meta := make(map[string]any, len(k.Meta))
		for kk, vv := range k.Meta {
			meta[kk] = vv
		}
		cursor["meta"] = meta
	}
	return &filament.CheckpointData{ResourceName: resource, Cursor: cursor}
}

// ParseKeyset decodes a key-space checkpoint (keyset, bitmap, or ctid). Reports false when cp
// is nil or is a different cursor shape. Tolerates both in-process ([]string) and
// JSON-round-tripped ([]any) encodings.
func ParseKeyset(cp filament.Checkpoint) (KeysetCheckpoint, bool) {
	if cp == nil {
		return KeysetCheckpoint{}, false
	}
	raw := cp.Raw()
	mode, _ := raw["mode"].(string)
	if mode != ModeKeyset && mode != ModeBitmap && mode != ModeCtid && mode != ModeIncremental && mode != ModeIncrementalBackfill {
		return KeysetCheckpoint{}, false
	}
	out := KeysetCheckpoint{Mode: mode, Cols: anyToStrs(raw["cols"]), Types: anyToStrs(raw["types"]), Meta: anyToStrMap(raw["meta"])}
	for _, s := range anySlice(raw["shards"]) {
		m, _ := s.(map[string]any)
		done, _ := m["done"].(bool)
		out.Shards = append(out.Shards, KeysetShard{
			Lo:   anyToStrs(m["lo"]),
			Hi:   anyToStrs(m["hi"]),
			Key:  anyToStrs(m["key"]),
			Done: done,
		})
	}
	return out, true
}

const (
	metaIncrementalCols     = "incremental_cols"
	metaIncrementalTypes    = "incremental_types"
	metaIncrementalStart    = "incremental_start"
	metaIncrementalLookback = "incremental_lookback_seconds"
)

// AsIncrementalBackfill annotates a keyset shard plan with the logical
// watermark captured before the initial snapshot began.
func AsIncrementalBackfill(plan KeysetCheckpoint, cols, types, start []string, lookbackSeconds int) KeysetCheckpoint {
	plan.Mode = ModeIncrementalBackfill
	if plan.Meta == nil {
		plan.Meta = map[string]string{}
	}
	plan.Meta[metaIncrementalCols] = encodeStrings(cols)
	plan.Meta[metaIncrementalTypes] = encodeStrings(types)
	plan.Meta[metaIncrementalStart] = encodeStrings(start)
	plan.Meta[metaIncrementalLookback] = strconv.Itoa(lookbackSeconds)
	return plan
}

// PromoteIncrementalBackfill replaces a completed shard plan with the logical
// watermark that seeds subsequent scheduled incremental runs.
func PromoteIncrementalBackfill(cp filament.Checkpoint) (filament.Checkpoint, bool) {
	plan, ok := ParseKeyset(cp)
	if !ok || plan.Mode != ModeIncrementalBackfill {
		return cp, false
	}
	cols, colsOK := decodeStrings(plan.Meta[metaIncrementalCols])
	types, typesOK := decodeStrings(plan.Meta[metaIncrementalTypes])
	start, startOK := decodeStrings(plan.Meta[metaIncrementalStart])
	if !colsOK || !typesOK || !startOK || len(cols) == 0 || len(cols) != len(types) {
		return cp, false
	}
	next := KeysetCheckpoint{
		Mode: ModeIncremental, Cols: cols, Types: types,
		Shards: []KeysetShard{{Key: start}},
		Meta:   map[string]string{metaIncrementalLookback: plan.Meta[metaIncrementalLookback]},
	}
	return next.ToCheckpoint(cp.Resource()), true
}

func encodeStrings(values []string) string {
	b, _ := json.Marshal(values)
	return string(b)
}

func decodeStrings(value string) ([]string, bool) {
	var out []string
	if value == "" || json.Unmarshal([]byte(value), &out) != nil {
		return nil, false
	}
	return out, true
}

// NewShardDelta is the per-batch checkpoint delta: shard part advanced to key.
func NewShardDelta(resource string, part int, key []string) *filament.CheckpointData {
	return &filament.CheckpointData{ResourceName: resource, Cursor: map[string]any{
		"mode": ModeKeyset,
		"part": part,
		"key":  strsToAny(key),
	}}
}

// MergeShardDelta applies a NewShardDelta onto base, advancing one shard's key.
// base must carry the shard layout from the plan; a delta for an out-of-range part is ignored.
func MergeShardDelta(base, delta filament.Checkpoint) filament.Checkpoint {
	if delta == nil {
		return base
	}
	d := delta.Raw()
	if mode, _ := d["mode"].(string); mode != ModeKeyset {
		return base
	}
	part := anyToInt(d["part"])
	key := anyToStrs(d["key"])

	ks, ok := ParseKeyset(base)
	if !ok {
		return base
	}
	if part < 0 || part >= len(ks.Shards) {
		return base
	}
	ks.Shards[part].Key = key
	return ks.ToCheckpoint(base.Resource())
}

// NewStreamDelta is the per-batch checkpoint of a change-stream read: the stream
// position (lsn) of the batch's last record and its per-run sequence. It is both
// the delta and the full checkpoint — a stream cursor has no layout to merge into.
func NewStreamDelta(resource, lsn string, seq uint64) *filament.CheckpointData {
	return &filament.CheckpointData{ResourceName: resource, Cursor: map[string]any{
		"mode": ModeStream,
		"lsn":  lsn,
		"seq":  int64(seq), //nolint:gosec // per-run batch counter, never near int64 max
	}}
}

// ParseStream decodes a stream cursor. Reports false when cp is nil or a different
// cursor shape. Tolerates a missing mode when an lsn is present (the batcher's
// legacy shape carried lsn/seq only).
func ParseStream(cp filament.Checkpoint) (lsn string, seq uint64, ok bool) {
	if cp == nil {
		return "", 0, false
	}
	raw := cp.Raw()
	mode, hasMode := raw["mode"].(string)
	lsn, _ = raw["lsn"].(string)
	if lsn == "" || (hasMode && mode != ModeStream) {
		return "", 0, false
	}
	return lsn, uint64(anyToInt(raw["seq"])), true //nolint:gosec // seq written as a non-negative int64 in NewStreamDelta
}

// MergeStream applies a stream delta onto base, keeping the newer position. The
// per-run seq guards against an out-of-order fold (concurrent writers can publish
// batch facts out of order); the higher seq wins, a tie keeps the delta.
func MergeStream(base, delta filament.Checkpoint) filament.Checkpoint {
	dLSN, dSeq, ok := ParseStream(delta)
	if !ok {
		return base
	}
	if _, bSeq, ok := ParseStream(base); ok && bSeq > dSeq {
		return base
	}
	return NewStreamDelta(delta.Resource(), dLSN, dSeq)
}

// coarseDeltaTag marks an ack/want completion delta — shared by bitmap and ctid, which
// both resume by per-shard Done rather than a key cursor.
const coarseDeltaTag = "coarse"

// NewCoarseAck is the per-batch completion delta for a coarse (bitmap/ctid) read: n rows of
// shard part were written. The tracker sums acks per part toward the part's expected total.
func NewCoarseAck(resource string, part, n int) *filament.CheckpointData {
	return &filament.CheckpointData{ResourceName: resource, Cursor: map[string]any{
		"mode": coarseDeltaTag, "part": part, "ack": n,
	}}
}

// NewCoarseDone is the completion marker: shard part will deliver exactly n rows in total.
// When acks reach n the tracker flags the shard Done.
func NewCoarseDone(resource string, part, n int) *filament.CheckpointData {
	return &filament.CheckpointData{ResourceName: resource, Cursor: map[string]any{
		"mode": coarseDeltaTag, "part": part, "want": n,
	}}
}

// CoarseDelta decodes a coarse ack/want delta. ok is false for any other cursor. hasWant
// distinguishes a completion marker (want set) from a plain ack.
func CoarseDelta(cp filament.Checkpoint) (part, ack, want int, hasWant, ok bool) {
	if cp == nil {
		return 0, 0, 0, false, false
	}
	raw := cp.Raw()
	if m, _ := raw["mode"].(string); m != coarseDeltaTag {
		return 0, 0, 0, false, false
	}
	_, hasWant = raw["want"]
	return anyToInt(raw["part"]), anyToInt(raw["ack"]), anyToInt(raw["want"]), hasWant, true
}

// ── encoding helpers ─────────────────────────────────────────────────────────

func strsToAny(s []string) []any {
	if len(s) == 0 {
		return nil
	}
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}

func anySlice(v any) []any {
	switch s := v.(type) {
	case []any:
		return s
	default:
		return nil
	}
}

func anyToStrs(v any) []string {
	switch s := v.(type) {
	case nil:
		return nil
	case []string:
		return s
	case []any:
		if len(s) == 0 {
			return nil
		}
		out := make([]string, len(s))
		for i, e := range s {
			out[i], _ = e.(string)
		}
		return out
	default:
		return nil
	}
}

func anyToStrMap(v any) map[string]string {
	m, ok := v.(map[string]any)
	if !ok || len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, vv := range m {
		if s, ok := vv.(string); ok {
			out[k] = s
		}
	}
	return out
}

func anyToInt(v any) int {
	switch n := v.(type) {
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
