package checkpoint

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/galaxy-io/filament"
)

// TestKeysetCheckpointRoundTrip checks the keyset cursor survives both an in-process
// pass and a JSON round-trip, since ParseKeyset must tolerate both []string and []any encodings.
func TestKeysetCheckpointRoundTrip(t *testing.T) {
	in := KeysetCheckpoint{
		Mode:  ModeKeyset,
		Cols:  []string{"id"},
		Types: []string{"bigint"},
		Shards: []KeysetShard{
			{Lo: nil, Hi: []string{"500"}, Key: []string{"123"}},
			{Lo: []string{"500"}, Hi: nil, Key: nil},
		},
	}
	cp := in.ToCheckpoint("orders")

	got, ok := ParseKeyset(cp)
	if !ok {
		t.Fatal("ParseKeyset: not recognized as keyset")
	}
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("in-process round-trip: got %+v, want %+v", got, in)
	}

	// JSON round-trip (DataStore persistence) then parse.
	raw, err := json.Marshal(cp.Raw())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got2, ok := ParseKeyset(&ingestion.CheckpointData{ResourceName: "orders", Cursor: decoded})
	if !ok {
		t.Fatal("ParseKeyset after JSON: not recognized")
	}
	if !reflect.DeepEqual(got2, in) {
		t.Fatalf("JSON round-trip: got %+v, want %+v", got2, in)
	}
}

// TestMergeShardDelta advances one shard's cursor without disturbing the layout or other
// shards, and ignores a delta for an out-of-range part.
func TestMergeShardDelta(t *testing.T) {
	base := KeysetCheckpoint{
		Cols:  []string{"id"},
		Types: []string{"bigint"},
		Shards: []KeysetShard{
			{Hi: []string{"500"}},
			{Lo: []string{"500"}},
		},
	}.ToCheckpoint("orders")

	merged := MergeShardDelta(base, NewShardDelta("orders", 1, []string{"742"}))
	ks, ok := ParseKeyset(merged)
	if !ok {
		t.Fatal("merged not keyset")
	}
	if got := ks.Shards[1].Key; !reflect.DeepEqual(got, []string{"742"}) {
		t.Fatalf("shard 1 key = %v, want [742]", got)
	}
	if ks.Shards[0].Key != nil {
		t.Fatalf("shard 0 key disturbed: %v", ks.Shards[0].Key)
	}
	if !reflect.DeepEqual(ks.Shards[1].Lo, []string{"500"}) {
		t.Fatalf("shard 1 lower bound lost: %v", ks.Shards[1].Lo)
	}

	// Out-of-range part is a no-op.
	same := MergeShardDelta(merged, NewShardDelta("orders", 9, []string{"x"}))
	if !reflect.DeepEqual(same.Raw(), merged.Raw()) {
		t.Fatal("out-of-range delta mutated the checkpoint")
	}

	// A delta onto a base with no layout is dropped (no shards invented).
	if got := MergeShardDelta(nil, NewShardDelta("orders", 0, []string{"1"})); got != nil {
		t.Fatalf("merge onto nil base = %v, want nil", got)
	}
}
