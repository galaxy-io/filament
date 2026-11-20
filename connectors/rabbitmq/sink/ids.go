package sink

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"

	"github.com/galaxy-io/filament/rowmodel"
)

// producerName is stable across run, attempt, and epoch so a retry reuses the
// broker deduplication sequence for one destination stream.
func producerName(tenant, pipeline, connection, streamName string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{tenant, pipeline, connection, streamName}, "\x00")))
	return "filament-" + hex.EncodeToString(sum[:8])
}

// watermarkFromQuery maps QueryPublisherSequence to the last stored publishing
// id. A missing sequence means the named producer has never published. A
// returned 0 is a real id, not an empty sequence. The protocol uses 0 for both
// only when the client collapses the missing-sequence error.
func watermarkFromQuery(last int64, err error) (int64, error) {
	if errors.Is(err, stream.OffsetNotFoundError) {
		return -1, nil
	}
	if err != nil {
		return 0, err
	}
	return last, nil
}

// assignPublishingIDs chooses one strictly increasing publishing id per row.
// sequences is nil when the batch has no _filament_sequence column. Those rows
// continue at watermark+1 and cannot be deduplicated on a later replay.
// When every sequence is at or below the watermark the same ids are returned
// so the broker drops the replay instead of storing a second copy.
func assignPublishingIDs(sequences []int64, watermark int64, rows int) ([]int64, error) {
	if rows == 0 {
		return nil, nil
	}
	if sequences == nil {
		if watermark == math.MaxInt64 {
			return nil, fmt.Errorf("rabbitmq sink: publishing id space exhausted")
		}
		ids := make([]int64, rows)
		next := watermark
		for i := range ids {
			if next == math.MaxInt64 {
				return nil, fmt.Errorf("rabbitmq sink: publishing id space exhausted")
			}
			next++
			ids[i] = next
		}
		return ids, nil
	}
	if len(sequences) != rows {
		return nil, fmt.Errorf("rabbitmq sink: sequence count differs from row count")
	}
	for i := 1; i < len(sequences); i++ {
		if sequences[i] <= sequences[i-1] {
			return nil, fmt.Errorf("rabbitmq sink: %s must strictly increase", rowmodel.AuditSequenceField)
		}
	}
	above := sequences[0] > watermark
	for _, id := range sequences[1:] {
		if (id > watermark) != above {
			return nil, fmt.Errorf("rabbitmq sink: publishing ids must be entirely above or entirely at or below the producer watermark %d", watermark)
		}
	}
	return append([]int64(nil), sequences...), nil
}
