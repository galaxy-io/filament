package source

import (
	"context"
	"hash/crc32"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/twmb/franz-go/pkg/kmsg"
)

func TestBoundedCompactedTail(t *testing.T) {
	batch := kmsg.RecordBatch{FirstOffset: 5, Length: 49, Magic: 2, LastOffsetDelta: 4}
	raw := batch.AppendTo(nil)
	batch.CRC = int32(crc32.Checksum(raw[21:], crc32.MakeTable(crc32.Castagnoli)))
	part := boundedPartition{key: partitionKey{"events", 0}, start: 5, end: 10}
	for _, data := range [][]byte{nil, batch.AppendTo(nil)} {
		response := kmsg.FetchResponseTopicPartition{Partition: 0, HighWatermark: 10, LastStableOffset: 10, RecordBatches: data}
		records, next, err := processBounded(part, 5, &response)
		if err != nil || next != 10 || len(records) != 0 {
			t.Fatalf("compacted tail: records=%v next=%d err=%v", records, next, err)
		}
	}
}

func TestBoundedFetchFailsClosed(t *testing.T) {
	part := boundedPartition{key: partitionKey{"events", 0}, start: 5, end: 10}
	for name, response := range map[string]kmsg.FetchResponseTopicPartition{
		"retention":     {LogStartOffset: 6, LastStableOffset: 10},
		"truncation":    {LastStableOffset: 9},
		"broker error":  {ErrorCode: 1, LastStableOffset: 10},
		"partial batch": {LastStableOffset: 10, RecordBatches: []byte{1, 2, 3}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := processBounded(part, 5, &response); err == nil {
				t.Fatal("accepted unsafe fetch")
			}
		})
	}
}

func TestBoundedConfigurationAndCapabilities(t *testing.T) {
	s := New()
	if _, err := s.PlanResources(t.Context(), nil, nil); err == nil {
		t.Fatal("unconfigured source accepted")
	}
	if err := s.Extract(t.Context(), nil, filament.ExtractOpts{Limit: -1}); err == nil {
		t.Fatal("negative limit accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := s.PlanResources(ctx, nil, nil); err != context.Canceled {
		t.Fatalf("cancellation: %v", err)
	}
	spec := s.Spec()
	if len(spec.Modes) != 1 || spec.Modes[0] != filament.ModeFull || len(spec.SourcePolicies) == 0 || spec.Stream == nil {
		t.Fatalf("missing bounded/continuous capabilities: %+v", spec)
	}
}
