package sink

import (
	"errors"
	"math"
	"testing"

	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"
)

func TestWatermarkTreatsMissingSequenceAsUnset(t *testing.T) {
	got, err := watermarkFromQuery(0, stream.OffsetNotFoundError)
	if err != nil || got != -1 {
		t.Fatalf("missing sequence: %d %v", got, err)
	}
	got, err = watermarkFromQuery(0, nil)
	if err != nil || got != 0 {
		t.Fatalf("stored zero: %d %v", got, err)
	}
	got, err = watermarkFromQuery(7, nil)
	if err != nil || got != 7 {
		t.Fatalf("stored id: %d %v", got, err)
	}
	if _, err := watermarkFromQuery(0, errors.New("offline")); err == nil {
		t.Fatal("query error ignored")
	}
}

func TestAssignPublishingIDs(t *testing.T) {
	ids, err := assignPublishingIDs([]int64{1, 2, 3}, -1, 3)
	if err != nil || !sameIDs(ids, []int64{1, 2, 3}) {
		t.Fatalf("fresh sequence: %v %v", ids, err)
	}
	ids, err = assignPublishingIDs([]int64{1, 2, 3}, 3, 3)
	if err != nil || !sameIDs(ids, []int64{1, 2, 3}) {
		t.Fatalf("replay must keep original ids, got %v %v", ids, err)
	}
	ids, err = assignPublishingIDs([]int64{4, 5}, 3, 2)
	if err != nil || !sameIDs(ids, []int64{4, 5}) {
		t.Fatalf("ids above watermark: %v %v", ids, err)
	}
	ids, err = assignPublishingIDs(nil, 10, 2)
	if err != nil || !sameIDs(ids, []int64{11, 12}) {
		t.Fatalf("synthetic ids: %v %v", ids, err)
	}
	if _, err := assignPublishingIDs([]int64{1, 1}, -1, 2); err == nil {
		t.Fatal("non-increasing sequence accepted")
	}
	if _, err := assignPublishingIDs([]int64{2, 4}, 3, 2); err == nil {
		t.Fatal("mixed watermark accepted")
	}
	if _, err := assignPublishingIDs(nil, math.MaxInt64, 1); err == nil {
		t.Fatal("exhausted id space accepted")
	}
}

func sameIDs(got, want []int64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
