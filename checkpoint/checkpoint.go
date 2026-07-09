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

import "github.com/galaxy-io/filament"

const (
	ModeKeyset = "keyset"
	ModeBitmap = "bitmap"
	ModeCtid   = "ctid"
	// ModeStream is a change-stream position cursor: a single monotonic location in
	// the source's replication log (Postgres WAL LSN, MySQL binlog file:pos) plus a
	// per-run sequence guard. Unlike the shard-based cursors above it has no layout;
	// resume simply restarts the stream at the recorded position.
	ModeStream = "stream"
)

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
func (k KeysetCheckpoint) ToCheckpoint(resource string) *ingestion.CheckpointData {
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
	return &ingestion.CheckpointData{ResourceName: resource, Cursor: cursor}
}

// ParseKeyset decodes a key-space checkpoint (keyset, bitmap, or ctid). Reports false when cp
// is nil or is a different cursor shape. Tolerates both in-process ([]string) and
// JSON-round-tripped ([]any) encodings.
func ParseKeyset(cp ingestion.Checkpoint) (KeysetCheckpoint, bool) {
	if cp == nil {
		return KeysetCheckpoint{}, false
	}
	raw := cp.Raw()
	mode, _ := raw["mode"].(string)
	if mode != ModeKeyset && mode != ModeBitmap && mode != ModeCtid {
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

// NewShardDelta is the per-batch checkpoint delta: shard part advanced to key.
func NewShardDelta(resource string, part int, key []string) *ingestion.CheckpointData {
	return &ingestion.CheckpointData{ResourceName: resource, Cursor: map[string]any{
		"mode": ModeKeyset,
		"part": part,
		"key":  strsToAny(key),
	}}
}

// MergeShardDelta applies a NewShardDelta onto base, advancing one shard's key.
// base must carry the shard layout from the plan; a delta for an out-of-range part is ignored.
func MergeShardDelta(base ingestion.Checkpoint, delta ingestion.Checkpoint) ingestion.Checkpoint {
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
func NewStreamDelta(resource, lsn string, seq uint64) *ingestion.CheckpointData {
	return &ingestion.CheckpointData{ResourceName: resource, Cursor: map[string]any{
		"mode": ModeStream,
		"lsn":  lsn,
		"seq":  int64(seq),
	}}
}

// ParseStream decodes a stream cursor. Reports false when cp is nil or a different
// cursor shape. Tolerates a missing mode when an lsn is present (the batcher's
// legacy shape carried lsn/seq only).
func ParseStream(cp ingestion.Checkpoint) (lsn string, seq uint64, ok bool) {
	if cp == nil {
		return "", 0, false
	}
	raw := cp.Raw()
	mode, hasMode := raw["mode"].(string)
	lsn, _ = raw["lsn"].(string)
	if lsn == "" || (hasMode && mode != ModeStream) {
		return "", 0, false
	}
	return lsn, uint64(anyToInt(raw["seq"])), true
}

// MergeStream applies a stream delta onto base, keeping the newer position. The
// per-run seq guards against an out-of-order fold (concurrent writers can publish
// batch facts out of order); the higher seq wins, a tie keeps the delta.
func MergeStream(base ingestion.Checkpoint, delta ingestion.Checkpoint) ingestion.Checkpoint {
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
func NewCoarseAck(resource string, part int, n int) *ingestion.CheckpointData {
	return &ingestion.CheckpointData{ResourceName: resource, Cursor: map[string]any{
		"mode": coarseDeltaTag, "part": part, "ack": n,
	}}
}

// NewCoarseDone is the completion marker: shard part will deliver exactly n rows in total.
// When acks reach n the tracker flags the shard Done.
func NewCoarseDone(resource string, part, n int) *ingestion.CheckpointData {
	return &ingestion.CheckpointData{ResourceName: resource, Cursor: map[string]any{
		"mode": coarseDeltaTag, "part": part, "want": n,
	}}
}

// CoarseDelta decodes a coarse ack/want delta. ok is false for any other cursor. hasWant
// distinguishes a completion marker (want set) from a plain ack.
func CoarseDelta(cp ingestion.Checkpoint) (part int, ack int, want int, hasWant bool, ok bool) {
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
