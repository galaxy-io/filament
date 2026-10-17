package source

import (
	"testing"

	"github.com/galaxy-io/filament/rowmodel"
)

func TestOffsetCodec(t *testing.T) {
	c := OffsetCodec{}
	for _, n := range []int64{0, 1, 42, 1<<31 + 7} {
		p := position(n)
		if err := c.Validate(p); err != nil {
			t.Fatalf("validate %d: %v", n, err)
		}
		got, err := c.Canonicalize(p)
		if err != nil {
			t.Fatalf("canonicalize %d: %v", n, err)
		}
		if string(got.Value) != string(p.Value) {
			t.Fatalf("canonicalize %d = %q, want %q", n, got.Value, p.Value)
		}
	}
}

func TestOffsetCodecRejectsInvalid(t *testing.T) {
	c := OffsetCodec{}
	for _, p := range []rowmodel.Position{
		{Codec: PositionCodec, Version: 0, Value: []byte("-1")},
		{Codec: "other", Version: 0, Value: []byte("1")},
		{Codec: PositionCodec, Version: 1, Value: []byte("1")},
	} {
		if err := c.Validate(p); err == nil {
			t.Fatalf("expected invalid position: %+v", p)
		}
	}
}

func TestOffsetCodecOrdersOffsets(t *testing.T) {
	got, err := (OffsetCodec{}).Compare(position(9), position(10))
	if err != nil {
		t.Fatal(err)
	}
	if got != rowmodel.PositionBefore {
		t.Fatalf("compare = %v, want before", got)
	}
}
